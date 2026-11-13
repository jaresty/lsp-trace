# Historical preparation03 source guard (not a current contract)

Disposition approved for phase23: archive the original four literal token fingerprints and original algorithm verbatim below. The old live-source equality test is superseded by separate default current structural guards. Accepted phase09 and phase18 non-diagnostic semantic repairs must not be erased or pinned away. This record is not a claim that the old live test passed on current sources.

Revision boundary: preparation03 source record in runtime-trace-716d-03/source-index.json (original source_test.go SHA-256 2b9cb5882406b0f54911b9e82339918385582f0756f28a2a41362e4cd4cfebc0), retained as supplied at current workspace HEAD c9720f9d95de93a127ea55dc92e71bc0284d196d before phase23 edits. Preserve this source verbatim; it includes unsafe historical erasure and weak selector guards for documentary fidelity, NOT for reuse as current validators.

## Reproduction qualification

Phase23 verified all four exact preimages against runtime-trace-716d-03/preimages.json and reproduced all four token fingerprints directly on those original preimages (PID 13137; current-diagnostic-guards-23/historical-preimages.log). This does NOT reproduce the old diagnostic-erased postimage equality test, nor prove noninterference. Full private source fixtures are deliberately not copied into the repository. Historical postimage erasure reproduction remains NOT RUN in phase23; external preimage availability is not a default test dependency. Without that retained evidence bundle, historical reproduction is UNAVAILABLE from this repository alone. No git/network/absolute-path test or skip substitutes for current coverage.

The scanner ignores positions/comments (mode 0), writes token.String(), NUL, literal, NUL for each token before EOF, then SHA-256 hex. These are not file SHA-256 values. The original erasure removes diagnostic imports/statements and the reference input context import by whole-line spans in reverse source order.

## Original source_test.go

