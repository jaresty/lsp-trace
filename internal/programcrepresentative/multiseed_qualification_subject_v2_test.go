package programcrepresentative

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	if err == nil || s.Trusted() || s.Identity() != "" {
		t.Fatalf("REAL_REVIEW_REQUIRED_ZERO_SUBJECT: trusted=%t err=%v", s.Trusted(), err)
	}
}

func TestReviewedNativeHarnessBindingsV2(t *testing.T) {
	t.Log("procedure: literal native actor roles, isolation, parent and exact raw projection pins; mutate returned data then reload")
	actors, err := ReviewedHarnessActorBindingsV2()
	if err != nil || len(actors) != 2 {
		t.Fatalf("ACTUAL_NATIVE_BINDINGS_ISSUED: actors=%d err=%v", len(actors), err)
	}
	want := []ReviewedHarnessActorBindingV2{
		{Actor: RecordedReviewActorV2{Kind: "PI_SESSION", ID: "01a1070d-1392-7797-8214-2c7836f6d572", IsolationKind: "PI_SESSION"}, Projection: ArtifactBindingDTO{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2", SHA256: "sha256:aaa334faa9232c1a92ac8f17209ac22ed8729025834a92508d0109e0e06064e6", ByteLength: 1755}},
		{Actor: RecordedReviewActorV2{Kind: "PI_ISOLATED_TASK", ID: "cf12200b-22aa-40e", ParentID: "01a1070d-1392-7797-8214-2c7836f6d572", IsolationKind: "PI_ISOLATED_TASK"}, Projection: ArtifactBindingDTO{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2", SHA256: "sha256:63e6b0da3ebd89bab3602360846f2f46a65f12cced6ce480eabf6b4e0cbff2dd", ByteLength: 1791}},
	}
	for i, a := range actors {
		w := want[i]
		if a.Actor.Kind != w.Actor.Kind || a.Actor.ID != w.Actor.ID || a.Actor.ParentID != w.Actor.ParentID || a.Actor.IsolationKind != w.Actor.IsolationKind || a.Actor.Projection != nil || a.Projection != w.Projection {
			t.Fatalf("ACTUAL_NATIVE_BINDING_METADATA: actor=%d", i)
		}
	}
	actors[0].Actor.ID = "mutated"
	actors[1].Actor.Projection = []byte("mutated")
	actors[0].Projection.SHA256 = "mutated"
	fresh, err := ReviewedHarnessActorBindingsV2()
	if err != nil || len(fresh) != 2 || fresh[0].Actor.ID != want[0].Actor.ID || fresh[1].Actor.Projection != nil || fresh[0].Projection != want[0].Projection {
		t.Fatal("FRESH_NATIVE_BINDING_LITERALS")
	}
	t.Log("ACTUAL_NATIVE_BINDINGS_ISSUED PASS; ACTUAL_NATIVE_BINDING_METADATA PASS; FRESH_NATIVE_BINDING_LITERALS PASS")
}

// Reads exactly the declaration's selected source paths, never a test-only pin.
type reviewedPinSourceV2 struct {
	files map[string]uint64
	wrong bool
	seen  int
}

func (r *reviewedPinSourceV2) ReadImplementationFileV2(name string, limit uint64) ([]byte, error) {
	n, ok := r.files[name]
	if !ok || limit != n {
		return nil, fmt.Errorf("UNSELECTED_OR_UNBOUNDED_SOURCE")
	}
	r.seen++
	f, err := os.Open(filepath.Join("..", "..", filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if uint64(len(raw)) != limit {
		return nil, fmt.Errorf("SOURCE_EXACT_READ_BOUND")
	}
	if r.wrong {
		raw[0] ^= 1
	}
	return raw, err
}
func TestReviewedImplementationPinsV2(t *testing.T) {
	t.Log("procedure: repository declaration -> canonical Go DTO identity -> exact selected source verification -> wrong bytes and aggregate off-by-one refusal")
	m, err := BuildStage6ImplementationV2(237568)
	if err != nil || m.Identity != "sha256:19481e7d36682e3d4c95a52121235f1139164e84f3221a20b59e04f6499a5b41" || m.RepositoryCommit != "f06e6fb2d2c38aca375bf7d8e622e6cb326272d1" || m.RepositoryTree != "abc78c0003d197d5fdcb003d76754968d3511d10" || len(m.Files) != 30 {
		t.Fatalf("REAL_MANIFEST_EXACT_DECLARATION: %v", err)
	}
	if err = validateImplementationShapeV2(m); err != nil {
		t.Fatalf("REAL_MANIFEST_GO_CANONICAL_IDENTITY: %v", err)
	}
	reader := &reviewedPinSourceV2{files: map[string]uint64{}}
	var total uint64
	for _, f := range m.Files {
		if strings.HasSuffix(f.Path, "_test.go") || strings.HasSuffix(f.Path, "multiseed_reviewed_bindings_v2.go") {
			t.Fatal("SELECTED_PRODUCTION_SEAMS_ONLY")
		}
		reader.files[f.Path] = f.ByteLength
		total += f.ByteLength
	}
	if total != 237568 {
		t.Fatal("EXACT_SELECTED_SOURCE_TOTAL")
	}
	if err = VerifyStage6ImplementationSourceV2(m, reader, total); err != nil || reader.seen != 30 {
		t.Fatalf("REAL_MANIFEST_EXACT_SOURCE: reads=%d err=%v", reader.seen, err)
	}
	for _, delta := range []int64{-1, 1} {
		if _, err := reader.ReadImplementationFileV2(m.Files[0].Path, uint64(int64(m.Files[0].ByteLength)+delta)); err == nil {
			t.Fatal("PER_READ_OFF_BY_ONE_REFUSED")
		}
	}
	reader.wrong = true
	if err = VerifyStage6ImplementationSourceV2(m, reader, total); err == nil {
		t.Fatal("WRONG_REAL_SOURCE_REFUSED")
	}
	in := multiSeedSubjectExpectedFixtureV2(t)
	in.Stage6Implementation, in.ImplementationSource, in.MaxImplementationSourceBytes = m, reader, total
	wrongSubject, wrongErr := NewProgramCRepresentativeQualificationSubjectV2(in)
	if wrongErr == nil || wrongSubject.Trusted() || wrongSubject.Identity() != "" {
		t.Fatal("WRONG_REAL_SOURCE_BEFORE_TRUST")
	}
	reader.wrong = false
	if err = VerifyStage6ImplementationSourceV2(m, reader, total-1); err == nil {
		t.Fatal("AGGREGATE_OFF_BY_ONE_REFUSED")
	}
	wrong := cloneStage6ImplementationDTO(m)
	wrong.Files[0].SHA256 = rawDigest([]byte("wrong"))
	reader.seen = 0
	if err = VerifyStage6ImplementationSourceV2(wrong, reader, total); err == nil || reader.seen != 0 {
		t.Fatal("WRONG_DECLARATION_BEFORE_READ")
	}
	fresh, _ := BuildStage6ImplementationV2(total)
	m.Files[0].Path = "mutated"
	if fresh.Files[0].Path == m.Files[0].Path {
		t.Fatal("FRESH_DECLARATION_LITERALS")
	}
	t.Log("REAL_MANIFEST_EXACT_DECLARATION PASS; REAL_MANIFEST_GO_CANONICAL_IDENTITY PASS; REAL_MANIFEST_EXACT_SOURCE PASS; WRONG_REAL_SOURCE_REFUSED PASS; AGGREGATE_OFF_BY_ONE_REFUSED PASS")
}
