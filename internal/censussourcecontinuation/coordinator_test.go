package censussourcecontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"slices"
	"sync"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceplan"
)

type memObjects struct {
	mu         sync.Mutex
	m          map[string][]byte
	puts, gets int
}

func (s *memObjects) Put(_ context.Context, b []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts++
	h := sha256.Sum256(b)
	k := "sha256:" + hex.EncodeToString(h[:])
	if s.m == nil {
		s.m = map[string][]byte{}
	}
	if old, ok := s.m[k]; ok && !bytes.Equal(old, b) {
		return "", errors.New("collision")
	}
	s.m[k] = bytes.Clone(b)
	return k, nil
}
func (s *memObjects) Get(_ context.Context, k string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets++
	b, ok := s.m[k]
	if !ok {
		return nil, errors.New("missing")
	}
	return bytes.Clone(b), nil
}

type memHeads struct {
	mu          sync.Mutex
	raw         []byte
	uncertain   bool
	uncertainAt int
	loads, cas  int
}

func (h *memHeads) Load(context.Context, string) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.loads++
	if h.raw == nil {
		return nil, nil
	}
	return bytes.Clone(h.raw), nil
}
func (h *memHeads) CompareAndSwap(_ context.Context, _ string, w HeadRevision, b []byte) CASResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cas++
	cur, _ := ParseHeadRevision(h.raw)
	if cur != w {
		return CASResult{Outcome: CASNotCommitted}
	}
	h.raw = bytes.Clone(b)
	if h.uncertain || h.uncertainAt == h.cas {
		h.uncertain = false
		h.uncertainAt = 0
		return CASResult{Outcome: CASCommitUncertain}
	}
	return CASResult{Outcome: CASCommitted}
}
func plan3(t *testing.T) censussourceplan.Plan {
	t.Helper()
	fs := []censusacquisition.SourceFile{{Path: "a.go", URI: "file:///w/a.go"}, {Path: "b.go", URI: "file:///w/b.go"}, {Path: "c.go", URI: "file:///w/c.go"}}
	p, e := censussourceplan.Build(fs, "/w", 1)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func terminal(p censussourceplan.Plan, o int, out Outcome) Terminal {
	discovery := censusacquisition.Discovery{}
	switch out {
	case OutcomeCompleteNonempty:
		discovery.Complete = true
		discovery.Closure = censusacquisition.DiscoveryClosureSuccessful
		discovery.Targets = []censusacquisition.PreparedTarget{{}}
	case OutcomeSuccessfulEmpty:
		discovery.Closure = censusacquisition.DiscoveryClosureSuccessful
	case OutcomePartial:
		discovery.Observations = []censusacquisition.DiscoveryObservation{{}}
	}
	return Terminal{PlanIdentity: p.Identity, Authority: 0, Ordinal: o, SourceFiles: append([]censusacquisition.SourceFile(nil), p.Partitions[o].Members...), Discovery: discovery, Outcome: out}
}
func TestCoordinateNextThreePartitionCASResume(t *testing.T) {
	ctx := context.Background()
	p := plan3(t)
	objs := &memObjects{}
	heads := &memHeads{}
	calls := 0
	r := Request{Plan: p, PartitionSize: 1, WorkflowKey: "w", HeadKey: "h", Objects: objs, Heads: heads, ExistingOrdinalZero: ptrTerminal(terminal(p, 0, OutcomeSuccessfulEmpty)), Discover: func(_ context.Context, o int, f []censusacquisition.SourceFile) (Terminal, error) {
		calls++
		return terminal(p, o, OutcomeSuccessfulEmpty), nil
	}}
	a, e := CoordinateNext(ctx, r)
	if e != nil || a.Status != StatusCommitted || a.CommittedPrefix != 1 || calls != 0 {
		t.Fatalf("ASSERT_THREE_PARTITION_PROGRESS adopt=%+v calls=%d err=%v", a, calls, e)
	}
	r.Expected = a.Head
	b, e := CoordinateNext(ctx, r)
	if e != nil || b.Status != StatusCommitted || b.CommittedPrefix != 2 || b.Ordinal != 1 || calls != 1 {
		t.Fatalf("ASSERT_COORDINATE_SINGLE_STEP next=%+v calls=%d err=%v", b, calls, e)
	}
	r.Expected = b.Head
	c, e := CoordinateNext(ctx, r)
	if e != nil || c.Ordinal != 2 || calls != 2 {
		t.Fatalf("ASSERT_THREE_PARTITION_PROGRESS ordinal2=%+v calls=%d err=%v", c, calls, e)
	}
}
func TestPartitionOneReceiptLinkedInterruptionResumesWithoutReplay(t *testing.T) {
	ctx := context.Background()
	p := plan3(t)
	objects, heads := &memObjects{}, &memHeads{}
	calls := []int{}
	r := Request{Plan: p, PartitionSize: 1, WorkflowKey: "w", HeadKey: "h", Objects: objects, Heads: heads, ExistingOrdinalZero: ptrTerminal(terminal(p, 0, OutcomeSuccessfulEmpty)), Discover: func(_ context.Context, ordinal int, _ []censusacquisition.SourceFile) (Terminal, error) {
		calls = append(calls, ordinal)
		return terminal(p, ordinal, OutcomeSuccessfulEmpty), nil
	}}
	adopted, err := CoordinateNext(ctx, r)
	if err != nil || adopted.Status != StatusCommitted || adopted.CommittedPrefix != 1 || len(calls) != 0 {
		t.Fatalf("ASSERT_PARTITION_ZERO_ADOPTED_NOT_REPLAYED result=%+v calls=%v err=%v", adopted, calls, err)
	}
	r.ExistingOrdinalZero = nil
	r.Expected = adopted.Head
	heads.uncertainAt = heads.cas + 2
	interrupted, err := CoordinateNext(ctx, r)
	if interrupted.Status != StatusIndeterminateExecution || !reflect.DeepEqual(calls, []int{1}) {
		t.Fatalf("ASSERT_PARTITION_ONE_RECEIPT_LINK_INTERRUPTED result=%+v calls=%v err=%v", interrupted, calls, err)
	}
	r.Expected, _ = ParseHeadRevision(heads.raw)
	resumed, err := CoordinateNext(ctx, r)
	if err != nil || resumed.Status != StatusCommitted || resumed.CommittedPrefix != 2 || !reflect.DeepEqual(calls, []int{1}) {
		t.Fatalf("ASSERT_PARTITION_ONE_EXACT_RESUME_NO_REPLAY result=%+v calls=%v err=%v", resumed, calls, err)
	}
	if slices.Contains(calls, 0) || slices.Contains(calls, 2) {
		t.Fatalf("ASSERT_PARTITION_TWO_UNTOUCHED calls=%v", calls)
	}
}

func TestUncertainAndClaimNeverReplay(t *testing.T) {
	ctx := context.Background()
	p := plan3(t)
	objs := &memObjects{}
	heads := &memHeads{uncertain: true}
	calls := 0
	r := Request{Plan: p, PartitionSize: 1, WorkflowKey: "w", HeadKey: "h", Objects: objs, Heads: heads, ExistingOrdinalZero: ptrTerminal(terminal(p, 0, OutcomeSuccessfulEmpty)), Discover: func(context.Context, int, []censusacquisition.SourceFile) (Terminal, error) {
		calls++
		return Terminal{}, errors.New("should not")
	}}
	got, _ := CoordinateNext(ctx, r)
	if got.Status != StatusIndeterminateExecution {
		t.Fatalf("ASSERT_HEAD_CAS_SOLE_AUTHORITY got=%+v", got)
	}
	r.ExistingOrdinalZero = nil
	r.Expected, _ = ParseHeadRevision(heads.raw)
	got, _ = CoordinateNext(ctx, r)
	if got.Status != StatusCommitted || got.CommittedPrefix != 1 || calls != 0 {
		t.Fatalf("ASSERT_RECEIPT_LINKED_RESUME_NO_REPLAY got=%+v calls=%d", got, calls)
	}
}
func TestCallbackFailureNoRetry(t *testing.T) {
	ctx := context.Background()
	p := plan3(t)
	objs := &memObjects{}
	heads := &memHeads{}
	r := Request{Plan: p, PartitionSize: 1, WorkflowKey: "w", HeadKey: "h", Objects: objs, Heads: heads, ExistingOrdinalZero: ptrTerminal(terminal(p, 0, OutcomeSuccessfulEmpty))}
	a, _ := CoordinateNext(ctx, r)
	r.Expected = a.Head
	r.ExistingOrdinalZero = nil
	calls := 0
	r.Discover = func(context.Context, int, []censusacquisition.SourceFile) (Terminal, error) {
		calls++
		return Terminal{}, errors.New("poison")
	}
	x, _ := CoordinateNext(ctx, r)
	r.Expected, _ = ParseHeadRevision(heads.raw)
	y, _ := CoordinateNext(ctx, r)
	if x.Status != StatusIndeterminateExecution || y.Status != StatusIndeterminateExecution || calls != 1 {
		t.Fatalf("ASSERT_CALLBACK_FAILURE_NO_RETRY first=%+v second=%+v calls=%d", x, y, calls)
	}
}
func TestCanonicalReceiptNilAndForeignFailClosed(t *testing.T) {
	p := plan3(t)
	x := terminal(p, 1, OutcomeUnknown)
	x.Discovery.Targets = nil
	raw, e := EncodeReceipt("w", 7, "selector", "idempotency", x)
	if e != nil {
		t.Fatal(e)
	}
	got, e := DecodeReceipt(raw, "w", p, 1)
	if e != nil || got.Discovery.Targets != nil {
		t.Fatalf("ASSERT_RECEIPT_CANONICAL_NIL_PRESERVED nil=%t err=%v", got.Discovery.Targets == nil, e)
	}
	raw = append(raw, ' ')
	if _, e = DecodeReceipt(raw, "w", p, 1); e == nil {
		t.Fatal("ASSERT_EXACT_CUSTODY noncanonical accepted")
	}
}
func ptrTerminal(v Terminal) *Terminal { return &v }

func rawHead(t *testing.T, h head) []byte {
	t.Helper()
	b, err := setHeadSelector(&h)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func baseRequest(p censussourceplan.Plan, objects *memObjects, heads *memHeads, calls *int) Request {
	return Request{Plan: p, PartitionSize: 1, WorkflowKey: "w", HeadKey: "h", Objects: objects, Heads: heads, Discover: func(context.Context, int, []censusacquisition.SourceFile) (Terminal, error) {
		*calls++
		return terminal(p, 1, OutcomeSuccessfulEmpty), nil
	}}
}

func TestMalformedHeadsRejectBeforeEffects(t *testing.T) {
	p := plan3(t)
	cases := []head{
		{SchemaVersion: headVersion, WorkflowKey: "w", PlanIdentity: p.Identity, State: "FUTURE", ClaimOrdinal: -1},
		{SchemaVersion: headVersion, WorkflowKey: "w", PlanIdentity: p.Identity, State: "ABSENT", CommittedPrefix: -1, ClaimOrdinal: -1},
		{SchemaVersion: headVersion, WorkflowKey: "w", PlanIdentity: p.Identity, State: "ABSENT", CommittedPrefix: 4, ClaimOrdinal: -1},
		{SchemaVersion: headVersion, WorkflowKey: "w", PlanIdentity: p.Identity, State: "CLAIMED", CommittedPrefix: 1, ClaimOrdinal: 2, ClaimGeneration: 1, IdempotencyKey: "i"},
		{SchemaVersion: headVersion, WorkflowKey: "w", PlanIdentity: p.Identity, State: "COMMITTED", CommittedPrefix: 3, ClaimOrdinal: 1, ClaimGeneration: 1, IdempotencyKey: "i", ReceiptSelector: "r"},
	}
	for i, bad := range cases {
		objects, heads, calls := &memObjects{}, &memHeads{raw: rawHead(t, bad)}, 0
		r := baseRequest(p, objects, heads, &calls)
		r.Expected, _ = ParseHeadRevision(heads.raw)
		if _, err := CoordinateNext(context.Background(), r); !errors.Is(err, ErrInvalid) || calls != 0 || objects.puts != 0 || objects.gets != 0 || heads.cas != 0 {
			t.Fatalf("ASSERT_MALFORMED_HEAD_PRE_EFFECT_%d err=%v calls=%d puts=%d gets=%d cas=%d", i, err, calls, objects.puts, objects.gets, heads.cas)
		}
	}
}

func TestGenerationOverflowRejectsBeforeEffects(t *testing.T) {
	p := plan3(t)
	for _, state := range []string{"COMMITTED", "CLAIMED"} {
		h := head{SchemaVersion: headVersion, WorkflowKey: "w", PlanIdentity: p.Identity, Generation: math.MaxUint64, State: state, CommittedPrefix: 1, ClaimOrdinal: 0, ClaimGeneration: math.MaxUint64, IdempotencyKey: "i", ReceiptSelector: "sha256:" + string(make([]byte, 64))}
		objects, heads, calls := &memObjects{}, &memHeads{raw: rawHead(t, h)}, 0
		r := baseRequest(p, objects, heads, &calls)
		r.Expected, _ = ParseHeadRevision(heads.raw)
		if _, err := CoordinateNext(context.Background(), r); !errors.Is(err, ErrInvalid) || calls != 0 || objects.puts != 0 || objects.gets != 0 || heads.cas != 0 {
			t.Fatalf("ASSERT_GENERATION_OVERFLOW_PRE_EFFECT state=%s err=%v calls=%d puts=%d gets=%d cas=%d", state, err, calls, objects.puts, objects.gets, heads.cas)
		}
	}
}

func TestCASResultHasClosedNonBooleanOutcome(t *testing.T) {
	typ := reflect.TypeOf(CASResult{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Type.Kind() == reflect.Bool {
			t.Fatalf("ASSERT_CAS_OUTCOME_NOT_BOOLEAN field=%s", typ.Field(i).Name)
		}
	}
}

func TestTerminalContradictionsRejectBeforePut(t *testing.T) {
	p := plan3(t)
	for name, mutate := range map[string]func(*Terminal){
		"nonempty_without_complete": func(x *Terminal) { x.Outcome = OutcomeCompleteNonempty },
		"empty_with_complete": func(x *Terminal) {
			x.Outcome = OutcomeSuccessfulEmpty
			x.Discovery.Complete = true
			x.Discovery.Closure = censusacquisition.DiscoveryClosureSuccessful
		},
		"partial_success_closure": func(x *Terminal) {
			x.Outcome = OutcomePartial
			x.Discovery.Closure = censusacquisition.DiscoveryClosureSuccessful
		},
	} {
		t.Run(name, func(t *testing.T) {
			objects, heads := &memObjects{}, &memHeads{}
			x := terminal(p, 0, OutcomeSuccessfulEmpty)
			mutate(&x)
			r := Request{Plan: p, PartitionSize: 1, WorkflowKey: "w", HeadKey: "h", Objects: objects, Heads: heads, ExistingOrdinalZero: &x}
			if _, err := CoordinateNext(context.Background(), r); !errors.Is(err, ErrInvalid) || objects.puts != 0 || heads.cas != 0 {
				t.Fatalf("ASSERT_TERMINAL_CONTRADICTION_PRE_PUT err=%v puts=%d cas=%d", err, objects.puts, heads.cas)
			}
		})
	}
}

func TestHeadJSONMutationMatrix(t *testing.T) {
	p := plan3(t)
	valid := head{SchemaVersion: headVersion, WorkflowKey: "w", PlanIdentity: p.Identity, State: "ABSENT", ClaimOrdinal: -1}
	b := rawHead(t, valid)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"state", "committed_prefix", "claim_ordinal", "claim_generation", "claim_selector", "idempotency_key", "receipt_selector"} {
		t.Run(field, func(t *testing.T) {
			copyMap := map[string]any{}
			for k, v := range m {
				copyMap[k] = v
			}
			delete(copyMap, field)
			raw, _ := json.Marshal(copyMap)
			heads, objects, calls := &memHeads{raw: raw}, &memObjects{}, 0
			r := baseRequest(p, objects, heads, &calls)
			if _, err := CoordinateNext(context.Background(), r); !errors.Is(err, ErrInvalid) || calls != 0 || objects.puts != 0 || heads.cas != 0 {
				t.Fatalf("ASSERT_HEAD_FIELD_MATRIX field=%s err=%v", field, err)
			}
		})
	}
}
