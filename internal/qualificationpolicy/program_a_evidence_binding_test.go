package qualificationpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/provider"
	"lsp-trace/internal/qualificationmatrix"
	"lsp-trace/internal/relations"
)

func TestProgramAA2CanonicalEvidenceComesOnlyFromProvisionOutput(t *testing.T) {
	declaration := programAProviderDeclaration("native@1.2.3")
	first, err := canonicalEffectiveConfiguration("custody:provider", []provider.Declaration{declaration})
	if err != nil {
		t.Fatal(err)
	}
	var decoded effectiveConfigurationEvidence
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Schema != effectiveConfigurationEvidenceSchema || decoded.Result != "PROVISIONED" || decoded.CustodyRef != "custody:provider" || len(decoded.Declarations) != 1 || decoded.Declarations[0].Identity != "native@1.2.3" || decoded.Declarations[0].Version != "1.2.3" || !reflect.DeepEqual(decoded.AllowedSelectors, []string{"auto", "none", "native@1.2.3"}) {
		t.Fatalf("ASSERT_A2_CANONICAL_COMPLETE_PROVISION_OUTPUT: %#v", decoded)
	}
	second, err := canonicalEffectiveConfiguration("custody:provider", []provider.Declaration{declaration})
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("ASSERT_A2_CANONICAL_ROUNDTRIP_DETERMINISTIC: equal=%v err=%v", reflect.DeepEqual(first, second), err)
	}
	changed := declaration
	changed.Version = "9.9.9"
	changed.Identity = "native@9.9.9"
	third, err := canonicalEffectiveConfiguration("custody:provider", []provider.Declaration{changed})
	if err != nil || reflect.DeepEqual(first, third) {
		t.Fatalf("ASSERT_A2_SEMANTIC_PROVIDER_OUTPUT_CHANGES_EVIDENCE: equal=%v err=%v", reflect.DeepEqual(first, third), err)
	}
	if _, ok := reflect.TypeOf(programAEvaluation{}.EffectiveConfiguration).FieldByName("Bytes"); ok {
		t.Fatal("ASSERT_A2_CALLER_EVIDENCE_SUBSTITUTION_SURFACE_REMOVED")
	}
}

func TestProgramAA3CanonicalEvidenceBindsSupportAccountingOutput(t *testing.T) {
	observations := []relations.Observation{{ID: "a", ExecutionID: "e1"}, {ID: "b", ExecutionID: "e1"}, {ID: "c"}}
	first, err := canonicalSupportAccounting(observations)
	if err != nil {
		t.Fatal(err)
	}
	var decoded supportAccountingEvidence
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Schema != supportAccountingEvidenceSchema || decoded.ComputationVersion != supportComputationVersion || decoded.Denominator != 3 || len(decoded.SupportGroups) != 2 || len(decoded.Omissions) != 0 || decoded.InputGraphDigest == "" {
		t.Fatalf("ASSERT_A3_EXACT_DENOMINATOR_GROUPS_QUALIFICATION_OMISSIONS: %#v", decoded)
	}
	for _, group := range decoded.SupportGroups {
		if group.Qualification != "MINIMUM_DEPENDENCE_CLASS" {
			t.Fatalf("ASSERT_A3_GROUP_QUALIFICATION_BOUND: %#v", group)
		}
	}
	changed := append([]relations.Observation(nil), observations...)
	changed[1].ExecutionID = "e2"
	second, err := canonicalSupportAccounting(changed)
	if err != nil || reflect.DeepEqual(first, second) {
		t.Fatalf("ASSERT_A3_SEMANTIC_OUTPUT_SUBSTITUTION_REJECTED: equal=%v err=%v", reflect.DeepEqual(first, second), err)
	}
	if _, ok := reflect.TypeOf(programAEvaluation{}.SupportAccounting).FieldByName("Bytes"); ok {
		t.Fatal("ASSERT_A3_CALLER_EVIDENCE_RELABEL_SURFACE_REMOVED")
	}
}

func TestProgramAA4RequiresCanonicalV2CompleteRealPassMatrix(t *testing.T) {
	profile, results := programAQualificationFixture(t)
	canonical, err := canonicalQualification(profile, qualificationmatrix.AdmissionRequest{Results: results})
	if err != nil {
		t.Fatalf("ASSERT_A4_CANONICAL_V2_RESULT_ACCEPTED: %v", err)
	}
	var decoded qualificationEvidence
	if err := json.Unmarshal(canonical, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ProfileSchema != qualificationmatrix.SchemaVersionV2 || decoded.ProfileDigest == "" || decoded.ResultMatrixDigest == "" || len(decoded.ProviderBindings) != len(results) || len(decoded.Statuses) != len(results) {
		t.Fatalf("ASSERT_A4_PROFILE_RESULT_DIGEST_AND_STATUS_BINDING: %#v", decoded)
	}

	v1 := profile
	v1.SchemaVersion = qualificationmatrix.SchemaVersion
	if _, err := canonicalQualification(v1, qualificationmatrix.AdmissionRequest{Results: results}); err == nil || !strings.Contains(err.Error(), "requires qualification profile v2") {
		t.Fatalf("ASSERT_A4_V1_HISTORICAL_VALIDATION_CANNOT_ISSUE: %v", err)
	}
	if _, err := canonicalQualification(profile, qualificationmatrix.AdmissionRequest{Results: results[:len(results)-1]}); err == nil || !strings.Contains(err.Error(), "missing required cell") {
		t.Fatalf("ASSERT_A4_MISSING_CELL_REJECTED: %v", err)
	}
	for name, mutate := range map[string]func([]qualificationmatrix.Result){
		"unknown":           func(rs []qualificationmatrix.Result) { rs[0].Status = qualificationmatrix.StatusUnknown },
		"waived":            func(rs []qualificationmatrix.Result) { rs[0].Waiver = &qualificationmatrix.Waiver{} },
		"synthetic":         func(rs []qualificationmatrix.Result) { rs[0].RealServerEvidence = false },
		"provider mismatch": func(rs []qualificationmatrix.Result) { rs[0].EvidenceProviderVersion = "swapped" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := append([]qualificationmatrix.Result(nil), results...)
			mutate(changed)
			if _, err := canonicalQualification(profile, qualificationmatrix.AdmissionRequest{Results: changed}); err == nil {
				t.Fatalf("ASSERT_A4_INVALID_STATUS_WAIVER_SYNTHETIC_OR_BINDING_REJECTED: %s", name)
			}
		})
	}
}

