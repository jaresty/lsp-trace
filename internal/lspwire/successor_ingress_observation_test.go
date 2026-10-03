package lspwire

import (
	"bytes"
	"testing"
)

func TestSuccessorObservationRealTransitionOrderAndBound(t *testing.T) {
	frame := append(successorHeader(30), successorMessage()...)
	r, err := NewSuccessorIngressReader(bytes.NewReader(frame), testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	rec := &ingressTestRecorder{limit: 4}
	r.recorder = rec
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 4 || !rec.overflow {
		t.Fatalf("events=%d overflow=%v want 4/true", len(rec.events), rec.overflow)
	}
	if rec.events[0].Stage != ingressEventRead || rec.events[0].AfterA != 52 || rec.events[0].AfterUnread != 52 || rec.events[0].Requested != 4096 || rec.events[0].ReadN != 52 {
		t.Fatalf("first event=%+v", rec.events[0])
	}
	if rec.events[0].Supported&ingressSupportLedger == 0 || rec.events[0].Present&ingressSupportLedger == 0 {
		t.Fatalf("support/presence=%x/%x", rec.events[0].Supported, rec.events[0].Present)
	}
}

func TestSuccessorObservationNilAndTrustedNoninterference(t *testing.T) {
	frame := append(successorHeader(30), successorMessage()...)
	a, _ := NewSuccessorIngressReader(bytes.NewReader(frame), testSuccessorOptions())
	b, _ := NewSuccessorIngressReader(bytes.NewReader(frame), testSuccessorOptions())
	b.recorder = &ingressTestRecorder{limit: 64}
	ma, ea := a.ReadFrame()
	mb, eb := b.ReadFrame()
	if ea != nil || eb != nil || ma.Method != mb.Method {
		t.Fatalf("results %v/%v %+v/%+v", ea, eb, ma, mb)
	}
	if a.acquired != b.acquired || a.consumed != b.consumed || a.validated != b.validated || len(a.prefetch) != len(b.prefetch) || a.sealed != b.sealed || a.terminalOutcome != b.terminalOutcome {
		t.Fatal("recorder interfered")
	}
}

func TestSuccessorObservationCaptureBeforeDecode(t *testing.T) {
	frame := append(successorHeader(30), successorMessage()...)
	capture := &successorCapture{}
	o := testSuccessorOptions()
	o.Capture = capture
	r, _ := NewSuccessorIngressReader(bytes.NewReader(frame), o)
	rec := &ingressTestRecorder{limit: 64}
	r.recorder = rec
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	captureAt, decodeAt := -1, -1
	for i, e := range rec.events {
		if e.Stage == ingressEventCapture {
			captureAt = i
		}
		if e.Stage == ingressEventDecode {
			decodeAt = i
		}
	}
	if captureAt < 0 || decodeAt < 0 || captureAt >= decodeAt || len(capture.frames) != 1 {
		t.Fatalf("capture/decode=%d/%d captures=%d", captureAt, decodeAt, len(capture.frames))
	}
}
