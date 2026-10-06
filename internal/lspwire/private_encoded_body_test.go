package lspwire

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteEncodedBodyPrivateMatchesPublicWriter(t *testing.T) {
	message := Message{JSONRPC: Version, ID: json.RawMessage(`17`), Method: "textDocument/definition", Params: json.RawMessage(`{"textDocument":{"uri":"file:///tmp/a.go"},"position":{"line":1,"character":2}}`)}
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	var public, private bytes.Buffer
	if err := NewWriter(&public, DefaultLimits()).Write(message); err != nil {
		t.Fatal(err)
	}
	if err := NewWriter(&private, DefaultLimits()).WriteEncodedBodyPrivate(body); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(private.Bytes(), public.Bytes()) {
		t.Fatalf("ASSERT_PRIVATE_ENCODED_WRITER_PARITY\nprivate=%q\npublic=%q", private.Bytes(), public.Bytes())
	}
}

func TestWriteEncodedBodyPrivatePreservesObserverStages(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":null}`)
	var out bytes.Buffer
	var events []Event
	writer := NewWriterObserved(&out, DefaultLimits(), func(event Event) { events = append(events, event) })
	if err := writer.WriteEncodedBodyPrivate(body); err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[0].Stage != EventMarshal || events[1].Stage != EventFrame || events[2].Stage != EventBodyWrite || events[3].Stage != EventFlush {
		t.Fatalf("ASSERT_PRIVATE_ENCODED_WRITER_EVENTS events=%+v", events)
	}
	if events[0].Bytes != int64(len(body)) || events[2].Bytes != int64(len(body)) {
		t.Fatalf("ASSERT_PRIVATE_ENCODED_WRITER_EVENT_BYTES events=%+v", events)
	}
}
