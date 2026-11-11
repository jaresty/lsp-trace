//go:build lsp_trace_controlled_describe_fixture

package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPrivatePrepareFailureNativeStages(t *testing.T) {
	t.Log("PROCEDURE: fixed literal pipe native Prepare; actual original CAS seam faults, no configurable backend; compare windows/debits/Observe count")
	for _, mode := range []string{"reserve", "reserve-uncertain", "start", "capture", "observe", "rprep", "persist", "packet", "healthy"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "capture" {
				privatePrepareCaptureExpectedFailureV1(t)
				return
			}
			unfunded := -1
			if mode == "reserve" {
				unfunded = 0
			}
			q, fixture := prepareDescribeNativeFixtureV1(t, unfunded)
			if mode == "start" {
				q.Capture.Start.Process.Args = []string{"fixed-starter-refuses-args"}
			}
			if mode == "packet" {
				q.PacketLimit = 1
			}
			failCAS := 0
			switch mode {
			case "reserve-uncertain":
				failCAS = 1
			case "observe":
				failCAS = 2
			case "rprep":
				failCAS = 3
			case "persist":
				failCAS = 5
			}
			casCalls := 0
			var categories []string
			marker := errors.New("diagnostic TEST CAS failure")
			q.Owner.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, raw []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
				casCalls++
				categories = append(categories, privatePrepareCASCategoryV1(t, raw))
				if casCalls == failCAS && mode != "reserve-uncertain" {
					return nil, marker
				}
				receipt, err := publication.CompareAndReplaceBoundFile(ctx, r, s, p, raw, v)
				if casCalls == failCAS && mode == "reserve-uncertain" && err == nil {
					return receipt, marker
				}
				return receipt, err
			}
			out, e := PrepareDescribeV1(context.Background(), q)
			h, readErr := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
			if readErr != nil || fixture.Manager().Census().Workers != 0 {
				t.Fatal("ASSERT_DIAG_NATIVE_TRACE: FAIL")
			}
			if mode == "healthy" || mode == "packet" {
				wantCategories := []string{"RESERVE", "OBSERVE", "RESERVE", "OBSERVE", "RESERVE", "GENESIS", "OBSERVE", "RESERVE"}
				if mode == "healthy" {
					wantCategories = append(wantCategories, "OBSERVE")
				}
				if !reflect.DeepEqual(categories, wantCategories) {
					t.Fatal("ASSERT_DIAG_CANONICAL_CATEGORIES: FAIL", categories)
				}
				t.Log("ASSERT_DIAG_CANONICAL_CATEGORIES: PASS", mode, categories)
			}
			if mode == "healthy" {
				if e != nil || len(out.Packet) == 0 || h.Windows != 4 || casCalls != 9 || h.Disposition != "OBSERVED" {
					t.Fatal("ASSERT_DIAG_NATIVE_TRACE: FAIL healthy", casCalls, h.Windows)
				}
				if got := ProjectPrivatePrepareFailureV1(e); got.Stage != PrivatePrepareStageNoneV1 || got.Reason != privatePrepareNoReasonV1 {
					t.Fatal("ASSERT_DIAG_NATIVE_TRACE: FAIL healthy diagnostic")
				}
				t.Log("ASSERT_DIAG_NATIVE_TRACE: PASS healthy")
				return
			}
			want := map[string]PrivatePrepareStageV1{"reserve": privatePrepareReserveV1, "reserve-uncertain": privatePrepareReserveV1, "start": privatePrepareStartV1, "capture": privatePrepareCaptureV1, "observe": privatePrepareObserveV1, "rprep": privatePrepareRprepV1, "persist": privatePreparePersistV1, "packet": privatePreparePacketV1}[mode]
			codes := ProjectPrivatePrepareFailureV1(e)
			if e == nil || len(out.Packet) != 0 || codes.Stage != want || !errors.Is(e, ErrPreparedDescribeV1) {
				t.Fatal("ASSERT_DIAG_NATIVE_STAGES: FAIL", mode, codes.Stage.String(), codes.Reason.String())
			}
			if mode == "reserve" && (h.Windows != 0 || casCalls != 0 || h.Charged != (countersV2{})) {
				t.Fatal("ASSERT_DIAG_NATIVE_TRACE: FAIL reserve")
			}
			if mode == "reserve-uncertain" && (!errors.Is(e, errUncertainV2) || codes.Reason != privatePrepareCommitUncertainV1 || !q.Owner.uncertain || casCalls != 1 || h.Windows != 1 || h.Disposition != "PENDING") {
				t.Fatal("ASSERT_DIAG_RESERVE_UNCERTAIN: FAIL")
			}
			if mode == "observe" && (!errors.Is(e, marker) || !q.Owner.uncertain || casCalls != 2 || h.Windows != 1 || h.Disposition != "PENDING") {
				t.Fatal("ASSERT_DIAG_NATIVE_TRACE: FAIL observe")
			}
			if mode == "rprep" && (casCalls != 3 || h.Windows != 1 || h.Disposition != "OBSERVED") {
				t.Fatal("ASSERT_DIAG_NATIVE_TRACE: FAIL rprep")
			}
			if mode == "persist" && (casCalls != 5 || h.Windows != 2 || h.Disposition != "OBSERVED") {
				t.Fatal("ASSERT_DIAG_NATIVE_TRACE: FAIL persist")
			}
			if mode == "packet" && (casCalls != 8 || h.Windows != 4 || h.Disposition != "PENDING") {
				t.Fatal("ASSERT_DIAG_NATIVE_TRACE: FAIL packet")
			}
			t.Log("ASSERT_DIAG_NATIVE_STAGES: PASS", mode)
		})
	}
}

