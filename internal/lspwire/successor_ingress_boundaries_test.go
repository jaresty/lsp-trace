package lspwire

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// These guards use deterministic literals pinned by the ADR0011 P1 Unit 1 v4
// packet. They intentionally exercise the production ReadFrame driver. Private
// step-seam cases remain blocked until the specification's seam exists.

var errSuccessorBoundaryUnexpectedRead = errors.New("successor boundary: unexpected read")

type successorBoundaryRead struct {
	data []byte
	err  error
	want int
}

type successorBoundaryReader struct {
	reads           []successorBoundaryRead
	calls           int
	max             int
	requestMismatch bool
}

type successorBoundaryCappedReader struct {
	r     io.Reader
	calls int
	max   int
}

type successorBoundaryCapture struct{ frames [][]byte }

func (c *successorBoundaryCapture) ObserveOriginalFrame(frame []byte) {
	c.frames = append(c.frames, append([]byte(nil), frame...))
}

func (r *successorBoundaryCappedReader) Read(p []byte) (int, error) {
	if len(p) > r.max {
		r.max = len(p)
	}
	if len(p) > 4096 {
		p = p[:4096]
	}
	r.calls++
	return r.r.Read(p)
}

func (r *successorBoundaryReader) Read(p []byte) (int, error) {
	if len(p) > r.max {
		r.max = len(p)
	}
	if r.calls >= len(r.reads) {
		r.calls++
		return 0, errSuccessorBoundaryUnexpectedRead
	}
	x := r.reads[r.calls]
	r.calls++
	if x.want >= 0 && len(p) != x.want {
		r.requestMismatch = true
	}
	copy(p, x.data)
	return len(x.data), x.err
}

func successorBoundaryHeaderAt() []byte {
	const prefix = "Content-Length: 30\r\nX-Pad: "
	const suffix = "\r\n\r\n"
	return []byte(prefix + string(bytes.Repeat([]byte{'a'}, 65536-len(prefix)-len(suffix))) + suffix)
}

func successorBoundaryHeaderOver() []byte {
	const prefix = "Content-Length: 30\r\nX-Pad: "
	return []byte(prefix + string(bytes.Repeat([]byte{'a'}, 65510)))
}

func successorBoundaryFrame(bodyBytes int) []byte {
	body := append([]byte(nil), successorMessage()...)
	body = append(body, bytes.Repeat([]byte{' '}, bodyBytes-len(body))...)
	return append(successorHeader(bodyBytes), body...)
}

func TestSuccessorBoundaryHeaderAtAndOver(t *testing.T) {
	t.Run("at", func(t *testing.T) {
		header := successorBoundaryHeaderAt()
		if len(header) != 65536 {
			t.Fatalf("fixture header bytes=%d want=65536", len(header))
		}
		stream := append(append([]byte(nil), header...), successorMessage()...)
		r, err := NewSuccessorIngressReader(bytes.NewReader(stream), testSuccessorOptions())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.ReadFrame(); err != nil {
			t.Fatalf("ReadFrame at header bound: %v", err)
		}
		if r.acquired != 65566 || r.consumed != 65566 || r.validated != 65566 {
			t.Fatalf("A/C/V=%d/%d/%d want 65566/65566/65566", r.acquired, r.consumed, r.validated)
		}
	})

	t.Run("over stops on observation byte", func(t *testing.T) {
		over := successorBoundaryHeaderOver()
		if len(over) != 65537 {
			t.Fatalf("fixture header-over bytes=%d want=65537", len(over))
		}
		reads := make([]successorBoundaryRead, 0, 17)
		for i := 0; i < 16; i++ {
			reads = append(reads, successorBoundaryRead{data: over[i*4096 : (i+1)*4096], want: 4096})
		}
		reads = append(reads, successorBoundaryRead{data: over[65536:], want: 1})
		s := &successorBoundaryReader{reads: reads}
		r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
		if err != nil {
			t.Fatal(err)
		}
		_, got := r.ReadFrame()
		if got == nil {
			t.Fatal("header-over accepted")
		}
		if s.calls != 17 || s.requestMismatch || r.acquired != 65537 || r.consumed != 65537 || r.validated != 0 {
			t.Fatalf("calls/requestMismatch/A/C/V=%d/%v/%d/%d/%d want 17/false/65537/65537/0", s.calls, s.requestMismatch, r.acquired, r.consumed, r.validated)
		}
	})
}

