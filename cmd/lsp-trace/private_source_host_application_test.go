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

	"lsp-trace/internal/runtimeprofile"
)

func TestPrivateSourceHostTerminalBinding(t *testing.T) {
	fs := token.NewFileSet()
	f, e := parser.ParseFile(fs, "private_source_host_application.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	var shutdown, close, check token.Pos
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if s, ok := c.Fun.(*ast.SelectorExpr); ok {
				if x, ok := s.X.(*ast.Ident); ok {
					if x.Name == "manager" && s.Sel.Name == "Shutdown" {
						shutdown = c.Pos()
					}
					if x.Name == "root" && s.Sel.Name == "Close" {
						close = c.Pos()
					}
					if x.Name == "ctx" && s.Sel.Name == "Err" {
						check = c.Pos()
					}
				}
			}
		}
		return true
	})
	raw, e := os.ReadFile("private_source_host_application.go")
	if e != nil || shutdown == 0 || close <= shutdown || check <= close || !strings.Contains(string(raw), "!time.Now().Before(c.Deadline)") || !strings.Contains(string(raw), `out.Disposition = "FAILED_RETAINED"`) {
		t.Fatal("ASSERT_CLI_LAST_OWNED_CLOSE_BEFORE_SUCCESS", e)
	}
	t.Log("ASSERT_CLI_LAST_OWNED_CLOSE_BEFORE_SUCCESS: PASS (finite AST supplement; original deadline; no atomic-clock proof)")
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
