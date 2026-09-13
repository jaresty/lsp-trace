package integratedconformance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalModuleCannotReachTraceCustodyAuthority(t *testing.T) {
	root := repositoryRoot(t)
	module := t.TempDir()
	goMod := "module example.invalid/external\n\ngo 1.24\n\nrequire lsp-trace v0.0.0\nreplace lsp-trace => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatal(err)
	}
	build := func(source string) error {
		if err := os.WriteFile(filepath.Join(module, "main.go"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "build", "-mod=mod", ".")
		cmd.Dir = module
		cmd.Env = append(os.Environ(), "GOWORK=off")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return &buildError{err: err, output: string(output)}
		}
		return nil
	}
	ordinary := `package main
import (
  "lsp-trace/acquisitionops"
  "lsp-trace/sessionruntime"
)
var _ acquisitionops.Input
var _ acquisitionops.Manifest
var _ = acquisitionops.NewExecutor
var _ sessionruntime.DocumentResult
func main() {}
`
	if err := build(ordinary); err != nil {
		t.Fatalf("ASSERT_EXTERNAL_ORDINARY_ACQUISITION_COMPILES: %v", err)
	}
	for name, source := range map[string]string{
		"explicit-admission":  `package main; import "lsp-trace/acquisitionops"; var _ = acquisitionops.NewExplicitTraceAdmission; func main(){}`,
		"explicit-execute":    `package main; import "lsp-trace/acquisitionops"; var _ = (*acquisitionops.Executor).ExecuteExplicitTrace; func main(){}`,
		"prepared-mint":       `package main; import "lsp-trace/sessionruntime"; var _ = sessionruntime.PrepareDocumentForOperation; func main(){}`,
		"prepared-capability": `package main; import "lsp-trace/sessionruntime"; var _ sessionruntime.PreparedDocumentCapability; func main(){}`,
		"authority-internal":  `package main; import _ "lsp-trace/internal/acquisitionauthority"; func main(){}`,
		"trace-runtime":       `package main; import "lsp-trace/traceops"; var _ traceops.Runtime; func main(){}`,
		"trace-constructor":   `package main; import "lsp-trace/traceops"; var _ = traceops.NewExecutor; func main(){}`,
		"trace-executor":      `package main; import "lsp-trace/traceops"; var _ *traceops.Executor; func main(){}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := build(source); err == nil {
				t.Fatal("ASSERT_EXTERNAL_PRIVILEGED_TRACE_CUSTODY_DOES_NOT_COMPILE")
			}
		})
	}
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.output }

func TestAuthorityMintCallsAreOwnedByPrivateOrchestrationRoutes(t *testing.T) {
	root := repositoryRoot(t)
	fset := token.NewFileSet()
	forbidden := map[string]bool{"MintSeedAuthority": true, "PrepareSource": true, "SealPreparedSource": true}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		if strings.HasPrefix(filepath.ToSlash(relative), "internal/acquisitionorchestration/") || strings.HasPrefix(filepath.ToSlash(relative), "internal/acquisitionauthority/") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !forbidden[selector.Sel.Name] {
				return true
			}
			rel := filepath.ToSlash(relative)
			if rel == "cmd/lsp-trace/census_batch_acquirer.go" && selector.Sel.Name == "MintSeedAuthority" {
				return true
			}
			t.Errorf("ASSERT_AUTHORITY_MINT_CALLSITE_PRIVATE_OWNER_ONLY: %s calls %s", rel, selector.Sel.Name)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
