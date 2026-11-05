//go:build darwin && arm64

package programc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
	rg "lsp-trace/internal/referencegrouping"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTypedBundleBlockedDeliveryWall(t *testing.T) {
	withSupervisorDefaults(t)
	supervisorWall = 150 * time.Millisecond
	t.Setenv("PROGRAMC_TEST_WORKER", "never-read")
	pidfile := filepath.Join(t.TempDir(), "pid.json")
	t.Setenv("PROGRAMC_TEST_PIDFILE", pidfile)
	done := make(chan *SupervisionFailure, 1)
	go func() {
		_, _, f := ComputeSupervised(context.Background(), bytes.Repeat([]byte("x"), 2<<20), 1)
		done <- f
	}()
	var f *SupervisionFailure
	escaped := false
	select {
	case f = <-done:
	case <-time.After(2 * time.Second):
		escaped = true
		// Outer RED harness remains bounded even against the old blocked-write bug.
		raw, err := os.ReadFile(pidfile)
		if err != nil {
			t.Fatal(err)
		}
		var p watchdogPIDs
		if err = json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
		if sameProcessIdentity(p.Worker) {
			_ = syscall.Kill(-p.Worker.PGID, syscall.SIGKILL)
		}
		select {
		case f = <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("outer RED cleanup failed")
		}
	}
	raw, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatal(err)
	}
	var p watchdogPIDs
	if err = json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if sameProcessIdentity(p.Worker) {
		t.Fatal("ASSERT_INPUT_WALL_REAP: child remains")
	}
	if escaped || f == nil || f.Code != CodeTimeout {
		t.Fatalf("ASSERT_INPUT_WALL_REAP: escaped=%v failure=%v", escaped, f)
	}
	t.Log("ASSERT_INPUT_WALL_REAP: PASS")
}

