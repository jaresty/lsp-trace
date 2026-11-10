//go:build darwin || linux

package sourceoperator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
)

func TestFailureDiagnosticRunGuardModeled(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, context.Background(), canceled} {
		out, err := Run(ctx, Request{})
		if !errors.Is(err, ErrRefused) || out.FailureDiagnosticV1().Stage != FailureGuardV1 || out.FailureDiagnosticV1().Class != FailureRefusedV1 {
			t.Fatal("ASSERT_RUN_GUARD_STAGE")
		}
	}
	t.Log("ASSERT_RUN_GUARD_STAGE: PASS (MODELED; no owner/provider startup)")
}
func TestFailureDiagnosticJSONModeled(t *testing.T) {
	out := Outcome{Schema: "SOURCE_NATIVE_V1", Disposition: "REFUSED"}
	baseline := struct {
		Schema, Disposition, NativeID string
		Published                     [2]bool
		Readback                      [2]bool
		Counts                        sc.PrivateSourceCounts
	}{out.Schema, out.Disposition, out.NativeID, out.Published, out.Readback, out.Counts}
	before, _ := json.Marshal(baseline)
	out.failV1(FailureCaptureReserveV1, errors.New("SECRET_SENTINEL"))
	out.failV1(FailurePublicationV1, context.Canceled)
	out.failure.PublicationClose = true
	out.terminalDeadline = time.Now()
	after, _ := json.Marshal(out)
	if string(before) != string(after) {
		t.Fatal("ASSERT_OUTCOME_JSON_COMPATIBLE")
	}
	if out.failure.Stage != FailureCaptureReserveV1 || out.failure.Class != FailureOtherV1 || !out.failure.PublicationClose {
		t.Fatal("ASSERT_FIRST_OPERATION_PRESERVED")
	}
	if FailureStageV1(255).String() != "unknown" || FailureClassV1(255).String() != "other" {
		t.Fatal("ASSERT_ENUM_CLOSED")
	}
	t.Log("ASSERT_OUTCOME_JSON_COMPATIBLE: PASS; ASSERT_FIRST_OPERATION_PRESERVED: PASS; ASSERT_ENUM_CLOSED: PASS (MODELED metadata)")
}