func privatePrepareCASCategoryV1(t *testing.T, raw []byte) string {
	t.Helper()
	var v struct{ Schema, Event string }
	if json.Unmarshal(raw, &v) != nil {
		t.Fatal("SETUP_DIAG_CAS_JSON")
	}
	if strings.HasSuffix(v.Schema, "/genesis") {
		return "GENESIS"
	}
	if v.Event == "RESERVE" || v.Event == "OBSERVE" {
		return v.Event
	}
	t.Fatal("ASSERT_DIAG_CANONICAL_CATEGORIES: FAIL unknown category")
	return "UNKNOWN"
}

type privatePrepareCaptureTraceV1 struct {
	Categories                                        []string
	Sequence, Windows                                 uint64
	Disposition                                       string
	Charged                                           [14]uint64
	Readback, OwnerUncertain                          bool
	PacketBytes                                       int
	ZeroContinuation                                  bool
	WorkersBeforeCleanup, WorkersAfterCleanup         uint64
	Stage, Reason, DeferredFinish                     string
	PreparedSentinel, MixedSentinel, UnjoinedSentinel bool
}

// This helper intentionally exits the test process with status1 through the
// UNCHANGED fixture's real failing t.Cleanup. It is never normal capture success.
func TestPrivatePrepareFailureCaptureChild(t *testing.T) {
	if os.Getenv("LSP_TRACE_DIAG_CAPTURE_EXPECTED_FAILURE") != "1" {
		return
	}
	var trace privatePrepareCaptureTraceV1
	var fixture *sessionruntime.ControlledDescribePipeFixture
	// Registered first, runs LAST: do not suppress, replace, retry or repair the
	// existing fixture cleanup, and preserve its native live-worker observation.
	t.Cleanup(func() {
		trace.WorkersAfterCleanup = uint64(fixture.Manager().Census().Workers)
		raw, err := json.Marshal(trace)
		if err != nil {
			t.Fatal("SETUP_CAPTURE_TRACE_ENCODING")
		}
		fmt.Println("DIAG_CAPTURE_TRACE=" + string(raw))
	})
	q, f := prepareDescribeNativeFixtureV1(t, -1)
	fixture = f
	q.Capture.Line = 1000
	q.Owner.k.cas = func(ctx context.Context, root *publication.Root, selector string, predecessor publication.BoundFilePredecessor, raw []byte, validate func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		trace.Categories = append(trace.Categories, privatePrepareCASCategoryV1(t, raw))
		return publication.CompareAndReplaceBoundFile(ctx, root, selector, predecessor, raw, validate)
	}
	out, err := PrepareDescribeV1(context.Background(), q)
	head, readErr := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
	codes := ProjectPrivatePrepareFailureV1(err)
	trace.Sequence, trace.Windows, trace.Disposition, trace.Charged = head.Sequence, head.Windows, head.Disposition, head.Charged.vector()
	trace.Readback, trace.OwnerUncertain = readErr == nil, q.Owner.uncertain
	trace.PacketBytes, trace.ZeroContinuation = len(out.Packet), out.Continuation == (PreparedDescribeAnchorsV1{})
	trace.WorkersBeforeCleanup = uint64(fixture.Manager().Census().Workers)
	trace.Stage, trace.Reason, trace.DeferredFinish = codes.Stage.String(), codes.Reason.String(), codes.Cleanup[0].String()
	trace.PreparedSentinel, trace.MixedSentinel, trace.UnjoinedSentinel = errors.Is(err, ErrPreparedDescribeV1), errors.Is(err, ErrPrivateMixedInputV2), errors.Is(err, sessionruntime.ErrPrivateProviderWindowUnjoined)
	if os.Getenv("LSP_TRACE_DIAG_CAPTURE_EXTRA_ERROR") == "1" {
		t.Error("ASSERT_UNRELATED_CHILD_FAILURE")
	}
}

