package censussourceassembly

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourcedriver"
	"lsp-trace/internal/censussourceplan"
)

type observingObjects struct {
	inner censussourcecontinuation.ImmutableStore
	puts  atomic.Int32
}

func (s *observingObjects) Put(ctx context.Context, raw []byte) (string, error) {
	s.puts.Add(1)
	return s.inner.Put(ctx, raw)
}

func (s *observingObjects) Get(ctx context.Context, selector string) ([]byte, error) {
	return s.inner.Get(ctx, selector)
}

type observingHeads struct {
	inner censussourcecontinuation.HeadStore
	mu    sync.Mutex
	cas   int
	at    int
	force censussourcecontinuation.CASResult
}

func (s *observingHeads) Load(ctx context.Context, key string) ([]byte, error) {
	return s.inner.Load(ctx, key)
}

func (s *observingHeads) CompareAndSwap(ctx context.Context, key string, expected censussourcecontinuation.HeadRevision, raw []byte) censussourcecontinuation.CASResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cas++
	if s.cas == s.at {
		if s.force.Outcome == censussourcecontinuation.CASCommitUncertain {
			committed := s.inner.CompareAndSwap(ctx, key, expected, raw)
			if committed.Outcome != censussourcecontinuation.CASCommitted || committed.Err != nil {
				return committed
			}
		}
		return s.force
	}
	return s.inner.CompareAndSwap(ctx, key, expected, raw)
}

func runStores(t *testing.T) (*observingObjects, *observingHeads) {
	t.Helper()
	objectStore, err := NewObjects(privateRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	headStore, err := NewHeads(privateRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return &observingObjects{inner: objectStore}, &observingHeads{inner: headStore}
}

func runRequest(t *testing.T, count, bound int, objects censussourcecontinuation.ImmutableStore, heads censussourcecontinuation.HeadStore, calls *[]int) Request {
	t.Helper()
	plan := terminalPlan(t, 1, count)
	return Request{
		Plan: plan, PartitionSize: 1, WorkflowKey: "stable-workflow", HeadKey: "stable-head",
		Objects: objects, Heads: heads, StepBound: bound,
		Discover: func(_ context.Context, ordinal int, members []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
			*calls = append(*calls, ordinal)
			if !reflect.DeepEqual(members, plan.Partitions[ordinal].Members) {
				t.Fatalf("ASSERT_ASSEMBLY_RUN_EXACT_MEMBERS ordinal=%d got=%+v want=%+v", ordinal, members, plan.Partitions[ordinal].Members)
			}
			return terminalDiscovery(0), nil
		},
	}
}

func TestRunCompletesThreePartitionsWithExactSequentialDiscovery(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	var active, maximum atomic.Int32
	req := runRequest(t, 3, 3, objects, heads, &calls)
	discover := req.Discover
	req.Discover = func(ctx context.Context, ordinal int, members []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); n > old && !maximum.CompareAndSwap(old, n); old = maximum.Load() {
		}
		return discover(ctx, ordinal, members)
	}
	result, terminal := Run(context.Background(), req)
	if result.Stop != censussourcedriver.StopComplete || result.State.CommittedPrefix != 3 || result.Steps != 3 || result.Transitions != 3 || result.Err != nil || !reflect.DeepEqual(calls, []int{0, 1, 2}) || maximum.Load() != 1 {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_THREE_COMPLETE result=%+v calls=%v maximum=%d", result, calls, maximum.Load())
	}
	if terminal == nil || terminal.Ordinal != 2 || !reflect.DeepEqual(terminal.Members, req.Plan.Partitions[2].Members) {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_LAST_TERMINAL terminal=%+v", terminal)
	}
}

func TestRunBoundOneAndDeadlineZero(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	result, terminal := Run(context.Background(), runRequest(t, 3, 1, objects, heads, &calls))
	if result.Stop != censussourcedriver.StopBoundReached || result.Steps != 1 || result.Transitions != 1 || !reflect.DeepEqual(calls, []int{0}) || terminal == nil || terminal.Ordinal != 0 {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_BOUND_ONE result=%+v calls=%v terminal=%+v", result, calls, terminal)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	objects, heads = runStores(t)
	calls = nil
	result, terminal = Run(ctx, runRequest(t, 3, 3, objects, heads, &calls))
	if result.Stop != censussourcedriver.StopDeadline || !errors.Is(result.Err, context.Canceled) || result.Steps != 0 || len(calls) != 0 || terminal != nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_DEADLINE_ZERO result=%+v calls=%v terminal=%+v", result, calls, terminal)
	}
}

func TestRunPreservesConflictRetainedAndIndeterminateStops(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*Request, *observingHeads)
		stop      censussourcedriver.Stop
	}{
		{"conflict", func(_ *Request, heads *observingHeads) {
			heads.at = 1
			heads.force = censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASNotCommitted}
		}, censussourcedriver.StopConflict},
		{"retained", func(req *Request, _ *observingHeads) {
			req.Discover = func(context.Context, int, []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
				return censusacquisition.Discovery{Observations: []censusacquisition.DiscoveryObservation{{}}}, nil
			}
		}, censussourcedriver.StopRetained},
		{"indeterminate", func(_ *Request, heads *observingHeads) {
			heads.at = 1
			heads.force = censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitUncertain, Err: errors.New("uncertain")}
		}, censussourcedriver.StopIndeterminateExecution},
	} {
		t.Run(tc.name, func(t *testing.T) {
			objects, heads := runStores(t)
			calls := []int{}
			req := runRequest(t, 3, 3, objects, heads, &calls)
			tc.configure(&req, heads)
			result, terminal := Run(context.Background(), req)
			if result.Stop != tc.stop || result.Steps != 1 || result.Transitions != 0 || result.Err != nil || terminal != nil {
				t.Fatalf("ASSERT_ASSEMBLY_RUN_EXACT_STOP result=%+v terminal=%+v", result, terminal)
			}
		})
	}
}

