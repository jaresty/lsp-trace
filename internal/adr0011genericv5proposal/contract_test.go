package adr0011genericv5proposal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaURI = "https://jaresty.github.io/lsp-trace/schemas/adr0011-generic-envelope-v4.schema.json"

func TestV5SchemaIDKindsAndOriginalPins(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "qualification", "originals")
	body, err := os.ReadFile(filepath.Join(root, "adr0011-generic-envelope-v4.schema.json"))
	if err != nil || len(body) != 21409 || hash(body) != "df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3" {
		t.Fatalf("V5 schema pin: %v", err)
	}
	var schema any
	if err := json.Unmarshal(body, &schema); err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaURI, schema); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(schemaURI + "#/$defs/capabilityExchange")
	if err != nil {
		t.Fatal(err)
	}
	frame := map[string]any{"length": 1, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "held:frame"}
	base := map[string]any{"request_direction": "CLIENT_TO_SERVER", "request_frame": frame, "request_id": 1, "request_method": "initialize", "request_params": map[string]any{}, "request_frame_ordinal": 0, "response_direction": "SERVER_TO_CLIENT", "response_frame": frame, "response_id": 1, "response_frame_ordinal": 1, "response_status": "SUCCESS", "response_result": map[string]any{"capabilities": map[string]any{}}, "response_error": nil, "target_write_frame_ordinal": 4}
	for _, tc := range []struct {
		name              string
		request, response any
		valid             bool
	}{
		{"string IDs", "", "", true},
		{"fractional numeric IDs", 1.5, 1.5, true},
		{"integer numeric IDs", 1, 1, true},
		{"boolean IDs", true, true, false},
		{"null IDs", nil, nil, false},
		{"array IDs", []any{}, []any{}, false},
		{"object IDs", map[string]any{}, map[string]any{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := make(map[string]any, len(base))
			for k, v := range base {
				value[k] = v
			}
			value["request_id"], value["response_id"] = tc.request, tc.response
			err := s.Validate(value)
			if (err == nil) != tc.valid {
				t.Errorf("%s schema valid=%v want=%v: %v", tc.name, err == nil, tc.valid, err)
			}
		})
	}
}

func TestV5TransportAndPolicyIdentity(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "qualification", "originals")
	read := func(name, digest string) map[string]any {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || hash(body) != digest {
			t.Fatalf("%s pin: %v", name, err)
		}
		var record map[string]any
		if err := json.Unmarshal(body, &record); err != nil {
			t.Fatal(err)
		}
		return record
	}
	transport := read("generic-lsp-exact-transport-v4.json", "f3e25fab98cc4b8b96b76978cf56220ccc81f4ef41455ab0401041ea281d0bc8")
	if transport["selector"] != "GENERIC_LSP_EXACT_TRANSPORT_V4" || transport["schema"] != schemaURI || transport["jsonrpc_id_raw_token_bytes"] != float64(MaxIDRawTokenBytes) || transport["jsonrpc_id_decoded_string_utf8_bytes"] != float64(MaxIDDecodedStringBytes) || transport["jsonrpc_id_max_abs_exponent"] != float64(MaxIDAbsoluteExponent) {
		t.Fatal("transport ID bounds or identity differ")
	}
	for _, tc := range []struct{ name, digest, method string }{
		{"generic-lsp-references-exact-v5.json", "7c46893fecf0dda7a4942b8f33d92f0c2f49793d003c7eab4e8995aeddf8da36", "textDocument/references"},
		{"generic-lsp-definition-exact-v5.json", "da62f5a161532fcb7c79dd93b0dca940a460c6e0c5fab83e8987de21c91b4dd3", "textDocument/definition"},
	} {
		p := read(tc.name, tc.digest)
		if p["selector_domain"] != "ADR0011-GENERIC-EXACT/5" || p["transport"] != transport["selector"] || p["envelope"] != schemaURI || p["method"] != tc.method || p["authority"] != float64(0) || p["accepted"] != false {
			t.Errorf("%s pointer/authority", tc.name)
		}
	}
}
