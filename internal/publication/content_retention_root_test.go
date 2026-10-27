//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Causal low-level witness: on Darwin/Linux a competitor can win after absence
// was observed. The legacy precommit duplicated-parent close loses its error.
// Keep that legacy behavior unchanged: callers must reject even PURE ErrExist.
func TestContentRetentionRootLateExistsHiddenParentClose(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	root, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	raw := []byte("equal late competitor")
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	got, err := root.Read(context.Background(), id, int64(len(raw)))
	if got != nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatal("absence not preserved", got, err)
	}
	sentinel := errors.New("duplicated parent close fault")
	installed := false
	sourceCloses := 0
	parentCloses := 0
	before, sourceClose, parentClose := testHookBoundFileBeforePublish, testHookBoundFileSourceClose, testHookBoundFileRootClose
	t.Cleanup(func() {
		testHookBoundFileBeforePublish = before
		testHookBoundFileSourceClose = sourceClose
		testHookBoundFileRootClose = parentClose
	})
	testHookBoundFileBeforePublish = func() {
		if err := os.WriteFile(filepath.Join(path, id), raw, 0600); err != nil {
			t.Fatal(err)
		}
		installed = true
	}
	testHookBoundFileSourceClose = func() error { sourceCloses++; return nil }
	testHookBoundFileRootClose = func() error { parentCloses++; return sentinel }
	receipt, err := root.Publish(context.Background(), id, raw)
	if !installed || sourceCloses != 1 || parentCloses != 1 {
		t.Fatal("fault path not exercised", installed, sourceCloses, parentCloses)
	}
	if receipt != nil || !errors.Is(err, os.ErrExist) {
		t.Fatal("conflict not exposed", receipt, err)
	}
	if errors.Is(err, sentinel) {
		t.Fatal("legacy hidden-close premise changed", err)
	}
	// Successful later equality cannot qualify the failed publication attempt.
	got, readErr := root.Read(context.Background(), id, int64(len(raw)))
	if readErr != nil || !bytes.Equal(got, raw) {
		t.Fatal("competitor not equal", readErr)
	}
}

func TestContentRetentionRootCloseFailsClosed(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	root, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	fileFault := errors.New("file close fault")
	handleFault := errors.New("handle close fault")
	fileClose := root.observed.closeFile
	handleClose := root.observed.closeHandle
	root.observed.closeFile = func(f *os.File) error { return errors.Join(fileClose(f), fileFault) }
	root.observed.closeHandle = func(r *os.Root) error { return errors.Join(handleClose(r), handleFault) }
	err = root.Close()
	if !errors.Is(err, fileFault) || !errors.Is(err, handleFault) {
		t.Fatal("closes lost", err)
	}
	if root.ValidatePrivate() == nil {
		t.Fatal("closed root usable")
	}
	if err := root.Close(); err != nil {
		t.Fatal("not idempotent", err)
	}
}
func TestContentRetentionRootAdmissionNoWrite(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	before, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	after, err := os.ReadDir(path)
	if err != nil || len(before) != len(after) {
		t.Fatal("constructor wrote", err)
	}
	link := path + "-alias"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if alias, err := OpenContentRetentionRoot(context.Background(), link); err == nil {
		alias.Close()
		t.Fatal("symlink admitted")
	}
	raw := []byte("too large invalid ID")
	for _, id := range []string{"../escape", "sha256:invalid"} {
		if _, err := root.Publish(context.Background(), id, raw); err == nil {
			t.Fatal("unsafe id", id)
		}
	}
	if _, err := root.Read(context.Background(), "sha256:invalid", ContentRetentionObjectLimit+1); err == nil {
		t.Fatal("oversize read")
	}
	if _, err := root.Publish(context.Background(), fmt.Sprintf("sha256:%x", sha256.Sum256(nil)), nil); err == nil {
		t.Fatal("empty publication")
	}
}
func TestContentRetentionRootPublishReopen(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	root, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	raw := []byte("new exact retained object")
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	receipt, err := root.Publish(context.Background(), id, raw)
	if err != nil || receipt == nil || !receipt.NamespaceAtomic {
		t.Fatal(receipt, err)
	}
	for i := 0; i < 2; i++ {
		got, err := root.Read(context.Background(), id, int64(len(raw)))
		if err != nil || !bytes.Equal(got, raw) {
			t.Fatal(err)
		}
	}
	alias, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer alias.Close()
	if same, err := root.SameDirectory(alias); err != nil || !same {
		t.Fatal("directory identity", err)
	}
	if _, err := root.Publish(context.Background(), "../escape", raw); err == nil {
		t.Fatal("selector admitted")
	}
	if _, err := root.Publish(context.Background(), id, []byte("wrong")); err == nil {
		t.Fatal("digest mismatch")
	}
	moved := path + "-moved"
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	got, err := root.Read(context.Background(), id, int64(len(raw)))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("ambient root redirected", err)
	}
	if _, err := os.Stat(filepath.Join(path, id)); !os.IsNotExist(err) {
		t.Fatal("replacement written")
	}
}