func TestRunCallbackErrorAndAdaptationFailureEmitNoTerminal(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	req := runRequest(t, 2, 2, objects, heads, &calls)
	req.Discover = func(context.Context, int, []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
		return censusacquisition.Discovery{}, errors.New("callback failed")
	}
	result, terminal := Run(context.Background(), req)
	if result.Stop != censussourcedriver.StopFailed || result.Steps != 1 || terminal != nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_CALLBACK_NO_TERMINAL result=%+v terminal=%+v", result, terminal)
	}

	objects, heads = runStores(t)
	req = runRequest(t, 2, 2, objects, heads, &calls)
	req.Discover = func(context.Context, int, []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
		return censusacquisition.Discovery{Closure: censusacquisition.DiscoveryClosure(99)}, nil
	}
	result, terminal = Run(context.Background(), req)
	if result.Stop != censussourcedriver.StopFailed || result.Steps != 1 || terminal != nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_ADAPT_NO_TERMINAL result=%+v terminal=%+v", result, terminal)
	}
}

func TestRunConstructsStoresFromSeparateRootsAndRejectsSharedRoot(t *testing.T) {
	calls := []int{}
	separate := runRequest(t, 1, 1, nil, nil, &calls)
	separate.ObjectRoot = privateRoot(t)
	separate.HeadRoot = privateRoot(t)
	result, terminal := Run(context.Background(), separate)
	if result.Stop != censussourcedriver.StopComplete || terminal == nil || !reflect.DeepEqual(calls, []int{0}) {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_SEPARATE_ROOTS result=%+v terminal=%+v calls=%v", result, terminal, calls)
	}

	calls = nil
	sharedRoot := privateRoot(t)
	shared := runRequest(t, 1, 1, nil, nil, &calls)
	shared.ObjectRoot = sharedRoot
	shared.HeadRoot = sharedRoot
	result, terminal = Run(context.Background(), shared)
	if result.Stop != censussourcedriver.StopFailed || result.Steps != 0 || terminal != nil || len(calls) != 0 {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_SHARED_ROOT_REJECTED result=%+v terminal=%+v calls=%v", result, terminal, calls)
	}
}

