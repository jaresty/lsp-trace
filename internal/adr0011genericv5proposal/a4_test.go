package adr0011genericv5proposal

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func a4Role(role string, predecessors []any, payload any) map[string]any {
	return map[string]any{"role": role, "identity": map[string]any{"session": "s", "generation": 1, "transaction": "tx"}, "original": map[string]any{"length": 0, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "held:original"}, "predecessors": predecessors, "payload": payload}
}
func a4JSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func a4Deps(roles ...string) []any {
	out := make([]any, 0, len(roles))
	for i, r := range roles {
		out = append(out, map[string]any{"role": r, "selector": string(rune('a' + i)), "digest": "sha256:" + strings.Repeat("0", 64)})
	}
	return out
}

func TestA4WholeRoleRejectsInvalidPayload(t *testing.T) {
	good := a4Role("POLICY", []any{}, map[string]any{"policy_uri": "file:///policy", "policy_sha256": "sha256:" + strings.Repeat("0", 64)})
	if e := A4Validate("POLICY", a4JSON(t, good)); e != nil {
		t.Fatalf("valid whole POLICY envelope rejected: %v", e)
	}
	bad := a4Role("POLICY", []any{}, map[string]any{"policy_uri": "file:///policy"})
	if e := A4Validate("POLICY", a4JSON(t, bad)); e == nil {
		t.Fatal("missing policy digest accepted")
	}
}
func TestA4RolePredecessorsAndWholeSchema(t *testing.T) {
	bytesDesc := map[string]any{"length": 0, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "held:bytes"}
	z := "sha256:" + strings.Repeat("0", 64)
	cases := []struct {
		role    string
		preds   []string
		payload any
	}{
		{"POLICY", nil, map[string]any{"policy_uri": "file:///p", "policy_sha256": z}},
		{"SCHEMA", nil, map[string]any{"schema_uri": "file:///s", "schema_sha256": z}},
		{"SOURCE", nil, map[string]any{"uri": "file:///s", "version": "v", "custody": "OWNER_BUFFER", "source_bytes": bytesDesc}},
		{"PROCESS", nil, map[string]any{"state": "EXTERNAL_PROCESS_UNMEASURED", "server_name": nil, "server_version": nil, "observed_config": bytesDesc}},
		{"QUERY", []string{"POLICY", "SCHEMA", "SOURCE"}, map[string]any{"uri": "file:///s", "version": "v", "method": "textDocument/references", "line": 0, "character": 0, "encoding": "utf-16", "source": z}},
		{"QUERY_APPLICABILITY", []string{"QUERY", "SOURCE"}, map[string]any{"workspace": "file:///w", "session": "s", "generation": 1, "transaction": "tx", "query_selector": "q", "query_source_selector": "src", "uri_bytes": bytesDesc, "language_id_present": false, "language_id_bytes": nil, "document_kind": "ORDINARY", "notebook_type_bytes": nil, "notebook_uri_bytes": nil, "notebook_source_selector": nil}},
		{"REQUEST_WRITE", []string{"QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "POLICY"}, map[string]any{"params": bytesDesc, "frame": bytesDesc, "actual_key": "key", "completed": true, "completed_frame_ordinal": 10}},
		{"INBOUND_FRAMES", []string{"REQUEST_WRITE"}, map[string]any{"frames": []any{}, "partial_prefix": nil, "wire_bytes": 0}},
		{"RESULT_READ", []string{"INBOUND_FRAMES", "REQUEST_WRITE"}, map[string]any{"actual_key": "key", "matched_frame_index": 0, "result_present": false, "result_token": nil, "parse": "MALFORMED"}},
		{"TARGET_EVENTS", []string{"RESULT_READ"}, map[string]any{"denominator": nil, "ordinals": []any{}}},
	}
	exchange := map[string]any{"request_direction": "CLIENT_TO_SERVER", "request_frame": bytesDesc, "request_id": 1, "request_method": "initialize", "request_params": map[string]any{}, "request_frame_ordinal": 1, "response_direction": "SERVER_TO_CLIENT", "response_frame": bytesDesc, "response_id": 1, "response_frame_ordinal": 2, "response_status": "SUCCESS", "response_result": map[string]any{}, "response_error": nil, "target_write_frame_ordinal": 10}
	cases = append(cases, struct {
		role    string
		preds   []string
		payload any
	}{"CAPABILITY_EVENTS", nil, map[string]any{"initialize": exchange, "initialized_notification": map[string]any{"direction": "CLIENT_TO_SERVER", "frame_ordinal": 3, "frame": bytesDesc}, "events": []any{}, "target_write_frame_ordinal": 10, "observed_frames": []any{}}})
	fixed := []string{"POLICY", "SCHEMA", "QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS"}
	terminalPayload := map[string]any{"N": 0, "B": 0, "T": 0, "E": nil, "E_B": 0, "E_T": 0, "P": 0, "A": 0, "disposition": "SYNTHETIC", "publication": "NOT_COMMITTED", "authority": 0, "accepted": false, "completeness": "UNKNOWN"}
	terminal := append(append([]string{}, fixed...), "SOURCE")
	cases = append(cases, struct {
		role    string
		preds   []string
		payload any
	}{"TERMINAL", terminal, terminalPayload})
	readback := append(append([]string{}, terminal...), "TERMINAL")
	cases = append(cases, struct {
		role    string
		preds   []string
		payload any
	}{"READBACK", readback, map[string]any{"originals_verified": []any{}, "fresh_external_final": false, "correspondence": "UNAVAILABLE"}})
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			good := a4Role(tc.role, a4Deps(tc.preds...), tc.payload)
			if e := A4Validate(tc.role, a4JSON(t, good)); e != nil {
				t.Fatalf("valid complete role: %v", e)
			}
			good["payload"] = map[string]any{"extraneous": true}
			if e := A4Validate(tc.role, a4JSON(t, good)); e == nil {
				t.Fatal("wrong payload accepted")
			}
		})
	}
	role := func(name string, preds []string, payload any) map[string]any {
		return a4Role(name, a4Deps(preds...), payload)
	}
	check := func(name string, preds []string, payload any, want bool) {
		t.Helper()
		e := A4Validate(name, a4JSON(t, role(name, preds, payload)))
		if (e == nil) != want {
			t.Errorf("%s predecessors=%d valid=%v: %v", name, len(preds), want, e)
		}
	}
	for _, n := range []int{256, 257} {
		p := append([]string{"RESULT_READ"}, a4Repeat("SOURCE", n-1)...)
		check("TARGET_EVENTS", p, map[string]any{"denominator": nil, "ordinals": []any{}}, n == 256)
	}
	check("QUERY_APPLICABILITY", []string{"QUERY"}, cases[5].payload, false)
	check("QUERY_APPLICABILITY", []string{"QUERY", "SOURCE", "SOURCE"}, cases[5].payload, false)
	check("REQUEST_WRITE", []string{"QUERY", "CAPABILITY_EVENTS", "POLICY"}, cases[6].payload, false)
	check("REQUEST_WRITE", []string{"QUERY", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS", "POLICY", "PROCESS"}, cases[6].payload, true)
	for _, n := range []int{265, 266} {
		p := append(append([]string{}, fixed...), a4Repeat("SOURCE", n-len(fixed))...)
		check("TERMINAL", p, terminalPayload, n == 265)
	}
	for _, n := range []int{267, 268} {
		p := append([]string{"TERMINAL"}, fixed...)
		p = append(p, a4Repeat("SOURCE", n-len(p)-1)...)
		p = append(p, "PROCESS")
		check("READBACK", p, cases[len(cases)-1].payload, n == 267)
	}
	goodTransaction := []any{}
	for _, tc := range cases {
		if tc.role != "READBACK" && tc.role != "TERMINAL" {
			goodTransaction = append(goodTransaction, role(tc.role, tc.preds, tc.payload))
		}
	}
	goodTransaction = append(goodTransaction, role("TERMINAL", terminal, terminalPayload))
	if e := A4Validate("TRANSACTION", a4JSON(t, goodTransaction)); e != nil {
		t.Fatalf("complete transaction: %v", e)
	}
	goodTransaction = append(goodTransaction, role("SOURCE", nil, cases[2].payload))
	if e := A4Validate("TRANSACTION", a4JSON(t, goodTransaction)); e != nil {
		t.Fatalf("second SOURCE transaction should remain schema-valid: %v", e)
	}
	if e := A4Validate("TRANSACTION", a4JSON(t, []any{})); e == nil {
		t.Fatal("empty transaction accepted")
	}
	// Ten fixed roles, 256 SOURCEs, optional PROCESS and READBACK: 268 records.
	for i := 2; i < 256; i++ {
		goodTransaction = append(goodTransaction, role("SOURCE", nil, cases[2].payload))
	}
	if len(goodTransaction) != 267 {
		t.Fatalf("pre-readback transaction count %d, want 267", len(goodTransaction))
	}
	if e := A4Validate("TRANSACTION", a4JSON(t, goodTransaction)); e != nil {
		t.Fatalf("256 SOURCEs with optional PROCESS rejected: %v", e)
	}
	readbackRecord := role("READBACK", readback, cases[len(cases)-1].payload)
	complete := append(append([]any{}, goodTransaction...), readbackRecord)
	if len(complete) != 268 || A4Validate("TRANSACTION", a4JSON(t, complete)) != nil {
		t.Fatal("268-record complete transaction rejected")
	}
	tooMany := append(append([]any{}, complete...), role("SOURCE", nil, cases[2].payload))
	if e := A4Validate("TRANSACTION", a4JSON(t, tooMany)); e == nil {
		t.Fatal("269-record transaction with 257 SOURCEs accepted")
	}
	tooManySources := append(append([]any{}, goodTransaction...), role("SOURCE", nil, cases[2].payload))
	if e := A4Validate("TRANSACTION", a4JSON(t, tooManySources)); e == nil {
		t.Fatal("257 SOURCEs accepted inside 268-record transaction")
	}
}
func a4Repeat(s string, n int) []string {
	x := make([]string, n)
	for i := range x {
		x[i] = s
	}
	return x
}

func TestA4PinnedSelectorSubset(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("..", "..", "docs", "qualification", "originals", "generic-lsp-v5-selector-vectors.proposed.json"))
	if e != nil {
		t.Fatal(e)
	}
	if hash(raw) != "8ecb9eee694ee58fa79f6c50c38c19dc437cedca92ae6484a68ff1748f09c970" {
		t.Fatal("V5 vector fixture pin changed")
	}
	var fixture struct {
		Selectors map[string]string `json:"selectors"`
	}
	if e = json.Unmarshal(raw, &fixture); e != nil {
		t.Fatal(e)
	}
	prev, e := os.ReadFile(filepath.Join("..", "..", "docs", "qualification", "originals", "generic-lsp-v2-selector-vectors.json"))
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Query string `json:"query_artifact_utf8_hex"`
		Event string `json:"event_artifact_utf8_hex"`
	}
	if e = json.Unmarshal(prev, &v); e != nil {
		t.Fatal(e)
	}
	query, e := hex.DecodeString(v.Query)
	if e != nil {
		t.Fatal(e)
	}
	event, e := hex.DecodeString(v.Event)
	if e != nil {
		t.Fatal(e)
	}
	for _, method := range []string{"references", "definition"} {
		for _, tc := range []struct {
			name, role string
			artifact   []byte
			suffix     []string
		}{
			{"query_version1_line0", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}},
			{"query_version2_line0", "QUERY", query, []string{"file:///a", "buffer:v2", hash([]byte("a")), "utf-16", "0", "0"}},
			{"query_version1_line1", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "1", "0"}},
			{"event_ordinal0", "TARGET_EVENTS", event, []string{"observed-key-1", hash([]byte("[]")), "0"}},
			{"event_ordinal1", "TARGET_EVENTS", event, []string{"observed-key-1", hash([]byte("[]")), "1"}},
		} {
			got, e := A4Selector(method, tc.role, "s", "1", tc.artifact, tc.suffix...)
			if e != nil || got != fixture.Selectors[method+"_"+tc.name] {
				t.Errorf("%s/%s selector: %s %v", method, tc.name, got, e)
			}
		}
	}
}

