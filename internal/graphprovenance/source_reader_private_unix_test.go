//go:build darwin || linux

package graphprovenance

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func sourceReaderWorkspacePrivateV1(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func sourceReaderWritePrivateV1(t *testing.T, name string, b []byte) {
	t.Helper()
	if err := os.WriteFile(name, b, 0644); err != nil {
		t.Fatal(err)
	}
}
func TestSourceReaderPrivateV1NoAliasTargetAccess(t *testing.T) {
	for _, directory := range []bool{false, true} {
		for _, equal := range []bool{false, true} {
			name := map[bool]string{false: "file", true: "directory"}[directory] + map[bool]string{false: "-different", true: "-equal"}[equal]
			t.Run(name, func(t *testing.T) {
				workspace := sourceReaderWorkspacePrivateV1(t)
				expected := []byte("expected source\n")
				other := []byte("different source\n")
				if equal {
					other = expected
				}
				if err := os.Mkdir(filepath.Join(workspace, "allowed"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(workspace, "forbidden"), 0755); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(workspace, "forbidden", "a.go")
				sourceReaderWritePrivateV1(t, target, other)
				sourceReaderWritePrivateV1(t, filepath.Join(workspace, "allowed", "a.go"), expected)
				forbidden, err := os.Stat(target)
				if err != nil {
					t.Fatal(err)
				}
				forbiddenDirectory, err := os.Stat(filepath.Dir(target))
				if err != nil {
					t.Fatal(err)
				}
				opened, readTargets := 0, 0
				ops := sourceReadOpsPrivateV1{
					hook: func(stage, name string, info os.FileInfo) error {
						if stage == "directory-opened" && os.SameFile(info, forbiddenDirectory) {
							t.Fatal("ASSERT_PRIVATE_NO_FORBIDDEN_DIRECTORY_OPEN")
						}
						if stage == "file-opened" {
							opened++
							if os.SameFile(info, forbidden) {
								t.Fatal("ASSERT_PRIVATE_NO_FORBIDDEN_OPEN")
							}
						}
						return nil
					},
					read: func(f *os.File, p []byte) (int, error) {
						info, err := f.Stat()
						if err != nil {
							return 0, err
						}
						if os.SameFile(info, forbidden) {
							readTargets++
							t.Error("ASSERT_PRIVATE_NO_FORBIDDEN_READ")
						}
						return f.Read(p)
					},
				}
				root, err := openSourceRootWithOpsPrivateV1(context.Background(), workspace, ops)
				if err != nil {
					t.Fatal(err)
				}
				defer root.Close()
				// Simulate replacement after a scope/root was bound, not initial discovery.
				if directory {
					if err = os.Rename(filepath.Join(workspace, "allowed"), filepath.Join(workspace, "old-allowed")); err != nil {
						t.Fatal(err)
					}
					if err = os.Symlink("forbidden", filepath.Join(workspace, "allowed")); err != nil {
						t.Fatal(err)
					}
				} else {
					if err = os.Remove(filepath.Join(workspace, "allowed", "a.go")); err != nil {
						t.Fatal(err)
					}
					if err = os.Symlink("../forbidden/a.go", filepath.Join(workspace, "allowed", "a.go")); err != nil {
						t.Fatal(err)
					}
				}
				b, err := root.read(context.Background(), "allowed/a.go", MaxFileBytes)
				if err == nil || b != nil || opened != 0 || readTargets != 0 {
					t.Fatalf("ASSERT_PRIVATE_NO_ALIAS_TARGET_ACCESS: bytes=%q err=%v opens=%d forbiddenReads=%d", b, err, opened, readTargets)
				}
				after, err := os.Stat(target)
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(target)
				if err != nil || !bytes.Equal(got, other) || !os.SameFile(forbidden, after) || !forbidden.ModTime().Equal(after.ModTime()) {
					t.Fatal("forbidden target changed", err)
				}
			})
		}
	}
}
func TestSourceReaderPrivateV1ReplacementRaces(t *testing.T) {
	for _, stage := range []string{"before-directory-open", "directory-opened", "before-file-open", "file-opened"} {
		t.Run(stage, func(t *testing.T) {
			workspace := sourceReaderWorkspacePrivateV1(t)
			for _, dir := range []string{"allowed", "forbidden"} {
				if err := os.Mkdir(filepath.Join(workspace, dir), 0755); err != nil {
					t.Fatal(err)
				}
				sourceReaderWritePrivateV1(t, filepath.Join(workspace, dir, "a.go"), []byte("equal source"))
			}
			forbidden, err := os.Stat(filepath.Join(workspace, "forbidden", "a.go"))
			if err != nil {
				t.Fatal(err)
			}
			reads, opens, swaps := 0, 0, 0
			ops := sourceReadOpsPrivateV1{
				hook: func(current, name string, info os.FileInfo) error {
					if current == "file-opened" {
						opens++
						if os.SameFile(info, forbidden) {
							t.Fatal("ASSERT_PRIVATE_RACE_WRONG_FD")
						}
					}
					if current != stage || swaps != 0 {
						return nil
					}
					swaps++
					if current == "before-directory-open" || current == "directory-opened" {
						if err := os.Rename(filepath.Join(workspace, "allowed"), filepath.Join(workspace, "pinned-allowed")); err != nil {
							return err
						}
						return os.Symlink("forbidden", filepath.Join(workspace, "allowed"))
					}
					if err := os.Rename(filepath.Join(workspace, "allowed", "a.go"), filepath.Join(workspace, "allowed", "pinned.go")); err != nil {
						return err
					}
					return os.Symlink("../forbidden/a.go", filepath.Join(workspace, "allowed", "a.go"))
				},
				read: func(f *os.File, b []byte) (int, error) {
					reads++
					info, err := f.Stat()
					if err != nil {
						return 0, err
					}
					if os.SameFile(info, forbidden) {
						t.Fatal("ASSERT_PRIVATE_RACE_WRONG_READ")
					}
					return f.Read(b)
				},
			}
			root, err := openSourceRootWithOpsPrivateV1(context.Background(), workspace, ops)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			b, err := root.read(context.Background(), "allowed/a.go", MaxFileBytes)
			if err == nil || b != nil || swaps != 1 || reads != 0 {
				t.Fatalf("ASSERT_PRIVATE_PINNED_REPLACEMENT: bytes=%q err=%v swaps=%d reads=%d opens=%d", b, err, swaps, reads, opens)
			}
			if stage != "file-opened" && opens != 0 {
				t.Fatal("unexpected leaf open", opens)
			}
		})
	}
}
func TestSourceReaderPrivateV1OrdinaryAndUncertainty(t *testing.T) {
	for _, kind := range []string{"readonly", "empty", "oversize", "cancel-before", "cancel-after-open", "root-retired", "root-replaced", "read-fault", "close-fault", "joined-read-close", "metadata-after-read", "joined-eof-fault"} {
		t.Run(kind, func(t *testing.T) {
			workspace := sourceReaderWorkspacePrivateV1(t)
			name := filepath.Join(workspace, "a.go")
			content := []byte("ordinary source")
			if kind == "empty" {
				content = nil
			}
			sourceReaderWritePrivateV1(t, name, content)
			if err := os.Chmod(name, 0444); err != nil {
				t.Fatal(err)
			}
			readFault, closeFault := errors.New("private injected read fault"), errors.New("private injected close fault")
			reads := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ops := sourceReadOpsPrivateV1{
				read: func(f *os.File, b []byte) (int, error) {
					reads++
					switch kind {
					case "read-fault", "joined-read-close":
						return 0, readFault
					case "joined-eof-fault":
						n, err := f.Read(b)
						return n, errors.Join(err, io.EOF, readFault)
					}
					return f.Read(b)
				},
				close: func(f *os.File) error {
					err := f.Close()
					if kind == "close-fault" || kind == "joined-read-close" {
						return errors.Join(err, closeFault)
					}
					return err
				},
				hook: func(stage, path string, info os.FileInfo) error {
					if stage == "file-opened" && kind == "cancel-after-open" {
						cancel()
					}
					if stage == "file-read" && kind == "metadata-after-read" {
						return os.Chmod(name, 0600)
					}
					return nil
				},
			}
			root, err := openSourceRootWithOpsPrivateV1(context.Background(), workspace, ops)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			limit := int64(MaxFileBytes)
			switch kind {
			case "oversize":
				limit = 1
			case "cancel-before":
				cancel()
			case "root-retired":
				if err = root.Close(); err != nil {
					t.Fatal(err)
				}
			case "root-replaced":
				if err = os.Rename(workspace, workspace+"-pinned"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.RemoveAll(workspace + "-pinned") })
				if err = os.Mkdir(workspace, 0755); err != nil {
					t.Fatal(err)
				}
				sourceReaderWritePrivateV1(t, name, content)
			}
			b, err := root.read(ctx, "a.go", limit)
			if kind == "readonly" || kind == "empty" {
				if err != nil || !bytes.Equal(b, content) || reads == 0 {
					t.Fatalf("ASSERT_PRIVATE_ORDINARY_SOURCE: %q %v reads=%d", b, err, reads)
				}
				return
			}
			if err == nil || b != nil {
				t.Fatalf("ASSERT_PRIVATE_UNCERTAINTY_NO_CONTENT: %q %v", b, err)
			}
			if (kind == "read-fault" || kind == "joined-read-close" || kind == "joined-eof-fault") && !errors.Is(err, readFault) {
				t.Fatal("read error lost", err)
			}
			if (kind == "close-fault" || kind == "joined-read-close") && !errors.Is(err, closeFault) {
				t.Fatal("close error lost", err)
			}
			if (kind == "oversize" || kind == "cancel-before" || kind == "cancel-after-open" || kind == "root-retired" || kind == "root-replaced") && reads != 0 {
				t.Fatal("read before admission", reads)
			}
		})
	}
}
func TestSourceReaderPrivateV1RootAncestorNoFollow(t *testing.T) {
	base := sourceReaderWorkspacePrivateV1(t)
	if err := os.Mkdir(filepath.Join(base, "real"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(base, "real", "workspace"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(base, "alias")); err != nil {
		t.Fatal(err)
	}
	if root, err := openSourceRootPrivateV1(context.Background(), filepath.Join(base, "alias", "workspace")); err == nil || root != nil {
		if root != nil {
			root.Close()
		}
		t.Fatal("ASSERT_PRIVATE_ROOT_ANCESTOR_SYMLINK")
	}
}
