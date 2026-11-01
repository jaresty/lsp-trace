//go:build darwin

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"lsp-trace/internal/publication"
)

// Test-only fixture bridge. A is the only original capture producer; B receives
// no original enumeration, Manager, guard or capture handles through this seam.
func ControlledRetainedCheckpointExportPrivateV3(t *testing.T, path string) {
	t.Helper()
	ctx := context.Background()
	grant := retainedProcessGrantV3()
	p, r := ControlledTwentyPagesPrivateV3(t, grant)
	raw, e := p.ExportRetainedCheckpointPrivateV3(ctx, r, Digest([]byte("controlled-retained-continuation")), grant)
	if e != nil {
		t.Fatal(e)
	}
	x := retainedProcessEnvelopeV3{Manifest: raw, Spec: p.state.frozen.state.spec, LedgerPath: p.state.ledger.state.path}
	for i, c := range p.state.captures {
		for j, b := range [][]byte{c.state.seeds, c.state.origin, c.state.artifact, p.state.origin.state.batches[i].binding.Input} {
			x.Expected[i][j] = Digest(b)
		}
	}
	out, _ := json.Marshal(x)
	if e = os.WriteFile(path, out, 0600); e != nil {
		t.Fatal(e)
	}
	f, e := os.OpenFile(path, os.O_WRONLY, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Sync(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	owner, _ := ControlledRetainedCheckpointOpenPrivateV3(t, path)
	reader, e := owner.SelectedBatchReader(ctx, 0, grant)
	if e != nil {
		t.Fatal(e)
	}
	progress, empty, e := reader.Advance(ctx, 1, 1, grant)
	if e != nil || progress.Status != "PAUSED_PAGE_BOUND" || progress.VerifiedPages != 1 {
		t.Fatal("ASSERT_RETAINED_PROCESS_A_PARTIAL_SELECTED_PAUSE", progress, e)
	}
	if _, e = empty.Read(ctx, grant); e == nil {
		t.Fatal("ASSERT_RETAINED_PARTIAL_NOT_COMPLETE")
	}
	next, e := reader.Checkpoint(ctx, Digest([]byte("partial-selected-cross-process")), grant)
	if e != nil {
		t.Fatal(e)
	}
	ControlledReplaceRetainedCheckpointManifestPrivateV3(t, path, next, owner)
	t.Log("ASSERT_RETAINED_PROCESS_A_PARTIAL_SELECTED_PAUSE: PASS; one selected page checkpointed")
}

// Test-only paired-byte inspection after a later source refusal. This observes
// retained historical bytes; it cannot mint a prepared/current-source owner.
func ControlledReadRetainedReferenceAfterFailurePrivateV3(t *testing.T, owner RetainedPageCheckpointOwnerPrivateV3, id string, grant PageCostPrivateV3) []byte {
	t.Helper()
	raw, e := owner.state.readPair(context.Background(), id, owner.state.manifest.Revision.Policy.ComputationUnitBytes, &grant)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func ControlledReplaceRetainedCheckpointManifestPrivateV3(t *testing.T, path string, raw []byte, owner RetainedPageCheckpointOwnerPrivateV3) {
	t.Helper()
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var x retainedProcessEnvelopeV3
	if json.Unmarshal(before, &x) != nil {
		t.Fatal("controlled envelope")
	}
	x.Manifest = raw
	out, _ := json.Marshal(x)
	if e = os.WriteFile(path, out, 0600); e != nil {
		t.Fatal(e)
	}
	if e = owner.state.ledger.Close(); e != nil {
		t.Fatal(e)
	}
}
func ControlledRetainedCheckpointOpenPrivateV3(t *testing.T, path string) (RetainedPageCheckpointOwnerPrivateV3, [20][4]string) {
	t.Helper()
	ctx := context.Background()
	grant := retainedProcessGrantV3()
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if len(raw) > 128<<10 {
		t.Fatal("controlled transfer bound")
	}
	var x retainedProcessEnvelopeV3
	if e = json.Unmarshal(raw, &x); e != nil {
		t.Fatal(e)
	}
	var m RetainedPageCheckpointManifestPrivateV3
	if canonicalRetainedPrivateV3(x.Manifest, &m) != nil {
		t.Fatal("controlled manifest")
	}
	l, e := openRetainedTestLedgerPrivateV3(ctx, x.LedgerPath, m.Application, grant, m.LedgerGeneration, m.LedgerTip)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = l.Close() })
	f, e := FreezeRetainedCheckpointSourcePrivateV3(ctx, x.Spec, l, grant)
	if e != nil {
		t.Fatal(e)
	}
	var outputs [2]*publication.ContentRetentionRoot
	for i, v := range m.Roots {
		outputs[i], e = publication.OpenContentRetentionRoot(ctx, v.Path)
		if e != nil {
			t.Fatal(e)
		}
		r := outputs[i]
		t.Cleanup(func() { _ = r.Close() })
	}
	source, e := publication.OpenSourceProtectionRootPrivateV2(ctx, x.Spec.Root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = source.Close() })
	roots, e := NewProducerPacketRootsPrivateV2(ctx, outputs, source, nil)
	if e != nil {
		t.Fatal(e)
	}
	owner, e := AdmitRetainedPageCheckpointPrivateV3(ctx, x.Manifest, f, roots, l, RetainedCheckpointDisclosurePrivateV3{m.Application, m.Revision.Policy.ConfigSHA256, m.Intent, RetainedCheckpointPreparationRevisionPrivateV3}, grant)
	if e != nil {
		t.Fatal(e)
	}
	return owner, x.Expected
}
