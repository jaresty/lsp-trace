package qualificationpolicy

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"lsp-trace/internal/custodyevidence"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/provider"
	"lsp-trace/internal/qualificationmatrix"
	"lsp-trace/internal/relations"
	"lsp-trace/internal/schema"
)

// programAEvaluatorAuthority is deliberately package-private. Production cannot
// evaluate or issue Program A evidence until the host supplies authenticated
// provisioning plus the matching private evaluator key.
type programAEvaluatorAuthority struct {
	verified *verifiedProgramAAuthority
	private  ed25519.PrivateKey
}

func newProgramAEvaluatorAuthority(provisioning hostProgramATrustProvisioning, private ed25519.PrivateKey) (*programAEvaluatorAuthority, error) {
	verified, err := provisionVerifiedProgramAAuthority(provisioning)
	if err != nil {
		return nil, err
	}
	if len(private) != ed25519.PrivateKeySize || !verified.publicKey.Equal(private.Public()) {
		return nil, fmt.Errorf("program A evaluator key does not match authenticated provisioning")
	}
	return &programAEvaluatorAuthority{verified: verified, private: append(ed25519.PrivateKey(nil), private...)}, nil
}

type retainedProgramAEvidence struct {
	CustodyRef  string
	Revision    string
	SubstrateID string
	Sequence    int
	Bytes       []byte
}

type programAEvaluation struct {
	Context                AssessmentContext
	Custody                retainedProgramAEvidence
	EffectiveConfiguration struct {
		retainedProgramAEvidence
		Declarations []provider.Declaration
	}
	Identity              retainedProgramAEvidence
	RelationNormalization retainedProgramAEvidence
	SupportAccounting     struct {
		retainedProgramAEvidence
		Observations []relations.Observation
	}
	Projection retainedProgramAEvidence
	Profile    qualificationmatrix.Profile
}

type programAEvaluationResult struct {
	Admission   ProgramAAdmission
	MissingAxes []Dimension
	FailedAxes  map[Dimension]string
}

func (a *programAEvaluatorAuthority) evaluateProgramA(in programAEvaluation) programAEvaluationResult {
	result := programAEvaluationResult{FailedAxes: map[Dimension]string{}}
	if a == nil || a.verified == nil || len(a.private) != ed25519.PrivateKeySize {
		result.FailedAxes[CustodyDimension] = "authenticated evaluator authority is unavailable"
		result.Admission = ProgramAAdmission{Status: SubstrateRejected, Reasons: []string{"authority: authenticated evaluator authority is unavailable"}}
		return result
	}

	type candidate struct {
		dimension Dimension
		evidence  retainedProgramAEvidence
		validate  func() error
	}
	candidates := []candidate{
		{CustodyDimension, in.Custody, func() error {
			_, err := custodyevidence.ValidateFor(in.Custody.Bytes, schema.FamilyOperationalCustody, "v1")
			return err
		}},
		{EffectiveConfigurationDimension, in.EffectiveConfiguration.retainedProgramAEvidence, func() error {
			_, err := provider.Provision(in.EffectiveConfiguration.Declarations)
			return err
		}},
		{IdentityDimension, in.Identity, func() error { return graph.ValidateSemanticBundle(in.Identity.Bytes) }},
		{RelationNormalizationDimension, in.RelationNormalization, func() error { return graph.ValidateNormalizedRelationsJSON(in.RelationNormalization.Bytes) }},
		{SupportAccountingDimension, in.SupportAccounting.retainedProgramAEvidence, func() error {
			if len(in.SupportAccounting.Observations) == 0 {
				return fmt.Errorf("support accounting requires retained observations")
			}
			_, err := relations.MinimumDependence(in.SupportAccounting.Observations)
			return err
		}},
		{ProjectionDimension, in.Projection, func() error { return schema.ValidateAllSeedInspection(in.Projection.Bytes) }},
	}

	verified := VerifiedProgramASubstrate{}
	for _, candidate := range candidates {
		if missingRetainedEvidence(candidate.evidence) {
			result.MissingAxes = append(result.MissingAxes, candidate.dimension)
			continue
		}
		if err := candidate.validate(); err != nil {
			result.FailedAxes[candidate.dimension] = err.Error()
			continue
		}
		receipt, err := a.issue(candidate.dimension, candidate.evidence, in.Context)
		if err != nil {
			result.FailedAxes[candidate.dimension] = err.Error()
			continue
		}
		setProgramAReceipt(&verified, candidate.dimension, receipt)
	}
	if err := qualificationmatrix.ValidateProfile(in.Profile); err != nil {
		result.FailedAxes[Dimension("qualification_matrix")] = err.Error()
	}
	if len(result.MissingAxes) != 0 || len(result.FailedAxes) != 0 {
		reasons := make([]string, 0, len(result.MissingAxes)+len(result.FailedAxes))
		for _, dimension := range result.MissingAxes {
			reasons = append(reasons, string(dimension)+": missing retained evidence")
		}
		for dimension, reason := range result.FailedAxes {
			reasons = append(reasons, string(dimension)+": "+reason)
		}
		result.Admission = rejectedProgramA(reasons)
		return result
	}
	admission, err := AdmitVerifiedProgramA(verified)
	if err != nil {
		result.FailedAxes[Dimension("composition")] = err.Error()
		result.Admission = rejectedProgramA([]string{"composition: " + err.Error()})
		return result
	}
	result.Admission = admission
	return result
}

