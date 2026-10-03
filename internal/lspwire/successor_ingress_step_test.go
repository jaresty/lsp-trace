package lspwire

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestSuccessorStepPrefetchPlanningAndSentinel(t *testing.T) {
	r, _ := NewSuccessorIngressReader(bytes.NewReader(nil), testSuccessorOptions())
	r.prefetch = r.prefetch[:5]
	got := r.step(ingressStepInput{Checkpoint: ingressPreRead, HeaderRemaining: 10000})
	if got.NeedRead != 4091 || cap(r.prefetch) != 4096 {
		t.Fatalf("NeedRead/cap=%d/%d want4091/4096", got.NeedRead, cap(r.prefetch))
	}
	r.consumed = 8388606
	copy(r.prefetch, "abc")
	r.prefetch = r.prefetch[:3]
	got = r.step(ingressStepInput{Checkpoint: ingressConsume, Consume: 3})
	if got.OutcomeKind != ingressOutcomeConsumptionLimit || r.consumed != 8388609 || !r.sealed {
		t.Fatalf("outcome/C/seal=%v/%d/%v", got.OutcomeKind, r.consumed, r.sealed)
	}
}

func TestSuccessorStepPriorityAndOriginalTerminalPersists(t *testing.T) {
	r, _ := NewSuccessorIngressReader(bytes.NewReader(nil), testSuccessorOptions())
	deadline, cancel := errors.New("deadline"), errors.New("cancel")
	got := r.step(ingressStepInput{Checkpoint: ingressPostRead, Returned: []byte("z"), Known: knownFailures{Deadline: deadline, Cancellation: cancel}})
	if got.OutcomeKind != ingressOutcomeDeadline || r.acquired != 1 || r.terminalOutcome != ingressOutcomeDeadline {
		t.Fatalf("got=%+v A=%d terminal=%v", got, r.acquired, r.terminalOutcome)
	}
	before := r.acquired
	got = r.step(ingressStepInput{Checkpoint: ingressPreRead})
	if got.OutcomeKind != ingressOutcomeRefusedSealed || r.terminalOutcome != ingressOutcomeDeadline || r.acquired != before {
		t.Fatalf("refusal=%+v terminal=%v", got, r.terminalOutcome)
	}
}

func TestSuccessorTerminalCategoriesAndSuccessActive(t *testing.T) {
	cases := []struct {
		name   string
		reads  []successorScriptRead
		mutate func(*SuccessorIngressOptions)
		want   ingressOutcome
	}{
		{"shortEOF", []successorScriptRead{{successorHeader(30), nil, 4096}, {successorMessage()[:29], io.EOF, 30}}, nil, ingressOutcomeTransportShortEOF},
		{"transport", []successorScriptRead{{successorHeader(30), nil, 4096}, {successorMessage(), errors.New("x"), 30}}, nil, ingressOutcomeTransport},
		{"decode", []successorScriptRead{{append(successorHeader(30), bytes.Repeat([]byte{'x'}, 30)...), nil, 4096}}, nil, ingressOutcomeDecodeValidation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := testSuccessorOptions()
			if tc.mutate != nil {
				tc.mutate(&o)
			}
			s := &successorScriptReader{t: t, reads: tc.reads}
			r, err := NewSuccessorIngressReader(s, o)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.ReadFrame(); err == nil {
				t.Fatal("wanted failure")
			}
			if !r.sealed || r.terminalOutcome != tc.want {
				t.Fatalf("seal/outcome=%v/%v want true/%v", r.sealed, r.terminalOutcome, tc.want)
			}
			calls := s.calls
			if _, err = r.ReadFrame(); err == nil || s.calls != calls || r.terminalOutcome != tc.want {
				t.Fatalf("repeat err/calls/outcome=%v/%d/%v", err, s.calls, r.terminalOutcome)
			}
		})
	}
	frame := append(successorHeader(30), successorMessage()...)
	r, _ := NewSuccessorIngressReader(bytes.NewReader(append(append([]byte{}, frame...), frame...)), testSuccessorOptions())
	if _, e := r.ReadFrame(); e != nil {
		t.Fatal(e)
	}
	if r.sealed {
		t.Fatal("success sealed")
	}
	if _, e := r.ReadFrame(); e != nil {
		t.Fatal(e)
	}
}

