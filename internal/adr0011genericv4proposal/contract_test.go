package adr0011genericv4proposal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const schemaURI = "https://jaresty.github.io/lsp-trace/schemas/adr0011-generic-envelope-v3.schema.json"

func hash(b []byte) string { d := sha256.Sum256(b); return hex.EncodeToString(d[:]) }
func read(t *testing.T, name string, size int, sha string) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "docs", "qualification", "originals", name)
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	if len(b) != size || hash(b) != sha {
		t.Fatalf("%s original changed: %d %s", name, len(b), hash(b))
	}
	return b
}
func compiler(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	b := read(t, "adr0011-generic-envelope-v3.schema.json", 21619, "7154843a373f8f55c9723b62b1691b7b9de7b6f66ea23082d504291f137701f1")
	var v any
	if e := json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	c := jsonschema.NewCompiler()
	if e := c.AddResource(schemaURI, v); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Compile(schemaURI); e != nil {
		t.Fatal(e)
	}
	return c
}
func deps(roles ...string) []any {
	out := make([]any, 0, len(roles))
	for i, role := range roles {
		out = append(out, map[string]any{"role": role, "selector": "sel:" + role + ":" + string(rune(0x100+i)), "digest": "sha256:" + strings.Repeat("0", 64)})
	}
	return out
}
func repeated(role string, n int) []string {
	x := make([]string, n)
	for i := range x {
		x[i] = role
	}
	return x
}
func TestV4RoleSpecificAtCapAndCapPlusOne(t *testing.T) {
	c := compiler(t)
	check := func(role string, roles []string, want bool) {
		t.Helper()
		p := schemaURI + "#/$defs/" + role + "/allOf/1/properties/predecessors"
		s, e := c.Compile(p)
		if e != nil {
			t.Fatal(e)
		}
		e = s.Validate(deps(roles...))
		if (e == nil) != want {
			t.Errorf("%s roles=%d expected valid=%v: %v", role, len(roles), want, e)
		}
	}
	fixed := []string{"POLICY", "SCHEMA", "QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS"}
	check("QUERY_APPLICABILITY", []string{"QUERY", "SOURCE"}, true)
	check("QUERY_APPLICABILITY", []string{"QUERY"}, false)
	check("QUERY_APPLICABILITY", []string{"QUERY", "SOURCE", "SOURCE"}, false)
	check("REQUEST_WRITE", []string{"QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "POLICY"}, true)
	check("REQUEST_WRITE", []string{"QUERY", "CAPABILITY_EVENTS", "POLICY"}, false)
	check("REQUEST_WRITE", []string{"QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "POLICY", "PROCESS"}, true)
	check("REQUEST_WRITE", []string{"QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "POLICY", "PROCESS", "SOURCE"}, false)
	check("TARGET_EVENTS", append([]string{"RESULT_READ"}, repeated("SOURCE", 255)...), true)
	check("TARGET_EVENTS", append([]string{"RESULT_READ"}, repeated("SOURCE", 256)...), false)
	check("TARGET_EVENTS", append([]string{"RESULT_READ", "RESULT_READ"}, repeated("SOURCE", 254)...), false)
	terminal := append(append([]string{}, fixed...), repeated("SOURCE", 256)...)
	check("TERMINAL", terminal, true)
	check("TERMINAL", append(append([]string{}, terminal...), "PROCESS"), false)
	check("TERMINAL", append(append([]string{}, terminal...), "SOURCE"), false)
	for _, role := range fixed {
		lost := []string{}
		for _, r := range terminal {
			if r != role {
				lost = append(lost, r)
			}
		}
		check("TERMINAL", lost, false)
	}
	readback := append(append([]string{"TERMINAL"}, terminal...), "PROCESS")
	check("READBACK", readback, true)
	check("READBACK", append(append([]string{}, readback...), "SOURCE"), false)
	check("READBACK", append(append([]string{}, readback...), "PROCESS"), false)
}
func TestV4ExchangePendingSuccessErrorShapes(t *testing.T) {
	c := compiler(t)
	s, e := c.Compile(schemaURI + "#/$defs/capabilityExchange")
	if e != nil {
		t.Fatal(e)
	}
	frame := map[string]any{"length": 1, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "held:frame"}
	base := map[string]any{"request_direction": "SERVER_TO_CLIENT", "request_frame": frame, "request_id": 1, "request_method": "client/registerCapability", "request_params": map[string]any{"registrations": []any{}}, "request_frame_ordinal": 1, "response_direction": "CLIENT_TO_SERVER", "response_frame": frame, "response_id": 1, "response_frame_ordinal": 2, "response_status": "SUCCESS", "response_result": nil, "response_error": nil, "target_write_frame_ordinal": 10}
	copyMap := func() map[string]any {
		b, _ := json.Marshal(base)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		return m
	}
	check := func(name string, m map[string]any, want bool) {
		t.Helper()
		err := s.Validate(m)
		if (err == nil) != want {
			t.Errorf("%s valid=%v: %v", name, want, err)
		}
	}
	check("successful response", copyMap(), true)
	pending := copyMap()
	pending["response_direction"] = nil
	pending["response_frame"] = nil
	pending["response_id"] = nil
	pending["response_frame_ordinal"] = nil
	pending["response_status"] = "PENDING"
	check("explicit pending", pending, true)
	missing := copyMap()
	delete(missing, "response_frame")
	check("missing response field", missing, false)
	mismatchShape := copyMap()
	mismatchShape["response_id"] = true
	check("boolean response ID", mismatchShape, false)
	errResp := copyMap()
	errResp["response_status"] = "ERROR"
	errResp["response_error"] = map[string]any{"code": -1, "message": "failed"}
	check("error response", errResp, true)
	invalidError := copyMap()
	invalidError["response_status"] = "ERROR"
	check("error missing error object", invalidError, false)
	reversed := copyMap()
	reversed["request_direction"] = "CLIENT_TO_SERVER"
	check("register wrong direction", reversed, false)
	init := copyMap()
	init["request_method"] = "initialize"
	init["request_direction"] = "CLIENT_TO_SERVER"
	init["response_direction"] = "SERVER_TO_CLIENT"
	check("initialize pair", init, true)
	// Cross-field ID equality and ordinal order deliberately require verifier-owned replay.
}
func TestV4TransactionRoleCardinality(t *testing.T) {
	c := compiler(t)
	s, e := c.Compile(schemaURI + "#/$defs/TRANSACTION")
	if e != nil {
		t.Fatal(e)
	}
	bytesDesc := map[string]any{"length": 0, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "held:bytes"}
	id := map[string]any{"session": "s", "generation": 1, "transaction": "tx"}
	zero := "sha256:" + strings.Repeat("0", 64)
	exchange := map[string]any{"request_direction": "CLIENT_TO_SERVER", "request_frame": bytesDesc, "request_id": 1, "request_method": "initialize", "request_params": map[string]any{}, "request_frame_ordinal": 1, "response_direction": "SERVER_TO_CLIENT", "response_frame": bytesDesc, "response_id": 1, "response_frame_ordinal": 2, "response_status": "SUCCESS", "response_result": map[string]any{}, "response_error": nil, "target_write_frame_ordinal": 10}
	role := func(name string, preds []any, payload any) map[string]any {
		return map[string]any{"role": name, "identity": id, "original": bytesDesc, "predecessors": preds, "payload": payload}
	}
	pol := role("POLICY", []any{}, map[string]any{"policy_uri": "file:///policy", "policy_sha256": zero})
	schema := role("SCHEMA", []any{}, map[string]any{"schema_uri": "file:///schema", "schema_sha256": zero})
	source := func() map[string]any {
		return role("SOURCE", []any{}, map[string]any{"uri": "file:///src", "version": "v1", "custody": "OWNER_BUFFER", "source_bytes": bytesDesc})
	}
	query := role("QUERY", deps("POLICY", "SCHEMA", "SOURCE"), map[string]any{"uri": "file:///src", "version": "v1", "method": "textDocument/references", "line": 0, "character": 0, "encoding": "utf-16", "source": zero})
	qa := role("QUERY_APPLICABILITY", deps("QUERY", "SOURCE"), map[string]any{"workspace": "file:///workspace", "session": "s", "generation": 1, "transaction": "tx", "query_selector": "q", "query_source_selector": "src", "uri_bytes": bytesDesc, "language_id_present": false, "language_id_bytes": nil, "document_kind": "ORDINARY", "notebook_type_bytes": nil, "notebook_uri_bytes": nil, "notebook_source_selector": nil})
	cap := role("CAPABILITY_EVENTS", []any{}, map[string]any{"initialize": exchange, "initialized_notification": map[string]any{"direction": "CLIENT_TO_SERVER", "frame_ordinal": 3, "frame": bytesDesc}, "events": []any{}, "target_write_frame_ordinal": 10, "observed_frames": []any{map[string]any{"direction": "CLIENT_TO_SERVER", "frame_ordinal": 1, "frame": bytesDesc}, map[string]any{"direction": "SERVER_TO_CLIENT", "frame_ordinal": 2, "frame": bytesDesc}, map[string]any{"direction": "CLIENT_TO_SERVER", "frame_ordinal": 3, "frame": bytesDesc}}})
	write := role("REQUEST_WRITE", deps("QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "POLICY"), map[string]any{"params": bytesDesc, "frame": bytesDesc, "actual_key": "key", "completed": true, "completed_frame_ordinal": 10})
	inbound := role("INBOUND_FRAMES", deps("REQUEST_WRITE"), map[string]any{"frames": []any{}, "partial_prefix": nil, "wire_bytes": 0})
	result := role("RESULT_READ", deps("INBOUND_FRAMES", "REQUEST_WRITE"), map[string]any{"actual_key": "key", "matched_frame_index": 0, "result_present": true, "result_token": bytesDesc, "parse": "WHOLE_PARSE_SUCCESS"})
	target := role("TARGET_EVENTS", deps("RESULT_READ"), map[string]any{"denominator": nil, "ordinals": []any{}})
	fixed := []string{"POLICY", "SCHEMA", "QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS"}
	terminalPreds := append(append([]string{}, fixed...), repeated("SOURCE", 256)...)
	terminal := role("TERMINAL", deps(terminalPreds...), map[string]any{"N": 0, "B": 0, "T": 0, "E": nil, "E_B": 0, "E_T": 0, "P": 0, "A": 0, "disposition": "SYNTHETIC", "publication": "NOT_COMMITTED", "authority": 0, "accepted": false, "completeness": "UNKNOWN"})
	process := role("PROCESS", []any{}, map[string]any{"state": "EXTERNAL_PROCESS_UNMEASURED", "server_name": nil, "server_version": nil, "observed_config": bytesDesc})
	readbackPreds := append(append([]string{"TERMINAL"}, terminalPreds...), "PROCESS")
	readback := role("READBACK", deps(readbackPreds...), map[string]any{"originals_verified": []any{}, "fresh_external_final": false, "correspondence": "UNAVAILABLE"})
	transaction := []any{pol, schema, query, qa, cap, write, inbound, result, target, terminal, process, readback}
	for i := 0; i < 256; i++ {
		transaction = append(transaction, source())
	}
	check := func(name string, items []any, want bool) {
		t.Helper()
		e := s.Validate(items)
		if (e == nil) != want {
			t.Errorf("%s: expected valid=%v: %v", name, want, e)
		}
	}
	check("256 SOURCE at transaction cap", transaction, true)
	capPayload := cap["payload"].(map[string]any)
	notification := capPayload["initialized_notification"]
	delete(capPayload, "initialized_notification")
	check("missing initialized notification", transaction, false)
	capPayload["initialized_notification"] = notification
	capPayload["initialized_notification"] = map[string]any{"direction": "SERVER_TO_CLIENT", "frame_ordinal": 3, "frame": bytesDesc}
	check("wrong initialized notification direction", transaction, false)
	capPayload["initialized_notification"] = notification
	check("257 SOURCE", append(append([]any{}, transaction...), source()), false)
	missingQA := []any{}
	for _, v := range transaction {
		if v.(map[string]any)["role"] != "QUERY_APPLICABILITY" {
			missingQA = append(missingQA, v)
		}
	}
	check("missing QUERY_APPLICABILITY", missingQA, false)
	check("duplicate QUERY_APPLICABILITY", append(append([]any{}, transaction...), qa), false)
}
func TestV4HistoricalPinsAndPolicyDomain(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		sha  string
	}{{"adr0011-generic-envelope-v2.schema.json", 13243, "f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e"}, {"generic-lsp-v2-selector-vectors.json", 2220, "6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e"}, {"generic-lsp-v3-selector-vectors.proposed.json", 4715, "d5a8e14631fbc10d4082ee1f8c2bfedd1b34c5821993986e301cb1f65141d0ee"}, {"generic-lsp-references-exact-v3.json", 1134, "8c76a71da5e888e278280bbb1802979b907ba7a4ad6b0aec910dfe06e24e225e"}, {"generic-lsp-definition-exact-v3.json", 1043, "52dd415cb5a331462042103014991fcd4ff43a777d45ab5a9abd7d83a45a8051"}} {
		read(t, tc.name, tc.size, tc.sha)
	}
	for _, name := range []string{"generic-lsp-references-exact-v4.json", "generic-lsp-definition-exact-v4.json"} {
		b, e := os.ReadFile(filepath.Join("..", "..", "docs", "qualification", "originals", name))
		if e != nil {
			t.Fatal(e)
		}
		var p struct {
			Domain            string `json:"selector_domain"`
			Shared            string `json:"shared_applicability_sha256"`
			Envelope          string `json:"envelope"`
			Transport         string `json:"transport"`
			Pending           string `json:"pending_response"`
			PendingUnregister string `json:"pending_unregister"`
		}
		if e = json.Unmarshal(b, &p); e != nil {
			t.Fatal(e)
		}
		if p.Domain != "ADR0011-GENERIC-EXACT/4" || p.Shared != "c73263cb1356abd9f2898e016ec31afbb6cb112924c881c0a1ef6034bb1d5a0d" || p.Envelope != schemaURI || p.Transport != "GENERIC_LSP_EXACT_TRANSPORT_V3" || p.Pending != "UNKNOWN_ONLY_AFTER_COMPLETED_INITIALIZATION_WHEN_NO_PRIOR_VERIFIED_SUPPORT_DECIDES_TARGET_AT_WRITE" || p.PendingUnregister != "PRESERVE_ACTIVE_UNTIL_SUCCESSFUL_ACKNOWLEDGEMENT" {
			t.Errorf("%s contract pointer", name)
		}
	}
	fixture, e := os.ReadFile(filepath.Join("..", "..", "docs", "qualification", "originals", "generic-lsp-v4-selector-vectors.proposed.json"))
	if e != nil {
		t.Fatal(e)
	}
	if len(fixture) != 7588 || hash(fixture) != "46a761e1ac3041aca7569018bf4062790ade6981c3ed6adb8880dce94bbdd92e" {
		t.Fatal("V4 vector fixture changed")
	}
	var f struct {
		Selectors                map[string]string `json:"selectors"`
		HistoricalCollisionCount int               `json:"historical_collision_count"`
	}
	if e = json.Unmarshal(fixture, &f); e != nil {
		t.Fatal(e)
	}
	if len(f.Selectors) != 54 || f.HistoricalCollisionCount != 0 {
		t.Fatal("V4 vector count/collision")
	}
	seen := map[string]bool{}
	for _, v := range f.Selectors {
		if seen[v] {
			t.Fatal("V4 vector duplicate")
		}
		seen[v] = true
	}
	if bytes.Equal(fixture, read(t, "generic-lsp-v3-selector-vectors.proposed.json", 4715, "d5a8e14631fbc10d4082ee1f8c2bfedd1b34c5821993986e301cb1f65141d0ee")) {
		t.Fatal("V3 substituted")
	}
}
