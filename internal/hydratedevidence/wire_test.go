package hydratedevidence

import (
	"bytes"
	"encoding/json"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"strings"
	"testing"
)

func TestTextRetainsNativeRecordIdentity(t *testing.T) {
	input, r, _ := mixedFixture(t)
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	text, e := Text(input, r, b)
	if e != nil {
		t.Fatal(e)
	}
	id := b.Origins[0].Record.NativeID
	if id == "" || !strings.Contains(text, id) {
		t.Fatal("human view omits native record identity")
	}
}

func TestWireAndText(t *testing.T) {
	input, r, _ := mixedFixture(t)
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	if ValidateJSON(input, r, []byte(`{"schema_version":"lsp-trace.hydrated-evidence.v1","complete":true}`)) == nil {
		t.Fatal("wire accepts missing origins")
	}
	if e = ValidateJSON(input, r, encoded(t, b)); e != nil {
		t.Fatal(e)
	}
	text, e := Text(input, r, b)
	if e != nil || !strings.Contains(text, Caller) || !strings.Contains(text, NonAuthoritative) || !strings.Contains(text, "A😀abc") {
		t.Fatalf("human authority/body view missing: %q %v", text, e)
	}
	r.Policy.IncludeBodies = false
	b, e = Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	text, e = Text(input, r, b)
	if e != nil || strings.Contains(text, "A😀abc") {
		t.Fatal("human view leaked excluded body")
	}
}
func TestOwnedSchema(t *testing.T) {
	doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(Schema()))
	if e != nil {
		t.Fatal(e)
	}
	c := jsonschema.NewCompiler()
	if e = c.AddResource("urn:lsp-trace:hydrated-evidence:internal:v1", doc); e != nil {
		t.Fatal(e)
	}
	s, e := c.Compile("urn:lsp-trace:hydrated-evidence:internal:v1")
	if e != nil {
		t.Fatal(e)
	}
	input, r, _ := mixedFixture(t)
	b, e := Hydrate(input, r)
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{encoded(t, b), input.Sidecars[0]} {
		v, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Validate(v); e != nil {
			t.Fatal(e)
		}
	}
	var v map[string]any
	json.Unmarshal(encoded(t, b), &v)
	delete(v, "origins")
	if s.Validate(v) == nil {
		t.Fatal("schema missing-origin acceptance")
	}
}
