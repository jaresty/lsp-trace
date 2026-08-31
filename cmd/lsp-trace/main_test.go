package main

import (
	"bytes"
	"os"
	"path/filepath"
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