func TestA4ExchangeConditionalShape(t *testing.T) {
	frame := map[string]any{"length": 1, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "held:f"}
	request := map[string]any{"request_direction": "CLIENT_TO_SERVER", "request_frame": frame, "request_id": 1, "request_method": "initialize", "request_params": map[string]any{}, "request_frame_ordinal": 0, "response_direction": nil, "response_frame": nil, "response_id": nil, "response_frame_ordinal": nil, "response_status": "PENDING", "response_result": nil, "response_error": nil, "target_write_frame_ordinal": 4}
	payload := map[string]any{"initialize": request, "initialized_notification": map[string]any{"direction": "CLIENT_TO_SERVER", "frame_ordinal": 2, "frame": frame}, "events": []any{}, "target_write_frame_ordinal": 4, "observed_frames": []any{}}
	check := func(valid bool, label string) {
		t.Helper()
		err := A4Validate("CAPABILITY_EVENTS", a4JSON(t, a4Role("CAPABILITY_EVENTS", []any{}, payload)))
		if (err == nil) != valid {
			t.Fatalf("%s: shape valid=%v want=%v: %v", label, err == nil, valid, err)
		}
	}
	check(true, "pending initialize shape is not B4 chronology")
	request["response_frame"] = frame
	check(false, "pending with response frame")
	request["response_frame"] = nil
	request["response_status"] = "ERROR"
	request["response_frame"] = frame
	request["response_direction"] = "SERVER_TO_CLIENT"
	request["response_frame_ordinal"] = 1
	request["response_error"] = map[string]any{"code": -32603, "message": "failed"}
	request["response_id"] = nil
	check(false, "null error response ID is not a matching exchange shape")
	request["response_id"] = 1
	check(true, "typed error response shape")
}

