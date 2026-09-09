// Command fake-lsp is a non-production deterministic LSP fixture.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"lsp-trace/internal/lspwire"
)

const (
	maxStderrBytes        = 4096
	fixtureCrashCode      = 86
	fixtureInputErrorCode = 2
	methodNotFoundCode    = -32601
)

var fixtureLimits = lspwire.Limits{MaxBodyBytes: 1 << 20, MaxHeaderBytes: 8 << 10}

type cappedWriter struct {
	w         io.Writer
	remaining int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	original := len(p)
	if len(p) > w.remaining {
		p = p[:w.remaining]
	}
	if len(p) > 0 {
		if _, err := w.w.Write(p); err != nil {
			return 0, err
		}
		w.remaining -= len(p)
	}
	return original, nil
}

type resultParams struct {
	Result json.RawMessage `json:"result"`
}
type barrierParams struct {
	Label string `json:"label"`
}
type lateReplyParams struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
}
type stderrParams struct {
	Text string `json:"text"`
}

func response(id, result json.RawMessage) lspwire.Message {
	if result == nil {
		result = json.RawMessage(`null`)
	}
	return lspwire.Message{JSONRPC: lspwire.Version, ID: id, Result: result}
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	errout := &cappedWriter{w: stderr, remaining: maxStderrBytes}
	if socket := os.Getenv("LSP_TRACE_FAKE_LSP_SCHEDULE_BARRIER"); socket != "" {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			fmt.Fprintf(errout, "fake-lsp scheduling barrier: %v\n", err)
			return fixtureInputErrorCode
		}
		if _, err := conn.Write([]byte("scheduled\n")); err != nil {
			_ = conn.Close()
			fmt.Fprintf(errout, "fake-lsp scheduling barrier: %v\n", err)
			return fixtureInputErrorCode
		}
		var release [1]byte
		if _, err := io.ReadFull(conn, release[:]); err != nil || release[0] != 1 {
			_ = conn.Close()
			fmt.Fprintf(errout, "fake-lsp scheduling barrier release: %v\n", err)
			return fixtureInputErrorCode
		}
		_ = conn.Close()
	}
	trace := func(event string) {
		if path := os.Getenv("LSP_TRACE_FAKE_LSP_TRACE"); path != "" {
			if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
				_, _ = fmt.Fprintln(f, event)
				_ = f.Close()
			}
		}
	}
	trace("spawn")
	if marker := os.Getenv("LSP_TRACE_FAKE_LSP_SCHEDULED"); marker != "" {
		if err := os.WriteFile(marker, []byte("scheduled\n"), 0o600); err != nil {
			fmt.Fprintf(errout, "fake-lsp scheduling marker: %v\n", err)
			return fixtureInputErrorCode
		}
	}
	r := lspwire.NewReader(stdin, fixtureLimits)
	w := lspwire.NewWriter(stdout, fixtureLimits)
	hanging := map[string]json.RawMessage{}
	documentURI := "file:///fixture/main.go"
	for {
		m, err := r.Read()
		if errors.Is(err, io.EOF) {
			return 0
		}
		if err != nil {
			fmt.Fprintf(errout, "fake-lsp fixture input error: %v\n", err)
			return fixtureInputErrorCode
		}
		if m.Method != "" {
			trace(m.Method)
		}
		switch m.Method {
		case "initialize":
			result := json.RawMessage(`{"capabilities":{"positionEncoding":"utf-16","callHierarchyProvider":true,"documentSymbolProvider":true},"serverInfo":{"name":"fake-lsp-fixture","version":"1"}}`)
			if err := w.Write(response(m.ID, result)); err != nil {
				fmt.Fprintln(errout, err)
				return fixtureInputErrorCode
			}
		case "shutdown":
			if err := w.Write(response(m.ID, nil)); err != nil {
				fmt.Fprintln(errout, err)
				return fixtureInputErrorCode
			}
		case "exit":
			return 0
		case "textDocument/didOpen":
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			if json.Unmarshal(m.Params, &p) == nil && p.TextDocument.URI != "" {
				documentURI = p.TextDocument.URI
			}
		case "textDocument/documentSymbol":
			mode := os.Getenv("LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL")
			if mode == "hang" {
				hanging[string(m.ID)] = append(json.RawMessage(nil), m.ID...)
				continue
			}
			rng := `{"start":{"line":0,"character":0},"end":{"line":0,"character":4}}`
			name := "leaf"
			if mode == "mismatch" {
				name = "other"
			}
			result := json.RawMessage(fmt.Sprintf(`[{"name":%q,"kind":12,"range":%s,"selectionRange":%s}]`, name, rng, rng))
			if mode == "hierarchical" {
				result = json.RawMessage(`[{"name":"document","kind":2,"range":{"start":{"line":0,"character":0},"end":{"line":20,"character":0}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"children":[{"name":"leaf","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}}},{"name":"peer","kind":12,"range":{"start":{"line":2,"character":0},"end":{"line":2,"character":4}},"selectionRange":{"start":{"line":2,"character":0},"end":{"line":2,"character":4}}},{"name":"Nested","kind":5,"range":{"start":{"line":4,"character":0},"end":{"line":10,"character":0}},"selectionRange":{"start":{"line":4,"character":0},"end":{"line":4,"character":6}},"children":[{"name":"hidden","kind":6,"range":{"start":{"line":5,"character":0},"end":{"line":5,"character":6}},"selectionRange":{"start":{"line":5,"character":0},"end":{"line":5,"character":6}}}]}]}]`)
			}
			if mode == "invalid" {
				result = json.RawMessage(`[{"name":null}]`)
			}
			if err := w.Write(response(m.ID, result)); err != nil {
				fmt.Fprintln(errout, err)
				return fixtureInputErrorCode
			}
		case "textDocument/prepareCallHierarchy":
			if os.Getenv("LSP_TRACE_FAKE_LSP_HANG_PREPARE") == "1" {
				hanging[string(m.ID)] = append(json.RawMessage(nil), m.ID...)
				continue
			}
			uri, _ := json.Marshal(documentURI)
			name, line := "leaf", 0
			if os.Getenv("LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL") == "hierarchical" && bytes.Contains(m.Params, []byte(`"line":2`)) {
				name, line = "peer", 2
			}
			result := json.RawMessage(fmt.Sprintf(`[{"name":%q,"kind":12,"uri":%s,"range":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":4}},"selectionRange":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":4}},"data":{"fixture":%q}}]`, name, uri, line, line, line, line, name))
			if err := w.Write(response(m.ID, result)); err != nil {
				fmt.Fprintln(errout, err)
				return fixtureInputErrorCode
			}
		case "callHierarchy/outgoingCalls":
			if err := w.Write(response(m.ID, json.RawMessage(`[]`))); err != nil {
				fmt.Fprintln(errout, err)
				return fixtureInputErrorCode
			}
		case "callHierarchy/incomingCalls":
			var p struct {
				Item struct {
					Name string `json:"name"`
				} `json:"item"`
			}
			_ = json.Unmarshal(m.Params, &p)
			result := json.RawMessage(`[]`)
			if p.Item.Name == "leaf" {
				uri, _ := json.Marshal(documentURI)
				result = json.RawMessage(fmt.Sprintf(`[{"from":{"name":"caller","kind":12,"uri":%s,"range":{"start":{"line":2,"character":0},"end":{"line":2,"character":6}},"selectionRange":{"start":{"line":2,"character":0},"end":{"line":2,"character":6}},"data":{"fixture":"caller"}},"fromRanges":[{"start":{"line":2,"character":1},"end":{"line":2,"character":2}}]}]`, uri))
			}
			if err := w.Write(response(m.ID, result)); err != nil {
				fmt.Fprintln(errout, err)
				return fixtureInputErrorCode
			}
		case "fixture/reply":
			var p resultParams
			if json.Unmarshal(m.Params, &p) != nil {
				p.Result = json.RawMessage(`null`)
			}
			if err := w.Write(response(m.ID, p.Result)); err != nil {
				fmt.Fprintln(errout, err)
				return fixtureInputErrorCode
			}
		case "fixture/barrier":
			var p barrierParams
			_ = json.Unmarshal(m.Params, &p)
			result, _ := json.Marshal(struct {
				Barrier string `json:"barrier"`
			}{p.Label})
			if err := w.Write(response(m.ID, result)); err != nil {
				fmt.Fprintln(errout, err)
				return fixtureInputErrorCode
			}
		case "fixture/hang":
			hanging[string(m.ID)] = append(json.RawMessage(nil), m.ID...)
		case "$/cancelRequest":
			var p struct {
				ID json.RawMessage `json:"id"`
			}
			if json.Unmarshal(m.Params, &p) == nil {
				params, _ := json.Marshal(struct {
					ID json.RawMessage `json:"id"`
				}{p.ID})
				_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "fixture/cancelObserved", Params: params})
			}
		case "fixture/lateReply":
			var p lateReplyParams
			if json.Unmarshal(m.Params, &p) == nil {
				if id, ok := hanging[string(p.ID)]; ok {
					_ = w.Write(response(id, p.Result))
					delete(hanging, string(p.ID))
				}
			}
		case "fixture/malformed":
			_, _ = io.WriteString(stdout, "Content-Length: nope\r\n\r\n{}")
		case "fixture/stderr":
			var p stderrParams
			_ = json.Unmarshal(m.Params, &p)
			_, _ = io.WriteString(errout, p.Text)
		case "fixture/crash":
			return fixtureCrashCode
		default:
			if len(m.ID) > 0 {
				_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Error: &lspwire.RPCError{Code: methodNotFoundCode, Message: "fixture method not found"}})
			}
		}
	}
}

func main() { os.Exit(run(os.Stdin, os.Stdout, os.Stderr)) }
