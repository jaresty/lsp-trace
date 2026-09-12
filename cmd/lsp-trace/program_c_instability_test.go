package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/programctestfixture"
	"lsp-trace/internal/schema"
)

func runInstabilityCLI(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "lsp-trace")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, output)
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), stdout.String(), stderr.String()
	}
	t.Fatalf("run CLI: %v", err)
	return -1, "", ""
}

func TestProgramCInstabilityCLIJSONPresentation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph-v5.json")
	if err := os.WriteFile(path, programctestfixture.ValidV5(t), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"program-c", "instability", "--seeds", "1,2", "--algorithm-version", "gonum-v0.17.1", "--parameters-sha256", "sha256:" + strings.Repeat("c", 64), "--resource-policy-sha256", "sha256:" + strings.Repeat("d", 64), path}
	code, stdout, stderr := runInstabilityCLI(t, "", args...)
	if code != 0 {
		t.Fatalf("ASSERT_A08_CLI_EXPOSED: code=%d stderr=%s", code, stderr)
	}
	raw := []byte(stdout)
	if _, err := schema.ValidateFor(raw, schema.FamilyCommunityInstability, "v1"); err != nil {
		t.Fatalf("ASSERT_A08_ACTUAL_SCHEMA: %v", err)
	}
	var artifact programc.InstabilityArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Outcome != "STABLE" || artifact.RunAccounting.CompletedPairCount != 15 || artifact.ClaimCeiling != programc.InstabilityClaimCeiling || artifact.StructuralClaimCeiling != programc.InstabilityStructuralCeiling {
		t.Fatalf("ASSERT_A08_STRUCTURAL_BOUNDED_PRESENTATION: %+v", artifact)
	}
}

func TestProgramCInstabilityCLIRejectsNonV5(t *testing.T) {
	args := []string{"program-c", "instability", "--seeds", "1,2", "--algorithm-version", "v", "--parameters-sha256", "sha256:" + strings.Repeat("c", 64), "--resource-policy-sha256", "sha256:" + strings.Repeat("d", 64), "-"}
	code, stdout, stderr := runInstabilityCLI(t, `{"schema_version":"lsp-trace.graph.v3"}`, args...)
	if code == 0 || stdout != "" {
		t.Fatalf("ASSERT_A08_V5_ONLY: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
