//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/sessionruntime"
)

func variableOwnedCodeRepo(t *testing.T) (string, string) {
	t.Helper()
	t.Setenv("GOWORK", "off")
	t.Setenv("GOENV", "off")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GO111MODULE", "on")
	root := variableOwnedGitRepo(t)
	files := map[string]string{
		"go.mod": "module owned\n\ngo 1.26.5\n", "go.sum": "",
		".gitignore":                                      "internal/censuslineagepilot/ignored*.go\ninternal/censuslineagepilot/assets/ignored*\n",
		"internal/censuslineagepilot/caller.go":           "package censuslineagepilot\nimport (\"embed\";_ \"owned/internal/helper\")\n//go:embed assets\nvar data embed.FS\nconst Owned = 1\n",
		"internal/censuslineagepilot/caller_test.go":      "package censuslineagepilot_test\nimport _ \"owned/internal/testdep\"\n",
		"internal/censuslineagepilot/platform_windows.go": "package censuslineagepilot\nimport _ \"owned/internal/platformdep\"\n",
		"internal/censuslineagepilot/native.h":            "#define OWNED 1\n",
		"internal/censuslineagepilot/assets/a.txt":        "owned embedded bytes\n",
		"internal/censuslineagepilot/assets/nested/b.txt": "nested retained bytes\n",
		"internal/helper/helper.go":                       "package helper\nconst Value = 1\n",
		"internal/testdep/test.go":                        "package testdep\n",
		"internal/platformdep/platform.go":                "package platformdep\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	variableOwnedGitSetup(t, root, "add", ".")
	variableOwnedGitSetup(t, root, "commit", "-qm", "owned code inputs")
	return root, strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD"))
}

// Only this owned framing seam substitutes the compiled-checkout path assertion.
// HEAD/blob reads and all byte/set verification still use real absolute Git.
// The real caller's runtime.Caller boundary is unchanged and tested separately.
func variableOwnedConfigWithCode(t *testing.T) (variableCallerConfig, string, variableGitRunner) {
	t.Helper()
	c, p, _ := variableOwnedConfig(t, 1, 0)
	c.CodeRoot, c.CodeHEAD = variableOwnedCodeRepo(t)
	variableSave(t, p, c)
	git := func(ctx context.Context, root string, args ...string) ([]byte, error) {
		q := strings.Join(args, " ")
		if root == c.CodeRoot && q == "rev-parse --show-toplevel" {
			return []byte(c.CodeRoot + "\n"), nil
		}
		if root == c.Scope.Root && q == "rev-parse HEAD" {
			return []byte(c.Scope.RevisionAssertion + "\n"), nil
		}
		return variableGit(ctx, root, args...)
	}
	return c, p, git
}
func variableOwnedVerify(t *testing.T, root, head string) (variableCodeWitness, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return variableVerifyCodeInputs(ctx, root, head)
}
func variableOwnedRepositorySnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	bytesRead := 0
	e := filepath.WalkDir(root, func(p string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if len(snapshot) > 512 {
			return errors.New("owned snapshot entry bound")
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		var raw []byte
		if !entry.IsDir() {
			if info.Size() > 4<<20 || bytesRead+int(info.Size()) > 8<<20 {
				return errors.New("owned snapshot byte bound")
			}
			raw, e = os.ReadFile(p)
			if e != nil {
				return e
			}
			bytesRead += len(raw)
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		snapshot[rel] = fmt.Sprintf("%v/%d/%x", info.Mode(), info.ModTime().UnixNano(), sha256.Sum256(raw))
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return snapshot
}

func TestVariableCallerV2CodeInputsPositive(t *testing.T) {
	c, p, git := variableOwnedConfigWithCode(t)
	before := variableOwnedRepositorySnapshot(t, c.CodeRoot)
	w, e := variableOwnedVerify(t, c.CodeRoot, c.CodeHEAD)
	if e != nil {
		t.Fatal(e)
	}
	if w.Profile != variableCodeProfile || w.Files < 10 || w.Packages != 4 || w.Bytes <= 0 || w.Bytes > w.Limits.MaxTotalBytes || w.InputsSHA256 == "" {
		t.Fatal("complete conservative local/test/platform/embed closure", w)
	}
	again, e := variableOwnedVerify(t, c.CodeRoot, c.CodeHEAD)
	if e != nil || again != w {
		t.Fatal("deterministic code witness", e, again, w)
	}
	factories := 0
	r, e := variableRunWithCode(context.Background(), "preflight", p, git, func() (*sessionruntime.Manager, error) { factories++; return nil, nil }, variableClose, variableVerifyCodeInputs)
	if e != nil || r.CodeInputs != w || factories != 0 {
		t.Fatal("readonly genuine code preflight", e, r.CodeInputs)
	}
	variableEmpty(t, c)
	if !reflect.DeepEqual(before, variableOwnedRepositorySnapshot(t, c.CodeRoot)) {
		t.Fatal("readonly verifier changed working/Git metadata bytes/modes/mtime")
	}
	t.Logf("ASSERT_HEAD_INPUT_CLOSURE: PASS; files=%d packages=%d bytes=%d directories=%d commands=%d digest=%s", w.Files, w.Packages, w.Bytes, w.Directories, w.GitCommands, w.InputsSHA256)
}
func TestVariableCallerV2CodeInputsHiddenChanges(t *testing.T) {
	for _, kind := range []string{"assume-unchanged", "ignored-extra-go"} {
		t.Run(kind, func(t *testing.T) {
			c, p, git := variableOwnedConfigWithCode(t)
			if kind == "assume-unchanged" {
				rel := "internal/censuslineagepilot/caller.go"
				variableOwnedGitSetup(t, c.CodeRoot, "update-index", "--assume-unchanged", rel)
				file := filepath.Join(c.CodeRoot, filepath.FromSlash(rel))
				raw, e := os.ReadFile(file)
				if e != nil {
					t.Fatal(e)
				}
				raw = bytes.Replace(raw, []byte("const Owned = 1"), []byte("const Owned = 2"), 1)
				if e = os.WriteFile(file, raw, 0600); e != nil {
					t.Fatal(e)
				}
			} else {
				if e := os.WriteFile(filepath.Join(c.CodeRoot, "internal/censuslineagepilot/ignored_extra.go"), []byte("package censuslineagepilot\nfunc Injected(){}\n"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			// Establish the exact old blind spot before applying the byte-level guard.
			variableOwnedGitSetup(t, c.CodeRoot, "diff", "--quiet", "--no-ext-diff", "--no-textconv")
			variableOwnedGitSetup(t, c.CodeRoot, "diff", "--cached", "--quiet", "--no-ext-diff", "--no-textconv")
			if got := variableOwnedGitSetup(t, c.CodeRoot, "ls-files", "--others", "--exclude-standard"); got != "" {
				t.Fatal("old clean gate prerequisite", got)
			}
			factory, starts, _, _ := variableOwnedFactory(t, "")
			_, e := variableRunWithCode(context.Background(), "execute-once", p, git, factory, variableClose, variableVerifyCodeInputs)
			if e == nil || starts.Load() != 0 {
				t.Fatalf("ASSERT_HEAD_HIDDEN_INPUT_REFUSED: %s accepted or started: %v starts=%d", kind, e, starts.Load())
			}
			variableEmpty(t, c)
			t.Log("ASSERT_HEAD_HIDDEN_INPUT_REFUSED: PASS; " + kind + " refused before marker/writes/start despite clean index/status")
		})
	}
}
func TestVariableCallerV2CodeInputsNegative(t *testing.T) {
	for _, kind := range []string{"embedded-modified", "embedded-untracked", "header-modified", "native-untracked", "missing-head-file", "sum-modified", "sum-missing", "module-symlink", "wrong-source", "size", "symlink", "embed-symlink", "replace", "workspace", "relative-import", "absolute-import", "overlay", "toolexec", "external-workspace", "implicit-workspace", "external-goenv", "ignored-hidden-positive"} {
		t.Run(kind, func(t *testing.T) {
			root, head := variableOwnedCodeRepo(t)
			write := func(rel, body string) {
				if e := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0600); e != nil {
					t.Fatal(e)
				}
			}
			switch kind {
			case "embedded-modified":
				write("internal/censuslineagepilot/assets/a.txt", "changed embedded bytes\n")
			case "embedded-untracked":
				write("internal/censuslineagepilot/assets/ignored_new.txt", "new embedded input\n")
			case "header-modified":
				write("internal/censuslineagepilot/native.h", "#define OWNED 2\n")
			case "native-untracked":
				write("internal/censuslineagepilot/new.S", "owned assembly injection\n")
			case "missing-head-file":
				if e := os.Remove(filepath.Join(root, "internal/helper/helper.go")); e != nil {
					t.Fatal(e)
				}
			case "sum-modified":
				write("go.sum", "changed checksum input\n")
			case "sum-missing":
				if e := os.Remove(filepath.Join(root, "go.sum")); e != nil {
					t.Fatal(e)
				}
			case "module-symlink":
				if e := os.Remove(filepath.Join(root, "go.mod")); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(filepath.Join(root, "internal/helper/helper.go"), filepath.Join(root, "go.mod")); e != nil {
					t.Fatal(e)
				}
			case "wrong-source":
				write("internal/platformdep/platform.go", "package platformdep\nconst Wrong = true\n")
			case "size":
				f, e := os.OpenFile(filepath.Join(root, "internal/censuslineagepilot/caller.go"), os.O_WRONLY, 0)
				if e != nil {
					t.Fatal(e)
				}
				e = errors.Join(f.Truncate(int64(variableCodePolicy().MaxFileBytes)+1), f.Close())
				if e != nil {
					t.Fatal(e)
				}
			case "symlink", "embed-symlink":
				rel := "internal/censuslineagepilot/native.h"
				if kind == "embed-symlink" {
					rel = "internal/censuslineagepilot/assets/a.txt"
				}
				file := filepath.Join(root, filepath.FromSlash(rel))
				if e := os.Remove(file); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(filepath.Join(root, "go.mod"), file); e != nil {
					t.Fatal(e)
				}
			case "replace":
				write("go.mod", "module owned\n\ngo 1.26.5\nreplace other => ../outside\n")
				variableOwnedGitSetup(t, root, "add", "go.mod")
				variableOwnedGitSetup(t, root, "commit", "-qm", "owned unsupported replace")
				head = strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD"))
			case "workspace":
				write("go.work", "go 1.26.5\nuse .\n")
				variableOwnedGitSetup(t, root, "add", "go.work")
				variableOwnedGitSetup(t, root, "commit", "-qm", "owned unsupported workspace")
				head = strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD"))
			case "relative-import", "absolute-import":
				imported := "./outside"
				if kind == "absolute-import" {
					imported = "/owned/outside"
				}
				write("internal/censuslineagepilot/caller.go", "package censuslineagepilot\nimport _ "+strconv.Quote(imported)+"\n")
				variableOwnedGitSetup(t, root, "add", "internal/censuslineagepilot/caller.go")
				variableOwnedGitSetup(t, root, "commit", "-qm", "owned unsupported import")
				head = strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD"))
			case "overlay":
				t.Setenv("GOFLAGS", "-overlay=/owned/no-overlay.json")
			case "toolexec":
				t.Setenv("GOFLAGS", "-toolexec=/owned/no-exec")
			case "external-workspace":
				t.Setenv("GOWORK", "/owned/no-workspace")
			case "implicit-workspace":
				t.Setenv("GOWORK", "")
			case "external-goenv":
				t.Setenv("GOENV", "/owned/no-goenv")
			case "ignored-hidden-positive":
				write("internal/censuslineagepilot/_ignored.go", "not compiled Go source\n")
				write("internal/censuslineagepilot/assets/.ignored", "Go embed directory walking ignores this\n")
			}
			_, e := variableOwnedVerify(t, root, head)
			if kind == "ignored-hidden-positive" {
				if e != nil {
					t.Fatal("standard ignored prefixes overconstrained", e)
				}
			} else if e == nil {
				t.Fatal("unsafe code inputs accepted", kind)
			}
		})
	}
}
func TestVariableCallerV2CodeWitnessClaimAndRecheck(t *testing.T) {
	for _, kind := range []string{"stable", "before-claim", "before-start"} {
		t.Run(kind, func(t *testing.T) {
			c, p, git := variableOwnedConfigWithCode(t)
			factory, starts, _, _ := variableOwnedFactory(t, "")
			calls := 0
			verify := func(ctx context.Context, root, head string) (variableCodeWitness, error) {
				calls++
				if kind == "before-claim" && calls == 2 || kind == "before-start" && calls == 3 {
					if e := os.WriteFile(filepath.Join(root, "internal/censuslineagepilot/native.h"), []byte("changed after initial code preflight\n"), 0600); e != nil {
						return variableCodeWitness{}, e
					}
				}
				return variableVerifyCodeInputs(ctx, root, head)
			}
			report, e := variableRunWithCode(context.Background(), "execute-once", p, git, factory, variableClose, verify)
			if kind == "stable" {
				if e != nil || starts.Load() != 1 || calls != 3 || report.CodeInputs.Profile != variableCodeProfile {
					t.Fatal("genuine code witness full owned path", e, calls, starts.Load())
				}
				raw, e := os.ReadFile(c.AttemptMarkerPath)
				if e != nil {
					t.Fatal(e)
				}
				var marker struct{ CodeInputs variableCodeWitness }
				if json.Unmarshal(raw, &marker) != nil || marker.CodeInputs != report.CodeInputs {
					t.Fatal("claim not bound to exact code witness")
				}
			} else {
				if e == nil || starts.Load() != 0 {
					t.Fatal("changed code inputs started", kind, e, starts.Load())
				}
				if kind == "before-claim" {
					variableEmpty(t, c)
				} else {
					if report.Phase != "FAILED_SPENT" {
						t.Fatal("post-claim code change not spent", report.Phase)
					}
					if _, e := os.Stat(c.AttemptMarkerPath); e != nil {
						t.Fatal(e)
					}
					if _, e := os.Stat(c.ReportPath); e != nil {
						t.Fatal(e)
					}
				}
			}
		})
	}
}
func TestVariableCallerV2GitRejectsHelperAndPromisorConfig(t *testing.T) {
	for _, kind := range []string{"clean-filter", "promisor"} {
		t.Run(kind, func(t *testing.T) {
			root, head := variableOwnedCodeRepo(t)
			receipt := filepath.Join(t.TempDir(), "forbidden-helper-invoked")
			if kind == "clean-filter" {
				variableOwnedGitSetup(t, root, "config", "filter.owned.clean", "touch '"+receipt+"'; cat")
			} else {
				variableOwnedGitSetup(t, root, "config", "remote.owned.promisor", "true")
			}
			_, e := variableOwnedVerify(t, root, head)
			if e == nil {
				t.Fatal("unsupported Git helper configuration accepted", kind)
			}
			_, e = variableGit(context.Background(), root, "diff", "--cached", "--quiet")
			if e == nil {
				t.Fatal("diff permitted helper/promisor configuration", kind)
			}
			if _, e := os.Stat(receipt); !os.IsNotExist(e) {
				t.Fatal("forbidden Git helper executed", e)
			}
		})
	}
}

func TestVariableCallerV2GitDisablesFSMonitor(t *testing.T) {
	root, _ := variableOwnedCodeRepo(t)
	trap := t.TempDir()
	receipt := filepath.Join(trap, "fsmonitor-invoked")
	hook := filepath.Join(trap, "owned-fsmonitor")
	if e := os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+receipt+"'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	variableOwnedGitSetup(t, root, "config", "core.fsmonitor", hook)
	if _, e := variableGit(context.Background(), root, "diff", "--cached", "--quiet"); e != nil {
		t.Fatal("safe fixed Git diff", e)
	}
	if _, e := os.Stat(receipt); !os.IsNotExist(e) {
		t.Fatal("fsmonitor helper ran", e)
	}
}

func TestVariableCallerV2AdditionalRootCapacity(t *testing.T) {
	c, _, _ := variableOwnedConfig(t, 1, 0)
	for i := 0; i < 15; i++ {
		c.ProtectedRoots = append(c.ProtectedRoots, "/owned/protected-"+strings.Repeat("x", i+1))
	}
	if e := variableValidate(c); e != nil {
		t.Fatal("15 additional roots pure capacity", e)
	}
	c.ProtectedRoots = append(c.ProtectedRoots, "/owned/sixteenth")
	if e := variableValidate(c); e == nil {
		t.Fatal("16 additional roots accepted")
	}
}
