package locationexecutionv5

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLedgerRawMessageCanonicalRoundTrip(t *testing.T) {
	root := t.TempDir()
	payload := map[string]any{"z": []any{json.RawMessage(`{"b":2,"a":1}`)}, "a": "first"}
	if err := AppendLedger(root, "payload", payload); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLedger(root); err != nil {
		t.Fatal(err)
	}
	var l Ledger
	if err := readStrict(filepath.Join(root, "EVENT_LEDGER.json"), &l); err != nil {
		t.Fatal(err)
	}
	if got := string(l.Entries[0].Payload); !strings.Contains(got, `"a":"first"`) || !strings.Contains(got, `"z":[{"a":1,"b":2}]`) {
		t.Fatalf("payload not canonicalized: %s", got)
	}
	if err := os.WriteFile(filepath.Join(root, "EVENT_LEDGER.copy.json"), canon(l), 0644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLedger(root); err != nil {
		t.Fatalf("persisted roundtrip failed: %v", err)
	}
}

func TestLedgerRejectsPayload2MutationRegression(t *testing.T) {
	root := t.TempDir()
	if err := AppendLedger(root, "one", map[string]any{"case": "01"}); err != nil {
		t.Fatal(err)
	}
	if err := AppendLedger(root, "two", map[string]any{"case": "02", "nested": map[string]any{"b": 2, "a": 1}}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLedger(root); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "EVENT_LEDGER.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(b), `"case":"02"`, `"case":"02x"`, 1)
	if err := os.WriteFile(p, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLedger(root); err == nil || !strings.Contains(err.Error(), "ledger payload 2") {
		t.Fatalf("expected payload 2 rejection, got %v", err)
	}
}

func TestStrictJSONRejectsUnknownDuplicateTrailing(t *testing.T) {
	root := t.TempDir()
	unknown := filepath.Join(root, "unknown.json")
	if err := os.WriteFile(unknown, []byte(`{"schema":"x","extra":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	var c Condition
	if err := readStrict(unknown, &c); err == nil {
		t.Fatal("expected unknown field rejection")
	}
	if _, err := canonicalRaw([]byte(`{"a":1}{"b":2}`)); err == nil {
		t.Fatal("expected trailing json rejection")
	}
	if _, err := canonicalRaw([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("expected duplicate key rejection")
	}
}

func TestConditionStrictlyAcceptsActualShapesAndRejectsNestedUnknown(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "plain.json")
	if err := os.WriteFile(plain, []byte(`{"schema":"condition","cancel":false,"deadlineExpired":false,"limitsProfile":"published"}`), 0644); err != nil {
		t.Fatal(err)
	}
	var plainCondition Condition
	if err := readStrict(plain, &plainCondition); err != nil {
		t.Fatalf("plain condition shape rejected: %v", err)
	}
	withBoundary := filepath.Join(root, "boundary.json")
	if err := os.WriteFile(withBoundary, []byte(`{"schema":"condition","boundarySetup":{"expandedSources":230,"expectedPrecedence":"frozen","maxFrozenPaths":230,"rawFrozenPaths":231,"maxWitnesses":4,"uniqueWitnesses":4},"cancel":false,"deadlineExpired":false,"limitsProfile":"published"}`), 0644); err != nil {
		t.Fatal(err)
	}
	var boundaryCondition Condition
	if err := readStrict(withBoundary, &boundaryCondition); err != nil {
		t.Fatalf("boundary condition shape rejected: %v", err)
	}
	if boundaryCondition.BoundarySetup == nil || boundaryCondition.BoundarySetup.RawFrozenPaths != 231 || boundaryCondition.BoundarySetup.UniqueWitnesses != 4 {
		t.Fatalf("boundary setup not decoded: %+v", boundaryCondition.BoundarySetup)
	}
	nestedUnknown := filepath.Join(root, "nested-unknown.json")
	if err := os.WriteFile(nestedUnknown, []byte(`{"schema":"condition","boundarySetup":{"expandedSources":230,"unexpected":true},"cancel":false,"deadlineExpired":false,"limitsProfile":"published"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := readStrict(nestedUnknown, &Condition{}); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected nested unknown rejection, got %v", err)
	}
}

func TestGateRejectsAssignmentArrayMutationAndRequiresOneDispatch(t *testing.T) {
	execRoot := t.TempDir()
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	writeMinimalAuthorization(t, execRoot)
	writeAssignments(t, execRoot, func(a *AssignmentFile) {})
	if err := writeBlockedAudit(execRoot, repoRoot); err != nil {
		t.Fatal(err)
	}
	if err := checkBlockedAudit(execRoot, repoRoot); err != nil {
		t.Fatalf("valid blocked audit rejected: %v", err)
	}
	var g PredispatchGate
	if err := readStrict(filepath.Join(execRoot, "PREDISPATCH_AUDIT_BLOCKED.json"), &g); err != nil {
		t.Fatal(err)
	}
	g.OneDispatch = "monolithic-runReal"
	badGateRoot := t.TempDir()
	writeMinimalAuthorization(t, badGateRoot)
	writeAssignments(t, badGateRoot, func(a *AssignmentFile) {})
	if err := writeJSON(filepath.Join(badGateRoot, "PREDISPATCH_AUDIT_BLOCKED.json"), g); err != nil {
		t.Fatal(err)
	}
	if err := checkBlockedAudit(badGateRoot, repoRoot); err == nil || !strings.Contains(err.Error(), "gate mismatch") {
		t.Fatalf("expected oneDispatch gate rejection, got %v", err)
	}
	badAssignmentRoot := t.TempDir()
	writeMinimalAuthorization(t, badAssignmentRoot)
	writeAssignments(t, badAssignmentRoot, func(a *AssignmentFile) {
		a.Cases[0].Producer.MayRead = append(a.Cases[0].Producer.MayRead, "oracle-candidate")
	})
	if err := writeBlockedAudit(badAssignmentRoot, repoRoot); err != nil {
		t.Fatal(err)
	}
	if err := checkBlockedAudit(badAssignmentRoot, repoRoot); err == nil || !strings.Contains(err.Error(), "producer assignment arrays") {
		t.Fatalf("expected assignment array rejection, got %v", err)
	}
}

func TestPhaseSpecificProducersBlockedWithoutGate(t *testing.T) {
	execRoot := t.TempDir()
	writeMinimalAuthorization(t, execRoot)
	writeAssignments(t, execRoot, func(a *AssignmentFile) {})
	err := RunPhase(execRoot, t.TempDir(), filepath.Clean(filepath.Join("..", "..")), "Producers")
	if err == nil || !strings.Contains(err.Error(), "real execution requires independent committed gate") {
		t.Fatalf("expected predispatch gate block, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(execRoot, "attempts")); !os.IsNotExist(statErr) {
		t.Fatalf("blocked Producers created attempts: %v", statErr)
	}
}

func writeMinimalAuthorization(t *testing.T, root string) {
	t.Helper()
	a := Authorization{Schema: AuthorizationSchema, Status: "SUCCESSOR_PRE_DISPATCH_DECLARED", SuccessorIdentity: RootIdentity, ExecutionRoot: "docs/pilot/adr0007/experiment/" + RootIdentity, PredecessorSealSHA256: PredecessorRootIdentity, PredecessorSealImmutable: true, AuthorizedFreezeRootIdentity: FreezeRootIdentity, AttemptID: "attempt-successor-01", ProducerAssignments: 26, ReviewerAssignments: 26, PhaseAPI: []string{"Plan", "Simulate", "Producers", "Reviewers", "Boundaries", "Reconcile", "Verify"}, DefaultMode: "SIMULATE_ONLY", RequiresIndependentCommittedGate: true, GateFile: "PREDISPATCH_GO.json", NoRealSemanticAttemptsBeforeGate: true, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", ExternalInference: "DISABLED"}
	if err := writeJSON(filepath.Join(root, "AUTHORIZATION.json"), a); err != nil {
		t.Fatal(err)
	}
}

func writeAssignments(t *testing.T, root string, mutate func(*AssignmentFile)) {
	t.Helper()
	a := AssignmentFile{Schema: AssignmentSchema, AttemptID: "attempt-successor-01", SuccessorIdentity: RootIdentity}
	for i := 1; i <= 26; i++ {
		caseID := fmt.Sprintf("case-%02d", i)
		if i == 1 {
			caseID = "01-exact-intersects"
		}
		a.Cases = append(a.Cases, AssignmentCase{CaseID: caseID, Ordinal: i,
			Producer: AssignmentRole{AssignmentID: fmt.Sprintf("successor-producer-%02d-%s-attempt-successor-01", i, caseID), AttemptID: "attempt-successor-01", Role: "producer", MayRead: []string{"frozen inputs only", "condition file", "binding file when present"}, Forbidden: []string{"oracle-candidate", "derivations", "reviewer output", "external inference", "semantic retry", "semantic repair", "predecessor attempts"}, MustWrite: []string{"RESULT.json", "PRODUCER_ATTEMPT.json"}},
			Reviewer: AssignmentRole{AssignmentID: fmt.Sprintf("successor-reviewer-%02d-%s-attempt-successor-01", i, caseID), AttemptID: "attempt-successor-01", Role: "reviewer", MayRead: []string{"producer committed result", "frozen oracle result", "frozen oracle derivation"}, Forbidden: []string{"external inference", "semantic retry", "semantic repair", "producer code changes", "predecessor attempts"}, MustWrite: []string{"REVIEW.json"}}})
	}
	mutate(&a)
	if err := writeJSON(filepath.Join(root, "ASSIGNMENTS.json"), a); err != nil {
		t.Fatal(err)
	}
}
