package main

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/publication"
)

type countingContinuationObjects struct {
	*censusSourceContinuationObjects
	puts int
}

func (s *countingContinuationObjects) Put(ctx context.Context, raw []byte) (string, error) {
	s.puts++
	return s.censusSourceContinuationObjects.Put(ctx, raw)
}

type failingContinuationObjects struct {
	*censusSourceContinuationObjects
}

func (s *failingContinuationObjects) Put(context.Context, []byte) (string, error) {
	return "", errors.New("injected object write failure")
}

type staleContinuationHeads struct {
	*censusSourceContinuationHeads
	cas int
}

func (s *staleContinuationHeads) CompareAndSwap(context.Context, string, censussourcecontinuation.HeadRevision, []byte) censussourcecontinuation.CASResult {
	s.cas++
	return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASNotCommitted}
}

type uncertainContinuationHeads struct {
	*censusSourceContinuationHeads
	cas, uncertainAt int
}

func (s *uncertainContinuationHeads) CompareAndSwap(ctx context.Context, key string, expected censussourcecontinuation.HeadRevision, replacement []byte) censussourcecontinuation.CASResult {
	s.cas++
	result := s.censusSourceContinuationHeads.CompareAndSwap(ctx, key, expected, replacement)
	if s.cas == s.uncertainAt && result.Outcome == censussourcecontinuation.CASCommitted {
		return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASCommitUncertain}
	}
	return result
}

