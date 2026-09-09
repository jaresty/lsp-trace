package qualificationpolicy

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type SubstrateStatus string

const (
	SubstrateAdmitted SubstrateStatus = "ADMITTED"
	SubstrateMissing  SubstrateStatus = "MISSING"
	SubstrateRejected SubstrateStatus = "REJECTED"
)

type Dimension string

const (
	CustodyDimension                Dimension = "custody"
	EffectiveConfigurationDimension Dimension = "effective_configuration"
	IdentityDimension               Dimension = "identity"
	RelationNormalizationDimension  Dimension = "relation_normalization"
	SupportAccountingDimension      Dimension = "support_accounting"
	ProjectionDimension             Dimension = "projection"
	QualificationDimension          Dimension = "qualification_matrix"
)

const ReceiptSchemaVersion = "lsp-trace.program-a-substrate-receipt.v2"
const receiptSignatureDomain = "llsp-trace.program-a-evidence-receipt.v2\x00"

const (
	ProgramAAdmissionFamilyV2  = "lsp-trace.program-a-admission"
	ProgramAAdmissionVersionV1 = "lsp-trace.program-a-admission.v1"
	ProgramAAdmissionVersionV2 = "lsp-trace.program-a-admission.v2"
	programAAdmissionPolicyV2  = "lsp-trace.program-a-admission-policy.v2"
	programAEvidenceDomainV2   = "lsp-trace.program-a-evidence-set.v2\x00"
)

// AssessmentContext identifies the one admission evaluation for which receipts are valid.
// Receipts are immutable and intentionally re-verifiable offline in this exact context.
type AssessmentContext struct {
	AssessmentID           string
	Nonce                  string
	IssuanceEpoch          int64
	EvaluationScope        string
	AdmissionPolicyID      string
	AdmissionPolicyVersion string
	Operation              string
}

// hostProgramATrustProvisioning is canonical host-only input. It is deliberately
// unexported: arbitrary Go callers cannot turn self-created keys into authority.
type hostProgramATrustProvisioning struct {
	authorityID                     string
	keyID                           string
	policyID                        string
	provisioningReceipt             []byte
	pinnedProvisioningReceiptDigest string
	publicKey                       ed25519.PublicKey
}

// verifiedProgramAAuthority is opaque and can only result from host provisioning
// verification in this package (or the package-private ephemeral test seam).
type verifiedProgramAAuthority struct {
	authorityID               string
	keyID                     string
	policyID                  string
	provisioningReceiptDigest string
	publicKey                 ed25519.PublicKey
}

func provisionVerifiedProgramAAuthority(p hostProgramATrustProvisioning) (*verifiedProgramAAuthority, error) {
	if strings.TrimSpace(p.authorityID) == "" || strings.TrimSpace(p.keyID) == "" || strings.TrimSpace(p.policyID) == "" {
		return nil, fmt.Errorf("program A host provisioning requires authority, key, and policy identity")
	}
	if len(p.publicKey) != ed25519.PublicKeySize || len(p.provisioningReceipt) == 0 {
		return nil, fmt.Errorf("program A host provisioning requires receipt and Ed25519 public key")
	}
	receiptSum := sha256.Sum256(p.provisioningReceipt)
	receiptDigest := "sha256:" + hex.EncodeToString(receiptSum[:])
	if receiptDigest != p.pinnedProvisioningReceiptDigest {
		return nil, fmt.Errorf("program A provisioning receipt does not match independently pinned digest")
	}
	keySum := sha256.Sum256(p.publicKey)
	if p.keyID != "sha256:"+hex.EncodeToString(keySum[:]) {
		return nil, fmt.Errorf("program A key identity mismatch")
	}
	return &verifiedProgramAAuthority{authorityID: p.authorityID, keyID: p.keyID, policyID: p.policyID, provisioningReceiptDigest: receiptDigest, publicKey: append(ed25519.PublicKey(nil), p.publicKey...)}, nil
}

var evaluatorContract = map[Dimension]struct{ family, version string }{
	CustodyDimension:                {"custody", "v1"},
	EffectiveConfigurationDimension: {"effective-configuration", "v1"},
	IdentityDimension:               {"identity", "v1"},
	RelationNormalizationDimension:  {"relation-normalization", "v1"},
	SupportAccountingDimension:      {"support-accounting", "v1"},
	ProjectionDimension:             {"projection", "v1"},
	QualificationDimension:          {"qualification-matrix", "v2"},
}

