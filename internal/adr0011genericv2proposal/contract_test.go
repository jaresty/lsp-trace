package adr0011genericv2proposal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaURI = "https://jaresty.github.io/lsp-trace/schemas/adr0011-generic-envelope-v2.schema.json"

func originals(t *testing.T) string {
	t.Helper()
	_, path, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	return filepath.Join(filepath.Dir(path), "..", "..", "docs", "qualification", "originals")
}

func schemaAt(t *testing.T, fragment string) *jsonschema.Schema {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(originals(t), "adr0011-generic-envelope-v2.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaURI, v); err != nil {
		t.Fatal(err)
	}
	compiled, err := c.Compile(schemaURI + fragment)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func schema(t *testing.T) *jsonschema.Schema { return schemaAt(t, "") }

func deps(roles ...string) []any {
	out := make([]any, 0, len(roles))
	for i, role := range roles {
		out = append(out, map[string]any{"role": role, "selector": "selected:" + role + ":" + strconv.Itoa(i), "digest": "sha256:" + strings.Repeat("0", 64)})
	}
	return out
}

func manySources(count int) []string {
	out := make([]string, count)
	for i := range out {
		out[i] = "SOURCE"
	}
	return out
}

func envelope(role string, predecessors []any) map[string]any {
	original := map[string]any{"length": 1, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "test:original"}
	var payload any
	switch role {
	case "TERMINAL":
		payload = map[string]any{"N": 0, "B": 0, "T": 0, "E": 0, "E_B": 0, "E_T": 0, "P": 0, "A": 0, "disposition": "SYNTHETIC", "publication": "NOT_COMMITTED", "authority": 0, "accepted": false, "completeness": "UNKNOWN"}
	case "TARGET_EVENTS":
		payload = map[string]any{"denominator": nil, "ordinals": []any{}}
	case "READBACK":
		payload = map[string]any{"originals_verified": []any{}, "fresh_external_final": false, "correspondence": "UNAVAILABLE"}
	}
	return map[string]any{"role": role, "identity": map[string]any{"session": "s", "generation": 1, "transaction": "t"}, "original": original, "predecessors": predecessors, "payload": payload}
}

func roleEnvelope(role, method string, index int, sources int) map[string]any {
	original := map[string]any{"length": 1, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "test:original"}
	var predecessors []any
	var payload any
	switch role {
	case "POLICY":
		payload = map[string]any{"policy_uri": "https://example.test/policy", "policy_sha256": original["sha256"]}
	case "SCHEMA":
		payload = map[string]any{"schema_uri": schemaURI, "schema_sha256": original["sha256"]}
	case "SOURCE":
		payload = map[string]any{"uri": "file:///source/" + strconv.Itoa(index), "version": "v1", "custody": "OWNER_BUFFER", "source_bytes": original}
	case "QUERY":
		predecessors = deps("POLICY", "SCHEMA", "SOURCE")
		payload = map[string]any{"uri": "file:///source/0", "version": "v1", "method": method, "line": 0, "character": 0, "encoding": "utf-16", "source": original["sha256"]}
	case "CAPABILITY_EVENTS":
		payload = map[string]any{"initialize": original, "events": []any{}, "snapshot_index": 0}
	case "REQUEST_WRITE":
		predecessors = deps("QUERY", "CAPABILITY_EVENTS", "POLICY")
		payload = map[string]any{"params": original, "frame": original, "actual_key": "key", "completed": true}
	case "INBOUND_FRAMES":
		predecessors = deps("REQUEST_WRITE")
		payload = map[string]any{"frames": []any{}, "partial_prefix": nil, "wire_bytes": 0}
	case "RESULT_READ":
		predecessors = deps("INBOUND_FRAMES", "REQUEST_WRITE")
		payload = map[string]any{"actual_key": "key", "matched_frame_index": 0, "result_present": true, "result_token": map[string]any{"length": 4, "sha256": original["sha256"], "private_ref": "test:null"}, "parse": "WHOLE_PARSE_SUCCESS"}
	case "TARGET_EVENTS":
		predecessors = deps(append([]string{"RESULT_READ"}, manySources(sources-1)...)...)
		payload = map[string]any{"denominator": 0, "ordinals": []any{}}
	case "TERMINAL":
		roles := []string{"POLICY", "SCHEMA", "QUERY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS"}
		predecessors = deps(append(roles, manySources(min(sources, 256))...)...)
		payload = envelope("TERMINAL", nil)["payload"]
	}
	if predecessors == nil {
		predecessors = []any{}
	}
	return map[string]any{"role": role, "identity": map[string]any{"session": "s", "generation": 1, "transaction": "t"}, "original": original, "predecessors": predecessors, "payload": payload}
}

func transaction(method string, sourceCount int) []any {
	roles := []string{"POLICY", "SCHEMA", "QUERY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS", "TERMINAL"}
	out := make([]any, 0, len(roles)+sourceCount)
	for _, role := range roles {
		out = append(out, roleEnvelope(role, method, 0, sourceCount))
	}
	for i := 0; i < sourceCount; i++ {
		out = append(out, roleEnvelope("SOURCE", method, i, sourceCount))
	}
	return out
}

func TestV2TransactionCardinalityAndNullSourceSet(t *testing.T) {
	compiled := schemaAt(t, "#/$defs/TRANSACTION")
	// Schema-only grouped selection; selector uniqueness and transaction binding
	// are separate contract obligations, not inferred from JSON Schema.
	for _, method := range []string{"textDocument/references", "textDocument/definition"} {
		t.Run(method, func(t *testing.T) {
			for _, tc := range []struct {
				name    string
				sources int
				valid   bool
			}{{"zero target null one source", 1, true}, {"256 sources", 256, true}, {"257 sources even with 264 terminal edges", 257, false}} {
				t.Run(tc.name, func(t *testing.T) {
					v := transaction(method, tc.sources)
					// A null result does not invent a target SOURCE.
					if tc.sources == 1 {
						for _, item := range v {
							r := item.(map[string]any)
							if r["role"] == "TARGET_EVENTS" && len(r["predecessors"].([]any)) != 1 {
								t.Fatal("null/zero-target fixture invented a target SOURCE")
							}
						}
					}
					if err := compiled.Validate(v); (err == nil) != tc.valid {
						t.Fatalf("V2 transaction %s sources=%d valid=%v: %v", method, tc.sources, tc.valid, err)
					}
				})
			}
			missing := transaction(method, 1)
			for i, r := range missing {
				if r.(map[string]any)["role"] == "RESULT_READ" {
					missing = append(missing[:i], missing[i+1:]...)
					break
				}
			}
			if err := compiled.Validate(missing); err == nil {
				t.Fatal("V2 missing mandatory RESULT_READ role accepted")
			}
		})
	}
}

func TestV2TerminalMandatoryPredecessorRoles(t *testing.T) {
	compiled := schema(t)
	fixed := []string{"POLICY", "SCHEMA", "QUERY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS"}
	valid := deps(append(append([]string{}, fixed...), "SOURCE")...)
	if err := compiled.Validate(envelope("TERMINAL", valid)); err != nil {
		t.Fatalf("V2 valid TERMINAL eight roles and query SOURCE rejected: %v", err)
	}
	t.Run("empty TERMINAL rejected", func(t *testing.T) {
		if err := compiled.Validate(envelope("TERMINAL", []any{})); err == nil {
			t.Fatal("V2 empty TERMINAL predecessors accepted")
		}
	})
	for i, role := range fixed {
		t.Run("missing "+role+" TERMINAL edge rejected", func(t *testing.T) {
			without := append(append([]any{}, valid[:i]...), valid[i+1:]...)
			if err := compiled.Validate(envelope("TERMINAL", without)); err == nil {
				t.Fatalf("V2 missing %s TERMINAL predecessor accepted", role)
			}
		})
	}
	t.Run("duplicate mandatory TERMINAL role rejected", func(t *testing.T) {
		duplicate := append(append([]any{}, valid...), deps("RESULT_READ")[0])
		if err := compiled.Validate(envelope("TERMINAL", duplicate)); err == nil {
			t.Fatal("V2 duplicate RESULT_READ TERMINAL predecessor role accepted")
		}
	})
}

func TestV2FrozenRolePredecessorBoundaries(t *testing.T) {
	compiled := schema(t)
	fixed := []string{"POLICY", "SCHEMA", "QUERY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS"}
	terminal264 := deps(append(append([]string{}, fixed...), manySources(256)...)...)
	target256 := deps(append([]string{"RESULT_READ"}, manySources(255)...)...)
	readback266 := deps(append(append([]string{}, fixed...), append([]string{"TERMINAL", "PROCESS"}, manySources(256)...)...)...)
	for _, tc := range []struct {
		name, role          string
		before              []any
		extra               string
		wantBase, wantExtra bool
	}{
		{"terminal 264 accepted 265 rejected", "TERMINAL", terminal264, "PROCESS", true, false},
		{"target events 256 accepted 257 rejected", "TARGET_EVENTS", target256, "SOURCE", true, false},
		{"readback 266 accepted 267 rejected", "READBACK", readback266, "SOURCE", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := compiled.Validate(envelope(tc.role, tc.before)); (err == nil) != tc.wantBase {
				t.Fatalf("V2 %s at-limit validation: %v", tc.role, err)
			}
			after := append(append([]any{}, tc.before...), map[string]any{"role": tc.extra, "selector": "selected:extra", "digest": "sha256:" + strings.Repeat("0", 64)})
			if err := compiled.Validate(envelope(tc.role, after)); (err == nil) != tc.wantExtra {
				t.Fatalf("V2 %s over-limit validation: %v", tc.role, err)
			}
		})
	}
}
