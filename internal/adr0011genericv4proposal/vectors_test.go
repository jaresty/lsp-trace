package adr0011genericv4proposal

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// Test-only independent recomputation, never imported by a selector issuer.
type sourceCase struct {
	Identity []json.RawMessage `json:"identity"`
	URI      string            `json:"uri_hex"`
	Version  *string           `json:"version_hex"`
	Content  *string           `json:"content_hex"`
	Custody  string            `json:"custody"`
}
type v4Fixture struct {
	Domain    string            `json:"domain"`
	Selectors map[string]string `json:"selectors"`
	Pins      map[string]struct {
		Length int    `json:"length"`
		SHA    string `json:"sha256"`
	} `json:"pins"`
}

func lp(b []byte) []byte {
	out := make([]byte, 8+len(b))
	binary.BigEndian.PutUint64(out[:8], uint64(len(b)))
	copy(out[8:], b)
	return out
}
func selector(fields ...string) string {
	var data []byte
	for _, f := range fields {
		data = append(data, lp([]byte(f))...)
	}
	return "sha256:" + hash(data)
}
func sourceSelector(method string, c sourceCase, schema, transport, policy []byte) (string, error) {
	if len(c.Identity) != 3 || c.Version == nil || c.Content == nil {
		return "", fmt.Errorf("absent source/identity")
	}
	var session, transaction string
	var generation uint64
	if err := json.Unmarshal(c.Identity[0], &session); err != nil {
		return "", err
	}
	if err := json.Unmarshal(c.Identity[1], &generation); err != nil {
		return "", err
	}
	if err := json.Unmarshal(c.Identity[2], &transaction); err != nil {
		return "", err
	}
	if session == "" || transaction == "" || generation == 0 {
		return "", fmt.Errorf("invalid transaction")
	}
	uri, err := hex.DecodeString(c.URI)
	if err != nil {
		return "", err
	}
	version, err := hex.DecodeString(*c.Version)
	if err != nil {
		return "", err
	}
	content, err := hex.DecodeString(*c.Content)
	if err != nil {
		return "", err
	}
	if len(uri) == 0 || version == nil || content == nil {
		return "", fmt.Errorf("missing bytes")
	}
	if !utf8.Valid(uri) || !utf8.Valid(version) {
		return "", fmt.Errorf("invalid UTF-8 source identity")
	}
	switch c.Custody {
	case "OWNER_BUFFER", "MANAGED_VIRTUAL", "CLEAN_REGISTERED_WORKTREE", "IMMUTABLE_SOURCE_SNAPSHOT":
	default:
		return "", fmt.Errorf("unknown custody")
	}
	tx := selector("ADR0011-GENERIC-TRANSACTION/1", session, strconv.FormatUint(generation, 10), transaction)
	var artifact []byte
	for _, field := range [][]byte{[]byte("ADR0011-GENERIC-SOURCE-ARTIFACT/1"), []byte(tx), uri, []byte("present"), version, []byte(strconv.Itoa(len(content))), []byte(hash(content)), []byte(c.Custody)} {
		artifact = append(artifact, lp(field)...)
	}
	prefix := []string{"ADR0011-GENERIC-EXACT/4", "GENERIC_LSP_" + strings.ToUpper(method) + "_EXACT_V4", "textDocument/" + method, "SOURCE", hash(schema), hash(transport), hash(policy), session, strconv.FormatUint(generation, 10), hash(artifact), tx}
	return selector(prefix...), nil
}
func TestV4AllPolicyDependentSelectorsIndependent(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "qualification", "originals")
	pins := map[string]struct {
		name string
		size int
		sha  string
	}{
		"schema":     {"adr0011-generic-envelope-v3.schema.json", 21619, "7154843a373f8f55c9723b62b1691b7b9de7b6f66ea23082d504291f137701f1"},
		"transport":  {"generic-lsp-exact-transport-v3.json", 1555, "e2d326f1424a56afa7fbaafc7417ccec52f67e03b977e44c19bd3adb6d9ae01b"},
		"shared":     {"generic-lsp-selector-applicability-v1.json", 2247, "c73263cb1356abd9f2898e016ec31afbb6cb112924c881c0a1ef6034bb1d5a0d"},
		"references": {"generic-lsp-references-exact-v4.json", 1930, "0756ed60f6ec8582f29d6e1d02aff6f112dff9e0aba977472f6833911c41c077"},
		"definition": {"generic-lsp-definition-exact-v4.json", 1839, "6155cc64a70c1d7928eb21edfec26df58a6740267141dbd94a30052cdd244a23"},
		"v2_vectors": {"generic-lsp-v2-selector-vectors.json", 2220, "6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e"},
		"v2_source":  {"generic-lsp-source-selector-v2-proposed-vectors.json", 5551, "7de0399f1fbd711abea08d26043492de4417574d8bdf13faf05577f14f48650f"},
	}
	fixtureBytes, err := os.ReadFile(filepath.Join(root, "generic-lsp-v4-selector-vectors.proposed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture v4Fixture
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Domain != "ADR0011-GENERIC-EXACT/4" || len(fixture.Selectors) != 54 || len(fixture.Pins) != 14 {
		t.Fatal("V4 vector domain/coverage")
	}
	originals := make(map[string][]byte)
	for key, pin := range pins {
		body, err := os.ReadFile(filepath.Join(root, pin.name))
		if err != nil {
			t.Fatal(err)
		}
		if len(body) != pin.size || hash(body) != pin.sha || fixture.Pins[key].Length != pin.size || fixture.Pins[key].SHA != pin.sha {
			t.Fatalf("pinned %s changed", key)
		}
		originals[key] = body
	}
	var shared struct {
		Filters struct {
			NotebookString   string `json:"notebook_string"`
			NotebookLanguage string `json:"notebook_language"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(originals["shared"], &shared); err != nil {
		t.Fatal(err)
	}
	if shared.Filters.NotebookString != "EXACT_UTF8_BYTES_OR_STAR_ALL" || shared.Filters.NotebookLanguage != "EXACT_UTF8_BYTES_OR_STAR_ALL" {
		t.Fatal("notebook string/language wildcard semantics missing from shared policy")
	}
	var previous struct {
		QueryHex  string            `json:"query_artifact_utf8_hex"`
		EventHex  string            `json:"event_artifact_utf8_hex"`
		Selectors map[string]string `json:"selectors"`
	}
	if err := json.Unmarshal(originals["v2_vectors"], &previous); err != nil {
		t.Fatal(err)
	}
	var oldSources struct {
		Cases     map[string]sourceCase `json:"cases"`
		Rejected  map[string]sourceCase `json:"rejected_inputs"`
		Selectors map[string]string     `json:"selectors"`
	}
	if err := json.Unmarshal(originals["v2_source"], &oldSources); err != nil {
		t.Fatal(err)
	}
	query, err := hex.DecodeString(previous.QueryHex)
	if err != nil {
		t.Fatal(err)
	}
	event, err := hex.DecodeString(previous.EventHex)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, method := range []string{"references", "definition"} {
		policy := originals[method]
		base := []string{"ADR0011-GENERIC-EXACT/4", "GENERIC_LSP_" + strings.ToUpper(method) + "_EXACT_V4", "textDocument/" + method}
		cases := []struct {
			name, role     string
			artifact       []byte
			extra          []string
			schema, policy []byte
		}{
			{"query_version1_line0", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, nil, nil},
			{"query_version2_line0", "QUERY", query, []string{"file:///a", "buffer:v2", hash([]byte("a")), "utf-16", "0", "0"}, nil, nil},
			{"query_version1_line1", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "1", "0"}, nil, nil},
			{"event_ordinal0", "TARGET_EVENTS", event, []string{"observed-key-1", hash([]byte("[]")), "0"}, nil, nil},
			{"event_ordinal1", "TARGET_EVENTS", event, []string{"observed-key-1", hash([]byte("[]")), "1"}, nil, nil},
			{"query_schema_substitution", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, append(append([]byte(nil), originals["schema"]...), '\n'), nil},
			{"query_policy_substitution", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, nil, append(append([]byte(nil), policy...), '\n')},
		}
		for _, tc := range cases {
			schema := originals["schema"]
			if tc.schema != nil {
				schema = tc.schema
			}
			p := policy
			if tc.policy != nil {
				p = tc.policy
			}
			fields := append(append([]string(nil), base...), tc.role, hash(schema), hash(originals["transport"]), hash(p), "s", "1", hash(tc.artifact))
			fields = append(fields, tc.extra...)
			key := method + "_" + tc.name
			got := selector(fields...)
			if fixture.Selectors[key] != got {
				t.Errorf("%s differs", key)
			}
			seen[key] = true
		}
		for name, entry := range oldSources.Cases {
			got, err := sourceSelector(method, entry, originals["schema"], originals["transport"], policy)
			if err != nil {
				t.Fatal(err)
			}
			key := method + "_source_" + name
			if fixture.Selectors[key] != got {
				t.Errorf("%s differs", key)
			}
			seen[key] = true
		}
		baseSource := oldSources.Cases["query"]
		for name, change := range map[string]string{"schema_lf": "schema", "policy_lf": "policy"} {
			schema, p := originals["schema"], policy
			if change == "schema" {
				schema = append(append([]byte(nil), schema...), '\n')
			} else {
				p = append(append([]byte(nil), policy...), '\n')
			}
			got, err := sourceSelector(method, baseSource, schema, originals["transport"], p)
			if err != nil {
				t.Fatal(err)
			}
			key := method + "_source_" + name
			if fixture.Selectors[key] != got {
				t.Errorf("%s differs", key)
			}
			seen[key] = true
		}

		tx := selector("ADR0011-GENERIC-TRANSACTION/1", "s", "1", "tx-1")
		sourceSel := fixture.Selectors[method+"_source_query"]
		querySel := fixture.Selectors[method+"_query_version1_line0"]
		qa := func(uri, language []byte, kind string, nt, nu []byte, ns string) string {
			presence := "absent"
			if language != nil {
				presence = "present"
			}
			npt, npu := "absent", "absent"
			if nt != nil {
				npt = "present"
			}
			if nu != nil {
				npu = "present"
			}
			fields := [][]byte{[]byte("ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1"), []byte(tx), []byte(querySel), []byte(sourceSel), uri, []byte(presence), language, []byte(kind), []byte(npt), nt, []byte(npu), nu, []byte(ns), []byte("file:///workspace"), []byte("s"), []byte("1"), []byte("tx-1")}
			var art []byte
			for _, f := range fields {
				art = append(art, lp(f)...)
			}
			return selector("ADR0011-GENERIC-EXACT/4", "GENERIC_LSP_"+strings.ToUpper(method)+"_EXACT_V4", "textDocument/"+method, "QUERY_APPLICABILITY", hash(originals["schema"]), hash(originals["transport"]), hash(policy), "s", "1", hash(art), tx)
		}
		for _, tc := range []struct {
			name          string
			uri, language []byte
			kind          string
			nt, nu        []byte
			ns            string
		}{
			{"ordinary", []byte("file:///query"), []byte("go"), "ORDINARY", nil, nil, ""},
			{"uri_byte", []byte("file:///querY"), []byte("go"), "ORDINARY", nil, nil, ""},
			{"notebook", []byte("file:///cell"), []byte("python"), "NOTEBOOK_CELL", []byte("jupyter"), []byte("file:///book"), "sha256:" + hash([]byte("held-notebook-source"))},
		} {
			key := method + "_query_applicability_" + tc.name
			if got := qa(tc.uri, tc.language, tc.kind, tc.nt, tc.nu, tc.ns); got != fixture.Selectors[key] {
				t.Errorf("%s independent recompute", key)
			}
			seen[key] = true
		}
		wire := func(v any) []byte {
			body, e := json.Marshal(v)
			if e != nil {
				t.Fatal(e)
			}
			return append([]byte("Content-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
		}
		// Go encoding/json sorts map keys independently of the Python generator.
		register := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "client/registerCapability", "params": map[string]any{"registrations": []any{map[string]any{"id": "r", "method": "textDocument/" + method, "registerOptions": map[string]any{"documentSelector": []any{map[string]any{"language": "go"}}}}}}})
		unregister := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "client/unregisterCapability", "params": map[string]any{"unregisterations": []any{map[string]any{"id": "r", "method": "textDocument/" + method}}}})
		initialize := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
		initSuccess := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"capabilities": map[string]any{}}})
		initialized := wire(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})
		success := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "result": nil})
		failure := wire(map[string]any{"jsonrpc": "2.0", "id": 1, "error": map[string]any{"code": -32603, "message": "failed"}})
		for _, tc := range []struct {
			name, methodName, status, ordinal string
			request, response                 []byte
			write                             string
		}{
			{"success", "client/registerCapability", "SUCCESS", "4", register, success, "10"},
			{"error", "client/registerCapability", "ERROR", "4", register, failure, "10"},
			{"pending", "client/registerCapability", "PENDING", "-1", register, nil, "10"},
			{"initialize_success", "initialize", "SUCCESS", "1", initialize, initSuccess, "10"},
			{"unregister_success", "client/unregisterCapability", "SUCCESS", "4", unregister, success, "10"},
			{"late_success", "client/registerCapability", "SUCCESS", "11", register, success, "10"},
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
			key := method + "_capability_exchange_" + tc.name
			got := selector("ADR0011-GENERIC-EXACT/4", "GENERIC_LSP_"+strings.ToUpper(method)+"_EXACT_V4", "textDocument/"+method, "CAPABILITY_EVENTS", hash(originals["schema"]), hash(originals["transport"]), hash(policy), "s", "1", hash(art), tx)
			if got != fixture.Selectors[key] {
				t.Errorf("%s independent recompute", key)
			}
			seen[key] = true
		}

		if len(oldSources.Rejected) != 7 {
			t.Fatal("missing rejected input coverage")
		}
		for name, entry := range oldSources.Rejected {
			if _, err := sourceSelector(method, entry, originals["schema"], originals["transport"], policy); err == nil {
				t.Errorf("%s/%s invalid source received selector", method, name)
			}
		}
	}
	if len(seen) != 54 {
		t.Fatalf("recomputed %d/54", len(seen))
	}
	v3raw, err := os.ReadFile(filepath.Join(root, "generic-lsp-v3-selector-vectors.proposed.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(v3raw) != 4715 || hash(v3raw) != "d5a8e14631fbc10d4082ee1f8c2bfedd1b34c5821993986e301cb1f65141d0ee" {
		t.Fatal("historical V3 vectors changed")
	}
	var priorV3 struct {
		Selectors map[string]string `json:"selectors"`
	}
	if err = json.Unmarshal(v3raw, &priorV3); err != nil {
		t.Fatal(err)
	}
	for _, sel := range fixture.Selectors {
		for _, old := range previous.Selectors {
			if sel == old {
				t.Fatal("V2 query/event selector collision")
			}
		}
		for _, old := range oldSources.Selectors {
			if sel == old {
				t.Fatal("V2 SOURCE selector collision")
			}
		}
		for _, old := range priorV3.Selectors {
			if sel == old {
				t.Fatal("V3 selector collision")
			}
		}
	}
}