func privatePrepareCaptureChildOutputV1(t *testing.T, extraError bool) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPrivatePrepareFailureCaptureChild$", "-test.v", "-test.timeout=15s")
	extra := "0"
	if extraError {
		extra = "1"
	}
	cmd.Env = append(os.Environ(), "LSP_TRACE_DIAG_CAPTURE_EXPECTED_FAILURE=1", "LSP_TRACE_DIAG_CAPTURE_EXTRA_ERROR="+extra)
	raw, err := cmd.CombinedOutput()
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 1 || ctx.Err() != nil {
		t.Fatal("ASSERT_DIAG_CAPTURE_EXPECTED_FAILURE: FAIL expected child exit1")
	}
	return raw
}

func privatePrepareCaptureOutputV1(raw []byte) bool {
	// Closed language of this exact child: one test, original fixture cleanup,
	// one canonical trace, one failed-test marker, final FAIL and newline.
	// No arbitrary file/line/error text is accepted, including root-close errors.
	if len(raw) > 4096 {
		return false
	}
	lines := bytes.Split(raw, []byte("\n"))
	if len(lines) != 8 || string(lines[0]) != "=== RUN   TestPrivatePrepareFailureCaptureChild" ||
		string(lines[1]) != "    private_prepare_describe_pipe_fixture_test.go:36: context deadline exceeded" ||
		string(lines[2]) != "        context deadline exceeded" ||
		string(lines[3]) != "        fixture lifecycle not joined" ||
		string(lines[6]) != "FAIL" || len(lines[7]) != 0 {
		return false
	}
	const failPrefix = "--- FAIL: TestPrivatePrepareFailureCaptureChild ("
	durationLine := string(lines[5])
	if !strings.HasPrefix(durationLine, failPrefix) || !strings.HasSuffix(durationLine, "s)") {
		return false
	}
	duration := strings.TrimSuffix(strings.TrimPrefix(durationLine, failPrefix), "s)")
	parts := strings.Split(duration, ".")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) != 2 || (len(parts[0]) > 1 && parts[0][0] == '0') {
		return false
	}
	for _, part := range parts {
		for _, digit := range []byte(part) {
			if digit < '0' || digit > '9' {
				return false
			}
		}
	}
	const tracePrefix = "DIAG_CAPTURE_TRACE="
	if !bytes.HasPrefix(lines[4], []byte(tracePrefix)) {
		return false
	}
	rawTrace := bytes.TrimPrefix(lines[4], []byte(tracePrefix))
	var trace privatePrepareCaptureTraceV1
	if json.Unmarshal(rawTrace, &trace) != nil {
		return false
	}
	// Current helper emits Marshal(struct), not a permissive JSON document.
	// Exact re-encoding rejects duplicate/unknown/missing fields and extra JSON.
	canonical, err := json.Marshal(trace)
	if err != nil || !bytes.Equal(canonical, rawTrace) {
		return false
	}
	if !reflect.DeepEqual(trace.Categories, []string{"RESERVE"}) || trace.Sequence != 2 || trace.Windows != 1 || trace.Disposition != "PENDING" || !trace.Readback || trace.OwnerUncertain || trace.PacketBytes != 0 || !trace.ZeroContinuation || trace.WorkersBeforeCleanup != 1 || trace.WorkersAfterCleanup != 1 || trace.Charged[0] == 0 {
		return false
	}
	return trace.Stage == "CAPTURE" && trace.Reason == "REFUSED" && trace.DeferredFinish == "UNJOINED" && trace.PreparedSentinel && trace.MixedSentinel && trace.UnjoinedSentinel
}