// ReceiptBytes is the canonical signed output supplied by an independent evaluator.
type ReceiptBytes struct {
	Dimension                 Dimension
	EvaluatorID               string
	SchemaVersion             string
	Family                    string
	Version                   string
	Digest                    string
	CustodyRef                string
	Revision                  string
	SubstrateID               string
	Sequence                  int
	Status                    SubstrateStatus
	AuthorityID               string
	KeyID                     string
	ProvisioningReceiptDigest string
	AssessmentID              string
	Nonce                     string
	IssuanceEpoch             int64
	EvaluationScope           string
	AdmissionPolicyID         string
	AdmissionPolicyVersion    string
	Operation                 string
	EvidenceDigest            string
	Signature                 []byte
}

// VerifiedReceipt has no exported state; only a verifiedProgramAAuthority can issue one.
type VerifiedReceipt struct{ receipt verifiedReceipt }
type verifiedReceipt struct {
	raw           ReceiptBytes
	receiptDigest string
}

func (a *verifiedProgramAAuthority) VerifyReceipt(raw ReceiptBytes, expected AssessmentContext) (VerifiedReceipt, error) {
	if a == nil || a.authorityID == "" || a.keyID == "" || a.provisioningReceiptDigest == "" || len(a.publicKey) != ed25519.PublicKeySize {
		return VerifiedReceipt{}, fmt.Errorf("program A receipt authority is not provisioned")
	}
	contract, ok := evaluatorContract[raw.Dimension]
	if !ok {
		return VerifiedReceipt{}, fmt.Errorf("receipt dimension %q is unsupported", raw.Dimension)
	}
	if raw.EvaluatorID != "lsp-trace/"+contract.family+"-evaluator" {
		return VerifiedReceipt{}, fmt.Errorf("%s: wrong evaluator", raw.Dimension)
	}
	if raw.SchemaVersion != ReceiptSchemaVersion || raw.Family != contract.family || raw.Version != contract.version {
		return VerifiedReceipt{}, fmt.Errorf("%s: wrong schema/family/version", raw.Dimension)
	}
	if raw.Status != SubstrateAdmitted {
		return VerifiedReceipt{}, fmt.Errorf("%s: %s", raw.Dimension, raw.Status)
	}
	if strings.TrimSpace(raw.CustodyRef) == "" || strings.TrimSpace(raw.Revision) == "" || strings.TrimSpace(raw.SubstrateID) == "" || strings.TrimSpace(raw.EvidenceDigest) == "" {
		return VerifiedReceipt{}, fmt.Errorf("%s: missing custody/revision/substrate/evidence", raw.Dimension)
	}
	if raw.AuthorityID != a.authorityID || raw.KeyID != a.keyID || raw.ProvisioningReceiptDigest != a.provisioningReceiptDigest || raw.AdmissionPolicyID != a.policyID {
		return VerifiedReceipt{}, fmt.Errorf("%s: authority provisioning mismatch", raw.Dimension)
	}
	if raw.AssessmentID != expected.AssessmentID || raw.Nonce != expected.Nonce || raw.IssuanceEpoch != expected.IssuanceEpoch || raw.EvaluationScope != expected.EvaluationScope || raw.AdmissionPolicyID != expected.AdmissionPolicyID || raw.AdmissionPolicyVersion != expected.AdmissionPolicyVersion || raw.Operation != expected.Operation {
		return VerifiedReceipt{}, fmt.Errorf("%s: assessment context mismatch", raw.Dimension)
	}
	for _, s := range receiptFields(raw) {
		if strings.ContainsRune(s, '\x00') {
			return VerifiedReceipt{}, fmt.Errorf("%s: receipt string contains NUL", raw.Dimension)
		}
	}
	payload := receiptPayload(raw)
	if raw.Digest != receiptDigest(raw) {
		return VerifiedReceipt{}, fmt.Errorf("%s: digest mismatch", raw.Dimension)
	}
	if !ed25519.Verify(a.publicKey, payload, raw.Signature) {
		return VerifiedReceipt{}, fmt.Errorf("%s: evaluator signature mismatch", raw.Dimension)
	}
	return VerifiedReceipt{receipt: verifiedReceipt{raw: raw, receiptDigest: raw.Digest}}, nil
}

