package mcpcontract

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	futureInputID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v1.schema.json"
	futureResultID  = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.transient-structural-result.v1.schema.json"
	futureSuccessID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-result.v1.schema.json"
	futureFailureID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-domain-error.v1.schema.json"
)

func compileFutureStructuralSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	files := map[string]string{
		futureInputID: "input-structural-context.v1.schema.json", futureResultID: "lsp-trace.transient-structural-result.v1.schema.json",
		futureSuccessID: "envelope-structural-context-result.v1.schema.json", futureFailureID: "envelope-structural-context-domain-error.v1.schema.json",
	}
	for id, name := range files {
		raw, err := os.ReadFile("testdata/schemas/" + name)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("compile document %s: %v", name, err)
		}
		if err := compiler.AddResource(id, doc); err != nil {
			t.Fatal(err)
		}
	}
	out := make(map[string]*jsonschema.Schema, len(files))
	for id := range files {
		compiled, err := compiler.Compile(id)
		if err != nil {
			t.Fatalf("compile %s: %v", id, err)
		}
		out[id] = compiled
	}
	return out
}

func validateFuture(t *testing.T, schema *jsonschema.Schema, value any, wantValid bool) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	err = schema.Validate(doc)
	if wantValid && err != nil {
		t.Fatalf("valid document rejected: %v\n%s", err, raw)
	}
	if !wantValid && err == nil {
		t.Fatalf("invalid document accepted: %s", raw)
	}
}

func validFutureInput() map[string]any {
	return map[string]any{"session_id": "session-alias", "generation": 1, "uri": "file:///workspace/main.go", "symbol": "Run", "down_depth": 2, "up_depth": 2, "max_nodes": 100, "timeout_ms": 5000, "request_timeout_ms": 1000, "analysis": map[string]any{"kind": "NEIGHBORHOOD"}}
}

func validAccounting() map[string]any {
	return map[string]any{"request_attempted": 1, "request_succeeded": 1, "request_failed": 0, "request_cancelled": 0, "prepared_attempted": 1, "prepared_returned": 1, "prepared_empty": 0, "prepared_failed": 0, "node_observed": 1, "node_admitted": 1, "node_rejected": 0, "node_omitted": 0, "occurrence_observed": 0, "occurrence_admitted": 0, "occurrence_rejected": 0, "occurrence_omitted": 0, "frontier_observed": 1, "frontier_expanded": 1, "frontier_unexpanded": 0, "deduplicated_nodes": 0, "deduplicated_occurrences": 0, "truncated": false}
}

func validFutureResult() map[string]any {
	return map[string]any{
		"schema_version": "lsp-trace.transient-structural-result.v1", "phase": "DELIVERY_CHECK", "state": "COMPLETE", "evidence_class": "TRANSIENT_LIVE", "authority": 0,
		"source_graph_complete": "UNKNOWN", "retained": false, "replayable": false, "publication_eligible": false, "hydration_eligible": false,
		"claim_ceiling":        "Under the named managed session generation, exact target, server responses, traversal bounds, and analysis policy, this bounded server-reported call graph has the reported structural properties.",
		"canonical_session_id": "ts_0123456789abcdef0123456789abcdef", "generation": 1, "target_node_id": "tn_0123456789abcdef0123456789abcdef", "position_encoding": "utf-16",
		"traversal_policy": map[string]any{"policy_id": "transient-calls-traversal.v1", "policy_digest": "sha256:" + strings.Repeat("1", 64), "down_depth": 2, "up_depth": 2, "max_nodes": 100},
		"resource_policy":  map[string]any{"policy_id": "transient-structural-resources.v1", "policy_digest": "sha256:" + strings.Repeat("2", 64), "timeout_ms": 5000, "request_timeout_ms": 1000},
		"analysis_policy":  map[string]any{"policy_id": "transient-neighborhood.v1", "policy_version": "1", "policy_digest": "sha256:" + strings.Repeat("3", 64)}, "graph_digest": "sha256:" + strings.Repeat("4", 64), "accounting": validAccounting(),
		"analysis": map[string]any{"kind": "NEIGHBORHOOD", "root_node_id": "tn_0123456789abcdef0123456789abcdef", "nodes": []any{map[string]any{"node_id": "tn_0123456789abcdef0123456789abcdef"}}, "edges": []any{}, "incoming_count": 0, "outgoing_count": 0, "frontier_count": 0},
	}
}

