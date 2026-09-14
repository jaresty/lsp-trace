package mcpcontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	futureInputID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v1.schema.json"
	futureResultID  = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.transient-structural-result.v1.schema.json"
	futureSuccessID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-result.v1.schema.json"
	futureFailureID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-structural-context-domain-error.v1.schema.json"
	futureRequestID = "sc_0123456789abcdef0123456789abcdef"
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
	return map[string]any{"session_id": "session-alias", "generation": 1, "uri": "file:///workspace/main.go", "symbol": "Run", "down_depth": 2, "up_depth": 2, "max_nodes": 100, "timeout_ms": 5000, "request_timeout_ms": 1000, "max_messages": futureDefaultMaxMessages, "max_bytes": futureDefaultMaxBytes, "analysis": map[string]any{"kind": "NEIGHBORHOOD"}}
}

func validAccounting() map[string]any {
	zeroReasons := func() map[string]any {
		return map[string]any{"DEPTH_BOUND": 0, "NODE_BOUND": 0, "REQUEST_BOUND": 0, "TIMEOUT": 0, "CANCELLATION": 0, "UNSUPPORTED_RESPONSE": 0, "MALFORMED_RESPONSE": 0, "DEDUPLICATION": 0}
	}
	return map[string]any{"request_attempted": 1, "request_succeeded": 1, "request_failed": 0, "request_cancelled": 0, "request_omission_reasons": zeroReasons(), "prepared_attempted": 1, "prepared_returned": 1, "prepared_empty": 0, "prepared_failed": 0, "node_observed": 1, "node_admitted": 1, "node_rejected": 0, "node_omitted": 0, "node_omission_reasons": zeroReasons(), "occurrence_observed": 1, "occurrence_admitted": 1, "occurrence_rejected": 0, "occurrence_omitted": 0, "occurrence_omission_reasons": zeroReasons(), "frontier_observed": 1, "frontier_expanded": 1, "frontier_unexpanded": 0, "frontier_omission_reasons": zeroReasons(), "deduplicated_nodes": 0, "deduplicated_occurrences": 0, "truncated": false}
}

