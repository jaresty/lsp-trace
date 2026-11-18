package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCensusFolderParsePure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-created")
	for _, args := range [][]string{{"census"}, {"census", missing}, {"census", missing, "--machine", "--source", "src", "--max-nodes", "12"}, {"census", "--machine", missing}} {
		o, err := parseCensusCLIOptions(args)
		if err != nil {
			t.Errorf("ASSERT_FOLDER_PARSE: args=%v err=%v", args, err)
			continue
		}
		if !o.Catalog || o.CatalogConfigPath != "" {
			t.Errorf("ASSERT_FOLDER_NATIVE_MODE: %+v", o)
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_FOLDER_PARSE_PURE: %v", err)
	}
}

// This checks real filesystem normalization and runner transfer, not inventory delivery.
func TestCensusFolderRunNativeTransfer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "missing", "config"))
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.test/folder\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(workspace, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(workspace, alias); err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(workspace)
	deps := productionCensusRunnerDependencies()
	deps.newStarter = func() (sessionruntime.Starter, error) { return sessionruntime.ManagedStarter{}, nil }
	deps.lookPath = func(command string) (string, error) {
		if command != "gopls" {
			t.Fatalf("ASSERT_FOLDER_PROFILE: %s", command)
		}
		return "/bin/echo", nil
	}
	deps.newManager = func(sessionruntime.Config) (*sessionruntime.Manager, error) { return nil, nil }
	deps.runCore = func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		t.Fatal("ordinary acquisition selected")
		return censusPublicationOutcome{}
	}
	calls := 0
	roots := map[string]bool{}
	deps.runNativeCatalog = func(o censusCLIOptions, cfg censusCoreConfig, out, errout io.Writer) int {
		calls++
		if o.Workspace != canonical || !o.Catalog || o.CatalogConfigPath != "" || !reflect.DeepEqual(o.Sources, []string{"src"}) || o.MaxBatchTargets != 3 || o.MaxNodes != 12 || cfg.runner.start.LanguageID != "go" || *cfg.limits.MaxNodes != 12 {
			t.Fatalf("ASSERT_FOLDER_TRANSFER: %+v language=%s", o, cfg.runner.start.LanguageID)
		}
		root, err := publication.OpenRoot(o.PublicationRoot)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		if err = root.ValidatePrivate(); err != nil {
			t.Fatalf("ASSERT_FOLDER_PRIVATE: %v", err)
		}
		if roots[o.PublicationRoot] || strings.HasPrefix(o.PublicationRoot, canonical+string(os.PathSeparator)) {
			t.Fatalf("ASSERT_FOLDER_FRESH_OUTSIDE_SOURCE: %s", o.PublicationRoot)
		}
		roots[o.PublicationRoot] = true
		return writeCensusFailure(errout, censusStageCommitted, o.Machine, errNativeCatalogAggregateUnavailable)
	}
	for _, machine := range []bool{false, true} {
		var out, errout bytes.Buffer
		args := []string{alias, "--source", "src", "--batch-targets", "3", "--max-nodes", "12"}
		if machine {
			args = append(args, "--machine")
		}
		code := runCensusWithDependencies(args, &out, &errout, deps)
		if code != 1 || out.Len() != 0 {
			t.Fatalf("ASSERT_FOLDER_HANDOFF_FAILURE: code=%d out=%q err=%q", code, out.String(), errout.String())
		}
		if machine {
			var diagnostic censusCLIDiagnostic
			if err := json.Unmarshal(errout.Bytes(), &diagnostic); err != nil || diagnostic.Stage != censusStageCommitted {
				t.Fatalf("ASSERT_FOLDER_MACHINE_HANDOFF_FAILURE: %s %v", errout.Bytes(), err)
			}
		} else if !strings.Contains(errout.String(), "native census catalog handoff unavailable") {
			t.Fatalf("ASSERT_FOLDER_HANDOFF_FAILURE: err=%q", errout.String())
		}
	}
	if calls != 2 {
		t.Fatalf("ASSERT_FOLDER_NATIVE_ONCE: calls=%d", calls)
	}
}

func TestCensusFolderInvalidSourcesBeforeProvider(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{filepath.Join(workspace, "missing")}, {file}, {workspace}, {workspace, "--source", "missing"}, {workspace, "--server", "gopls", "--source", "file"}} {
		deps := censusRunnerDependencies{abs: filepath.Abs, lookPath: func(string) (string, error) { t.Fatal("ASSERT_FOLDER_REFUSE_BEFORE_PROVIDER"); return "", nil }}
		var out, errout bytes.Buffer
		if code := runCensusWithDependencies(args, &out, &errout, deps); code != 1 || out.Len() != 0 {
			t.Fatalf("ASSERT_FOLDER_REFUSAL: %v %d", args, code)
		}
	}
}

