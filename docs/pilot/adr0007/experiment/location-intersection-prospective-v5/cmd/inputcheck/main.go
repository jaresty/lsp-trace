package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	src "lsp-trace/internal/sourceadmissionv2"
)

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
func read(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	return b
}

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: inputcheck schema SCHEMA INSTANCE | admission ENVELOPE"))
	}
	switch os.Args[1] {
	case "schema", "metadata":
		if (os.Args[1] == "schema" && len(os.Args) != 4) || (os.Args[1] == "metadata" && len(os.Args) != 5) {
			fail(fmt.Errorf("schema requires schema and instance paths; metadata also requires a closed-schema key"))
		}
		var schemaDoc, instance any
		if err := json.Unmarshal(read(os.Args[2]), &schemaDoc); err != nil {
			fail(err)
		}
		instancePath := os.Args[3]
		if os.Args[1] == "metadata" {
			root, ok := schemaDoc.(map[string]any)
			if !ok {
				fail(fmt.Errorf("metadata schema root"))
			}
			closed, ok := root["closed"].(map[string]any)
			if !ok {
				fail(fmt.Errorf("metadata closed schemas"))
			}
			schemaDoc, ok = closed[os.Args[3]]
			if !ok {
				fail(fmt.Errorf("unknown metadata schema %s", os.Args[3]))
			}
			instancePath = os.Args[4]
		}
		if err := json.Unmarshal(read(instancePath), &instance); err != nil {
			fail(err)
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		if err := compiler.AddResource("urn:input-schema", schemaDoc); err != nil {
			fail(err)
		}
		schema, err := compiler.Compile("urn:input-schema")
		if err != nil {
			fail(err)
		}
		if err := schema.Validate(instance); err != nil {
			fail(err)
		}
		fmt.Println("SCHEMA_OK")
	case "admission":
		if len(os.Args) != 3 {
			fail(fmt.Errorf("admission requires envelope path"))
		}
		var envelope struct {
			Outcome string               `json:"outcome"`
			Detail  string               `json:"detail"`
			Input   []src.SelectedSource `json:"input"`
		}
		if err := json.Unmarshal(read(os.Args[2]), &envelope); err != nil {
			fail(err)
		}
		result := src.Admit(envelope.Input, src.Limits{MaxSources: 1000, MaxSourceBytes: 1048576, MaxTotalBytes: 8388608})
		detail := strings.ToUpper(strings.ReplaceAll(result.Detail, " ", "_"))
		if result.Outcome == src.InvalidSource && detail == "FILE_DIGEST" {
			detail = "FILE_DIGEST"
		}
		if result.Outcome == src.ResourceLimit && detail == "SOURCES" {
			detail = "SOURCES"
		}
		if result.Outcome == src.DuplicateSource {
			detail = "DUPLICATE_PATH"
		}
		if result.Outcome == src.InvalidRequest {
			detail = "LIMITS_OR_SOURCES"
		}
		if string(result.Outcome) != envelope.Outcome || detail != envelope.Detail {
			fail(fmt.Errorf("admission mismatch got=%s/%s want=%s/%s", result.Outcome, detail, envelope.Outcome, envelope.Detail))
		}
		fmt.Printf("ADMISSION_OK %s %s\n", result.Outcome, detail)
	default:
		fail(fmt.Errorf("unknown mode"))
	}
}
