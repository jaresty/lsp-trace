package lspwire

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestReadWithFrameRetainsExactConsumedFrameNotDecodedReserialization(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"result":null}`
	frames := []string{
		frame(body),
		fmt.Sprintf("content-length: %d\nX-Test: spaced\n\n%s", len(body), body),
	}
	var first Message
	var firstDigest string
	for i, input := range frames {
		r := NewReader(strings.NewReader(input), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024})
		got, observed, err := r.ReadWithFrame()
		legacy, legacyErr := NewReader(strings.NewReader(input), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024}).Read()
		if err != nil || legacyErr != nil || !reflect.DeepEqual(got, legacy) || got.Kind() != KindSuccessResponse {
			t.Fatalf("ASSERT_LSP_READ_FRAME_LEGACY_PARITY: variant=%d err=%v legacyErr=%v got=%+v legacy=%+v", i, err, legacyErr, got, legacy)
		}
		want := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(input)))
		if observed.FrameBytes != int64(len(input)) || observed.FrameSHA256 != want {
			t.Fatalf("ASSERT_LSP_READ_EXACT_FRAME: variant=%d observed=%+v wantBytes=%d wantDigest=%s", i, observed, len(input), want)
		}
		if i > 0 && (!reflect.DeepEqual(got, first) || observed.FrameSHA256 == firstDigest) {
			t.Fatalf("ASSERT_LSP_READ_FRAME_DISTINCT_FRAMING: sameMessage=%v sameDigest=%v", reflect.DeepEqual(got, first), observed.FrameSHA256 == firstDigest)
		}
		first, firstDigest = got, observed.FrameSHA256
		t.Logf("ASSERT_LSP_READ_EXACT_FRAME: PASS variant=%d", i)
	}
}

func TestReadWithExactFrameReturnsBoundedExactBytes(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"result":null}`
	frames := []string{
		frame(body),
		fmt.Sprintf("content-length: %d\nX-Test: spaced\n\n%s", len(body), body),
	}
	r := NewReader(strings.NewReader(strings.Join(frames, "")), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024})
	for i, input := range frames {
		got, observed, raw, err := r.ReadWithExactFrame(int64(len(input)))
		if err != nil || got.Kind() != KindSuccessResponse || !bytes.Equal(raw, []byte(input)) || observed.FrameBytes != int64(len(input)) || observed.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(input))) {
			t.Fatalf("ASSERT_ADR0011_EXACT_FRAME_SUCCESS: variant=%d err=%v observed=%+v bytesEqual=%v", i, err, observed, bytes.Equal(raw, []byte(input)))
		}
		raw[0] ^= 1 // A returned slice cannot modify a later read's retained frame.
		t.Logf("ASSERT_ADR0011_EXACT_FRAME_SUCCESS: PASS variant=%d", i)
	}
}

func TestReadWithExactFrameCapAndPreflight(t *testing.T) {
	input := frame(`{"jsonrpc":"2.0","id":1,"result":null}`)
	for _, invalid := range []int64{0, -1} {
		r := NewReader(strings.NewReader(input), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024})
		got, observed, raw, err := r.ReadWithExactFrame(invalid)
		if !errors.Is(err, ErrInvalidFrameCaptureLimit) || !reflect.DeepEqual(got, Message{}) || observed != (ReadFrameObservation{}) || raw != nil {
			t.Fatalf("ASSERT_ADR0011_EXACT_FRAME_PREFLIGHT: cap=%d err=%v observed=%+v raw=%q", invalid, err, observed, raw)
		}
		_, _, raw, err = r.ReadWithExactFrame(int64(len(input)))
		if err != nil || !bytes.Equal(raw, []byte(input)) {
			t.Fatalf("ASSERT_ADR0011_EXACT_FRAME_PREFLIGHT: invalid cap consumed the frame: %v", err)
		}
		t.Log("ASSERT_ADR0011_EXACT_FRAME_PREFLIGHT: PASS")
	}
}

func TestReadWithExactFrameCapWithholdsOutput(t *testing.T) {
	input := frame(`{"jsonrpc":"2.0","id":1,"result":null}`)
	for _, limit := range []int64{int64(len(input)) - 1, 1} {
		got, observed, raw, err := NewReader(strings.NewReader(input), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024}).ReadWithExactFrame(limit)
		if !errors.Is(err, ErrFrameCaptureLimit) || !reflect.DeepEqual(got, Message{}) || observed != (ReadFrameObservation{}) || raw != nil {
			t.Fatalf("ASSERT_ADR0011_EXACT_FRAME_BOUND: cap=%d err=%v observed=%+v raw=%q", limit, err, observed, raw)
		}
		t.Log("ASSERT_ADR0011_EXACT_FRAME_BOUND: PASS")
	}
}

func TestReadWithExactFrameWithholdsFailedReads(t *testing.T) {
	for _, input := range []string{frame(`{"jsonrpc":`), "Content-Length: 50\r\n\r\n{}"} {
		got, observed, raw, err := NewReader(strings.NewReader(input), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024}).ReadWithExactFrame(1024)
		if err == nil || !reflect.DeepEqual(got, Message{}) || observed != (ReadFrameObservation{}) || raw != nil {
			t.Fatalf("ASSERT_ADR0011_EXACT_FRAME_FAILED_READ: err=%v observed=%+v raw=%q", err, observed, raw)
		}
		t.Log("ASSERT_ADR0011_EXACT_FRAME_FAILED_READ: PASS")
	}
}