func validFutureResult() map[string]any {
	return map[string]any{
		"schema_version": "lsp-trace.transient-structural-result.v1", "phase": "DELIVERY_CHECK", "state": "COMPLETE", "evidence_class": "TRANSIENT_LIVE", "authority": 0,
		"source_graph_complete": "UNKNOWN", "retained": false, "replayable": false, "publication_eligible": false, "hydration_eligible": false,
		"claim_ceiling":        "Under the named managed session generation, exact target, server responses, traversal bounds, and analysis policy, this bounded server-reported call graph has the reported structural properties.",
		"canonical_session_id": "ts_0123456789abcdef0123456789abcdef", "generation": 1, "target_node_id": "tn_0123456789abcdef0123456789abcdef", "position_encoding": "utf-16",
		"traversal_policy": map[string]any{"policy_id": futureTraversalPolicyID, "policy_version": futureAnalysisVersion, "policy_status": futurePolicyStatus, "policy_digest": futureTraversalPolicyDigest, "down_depth": 2, "up_depth": 2, "max_nodes": 100},
		"resource_policy":  map[string]any{"policy_id": futureResourcePolicyID, "policy_version": futureAnalysisVersion, "policy_status": futurePolicyStatus, "policy_digest": futureResourcePolicyDigest, "timeout_ms": 5000, "request_timeout_ms": 1000, "max_messages": futureDefaultMaxMessages, "max_bytes": futureDefaultMaxBytes},
		"analysis_policy":  map[string]any{"policy_id": futureNeighborhoodID, "policy_version": futureAnalysisVersion, "policy_status": futurePolicyStatus, "policy_digest": futureNeighborhoodPolicyDigest}, "graph_digest": "sha256:" + strings.Repeat("4", 64), "accounting": validAccounting(),
		"analysis": map[string]any{"kind": "NEIGHBORHOOD", "root_node_id": "tn_0123456789abcdef0123456789abcdef", "nodes": []any{map[string]any{"node_id": "tn_0123456789abcdef0123456789abcdef"}}, "edges": []any{map[string]any{"edge_id": "te_0123456789abcdef0123456789abcdef", "caller_node_id": "tn_0123456789abcdef0123456789abcdef", "callee_node_id": "tn_0123456789abcdef0123456789abcdef"}}, "incoming_count": 1, "outgoing_count": 1},
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
	impactResult["analysis_policy"].(map[string]any)["policy_id"] = futureImpactID
	impactResult["analysis_policy"].(map[string]any)["policy_digest"] = futureImpactPolicyDigest
	impactResult["analysis"] = map[string]any{"kind": "IMPACT", "root_node_id": "tn_0123456789abcdef0123456789abcdef", "direction": "INCOMING", "depth": 2, "nodes": []any{map[string]any{"node_id": "tn_0123456789abcdef0123456789abcdef"}}, "edges": []any{map[string]any{"edge_id": "te_0123456789abcdef0123456789abcdef", "caller_node_id": "tn_0123456789abcdef0123456789abcdef", "callee_node_id": "tn_0123456789abcdef0123456789abcdef"}}, "reachable_node_ids": []any{}, "witness_edge_ids": []any{}}
	validateFuture(t, s[futureResultID], impactResult, true)
	success := map[string]any{"envelope_version": "1", "envelope_schema_id": futureSuccessID, "tool": "lsp_trace_v1_structural_context", "request_id": futureRequestID, "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "result": result}
	validateFuture(t, s[futureSuccessID], success, true)
	failure := map[string]any{"envelope_version": "1", "envelope_schema_id": futureFailureID, "tool": "lsp_trace_v1_structural_context", "request_id": futureRequestID, "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "phase": "TRAVERSAL", "state": "PARTIAL", "error": map[string]any{"code": "PARTIAL"}}
	validateFuture(t, s[futureFailureID], failure, true)
}

func TestFutureStructuralNeighborhoodOmitsFrontierCount(t *testing.T) {
	result := validFutureResult()
	validateFuture(t, compileFutureStructuralSchemas(t)[futureResultID], result, true)
	if err := ValidateFutureStructuralSemanticsV1(validFutureInput(), result); err != nil {
		t.Fatalf("valid NEIGHBORHOOD without frontier_count rejected: %v", err)
	}
}

func TestFutureStructuralNeighborhoodRejectsFrontierCountAsUnknown(t *testing.T) {
	result := validFutureResult()
	result["analysis"].(map[string]any)["frontier_count"] = 0
	validateFuture(t, compileFutureStructuralSchemas(t)[futureResultID], result, false)
	if err := ValidateFutureStructuralSemanticsV1(validFutureInput(), result); err != errFutureShape {
		t.Fatalf("frontier_count got %v, want unknown-field shape sentinel", err)
	}
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
			v := map[string]any{"envelope_version": "1", "envelope_schema_id": futureFailureID, "tool": "lsp_trace_v1_structural_context", "request_id": futureRequestID, "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "phase": phase, "state": state, "error": map[string]any{"code": state}}
			validateFuture(t, s[futureFailureID], v, allowed[state])
		}
	}
	result := validFutureResult()
	success := map[string]any{"envelope_version": "1", "envelope_schema_id": futureSuccessID, "tool": "lsp_trace_v1_structural_context", "request_id": futureRequestID, "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "result": result}
	for _, field := range []string{"error", "phase", "state"} {
		v := cloneMap(t, success)
		v[field] = map[string]any{}
		validateFuture(t, s[futureSuccessID], v, false)
	}
	mismatchedSuccess := cloneMap(t, success)
	mismatchedSuccess["outcome"] = "EMPTY"
	validateFuture(t, s[futureSuccessID], mismatchedSuccess, false)
	failure := map[string]any{"envelope_version": "1", "envelope_schema_id": futureFailureID, "tool": "lsp_trace_v1_structural_context", "request_id": futureRequestID, "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "phase": "PREFLIGHT", "state": "UNSUPPORTED", "error": map[string]any{"code": "UNSUPPORTED"}}
	mismatchedError := cloneMap(t, failure)
	mismatchedError["error"].(map[string]any)["code"] = "TIMEOUT"
	validateFuture(t, s[futureFailureID], mismatchedError, false)
	failure["result"] = result
	validateFuture(t, s[futureFailureID], failure, false)
}

func TestFutureStructuralSchemaAcceptsRelationalCounterexamplesSemanticV1Rejects(t *testing.T) {
	schema := compileFutureStructuralSchemas(t)[futureResultID]
	cases := map[string]func(map[string]any){
		"root_target": func(v map[string]any) {
			v["analysis"].(map[string]any)["root_node_id"] = "tn_ffffffffffffffffffffffffffffffff"
		},
		"empty_iff_calls":     func(v map[string]any) { v["state"] = "EMPTY" },
		"node_arithmetic":     func(v map[string]any) { v["accounting"].(map[string]any)["node_observed"] = 2 },
		"prepared_arithmetic": func(v map[string]any) { v["accounting"].(map[string]any)["prepared_attempted"] = 2 },
		"request_reason_partition": func(v map[string]any) {
			v["accounting"].(map[string]any)["request_omission_reasons"].(map[string]any)["TIMEOUT"] = 1
		},
		"one_reason_per_omission": func(v map[string]any) {
			a := v["accounting"].(map[string]any)
			a["node_observed"] = 2
			a["node_omitted"] = 1
			a["node_omission_reasons"].(map[string]any)["NODE_BOUND"] = 0
		},
		"opaque_membership": func(v map[string]any) {
			v["analysis"].(map[string]any)["edges"].([]any)[0].(map[string]any)["callee_node_id"] = "tn_ffffffffffffffffffffffffffffffff"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := cloneMap(t, validFutureResult())
			mutate(v)
			validateFuture(t, schema, v, true) // relational counterexample is intentionally schema-valid
			if err := ValidateFutureStructuralSemanticsV1(validFutureInput(), v); err == nil {
				t.Fatal("semantic-v1 accepted relational counterexample")
			}
		})
	}
	t.Run("impact_directional_depth", func(t *testing.T) {
		v := cloneMap(t, validFutureResult())
		v["analysis_policy"].(map[string]any)["policy_id"] = futureImpactID
		v["analysis_policy"].(map[string]any)["policy_digest"] = futureImpactPolicyDigest
		v["analysis"] = map[string]any{"kind": "IMPACT", "root_node_id": v["target_node_id"], "direction": "INCOMING", "depth": 3, "nodes": []any{map[string]any{"node_id": v["target_node_id"]}}, "edges": []any{map[string]any{"edge_id": "te_0123456789abcdef0123456789abcdef", "caller_node_id": v["target_node_id"], "callee_node_id": v["target_node_id"]}}, "reachable_node_ids": []any{}, "witness_edge_ids": []any{}}
		validateFuture(t, schema, v, true)
		if err := ValidateFutureStructuralSemanticsV1(validFutureInput(), v); err == nil {
			t.Fatal("semantic-v1 accepted impact depth beyond up_depth")
		}
	})
	if err := ValidateFutureStructuralSemanticsV1(validFutureInput(), validFutureResult()); err != nil {
		t.Fatalf("semantic-v1 rejected valid fixture: %v", err)
	}
}

func TestFutureStructuralSemanticValidatorRejectsAdversarialInMemoryValues(t *testing.T) {
	for name, input := range map[string][2]map[string]any{"nil_input": {nil, validFutureResult()}, "nil_result": {validFutureInput(), nil}} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() != nil {
					t.Fatal("validator panicked")
				}
			}()
			if err := ValidateFutureStructuralSemanticsV1(input[0], input[1]); err != errFutureShape {
				t.Fatalf("got %v, want shape sentinel", err)
			}
		})
	}
	cases := map[string]func(map[string]any, map[string]any){
		"nil_node":   func(_ map[string]any, r map[string]any) { r["analysis"].(map[string]any)["nodes"] = []any{nil} },
		"wrong_node": func(_ map[string]any, r map[string]any) { r["analysis"].(map[string]any)["nodes"] = []any{"node"} },
		"nil_edge":   func(_ map[string]any, r map[string]any) { r["analysis"].(map[string]any)["edges"] = []any{nil} },
		"wrong_edge": func(_ map[string]any, r map[string]any) { r["analysis"].(map[string]any)["edges"] = []any{false} },
		"duplicate_node": func(_ map[string]any, r map[string]any) {
			a := r["analysis"].(map[string]any)
			a["nodes"] = append(a["nodes"].([]any), a["nodes"].([]any)[0])
		},
		"duplicate_edge": func(_ map[string]any, r map[string]any) {
			a := r["analysis"].(map[string]any)
			a["edges"] = append(a["edges"].([]any), a["edges"].([]any)[0])
		},
		"empty_node_id": func(_ map[string]any, r map[string]any) {
			r["analysis"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["node_id"] = ""
		},
		"unknown_node_key": func(_ map[string]any, r map[string]any) {
			r["analysis"].(map[string]any)["nodes"].([]any)[0].(map[string]any)["private"] = true
		},
		"missing_edge_id": func(_ map[string]any, r map[string]any) {
			delete(r["analysis"].(map[string]any)["edges"].([]any)[0].(map[string]any), "edge_id")
		},
		"float_count": func(_ map[string]any, r map[string]any) {
			r["accounting"].(map[string]any)["node_observed"] = float64(1)
		},
		"bool_count":   func(_ map[string]any, r map[string]any) { r["accounting"].(map[string]any)["node_observed"] = true },
		"string_count": func(_ map[string]any, r map[string]any) { r["accounting"].(map[string]any)["node_observed"] = "1" },
		"negative_count": func(_ map[string]any, r map[string]any) {
			r["accounting"].(map[string]any)["node_observed"] = int64(-1)
		},
		"uint_overflow": func(_ map[string]any, r map[string]any) {
			r["accounting"].(map[string]any)["node_observed"] = uint64(math.MaxUint64)
		},
		"sum_overflow": func(_ map[string]any, r map[string]any) {
			a := r["accounting"].(map[string]any)
			a["node_observed"], a["node_admitted"], a["node_rejected"] = int64(math.MaxInt64), int64(math.MaxInt64), int64(math.MaxInt64)
		},
		"too_many_nodes": func(_ map[string]any, r map[string]any) {
			r["analysis"].(map[string]any)["nodes"] = make([]any, futureMaxNodes+1)
		},
		"too_many_edges": func(_ map[string]any, r map[string]any) {
			r["analysis"].(map[string]any)["edges"] = make([]any, futureMaxEdges+1)
		},
		"oversized_session": func(i map[string]any, _ map[string]any) { i["session_id"] = strings.Repeat("s", 257) },
		"oversized_symbol":  func(i map[string]any, _ map[string]any) { i["symbol"] = strings.Repeat("s", 1025) },
		"invalid_utf8":      func(i map[string]any, _ map[string]any) { i["symbol"] = string([]byte{0xff}) },
		"wrong_depth_type":  func(i map[string]any, _ map[string]any) { i["down_depth"] = 1.0 },
		"depth_bound":       func(i map[string]any, _ map[string]any) { i["down_depth"] = 65 },
		"resource_bound":    func(i map[string]any, _ map[string]any) { i["timeout_ms"] = 60001 },
		"missing_input":     func(i map[string]any, _ map[string]any) { delete(i, "generation") },
		"unknown_input":     func(i map[string]any, _ map[string]any) { i["private"] = "secret" },
		"unknown_reason": func(_ map[string]any, r map[string]any) {
			r["accounting"].(map[string]any)["node_omission_reasons"].(map[string]any)["OTHER"] = 0
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			i, r := validFutureInput(), validFutureResult()
			mutate(i, r)
			for call := 0; call < 3; call++ {
				func() {
					defer func() {
						if recovered := recover(); recovered != nil {
							t.Fatalf("validator panicked")
						}
					}()
					err := ValidateFutureStructuralSemanticsV1(i, r)
					if err == nil {
						t.Fatal("adversarial value accepted")
					}
					if !strings.HasPrefix(err.Error(), "semantic-v1:") || strings.Contains(err.Error(), "secret") {
						t.Fatalf("unstable or value-echoing error: %v", err)
					}
				}()
			}
		})
	}
}

