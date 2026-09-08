package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectHelpIncludesHydratedOptions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runInspect([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("help wrote stdout: %q", stdout.String())
	}
	for _, want := range []string{"-hydrated", "-node", "-relation", "-seed"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("help omits %q: %q", want, stderr.String())
		}
	}
}

func TestHydratedCLIRejectsBeforeIO(t *testing.T) {
	for _, args := range [][]string{
		{"absent.json", "--hydrated", "--all-seeds"},
		{"absent.json", "--hydrated", "--seed", "seed"},
		{"absent.json", "--hydrated", "--seed="},
		{"absent.json", "--node", "x", "--all-seeds"},
		{"absent.json", "--hydrated=false", "--all-seeds"},
		{"absent.json", "--hydrated", "--page"},
		{"absent.json", "--hydrated", "--cursor", "bad"},
		{"absent.json", "--hydrated", "--max-work", "-1"},
		{"absent.json", "--hydrated", "--max-page-bytes", "4095"},
		{"absent.json", "--hydrated", "--position-encoding", "guess"},
		{"absent.json", "--hydrated", "--node", strings.Repeat("x", 1025)},
	} {
		var out, err bytes.Buffer
		if code := runInspect(args, &out, &err); code == 0 || out.Len() != 0 || !strings.Contains(err.String(), "INVALID_INPUT") || strings.Contains(err.String(), "no such file") {
			t.Fatalf("PUBLIC_CLI_PREFLIGHT FAIL: %q code=%d out=%s err=%s", args, code, out.String(), err.String())
		}
	}
	t.Log("PUBLIC_CLI_PREFLIGHT PASS")
}
func TestHydratedCLIExplicitInputSafety(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.json")
	raw, err := os.ReadFile("../../internal/hydratedevidence/testdata/focused-fr20.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err = os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{dir, link} {
		var out, stderr bytes.Buffer
		if runInspect([]string{input, "--hydrated", "--node", "unknown", "--json"}, &out, &stderr) == 0 || out.Len() != 0 {
			t.Fatal("PUBLIC_CLI_INPUT_SAFETY FAIL", input)
		}
	}
	var out, stderr bytes.Buffer
	if runInspect([]string{source, "--hydrated", "--max-input-bytes", "100", "--json"}, &out, &stderr) == 0 || out.Len() != 0 {
		t.Fatal("PUBLIC_CLI_INPUT_SAFETY FAIL: byte limit")
	}
	t.Log("PUBLIC_CLI_INPUT_SAFETY PASS")
}