func privatePrepareCaptureExpectedFailureV1(t *testing.T) {
	t.Helper()
	raw := privatePrepareCaptureChildOutputV1(t, false)
	if !privatePrepareCaptureOutputV1(raw) {
		t.Fatal("ASSERT_DIAG_CAPTURE_EXPECTED_FAILURE: FAIL child output outside expected language")
	}
	t.Logf("retained expected-failure child output (NOT success capture):\n%s", raw)
	t.Log("ASSERT_DIAG_CAPTURE_PENDING_TRACE: PASS; ASSERT_DIAG_CAPTURE_ATTRIBUTION: PASS; ASSERT_DIAG_CAPTURE_EXPECTED_FAILURE: PASS")
}

func TestPrivatePrepareFailureCaptureOracleGrammar(t *testing.T) {
	t.Log("PROCEDURE: pure complete-transcript classification; accepting cleanup controls and independent malformed/extra-output fixtures")
	trace := privatePrepareCaptureTraceV1{Categories: []string{"RESERVE"}, Sequence: 2, Windows: 1, Disposition: "PENDING", Charged: [14]uint64{1}, Readback: true, ZeroContinuation: true, WorkersBeforeCleanup: 1, WorkersAfterCleanup: 1, Stage: "CAPTURE", Reason: "REFUSED", DeferredFinish: "UNJOINED", PreparedSentinel: true, MixedSentinel: true, UnjoinedSentinel: true}
	encoded, err := json.Marshal(trace)
	if err != nil {
		t.Fatal("SETUP_CAPTURE_ORACLE_FIXTURE")
	}
	traceLine := "DIAG_CAPTURE_TRACE=" + string(encoded)
	base := "=== RUN   TestPrivatePrepareFailureCaptureChild\n    private_prepare_describe_pipe_fixture_test.go:36: context deadline exceeded\n        context deadline exceeded\n        fixture lifecycle not joined\n" + traceLine + "\n--- FAIL: TestPrivatePrepareFailureCaptureChild (3.00s)\nFAIL\n"
	for _, sample := range []struct {
		name, raw string
		want      bool
	}{
		{"expected", base, true},
		{"duration_variation", strings.Replace(base, "(3.00s)", "(12.34s)", 1), true},
		{"zero_duration", strings.Replace(base, "(3.00s)", "(0.00s)", 1), true},
		{"extra_error", strings.Replace(base, traceLine, "    another_test.go:19: unexpected failure\n"+traceLine, 1), false},
		{"extra_root_close", strings.Replace(base, traceLine, "    private_prepare_describe_pipe_fixture_test.go:44: publication root close refused\n"+traceLine, 1), false},
		{"unknown_cleanup_origin", strings.Replace(base, "pipe_fixture_test.go:36:", "pipe_fixture_test.go:44:", 1), false},
		{"unknown_error", strings.Replace(base, "fixture lifecycle not joined", "unrelated failure", 1), false},
		{"panic", "panic: unknown failure\n" + base, false},
		{"extra_failed_test", strings.Replace(base, "\nFAIL\n", "\n--- FAIL: UnrelatedTest (0.00s)\nFAIL\n", 1), false},
		{"extra_run_marker", "=== RUN   UnrelatedTest\n" + base, false},
		{"missing_trace", strings.Replace(base, traceLine+"\n", "", 1), false},
		{"duplicate_trace", strings.Replace(base, traceLine, traceLine+"\n"+traceLine, 1), false},
		{"missing_trace_field", strings.Replace(base, "\"OwnerUncertain\":false,", "", 1), false},
		{"duplicate_json_field", strings.Replace(base, "{\"Categories\":", "{\"Stage\":\"CAPTURE\",\"Categories\":", 1), false},
		{"unknown_json_field", strings.Replace(base, "{\"Categories\":", "{\"Unknown\":1,\"Categories\":", 1), false},
		{"trailing_json", strings.Replace(base, traceLine, traceLine+" {}", 1), false},
		{"wrong_stage", strings.Replace(base, "\"Stage\":\"CAPTURE\"", "\"Stage\":\"OBSERVE\"", 1), false},
		{"wrong_custody", strings.Replace(base, "\"WorkersAfterCleanup\":1", "\"WorkersAfterCleanup\":0", 1), false},
		{"missing_failed_marker", strings.Replace(base, "--- FAIL: TestPrivatePrepareFailureCaptureChild (3.00s)\n", "", 1), false},
		{"missing_final_fail", strings.TrimSuffix(base, "FAIL\n"), false},
		{"trailing_garbage", base + "unrelated output", false},
		{"trailing_blank", base + "\n", false},
		{"missing_newline", strings.TrimSuffix(base, "\n"), false},
		{"nonnumeric_duration", strings.Replace(base, "(3.00s)", "(NaNs)", 1), false},
		{"duration_garbage", strings.Replace(base, "(3.00s)", "(3.00s)garbage", 1), false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if got := privatePrepareCaptureOutputV1([]byte(sample.raw)); got != sample.want {
				t.Fatal("ASSERT_DIAG_CAPTURE_ORACLE_GRAMMAR: FAIL", sample.name, got, sample.want)
			}
			t.Log("ASSERT_DIAG_CAPTURE_ORACLE_GRAMMAR: PASS", sample.name)
		})
	}
}

