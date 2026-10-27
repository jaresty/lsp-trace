//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/publication"
)

// Recursive, no-follow tree observation includes names, directory/file identity,
// permissions, content and mtime. Atime is deliberately not a write assertion.
func topologyTreePrivate(t *testing.T, paths [3]string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			content := ""
			if info.Mode().IsRegular() {
				raw, e := os.ReadFile(path)
				if e != nil {
					return e
				}
				content = Digest(raw)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				content, err = os.Readlink(path)
				if err != nil {
					return err
				}
			}
			result[path] = fmt.Sprintf("%v %v %v %s", info.Mode(), info.ModTime().UnixNano(), scopeIdentity(info), content)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}
func topologyUnchangedPrivate(t *testing.T, f executionClaimFixturePrivate, run func()) {
	t.Helper()
	before := topologyTreePrivate(t, f.inputPaths)
	run()
	if !reflect.DeepEqual(before, topologyTreePrivate(t, f.inputPaths)) {
		t.Fatal("input tree changed")
	}
}
func topologyOutputPrivate(t *testing.T, f *executionClaimFixturePrivate, role int, path string) {
	t.Helper()
	r, err := publication.OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	f.roots[role], f.outputPaths[role] = r, path
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
}
func topologyRefusedPrivate(t *testing.T, f executionClaimFixturePrivate) {
	t.Helper()
	topologyUnchangedPrivate(t, f, func() {
		p, err := preflightScopedExecutionRootsPrivate(context.Background(), f.restart, f.inputPaths, f.roots)
		if p != nil || err == nil {
			if p != nil {
				_ = p.Close()
			}
			t.Fatal("overlap admitted")
		}
	})
}
func TestScopedExecutionTopologyPrivateDisjoint(t *testing.T) {
	f := executionClaimFixture(t)
	topologyUnchangedPrivate(t, f, func() {
		p := executionTopologyProofPrivate(t, f)
		o := f.owner(t)
		s, err := newScopedExecutionStorePrivate(context.Background(), o, f.roots, p)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		raw, err := s.Load(context.Background(), o.selector)
		if err != nil || raw != nil {
			t.Fatalf("absent load %q %v", raw, err)
		}
	})
}
func TestScopedExecutionTopologyPrivateOutputPairs(t *testing.T) {
	for a := 0; a < 3; a++ {
		for b := a + 1; b < 3; b++ {
			for _, kind := range []string{"same-inode", "a-ancestor", "b-ancestor"} {
				t.Run(fmt.Sprintf("%d-%d/%s", a, b, kind), func(t *testing.T) {
					f := executionClaimFixture(t)
					switch kind {
					case "same-inode":
						topologyOutputPrivate(t, &f, b, f.outputPaths[a]) // distinct capability, same admitted inode
					default:
						parent, child := a, b
						if kind == "b-ancestor" {
							parent, child = b, a
						}
						path := filepath.Join(f.outputPaths[parent], "child")
						if err := os.Mkdir(path, 0700); err != nil {
							t.Fatal(err)
						}
						topologyOutputPrivate(t, &f, child, path)
					}
					topologyRefusedPrivate(t, f)
				})
			}
		}
	}
}
func TestScopedExecutionTopologyPrivateEveryInputOutput(t *testing.T) {
	for input := 0; input < 3; input++ {
		for output := 0; output < 3; output++ {
			for _, kind := range []string{"equal", "output-ancestor", "output-descendant"} {
				t.Run(fmt.Sprintf("input-%d/output-%d/%s", input, output, kind), func(t *testing.T) {
					f := executionClaimFixture(t)
					path := f.inputPaths[input]
					switch kind {
					case "output-ancestor":
						path = filepath.Dir(path)
					case "output-descendant":
						path = filepath.Join(path, "owned-test-child")
						if err := os.Mkdir(path, 0700); err != nil {
							t.Fatal(err)
						}
					}
					topologyOutputPrivate(t, &f, output, path)
					topologyRefusedPrivate(t, f)
				})
			}
		}
	}
}
func TestScopedExecutionTopologyPrivateInputPinsAndNoFollow(t *testing.T) {
	for role := 0; role < 3; role++ {
		for _, kind := range []string{"wrong-identity", "symlink-component"} {
			t.Run(fmt.Sprintf("%d/%s", role, kind), func(t *testing.T) {
				f := executionClaimFixture(t)
				paths := f.inputPaths
				if kind == "wrong-identity" {
					paths[role] = f.outputPaths[0]
				} else {
					// A symlink to the correct inode is still not an exact no-follow path.
					alias := filepath.Join(filepath.Dir(f.outputPaths[0]), "input-alias")
					if err := os.Symlink(filepath.Dir(paths[role]), alias); err != nil {
						t.Fatal(err)
					}
					paths[role] = filepath.Join(alias, filepath.Base(paths[role]))
				}
				topologyUnchangedPrivate(t, f, func() {
					p, err := preflightScopedExecutionRootsPrivate(context.Background(), f.restart, paths, f.roots)
					if err == nil || p != nil {
						t.Fatal("asserted input identity/path admitted")
					}
				})
			})
		}
	}
}
func TestScopedExecutionTopologyPrivateStoreProofBinding(t *testing.T) {
	f := executionClaimFixture(t)
	o := f.owner(t)
	p := executionTopologyProofPrivate(t, f)
	other := executionClaimFixture(t)
	mixed := other
	mixed.roots = f.roots
	for _, kind := range []string{"nil", "zero", "mixed-outputs", "mixed-inputs", "owner-roles", "closed-proof", "closed-output"} {
		t.Run(kind, func(t *testing.T) {
			proof := p
			owner := o
			roots := f.roots
			switch kind {
			case "nil":
				proof = nil
			case "zero":
				proof = &scopedExecutionTopologyPrivate{}
			case "mixed-outputs":
				proof = executionTopologyProofPrivate(t, other)
			case "mixed-inputs":
				proof = executionTopologyProofPrivate(t, mixed)
			case "owner-roles":
				swapped := f
				swapped.roots[0], swapped.roots[1] = swapped.roots[1], swapped.roots[0]
				owner = swapped.owner(t)
			case "closed-proof":
				proof = executionTopologyProofPrivate(t, f)
				if err := proof.Close(); err != nil {
					t.Fatal(err)
				}
			case "closed-output":
				own := executionClaimFixture(t)
				proof = executionTopologyProofPrivate(t, own)
				owner = own.owner(t)
				roots = own.roots
				if err := roots[0].Close(); err != nil {
					t.Fatal(err)
				}
			}
			topologyUnchangedPrivate(t, f, func() {
				s, err := newScopedExecutionStorePrivate(context.Background(), owner, roots, proof)
				if s != nil || err == nil {
					if s != nil {
						_ = s.Close()
					}
					t.Fatal("unmatched proof reached store")
				}
			})
		})
	}
}
func TestScopedExecutionTopologyPrivateReplacementBoundaries(t *testing.T) {
	for _, phase := range []string{"before-preflight", "before-store", "before-operation"} {
		for _, role := range []string{"input-primary", "input-backup", "input-journal", "output-primary", "output-backup", "output-coordinator", "output-parent-symlink"} {
			t.Run(phase+"/"+role, func(t *testing.T) {
				f := executionClaimFixture(t)
				o := f.owner(t)
				var p *scopedExecutionTopologyPrivate
				var s *scopedExecutionStorePrivate
				var err error
				if phase != "before-preflight" {
					p = executionTopologyProofPrivate(t, f)
				}
				if phase == "before-operation" {
					s, err = newScopedExecutionStorePrivate(context.Background(), o, f.roots, p)
					if err != nil {
						t.Fatal(err)
					}
					defer s.Close()
				}
				path := ""
				input := -1
				switch role {
				case "input-primary":
					input = 0
				case "input-backup":
					input = 1
				case "input-journal":
					input = 2
				case "output-primary":
					path = f.outputPaths[0]
				case "output-backup":
					path = f.outputPaths[1]
				case "output-coordinator":
					path = f.outputPaths[2]
				case "output-parent-symlink":
					path = filepath.Dir(f.outputPaths[0])
				}
				if input >= 0 {
					path = f.inputPaths[input]
				}
				moved := path + "-test-moved"
				if err = os.Rename(path, moved); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(moved); err != nil {
						t.Error(err)
					}
				})
				if role == "output-parent-symlink" {
					err = os.Symlink(moved, path)
				} else {
					err = os.Mkdir(path, 0700)
				}
				if err != nil {
					t.Fatal(err)
				}
				// Snapshot the actual retained tree, now at its renamed exact fixture path.
				observed := f
				if input >= 0 {
					observed.inputPaths[input] = moved
				}
				topologyUnchangedPrivate(t, observed, func() {
					switch phase {
					case "before-preflight":
						proof, e := preflightScopedExecutionRootsPrivate(context.Background(), f.restart, f.inputPaths, f.roots)
						if e == nil || proof != nil {
							t.Fatal("replaced path admitted")
						}
					case "before-store":
						store, e := newScopedExecutionStorePrivate(context.Background(), o, f.roots, p)
						if e == nil || store != nil {
							t.Fatal("stale proof admitted")
						}
					case "before-operation":
						if _, e := s.Load(context.Background(), o.selector); e == nil {
							t.Fatal("stale path reached Load")
						}
						if r, e := s.CompareAbsent(context.Background(), o.selector, o.raw); e == nil || r != nil {
							t.Fatal("stale path reached CAS")
						}
					}
				})
				// No claim was published through either the old descriptor or new pathname.
				coordinator := f.outputPaths[2]
				if role == "output-coordinator" {
					coordinator = moved
				}
				if _, e := os.Lstat(filepath.Join(coordinator, o.selector)); !errors.Is(e, os.ErrNotExist) {
					t.Fatalf("invalid topology wrote head: %v", e)
				}
			})
		}
	}
}
func TestScopedExecutionTopologyPrivateCloseErrors(t *testing.T) {
	for _, kind := range []string{"success-close", "failure-cleanup", "open-error-with-handle"} {
		t.Run(kind, func(t *testing.T) {
			f := executionClaimFixture(t)
			fault := errors.New("input read-only close fault")
			openFault := errors.New("read-only open returned capability and error")
			opened, closed := 0, 0
			ops := scopedExecutionTopologyOpsPrivate{
				open: func(c context.Context, path string) (*publication.ObservedRoot, error) {
					r, e := publication.OpenObservedRoot(c, path)
					if r != nil {
						opened++
					}
					if kind == "open-error-with-handle" && opened == 2 {
						e = errors.Join(e, openFault)
					}
					return r, e
				},
				close: func(r *publication.ObservedRoot) error { closed++; return errors.Join(r.Close(), fault) },
			}
			if kind == "failure-cleanup" {
				topologyOutputPrivate(t, &f, 1, f.outputPaths[0])
			}
			topologyUnchangedPrivate(t, f, func() {
				p, err := preflightScopedExecutionRootsWithOpsPrivate(context.Background(), f.restart, f.inputPaths, f.roots, ops)
				if kind == "success-close" {
					if err != nil {
						t.Fatal(err)
					}
					copied := *p
					err = p.Close()
					if copied.validate(context.Background()) == nil {
						t.Fatal("copied proof survived close")
					}
					if e := p.Close(); e != nil {
						t.Fatal(e)
					}
				} else if p != nil {
					t.Fatal("bad topology minted proof")
				}
				expected := 3
				if kind == "open-error-with-handle" {
					expected = 2
					if !errors.Is(err, openFault) {
						t.Fatal("open error lost", err)
					}
				}
				if !errors.Is(err, fault) || opened != expected || closed != opened {
					t.Fatalf("close causes/count %d/%d %v", closed, opened, err)
				}
			})
		})
	}
}
