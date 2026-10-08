package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/adr0007locationv5"
)

func TestLedgerCreateNewRejectsOverwrite(t *testing.T) {
	d := t.TempDir()
	if err := writeLedger(d, "GATE_CONSUMED", map[string]any{"n": 1}); err != nil {
		t.Fatal(err)
	}
	if err := writeLedger(d, "GATE_CONSUMED", map[string]any{"n": 2}); err == nil {
		t.Fatalf("expected existing ledger to fail create-new")
	}
	b, err := os.ReadFile(filepath.Join(d, "EVENT_LEDGER.json"))
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Schema string        `json:"schema"`
		Events []ledgerEvent `json:"events"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	if w.Schema != ledgerSchema || len(w.Events) != 1 || w.Events[0].Type != "GATE_CONSUMED" || w.Events[0].Prev != "GENESIS" {
		t.Fatalf("bad create-new ledger %#v", w)
	}
}

func TestAuditStrictRejectsBadProducerJSON(t *testing.T) {
	var p producer
	for _, raw := range [][]byte{[]byte(`{"schema":"x","schema":"y"}`), []byte(`{"schema":"x"}0`), []byte(`{"schema":"x","caseId":"c","assignmentId":"a","result":{},"processCustody":{},"extra":1}`)} {
		f := filepath.Join(t.TempDir(), "x.json")
		if err := os.WriteFile(f, raw, 0644); err != nil {
			t.Fatal(err)
		}
		if err := strict(f, &p); err == nil {
			t.Fatalf("expected strict failure for %s", raw)
		}
	}
}

func TestPhaseSequenceCannotRepeatOrSkip(t *testing.T) {
	d := t.TempDir()
	if err := consumePhase(d, "precheck"); err != nil {
		t.Fatal(err)
	}
	if err := consumePhase(d, "precheck"); err == nil {
		t.Fatalf("expected repeat phase failure")
	}
	if err := consumePhase(d, "producer"); err == nil || !strings.Contains(err.Error(), "phase transition") {
		t.Fatalf("expected skipped gate transition failure, got %v", err)
	}
}

func TestReviewCreateNewRejectsExistingPlan(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "producer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "REVIEW_PLAN.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := review(d); err == nil {
		t.Fatalf("expected review plan overwrite failure")
	}
}

func TestVerifierRejectsMissingDuplicateLeaves(t *testing.T) {
	d := t.TempDir()
	frozen := filepath.Join(d, "frozen")
	if err := os.MkdirAll(filepath.Join(frozen, "inputs", "only"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(frozen, "inputs", "only", "REQUEST.raw.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verify(d, frozen); err == nil || !strings.Contains(err.Error(), "leaf count mismatch") {
		t.Fatalf("expected derived leaf count failure, got %v", err)
	}
}

func TestDisposableFullPhaseExecutionVerifyAndTamperNegatives(t *testing.T) {
	root := t.TempDir()
	frozen := frozenCorpusRoot(t)
	cases, err := leafCases(filepath.Join(frozen, "inputs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 26 {
		t.Fatalf("fixture case count got %d", len(cases))
	}
	for _, c := range cases {
		writeDisposableCase(t, root, frozen, c)
	}
	if err := verify(root, frozen); err != nil {
		t.Fatalf("full disposable verify failed: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "INDEPENDENT_VERIFIER.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"leafRequests":26`) || !strings.Contains(string(b), `"producerResults":26`) || !strings.Contains(string(b), `"reviewerResults":26`) || !strings.Contains(string(b), `"boundaryBundles":4`) {
		t.Fatalf("verifier did not derive 26/26/4 counts: %s", b)
	}
	if err := verify(root, frozen); err == nil {
		t.Fatalf("repeat verifier artifact overwrite accepted")
	}
	os.Remove(filepath.Join(root, "INDEPENDENT_VERIFIER.json"))
	if err := os.WriteFile(filepath.Join(root, "reviewer", cases[0], "REVIEW.json"), []byte(`{"schema":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verify(root, frozen); err == nil {
		t.Fatalf("review tamper accepted")
	}
}

func frozenCorpusRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "docs", "pilot", "adr0007", "experiment", "location-intersection-prospective-v5"))
}

func writeDisposableCase(t *testing.T, root, frozen, c string) {
	t.Helper()
	input := filepath.Join(frozen, "inputs", c)
	raw, err := os.ReadFile(filepath.Join(input, "REQUEST.raw.json"))
	if err != nil {
		t.Fatal(err)
	}
	condRaw, err := os.ReadFile(filepath.Join(input, "CONDITION.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cond conditionFile
	if err := strictBytes("CONDITION.json", condRaw, &cond); err != nil {
		t.Fatal(err)
	}
	var bind []byte
	if b, err := os.ReadFile(filepath.Join(input, "BINDING.json")); err == nil {
		bind = b
	} else if _, err := os.Stat(filepath.Join(input, "BINDING.ABSENT")); err == nil {
		bind = nil
	} else {
		t.Fatal(err)
	}
	res, err := adr0007locationv5.Evaluate(raw, bind, adr0007locationv5.StaticControl{Cancel: cond.Cancel, Deadline: cond.DeadlineExpired}, adr0007locationv5.PublishedLimits())
	if err != nil {
		t.Fatal(err)
	}
	resBytes, _ := adr0007locationv5.Canonical(res)
	prod := map[string]any{"schema": "lsp-trace.adr0007.location-v5.producer-execution.v1", "caseId": c, "assignmentId": "prod-" + c, "result": json.RawMessage(resBytes), "processCustody": map[string]any{"inputDigests": map[string]string{"EXPECTED_RESULT_SHA256": digest(resBytes)}, "exitCode": 0}}
	writeJSON(t, filepath.Join(root, "producer", c, "RESULT.json"), prod)
	d := derivation{Schema: derivationSchema, CaseID: c, ProducerAssignmentID: "prod-" + c, ReviewerAssignmentID: "rev-" + c, ProducerResultSHA256: digest(resBytes), OracleResultSHA256: digest(resBytes), ActualResultSHA256: digest(resBytes), ExpectedResultSHA256: digest(resBytes), ResultEqual: true, CausalReference: "actual result exactly matches independently recomputed oracle from frozen request/binding/condition"}
	writeJSON(t, filepath.Join(root, "reviewer", c, "DERIVATION.json"), d)
	rev := map[string]any{"schema": "lsp-trace.adr0007.location-v5.reviewer-execution.v1", "caseId": c, "assignmentId": "rev-" + c, "derivation": d, "reviewBytes": digest(canon(d)), "custody": map[string]any{"independent": true}, "state": "REVIEWED", "role": "reviewer", "attempt": "attempt-1"}
	writeJSON(t, filepath.Join(root, "reviewer", c, "REVIEW.json"), rev)
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBoundaryStrictRequiresReplayFields(t *testing.T) {
	d := t.TempDir()
	frozen := filepath.Join(d, "frozen")
	bdir := filepath.Join(frozen, "oracle-candidate", "boundaries", "one")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bdir, "BOUNDARY.json"), []byte(`{"expected":{},"actual":{},"digest":"sha256:x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := boundary(d, frozen); err == nil || !strings.Contains(err.Error(), "missing custody") {
		t.Fatalf("expected strict boundary replay failure, got %v", err)
	}
}