func TestPrivatePrepareFailureCaptureOracleExtraChild(t *testing.T) {
	t.Log("PROCEDURE: exact child helper plus extra t.Error; unchanged real cleanup still fails; evaluate the same oracle used by native capture")
	raw := privatePrepareCaptureChildOutputV1(t, true)
	rejected := !privatePrepareCaptureOutputV1(raw)
	// Fixture control ONLY: classify the full transcript first. Removing exactly
	// this declared injection must recover the ordinary expected cleanup output;
	// no other error is stripped or permitted by the native-capture test oracle.
	extraLine := []byte("    private_prepare_failure_v1_pipe_fixture_test.go:183: ASSERT_UNRELATED_CHILD_FAILURE\n")
	control := bytes.Replace(raw, extraLine, nil, 1)
	if !rejected || bytes.Count(raw, extraLine) != 1 || !privatePrepareCaptureOutputV1(control) {
		t.Fatal("ASSERT_DIAG_CAPTURE_ORACLE_EXTRA: FAIL accepted unrelated child failure")
	}
	t.Logf("rejected extra-error child output:\n%s", raw)
	t.Log("ASSERT_DIAG_CAPTURE_ORACLE_EXTRA: PASS")
}

func TestPrivatePrepareFailureReplayWindowActual(t *testing.T) {
	for _, mode := range []string{"replay", "window"} {
		t.Run(mode, func(t *testing.T) {
			q, _ := prepareDescribeNativeFixtureV1(t, -1)
			op, e := q.Owner.ReserveMixedInputV2(context.Background(), q.CaptureAttempt, q.Capture)
			if e != nil {
				t.Fatal("SETUP_MIXED_RESERVE")
			}
			before, e := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
			if e != nil {
				t.Fatal("SETUP_MIXED_HEAD")
			}
			want := privatePrepareReplayV1
			if mode == "replay" {
				q.Capture.Start.Process.Args = []string{"changed-after-admission"}
			} else {
				op.state.envelope = sessionruntime.PrivateProviderWindowEnvelope{}
				want = privatePrepareWindowV1
			}
			out, e := ExecuteMixedInputV2(context.Background(), op, q.Capture)
			after, reopenErr := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
			if e == nil || len(out.Ledger) != 0 || ProjectPrivatePrepareFailureV1(e).Stage != want || reopenErr != nil || !reflect.DeepEqual(before, after) || !op.state.used {
				t.Fatal("ASSERT_DIAG_REPLAY_WINDOW: FAIL", mode)
			}
			// Retry must remain refused; no extra Reserve, Observe or context renewal.
			if _, retry := ExecuteMixedInputV2(context.Background(), op, q.Capture); retry == nil {
				t.Fatal("ASSERT_DIAG_REPLAY_WINDOW: FAIL retry")
			}
			if !time.Now().Before(q.Persist.Deadline) {
				t.Fatal("SETUP_MIXED_EXPIRED")
			}
			t.Log("ASSERT_DIAG_REPLAY_WINDOW: PASS", mode)
		})
	}
}

