//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScopeBundleJournalIdentityAndNamespaces(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	ctx := context.Background()
	root, err := OpenContentRetentionRoot(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	identity, err := root.DirectoryIdentity()
	if err != nil || identity.Inode == 0 {
		t.Fatal(identity, err)
	}
	other, err := OpenContentRetentionRoot(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := other.DirectoryIdentity()
	if err != nil || got != identity {
		t.Fatal("scalar identity differs", err)
	}
	if err = other.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = other.DirectoryIdentity(); err == nil {
		t.Fatal("closed identity capability")
	}
	raw := []byte(`{"fixture":"immutable attempt"}`)
	id := DigestForTest(raw)
	receipt, err := root.PublishScopeBundleReceipt(ctx, id, raw)
	if err != nil || receipt == nil || receipt.FinalSelector != "feature-scope-bundle-attempt-private-v1-"+id[7:]+".json" {
		t.Fatal("attempt physical namespace", receipt, err)
	}
	read, err := root.ReadScopeBundleReceipt(ctx, id)
	if err != nil || !bytes.Equal(read, raw) {
		t.Fatal(err)
	}
	if _, err = root.Read(ctx, id, int64(len(raw))); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("attempt leaked into data namespace", err)
	}
	if _, err = root.PublishScopeBundleReceipt(ctx, id, raw); err == nil {
		t.Fatal("immutable receipt replaced")
	}
	if _, err = root.ReadScopeBundleReceipt(ctx, "../escape"); err == nil {
		t.Fatal("unsafe receipt ID")
	}
	if _, err = root.ReadScopeBundleHead(ctx, "../escape"); err == nil {
		t.Fatal("unsafe head ID")
	}
	selector, _ := scopeBundleAttemptSelector(id)
	if err = os.WriteFile(filepath.Join(path, selector), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = root.ReadScopeBundleReceipt(ctx, id); err == nil {
		t.Fatal("receipt corruption admitted")
	}
}

func TestScopeBundleJournalCloseAwareReceiptRead(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	root, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"fixture":"receipt"}`)
	id := DigestForTest(raw)
	if _, err = root.PublishScopeBundleReceipt(context.Background(), id, raw); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("journal root close failed")
	closeHandle := root.observed.closeHandle
	root.observed.closeHandle = func(h *os.Root) error { return errors.Join(closeHandle(h), sentinel) }
	if err = root.Close(); !errors.Is(err, sentinel) {
		t.Fatal("journal root close erased", err)
	}
	if _, err = root.ReadScopeBundleReceipt(context.Background(), id); err == nil {
		t.Fatal("closed journal capability usable")
	}
}
