// Command fake-lsp is a non-production deterministic LSP fixture.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	r := lspwire.NewReader(stdin, fixtureLimits)
	w := lspwire.NewWriter(stdout, fixtureLimits)
	hanging := map[string]json.RawMessage{}
	for {
		m, err := r.Read()
		if errors.Is(err, io.EOF) {
			return 0
		}
		if err != nil {
			fmt.Fprintf(errout, "fake-lsp fixture input error: %v\n", err)
			return fixtureInputErrorCode
		}
		switch m.Method {
		case "initialize":
			result := json.RawMessage(`{"capabilities":{},"serverInfo":{"name":"fake-lsp-fixture","version":"1"}}`)
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
