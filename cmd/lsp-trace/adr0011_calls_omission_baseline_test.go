package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This pins the pre-typed-input Program C CLI process response for an omitted
// family selector at e66d5a7b. Explicit CALLS_ONLY is not yet a public selector;
// relations:["CALLS"] is a different slice contract, not a parity surrogate.
func TestADR0011HistoricalOmittedCallsOnlyProcessByteBaseline(t *testing.T) {
	input := composeCLIInput(t)
	root := t.TempDir()
	in := filepath.Join(root, "input.json")
	if err := os.WriteFile(in, input, 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "lsp-trace")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build baseline process: %v: %s", err, out)
	}
	run := exec.Command(binary, "program-c", "compose", in)
	output, err := run.Output()
	if err != nil {
		t.Fatalf("historical omitted process: %v", err)
	}
	const baselineByteLength = 43541
	const baselineSHA256 = "d9630f80e4b8df33f345c8dfb52659b9813adeca5f884935d6dae680e39e95c5"
	digest := fmt.Sprintf("%x", sha256.Sum256(output))
	if len(output) != baselineByteLength || digest != baselineSHA256 {
		t.Fatalf("ASSERT_ADR0011_PRECHANGE_OMITTED_CALLS_ONLY_PROCESS_BYTES: length=%d sha256=%s", len(output), digest)
	}
	// Current Program C has no explicit family selector. It must not quietly
	// ignore one and make an invalid omitted/explicit parity claim possible.
	var proposed map[string]any
	if err := json.Unmarshal(input, &proposed); err != nil {
		t.Fatal(err)
	}
	proposed["input_family"] = "CALLS_ONLY"
	proposedBytes, err := json.Marshal(proposed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(in, proposedBytes, 0600); err != nil {
		t.Fatal(err)
	}
	explicit := exec.Command(binary, "program-c", "compose", in)
	if got, err := explicit.Output(); err == nil || len(got) != 0 {
		t.Fatalf("ASSERT_ADR0011_EXPLICIT_FAMILY_NOT_YET_SUPPORTED: exit=%v stdout_bytes=%d", err, len(got))
	}
}
