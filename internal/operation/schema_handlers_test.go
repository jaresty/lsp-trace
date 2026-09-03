package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/schema"
)

func TestSchemaGetHandlerReturnsExactCoreBytes(t *testing.T) {
	assertion := "P1_SCHEMA_GET_EXACT_BYTES"
	want, err := schema.BytesFor(schema.FamilyGraph, "v1")
	if err != nil {
		t.Fatal(err)
	}
	got, failure := SchemaGetHandler(context.Background(), Request{Input: json.RawMessage(`{"schema":{"family":"graph","version":"v1"}}`)})
	if failure != nil || !bytes.Equal(got.Artifact, want) {
		t.Fatalf("%s: equal=%v failure=%v", assertion, bytes.Equal(got.Artifact, want), failure)
	}
}

func TestValidateHandlerPreservesBytesAndDetectedVersion(t *testing.T) {
	assertion := "P2_VALIDATE_CORE_PARITY"
	document := `{ "schema_version":"lsp-trace.graph.v1","invocation":{},"capabilities":{},"targets":[],"nodes":[],"edges":[],"terminals":[],"frontier":[],"diagnostics":[],"summary":{} }`
	request := Request{Input: json.RawMessage(`{"input":` + document + `,"schema":{"family":"graph","version":"v1"}}`)}
	got, failure := ValidateHandler(context.Background(), request)
	if failure != nil || !bytes.Equal(got.Artifact, []byte(document)) {
		t.Fatalf("%s: artifact=%q failure=%v", assertion, got.Artifact, failure)
	}
	value, ok := got.Value.(ValidationResult)
	if !ok || value.SchemaVersion != "lsp-trace.graph.v1" {
		t.Fatalf("%s: value=%#v", assertion, got.Value)
	}
}

func TestValidateHandlerClassifiesFamilyAndDocumentDiagnostics(t *testing.T) {
	tests := []struct {
		name, input, code, diagnostic string
	}{
		{"family", `{"input":{},"schema":{"family":"future","version":"v1"}}`, "INPUT_FAMILY_MISMATCH", `unsupported schema family "future"`},
		{"structure", `{"input":{"filter_schema_version":"lsp-trace.filter.v1"},"schema":{"family":"filter","version":"v1"}}`, "INPUT_INVALID", "schema validation lsp-trace.filter.v1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, failure := ValidateHandler(context.Background(), Request{Input: json.RawMessage(tc.input)})
			if failure == nil || failure.Code != tc.code || len(failure.Diagnostics) != 1 || !strings.Contains(failure.Diagnostics[0], tc.diagnostic) || failure.Err == nil || failure.Err.Error() != failure.Diagnostics[0] {
				t.Fatalf("P3_SCHEMA_DIAGNOSTIC_CLASSIFICATION: failure=%#v", failure)
			}
		})
	}
}

func TestSchemaHandlersPreserveFamilyVersionBehavior(t *testing.T) {
	for _, tc := range []struct{ family, version string }{{"graph", "v1"}, {"graph", "lsp-trace.graph.v2"}, {"inspect", "v1"}, {"filter", "lsp-trace.filter.v1"}} {
		input := json.RawMessage(`{"schema":{"family":"` + tc.family + `","version":"` + tc.version + `"}}`)
		got, failure := SchemaGetHandler(context.Background(), Request{Input: input})
		want, err := schema.BytesFor(tc.family, tc.version)
		if err != nil || failure != nil || !bytes.Equal(got.Artifact, want) {
			t.Fatalf("P4_SCHEMA_FAMILY_VERSION_PARITY %s/%s: err=%v failure=%v", tc.family, tc.version, err, failure)
		}
	}
}

func TestSchemaHandlersConcurrentDeterminism(t *testing.T) {
	const workers = 32
	results := make(chan string, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, failure := ValidateHandler(context.Background(), Request{Input: json.RawMessage(`{"input":{"filter_schema_version":"lsp-trace.filter.v1"},"schema":{"family":"filter","version":"v1"}}`)})
			results <- failure.Error()
		}()
	}
	wg.Wait()
	close(results)
	var first string
	for got := range results {
		if first == "" {
			first = got
		}
		if got != first || !strings.Contains(got, "schema validation lsp-trace.filter.v1") {
			t.Fatalf("P3_SCHEMA_CONCURRENT_DIAGNOSTICS: first=%q got=%q", first, got)
		}
	}
}
