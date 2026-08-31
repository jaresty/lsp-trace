package lsp

import (
	"bufio"
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
