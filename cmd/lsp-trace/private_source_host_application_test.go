//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/inventorysuccessor/sourceoperator"
	"lsp-trace/internal/runtimeprofile"
)

func TestPrivateSourceHostTerminalBinding(t *testing.T) {
	// MODELED: structural custody/order supplement plus pure terminal behavior.
	// This is not physical timing, provider execution, or atomic-clock proof.
	fs := token.NewFileSet()
	function := func(path, name string) *ast.FuncDecl {
		f, err := parser.ParseFile(fs, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
				return fn
			}
		}
		t.Fatalf("missing terminal-chain function %s", name)
		return nil
	}
	ident := func(e ast.Expr, name string) bool { id, ok := e.(*ast.Ident); return ok && id.Name == name }
	selector := func(e ast.Expr, receiver, name string) bool {
		s, ok := e.(*ast.SelectorExpr)
		return ok && ident(s.X, receiver) && s.Sel.Name == name
	}
	call := func(e ast.Expr, receiver, name string) bool {
		c, ok := e.(*ast.CallExpr)
		return ok && selector(c.Fun, receiver, name)
	}
	owner := function("private_source_host_application.go", "executePrivateSourceHostOwned")
	var run, tail, tailBind, tailContext, shutdown, close, now, total, cleanup, build, finish token.Pos
	var buildCall *ast.CallExpr
	bound := map[string]bool{}
	tailValidated := false
	nowCount := 0
	ast.Inspect(owner.Body, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
			lhs, rhs := a.Lhs[0], a.Rhs[0]
			if ident(lhs, "terminalDeadline") && ident(rhs, "tail") {
				tailBind = a.Pos()
			}
			for _, binding := range []struct{ name, receiver, method string }{{"shutdownErr", "manager", "Shutdown"}, {"closeErr", "root", "Close"}, {"now", "time", "Now"}} {
				if ident(lhs, binding.name) && call(rhs, binding.receiver, binding.method) {
					bound[binding.name] = true
				}
			}
			if ident(lhs, "executionErr") && ident(rhs, "e") && a.Pos() > run {
				bound["executionErr"] = true
			}
			if ident(lhs, "terminalDeadline") && selector(rhs, "c", "Deadline") {
				bound["initialDeadline"] = true
			}
			if ident(lhs, "cleanupCtx") && ident(rhs, "ctx") {
				bound["initialContext"] = true
			}
			if ident(lhs, "diagnostic") {
				if c, ok := rhs.(*ast.CallExpr); ok && ident(c.Fun, "privateSourceDiagnosticV1") {
					bound["diagnostic"] = true
				}
			}
		}
		if branch, ok := n.(*ast.IfStmt); ok {
			if c, ok := branch.Cond.(*ast.CallExpr); ok && selector(c.Fun, "tail", "After") && len(c.Args) == 1 && selector(c.Args[0], "c", "Deadline") && branch.Else != nil {
				invalid := false
				ast.Inspect(branch.Body, func(n ast.Node) bool {
					if a, ok := n.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && len(a.Rhs) == 1 && ident(a.Lhs[0], "tailInvalid") && ident(a.Rhs[0], "true") {
						invalid = true
					}
					return true
				})
				selected := false
				ast.Inspect(branch.Else, func(n ast.Node) bool {
					if a, ok := n.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && len(a.Rhs) == 1 && ident(a.Lhs[0], "terminalDeadline") && ident(a.Rhs[0], "tail") {
						selected = true
					}
					return true
				})
				tailValidated = invalid && selected
			}
		}
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch {
		case selector(c.Fun, "sourceoperator", "Run"):
			run = c.Pos()
		case selector(c.Fun, "out", "TerminalDeadlineV2"):
			tail = c.Pos()
		case selector(c.Fun, "context", "WithDeadline") && len(c.Args) == 2 && ident(c.Args[0], "ctx") && ident(c.Args[1], "tail"):
			tailContext = c.Pos()
		case selector(c.Fun, "manager", "Shutdown"):
			if len(c.Args) != 1 || !ident(c.Args[0], "cleanupCtx") {
				t.Fatal("ASSERT_CLI_CLEANUP_CONTEXT")
			}
			shutdown = c.Pos()
		case selector(c.Fun, "root", "Close"):
			close = c.Pos()
		case selector(c.Fun, "time", "Now"):
			now = c.Pos()
			nowCount++
		case selector(c.Fun, "ctx", "Err"):
			total = c.Pos()
		case selector(c.Fun, "cleanupCtx", "Err"):
			cleanup = c.Pos()
		case ident(c.Fun, "privateSourceDiagnosticV1"):
			build = c.Pos()
			buildCall = c
		case ident(c.Fun, "finishPrivateSourceV1"):
			finish = c.Pos()
			if len(c.Args) != 4 || !ident(c.Args[0], "out") || !ident(c.Args[1], "diagnostic") {
				t.Fatal("ASSERT_CLI_FINISH_BINDING")
			}
		}
		return true
	})
	// Argument evaluations belong to Build: their source positions follow the
	// call's opening token, but still must follow the final owned Close.
	if !(run > 0 && tail > run && tailBind > tail && tailContext > tailBind && shutdown > tailContext && close > shutdown && now > close && total > close && cleanup > close && build > now && finish > total && finish > cleanup && finish > build && nowCount == 1) {
		t.Fatal("ASSERT_CLI_LAST_OWNED_CLOSE_BEFORE_SUCCESS")
	}
	for _, name := range []string{"shutdownErr", "closeErr", "now", "executionErr", "initialDeadline", "initialContext", "diagnostic"} {
		if !bound[name] {
			t.Fatal("ASSERT_CLI_TERMINAL_RESULT_BINDING", name)
		}
	}
	if !tailValidated {
		t.Fatal("ASSERT_CLI_TAIL_VALIDATION")
	}
	if buildCall == nil || len(buildCall.Args) != 10 {
		t.Fatal("ASSERT_CLI_TERMINAL_ARGUMENTS")
	}
	// Check all ten actual arguments independently (no obsolete inline literals).
	expected := []string{"out", "executionErr", "shutdownErr", "closeErr", "", "", "contractFailed", "tailInvalid", "now", "terminalDeadline"}
	for i, name := range expected {
		if name != "" && !ident(buildCall.Args[i], name) {
			t.Fatal("ASSERT_CLI_TERMINAL_ARGUMENTS", i)
		}
	}
	if !call(buildCall.Args[4], "ctx", "Err") || !call(buildCall.Args[5], "cleanupCtx", "Err") {
		t.Fatal("ASSERT_CLI_TERMINAL_CONTEXT_OBSERVATIONS")
	}
	builder := function("private_source_failure_diagnostic_v1.go", "privateSourceDiagnosticV1")
	expiryBound := false
	ast.Inspect(builder.Body, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok || !ident(kv.Key, "TerminalDeadlineExpired") {
			return true
		}
		u, ok := kv.Value.(*ast.UnaryExpr)
		if !ok || u.Op != token.NOT {
			return true
		}
		c, ok := u.X.(*ast.CallExpr)
		expiryBound = ok && selector(c.Fun, "now", "Before") && len(c.Args) == 1 && ident(c.Args[0], "deadline")
		return true
	})
	if !expiryBound {
		t.Fatal("ASSERT_CLI_SELECTED_DEADLINE_EXPIRY")
	}
	// Actual pure behavior distinguishes live vs equal/expired tail, close-only
	// refusal, failure-only disposition, and stdout failure's class precedence.
	sample := time.Unix(100, 0)
	for _, mode := range []string{"live", "close", "equal", "expired"} {
		deadline := sample.Add(time.Second)
		var closeErr error
		if mode == "close" {
			closeErr = os.ErrInvalid
		}
		if mode == "equal" {
			deadline = sample
		}
		if mode == "expired" {
			deadline = sample.Add(-time.Nanosecond)
		}
		out := sourceoperator.Outcome{Schema: "SOURCE_NATIVE_V1", Disposition: "MODELED_SUCCESS"}
		d := privateSourceDiagnosticV1(out, nil, nil, closeErr, nil, nil, false, false, sample, deadline)
		var stdout, stderr bytes.Buffer
		code := finishPrivateSourceV1(out, d, &stdout, &stderr)
		failed := mode != "live"
		if d.failed() != failed || (code == 1) != failed {
			t.Fatal("ASSERT_CLI_TERMINAL_PREDICATE", mode)
		}
		var got sourceoperator.Outcome
		if json.Unmarshal(stdout.Bytes(), &got) != nil || (got.Disposition == "FAILED_RETAINED") != failed || (!failed && got.Disposition != out.Disposition) {
			t.Fatal("ASSERT_CLI_TERMINAL_DISPOSITION", mode)
		}
		if !failed && stderr.Len() != 0 {
			t.Fatal("ASSERT_CLI_TERMINAL_SUCCESS_SILENT")
		}
		stderr.Reset()
		if finishPrivateSourceV1(out, d, diagnosticFailWriterV1{}, &stderr) != 1 || !strings.HasPrefix(stderr.String(), "private source application: OUTPUT_REFUSED\n") {
			t.Fatal("ASSERT_CLI_OUTPUT_PRECEDENCE", mode)
		}
	}
	t.Log("ASSERT_CLI_LAST_OWNED_CLOSE_BEFORE_SUCCESS: PASS; ASSERT_CLI_SELECTED_DEADLINE_EXPIRY: PASS; ASSERT_CLI_TERMINAL_PREDICATE: PASS; ASSERT_CLI_TERMINAL_DISPOSITION: PASS; ASSERT_CLI_OUTPUT_PRECEDENCE: PASS (MODELED AST/pure terminal supplement)")
}

