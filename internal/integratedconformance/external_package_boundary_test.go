package integratedconformance

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	assertCoreNoImports    = "ASSERT_EXTERNAL_BOUNDARY_CORE_GO_NO_EMBER_GLINT_IMPORTS"
	assertCoreNoMatchers   = "ASSERT_EXTERNAL_BOUNDARY_CORE_GO_NO_SEMANTIC_MATCHERS"
	assertProviderPackage  = "ASSERT_EXTERNAL_BOUNDARY_PROVIDER_PACKAGE_INSTALLABLE_VERSIONED"
	assertArchivesExclude  = "ASSERT_EXTERNAL_BOUNDARY_CORE_ARCHIVES_EXCLUDE_PROVIDER"
)

func TestExternalPackageBoundary(t *testing.T) {
	root := repositoryRoot(t)

	t.Run(assertCoreNoImports, func(t *testing.T) {
		assertNoCoreGoMatch(t, root, assertCoreNoImports, func(path, line string) bool {
			return strings.Contains(line, `"lsp-trace/internal/emberglintprovider"`) ||
				strings.Contains(line, `"lsp-trace/providers/ember-glint`)
		})
	})

	t.Run(assertCoreNoMatchers, func(t *testing.T) {
		assertNoCoreGoMatch(t, root, assertCoreNoMatchers, func(path, line string) bool {
			lower := strings.ToLower(line)
			return strings.Contains(lower, "emberglintprovider") ||
				strings.Contains(lower, "ember-glint-provider") ||
				strings.Contains(lower, "noqualifiedanalyzer")
		})
	})

	t.Run(assertProviderPackage, func(t *testing.T) {
		providerRoot := filepath.Join(root, "providers", "ember-glint")
		for _, name := range []string{"README.md", "package.json", "package-lock.json"} {
			if info, err := os.Stat(filepath.Join(providerRoot, name)); err != nil || info.IsDir() {
				t.Fatalf("%s: missing package boundary file %s", assertProviderPackage, name)
			}
		}
		body, err := os.ReadFile(filepath.Join(providerRoot, "package.json"))
		if err != nil {
			t.Fatalf("%s: %v", assertProviderPackage, err)
		}
		text := string(body)
		for _, field := range []string{`"name"`, `"version"`, `"files"`} {
			if !strings.Contains(text, field) {
				t.Fatalf("%s: package metadata lacks %s", assertProviderPackage, field)
			}
		}
	})

	t.Run(assertArchivesExclude, func(t *testing.T) {
		body, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
		if err != nil {
			t.Fatalf("%s: %v", assertArchivesExclude, err)
		}
		lower := strings.ToLower(string(body))
		if strings.Contains(lower, "ember") || strings.Contains(lower, "glint") || strings.Contains(lower, "providers/") {
			t.Fatalf("%s: core GoReleaser configuration references provider content", assertArchivesExclude)
		}
	})
}

func assertNoCoreGoMatch(t *testing.T, root, assertion string, matches func(path, line string) bool) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "providers" || name == "provider" || name == "qualification" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "external_package_boundary_test.go") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for lineNumber := 1; scanner.Scan(); lineNumber++ {
			if matches(path, scanner.Text()) {
				relative, _ := filepath.Rel(root, path)
				t.Errorf("%s: forbidden core Go ownership at %s:%d", assertion, filepath.ToSlash(relative), lineNumber)
			}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("%s: scan core Go: %v", assertion, err)
	}
}