func TestA4QueryAllowsNonFileURI(t *testing.T) {
	artifact := []byte("held-query")
	if _, err := A4Selector("references", "QUERY", "s", "1", artifact, "https://example.org/source.go", "buffer:v1", hash([]byte("source")), "utf-16", "0", "0"); err != nil {
		t.Fatalf("valid non-file query URI rejected: %v", err)
	}
}

func TestA4NullErrorIDIsNotCorrespondence(t *testing.T) {
	frame := map[string]any{"length": 0, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "held:frame"}
	exchange := map[string]any{"request_direction": "CLIENT_TO_SERVER", "request_frame": frame, "request_id": 1, "request_method": "initialize", "request_params": map[string]any{}, "request_frame_ordinal": 0, "response_direction": "SERVER_TO_CLIENT", "response_frame": frame, "response_id": 1, "response_frame_ordinal": 1, "response_status": "ERROR", "response_result": nil, "response_error": map[string]any{"code": -1, "message": "failed"}, "target_write_frame_ordinal": 4}
	payload := map[string]any{"initialize": exchange, "initialized_notification": map[string]any{"direction": "CLIENT_TO_SERVER", "frame_ordinal": 2, "frame": frame}, "events": []any{}, "target_write_frame_ordinal": 4, "observed_frames": []any{}}
	if e := A4Validate("CAPABILITY_EVENTS", a4JSON(t, a4Role("CAPABILITY_EVENTS", []any{}, payload))); e != nil {
		t.Fatalf("valid ERROR exchange rejected: %v", e)
	}
	// A raw JSON-RPC error frame may have a null ID, but it cannot be
	// represented as verified matching exchange correspondence.
	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	response := []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-1,"message":"failed"}}`)
	wire := func(b []byte) []byte { return append([]byte("Content-Length: "+strconv.Itoa(len(b))+"\r\n\r\n"), b...) }
	if _, e := ValidateFramedPair(wire(request), wire(response)); e == nil {
		t.Fatal("null ERROR response ID established correspondence")
	}
	exchange["response_id"] = nil
	if e := A4Validate("CAPABILITY_EVENTS", a4JSON(t, a4Role("CAPABILITY_EVENTS", []any{}, payload))); e == nil {
		t.Fatal("unmatched null exchange ID accepted")
	}
	exchange["response_id"] = 1
	exchange["request_id"] = nil
	if e := A4Validate("CAPABILITY_EVENTS", a4JSON(t, a4Role("CAPABILITY_EVENTS", []any{}, payload))); e == nil {
		t.Fatal("null request ID incorrectly accepted")
	}
	// This schema-only guard does not assert typed correspondence or invoke B4.
}

func TestA4RejectsMalformedTransactionSuffix(t *testing.T) {
	if _, err := A4Selector("references", "SOURCE", "s", "1", []byte("source-artifact"), "sha256:bad"); err == nil {
		t.Fatal("malformed held transaction selector accepted")
	}
}

func TestA4OriginalIndependentOfCallerCWD(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := A4Original("schema"); err != nil {
		t.Fatalf("pinned original depends on caller cwd: %v", err)
	}
}

func TestA4OriginalCloneAndSelectorInput(t *testing.T) {
	for _, pin := range []struct {
		role   string
		length int
		digest string
	}{
		{"schema", 21409, "df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3"},
		{"transport", 1784, "f3e25fab98cc4b8b96b76978cf56220ccc81f4ef41455ab0401041ea281d0bc8"},
		{"shared", 2247, "c73263cb1356abd9f2898e016ec31afbb6cb112924c881c0a1ef6034bb1d5a0d"},
		{"references", 2210, "7c46893fecf0dda7a4942b8f33d92f0c2f49793d003c7eab4e8995aeddf8da36"},
		{"definition", 2119, "da62f5a161532fcb7c79dd93b0dca940a460c6e0c5fab83e8987de21c91b4dd3"},
	} {
		body, e := A4Original(pin.role)
		if e != nil || len(body) != pin.length || hash(body) != pin.digest {
			t.Fatalf("%s pin: %v", pin.role, e)
		}
	}
	a, e := A4Original("schema")
	if e != nil {
		t.Fatal(e)
	}
	b, e := A4Original("schema")
	if e != nil {
		t.Fatal(e)
	}
	if len(a) != 21409 || hash(a) != "df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3" {
		t.Fatal("schema pin")
	}
	a[0] ^= 1
	if a[0] == b[0] {
		t.Fatal("original returned without clone")
	}
	if _, e = A4Original("unknown"); e == nil {
		t.Fatal("unknown original accepted")
	}
	if _, e = A4Selector("not-a-method", "QUERY", "s", "1", []byte("x")); e == nil {
		t.Fatal("unknown method accepted")
	}
	if _, e = A4Selector("references", "not-a-role", "s", "1", []byte("x")); e == nil {
		t.Fatal("unknown role accepted")
	}
}
