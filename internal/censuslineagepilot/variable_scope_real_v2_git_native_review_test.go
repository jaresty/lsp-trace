//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Only compiled-path/source-revision framing is owned. All Git object/config
// observations, index/working bytes and the verifier itself are real operations
// on disposable repositories; no fake tree or helper factory supplies proof.
func variableOwnedReviewConfig(t *testing.T, root, head string) (variableCallerConfig, string, variableGitRunner) {
	t.Helper()
	c, p, _ := variableOwnedConfig(t, 1, 0)
	c.CodeRoot = root
	c.CodeHEAD = head
	variableSave(t, p, c)
	git := func(ctx context.Context, r string, args ...string) ([]byte, error) {
		q := strings.Join(args, " ")
		if r == c.CodeRoot && q == "rev-parse --show-toplevel" {
			return []byte(r + "\n"), nil
		}
		if r == c.Scope.Root && q == "rev-parse HEAD" {
			return []byte(c.Scope.RevisionAssertion + "\n"), nil
		}
		return variableGit(ctx, r, args...)
	}
	return c, p, git
}
func TestVariableCallerV2EffectiveWorktreeConfigNoHelper(t *testing.T) {
	for _, kind := range []string{"direct", "included"} {
		t.Run(kind, func(t *testing.T) {
			main, head := variableOwnedCodeRepo(t)
			parent, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			linked := filepath.Join(parent, "linked")
			variableOwnedGitSetup(t, main, "worktree", "add", "-q", "-b", "owned-linked", linked)
			variableOwnedGitSetup(t, main, "config", "extensions.worktreeConfig", "true")
			trap := t.TempDir()
			receipt := filepath.Join(trap, "clean-helper-invoked")
			helper := filepath.Join(trap, "owned-clean")
			body := "#!/bin/sh\nprintf invoked > '" + receipt + "'\n/bin/cat\n"
			if e := os.WriteFile(helper, []byte(body), 0700); e != nil {
				t.Fatal(e)
			}
			if kind == "direct" {
				variableOwnedGitSetup(t, linked, "config", "--worktree", "filter.owned.clean", helper)
			} else {
				included := filepath.Join(trap, "owned-helper-config")
				if e := os.WriteFile(included, []byte("[filter \"owned\"]\nclean = "+helper+"\n"), 0600); e != nil {
					t.Fatal(e)
				}
				variableOwnedGitSetup(t, linked, "config", "--worktree", "include.path", included)
			}
			attr := strings.TrimSpace(variableOwnedGitSetup(t, linked, "rev-parse", "--git-path", "info/attributes"))
			if !filepath.IsAbs(attr) {
				attr = filepath.Join(linked, attr)
			}
			if e := os.MkdirAll(filepath.Dir(attr), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(attr, []byte("internal/censuslineagepilot/caller.go filter=owned\n"), 0600); e != nil {
				t.Fatal(e)
			}
			file := filepath.Join(linked, "internal/censuslineagepilot/caller.go")
			raw, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(file, append(raw, []byte("\n// owned dirty input triggers clean conversion\n")...), 0600); e != nil {
				t.Fatal(e)
			}
			c, p, git := variableOwnedReviewConfig(t, linked, head)
			factory, starts, _, _ := variableOwnedFactory(t, "")
			_, runErr := variableRunWithCode(context.Background(), "execute-once", p, git, factory, variableClose, variableVerifyCodeInputs)
			_, receiptErr := os.Stat(receipt)
			if runErr == nil || starts.Load() != 0 || !os.IsNotExist(receiptErr) {
				t.Fatalf("ASSERT_EFFECTIVE_WORKTREE_CONFIG_NO_HELPER: helper_receipt=%v starts=%d err=%v", receiptErr, starts.Load(), runErr)
			}
			variableEmpty(t, c)
			if _, e := variableOwnedVerify(t, linked, head); e == nil {
				t.Fatal("code verifier missed effective worktree configuration")
			}
			if _, e := os.Stat(receipt); !os.IsNotExist(e) {
				t.Fatal("readonly effective-config query executed helper", e)
			}
			t.Log("ASSERT_EFFECTIVE_WORKTREE_CONFIG_NO_HELPER: PASS; " + kind + " worktree config refused before conversion, marker and start; receipt absent")
		})
	}
}
func TestVariableCallerV2UnreplacedHeadTree(t *testing.T) {
	root, a := variableOwnedCodeRepo(t)
	rel := "internal/censuslineagepilot/caller.go"
	aBlob := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", a+":"+rel))
	aTree := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", a+"^{tree}"))
	file := filepath.Join(root, filepath.FromSlash(rel))
	raw, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	original := bytes.Clone(raw)
	raw = bytes.Replace(raw, []byte("const Owned = 1"), []byte("const Owned = 2"), 1)
	if e := os.WriteFile(file, raw, 0600); e != nil {
		t.Fatal(e)
	}
	variableOwnedGitSetup(t, root, "add", rel)
	variableOwnedGitSetup(t, root, "commit", "-qm", "owned replacement B")
	b := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD"))
	bBlob := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", b+":"+rel))
	if aBlob == bBlob {
		t.Fatal("replacement fixture did not distinguish trees")
	}
	variableOwnedGitSetup(t, root, "update-ref", "HEAD", a)
	variableOwnedGitSetup(t, root, "replace", a, b)
	// Normal Git still exposes A as HEAD but interprets B's tree. This raw setup
	// observation deliberately does not use the runner being tested.
	if got := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD")); got != a {
		t.Fatal("HEAD label prerequisite", got)
	}
	if got := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", a+":"+rel)); got != bBlob {
		t.Fatal("replacement tree prerequisite", got, bBlob)
	}
	variableOwnedGitSetup(t, root, "diff", "--quiet", "--no-ext-diff", "--no-textconv")
	variableOwnedGitSetup(t, root, "diff", "--cached", "--quiet", "--no-ext-diff", "--no-textconv")
	// Attempted ambient opt-out cannot override the hardened runner.
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "0")
	w, e := variableOwnedVerify(t, root, a)
	if e == nil {
		t.Fatalf("ASSERT_UNREPLACED_HEAD_TREE: accepted B under pinned A; witnessHEAD=%s A=%s B=%s workingBlob=%s originalBlob=%s", w.CodeHEAD, a, b, bBlob, aBlob)
	}
	c, p, git := variableOwnedReviewConfig(t, root, a)
	factory, starts, _, _ := variableOwnedFactory(t, "")
	if _, e := variableRunWithCode(context.Background(), "execute-once", p, git, factory, variableClose, variableVerifyCodeInputs); e == nil || starts.Load() != 0 {
		t.Fatal("replaced HEAD reached startup", e, starts.Load())
	}
	variableEmpty(t, c)
	// Restore only the working bytes to original A while leaving the mutable
	// replacement ref present. The raw verifier must now bind A's original tree.
	if e := os.WriteFile(file, original, 0600); e != nil {
		t.Fatal(e)
	}
	originalWitness, e := variableOwnedVerify(t, root, a)
	if e != nil || originalWitness.CodeHEAD != a || originalWitness.CommitTreeOID != aTree {
		t.Fatal("original unreplaced commit/tree not bound", e, originalWitness)
	}
	if got := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", a+":"+rel)); got != bBlob {
		t.Fatal("verifier modified replacement refs", got)
	}
	t.Log("ASSERT_UNREPLACED_HEAD_TREE: PASS; B refused before marker/start; restoring A verifies only A's original commit/tree with replacement ref still present")
}
func TestVariableCallerV2NativeSyntaxRejectedWithoutGrammarClaim(t *testing.T) {
	cases := []struct{ name, ext, body string }{
		{"cpp-relative", ".s", "#include \"../shared.h\"\n"},
		{"cpp-macro", ".S", "# include HEADER_MACRO\n"},
		{"asm-include", ".s", ".include \"../shared.h\"\n"},
		{"nasm-percent", ".asm", "%include \"../shared.h\"\n"},
		{"binary-incbin", ".s", ".incbin \"../object.bin\"\n"},
		{"toolchain-header-not-allowlisted", ".s", "#include \"textflag.h\"\n"},
		{"c-preprocessor", ".c", "#include \"../shared.h\"\n"},
		{"fortran-include", ".F", "include '../shared.h'\n"},
		{"swig-include", ".swig", "%include \"../shared.h\"\n"},
		{"native-object", ".syso", "owned opaque object bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := variableOwnedCodeRepo(t)
			rel := "internal/censuslineagepilot/owned" + tc.ext
			if e := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(tc.body), 0600); e != nil {
				t.Fatal(e)
			}
			variableOwnedGitSetup(t, root, "add", rel)
			variableOwnedGitSetup(t, root, "commit", "-qm", "owned unsupported native input")
			head := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD"))
			_, e := variableOwnedVerify(t, root, head)
			if e == nil || !strings.Contains(e.Error(), "native/assembly") {
				t.Fatal("unsupported native syntax did not fail closed explicitly", tc.name, e)
			}
		})
	}
}