func cloneMap(t *testing.T, in map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(in)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFutureStructuralContextSchemasCompileAndAcceptClosedControls(t *testing.T) {
	s := compileFutureStructuralSchemas(t)
	validateFuture(t, s[futureInputID], validFutureInput(), true)
	position := validFutureInput()
	delete(position, "symbol")
	position["line"], position["character"] = 0, 0
	position["analysis"] = map[string]any{"kind": "IMPACT", "direction": "OUTGOING", "depth": 1}
	validateFuture(t, s[futureInputID], position, true)
	result := validFutureResult()
	validateFuture(t, s[futureResultID], result, true)
	impactResult := cloneMap(t, result)
	impactResult["analysis_policy"].(map[string]any)["policy_id"] = "transient-impact.v1"
	impactResult["analysis"] = map[string]any{"kind": "IMPACT", "root_node_id": "tn_0123456789abcdef0123456789abcdef", "direction": "INCOMING", "depth": 2, "reachable_node_ids": []any{}, "witness_edge_ids": []any{}}
	validateFuture(t, s[futureResultID], impactResult, true)
	success := map[string]any{"envelope_version": "1", "envelope_schema_id": futureSuccessID, "tool": "lsp_trace_v1_structural_context", "request_id": "r1", "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "result": result}
	validateFuture(t, s[futureSuccessID], success, true)
	failure := map[string]any{"envelope_version": "1", "envelope_schema_id": futureFailureID, "tool": "lsp_trace_v1_structural_context", "request_id": "r1", "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "phase": "TRAVERSAL", "state": "PARTIAL", "error": map[string]any{"code": "PARTIAL"}}
	validateFuture(t, s[futureFailureID], failure, true)
}

func TestFutureStructuralInputRejectsEveryForbiddenSurfaceAndBadBounds(t *testing.T) {
	s := compileFutureStructuralSchemas(t)[futureInputID]
	for _, field := range []string{"output_selector", "graph_provenance", "workspace_revision", "custody", "capture", "capture_supply", "source_snapshot", "source_supply", "publication", "artifact", "artifact_selector", "retained_input", "hydration", "include_bodies", "private_root", "path", "providers", "relations", "adapters", "metadata"} {
		t.Run(field, func(t *testing.T) { v := validFutureInput(); v[field] = true; validateFuture(t, s, v, false) })
	}
	for _, field := range []string{"generation", "down_depth", "up_depth", "max_nodes", "timeout_ms", "request_timeout_ms"} {
		t.Run("missing_"+field, func(t *testing.T) { v := validFutureInput(); delete(v, field); validateFuture(t, s, v, false) })
	}
	for name, mutate := range map[string]func(map[string]any){
		"both_targets": func(v map[string]any) { v["line"], v["character"] = 0, 0 }, "partial_position": func(v map[string]any) { delete(v, "symbol"); v["line"] = 0 },
		"unknown_analysis": func(v map[string]any) { v["analysis"] = map[string]any{"kind": "CENTRALITY"} }, "impact_missing_direction": func(v map[string]any) { v["analysis"] = map[string]any{"kind": "IMPACT", "depth": 1} },
		"impact_missing_depth": func(v map[string]any) { v["analysis"] = map[string]any{"kind": "IMPACT", "direction": "INCOMING"} }, "impact_zero_depth": func(v map[string]any) {
			v["analysis"] = map[string]any{"kind": "IMPACT", "direction": "INCOMING", "depth": 0}
		},
		"depth_over_bound": func(v map[string]any) { v["down_depth"] = 65 }, "zero_max_nodes": func(v map[string]any) { v["max_nodes"] = 0 }, "unknown_metadata": func(v map[string]any) { v["metadata"] = map[string]any{} },
	} {
		t.Run(name, func(t *testing.T) { v := validFutureInput(); mutate(v); validateFuture(t, s, v, false) })
	}
}

func TestFutureStructuralResultRejectsPrivacyShapesAndMutableClaims(t *testing.T) {
	s := compileFutureStructuralSchemas(t)[futureResultID]
	for name, mutate := range map[string]func(map[string]any){
		"uri_node": func(v map[string]any) { v["target_node_id"] = "file:///private/a.go" }, "path_node": func(v map[string]any) { v["target_node_id"] = "/Users/private/a.go" },
		"symbol_name": func(v map[string]any) { v["analysis"].(map[string]any)["name"] = "Run" }, "source": func(v map[string]any) { v["analysis"].(map[string]any)["source"] = "secret" },
		"diagnostic": func(v map[string]any) { v["analysis"].(map[string]any)["diagnostic"] = "bad" }, "command": func(v map[string]any) { v["analysis"].(map[string]any)["command"] = "server" },
		"env": func(v map[string]any) { v["analysis"].(map[string]any)["env"] = "TOKEN=x" }, "provider": func(v map[string]any) { v["analysis"].(map[string]any)["provider"] = "gopls" },
		"authority": func(v map[string]any) { v["authority"] = 1 }, "completeness": func(v map[string]any) { v["source_graph_complete"] = "COMPLETE" }, "retained": func(v map[string]any) { v["retained"] = true },
		"claim": func(v map[string]any) { v["claim_ceiling"] = "safe to refactor" }, "phase": func(v map[string]any) { v["phase"] = "ANALYSIS" }, "state": func(v map[string]any) { v["state"] = "PARTIAL" },
		"missing_accounting": func(v map[string]any) { delete(v, "accounting") }, "negative_count": func(v map[string]any) { v["accounting"].(map[string]any)["node_observed"] = -1 },
		"unknown_analysis": func(v map[string]any) { v["analysis"] = map[string]any{"kind": "CENTRALITY"} }, "policy_analysis_mismatch": func(v map[string]any) { v["analysis_policy"].(map[string]any)["policy_id"] = "transient-impact.v1" }, "metadata": func(v map[string]any) { v["metadata"] = map[string]any{} },
	} {
		t.Run(name, func(t *testing.T) { v := cloneMap(t, validFutureResult()); mutate(v); validateFuture(t, s, v, false) })
	}
}

func TestFutureStructuralPhaseStateMatrixAndEnvelopeExclusivity(t *testing.T) {
	s := compileFutureStructuralSchemas(t)
	legal := map[string][]string{"PREFLIGHT": {"UNSUPPORTED", "AMBIGUOUS_TARGET", "TARGET_NOT_FOUND", "RESOURCE_LIMIT", "TIMEOUT", "CANCELLED", "GENERATION_CHANGED", "INVALID_SERVER_RESPONSE"}, "TRAVERSAL": {"PARTIAL", "TRUNCATED", "RESOURCE_LIMIT", "TIMEOUT", "CANCELLED", "GENERATION_CHANGED", "INVALID_SERVER_RESPONSE"}, "ADMISSION": {"RESOURCE_LIMIT", "CANCELLED", "GENERATION_CHANGED", "INVALID_SERVER_RESPONSE"}, "ANALYSIS": {"RESOURCE_LIMIT", "TIMEOUT", "CANCELLED", "GENERATION_CHANGED", "ANALYSIS_FAILED"}, "DELIVERY_CHECK": {"CANCELLED", "GENERATION_CHANGED"}}
	allStates := []string{"UNSUPPORTED", "AMBIGUOUS_TARGET", "TARGET_NOT_FOUND", "PARTIAL", "TRUNCATED", "RESOURCE_LIMIT", "TIMEOUT", "CANCELLED", "GENERATION_CHANGED", "INVALID_SERVER_RESPONSE", "ANALYSIS_FAILED"}
	for phase, states := range legal {
		allowed := map[string]bool{}
		for _, state := range states {
			allowed[state] = true
		}
		for _, state := range allStates {
			v := map[string]any{"envelope_version": "1", "envelope_schema_id": futureFailureID, "tool": "lsp_trace_v1_structural_context", "request_id": "r1", "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "phase": phase, "state": state, "error": map[string]any{"code": state}}
			validateFuture(t, s[futureFailureID], v, allowed[state])
		}
	}
	result := validFutureResult()
	success := map[string]any{"envelope_version": "1", "envelope_schema_id": futureSuccessID, "tool": "lsp_trace_v1_structural_context", "request_id": "r1", "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "result": result}
	for _, field := range []string{"error", "phase", "state"} {
		v := cloneMap(t, success)
		v[field] = map[string]any{}
		validateFuture(t, s[futureSuccessID], v, false)
	}
	mismatchedSuccess := cloneMap(t, success)
	mismatchedSuccess["outcome"] = "EMPTY"
	validateFuture(t, s[futureSuccessID], mismatchedSuccess, false)
	failure := map[string]any{"envelope_version": "1", "envelope_schema_id": futureFailureID, "tool": "lsp_trace_v1_structural_context", "request_id": "r1", "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "phase": "PREFLIGHT", "state": "UNSUPPORTED", "error": map[string]any{"code": "UNSUPPORTED"}}
	mismatchedError := cloneMap(t, failure)
	mismatchedError["error"].(map[string]any)["code"] = "TIMEOUT"
	validateFuture(t, s[futureFailureID], mismatchedError, false)
	failure["result"] = result
	validateFuture(t, s[futureFailureID], failure, false)
}

func TestFutureStructuralContractsRemainUnregistered(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range manifest.Tools {
		if tool.Name == "lsp_trace_v1_structural_context" {
			t.Fatal("FUTURE operation 35 must remain unregistered until operation 34 lands")
		}
	}
	if len(manifest.Tools) != 13 {
		t.Fatalf("public capability contract count changed: %d", len(manifest.Tools))
	}
}
