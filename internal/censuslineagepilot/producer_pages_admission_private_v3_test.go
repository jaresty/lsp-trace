//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

func TestProducerPagesPrivateV3AdmissionContinuation(t *testing.T) {
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	ctx := context.Background()
	grant := pageAllowancePrivateV3()
	r, _, e := p.Advance(ctx, 1, grant, "")
	if e != nil {
		t.Fatal(e)
	}
	reader, e := p.SelectedBatchReader(ctx, r, 0, grant)
	if e != nil {
		t.Fatal(e)
	}
	if _, b, e := reader.Advance(ctx, 1, PageCostPrivateV3{}); !errors.Is(e, ErrPagesPausedPrivateV3) || b.valid() {
		t.Fatal("ASSERT_V3_BATCH_ZERO_PAUSE", e)
	}
	if _, _, e = reader.Advance(ctx, 1, grant); e == nil {
		t.Fatal("ASSERT_V3_BATCH_GENERATION_SPENT")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	read := p.state.ops.read
	reads := 0
	p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		raw, e := read(ctx, root, id, n)
		reads++
		if reads == 5 {
			cancel()
		}
		return raw, e
	}
	progress, b, e := reader.Advance(cancelCtx, 2, grant)
	if !errors.Is(e, context.Canceled) || progress.PagesVerified != 1 || b.valid() {
		t.Fatal("ASSERT_V3_BATCH_CANCEL_PREFIX", progress, e)
	}
	p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		reads++
		return read(ctx, root, id, n)
	}
	progress, b, e = reader.Advance(ctx, 3, grant)
	if e != nil || !b.valid() || progress.PagesVerified != r.Cursor.PageCount {
		t.Fatal("ASSERT_V3_BATCH_RESUME_ADMITTED", progress, e)
	}
	if reads != 2*int(r.Cursor.PageCount)+6 {
		t.Fatal("ASSERT_V3_BATCH_NO_PREFIX_REPLAY", reads, r.Cursor.PageCount)
	}
	if _, _, e = reader.Advance(ctx, 4, grant); e == nil {
		t.Fatal("ASSERT_V3_BATCH_TERMINAL_REPLAY")
	}
	if _, e = json.Marshal(b); e == nil {
		t.Fatal("ASSERT_V3_BATCH_OPAQUE")
	}
	// Mutating a detached clone must not alter the owner's selected source.
	v, e := b.ReadStage(ctx, grant)
	if e != nil {
		t.Fatal(e)
	}
	v.OriginalNativeV3[0] = '!'
	if len(b.state.source.Evidence.GraphBytes) == 0 || b.state.source.Evidence.GraphBytes[0] == '!' {
		t.Fatal("ASSERT_V3_BATCH_CLONE_ISOLATION")
	}
}
func TestProducerPagesPrivateV3AdmissionForeignRevision(t *testing.T) {
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	ctx := context.Background()
	g := pageAllowancePrivateV3()
	r, _, e := p.Advance(ctx, 1, g, "")
	if e != nil {
		t.Fatal(e)
	}
	r.Binding = Digest([]byte("foreign"))
	before := p.BudgetSpent()
	if _, e = p.SelectedBatchReader(ctx, r, 0, g); e == nil || before == p.BudgetSpent() {
		t.Fatal("ASSERT_V3_FOREIGN_REVISION_CHARGED", e)
	}
	r.Binding = p.state.binding
	r.BatchDenominator++
	c, e := p.SelectedBatchReader(ctx, r, 0, g)
	if e != nil {
		t.Fatal(e)
	}
	if _, b, e := c.Advance(ctx, 1, g); e == nil || b.valid() {
		t.Fatal("ASSERT_V3_FOREIGN_DENOMINATOR")
	}
}
func TestProducerPagesPrivateV3AdmissionCorruptAfterPause(t *testing.T) {
	for _, mode := range []string{"corrupt", "cross-object", "source-changed"} {
		t.Run(mode, func(t *testing.T) {
			p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
			ctx := context.Background()
			g := pageAllowancePrivateV3()
			r, _, e := p.Advance(ctx, 1, g, "")
			if e != nil {
				t.Fatal(e)
			}
			c, e := p.SelectedBatchReader(ctx, r, 0, g)
			if e != nil {
				t.Fatal(e)
			}
			cancelCtx, cancel := context.WithCancel(ctx)
			read := p.state.ops.read
			reads := 0
			p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
				raw, e := read(ctx, root, id, n)
				reads++
				if reads == 5 {
					cancel()
				}
				return raw, e
			}
			progress, _, e := c.Advance(cancelCtx, 1, g)
			cancel()
			if !errors.Is(e, context.Canceled) || progress.PagesVerified != 1 {
				t.Fatal(e, progress)
			}
			switch mode {
			case "source-changed":
				if e = os.WriteFile(filepath.Join(p.state.frozen.state.spec.Root, p.state.frozen.state.spec.Members[0].CanonicalRelativePath), []byte("package changed\n"), 0600); e != nil {
					t.Fatal(e)
				}
				p.state.ops.read = read
			default:
				badID := c.state.proof.last
				p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
					if id == badID {
						if mode == "cross-object" {
							return read(ctx, root, r.Cursor.Last, n)
						}
						return []byte("corrupt retained bytes"), nil
					}
					return read(ctx, root, id, n)
				}
			}
			before := p.BudgetSpent()
			progress, b, e := c.Advance(ctx, 2, g)
			if e == nil || b.valid() || progress.Status != "FAILED" || progress.PagesVerified != 1 || before == p.BudgetSpent() {
				t.Fatal("ASSERT_V3_FAILED_SUFFIX_PRESERVES_PREFIX_CHARGED", progress, e)
			}
			p.state.ops.read = read
			if _, b, e := c.Advance(ctx, 3, g); e == nil || b.valid() {
				t.Fatal("ASSERT_V3_FAILED_TERMINAL_NO_REPLAY")
			}
		})
	}
}
