//go:build linux || darwin

package publication

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestObservedRootAncestorSwapClosesAllOwnedHandles(t *testing.T) {
	for _, privateTmp := range []bool{false, true} {
		name := "other-private-root"
		if privateTmp {
			name = "alias-to-private-tmp"
		}
		t.Run(name, func(t *testing.T) {
			path, id, raw := observedRootFixture(t)
			other, _, _ := observedRootFixture(t)
			targetAncestor := filepath.Dir(other)
			if privateTmp {
				base, e := os.MkdirTemp("/private/tmp", "lsp-trace-root-swap-test-")
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() {
					if e := os.RemoveAll(base); e != nil {
						t.Error(e)
					}
				})
				targetAncestor = base
				root := filepath.Join(base, "root")
				if e := os.Mkdir(root, 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(filepath.Join(root, id), raw, 0600); e != nil {
					t.Fatal(e)
				}
			}
			ops, log := trackingObservedRootOps(t)
			open := ops.openDir
			swapped := false
			ops.openDir = func(parent *os.File, name string) (*os.File, error) {
				if name == "ancestor" && !swapped {
					swapped = true
					ancestor := filepath.Dir(path)
					if e := os.Rename(ancestor, ancestor+"-saved"); e != nil {
						t.Fatal(e)
					}
					if e := os.Symlink(targetAncestor, ancestor); e != nil {
						t.Fatal(e)
					}
				}
				return open(parent, name)
			}
			r, e := openObservedRoot(context.Background(), path, ops)
			if !swapped {
				t.Fatal("substitution event not reached")
			}
			if e == nil || r != nil {
				if r != nil {
					r.Close()
				}
				t.Fatal("ancestor alias admitted")
			}
			if len(log.handles) != 0 {
				t.Fatal("created content-reading capability after forbidden component")
			}
			log.assertClosed(t)
		})
	}
}