func TestFutureStructuralSemanticValidatorMatchesSchemaURIAndUnicodeBounds(t *testing.T) {
	schema := compileFutureStructuralSchemas(t)[futureInputID]
	raw, err := os.ReadFile(filepath.Join("testdata", "schemas", "input-structural-context.v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schemaDocument map[string]any
	if err := json.Unmarshal(raw, &schemaDocument); err != nil {
		t.Fatal(err)
	}
	properties := schemaDocument["properties"].(map[string]any)
	uriSchema := properties["uri"].(map[string]any)
	if got := uriSchema["pattern"]; got != futureAbsoluteURIPattern.String() {
		t.Fatalf("schema/semantic URI pattern drift: schema=%q semantic=%q", got, futureAbsoluteURIPattern.String())
	}
	if _, present := uriSchema["format"]; present {
		t.Fatal("draft URI subset must not add divergent format validation")
	}
	for _, uri := range []string{
		"file:///workspace/main.go", "https://example.com/a%20b", "custom:界/path",
	} {
		t.Run("valid_uri_"+strings.NewReplacer(":", "_", "/", "_").Replace(uri), func(t *testing.T) {
			i := validFutureInput()
			i["uri"] = uri
			validateFuture(t, schema, i, true)
			if err := ValidateFutureStructuralSemanticsV1(i, validFutureResult()); err != nil {
				t.Fatalf("semantic-v1 rejected schema-valid URI %q: %v", uri, err)
			}
		})
	}
	for _, uri := range []string{"not-a-uri", "../relative.go", "/bare/path", "C:/windows/path", "http://exa mple.com", "custom:\tvalue", "custom:\u007f", "file:///%", "file:///%0", "file:///%zz", "file:///%2G"} {
		t.Run("bad_uri_"+strings.NewReplacer(":", "_", "/", "_").Replace(uri), func(t *testing.T) {
			i := validFutureInput()
			i["uri"] = uri
			validateFuture(t, schema, i, false)
			if err := ValidateFutureStructuralSemanticsV1(i, validFutureResult()); err == nil {
				t.Fatalf("semantic-v1 accepted schema-invalid URI %q", uri)
			}
		})
	}
	for name, tc := range map[string]struct {
		uri   string
		valid bool
	}{
		"exact_max_ascii":   {"custom:" + strings.Repeat("a", futureMaxURICodePoints-7), true},
		"exact_max_unicode": {"custom:" + strings.Repeat("界", futureMaxURICodePoints-7), true},
		"over_max_ascii":    {"custom:" + strings.Repeat("a", futureMaxURICodePoints-6), false},
		"over_max_unicode":  {"custom:" + strings.Repeat("界", futureMaxURICodePoints-6), false},
	} {
		t.Run(name, func(t *testing.T) {
			i := validFutureInput()
			i["uri"] = tc.uri
			validateFuture(t, schema, i, tc.valid)
			err := ValidateFutureStructuralSemanticsV1(i, validFutureResult())
			if (err == nil) != tc.valid {
				t.Fatalf("semantic/schema URI disagreement: semantic=%v valid=%v", err, tc.valid)
			}
		})
	}
	// JSON has only Unicode scalar strings: encoding/json replaces invalid Go UTF-8
	// before schema evaluation. Standalone semantics must reject the original value.
	i := validFutureInput()
	i["uri"] = "custom:" + string([]byte{0xff})
	if err := ValidateFutureStructuralSemanticsV1(i, validFutureResult()); err == nil {
		t.Fatal("semantic-v1 accepted invalid UTF-8 URI")
	}
	for name, boundary := range map[string]struct {
		field                string
		validCount, badCount int
	}{
		"session_200_multibyte": {"session_id", 200, 257},
		"session_max_multibyte": {"session_id", 256, 257},
		"symbol_max_multibyte":  {"symbol", 1024, 1025},
	} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				count int
				valid bool
			}{{boundary.validCount, true}, {boundary.badCount, false}} {
				i := validFutureInput()
				i[boundary.field] = strings.Repeat("界", tc.count)
				validateFuture(t, schema, i, tc.valid)
				err := ValidateFutureStructuralSemanticsV1(i, validFutureResult())
				if (err == nil) != tc.valid {
					t.Fatalf("%s code points=%d: got %v, valid=%v", boundary.field, tc.count, err, tc.valid)
				}
			}
		})
	}
}

