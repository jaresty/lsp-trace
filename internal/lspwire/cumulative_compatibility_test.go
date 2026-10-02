package lspwire

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"
)

// Both ordinary readers and the accepted per-frame-only path must capture all
// consumed body bytes before notifying the body observer, even on a short read.
func TestCumulativeDisabledCaptureBeforeBodyObserver(t *testing.T) {
	for _, perFrame := range []bool{false, true} {
		for _, partial := range []bool{false, true} {
			t.Run(fmt.Sprintf("perFrame=%t/partial=%t", perFrame, partial), func(t *testing.T) {
				body := []byte(`{"jsonrpc":"2.0","method":"ping"}`)
				header := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
				if partial {
					body = body[:len(body)-1]
				}
				wire := append(header, body...)
				var r *Reader
				observed := false
				r = NewReaderObserved(bytes.NewReader(wire), DefaultLimits(), func(e Event) {
					if e.Stage != EventBodyRead {
						return
					}
					observed = true
					if r.frameCapture.bytes != int64(len(wire)) || !bytes.Equal(r.frameCapture.raw, wire) {
						t.Errorf("capture-before-body-observer: captured=%d want=%d", r.frameCapture.bytes, len(wire))
					}
				})
				if perFrame {
					r.LimitOriginalFrameBytes(4096)
				}
				_, _, _, _, err := r.ReadWithFrameIfWithin(4096)
				if partial && !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("partial error=%v", err)
				}
				if !partial && err != nil {
					t.Fatal(err)
				}
				if !observed {
					t.Fatal("body observer not reached")
				}
			})
		}
	}
}

func TestCumulativeFailedReadsDoNotCharge(t *testing.T) {
	for _, body := range []string{`{`, `{"jsonrpc":"1.0","method":"ping"}`, `{"jsonrpc":"2.0"}`} {
		t.Run(body, func(t *testing.T) {
			good := `{"jsonrpc":"2.0","method":"ping"}`
			frame := func(s string) string { return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(s), s) }
			r := NewReader(bytes.NewBufferString(frame(body)+frame(good)), DefaultLimits())
			r.LimitCumulativeOriginalFrameBytes(int64(len(frame(good)) + len(frame(body))))
			if _, err := r.Read(); err == nil {
				t.Fatal("invalid frame accepted")
			}
			if r.cumulativeFrameBytes != 0 {
				t.Errorf("failed-read-charge: got=%d want=0", r.cumulativeFrameBytes)
			}
			if _, err := r.Read(); err != nil {
				t.Errorf("valid frame after excluded failure: %v", err)
			}
		})
	}
	for _, wire := range []string{"Content-Len", "Content-Length: 10\r\n\r\n{"} {
		r := NewReader(bytes.NewBufferString(wire), DefaultLimits())
		r.LimitCumulativeOriginalFrameBytes(4096)
		if _, err := r.Read(); err == nil {
			t.Fatal("partial frame accepted")
		}
		if r.cumulativeFrameBytes != 0 {
			t.Fatalf("partial-read-charge: got=%d", r.cumulativeFrameBytes)
		}
	}
}