// These are controlled offline evidence constructors, never provider receipts.
// Two real scheduled cases make an after-first-case barrier distinguish a
// partially executed stage from a completed report.
func typedStageBundlePrivateTest(t *testing.T) GroupingCaseBundlePrivate {
	t.Helper()
	a, b := node("stage-a", 0), node("stage-b", 1)
	raw := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}}})
	empty := validV5(t, nil, nil)
	metadata := programccompose.ExactMetadata{WorkspaceIdentity: "controlled-stage-workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "controlled-private"}
	composite, e := programccompose.Compose([]programccompose.Input{{Bytes: raw, Identity: "controlled-stage-calls", SHA256: bundleReportDigestPrivate(raw), ByteLength: len(raw), ExactMetadata: metadata}, {Bytes: empty, Identity: "controlled-stage-empty", SHA256: bundleReportDigestPrivate(empty), ByteLength: len(empty), ExactMetadata: metadata}})
	if e != nil {
		t.Fatal(e)
	}
	admission, e := programcadmission.Admit(composite.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	typed, e := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: admission.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
	if e != nil {
		t.Fatal(e)
	}
	projection, e := programcadmission.PreparePrivateLeidenProjection(typed)
	if e != nil {
		t.Fatal(e)
	}
	marshal := func(v any) []byte {
		r, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append(r, '\n')
	}
	type occurrence struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	rows := []occurrence{}
	outcomes := []rg.CandidateOutcome{}
	for _, o := range typed.Occurrences {
		rows = append(rows, occurrence{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.RelationID})
		outcomes = append(outcomes, rg.CandidateOutcome{OccurrenceIdentity: o.Identity, Disposition: "ADMITTED"})
	}
	ledger := marshal(struct {
		Domain      string       `json:"domain"`
		Version     string       `json:"version"`
		Claim       string       `json:"claim"`
		Occurrences []occurrence `json:"occurrences"`
	}{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows})
	preds := []rg.PredecessorRecord{{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger}, {Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: marshal(projection.CompositeSourceBinding())}, {Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: projection.RetainedCallsCanonicalBytes()}}
	artifact, e := rg.BuildInput(rg.InputRequest{TypedInput: typed, Predecessors: preds, OccurrenceLedgerBytes: ledger, EndpointCustody: []rg.EndpointCustody{{Endpoint: a.ID, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"}, {Endpoint: b.ID, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"}}, OccurrenceAdmissionPolicyID: "controlled-occurrence-v1", CompositionPolicyID: "controlled-identity-v1", PrivacyPolicy: "controlled-private", EligibleKinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Limits: rg.ResourceLimits{MaxArtifacts: 16, MaxNodes: 32, MaxOccurrences: 32, MaxContributions: 32, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}, CandidateOutcomes: outcomes, Accounting: rg.Accounting{Candidates: len(rows), Included: len(rows), Terminal: true}})
	if e != nil {
		t.Fatal(e)
	}
	input, e := rg.AdmitInput(artifact, preds)
	if e != nil {
		t.Fatal(e)
	}
	policy, e := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "CONTROLLED_STAGE", Epoch: 8, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: []rg.GroupingPolicyRule{{RelationKind: "CALLS", Action: "INCLUDE", RationaleCode: "CONTROLLED_STAGE", Weight: rg.Rational{Numerator: 1, Denominator: 1}}, {RelationKind: "REFERENCES_SYMBOL", Action: "EXCLUDE", RationaleCode: "CONTROLLED_STAGE", Weight: rg.Rational{Numerator: 0, Denominator: 1}}, {RelationKind: "RESOLVES_TO_DEFINITION", Action: "EXCLUDE", RationaleCode: "CONTROLLED_STAGE", Weight: rg.Rational{Numerator: 0, Denominator: 1}}}, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1 << 20, MaxDenominator: 1 << 20, MaxCap: 128}})
	if e != nil {
		t.Fatal(e)
	}
	req := GroupingCaseBundleRequestPrivate{Schema: GroupingCaseBundleSchemaPrivate, BatchID: "controlled-stage-two", ImplementationID: "controlled-stage-not-qualified", ImplementationBytes: []byte("offline declaration\n")}
	for i, seed := range []uint64{7, 19} {
		c, e := NewGroupingEvaluationCasePrivate(GroupingEvaluationCaseRequestPrivate{CaseID: fmt.Sprintf("controlled-stage-%d", i), SelectorID: "controlled-selector", RunID: "controlled-run", SelectorBytes: []byte("not an executed query\n"), Input: input, Policy: policy, ProjectionLimits: rg.CompositionLimits{MaxRawBytes: 1 << 20, MaxOccurrences: 128, MaxPairContributions: 128, MaxPairs: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxDenominator: 1 << 20, MaxAccumulator: 1 << 20}, Seed: seed})
		if e != nil {
			t.Fatal(e)
		}
		nodes := c.Projection().NodeIdentities()
		pairs := []GroupingPairJudgmentPrivate{{Left: nodes[0], Right: nodes[1], Label: "SAME"}}
		j, e := NewGroupingJudgmentsPrivate(c, "independent-controlled", pairs)
		if e != nil {
			t.Fatal(e)
		}
		req.Slots = append(req.Slots, GroupingCaseBundleSlotPrivate{InputBytes: input.CanonicalBytes(), PolicyBytes: policy.CanonicalBytes(), Predecessors: preds, CaseID: fmt.Sprintf("controlled-stage-%d", i), SelectorID: "controlled-selector", RunID: "controlled-run", SelectorBytes: []byte("not an executed query\n"), ProjectionLimits: rg.CompositionLimits{MaxRawBytes: 1 << 20, MaxOccurrences: 128, MaxPairContributions: 128, MaxPairs: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxDenominator: 1 << 20, MaxAccumulator: 1 << 20}, Seed: seed, JudgmentID: "independent-controlled", Pairs: pairs, CaseBytes: c.CanonicalBytes(), JudgmentBytes: j.CanonicalBytes()})
	}
	bundle, e := NewGroupingCaseBundlePrivate(req)
	if e != nil {
		t.Fatal(e)
	}
	return bundle
}

type typedStageEventPrivateTest struct {
	Phase                                              string
	Ordinal, InitialCases, ReplayCases, CheckerEntries int
}

func writeTypedStageEventPrivateTest(event typedStageEventPrivateTest) {
	path := os.Getenv("PROGRAMC_TEST_STAGE_FILE")
	if path == "" {
		return
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		panic(e)
	}
	e = json.NewEncoder(f).Encode(event)
	if e == nil {
		e = f.Sync()
	}
	_ = f.Close()
	if e != nil {
		panic(e)
	}
}

var typedStageMemoryPrivateTest []byte

