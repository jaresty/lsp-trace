package adr0011genericv5proposal

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Independently drive the private A4 implementation over every frozen vector.
// The frozen proposal comparator remains an independent, unchanged test.
func TestA4All54FrozenVectors(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "qualification", "originals")
	load := func(name string) []byte {
		t.Helper()
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var fixture struct {
		Selectors map[string]string `json:"selectors"`
	}
	if e := json.Unmarshal(load("generic-lsp-v5-selector-vectors.proposed.json"), &fixture); e != nil {
		t.Fatal(e)
	}
	var prior struct {
		Query string `json:"query_artifact_utf8_hex"`
		Event string `json:"event_artifact_utf8_hex"`
	}
	if e := json.Unmarshal(load("generic-lsp-v2-selector-vectors.json"), &prior); e != nil {
		t.Fatal(e)
	}
	query, e := hex.DecodeString(prior.Query)
	if e != nil {
		t.Fatal(e)
	}
	event, e := hex.DecodeString(prior.Event)
	if e != nil {
		t.Fatal(e)
	}
	var sources struct {
		Cases map[string]sourceCase `json:"cases"`
	}
	if e := json.Unmarshal(load("generic-lsp-source-selector-v2-proposed-vectors.json"), &sources); e != nil {
		t.Fatal(e)
	}
	schema, e := A4Original("schema")
	if e != nil {
		t.Fatal(e)
	}
	transport, e := A4Original("transport")
	if e != nil {
		t.Fatal(e)
	}
	seen := make(map[string]bool)
	compare := func(method, key, role string, art []byte, selected bool, sch, pol []byte, suffix ...string) {
		t.Helper()
		if seen[key] {
			t.Fatalf("duplicate A4 vector %s", key)
		}
		want, ok := fixture.Selectors[key]
		if !ok {
			t.Fatalf("missing frozen A4 vector %s", key)
		}
		got, err := a4SelectorWithIdentity(method, role, "s", "1", art, sch, transport, pol, suffix...)
		if err != nil || got != want {
			t.Fatalf("%s private preimage: got %s want %s err %v", key, got, want, err)
		}
		pinned, pinErr := A4Selector(method, role, "s", "1", art, suffix...)
		if selected && (pinErr != nil || pinned != want) {
			t.Fatalf("%s pinned original: got %s want %s err %v", key, pinned, want, pinErr)
		}
		if !selected && pinErr == nil && pinned == want {
			t.Fatalf("%s substituted original selected as pinned", key)
		}
		seen[key] = true
	}
	source := func(method, key string, c sourceCase, sch, pol []byte, selected bool) {
		t.Helper()
		if len(c.Identity) != 3 || c.Version == nil || c.Content == nil {
			t.Fatalf("%s absent SOURCE", key)
		}
		var session, transaction string
		var generation uint64
		if e := json.Unmarshal(c.Identity[0], &session); e != nil {
			t.Fatal(e)
		}
		if e := json.Unmarshal(c.Identity[1], &generation); e != nil {
			t.Fatal(e)
		}
		if e := json.Unmarshal(c.Identity[2], &transaction); e != nil {
			t.Fatal(e)
		}
		if session == "" || generation == 0 || transaction == "" {
			t.Fatalf("%s invalid held TX", key)
		}
		uri, e := hex.DecodeString(c.URI)
		if e != nil {
			t.Fatal(e)
		}
		version, e := hex.DecodeString(*c.Version)
		if e != nil {
			t.Fatal(e)
		}
		content, e := hex.DecodeString(*c.Content)
		if e != nil {
			t.Fatal(e)
		}
		tx := selector("ADR0011-GENERIC-TRANSACTION/1", session, strconv.FormatUint(generation, 10), transaction)
		var art []byte
		for _, f := range [][]byte{[]byte("ADR0011-GENERIC-SOURCE-ARTIFACT/1"), []byte(tx), uri, []byte("present"), version, []byte(strconv.Itoa(len(content))), []byte(hash(content)), []byte(c.Custody)} {
			art = append(art, lp(f)...)
		}
		if seen[key] {
			t.Fatalf("duplicate A4 SOURCE vector %s", key)
		}
		want, ok := fixture.Selectors[key]
		if !ok {
			t.Fatalf("missing frozen A4 SOURCE vector %s", key)
		}
		gen := strconv.FormatUint(generation, 10)
		got, err := a4SelectorWithIdentity(method, "SOURCE", session, gen, art, sch, transport, pol, tx)
		if err != nil || got != want {
			t.Fatalf("%s private SOURCE preimage got %s want %s err %v", key, got, want, err)
		}
		pinned, pinErr := A4Selector(method, "SOURCE", session, gen, art, tx)
		if selected && (pinErr != nil || pinned != want) {
			t.Fatalf("%s pinned SOURCE got %s want %s err %v", key, pinned, want, pinErr)
		}
		if !selected && pinErr == nil && pinned == want {
			t.Fatalf("%s substituted SOURCE selected as pinned", key)
		}
		seen[key] = true
	}
	for _, method := range []string{"references", "definition"} {
		policy, e := A4Original(method)
		if e != nil {
			t.Fatal(e)
		}
		for _, tc := range []struct {
			name, role string
			art        []byte
			suffix     []string
			sch, pol   []byte
		}{
			{"query_version1_line0", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, nil, nil},
			{"query_version2_line0", "QUERY", query, []string{"file:///a", "buffer:v2", hash([]byte("a")), "utf-16", "0", "0"}, nil, nil},
			{"query_version1_line1", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "1", "0"}, nil, nil},
			{"event_ordinal0", "TARGET_EVENTS", event, []string{"observed-key-1", hash([]byte("[]")), "0"}, nil, nil},
			{"event_ordinal1", "TARGET_EVENTS", event, []string{"observed-key-1", hash([]byte("[]")), "1"}, nil, nil},
			{"query_schema_substitution", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, append(append([]byte(nil), schema...), '\n'), nil},
			{"query_policy_substitution", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, nil, append(append([]byte(nil), policy...), '\n')},
		} {
			sch, pol := schema, policy
			if tc.sch != nil {
				sch = tc.sch
			}
			if tc.pol != nil {
				pol = tc.pol
			}
			compare(method, method+"_"+tc.name, tc.role, tc.art, tc.sch == nil && tc.pol == nil, sch, pol, tc.suffix...)
		}
		for name, c := range sources.Cases {
			source(method, method+"_source_"+name, c, schema, policy, true)
		}
		for name, change := range map[string]string{"schema_lf": "schema", "policy_lf": "policy"} {
			sch, pol := schema, policy
			if change == "schema" {
				sch = append(append([]byte(nil), schema...), '\n')
			} else {
				pol = append(append([]byte(nil), policy...), '\n')
			}
			source(method, method+"_source_"+name, sources.Cases["query"], sch, pol, false)
		}
		tx := selector("ADR0011-GENERIC-TRANSACTION/1", "s", "1", "tx-1")
		querySel := fixture.Selectors[method+"_query_version1_line0"]
		srcSel := fixture.Selectors[method+"_source_query"]
		for _, tc := range []struct {
			name      string
			uri, lang []byte
			kind      string
			nt, nu    []byte
			ns        string
		}{
			{"ordinary", []byte("file:///query"), []byte("go"), "ORDINARY", nil, nil, ""},
			{"uri_byte", []byte("file:///querY"), []byte("go"), "ORDINARY", nil, nil, ""},
			{"notebook", []byte("file:///cell"), []byte("python"), "NOTEBOOK_CELL", []byte("jupyter"), []byte("file:///book"), "sha256:" + hash([]byte("held-notebook-source"))},
		} {
			presence := "absent"
			if tc.lang != nil {
				presence = "present"
			}
			npt, npu := "absent", "absent"
			if tc.nt != nil {
				npt = "present"
			}
			if tc.nu != nil {
				npu = "present"
			}
			var art []byte
			for _, f := range [][]byte{[]byte("ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1"), []byte(tx), []byte(querySel), []byte(srcSel), tc.uri, []byte(presence), tc.lang, []byte(tc.kind), []byte(npt), tc.nt, []byte(npu), tc.nu, []byte(tc.ns), []byte("file:///workspace"), []byte("s"), []byte("1"), []byte("tx-1")} {
				art = append(art, lp(f)...)
			}
			compare(method, method+"_query_applicability_"+tc.name, "QUERY_APPLICABILITY", art, true, schema, policy, tx)
		}
		wire := func(v any) []byte {
			t.Helper()
			body, e := json.Marshal(v)
			if e != nil {
				t.Fatal(e)
			}
			return append([]byte("Content-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
		}
		register := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "client/registerCapability", "params": map[string]any{"registrations": []any{map[string]any{"id": "r", "method": "textDocument/" + method, "registerOptions": map[string]any{"documentSelector": []any{map[string]any{"language": "go"}}}}}}})
		unregister := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "client/unregisterCapability", "params": map[string]any{"unregisterations": []any{map[string]any{"id": "r", "method": "textDocument/" + method}}}})
		initialize := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
		initSuccess := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"capabilities": map[string]any{}}})
		initialized := wire(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})
		success := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "result": nil})
		failure := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "error": map[string]any{"code": -32603, "message": "failed"}})
		for _, tc := range []struct {
			name, methodName, status, ordinal, write string
			request, response                        []byte
		}{
			{"success", "client/registerCapability", "SUCCESS", "4", "10", register, success},
			{"error", "client/registerCapability", "ERROR", "4", "10", register, failure},
			{"pending", "client/registerCapability", "PENDING", "-1", "10", register, nil},
			{"initialize_success", "initialize", "SUCCESS", "1", "10", initialize, initSuccess},
			{"unregister_success", "client/unregisterCapability", "SUCCESS", "4", "10", unregister, success},
			{"late_success", "client/registerCapability", "SUCCESS", "11", "10", register, success},
		} {
			count := "2"
			if tc.name == "initialize_success" {
				count = "1"
			}
			fields := [][]byte{[]byte("ADR0011-GENERIC-CAPABILITY-EXCHANGE-ARTIFACT/2"), []byte(tx), []byte(tc.write), []byte(count), []byte("CLIENT_TO_SERVER"), initialize, []byte("number:1"), []byte("initialize"), []byte("0"), []byte("SERVER_TO_CLIENT"), initSuccess, []byte("number:1"), []byte("1"), []byte("SUCCESS")}
			observed := [][]byte{[]byte("CLIENT_TO_SERVER"), []byte("0"), initialize, []byte("SERVER_TO_CLIENT"), []byte("1"), initSuccess, []byte("CLIENT_TO_SERVER"), []byte("2"), initialized}
			if tc.name != "initialize_success" {
				responseDir, responseID := "CLIENT_TO_SERVER", "number:1"
				if tc.response == nil {
					responseDir, responseID = "ABSENT", "ABSENT"
				}
				fields = append(fields, []byte("SERVER_TO_CLIENT"), tc.request, []byte("number:1"), []byte(tc.methodName), []byte("3"), []byte(responseDir), tc.response, []byte(responseID), []byte(tc.ordinal), []byte(tc.status))
				observed = append(observed, []byte("SERVER_TO_CLIENT"), []byte("3"), tc.request)
				if tc.response != nil {
					observed = append(observed, []byte("CLIENT_TO_SERVER"), []byte(tc.ordinal), tc.response)
				}
			}
			fields = append(fields, []byte("CLIENT_TO_SERVER"), []byte("2"), initialized, []byte(strconv.Itoa(len(observed)/3)))
			fields = append(fields, observed...)
			var art []byte
			for _, f := range fields {
				art = append(art, lp(f)...)
			}
			compare(method, method+"_capability_exchange_"+tc.name, "CAPABILITY_EVENTS", art, true, schema, policy, tx)
		}
	}
	if len(seen) != 54 || len(fixture.Selectors) != 54 {
		t.Fatalf("private A4 selector coverage %d/54, fixture %d", len(seen), len(fixture.Selectors))
	}
	for key := range fixture.Selectors {
		if !seen[key] {
			t.Fatalf("unexercised A4 frozen vector %s", key)
		}
	}
}
