package inventorytracediag

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// These are source-validator witnesses, not executed native behavior.
func TestTraceErasureRefusesUnsafe(t *testing.T) {
	disabled(t)
	for _, body := range []string{
		`if inventorytracediag.Enabled() || mutate(){ inventorytracediag.Event(ctx,inventorytracediag.Execute) }`,
		`if inventorytracediag.Enabled(){inventorytracediag.Event(ctx,inventorytracediag.Execute)}else{native()}`,
		`if x:=native();inventorytracediag.Enabled(){inventorytracediag.Event(ctx,inventorytracediag.Execute)}`,
		`if inventorytracediag.Enabled(){ctx=context.Background()}`,
		`if inventorytracediag.Enabled(){return}`,
		`if inventorytracediag.Enabled(){go native()}`,
		`if inventorytracediag.Enabled(){ch <- x}`,
		`if inventorytracediag.Enabled(){for{native()}}`,
		`if inventorytracediag.Enabled(){inventorytracediag.Event(native(),inventorytracediag.Execute)}`,
	} {
		src := []byte("package p\nimport (\n\"lsp-trace/internal/inventorytracediag\"\n)\nfunc f(){\n" + body + "\n}\n")
		if _, err := eraseDiagnosticSource("candidate", src); err == nil {
			t.Errorf("CURRENT_ERASURE_UNSAFE_REMOVED body=%s", body)
		}
	}
}

func TestTraceErasureNativeBranchPreserved(t *testing.T) {
	disabled(t)
	src := []byte("package p\nimport (\n\"context\"\n\"lsp-trace/internal/inventorytracediag\"\n)\nfunc f(){\ndefer inventorytracediag.Begin(ctx,inventorytracediag.Execute).End()\nif failure != nil { inventorytracediag.Event(ctx,inventorytracediag.Execute); return failure }; native()\n}\n")
	out, e := eraseDiagnosticSource("candidate", src)
	if e != nil {
		t.Fatal(e)
	}
	f := parseCurrent(t, out)
	fn := f.Decls[1].(*ast.FuncDecl)
	if len(fn.Body.List) != 2 {
		t.Fatalf("CURRENT_ERASURE_NATIVE_BRANCH statements=%d", len(fn.Body.List))
	}
	branch, ok := fn.Body.List[0].(*ast.IfStmt)
	if !ok {
		t.Fatal("CURRENT_ERASURE_NATIVE_BRANCH lost condition")
	}
	// Replacing a call on a semicolon-separated line leaves an inert EmptyStmt.
	var native []ast.Stmt
	for _, st := range branch.Body.List {
		if _, empty := st.(*ast.EmptyStmt); !empty {
			native = append(native, st)
		}
	}
	if render(branch.Cond) != "failure != nil" || len(native) != 1 || render(native[0]) != "return failure" || render(fn.Body.List[1]) != "native()" {
		t.Fatal("CURRENT_ERASURE_NATIVE_BRANCH changed native work")
	}
}

func TestTraceOwnershipDiscriminators(t *testing.T) {
	disabled(t)
	for _, s := range []struct {
		name, src string
		want      bool
	}{
		{"direct", `package p;import "runtime/trace";func f(){trace.Start(nil)}`, false},
		{"alias", `package p;import tr "runtime/trace";func f(){tr.Start(nil)}`, false},
		{"capture", `package p;import "runtime/trace";var f=trace.Start`, false},
		{"aliascapture", `package p;import tr "runtime/trace";var f=tr.NewTask`, false},
		{"stopcapture", `package p;import tr "runtime/trace";var f=tr.Stop`, false},
		{"dot", `package p;import . "runtime/trace";func f(){Start(nil)}`, false},
		{"fake", `package p;import trace "example.invalid/unrelated";func f(){trace.Start(nil)}`, true},
		{"allowed", `package p;import tr "runtime/trace";func f(){tr.IsEnabled();tr.Log(nil,"","");tr.StartRegion(nil,"")}`, true},
	} {
		t.Run(s.name, func(t *testing.T) {
			if got := traceNoOwnership(parseCurrent(t, []byte(s.src))); got != s.want {
				t.Errorf("CURRENT_NO_OWNERSHIP %s got=%v want=%v", s.name, got, s.want)
			}
		})
	}
}

func TestTraceErasureImportResolution(t *testing.T) {
	disabled(t)
	for _, s := range []struct {
		name, imp, body string
		want            bool
	}{
		{"alias", `d "lsp-trace/internal/inventorytracediag"`, `if d.Enabled(){d.Event(ctx,d.Execute)}`, true},
		{"unrelated", `inventorytracediag "example.invalid/unrelated"`, `inventorytracediag.Event(ctx,inventorytracediag.Execute)`, false},
		{"dot", `. "lsp-trace/internal/inventorytracediag"`, `Event(ctx,Execute)`, false},
		{"capture", `d "lsp-trace/internal/inventorytracediag"`, `f:=d.Event;_ = f`, false},
	} {
		t.Run(s.name, func(t *testing.T) {
			src := []byte("package p\nimport (\n" + s.imp + "\n)\nfunc f(){\n" + s.body + "\n}\n")
			_, e := eraseDiagnosticSource("candidate", src)
			if (e == nil) != s.want {
				t.Errorf("CURRENT_ERASURE_IMPORT %s err=%v want=%v", s.name, e, s.want)
			}
		})
	}
}

func parseCurrent(t *testing.T, src []byte) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "candidate", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