// Only this _test.go helper installs the observer or implements a test stall.
// No production environment flag or report-replacement callback exists.
func runTypedBundleFixtureWorkerPrivateTest(mode string) (bool, int) {
	if mode == "typed-late-response" {
		var out bytes.Buffer
		_, code := RunPrivateWorker([]string{workerArgument}, os.Stdin, &out, os.Stderr)
		if code != 0 {
			return true, code
		}
		var response workerResponse
		if e := json.Unmarshal(out.Bytes(), &response); e != nil || response.Bundle == nil || response.Failure != nil {
			return true, 129
		}
		writeTypedStageEventPrivateTest(typedStageEventPrivateTest{Phase: "response-ready"})
		for {
			if _, e := os.Stat(os.Getenv("PROGRAMC_TEST_STAGE_FILE") + ".release"); e == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		_, e := os.Stdout.Write(out.Bytes())
		if e != nil {
			return true, 130
		}
		return true, 0
	}
	if !strings.HasPrefix(mode, "typed-stage-") && !strings.HasPrefix(mode, "typed-check-") {
		return false, 0
	}
	if strings.HasPrefix(mode, "typed-stage-") {
		initial, replay, entries := 0, 0, 0
		groupingMeasurementObserverPrivate = func(stage groupingMeasurementStagePrivate, ordinal int) {
			if stage == measurementCheckerEntryPrivate {
				entries++
				return
			}
			if stage != measurementCaseEvaluatedPrivate {
				return
			}
			phase := "execution"
			if entries > 0 {
				phase = "replay"
				replay++
			} else {
				initial++
			}
			event := typedStageEventPrivateTest{phase, ordinal, initial, replay, entries}
			writeTypedStageEventPrivateTest(event)
			if ordinal != 1 || !strings.Contains(mode, phase) {
				return
			}
			if strings.HasSuffix(mode, "-rss") {
				typedStageMemoryPrivateTest = make([]byte, 512<<20)
				for i := 0; i < len(typedStageMemoryPrivateTest); i += 4096 {
					typedStageMemoryPrivateTest[i] = 1
				}
			}
			for {
				if _, e := os.Stat(os.Getenv("PROGRAMC_TEST_STAGE_FILE") + ".release"); e == nil {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			runtime.KeepAlive(typedStageMemoryPrivateTest)
		}
		_, code := RunPrivateWorker([]string{workerArgument}, os.Stdin, os.Stdout, os.Stderr)
		return true, code
	}
	var request workerRequest
	if e := strictDecode(os.Stdin, MaxWorkerInputBytes, &request); e != nil || request.BundleExpected == nil {
		return true, 123
	}
	bundle, e := ImportGroupingCaseBundlePrivate(*request.BundleExpected, request.Input)
	if e != nil {
		return true, 124
	}
	generated, e := bundle.ExecuteReport()
	if e != nil {
		return true, 125
	}
	report := generated.CanonicalBytes()
	if mode == "typed-check-counter" {
		doc := generated.Document()
		doc.Rows[0].Counts.TruePositive++
		report, e = groupingCanonicalPrivate(doc)
	} else if mode == "typed-check-foreign" {
		batch, e2 := NewGroupingMeasurementBatchPrivate(GroupingMeasurementBatchRequestPrivate{BatchID: "foreign-controlled-batch", ImplementationID: bundle.batch.doc.ImplementationID, ImplementationBytes: bundle.batch.doc.ImplementationBytes, Slots: bundle.batch.slots})
		if e2 != nil {
			return true, 126
		}
		foreign, e2 := ExecuteGroupingMeasurementBatchPrivate(batch)
		if e2 != nil {
			return true, 127
		}
		report = foreign.CanonicalBytes()
	}
	if e != nil {
		return true, 128
	}
	// Corruption happens only in this controlled test worker's local variables;
	// normal dispatch cannot accept a supplied unchecked report.
	return true, writeWorkerResponse(os.Stdout, finishBundleWorkerReportPrivate(request, bundle, report))
}

func TestTypedBundleActualStageContainment(t *testing.T) {
	bundle := typedStageBundlePrivateTest(t)
	direct, e := bundle.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	if e = bundle.CheckReport(direct.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	for _, phase := range []string{"execution", "replay"} {
		for _, action := range []string{"cancel", "wall", "rss"} {
			t.Run(phase+"-"+action, func(t *testing.T) {
				withSupervisorDefaults(t)
				supervisorWall = 2 * time.Second
				if action == "rss" {
					supervisorRSSBytes = 256 << 20
				}
				t.Setenv("PROGRAMC_TEST_WORKER", "typed-stage-"+phase+"-"+action)
				stagefile := filepath.Join(t.TempDir(), "stage.jsonl")
				pidfile := filepath.Join(t.TempDir(), "pid.json")
				t.Setenv("PROGRAMC_TEST_STAGE_FILE", stagefile)
				t.Setenv("PROGRAMC_TEST_PIDFILE", pidfile)
				expected, e := NewGroupingBundleExpectationPrivate(bundle)
				if e != nil {
					t.Fatal(e)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				type terminal struct {
					report  []byte
					failure *SupervisionFailure
				}
				done := make(chan terminal, 1)
				delivered, reapedEvents := 0, 0
				oldObserver := supervisorObserverPrivate
				t.Cleanup(func() { supervisorObserverPrivate = oldObserver })
				supervisorObserverPrivate = func(stage supervisorStagePrivate) {
					if stage == supervisorDeliveryFinishedPrivate {
						delivered++
					}
					if stage == supervisorReapedPrivate {
						reapedEvents++
					}
				}
				var identityTrace []string
				originalState, originalStart, originalKill := supervisorGroupState, supervisorProcessStart, supervisorGroupKill
				supervisorGroupState = func(ctx context.Context, pgid int) ([]processGroupMember, error) {
					members, e := originalState(ctx, pgid)
					identityTrace = append(identityTrace, fmt.Sprintf("state=%+v err=%v", members, e))
					return members, e
				}
				supervisorProcessStart = func(ctx context.Context, pid int) (string, error) {
					start, e := originalStart(ctx, pid)
					identityTrace = append(identityTrace, fmt.Sprintf("start=%q err=%v", start, e))
					return start, e
				}
				supervisorGroupKill = func(pid int, signal syscall.Signal) error {
					e := originalKill(pid, signal)
					identityTrace = append(identityTrace, fmt.Sprintf("kill=%d err=%v", pid, e))
					return e
				}
				waits := 0
				originalWait := supervisorCommandWait
				supervisorCommandWait = func(cmd *exec.Cmd) error { waits++; return originalWait(cmd) }
				go func() {
					r, _, f := ExecuteGroupingCaseBundleSupervisedPrivate(ctx, expected, bundle.CanonicalBytes())
					done <- terminal{r, f}
				}()
				found := false
				var observed typedStageEventPrivateTest
				cutoff := time.NewTimer(4 * time.Second)
				defer cutoff.Stop()
				var result terminal
				finished := false
				for !found && !finished {
					select {
					case result = <-done:
						finished = true
					case <-cutoff.C:
						cancel()
						t.Fatal("ASSERT_TYPED_ACTUAL_STAGE: outer fixture wall")
					default:
						raw, _ := os.ReadFile(stagefile)
						for _, line := range bytes.Split(raw, []byte("\n")) {
							var event typedStageEventPrivateTest
							if json.Unmarshal(line, &event) == nil && event.Phase == phase && event.Ordinal == 1 {
								found = true
								observed = event
								break
							}
						}
						if !found {
							time.Sleep(5 * time.Millisecond)
						}
					}
				}
				if found && action == "cancel" {
					cancel()
				}
				if !finished {
					select {
					case result = <-done:
					case <-time.After(4 * time.Second):
						cancel()
						t.Fatal("ASSERT_TYPED_ACTUAL_STAGE: worker did not terminate")
					}
				}
				want := CodeTimeout
				if action == "cancel" {
					want = CodeCancelled
				}
				if action == "rss" {
					want = CodeMemoryLimit
				}
				raw, e := os.ReadFile(pidfile)
				if e != nil {
					t.Fatal(e)
				}
				var p watchdogPIDs
				if e = json.Unmarshal(raw, &p); e != nil {
					t.Fatal(e)
				}
				if !found || observed.InitialCases != 1 && phase == "execution" || phase == "replay" && (observed.InitialCases != 2 || observed.ReplayCases != 1 || observed.CheckerEntries != 1) || result.failure == nil || result.failure.Code != want || len(result.report) != 0 || waits != 1 || delivered != 1 || reapedEvents != 1 || sameProcessIdentity(p.Worker) {
					t.Logf("CONTROLLED_IDENTITY_TRACE: %q", identityTrace)
					// Unexpected failing versions still get bounded outer-harness
					// cleanup; this is not the successful sole-Cmd.Wait guard.
					if sameProcessIdentity(p.Worker) {
						if killErr := syscall.Kill(-p.Worker.PGID, syscall.SIGKILL); killErr == nil {
							process, _ := os.FindProcess(p.Worker.PID)
							reaped := make(chan bool, 1)
							go func() { state, _ := process.Wait(); reaped <- state != nil }()
							select {
							case ok := <-reaped:
								t.Logf("OUTER_CONTROLLED_FAILURE_REAP: %v", ok)
							case <-time.After(2 * time.Second):
								t.Error("outer controlled reap timeout")
							}
						}
					}
					t.Fatalf("ASSERT_TYPED_ACTUAL_STAGE: phase=%s action=%s found=%v event=%+v failure=%v report=%d waits=%d live=%v", phase, action, found, observed, result.failure, len(result.report), waits, sameProcessIdentity(p.Worker))
				}
				t.Logf("ASSERT_TYPED_ACTUAL_STAGE: PASS phase=%s action=%s event=%+v waits=%d delivered=%d reaped=%d", phase, action, observed, waits, delivered, reapedEvents)
			})
		}
	}
}

func TestTypedBundleVerificationWindowAndOutput(t *testing.T) {
	bundle := typedStageBundlePrivateTest(t)
	expected, e := NewGroupingBundleExpectationPrivate(bundle)
	if e != nil {
		t.Fatal(e)
	}
	direct, e := bundle.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"near-wall-success", "late-before-verify", "late-after-verify", "typed-stdout-limit", "late-valid-worker-response"} {
		t.Run(mode, func(t *testing.T) {
			withSupervisorDefaults(t)
			supervisorWall = 10 * time.Second
			pidfile := filepath.Join(t.TempDir(), "pid.json")
			stagefile := filepath.Join(t.TempDir(), "stage.jsonl")
			t.Setenv("PROGRAMC_TEST_PIDFILE", pidfile)
			t.Setenv("PROGRAMC_TEST_STAGE_FILE", stagefile)
			t.Setenv("PROGRAMC_TEST_WORKER", "typed-stage-record")
			if mode == "late-valid-worker-response" {
				t.Setenv("PROGRAMC_TEST_WORKER", "typed-late-response")
			}
			if mode == "typed-stdout-limit" {
				supervisorOutputBytes = 1024
			}
			window := 2 * time.Second
			if mode == "near-wall-success" {
				window = 5 * time.Second
			}
			deadline := time.Now().Add(window)
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			defer cancel()
			oldObserver := supervisorObserverPrivate
			t.Cleanup(func() { supervisorObserverPrivate = oldObserver })
			delivered, reaped, before, after := 0, 0, 0, 0
			supervisorObserverPrivate = func(stage supervisorStagePrivate) {
				switch stage {
				case supervisorDeliveryFinishedPrivate:
					delivered++
				case supervisorReapedPrivate:
					reaped++
				case supervisorBeforeVerifyPrivate:
					before++
					if mode == "near-wall-success" {
						timer := time.NewTimer(time.Until(deadline.Add(-time.Second)))
						defer timer.Stop()
						<-timer.C
					}
					if mode == "late-before-verify" {
						<-ctx.Done()
					}
				case supervisorAfterVerifyPrivate:
					after++
					if mode == "late-after-verify" {
						<-ctx.Done()
					}
				}
			}
			report, obs, failed := ExecuteGroupingCaseBundleSupervisedPrivate(ctx, expected, bundle.CanonicalBytes())
			raw, e := os.ReadFile(pidfile)
			if e != nil {
				t.Fatal(e)
			}
			var p watchdogPIDs
			if e = json.Unmarshal(raw, &p); e != nil {
				t.Fatal(e)
			}
			if delivered != 1 || reaped != 1 || sameProcessIdentity(p.Worker) {
				t.Fatalf("ASSERT_TYPED_WINDOW_REAP: mode=%s delivered=%d reaped=%d live=%v", mode, delivered, reaped, sameProcessIdentity(p.Worker))
			}
			switch mode {
			case "near-wall-success":
				if failed != nil || !bytes.Equal(report, direct.CanonicalBytes()) || before != 1 || after != 1 || obs.ElapsedNanos < int64(3*time.Second) || time.Until(deadline) > 1100*time.Millisecond || !time.Now().Before(deadline) {
					t.Fatalf("ASSERT_TYPED_NEAR_WALL_EXACT: failure=%v observation=%+v before=%d after=%d", failed, obs, before, after)
				}
				t.Log("ASSERT_TYPED_NEAR_WALL_EXACT: PASS (one-second scheduling margin in five-second absolute window)")
			case "late-before-verify":
				if failed == nil || failed.Code != CodeCancelled || len(report) != 0 || before != 1 || after != 0 {
					t.Fatalf("ASSERT_TYPED_LATE_BEFORE_VERIFY: failure=%v before=%d after=%d bytes=%d", failed, before, after, len(report))
				}
				t.Log("ASSERT_TYPED_LATE_BEFORE_VERIFY: PASS")
			case "late-after-verify":
				if failed == nil || failed.Code != CodeCancelled || len(report) != 0 || before != 1 || after != 1 || time.Now().Before(deadline) {
					t.Fatalf("ASSERT_TYPED_LATE_AFTER_VERIFY: failure=%v before=%d after=%d bytes=%d", failed, before, after, len(report))
				}
				t.Log("ASSERT_TYPED_LATE_AFTER_VERIFY: PASS")
			case "typed-stdout-limit":
				if failed == nil || failed.Code != CodeOutputLimit || len(report) != 0 {
					t.Fatalf("ASSERT_TYPED_STDOUT_LIMIT: failure=%v bytes=%d", failed, len(report))
				}
				t.Log("ASSERT_TYPED_STDOUT_LIMIT: PASS (actual typed response exceeds controlled 1024-byte ceiling)")
			case "late-valid-worker-response":
				stage, e := os.ReadFile(stagefile)
				if e != nil || !bytes.Contains(stage, []byte("response-ready")) || failed == nil || (failed.Code != CodeCancelled && failed.Code != CodeTimeout) || len(report) != 0 {
					t.Fatalf("ASSERT_TYPED_LATE_VALID_RESPONSE: marker=%q failure=%v bytes=%d err=%v", stage, failed, len(report), e)
				}
				t.Log("ASSERT_TYPED_LATE_VALID_RESPONSE: PASS (actual checked response held past deadline; worker killed/reaped)")
			}
		})
	}
}

func TestTypedBundleObserverNoninterference(t *testing.T) {
	bundle := typedStageBundlePrivateTest(t)
	baseline, e := bundle.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	old := groupingMeasurementObserverPrivate
	t.Cleanup(func() { groupingMeasurementObserverPrivate = old })
	for _, mode := range []string{"nil", "record", "panic"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			groupingMeasurementObserverPrivate = nil
			if mode != "nil" {
				groupingMeasurementObserverPrivate = func(groupingMeasurementStagePrivate, int) {
					calls++
					if mode == "panic" {
						panic("controlled notification panic")
					}
				}
			}
			got, e := bundle.ExecuteReport()
			if e != nil || !bytes.Equal(got.CanonicalBytes(), baseline.CanonicalBytes()) {
				t.Fatalf("ASSERT_TYPED_OBSERVER_NONINTERFERENCE: mode=%s err=%v", mode, e)
			}
			if e = bundle.CheckReport(got.CanonicalBytes()); e != nil {
				t.Fatal(e)
			}
			if mode != "nil" && calls != 5 {
				t.Fatalf("ASSERT_TYPED_OBSERVER_NONINTERFERENCE: mode=%s calls=%d", mode, calls)
			}
			t.Logf("ASSERT_TYPED_OBSERVER_NONINTERFERENCE: PASS mode=%s notifications=%d", mode, calls)
		})
	}
}

func TestTypedBundleGroupZombieAtConfirmation(t *testing.T) {
	withSupervisorDefaults(t)
	observations, kills := 0, 0
	supervisorProcessStart = func(context.Context, int) (string, error) { return "controlled-start", nil }
	supervisorGroupState = func(context.Context, int) ([]processGroupMember, error) {
		observations++
		state := "S"
		if observations == 3 {
			state = "?E"
		}
		if observations >= 4 {
			state = "Z"
		}
		return []processGroupMember{{PID: 42, PGID: 42, State: state}}, nil
	}
	supervisorGroupKill = func(int, syscall.Signal) error {
		kills++
		if kills > 1 {
			return syscall.EPERM
		}
		return nil
	}
	e := terminateWorkerGroup(processGroupIdentity{PID: 42, PGID: 42, Start: "controlled-start"})
	if e != nil || kills != 1 {
		t.Fatalf("ASSERT_GROUP_ZOMBIE_RECONFIRM: kills=%d observations=%d err=%v", kills, observations, e)
	}
	t.Log("ASSERT_GROUP_ZOMBIE_RECONFIRM: PASS")
}

func TestTypedBundleLegacyRequestShapeCompatibility(t *testing.T) {
	a, b := node("legacy-shape-a", 0), node("legacy-shape-b", 1)
	input := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	raw, e := json.Marshal(workerRequest{Version: workerVersion, Input: input, Seed: 7})
	if e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"bundle_expected", "BUNDLE_EXPECTED"} {
		t.Run(field, func(t *testing.T) {
			candidate := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(fmt.Sprintf(",%q:null}", field))...)
			var out bytes.Buffer
			handled, code := RunPrivateWorker([]string{workerArgument}, bytes.NewReader(candidate), &out, &bytes.Buffer{})
			var response workerResponse
			if e = json.Unmarshal(out.Bytes(), &response); e != nil {
				t.Fatal(e)
			}
			if !handled || code != 0 || response.Outcome != nil || response.Failure == nil || response.Failure.Code != CodeInternalAdmission || response.Failure.Message != "invalid worker request" {
				t.Fatalf("ASSERT_LEGACY_UNKNOWN_NULL_REQUEST: field=%s response=%s", field, out.Bytes())
			}
			t.Log("ASSERT_LEGACY_UNKNOWN_NULL_REQUEST: PASS")
		})
	}
}