func TestSuccessorPartialTransportErrorClassAndSeal(t *testing.T) {
	errX := errors.New("partial transport")
	s := &successorScriptReader{t: t, reads: []successorScriptRead{
		{successorHeader(30), nil, 4096},
		{[]byte("xx"), errX, 30},
	}}
	r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	rec := &ingressTestRecorder{limit: 64}
	r.recorder = rec

	if _, err = r.ReadFrame(); !errors.Is(err, errX) {
		t.Fatalf("ReadFrame error=%v want errors.Is(errX)", err)
	}
	if r.acquired != 24 || r.consumed != 24 || r.validated != 0 {
		t.Fatalf("A/C/V=%d/%d/%d want24/24/0", r.acquired, r.consumed, r.validated)
	}
	if r.terminalOutcome != ingressOutcomeTransport {
		t.Fatalf("outcome=%v want=%v", r.terminalOutcome, ingressOutcomeTransport)
	}
	if !r.sealed {
		t.Fatal("partial transport failure did not seal")
	}

	sealIndex, outcomeIndex := -1, -1
	for i, event := range rec.events {
		switch event.Stage {
		case ingressEventCapture, ingressEventDecode:
			t.Fatalf("unexpected post-incomplete-body event stage=%v", event.Stage)
		case ingressEventSeal:
			if sealIndex >= 0 {
				t.Fatal("more than one seal event")
			}
			sealIndex = i
			if event.Outcome != ingressOutcomeTransport {
				t.Fatalf("seal outcome=%v want=%v", event.Outcome, ingressOutcomeTransport)
			}
		case ingressEventOutcome:
			if outcomeIndex >= 0 {
				t.Fatal("more than one outcome event")
			}
			outcomeIndex = i
			if event.Outcome != ingressOutcomeTransport {
				t.Fatalf("event outcome=%v want=%v", event.Outcome, ingressOutcomeTransport)
			}
		}
	}
	if sealIndex < 0 || outcomeIndex != sealIndex+1 {
		t.Fatalf("seal/outcome order=%d/%d want adjacent selected order", sealIndex, outcomeIndex)
	}

	calls, events := s.calls, len(rec.events)
	a, c, v := r.acquired, r.consumed, r.validated
	originalKind, originalErr := r.terminalOutcome, err
	if _, repeatErr := r.ReadFrame(); !errors.Is(repeatErr, errIngressSealed) {
		t.Fatalf("repeat error=%v want sealed refusal", repeatErr)
	}
	if s.calls != calls || len(rec.events) != events || r.acquired != a || r.consumed != c || r.validated != v {
		t.Fatalf("repeat activity calls/events/A/C/V=%d/%d/%d/%d/%d want%d/%d/%d/%d/%d", s.calls, len(rec.events), r.acquired, r.consumed, r.validated, calls, events, a, c, v)
	}
	if r.terminalOutcome != originalKind || !errors.Is(originalErr, errX) {
		t.Fatalf("original kind/error changed=%v/%v", r.terminalOutcome, originalErr)
	}
}

func TestSuccessorMetadataValidationOnly(t *testing.T) {
	o := testSuccessorOptions()
	o.History = &ImmutableHistoryMetadata{Identity: "h", Cut: "c", Acquired: 8388608, Outstanding: 8388608}
	r, e := NewSuccessorIngressReader(bytes.NewReader(nil), o)
	if e != nil || r.acquired != 0 || r.consumed != 0 || r.validated != 0 {
		t.Fatalf("at metadata %v %d/%d/%d", e, r.acquired, r.consumed, r.validated)
	}
	for _, h := range []*ImmutableHistoryMetadata{{Identity: "h", Cut: "c", Acquired: 8388609}, {Identity: "h", Cut: "c", Outstanding: 8388609}, {Identity: "", Cut: "c"}, {Identity: "h", Cut: ""}} {
		o.History = h
		if _, e := NewSuccessorIngressReader(bytes.NewReader(nil), o); e == nil {
			t.Fatalf("accepted metadata %+v", h)
		}
	}
}

