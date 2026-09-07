package main

import (
	"bytes"
	"lsp-trace/internal/boundedanalysis"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundedVerifyRejectsOversizedSelectorBeforeDecode(t *testing.T) {
	file := filepath.Join(t.TempDir(), "oversized-selector.json")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(boundedanalysis.MaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runVerify([]string{"--family", boundedanalysis.Family, "--version", "v1", file}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "LIMIT") {
		t.Fatalf("ASSERT_BOUNDED_VERIFY_PREDECODE_LIMIT: code=%d stderr=%s", code, stderr.String())
	}
}