func TestVariableCallerV2GitRawCommandAllowlist(t *testing.T) {
	for _, args := range [][]string{{"diff", "--quiet"}, {"cat-file", "--filters", "HEAD:owned.go"}, {"cat-file", "--textconv", "HEAD:owned.go"}, {"hash-object", "--path=owned.go", "owned.go"}, {"checkout", "HEAD"}, {"config", "--local", "--get", "filter.owned.clean"}} {
		if variableReadOnlyGitArgs(args) {
			t.Fatal("unsafe or scope-narrowed Git form admitted", args)
		}
		if _, e := variableGit(context.Background(), "/owned/not-opened", args...); e == nil {
			t.Fatal("unsafe Git command reached execution", args)
		}
	}
}

func TestVariableCallerV2NativeOutsideIncludeRefused(t *testing.T) {
	root, _ := variableOwnedCodeRepo(t)
	if e := os.WriteFile(filepath.Join(root, "internal/shared.h"), []byte("#define OWNED_VALUE 1\n"), 0600); e != nil {
		t.Fatal(e)
	}
	asm := "#include \"../shared.h\"\nTEXT ·Owned(SB), $0-0\n RET\n"
	if e := os.WriteFile(filepath.Join(root, "internal/censuslineagepilot/owned.s"), []byte(asm), 0600); e != nil {
		t.Fatal(e)
	}
	variableOwnedGitSetup(t, root, "add", ".")
	variableOwnedGitSetup(t, root, "commit", "-qm", "owned assembly include")
	head := strings.TrimSpace(variableOwnedGitSetup(t, root, "rev-parse", "HEAD"))
	variableOwnedGitSetup(t, root, "update-index", "--assume-unchanged", "internal/shared.h")
	if e := os.WriteFile(filepath.Join(root, "internal/shared.h"), []byte("#define OWNED_VALUE 2\n"), 0600); e != nil {
		t.Fatal(e)
	}
	variableOwnedGitSetup(t, root, "diff", "--quiet", "--no-ext-diff", "--no-textconv")
	variableOwnedGitSetup(t, root, "diff", "--cached", "--quiet", "--no-ext-diff", "--no-textconv")
	c, p, git := variableOwnedReviewConfig(t, root, head)
	factory, starts, _, _ := variableOwnedFactory(t, "")
	_, e := variableRunWithCode(context.Background(), "execute-once", p, git, factory, variableClose, variableVerifyCodeInputs)
	if e == nil || starts.Load() != 0 {
		t.Fatalf("ASSERT_NATIVE_INCLUDE_REFUSED: outside changed header escaped closure; starts=%d err=%v", starts.Load(), e)
	}
	if !strings.Contains(e.Error(), "native/assembly") {
		t.Fatal("native indirection needs explicit unsupported refusal", e)
	}
	variableEmpty(t, c)
	t.Log("ASSERT_NATIVE_INCLUDE_REFUSED: PASS; native include indirection refused before marker/start despite clean Git index/status")
}
