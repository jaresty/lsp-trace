package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"lsp-trace/internal/jsonrpc"
)

type peerMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func readPeer(t *testing.T, r *bufio.Reader) peerMessage {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if _, err := fmt.Sscanf(line, "Content-Length: %d\r\n", &n); err != nil {
		t.Fatal(err)
	}
	if line, err = r.ReadString('\n'); err != nil || line != "\r\n" {
		t.Fatalf("separator %q, %v", line, err)
	}
	body := make([]byte, n)
	if _, err := r.Read(body); err != nil {
		t.Fatal(err)
	}
	var m peerMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func writePeer(t *testing.T, c net.Conn, m peerMessage) {
	t.Helper()
	b, _ := json.Marshal(m)
	if _, err := fmt.Fprintf(c, "Content-Length: %d\r\n\r\n%s", len(b), b); err != nil {
		t.Fatal(err)
	}
}

func TestSupportsTypeHierarchy(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want bool
	}{{"", false}, {"null", false}, {"false", false}, {"true", true}, {`{"workDoneProgress":true}`, true}} {
		client := &Client{InitializeResult: InitializeResult{Capabilities: ServerCapabilities{TypeHierarchyProvider: json.RawMessage(tc.raw)}}}
		if got := client.SupportsTypeHierarchy(); got != tc.want {
			t.Fatalf("ASSERT_TYPE_HIERARCHY_CAPABILITY_%s: got %v want %v", tc.raw, got, tc.want)
		}
	}
}

func TestClientTypeHierarchyRequests(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := NewClient(jsonrpc.New(clientConn, clientConn))
	item := TypeHierarchyItem{Name: "I", URI: "file:///i", Data: json.RawMessage(`{"opaque":1}`)}
	go func() {
		r := bufio.NewReader(serverConn)
		prepare := readPeer(t, r)
		if prepare.Method != "textDocument/prepareTypeHierarchy" {
			t.Errorf("ASSERT_PREPARE_TYPE_METHOD: %s", prepare.Method)
		}
		writePeer(t, serverConn, peerMessage{JSONRPC: "2.0", ID: prepare.ID, Result: json.RawMessage(`[{"name":"I","kind":11,"uri":"file:///i","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"data":{"opaque":1}}]`)})
		subtypes := readPeer(t, r)
		if subtypes.Method != "typeHierarchy/subtypes" || !bytes.Contains(subtypes.Params, []byte(`"opaque":1`)) {
			t.Errorf("ASSERT_SUBTYPES_EXACT_ITEM: %s %s", subtypes.Method, subtypes.Params)
		}
		writePeer(t, serverConn, peerMessage{JSONRPC: "2.0", ID: subtypes.ID, Result: json.RawMessage(`[]`)})
	}()
	items, err := client.PrepareTypeHierarchy(context.Background(), PrepareTypeHierarchyParams{})
	if err != nil || len(items) != 1 {
		t.Fatalf("ASSERT_PREPARE_TYPE_RESULT: %#v %v", items, err)
	}
	children, err := client.Subtypes(context.Background(), item)
	if err != nil || len(children) != 0 {
		t.Fatalf("ASSERT_SUBTYPES_RESULT: %#v %v", children, err)
	}
}

func TestClientNullCallHierarchyResponses(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := NewClient(jsonrpc.New(clientConn, clientConn))
	go func() {
		r := bufio.NewReader(serverConn)
		for range 2 {
			m := readPeer(t, r)
			writePeer(t, serverConn, peerMessage{JSONRPC: "2.0", ID: m.ID, Result: json.RawMessage(`null`)})
		}
	}()
	items, err := client.PrepareCallHierarchy(context.Background(), PrepareCallHierarchyParams{})
	if err != nil || items != nil {
		t.Fatalf("ASSERT_NULL_PREPARE_ACCEPTED: items=%#v err=%v", items, err)
	}
	calls, wasNull, err := client.IncomingCalls(context.Background(), CallHierarchyItem{})
	if err != nil || calls != nil || !wasNull {
		t.Fatalf("ASSERT_NULL_INCOMING_ACCEPTED: calls=%#v wasNull=%v err=%v", calls, wasNull, err)
	}
}

func TestClientLifecycleSequence(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	client := NewClient(jsonrpc.New(clientConn, clientConn))
	methods := make(chan []string, 1)
	go func() {
		r := bufio.NewReader(serverConn)
		var got []string
		m := readPeer(t, r)
		got = append(got, m.Method)
		var initialize struct {
			Params struct {
				Capabilities struct {
					TextDocument struct {
						TypeHierarchy struct {
							DynamicRegistration bool `json:"dynamicRegistration"`
						} `json:"typeHierarchy"`
					} `json:"textDocument"`
				} `json:"capabilities"`
			} `json:"params"`
		}
		body := m.Params
		if err := json.Unmarshal(body, &initialize.Params); err != nil {
			t.Error(err)
		}
		if initialize.Params.Capabilities.TextDocument.TypeHierarchy.DynamicRegistration {
			t.Error("ASSERT_TYPE_HIERARCHY_STATIC_REGISTRATION: dynamic registration must be false")
		}
		if initialize.Params.Capabilities.TextDocument.TypeHierarchy == (struct {
			DynamicRegistration bool `json:"dynamicRegistration"`
		}{}) {
			// A false zero value alone cannot prove the capability was advertised.
			if !bytes.Contains(body, []byte(`"typeHierarchy"`)) {
				t.Error("ASSERT_TYPE_HIERARCHY_ADVERTISED: initialize omitted typeHierarchy")
			}
		}
		writePeer(t, serverConn, peerMessage{JSONRPC: "2.0", ID: m.ID, Result: json.RawMessage(`{"capabilities":{"callHierarchyProvider":true}}`)})
		got = append(got, readPeer(t, r).Method)
		m = readPeer(t, r)
		got = append(got, m.Method)
		writePeer(t, serverConn, peerMessage{JSONRPC: "2.0", ID: m.ID, Result: json.RawMessage(`null`)})
		got = append(got, readPeer(t, r).Method)
		methods <- got
	}()
	if err := client.Initialize(context.Background(), "file:///workspace"); err != nil {
		t.Fatal(err)
	}
	if err := client.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := <-methods
	want := []string{"initialize", "initialized", "shutdown", "exit"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("methods = %#v", got)
		}
	}
}
