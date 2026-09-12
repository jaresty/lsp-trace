package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func censusPrivateRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCensusArgumentParsingAndDefaults(t *testing.T) {
	root := censusPrivateRoot(t)
	var humanOut, humanErr bytes.Buffer
	if code := runCensus([]string{"--publication-root", root}, &humanOut, &humanErr); code != 0 || !strings.Contains(humanOut.String(), "status: READY") {
		t.Fatalf("ASSERT_CENSUS_DEFAULTS: code=%d stdout=%q stderr=%q", code, humanOut.String(), humanErr.String())
	}
	var jsonOut, jsonErr bytes.Buffer
	if code := runCensus([]string{"--publication-root", root, "--json"}, &jsonOut, &jsonErr); code != 0 {
		t.Fatalf("ASSERT_CENSUS_JSON: code=%d stderr=%q", code, jsonErr.String())
	}
	for _, field := range []string{`"schema_version":"lsp-trace.census-plan.v1"`, `"status":"READY"`, `"targets":0`, `"batches":0`} {
		if !strings.Contains(jsonOut.String(), field) {
			t.Fatalf("ASSERT_CENSUS_JSON_OUTPUT: missing=%q output=%q", field, jsonOut.String())
		}
	}
}

func TestCensusMalformedAndConflictingArgs(t *testing.T) {
	root := censusPrivateRoot(t)
	cases := [][]string{
		{"--publication-root"},
		{"--unknown"},
		{"unexpected"},
		{"--publication-root", root, "--json", "--format", "human"},
		{"--publication-root", root, "--format", "xml"},
		{"--publication-root", root, "--include", "!secret/**"},
		{"--publication-root", root, "--exclude", "src/["},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := runCensus(args, &stdout, &stderr); code == 0 || stdout.Len() != 0 {
			t.Fatalf("ASSERT_CENSUS_ARGUMENT_REJECTION: args=%q code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestCensusHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCensus([]string{"--help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "usage: lsp-trace census") || stderr.Len() != 0 {
		t.Fatalf("ASSERT_CENSUS_HELP: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCensusProcessDispatch(t *testing.T) {
	root := censusPrivateRoot(t)
	cmd := exec.Command(os.Args[0], "-test.run=TestCensusProcessHelper", "--", "census", "--publication-root", root, "--json")
	cmd.Env = append(os.Environ(), "LSP_TRACE_CENSUS_HELPER=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ASSERT_CENSUS_PROCESS_DISPATCH: %v output=%q", err, out)
	}
	if !strings.Contains(string(out), `"status":"READY"`) {
		t.Fatalf("ASSERT_CENSUS_PROCESS_OUTPUT: %q", out)
	}
}

func TestCensusProcessHelper(t *testing.T) {
	if os.Getenv("LSP_TRACE_CENSUS_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(run(os.Args[i+1:]))
		}
	}
	os.Exit(97)
}
