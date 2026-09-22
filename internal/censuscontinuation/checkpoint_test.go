package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
)

func TestMemoryStoreNoReplaceIdempotentAndDefensive(t *testing.T) {
	s := NewMemoryStore()
	b := []byte(`{"a":1}`)
	id, err := s.Put(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := s.Put(context.Background(), append([]byte(nil), b...)); err != nil || again != id {
		t.Fatalf("ASSERT_STORE_IDEMPOTENT: %v %q", err, again)
	}
	b[0] = 'x'
	got, err := s.Get(context.Background(), id)
	if err != nil || !bytes.Equal(got, []byte(`{"a":1}`)) {
		t.Fatalf("ASSERT_STORE_DEFENSIVE: %v %q", err, got)
	}
	got[0] = 'x'
	got2, _ := s.Get(context.Background(), id)
	if bytes.Equal(got, got2) {
		t.Fatal("ASSERT_STORE_GET_DEFENSIVE")
	}
	if err := s.PutAt(context.Background(), id, []byte(`{"a":2}`)); err == nil {
		t.Fatal("ASSERT_STORE_NO_REPLACE")
	}
}

func TestVerifyChainRejectsCoordinatedHandoffSubstitution(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	contract := testContract(t)
	contractBytes, err := contract.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	contractRef, err := putArtifact(ctx, store, "continuation_contract", contractBytes)
	if err != nil {
		t.Fatal(err)
	}
	handoffRef, err := putArtifact(ctx, store, "handoff", []byte(`{"schema_version":"foreign"}`))
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := NewCheckpoint(CheckpointInput{
		Stage:      StageCensusCommitted,
		CensusID:   digestOf([]byte("census")),
		HandoffID:  handoffRef.ID,
		ContractID: contract.ID(),
		ProfileID:  contract.ProfileID(),
		Artifacts:  []ArtifactRef{contractRef, handoffRef},
		Status:     StatusRunning,
	})
	if err != nil {
		t.Fatal(err)
	}
	selector, err := putCheckpoint(ctx, store, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChain(ctx, store, selector); err == nil {
		t.Fatal("ASSERT_CHECKPOINT_HANDOFF_SUBSTITUTION_REJECTED")
	}
}

func TestCheckpointArtifactAndChainTamperMatrix(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	contract := testContract(t)
	contractBytes, _ := contract.Bytes()
	contractRef, _ := putArtifact(ctx, store, "continuation_contract", contractBytes)
	handoffRef, _ := putArtifact(ctx, store, "handoff", []byte(`{"fixture":"handoff"}`))
	base := CheckpointInput{Stage: StageCensusCommitted, CensusID: digestOf([]byte("census")), HandoffID: digestOf([]byte("handoff")), ContractID: contract.ID(), ProfileID: contract.ProfileID(), Artifacts: []ArtifactRef{contractRef, handoffRef}, Status: StatusRunning}

	t.Run("unrelated store object allowed", func(t *testing.T) {
		checkpoint, err := NewCheckpoint(base)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Put(ctx, []byte(`{"unrelated":true}`)); err != nil {
			t.Fatal(err)
		}
		if err := VerifyCheckpoint(ctx, store, checkpoint, ""); err != nil {
			t.Fatal(err)
		}
	})

	cases := []struct {
		name   string
		mutate func(*CheckpointInput)
	}{
		{"missing member", func(in *CheckpointInput) { in.Artifacts = in.Artifacts[:1] }},
		{"extra member", func(in *CheckpointInput) {
			in.Artifacts = append(in.Artifacts, ArtifactRef{Kind: "program_c", ID: handoffRef.ID, Digest: handoffRef.Digest, ByteLength: handoffRef.ByteLength})
		}},
		{"order", func(in *CheckpointInput) { in.Artifacts[0], in.Artifacts[1] = in.Artifacts[1], in.Artifacts[0] }},
		{"foreign kind", func(in *CheckpointInput) { in.Artifacts[1].Kind = "foreign" }},
		{"wrong selector", func(in *CheckpointInput) { in.Artifacts[1].ID = digestOf([]byte("wrong")) }},
		{"wrong digest", func(in *CheckpointInput) { in.Artifacts[1].Digest = digestOf([]byte("wrong")) }},
		{"wrong length", func(in *CheckpointInput) { in.Artifacts[1].ByteLength++ }},
		{"broken prior", func(in *CheckpointInput) { in.PriorCheckpointID = "broken" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			in.Artifacts = append([]ArtifactRef(nil), base.Artifacts...)
			tc.mutate(&in)
			if checkpoint, err := NewCheckpoint(in); err == nil {
				if err := VerifyCheckpoint(ctx, store, checkpoint, ""); err == nil {
					t.Fatal("ASSERT_CHECKPOINT_TAMPER_REJECTED")
				}
			}
		})
	}
}

func TestCheckpointStrictParserMatrix(t *testing.T) {
	contract := testContract(t)
	contractBytes, _ := contract.Bytes()
	contractID := digestOf(contractBytes)
	handoff := []byte(`{"fixture":"handoff"}`)
	handoffID := digestOf(handoff)
	checkpoint, err := NewCheckpoint(CheckpointInput{Stage: StageCensusCommitted, CensusID: digestOf([]byte("census")), HandoffID: digestOf([]byte("handoff")), ContractID: contract.ID(), ProfileID: contract.ProfileID(), Artifacts: []ArtifactRef{{Kind: "continuation_contract", ID: contractID, Digest: contractID, ByteLength: uint64(len(contractBytes))}, {Kind: "handoff", ID: handoffID, Digest: handoffID, ByteLength: uint64(len(handoff))}}, Status: StatusRunning})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := checkpoint.Bytes()
	var object map[string]any
	_ = json.Unmarshal(raw, &object)
	object["unknown"] = true
	unknown, _ := json.Marshal(object)
	duplicate := bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"schema_version":"duplicate","schema_version":`), 1)
	pretty := append([]byte("\n"), raw...)
	trailing := append(append([]byte(nil), raw...), []byte(` {}`)...)
	for name, candidate := range map[string][]byte{"duplicate": duplicate, "unknown": unknown, "noncanonical": pretty, "trailing": trailing} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCheckpoint(candidate); err == nil {
				t.Fatal("ASSERT_CHECKPOINT_STRICT_PARSE_REJECTED")
			}
		})
	}
}

func TestFailedCaptureCheckpointDiagnosticRoundTripAndLegacyCompatibility(t *testing.T) {
	base := CheckpointInput{Stage: StageProgramCComputed, CensusID: digestOf([]byte("census")), HandoffID: digestOf([]byte("handoff")), ContractID: digestOf([]byte("contract")), ProfileID: digestOf([]byte("profile")), Artifacts: []ArtifactRef{{Kind: "continuation_contract", ID: digestOf([]byte("contract-bytes")), Digest: digestOf([]byte("contract-bytes")), ByteLength: 1}, {Kind: "handoff", ID: digestOf([]byte("handoff-bytes")), Digest: digestOf([]byte("handoff-bytes")), ByteLength: 1}, {Kind: "program_c", ID: digestOf([]byte("program")), Digest: digestOf([]byte("program")), ByteLength: 25645747}}, Status: StatusFailedCapture}
	base.Diagnostics = []Diagnostic{{Code: "CAPTURE", Stage: StageProgramCComputed, Category: "input", Observed: 135, Limit: 100, FailedField: "bindings", Invariant: "OBSERVED_MUST_NOT_EXCEED_LIMIT", CallerAction: "INCREASE_BOUNDED_CAPTURE_LIMIT", Recovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}}
	cp, err := NewCheckpoint(base)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := cp.Bytes()
	parsed, err := ParseCheckpoint(raw)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := parsed.CaptureFailureDiagnostic()
	if !ok || d != base.Diagnostics[0] {
		t.Fatalf("ASSERT_FAILED_CAPTURE_DIAGNOSTIC_ROUNDTRIP: %#v", d)
	}

	legacy := base
	legacy.Diagnostics = []Diagnostic{{Code: "CAPTURE", Stage: StageProgramCComputed}}
	w := checkpointWire{SchemaVersion: CheckpointSchema, Stage: legacy.Stage, CensusID: legacy.CensusID, HandoffID: legacy.HandoffID, ContractID: legacy.ContractID, ProfileID: legacy.ProfileID, Artifacts: legacy.Artifacts, Completeness: ContinuationCompleteness, Status: legacy.Status, Diagnostics: legacy.Diagnostics}
	w.CheckpointID = identityJSON("lsp-trace:census-continuation-checkpoint:v1", w, func(v *checkpointWire) { v.CheckpointID = "" })
	legacyRaw, _ := json.Marshal(w)
	if _, err := ParseCheckpoint(legacyRaw); err != nil {
		t.Fatalf("ASSERT_LEGACY_FAILED_CAPTURE_PARSE: %v", err)
	}
	if _, err := NewCheckpoint(legacy); err == nil {
		t.Fatal("ASSERT_NEW_FAILED_CAPTURE_REQUIRES_TYPED_DIAGNOSTIC")
	}

	for _, mutate := range []func(*Diagnostic){
		func(d *Diagnostic) { d.Category = "path" }, func(d *Diagnostic) { d.Observed = d.Limit }, func(d *Diagnostic) { d.FailedField = "raw_path" }, func(d *Diagnostic) { d.CallerAction = "READ_SOURCE" },
	} {
		bad := base
		bad.Diagnostics = append([]Diagnostic(nil), base.Diagnostics...)
		mutate(&bad.Diagnostics[0])
		if _, err := NewCheckpoint(bad); err == nil {
			t.Fatal("ASSERT_FAILED_CAPTURE_DIAGNOSTIC_ENUM_CAP_REJECTED")
		}
	}
}

func TestMemoryStoreConcurrentNoReplace(t *testing.T) {
	s := NewMemoryStore()
	const n = 32
	ids := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := s.Put(context.Background(), []byte(`{"same":true}`))
			ids <- id
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	var want string
	for err := range errs {
		if err != nil {
			t.Fatal("ASSERT_CONCURRENT_PUT:", err)
		}
	}
	for id := range ids {
		if want == "" {
			want = id
		}
		if id != want {
			t.Fatal("ASSERT_CONCURRENT_IDENTITY")
		}
	}
}

func TestCheckpointCanonicalChainAndTamperRejection(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	aID, _ := s.Put(ctx, []byte(`{"artifact":1}`))
	contract := testContract(t)
	contractBytes, _ := contract.Bytes()
	contractID, _ := s.Put(ctx, contractBytes)
	cp, err := NewCheckpoint(CheckpointInput{Stage: StageCensusCommitted, CensusID: digestOf([]byte("census")), HandoffID: digestOf([]byte("handoff")), ContractID: contract.ID(), ProfileID: contract.ProfileID(), Artifacts: []ArtifactRef{{Kind: "continuation_contract", ID: contractID, Digest: contractID, ByteLength: uint64(len(contractBytes))}, {Kind: "handoff", ID: aID, Digest: aID, ByteLength: uint64(len(`{"artifact":1}`))}}, Status: StatusRunning})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := cp.Bytes()
	parsed, err := ParseCheckpoint(raw)
	if err != nil || parsed.ID() != cp.ID() {
		t.Fatalf("ASSERT_CHECKPOINT_ROUNDTRIP: %v", err)
	}
	if err := VerifyCheckpoint(ctx, s, parsed, ""); err != nil {
		t.Fatal("ASSERT_CHECKPOINT_VERIFY:", err)
	}
	bad := append([]byte(nil), raw...)
	bad[len(bad)/2] ^= 1
	if _, err := ParseCheckpoint(bad); err == nil {
		t.Fatal("ASSERT_CHECKPOINT_TAMPER_REJECTED")
	}
	if err := VerifyCheckpoint(ctx, NewMemoryStore(), parsed, ""); err == nil {
		t.Fatal("ASSERT_CHECKPOINT_MISSING_REJECTED")
	}
}
