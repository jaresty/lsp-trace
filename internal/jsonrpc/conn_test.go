package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func writeTestMessage(t *testing.T, w net.Conn, m message) {
	t.Helper()
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		t.Fatal(err)
	}
}

func TestConnCorrelatesOutOfOrderResponses(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	conn := New(client, client)

	go func() {
		r := bufio.NewReader(server)
		first, _ := readMessage(r)
		second, _ := readMessage(r)
		secondResult, _ := json.Marshal(second.Method)
		firstResult, _ := json.Marshal(first.Method)
		writeTestMessage(t, server, message{JSONRPC: "2.0", ID: second.ID, Result: secondResult})
		writeTestMessage(t, server, message{JSONRPC: "2.0", ID: first.ID, Result: firstResult})
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	results := make([]string, 2)
	for i, method := range []string{"first", "second"} {
		i, method := i, method
		go func() {
			defer wg.Done()
			if err := conn.Call(context.Background(), method, nil, &results[i]); err != nil {
				t.Errorf("Call: %v", err)
			}
		}()
	}
	wg.Wait()
	if results[0] != "first" || results[1] != "second" {
		t.Fatalf("results = %#v", results)
	}
}

func TestCallTimeoutRemovesPendingRequest(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	conn := New(client, client)
	go func() { _, _ = readMessage(bufio.NewReader(server)) }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := conn.Call(ctx, "slow", nil, nil); err != context.DeadlineExceeded {
		t.Fatalf("error = %v", err)
	}
	conn.stateMu.Lock()
	pending := len(conn.pending)
	conn.stateMu.Unlock()
	if pending != 0 {
		t.Fatalf("pending requests = %d", pending)
	}
}

func TestServerRequestGetsMethodNotFoundResponse(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	_ = New(client, client)
	writeTestMessage(t, server, message{JSONRPC: "2.0", ID: json.RawMessage(`"server-1"`), Method: "workspace/configuration", Params: json.RawMessage(`{}`)})
	response, err := readMessage(bufio.NewReader(server))
	if err != nil {
		t.Fatal(err)
	}
	if string(response.ID) != `"server-1"` || response.Error == nil || response.Error.Code != -32601 {
		t.Fatalf("response = %#v", response)
	}
}

func TestTraceRecordsOrderedTraffic(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	var mu sync.Mutex
	var events []TraceEvent
	conn := NewWithTrace(client, client, func(e TraceEvent) { mu.Lock(); events = append(events, e); mu.Unlock() })
	go func() {
		r := bufio.NewReader(server)
		req, _ := readMessage(r)
		writeTestMessage(t, server, message{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`null`)})
	}()
	if err := conn.Call(context.Background(), "shutdown", nil, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 || events[0].Sequence != 1 || events[0].Direction != "send" || events[1].Sequence != 2 || events[1].Direction != "receive" {
		t.Fatalf("events = %#v", events)
	}
}

func TestReadMessageRejectsInvalidContentLength(t *testing.T) {
	for _, input := range []string{
		"X: y\r\n\r\n{}",
		"Content-Length: -1\r\n\r\n",
		"Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}",
		"Content-Length: 16777217\r\n\r\n",
	} {
		if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil {
			t.Errorf("readMessage(%q) succeeded", input)
		}
	}
}
