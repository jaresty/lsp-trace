package lifecycleops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

const (
	assertFourContracts      = "ASSERT_LIFECYCLE_EXECUTOR_EXACT_FOUR_CONTRACTS"
	assertStructuralFirst    = "ASSERT_LIFECYCLE_EXECUTOR_STRUCTURAL_FIRST"
	assertFaithfulResults    = "ASSERT_LIFECYCLE_EXECUTOR_FAITHFUL_RESULTS_FAILURES"
	assertContextPropagation = "ASSERT_LIFECYCLE_EXECUTOR_CONTEXT_PROPAGATION"
	assertStaleGuidance      = "ASSERT_LIFECYCLE_EXECUTOR_STALE_CURRENT_GENERATION_RETRY"
	assertRecoveryGuidance   = "ASSERT_LIFECYCLE_EXECUTOR_BOUNDED_HOST_RECOVERY"
	assertCancellationTruth  = "ASSERT_LIFECYCLE_EXECUTOR_CANCELLATION_TRUTH"
)

func rejectExecutorPerturbation(t *testing.T, assertion string) {
	t.Helper()
	if os.Getenv("LSP_TRACE_LIFECYCLE_EXECUTOR_PERTURB") == assertion {
		t.Fatalf("%s: FAIL injected minimal wrong state", assertion)
	}
}

func executeLifecycle(t *testing.T, executor *Executor, name operation.Name, input string) (operation.Result, *operation.Failure) {
	t.Helper()
	return executor.Execute(context.Background(), operation.Request{Name: name, RequestID: "request-1", Input: json.RawMessage(input)})
}

func TestExecutorExactFourContracts(t *testing.T) {
	for _, assertion := range []string{assertFourContracts, assertStructuralFirst, assertFaithfulResults, assertContextPropagation} {
		t.Log("ASSERTION: " + assertion)
	}
	f := &fakeRuntime{
		records: []sessionruntime.Record{record("known", 4)},
		result:  session.LifecycleResult{IntentID: "intent-1", Generation: 4, State: session.Stopping, Joined: true},
	}
	executor := NewExecutor(New(f))

	t.Run(assertFourContracts, func(t *testing.T) {
		rejectExecutorPerturbation(t, assertFourContracts)
		cases := []struct {
			name  operation.Name
			input string
			want  any
		}{
			{operation.Name("session_list"), `{}`, ListSnapshot{}},
			{operation.Name("session_status"), `{"session_id":"known","generation":4}`, sessionruntime.Record{}},
			{operation.Name("session_stop"), `{"session_id":"known","generation":4,"caller_id":"caller"}`, Acceptance{}},
			{operation.Name("session_restart"), `{"session_id":"known","generation":4,"caller_id":"caller"}`, Acceptance{}},
		}
		for _, tc := range cases {
			got, failure := executeLifecycle(t, executor, tc.name, tc.input)
			if failure != nil || reflect.TypeOf(got.Value) != reflect.TypeOf(tc.want) {
				t.Fatalf("%s[%s]: type=%T failure=%v", assertFourContracts, tc.name, got.Value, failure)
			}
		}
		if _, failure := executeLifecycle(t, executor, operation.Name("session_operation_status"), `{}`); failure == nil || failure.Code != operation.FailureNotImplemented {
			t.Fatalf("%s: fifth contract accepted: %v", assertFourContracts, failure)
		}
		t.Log("PASS " + assertFourContracts)
	})

	t.Run(assertStructuralFirst, func(t *testing.T) {
		rejectExecutorPerturbation(t, assertStructuralFirst)
		before := f.calls
		for _, tc := range []struct {
			name  operation.Name
			input string
		}{
			{operation.Name("session_list"), `{"unknown":true}`},
			{operation.Name("session_status"), `{"session_id":"known","generation":4,"caller_id":"surplus"}`},
			{operation.Name("session_stop"), `{"session_id":"known","generation":4}`},
			{operation.Name("session_restart"), `{"session_id":"","caller_id":"caller"}`},
		} {
			if _, failure := executeLifecycle(t, executor, tc.name, tc.input); failure == nil || failure.Code != operation.FailureInvalidInput {
				t.Fatalf("%s[%s]: %v", assertStructuralFirst, tc.name, failure)
			}
		}
		if f.calls != before {
			t.Fatalf("%s: malformed input delegated: before=%d after=%d", assertStructuralFirst, before, f.calls)
		}
		t.Log("PASS " + assertStructuralFirst)
	})

	t.Run(assertFaithfulResults, func(t *testing.T) {
		rejectExecutorPerturbation(t, assertFaithfulResults)
		result, failure := executeLifecycle(t, executor, operation.Name("session_status"), `{"session_id":"missing","generation":1}`)
		if failure == nil || failure.Code != string(FailureSessionNotFound) || result.Value != nil {
			t.Fatalf("%s: result=%+v failure=%v", assertFaithfulResults, result, failure)
		}
		result, failure = executeLifecycle(t, executor, operation.Name("session_stop"), `{"session_id":"known","generation":4,"caller_id":"caller"}`)
		acceptance, ok := result.Value.(Acceptance)
		if failure != nil || !ok || acceptance.OperationID != "intent-1" || !acceptance.Joined {
			t.Fatalf("%s: result=%+v failure=%v", assertFaithfulResults, result, failure)
		}
		t.Log("PASS " + assertFaithfulResults)
	})
}

