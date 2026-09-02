package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSchemaGetSupportsFamiliesAndPreservesGraphAlias(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "inspect family", args: []string{"get", "--family", "inspect", "--version", "v1"}, want: `"lsp-trace.inspect.v1"`},
		{name: "filter family", args: []string{"get", "--family", "filter", "--version", "v1"}, want: `"lsp-trace.filter.v1"`},
		{name: "graph family", args: []string{"get", "--family", "graph", "--version", "v3"}, want: `"lsp-trace.graph.v3"`},
		{name: "graph alias", args: []string{"get", "--schema", "v1"}, want: `"lsp-trace.graph.v1"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runSchema(tt.args, &stdout, &stderr); code != 0 {
				t.Fatalf("ASSERT_SCHEMA_FAMILY_RETRIEVAL: code=%d stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("ASSERT_SCHEMA_FAMILY_RETRIEVAL: output missing %s", tt.want)
			}
		})
	}
}

func TestValidateParsesFilterFamilyAndReturnsStableFailures(t *testing.T) {
	var first, second bytes.Buffer
	args := []string{"--family", "filter", "--version", "v1", "-"}
	if code := runValidate(args, strings.NewReader("not-json"), &bytes.Buffer{}, &first); code != 1 || !strings.Contains(first.String(), "invalid JSON") {
		t.Fatalf("ASSERT_VALIDATE_FILTER_FAMILY_GRAMMAR: code=%d stderr=%q", code, first.String())
	}
	if code := runValidate(args, strings.NewReader("not-json"), &bytes.Buffer{}, &second); code != 1 || first.String() != second.String() {
		t.Fatalf("ASSERT_VALIDATE_FAMILY_FAILURE_STABLE: first=%q second=%q", first.String(), second.String())
	}
}

func TestSchemaGetRejectsAmbiguousOrUnsupportedFamilyForms(t *testing.T) {
	for _, args := range [][]string{
		{"get", "--family", "filter"},
		{"get", "--version", "v1"},
		{"get", "--family", "filter", "--version", "v2"},
		{"get", "--family", "filter", "--version", "v1", "--schema", "v1"},
	} {
		var stdout, stderr bytes.Buffer
		if code := runSchema(args, &stdout, &stderr); code != 1 || stdout.Len() != 0 {
			t.Fatalf("ASSERT_SCHEMA_FAMILY_FAIL_CLOSED: args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}
