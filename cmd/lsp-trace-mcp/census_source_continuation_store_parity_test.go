package main

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
)

type parityCountingObjects struct {
	censussourcecontinuation.ImmutableStore
	puts int
}

func (s *parityCountingObjects) Put(ctx context.Context, raw []byte) (string, error) {
	s.puts++
	return s.ImmutableStore.Put(ctx, raw)
}

type parityCountingHeads struct {
	censussourcecontinuation.HeadStore
	cas int
}

func (s *parityCountingHeads) CompareAndSwap(ctx context.Context, key string, expected censussourcecontinuation.HeadRevision, raw []byte) censussourcecontinuation.CASResult {
	s.cas++
	return s.HeadStore.CompareAndSwap(ctx, key, expected, raw)
}

type parityRun struct {
	objectSelector string
	objectBytes    []byte
	headSelector   string
	headBytes      []byte
	committed      censussourcecontinuation.CASOutcome
	stale          censussourcecontinuation.CASOutcome
	resume         censussourcecontinuation.Result
	discoveries    []int
	putsBefore     int
	putsAfter      int
	casBefore      int
	casAfter       int
}

func runContinuationStoreParity(t *testing.T, neutral bool, plan censussourceplan.Plan) parityRun {
	t.Helper()
	root := openCensusRuntimeRoot(t)
	var objects censussourcecontinuation.ImmutableStore
	var heads censussourcecontinuation.HeadStore
	var err error
	if neutral {
		objects, err = censussourceassembly.NewObjects(root)
		if err == nil {
			heads, err = censussourceassembly.NewHeads(root)
		}
	} else {
		objects, err = newCensusSourceContinuationObjects(root)
		if err == nil {
			heads, err = newCensusSourceContinuationHeads(root)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"parity":"object"}`)
	objectSelector, err := objects.Put(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	objectBytes, err := objects.Get(context.Background(), objectSelector)
	if err != nil {
		t.Fatal(err)
	}
	countObjects := &parityCountingObjects{ImmutableStore: objects}
	countHeads := &parityCountingHeads{HeadStore: heads}
	ordinalZero := continuationTerminal(plan, 0, censussourcecontinuation.OutcomeSuccessfulEmpty)
	discoveries := []int{}
	request := censussourcecontinuation.Request{
		Plan: plan, PartitionSize: 1, WorkflowKey: "parity", HeadKey: "parity", ExistingOrdinalZero: &ordinalZero,
		Objects: countObjects, Heads: countHeads,
		Discover: func(_ context.Context, ordinal int, _ []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error) {
			discoveries = append(discoveries, ordinal)
			return continuationTerminal(plan, ordinal, censussourcecontinuation.OutcomeSuccessfulEmpty), nil
		},
	}
	first, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || first.Status != censussourcecontinuation.StatusCommitted {
		t.Fatalf("ASSERT_STORE_PARITY_FIRST_COMMIT neutral=%t result=%+v err=%v", neutral, first, err)
	}
	headBytes, err := heads.Load(context.Background(), request.HeadKey)
	if err != nil {
		t.Fatal(err)
	}
	headSelector, err := censussourceassembly.HeadSelector(request.HeadKey)
	if err != nil {
		t.Fatal(err)
	}
	if exact, err := root.ReadSelector(headSelector, censussourceassembly.MaxBytes); err != nil || !bytes.Equal(exact, headBytes) {
		t.Fatalf("ASSERT_STORE_PARITY_EXACT_HEAD_BYTES neutral=%t exact=%q head=%q err=%v", neutral, exact, headBytes, err)
	}
	stale := heads.CompareAndSwap(context.Background(), request.HeadKey, censussourcecontinuation.HeadRevision{}, headBytes)
	otherRoot := openCensusRuntimeRoot(t)
	var freshHeads censussourcecontinuation.HeadStore
	if neutral {
		freshHeads, err = censussourceassembly.NewHeads(otherRoot)
	} else {
		freshHeads, err = newCensusSourceContinuationHeads(otherRoot)
	}
	if err != nil {
		t.Fatal(err)
	}
	committed := freshHeads.CompareAndSwap(context.Background(), "commit", censussourcecontinuation.HeadRevision{}, headBytes)
	request.ExistingOrdinalZero = nil
	request.Expected = first.Head
	second, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || second.Status != censussourcecontinuation.StatusCommitted {
		t.Fatalf("ASSERT_STORE_PARITY_SECOND_COMMIT neutral=%t result=%+v err=%v", neutral, second, err)
	}
	putsBefore, casBefore := countObjects.puts, countHeads.cas
	request.Expected = second.Head
	resumed, err := censussourcecontinuation.CoordinateNext(context.Background(), request)
	if err != nil || resumed.Status != censussourcecontinuation.StatusCommitted {
		t.Fatalf("ASSERT_STORE_PARITY_RESUME neutral=%t result=%+v err=%v", neutral, resumed, err)
	}
	return parityRun{
		objectSelector: objectSelector, objectBytes: objectBytes, headSelector: headSelector, headBytes: headBytes,
		committed: committed.Outcome, stale: stale.Outcome, resume: resumed, discoveries: discoveries,
		putsBefore: putsBefore, putsAfter: countObjects.puts, casBefore: casBefore, casAfter: countHeads.cas,
	}
}

func TestCensusSourceContinuationNeutralStoreParity(t *testing.T) {
	plan := continuationPlan(t, 1)
	legacy := runContinuationStoreParity(t, false, plan)
	neutral := runContinuationStoreParity(t, true, plan)
	if legacy.objectSelector != neutral.objectSelector || !bytes.Equal(legacy.objectBytes, neutral.objectBytes) {
		t.Fatalf("ASSERT_STORE_PARITY_OBJECT_SELECTOR_BYTES legacy=%+v neutral=%+v", legacy, neutral)
	}
	if legacy.headSelector != neutral.headSelector || !bytes.Equal(legacy.headBytes, neutral.headBytes) {
		t.Fatalf("ASSERT_STORE_PARITY_HEAD_SELECTOR_BYTES legacy=%+v neutral=%+v", legacy, neutral)
	}
	if legacy.committed != censussourcecontinuation.CASCommitted || legacy.stale != censussourcecontinuation.CASNotCommitted || legacy.committed != neutral.committed || legacy.stale != neutral.stale {
		t.Fatalf("ASSERT_STORE_PARITY_CAS_CLASSIFICATIONS legacy=%+v neutral=%+v", legacy, neutral)
	}
	if !reflect.DeepEqual(legacy.resume, neutral.resume) || !reflect.DeepEqual(legacy.discoveries, neutral.discoveries) || legacy.putsAfter-legacy.putsBefore != neutral.putsAfter-neutral.putsBefore || legacy.casAfter-legacy.casBefore != neutral.casAfter-neutral.casBefore {
		t.Fatalf("ASSERT_STORE_PARITY_OPERATION_COUNTS legacy=%+v neutral=%+v", legacy, neutral)
	}
}