func continuationPlan(t *testing.T, partitionSize int) censussourceplan.Plan {
	t.Helper()
	workspace := t.TempDir()
	files := make([]censusacquisition.SourceFile, 3)
	for i, name := range []string{"a.go", "b.go", "c.go"} {
		files[i] = censusacquisition.SourceFile{Path: name, URI: "file://" + filepath.ToSlash(filepath.Join(workspace, name)), LanguageID: "go"}
	}
	plan, err := censussourceplan.Build(files, workspace, partitionSize)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func continuationTerminal(plan censussourceplan.Plan, ordinal int, outcome censussourcecontinuation.Outcome) censussourcecontinuation.Terminal {
	discovery := censusacquisition.Discovery{}
	switch outcome {
	case censussourcecontinuation.OutcomeCompleteNonempty:
		discovery.Complete = true
		discovery.Closure = censusacquisition.DiscoveryClosureSuccessful
		discovery.Targets = []censusacquisition.PreparedTarget{{}}
	case censussourcecontinuation.OutcomeSuccessfulEmpty:
		discovery.Closure = censusacquisition.DiscoveryClosureSuccessful
	case censussourcecontinuation.OutcomePartial:
		discovery.Observations = []censusacquisition.DiscoveryObservation{{}}
	}
	return censussourcecontinuation.Terminal{PlanIdentity: plan.Identity, Ordinal: ordinal, SourceFiles: append([]censusacquisition.SourceFile(nil), plan.Partitions[ordinal].Members...), Discovery: discovery, Outcome: outcome}
}

func TestCensusSourceContinuationCoordinatorCommitsCanonicalHead(t *testing.T) {
	root := openCensusRuntimeRoot(t)
	objects, err := newCensusSourceContinuationObjects(root)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := newCensusSourceContinuationHeads(root)
	if err != nil {
		t.Fatal(err)
	}
	plan := continuationPlan(t, 1)
	terminal := continuationTerminal(plan, 0, censussourcecontinuation.OutcomeSuccessfulEmpty)
	request := censussourcecontinuation.Request{Plan: plan, PartitionSize: 1, WorkflowKey: "cmd-store-integration", HeadKey: "cmd-store-integration", ExistingOrdinalZero: &terminal, Objects: objects, Heads: heads}
	first, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || first.Status != censussourcecontinuation.StatusCommitted || first.CommittedPrefix != 1 || first.Ordinal != 0 || first.Outcome != censussourcecontinuation.OutcomeSuccessfulEmpty || first.Head.Generation == 0 || first.Head.Selector == "" || first.ReceiptSelector == "" {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_SUCCESS_ADVANCES: result=%+v err=%v", first, err)
	}
	if terminal.Authority != 0 || plan.Authority != 0 {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_AUTHORITY_ZERO: plan=%d terminal=%d", plan.Authority, terminal.Authority)
	}
	request.ExistingOrdinalZero = nil
	request.Expected = first.Head
	discovered := []int{}
	request.Discover = func(_ context.Context, ordinal int, members []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error) {
		discovered = append(discovered, ordinal)
		return continuationTerminal(plan, ordinal, censussourcecontinuation.OutcomeSuccessfulEmpty), nil
	}
	second, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || second.Status != censussourcecontinuation.StatusCommitted || second.CommittedPrefix != 2 || !reflect.DeepEqual(discovered, []int{1}) {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_ORDINAL_ZERO_NOT_REDISCOVERED: first=%+v second=%+v discovered=%v err=%v", first, second, discovered, err)
	}
	if second.Ordinal != 1 || len(discovered) != 1 {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_ONE_SELECTED_PARTITION: result=%+v discovered=%v", second, discovered)
	}
}

func TestCensusSourceContinuationClaimWithoutReceiptIsIndeterminate(t *testing.T) {
	root := openCensusRuntimeRoot(t)
	objects, _ := newCensusSourceContinuationObjects(root)
	heads, _ := newCensusSourceContinuationHeads(root)
	plan := continuationPlan(t, 1)
	request := censussourcecontinuation.Request{Plan: plan, PartitionSize: 1, WorkflowKey: "claim-without-receipt", HeadKey: "claim-without-receipt", ExistingOrdinalZero: ptrContinuationTerminal(continuationTerminal(plan, 0, censussourcecontinuation.OutcomeSuccessfulEmpty)), Objects: objects, Heads: heads}
	first, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || first.Status != censussourcecontinuation.StatusCommitted {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_CLAIM_PRECONDITION: result=%+v err=%v", first, err)
	}
	request.Expected, request.ExistingOrdinalZero = first.Head, nil
	claimed, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || claimed.Status != censussourcecontinuation.StatusIndeterminateExecution || claimed.CommittedPrefix != 1 || claimed.Ordinal != 1 || claimed.ReceiptSelector != "" || claimed.Head.Generation <= first.Head.Generation || claimed.Head.Selector == "" {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_CLAIM_WITHOUT_RECEIPT_INDETERMINATE: first=%+v claimed=%+v err=%v", first, claimed, err)
	}
}

func ptrContinuationTerminal(terminal censussourcecontinuation.Terminal) *censussourcecontinuation.Terminal {
	return &terminal
}

func TestCensusSourceContinuationNonSuccessRetains(t *testing.T) {
	plan := continuationPlan(t, censusRuntimeSourcePartitionSize)
	mappings := []struct {
		private   privateConstituentDiscoveryOutcome
		public    censussourcecontinuation.Outcome
		discovery censusacquisition.Discovery
	}{
		{privateDiscoveryPartial, censussourcecontinuation.OutcomePartial, validMixedTerminalDiscovery(t, plan)},
		{privateDiscoveryFailed, censussourcecontinuation.OutcomeFailed, validLifecycleTerminalDiscovery(t, plan, censusacquisition.ErrSessionPoisoned)},
		{privateDiscoveryRefused, censussourcecontinuation.OutcomeRefused, validLifecycleTerminalDiscovery(t, plan, censusacquisition.ErrLifecycleConflict)},
		{privateDiscoveryUnknown, censussourcecontinuation.OutcomeUnknown, censusacquisition.Discovery{}},
	}
	for _, mapping := range mappings {
		terminal := &privateConstituentDiscoveryTerminal{planIdentity: plan.Identity, ordinal: 0, members: append([]censusacquisition.SourceFile(nil), plan.Partitions[0].Members...), discovery: mapping.discovery, outcome: mapping.private}
		adapted, err := privateDiscoveryContinuationTerminal(plan, terminal)
		if err != nil || adapted.Outcome != mapping.public {
			t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_NON_SUCCESS_REFUSED: private=%s public=%s got=%+v err=%v", mapping.private, mapping.public, adapted, err)
		}
	}
	root := openCensusRuntimeRoot(t)
	objects, _ := newCensusSourceContinuationObjects(root)
	heads, _ := newCensusSourceContinuationHeads(root)
	terminal := continuationTerminal(plan, 0, censussourcecontinuation.OutcomeUnknown)
	result, err := censussourcecontinuation.CoordinateNext(context.Background(), censussourcecontinuation.Request{Plan: plan, PartitionSize: censusRuntimeSourcePartitionSize, WorkflowKey: "unknown", HeadKey: "unknown", ExistingOrdinalZero: &terminal, Objects: objects, Heads: heads})
	if err != nil || result.Status != censussourcecontinuation.StatusRetained || result.CommittedPrefix != 0 {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_NON_SUCCESS_REFUSED: outcome=UNKNOWN result=%+v err=%v", result, err)
	}
}

func TestCensusSourceContinuationReceiptResumeDoesNotRepublish(t *testing.T) {
	root := openCensusRuntimeRoot(t)
	baseObjects, _ := newCensusSourceContinuationObjects(root)
	baseHeads, _ := newCensusSourceContinuationHeads(root)
	objects := &countingContinuationObjects{censusSourceContinuationObjects: baseObjects}
	heads := &uncertainContinuationHeads{censusSourceContinuationHeads: baseHeads, uncertainAt: 4}
	plan := continuationPlan(t, 1)
	ordinalZero := continuationTerminal(plan, 0, censussourcecontinuation.OutcomeSuccessfulEmpty)
	calls := []int{}
	request := censussourcecontinuation.Request{Plan: plan, PartitionSize: 1, WorkflowKey: "uncertain", HeadKey: "uncertain", ExistingOrdinalZero: &ordinalZero, Objects: objects, Heads: heads, Discover: func(_ context.Context, ordinal int, _ []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error) {
		calls = append(calls, ordinal)
		return continuationTerminal(plan, ordinal, censussourcecontinuation.OutcomeSuccessfulEmpty), nil
	}}
	first, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || first.Status != censussourcecontinuation.StatusCommitted {
		t.Fatal(first, err)
	}
	request.ExistingOrdinalZero = nil
	request.Expected = first.Head
	interrupted, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || interrupted.Status != censussourcecontinuation.StatusIndeterminateExecution || !reflect.DeepEqual(calls, []int{1}) {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_EXACT_RECEIPT_RESUME: interrupted=%+v calls=%v err=%v", interrupted, calls, err)
	}
	puts, cas := objects.puts, heads.cas
	raw, err := heads.Load(context.Background(), request.HeadKey)
	if err != nil {
		t.Fatal(err)
	}
	request.Expected, err = censussourcecontinuation.ParseHeadRevision(raw)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || resumed.Status != censussourcecontinuation.StatusCommitted || resumed.CommittedPrefix != 2 || resumed.Ordinal != 1 || resumed.Outcome != censussourcecontinuation.OutcomeSuccessfulEmpty || !reflect.DeepEqual(calls, []int{1}) || objects.puts != puts || heads.cas != cas+1 {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_NO_REPLAY_OR_REPUBLICATION: resumed=%+v calls=%v puts=%d/%d cas=%d/%d err=%v", resumed, calls, objects.puts, puts, heads.cas, cas+1, err)
	}
}

func TestCensusSourceContinuationAdapterFaultsFailClosed(t *testing.T) {
	t.Run("malformed-stored-head", func(t *testing.T) {
		root := openCensusRuntimeRoot(t)
		heads, _ := newCensusSourceContinuationHeads(root)
		selector, err := censusSourceContinuationHeadSelector("malformed")
		if err != nil {
			t.Fatal(err)
		}
		published := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: selector, Bytes: []byte("{}"), ArtifactSchemaID: "test.invalid"})
		if published.Failure != nil {
			t.Fatal(published.Failure)
		}
		if raw, err := heads.Load(context.Background(), "malformed"); err == nil || raw != nil {
			t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_MALFORMED_HEAD_FAILS_CLOSED: raw=%q err=%v", raw, err)
		}
	})

	for _, tc := range []struct {
		name string
		wrap func(*censusSourceContinuationObjects, *censusSourceContinuationHeads) (censussourcecontinuation.ImmutableStore, censussourcecontinuation.HeadStore)
	}{
		{name: "object-write", wrap: func(objects *censusSourceContinuationObjects, heads *censusSourceContinuationHeads) (censussourcecontinuation.ImmutableStore, censussourcecontinuation.HeadStore) {
			return &failingContinuationObjects{objects}, heads
		}},
		{name: "stale-cas", wrap: func(objects *censusSourceContinuationObjects, heads *censusSourceContinuationHeads) (censussourcecontinuation.ImmutableStore, censussourcecontinuation.HeadStore) {
			return objects, &staleContinuationHeads{censusSourceContinuationHeads: heads}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := openCensusRuntimeRoot(t)
			baseObjects, _ := newCensusSourceContinuationObjects(root)
			baseHeads, _ := newCensusSourceContinuationHeads(root)
			objects, heads := tc.wrap(baseObjects, baseHeads)
			plan := continuationPlan(t, 1)
			discoveryCalls := 0
			terminal := continuationTerminal(plan, 0, censussourcecontinuation.OutcomeSuccessfulEmpty)
			result, err := censussourcecontinuation.CoordinateNext(context.Background(), censussourcecontinuation.Request{Plan: plan, PartitionSize: 1, WorkflowKey: tc.name, HeadKey: tc.name, ExistingOrdinalZero: &terminal, Objects: objects, Heads: heads, Discover: func(context.Context, int, []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error) {
				discoveryCalls++
				return censussourcecontinuation.Terminal{}, errors.New("unauthorized discovery")
			}})
			if tc.name == "object-write" {
				if err == nil || result != (censussourcecontinuation.Result{}) || discoveryCalls != 0 {
					t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_OBJECT_WRITE_FAILS_CLOSED: result=%+v discovery=%d err=%v", result, discoveryCalls, err)
				}
				return
			}
			if err != nil || result.Status != censussourcecontinuation.StatusConflict || result.CommittedPrefix != 0 || discoveryCalls != 0 {
				t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_STALE_CAS_FAILS_CLOSED: result=%+v discovery=%d err=%v", result, discoveryCalls, err)
			}
		})
	}
}