func TestCensusFolderBareCurrentDirectory(t *testing.T) {
	workspace, store := t.TempDir(), t.TempDir()
	store, _ = filepath.EvalSymlinks(store)
	if e := os.Chmod(store, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(workspace, "go.work"), []byte("go 1.25\n"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Chdir(workspace)
	o, e := parseCensusCLIOptions([]string{"census", "--publication-root", store})
	if e != nil {
		t.Fatal(e)
	}
	got, e := resolveCensusFolderInvocation(o)
	canonical, _ := filepath.EvalSymlinks(workspace)
	if e != nil || got.Workspace != canonical || got.Server != "gopls" || got.MaxBatchTargets != 16 {
		t.Fatalf("ASSERT_FOLDER_CURRENT_DIRECTORY: %+v %v", got, e)
	}
}

func TestCensusFolderBounds(t *testing.T) {
	for _, args := range [][]string{{"--max-nodes", "0"}, {"--max-nodes", "10001"}, {"--batch-targets", "64"}, {"--down-depth", "-1"}, {"--timeout", "0s"}, {"--request-timeout", "2m", "--timeout", "1m"}} {
		if _, e := parseCensusCLIOptions(append([]string{"census", "folder"}, args...)); e == nil {
			t.Fatalf("ASSERT_FOLDER_BOUNDS: %v", args)
		}
	}
}

func TestCensusFolderResolutionSafety(t *testing.T) {
	workspace, store := t.TempDir(), t.TempDir()
	store, _ = filepath.EvalSymlinks(store)
	workspace, _ = filepath.EvalSymlinks(workspace)
	if err := os.Chmod(store, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	parse := func(extra ...string) censusCLIOptions {
		t.Helper()
		o, e := parseCensusCLIOptions(append([]string{"census", workspace}, extra...))
		if e != nil {
			t.Fatal(e)
		}
		return o
	}
	t.Run("explicit-server-without-evidence", func(t *testing.T) {
		empty := t.TempDir()
		o := parse("--server", "custom", "--publication-root", store)
		o.Workspace = empty
		got, e := resolveCensusFolderInvocation(o)
		if e != nil || got.Server != "custom" {
			t.Fatalf("ASSERT_FOLDER_EXPLICIT_SERVER: %+v %v", got, e)
		}
	})
	t.Run("explicit-profile-without-evidence", func(t *testing.T) {
		o := parse("--profile", "selected", "--config", "profiles.toml", "--publication-root", store)
		o.Workspace = t.TempDir()
		got, e := resolveCensusFolderInvocation(o)
		if e != nil || got.Profile != "selected" || got.ConfigPath != "profiles.toml" || got.Server != "" {
			t.Fatalf("ASSERT_FOLDER_EXPLICIT_PROFILE: %+v %v", got, e)
		}
	})
	t.Run("ambiguous", func(t *testing.T) {
		marker := filepath.Join(workspace, "package.json")
		if e := os.WriteFile(marker, []byte("{}"), 0600); e != nil {
			t.Fatal(e)
		}
		defer os.Remove(marker)
		if _, e := resolveCensusFolderInvocation(parse("--publication-root", store)); e == nil || !strings.Contains(e.Error(), "ambiguous") {
			t.Fatalf("ASSERT_FOLDER_AMBIGUOUS: %v", e)
		}
	})
	t.Run("output-in-source", func(t *testing.T) {
		if _, e := resolveCensusFolderInvocation(parse("--publication-root", workspace)); e == nil {
			t.Fatal("ASSERT_FOLDER_OUTPUT_SOURCE")
		}
	})
	t.Run("output-symlink", func(t *testing.T) {
		alias := filepath.Join(t.TempDir(), "alias")
		if e := os.Symlink(store, alias); e != nil {
			t.Fatal(e)
		}
		if _, e := resolveCensusFolderInvocation(parse("--publication-root", alias)); e == nil {
			t.Fatal("ASSERT_FOLDER_OUTPUT_SYMLINK")
		}
	})
	t.Run("output-not-private", func(t *testing.T) {
		public := t.TempDir()
		public, _ = filepath.EvalSymlinks(public)
		if e := os.Chmod(public, 0755); e != nil {
			t.Fatal(e)
		}
		if _, e := resolveCensusFolderInvocation(parse("--publication-root", public)); e == nil {
			t.Fatal("ASSERT_FOLDER_OUTPUT_PRIVACY")
		}
	})
	t.Run("source-escape", func(t *testing.T) {
		alias := filepath.Join(workspace, "escape")
		if e := os.Symlink(t.TempDir(), alias); e != nil {
			t.Fatal(e)
		}
		if _, e := resolveCensusFolderInvocation(parse("--source", "escape", "--publication-root", store)); e == nil {
			t.Fatal("ASSERT_FOLDER_SOURCE_ESCAPE")
		}
	})
	t.Run("fresh-no-replace", func(t *testing.T) {
		first, e := censusOwnedDirectory(store, "fixed-run", true)
		if e != nil {
			t.Fatal(e)
		}
		marker := filepath.Join(first, "retained")
		if e := os.WriteFile(marker, []byte("original"), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := censusOwnedDirectory(store, "fixed-run", true); e == nil {
			t.Fatal("ASSERT_FOLDER_NO_OVERWRITE")
		}
		raw, e := os.ReadFile(marker)
		if e != nil || string(raw) != "original" {
			t.Fatalf("ASSERT_FOLDER_RETAINED: %q %v", raw, e)
		}
	})
	t.Run("owned-namespace-symlink", func(t *testing.T) {
		if e := os.Symlink(store, filepath.Join(store, "alias")); e != nil {
			t.Fatal(e)
		}
		if _, e := censusOwnedDirectory(store, "alias", false); e == nil {
			t.Fatal("ASSERT_FOLDER_NAMESPACE_NOFOLLOW")
		}
	})
}

func TestCensusFolderHelpHonest(t *testing.T) {
	var out, errout bytes.Buffer
	if code := runCensusWithDependencies([]string{"--help"}, &out, &errout, censusRunnerDependencies{}); code != 0 {
		t.Fatal(code)
	}
	for _, text := range []string{"census [FOLDER]", "Legacy explicit acquisition", "semantic inventory handoff is currently unavailable", "Failed runs are retained"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("ASSERT_FOLDER_HELP: missing %q", text)
		}
	}
}

func TestCensusFolderModeConflicts(t *testing.T) {
	for _, flags := range [][]string{{"--workspace", "/work"}, {"--catalog"}, {"--catalog-config", "/host.json"}, {"--caller-host"}, {"--resume", "g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json"}, {"--enumeration-preflight", "--machine"}, {"--private-enumeration-projection", "--machine"}, {"--private-control-flow-diagnostic-path", "/private/log"}} {
		args := append([]string{"folder"}, flags...)
		var out, errout bytes.Buffer
		if code := runCensusWithDependencies(args, &out, &errout, censusRunnerDependencies{}); code == 0 {
			t.Errorf("ASSERT_FOLDER_CONFLICT: %v", args)
		}
	}
}

// Default ancestry is intentionally absent, on Darwin as well as XDG systems.
func censusFolderDefaultHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "missing", "config"))
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return base
}