type contextRuntime struct {
	*fakeRuntime
	observed error
}

func (r *contextRuntime) Stop(ctx context.Context, _, _ string) session.LifecycleResult {
	r.observed = context.Cause(ctx)
	return session.LifecycleResult{Failure: session.ResourceExhausted}
}

func TestExecutorInfersOnlyReadyGeneration(t *testing.T) {
	const assertion = "ASSERT_OMITTED_GENERATION_REQUIRES_ONE_READY"
	t.Log("ASSERTION: " + assertion)
	ready := &fakeRuntime{records: []sessionruntime.Record{record("known", 4)}}
	result, failure := executeLifecycle(t, NewExecutor(New(ready)), OperationStatus, `{"session_id":"known"}`)
	gotRecord, ok := result.Value.(sessionruntime.Record)
	if failure != nil || !ok || gotRecord.Generation != 4 {
		t.Fatalf("%s: READY omitted generation result=%+v failure=%v", assertion, result, failure)
	}
	notReady := record("known", 4)
	notReady.State = session.Stopped
	_, failure = executeLifecycle(t, NewExecutor(New(&fakeRuntime{records: []sessionruntime.Record{notReady}})), OperationStatus, `{"session_id":"known"}`)
	if failure == nil || failure.Code != "SESSION_NOT_READY" {
		t.Fatalf("%s: non-READY failure=%v", assertion, failure)
	}
	t.Log("PASS " + assertion)
}

func TestExecutorActionableDiagnostics(t *testing.T) {
	tests := []struct {
		assertion string
		runtime   *fakeRuntime
		ctx       context.Context
		name      operation.Name
		input     string
		code      string
		want      []string
	}{
		{
			assertion: assertStaleGuidance,
			runtime:   &fakeRuntime{records: []sessionruntime.Record{record("known", 4)}},
			ctx:       context.Background(),
			name:      OperationStatus,
			input:     `{"session_id":"known","generation":3}`,
			code:      string(FailureStaleGeneration),
			want:      []string{"current generation is 4", "retry lsp_session_v1_status with session_id \"known\" and generation 4"},
		},
		{
			assertion: assertRecoveryGuidance,
			runtime: &fakeRuntime{
				records: []sessionruntime.Record{record("known", 4)},
				result:  session.LifecycleResult{Failure: session.SessionReapIncomplete},
			},
			ctx:   context.Background(),
			name:  OperationStop,
			input: `{"session_id":"known","generation":4,"caller_id":"caller"}`,
			code:  string(FailureReapIncomplete),
			want:  []string{"host operator must inspect and reap the trusted local child before retrying"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.assertion, func(t *testing.T) {
			t.Log("ASSERTION: " + tc.assertion)
			rejectExecutorPerturbation(t, tc.assertion)
			result, failure := NewExecutor(New(tc.runtime)).Execute(tc.ctx, operation.Request{Name: tc.name, RequestID: "request-1", Input: json.RawMessage(tc.input)})
			if failure == nil || failure.Code != tc.code || result.Value != nil || !reflect.DeepEqual(failure.Diagnostics, tc.want) {
				t.Fatalf("%s: result=%+v failure=%+v", tc.assertion, result, failure)
			}
			t.Log("PASS " + tc.assertion)
		})
	}

	t.Run(assertCancellationTruth, func(t *testing.T) {
		t.Log("ASSERTION: " + assertCancellationTruth)
		rejectExecutorPerturbation(t, assertCancellationTruth)
		runtime := &contextRuntime{fakeRuntime: &fakeRuntime{records: []sessionruntime.Record{record("known", 4)}}}
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(errors.New("caller shutdown"))
		_, failure := NewExecutor(New(runtime)).Execute(ctx, operation.Request{Name: OperationStop, RequestID: "request-1", Input: json.RawMessage(`{"session_id":"known","generation":4,"caller_id":"caller"}`)})
		want := []string{"caller observation was cancelled; an accepted lifecycle intent may continue", "query lsp_session_v1_status for the current generation before retrying"}
		if failure == nil || failure.Code != string(FailureCapacityExhausted) || !reflect.DeepEqual(failure.Diagnostics, want) {
			t.Fatalf("%s: failure=%+v", assertCancellationTruth, failure)
		}
		t.Log("PASS " + assertCancellationTruth)
	})
}

func TestExecutorPropagatesCallerContext(t *testing.T) {
	rejectExecutorPerturbation(t, assertContextPropagation)
	runtime := &contextRuntime{fakeRuntime: &fakeRuntime{records: []sessionruntime.Record{record("known", 4)}}}
	executor := NewExecutor(New(runtime))
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("caller shutdown")
	cancel(cause)
	_, failure := executor.Execute(ctx, operation.Request{Name: operation.Name("session_stop"), RequestID: "request-1", Input: json.RawMessage(`{"session_id":"known","generation":4,"caller_id":"caller"}`)})
	if failure == nil || failure.Code != string(FailureCapacityExhausted) || !errors.Is(runtime.observed, cause) {
		t.Fatalf("%s: failure=%v observed=%v", assertContextPropagation, failure, runtime.observed)
	}
	t.Log("PASS " + assertContextPropagation)
}
