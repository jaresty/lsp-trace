package graphprovenance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
)

func TestV2CarrierValidInventoryWidths(t *testing.T) {
	for _, width := range []uint32{0, 1} {
		t.Run(fmt.Sprintf("width=%d", width), func(t *testing.T) {
			base, root := coordinatorV2Fixture(t, nil)
			req := base.Request
			req.Mode = acquisition.Slice
			req.Root.Locator.Line = nil
			req.Root.Locator.Character = nil
			req.Root.Locator.Symbol = "Same"
			line := uint32(5)
			req.RequiredTargets[0].Locator.Line = &line
			uri := req.Root.Locator.URI
			item := func(line uint32) lsp.CallHierarchyItem {
				span := lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: width}}
				return lsp.CallHierarchyItem{Name: "Same", Kind: 12, URI: uri, Range: span, SelectionRange: span, Data: json.RawMessage(`{"uri":"file:///opaque.go","selection_range":{"bad":true}}`)}
			}
			wire := acquisition.NewWireClient(func(_ context.Context, q acquisition.WireRequest) (json.RawMessage, error) {
				switch q.Method {
				case "textDocument/documentSymbol":
					a := item(0)
					return json.Marshal([]lsp.DocumentSymbol{{Name: a.Name, Kind: a.Kind, Range: a.Range, SelectionRange: a.SelectionRange}})
				case "textDocument/prepareCallHierarchy":
					var p lsp.PrepareCallHierarchyParams
					if err := json.Unmarshal(q.Params, &p); err != nil {
						return nil, err
					}
					return json.Marshal([]lsp.CallHierarchyItem{item(p.Position.Line)})
				case "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls":
					var p struct {
						Item lsp.CallHierarchyItem `json:"item"`
					}
					if err := json.Unmarshal(q.Params, &p); err != nil {
						return nil, err
					}
					if q.Method == "callHierarchy/outgoingCalls" && p.Item.Range.Start.Line == 0 {
						return json.Marshal([]lsp.CallHierarchyOutgoingCall{{To: item(5), FromRanges: []lsp.Range{item(0).Range}}})
					}
					if q.Method == "callHierarchy/incomingCalls" && p.Item.Range.Start.Line == 5 {
						return json.Marshal([]lsp.CallHierarchyIncomingCall{{From: item(0), FromRanges: []lsp.Range{item(0).Range}}})
					}
				}
				return json.RawMessage(`[]`), nil
			})
			r, err := acquisition.Acquire(context.Background(), wire, req)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Graph.Nodes) != 2 {
				t.Fatal("two graph nodes required")
			}
			r.Graph.Nodes[0], r.Graph.Nodes[1] = r.Graph.Nodes[1], r.Graph.Nodes[0]
			raw, err := CaptureV2(context.Background(), r, root)
			if err != nil {
				t.Fatal(err)
			}
			var e EvidenceV2
			if err = json.Unmarshal(raw, &e); err != nil {
				t.Fatal(err)
			}
			by := map[string]BindingV2{}
			for _, b := range e.Bindings {
				by[b.Pointer] = b
				if b.AnchorStatus == "INVALID_COORDINATES" {
					t.Fatal("valid full inventory", b)
				}
				if strings.Contains(b.Pointer, "/data/") || b.URI == "file:///opaque.go" {
					t.Fatal("opaque data walked", b)
				}
			}
			expected := []string{}
			var native struct {
				Nodes    []graph.Node            `json:"nodes"`
				Portable []graph.PortableLocator `json:"portable_locators"`
			}
			if err = json.Unmarshal(e.GraphBytes, &native); err != nil {
				t.Fatal(err)
			}
			if len(native.Nodes) != 2 || len(native.Portable) != 2 {
				t.Fatal("two distinct native nodes required")
			}
			for i := range native.Nodes {
				expected = append(expected, fmt.Sprintf("/graph/nodes/%d/range", i), fmt.Sprintf("/graph/nodes/%d/selection_range", i), fmt.Sprintf("/graph/portable_locators/%d/provenance/source/selection_range", i))
			}
			seen := map[string]bool{}
			for i, q := range r.Requests {
				p := fmt.Sprintf("/acquisition/requests/%d", i)
				var rows []json.RawMessage
				if err = json.Unmarshal(q.Response, &rows); err != nil {
					t.Fatal(err)
				}
				if len(rows) == 0 {
					continue
				}
				seen[q.Method] = true
				switch q.Method {
				case "textDocument/documentSymbol", "textDocument/prepareCallHierarchy":
					expected = append(expected, p+"/response/0/range", p+"/response/0/selectionRange")
				case "callHierarchy/outgoingCalls":
					expected = append(expected, p+"/params/item/range", p+"/params/item/selectionRange", p+"/response/0/to/range", p+"/response/0/to/selectionRange", p+"/response/0/fromRanges/0")
				case "callHierarchy/incomingCalls":
					expected = append(expected, p+"/params/item/range", p+"/params/item/selectionRange", p+"/response/0/from/range", p+"/response/0/from/selectionRange", p+"/response/0/fromRanges/0")
				}
			}
			for _, method := range []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls"} {
				if !seen[method] {
					t.Fatal("missing carrier family", method)
				}
			}
			for i := range r.Targets {
				expected = append(expected, fmt.Sprintf("/acquisition/targets/%d/resolution/prepared/range", i), fmt.Sprintf("/acquisition/targets/%d/resolution/prepared/selectionRange", i))
			}
			for i, o := range r.EdgeObservations {
				for j := range o.CallSites {
					expected = append(expected, fmt.Sprintf("/acquisition/edge_observations/%d/call_sites/%d", i, j))
				}
			}
			for _, p := range expected {
				if b := by[p]; b.AnchorStatus != "VALID_COORDINATES" || b.URI != uri || b.Attribution != "SOURCE" {
					t.Fatalf("missing/invalid carrier %s: %+v", p, b)
				}
			}
			// Exact node references, not same-name/same-URI guesses: both nodes have
			// identical names and URIs but disjoint ranges. Native admission rejects
			// missing/wrong references, duplicates, and mismatched containment first.
			for _, state := range []string{"missing", "dangling", "wrong_node", "duplicate_node", "outside"} {
				t.Run(state, func(t *testing.T) {
					var doc map[string]any
					if err := json.Unmarshal(e.GraphBytes, &doc); err != nil {
						t.Fatal(err)
					}
					nodes := doc["nodes"].([]any)
					sources := doc["portable_locators"].([]any)
					source := sources[0].(map[string]any)["provenance"].(map[string]any)["source"].(map[string]any)
					switch state {
					case "missing":
						delete(source, "node_id")
					case "dangling":
						source["node_id"] = "absent-node"
					case "wrong_node":
						for _, n := range nodes {
							id := n.(map[string]any)["id"]
							if id != source["node_id"] {
								source["node_id"] = id
								break
							}
						}
					case "duplicate_node":
						nodes[1] = nodes[0]
					case "outside":
						source["selection_range"] = map[string]any{"start": map[string]any{"line": 99, "character": 0}, "end": map[string]any{"line": 99, "character": width}}
					}
					bad, _ := json.Marshal(doc)
					want := "replay identity mismatch: portable locators"
					if state == "duplicate_node" {
						want = "duplicate canonical node"
					}
					if _, err := graph.DecodeNativeV3(bad); err == nil || !strings.Contains(err.Error(), want) {
						t.Fatal("native ownership guard", state, err)
					}
					altered := e
					altered.GraphBytes = bad
					altered.GraphDigest = digest(VersionV2+":graph", bad)
					if err := ValidateV2(altered); err == nil {
						t.Fatal("invalid native ownership admitted by V2", state)
					}
				})
			}
			if err = os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			if _, err = ValidateFor(raw, Family, "v2"); err != nil {
				t.Fatal("source-free inventory", err)
			}
			t.Logf("%d exact valid range carriers; %d total bindings", len(expected), len(e.Bindings))
		})
	}
}
