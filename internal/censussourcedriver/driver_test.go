package censussourcedriver

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func committed(prefix, count int, identity string) State {
	return State{Status: StatusCommitted, CommittedPrefix: prefix, PartitionCount: count, PlanIdentity: identity}
}

func TestRepeatCompletesThreePartitions(t *testing.T) {
	calls := 0
	got := Repeat(context.Background(), Request{
		Initial: committed(1, 3, "plan-a"),
		Bound:   3,
		Step: func(context.Context, State) (State, error) {
			calls++
			return committed(calls+1, 3, "plan-a"), nil
		},
	})
	if got.Stop != StopComplete || got.State != committed(3, 3, "plan-a") || got.Steps != 2 || got.Transitions != 2 || got.Err != nil {
		t.Fatalf("ASSERT_COMPLETE result=%+v calls=%d", got, calls)
	}
}

func TestRepeatAlreadyCompleteDoesNotStep(t *testing.T) {
	calls := 0
	got := Repeat(context.Background(), Request{
		Initial: committed(3, 3, "plan-a"),
		Bound:   1,
		Step: func(context.Context, State) (State, error) {
			calls++
			return State{}, nil
		},
	})
	if got.Stop != StopComplete || calls != 0 || got.Steps != 0 || got.Transitions != 0 || got.Err != nil {
		t.Fatalf("ASSERT_ALREADY_COMPLETE result=%+v calls=%d", got, calls)
	}
}

func TestRepeatBoundExactlyOne(t *testing.T) {
	calls := 0
	got := Repeat(context.Background(), Request{
		Initial: committed(1, 3, "plan-a"),
		Bound:   1,
		Step: func(context.Context, State) (State, error) {
			calls++
			return committed(2, 3, "plan-a"), nil
		},
	})
	if got.Stop != StopBoundReached || got.State.CommittedPrefix != 2 || calls != 1 || got.Steps != 1 || got.Transitions != 1 || got.Err != nil {
		t.Fatalf("ASSERT_BOUND_ONE result=%+v calls=%d", got, calls)
	}
}

func TestRepeatExpiredContextDoesNotStep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	got := Repeat(ctx, Request{
		Initial: committed(1, 3, "plan-a"),
		Bound:   2,
		Step: func(context.Context, State) (State, error) {
			calls++
			return State{}, nil
		},
	})
	if got.Stop != StopDeadline || !errors.Is(got.Err, context.Canceled) || calls != 0 || got.Steps != 0 {
		t.Fatalf("ASSERT_EXPIRED result=%+v calls=%d", got, calls)
	}
}

func TestRepeatTerminalStatusesDoNotRetry(t *testing.T) {
	for _, tc := range []struct {
		status Status
		stop   Stop
	}{
		{StatusConflict, StopConflict},
		{StatusRetained, StopRetained},
		{StatusIndeterminateExecution, StopIndeterminateExecution},
	} {
		t.Run(string(tc.status), func(t *testing.T) {
			calls := 0
			got := Repeat(context.Background(), Request{
				Initial: committed(1, 3, "plan-a"),
				Bound:   3,
				Step: func(context.Context, State) (State, error) {
					calls++
					return State{Status: tc.status, CommittedPrefix: 1, PartitionCount: 3, PlanIdentity: "plan-a"}, nil
				},
			})
			if got.Stop != tc.stop || calls != 1 || got.Steps != 1 || got.Transitions != 0 || got.Err != nil {
				t.Fatalf("ASSERT_NO_RETRY result=%+v calls=%d", got, calls)
			}
		})
	}
}

func TestRepeatStepErrorStopsFailed(t *testing.T) {
	want := errors.New("step failed")
	calls := 0
	got := Repeat(context.Background(), Request{
		Initial: committed(1, 3, "plan-a"),
		Bound:   3,
		Step: func(context.Context, State) (State, error) {
			calls++
			return State{}, want
		},
	})
	if got.Stop != StopFailed || !errors.Is(got.Err, want) || calls != 1 || got.Steps != 1 || got.Transitions != 0 {
		t.Fatalf("ASSERT_STEP_ERROR result=%+v calls=%d", got, calls)
	}
}