func TestSuccessorBoundaryFrameAtAndOver(t *testing.T) {
	t.Run("at and read requests bounded", func(t *testing.T) {
		frame := successorBoundaryFrame(2097125)
		if len(frame) != 2097152 {
			t.Fatalf("fixture frame bytes=%d want=2097152", len(frame))
		}
		s := &successorBoundaryCappedReader{r: bytes.NewReader(frame)}
		r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
		if err != nil {
			t.Fatal(err)
		}
		_, got := r.ReadFrame()
		if got != nil {
			t.Fatalf("ReadFrame at frame bound: %v", got)
		}
		if s.calls != 512 || s.max > 4096 || r.acquired != 2097152 || r.consumed != 2097152 || r.validated != 2097152 {
			t.Fatalf("calls/max/A/C/V=%d/%d/%d/%d/%d want 512/<=4096/2097152/2097152/2097152", s.calls, s.max, r.acquired, r.consumed, r.validated)
		}
	})

	t.Run("over rejects from header without body read", func(t *testing.T) {
		header := successorHeader(2097126)
		s := &successorBoundaryReader{reads: []successorBoundaryRead{{data: header, want: 4096}}}
		r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
		if err != nil {
			t.Fatal(err)
		}
		_, got := r.ReadFrame()
		if got == nil {
			t.Fatal("frame-over accepted")
		}
		if s.calls != 1 || r.acquired != 27 || r.consumed != 27 || r.validated != 0 {
			t.Fatalf("calls/A/C/V=%d/%d/%d/%d want 1/27/27/0", s.calls, r.acquired, r.consumed, r.validated)
		}
	})
}

func TestSuccessorBoundaryCumulativeAt(t *testing.T) {
	frame := successorBoundaryFrame(2097125)
	stream := io.MultiReader(bytes.NewReader(frame), bytes.NewReader(frame), bytes.NewReader(frame), bytes.NewReader(frame))
	r, err := NewSuccessorIngressReader(stream, testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := r.ReadFrame(); err != nil {
			t.Fatalf("frame %d at cumulative bound: %v", i+1, err)
		}
	}
	if r.acquired != 8388608 || r.consumed != 8388608 || r.validated != 8388608 {
		t.Fatalf("at A/C/V=%d/%d/%d want 8388608/8388608/8388608", r.acquired, r.consumed, r.validated)
	}
}

func TestSuccessorBoundaryCumulativeCrossing(t *testing.T) {
	frame := successorBoundaryFrame(2097125)
	cross := append(successorHeader(30), successorMessage()...)
	stream := io.MultiReader(bytes.NewReader(frame), bytes.NewReader(frame), bytes.NewReader(frame), bytes.NewReader(frame), bytes.NewReader(cross))
	r, err := NewSuccessorIngressReader(stream, testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := r.ReadFrame(); err != nil {
			t.Fatalf("setup frame %d: %v", i+1, err)
		}
	}
	_, got := r.ReadFrame()
	if got == nil {
		t.Fatal("cumulative crossing frame accepted")
	}
	if r.consumed != 8388609 || r.validated != 8388608 {
		t.Fatalf("cross C/V=%d/%d want 8388609/8388608", r.consumed, r.validated)
	}
}

func TestSuccessorBoundaryMessageAttemptReachedPrecedence(t *testing.T) {
	r := &SuccessorIngressReader{}
	for _, tc := range []struct {
		name string
		in   ingressReached
		want ingressOutcome
	}{
		{name: "preflight", in: ingressReached{Preflight: errors.New("preflight"), MessageAttempts: errIngressMessageAttempts}, want: ingressOutcomePreflight},
		{name: "deadline", in: ingressReached{Deadline: errors.New("deadline"), MessageAttempts: errIngressMessageAttempts}, want: ingressOutcomeDeadline},
		{name: "cancellation", in: ingressReached{Cancellation: errors.New("cancellation"), MessageAttempts: errIngressMessageAttempts}, want: ingressOutcomeCancellation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.selectReached(tc.in); got.Kind != tc.want {
				t.Fatalf("reached precedence=%v want %v", got.Kind, tc.want)
			}
		})
	}
}

