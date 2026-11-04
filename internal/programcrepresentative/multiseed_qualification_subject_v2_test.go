package programcrepresentative

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func multiSeedSubjectExpectedFixtureV2(t *testing.T) QualificationSubjectExpectedContextV2 {
	t.Helper()
	d := rawDigest([]byte("controlled-only-subject"))
	raw := []byte("controlled test-only implementation source\n")
	h := sha1.New()
	fmt.Fprintf(h, "blob %d%c", len(raw), byte(0))
	h.Write(raw)
	implementation := Stage6ImplementationDTO{RepositoryCommit: strings.Repeat("c", 40), RepositoryTree: strings.Repeat("d", 40), Files: []ImplementationFileDTO{{Path: "internal/programcrepresentative/test-only.go", GitBlobSHA1: fmt.Sprintf("%x", h.Sum(nil)), SHA256: rawDigest(raw), ByteLength: uint64(len(raw))}}}
	n, err := CanonicalSizeStage6ImplementationDTO(implementation)
	pre, err := marshalSizedV2(implementation, n, err)
	if err != nil {
		t.Fatal(err)
	}
	implementation.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-stage6-implementation:v2", pre)
	adapter, err := BuildAdapterContractV2("leiden/v1")
	if err != nil {
		t.Fatal(err)
	}
	return QualificationSubjectExpectedContextV2{AdapterContract: adapter, Stage6Implementation: implementation, CorpusIdentity: CorpusIdentityDTO{FrozenCorpusSHA256: d, CensusReceiptSHA256: d, AvailabilityLedgerSHA256: d, ProgramCPlanSHA256: d, ProgramCBindingsSHA256: d}, WorkflowIdentity: QualificationWorkflowIdentityDTO{Profile: ProfileCallsOnlyFrozenCensusV1, Contract: "adr0012-private-successor-v1", CensusReceiptSHA256: d, AvailabilityLedgerSHA256: d, SourcePinSHA256: d, ProgramCPlanSHA256: d, ProgramCBindingsSHA256: d, PrivacyPolicySHA256: d, RetentionPolicySHA256: d, ResourcePolicySHA256: d, ReplayPolicySHA256: d, QualificationPolicySHA256: d, Provenance: "PREDECESSOR_RETAINED_CALLER_ASSERTED"}, SourcePinSHA256: d, ReceiptInventory: []ReceiptInventoryEntryDTO{{PlanOrdinal: 0, ReceiptOrdinal: 1, ReceiptSchemaVersion: programCReceiptSchemaV4, ReceiptSelector: d, ReceiptSHA256: d, ReceiptByteLength: 100}}, TerminalProgramCHead: TerminalProgramCHeadDTO{SchemaVersion: programCHeadSchemaVersion, WorkflowKey: "census", HeadKey: "census-slot", Generation: 3, Selector: d, HeadBytesSHA256: d, ByteLength: 100, CommittedPrefix: 1, ConstituentCount: 1}, MaxImplementationSourceBytes: 1000}
}
func TestMultiSeedV2SubjectPureWireAndCopy(t *testing.T) {
	t.Log("procedure: test-local expected context -> pure subject bytes -> strict untrusted parse; mutate arrays without changing original")
	in := multiSeedSubjectExpectedFixtureV2(t)
	s, err := newQualificationSubjectV2FromExpected(in)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := s.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseProgramCRepresentativeQualificationSubjectV2(raw)
	if err != nil || parsed.Trusted() {
		t.Fatalf("STRICT_V2_SUBJECT_PARSE_UNTRUSTED: %v", err)
	}
	dto, err := s.DTO()
	if err != nil {
		t.Fatal(err)
	}
	if dto.MechanicsSchemaVersion != MechanicsSchemaVersionV2 || dto.AcceptedDownstreamAmendmentSHA256 != AcceptedDownstreamAmendmentSHA256V2 || dto.AdapterContract.SchemaVersion != AdapterContractSchemaVersionV2 {
		t.Fatal("EXACT_V2_SUBJECT_REGISTRY")
	}
	dto.ReceiptInventory[0].ReceiptSelector = "changed"
	dto.AdapterContract.Domains[0] = "changed"
	dto.Stage6Implementation.Files[0].Path = "changed"
	in.AdapterContract.Domains[0] = "input-changed"
	in.ReceiptInventory[0].ReceiptSelector = "input-changed"
	in.Stage6Implementation.Files[0].Path = "input-changed"
	again, _ := s.CanonicalBytes()
	copy, _ := s.DTO()
	if !bytes.Equal(raw, again) || copy.ReceiptInventory[0].ReceiptSelector == "changed" || copy.Stage6Implementation.Files[0].Path == "changed" || copy.AdapterContract.Domains[0] == "changed" {
		t.Fatal("DEFENSIVE_V2_SUBJECT_COPY")
	}
	for _, bad := range [][]byte{append(append([]byte{}, raw...), '\n'), bytes.Replace(raw, []byte(`"profile":`), []byte(`"Profile":`), 1), bytes.Replace(raw, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1), bytes.Replace(raw, []byte(QualificationSubjectSchemaVersionV2), []byte(QualificationSubjectSchemaVersion), 1)} {
		if _, err := ParseProgramCRepresentativeQualificationSubjectV2(bad); err == nil {
			t.Fatal("STRICT_V2_SUBJECT_REFUSAL")
		}
	}
}
func TestMultiSeedV2SubjectRequiresRealReviewedPins(t *testing.T) {
	t.Log("procedure: public subject constructor must refuse a test-local manifest; no old v1 pin or caller override can grant issuance")
	in := multiSeedSubjectExpectedFixtureV2(t)
	s, err := NewProgramCRepresentativeQualificationSubjectV2(in)
	if !errors.Is(err, ErrMultiSeedV2ReviewRequired) || s.Trusted() || s.Identity() != "" {
		t.Fatalf("REAL_REVIEW_REQUIRED_ZERO_SUBJECT: trusted=%t err=%v", s.Trusted(), err)
	}
	manifest, err := BuildStage6ImplementationV2(1000)
	if !errors.Is(err, ErrMultiSeedV2ReviewRequired) || manifest.Identity != "" {
		t.Fatal("NO_FABRICATED_V2_IMPLEMENTATION_PINS")
	}
}
