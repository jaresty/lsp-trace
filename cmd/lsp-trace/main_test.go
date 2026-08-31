package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
	if code != 1 || stdout != "" || stderr == "" {
		t.Fatalf("stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func captureRun(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW
	code := run(args)
	_ = outW.Close()
	_ = errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	var out, stderr bytes.Buffer
	_, _ = out.ReadFrom(outR)
	_, _ = stderr.ReadFrom(errR)
	return out.String(), stderr.String(), code
}