// This is a SIMULATED close report after actual join; not an OS-close fault.
// No native failure is refunded or hidden by the subprocess oracle.
type privatePrepareFinishTraceV1 struct {
	Mode, Stage, Reason, Disposition string
	Categories                       []string
	Sequence, Windows                uint64
	Charged                          [14]uint64
	ZeroAvailability, OwnerUncertain bool
	Before, After                    sessionruntime.ControlledDescribeCloseSnapshot
	WorkersBefore, WorkersAfter      uint64
}

func TestPrivatePrepareFailureFinishSimulatedCloseReportChild(t *testing.T) {
	mode := ""
	for i, arg := range os.Args {
		if arg == "private-finish-close-report" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
		}
	}
	if mode == "" {
		return
	}
	if mode != "prepare" && mode != "mixed" {
		t.Fatal("SETUP_FINISH_MODE")
	}
	trace := privatePrepareFinishTraceV1{Mode: mode}
	var fixture *sessionruntime.ControlledDescribePipeFixture
	// First registered, last executed: observe the unchanged real cleanup.
	t.Cleanup(func() {
		if fixture != nil {
			trace.After = fixture.CloseSnapshot()
			trace.WorkersAfter = uint64(fixture.Manager().Census().Workers)
		}
		raw, err := json.Marshal(trace)
		if err != nil {
			t.Fatal("SETUP_FINISH_TRACE")
		}
		fmt.Printf("FINISH_TRACE=%s\n", raw)
	})
	q, f := prepareDescribeNativeFixtureV1(t, -1, true)
	fixture = f
	q.Owner.k.cas = func(ctx context.Context, root *publication.Root, selector string, predecessor publication.BoundFilePredecessor, raw []byte, validate func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		trace.Categories = append(trace.Categories, privatePrepareCASCategoryV1(t, raw))
		return publication.CompareAndReplaceBoundFile(ctx, root, selector, predecessor, raw, validate)
	}
	var err error
	if mode == "prepare" {
		var out StagedPrepareDescribeV1
		out, err = PrepareDescribeV1(context.Background(), q)
		trace.ZeroAvailability = reflect.DeepEqual(out, StagedPrepareDescribeV1{})
	} else {
		op, e := q.Owner.ReserveMixedInputV2(context.Background(), q.CaptureAttempt, q.Capture)
		if e != nil {
			t.Fatal("SETUP_FINISH_RESERVE", e)
		}
		out, e := ExecuteMixedInputV2(context.Background(), op, q.Capture)
		err = e
		trace.ZeroAvailability = reflect.DeepEqual(out, PrivateManagedGroupingInput{})
	}
	head, readErr := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
	codes := ProjectPrivatePrepareFailureV1(err)
	trace.Stage, trace.Reason = codes.Stage.String(), codes.Reason.String()
	trace.Sequence, trace.Windows, trace.Disposition, trace.Charged = head.Sequence, head.Windows, head.Disposition, head.Charged.vector()
	trace.OwnerUncertain = q.Owner.uncertain
	trace.Before = fixture.CloseSnapshot()
	trace.WorkersBefore = uint64(fixture.Manager().Census().Workers)
	if readErr != nil || err == nil || codes.Stage != privatePrepareFinishV1 || codes.Reason != privatePrepareUnjoinedV1 ||
		!errors.Is(err, ErrPrivateMixedInputV2) || !errors.Is(err, sessionruntime.ErrPrivateProviderWindowUnjoined) || mode == "prepare" && !errors.Is(err, ErrPreparedDescribeV1) ||
		!trace.ZeroAvailability || head.Disposition != "PENDING" || head.Sequence != 2 || head.Windows != 1 || head.Charged == (countersV2{}) || q.Owner.uncertain ||
		!reflect.DeepEqual(trace.Categories, []string{"RESERVE"}) || trace.Before.CloseReports == 0 || trace.Before.ReferenceReplies != 1 || !trace.Before.PipeJoined || !trace.Before.PhysicalCloseOK || trace.WorkersBefore != 1 {
		t.Fatal("ASSERT_DIAG_FINISH_SIMULATED_CLOSE_REPORT: FAIL", mode, trace.Stage, trace.Reason, err)
	}
	t.Log("ASSERT_DIAG_FINISH_SIMULATED_CLOSE_REPORT: PASS; SIMULATED close report after actual join; not OS-close-fault")
}

