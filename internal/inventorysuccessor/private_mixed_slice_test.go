//go:build lsp_trace_controlled_describe_fixture

package inventorysuccessor

import (
	"context"
	"reflect"
	"testing"

	"lsp-trace/internal/programcadmission"
)

// Uses the funded native reservation and its real private frozen request, not a
// clone surrogate. Populated process arrays are frozen only, never executed.
func TestPrivateMixedSliceFreeze(t *testing.T) {
	t.Log("PROCEDURE: TestPrivateMixedSliceFreeze; ReserveMixedInputV2 -> inspect funded frozen request -> mutate caller arrays")
	for _, shape := range []string{"nil", "empty", "populated"} {
		t.Run(shape, func(t *testing.T) {
			q, _ := prepareDescribeNativeFixtureV1(t, -1)
			switch shape {
			case "empty":
				q.Capture.Start.Process.Args = []string{}
				q.Capture.Start.Process.Env = []string{}
				q.Capture.Start.RelationProviders = []string{}
			case "populated":
				q.Capture.Start.Process.Args = []string{"freeze-only-arg"}
				q.Capture.Start.Process.Env = []string{"SLICE_LITERAL=1"}
				q.Capture.Start.RelationProviders = []string{"builtin"}
			}
			op, e := q.Owner.ReserveMixedInputV2(context.Background(), q.CaptureAttempt, q.Capture)
			if e != nil || op.state == nil {
				t.Fatal("SETUP_SLICE_RESERVATION", e)
			}
			if op.state.head == q.Persist.InitialHead || q.Owner.head != op.state.head {
				t.Fatal("SETUP_SLICE_CONFIRMED_FUNDING")
			}
			if !reflect.DeepEqual(q.Capture, op.state.request) {
				t.Error("ASSERT_INV_SLICE_SHAPE: FAIL")
			} else {
				t.Log("ASSERT_INV_SLICE_SHAPE: PASS")
			}
			if shape != "populated" {
				return
			}
			q.Capture.Kinds[0] = programcadmission.TypedCalls
			q.Capture.Start.Process.Args[0] = "changed-arg"
			q.Capture.Start.Process.Env[0] = "CHANGED=1"
			q.Capture.Start.RelationProviders[0] = "changed-provider"
			frozen := op.state.request
			if frozen.Kinds[0] != programcadmission.TypedReferencesSymbol || frozen.Start.Process.Args[0] != "freeze-only-arg" || frozen.Start.Process.Env[0] != "SLICE_LITERAL=1" || frozen.Start.RelationProviders[0] != "builtin" {
				t.Error("ASSERT_INV_SLICE_COPY: FAIL")
			} else {
				t.Log("ASSERT_INV_SLICE_COPY: PASS")
			}
		})
	}
}

func TestPrivateMixedSliceChangedReplay(t *testing.T) {
	t.Log("PROCEDURE: TestPrivateMixedSliceChangedReplay; funded reservation -> mutate original Args -> exact early refusal, zero runtime workers")
	q, _ := prepareDescribeNativeFixtureV1(t, -1)
	q.Capture.Start.Process.Args = []string{"original-caller-arg"}
	op, e := q.Owner.ReserveMixedInputV2(context.Background(), q.CaptureAttempt, q.Capture)
	if e != nil {
		t.Fatal("SETUP_SLICE_RESERVATION", e)
	}
	changed := q.Capture
	changed.Start.Process.Args[0] = "mutated-caller-arg"
	_, e = ExecuteMixedInputV2(context.Background(), op, changed)
	if e == nil || e.Error() != ErrPrivateMixedInputV2.Error() || !op.state.used || q.Capture.Manager.Census().Workers != 0 {
		t.Error("ASSERT_INV_SLICE_REPLAY: FAIL", e)
	} else {
		t.Log("ASSERT_INV_SLICE_REPLAY: PASS")
	}
}
