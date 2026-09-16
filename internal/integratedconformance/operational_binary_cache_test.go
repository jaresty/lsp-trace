package integratedconformance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type productionBinaryBuild func(repository, output, packagePath string) (string, error)

type productionBinaryResult struct {
	once sync.Once
	path string
	err  error
}

type productionBinaryCache struct {
	dir   string
	build productionBinaryBuild
	mu    sync.Mutex
	bins  map[string]*productionBinaryResult
}

var packageProductionBinaries *productionBinaryCache

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lsp-trace-integratedconformance-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	packageProductionBinaries = newProductionBinaryCache(dir, buildProductionBinary)
	code := m.Run()
	if err := os.RemoveAll(dir); err != nil && code == 0 {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}

func newProductionBinaryCache(dir string, build productionBinaryBuild) *productionBinaryCache {
	return &productionBinaryCache{dir: dir, build: build, bins: make(map[string]*productionBinaryResult)}
}

func (c *productionBinaryCache) binary(repository, packagePath string) (string, error) {
	key := repository + "\x00" + packagePath
	c.mu.Lock()
	result := c.bins[key]
	if result == nil {
		result = &productionBinaryResult{}
		c.bins[key] = result
	}
	c.mu.Unlock()

	result.once.Do(func() {
		owner, err := os.MkdirTemp(c.dir, "binary-")
		if err != nil {
			result.err = err
			return
		}
		output := filepath.Join(owner, "binary")
		result.path, result.err = c.build(repository, output, packagePath)
		if result.err == nil {
			result.err = os.Chmod(result.path, 0o555)
		}
	})
	return result.path, result.err
}

func cachedProductionBinary(t *testing.T, repository, packagePath string) string {
	t.Helper()
	path, err := packageProductionBinaries.binary(repository, packagePath)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOperationalHarnessBuildsEachProductionBinaryOnce(t *testing.T) {
	const assertion = "ASSERT_OPERATIONAL_HARNESS_BUILDS_EACH_PRODUCTION_BINARY_ONCE"
	t.Log("ASSERTION: " + assertion)

	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "builds.log")
	goPath := filepath.Join(fakeBin, "go")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$4" >> "$BUILD_LOG"
printf '#!/bin/sh\nexit 0\n' > "$3"
chmod +x "$3"
`
	if err := os.WriteFile(goPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BUILD_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	cache := newProductionBinaryCache(t.TempDir(), buildProductionBinary)
	first := newOperationalHarnessWithCache(t, cache)
	second := newOperationalHarnessWithCache(t, cache)

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, packagePath := range strings.Fields(string(raw)) {
		counts[packagePath]++
	}
	if counts["./cmd/lsp-trace"] != 1 || counts["./cmd/lsp-trace-mcp"] != 1 {
		t.Fatalf("%s: build_counts=%v", assertion, counts)
	}
	if first.cli != second.cli || first.mcp != second.mcp {
		t.Fatalf("%s: paths differ first=(%q,%q) second=(%q,%q)", assertion, first.cli, first.mcp, second.cli, second.mcp)
	}
	if first.cli == first.mcp {
		t.Fatalf("%s: CLI and MCP paths alias at %q", assertion, first.cli)
	}
	t.Logf("PASS %s build_counts=%v cli=%q mcp=%q", assertion, counts, first.cli, first.mcp)
}

func TestProductionBinaryCacheSeparatesEqualBasenames(t *testing.T) {
	const assertion = "ASSERT_PRODUCTION_BINARY_CACHE_SEPARATES_EQUAL_BASENAMES"
	t.Log("ASSERTION: " + assertion)

	var mu sync.Mutex
	calls := make(map[string]int)
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	cache := newProductionBinaryCache(t.TempDir(), func(repository, output, packagePath string) (string, error) {
		identity := repository + "\x00" + packagePath
		mu.Lock()
		calls[identity]++
		mu.Unlock()
		ready <- struct{}{}
		<-release
		if err := os.WriteFile(output, []byte(identity), 0o700); err != nil {
			return "", err
		}
		return output, nil
	})

	type outcome struct {
		path string
		err  error
	}
	identities := [][2]string{{"repository-a", "./first/tool"}, {"repository-b", "./second/tool"}}
	outcomes := make(chan outcome, len(identities))
	for _, identity := range identities {
		go func(repository, packagePath string) {
			path, err := cache.binary(repository, packagePath)
			outcomes <- outcome{path: path, err: err}
		}(identity[0], identity[1])
	}
	for range identities {
		<-ready
	}
	close(release)

	results := make([]outcome, 0, len(identities))
	for range identities {
		result := <-outcomes
		if result.err != nil {
			t.Fatalf("%s: binary: %v", assertion, result.err)
		}
		results = append(results, result)
	}
	if results[0].path == results[1].path {
		t.Fatalf("%s: distinct identities alias at %q", assertion, results[0].path)
	}
	contents := make(map[string]string)
	for _, result := range results {
		raw, err := os.ReadFile(result.path)
		if err != nil {
			t.Fatalf("%s: read %q: %v", assertion, result.path, err)
		}
		contents[string(raw)] = result.path
	}
	for _, identity := range identities {
		key := identity[0] + "\x00" + identity[1]
		if calls[key] != 1 {
			t.Fatalf("%s: identity %q built %d times", assertion, key, calls[key])
		}
		if contents[key] == "" {
			t.Fatalf("%s: identity %q content missing: %v", assertion, key, contents)
		}
	}
	t.Logf("PASS %s calls=%v paths=(%q,%q)", assertion, calls, results[0].path, results[1].path)
}

func TestProductionBinaryCacheRetainsFailure(t *testing.T) {
	const assertion = "ASSERT_PRODUCTION_BINARY_CACHE_RETAINS_FAILURE"
	calls := 0
	cache := newProductionBinaryCache(t.TempDir(), func(_, _, _ string) (string, error) {
		calls++
		return "", fmt.Errorf("build failed with stderr")
	})

	_, first := cache.binary("repository", "./cmd/lsp-trace")
	_, second := cache.binary("repository", "./cmd/lsp-trace")
	if calls != 1 || first == nil || second == nil || first.Error() != second.Error() {
		t.Fatalf("%s: calls=%d first=%v second=%v", assertion, calls, first, second)
	}
	t.Logf("PASS %s calls=%d error=%q", assertion, calls, first.Error())
}