func TestRunRejectsMalformedInputsBeforeDiscovery(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	valid := runRequest(t, 1, 1, objects, heads, &calls)
	cases := []Request{valid, valid, valid, valid, valid}
	cases[0].PartitionSize = 0
	cases[1].StepBound = 0
	cases[2].WorkflowKey = ""
	cases[3].Objects = nil
	cases[4].Heads = nil
	for i, req := range cases {
		result, terminal := Run(context.Background(), req)
		if result.Stop != censussourcedriver.StopFailed || result.Steps != 0 || terminal != nil {
			t.Fatalf("ASSERT_ASSEMBLY_RUN_INVALID_%d result=%+v terminal=%+v", i, result, terminal)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_INVALID_NO_DISCOVERY calls=%v", calls)
	}

	objects, heads = runStores(t)
	badHeads := &malformedRunHeads{HeadStore: heads}
	result, terminal := Run(context.Background(), runRequest(t, 1, 1, objects, badHeads, &calls))
	if result.Stop != censussourcedriver.StopFailed || terminal != nil || len(calls) != 0 {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_MALFORMED_HEAD result=%+v terminal=%+v calls=%v", result, terminal, calls)
	}
}

type malformedRunHeads struct {
	censussourcecontinuation.HeadStore
}

func (s *malformedRunHeads) Load(context.Context, string) ([]byte, error) {
	return []byte("not-json"), nil
}

func TestRunStableIdentityAndReceiptLinkedResumeDoesNotRepeatWork(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	req := runRequest(t, 2, 2, objects, heads, &calls)
	heads.at = 1
	heads.force = censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitUncertain, Err: errors.New("uncertain")}
	first, terminal := Run(context.Background(), req)
	if first.Stop != censussourcedriver.StopIndeterminateExecution || terminal != nil || !reflect.DeepEqual(calls, []int{0}) {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_UNCERTAIN_FIRST result=%+v terminal=%+v calls=%v", first, terminal, calls)
	}
	puts := objects.puts.Load()
	heads.at = 0
	calls = nil
	req.StepBound = 1
	second, terminal := Run(context.Background(), req)
	if second.Stop != censussourcedriver.StopBoundReached || second.State.CommittedPrefix != 1 || len(calls) != 0 || objects.puts.Load() != puts || terminal != nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_UNCERTAIN_RESUME result=%+v terminal=%+v calls=%v puts=%d/%d", second, terminal, calls, puts, objects.puts.Load())
	}

	raw, err := heads.Load(context.Background(), req.HeadKey)
	if err != nil || len(raw) == 0 || bytes.Contains(raw, []byte("foreign-workflow")) {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_STABLE_HEAD_IDENTITY raw=%q err=%v", raw, err)
	}
}

func TestRunPersistedCompletePerformsZeroWork(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	req := runRequest(t, 2, 2, objects, heads, &calls)
	first, terminal := Run(context.Background(), req)
	if first.Stop != censussourcedriver.StopComplete || terminal == nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_PERSISTED_COMPLETE_SETUP result=%+v terminal=%+v", first, terminal)
	}

	calls = nil
	puts := objects.puts.Load()
	heads.mu.Lock()
	cas := heads.cas
	heads.mu.Unlock()
	second, terminal := Run(context.Background(), req)
	heads.mu.Lock()
	casAfter := heads.cas
	heads.mu.Unlock()
	if second.Stop != censussourcedriver.StopComplete || second.State.CommittedPrefix != 2 || second.Steps != 0 || second.Transitions != 0 || len(calls) != 0 || objects.puts.Load() != puts || casAfter != cas || terminal != nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_PERSISTED_COMPLETE_ZERO_WORK result=%+v calls=%v puts=%d/%d cas=%d/%d terminal=%+v", second, calls, puts, objects.puts.Load(), cas, casAfter, terminal)
	}
}

func TestRunPersistedPrefixCancellationAndNewTransitionCounts(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	req := runRequest(t, 3, 1, objects, heads, &calls)
	first, terminal := Run(context.Background(), req)
	if first.Stop != censussourcedriver.StopBoundReached || first.State.CommittedPrefix != 1 || first.Transitions != 1 || terminal == nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_PREFIX_SETUP result=%+v terminal=%+v", first, terminal)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = nil
	cancelled, terminal := Run(ctx, req)
	if cancelled.Stop != censussourcedriver.StopDeadline || !errors.Is(cancelled.Err, context.Canceled) || cancelled.State.CommittedPrefix != 1 || cancelled.Steps != 0 || cancelled.Transitions != 0 || len(calls) != 0 || terminal != nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_CANCEL_PERSISTED_PREFIX result=%+v calls=%v terminal=%+v", cancelled, calls, terminal)
	}

	calls = nil
	req.StepBound = 3
	resumed, terminal := Run(context.Background(), req)
	if resumed.Stop != censussourcedriver.StopComplete || resumed.State.CommittedPrefix != 3 || resumed.Steps != 2 || resumed.Transitions != 2 || !reflect.DeepEqual(calls, []int{1, 2}) || terminal == nil || terminal.Ordinal != 2 {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_COUNTS_ONLY_NEW_TRANSITIONS result=%+v calls=%v terminal=%+v", resumed, calls, terminal)
	}
}

