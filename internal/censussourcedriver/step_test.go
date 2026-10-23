package censussourcedriver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"sync"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
)

type stepObjects struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *stepObjects) Put(_ context.Context, value []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	digest := sha256.Sum256(value)
	key := "sha256:" + hex.EncodeToString(digest[:])
	if s.m == nil {
		s.m = map[string][]byte{}
	}
	s.m[key] = bytes.Clone(value)
	return key, nil
}

func (s *stepObjects) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.m[key]
	if !ok {
		return nil, errors.New("missing object")
	}
	return bytes.Clone(value), nil
}

type stepHeads struct {
	mu  sync.Mutex
	raw []byte
}

func (s *stepHeads) Load(context.Context, string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.raw), nil
}

func (s *stepHeads) CompareAndSwap(_ context.Context, _ string, expected censussourcecontinuation.HeadRevision, value []byte) censussourcecontinuation.CASResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := censussourcecontinuation.ParseHeadRevision(s.raw)
	if err != nil || current != expected {
		return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASNotCommitted}
	}
	s.raw = bytes.Clone(value)
	return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitted}
}

func stepPlan(t *testing.T) censussourceplan.Plan {
	t.Helper()
	files := []censusacquisition.SourceFile{
		{Path: "a.go", URI: "file:///workspace/a.go", LanguageID: "go"},
		{Path: "b.go", URI: "file:///workspace/b.go", LanguageID: "go"},
		{Path: "c.go", URI: "file:///workspace/c.go", LanguageID: "go"},
	}
	plan, err := censussourceplan.Build(files, "/workspace", 1)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func stepTerminal(plan censussourceplan.Plan, ordinal int, files []censusacquisition.SourceFile, outcome censussourcecontinuation.Outcome) censussourcecontinuation.Terminal {
	discovery := censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosureSuccessful}
	if outcome == censussourcecontinuation.OutcomeCompleteNonempty {
		discovery.Complete = true
		discovery.Targets = []censusacquisition.PreparedTarget{{}}
	}
	return censussourcecontinuation.Terminal{PlanIdentity: plan.Identity, Authority: 0, Ordinal: ordinal, SourceFiles: append([]censusacquisition.SourceFile(nil), files...), Discovery: discovery, Outcome: outcome}
}

func stepRequest(t *testing.T, workflow string, objects *stepObjects, heads *stepHeads, calls *[]int) StepRequest {
	t.Helper()
	plan := stepPlan(t)
	return StepRequest{
		Plan: plan, PartitionSize: 1, WorkflowKey: workflow, HeadKey: workflow, Objects: objects, Heads: heads,
		Discover: func(_ context.Context, ordinal int, files []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error) {
			*calls = append(*calls, ordinal)
			if !reflect.DeepEqual(files, plan.Partitions[ordinal].Members) {
				t.Fatalf("ASSERT_STEP_EXACT_MEMBERS ordinal=%d got=%+v want=%+v", ordinal, files, plan.Partitions[ordinal].Members)
			}
			return stepTerminal(plan, ordinal, files, censussourcecontinuation.OutcomeSuccessfulEmpty), nil
		},
	}
}

func TestStepFreshOrdinalZeroExactMembers(t *testing.T) {
	objects, heads := &stepObjects{}, &stepHeads{}
	calls := []int{}
	result, err := Step(context.Background(), stepRequest(t, "stable-workflow", objects, heads, &calls))
	if err != nil || !reflect.DeepEqual(calls, []int{0}) || result.SelectedOrdinal != 0 || result.SelectedOutcome != censussourcecontinuation.OutcomeSuccessfulEmpty || result.Continuation.Status != censussourcecontinuation.StatusCommitted || result.Continuation.Ordinal != 0 || result.Continuation.Outcome != result.SelectedOutcome {
		t.Fatalf("ASSERT_STEP_FRESH_ORDINAL_ZERO result=%+v calls=%v err=%v", result, calls, err)
	}
}