func TestTypedBundleLegacyResponseShapeCompatibility(t *testing.T) {
	withSupervisorDefaults(t)
	t.Setenv("PROGRAMC_TEST_WORKER", "legacy-null-extension")
	a, b := node("legacy-response-a", 0), node("legacy-response-b", 1)
	input := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	out, _, failed := ComputeSupervised(context.Background(), input, 7)
	if failed == nil || failed.Code != CodeMalformedOutput || failed.Message != "invalid worker response" || out.Outcome != "" {
		t.Fatalf("ASSERT_LEGACY_UNKNOWN_NULL_RESPONSE: failure=%v outcome=%s", failed, out.Outcome)
	}
	t.Log("ASSERT_LEGACY_UNKNOWN_NULL_RESPONSE: PASS")
}

func TestTypedBundleRequestDuplicateNoMeasurement(t *testing.T) {
	bundle := typedStageBundlePrivateTest(t)
	direct, e := bundle.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	if e = bundle.CheckReport(direct.CanonicalBytes()); e != nil {
		t.Fatal(e)
	}
	expected := bundle.Commitment()
	raw, e := json.Marshal(workerRequest{Version: bundleWorkerVersionPrivate, Input: bundle.CanonicalBytes(), BundleExpected: &expected})
	if e != nil {
		t.Fatal(e)
	}
	old := groupingMeasurementObserverPrivate
	t.Cleanup(func() { groupingMeasurementObserverPrivate = old })
	notifications := 0
	groupingMeasurementObserverPrivate = func(groupingMeasurementStagePrivate, int) { notifications++ }
	dispatch := func(candidate []byte) workerResponse {
		t.Helper()
		var out bytes.Buffer
		handled, code := RunPrivateWorker([]string{workerArgument}, bytes.NewReader(candidate), &out, &bytes.Buffer{})
		if !handled || code != 0 {
			t.Fatalf("dispatch handled=%v code=%d", handled, code)
		}
		var response workerResponse
		if e := json.Unmarshal(out.Bytes(), &response); e != nil {
			t.Fatal(e)
		}
		return response
	}
	positive := dispatch(raw)
	if positive.Failure != nil || !bytes.Equal(positive.Report, direct.CanonicalBytes()) || notifications != 5 {
		t.Fatalf("ASSERT_TYPED_REQUEST_WORK_POSITIVE: failure=%v notifications=%d bytes=%d", positive.Failure, notifications, len(positive.Report))
	}
	t.Log("ASSERT_TYPED_REQUEST_WORK_POSITIVE: PASS notifications=5")
	for _, tc := range []struct{ name, from, to string }{
		{"seed", `"seed":0`, `"seed":1,"seed":0`},
		{"version", `"version":`, `"version":"unknown-before-typed","version":`},
		{"expected-id", `"id":`, `"id":"wrong-before-correct","id":`},
		{"expected-digest", `"digest":`, `"digest":"wrong-before-correct","digest":`},
		{"expected-length", `"byte_length":`, `"byte_length":1,"byte_length":`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notifications = 0
			if bytes.Count(raw, []byte(tc.from)) != 1 {
				t.Fatal("fixture replacement count")
			}
			response := dispatch(bytes.Replace(raw, []byte(tc.from), []byte(tc.to), 1))
			if response.Failure == nil || response.Failure.Code != CodeInternalAdmission || len(response.Report) != 0 || notifications != 0 {
				t.Fatalf("ASSERT_TYPED_DUPLICATE_NO_WORK: case=%s failure=%v bytes=%d notifications=%d", tc.name, response.Failure, len(response.Report), notifications)
			}
			t.Logf("ASSERT_TYPED_DUPLICATE_NO_WORK: PASS case=%s notifications=0", tc.name)
		})
	}
}

