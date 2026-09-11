package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgramCLeidenRequiresTopK(t *testing.T) {
	var out, errout strings.Builder
	if code := runProgramCLeiden([]string{"--seed", "1", "-"}, strings.NewReader("{}"), &out, &errout); code != 1 || out.Len() != 0 || !strings.Contains(errout.String(), "--pagerank-top-k") {
		t.Fatalf("ASSERT_PROGRAM_C_TOP_K_MANDATORY: code=%d stdout=%q stderr=%q", code, out.String(), errout.String())
	}
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