func TestSuccessorScratchReservationAliasAndOverlapV2(t *testing.T) {
	l := newIngressStorageLedger(20)
	if !l.reserveNew(1, 8, ingressStorageScratch) || !l.alias(1) {
		t.Fatal("old/alias")
	}
	if !l.reserveGrowth(1, 2, 12, ingressStorageScratch) || l.peak != 20 || l.live != 20 {
		t.Fatalf("aliased growth live/peak=%d/%d want20/20", l.live, l.peak)
	}
	if !l.releaseAlias(1) || l.live != 12 {
		t.Fatalf("final alias release live=%d want12", l.live)
	}

	n := newIngressStorageLedger(20)
	if !n.reserveNew(1, 8, ingressStorageScratch) || !n.reserveGrowth(1, 2, 12, ingressStorageScratch) || n.peak != 20 || n.live != 12 {
		t.Fatalf("no-alias growth live/peak=%d/%d", n.live, n.peak)
	}

	d := newIngressStorageLedger(19)
	if !d.reserveNew(1, 8, ingressStorageScratch) {
		t.Fatal("deny old")
	}
	if d.reserveGrowth(1, 2, 12, ingressStorageScratch) || d.live != 8 || d.peak != 8 {
		t.Fatalf("denial mutated live/peak=%d/%d", d.live, d.peak)
	}

	p := newIngressStorageLedger(4096)
	if !p.reserveNew(1, 4096, ingressStoragePrefetch) || p.reserveNew(2, 1, ingressStoragePrefetch) {
		t.Fatal("prefetch cap")
	}
}

func TestSuccessorRealReadReservationAndRequestedObservationV2(t *testing.T) {
	header, body := successorHeader(30), successorMessage()
	s := &successorScriptReader{t: t, reads: []successorScriptRead{{header, nil, 4096}, {body, nil, 30}}}
	r, _ := NewSuccessorIngressReader(s, testSuccessorOptions())
	rec := &ingressTestRecorder{limit: 64}
	r.recorder = rec
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	var requests []int
	for _, e := range rec.events {
		if e.Stage == ingressEventRead {
			requests = append(requests, e.Requested)
		}
	}
	if len(requests) != 2 || requests[0] != 4096 || requests[1] != 30 {
		t.Fatalf("requests=%v want[4096 30]", requests)
	}
	if r.storage.peak < 4096+30 {
		t.Fatalf("real allocations not reserved peak=%d", r.storage.peak)
	}

	r2, _ := NewSuccessorIngressReader(bytes.NewReader(append(header, body...)), testSuccessorOptions())
	r2.storage.budget = 4096 + 29
	if _, err := r2.ReadFrame(); !errors.Is(err, errIngressReserve) || r2.sealed == false || r2.terminalOutcome != ingressOutcomeReserveStorage {
		t.Fatalf("reserve denial err/seal/outcome=%v/%v/%v", err, r2.sealed, r2.terminalOutcome)
	}
}

func TestSuccessorReachedFailureSelectorAndValidationTerminalsV2(t *testing.T) {
	r, _ := NewSuccessorIngressReader(bytes.NewReader(nil), testSuccessorOptions())
	cancel := errors.New("cancel")
	got := r.selectReached(ingressReached{Cancellation: cancel, Reserve: errIngressReserve})
	if got.Kind != ingressOutcomeCancellation {
		t.Fatalf("cancel priority=%v", got.Kind)
	}

	for _, body := range [][]byte{[]byte(`{"jsonrpc":"1.0","method":"x"}`), []byte(`{"jsonrpc":"2.0"}`)} {
		frame := append(successorHeader(len(body)), body...)
		x, _ := NewSuccessorIngressReader(bytes.NewReader(frame), testSuccessorOptions())
		if _, e := x.ReadFrame(); e == nil || !x.sealed || x.terminalOutcome != ingressOutcomeDecodeValidation {
			t.Fatalf("validation terminal %v/%v/%v", e, x.sealed, x.terminalOutcome)
		}
	}
}
