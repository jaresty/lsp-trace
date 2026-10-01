package schemas_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Proposal-only decoded-shape controls; independent original-byte replay remains a separate obligation.
func TestADR0011DefinitionTerminalV2OldMappingRED(t *testing.T) {
	old := builtinRole(t, builtinBytes(t, "adr0011-definition-records.proposed.schema.json"), "terminal")
	if err := old.Validate(map[string]any{"version": "definition.terminal.v1", "outcome": "EXCLUDED_VERIFIED"}); err == nil {
		t.Fatal("ASSERT_OLD_EXCLUSION_MAPPING_MUST_REJECT")
	}
	if builtinRole(t, builtinBytes(t, "adr0011-definition-terminal-v2.proposed.schema.json"), "terminal") == nil {
		t.Fatal("ASSERT_SUCCESSOR_ROLE")
	}
}
func TestADR0011DefinitionTerminalV2Synthetic(t *testing.T) {
	role := builtinRole(t, builtinBytes(t, "adr0011-definition-terminal-v2.proposed.schema.json"), "terminal")
	uri := "https://jaresty.github.io/lsp-trace/schemas/adr0011-definition-terminal-v2.proposed.schema.json#/$defs/terminal"
	ref := map[string]any{"role_uri": uri, "selector": "original.json", "byte_length": 1, "digest": "sha256:" + strings.Repeat("a", 64)}
	orig := map[string]any{"schema": ref, "bytes": ref}
	element := func(n int) map[string]any {
		return map[string]any{"kind": "ELEMENT", "ordinal": n, "begin": ref, "terminal": ref}
	}
	whole := map[string]any{"kind": "WHOLE_PARSE_SUCCESS", "evidence": ref}
	sources := []any{map[string]any{"ordinal": 0, "availability": "EXACT_AVAILABLE", "original": orig}, map[string]any{"ordinal": 1, "availability": "EXACT_UNAVAILABLE", "original": orig}}
	base := map[string]any{"version": "definition.terminal.v2.proposed", "method": orig, "query": orig, "evaluation": orig, "result": orig, "result_shape": "LOCATION_ARRAY", "policy": orig, "profile": orig, "implementation": ref, "events": []any{element(0), element(1), whole}, "target_sources": sources, "publication": "COMMITTED_VERIFIED", "external": map[string]any{"publication_readback": ref, "final_fresh_readback": ref}, "outcome": "EXCLUDED_VERIFIED", "query_disposition": "EXCLUDED", "N": 2, "B": 2, "E": 2, "EB": 2, "ET": 2, "P": 2, "T": 1, "A": 0}
	clone := func() map[string]any {
		b, _ := json.Marshal(base)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		return m
	}
	check := func(name string, value map[string]any, pass bool) {
		t.Helper()
		err := role.Validate(value)
		if (err == nil) != pass {
			t.Fatalf("%s pass=%v err=%v", name, pass, err)
		}
	}
	check("excluded", clone(), true)
	bad := clone()
	bad["outcome"] = "COMPLETE_EMPTY"
	check("old mapping", bad, false)
	bad = clone()
	bad["events"] = []any{element(0), whole}
	check("missing ordinal terminal", bad, false)
	bad = clone()
	bad["events"] = []any{element(0), element(1)}
	check("missing whole parse", bad, false)
	bad = clone()
	bad["external"] = nil
	check("claimant verified without external", bad, false)
	bad = clone()
	bad["verified"] = true
	check("claimant verified boolean", bad, false)
	bad = clone()
	bad["target_sources"] = []any{sources[0], sources[0]}
	check("wrong ordinal", bad, false)
	empty := clone()
	empty["outcome"] = "COMPLETE_EMPTY"
	empty["result_shape"] = "NULL"
	empty["query_disposition"] = "EMPTY"
	empty["E"] = 0
	empty["EB"] = 0
	empty["ET"] = 0
	empty["P"] = 0
	empty["events"] = []any{whole}
	empty["target_sources"] = []any{}
	check("null or empty array counts", empty, true)
	bad = clone()
	bad["E"] = 0
	check("nonempty cannot empty denominator", bad, false)
	unverified := clone()
	unverified["publication"] = "COMMITTED_UNVERIFIED"
	unverified["external"] = nil
	unverified["outcome"] = nil
	unverified["query_disposition"] = nil
	unverified["T"] = 0
	check("committed unverified", unverified, true)
	bad = clone()
	bad["publication"] = "COMMITTED_UNVERIFIED"
	check("unverified cannot issue", bad, false)
	bad = clone()
	bad["policy"].(map[string]any)["bytes"].(map[string]any)["extra"] = true
	check("closed nested", bad, false)
}