func TestCensusFolderDefaultAncestrySafety(t *testing.T) {
	for _, state := range []string{"absent", "readable-ancestor", "writable-ancestor", "owned-symlink", "public-owned", "overlap"} {
		t.Run(state, func(t *testing.T) {
			base := censusFolderDefaultHome(t)
			workspace := t.TempDir()
			ancestor := filepath.Dir(base)
			if state != "absent" && state != "overlap" {
				if err := os.MkdirAll(ancestor, 0700); err != nil {
					t.Fatal(err)
				}
			}
			switch state {
			case "readable-ancestor":
				if err := os.Chmod(ancestor, 0755); err != nil {
					t.Fatal(err)
				}
			case "writable-ancestor":
				if err := os.Chmod(ancestor, 0777); err != nil {
					t.Fatal(err)
				}
			case "owned-symlink", "public-owned":
				if err := os.Mkdir(base, 0700); err != nil {
					t.Fatal(err)
				}
				owned := filepath.Join(base, "lsp-trace")
				if state == "owned-symlink" {
					if err := os.Symlink(t.TempDir(), owned); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Mkdir(owned, 0755); err != nil {
						t.Fatal(err)
					}
				}
			case "overlap":
				workspace = ancestor
				// The source ancestor exists, but none of the config descendants do.
				if err := os.MkdirAll(workspace, 0700); err != nil {
					t.Fatal(err)
				}
			}
			o, err := parseCensusCLIOptions([]string{"census", workspace, "--server", "fake"})
			if err != nil {
				t.Fatal(err)
			}
			got, err := resolveCensusFolderInvocation(o)
			wantOK := state == "absent" || state == "readable-ancestor"
			if (err == nil) != wantOK {
				t.Fatalf("ASSERT_DEFAULT_ANCESTRY_%s: err=%v", state, err)
			}
			if wantOK {
				root, e := publication.OpenRoot(got.PublicationRoot)
				if e != nil {
					t.Fatal(e)
				}
				e = errors.Join(root.ValidatePrivate(), root.Close())
				if e != nil {
					t.Fatalf("ASSERT_DEFAULT_REOPEN_PRIVATE: %v", e)
				}
				info, e := os.Stat(base)
				if e != nil || info.Mode().Perm() != 0700 {
					t.Fatalf("ASSERT_DEFAULT_NEW_PRIVATE: %v %v", info, e)
				}
			} else if state == "writable-ancestor" || state == "overlap" {
				if _, e := os.Lstat(base); !os.IsNotExist(e) {
					t.Fatalf("ASSERT_DEFAULT_NO_CREATE_%s: %v", state, e)
				}
			} else if state == "public-owned" {
				info, e := os.Stat(filepath.Join(base, "lsp-trace"))
				if e != nil || info.Mode().Perm() != 0755 {
					t.Fatalf("ASSERT_DEFAULT_NO_CHMOD: %v %v", info, e)
				}
			}
			if !wantOK {
				deps := productionCensusRunnerDependencies()
				deps.lookPath = func(string) (string, error) { t.Fatal("ASSERT_DEFAULT_NO_PROVIDER_START"); return "", nil }
				var out, errout bytes.Buffer
				if code := runCensusWithDependencies([]string{workspace, "--server", "fake", "--machine"}, &out, &errout, deps); code != 1 || out.Len() != 0 {
					t.Fatalf("ASSERT_DEFAULT_RUNNER_REFUSAL: %d %q", code, out.String())
				}
			}
			if state == "writable-ancestor" {
				info, e := os.Stat(ancestor)
				if e != nil || info.Mode().Perm() != 0777 {
					t.Fatalf("ASSERT_DEFAULT_ANCESTOR_NO_CHMOD: %v %v", info, e)
				}
			}
		})
	}
}

