//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	ai "lsp-trace/internal/applicationinventory"
	"lsp-trace/internal/lsp"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// No production hook: only the existing offline source/provider fixture varies.
// Comments are after the price() call so its original query offsets do not move.
func scaleFixtureSource(t *testing.T, workspace string) {
	text := os.Getenv("DURABLE_SCALE_PADDING")
	if text == "" {
		return
	}
	n, e := strconv.Atoi(text)
	if e != nil || (n != 0 && n != 256 && n != 1024) {
		t.Fatal("unapproved scale arm")
	}
	for _, name := range []string{"checkout.go", "extra.go", "extra2.go"} {
		path := filepath.Join(workspace, name)
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if n > 0 {
			raw = bytes.Replace(raw, []byte("}\n"), []byte("/*"+strings.Repeat("x", n)+"*/ }\n"), 1)
		}
		lines := strings.Split(string(raw), "\n")
		if len(lines) != 3 || lines[1][29:34] != "price" {
			t.Fatal("source query offset moved")
		}
		if _, e = parser.ParseFile(token.NewFileSet(), path, raw, parser.AllErrors); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		t.Logf("SCALE_SOURCE file=%s padding=%d bytes=%d functionBytes=%d query=1:29-34", name, n, len(raw), len(lines[1]))
	}
}

func scaleFixtureItem(item lsp.CallHierarchyItem) lsp.CallHierarchyItem {
	if os.Getenv("DURABLE_SCALE_PADDING") == "" {
		return item
	}
	u, e := url.Parse(item.URI)
	if e != nil {
		panic(e)
	}
	raw, e := os.ReadFile(u.Path)
	if e != nil {
		panic(e)
	}
	fs := token.NewFileSet()
	f, e := parser.ParseFile(fs, u.Path, raw, parser.AllErrors)
	if e != nil {
		panic(e)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != item.Name {
			continue
		}
		pos := func(p token.Pos) lsp.Position {
			v := fs.Position(p)
			return lsp.Position{Line: uint32(v.Line - 1), Character: uint32(v.Column - 1)}
		}
		item.Range = lsp.Range{Start: pos(fn.Pos()), End: pos(fn.End())}
		item.SelectionRange = lsp.Range{Start: pos(fn.Name.Pos()), End: pos(fn.Name.End())}
		return item
	}
	panic("controlled source declaration not found")
}

func TestCensusEvidenceSizeMatrix(t *testing.T) {
	// Frozen before execution: no adaptive budget or size tuning after refusals.
	for _, padding := range []int{0, 256, 1024} {
		t.Run(strconv.Itoa(padding), func(t *testing.T) {
			t.Setenv("DURABLE_SCALE_PADDING", strconv.Itoa(padding))
			started := time.Now()
			p := &contractProcess{t: t, home: folderInventoryTemp(t), mode: "three-item"}
			var e error
			p.exe, e = os.Executable()
			if e != nil {
				t.Fatal(e)
			}
			raw := p.call("start", nil, nil)
			var refused struct {
				ScalePretaskRefusal bool
				Code                int
				Diagnostic          string
			}
			if json.Unmarshal(raw, &refused) != nil {
				t.Fatal("start response")
			}
			if refused.ScalePretaskRefusal {
				if padding == 0 || refused.Code == 0 {
					t.Fatal("baseline failed or invalid refusal")
				}
				heads, _ := filepath.Glob(filepath.Join(p.home, "Library", "Application Support", "lsp-trace", "census", "durable", "inventory-run-*.json"))
				if len(heads) != 0 {
					t.Fatal("pretask refusal issued a head")
				}
				t.Logf("SCALE_TERMINAL padding=%d disposition=PRETASK_REFUSAL elapsed=%s diagnostic=%s prep=NOT_PERSISTED input=NOT_ISSUED", padding, time.Since(started), refused.Diagnostic)
				return
			}
			var task ai.DurableTask
			if json.Unmarshal(raw, &task) != nil || task.Status != "PENDING" {
				t.Fatal("initial task")
			}
			t.Logf("SCALE_PREPARATION padding=%d elapsed=%s used=%d work=%d packets=%d", padding, time.Since(started), task.Used, task.Work, task.Packets)
			denied := runCensusLargeRequiredInputDeliveryForProcess(t, p, task, true)
			if padding == 0 && denied {
				t.Fatal("baseline failed")
			}
			disposition := "PUBLISHED_VERIFIED"
			if denied {
				disposition = "STAGED_REFUSAL"
			}
			t.Logf("SCALE_TERMINAL padding=%d disposition=%s elapsed=%s", padding, disposition, time.Since(started))
		})
	}
}
