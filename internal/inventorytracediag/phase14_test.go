package inventorytracediag

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// These are source decision tests, not claims of observed runtime emissions.
// Event has no isolated observer: observing it would require forbidden trace activation.
var phase14Codes = []string{
	"ProjectionRefused", "CompositeBindingBoundRefused", "CompositeBindingJSONRefused",
	"NativePredecessorCountRefused", "NativePredecessorBytesRefused", "NativeAggregateBoundRefused",
}
var phase14Names = []string{
	"PROJECTION_REFUSED", "COMPOSITE_BINDING_BOUND_REFUSED", "COMPOSITE_BINDING_JSON_REFUSED",
	"NATIVE_PREDECESSOR_COUNT_REFUSED", "NATIVE_PREDECESSOR_BYTES_REFUSED", "NATIVE_AGGREGATE_BOUND_REFUSED",
}

func phase14Text(n ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), n); err != nil {
		panic(err)
	}
	return b.String()
}

// Match actual return-owning blocks in the production interval, not text occurrences
// elsewhere in the file. Keeping the combined aggregate guard intact is intentional.
func phase14Decisions(src []byte) (guards, codes []string, err error) {
	f, err := parser.ParseFile(token.NewFileSet(), "assembly.go", src, 0)
	if err != nil {
		return nil, nil, err
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "privateGroupingAssemble" {
			continue
		}
		var lo, hi token.Pos
		var loCount, hiCount int
		ast.Inspect(fn, func(n ast.Node) bool {
			e, ok := n.(*ast.ExprStmt)
			if !ok {
				return true
			}
			c, ok := e.X.(*ast.CallExpr)
			if !ok || !selector(c.Fun, "inventorytracediag", "Event") || len(c.Args) != 2 || render(c.Args[0]) != "context.Background()" {
				return true
			}
			if selector(c.Args[1], "inventorytracediag", "ProjectionAttempt") {
				lo = e.Pos()
				loCount++
			}
			if selector(c.Args[1], "inventorytracediag", "BuildInputAttempt") {
				hi = e.Pos()
				hiCount++
			}
			return true
		})
		if loCount != 1 || hiCount != 1 || lo >= hi {
			return nil, nil, fmt.Errorf("assembly brackets missing, duplicate or reversed")
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			b, ok := n.(*ast.IfStmt)
			if !ok || b.Pos() <= lo || b.End() >= hi {
				return true
			}
			for i, s := range b.Body.List {
				if _, ok := s.(*ast.ReturnStmt); !ok {
					continue
				}
				if b.Init != nil || b.Else != nil {
					guards = append(guards, "UNSUPPORTED_BRANCH")
					codes = append(codes, "UNSUPPORTED_BRANCH")
					continue
				}
				guards = append(guards, phase14Text(b.Cond))
				code := "MISSING"
				if i > 0 {
					if e, ok := b.Body.List[i-1].(*ast.ExprStmt); ok {
						if c, ok := e.X.(*ast.CallExpr); ok && selector(c.Fun, "inventorytracediag", "Event") && len(c.Args) == 2 && phase14Text(c.Args[0]) == "context.Background()" {
							if s, ok := c.Args[1].(*ast.SelectorExpr); ok {
								if id, ok := s.X.(*ast.Ident); ok && id.Name == "inventorytracediag" {
									code = s.Sel.Name
								}
							}
						}
					}
				}
				codes = append(codes, code)
			}
			return true
		})
	}
	return guards, codes, nil
}

func TestPhase14AssemblyExitCodes(t *testing.T) {
	disabled(t)
	src, err := os.ReadFile("../inventorysuccessor/private_managed_input_operation_v2.go")
	if err != nil {
		t.Fatal(err)
	}
	guards, codes, err := phase14Decisions(src)
	if err != nil {
		t.Fatal(err)
	}
	wantGuards := []string{
		"err != nil", "!ok", "err != nil", "len(out.Predecessors)+1 > 8",
		"uint64(len(p.Bytes)) > (1<<20)-total",
		"!ok || total > 1<<20 || bound > (1<<20)-total || 65536 > (1<<20)-total-bound",
	}
	if len(codes) != len(phase14Codes) {
		t.Fatalf("ASSEMBLY_EXIT_CARDINALITY got=%d want=%d", len(codes), len(phase14Codes))
	}
	for i, want := range phase14Codes {
		t.Run(want, func(t *testing.T) {
			if guards[i] != wantGuards[i] {
				t.Fatalf("ASSEMBLY_GUARD_%s got=%q want=%q", want, guards[i], wantGuards[i])
			}
			if codes[i] != want {
				t.Fatalf("ASSEMBLY_EXIT_%s got=%s want=%s", want, codes[i], want)
			}
			t.Logf("ASSEMBLY_EXIT_%s PASS", want)
		})
	}
	// Discriminator: plausible swapped static codes must be distinguished by the
	// same production AST reader even though both codes remain valid registry entries.
	for i := 1; i < len(phase14Codes); i++ {
		from := "inventorytracediag." + phase14Codes[i]
		to := "inventorytracediag." + phase14Codes[i-1]
		if !bytes.Contains(src, []byte(from)) {
			t.Fatalf("ASSEMBLY_SWAP_SETUP_%s missing", phase14Codes[i])
		}
		changed := bytes.Replace(src, []byte(from), []byte(to), 1)
		_, got, err := phase14Decisions(changed)
		if err != nil || len(got) != len(codes) || got[i] == phase14Codes[i] {
			t.Fatalf("ASSEMBLY_SWAP_DISCRIMINATOR_%s failed: %v", phase14Codes[i], err)
		}
		t.Logf("ASSEMBLY_SWAP_DISCRIMINATOR_%s REJECTED", phase14Codes[i])
	}
}

func TestPhase14RegistryDisabled(t *testing.T) {
	disabled(t)
	for _, want := range phase14Names {
		t.Run(want, func(t *testing.T) {
			index := -1
			for i, name := range codeNames {
				if name == want {
					if index >= 0 {
						t.Fatalf("ASSEMBLY_REGISTRY_%s duplicate", want)
					}
					index = i
				}
			}
			if index < 0 || !Valid(Code(index)) {
				t.Fatalf("ASSEMBLY_REGISTRY_%s missing", want)
			}
			if strings.Trim(want, "ABCDEFGHIJKLMNOPQRSTUVWXYZ_") != "" {
				t.Fatalf("ASSEMBLY_REGISTRY_%s nonstatic", want)
			}
			if n := testing.AllocsPerRun(100, func() { Event(nil, Code(index)); Begin(nil, Code(index)).End() }); n != 0 || Enabled() {
				t.Fatalf("ASSEMBLY_DISABLED_%s allocations=%v enabled=%v", want, n, Enabled())
			}
			t.Logf("ASSEMBLY_REGISTRY_%s PASS disabled-noop", want)
		})
	}
}