func TestTypedBundleLegacyDuplicateSelectionCompatibility(t *testing.T) {
	a, b := node("legacy-duplicate-a", 0), node("legacy-duplicate-b", 1)
	input := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	raw, e := json.Marshal(legacyWorkerRequest{Version: workerVersion, Input: input, Seed: 7})
	if e != nil {
		t.Fatal(e)
	}
	dispatch := func(candidate []byte) []byte {
		t.Helper()
		var out bytes.Buffer
		handled, code := RunPrivateWorker([]string{workerArgument}, bytes.NewReader(candidate), &out, &bytes.Buffer{})
		if !handled || code != 0 {
			t.Fatal("legacy dispatch protocol")
		}
		return out.Bytes()
	}
	baseline := dispatch(raw)
	var normal workerResponse
	if e = json.Unmarshal(baseline, &normal); e != nil || normal.Outcome == nil || normal.Failure != nil {
		t.Fatalf("legacy positive fixture: %s err=%v", baseline, e)
	}
	for _, tc := range []struct{ name, from, to string }{
		{"seed", `"seed":7`, `"seed":1,"seed":7`},
		{"unknown-then-legacy", `"version":`, `"version":"unknown-before-legacy","version":`},
		{"typed-then-legacy", `"version":`, `"version":"lsp-trace.private-program-c-bundle-worker.v1","version":`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := bytes.Replace(raw, []byte(tc.from), []byte(tc.to), 1)
			// Original snapshot shape/decoder: three fields, DisallowUnknownFields,
			// encoding/json duplicate last-wins. No strictjson call is used here.
			var original struct {
				Version string `json:"version"`
				Input   []byte `json:"input"`
				Seed    uint64 `json:"seed"`
			}
			decoder := json.NewDecoder(bytes.NewReader(candidate))
			decoder.DisallowUnknownFields()
			if e := decoder.Decode(&original); e != nil || original.Version != workerVersion || original.Seed != 7 || !bytes.Equal(original.Input, input) {
				t.Fatalf("original legacy last-wins control: %+v err=%v", original, e)
			}
			if got := dispatch(candidate); !bytes.Equal(got, baseline) {
				t.Fatalf("ASSERT_LEGACY_DUPLICATE_SELECTION: case=%s got=%s baseline=%s", tc.name, got, baseline)
			}
			t.Logf("ASSERT_LEGACY_DUPLICATE_SELECTION: PASS case=%s seed=7", tc.name)
		})
	}
	unknown := bytes.Replace(raw, []byte(`"version":`), []byte(`"version":"lsp-trace.private-program-c-bundle-worker.v1","version":`), 1)
	unknown = bytes.Replace(unknown, []byte(`"version":"lsp-trace.private-program-c-worker.v1"`), []byte(`"version":"unknown-last"`), 1)
	var refused workerResponse
	if e = json.Unmarshal(dispatch(unknown), &refused); e != nil || refused.Failure == nil || refused.Outcome != nil || len(refused.Report) != 0 {
		t.Fatalf("ASSERT_UNKNOWN_LAST_VERSION_REFUSED: response=%+v err=%v", refused, e)
	}
	t.Log("ASSERT_UNKNOWN_LAST_VERSION_REFUSED: PASS")
}

func TestTypedBundleReportCheckBarrier(t *testing.T) {
	bundle := typedStageBundlePrivateTest(t)
	expected, e := NewGroupingBundleExpectationPrivate(bundle)
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"typed-check-counter", "typed-check-foreign"} {
		t.Run(mode, func(t *testing.T) {
			withSupervisorDefaults(t)
			t.Setenv("PROGRAMC_TEST_WORKER", mode)
			report, _, failed := ExecuteGroupingCaseBundleSupervisedPrivate(context.Background(), expected, bundle.CanonicalBytes())
			if failed == nil || failed.Code != CodeInternalComputation || len(report) != 0 {
				t.Fatalf("ASSERT_TYPED_REPORT_CHECK_BARRIER: mode=%s failure=%v bytes=%d", mode, failed, len(report))
			}
			t.Log("ASSERT_TYPED_REPORT_CHECK_BARRIER: PASS")
		})
	}
}