func TestProgramAReceiptDigestEqualsCanonicalVerifierOutput(t *testing.T) {
	profile, results := programAQualificationFixture(t)
	canonical, err := canonicalQualification(profile, qualificationmatrix.AdmissionRequest{Results: results})
	if err != nil {
		t.Fatal(err)
	}
	authority := evaluatorAuthorityForTest(t)
	receipt, err := authority.issue(QualificationDimension, retainedMetadata(QualificationDimension, "d45b469", "program-a", 4), canonical, testAssessmentContext("assessment-a4", "RANKING", "NORMATIVE_PROGRAM_A"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canonical)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if receipt.receipt.raw.EvidenceDigest != want {
		t.Fatalf("ASSERT_RECEIPT_DIGEST_EQUALS_CANONICAL_VERIFIER_BYTES: got=%s want=%s", receipt.receipt.raw.EvidenceDigest, want)
	}
}

func programAProviderDeclaration(identity string) provider.Declaration {
	version := strings.Split(identity, "@")[1]
	return provider.Declaration{
		Identity: identity, Version: version,
		Protocol:   provider.ProtocolIdentity{Name: "lsp-trace.provider-observations", Version: "1"},
		Executable: "/host/provider", ExecutableAvailable: true, ConformanceVerified: true,
		Capabilities: provider.Capabilities{Relations: []string{"CALLS"}, Languages: []string{"go"}, Frameworks: []string{"none"}},
		Limits:       provider.Limits{RequestBytes: 4096, ResponseBytes: 8192, ProtocolMessages: 2, StderrBytes: 128, WallTime: time.Second, TerminationGrace: time.Millisecond},
	}
}

func programAQualificationFixture(t *testing.T) (qualificationmatrix.Profile, []qualificationmatrix.Result) {
	t.Helper()
	axes := []qualificationmatrix.Axis{
		{Name: "relation_family", Members: []string{"CALLS"}}, {Name: "custody_adapter", Members: []string{"GIT_WORKTREE"}},
		{Name: "state", Members: []string{"COMPLETE"}}, {Name: "projection_class", Members: []string{"DIRECTED"}},
		{Name: "transport", Members: []string{"CLI"}}, {Name: "publication_mode", Members: []string{"IMMUTABLE"}},
		{Name: "provider_class", Members: []string{"GOPLS"}}, {Name: "provider_version", Members: []string{"0.20.0"}},
		{Name: "language", Members: []string{"GO"}}, {Name: "framework", Members: []string{"NONE"}},
	}
	profile := qualificationmatrix.Profile{
		SchemaVersion: qualificationmatrix.SchemaVersionV2, ProfileID: "program-a-v2", Version: "2.0.0", Authority: "qualification-authority",
		CustodyReceiptID: "custody-profile-v2", CustodyAuthenticationState: "AUTHENTICATED", Axes: axes,
		Products: []qualificationmatrix.Product{
			{ID: "relation", Version: "1", Axes: []string{"relation_family"}}, {ID: "custody", Version: "1", Axes: []string{"custody_adapter"}},
			{ID: "state", Version: "1", Axes: []string{"state"}}, {ID: "projection", Version: "1", Axes: []string{"projection_class"}},
			{ID: "publication", Version: "1", Axes: []string{"publication_mode"}},
			{ID: "provider", Version: "2", Axes: []string{"language", "provider_class", "provider_version", "framework", "transport"}},
		},
	}
	cells, err := qualificationmatrix.Generate(profile)
	if err != nil {
		t.Fatal(err)
	}
	profile.FoundationalCellIDs = make([]string, len(cells))
	results := make([]qualificationmatrix.Result, len(cells))
	for i, cell := range cells {
		profile.FoundationalCellIDs[i] = cell.ID
		results[i] = qualificationmatrix.Result{CellID: cell.ID, Status: qualificationmatrix.StatusPass, RealServerEvidence: true, EvidenceProviderClass: cell.Values["provider_class"], EvidenceProviderVersion: cell.Values["provider_version"]}
	}
	return profile, results
}