func TestCensusFolderFailureLocator(t *testing.T) {
	for _, failure := range []string{"profile", "lookup", "starter", "manager", "native"} {
		for _, machine := range []bool{false, true} {
			name := failure + "/human"
			if machine {
				name = failure + "/machine"
			}
			t.Run(name, func(t *testing.T) {
				censusFolderDefaultHome(t)
				workspace := t.TempDir()
				deps := productionCensusRunnerDependencies()
				allocated := ""
				deps.resolveFolder = func(o censusCLIOptions) (censusCLIOptions, error) {
					got, err := resolveCensusFolderInvocation(o)
					if err == nil {
						allocated = got.PublicationRoot
					}
					return got, err
				}
				injected := errors.New("injected bounded failure")
				deps.lookPath = func(string) (string, error) {
					if failure == "lookup" {
						return "", injected
					}
					return "/bin/echo", nil
				}
				deps.newStarter = func() (sessionruntime.Starter, error) {
					if failure == "starter" {
						return nil, injected
					}
					return sessionruntime.ManagedStarter{}, nil
				}
				deps.newManager = func(sessionruntime.Config) (*sessionruntime.Manager, error) {
					if failure == "manager" {
						return nil, injected
					}
					return nil, nil
				}
				deps.runNativeCatalog = func(o censusCLIOptions, _ censusCoreConfig, _ io.Writer, errout io.Writer) int {
					return writeCensusFailure(errout, censusStageCommitted, o.Machine, errNativeCatalogAggregateUnavailable)
				}
				args := []string{workspace, "--server", "fake"}
				if failure == "profile" {
					args = []string{workspace, "--profile", "missing", "--config", filepath.Join(workspace, "missing.json")}
				}
				if machine {
					args = append(args, "--machine")
				}
				var out, errout bytes.Buffer
				code := runCensusWithDependencies(args, &out, &errout, deps)
				if code != 1 || out.Len() != 0 || allocated == "" {
					t.Fatalf("ASSERT_LOCATOR_FAILURE_CONTRACT: %d %q %q allocated=%q", code, out.String(), errout.String(), allocated)
				}
				if machine {
					var got censusCLIDiagnostic
					decoder := json.NewDecoder(&errout)
					decoder.DisallowUnknownFields()
					if e := decoder.Decode(&got); e != nil {
						t.Fatal(e)
					}
					if e := decoder.Decode(new(any)); e != io.EOF {
						t.Fatalf("ASSERT_LOCATOR_SINGLE_JSON: %v", e)
					}
					stage := censusStageConfig
					if failure == "native" {
						stage = censusStageCommitted
					}
					want, _ := buildCensusCLIDiagnostic(stage, nil)
					detail := got.Detail
					got.Detail = ""
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("ASSERT_LOCATOR_ORIGINAL_DIAGNOSTIC: got=%+v want=%+v", got, want)
					}
					if !strings.Contains(detail, allocated) {
						t.Fatalf("ASSERT_LOCATOR_EXACT_MACHINE: detail=%q allocated=%q", detail, allocated)
					}
				} else if !strings.Contains(errout.String(), allocated) {
					t.Fatalf("ASSERT_LOCATOR_EXACT_HUMAN: err=%q allocated=%q", errout.String(), allocated)
				}
				root, e := publication.OpenRoot(allocated)
				if e != nil {
					t.Fatal(e)
				}
				if e := errors.Join(root.ValidatePrivate(), root.Close()); e != nil {
					t.Fatalf("ASSERT_LOCATOR_RETAINED_PRIVATE: %v", e)
				}
				marker := filepath.Join(allocated, "retained-marker")
				if e := os.WriteFile(marker, []byte("retained"), 0600); e != nil {
					t.Fatal(e)
				}
				if _, e := censusOwnedDirectory(filepath.Dir(allocated), filepath.Base(allocated), true); e == nil {
					t.Fatal("ASSERT_LOCATOR_NO_REPLACE")
				}
				raw, e := os.ReadFile(marker)
				if e != nil || string(raw) != "retained" {
					t.Fatalf("ASSERT_LOCATOR_RETAINED_CONTENT: %q %v", raw, e)
				}
			})
		}
	}
}