func privatePrepareFinishChildOutputV1(t *testing.T, mode string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPrivatePrepareFailureFinishSimulatedCloseReportChild$", "-test.v", "-test.timeout=20s", "--", "private-finish-close-report", mode)
	raw, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("ASSERT_DIAG_FINISH_CHILD_EXIT: FAIL %v %s", err, raw)
	}
	return raw
}

func TestPrivatePrepareFailureFinishSimulatedCloseReport(t *testing.T) {
	for _, mode := range []string{"prepare", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			raw := privatePrepareFinishChildOutputV1(t, mode)
			t.Logf("actual expected-failure child transcript:\n%s", raw)
			if !privatePrepareFinishOutputV1(raw, mode) {
				t.Fatal("ASSERT_DIAG_FINISH_ORACLE: FAIL")
			}
			t.Log("ASSERT_DIAG_FINISH_ORACLE: PASS")
		})
	}
}

// This grammar belongs only to this exact fixed FINISH child. It neither
// changes nor reuses the capture expected-failure classifier.
func privatePrepareFinishOutputV1(raw []byte, mode string) bool {
	if len(raw) > 4096 || mode != "prepare" && mode != "mixed" {
		return false
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) != 9 || lines[0] != "=== RUN   TestPrivatePrepareFailureFinishSimulatedCloseReportChild" ||
		lines[1] != "    private_prepare_failure_v1_pipe_fixture_test.go:440: ASSERT_DIAG_FINISH_SIMULATED_CLOSE_REPORT: PASS; SIMULATED close report after actual join; not OS-close-fault" ||
		lines[2] != "    private_prepare_describe_pipe_fixture_test.go:36: context deadline exceeded" ||
		lines[3] != "        context deadline exceeded" || lines[4] != "        fixture lifecycle not joined" || lines[7] != "FAIL" || lines[8] != "" {
		return false
	}
	const failPrefix = "--- FAIL: TestPrivatePrepareFailureFinishSimulatedCloseReportChild ("
	if !strings.HasPrefix(lines[6], failPrefix) || !strings.HasSuffix(lines[6], "s)") {
		return false
	}
	duration := strings.TrimSuffix(strings.TrimPrefix(lines[6], failPrefix), "s)")
	parts := strings.Split(duration, ".")
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) != 2 || len(parts[0]) > 1 && parts[0][0] == '0' {
		return false
	}
	for _, part := range parts {
		for _, digit := range []byte(part) {
			if digit < '0' || digit > '9' {
				return false
			}
		}
	}
	const prefix = "FINISH_TRACE="
	if !strings.HasPrefix(lines[5], prefix) {
		return false
	}
	body := strings.TrimPrefix(lines[5], prefix)
	var trace privatePrepareFinishTraceV1
	if json.Unmarshal([]byte(body), &trace) != nil {
		return false
	}
	canonical, err := json.Marshal(trace)
	if err != nil || string(canonical) != body {
		return false
	}
	want := sessionruntime.ControlledDescribeCloseSnapshot{CloseReports: 2, ReferenceReplies: 1, PipeJoined: true, PhysicalCloseOK: true}
	if trace.Mode != mode || trace.Stage != "FINISH" || trace.Reason != "UNJOINED" || trace.Disposition != "PENDING" ||
		!reflect.DeepEqual(trace.Categories, []string{"RESERVE"}) || trace.Sequence != 2 || trace.Windows != 1 || !trace.ZeroAvailability || trace.OwnerUncertain ||
		trace.Before != want || trace.After != want || trace.WorkersBefore != 1 || trace.WorkersAfter != 1 {
		return false
	}
	for _, charged := range trace.Charged {
		if charged == 0 {
			return false
		}
	}
	return true
}

