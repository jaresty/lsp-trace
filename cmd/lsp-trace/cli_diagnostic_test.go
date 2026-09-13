package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decodeMachineDiagnostics(raw string) ([]cliDiagnostic, error) {
	if raw == "" || !strings.HasSuffix(raw, "\n") {
		return nil, fmt.Errorf("diagnostics must be nonempty newline-terminated JSONL")
	}
	lines := strings.Split(strings.TrimSuffix(raw, "\n"), "\n")
	events := make([]cliDiagnostic, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			return nil, fmt.Errorf("empty JSONL record")
		}
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		var event cliDiagnostic
		if err := dec.Decode(&event); err != nil {
			return nil, err
		}
		if event.SchemaVersion == "" || event.Code == "" || event.Severity == "" || event.Operation == "" || event.Replacement == "" || event.Status == "" || event.RemovalRelease == "" {
			return nil, fmt.Errorf("empty required field")
		}
		if event.SchemaVersion != cliDiagnosticVersion || event.RemovalRelease != cliRemovalRelease {
			return nil, fmt.Errorf("unexpected version or removal release")
		}
		events = append(events, event)
	}
	return events, nil
}

func TestPrimaryHelpKeepsLegacyAcquisitionCommandsVisible(t *testing.T) {
	stdout, stderr, code := captureRun(t, []string{"--help"})
	if code != 0 || stderr != "" {
		t.Fatalf("ASSERT_PRIMARY_HELP_STREAMS: code=%d stderr=%q", code, stderr)
	}
	for _, visible := range []string{"lsp-trace slice ", "lsp-trace incoming ", "lsp-trace trace "} {
		if !strings.Contains(stdout, visible) {
			t.Fatalf("ASSERT_PRIMARY_HELP_LEGACY_VISIBLE: missing=%q help=%q", visible, stdout)
		}
	}
}

func TestHelpLanePointersDescribeProposalsAsUnavailable(t *testing.T) {
	legacy, legacyErr, legacyCode := captureRun(t, []string{"legacy"})
	if legacyCode != 0 || legacyErr != "" || !strings.Contains(legacy, "census and context are unavailable proposals") {
		t.Fatalf("ASSERT_PROPOSED_NOT_ACTIONABLE: code=%d stdout=%q stderr=%q", legacyCode, legacy, legacyErr)
	}
}

func TestLegacyHelpAndInvalidSyntaxDoNotWarn(t *testing.T) {
	for _, args := range [][]string{{"slice", "--help"}, {"incoming", "--help"}, {"slice", "--unknown"}, {"incoming", "--schema", "v9"}, {"slice"}} {
		_, stderr, _ := captureRun(t, args)
		if strings.Contains(stderr, "deprecated") || strings.Contains(stderr, cliCodeLegacyOperation) {
			t.Fatalf("ASSERT_INVALID_OR_HELP_NO_DEPRECATION: args=%v stderr=%q", args, stderr)
		}
	}
}