func TestRunClaimedCustodySafety(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	req := runRequest(t, 3, 1, objects, heads, &calls)
	first, _ := Run(context.Background(), req)
	if first.State.CommittedPrefix != 1 {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_CLAIMED_SETUP result=%+v", first)
	}

	heads.mu.Lock()
	heads.at = heads.cas + 1
	heads.force = censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitUncertain, Err: errors.New("uncertain claim")}
	heads.mu.Unlock()
	calls = nil
	claimed, terminal := Run(context.Background(), req)
	if claimed.Stop != censussourcedriver.StopIndeterminateExecution || claimed.State.CommittedPrefix != 1 || claimed.Steps != 1 || claimed.Transitions != 0 || len(calls) != 0 || terminal != nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_UNRECEIPTED_CLAIM_CREATED result=%+v calls=%v terminal=%+v", claimed, calls, terminal)
	}

	heads.mu.Lock()
	cas := heads.cas
	heads.at = 0
	heads.mu.Unlock()
	calls = nil
	persisted, terminal := Run(context.Background(), req)
	heads.mu.Lock()
	casAfter := heads.cas
	heads.mu.Unlock()
	if persisted.Stop != censussourcedriver.StopIndeterminateExecution || persisted.State.CommittedPrefix != 1 || persisted.Steps != 0 || persisted.Transitions != 0 || len(calls) != 0 || casAfter != cas || terminal != nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_UNRECEIPTED_CLAIM_PERSISTS result=%+v calls=%v cas=%d/%d terminal=%+v", persisted, calls, cas, casAfter, terminal)
	}
}

func TestRunOptionalManagerIdentityPairingAndStability(t *testing.T) {
	objects, heads := runStores(t)
	calls := []int{}
	req := runRequest(t, 2, 2, objects, heads, &calls)
	req.Identity = Identity{SessionID: "opaque-session", ManagerGeneration: 41}
	result, terminal := Run(context.Background(), req)
	if result.Stop != censussourcedriver.StopComplete || result.State.SessionID != req.Identity.SessionID || result.State.ManagerGeneration != req.Identity.ManagerGeneration || terminal == nil {
		t.Fatalf("ASSERT_ASSEMBLY_RUN_IDENTITY_STABLE result=%+v terminal=%+v", result, terminal)
	}

	for _, identity := range []Identity{{SessionID: "unpaired"}, {ManagerGeneration: 1}} {
		objects, heads = runStores(t)
		calls = nil
		invalid := runRequest(t, 1, 1, objects, heads, &calls)
		invalid.Identity = identity
		result, terminal = Run(context.Background(), invalid)
		if result.Stop != censussourcedriver.StopFailed || result.Steps != 0 || len(calls) != 0 || terminal != nil {
			t.Fatalf("ASSERT_ASSEMBLY_RUN_IDENTITY_PAIR_REJECT identity=%+v result=%+v calls=%v terminal=%+v", identity, result, calls, terminal)
		}
	}
}

func TestRunAPIHasNoAggregateOrForbiddenDependencies(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "run.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"cmd/", "manager", "lifecycle", "diagnostic", "internal/censussourceplan/build", "internal/operation", "internal/schema", "output", "graph", "aggregate", "programc", "leiden", "calls"} {
			if bytes.Contains(bytes.ToLower([]byte(path)), []byte(forbidden)) {
				t.Fatalf("ASSERT_ASSEMBLY_RUN_FORBIDDEN_IMPORT path=%q value=%q", path, forbidden)
			}
		}
	}
}

var _ = censussourceplan.Plan{}
