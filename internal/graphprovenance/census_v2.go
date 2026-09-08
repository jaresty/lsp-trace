package graphprovenance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"lsp-trace/internal/acquisition"
)

// CensusV2 addresses /graph/... in authoritative GraphBytes and
// /acquisition/... in the complete descriptor. It derives locators from Request,
// never from resolution success, and does not interpret names or free text.
func CensusV2(r acquisition.Result) ([]Binding, error) {
	graphBytes, err := json.Marshal(r.Graph)
	if err != nil {
		return nil, err
	}
	if err = preflightGraphV2(graphBytes); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if err = preflightV2(raw, MaxEnvelopeBytesV2); err != nil {
		return nil, err
	}
	if err = acquisition.ValidateResult(r); err != nil {
		return nil, err
	}
	var doc map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = d.Decode(&doc); err != nil {
		return nil, err
	}
	nodes := map[string]string{}
	for _, n := range r.Graph.Nodes {
		nodes[n.ID] = n.URI
	}
	labels := map[string]string{r.Request.Root.ID: r.Request.Root.Locator.URI}
	for _, t := range r.Request.RequiredTargets {
		labels[t.ID] = t.Locator.URI
	}
	out := []Binding{}
	used := 0
	var failure error
	add := func(pointer, uri string) {
		if failure != nil {
			return
		}
		used += len(pointer) + len(uri) + 128
		if len(out) >= MaxBindings || used > MaxCensusBytes {
			failure = errors.New("V2 census budget exceeded")
			return
		}
		attr := "SOURCE"
		if uri == "" {
			attr = "NON_SOURCE"
		}
		out = append(out, Binding{Pointer: pointer, URI: uri, Attribution: attr, ReceiptIDs: []string{}})
	}
	escape := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
	var walk func(any, string)
	walk = func(value any, p string) {
		if failure != nil {
			return
		}
		switch v := value.(type) {
		case map[string]any:
			text := func(k string) string { s, _ := v[k].(string); return s }
			uri := text("uri")
			if _, exists := v["position"]; exists {
				owner := ""
				for _, key := range []string{"identity", "textDocument"} {
					if object, ok := v[key].(map[string]any); ok {
						owner, _ = object["uri"].(string)
						if owner != "" {
							break
						}
					}
				}
				add(p+"/position", owner)
			}
			if strings.HasPrefix(p, "/graph/diagnostics/") && strings.Count(p, "/") == 3 {
				add(p, nodes[text("node_id")])
			}
			if _, ok := v["at"]; ok {
				add(p+"/at", labels[text("label")])
			}
			if _, ok := v["seed_at"]; ok {
				add(p+"/seed_at", labels[text("seed_label")])
			}
			kind := text("evidence_kind")
			if kind == "PREPARED_TARGET" || kind == "REACHED_NODE" {
				add(p+"/endpoint_id", nodes[text("endpoint_id")])
			} else if _, exists := v["endpoint_id"]; exists {
				add(p+"/endpoint_id", "")
			}
			if _, ok := v["locator"]; ok {
				if id := text("node_id"); id != "" {
					add(p+"/locator", nodes[id])
				} else if text("kind") == "SOURCE_ARTIFACT" {
					add(p+"/locator", text("locator"))
				}
			}
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if k == "data" || k == "observation" {
					continue
				}
				q := p + "/" + escape(k)
				if s, ok := v[k].(string); ok {
					switch {
					case k == "uri" || k == "resolved_uri" || k == "source_uri":
						if k == "resolved_uri" && s == "" {
							s = labels[text("label")]
						}
						add(q, s)
					case k == "symbol":
						add(q, uri)
					case k == "node_id" || strings.HasSuffix(k, "_node_id"):
						add(q, nodes[s])
					case k == "id" && uri != "" && v["selection_range"] != nil:
						add(q, uri)
					case k == "id" || k == "request_id" || k == "edge_group_id" || k == "relation_id" || k == "execution_bundle_id" || k == "target_id" || k == "context_id":
						add(q, "")
					case (k == "from" || k == "to") && strings.HasSuffix(p, "/connection"):
						add(q, nodes[s])
					}
				}
				if xs, ok := v[k].([]any); ok {
					if k == "node_ids" || strings.HasSuffix(k, "_node_ids") || k == "prepared_target_ids" || k == "reached_node_ids" || k == "frontier_ids" || k == "successful_empty_ids" || k == "start_ids" || (k == "targets" && p == "/graph") || (k == "nodes" && strings.HasSuffix(p, "/path")) {
						for i, id := range xs {
							s, _ := id.(string)
							add(fmt.Sprintf("%s/%d", q, i), nodes[s])
						}
					}
					if k == "call_sites" {
						owner := nodes[text("caller_node_id")]
						if owner == "" {
							for _, e := range r.Graph.Edges {
								if e.RelationID == text("relation_id") {
									owner = nodes[e.CallerNodeID]
									break
								}
							}
						}
						for i := range xs {
							add(fmt.Sprintf("%s/%d", q, i), owner)
						}
					}
				}
				if k == "range" || k == "selection_range" || k == "selectionRange" {
					owner := uri
					if owner == "" {
						owner = nodes[text("node_id")]
					}
					if owner != "" {
						add(q, owner)
					}
				}
				walk(v[k], q)
			}
		case []any:
			for i, x := range v {
				walk(x, fmt.Sprintf("%s/%d", p, i))
			}
		}
	}
	walk(doc["graph"], "/graph")
	delete(doc, "graph")
	walk(doc, "/acquisition")
	if failure != nil {
		return nil, failure
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pointer < out[j].Pointer })
	for i := 1; i < len(out); i++ {
		if out[i].Pointer == out[i-1].Pointer {
			return nil, errors.New("duplicate V2 census pointer")
		}
	}
	return out, nil
}
