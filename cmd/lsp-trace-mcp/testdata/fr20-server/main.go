// Deterministic, test-only FR20 protocol fixture. No source reads or writes.
package main

import (
	"encoding/json"
	"lsp-trace/internal/lspwire"
	"net/url"
	"os"
	"path"
	"strings"
)

func item(uri string) map[string]any {
	name := strings.TrimSuffix(path.Base(uri), ".go")
	r := map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 8}}
	return map[string]any{"name": name, "kind": 12, "uri": uri, "range": r, "selectionRange": r, "data": map[string]any{"exact": 9007199254740993}}
}
func main() {
	limits := lspwire.Limits{MaxBodyBytes: 1 << 20, MaxHeaderBytes: 8 << 10}
	r, w := lspwire.NewReader(os.Stdin, limits), lspwire.NewWriter(os.Stdout, limits)
	for {
		m, err := r.Read()
		if err != nil {
			return
		}
		if m.Method == "exit" {
			return
		}
		if len(m.ID) == 0 {
			continue
		}
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Item struct {
				URI string `json:"uri"`
			} `json:"item"`
		}
		_ = json.Unmarshal(m.Params, &p)
		var result any
		switch m.Method {
		case "initialize":
			result = map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "callHierarchyProvider": true}}
		case "textDocument/documentSymbol":
			n := item(p.TextDocument.URI)
			delete(n, "uri")
			delete(n, "data")
			result = []any{n}
		case "textDocument/prepareCallHierarchy":
			switch path.Base(p.TextDocument.URI) {
			case "missing.go":
				result = []any{}
			case "ambiguous.go":
				a, b := item(p.TextDocument.URI), item(p.TextDocument.URI)
				b["detail"] = "other identity"
				result = []any{a, b}
			default:
				result = []any{item(p.TextDocument.URI)}
			}
		case "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
			u, _ := url.Parse(p.Item.URI)
			name := path.Base(u.Path)
			if name == "failed.go" {
				_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Error: &lspwire.RPCError{Code: -32601, Message: "fixture unsupported"}})
				continue
			}
			neighbor := ""
			if m.Method == "callHierarchy/outgoingCalls" {
				switch name {
				case "a.go":
					neighbor = "b.go"
				case "b.go":
					neighbor = "c.go"
				}
			} else {
				switch name {
				case "c.go":
					neighbor = "b.go"
				case "b.go":
					neighbor = "a.go"
				}
			}
			result = []any{}
			if neighbor != "" {
				u.Path = path.Join(path.Dir(u.Path), neighbor)
				key := "to"
				if m.Method == "callHierarchy/incomingCalls" {
					key = "from"
				}
				result = []any{map[string]any{key: item(u.String()), "fromRanges": []any{map[string]any{"start": map[string]int{"line": 0, "character": 1}, "end": map[string]int{"line": 0, "character": 2}}}}}
			}
		}
		raw, _ := json.Marshal(result)
		if err := w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw}); err != nil {
			return
		}
	}
}
