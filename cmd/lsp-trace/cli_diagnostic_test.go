package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func machineDiagnosticLines(raw string) bool {
	for _, line := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		if line == "" {
			continue
		}
		var event cliDiagnostic
		if json.Unmarshal([]byte(line), &event) != nil || event.SchemaVersion != cliDiagnosticVersion {
			return false
		}
	}
	return true
}

func TestPrimaryHelpHidesLegacyAcquisitionCommands(t *testing.T) {
	stdout, stderr, code := captureRun(t, []string{"--help"})
	if code != 0 || stderr != "" {
		t.Fatalf("ASSERT_PRIMARY_HELP_STREAMS: code=%d stderr=%q", code, stderr)
	}
	for _, hidden := range []string{"lsp-trace slice ", "lsp-trace incoming "} {
		if strings.Contains(stdout, hidden) {
			t.Fatalf("ASSERT_PRIMARY_HELP_HIDES_LEGACY: found=%q help=%q", hidden, stdout)
		}
	}
	for _, visible := range []string{"lsp-trace trace ", "lsp-trace advanced", "lsp-trace legacy"} {
		if !strings.Contains(stdout, visible) {
			t.Fatalf("ASSERT_PRIMARY_HELP_VISIBLE_LANES: missing=%q help=%q", visible, stdout)
		}
	}
}

func TestHelpLanePointersAreCallableAndDoNotClaimFutureCommands(t *testing.T) {
	advanced, advancedErr, advancedCode := captureRun(t, []string{"advanced"})
	legacy, legacyErr, legacyCode := captureRun(t, []string{"legacy"})
	if advancedCode != 0 || legacyCode != 0 || advancedErr != "" || legacyErr != "" || !strings.Contains(advanced, "advanced operations:") || !strings.Contains(legacy, "slice, incoming") || !strings.Contains(legacy, "census and context FUTURE/PROPOSED") {
		t.Fatalf("ASSERT_HELP_LANES_CALLABLE: advanced=(%d,%q,%q) legacy=(%d,%q,%q)", advancedCode, advanced, advancedErr, legacyCode, legacy, legacyErr)
	}
}

func TestLegacyHelpDoesNotWarn(t *testing.T) {
	for _, operation := range []string{"slice", "incoming"} {
		stdout, stderr, code := captureRun(t, []string{operation, "--help"})
		if code != 0 || stderr != "" || stdout == "" {
			t.Fatalf("ASSERT_HELP_IS_NOT_INVOCATION: op=%s code=%d stdout=%q stderr=%q", operation, code, stdout, stderr)
		}
	}
}

func TestLegacyHumanWarningExactlyOnceAfterOperationSelection(t *testing.T) {
	stdout, stderr, code := captureRun(t, []string{"slice"})
	if code != 1 || stdout != "" || strings.Count(stderr, "warning: slice is deprecated; migrate to trace") != 1 {
		t.Fatalf("ASSERT_LEGACY_WARNING_ONCE: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestLegacyMachineDiagnosticsGoldenClosedAndPrivate(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "SECRET_SOURCE_TOKEN")
	stdout, stderr, code := captureRun(t, []string{"slice", "--machine", "--workspace", secret})
	if code != 1 || stdout != "" || !machineDiagnosticLines(stderr) {
		t.Fatalf("ASSERT_MACHINE_DIAGNOSTIC_JSONL: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	const golden = "{\"schema_version\":\"lsp-trace.cli-diagnostic.v1\",\"code\":\"CLI_LEGACY_OPERATION\",\"severity\":\"warning\",\"operation\":\"slice\",\"replacement\":\"trace\",\"replacement_status\":\"AVAILABLE\"}\n{\"schema_version\":\"lsp-trace.cli-diagnostic.v1\",\"code\":\"CLI_INVOCATION_ERROR\",\"severity\":\"error\",\"operation\":\"slice\",\"replacement\":\"trace\",\"replacement_status\":\"AVAILABLE\"}\n"
	if stderr != golden || strings.Contains(stderr, secret) || strings.Contains(stderr, "SECRET_SOURCE_TOKEN") {
		t.Fatalf("ASSERT_MACHINE_DIAGNOSTIC_GOLDEN_PRIVATE: stderr=%q", stderr)
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &fields); err != nil || len(fields) != 6 {
			t.Fatalf("ASSERT_MACHINE_DIAGNOSTIC_CLOSED_SIX_FIELDS: fields=%v err=%v", fields, err)
		}
	}
}

func TestLegacyFileCensusReplacementRemainsProposed(t *testing.T) {
	stdout, stderr, code := captureRun(t, []string{"slice", "--machine", "--from-file", "src"})
	if code != 1 || stdout != "" || !strings.Contains(stderr, `"replacement":"census"`) || !strings.Contains(stderr, `"replacement_status":"FUTURE/PROPOSED"`) {
		t.Fatalf("ASSERT_CENSUS_REPLACEMENT_NOT_CLAIMED_IMPLEMENTED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestLegacyMachineDiagnosticMutationDetection(t *testing.T) {
	valid := `{"schema_version":"lsp-trace.cli-diagnostic.v1","code":"CLI_LEGACY_OPERATION","severity":"warning","operation":"slice","replacement":"trace","replacement_status":"AVAILABLE"}\n`
	for name, mutated := range map[string]string{
		"version":  strings.Replace(valid, cliDiagnosticVersion, "v0", 1),
		"not-json": "warning: deprecated\n",
	} {
		if machineDiagnosticLines(mutated) {
			t.Fatalf("ASSERT_MACHINE_DIAGNOSTIC_MUTATION_%s", name)
		}
	}
}

func TestMachineModePreservesLegacyStdoutBytesAndExitCode(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--at", "main.go:1:1"}
	humanOut, _, humanCode := captureRun(t, base)
	machineArgs := append(append([]string{}, base...), "--machine")
	machineOut, machineErr, machineCode := captureRun(t, machineArgs)
	if humanCode != machineCode || !bytes.Equal([]byte(humanOut), []byte(machineOut)) || !machineDiagnosticLines(machineErr) {
		t.Fatalf("ASSERT_MACHINE_STDOUT_BYTE_PARITY: humanCode=%d machineCode=%d human=%q machine=%q stderr=%q", humanCode, machineCode, humanOut, machineOut, machineErr)
	}
}