func TestSuccessorBoundaryMessageAttemptsAtAndOver(t *testing.T) {
	frame := append(successorHeader(30), successorMessage()...)
	stream := bytes.Repeat(frame, 66)
	capture := &successorBoundaryCapture{}
	s := &successorBoundaryCappedReader{r: bytes.NewReader(stream)}
	options := testSuccessorOptions()
	options.Limits.MessageAttempts = 64
	options.Capture = capture
	r, err := NewSuccessorIngressReader(s, options)
	if err != nil {
		t.Fatal(err)
	}
	r.recorder = &ingressTestRecorder{limit: 1024}
	for i := 1; i <= 64; i++ {
		if _, got := r.ReadFrame(); got != nil {
			t.Fatalf("message %d at attempt bound: %v", i, got)
		}
	}
	wantValidated := uint64(64 * len(frame))
	if r.acquired != uint64(len(stream)) || r.consumed != wantValidated || r.validated != wantValidated || r.messageAttempts != 64 || len(capture.frames) != 64 || r.sealed {
		t.Fatalf("at A/C/V/attempts/captures/sealed=%d/%d/%d/%d/%d/%v want %d/%d/%d/64/64/false", r.acquired, r.consumed, r.validated, r.messageAttempts, len(capture.frames), r.sealed, len(stream), wantValidated, wantValidated)
	}
	readsAt := s.calls
	acquiredAt, eventsAt := r.acquired, len(r.recorder.events)
	_, got := r.ReadFrame()
	if !errors.Is(got, errIngressMessageAttempts) || !r.sealed || r.terminalOutcome != ingressOutcomeMessageAttempts {
		t.Fatalf("65th outcome=%v sealed=%v terminal=%v", got, r.sealed, r.terminalOutcome)
	}
	if s.calls != readsAt || r.acquired != acquiredAt || r.consumed != wantValidated || r.validated != wantValidated || r.messageAttempts != 65 || len(capture.frames) != 64 {
		t.Fatalf("65th reads/A/C/V/attempts/captures=%d/%d/%d/%d/%d/%d want %d/%d/%d/%d/65/64", s.calls, r.acquired, r.consumed, r.validated, r.messageAttempts, len(capture.frames), readsAt, acquiredAt, wantValidated, wantValidated)
	}
	if len(r.recorder.events) != eventsAt+2 || r.recorder.events[eventsAt].Stage != ingressEventSeal || r.recorder.events[eventsAt+1].Stage != ingressEventOutcome {
		t.Fatalf("65th event order=%+v", r.recorder.events[eventsAt:])
	}
	_, again := r.ReadFrame()
	if !errors.Is(again, errIngressSealed) || s.calls != readsAt || len(r.recorder.events) != eventsAt+2 || r.terminalOutcome != ingressOutcomeMessageAttempts {
		t.Fatalf("66th err=%v reads=%d events=%d terminal=%v", again, s.calls, len(r.recorder.events), r.terminalOutcome)
	}
}

func TestSuccessorBoundaryFullAndShortEOF(t *testing.T) {
	header, body := successorHeader(30), successorMessage()
	for _, tc := range []struct {
		name    string
		body    []byte
		ok      bool
		a, c, v uint64
	}{
		{name: "full", body: body, ok: true, a: 52, c: 52, v: 52},
		{name: "short", body: body[:29], ok: false, a: 51, c: 51, v: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &successorBoundaryReader{reads: []successorBoundaryRead{{data: header, want: 4096}, {data: tc.body, err: io.EOF, want: 30}}}
			r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
			if err != nil {
				t.Fatal(err)
			}
			_, got := r.ReadFrame()
			if (got == nil) != tc.ok {
				t.Fatalf("err=%v ok=%v", got, tc.ok)
			}
			if r.acquired != tc.a || r.consumed != tc.c || r.validated != tc.v {
				t.Fatalf("A/C/V=%d/%d/%d want %d/%d/%d", r.acquired, r.consumed, r.validated, tc.a, tc.c, tc.v)
			}
		})
	}
}
