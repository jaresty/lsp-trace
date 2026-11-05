package lspwire

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

type privateFrameCountReader struct {
	io.Reader
	n int
}

func (r *privateFrameCountReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	r.n += n
	return n, e
}
func TestPrivateGroupingFramePreReadBounds(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		max        int
	}{
		{"header", strings.Repeat("x", 8192), 65},
		{"huge-body", "Content-Length: 4611686018427387904\r\n\r\n" + strings.Repeat("x", 1024), len("Content-Length: 4611686018427387904\r\n\r\n")},
		{"nonnumeric", "Content-Length: nope\r\n\r\n" + strings.Repeat("x", 1024), len("Content-Length: nope\r\n")},
		{"frame-residual", "Content-Length: 80\r\n\r\n" + strings.Repeat("x", 80), len("Content-Length: 80\r\n\r\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &privateFrameCountReader{Reader: strings.NewReader(tc.wire)}
			cap := int64(1088)
			if tc.name == "frame-residual" {
				cap = 90
			}
			_, _, raw, _, err := ReadPrivateBoundedFrame(r, Limits{1024, 64}, cap, 0)
			if err == nil || r.n > tc.max || raw != nil {
				t.Fatalf("ASSERT_RESOURCE_FRAME_PRE_READ: read=%d max=%d err=%v", r.n, tc.max, err)
			}
		})
	}
}
func TestPrivateGroupingFrameRetainedNotRawBound(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":null}` + strings.Repeat(" ", 80<<10))
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	r := &privateFrameCountReader{Reader: bytes.NewReader(frame)}
	msg, o, raw, retained, err := ReadPrivateBoundedFrame(r, Limits{128 << 10, 4096}, 132<<10, 64<<10)
	sum := sha256.Sum256(frame)
	if err != nil || string(msg.Result) != "null" || raw != nil || retained || o.FrameBytes != int64(len(frame)) || o.FrameSHA256 != fmt.Sprintf("sha256:%x", sum) || r.n != len(frame) {
		t.Fatalf("ASSERT_RESOURCE_RETAINED_NOT_RAW: %+v retained=%v read=%d err=%v", o, retained, r.n, err)
	}
}
func TestPrivateGroupingFrameObserversNilAPI(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":[]}`)
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	for _, retain := range []int64{0, 1, 4096} {
		plain := &privateFrameCountReader{Reader: bytes.NewReader(append(append([]byte(nil), frame...), frame...))}
		observed := &privateFrameCountReader{Reader: bytes.NewReader(append(append([]byte(nil), frame...), frame...))}
		a, ao, ar, ak, ae := ReadPrivateBoundedFrame(plain, Limits{128 << 10, 4096}, 132<<10, retain)
		b, bo, br, bk, be := ReadPrivateBoundedFrameObservedAtDecode(observed, Limits{128 << 10, 4096}, 132<<10, retain, nil, nil, nil)
		if ae != nil || be != nil || !reflect.DeepEqual(a, b) || ao != bo || !bytes.Equal(ar, br) || ak != bk || plain.n != len(frame) || observed.n != len(frame) {
			t.Fatalf("ASSERT_PRIVATE_OBSERVER_PLAIN_API: retain=%d a=%v b=%v read=%d/%d", retain, ae, be, plain.n, observed.n)
		}
	}
}

func TestPrivateGroupingFrameEqualityAndUnknown(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":null}`)
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	for _, delta := range []int{0, -1} {
		r := &privateFrameCountReader{Reader: bytes.NewReader(frame)}
		_, _, raw, ok, err := ReadPrivateBoundedFrame(r, Limits{128 << 10, 4096}, int64(len(frame)+delta), int64(len(frame)+delta))
		if delta == 0 && (err != nil || !ok || !bytes.Equal(raw, frame)) {
			t.Fatal("ASSERT_RESOURCE_FRAME_EQUALITY", err)
		}
		if delta < 0 && (err == nil || r.n >= len(frame)) {
			t.Fatal("ASSERT_RESOURCE_FRAME_REJECT_BEFORE_BODY", err, r.n)
		}
	}
	r := &privateFrameCountReader{Reader: bytes.NewReader(frame)}
	if _, _, _, _, err := ReadPrivateBoundedFrame(r, Limits{}, 100, 0); err == nil || r.n != 0 {
		t.Fatal("ASSERT_RESOURCE_UNKNOWN_FRAME_ZERO_READ")
	}
}