```go
package inventorytracediag

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Token fingerprints ignore positions/comments, not operation tokens. These
// guard the supplied preimage revision; they are structural, not runtime proof.
var preimageTokens = map[string]string{
	"internal/inventorysuccessor/private_mixed_input_operation_v2.go":   "429c9ec8ef43d8e2bc4990a3a615db8a5b484ebbc94e367725d5938d4aadbcce",
	"internal/inventorysuccessor/private_managed_input_operation_v2.go": "8c25824ec2ff1206c09fe45ce8971423975a0311add1b7a51cc0bdea0d1add4b",
	"sessionruntime/private_provider_window.go":                         "21c470ced483ff14db8e2911a5536245d042e506278a1468d12812f7163317ab",
	"internal/referencegrouping/input.go":                               "7e2abb5395ef86397d55dbe2b6e39d10f6a8be737899249b53433b84bca5f47e",
}

func tokenHash(t *testing.T, src []byte) string {
	t.Helper()
	var s scanner.Scanner
	set := token.NewFileSet()
	s.Init(set.AddFile("source", -1, len(src)), src, func(_ token.Position, msg string) { t.Fatal(msg) }, 0)
	var b bytes.Buffer
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		b.WriteString(tok.String())
		b.WriteByte(0)
		b.WriteString(lit)
		b.WriteByte(0)
	}
	h := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(h[:])
}
func diagExpr(n ast.Node) bool {
	yes := false
	ast.Inspect(n, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := s.X.(*ast.Ident); ok && id.Name == "inventorytracediag" {
				yes = true
			}
		}
		return true
	})
	return yes
}
func directDiagnosticCall(call *ast.CallExpr) bool {
	s, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := s.X.(*ast.Ident); ok && id.Name == "inventorytracediag" {
		return true
	}
	if s.Sel.Name != "End" {
		return false
	}
	inner, ok := s.X.(*ast.CallExpr)
	return ok && selector(inner.Fun, "inventorytracediag", "Begin")
}
func diagStatement(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		return ok && directDiagnosticCall(call)
	case *ast.DeferStmt:
		return directDiagnosticCall(s.Call)
	case *ast.IfStmt:
		if diagExpr(s.Cond) {
			return true
		}
		if s.Init != nil || s.Else != nil || len(s.Body.List) == 0 {
			return false
		}
		for _, child := range s.Body.List {
			if !diagStatement(child) {
				return false
			}
		}
		return true
	}
	return false
}
func erased(t *testing.T, path string, src []byte) []byte {
	t.Helper()
	set := token.NewFileSet()
	f, e := parser.ParseFile(set, path, src, 0)
	if e != nil {
		t.Fatal(e)
	}
	type span struct{ lo, hi int }
	var spans []span
	remove := func(n ast.Node) {
		lo := set.Position(n.Pos()).Offset
		hi := set.Position(n.End()).Offset
		for lo > 0 && src[lo-1] != '\n' {
			lo--
		}
		for hi < len(src) && src[hi] != '\n' {
			hi++
		}
		if hi < len(src) {
			hi++
		}
		spans = append(spans, span{lo, hi})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if i, ok := n.(*ast.ImportSpec); ok && (i.Path.Value == `"lsp-trace/internal/inventorytracediag"` || path == "internal/referencegrouping/input.go" && i.Path.Value == `"context"`) {
			remove(i)
			return false
		}
		if s, ok := n.(ast.Stmt); ok && diagStatement(s) {
			remove(s)
			return false
		}
		return true
	})
	sort.Slice(spans, func(i, j int) bool { return spans[i].lo > spans[j].lo })
	out := append([]byte(nil), src...)
	for _, p := range spans {
		out = append(out[:p.lo], out[p.hi:]...)
	}
	return out
}
func TestTraceSourceGuards(t *testing.T) {
	disabled(t)
	required := map[string][]string{
		"internal/inventorysuccessor/private_mixed_input_operation_v2.go":   {"Execute", "DeferredFinish", "DeferredFinishDone", "LeaseCallback", "CaptureRefused", "NativeCallsRefused", "IssueRefused", "LedgerUnavailable", "LedgerEmpty", "Confirmed"},
		"internal/inventorysuccessor/private_managed_input_operation_v2.go": {"Issue", "OwnedDeadlineRefused", "OwnedIdentityRefused", "OwnedTargetRefused", "BindOwnedRefused", "QueryBytesRefused", "Build", "BuildRefused", "TypedCallsRefused", "TypedMethodsRefused", "ProjectionRefused", "BuildInputRefused", "ParseAdmitRefused"},
		"sessionruntime/private_provider_window.go":                         {"ReadinessRegion", "ReadinessRefused", "CapabilityUnsupported"},
		"internal/referencegrouping/input.go":                               {"NativeProofWithoutCalls"},
	}
	for path, want := range preimageTokens {
		src, e := os.ReadFile(filepath.Join("../..", path))
		if e != nil {
			t.Fatal(e)
		}
		if got := tokenHash(t, erased(t, path, src)); got != want {
			t.Fatalf("ASSERT_TRACE_NATIVE_TOKEN_ERASURE %s got=%s", path, got)
		}
		set := token.NewFileSet()
		f, e := parser.ParseFile(set, path, src, 0)
		if e != nil {
			t.Fatal(e)
		}
		found := map[string]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok && id.Name == "inventorytracediag" {
					found[s.Sel.Name] = true
				}
			}
			if c, ok := n.(*ast.CallExpr); ok {
				if s, ok := c.Fun.(*ast.SelectorExpr); ok {
					if id, ok := s.X.(*ast.Ident); ok && id.Name == "trace" && (s.Sel.Name == "Start" || s.Sel.Name == "Stop" || s.Sel.Name == "NewTask") {
						t.Fatal("ASSERT_TRACE_NO_OWNERSHIP")
					}
				}
			}
			return true
		})
		for _, name := range required[path] {
			if !found[name] {
				t.Fatalf("ASSERT_TRACE_SITE_PRESENCE %s %s", path, name)
			}
		}
	}
	src, e := os.ReadFile("trace.go")
	if e != nil {
		t.Fatal(e)
	}
	f, e := parser.ParseFile(token.NewFileSet(), "trace.go", src, 0)
	if e != nil {
		t.Fatal(e)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if s, ok := c.Fun.(*ast.SelectorExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok && id.Name == "trace" && (s.Sel.Name == "Start" || s.Sel.Name == "Stop" || s.Sel.Name == "NewTask") {
					t.Fatal("ASSERT_TRACE_NO_OWNERSHIP")
				}
			}
		}
		return true
	})
	t.Log("ASSERT_TRACE_NATIVE_TOKEN_ERASURE PASS (structural)")
	t.Log("ASSERT_TRACE_SITE_PRESENCE PASS (structural)")
	t.Log("ASSERT_TRACE_NO_OWNERSHIP PASS (structural)")
}
```
