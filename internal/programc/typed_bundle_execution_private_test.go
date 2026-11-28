package programc

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/graph"
)

func TestExecutionCoreExactOutcome(t *testing.T) {
	a, b := node("a", 0), node("b", 1)
	input := validV5(t, []graph.Node{a, b}, []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{}}}})
	want, f := Compute(input, 19)
	if f != nil {
		t.Fatal(f)
	}
	got, f := Compute(bytes.Clone(input), 19)
	if f != nil {
		t.Fatal(f)
	}
	if got.LogicalDigest != want.LogicalDigest || !equalCommunities(got.Communities, want.Communities) || !bytes.Equal(got.Source.InputBytes(), want.Source.InputBytes()) || !bytes.Equal(got.Source.GraphV5Bytes(), want.Source.GraphV5Bytes()) {
		t.Fatal("ASSERT_CORE_SUCCESS_EXACT")
	}
}
func TestExecutionCanonicalCommunities(t *testing.T) {
	a, b := node("a", 0), node("b", 1)
	got, f := Compute(validV5(t, []graph.Node{b, a}, nil), 7)
	if f != nil {
		t.Fatal(f)
	}
	if got.Outcome != "EMPTY" || !validCanonicalCommunities(got.Communities, got.Projection.NodeIdentities) {
		t.Fatal("ASSERT_EDGELESS_EXACT")
	}
	nodes := []string{"a", "b"}
	for _, tc := range []struct {
		name  string
		c     []Community
		valid bool
	}{
		{"valid", []Community{{Members: []string{"a"}}, {Members: []string{"b"}}}, true},
		{"omitted", []Community{{Members: []string{"a"}}}, false},
		{"duplicate", []Community{{Members: []string{"a"}}, {Members: []string{"a"}}}, false},
		{"empty", []Community{{}, {Members: []string{"a", "b"}}}, false},
		{"unordered-members", []Community{{Members: []string{"b", "a"}}}, false},
		{"unordered-communities", []Community{{Members: []string{"b"}}, {Members: []string{"a"}}}, false},
		{"foreign", []Community{{Members: []string{"a", "c"}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if validCanonicalCommunities(tc.c, nodes) != tc.valid {
				t.Fatal("ASSERT_CANONICAL_PARTITION")
			}
		})
	}
	if MaxWorkerInputBytes != int64(64<<20) {
		t.Fatal("ASSERT_SHARED_INPUT_BOUND")
	}
}
func TestBundleExecutionBoundaries(t *testing.T) {
	b := typedStageBundlePrivateTest(t)
	expected, e := NewGroupingBundleExpectationPrivate(b)
	if e != nil {
		t.Fatal(e)
	}
	direct, e := b.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"normal", "nil-context", "cancelled", "expired", "empty", "large", "missing-expectation", "corrupt", "truncated", "foreign-expectation", "foreign-input"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			raw := b.CanonicalBytes()
			exp := expected
			switch mode {
			case "nil-context":
				ctx = nil
			case "cancelled":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			case "expired":
				c, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
				ctx = c
			case "empty":
				raw = nil
			case "large":
				raw = make([]byte, GroupingCaseBundleMaxBytesPrivate+1)
			case "missing-expectation":
				exp = GroupingBundleExpectationPrivate{}
			case "corrupt":
				raw[len(raw)/2] ^= 1
			case "truncated":
				raw = raw[:len(raw)-1]
			case "foreign-expectation":
				exp.commitment.ID = "foreign"
			case "foreign-input":
				raw = bytes.Replace(raw, []byte("controlled-stage-two"), []byte("controlled-stage-new"), 1)
			}
			got, obs, f := ExecuteGroupingCaseBundlePrivate(ctx, exp, raw)
			if mode == "normal" || mode == "nil-context" {
				if f != nil || !bytes.Equal(got, direct.CanonicalBytes()) || !obs.Imported || !obs.Executed || !obs.Checked {
					t.Fatalf("ASSERT_EXECUTION_EXACT: %v %+v", f, obs)
				}
			} else {
				if f == nil || got != nil || obs.Imported || obs.Executed || obs.Checked {
					t.Fatalf("ASSERT_ENTRY_NO_OUTPUT: %v %+v", f, obs)
				}
				if mode == "cancelled" && f.Code != CodeCancelled {
					t.Fatal("ASSERT_CANCEL_CODE")
				}
				if mode == "expired" && f.Code != CodeTimeout {
					t.Fatal("ASSERT_EXPIRED_CODE")
				}
			}
		})
	}
}
func TestBundleExecutionDetached(t *testing.T) {
	b := typedStageBundlePrivateTest(t)
	expected, _ := NewGroupingBundleExpectationPrivate(b)
	raw := b.CanonicalBytes()
	original := bytes.Clone(raw)
	direct, e := b.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	got, _, f := executeGroupingCaseBundlePrivate(context.Background(), expected, raw, func(s bundleExecutionStagePrivate) {
		if s == bundleInputDetachedPrivate {
			for i := range raw {
				raw[i] = 0
			}
		}
	})
	if f != nil || !bytes.Equal(got, direct.CanonicalBytes()) || bytes.Equal(raw, original) {
		t.Fatalf("ASSERT_INPUT_DETACHED: %v", f)
	}
	got[0] ^= 1
	again, _, f := ExecuteGroupingCaseBundlePrivate(nil, expected, original)
	if f != nil || !bytes.Equal(again, direct.CanonicalBytes()) || bytes.Equal(got, again) {
		t.Fatalf("ASSERT_OUTPUT_DETACHED: %v", f)
	}
}
func TestBundleExecutionCancellationStages(t *testing.T) {
	b := typedStageBundlePrivateTest(t)
	expected, _ := NewGroupingBundleExpectationPrivate(b)
	for _, stage := range []bundleExecutionStagePrivate{bundleInputDetachedPrivate, bundleExecutionStartedPrivate, bundleExecutionFinishedPrivate, bundleReplayStartedPrivate, bundleReplayFinishedPrivate} {
		t.Run(string(rune('0'+stage)), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reached := false
			got, obs, f := executeGroupingCaseBundlePrivate(ctx, expected, b.CanonicalBytes(), func(s bundleExecutionStagePrivate) {
				if s == stage {
					reached = true
					cancel()
				}
			})
			if !reached || f == nil || f.Code != CodeCancelled || got != nil {
				t.Fatalf("ASSERT_CANCEL_STAGE_NO_OUTPUT: stage=%d reached=%v f=%v obs=%+v", stage, reached, f, obs)
			}
			// Even cancellation at stage entry cannot interrupt the synchronous call.
			if stage >= bundleExecutionStartedPrivate && !obs.Executed {
				t.Fatal("ASSERT_EXECUTION_RETURNS_BEFORE_SUPPRESSION")
			}
			if stage >= bundleReplayStartedPrivate && !obs.Checked {
				t.Fatal("ASSERT_FULL_REPLAY_RETURNS_BEFORE_SUPPRESSION")
			}
		})
	}
}
func TestBundleExecutionReportBarrier(t *testing.T) {
	b := typedStageBundlePrivateTest(t)
	generated, e := b.ExecuteReport()
	if e != nil {
		t.Fatal(e)
	}
	valid := generated.CanonicalBytes()
	if got, f := checkBundleExecutionReportPrivate(b, valid); f != nil || !bytes.Equal(got, valid) {
		t.Fatalf("ASSERT_FULL_CHECK_POSITIVE: %v", f)
	}
	for _, mode := range []string{"counter", "foreign"} {
		t.Run(mode, func(t *testing.T) {
			report := valid
			if mode == "counter" {
				doc := generated.Document()
				doc.Rows[0].Counts.TruePositive++
				report, e = groupingCanonicalPrivate(doc)
			} else {
				batch, err := NewGroupingMeasurementBatchPrivate(GroupingMeasurementBatchRequestPrivate{BatchID: "foreign-controlled-batch", ImplementationID: b.batch.doc.ImplementationID, ImplementationBytes: b.batch.doc.ImplementationBytes, Slots: b.batch.slots})
				if err != nil {
					t.Fatal(err)
				}
				foreign, err := ExecuteGroupingMeasurementBatchPrivate(batch)
				if err != nil {
					t.Fatal(err)
				}
				report = foreign.CanonicalBytes()
			}
			if e != nil {
				t.Fatal(e)
			}
			got, f := checkBundleExecutionReportPrivate(b, report)
			if f == nil || f.Code != CodeInternalComputation || got != nil {
				t.Fatalf("ASSERT_TYPED_REPORT_CHECK_BARRIER: mode=%s failure=%v", mode, f)
			}
		})
	}
}
func TestBundleExecutionNotificationIsolation(t *testing.T) {
	b := typedStageBundlePrivateTest(t)
	expected, _ := NewGroupingBundleExpectationPrivate(b)
	baseline, _, f := ExecuteGroupingCaseBundlePrivate(nil, expected, b.CanonicalBytes())
	if f != nil {
		t.Fatal(f)
	}
	for _, mode := range []string{"record", "panic"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			got, obs, f := executeGroupingCaseBundlePrivate(context.Background(), expected, b.CanonicalBytes(), func(bundleExecutionStagePrivate) {
				calls++
				if mode == "panic" {
					panic("diagnostic only")
				}
			})
			if f != nil || !bytes.Equal(got, baseline) || calls != 5 || !obs.Checked {
				t.Fatalf("ASSERT_NOTIFICATION_NONINTERFERENCE: %v calls=%d", f, calls)
			}
		})
	}
	f = executionFailurePrivate(CodeInternalComputation, errors.New(strings.Repeat("sensitive", 1000)))
	if len(f.Message) != 1024 {
		t.Fatal("ASSERT_EXECUTION_FAILURE_BOUND")
	}
}
