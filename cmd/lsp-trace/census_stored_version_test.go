//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/censusresult"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCensusUnsupportedStoredVersionDiagnostic(t *testing.T) {
	schemaRaw, e := os.ReadFile("../../internal/schema/schemas/lsp-trace.census-operation-diagnostic.v2.schema.json")
	if e != nil {
		t.Fatal(e)
	}
	doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(schemaRaw))
	if e != nil {
		t.Fatal(e)
	}
	compiler := jsonschema.NewCompiler()
	const id = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-operation-diagnostic.v2.schema.json"
	if e = compiler.AddResource(id, doc); e != nil {
		t.Fatal(e)
	}
	schema, e := compiler.Compile(id)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"pending", "published"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("../../internal/applicationinventory/testdata/stored-v1/" + name + ".json")
			if e != nil {
				t.Fatal(e)
			}
			var env struct {
				Content struct{ RunID, TaskID, Role string }
			}
			if json.Unmarshal(raw, &env) != nil {
				t.Fatal("fixture")
			}
			h := env.Content
			dir, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			if e = os.Chmod(dir, 0700); e != nil {
				t.Fatal(e)
			}
			t.Setenv(censusDurableStoreEnv, dir) // existing inventory must never fall back to one-item
			path := filepath.Join(dir, "inventory-run-"+h.RunID+".json")
			if e = os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			before, _ := os.ReadDir(dir)
			for _, args := range [][]string{{"task", h.RunID}, {"read", h.RunID, h.TaskID, "input", "0", "32768"}, {"submit", h.RunID, h.TaskID, h.Role}, {"result", h.RunID, h.TaskID}} {
				var out, diag bytes.Buffer
				code := runCensusDurable(args, bytes.NewBufferString("never interpreted"), &out, &diag)
				d, e := censusresult.DecodeOperationDiagnostic(diag.Bytes())
				if code != 1 || out.Len() != 0 || e != nil || d.Reason != censusresult.UnsupportedStoredVersion || d.SchemaVersion != censusresult.StoredVersionDiagnosticSchemaVersion || d.Action != "STOP" || d.Retry || d.Recovery != nil {
					t.Fatalf("ASSERT_UNSUPPORTED_DIAGNOSTIC: %v %s", e, diag.String())
				}
				value, e := jsonschema.UnmarshalJSON(bytes.NewReader(diag.Bytes()))
				if e != nil || schema.Validate(value) != nil {
					t.Fatal("v2 schema", e)
				}
				if operationSchema(t).Validate(value) == nil {
					t.Fatal("old closed schema accepted new reason")
				}
				if bytes.Contains(diag.Bytes(), []byte(dir)) || bytes.Contains(diag.Bytes(), []byte(h.RunID)) {
					t.Fatal("identifier/path disclosure")
				}
				got, _ := os.ReadFile(path)
				files, _ := os.ReadDir(dir)
				if !bytes.Equal(raw, got) || !reflect.DeepEqual(before, files) {
					t.Fatal("ASSERT_VERSION_REFUSAL_ZERO_EFFECTS")
				}
			}
			t.Log("ASSERT_UNSUPPORTED_DIAGNOSTIC: PASS; no fallback, no reads of answers, no artifacts or head writes")
		})
	}
}
