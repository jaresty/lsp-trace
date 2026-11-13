//go:build darwin || linux

package inventorysuccessor_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

// Independent framed SELF, never a model/network/production server. Knobs are
// response fixtures, not native admissions. Native classification is checked
// separately from these provider-emission observations.
type configurationFixtureV3 struct {
	Incoming, Outgoing, References, Definitions string
}

func TestConfigurationV3SELFResponder(t *testing.T) {
	at := -1
	for i, a := range os.Args {
		if a == "configuration-self-v3" {
			at = i
			break
		}
	}
	if at < 0 {
		return
	}
	root := os.Args[at+1]
	var knobs configurationFixtureV3
	if json.Unmarshal([]byte(os.Args[at+2]), &knobs) != nil {
		os.Exit(71)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "root.go")}).String()
	pos := func(line, start, end uint32) lsp.Range {
		return lsp.Range{Start: lsp.Position{Line: line, Character: start}, End: lsp.Position{Line: line, Character: end}}
	}
	item := func(name string, line, end uint32) lsp.CallHierarchyItem {
		return lsp.CallHierarchyItem{Name: name, URI: uri, Kind: 12, Range: pos(line, 0, 22), SelectionRange: pos(line, 5, end)}
	}
	query, other, third := item("Root", 1, 9), item("Other", 2, 10), item("Third", 3, 10)
	reader := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	for {
		msg, e := reader.Read()
		if e != nil || msg.Method == "exit" {
			os.Exit(0)
		}
		if len(msg.ID) == 0 {
			continue
		}
		var result any
		state := "CONTROL"
		count := 0
		switch msg.Method {
		case "initialize":
			result = map[string]any{"serverInfo": map[string]string{"name": "configuration-controlled-self", "version": "v3"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "callHierarchyProvider": knobs.Incoming != "unavailable" && knobs.Outgoing != "unavailable", "referencesProvider": knobs.References != "unavailable", "definitionProvider": knobs.Definitions != "unavailable"}}
		case "textDocument/documentSymbol":
			result = []any{map[string]any{"name": query.Name, "kind": 12, "range": query.Range, "selectionRange": query.SelectionRange}, map[string]any{"name": other.Name, "kind": 12, "range": other.Range, "selectionRange": other.SelectionRange}, map[string]any{"name": third.Name, "kind": 12, "range": third.Range, "selectionRange": third.SelectionRange}}
		case "textDocument/references", "textDocument/definition":
			state = knobs.References
			if msg.Method == "textDocument/definition" {
				state = knobs.Definitions
			}
			locations := []any{}
			if state != "empty" && state != "unavailable" {
				locations = append(locations, map[string]any{"uri": uri, "range": other.SelectionRange})
			}
			if state == "multiple" {
				locations = append(locations, map[string]any{"uri": uri, "range": third.SelectionRange})
			}
			if state == "duplicates" {
				locations = append(locations, map[string]any{"uri": uri, "range": other.SelectionRange})
			}
			count = len(locations)
			result = locations
		case "textDocument/prepareCallHierarchy":
			result = []lsp.CallHierarchyItem{query}
		case "callHierarchy/incomingCalls":
			state = knobs.Incoming
			calls := []lsp.CallHierarchyIncomingCall{}
			if state != "empty" && state != "unavailable" {
				sites := []lsp.Range{pos(2, 14, 18)}
				if state == "duplicates" {
					sites = append(sites, sites[0])
				}
				calls = append(calls, lsp.CallHierarchyIncomingCall{From: other, FromRanges: sites})
			}
			count = len(calls)
			result = calls
		case "callHierarchy/outgoingCalls":
			state = knobs.Outgoing
			calls := []lsp.CallHierarchyOutgoingCall{}
			if state != "empty" && state != "unavailable" {
				sites := []lsp.Range{pos(1, 13, 18)}
				if state == "duplicates" {
					sites = append(sites, sites[0])
				}
				calls = append(calls, lsp.CallHierarchyOutgoingCall{To: other, FromRanges: sites})
			}
			count = len(calls)
			result = calls
		}
		if len(os.Args) > at+3 {
			event, _ := json.Marshal(map[string]any{"method": msg.Method, "provider_response_state": state, "items": count})
			f, err := os.OpenFile(os.Args[at+3], os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
			if err != nil {
				os.Exit(73)
			}
			_, err = f.Write(append(event, '\n'))
			err2 := f.Close()
			if err != nil || err2 != nil {
				os.Exit(74)
			}
		}
		raw, _ := json.Marshal(result)
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, raw)
		if _, e = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body); e != nil {
			os.Exit(72)
		}
	}
}
