package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllCasesConformToContract(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	cmd := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v4-private-check")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("private checker failed: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "SUMMARY cases=50 complete=21 failed=29 contract_validations=50") {
		t.Fatalf("missing exact summary; output:\n%s", got)
	}
	if n := strings.Count(got, " contract=ok"); n != 50 {
		t.Fatalf("contract validation count = %d, want 50\n%s", n, got)
	}
}