func TestFutureStructuralSemanticValidatorRejectsInvalidOrOversizedReferenceBeforePattern(t *testing.T) {
	for name, id := range map[string]string{
		"invalid_utf8": "tn_" + string([]byte{0xff}) + strings.Repeat("0", 31),
		"over_limit":   "tn_" + strings.Repeat("0", 1<<20),
	} {
		t.Run(name, func(t *testing.T) {
			i, r := validFutureInput(), validFutureResult()
			i["analysis"] = map[string]any{"kind": "IMPACT", "direction": "OUTGOING", "depth": 1}
			r["analysis_policy"].(map[string]any)["policy_id"] = futureImpactID
			r["analysis_policy"].(map[string]any)["policy_digest"] = futureImpactPolicyDigest
			a := r["analysis"].(map[string]any)
			a["kind"], a["direction"], a["depth"] = "IMPACT", "OUTGOING", 1
			delete(a, "incoming_count")
			delete(a, "outgoing_count")
			a["reachable_node_ids"] = []any{id}
			a["witness_edge_ids"] = []any{}
			if err := ValidateFutureStructuralSemanticsV1(i, r); err != errFutureValue {
				t.Fatalf("got %v, want value sentinel", err)
			}
		})
	}
}

func TestFutureStructuralSemanticValidatorRejectsBothPolicyKindMismatches(t *testing.T) {
	for _, tc := range []struct{ policyID, kind string }{
		{"transient-impact.v1", "NEIGHBORHOOD"},
		{"transient-neighborhood.v1", "IMPACT"},
	} {
		t.Run(tc.policyID+"_"+tc.kind, func(t *testing.T) {
			i, r := validFutureInput(), validFutureResult()
			r["analysis_policy"].(map[string]any)["policy_id"] = tc.policyID
			if tc.kind == "IMPACT" {
				i["analysis"] = map[string]any{"kind": "IMPACT", "direction": "OUTGOING", "depth": 1}
				r["analysis"] = map[string]any{"kind": "IMPACT", "root_node_id": r["target_node_id"], "direction": "OUTGOING", "depth": 1, "nodes": []any{map[string]any{"node_id": r["target_node_id"]}}, "edges": []any{}, "reachable_node_ids": []any{}, "witness_edge_ids": []any{}}
				r["state"] = "EMPTY"
			}
			if err := ValidateFutureStructuralSemanticsV1(i, r); err == nil {
				t.Fatal("semantic-v1 accepted policy-kind mismatch")
			}
		})
	}
}

