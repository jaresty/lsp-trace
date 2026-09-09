package qualificationpolicy

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

const (
	effectiveConfigurationEvidenceSchema = "lsp-trace.program-a-effective-configuration-evidence.v1"
	supportAccountingEvidenceSchema      = "lsp-trace.program-a-support-accounting-evidence.v1"
	qualificationEvidenceSchema          = "lsp-trace.program-a-qualification-evidence.v1"
	supportComputationVersion            = "minimum-dependence.v1"
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

type retainedProgramAMetadata struct {
	CustodyRef  string
	Revision    string
	SubstrateID string
	Sequence    int
}

type retainedProgramAEvidence struct {
	retainedProgramAMetadata
	Bytes []byte
}

type programAEvaluation struct {
	Context                AssessmentContext
	Custody                retainedProgramAEvidence
	EffectiveConfiguration struct {
		retainedProgramAMetadata
		Declarations []provider.Declaration
	}
	Identity              retainedProgramAEvidence
	RelationNormalization retainedProgramAEvidence
	SupportAccounting     struct {
		retainedProgramAMetadata
		Observations []relations.Observation
	}
	Projection    retainedProgramAEvidence
	Qualification struct {
		retainedProgramAMetadata
		Profile qualificationmatrix.Profile
		Result  qualificationmatrix.AdmissionRequest
	}
}

type programAEvaluationResult struct {
	Admission   ProgramAAdmissionV2
	MissingAxes []Dimension
	FailedAxes  map[Dimension]string
}

type effectiveConfigurationEvidence struct {
	Schema           string                 `json:"schema"`
	CustodyRef       string                 `json:"custody_ref"`
	Declarations     []provider.Declaration `json:"declarations"`
	AllowedSelectors []string               `json:"allowed_selectors"`
	Result           string                 `json:"result"`
}

type supportGroupEvidence struct {
	Members       []string `json:"members"`
	Qualification string   `json:"qualification"`
}

type supportAccountingEvidence struct {
	Schema             string                 `json:"schema"`
	InputGraphDigest   string                 `json:"input_graph_digest"`
	ComputationVersion string                 `json:"computation_version"`
	Denominator        int                    `json:"denominator"`
	SupportGroups      []supportGroupEvidence `json:"support_groups"`
	Omissions          []string               `json:"omissions"`
}

type qualificationEvidence struct {
	Schema             string                               `json:"schema"`
	ProfileSchema      string                               `json:"profile_schema"`
	ProfileDigest      string                               `json:"profile_digest"`
	ResultMatrixDigest string                               `json:"result_matrix_digest"`
	ProviderBindings   []qualificationProviderBinding       `json:"provider_bindings"`
	Statuses           []qualificationStatusBinding         `json:"statuses"`
	Profile            qualificationmatrix.Profile          `json:"profile"`
	Result             qualificationmatrix.AdmissionRequest `json:"result"`
}

type qualificationProviderBinding struct {
	CellID          string `json:"cell_id"`
	ProviderClass   string `json:"provider_class"`
	ProviderVersion string `json:"provider_version"`
}

type qualificationStatusBinding struct {
	CellID string                     `json:"cell_id"`
	Status qualificationmatrix.Status `json:"status"`
}

func (a *programAEvaluatorAuthority) evaluateProgramA(in programAEvaluation) programAEvaluationResult {
	result := programAEvaluationResult{FailedAxes: map[Dimension]string{}}
	if a == nil || a.verified == nil || len(a.private) != ed25519.PrivateKeySize {
		result.FailedAxes[CustodyDimension] = "authenticated evaluator authority is unavailable"
		result.Admission = ProgramAAdmissionV2{Status: SubstrateRejected, Reasons: []string{"authority: authenticated evaluator authority is unavailable"}}
		return result
	}

	type candidate struct {
		dimension Dimension
		metadata  retainedProgramAMetadata
		bytes     func() ([]byte, error)
	}
	candidates := []candidate{
		{CustodyDimension, in.Custody.retainedProgramAMetadata, func() ([]byte, error) {
			_, err := custodyevidence.ValidateFor(in.Custody.Bytes, schema.FamilyOperationalCustody, "v1")
			return in.Custody.Bytes, err
		}},
		{EffectiveConfigurationDimension, in.EffectiveConfiguration.retainedProgramAMetadata, func() ([]byte, error) {
			return canonicalEffectiveConfiguration(in.EffectiveConfiguration.CustodyRef, in.EffectiveConfiguration.Declarations)
		}},
		{IdentityDimension, in.Identity.retainedProgramAMetadata, func() ([]byte, error) {
			return in.Identity.Bytes, graph.ValidateSemanticBundle(in.Identity.Bytes)
		}},
		{RelationNormalizationDimension, in.RelationNormalization.retainedProgramAMetadata, func() ([]byte, error) {
			return in.RelationNormalization.Bytes, graph.ValidateNormalizedRelationsJSON(in.RelationNormalization.Bytes)
		}},
		{SupportAccountingDimension, in.SupportAccounting.retainedProgramAMetadata, func() ([]byte, error) {
			return canonicalSupportAccounting(in.SupportAccounting.Observations)
		}},
		{ProjectionDimension, in.Projection.retainedProgramAMetadata, func() ([]byte, error) {
			return in.Projection.Bytes, schema.ValidateAllSeedInspection(in.Projection.Bytes)
		}},
		{QualificationDimension, in.Qualification.retainedProgramAMetadata, func() ([]byte, error) {
			return canonicalQualification(in.Qualification.Profile, in.Qualification.Result)
		}},
	}

	verified := VerifiedProgramASubstrateV2{}
	for _, candidate := range candidates {
		if missingRetainedMetadata(candidate.metadata) {
			result.MissingAxes = append(result.MissingAxes, candidate.dimension)
			continue
		}
		evidenceBytes, err := candidate.bytes()
		if err != nil {
			result.FailedAxes[candidate.dimension] = err.Error()
			continue
		}
		if len(evidenceBytes) == 0 {
			result.MissingAxes = append(result.MissingAxes, candidate.dimension)
			continue
		}
		receipt, err := a.issue(candidate.dimension, candidate.metadata, evidenceBytes, in.Context)
		if err != nil {
			result.FailedAxes[candidate.dimension] = err.Error()
			continue
		}
		setProgramAReceiptV2(&verified, candidate.dimension, receipt)
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
	admission, err := AdmitVerifiedProgramAV2(verified)
	if err != nil {
		result.FailedAxes[Dimension("composition")] = err.Error()
		result.Admission = rejectedProgramA([]string{"composition: " + err.Error()})
		return result
	}
	result.Admission = admission
	return result
}

func canonicalEffectiveConfiguration(custodyRef string, declarations []provider.Declaration) ([]byte, error) {
	provisioned, err := provider.Provision(declarations)
	if err != nil {
		return nil, err
	}
	out := effectiveConfigurationEvidence{Schema: effectiveConfigurationEvidenceSchema, CustodyRef: custodyRef, Declarations: provisioned.Declarations, AllowedSelectors: provisioned.Admission.Allowed(), Result: "PROVISIONED"}
	return canonicalRoundTrip(out, func(decoded effectiveConfigurationEvidence) error {
		if decoded.Schema != effectiveConfigurationEvidenceSchema || decoded.CustodyRef != custodyRef || decoded.Result != "PROVISIONED" {
			return fmt.Errorf("effective configuration canonical roundtrip identity mismatch")
		}
		reprovisioned, err := provider.Provision(decoded.Declarations)
		if err != nil {
			return err
		}
		if !equalJSON(reprovisioned.Declarations, provisioned.Declarations) || !equalStrings(reprovisioned.Admission.Allowed(), decoded.AllowedSelectors) {
			return fmt.Errorf("effective configuration canonical roundtrip result mismatch")
		}
		return nil
	})
}

func canonicalSupportAccounting(observations []relations.Observation) ([]byte, error) {
	if len(observations) == 0 {
		return nil, fmt.Errorf("support accounting requires retained observations")
	}
	classes, err := relations.MinimumDependence(observations)
	if err != nil {
		return nil, err
	}
	canonicalObservations := append([]relations.Observation(nil), observations...)
	sort.Slice(canonicalObservations, func(i, j int) bool { return canonicalObservations[i].ID < canonicalObservations[j].ID })
	for i := range canonicalObservations {
		sort.Strings(canonicalObservations[i].UpstreamObservationIDs)
	}
	graphBytes, err := json.Marshal(canonicalObservations)
	if err != nil {
		return nil, err
	}
	graphSum := sha256.Sum256(graphBytes)
	groups := make([]supportGroupEvidence, len(classes))
	for i, members := range classes {
		groups[i] = supportGroupEvidence{Members: append([]string(nil), members...), Qualification: "MINIMUM_DEPENDENCE_CLASS"}
	}
	out := supportAccountingEvidence{Schema: supportAccountingEvidenceSchema, InputGraphDigest: "sha256:" + hex.EncodeToString(graphSum[:]), ComputationVersion: supportComputationVersion, Denominator: len(observations), SupportGroups: groups, Omissions: []string{}}
	return canonicalRoundTrip(out, func(decoded supportAccountingEvidence) error {
		if decoded.Schema != supportAccountingEvidenceSchema || decoded.ComputationVersion != supportComputationVersion || decoded.Denominator != len(observations) || len(decoded.Omissions) != 0 || !equalJSON(decoded.SupportGroups, groups) {
			return fmt.Errorf("support accounting canonical roundtrip mismatch")
		}
		return nil
	})
}

func canonicalQualification(profile qualificationmatrix.Profile, result qualificationmatrix.AdmissionRequest) ([]byte, error) {
	if profile.SchemaVersion != qualificationmatrix.SchemaVersionV2 {
		return nil, fmt.Errorf("Program A A4 requires qualification profile v2")
	}
	profileBytes, err := qualificationmatrix.CanonicalBytes(profile)
	if err != nil {
		return nil, err
	}
	var canonicalProfile qualificationmatrix.Profile
	if err := json.Unmarshal(profileBytes, &canonicalProfile); err != nil {
		return nil, fmt.Errorf("decode canonical qualification profile: %w", err)
	}
	cells, err := qualificationmatrix.Generate(canonicalProfile)
	if err != nil {
		return nil, err
	}
	required := make(map[string]qualificationmatrix.Cell, len(cells))
	for _, cell := range cells {
		required[cell.ID] = cell
	}
	seen := make(map[string]bool, len(result.Results))
	canonicalResult := result
	canonicalResult.Results = append([]qualificationmatrix.Result(nil), result.Results...)
	canonicalResult.RequestedClaims = append([]string(nil), result.RequestedClaims...)
	canonicalResult.RequestedOperations = append([]string(nil), result.RequestedOperations...)
	sort.Slice(canonicalResult.Results, func(i, j int) bool { return canonicalResult.Results[i].CellID < canonicalResult.Results[j].CellID })
	sort.Strings(canonicalResult.RequestedClaims)
	sort.Strings(canonicalResult.RequestedOperations)
	bindings := make([]qualificationProviderBinding, 0, len(canonicalResult.Results))
	statuses := make([]qualificationStatusBinding, 0, len(canonicalResult.Results))
	for _, r := range canonicalResult.Results {
		cell, ok := required[r.CellID]
		if !ok || seen[r.CellID] {
			return nil, fmt.Errorf("qualification result has unknown or duplicate cell %q", r.CellID)
		}
		seen[r.CellID] = true
		if r.Status != qualificationmatrix.StatusPass || !r.RealServerEvidence || r.Waiver != nil {
			return nil, fmt.Errorf("qualification cell %q must be PASS with real-server evidence and no waiver", r.CellID)
		}
		if want := cell.Values["provider_class"]; want != "" && r.EvidenceProviderClass != want {
			return nil, fmt.Errorf("qualification cell %q provider class mismatch", r.CellID)
		}
		if want := cell.Values["provider_version"]; want != "" && r.EvidenceProviderVersion != want {
			return nil, fmt.Errorf("qualification cell %q provider version mismatch", r.CellID)
		}
		bindings = append(bindings, qualificationProviderBinding{CellID: r.CellID, ProviderClass: r.EvidenceProviderClass, ProviderVersion: r.EvidenceProviderVersion})
		statuses = append(statuses, qualificationStatusBinding{CellID: r.CellID, Status: r.Status})
	}
	for id := range required {
		if !seen[id] {
			return nil, fmt.Errorf("qualification result missing required cell %q", id)
		}
	}
	for _, id := range canonicalProfile.FoundationalCellIDs {
		if !seen[id] {
			return nil, fmt.Errorf("qualification result missing foundational cell %q", id)
		}
	}
	resultBytes, err := json.Marshal(canonicalResult)
	if err != nil {
		return nil, err
	}
	profileSum, resultSum := sha256.Sum256(profileBytes), sha256.Sum256(resultBytes)
	out := qualificationEvidence{Schema: qualificationEvidenceSchema, ProfileSchema: canonicalProfile.SchemaVersion, ProfileDigest: "sha256:" + hex.EncodeToString(profileSum[:]), ResultMatrixDigest: "sha256:" + hex.EncodeToString(resultSum[:]), ProviderBindings: bindings, Statuses: statuses, Profile: canonicalProfile, Result: canonicalResult}
	return canonicalRoundTrip(out, func(decoded qualificationEvidence) error {
		if decoded.Schema != qualificationEvidenceSchema || decoded.ProfileSchema != qualificationmatrix.SchemaVersionV2 || decoded.ProfileDigest != out.ProfileDigest || decoded.ResultMatrixDigest != out.ResultMatrixDigest || !equalJSON(decoded.ProviderBindings, bindings) || !equalJSON(decoded.Statuses, statuses) {
			return fmt.Errorf("qualification canonical roundtrip binding mismatch")
		}
		decodedProfile, err := qualificationmatrix.CanonicalBytes(decoded.Profile)
		if err != nil || !bytes.Equal(decodedProfile, profileBytes) {
			return fmt.Errorf("qualification canonical roundtrip profile mismatch")
		}
		decodedResult, err := json.Marshal(decoded.Result)
		if err != nil || !bytes.Equal(decodedResult, resultBytes) {
			return fmt.Errorf("qualification canonical roundtrip result mismatch")
		}
		return nil
	})
}

func canonicalRoundTrip[T any](value T, validate func(T) error) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	var decoded T
	dec := json.NewDecoder(bytes.NewReader(b.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&decoded); err != nil {
		return nil, err
	}
	if err := validate(decoded); err != nil {
		return nil, err
	}
	var roundTrip bytes.Buffer
	roundTripEncoder := json.NewEncoder(&roundTrip)
	roundTripEncoder.SetEscapeHTML(false)
	if err := roundTripEncoder.Encode(decoded); err != nil {
		return nil, err
	}
	if !bytes.Equal(b.Bytes(), roundTrip.Bytes()) {
		return nil, fmt.Errorf("canonical evidence bytes do not roundtrip")
	}
	return b.Bytes(), nil
}

func equalJSON(a, b any) bool {
	aBytes, aErr := json.Marshal(a)
	bBytes, bErr := json.Marshal(b)
	return aErr == nil && bErr == nil && bytes.Equal(aBytes, bBytes)
}

func equalStrings(a, b []string) bool {
	return equalJSON(a, b)
}

func missingRetainedMetadata(e retainedProgramAMetadata) bool {
	return strings.TrimSpace(e.CustodyRef) == "" || strings.TrimSpace(e.Revision) == "" || strings.TrimSpace(e.SubstrateID) == ""
}

func (a *programAEvaluatorAuthority) issue(d Dimension, metadata retainedProgramAMetadata, verifierOutput []byte, context AssessmentContext) (VerifiedReceipt, error) {
	contract := evaluatorContract[d]
	sum := sha256.Sum256(verifierOutput)
	raw := ReceiptBytes{
		Dimension: d, EvaluatorID: "lsp-trace/" + contract.family + "-evaluator", SchemaVersion: ReceiptSchemaVersion,
		Family: contract.family, Version: contract.version, CustodyRef: metadata.CustodyRef, Revision: metadata.Revision,
		SubstrateID: metadata.SubstrateID, Sequence: metadata.Sequence, Status: SubstrateAdmitted,
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

func setProgramAReceiptV2(out *VerifiedProgramASubstrateV2, d Dimension, receipt VerifiedReceipt) {
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
	case QualificationDimension:
		out.Qualification = receipt
	}
}

func rejectedProgramA(reasons []string) ProgramAAdmissionV2 {
	stable := append([]string(nil), reasons...)
	sort.Strings(stable)
	return ProgramAAdmissionV2{Status: SubstrateRejected, Reasons: stable}
}
