package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"lsp-trace/internal/schema"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func main() {
	raw, err := os.ReadFile("internal/hydratedevidence/testdata/focused-fr20.v2.json")
	if err != nil { panic(err) }
	schemaRaw, err := schema.BytesFor(schema.FamilyGraphProvenance, "v2")
	if err != nil { panic(err) }
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaRaw))
	if err != nil { panic(err) }
	const id = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph-provenance.v2.schema.json"
	if err = compiler.AddResource(id, doc); err != nil { panic(err) }
	compiled, err := compiler.Compile(id)
	if err != nil { panic(err) }
	var outer any
	if err = json.Unmarshal(raw, &outer); err != nil { panic(err) }
	if err = compiled.Validate(outer); err != nil { panic(err) }
	var carrier struct { GraphBytes string `json:"graph_bytes"`; SchemaVersion string `json:"schema_version"` }
	if err = json.Unmarshal(raw, &carrier); err != nil { panic(err) }
	graphRaw, err := base64.StdEncoding.DecodeString(carrier.GraphBytes)
	if err != nil { panic(err) }
	var graph struct { SchemaVersion string `json:"schema_version"`; Diagnostics []map[string]any `json:"diagnostics"` }
	if err = json.Unmarshal(graphRaw, &graph); err != nil { panic(err) }
	fmt.Printf("OUTER_SCHEMA_PASS=%s INNER_JSON_PASS=%s DIAGNOSTIC=%v\n", carrier.SchemaVersion, graph.SchemaVersion, graph.Diagnostics)
}