func TestCensusSourceContinuationStoresAcceptFreshPrivateRoot(t *testing.T) {
	root := openCensusRuntimeRoot(t)
	if err := root.ValidatePrivate(); err != nil {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_FRESH_ROOT_PRIVATE: %v", err)
	}
	objects, err := newCensusSourceContinuationObjects(root)
	if err != nil {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_OBJECT_STORE_FRESH_ROOT: %v", err)
	}
	wantObject := []byte(`{"object":"fresh"}`)
	selector, err := objects.Put(context.Background(), wantObject)
	if err != nil {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_OBJECT_STORE_PUT: %v", err)
	}
	gotObject, err := objects.Get(context.Background(), selector)
	if err != nil {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_OBJECT_STORE_GET: %v", err)
	}
	if string(gotObject) != string(wantObject) {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_OBJECT_STORE_ROUND_TRIP: got=%q want=%q", gotObject, wantObject)
	}
	heads, err := newCensusSourceContinuationHeads(root)
	if err != nil {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_HEAD_STORE_FRESH_ROOT: %v", err)
	}
	raw, err := heads.Load(context.Background(), "fresh-private-workflow")
	if err != nil {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_FRESH_HEAD_ABSENT: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("ASSERT_PRIVATE_SOURCE_CONTINUATION_FRESH_HEAD_EMPTY: %q", raw)
	}
}