func TestRepeatRejectsInvalidRequest(t *testing.T) {
	validStep := func(context.Context, State) (State, error) { return committed(2, 3, "plan-a"), nil }
	for _, tc := range []struct {
		name string
		ctx  context.Context
		req  Request
	}{
		{"nil context", nil, Request{Initial: committed(1, 3, "plan-a"), Bound: 1, Step: validStep}},
		{"zero bound", context.Background(), Request{Initial: committed(1, 3, "plan-a"), Step: validStep}},
		{"nil step", context.Background(), Request{Initial: committed(1, 3, "plan-a"), Bound: 1}},
		{"negative prefix", context.Background(), Request{Initial: committed(-1, 3, "plan-a"), Bound: 1, Step: validStep}},
		{"overflow initial", context.Background(), Request{Initial: committed(4, 3, "plan-a"), Bound: 1, Step: validStep}},
		{"noncommitted initial", context.Background(), Request{Initial: State{Status: StatusConflict, CommittedPrefix: 1, PartitionCount: 3}, Bound: 1, Step: validStep}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Repeat(tc.ctx, tc.req)
			if got.Stop != StopFailed || !errors.Is(got.Err, ErrInvalid) || got.Steps != 0 || got.Transitions != 0 {
				t.Fatalf("ASSERT_INVALID_REQUEST result=%+v", got)
			}
		})
	}
}

func TestRepeatRejectsInvalidStepState(t *testing.T) {
	for _, tc := range []struct {
		name string
		next State
	}{
		{"prefix regression", committed(0, 3, "plan-a")},
		{"prefix overflow", committed(4, 3, "plan-a")},
		{"count change", committed(2, 4, "plan-a")},
		{"identity change", committed(2, 3, "plan-b")},
		{"identity added", committed(2, 3, "plan-a")},
		{"committed without progress", committed(1, 3, "plan-a")},
		{"conflict moved prefix", State{Status: StatusConflict, CommittedPrefix: 2, PartitionCount: 3, PlanIdentity: "plan-a"}},
		{"unknown status", State{Status: Status("OTHER"), CommittedPrefix: 1, PartitionCount: 3, PlanIdentity: "plan-a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := committed(1, 3, "plan-a")
			if tc.name == "identity added" {
				initial.PlanIdentity = ""
			}
			got := Repeat(context.Background(), Request{
				Initial: initial,
				Bound:   2,
				Step: func(context.Context, State) (State, error) {
					return tc.next, nil
				},
			})
			if got.Stop != StopFailed || !errors.Is(got.Err, ErrInvalid) || got.Steps != 1 || got.Transitions != 0 {
				t.Fatalf("ASSERT_INVALID_STEP result=%+v", got)
			}
		})
	}
}

func TestRepeatStepCallsDoNotOverlap(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	got := Repeat(context.Background(), Request{
		Initial: committed(0, 3, "plan-a"),
		Bound:   3,
		Step: func(_ context.Context, current State) (State, error) {
			n := active.Add(1)
			defer active.Add(-1)
			for {
				old := maximum.Load()
				if n <= old || maximum.CompareAndSwap(old, n) {
					break
				}
			}
			return committed(current.CommittedPrefix+1, 3, "plan-a"), nil
		},
	})
	if got.Stop != StopComplete || maximum.Load() != 1 || got.Steps != 3 || got.Transitions != 3 {
		t.Fatalf("ASSERT_NON_OVERLAP result=%+v maximum=%d", got, maximum.Load())
	}
}

func TestRepeatCancellationBetweenSteps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	got := Repeat(ctx, Request{
		Initial: committed(1, 3, "plan-a"),
		Bound:   3,
		Step: func(context.Context, State) (State, error) {
			calls++
			cancel()
			return committed(2, 3, "plan-a"), nil
		},
	})
	if got.Stop != StopDeadline || !errors.Is(got.Err, context.Canceled) || calls != 1 || got.Steps != 1 || got.Transitions != 1 || got.State.CommittedPrefix != 2 {
		t.Fatalf("ASSERT_CANCEL_BETWEEN result=%+v calls=%d", got, calls)
	}
}

func TestSharedAtomicStepDeclared(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if filepath.Ext(path) != ".go" || filepath.Base(path) == "driver_test.go" {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == "Step" {
				return
			}
		}
	}
	t.Fatal("ASSERT_SHARED_ATOMIC_STEP_DECLARED: package-level func Step is absent")
}

func TestDriverProductionImportsRemainGeneric(t *testing.T) {
	path := filepath.Join("driver.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{`"context"`: true, `"errors"`: true}
	for _, imp := range file.Imports {
		if !allowed[imp.Path.Value] {
			t.Errorf("ASSERT_GENERIC_IMPORT unexpected import %s", imp.Path.Value)
		}
	}
	if len(file.Imports) != len(allowed) {
		got := make([]string, 0, len(file.Imports))
		for _, imp := range file.Imports {
			got = append(got, imp.Path.Value)
		}
		t.Fatalf("ASSERT_GENERIC_IMPORT imports=%v", got)
	}
}
