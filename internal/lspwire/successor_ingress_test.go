package lspwire

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

const (
	testSuccessorHeaderLimit  = uint64(65536)
	testSuccessorFrameLimit   = uint64(2097152)
	testSuccessorConsumeLimit = uint64(8388608)
	testSuccessorAcquireLimit = uint64(8392705)
)

func testSuccessorOptions() SuccessorIngressOptions {
	return SuccessorIngressOptions{
		OwnerID: "unit1-owner",
		Limits: SuccessorIngressLimits{
			HeaderBytes: testSuccessorHeaderLimit, FrameBytes: testSuccessorFrameLimit,
			ConsumptionBytes: testSuccessorConsumeLimit, AcquisitionBytes: testSuccessorAcquireLimit,
			MaxReadBytes: 4096, PrefetchBytes: 4096,
			HistoryAcquiredBytes: 8388608, HistoryOutstandingBytes: 8388608,
		},
	}
}

func successorMessage() []byte     { return []byte(`{"jsonrpc":"2.0","method":"x"}`) }
func successorHeader(n int) []byte { return []byte("Content-Length: " + itoa(n) + "\r\n\r\n") }
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type successorCapture struct{ frames [][]byte }

func (c *successorCapture) ObserveOriginalFrame(p []byte) {
	c.frames = append(c.frames, append([]byte(nil), p...))
}

type successorScriptRead struct {
	data []byte
	err  error
	want int
}
type successorScriptReader struct {
	t     *testing.T
	reads []successorScriptRead
	calls int
}

func (r *successorScriptReader) Read(p []byte) (int, error) {
	r.t.Helper()
	if r.calls >= len(r.reads) {
		r.t.Fatalf("unexpected read %d", r.calls+1)
	}
	x := r.reads[r.calls]
	r.calls++
	if x.want != 0 && len(p) != x.want {
		r.t.Fatalf("read %d request=%d want=%d", r.calls, len(p), x.want)
	}
	if len(p) > 4096 {
		r.t.Fatalf("read %d request=%d exceeds 4096", r.calls, len(p))
	}
	copy(p, x.data)
	return len(x.data), x.err
}

func TestSuccessorIngressHistoryBoundsAndNoRetrocharge(t *testing.T) {
	o := testSuccessorOptions()
	o.History = &ImmutableHistoryMetadata{Identity: "history", Cut: "cut", Acquired: 8388608, Outstanding: 8388608}
	frame := append(successorHeader(30), successorMessage()...)
	r, err := NewSuccessorIngressReader(bytes.NewReader(frame), o)
	if err != nil {
		t.Fatalf("at-bound constructor: %v", err)
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatalf("read: %v", err)
	}
	if r.acquired != 52 || r.consumed != 52 || r.validated != 52 {
		t.Fatalf("A/C/V=%d/%d/%d want 52/52/52", r.acquired, r.consumed, r.validated)
	}
	if o.History.Acquired != 8388608 || o.History.Outstanding != 8388608 {
		t.Fatalf("history mutated: %+v", *o.History)
	}

	o.History = &ImmutableHistoryMetadata{Identity: "history", Cut: "cut", Acquired: 8388609}
	if _, err := NewSuccessorIngressReader(bytes.NewReader(nil), o); err == nil {
		t.Fatal("history acquired over bound accepted")
	}
}

func TestSuccessorIngressFullEOFAndShortEOF(t *testing.T) {
	header, body := successorHeader(30), successorMessage()
	for _, tc := range []struct {
		name    string
		body    []byte
		bodyErr error
		wantErr bool
		a, c, v uint64
	}{
		{"full", body, io.EOF, false, 52, 52, 52},
		{"short", body[:29], io.EOF, true, 51, 51, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &successorScriptReader{t: t, reads: []successorScriptRead{{header, nil, 4096}, {tc.body, tc.bodyErr, 30}}}
			r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
			if err != nil {
				t.Fatal(err)
			}
			_, got := r.ReadFrame()
			if (got != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", got, tc.wantErr)
			}
			if r.acquired != tc.a || r.consumed != tc.c || r.validated != tc.v {
				t.Fatalf("A/C/V=%d/%d/%d want %d/%d/%d", r.acquired, r.consumed, r.validated, tc.a, tc.c, tc.v)
			}
		})
	}
}

func TestSuccessorIngressCarryConsumesBeforeRead(t *testing.T) {
	frame := append(successorHeader(30), successorMessage()...)
	first := append(append([]byte(nil), frame...), successorHeader(30)[:5]...)
	second := append(append([]byte(nil), successorHeader(30)[5:]...), successorMessage()...)
	s := &successorScriptReader{t: t, reads: []successorScriptRead{{first, nil, 4096}, {second, nil, 4096}}}
	r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if r.acquired != 57 || r.consumed != 52 || len(r.prefetch) != 5 {
		t.Fatalf("first A/C/prefetch=%d/%d/%d", r.acquired, r.consumed, len(r.prefetch))
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if s.calls != 2 || r.acquired != 104 || r.consumed != 104 || r.validated != 104 || len(r.prefetch) != 0 {
		t.Fatalf("final calls/A/C/V/prefetch=%d/%d/%d/%d/%d", s.calls, r.acquired, r.consumed, r.validated, len(r.prefetch))
	}
}

func TestSuccessorIngressReturnedBytesChargedBeforeTransportError(t *testing.T) {
	errX := errors.New("errX")
	header := successorHeader(30)
	s := &successorScriptReader{t: t, reads: []successorScriptRead{{header, nil, 4096}, {[]byte("xx"), errX, 30}}}
	r, err := NewSuccessorIngressReader(s, testSuccessorOptions())
	if err != nil {
		t.Fatal(err)
	}
	_, got := r.ReadFrame()
	if !errors.Is(got, errX) {
		t.Fatalf("err=%v want errX", got)
	}
	if r.acquired != 24 || r.consumed != 24 || r.validated != 0 {
		t.Fatalf("A/C/V=%d/%d/%d want 24/24/0", r.acquired, r.consumed, r.validated)
	}
}
