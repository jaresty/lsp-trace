//go:build darwin || linux

package censuslineagepilot_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// One test-only boundary for both entrypoints, immediately after strict parsing.
// The continuation contains all root/runtime/protocol/claim/semantic operations.
func scopedRehearsalPurposeBeforeStartupPrivate(cfg pilot.ScopedRehearsalConfigPrivate, partial bool, startup func()) error {
	allowed := (!partial && (cfg.Schema == "lsp-trace.scoped-rehearsal.private.v1" || cfg.Schema == "lsp-trace.scoped-rehearsal.private.v2")) || (partial && cfg.Schema == partialConfigSchemaPrivate)
	if !allowed {
		return errors.New("private rehearsal purpose/configuration mismatch before startup")
	}
	startup()
	return nil
}

func TestScopedRehearsalPurposeBeforeStartupPrivate(t *testing.T) {
	for _, partial := range []bool{false, true} {
		for _, schema := range []string{"lsp-trace.scoped-rehearsal.private.v1", "lsp-trace.scoped-rehearsal.private.v2", partialConfigSchemaPrivate, "", "lsp-trace.scoped-rehearsal.private.v4"} {
			name := "full/"
			if partial {
				name = "partial/"
			}
			t.Run(name+schema, func(t *testing.T) {
				roots, starter, protocol, claim, semantic := 0, 0, 0, 0, 0
				err := scopedRehearsalPurposeBeforeStartupPrivate(pilot.ScopedRehearsalConfigPrivate{Schema: schema}, partial, func() { roots++; starter++; protocol++; claim++; semantic++ })
				allowed := (!partial && (schema == "lsp-trace.scoped-rehearsal.private.v1" || schema == "lsp-trace.scoped-rehearsal.private.v2")) || (partial && schema == partialConfigSchemaPrivate)
				if !allowed {
					if err == nil || roots != 0 || starter != 0 || protocol != 0 || claim != 0 || semantic != 0 {
						t.Fatalf("ASSERT_WRONG_PURPOSE_ZERO_STARTUP roots=%d starter=%d protocol=%d claim=%d semantic=%d err=%v", roots, starter, protocol, claim, semantic, err)
					}
				} else if err != nil || roots != 1 || starter != 1 || protocol != 1 || claim != 1 || semantic != 1 {
					t.Fatal("ASSERT_VALID_PURPOSE_STARTUP_ONCE", err)
				}
			})
		}
	}
}

// Behavioral callback tests alone could pass while an entrypoint bypasses them.
// Bind both real entrypoint bodies to this same gate and require all startup and
// execution calls to remain lexically inside its continuation, after parse/error.
func TestScopedRehearsalPurposeEntrypointWiringPrivate(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location unavailable")
	}
	for _, entry := range []struct{ file, name, purpose string }{
		{"scoped_live_constituent_rehearsal_private_test.go", "TestScopedBatchLiveConstituentRehearsalPrivate", "false"},
		{"scoped_partial_live_rehearsal_private_test.go", "TestScopedPartialLiveInspectionRehearsalPrivate", "true"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(here), entry.file), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			var fn *ast.FuncDecl
			for _, decl := range file.Decls {
				if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == entry.name {
					fn = f
				}
			}
			if fn == nil {
				t.Fatal("entrypoint absent")
			}
			parseIndex, gateIndex := -1, -1
			var continuation *ast.FuncLit
			for i, stmt := range fn.Body.List {
				if assignment, ok := stmt.(*ast.AssignStmt); ok {
					for _, rhs := range assignment.Rhs {
						if call, ok := rhs.(*ast.CallExpr); ok {
							if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ParseScopedRehearsalConfigPrivate" {
								parseIndex = i
							}
						}
					}
				}
				if branch, ok := stmt.(*ast.IfStmt); ok {
					if init, ok := branch.Init.(*ast.AssignStmt); ok && len(init.Rhs) == 1 {
						if call, ok := init.Rhs[0].(*ast.CallExpr); ok {
							if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "scopedRehearsalPurposeBeforeStartupPrivate" {
								gateIndex = i
								if len(call.Args) != 3 {
									t.Fatal("gate argument shape")
								}
								cfg, ok := call.Args[0].(*ast.Ident)
								if !ok || cfg.Name != "cfg" {
									t.Fatal("gate must validate parsed config")
								}
								purpose, ok := call.Args[1].(*ast.Ident)
								if !ok || purpose.Name != entry.purpose {
									t.Fatal("wrong entrypoint purpose")
								}
								continuation, _ = call.Args[2].(*ast.FuncLit)
							}
						}
					}
				}
			}
			if parseIndex < 0 || gateIndex != parseIndex+2 || continuation == nil {
				t.Fatal("ASSERT_PURPOSE_GATE_IMMEDIATELY_AFTER_PARSE_ERROR_CHECK")
			}
			guard, ok := fn.Body.List[parseIndex+1].(*ast.IfStmt)
			if !ok {
				t.Fatal("parse error guard absent")
			}
			condition, ok := guard.Cond.(*ast.BinaryExpr)
			if !ok || condition.Op != token.NEQ {
				t.Fatal("parse error condition")
			}
			left, lok := condition.X.(*ast.Ident)
			right, rok := condition.Y.(*ast.Ident)
			if !lok || !rok || left.Name != "err" || right.Name != "nil" {
				t.Fatal("parse errors must fail before purpose gate")
			}
			fatal := false
			ast.Inspect(guard.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Fatal" {
						fatal = true
					}
				}
				return true
			})
			if !fatal {
				t.Fatal("parse error guard must terminate")
			}
			seen := map[string]int{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch f := call.Fun.(type) {
				case *ast.Ident:
					name = f.Name
				case *ast.SelectorExpr:
					name = f.Sel.Name
				}
				switch name {
				case "OpenScopedRehearsalPrivate", "NewLocalDarwinSupervisor", "New", "Start", "BeginReadiness", "WaitReadiness", "executeScopedRehearsalPrivate", "executeScopedPartialRehearsalPrivate":
					if call.Pos() < continuation.Body.Pos() || call.End() > continuation.Body.End() {
						t.Fatalf("ASSERT_STARTUP_OUTSIDE_PURPOSE_GATE %s", name)
					}
					seen[name]++
				}
				return true
			})
			if seen["OpenScopedRehearsalPrivate"] != 1 || seen["Start"] != 1 || seen["NewLocalDarwinSupervisor"] != 1 || seen["executeScopedRehearsalPrivate"]+seen["executeScopedPartialRehearsalPrivate"] != 1 {
				t.Fatal("ASSERT_REAL_STARTUP_CONTINUATION", seen)
			}
		})
	}
}
