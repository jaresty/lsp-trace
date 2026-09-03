package lspwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func frame(body string) string { return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body) }

func TestReaderRejectsMalformedFramesAndBodies(t *testing.T) {
	valid := `{"jsonrpc":"2.0","id":1,"result":{}}`
	tests := []struct {
		name string
		wire string
		max  int64
		want error
	}{
		{"malformed length", "Content-Length: nope\r\n\r\n{}", 1024, ErrInvalidContentLength},
		{"duplicate length", "Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}", 1024, ErrDuplicateContentLength},
		{"missing length", "X-Test: yes\r\n\r\n{}", 1024, ErrMissingContentLength},
		{"oversized", frame(valid), 2, ErrFrameTooLarge},
		{"malformed json", frame(`{"jsonrpc":`), 1024, ErrMalformedJSON},
		{"wrong version", frame(`{"jsonrpc":"1.0","id":1,"result":{}}`), 1024, ErrWrongVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewReader(strings.NewReader(tt.wire), Limits{MaxBodyBytes: tt.max, MaxHeaderBytes: 1024}).Read()
			if !errors.Is(err, tt.want) {
				t.Fatalf("%s: got %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}

func TestWriterUsesBoundedContentLengthFraming(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, Limits{MaxBodyBytes: 1024})
	msg := Message{JSONRPC: Version, Method: "initialized", Params: json.RawMessage(`{}`)}
	if err := w.Write(msg); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`) {
		t.Fatalf("unexpected frame %q", got)
	}
}

func TestClassification(t *testing.T) {
	tests := []struct {
		name, body string
		want       Kind
	}{
		{"request", `{"jsonrpc":"2.0","id":1,"method":"x"}`, KindRequest},
		{"notification", `{"jsonrpc":"2.0","method":"x"}`, KindNotification},
		{"success", `{"jsonrpc":"2.0","id":1,"result":null}`, KindSuccessResponse},
		{"error", `{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"x"}}`, KindErrorResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewReader(strings.NewReader(frame(tt.body)), DefaultLimits()).Read()
			if err != nil {
				t.Fatal(err)
			}
			if got := m.Kind(); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
	for _, body := range []string{
		`{"jsonrpc":"2.0","id":1}`,
		`{"jsonrpc":"2.0","id":1,"method":"x","result":null}`,
		`{"jsonrpc":"2.0","id":1,"result":null,"error":{"code":-1,"message":"x"}}`,
	} {
		if _, err := NewReader(strings.NewReader(frame(body)), DefaultLimits()).Read(); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("ambiguous %s: %v", body, err)
		}
	}
}

func TestGenerationBoundResponsesAndTombstones(t *testing.T) {
	p := NewPending(2)
	a := p.Begin(7)
	b := p.Begin(7)
	if a.ID == b.ID {
		t.Fatal("duplicate IDs")
	}
	if got := p.Accept(ResponseKey{Generation: 7, ID: 999}); got != ResponseUnknown {
		t.Fatalf("unknown: %v", got)
	}
	if got := p.Accept(ResponseKey{Generation: 8, ID: a.ID}); got != ResponseWrongGeneration {
		t.Fatalf("generation: %v", got)
	}
	if got := p.Accept(a); got != ResponseAccepted {
		t.Fatalf("accepted: %v", got)
	}
	if got := p.Accept(a); got != ResponseDuplicate {
		t.Fatalf("duplicate: %v", got)
	}
	if got := p.Accept(b); got != ResponseAccepted {
		t.Fatalf("second: %v", got)
	}
	c := p.Begin(7)
	if got := p.Accept(c); got != ResponseAccepted {
		t.Fatal(got)
	}
	if p.TombstoneCount() != 2 {
		t.Fatalf("tombstones %d", p.TombstoneCount())
	}
	if got := p.Accept(a); got != ResponseUnknown {
		t.Fatalf("evicted tombstone: %v", got)
	}
}

func TestCancellationWritesExactlyOnceWhilePending(t *testing.T) {
	p := NewPending(2)
	key := p.Begin(3)
	var out bytes.Buffer
	w := NewWriter(&out, DefaultLimits())
	if state, err := p.Cancel(w, key); err != nil || state != CancelWritten {
		t.Fatalf("first: %v %v", state, err)
	}
	first := out.String()
	if state, err := p.Cancel(w, key); err != nil || state != CancelAlreadyWritten {
		t.Fatalf("second: %v %v", state, err)
	}
	if out.String() != first {
		t.Fatal("duplicate cancellation write")
	}
	m, err := NewReader(strings.NewReader(first), DefaultLimits()).Read()
	if err != nil {
		t.Fatal(err)
	}
	if m.Method != "$/cancelRequest" || string(m.Params) != fmt.Sprintf(`{"id":%d}`, key.ID) {
		t.Fatalf("cancel payload: %+v", m)
	}
	p.Accept(key)
	if state, err := p.Cancel(w, key); err != nil || state != CancelNotPending {
		t.Fatalf("completed: %v %v", state, err)
	}
}

func TestInitializationPrimitivesAreDeterministic(t *testing.T) {
	cfg := InitializeConfig{ProcessID: 42, RootURI: "file:///workspace", ClientName: "lsp-trace", ClientVersion: "test", Trace: "off"}
	a, err := InitializeRequest(RequestKey{Generation: 9, ID: 4}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := InitializeRequest(RequestKey{Generation: 9, ID: 4}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if !bytes.Equal(aj, bj) {
		t.Fatalf("nondeterministic: %s != %s", aj, bj)
	}
	if a.Method != "initialize" || string(a.ID) != "4" {
		t.Fatalf("request: %+v", a)
	}
	x := InitializedNotification()
	y := InitializedNotification()
	xj, _ := json.Marshal(x)
	yj, _ := json.Marshal(y)
	if !bytes.Equal(xj, yj) || x.Method != "initialized" || string(x.Params) != `{}` {
		t.Fatalf("initialized: %s %s", xj, yj)
	}
}
