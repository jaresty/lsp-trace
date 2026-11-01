//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

func TestProducerPagesPrivateV3SelectedCancelContinue(t *testing.T) {
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	r, _, err := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if err != nil {
		t.Fatal(err)
	}
	object := uint64(len(p.state.objects) - 1)
	reader, err := p.ObjectReader(context.Background(), r, object, pageAllowancePrivateV3())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reads := 0
	originalRead := p.state.ops.read
	p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		b, err := originalRead(ctx, root, id, n)
		reads++
		// Two revision reads and a complete page pair. Cancel during the
		// NEXT page attempt, after the first page's distinct-object check.
		if reads == 5 {
			cancel()
		}
		return b, err
	}
	progress, out, err := reader.Advance(ctx, 1, pageAllowancePrivateV3())
	if !errors.Is(err, context.Canceled) || progress.PagesVerified != 1 || out != nil {
		t.Fatal("ASSERT_SELECTED_CANCEL_PRESERVES_PREFIX", progress, err)
	}
	p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		reads++
		return originalRead(ctx, root, id, n)
	}
	progress, out, err = reader.Advance(context.Background(), 2, pageAllowancePrivateV3())
	if err != nil || !bytes.Equal(out, p.state.objects[object].raw) {
		t.Fatal("ASSERT_SELECTED_CANCEL_CONTINUES_EXACT", progress, err)
	}
	if reads != 2*int(r.Cursor.PageCount)+6 {
		t.Fatal("ASSERT_SELECTED_CANCEL_NO_PAGE_REPLAY", reads, r.Cursor.PageCount)
	}
	if _, err = json.Marshal(reader); err == nil {
		t.Fatal("ASSERT_SELECTED_HANDLE_NO_JSON_RESTORE")
	}
}

func TestProducerPagesPrivateV3SelectedChangedSource(t *testing.T) {
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	r, _, err := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := p.ObjectReader(context.Background(), r, 0, pageAllowancePrivateV3())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(p.state.frozen.state.spec.Root, p.state.frozen.state.spec.Members[0].CanonicalRelativePath)
	if err = os.WriteFile(path, []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := p.BudgetSpent()
	progress, out, err := reader.Advance(context.Background(), 1, pageAllowancePrivateV3())
	if err == nil || out != nil || progress.Status != "FAILED" || before == p.BudgetSpent() {
		t.Fatal("ASSERT_SELECTED_CHANGED_SOURCE_CHARGED_REFUSAL", progress, err)
	}
}