func TestLegacyHumanWarningExactlyOnceAfterValidatedInvocation(t *testing.T) {
	workspace := t.TempDir()
	stdout, stderr, code := captureRun(t, []string{"slice", "--workspace", workspace, "--server", "missing", "--at", "main.go:1:1"})
	if code != 1 || stdout != "" || strings.Count(stderr, "warning: slice is deprecated; migrate to trace") != 1 {
		t.Fatalf("ASSERT_LEGACY_WARNING_ONCE: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestVersionedLegacyWarningsFollowAcquisitionSyntaxValidation(t *testing.T) {
	for _, mode := range []string{"slice", "incoming"} {
		for _, version := range []string{"v2", "v3"} {
			t.Run(mode+"-"+version, func(t *testing.T) {
				base := []string{mode, "--acquisition-version", version}
				invalid := [][]string{
					append(append([]string{}, base...), "--unknown"),
					{mode, "--acquisition-version"},
					append(append([]string{}, base...), "--group-by", "invalid"),
					append(append([]string{}, base...), "positional"),
				}
				for _, args := range invalid {
					stdout, stderr, code := captureRun(t, args)
					if code != 1 || strings.Contains(stderr, cliCodeLegacyOperation) || strings.Contains(strings.ToLower(stderr), "deprecated") {
						t.Fatalf("ASSERT_VERSIONED_INVALID_NO_LEGACY_WARNING: args=%v code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
					}
					machine := append([]string{mode, "--machine"}, args[1:]...)
					machineOut, machineErr, machineCode := captureRun(t, machine)
					events, err := decodeMachineDiagnostics(machineErr)
					if machineCode != code || machineOut != stdout || err != nil || len(events) != 1 || events[0].Code != cliCodeInvocationError {
						t.Fatalf("ASSERT_VERSIONED_VALIDATION_MATCHES_DISPATCH: args=%v human=(%d,%q,%q) machine=(%d,%q,%q) events=%+v err=%v", args, code, stdout, stderr, machineCode, machineOut, machineErr, events, err)
					}
				}

				help := append(append([]string{}, base...), "--help")
				stdout, stderr, code := captureRun(t, help)
				if code != 1 || strings.Contains(stderr, cliCodeLegacyOperation) || strings.Contains(strings.ToLower(stderr), "deprecated") {
					t.Fatalf("ASSERT_VERSIONED_HELP_NO_LEGACY_WARNING: args=%v code=%d stdout=%q stderr=%q", help, code, stdout, stderr)
				}
				machineHelp := append([]string{mode, "--machine"}, help[1:]...)
				machineOut, machineErr, machineCode := captureRun(t, machineHelp)
				helpEvents, helpErr := decodeMachineDiagnostics(machineErr)
				if machineCode != code || machineOut != stdout || helpErr != nil || len(helpEvents) != 1 || helpEvents[0].Code != cliCodeInvocationError {
					t.Fatalf("ASSERT_VERSIONED_HELP_NO_LEGACY_WARNING: args=%v code=%d stdout=%q stderr=%q events=%+v err=%v", machineHelp, machineCode, machineOut, machineErr, helpEvents, helpErr)
				}

				workspace := t.TempDir()
				marker := filepath.Join(workspace, "provider-started")
				validateArgs := []string{"--workspace", workspace, "--server", "/bin/sh", "--server-arg", "-c", "--server-arg", "touch " + marker, "--seed-manifest", filepath.Join(workspace, "missing.json")}
				if validationCode := validateAcquisitionVersion(mode, version, validateArgs); validationCode != 0 {
					t.Fatalf("ASSERT_VERSIONED_VALIDATION_PRESTART: version=%s mode=%s code=%d", version, mode, validationCode)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("ASSERT_VERSIONED_VALIDATION_PRESTART: version=%s mode=%s provider marker err=%v", version, mode, err)
				}

				valid := append(append([]string{}, base...), "--workspace", t.TempDir(), "--server", "missing", "--seed-manifest", "missing.json")
				stdout, stderr, code = captureRun(t, valid)
				if code != 1 || stdout != "" || strings.Count(stderr, "warning: "+mode+" is deprecated; migrate to trace") != 1 {
					t.Fatalf("ASSERT_VERSIONED_VALID_WARNING_ONCE: args=%v code=%d stdout=%q stderr=%q", valid, code, stdout, stderr)
				}
				machine := append([]string{mode, "--machine"}, valid[1:]...)
				machineOut, machineErr, machineCode = captureRun(t, machine)
				events, err := decodeMachineDiagnostics(machineErr)
				legacyCount := 0
				for _, event := range events {
					if event.Code == cliCodeLegacyOperation {
						legacyCount++
						if mode == "incoming" && event.Status != "CONDITIONAL" {
							t.Fatalf("ASSERT_INCOMING_REPLACEMENT_CONDITIONAL: event=%+v", event)
						}
					}
				}
				if machineCode != code || machineOut != stdout || err != nil || legacyCount != 1 {
					t.Fatalf("ASSERT_VERSIONED_VALID_WARNING_ONCE: args=%v code=%d stdout=%q stderr=%q events=%+v err=%v", machine, machineCode, machineOut, machineErr, events, err)
				}

				passThrough := append(append([]string{}, base...), "--workspace", t.TempDir(), "--server", "missing", "--server-arg", "--machine", "--seed-manifest", "missing.json")
				_, passErr, passCode := captureRun(t, passThrough)
				if passCode != 1 || strings.Count(passErr, "warning: "+mode+" is deprecated; migrate to trace") != 1 {
					t.Fatalf("ASSERT_VERSIONED_SERVER_ARG_MACHINE_PASSTHROUGH: args=%v code=%d stderr=%q", passThrough, passCode, passErr)
				}
			})
		}
	}
}

func TestDuplicateMachineIsOneStrictJSONLSyntaxErrorIncludingHelp(t *testing.T) {
	for _, args := range [][]string{{"slice", "--machine", "--machine"}, {"incoming", "--machine", "--machine", "--help"}} {
		stdout, stderr, code := captureRun(t, args)
		events, err := decodeMachineDiagnostics(stderr)
		if code != 1 || stdout != "" || err != nil || len(events) != 1 || events[0].Code != cliCodeInvocationError || events[0].Severity != "error" {
			t.Fatalf("ASSERT_DUPLICATE_MACHINE_STRICT: args=%v code=%d stdout=%q stderr=%q events=%+v err=%v", args, code, stdout, stderr, events, err)
		}
	}
}

func TestMachineFlagValuePassThroughAndParity(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cases := [][]string{
		{"incoming", "--workspace", workspace, "--server", "missing", "--server-arg", "--machine", "--at", "main.go:1:1"},
		{"incoming", "--workspace=" + workspace, "--server=missing", "--server-env=A=1", "--server-env", "B=2", "--at=main.go:1:1"},
	}
	for _, base := range cases {
		humanOut, _, humanCode := captureRun(t, base)
		machine := append([]string{base[0], "--machine"}, base[1:]...)
		machineOut, machineErr, machineCode := captureRun(t, machine)
		if humanCode != machineCode || !bytes.Equal([]byte(humanOut), []byte(machineOut)) {
			t.Fatalf("ASSERT_MACHINE_STDOUT_EXIT_PARITY: base=%v human=(%d,%q) machine=(%d,%q)", base, humanCode, humanOut, machineCode, machineOut)
		}
		events, err := decodeMachineDiagnostics(machineErr)
		if err != nil || len(events) == 0 {
			t.Fatalf("ASSERT_MACHINE_JSONL: %q %v", machineErr, err)
		}
	}
}

func TestLegacyMachineDiagnosticGoldenClosedPrivateAndRemovalUnscheduled(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "SECRET_SOURCE_TOKEN")
	_, stderr, _ := captureRun(t, []string{"slice", "--machine", "--workspace", secret, "--server", "missing", "--at", "main.go:1:1"})
	events, err := decodeMachineDiagnostics(stderr)
	if err != nil || len(events) != 2 || strings.Contains(stderr, secret) {
		t.Fatalf("ASSERT_MACHINE_PRIVATE_CLOSED: stderr=%q events=%+v err=%v", stderr, events, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &fields); err != nil || len(fields) != 7 {
			t.Fatalf("ASSERT_CLOSED_SEVEN_FIELDS: fields=%v err=%v", fields, err)
		}
	}
}

func TestMachineDiagnosticParserRejectsEmptyUnknownAndMutations(t *testing.T) {
	valid := "{\"schema_version\":\"lsp-trace.cli-diagnostic.v1\",\"code\":\"CLI_LEGACY_OPERATION\",\"severity\":\"warning\",\"operation\":\"slice\",\"replacement\":\"trace\",\"replacement_status\":\"AVAILABLE\",\"removal_release\":\"UNSCHEDULED\"}\n"
	if events, err := decodeMachineDiagnostics(valid); err != nil || len(events) != 1 {
		t.Fatalf("valid fixture: %v %+v", err, events)
	}
	for name, mutated := range map[string]string{
		"empty": "", "unknown": strings.Replace(valid, "{", "{\"extra\":\"x\",", 1),
		"empty-required": strings.Replace(valid, "\"slice\"", "\"\"", 1), "version": strings.Replace(valid, cliDiagnosticVersion, "v0", 1),
		"not-json": "warning: deprecated\n", "literal-backslash-n": strings.TrimSuffix(valid, "\n") + `\n`,
	} {
		if _, err := decodeMachineDiagnostics(mutated); err == nil {
			t.Fatalf("ASSERT_MACHINE_MUTATION_%s", name)
		}
	}
}

func TestLegacyFileCensusReplacementExplicitlyUnavailableProposed(t *testing.T) {
	workspace := t.TempDir()
	_, stderr, _ := captureRun(t, []string{"slice", "--machine", "--from-file", "src", "--workspace", workspace, "--server", "missing"})
	if !strings.Contains(stderr, `"replacement":"census"`) || !strings.Contains(stderr, `"replacement_status":"FUTURE/PROPOSED_UNAVAILABLE"`) {
		t.Fatalf("ASSERT_CENSUS_UNAVAILABLE_PROPOSAL: %q", stderr)
	}
}
