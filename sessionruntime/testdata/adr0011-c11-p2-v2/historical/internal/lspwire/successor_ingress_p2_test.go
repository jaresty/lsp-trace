package lspwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type p2Capture struct {
	frames  [][]byte
	results [][]byte
}

func (c *p2Capture) ObserveOriginalFrame(p []byte) {
	c.frames = append(c.frames, append([]byte(nil), p...))
}
func (c *p2Capture) ObserveOriginalResult(p []byte) {
	c.results = append(c.results, append([]byte(nil), p...))
}

func p2Frame(body []byte) []byte { return append(successorHeader(len(body)), body...) }

func TestC09C10CompleteReceivedBodyLimitPerMessage(t *testing.T) {
	for _, tc := range []struct {
		name string
		cap  uint64
	}{
		{"definition", 1048576},
		{"references", 1572864},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, delta := range []int{-1, 0, 1} {
				o := testSuccessorOptions()
				o.Limits.FrameBytes = 2 << 20
				o.Limits.CompleteMessageBytes = tc.cap
				body := []byte(`{"jsonrpc":"2.0","method":"n","params":"` + strings.Repeat("x", int(tc.cap)+delta-42) + `"}`)
				r, err := NewSuccessorIngressReader(bytes.NewReader(p2Frame(body)), o)
				if err != nil {
					t.Fatal(err)
				}
				_, got := r.ReadFrame()
				if delta <= 0 && got != nil {
					t.Fatalf("ASSERT_C0X_RECEIVED_BODY_AT_CAP delta=%d err=%v len=%d", delta, got, len(body))
				}
				if delta <= 0 && r.LastReadObservation() != (SuccessorReadObservation{BodyBytes: uint64(len(body)), MessageBytes: uint64(len(body)), FrameBytes: uint64(len(p2Frame(body))), BodyOffset: uint64(len(successorHeader(len(body))))}) {
					t.Fatalf("ASSERT_C0X_RECEIVED_BODY_OBSERVATION got=%+v", r.LastReadObservation())
				}
				if delta > 0 && !errors.Is(got, ErrCompleteMessageTooLarge) {
					t.Fatalf("ASSERT_C0X_RECEIVED_BODY_OVER_CAP err=%v len=%d", got, len(body))
				}
				if delta > 0 && r.LastReadObservation() != (SuccessorReadObservation{}) {
					t.Fatalf("ASSERT_C0X_REFUSAL_NO_OBSERVATION got=%+v", r.LastReadObservation())
				}
			}
		})
	}
}

func TestC09C10SuccessorMessageEncodedSizeMatchesJSONMarshal(t *testing.T) {
	messages := []Message{
		{JSONRPC: Version, ID: json.RawMessage(`1`), Method: "x<&>\n", Params: json.RawMessage(` {"x": 1} `)},
		{JSONRPC: Version, ID: json.RawMessage(`"id"`), Result: json.RawMessage(`[1, 2]`)},
		{JSONRPC: Version, ID: json.RawMessage(`-7`), Error: &RPCError{Code: -32601, Message: "bad\u2028line", Data: json.RawMessage(`{"a":true}`)}},
	}
	for _, message := range messages {
		encoded, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		if got := successorMessageEncodedSize(message); got != uint64(len(encoded)) {
			t.Fatalf("ASSERT_C0X_EXACT_ENCODED_SIZE got=%d want=%d encoded=%s", got, len(encoded), encoded)
		}
	}
}

func TestC11C12ExactOriginalTopLevelResultToken(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
		ok   bool
	}{
		{"whitespace", ` { "jsonrpc":"2.0", "id":1, "result" :  [ 1, {"x":"y"} ]  } `, `[ 1, {"x":"y"} ]`, true},
		{"escaped", `{"jsonrpc":"2.0","id":1,"result":"a\\n\\u0062"}`, `"a\\n\\u0062"`, true},
		{"null", `{"jsonrpc":"2.0","id":1,"result":null}`, `null`, true},
		{"nested equal text", `{"jsonrpc":"2.0","id":1,"other":{"result":[1]},"result":[1]}`, `[1]`, true},
		{"duplicate", `{"jsonrpc":"2.0","id":1,"result":[],"result":[]}`, ``, false},
		{"missing", `{"jsonrpc":"2.0","id":1}`, ``, false},
		{"malformed", `{"jsonrpc":"2.0","id":1,"result":[}`, ``, false},
		{"trailing", `{"jsonrpc":"2.0","id":1,"result":[]} x`, ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capture := &p2Capture{}
			o := testSuccessorOptions()
			o.Limits.ResultTokenBytes = 524288
			o.Capture = capture
			r, err := NewSuccessorIngressReader(bytes.NewReader(p2Frame([]byte(tc.body))), o)
			if err != nil {
				t.Fatal(err)
			}
			_, got := r.ReadFrame()
			if tc.ok {
				if got != nil || len(capture.results) != 1 || string(capture.results[0]) != tc.want {
					t.Fatalf("ASSERT_C11_C12_EXACT_TOKEN err=%v got=%q", got, capture.results)
				}
			} else if got == nil || len(capture.results) != 0 {
				t.Fatalf("ASSERT_C11_C12_INVALID_WITHHELD err=%v captures=%d", got, len(capture.results))
			}
		})
	}
}

func TestC11C12ExactResultTokenLimitCountBeforeCopy(t *testing.T) {
	for _, n := range []int{524288, 524289} {
		capture := &p2Capture{}
		o := testSuccessorOptions()
		o.Limits.FrameBytes = 2 << 20
		o.Limits.ResultTokenBytes = 524288
		o.Capture = capture
		token := `"` + strings.Repeat("x", n-2) + `"`
		body := []byte(`{"jsonrpc":"2.0","id":1,"result":` + token + `}`)
		r, err := NewSuccessorIngressReader(bytes.NewReader(p2Frame(body)), o)
		if err != nil {
			t.Fatal(err)
		}
		_, got := r.ReadFrame()
		if n == 524288 {
			if got != nil || len(capture.results) != 1 || len(capture.results[0]) != n {
				t.Fatalf("ASSERT_C11_C12_TOKEN_AT_CAP err=%v captures=%d", got, len(capture.results))
			}
		} else if !errors.Is(got, ErrResultTokenTooLarge) || len(capture.results) != 0 {
			t.Fatalf("ASSERT_C11_C12_TOKEN_OVER_CAP err=%v captures=%d", got, len(capture.results))
		}
	}
}
