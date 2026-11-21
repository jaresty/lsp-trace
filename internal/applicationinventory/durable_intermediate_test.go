//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	tp "lsp-trace/internal/targetpacket"
)

// Pure canonical data through production intrinsic readers. No opaque native
// fixture, Start, reserve, task, Read, Submit, Result or timed proof is invoked.
func intermediateData(t *testing.T, n int) (DurableStore, durableInventoryState) {
	t.Helper()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	s := DurableStore{root, dir}
	v := durableInventoryState{Schema: "lsp-trace.inventory-state.v1", RunID: strings.Repeat("a", 64), Interpreter: "interpreter", Reviewer: "interpreter", Report: initialInventoryReport()}
	v.Policy.ReviewMode = cat.ReviewModeSameThreadV3
	v.Report.ReviewMode = cat.ReviewModeSameThreadV3
	unit := tp.PrivateSourceUnit{Version: tp.PrivateSourceVersion, ClaimCeiling: tp.PrivateSourceCeiling, ID: "shared-unit", Body: "shared source", SourceDigest: cat.NativeDigestV3([]byte("shared source"))}
	req, e := describerequest.FormatPrivateSourceRequestCandidate(unit, 1000, 8<<20)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < n; i++ {
		id := string(rune('a' + i))
		d := cat.SelectedEvidenceV3{ID: id, HandoffID: "data", CheckpointID: "data", Interpreter: v.Interpreter, Unit: unit, Request: req, Record: json.RawMessage(`{"data":"only"}`)}
		v.Descriptions = append(v.Descriptions, ownerStore(t, s, "inventory-description-", d))
		v.Prepared = append(v.Prepared, publication.StoredObject{ID: strings.Repeat("b", 64), Bytes: 1})
		v.Report.Census = append(v.Report.Census, Lead{ID: id})
		v.Report.Dispositions = append(v.Report.Dispositions, Disposition{id, "DESCRIBED_UNMATCHED", "NO_SEMANTIC_MEMBERSHIP"})
	}
	v.Cursor = n
	v.Report.SuccessfulDescriptions = n
	v.Report.WithoutMembership = n
	return s, v
}
func legacyIntermediateBase(t *testing.T, s DurableStore, v durableInventoryState, b *cat.CallerBudgetV3) cat.NativeCatalogV3 {
	t.Helper()
	ds, e := s.descriptions(context.Background(), v, b)
	if e != nil {
		t.Fatal(e)
	}
	ns := make([]cat.NativeBoundDescriptionV3, len(ds))
	for i := range ds {
		ns[i] = ds[i]
	}
	c, e := cat.BuildNativeV3(context.Background(), b, v.Interpreter, ns, v.Policy.ReviewMode)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func decisionIntermediate(w cat.NativeCatalogWireV3) []byte {
	raw, _ := json.Marshal(cat.NativeDecisionV3{Predecessor: w.Predecessor, Actor: w.Actor, Reason: w.Reason, Affected: w.Affected, Groups: w.Groups})
	return raw
}
func assertIntermediateInput(t *testing.T, s DurableStore, c cat.IntermediateCatalogV3, p intermediateInventoryCatalog, b *cat.CallerBudgetV3) {
	t.Helper()
	ctx := context.Background()
	if e := s.materializeIntermediateInput(ctx, c, p, b); e != nil {
		t.Fatal(e)
	}
	h := durableInventoryHead{RunID: strings.Repeat("a", 64), TaskID: strings.Repeat("b", 64), Phase: "GroupDecision", Input: p.Input, InputPrefix: "inventory-catalog-"}
	if e := s.buildDelivery(ctx, &h, b); e != nil {
		t.Fatal(e)
	}
	index, e := s.readChunkIndex(h)
	if e != nil || index.Original != p.Input || index.Original == p.Record {
		t.Fatal("semantic/chunk identity", e)
	}
	var joined []byte
	for _, ref := range index.Chunks {
		raw, e := publication.ReadObject(s.Root, "inventory-chunk-", ref)
		if e != nil {
			t.Fatal(e)
		}
		joined = append(joined, raw...)
	}
	full, e := c.Catalog().Bytes(ctx)
	if e != nil || !bytes.Equal(joined, full) {
		t.Fatal("full agent input", e)
	}
	before := inventoryAcceptance(h)
	h.Input = p.Record
	if bytes.Equal(before, inventoryAcceptance(h)) {
		t.Fatal("semantic input absent from acceptance")
	}
}
func TestIntermediateControllerRetainedParity(t *testing.T) {
	s, v := ownerCheckFixture(t)
	ctx := context.Background()
	b := ownerBudget(t, 64<<20)
	lb := ownerBudget(t, 64<<20)
	c, e := s.intermediateOwner(ctx, v, b)
	if e != nil {
		t.Fatal(e)
	}
	legacy := legacyIntermediateBase(t, s, v, lb)
	refs := []publication.StoredObject{v.Base, v.Proposed, v.Corrected}
	compactTotal := 0
	fullTotal := 0
	var catalogs [][]byte
	for i, ref := range refs {
		raw, e := os.ReadFile(filepath.Join(os.Getenv("LSP_TRACE_OWNER_FIXTURE"), "inventory-catalog-"+ref.ID+".json"))
		if e != nil {
			t.Fatal(e)
		}
		var w cat.NativeCatalogWireV3
		if strictInventory(raw, &w) != nil {
			t.Fatal("retained wire")
		}
		if i > 0 {
			c, e = applyIntermediateInventoryDecision(ctx, v, c, decisionIntermediate(w), i == 2, b)
			if e != nil {
				t.Fatal(e)
			}
			legacy, e = applyInventoryDecision(ctx, lb, legacy, decisionIntermediate(w), i == 2, v.Reviewer)
			if e != nil {
				t.Fatal(e)
			}
		}
		got, e := c.Catalog().Bytes(ctx)
		want, le := legacy.Bytes(ctx)
		if e != nil || le != nil || !bytes.Equal(got, raw) || !bytes.Equal(got, want) || c.Catalog().Revision() != legacy.Revision() {
			t.Fatal("full retained parity", i, e, le)
		}
		search, e := c.Catalog().SearchGroups(ctx, "", 100)
		oldSearch, le := legacy.SearchGroups(ctx, "", 100)
		if e != nil || le != nil || !reflect.DeepEqual(search, oldSearch) {
			t.Fatal("search parity")
		}
		p, e := s.retainIntermediate(ctx, c, b)
		if e != nil || p.Input != ref || p.Record == p.Input {
			t.Fatal("stored vs input identity", i, e)
		}
		compactTotal += p.Record.Bytes
		fullTotal += p.Input.Bytes
		catalogs = append(catalogs, got)
		reopened, e := s.reopenIntermediate(ctx, v, p, []string{"GroupDecision", "Reviewer", "Published"}[i], ownerBudget(t, 64<<20))
		if e != nil {
			t.Fatal("controller reopen", e)
		}
		reopenedRaw, e := reopened.Catalog().Bytes(ctx)
		if e != nil || !bytes.Equal(reopenedRaw, raw) {
			t.Fatal("controller bytes", e)
		}
		assertIntermediateInput(t, s, c, p, b)
	}
	// Exercise the existing four-full-artifact publisher, not a new ref publisher.
	canonicalParent, e := filepath.EvalSymlinks(s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	published, e := publishInventory(ctx, Request{Destination: filepath.Join(canonicalParent, "candidate-publication")}, ownerBudget(t, 64<<20), v.Report, v.ManifestCost, catalogs[0], catalogs[1], catalogs[2])
	if e != nil || !published.Published {
		t.Fatal("publication", e)
	}
	manifestRaw, e := os.ReadFile(filepath.Join(s.Directory, "candidate-publication", "manifest.json"))
	var manifest PublicationManifestV3
	if e != nil || strictInventory(manifestRaw, &manifest) != nil || len(manifest.Artifacts) != 4 {
		t.Fatal("four artifacts", e)
	}
	for _, artifact := range manifest.Artifacts {
		raw, e := os.ReadFile(filepath.Join(s.Directory, "candidate-publication", artifact.Name))
		if e != nil || len(raw) != artifact.Bytes || cat.NativeDigestV3(raw) != artifact.SHA256 {
			t.Fatal("publication readback", e)
		}
		if artifact.Name == "catalog-corrected.json" && artifact.SHA256 != "sha256:"+v.Corrected.ID {
			t.Fatal("corrected semantic input identity")
		}
	}
	used, _ := b.Usage()
	oldUsed, _ := lb.Usage()
	t.Logf("scripted retained parity: groups 0,2,1; compact-record bytes=%d full semantic bytes=%d candidate build/apply/Bytes/search/retain/task-materialization+chunks reservations=%d legacy build/apply/Bytes/search reservations=%d (different operation sets; not a savings claim)", compactTotal, fullTotal, used, oldUsed)
}
func TestIntermediateControllerReviewerBinding(t *testing.T) {
	s, v := intermediateData(t, 0)
	ctx := context.Background()
	b := ownerBudget(t, 64<<20)
	c, e := s.intermediateOwner(ctx, v, b)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(cat.NativeDecisionV3{Predecessor: c.Catalog().Revision(), Actor: v.Interpreter, Reason: "proposal", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
	c, e = applyIntermediateInventoryDecision(ctx, v, c, raw, false, b)
	if e != nil {
		t.Fatal(e)
	}
	v.Reviewer = "assigned-reviewer"
	raw, _ = json.Marshal(cat.NativeDecisionV3{Predecessor: c.Catalog().Revision(), Actor: v.Interpreter, Reason: "unassigned review", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
	if _, e = applyIntermediateInventoryDecision(ctx, v, c, raw, true, b); e != ErrInventoryV3 {
		t.Fatal("unassigned reviewer accepted", e)
	}
	raw, _ = json.Marshal(cat.NativeDecisionV3{Predecessor: c.Catalog().Revision(), Actor: v.Reviewer, Reason: "assigned review", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
	if _, e = applyIntermediateInventoryDecision(ctx, v, c, raw, true, b); e != nil {
		t.Fatal("rejected actor consumed successor", e)
	}
}

func TestIntermediateControllerSharedBodyIdentities(t *testing.T) {
	for _, distinct := range []bool{false, true} {
		s, v := intermediateData(t, 2)
		if distinct {
			d := ownerEvidence(t, s, v.Descriptions[1])
			d.Unit.ID += "-independent"
			var e error
			d.Request, e = describerequest.FormatPrivateSourceRequestCandidate(d.Unit, 1000, 8<<20)
			if e != nil {
				t.Fatal(e)
			}
			v.Descriptions[1] = ownerStore(t, s, "inventory-description-", d)
		}
		c, e := s.intermediateOwner(context.Background(), v, ownerBudget(t, 64<<20))
		if e != nil {
			t.Fatal(e)
		}
		a, b := c.Catalog().MembershipCounts()
		if a != 0 || b != 2 {
			t.Fatal("identity denominator")
		}
		legacy := legacyIntermediateBase(t, s, v, ownerBudget(t, 64<<20))
		x, _ := c.Catalog().Bytes(context.Background())
		y, _ := legacy.Bytes(context.Background())
		if !bytes.Equal(x, y) {
			t.Fatal("shared body identity parity")
		}
	}
}

func TestIntermediateControllerMixedPendingZero(t *testing.T) {
	for _, n := range []int{0, 2} {
		t.Run(string(rune('0'+n)), func(t *testing.T) {
			s, v := intermediateData(t, n)
			ctx := context.Background()
			for _, status := range []string{"UNTOUCHED", "UNAVAILABLE", "OBSERVED_EMPTY", "PREPARATION_FAILED"} {
				v.Report.Census = append(v.Report.Census, Lead{ID: status})
				v.Report.Dispositions = append(v.Report.Dispositions, Disposition{status, status, "preserved"})
				v.Prepared = append(v.Prepared, publication.StoredObject{})
			}
			v.Cursor = len(v.Prepared)
			v.Report.UntouchedGaps = []string{"explicit-gap"}
			b := ownerBudget(t, 64<<20)
			c, e := s.intermediateOwner(ctx, v, b)
			if e != nil {
				t.Fatal(e)
			}
			full := legacyIntermediateBase(t, s, v, ownerBudget(t, 64<<20))
			a, _ := c.Catalog().Bytes(ctx)
			z, _ := full.Bytes(ctx)
			if !bytes.Equal(a, z) {
				t.Fatal("mixed parity")
			}
			ds, e := s.descriptions(ctx, v, b)
			if e != nil {
				t.Fatal(e)
			}
			got, _, e := reconcileInventory(ctx, b, v.Report, ds, c.Catalog())
			want, _, le := reconcileInventory(ctx, ownerBudget(t, 64<<20), v.Report, ds, full)
			if e != nil || le != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("reconciliation", e, le)
			}
			if !reflect.DeepEqual(got.Census, v.Report.Census) || !reflect.DeepEqual(got.UntouchedGaps, v.Report.UntouchedGaps) {
				t.Fatal("denominator/gaps")
			}
			p, e := s.retainIntermediate(ctx, c, b)
			if e != nil {
				t.Fatal(e)
			}
			assertIntermediateInput(t, s, c, p, b)
			v.Prepared = append(v.Prepared, publication.StoredObject{ID: strings.Repeat("c", 64), Bytes: 1})
			v.Report.Census = append(v.Report.Census, Lead{ID: "pending"})
			v.Report.Dispositions = append(v.Report.Dispositions, Disposition{"pending", "PREPARED", "DESCRIBE_PENDING"})
			if _, e = s.intermediateOwner(ctx, v, ownerBudget(t, 64<<20)); e != nil || v.Report.Dispositions[len(v.Report.Dispositions)-1].Status != "PREPARED" {
				t.Fatal("pending misclassified", e)
			}
		})
	}
}

// This is a conservative DATA-ONLY candidate envelope, not an installed quote,
// admission proof, ledger rewrite or assertion that these bounds are minimal.
func TestIntermediateInitialProjection(t *testing.T) {
	limits := dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}
	refs := []publication.StoredObject{{ID: strings.Repeat("a", 64), Bytes: 8391}, {ID: strings.Repeat("b", 64), Bytes: 8382}, {ID: strings.Repeat("c", 64), Bytes: 8385}}
	q, e := quoteDurableCompletion(refs, 3675, 43924, limits, []uint64{70681, 70678, 70679})
	if e != nil {
		t.Fatal(e)
	}
	stages, ok := capacitySum(q.Stages...)
	if !ok {
		t.Fatal("stages")
	}
	delivery, packets, e := chunkDeliveryQuote(8391, 410, 0)
	if e != nil {
		t.Fatal(e)
	}
	escrow, ep, e := successorEscrow(durableInventoryHead{Phase: "Describe", Sequence: 1, Completion: q})
	if e != nil {
		t.Fatal(e)
	}
	initial := uint64(12662532) + stages + delivery + escrow
	if initial != 66082293 {
		t.Fatalf("baseline changed: %d", initial)
	}
	// Existing description upper: input + maximum adapter + fixed metadata.
	d := uint64(0)
	for _, ref := range refs {
		d += uint64(ref.Bytes + limits.MaxAdapterBytes + 2048)
	}
	snapshot := 8*d + 8192*uint64(len(refs)) + 4096
	base, grown := uint64(q.BaseUpper), uint64(q.ProposedUpper)
	full := []uint64{base, grown, grown}
	var retainAndInput uint64
	for _, n := range full {
		// Store 16n+16384, retain hash 2n+4096; materialize full input
		// 8n+16384 plus an additional full Bytes copy n+1.
		retainAndInput += 27*n + 36865
	}
	// Reopen reconstructs its owner from selected descriptions. The old final
	// Describe stage bounds a base rebuild (deliberately includes unused work).
	// The old Group stage bounds the additional full seal at each reopen.
	// Compact DTO upper: full bytes plus <=128 per ref and closed outer keys.
	var compactRead uint64
	for _, n := range []uint64{base, grown} {
		compactRead += 8*(n+128*uint64(len(refs))+512) + 2*n + 16384
	}
	extra := 3*snapshot + retainAndInput + 2*q.Stages[2] + 2*q.Stages[3] + compactRead
	total := initial + extra
	t.Logf("full initial legacy=%d snapshot-each=%d retain/full-input=%d owner-rebuild-upper=%d extra-reseal-upper=%d compact-read-upper=%d candidate-conservative-total=%d excess=%d work=%d legacy-protected-packets=%d (candidate outer stages NOT installed; no fresh lifecycle)", initial, snapshot, retainAndInput, 2*q.Stages[2], 2*q.Stages[3], compactRead, total, total-durableRunLimit, total*32, uint64(len(q.Stages))+packets+ep)
	if total <= durableRunLimit {
		t.Fatal("candidate conservative costs unexpectedly fit")
	}
}

func TestIntermediateMatchedOperationCosts(t *testing.T) {
	s, v := ownerCheckFixture(t)
	ctx := context.Background()
	oldBudget, newBudget := ownerBudget(t, 64<<20), ownerBudget(t, 64<<20)
	old := legacyIntermediateBase(t, s, v, oldBudget)
	candidate, e := s.intermediateOwner(ctx, v, newBudget)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := oldBudget.Usage()
	b, _ := newBudget.Usage()
	t.Logf("matched base construction old=%d candidate=%d extra=%d", a, b, b-a)
	for i, ref := range []publication.StoredObject{v.Base, v.Proposed, v.Corrected} {
		if i > 0 {
			raw, e := os.ReadFile(filepath.Join(os.Getenv("LSP_TRACE_OWNER_FIXTURE"), "inventory-catalog-"+ref.ID+".json"))
			if e != nil {
				t.Fatal(e)
			}
			var w cat.NativeCatalogWireV3
			if strictInventory(raw, &w) != nil {
				t.Fatal("wire")
			}
			beforeOld, _ := oldBudget.Usage()
			beforeNew, _ := newBudget.Usage()
			old, e = applyInventoryDecision(ctx, oldBudget, old, decisionIntermediate(w), i == 2, v.Reviewer)
			if e != nil {
				t.Fatal(e)
			}
			candidate, e = applyIntermediateInventoryDecision(ctx, v, candidate, decisionIntermediate(w), i == 2, newBudget)
			if e != nil {
				t.Fatal(e)
			}
			a, _ = oldBudget.Usage()
			b, _ = newBudget.Usage()
			if a-beforeOld != b-beforeNew {
				t.Fatal("conservative seal charge changed")
			}
			t.Logf("matched transition %d old=%d candidate=%d", i, a-beforeOld, b-beforeNew)
		}
		beforeOld, _ := oldBudget.Usage()
		beforeNew, _ := newBudget.Usage()
		x, e := old.Bytes(ctx)
		y, ce := candidate.Catalog().Bytes(ctx)
		if e != nil || ce != nil || !bytes.Equal(x, y) {
			t.Fatal("Bytes")
		}
		a, _ = oldBudget.Usage()
		b, _ = newBudget.Usage()
		if a-beforeOld != b-beforeNew {
			t.Fatal("full Bytes charge")
		}
		beforeOld, _ = oldBudget.Usage()
		beforeNew, _ = newBudget.Usage()
		_, e = old.SearchGroups(ctx, "", 100)
		_, ce = candidate.Catalog().SearchGroups(ctx, "", 100)
		if e != nil || ce != nil {
			t.Fatal("search")
		}
		a, _ = oldBudget.Usage()
		b, _ = newBudget.Usage()
		if a-beforeOld != b-beforeNew {
			t.Fatal("full search charge")
		}
		beforeNew, _ = newBudget.Usage()
		selection, e := s.retainIntermediate(ctx, candidate, newBudget)
		if e != nil {
			t.Fatal(e)
		}
		b, _ = newBudget.Usage()
		t.Logf("revision %d compact retain=%d record=%d full=%d", i, b-beforeNew, selection.Record.Bytes, selection.Input.Bytes)
		rb := ownerBudget(t, 64<<20)
		_, e = s.reopenIntermediate(ctx, v, selection, []string{"GroupDecision", "Reviewer", "Published"}[i], rb)
		if e != nil {
			t.Fatal(e)
		}
		ru, _ := rb.Usage()
		stored, err := publication.StoreObject(s.Root, "inventory-catalog-", x)
		if err != nil || stored != ref {
			t.Fatal("legacy reopen fixture", err)
		}
		legacyReopenBudget := ownerBudget(t, 64<<20)
		if _, err = cat.ReopenStoredNativeV3(ctx, s.Root, ref, legacyReopenBudget, v.Interpreter, i == 2); err != nil {
			t.Fatal(err)
		}
		legacyReopenUsed, _ := legacyReopenBudget.Usage()
		mb := ownerBudget(t, 64<<20)
		materialized, err := s.reopenIntermediate(ctx, v, selection, []string{"GroupDecision", "Reviewer", "Published"}[i], mb)
		if err != nil {
			t.Fatal(err)
		}
		beforeMaterialize, _ := mb.Usage()
		assertIntermediateInput(t, s, materialized, selection, mb)
		mu, _ := mb.Usage()
		mu -= beforeMaterialize
		t.Logf("revision %d reopen old=%d candidate incl owner rebuild=%d full input+chunk helper=%d", i, legacyReopenUsed, ru, mu)
	}
}

func TestIntermediateControllerEvidenceGuards(t *testing.T) {
	for _, kind := range []string{"invalid-ref", "missing", "corrupt", "swap", "unreferenced", "wrong-input"} {
		t.Run(kind, func(t *testing.T) {
			s, v := intermediateData(t, 2)
			ctx := context.Background()
			c, e := s.intermediateOwner(ctx, v, ownerBudget(t, 64<<20))
			if e != nil {
				t.Fatal(e)
			}
			p, e := s.retainIntermediate(ctx, c, ownerBudget(t, 64<<20))
			if e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(s.Directory, "inventory-description-"+v.Descriptions[0].ID+".json")
			switch kind {
			case "invalid-ref":
				p.Record.ID = "invalid"
			case "missing":
				if e = os.Remove(path); e != nil {
					t.Fatal(e)
				}
			case "corrupt":
				if e = os.WriteFile(path, []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
			case "swap":
				raw, e := publication.ReadObject(s.Root, "inventory-description-", v.Descriptions[1])
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
			case "unreferenced":
				d := ownerEvidence(t, s, v.Descriptions[0])
				d.ID = "unreferenced"
				ownerStore(t, s, "inventory-description-", d)
			case "wrong-input":
				p.Input.ID = strings.Repeat("f", 64)
			}
			// Expected controller selection remains unchanged, even when a new object is rehashed.
			full, _ := c.Catalog().Bytes(ctx)
			expected := publication.StoredObject{ID: cat.NativeDigestV3(full)[7:], Bytes: len(full)}
			v.Base = expected // data-only expected controller state, never an accepted head
			_, e = s.reopenIntermediate(ctx, v, p, "GroupDecision", ownerBudget(t, 64<<20))
			if kind == "unreferenced" {
				if e != nil {
					t.Fatal("unreferenced object became authority", e)
				}
			} else if e == nil {
				t.Fatal("actual guard accepted", kind)
			}
		})
	}
}
