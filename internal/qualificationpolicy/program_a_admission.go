package qualificationpolicy

import (
	"crypto/ed25519"
	"crypto/sha256"
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
)

const ReceiptSchemaVersion = "lsp-trace.program-a-substrate-receipt.v1"

var evaluatorPublicKey = ed25519.PublicKey{0xd7, 0x5a, 0x98, 0x01, 0x82, 0xb1, 0x0a, 0xb7, 0xd5, 0x4b, 0xfe, 0xd3, 0xc9, 0x64, 0x07, 0x3a, 0x0e, 0xe1, 0x72, 0xf3, 0xda, 0xa6, 0x23, 0x25, 0xaf, 0x02, 0x1a, 0x68, 0xf7, 0x07, 0x51, 0x1a}

var evaluatorContract = map[Dimension]struct{ family, version string }{
	CustodyDimension:                {"custody", "v1"},
	EffectiveConfigurationDimension: {"effective-configuration", "v1"},
	IdentityDimension:               {"identity", "v1"},
	RelationNormalizationDimension:  {"relation-normalization", "v1"},
	SupportAccountingDimension:      {"support-accounting", "v1"},
	ProjectionDimension:             {"projection", "v1"},
}

// ReceiptBytes is the canonical output supplied by an independent evaluator.
type ReceiptBytes struct {
	Dimension     Dimension
	EvaluatorID   string
	SchemaVersion string
	Family        string
	Version       string
	Digest        string
	CustodyRef    string
	Revision      string
	SubstrateID   string
	Sequence      int
	Status        SubstrateStatus
	Signature     []byte
}

// VerifiedReceipt has no exported state; only VerifyReceipt can issue one.
type VerifiedReceipt struct{ receipt verifiedReceipt }
type verifiedReceipt struct{ raw ReceiptBytes }

func VerifyReceipt(raw ReceiptBytes) (VerifiedReceipt, error) {
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
	if strings.TrimSpace(raw.CustodyRef) == "" || strings.TrimSpace(raw.Revision) == "" || strings.TrimSpace(raw.SubstrateID) == "" {
		return VerifiedReceipt{}, fmt.Errorf("%s: missing custody/revision/substrate", raw.Dimension)
	}
	payload := receiptPayload(raw)
	if raw.Digest != receiptDigest(raw) {
		return VerifiedReceipt{}, fmt.Errorf("%s: digest mismatch", raw.Dimension)
	}
	if !ed25519.Verify(evaluatorPublicKey, payload, raw.Signature) {
		return VerifiedReceipt{}, fmt.Errorf("%s: evaluator signature mismatch", raw.Dimension)
	}
	return VerifiedReceipt{receipt: verifiedReceipt{raw: raw}}, nil
}

func receiptPayload(raw ReceiptBytes) []byte {
	return []byte(strings.Join([]string{string(raw.Dimension), raw.EvaluatorID, raw.SchemaVersion, raw.Family, raw.Version, raw.CustodyRef, raw.Revision, raw.SubstrateID, fmt.Sprint(raw.Sequence), string(raw.Status)}, "\x00"))
}
func receiptDigest(raw ReceiptBytes) string {
	sum := sha256.Sum256(receiptPayload(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}

type VerifiedProgramASubstrate struct {
	Custody, EffectiveConfiguration, Identity            VerifiedReceipt
	RelationNormalization, SupportAccounting, Projection VerifiedReceipt
}

type ProgramAAdmission struct {
	Status      SubstrateStatus `json:"status"`
	Reasons     []string        `json:"reasons"`
	Revision    string          `json:"revision,omitempty"`
	SubstrateID string          `json:"substrate_id,omitempty"`
}

func AdmitVerifiedProgramA(input VerifiedProgramASubstrate) (ProgramAAdmission, error) {
	receipts := []struct {
		dimension Dimension
		receipt   VerifiedReceipt
	}{
		{CustodyDimension, input.Custody}, {EffectiveConfigurationDimension, input.EffectiveConfiguration}, {IdentityDimension, input.Identity},
		{RelationNormalizationDimension, input.RelationNormalization}, {SupportAccountingDimension, input.SupportAccounting}, {ProjectionDimension, input.Projection},
	}
	reasons := make([]string, 0)
	var revision, substrate string
	for _, item := range receipts {
		raw := item.receipt.receipt.raw
		if raw.Dimension != item.dimension {
			reasons = append(reasons, string(item.dimension)+": no validated receipt")
			continue
		}
		if revision == "" {
			revision, substrate = raw.Revision, raw.SubstrateID
		}
		if raw.Revision != revision {
			reasons = append(reasons, string(item.dimension)+": revision mismatch")
		}
		if raw.SubstrateID != substrate {
			reasons = append(reasons, string(item.dimension)+": substrate mismatch")
		}
	}
	n, s, p := input.RelationNormalization.receipt.raw.Sequence, input.SupportAccounting.receipt.raw.Sequence, input.Projection.receipt.raw.Sequence
	if n >= s {
		reasons = append(reasons, "pipeline: relation_normalization must precede support_accounting")
	}
	if s >= p {
		reasons = append(reasons, "pipeline: support_accounting must precede projection")
	}
	sort.Strings(reasons)
	if len(reasons) != 0 {
		return ProgramAAdmission{Status: SubstrateRejected, Reasons: reasons}, nil
	}
	return ProgramAAdmission{Status: SubstrateAdmitted, Reasons: []string{}, Revision: revision, SubstrateID: substrate}, nil
}
