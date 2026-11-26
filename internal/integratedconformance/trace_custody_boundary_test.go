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

func authorityCallViolations(root string) ([]string, error) {
	fset := token.NewFileSet()
	forbidden := map[string]bool{"MintSeedAuthority": true, "PrepareSource": true, "SealPreparedSource": true}
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// Evidence copies and agent configuration are not production source, at any depth.
			if entry.Name() == ".git" || entry.Name() == ".pi" {
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
			// Closed PRIVATE V2 producer only: the origin is minted from actual
			// guarded discovery; no ordinary/public acquisition admission changes.
			if rel == "internal/censuslineagepilot/scope_producer_private_v2.go" && selector.Sel.Name == "MintSeedAuthority" {
				for _, decl := range file.Decls {
					fn, ok := decl.(*ast.FuncDecl)
					if ok && fn.Name.Name == "discoverScopeOutcomePrivateV2" && fn.Recv == nil && fn.Body != nil && call.Pos() >= fn.Body.Pos() && call.End() <= fn.Body.End() {
						return true
					}
				}
			}
			violations = append(violations, rel+" calls "+selector.Sel.Name)
			return true
		})
		return nil
	})
	return violations, err
}

func TestAuthorityMintCallsAreOwnedByPrivateOrchestrationRoutes(t *testing.T) {
	violations, err := authorityCallViolations(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Errorf("ASSERT_AUTHORITY_MINT_CALLSITE_PRIVATE_OWNER_ONLY: %s", violation)
	}
}

// Run the new owner scan only on an owned source copy, without walking any
// retained/evidence roots in the working repository.
func TestAuthorityScanSealedCensusV2OwnerOnly(t *testing.T) {
	root := t.TempDir()
	rel := "internal/censuslineagepilot/scope_producer_private_v2.go"
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(target, raw, 0600); err != nil {
		t.Fatal(err)
	}
	violations, err := authorityCallViolations(root)
	if err != nil || len(violations) != 0 {
		t.Fatalf("ASSERT_V2_CLOSED_MINT_OWNER: %v %v", violations, err)
	}
	// The old wrapper, a foreign function, and a same-named method must not
	// acquire the narrow exception just because they live in the producer file.
	for _, owner := range []string{"func DiscoverScopePrivateV2(", "func forbiddenOwnerCopy(", "func (x fakeOwner) discoverScopeOutcomePrivateV2("} {
		changed := strings.Replace(string(raw), "func discoverScopeOutcomePrivateV2(", owner, 1)
		if err = os.WriteFile(target, []byte(changed), 0600); err != nil {
			t.Fatal(err)
		}
		violations, err = authorityCallViolations(root)
		if err != nil || len(violations) != 1 || violations[0] != rel+" calls MintSeedAuthority" {
			t.Fatalf("ASSERT_V2_EXACT_MINT_OWNER: %s %v %v", owner, violations, err)
		}
	}
	raw = append(raw, []byte("\nfunc forbiddenOwnerCopy() { acquisitionauthority.MintSeedAuthority() }\n")...)
	if err = os.WriteFile(target, raw, 0600); err != nil {
		t.Fatal(err)
	}
	violations, err = authorityCallViolations(root)
	if err != nil || len(violations) != 1 || violations[0] != rel+" calls MintSeedAuthority" {
		t.Fatalf("ASSERT_V2_NO_FILE_WIDE_MINT_EXCEPTION: %v %v", violations, err)
	}
}

func TestAuthorityScanExcludesEvidenceButRejectsProductionCalls(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"src/forbidden.go", "src/.pi/evidence/copied.go", "internal/censuslineagepilot/scope_producer_private_v2.go"} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package fixture\nfunc forbidden() { acquisitionauthority.MintSeedAuthority() }\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	violations, err := authorityCallViolations(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 2 || violations[0] != "internal/censuslineagepilot/scope_producer_private_v2.go calls MintSeedAuthority" || violations[1] != "src/forbidden.go calls MintSeedAuthority" {
		t.Fatalf("ASSERT_AUTHORITY_SOURCE_VS_EVIDENCE: violations=%v", violations)
	}
}