func TestFutureStructuralSemanticValidatorBoundsObjectShapesBeforeTraversal(t *testing.T) {
	oversized := func(base map[string]any, count int) map[string]any {
		for n := 0; n < count; n++ {
			base[fmt.Sprintf("unknown_%d", n)] = n
		}
		return base
	}
	for name, mutate := range map[string]func(map[string]any, map[string]any){
		"top_input":       func(i, _ map[string]any) { oversized(i, 1000) },
		"top_result":      func(_, r map[string]any) { oversized(r, 1000) },
		"input_analysis":  func(i, _ map[string]any) { oversized(i["analysis"].(map[string]any), 1000) },
		"result_analysis": func(_, r map[string]any) { oversized(r["analysis"].(map[string]any), 1000) },
		"analysis_policy": func(_, r map[string]any) { oversized(r["analysis_policy"].(map[string]any), 1000) },
		"accounting":      func(_, r map[string]any) { oversized(r["accounting"].(map[string]any), 1000) },
		"reason_map": func(_, r map[string]any) {
			oversized(r["accounting"].(map[string]any)["node_omission_reasons"].(map[string]any), 1000)
		},
		"node": func(_, r map[string]any) {
			oversized(r["analysis"].(map[string]any)["nodes"].([]any)[0].(map[string]any), 1000)
		},
		"edge": func(_, r map[string]any) {
			oversized(r["analysis"].(map[string]any)["edges"].([]any)[0].(map[string]any), 1000)
		},
	} {
		t.Run(name, func(t *testing.T) {
			i, r := validFutureInput(), validFutureResult()
			mutate(i, r)
			if err := ValidateFutureStructuralSemanticsV1(i, r); err != errFutureShape {
				t.Fatalf("got %v, want shape sentinel", err)
			}
		})
	}
}

