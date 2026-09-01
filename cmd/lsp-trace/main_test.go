package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lsp"
)

func validArgs(workspace string) []string {
	return []string{"--workspace", workspace, "--server", "server", "--at", "main.go:1:1"}
}

func TestParseRejectsTrailingArguments(t *testing.T) {
	_, err := parse(append(validArgs(t.TempDir()), "unexpected"))
	if err == nil || !strings.Contains(err.Error(), "unexpected positional") {
		t.Fatalf("parse error = %v, want unexpected positional argument", err)
	}
}

func TestParseValidatesFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"required", nil, "required"},
		{"negative limit", append(validArgs(t.TempDir()), "--max-depth", "-1"), "non-negative"},
		{"zero request timeout", append(validArgs(t.TempDir()), "--request-timeout", "0"), "request-timeout must be greater than zero"},
		{"bad env missing equals", append(validArgs(t.TempDir()), "--server-env", "KEY"), "invalid --server-env"},
		{"bad env empty key", append(validArgs(t.TempDir()), "--server-env", "=value"), "invalid --server-env"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parse error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestRunPackagesCallerSuppliedInvocationProvenance(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(validArgs(workspace),
		"--provenance-invocation-id", "run-123",
		"--provenance-caller", "audit-agent",
		"--provenance-source", "review-request",
		"--provenance-source-revision", "commit-abc",
		"--provenance-server-version", "server-1.2.3",
		"--provenance-timestamp", "2026-08-31T19:00:00Z",
		"--provenance-tool-version", "v0.3.0",
	)
	stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
	var receipt struct {
		Tool struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"tool"`
		Invocation struct {
			WorkspaceURI string `json:"workspace_uri"`
			Server       struct {
				Command string `json:"command"`
			} `json:"server"`
			Provenance struct {
				InvocationID   string `json:"invocation_id"`
				Caller         string `json:"caller"`
				Source         string `json:"source"`
				SourceRevision string `json:"source_revision"`
				ServerVersion  string `json:"server_version"`
				Timestamp      string `json:"timestamp"`
			} `json:"provenance"`
		} `json:"invocation"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("ASSERT_RECEIPT_STDOUT_JSON: %v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if receipt.Tool.Name != "lsp-trace" || receipt.Tool.Version != "v0.3.0" {
		t.Fatalf("ASSERT_RECEIPT_TOOL_IDENTITY: tool=%#v stdout=%s", receipt.Tool, stdout)
	}
	if receipt.Invocation.WorkspaceURI == "" || receipt.Invocation.Server.Command != "server" {
		t.Fatalf("ASSERT_RECEIPT_INVOCATION_PARAMETERS: invocation=%#v", receipt.Invocation)
	}
	if got := receipt.Invocation.Provenance; got.InvocationID != "run-123" || got.Caller != "audit-agent" || got.Source != "review-request" || got.SourceRevision != "commit-abc" || got.ServerVersion != "server-1.2.3" || got.Timestamp != "2026-08-31T19:00:00Z" {
		t.Fatalf("ASSERT_CALLER_SUPPLIED_PROVENANCE: provenance=%#v", got)
	}
	if code != 1 || !strings.Contains(stderr, "spawn:") {
		t.Fatalf("ASSERT_RECEIPT_STREAM_CONTRACT: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
}

func TestRunUsesUnknownForOmittedInvocationProvenance(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ := captureRun(t, append([]string{"incoming"}, validArgs(workspace)...))
	var receipt struct {
		Invocation struct {
			Provenance map[string]string `json:"provenance"`
		} `json:"invocation"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("ASSERT_UNKNOWN_PROVENANCE_STDOUT_JSON: %v stdout=%q", err, stdout)
	}
	for _, key := range []string{"invocation_id", "caller", "source", "source_revision", "server_version", "timestamp"} {
		if receipt.Invocation.Provenance[key] != "UNKNOWN" {
			t.Fatalf("ASSERT_OMITTED_PROVENANCE_UNKNOWN: key=%s provenance=%#v", key, receipt.Invocation.Provenance)
		}
	}
}

func TestParseAcceptsTopmostSiblingsOptIn(t *testing.T) {
	cfg, err := parse(append(validArgs(t.TempDir()), "--expand-topmost-siblings"))
	if err != nil || !cfg.topmostSiblings {
		t.Fatalf("ASSERT_TOPMOST_SIBLINGS_CLI_OPT_IN: enabled=%t err=%v", cfg.topmostSiblings, err)
	}
}

func TestUsageAdvertisesIncomingAndEmbeddedSkill(t *testing.T) {
	if !strings.Contains(usageText, "lsp-trace incoming") || !strings.Contains(usageText, "lsp-trace skill get") {
		t.Fatalf("ASSERT_USAGE_ADVERTISES_ALL_COMMANDS: %q", usageText)
	}
}

func TestEmbeddedSkillGetIsExactAndHermetic(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runSkill([]string{"get"}, &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.String() != embeddedSkill {
		t.Fatalf("ASSERT_EMBEDDED_SKILL_GET: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	requiredContract := []string{
		"name: lsp-trace",
		"--expand-dispatch-family",
		"--expand-topmost-siblings",
		`{"seeds":[`,
		"Produce a provisional feature inventory",
		"do not establish feature identity",
		"evidence_semantics",
		"trace_receipt",
		"support_contribution",
		"--provenance-source-revision",
		"exactly one `seeds` result",
		"direct canonical call-relation IDs",
		"does not resolve cwd symlink aliases",
		"Field authority",
	}
	for _, required := range requiredContract {
		if !strings.Contains(stdout.String(), required) {
			t.Fatalf("ASSERT_EMBEDDED_SKILL_DOWNSTREAM_CONTRACT: missing=%q skill=%s", required, stdout.String())
		}
	}
	stdout.Reset()
	stderr.Reset()
	if code := runSkill([]string{"list"}, &stdout, &stderr); code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage: lsp-trace skill get") {
		t.Fatalf("ASSERT_EMBEDDED_SKILL_REJECTS_UNSUPPORTED: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestParseAcceptsDispatchFamilyOptIn(t *testing.T) {
	cfg, err := parse(append(validArgs(t.TempDir()), "--expand-dispatch-family"))
	if err != nil || !cfg.expandDispatchFamily {
		t.Fatalf("ASSERT_DISPATCH_FAMILY_CLI_OPT_IN: enabled=%t err=%v", cfg.expandDispatchFamily, err)
	}
}

type dispatchIntegrationClient struct{}

func (dispatchIntegrationClient) SupportsTypeHierarchy() bool { return true }
func (dispatchIntegrationClient) PrepareTypeHierarchy(context.Context, lsp.PrepareTypeHierarchyParams) ([]lsp.TypeHierarchyItem, error) {
	return []lsp.TypeHierarchyItem{{Name: "Contract", URI: "file:///contract", SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}}, nil
}
func (dispatchIntegrationClient) Subtypes(_ context.Context, item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error) {
	if item.Name != "Contract" {
		return nil, nil
	}
	return []lsp.TypeHierarchyItem{{Name: "Implementation", URI: "file:///implementation", SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}}, nil
}

func TestResolveDispatchRelationshipsPreservesSeedAndSeparateNodes(t *testing.T) {
	relationships, diagnostics := resolveDispatchRelationships(context.Background(), dispatchIntegrationClient{}, lsp.PrepareTypeHierarchyParams{}, "entry")
	if len(relationships) != 1 || relationships[0].SeedLabel != "entry" || relationships[0].Interface.Name != "Contract" || relationships[0].Implementation.Name != "Implementation" || len(diagnostics) != 0 {
		t.Fatalf("ASSERT_DISPATCH_INTEGRATION_RELATIONSHIP: relationships=%#v diagnostics=%#v", relationships, diagnostics)
	}
}

func TestParseConcurrencyContract(t *testing.T) {
	if _, err := parse(append(validArgs(t.TempDir()), "--concurrency", "1")); err != nil {
		t.Fatalf("concurrency 1: %v", err)
	}
	for _, value := range []string{"0", "2", "-1"} {
		t.Run(value, func(t *testing.T) {
			_, err := parse(append(validArgs(t.TempDir()), "--concurrency", value))
			if err == nil || !strings.Contains(err.Error(), "--concurrency must be 1") {
				t.Fatalf("parse error = %v, want concurrency validation", err)
			}
		})
	}
}

func TestParseLogLevelContract(t *testing.T) {
	for _, level := range []string{"error", "warn", "info", "debug"} {
		t.Run(level, func(t *testing.T) {
			if _, err := parse(append(validArgs(t.TempDir()), "--log-level", level)); err != nil {
				t.Fatalf("valid log level: %v", err)
			}
		})
	}
	_, err := parse(append(validArgs(t.TempDir()), "--log-level", "trace"))
	if err == nil || !strings.Contains(err.Error(), "invalid --log-level") {
		t.Fatalf("parse error = %v, want log-level validation", err)
	}
}

func TestParseAcceptsRepeatableFlags(t *testing.T) {
	args := append(validArgs(t.TempDir()), "--server-arg", "--stdio", "--server-arg", "x", "--server-env", "A=1", "--server-env", "B=")
	cfg, err := parse(args)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.args, ","); got != "--stdio,x" {
		t.Fatalf("args = %q", got)
	}
	if got := strings.Join(cfg.env, ","); got != "A=1,B=" {
		t.Fatalf("env = %q", got)
	}
	if cfg.requestTimeout != 30*time.Second {
		t.Fatalf("request timeout = %s", cfg.requestTimeout)
	}
}

func TestParseRejectsDuplicateServerEnvNames(t *testing.T) {
	for _, declarations := range [][]string{{"TOKEN=one", "TOKEN=one"}, {"TOKEN=one", "TOKEN=two"}} {
		args := append(validArgs(t.TempDir()), "--server-env", declarations[0], "--server-env", declarations[1])
		stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
		if code != 1 || stdout != "" || strings.TrimSpace(stderr) != `duplicate --server-env name "TOKEN"` {
			t.Fatalf("ASSERT_DUPLICATE_SERVER_ENV_PARSE_REJECTION: declarations=%v code=%d stdout=%q stderr=%q", declarations, code, stdout, stderr)
		}
	}
}

func TestParseAcceptsDistinctServerEnvNames(t *testing.T) {
	if _, err := parse(append(validArgs(t.TempDir()), "--server-env", "TOKEN=one", "--server-env", "OTHER=two")); err != nil {
		t.Fatalf("ASSERT_DISTINCT_SERVER_ENV_NAMES_PASS: %v", err)
	}
}

func TestParseAcceptsRepeatedAtAndSeedFile(t *testing.T) {
	workspace := t.TempDir()
	seedFile := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(seedFile, []byte(`{"seeds":[{"label":"interface","at":"main.go:1:1"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--workspace", workspace, "--server", "server", "--at", "main.go:1:1", "--at", "main.go:2:1", "--seed-file", seedFile}
	if _, err := parse(args); err != nil {
		t.Fatalf("ASSERT_REPEATABLE_AT_ACCEPTED: %v", err)
	}
}

func TestParseRejectsZeroSeedsAndInvalidOrDuplicateLabels(t *testing.T) {
	workspace := t.TempDir()
	base := []string{"--workspace", workspace, "--server", "server"}
	if _, err := parse(base); err == nil || !strings.Contains(err.Error(), "seed") {
		t.Fatalf("ASSERT_SEED_FILE_VALIDATION: zero seeds error=%v", err)
	}
	for name, body := range map[string]string{
		"invalid":   `{"seeds":[{"label":"","at":"main.go:1:1"}]}`,
		"duplicate": `{"seeds":[{"label":"same","at":"main.go:1:1"},{"label":"same","at":"main.go:2:1"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "seeds.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := parse(append(base, "--seed-file", path)); err == nil || !strings.Contains(err.Error(), "label") {
				t.Fatalf("ASSERT_SEED_FILE_VALIDATION: %s error=%v", name, err)
			}
		})
	}
}

func TestParseAt(t *testing.T) {
	path, line, col, err := parseAt("C:\\src\\file.go:12:34")
	if err != nil || path != "C:\\src\\file.go" || line != 12 || col != 34 {
		t.Fatalf("parseAt = %q,%d,%d,%v", path, line, col, err)
	}
	for _, input := range []string{"file.go", "file.go:x:1", "file.go:1:0", ":1:1"} {
		t.Run(input, func(t *testing.T) {
			if _, _, _, err := parseAt(input); err == nil {
				t.Fatalf("parseAt(%q) succeeded", input)
			}
		})
	}
}

func TestRunUsageAndParseErrorsUseStderrOnly(t *testing.T) {
	for _, args := range [][]string{nil, {"outgoing"}, {"incoming", "--workspace", "x"}} {
		stdout, stderr, code := captureRun(t, args)
		if code != 1 || stdout != "" || stderr == "" {
			t.Fatalf("run(%v) stdout=%q stderr=%q code=%d", args, stdout, stderr, code)
		}
	}
}

func TestRuntimeHelperServer(t *testing.T) {
	if os.Getenv("LSP_TRACE_RUNTIME_HELPER") == "" {
		return
	}
	fmt.Fprintln(os.Stderr, "runtime helper diagnostic")
	r := bufio.NewReader(os.Stdin)
	for {
		body, err := readRuntimeMessage(r)
		if err != nil {
			return
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(body, &msg) != nil || len(msg.ID) == 0 {
			continue
		}
		switch msg.Method {
		case "initialize":
			if os.Getenv("LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE") == "1" {
				return
			}
			writeRuntimeResponse(msg.ID, map[string]any{"capabilities": map[string]any{"callHierarchyProvider": true}})
		case "textDocument/prepareCallHierarchy":
			time.Sleep(3 * time.Second)
			writeRuntimeResponse(msg.ID, []any{})
		case "shutdown":
			writeRuntimeResponse(msg.ID, nil)
			return
		}
	}
}

func readRuntimeMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(name, "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing content length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeRuntimeResponse(id json.RawMessage, result any) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func assertMixedFailureSeeds(t *testing.T, resultJSON []byte, globalPhase string) {
	t.Helper()
	var got struct {
		SchemaVersion string `json:"schema_version"`
		Invocation    struct {
			Seeds []struct {
				Label string `json:"label"`
			} `json:"seeds"`
		} `json:"invocation"`
		Seeds []struct {
			Label   string `json:"label"`
			Failure *struct {
				Phase string `json:"phase"`
			} `json:"failure"`
		} `json:"seeds"`
		Summary struct {
			TraversalComplete bool `json:"traversal_complete"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(resultJSON, &got); err != nil {
		t.Fatalf("ASSERT_MIXED_FAILURE_SERIALIZES: %v: %s", err, resultJSON)
	}
	phases := map[string]string{}
	resultLabels := map[string]struct{}{}
	for _, seed := range got.Seeds {
		if _, duplicate := resultLabels[seed.Label]; duplicate {
			t.Fatalf("ASSERT_MIXED_PREFLIGHT_GLOBAL_UNIQUE_RESULT_LABELS: %#v", got.Seeds)
		}
		resultLabels[seed.Label] = struct{}{}
		if seed.Failure != nil {
			phases[seed.Label] = seed.Failure.Phase
		}
	}
	invocationLabels := map[string]struct{}{}
	for _, seed := range got.Invocation.Seeds {
		invocationLabels[seed.Label] = struct{}{}
	}
	if got.SchemaVersion != "lsp-trace.graph.v3" || len(got.Invocation.Seeds) != 2 || len(got.Seeds) != 2 || !reflect.DeepEqual(invocationLabels, resultLabels) || phases["bad"] != "source" || phases["good"] != globalPhase || got.Summary.TraversalComplete {
		t.Fatalf("ASSERT_MIXED_PREFLIGHT_GLOBAL_ONE_RESULT_PER_SEED: schema=%q invocation=%#v seeds=%#v phases=%#v complete=%v", got.SchemaVersion, got.Invocation.Seeds, got.Seeds, phases, got.Summary.TraversalComplete)
	}
}

func TestRunMixedSourceAndSpawnFailurePreservesEverySeed(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ := captureRun(t, []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--seed-file", writeMixedSeedFile(t)})
	assertMixedFailureSeeds(t, []byte(stdout), "spawn")
}

func TestRunMixedSourceAndTraceOpenFailurePreservesEverySeed(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ := captureRun(t, []string{"incoming", "--workspace", workspace, "--server", "server", "--seed-file", writeMixedSeedFile(t), "--trace-lsp", t.TempDir()})
	assertMixedFailureSeeds(t, []byte(stdout), "trace")
}

func writeMixedSeedFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(path, []byte(`{"seeds":[{"label":"bad","at":"missing.go:1:1"},{"label":"good","at":"good.go:1:1"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertPublishedMixedFailure(t *testing.T, args []string, globalPhase string) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "bundle.json")
	stdout, stderr, code := captureRun(t, append(args, "--output", output))
	if code == 0 || stdout != "" {
		t.Fatalf("ASSERT_MIXED_FAILURE_PUBLISH_EXIT: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	artifact, err := readSelectedArtifact(output)
	if err != nil {
		t.Fatal(err)
	}
	assertMixedFailureSeeds(t, artifact, globalPhase)
	verifyOut, verifyErr, verifyCode := captureRun(t, []string{"verify", output})
	if verifyCode != 0 || verifyOut != "verified integrity and custody\n" || verifyErr != "" {
		t.Fatalf("ASSERT_MIXED_FAILURE_PUBLISH_VERIFY: code=%d stdout=%q stderr=%q", verifyCode, verifyOut, verifyErr)
	}
}

func TestMixedMissingSourceAndSpawnPreservesEverySeedResult(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assertPublishedMixedFailure(t, []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--seed-file", writeMixedSeedFile(t)}, "spawn")
}

func TestMixedMissingSourceAndTraceOpenPreservesEverySeedResult(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assertPublishedMixedFailure(t, []string{"incoming", "--workspace", workspace, "--server", "server", "--seed-file", writeMixedSeedFile(t), "--trace-lsp", t.TempDir()}, "trace")
}

func TestMixedMissingSourceAndInitializePreservesEverySeedResult(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"incoming", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=TestRuntimeHelperServer", "--server-env", "LSP_TRACE_RUNTIME_HELPER=1", "--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1", "--seed-file", writeMixedSeedFile(t), "--request-timeout", "1s"}
	assertPublishedMixedFailure(t, args, "initialize")
}

func TestRunInitializeFailureIsIncomplete(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(validArgs(workspace),
		"--server", os.Args[0],
		"--server-arg", "-test.run=TestRuntimeHelperServer",
		"--server-env", "LSP_TRACE_RUNTIME_HELPER=1",
		"--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1",
		"--request-timeout", "1s",
	)
	stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
	var result struct {
		Summary struct {
			TraversalComplete bool `json:"traversal_complete"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not graph JSON: %v; stdout=%q stderr=%q", err, stdout, stderr)
	}
	if code != 1 || result.Summary.TraversalComplete {
		t.Fatalf("ASSERT_INITIALIZE_FAILURE_INCOMPLETE: code=%d complete=%t stdout=%s stderr=%s", code, result.Summary.TraversalComplete, stdout, stderr)
	}
}

func TestRunMixedSourceAndInitializeFailurePreservesEverySeed(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"incoming", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=TestRuntimeHelperServer", "--server-env", "LSP_TRACE_RUNTIME_HELPER=1", "--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1", "--seed-file", writeMixedSeedFile(t), "--request-timeout", "1s"}
	stdout, _, _ := captureRun(t, args)
	assertMixedFailureSeeds(t, []byte(stdout), "initialize")
}

func TestRunRequestTimeoutTraceStderrAndExitPolicy(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(t.TempDir(), "protocol.jsonl")
	args := append(validArgs(workspace),
		"--server", os.Args[0],
		"--server-arg", "-test.run=TestRuntimeHelperServer",
		"--server-env", "LSP_TRACE_RUNTIME_HELPER=1",
		"--request-timeout", "1s",
		"--trace-lsp", tracePath,
		"--log-level", "debug",
	)
	stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
	if code != 2 {
		t.Fatalf("code = %d, want structured-incomplete exit 2; stdout=%s stderr=%s", code, stdout, stderr)
	}
	var result struct {
		Terminals []struct {
			Reason string `json:"reason"`
		} `json:"terminals"`
		Diagnostics []struct {
			Phase   string `json:"phase"`
			Message string `json:"message"`
		} `json:"diagnostics"`
		Summary struct {
			Complete bool `json:"complete"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not one graph JSON document: %v; %q", err, stdout)
	}
	if result.Summary.Complete || len(result.Terminals) == 0 || result.Terminals[0].Reason != "REQUEST_TIMEOUT" {
		t.Fatalf("result = %+v, want REQUEST_TIMEOUT incomplete graph", result)
	}
	if !strings.Contains(stderr, "runtime helper diagnostic") {
		t.Fatalf("stderr = %q, want captured server diagnostic", stderr)
	}
	foundServerStderr := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Phase == "server-stderr" && strings.Contains(diagnostic.Message, "runtime helper diagnostic") {
			foundServerStderr = true
		}
	}
	if !foundServerStderr {
		t.Fatalf("diagnostics = %+v, want retained server stderr", result.Diagnostics)
	}
	trace, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(trace), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("trace lines = %d, want sent and received events", len(lines))
	}
	for _, line := range lines {
		var event struct {
			Sequence  uint64          `json:"sequence"`
			Direction string          `json:"direction"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &event); err != nil || event.Sequence == 0 || event.Direction == "" || !json.Valid(event.Payload) {
			t.Fatalf("invalid trace event %q: %+v, %v", line, event, err)
		}
	}
}

func TestRunOutputOpenFailureUsesStderrOnly(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(target, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(validArgs(workspace), "--output", filepath.Join(workspace, "missing", "result.json"))
	stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
	if code != 1 || !json.Valid([]byte(stdout)) || !strings.Contains(stderr, "publish:") {
		t.Fatalf("ASSERT_OUTPUT_PUBLICATION_FAILURE_RETENTION: stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestCaptureRunDrainsLargeStdout(t *testing.T) {
	old := embeddedSkill
	embeddedSkill = strings.Repeat("large-output\n", 1<<17)
	t.Cleanup(func() { embeddedSkill = old })
	stdout, stderr, code := captureRun(t, []string{"skill", "get"})
	if code != 0 || stderr != "" || stdout != embeddedSkill {
		t.Fatalf("ASSERT_P4_LARGE_STREAM_CAPTURE: code=%d stdout=%d want=%d stderr=%q", code, len(stdout), len(embeddedSkill), stderr)
	}
}

func TestParseSliceUsesSymmetricDepthFlagsAndRejectsMaxDepth(t *testing.T) {
	cfg, err := parseSlice([]string{"--workspace", "/w", "--server", "server", "--from-file", "a.go", "--down-depth", "2", "--up-depth", "7"})
	if err != nil || cfg.downDepth != 2 || cfg.upDepth != 7 || cfg.fromFile != "a.go" {
		t.Fatalf("ASSERT_SLICE_SYMMETRIC_DEPTH_FLAGS: cfg=%#v err=%v", cfg, err)
	}
	if _, err := parseSlice([]string{"--workspace", "/w", "--server", "server", "--from-file", "a.go", "--max-depth", "7"}); err == nil {
		t.Fatal("ASSERT_SLICE_MAX_DEPTH_NOT_EXPOSED: accepted --max-depth")
	}
}

func TestSchemaGetAndValidateCommands(t *testing.T) {
	for _, version := range []string{"v1", "v2", "v3"} {
		version := version
		t.Run("schema get "+version, func(t *testing.T) {
			stdout, stderr, code := captureRun(t, []string{"schema", "get", "--schema", version})
			full := "lsp-trace.graph." + version
			if code != 0 || stderr != "" || !strings.Contains(stdout, `"$schema": "https://json-schema.org/draft/2020-12/schema"`) || !strings.Contains(stdout, `"const": "`+full+`"`) {
				t.Errorf("ASSERT_SCHEMA_GET_%s: code=%d stdout=%q stderr=%q", strings.ToUpper(version), code, stdout, stderr)
			}
		})
	}

	valid := filepath.Join(t.TempDir(), "valid-v1.json")
	validJSON := `{"schema_version":"lsp-trace.graph.v1","invocation":{},"capabilities":{},"targets":[],"nodes":[],"edges":[],"terminals":[],"frontier":[],"diagnostics":[],"summary":{}}`
	if err := os.WriteFile(valid, []byte(validJSON), 0600); err != nil {
		t.Fatal(err)
	}
	t.Run("validate autodetect", func(t *testing.T) {
		stdout, stderr, code := captureRun(t, []string{"validate", valid})
		if code != 0 || stdout != "valid lsp-trace.graph.v1\n" || stderr != "" {
			t.Errorf("ASSERT_SCHEMA_VALIDATE_AUTODETECT: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
	t.Run("validate mismatch", func(t *testing.T) {
		stdout, stderr, code := captureRun(t, []string{"validate", "--schema", "v2", valid})
		if code == 0 || stdout != "" || !strings.Contains(stderr, "schema version mismatch") {
			t.Errorf("ASSERT_SCHEMA_VALIDATE_EXPLICIT_MISMATCH: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
	t.Run("validate stdin", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := runValidate([]string{"-"}, strings.NewReader(validJSON), &stdout, &stderr)
		if code != 0 || stdout.String() != "valid lsp-trace.graph.v1\n" || stderr.String() != "" {
			t.Errorf("ASSERT_SCHEMA_VALIDATE_STDIN: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	invalid := filepath.Join(t.TempDir(), "invalid-v1.json")
	if err := os.WriteFile(invalid, []byte(`{"schema_version":"lsp-trace.graph.v1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Run("validate required field", func(t *testing.T) {
		stdout, stderr, code := captureRun(t, []string{"validate", invalid})
		if code == 0 || stdout != "" || !strings.Contains(stderr, "schema validation") || !strings.Contains(stderr, "invocation") {
			t.Errorf("ASSERT_SCHEMA_VALIDATE_REQUIRED_FIELD: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
}

func captureRun(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW
	var out, stderr bytes.Buffer
	var drains sync.WaitGroup
	drains.Add(2)
	go func() { defer drains.Done(); _, _ = out.ReadFrom(outR) }()
	go func() { defer drains.Done(); _, _ = stderr.ReadFrom(errR) }()
	code := run(args)
	_ = outW.Close()
	_ = errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	drains.Wait()
	return out.String(), stderr.String(), code
}
