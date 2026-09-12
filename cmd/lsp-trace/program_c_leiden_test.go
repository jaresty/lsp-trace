package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcpresentation"
	"lsp-trace/internal/programctestfixture"
)

func TestVerifyDefaultSelectorRoutesGraphProvenanceV5(t *testing.T) {
	valid := programctestfixture.ValidV5(t)
	validSelector := filepath.Join(t.TempDir(), "valid-v5.selector.json")
	if err := publishValidatedBundle(validSelector, valid, admitAcquisitionV5); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := runVerify([]string{validSelector}, &stdout, &stderr); code != 0 || stdout.String() != "verified integrity and custody\n" || stderr.Len() != 0 {
		t.Fatalf("ASSERT_VERIFY_DEFAULT_SELECTOR_ROUTES_GRAPH_PROVENANCE_V5: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	malformed := []byte(`{"schema_version":"lsp-trace.graph-provenance.v5"}`)
	malformedSelector := filepath.Join(t.TempDir(), "malformed-v5.selector.json")
	if err := publishValidatedBundle(malformedSelector, malformed, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runVerify([]string{malformedSelector}, &stdout, &stderr); code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "verify semantic receipt:") {
		t.Fatalf("ASSERT_VERIFY_DEFAULT_SELECTOR_V5_SEMANTICS_FAIL_CLOSED: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestProgramCLeidenRequiresTopK(t *testing.T) {
	var out, errout strings.Builder
	if code := runProgramCLeiden([]string{"--seed", "1", "-"}, strings.NewReader("{}"), &out, &errout); code != 1 || out.Len() != 0 || !strings.Contains(errout.String(), "--pagerank-top-k") {
		t.Fatalf("ASSERT_PROGRAM_C_TOP_K_MANDATORY: code=%d stdout=%q stderr=%q", code, out.String(), errout.String())
	}
}
func TestProgramCLeidenValidPathStdinTextJSONParityAndNoLeak(t *testing.T) {
	raw := programctestfixture.ValidV5(t)
	d := t.TempDir()
	p := filepath.Join(d, "graph-provenance-v5.json")
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(format, path string) string {
		t.Helper()
		var out, errout strings.Builder
		in := strings.NewReader("")
		if path == "-" {
			in = strings.NewReader(string(raw))
		}
		code := runProgramCLeiden([]string{"--seed", "19", "--pagerank-top-k", "2", "--hub-top-k", "2", "--format", format, path}, in, &out, &errout)
		if code != 0 || errout.Len() != 0 {
			t.Fatalf("ASSERT_VALID_V5_CLI_%s: code=%d stderr=%q", format, code, errout.String())
		}
		return out.String()
	}
	jsonPath, jsonStdin := run("json", p), run("json", "-")
	textPath, textStdin := run("text", p), run("text", "-")
	if jsonPath != jsonStdin || textPath != textStdin {
		t.Fatal("ASSERT_VALID_V5_PATH_STDIN_BYTE_PARITY")
	}
	if strings.Contains(jsonPath, programctestfixture.OpaqueMarker) || strings.Contains(jsonPath, programctestfixture.SourceBodyMarker) {
		t.Fatal("ASSERT_JSON_OPAQUE_DATA_AND_SOURCE_BODY_DO_NOT_LEAK")
	}
	if strings.Contains(textPath, programctestfixture.OpaqueMarker) || strings.Contains(textPath, programctestfixture.SourceBodyMarker) {
		t.Fatal("ASSERT_TEXT_OPAQUE_DATA_AND_SOURCE_BODY_DO_NOT_LEAK")
	}
	for _, want := range []string{programcpresentation.Disclaimer, programcpresentation.Stability, "file:///fixture/input.go:1:1-1:3", "COMMUNITIES", "CROSS-COMMUNITY CALLS"} {
		if !strings.Contains(textPath, want) {
			t.Fatalf("ASSERT_TEXT_PARITY_MARKER_%q", want)
		}
	}
	o, failure := programc.Compute(raw, 19)
	if failure != nil {
		t.Fatal(failure)
	}
	b, err := programc.ComputeBoundary(o, programc.BoundaryRequest{PageRankTopK: 2, HubTopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(jsonPath), &got); err != nil {
		t.Fatal(err)
	}
	var wantCommunities any
	boundaryBytes, _ := json.Marshal(b.Communities)
	_ = json.Unmarshal(boundaryBytes, &wantCommunities)
	gotEvidence := map[string]any{}
	for _, community := range got["communities"].([]any) {
		wire := community.(map[string]any)
		gotEvidence[wire["community_id"].(string)] = wire["evidence"]
	}
	wantEvidence := map[string]any{}
	for _, community := range wantCommunities.([]any) {
		wire := community.(map[string]any)
		wantEvidence[wire["community_id"].(string)] = wire
	}
	if !reflect.DeepEqual(gotEvidence, wantEvidence) {
		t.Fatalf("ASSERT_EXACT_UNCHANGED_BOUNDARY_EVIDENCE: got=%s want=%s", mustJSON(gotEvidence), mustJSON(wantEvidence))
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return fmt.Sprint(string(b))
}

func TestProgramCLeidenPathStdinMalformedParity(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.json")
	if err := os.WriteFile(p, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(path string) (int, string) {
		var out, errout strings.Builder
		in := strings.NewReader("")
		if path == "-" {
			in = strings.NewReader("{}")
		}
		code := runProgramCLeiden([]string{"--seed", "1", "--pagerank-top-k", "1", "--hub-top-k", "1", "--format", "json", path}, in, &out, &errout)
		if out.Len() != 0 {
			t.Fatalf("ASSERT_NO_PARTIAL_DISPLAY: %q", out.String())
		}
		return code, errout.String()
	}
	pc, pe := run(p)
	sc, se := run("-")
	if pc != 1 || sc != 1 || pe != se {
		t.Fatalf("ASSERT_STDIN_PATH_PARITY: path=(%d,%q) stdin=(%d,%q)", pc, pe, sc, se)
	}
}