func missingRetainedEvidence(e retainedProgramAEvidence) bool {
	return strings.TrimSpace(e.CustodyRef) == "" || strings.TrimSpace(e.Revision) == "" || strings.TrimSpace(e.SubstrateID) == "" || len(e.Bytes) == 0
}

func (a *programAEvaluatorAuthority) issue(d Dimension, evidence retainedProgramAEvidence, context AssessmentContext) (VerifiedReceipt, error) {
	contract := evaluatorContract[d]
	sum := sha256.Sum256(evidence.Bytes)
	raw := ReceiptBytes{
		Dimension: d, EvaluatorID: "lsp-trace/" + contract.family + "-evaluator", SchemaVersion: ReceiptSchemaVersion,
		Family: contract.family, Version: contract.version, CustodyRef: evidence.CustodyRef, Revision: evidence.Revision,
		SubstrateID: evidence.SubstrateID, Sequence: evidence.Sequence, Status: SubstrateAdmitted,
		AuthorityID: a.verified.authorityID, KeyID: a.verified.keyID, ProvisioningReceiptDigest: a.verified.provisioningReceiptDigest,
		AssessmentID: context.AssessmentID, Nonce: context.Nonce, IssuanceEpoch: context.IssuanceEpoch,
		EvaluationScope: context.EvaluationScope, AdmissionPolicyID: context.AdmissionPolicyID,
		AdmissionPolicyVersion: context.AdmissionPolicyVersion, Operation: context.Operation,
		EvidenceDigest: "sha256:" + hex.EncodeToString(sum[:]),
	}
	raw.Digest = receiptDigest(raw)
	raw.Signature = ed25519.Sign(a.private, receiptPayload(raw))
	return a.verified.VerifyReceipt(raw, context)
}

func setProgramAReceipt(out *VerifiedProgramASubstrate, d Dimension, receipt VerifiedReceipt) {
	switch d {
	case CustodyDimension:
		out.Custody = receipt
	case EffectiveConfigurationDimension:
		out.EffectiveConfiguration = receipt
	case IdentityDimension:
		out.Identity = receipt
	case RelationNormalizationDimension:
		out.RelationNormalization = receipt
	case SupportAccountingDimension:
		out.SupportAccounting = receipt
	case ProjectionDimension:
		out.Projection = receipt
	}
}

func rejectedProgramA(reasons []string) ProgramAAdmission {
	stable := append([]string(nil), reasons...)
	sort.Strings(stable)
	return ProgramAAdmission{Status: SubstrateRejected, Reasons: stable}
}
