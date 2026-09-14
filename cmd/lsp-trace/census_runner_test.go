package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func TestRunCensusHelpGrammarIsValidatedWithoutSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
		wantHelp bool
		machine  bool
	}{
		{name: "long", args: []string{"--help"}, wantHelp: true},
		{name: "short", args: []string{"-h"}, wantHelp: true},
		{name: "leading-machine-long", args: []string{"--machine", "--help"}, wantHelp: true, machine: true},
		{name: "leading-machine-short", args: []string{"--machine", "-h"}, wantHelp: true, machine: true},
		{name: "help-after-valid-option", args: []string{"--workspace", "ignored", "--help"}, wantHelp: true},
		{name: "help-before-valid-option", args: []string{"--help", "--workspace", "ignored"}, wantHelp: true},
		{name: "help-before-positional", args: []string{"--help", "positional"}, wantCode: 1},
		{name: "short-help-before-positional", args: []string{"-h", "positional"}, wantCode: 1},
		{name: "help-before-machine", args: []string{"--help", "--machine"}, wantHelp: true, machine: true},
		{name: "help-after-machine-server-arg", args: []string{"--server-arg", "--machine", "--help"}, wantHelp: true},
		{name: "help-after-double-dash-server-arg", args: []string{"--server-arg", "--", "--help"}, wantHelp: true},
		{name: "help-before-machine-server-arg", args: []string{"--help", "--server-arg", "--machine"}, wantHelp: true},
		{name: "help-before-double-dash-server-arg", args: []string{"--help", "--server-arg", "--"}, wantHelp: true},
		{name: "duplicate-leading-machine-help", args: []string{"--machine", "--machine=true", "--help"}, wantCode: 1, machine: true},
		{name: "invalid-leading-machine-help", args: []string{"--machine=maybe", "--help"}, wantCode: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			called := false
			deps := productionCensusRunnerDependencies()
			deps.openRoot = func(string) (*publication.Root, error) { called = true; return nil, errors.New("called") }
			code := runCensusWithDependencies(tc.args, &stdout, &stderr, deps)
			if code != tc.wantCode || called {
				t.Fatalf("ASSERT_CENSUS_HELP_VALIDATED_NO_SIDE_EFFECT args=%v code=%d want=%d called=%t stdout=%q stderr=%q", tc.args, code, tc.wantCode, called, stdout.String(), stderr.String())
			}
			if tc.wantHelp {
				if stderr.Len() != 0 || strings.Count(stdout.String(), censusUsage) != 1 {
					t.Fatalf("ASSERT_CENSUS_HELP_ONCE args=%v stdout=%q stderr=%q", tc.args, stdout.String(), stderr.String())
				}
				return
			}
			if stdout.Len() != 0 || strings.Count(stderr.String(), "\n") != 1 {
				t.Fatalf("ASSERT_CENSUS_HELP_INVALID_CLOSED args=%v stdout=%q stderr=%q", tc.args, stdout.String(), stderr.String())
			}
			if tc.machine {
				var d censusCLIDiagnostic
				if err := json.Unmarshal(stderr.Bytes(), &d); err != nil || d.Stage != censusStageSyntax {
					t.Fatalf("ASSERT_CENSUS_MACHINE_SYNTAX_JSONL args=%v diagnostic=%+v err=%v", tc.args, d, err)
				}
			}
		})
	}
}

func TestCensusHelpSurfacesHaveExactSupportedFlagParity(t *testing.T) {
	want := []string{"--config", "--down-depth", "--exclude", "--include", "--machine", "--max-nodes", "--profile", "--publication-root", "--request-timeout", "--server", "--server-arg", "--source", "--timeout", "--up-depth", "--workspace"}
	flags := func(text string) []string {
		t.Helper()
		seen := map[string]bool{}
		for _, field := range strings.Fields(text) {
			field = strings.Trim(field, "[]()")
			field = strings.TrimSuffix(field, "...")
			if strings.HasPrefix(field, "--") {
				seen[field] = true
			}
		}
		got := make([]string, 0, len(seen))
		for flag := range seen {
			got = append(got, flag)
		}
		slices.Sort(got)
		return got
	}
	if got := flags(censusUsage); !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_CENSUS_COMMAND_HELP_SUPPORTED_FLAGS got=%v want=%v", got, want)
	}
	if got := flags(usageText[strings.Index(usageText, "lsp-trace census "):strings.Index(usageText, "\n  lsp-trace slice")]); !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_CENSUS_TOP_LEVEL_HELP_SUPPORTED_FLAGS got=%v want=%v", got, want)
	}
}

