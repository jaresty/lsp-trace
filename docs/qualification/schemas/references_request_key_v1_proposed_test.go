package schemas

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

func TestProposedRequestKeyV1Shape(t *testing.T) {
	bytes, err := os.ReadFile("adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(bytes, &root); err != nil {
		t.Fatal(err)
	}
	defs := root["$defs"].(map[string]any)
	key := defs["requestKey"].(map[string]any)
	if key["minLength"] != float64(33) || key["maxLength"] != float64(71) {
		t.Fatal("wrong key length bounds")
	}
	re := regexp.MustCompile(key["pattern"].(string))
	for _, s := range []string{"lsp-trace.request-key.v1:g=7;id=31", "lsp-trace.request-key.v1:g=0;id=18446744073709551615"} {
		if !re.MatchString(s) {
			t.Fatalf("rejected %q", s)
		}
	}
	for _, s := range []string{"lsp-trace.request-key.v1:g=07;id=31", "lsp-trace.request-key.v1:id=31;g=7", "lsp-trace.request-key.v2:g=7;id=31"} {
		if re.MatchString(s) {
			t.Fatalf("accepted %q", s)
		}
	}
	for _, name := range []string{"context", "events", "responseRead", "methodRecord", "targetRecord", "ownerRead"} {
		field := "request_key"
		if name == "methodRecord" || name == "targetRecord" {
			field = "RequestKey"
		}
		got := defs[name].(map[string]any)["properties"].(map[string]any)[field].(map[string]any)["$ref"]
		if got != "#/$defs/requestKey" {
			t.Fatalf("%s.%s: %v", name, field, got)
		}
	}
}