func TestFutureStructuralSemanticValidatorRepeatedConcurrentCalls(t *testing.T) {
	i, r := validFutureInput(), validFutureResult()
	i["uri"] = "not-a-uri"
	const workers, calls = 16, 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wg.Done()
			for call := 0; call < calls; call++ {
				if err := ValidateFutureStructuralSemanticsV1(i, r); err == nil {
					t.Errorf("call %d accepted malformed URI", call)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestFutureStructuralSemanticValidatorRejectsDuplicateImpactSets(t *testing.T) {
	for _, key := range []string{"reachable_node_ids", "witness_edge_ids"} {
		t.Run(key, func(t *testing.T) {
			i, r := validFutureInput(), validFutureResult()
			i["analysis"] = map[string]any{"kind": "IMPACT", "direction": "OUTGOING", "depth": 1}
			r["analysis_policy"].(map[string]any)["policy_id"] = futureImpactID
			r["analysis_policy"].(map[string]any)["policy_digest"] = futureImpactPolicyDigest
			a := r["analysis"].(map[string]any)
			a["kind"], a["direction"], a["depth"] = "IMPACT", "OUTGOING", 1
			delete(a, "incoming_count")
			delete(a, "outgoing_count")
			if key == "reachable_node_ids" {
				a[key] = []any{r["target_node_id"], r["target_node_id"]}
				a["witness_edge_ids"] = []any{}
			} else {
				id := a["edges"].([]any)[0].(map[string]any)["edge_id"]
				a[key] = []any{id, id}
				a["reachable_node_ids"] = []any{}
			}
			if err := ValidateFutureStructuralSemanticsV1(i, r); err != errFutureDuplicate {
				t.Fatalf("got %v, want duplicate sentinel", err)
			}
		})
	}
}

func TestFutureStructuralCorrelationIDIsHostGeneratedAndOpaque(t *testing.T) {
	pattern := regexp.MustCompile(`^sc_[0-9a-f]{32}$`)
	first, err := NewFutureStructuralCorrelationID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewFutureStructuralCorrelationID()
	if err != nil {
		t.Fatal(err)
	}
	if !pattern.MatchString(first) || !pattern.MatchString(second) {
		t.Fatalf("invalid correlation IDs: %q %q", first, second)
	}
	if first == second {
		t.Fatal("fresh host correlation IDs collided")
	}
	// Envelope constructors must carry one host ID into either terminal shape.
	successID, errorID := first, first
	if successID != errorID {
		t.Fatal("success/error correlation IDs differ")
	}
}

func TestFutureStructuralReviewBlockers(t *testing.T) {
	s := compileFutureStructuralSchemas(t)
	for _, bad := range []string{"/Users/alice/work.go", `C:\\Users\\alice\\work.go`, "file:///private/work.go", "../private/work.go", "%2FUsers%2Falice%2Fwork.go", "sc_0123"} {
		for _, schemaID := range []string{futureSuccessID, futureFailureID} {
			var envelope map[string]any
			if schemaID == futureSuccessID {
				envelope = map[string]any{"envelope_version": "1", "envelope_schema_id": futureSuccessID, "tool": "lsp_trace_v1_structural_context", "request_id": bad, "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "result": validFutureResult()}
			} else {
				envelope = map[string]any{"envelope_version": "1", "envelope_schema_id": futureFailureID, "tool": "lsp_trace_v1_structural_context", "request_id": bad, "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "phase": "PREFLIGHT", "state": "UNSUPPORTED", "error": map[string]any{"code": "UNSUPPORTED"}}
			}
			validateFuture(t, s[schemaID], envelope, false)
		}
	}
	result := validFutureResult()
	for _, name := range []string{"traversal_policy", "resource_policy", "analysis_policy"} {
		if result[name].(map[string]any)["policy_status"] != "PROVISIONAL_NONCERTIFIED" {
			t.Fatalf("%s lacks PROVISIONAL_NONCERTIFIED policy_status", name)
		}
	}
	for _, name := range []string{"node_omission_reasons", "occurrence_omission_reasons", "frontier_omission_reasons", "request_omission_reasons"} {
		if _, ok := result["accounting"].(map[string]any)[name]; !ok {
			t.Fatalf("accounting lacks %s", name)
		}
	}
}

func TestStructuralContractsRemainAdditiveToHistoricalManifest(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	registered := WithStructuralContext(manifest)
	if len(registered.Tools) != len(manifest.Tools)+1 || registered.Tools[len(registered.Tools)-1].Name != StructuralContextTool {
		t.Fatal("operation 35 must append exactly one canonical tool")
	}
	if len(manifest.Tools) != 13 {
		t.Fatalf("public capability contract count changed: %d", len(manifest.Tools))
	}
}