func TestPrivatePrepareFailureFinishDefaultHealthyControl(t *testing.T) {
	q, fixture := prepareDescribeNativeFixtureV1(t, -1)
	out, err := PrepareDescribeV1(context.Background(), q)
	snapshot := fixture.CloseSnapshot()
	if err != nil || len(out.Packet) == 0 || fixture.Manager().Census().Workers != 0 ||
		snapshot.CloseReports != 2 || snapshot.ReferenceReplies != 1 || !snapshot.PipeJoined || !snapshot.PhysicalCloseOK {
		t.Fatal("ASSERT_DIAG_FINISH_DEFAULT_HEALTHY: FAIL", err, snapshot)
	}
	t.Log("ASSERT_DIAG_FINISH_DEFAULT_HEALTHY: PASS")
}

func TestPrivatePrepareFailureFinishOracleGrammar(t *testing.T) {
	trace := privatePrepareFinishTraceV1{Mode: "prepare", Stage: "FINISH", Reason: "UNJOINED", Disposition: "PENDING", Categories: []string{"RESERVE"}, Sequence: 2, Windows: 1, ZeroAvailability: true, WorkersBefore: 1, WorkersAfter: 1}
	trace.Before = sessionruntime.ControlledDescribeCloseSnapshot{CloseReports: 2, ReferenceReplies: 1, PipeJoined: true, PhysicalCloseOK: true}
	trace.After = trace.Before
	for i := range trace.Charged {
		trace.Charged[i] = 1
	}
	raw, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	base := "=== RUN   TestPrivatePrepareFailureFinishSimulatedCloseReportChild\n" +
		"    private_prepare_failure_v1_pipe_fixture_test.go:440: ASSERT_DIAG_FINISH_SIMULATED_CLOSE_REPORT: PASS; SIMULATED close report after actual join; not OS-close-fault\n" +
		"    private_prepare_describe_pipe_fixture_test.go:36: context deadline exceeded\n        context deadline exceeded\n        fixture lifecycle not joined\nFINISH_TRACE=" + string(raw) +
		"\n--- FAIL: TestPrivatePrepareFailureFinishSimulatedCloseReportChild (3.00s)\nFAIL\n"
	for _, sample := range []struct {
		name, raw string
		want      bool
	}{
		{"valid", base, true},
		{"extra-error", strings.Replace(base, "FINISH_TRACE=", "    unrelated_test.go:1: unrelated failure\nFINISH_TRACE=", 1), false},
		{"root-error", strings.Replace(base, "FINISH_TRACE=", "    private_prepare_describe_pipe_fixture_test.go:44: root close failed\nFINISH_TRACE=", 1), false},
		{"panic", base + "panic: extra\n", false},
		{"extra-marker", base + "FAIL\n", false},
		{"trailing", base + "\n", false},
		{"missing-final", strings.TrimSuffix(base, "FAIL\n"), false},
		{"missing-cleanup", strings.Replace(base, "        fixture lifecycle not joined\n", "", 1), false},
		{"wrong-stage", strings.Replace(base, `"Stage":"FINISH"`, `"Stage":"CAPTURE"`, 1), false},
		{"wrong-mode", strings.Replace(base, `"Mode":"prepare"`, `"Mode":"mixed"`, 1), false},
		{"forged-join", strings.Replace(base, `"PipeJoined":true`, `"PipeJoined":false`, 1), false},
		{"forged-workers", strings.Replace(base, `"WorkersAfter":1`, `"WorkersAfter":0`, 1), false},
		{"unknown-field", strings.Replace(base, `"Mode":"prepare"`, `"Unknown":true,"Mode":"prepare"`, 1), false},
		{"duplicate-field", strings.Replace(base, `"Mode":"prepare"`, `"Mode":"prepare","Mode":"prepare"`, 1), false},
		{"bad-duration", strings.Replace(base, "(3.00s)", "(NaNs)", 1), false},
		{"byte-bound", base + strings.Repeat("x", 4096), false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if got := privatePrepareFinishOutputV1([]byte(sample.raw), "prepare"); got != sample.want {
				t.Fatal("ASSERT_DIAG_FINISH_GRAMMAR: FAIL", got, sample.want)
			}
			t.Log("ASSERT_DIAG_FINISH_GRAMMAR: PASS")
		})
	}
}