func TestStepExistingHeadSelectsNextWithoutReplay(t *testing.T) {
	objects, heads := &stepObjects{}, &stepHeads{}
	calls := []int{}
	request := stepRequest(t, "stable-workflow", objects, heads, &calls)
	if _, err := Step(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	calls = nil
	result, err := Step(context.Background(), request)
	if err != nil || !reflect.DeepEqual(calls, []int{1}) || result.SelectedOrdinal != 1 || result.Continuation.Ordinal != 1 {
		t.Fatalf("ASSERT_STEP_EXISTING_HEAD_NEXT_NO_REPLAY result=%+v calls=%v err=%v", result, calls, err)
	}
}

func TestStepAdvancesOnlyOneOfThreePartitions(t *testing.T) {
	objects, heads := &stepObjects{}, &stepHeads{}
	calls := []int{}
	result, err := Step(context.Background(), stepRequest(t, "stable-workflow", objects, heads, &calls))
	if err != nil || result.Continuation.CommittedPrefix != 1 || !reflect.DeepEqual(calls, []int{0}) {
		t.Fatalf("ASSERT_STEP_ONE_OF_THREE result=%+v calls=%v err=%v", result, calls, err)
	}
}

func TestStepCallbackErrorStillCoordinatesExactlyOnce(t *testing.T) {
	objects, heads := &stepObjects{}, &stepHeads{}
	request := stepRequest(t, "stable-workflow", objects, heads, new([]int))
	request.Discover = func(context.Context, int, []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error) {
		return censussourcecontinuation.Terminal{}, errors.New("sentinel discovery")
	}
	coordinateCalls := 0
	_, err := step(context.Background(), request, func(ctx context.Context, request censussourcecontinuation.Request) (censussourcecontinuation.Result, error) {
		coordinateCalls++
		return censussourcecontinuation.CoordinateNext(ctx, request)
	})
	if !errors.Is(err, ErrStep) || coordinateCalls != 1 {
		t.Fatalf("ASSERT_STEP_CALLBACK_ERROR_COORDINATES_ONCE calls=%d err=%v", coordinateCalls, err)
	}
}

func TestStepRejectsMalformedHeadBeforeDiscovery(t *testing.T) {
	objects, heads := &stepObjects{}, &stepHeads{raw: []byte("not-json")}
	calls := []int{}
	_, err := Step(context.Background(), stepRequest(t, "stable-workflow", objects, heads, &calls))
	if !errors.Is(err, ErrStep) || len(calls) != 0 {
		t.Fatalf("ASSERT_STEP_MALFORMED_HEAD_FAILS_CLOSED calls=%v err=%v", calls, err)
	}
}

func TestStepFailsClosedOnCoordinatorStatusAndMismatch(t *testing.T) {
	for _, test := range []struct {
		name   string
		result censussourcecontinuation.Result
	}{
		{"conflict", censussourcecontinuation.Result{Status: censussourcecontinuation.StatusConflict, Ordinal: 0, Outcome: censussourcecontinuation.OutcomeSuccessfulEmpty}},
		{"retained", censussourcecontinuation.Result{Status: censussourcecontinuation.StatusRetained, Ordinal: 0, Outcome: censussourcecontinuation.OutcomeSuccessfulEmpty}},
		{"indeterminate", censussourcecontinuation.Result{Status: censussourcecontinuation.StatusIndeterminateExecution, Ordinal: 0, Outcome: censussourcecontinuation.OutcomeSuccessfulEmpty}},
		{"ordinal mismatch", censussourcecontinuation.Result{Status: censussourcecontinuation.StatusCommitted, Ordinal: 1, Outcome: censussourcecontinuation.OutcomeSuccessfulEmpty}},
		{"outcome mismatch", censussourcecontinuation.Result{Status: censussourcecontinuation.StatusCommitted, Ordinal: 0, Outcome: censussourcecontinuation.OutcomeCompleteNonempty}},
	} {
		t.Run(test.name, func(t *testing.T) {
			objects, heads := &stepObjects{}, &stepHeads{}
			calls := []int{}
			request := stepRequest(t, "stable-workflow", objects, heads, &calls)
			coordinateCalls := 0
			result, err := step(context.Background(), request, func(context.Context, censussourcecontinuation.Request) (censussourcecontinuation.Result, error) {
				coordinateCalls++
				return test.result, nil
			})
			if !errors.Is(err, ErrStep) || coordinateCalls != 1 || result.Continuation != test.result {
				t.Fatalf("ASSERT_STEP_STATUS_OR_MISMATCH_FAILS_CLOSED result=%+v calls=%d err=%v", result, coordinateCalls, err)
			}
		})
	}
}

func TestStepPreservesStableWorkflowAndHeadIdentity(t *testing.T) {
	objects, heads := &stepObjects{}, &stepHeads{}
	calls := []int{}
	request := stepRequest(t, "stable-workflow", objects, heads, &calls)
	for invocation := 0; invocation < 2; invocation++ {
		_, err := step(context.Background(), request, func(ctx context.Context, got censussourcecontinuation.Request) (censussourcecontinuation.Result, error) {
			if got.WorkflowKey != "stable-workflow" || got.HeadKey != "stable-workflow" {
				t.Fatalf("ASSERT_STEP_STABLE_WORKFLOW_IDENTITY request=%+v", got)
			}
			return censussourcecontinuation.CoordinateNext(ctx, got)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
