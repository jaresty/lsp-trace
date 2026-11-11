//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	inv "lsp-trace/internal/inventorysuccessor"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPrivateSourceCompleteDescribeConfig(t *testing.T) {
	c := privateSourceCompleteDescribeConfig{Schema: "private-source-complete-describe.v1", AccountingRoot: "/private/var/not-opened", Workspace: "/private/var/source-not-opened", SuppliedFile: "/private/var/answer-not-opened", CanonicalJob: []byte("not a grant"), ProtectedRoots: []string{"/private/var/not-opened"}}
	c.Open.Anchors.RootPath = c.AccountingRoot
	c.Open.Deadline = time.Now().Add(time.Second)
	for i := range c.Destinations {
		c.Destinations[i].Path = "/private/var/output-not-opened"
	}
	for _, fault := range []string{"valid", "HOST", "Manager", "Model", "Prepare", "unknown", "duplicate", "trailing", "noncanonical", "oversize"} {
		t.Run(fault, func(t *testing.T) {
			raw, _ := json.Marshal(c)
			switch fault {
			case "valid":
			case "duplicate":
				raw = append([]byte("{\"Schema\":\""+c.Schema+"\","), raw[1:]...)
			case "trailing":
				raw = append(raw, []byte("{}")...)
			case "noncanonical":
				raw = append(raw, '\n')
			case "oversize":
				raw = bytes.Repeat([]byte("x"), 65537)
			default:
				raw = append([]byte("{\""+fault+"\":true,"), raw[1:]...)
			}
			_, e := decodePrivateSourceCompleteDescribeConfig(raw)
			if (e == nil) != (fault == "valid") {
				t.Fatal("ASSERT_COMPLETE_CLI_STRICT", fault, e)
			}
		})
	}
}
func TestPrivateSourceCompleteDescribeFileRefused(t *testing.T) {
	for _, fault := range []string{"fifo", "symlink", "mode", "oversize", "unknown-control"} {
		t.Run(fault, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config")
			switch fault {
			case "fifo":
				if e := syscall.Mkfifo(p, 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				if e := os.WriteFile(p+".target", []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(p+".target", p); e != nil {
					t.Fatal(e)
				}
			default:
				raw := []byte("{\"HOST\":true}")
				mode := os.FileMode(0600)
				if fault == "mode" {
					mode = 0644
				}
				if fault == "oversize" {
					raw = bytes.Repeat([]byte("x"), 65537)
				}
				if e := os.WriteFile(p, raw, mode); e != nil {
					t.Fatal(e)
				}
			}
			var out, diag bytes.Buffer
			if runPrivateHostApplication([]string{"source-complete-describe", p}, nil, &out, &diag) == 0 || out.Len() != 0 || !strings.Contains(diag.String(), "CONFIG_REFUSED") {
				t.Fatal("ASSERT_COMPLETE_CLI_FILE_ZERO", fault, diag.String())
			}
		})
	}
}
func TestPrivateSourceCompleteDescribeNoExecution(t *testing.T) {
	raw, e := os.ReadFile("private_source_complete_describe.go")
	if e != nil {
		t.Fatal(e)
	}
	f, e := parser.ParseFile(token.NewFileSet(), "complete-cli.go", raw, 0)
	if e != nil {
		t.Fatal(e)
	}
	fields := map[string]bool{}
	for _, n := range []string{"Schema", "AccountingRoot", "ExpectedHead", "Attempt", "Workspace", "SuppliedFile", "CanonicalJob", "Verification", "Open", "Destinations", "ProtectedRoots"} {
		fields[n] = true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		s, ok := n.(*ast.TypeSpec)
		if !ok || s.Name.Name != "privateSourceCompleteDescribeConfig" {
			return true
		}
		st, ok := s.Type.(*ast.StructType)
		if !ok {
			t.Fatal("config not struct")
		}
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				if !fields[name.Name] {
					t.Error("ASSERT_COMPLETE_CLI_NO_EXECUTION unexpected input", name.Name)
				}
			}
		}
		return false
	})
	for _, needle := range []string{"sessionruntime", "os/exec", "PrepareDescribeV1(", "ReserveMixedInputV2(", ".Start(", ".Run(", ".Finish("} {
		if bytes.Contains(raw, []byte(needle)) {
			t.Error("ASSERT_COMPLETE_CLI_NO_EXECUTION", needle)
		}
	}
	if !t.Failed() {
		t.Log("ASSERT_COMPLETE_CLI_NO_EXECUTION: PASS (bounded direct source/control-shape guard)")
	}
}
func TestPrivateSourceCompleteDescribeOutputFailure(t *testing.T) {
	for _, w := range []interface{ Write([]byte) (int, error) }{readyShortWriter{}, readyErrorWriter{}} {
		var diag bytes.Buffer
		if finishPrivateSourceCompleteDescribe(context.Background(), time.Now().Add(time.Second), inv.CompletedDescribeV1{ObjectID: "writer-only-not-native-qualification"}, nil, w, &diag) == 0 || !strings.Contains(diag.String(), "POSSIBLY_PARTIAL") {
			t.Fatal("ASSERT_COMPLETE_CLI_PARTIAL_OUTPUT")
		}
	}
}