func receiptFields(raw ReceiptBytes) []string {
	return []string{string(raw.Dimension), raw.EvaluatorID, raw.SchemaVersion, raw.Family, raw.Version, raw.CustodyRef, raw.Revision, raw.SubstrateID, fmt.Sprint(raw.Sequence), string(raw.Status), raw.AuthorityID, raw.KeyID, raw.ProvisioningReceiptDigest, raw.AssessmentID, raw.Nonce, fmt.Sprint(raw.IssuanceEpoch), raw.EvaluationScope, raw.AdmissionPolicyID, raw.AdmissionPolicyVersion, raw.Operation, raw.EvidenceDigest}
}
func receiptPayload(raw ReceiptBytes) []byte {
	payload := []byte(receiptSignatureDomain)
	for _, field := range receiptFields(raw) {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(field)))
		payload = append(payload, size[:]...)
		payload = append(payload, field...)
	}
	return payload
}
func receiptDigest(raw ReceiptBytes) string {
	sum := sha256.Sum256(receiptPayload(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}

type VerifiedProgramASubstrate struct {
	Custody, EffectiveConfiguration, Identity            VerifiedReceipt
	RelationNormalization, SupportAccounting, Projection VerifiedReceipt
}

// VerifiedProgramASubstrateV2 is the explicit seven-receipt contract. Qualification
// remains opaque and cannot be folded into or substituted for a v1 evaluator axis.
type VerifiedProgramASubstrateV2 struct {
	Custody, EffectiveConfiguration, Identity                           VerifiedReceipt
	RelationNormalization, SupportAccounting, Projection, Qualification VerifiedReceipt
}

type ProgramAAdmission struct {
	Status                    SubstrateStatus `json:"status"`
	Reasons                   []string        `json:"reasons"`
	Revision                  string          `json:"revision,omitempty"`
	SubstrateID               string          `json:"substrate_id,omitempty"`
	authorityID               string
	provisioningReceiptDigest string
	keyID                     string
	assessmentID              string
	nonce                     string
	issuanceEpoch             int64
	evaluationScope           string
	admissionPolicyID         string
	admissionPolicyVersion    string
	operation                 string
	receiptDigests            [6]string
}

// ProgramAAdmissionV2 is distinct from v1 so receipt cardinality and digest
// domains cannot be confused or downgraded through assignment or shared helpers.
type ProgramAAdmissionV2 struct {
	Status                    SubstrateStatus `json:"status"`
	Reasons                   []string        `json:"reasons"`
	Revision                  string          `json:"revision,omitempty"`
	SubstrateID               string          `json:"substrate_id,omitempty"`
	authorityID               string
	provisioningReceiptDigest string
	keyID                     string
	assessmentID              string
	nonce                     string
	issuanceEpoch             int64
	evaluationScope           string
	admissionPolicyID         string
	admissionPolicyVersion    string
	operation                 string
	admissionFamily           string
	admissionVersion          string
	evidenceDomain            string
	receiptDigests            [7]string
}

type programAReceipt struct {
	dimension Dimension
	receipt   VerifiedReceipt
}

type programAComposition struct {
	status                                                                                                          SubstrateStatus
	reasons                                                                                                         []string
	revision, substrate, authority, key, provisioning, assessment, nonce, scope, policyID, policyVersion, operation string
	issuance                                                                                                        int64
	digests                                                                                                         []string
}

func AdmitVerifiedProgramA(input VerifiedProgramASubstrate) (ProgramAAdmission, error) {
	receipts := []programAReceipt{{CustodyDimension, input.Custody}, {EffectiveConfigurationDimension, input.EffectiveConfiguration}, {IdentityDimension, input.Identity}, {RelationNormalizationDimension, input.RelationNormalization}, {SupportAccountingDimension, input.SupportAccounting}, {ProjectionDimension, input.Projection}}
	c := composeProgramA(receipts)
	if c.status != SubstrateAdmitted {
		return ProgramAAdmission{Status: c.status, Reasons: c.reasons}, nil
	}
	var digests [6]string
	copy(digests[:], c.digests)
	return ProgramAAdmission{Status: c.status, Reasons: []string{}, Revision: c.revision, SubstrateID: c.substrate, authorityID: c.authority, keyID: c.key, provisioningReceiptDigest: c.provisioning, assessmentID: c.assessment, nonce: c.nonce, issuanceEpoch: c.issuance, evaluationScope: c.scope, admissionPolicyID: c.policyID, admissionPolicyVersion: c.policyVersion, operation: c.operation, receiptDigests: digests}, nil
}

func AdmitVerifiedProgramAV2(input VerifiedProgramASubstrateV2) (ProgramAAdmissionV2, error) {
	receipts := []programAReceipt{{CustodyDimension, input.Custody}, {EffectiveConfigurationDimension, input.EffectiveConfiguration}, {IdentityDimension, input.Identity}, {RelationNormalizationDimension, input.RelationNormalization}, {SupportAccountingDimension, input.SupportAccounting}, {ProjectionDimension, input.Projection}, {QualificationDimension, input.Qualification}}
	c := composeProgramA(receipts)
	if c.status != SubstrateAdmitted {
		return ProgramAAdmissionV2{Status: c.status, Reasons: c.reasons}, nil
	}
	if c.policyID != programAAdmissionPolicyV2 || c.policyVersion != "v2" {
		return ProgramAAdmissionV2{Status: SubstrateRejected, Reasons: []string{"admission: Program A v2 requires policy identity and version v2"}}, nil
	}
	var digests [7]string
	copy(digests[:], c.digests)
	return ProgramAAdmissionV2{Status: c.status, Reasons: []string{}, Revision: c.revision, SubstrateID: c.substrate, authorityID: c.authority, keyID: c.key, provisioningReceiptDigest: c.provisioning, assessmentID: c.assessment, nonce: c.nonce, issuanceEpoch: c.issuance, evaluationScope: c.scope, admissionPolicyID: c.policyID, admissionPolicyVersion: c.policyVersion, operation: c.operation, admissionFamily: ProgramAAdmissionFamilyV2, admissionVersion: ProgramAAdmissionVersionV2, evidenceDomain: programAEvidenceDomainV2, receiptDigests: digests}, nil
}

func composeProgramA(receipts []programAReceipt) programAComposition {
	reasons := make([]string, 0)
	var revision, substrate, authority, key, provisioning, assessment, nonce, scope, policyID, policyVersion, operation string
	var issuance int64
	digests := make([]string, len(receipts))
	for i, item := range receipts {
		raw := item.receipt.receipt.raw
		if raw.Dimension != item.dimension {
			reasons = append(reasons, string(item.dimension)+": no validated receipt")
			continue
		}
		if revision == "" {
			revision, substrate, authority, key, provisioning, assessment, nonce, issuance, scope, policyID, policyVersion, operation = raw.Revision, raw.SubstrateID, raw.AuthorityID, raw.KeyID, raw.ProvisioningReceiptDigest, raw.AssessmentID, raw.Nonce, raw.IssuanceEpoch, raw.EvaluationScope, raw.AdmissionPolicyID, raw.AdmissionPolicyVersion, raw.Operation
		}
		if raw.Revision != revision {
			reasons = append(reasons, string(item.dimension)+": revision mismatch")
		}
		if raw.SubstrateID != substrate {
			reasons = append(reasons, string(item.dimension)+": substrate mismatch")
		}
		if raw.AuthorityID != authority || raw.KeyID != key {
			reasons = append(reasons, string(item.dimension)+": authority mismatch")
		}
		if raw.ProvisioningReceiptDigest != provisioning {
			reasons = append(reasons, string(item.dimension)+": provisioning mismatch")
		}
		if raw.AssessmentID != assessment || raw.Nonce != nonce || raw.IssuanceEpoch != issuance {
			reasons = append(reasons, string(item.dimension)+": assessment mismatch")
		}
		if raw.EvaluationScope != scope || raw.AdmissionPolicyID != policyID || raw.AdmissionPolicyVersion != policyVersion || raw.Operation != operation {
			reasons = append(reasons, string(item.dimension)+": evaluation scope/policy mismatch")
		}
		digests[i] = item.receipt.receipt.receiptDigest
	}
	var n, s, p int
	for _, item := range receipts {
		switch item.dimension {
		case RelationNormalizationDimension:
			n = item.receipt.receipt.raw.Sequence
		case SupportAccountingDimension:
			s = item.receipt.receipt.raw.Sequence
		case ProjectionDimension:
			p = item.receipt.receipt.raw.Sequence
		}
	}
	if n >= s {
		reasons = append(reasons, "pipeline: relation_normalization must precede support_accounting")
	}
	if s >= p {
		reasons = append(reasons, "pipeline: support_accounting must precede projection")
	}
	sort.Strings(reasons)
	if len(reasons) != 0 {
		return programAComposition{status: SubstrateRejected, reasons: reasons}
	}
	return programAComposition{status: SubstrateAdmitted, reasons: []string{}, revision: revision, substrate: substrate, authority: authority, key: key, provisioning: provisioning, assessment: assessment, nonce: nonce, issuance: issuance, scope: scope, policyID: policyID, policyVersion: policyVersion, operation: operation, digests: digests}
}
