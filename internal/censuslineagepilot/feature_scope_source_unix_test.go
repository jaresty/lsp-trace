//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestFeatureScopeTransportFaults(t *testing.T) {
	for _, kind := range []string{"file-substitution", "directory-substitution", "root-substitution", "late-substitution", "rewrite-restore-mtime", "cancel-after-open", "cancel-after-read", "hook-error", "close-error", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			root := scopeFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fired := false
			closed := 0
			sentinel := errors.New("fixture transport failure")
			if kind == "fifo" {
				p := filepath.Join(root, fixtureScopePaths[0])
				if e := os.Remove(p); e != nil {
					t.Fatal(e)
				}
				if e := unix.Mkfifo(p, 0600); e != nil {
					t.Fatal(e)
				}
			}
			ops := scopeReadOps{close: func(f *os.File) error {
				closed++
				e := f.Close()
				if kind == "close-error" && closed == 1 {
					return errors.Join(e, sentinel)
				}
				return e
			}}
			ops.hook = func(stage, path string) error {
				if fired {
					return nil
				}
				if kind == "root-substitution" && stage == "root-opened" {
					fired = true
					if e := os.Rename(root, root+"-old"); e != nil {
						return e
					}
					t.Cleanup(func() { os.RemoveAll(root + "-old") })
					return os.Mkdir(root, 0755)
				}
				if stage == "file-opened" && path == fixtureScopePaths[0] {
					switch kind {
					case "file-substitution":
						fired = true
						p := filepath.Join(root, path)
						if e := os.Rename(p, p+"-old"); e != nil {
							return e
						}
						return os.WriteFile(p, []byte("substituted"), 0644)
					case "directory-substitution":
						fired = true
						p := filepath.Join(root, "cmd/lsp-trace")
						if e := os.Rename(p, p+"-old"); e != nil {
							return e
						}
						return os.Mkdir(p, 0755)
					case "cancel-after-open":
						fired = true
						cancel()
					case "hook-error":
						fired = true
						return sentinel
					}
				}
				if stage == "file-read" && kind == "rewrite-restore-mtime" {
					fired = true
					p := filepath.Join(root, path)
					info, e := os.Stat(p)
					if e != nil {
						return e
					}
					if e = os.WriteFile(p, []byte(strings.Repeat("x", int(info.Size()))), 0644); e != nil {
						return e
					}
					return os.Chtimes(p, info.ModTime(), info.ModTime())
				}
				if stage == "file-read" && kind == "cancel-after-read" {
					fired = true
					cancel()
				}
				if stage == "before-final" && kind == "late-substitution" {
					fired = true
					p := filepath.Join(root, fixtureScopePaths[0])
					if e := os.Remove(p); e != nil {
						return e
					}
					return os.Symlink(filepath.Join(root, fixtureScopePaths[1]), p)
				}
				return nil
			}
			b, e := freezeFeatureScopeWithOps(ctx, root, fixtureRevision, nil, ops)
			if e == nil || b.state != nil {
				t.Fatalf("P3 transport %s accepted", kind)
			}
			if closed == 0 {
				t.Fatal("P3 no descriptor closes")
			}
			if kind == "close-error" && !errors.Is(e, sentinel) {
				t.Fatal("P3 close error lost")
			}
			if strings.HasPrefix(kind, "cancel-") && !errors.Is(e, context.Canceled) {
				t.Fatal("P3 cancel lost")
			}
		})
	}
}
func TestFeatureScopePinnedRootRename(t *testing.T) {
	root := scopeFixture(t)
	moved := root + "-moved"
	t.Cleanup(func() { os.RemoveAll(moved) })
	fired := false
	b, e := freezeFeatureScopeWithOps(context.Background(), root, fixtureRevision, nil, scopeReadOps{hook: func(stage, path string) error {
		if stage == "file-opened" && !fired {
			fired = true
			return os.Rename(root, moved)
		}
		return nil
	}})
	if e != nil || len(b.Files()) != 6 {
		t.Fatalf("pinned post-admission rename: %v", e)
	}
}
func TestFeatureScopeBoundsEquality(t *testing.T) {
	root := scopeFixture(t)
	sizes := []int{featureScopeFileLimit, featureScopeFileLimit, featureScopeFileLimit, featureScopeFileLimit - 2, 1, 1}
	for i, p := range fixtureScopePaths {
		if e := os.WriteFile(filepath.Join(root, p), make([]byte, sizes[i]), 0644); e != nil {
			t.Fatal(e)
		}
	}
	b, e := freezeFeatureScope(context.Background(), root, fixtureRevision, nil)
	if e != nil || b.state == nil {
		t.Fatalf("P2 equality must pass: %v", e)
	}
	p := filepath.Join(root, fixtureScopePaths[5])
	if e := os.WriteFile(p, []byte("xx"), 0644); e != nil {
		t.Fatal(e)
	}
	b, e = freezeFeatureScope(context.Background(), root, fixtureRevision, nil)
	if e == nil || b.state != nil {
		t.Fatal("P3 total bound plus one accepted")
	}
}
func TestFeatureScopeAncestorSymlink(t *testing.T) {
	root := scopeFixture(t)
	parent := filepath.Dir(root)
	link := filepath.Join(parent, "source-link")
	if e := os.Symlink(root, link); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(link)
	b, e := freezeFeatureScope(context.Background(), link, fixtureRevision, nil)
	if e == nil || b.state != nil {
		t.Fatal("P3 root symlink accepted")
	}
}