func TestPrivateSourceHostDispatch(t *testing.T) {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	c := privateSourceHostConfig{Schema: "private-source-host-application.v1", Mode: "PREVIEW", AccountingRoot: filepath.Join(dir, "missing-owner"), CanonicalJob: []byte("unissued"), ProcessPath: "/never-start-provider", Provider: "lsp", Workspace: "/never-open-workspace", Profile: runtimeprofile.Selector{Workspace: "/never-open-workspace"}, Deadline: time.Now().Add(time.Second), OutputRoots: [2]string{filepath.Join(dir, "missing-a"), filepath.Join(dir, "missing-b")}}
	c.Pin = privateSourceConfigPin(c)
	raw, _ := json.Marshal(c)
	path := filepath.Join(dir, "config.json")
	invoke := func(raw []byte) (int, string, string) {
		t.Helper()
		if e := os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		var out, err bytes.Buffer
		code := runPrivateHostApplication([]string{"source-describe", path}, strings.NewReader(""), &out, &err)
		return code, out.String(), err.String()
	}
	code, out, err := invoke(raw)
	if code != 0 || !strings.Contains(out, "PREVIEW_NO_EXECUTION") || err != "" {
		t.Fatal("ASSERT_CLI_PREVIEW", code, out, err)
	}
	for _, p := range []string{c.AccountingRoot, c.OutputRoots[0], c.OutputRoots[1]} {
		if _, e := os.Lstat(p); !os.IsNotExist(e) {
			t.Fatal("ASSERT_CLI_PREVIEW_NO_EFFECT", p, e)
		}
	}
	c.Mode = "EXECUTE"
	c.Pin = privateSourceConfigPin(c)
	raw, _ = json.Marshal(c)
	code, _, err = invoke(raw)
	if code == 0 || !strings.Contains(err, "OWNER_ADMISSION_REFUSED") {
		t.Fatal("ASSERT_CLI_OWNER_BEFORE_EFFECTS", code, err)
	}
	for _, p := range c.OutputRoots {
		if _, e := os.Lstat(p); !os.IsNotExist(e) {
			t.Fatal("ASSERT_CLI_OWNER_BEFORE_EFFECTS", p, e)
		}
	}
	for _, bad := range [][]byte{append(append([]byte(nil), raw...), []byte(` {}`)...), bytes.Replace(raw, []byte(`"Schema":`), []byte(`"unknown":1,"Schema":`), 1), bytes.Replace(raw, []byte(`"EXECUTE"`), []byte(`"execute"`), 1)} {
		code, _, err = invoke(bad)
		if code == 0 || !strings.Contains(err, "CONFIG_REFUSED") {
			t.Fatal("ASSERT_CLI_STRICT_CONFIG", code, err)
		}
	}
	t.Log("ASSERT_CLI_PREVIEW: PASS; ASSERT_CLI_OWNER_BEFORE_EFFECTS: PASS; ASSERT_CLI_STRICT_CONFIG: PASS")
}
