package graphprovenance

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
)

const MaxCensusBytes = 8 << 20
const MaxBindings = 50000

// Census derives every binding pointer from the validated graph, never from an
// envelope's submitted IDs. Pointers address the decoded exact graph bytes.
// Node references in redundant native seed/locator structures are also covered.
func Census(raw []byte) ([]Binding, error) {
	if len(raw) > MaxGraphBytes {
		return nil, errors.New("graph byte limit exceeded")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return nil, err
	}
	if _, err := schema.Validate(raw, "v3"); err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	for _, key := range []string{"sibling_candidates", "dispatch_relationships"} {
		if values, ok := doc[key].([]any); ok && len(values) > 0 {
			return nil, errors.New("provenance supports managed CALLS-only slice")
		}
	}
	slice, ok := doc["slice"].(map[string]any)
	if !ok || slice["start_mode"] != "at" || slice["preparation"] != nil {
		return nil, errors.New("provenance requires managed at slice")
	}
	var nodes struct {
		Nodes []graph.Node `json:"nodes"`
	}
	_ = json.Unmarshal(raw, &nodes)
	nodeURI := map[string]string{}
	for _, n := range nodes.Nodes {
		nodeURI[n.ID] = n.URI
	}
	out := []Binding{}
	var censusErr error
	censusBytes := 0
	add := func(pointer, uri string) {
		if censusErr != nil {
			return
		}
		censusBytes += len(pointer) + len(uri) + 128
		if len(out) >= MaxBindings || censusBytes > MaxCensusBytes {
			censusErr = errors.New("graph source census budget exceeded")
			return
		}
		attribution := "SOURCE"
		if uri == "" {
			attribution = "NON_SOURCE"
		}
		out = append(out, Binding{Pointer: pointer, URI: uri, Attribution: attribution, ReceiptIDs: []string{}})
	}
	escape := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
	var walk func(any, string)
	walk = func(value any, pointer string) {
		if censusErr != nil {
			return
		}
		switch v := value.(type) {
		case map[string]any:
			if strings.HasPrefix(pointer, "/diagnostics/") && strings.Count(pointer, "/") == 2 {
				id, _ := v["node_id"].(string)
				add(pointer, nodeURI[id])
			}
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				// LSP item data is opaque server state, not a source locator contract.
				if k == "data" {
					continue
				}
				p := pointer + "/" + escape(k)
				if text, ok := v[k].(string); ok {
					switch {
					case k == "uri" || k == "resolved_uri" || k == "source_uri":
						add(p, text)
					case k == "node_id" || strings.HasSuffix(k, "_node_id"):
						add(p, nodeURI[text])
					case k == "id" && nodeURI[text] != "":
						add(p, nodeURI[text])
					}
				}
				if ids, ok := v[k].([]any); ok && (strings.HasSuffix(k, "_node_ids") || k == "prepared_target_ids" || k == "targets") {
					for i, id := range ids {
						text, _ := id.(string)
						add(fmt.Sprintf("%s/%d", p, i), nodeURI[text])
					}
				}
				if sites, ok := v[k].([]any); ok && k == "call_sites" {
					id, _ := v["caller_node_id"].(string)
					for i := range sites {
						add(fmt.Sprintf("%s/%d", p, i), nodeURI[id])
					}
				}
				if (k == "range" || k == "selection_range") && v["uri"] != nil {
					uri, _ := v["uri"].(string)
					add(p, uri)
				}
				walk(v[k], p)
			}
		case []any:
			for i, x := range v {
				walk(x, fmt.Sprintf("%s/%d", pointer, i))
			}
		}
	}
	walk(doc, "")
	if censusErr != nil {
		return nil, censusErr
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pointer < out[j].Pointer })
	for i := 1; i < len(out); i++ {
		if out[i].Pointer == out[i-1].Pointer {
			return nil, errors.New("duplicate census pointer")
		}
	}
	return out, nil
}
