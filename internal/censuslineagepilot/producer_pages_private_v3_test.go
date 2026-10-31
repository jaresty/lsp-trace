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

func pageAllowancePrivateV3() PageCostPrivateV3 {
	return PageCostPrivateV3{1 << 32, 1 << 33, 1 << 30, 1 << 32}
}
func pageOwnerPrivateV3(t *testing.T, payload uint64) ProducerPagesPrivateV3 {
	t.Helper()
	return pageOwnerCountsPrivateV3(t, payload, 2, 1)
}
func pageOwnerCountsPrivateV3(t *testing.T, payload uint64, nominations, contextFiles int) ProducerPagesPrivateV3 {
	t.Helper()
	f, o, c, _ := packetFixturePrivateV2(t, nominations, contextFiles)
	roots, _, _ := packetRootsPrivateV2(t, f)
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	ledger, e := CreatePageLedgerPrivateV3(context.Background(), dir, Digest([]byte("test application")), pageAllowancePrivateV3())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := ledger.Close(); e != nil {
			t.Error(e)
		}
	})
	p, e := NewProducerPagesPrivateV3(context.Background(), f, o, c, roots, PagePolicyPrivateV3{payload, 1 << 20, Digest([]byte("explicit test config"))}, ledger)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestProducerPagesPrivateV3Connected(t *testing.T) {
	p := pageOwnerPrivateV3(t, 16384)
	r, id, e := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != "COMPLETE_ORIGIN_PAGES" || r.Cursor.Object != r.ObjectDenominator || r.Cursor.PageCount <= r.ObjectDenominator || r.BatchDenominator != uint64(len(p.state.captures)) || !validPageDigestPrivateV3(id) {
		t.Fatal("ASSERT_COMPLETE_HASH_BOUND_DENOMINATOR", r)
	}
	if e = p.VerifyPrefix(context.Background(), r, pageAllowancePrivateV3()); e != nil {
		t.Fatal(e)
	}
	for i, obj := range p.state.objects {
		b, e := p.ReadObject(context.Background(), r, uint64(i), pageAllowancePrivateV3())
		if e != nil || !bytes.Equal(b, obj.raw) {
			t.Fatal("ASSERT_EVERY_ORIGINAL_ORDERED_ROLE_EXACT", i, e)
		}
	}
	batch, e := p.Batch(context.Background(), r, 0, pageAllowancePrivateV3())
	if e != nil {
		t.Fatal(e)
	}
	view, e := batch.Read(context.Background(), pageAllowancePrivateV3())
	if e != nil || !bytes.Equal(view.SourceArtifact, p.state.captures[0].state.artifact) || view.Qualification != "PRIVATE_PAGED_ORIGINAL_OBSERVATIONS_ONLY_NOT_AST_ADMISSION" {
		t.Fatal("ASSERT_NEW_TYPED_BATCH_NOT_LEGACY_OWNER", e)
	}
	view.SourceArtifact[0] = '!'
	again, e := batch.Read(context.Background(), pageAllowancePrivateV3())
	if e != nil || again.SourceArtifact[0] == '!' {
		t.Fatal("ASSERT_DETACHED_PAGED_BATCH", e)
	}
	if _, e = json.Marshal(batch); e == nil {
		t.Fatal("ASSERT_PAGED_BATCH_NOT_DTO_RESTORABLE")
	}
	if _, e = json.Marshal(p); e == nil {
		t.Fatal("ASSERT_NO_OWNER_SERIALIZATION")
	}
	if _, _, e = p.Advance(context.Background(), 1, pageAllowancePrivateV3(), id); e == nil {
		t.Fatal("ASSERT_NO_SPENT_GENERATION_REPLAY")
	}
}
func TestProducerPagesPrivateV3BudgetPause(t *testing.T) {
	p := pageOwnerPrivateV3(t, 1024)
	unit := pagePairCostPrivateV3(1024 + producerPageHeaderPrivateV3 + 4)
	meta := pagePairCostPrivateV3(producerPageHeaderPrivateV3)
	exchange, _ := meta.plus(unit)
	exchange, _ = exchange.plus(sourceCheckCostRequiredPrivateV3(t, p))
	exchange, _ = exchange.plus(pageJournalCostPrivateV3)
	exchange, _ = exchange.plus(pageLedgerOverheadPrivateV3)
	exchange, _ = exchange.plus(pageLedgerOverheadPrivateV3)
	r, id, e := p.Advance(context.Background(), 1, exchange, "")
	if !errors.Is(e, ErrPagesPausedPrivateV3) || id == "" || r.Cursor.PageCount != 1 || r.Status != "PAUSED_BUDGET" {
		t.Fatal("ASSERT_USEFUL_PAIRED_PREFIX_PAUSE", r, id, e)
	}
	spent := p.BudgetSpent()
	if e = p.VerifyPrefix(context.Background(), r, pageAllowancePrivateV3()); e != nil {
		t.Fatal(e)
	}
	again, next, e := p.Advance(context.Background(), 2, pageAllowancePrivateV3(), id)
	if e != nil || next == id || again.Previous != id || again.Cursor.PageCount <= r.Cursor.PageCount || !spent.fits(again.Spent) {
		t.Fatal("ASSERT_CUMULATIVE_CONTINUATION", e)
	}
	// Earlier immutable revision remains valid after publishing a successor.
	if e = p.VerifyPrefix(context.Background(), r, pageAllowancePrivateV3()); e != nil {
		t.Fatal("ASSERT_NO_RETROACTIVE_ERASURE", e)
	}
}
func TestProducerPagesPrivateV3ExactBinaryFrame(t *testing.T) {
	raw := append([]byte("{\n\"n\":1.000e+2,\"calls\":[null,null],\"s\":\"π\"}\n"), 0, 0xff, 0xfe)
	var all []byte
	for offset := 0; offset < len(raw); {
		n := 3
		if n > len(raw)-offset {
			n = len(raw) - offset
		}
		h := pageHeaderPrivateV3{Schema: ProducerPagesSchemaPrivateV3, Offset: uint64(offset), Length: uint64(len(raw))}
		b, e := encodePagePrivateV3(h, raw[offset:offset+n])
		if e != nil {
			t.Fatal(e)
		}
		got, part, e := decodePagePrivateV3(b)
		if e != nil || got != h {
			t.Fatal(e)
		}
		all = append(all, part...)
		offset += n
	}
	if !bytes.Equal(all, raw) {
		t.Fatal("ASSERT_UTF8_BINARY_NUMERIC_NULL_TAIL_EXACT")
	}
}
func TestProducerPagesPrivateV3Refusals(t *testing.T) {
	p := pageOwnerPrivateV3(t, 16384)
	r, _, e := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"missing", "reorder", "foreign", "denominator", "raw-count", "offset", "policy", "authority", "claim", "complete-partial"} {
		t.Run(kind, func(t *testing.T) {
			bad := r
			switch kind {
			case "missing":
				bad.Cursor.PageCount--
			case "reorder":
				bad.Cursor.Object--
			case "foreign":
				bad.Binding = Digest([]byte("foreign"))
			case "denominator":
				bad.ObjectDenominator++
			case "raw-count":
				bad.Cursor.RawBytes++
			case "offset":
				bad.Cursor.Offset = 1
			case "policy":
				bad.Policy.ConfigSHA256 = Digest([]byte("changed"))
			case "authority":
				bad.Authority = 1
			case "claim":
				bad.Source = "COMPLETE"
			case "complete-partial":
				bad.Cursor = PageCursorPrivateV3{}
			}
			if e := p.VerifyPrefix(context.Background(), bad, pageAllowancePrivateV3()); e == nil {
				t.Fatal("ASSERT_FORGED_COVERAGE_REFUSED", kind)
			}
		})
	}
}
func TestProducerPagesPrivateV3FailedAttemptCharged(t *testing.T) {
	p := pageOwnerPrivateV3(t, 16384)
	before := p.BudgetSpent()
	calls := 0
	ops := p.state.ops
	p.state.ops.publish = func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error) {
		calls++
		return nil, os.ErrPermission
	}
	_, _, e := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if e == nil || calls == 0 || p.BudgetSpent() == before {
		t.Fatal("ASSERT_FAILED_PREFIX_NONREFUNDABLE", e)
	}
	p.state.ops = ops
	if _, _, e = p.Advance(context.Background(), 1, pageAllowancePrivateV3(), ""); e == nil {
		t.Fatal("ASSERT_FAILED_GENERATION_SPENT")
	}
}
func TestProducerPagesPrivateV3PhysicalPairAndCorruption(t *testing.T) {
	p := pageOwnerPrivateV3(t, 8192)
	r, _, e := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	observations, e := publication.ObserveProducerPacketPairCustodyPrivateV2(context.Background(), p.state.roots.state.outputs, r.Cursor.Last)
	if e != nil || observations[0].ObjectInode == observations[1].ObjectInode && observations[0].ObjectDevice == observations[1].ObjectDevice {
		t.Fatal("ASSERT_DISTINCT_PAGE_INODES", e)
	}
	file := filepath.Join(observations[1].RootPath, r.Cursor.Last)
	if e = os.WriteFile(file, []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = p.VerifyPrefix(context.Background(), r, pageAllowancePrivateV3()); e == nil {
		t.Fatal("ASSERT_FRESH_PAIRED_READBACK")
	}
}
func TestProducerPagesPrivateV3VerifierResumable(t *testing.T) {
	// One original member keeps repeated policy-max source reservations within the
	// unchanged application allowance. Still verify one page per exchange; do not
	// raise allowances or reset the ledger to hide the conservative cost.
	p := pageOwnerCountsPrivateV3(t, 16384, 1, 0)
	r, _, e := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	verifier, e := p.PrefixVerifier(r)
	if e != nil {
		t.Fatal(e)
	}
	if r.Cursor.PageCount < 2 {
		t.Fatal("ASSERT_VERIFIER_MULTIPLE_BOUNDED_EXCHANGES")
	}
	max := p.state.policy.PayloadBytes + producerPageHeaderPrivateV3 + 4
	grant, _ := sourceCheckCostRequiredPrivateV3(t, p).plus(PageCostPrivateV3{2 * (max + 1), 8 * (max + 1), 0, producerPacketTopologyWorkPrivateV2 + 16})
	grant, _ = grant.plus(pageLedgerOverheadPrivateV3)
	grant, _ = grant.plus(pageLedgerOverheadPrivateV3)
	grant, _ = grant.plus(revisionReadCostPrivateV3())
	grant, _ = grant.plus(pageLedgerOverheadPrivateV3)
	reads := 0
	read := p.state.ops.read
	p.state.ops.read = func(ctx context.Context, root *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		reads++
		return read(ctx, root, id, n)
	}
	var progress PageVerificationProgressPrivateV3
	for generation := uint64(1); generation <= r.Cursor.PageCount; generation++ {
		progress, e = verifier.Advance(context.Background(), generation, grant)
		if e != nil && !errors.Is(e, ErrPagesPausedPrivateV3) {
			t.Fatal(e)
		}
		if progress.PagesVerified != generation {
			t.Fatal("ASSERT_VERIFIER_MAKES_BOUNDED_RESUMABLE_PROGRESS", generation, progress)
		}
	}
	if progress.Status != "VERIFIED_ORIGINAL_PREFIX" || reads != 4*int(r.Cursor.PageCount) {
		t.Fatal("ASSERT_NO_FULL_PREFIX_RESCAN_PER_EXCHANGE", reads, progress)
	}
	if _, e = verifier.Advance(context.Background(), r.Cursor.PageCount, grant); e == nil {
		t.Fatal("ASSERT_VERIFIER_SPENT_GENERATION")
	}
}
func TestProducerPagesPrivateV3ChangedSource(t *testing.T) {
	p := pageOwnerPrivateV3(t, 16384)
	path := filepath.Join(p.state.frozen.state.spec.Root, p.state.frozen.state.spec.Members[0].CanonicalRelativePath)
	if e := os.WriteFile(path, []byte("package changed\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), ""); e == nil {
		t.Fatal("ASSERT_CHANGED_SOURCE_REFUSED")
	}
}
func TestProducerPagesPrivateV3LargeRawObjectSegmented(t *testing.T) {
	p := pageOwnerPrivateV3(t, 32768)
	// Isolated byte-codec intervention, NOT a new original success owner or semantic
	// fixture: use an existing source-body role for arbitrary binary segmentation.
	p.state.objects[0].raw = bytes.Repeat([]byte{0, 0xff, 'π' >> 8}, producerPageSafetyPrivateV3+17)
	r, _, e := p.Advance(context.Background(), 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	if e = p.VerifyPrefix(context.Background(), r, pageAllowancePrivateV3()); e != nil {
		t.Fatal(e)
	}
	if _, e = p.ReadObject(context.Background(), r, 0, pageAllowancePrivateV3()); !errors.Is(e, ErrPagesUnitPrivateV3) {
		t.Fatal("ASSERT_RETAINED_OVERSIZE_NOT_COMPUTATION_TRUNCATION", e)
	}
}