func TestCensusFolderPreallocationNoLocator(t *testing.T) {
	for _, machine := range []bool{false, true} {
		var out, errout bytes.Buffer
		args := []string{filepath.Join(t.TempDir(), "missing")}
		if machine {
			args = append(args, "--machine")
		}
		code := runCensusWithDependencies(args, &out, &errout, censusRunnerDependencies{})
		detail := errout.String()
		if machine {
			var got censusCLIDiagnostic
			if e := json.Unmarshal(errout.Bytes(), &got); e != nil {
				t.Fatal(e)
			}
			detail = got.Detail
		}
		if code != 1 || out.Len() != 0 || machine && detail != "" || strings.Contains(detail, "Retained census run directory:") {
			t.Fatalf("ASSERT_NO_PREALLOCATION_LOCATOR: code=%d out=%q detail=%q", code, out.String(), detail)
		}
	}
}

func TestCensusFolderLegacyDiagnosticsPreserved(t *testing.T) {
	for _, machine := range []bool{false, true} {
		var out, errout bytes.Buffer
		deps := productionCensusRunnerDependencies()
		deps.resolveFolder = func(censusCLIOptions) (censusCLIOptions, error) {
			t.Fatal("ASSERT_LEGACY_NO_FOLDER_RESOLUTION")
			return censusCLIOptions{}, nil
		}
		deps.lookPath = func(string) (string, error) { return "", errors.New("legacy failure") }
		store := t.TempDir()
		if e := os.Chmod(store, 0700); e != nil {
			t.Fatal(e)
		}
		store, _ = filepath.EvalSymlinks(store)
		args := []string{"--workspace", t.TempDir(), "--server", "fake", "--publication-root", store}
		if machine {
			args = append(args, "--machine")
		}
		if code := runCensusWithDependencies(args, &out, &errout, deps); code != 1 || out.Len() != 0 {
			t.Fatalf("ASSERT_LEGACY_FAILURE_CONTRACT: %d %q", code, out.String())
		}
		if machine {
			want, _ := buildCensusCLIDiagnostic(censusStageConfig, nil)
			raw, _ := marshalCensusCLIDiagnostic(want)
			if !bytes.Equal(raw, errout.Bytes()) {
				t.Fatalf("ASSERT_LEGACY_DIAGNOSTIC_BYTES: %q", errout.String())
			}
		} else if errout.String() != "legacy failure\n" {
			t.Fatalf("ASSERT_LEGACY_DIAGNOSTIC_TEXT: %q", errout.String())
		}
	}
}

func TestCensusFolderScopeAndServerArgSyntax(t *testing.T) {
	o, err := parseCensusCLIOptions([]string{"census", ".", "--server", "custom", "--server-arg", "--caller-host", "--source", "src", "--include", "**/*.go", "--batch-targets", "3"})
	if err != nil || !reflect.DeepEqual(o.ServerArgs, []string{"--caller-host"}) || !reflect.DeepEqual(o.Sources, []string{"src"}) || o.MaxBatchTargets != 3 {
		t.Fatalf("ASSERT_FOLDER_SCOPE_ARGS: %+v %v", o, err)
	}
}