func TestReadWithFrameIfWithinPreservesOverCapMessageAndNextFrame(t *testing.T) {
	notification := `{"jsonrpc":"2.0","method":"$/progress","params":{"token":"` + strings.Repeat("x", 160) + `"}}`
	first := fmt.Sprintf("content-length: %d\nX-Notification: yes\n\n%s", len(notification), notification)
	response := `{"jsonrpc":"2.0","id":1,"result":null}`
	second := fmt.Sprintf("content-length: %d\nX-Response: yes\n\n%s", len(response), response)
	capBytes := int64(len(second))
	if len(first) <= len(second) {
		t.Fatal("fixture: first must exceed the selected capture cap")
	}
	r := NewReader(strings.NewReader(first+second), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024})
	msg, observed, raw, retained, err := r.ReadWithFrameIfWithin(capBytes)
	if err != nil || msg.Kind() != KindNotification || retained || raw != nil || observed.FrameBytes != int64(len(first)) || observed.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(first))) {
		t.Fatalf("ASSERT_ADR0011_DEFERRED_UNSELECTED_MESSAGE: err=%v kind=%v observed=%+v retained=%v raw=%q", err, msg.Kind(), observed, retained, raw)
	}
	t.Log("ASSERT_ADR0011_DEFERRED_UNSELECTED_MESSAGE: PASS")
	msg, observed, raw, retained, err = r.ReadWithFrameIfWithin(capBytes)
	if err != nil || msg.Kind() != KindSuccessResponse || !retained || !bytes.Equal(raw, []byte(second)) || observed.FrameBytes != int64(len(second)) || observed.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(second))) {
		t.Fatalf("ASSERT_ADR0011_DEFERRED_SELECTED_FRAME: err=%v kind=%v observed=%+v retained=%v equal=%v", err, msg.Kind(), observed, retained, bytes.Equal(raw, []byte(second)))
	}
	t.Log("ASSERT_ADR0011_DEFERRED_SELECTED_FRAME: PASS")
}

func TestReadWithFrameIfWithinPreflightAndMalformedWithhold(t *testing.T) {
	input := frame(`{"jsonrpc":"2.0","id":1,"result":null}`)
	r := NewReader(strings.NewReader(input), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024})
	msg, observed, raw, retained, err := r.ReadWithFrameIfWithin(0)
	if !errors.Is(err, ErrInvalidFrameCaptureLimit) || !reflect.DeepEqual(msg, Message{}) || observed != (ReadFrameObservation{}) || raw != nil || retained {
		t.Fatalf("ASSERT_ADR0011_DEFERRED_PREFLIGHT: err=%v observed=%+v retained=%v raw=%q", err, observed, retained, raw)
	}
	msg, _, raw, retained, err = r.ReadWithFrameIfWithin(int64(len(input)))
	if err != nil || msg.Kind() != KindSuccessResponse || !retained || !bytes.Equal(raw, []byte(input)) {
		t.Fatal("ASSERT_ADR0011_DEFERRED_PREFLIGHT: invalid cap consumed the frame")
	}
	t.Log("ASSERT_ADR0011_DEFERRED_PREFLIGHT: PASS")
	for _, invalid := range []string{frame(`{"jsonrpc":`), "Content-Length: 50\r\n\r\n{}"} {
		msg, observed, raw, retained, err := NewReader(strings.NewReader(invalid), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024}).ReadWithFrameIfWithin(1024)
		if err == nil || !reflect.DeepEqual(msg, Message{}) || observed != (ReadFrameObservation{}) || raw != nil || retained {
			t.Fatalf("ASSERT_ADR0011_DEFERRED_FAILED_READ: err=%v observed=%+v retained=%v raw=%q", err, observed, retained, raw)
		}
		t.Log("ASSERT_ADR0011_DEFERRED_FAILED_READ: PASS")
	}
}

func TestReadWithFrameWithholdsIncompleteOrInvalidFrames(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"bad-header", "Content-Length: nope\r\n\r\n{}"},
		{"short-body", "Content-Length: 50\r\n\r\n{}"},
		{"malformed-json", frame(`{"jsonrpc":`)},
		{"wrong-version", frame(`{"jsonrpc":"1.0","id":1,"result":null}`)},
		{"invalid-message", frame(`{"jsonrpc":"2.0","id":1}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, observed, err := NewReader(strings.NewReader(tc.input), Limits{MaxBodyBytes: 1024, MaxHeaderBytes: 1024}).ReadWithFrame()
			if err == nil || observed != (ReadFrameObservation{}) {
				t.Fatalf("ASSERT_LSP_READ_NO_FAILED_FRAME: case=%s err=%v observation=%+v", tc.name, err, observed)
			}
			t.Log("ASSERT_LSP_READ_NO_FAILED_FRAME: PASS")
		})
	}
}
