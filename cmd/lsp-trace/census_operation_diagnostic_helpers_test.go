package main

import (
	"bytes"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/censusresult"
	"os"
	"testing"
)

func operationSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, e := os.ReadFile("../../internal/schema/schemas/lsp-trace.census-operation-diagnostic.v1.schema.json")
	if e != nil {
		t.Fatal(e)
	}
	doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	const id = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-operation-diagnostic.v1.schema.json"
	if e = c.AddResource(id, doc); e != nil {
		t.Fatal(e)
	}
	s, e := c.Compile(id)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func checkOperationBytes(t *testing.T, raw []byte) censusresult.OperationDiagnostic {
	t.Helper()
	d, e := censusresult.DecodeOperationDiagnostic(raw)
	if e != nil {
		t.Fatalf("codec: %v: %s", e, raw)
	}
	v, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	if e = operationSchema(t).Validate(v); e != nil {
		t.Fatalf("schema: %v", e)
	}
	if bytes.Contains(raw, []byte("SECRET_MARKER")) {
		t.Fatalf("secret disclosed: %s", raw)
	}
	return d
}