func TestRunCensusProfileCLIOverridesAndOutcomeStreams(t *testing.T) {
	workspace, root := t.TempDir(), t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "profiles.toml")
	if err := os.WriteFile(config, []byte("[profiles.test]\ncommand = \"profile-server\"\nargs = [\"profile-arg\"]\nlanguage_ids = [\"go\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(command string) (string, error) {
		if command != "cli-server" {
			t.Fatalf("ASSERT_CENSUS_PROFILE_SERVER_OVERRIDE command=%q", command)
		}
		return "/bin/echo", nil
	}
	coreCalls := 0
	deps.runCore = func(options censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
		coreCalls++
		if cfg.runner.start.Process.Path != "/bin/echo" || !reflect.DeepEqual(cfg.runner.start.Process.Args, []string{"--machine"}) || cfg.runner.start.LanguageID != "go" || options.Workspace != workspace {
			t.Fatalf("ASSERT_CENSUS_PROFILE_OVERRIDE_TRANSFER process=%+v language=%q workspace=%q", cfg.runner.start.Process, cfg.runner.start.LanguageID, options.Workspace)
		}
		if cfg.runner.timeout != 10*time.Minute || cfg.limits.TimeoutMS == nil || *cfg.limits.TimeoutMS != 60000 || cfg.limits.RequestTimeoutMS == nil || *cfg.limits.RequestTimeoutMS != 30000 {
			t.Fatalf("ASSERT_CENSUS_OUTER_AND_BATCH_TIMEOUTS_SEPARATE: runner=%s limits=%+v", cfg.runner.timeout, cfg.limits)
		}
		p := validCensusProjection()
		r, err := buildCensusCLIResult(p, validCensusReceipt())
		if err != nil {
			t.Fatal(err)
		}
		d := mustDiagnostic(t, censusStageCommitted)
		return censusPublicationOutcome{Result: &r, Diagnostic: d}
	}
	args := []string{"--machine", "--workspace", workspace, "--source", ".", "--publication-root", root, "--profile", "test", "--config", config, "--server", "cli-server", "--server-arg", "--machine", "--timeout", "10m", "--request-timeout", "30s"}
	var stdout, stderr bytes.Buffer
	if code := runCensusWithDependencies(args, &stdout, &stderr, deps); code != 0 || coreCalls != 1 || stderr.Len() != 0 || strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("ASSERT_CENSUS_COMMITTED_DEGRADATION_SUCCESS code=%d calls=%d stdout=%q stderr=%q", code, coreCalls, stdout.String(), stderr.String())
	}
	var result censusCLIResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Status != "SUCCEEDED" {
		t.Fatalf("ASSERT_CENSUS_MACHINE_RESULT result=%+v err=%v", result, err)
	}
}

func TestCensusExecutableHelpAndMalformedMachine(t *testing.T) {
	cli := buildFR23Binary(t, "lsp-trace-census", "./cmd/lsp-trace")
	for _, tc := range []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout bool
	}{
		{"help", []string{"census", "--help"}, 0, true},
		{"machine-malformed", []string{"census", "--machine", "positional"}, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(cli, tc.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			code := 0
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tc.wantCode {
				t.Fatalf("ASSERT_CENSUS_PROCESS_EXIT got=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if tc.wantStdout {
				if stderr.Len() != 0 || strings.Count(stdout.String(), censusUsage) != 1 {
					t.Fatalf("ASSERT_CENSUS_PROCESS_HELP stdout=%q stderr=%q", stdout.String(), stderr.String())
				}
				return
			}
			if stdout.Len() != 0 || strings.Count(stderr.String(), "\n") != 1 {
				t.Fatalf("ASSERT_CENSUS_PROCESS_CLOSED stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			var diagnostic censusCLIDiagnostic
			if err := json.Unmarshal(stderr.Bytes(), &diagnostic); err != nil || diagnostic.Stage != censusStageSyntax {
				t.Fatalf("ASSERT_CENSUS_PROCESS_MACHINE_JSON diagnostic=%+v err=%v", diagnostic, err)
			}
		})
	}
}

func TestRunCensusPublicationPreflightBeforeCore(t *testing.T) {
	deps := productionCensusRunnerDependencies()
	coreCalls := 0
	deps.runCore = func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		coreCalls++
		return censusPublicationOutcome{}
	}
	var stdout, stderr bytes.Buffer
	args := []string{"--machine", "--workspace", t.TempDir(), "--publication-root", filepath.Join(t.TempDir(), "missing"), "--server", "server"}
	if code := runCensusWithDependencies(args, &stdout, &stderr, deps); code == 0 || coreCalls != 0 || stdout.Len() != 0 {
		t.Fatalf("ASSERT_CENSUS_PREFLIGHT_BEFORE_CORE code=%d calls=%d stdout=%q", code, coreCalls, stdout.String())
	}
	var diagnostic censusCLIDiagnostic
	if err := json.Unmarshal(stderr.Bytes(), &diagnostic); err != nil || diagnostic.Stage != censusStageConfig {
		t.Fatalf("ASSERT_CENSUS_PREFLIGHT_DIAGNOSTIC diagnostic=%+v err=%v stderr=%q", diagnostic, err, stderr.String())
	}
}
