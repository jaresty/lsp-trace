//go:build darwin || linux

package applicationinventory

import (
	"context"
	"encoding/json"
	"errors"
	"lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	tp "lsp-trace/internal/targetpacket"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Retained data is copied to a new Root. This instantiates controller state as
// data only: it does not authenticate an accepted head or revive a lifecycle.
func ownerCheckFixture(t *testing.T) (DurableStore, durableInventoryState) {
	t.Helper()
	dir := os.Getenv("LSP_TRACE_OWNER_FIXTURE")
	if dir == "" {
		t.Skip("external retained data-only fixture not configured")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "inventory-state-8eb0d15dc1fd3edd81cb6fd2dfe8e5ff15b0d760b1bb0cc86afabd725c4596e3.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v durableInventoryState
	if strictInventory(raw, &v) != nil {
		t.Fatal("fixture state")
	}
	dst := t.TempDir()
	if err := os.Chmod(dst, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dst)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	for _, set := range []struct {
		prefix string
		refs   []publication.StoredObject
	}{{"inventory-evidence-", v.Prepared}, {"inventory-description-", v.Descriptions}} {
		for _, r := range set.refs {
			if !r.Valid() {
				continue
			}
			raw, err = os.ReadFile(filepath.Join(dir, set.prefix+r.ID+".json"))
			if err != nil {
				t.Fatal(err)
			}
			got, e := publication.StoreObject(root, set.prefix, raw)
			if e != nil || got != r {
				t.Fatal("fixture object", e)
			}
		}
	}
	return DurableStore{Root: root, Directory: dst}, v
}
func ownerBudget(t *testing.T, n uint64) *sc.CallerBudgetV3 {
	t.Helper()
	b, e := sc.NewCallerBudgetV3(time.Now().Add(30*time.Second), n)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func ownerStore(t *testing.T, s DurableStore, prefix string, v any) publication.StoredObject {
	t.Helper()
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	r, e := publication.StoreObject(s.Root, prefix, raw)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func ownerEvidence(t *testing.T, s DurableStore, r publication.StoredObject) cat.SelectedEvidenceV3 {
	t.Helper()
	raw, e := publication.ReadObject(s.Root, "inventory-description-", r)
	if e != nil {
		t.Fatal(e)
	}
	var d cat.SelectedEvidenceV3
	if strictInventory(raw, &d) != nil {
		t.Fatal("description fixture")
	}
	return d
}

// The accepted state's references are held fixed; a newly hashed object is not
// authority to replace one of them.
func TestOwnerSelectedReferenceBinding(t *testing.T) {
	for _, kind := range []string{"corrupt", "swap", "unreferenced", "intrinsic"} {
		t.Run(kind, func(t *testing.T) {
			s, v := ownerCheckFixture(t)
			ref := v.Descriptions[0]
			path := filepath.Join(s.Directory, "inventory-description-"+ref.ID+".json")
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "corrupt":
				raw[len(raw)/2] ^= 1
			case "swap":
				raw, e = publication.ReadObject(s.Root, "inventory-description-", v.Descriptions[1])
				if e != nil {
					t.Fatal(e)
				}
			case "unreferenced":
				d := ownerEvidence(t, s, ref)
				d.Interpreter += "-unreferenced"
				ownerStore(t, s, "inventory-description-", d)
			case "intrinsic":
				d := ownerEvidence(t, s, ref)
				d.Request.RecordID = "invalid"
				v.Descriptions[0] = ownerStore(t, s, "inventory-description-", d)
			}
			if kind == "corrupt" || kind == "swap" {
				if e = os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
			}
			_, e = s.descriptions(context.Background(), v, ownerBudget(t, 64<<20))
			if kind == "unreferenced" {
				if e != nil {
					t.Fatal(e)
				}
			} else if !errors.Is(e, sc.ErrDurableDescribeV1) {
				t.Fatalf("intrinsic/hash boundary: %v", e)
			}
		})
	}
}

// This DTO is test data only; production retains the existing private reader.
type ownerPreparationFixture struct {
	Schema, RunID, LeadID            string
	Selected                         sc.PrivateSelectedEndpointV2
	Accounting                       inv.CompletedSelectedAccountingV1
	Unit                             tp.PrivateSourceUnit
	Request                          describerequest.Record
	Handoff, Checkpoint, Interpreter string
	Host                             dw.ResponseHostV2
	Limits                           dw.SuppliedResponseLimits
}

func ownerPreparation(t *testing.T, s DurableStore, r publication.StoredObject) ownerPreparationFixture {
	t.Helper()
	raw, e := publication.ReadObject(s.Root, "inventory-evidence-", r)
	if e != nil {
		t.Fatal(e)
	}
	var p ownerPreparationFixture
	if strictInventory(raw, &p) != nil {
		t.Fatal("preparation fixture")
	}
	return p
}
func TestOwnerHistoricalPreparationAvailability(t *testing.T) {
	for _, kind := range []string{"deleted", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			s, v := ownerCheckFixture(t)
			p := ownerPreparation(t, s, v.Prepared[0])
			path := filepath.Join(s.Directory, "inventory-evidence-"+v.Prepared[0].ID+".json")
			var e error
			if kind == "deleted" {
				e = os.Remove(path)
			} else {
				e = os.WriteFile(path, []byte("{}"), 0600)
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.descriptions(context.Background(), v, ownerBudget(t, 64<<20)); e != nil {
				t.Fatalf("ordinary reopen requires historical preparation: %v", e)
			}
			// Completion still needs the original preparation; refusal occurs before any
			// selected completion or response can be constructed.
			if _, e = sc.CompleteInventoryEvidence(context.Background(), s.Root, v.Prepared[0], v.RunID, v.Report.Census[0].ID, p.Interpreter, time.Now().Add(time.Second), ownerBudget(t, 64<<20), nil); !errors.Is(e, sc.ErrDurableDescribeV1) {
				t.Fatalf("completion preparation boundary: %v", e)
			}
		})
	}
}
func TestOwnerReopenBudgetBeforeEffect(t *testing.T) {
	s, v := ownerCheckFixture(t)
	total := uint64(len(v.Prepared))*1024 + 4096
	for _, r := range v.Descriptions {
		total += 6*uint64(r.Bytes) + 16384
	}
	for _, n := range []uint64{total - 1, total} {
		b := ownerBudget(t, n)
		_, e := s.descriptions(context.Background(), v, b)
		if (e == nil) != (n == total) {
			t.Fatalf("exact reopen budget: %d/%d: %v", n, total, e)
		}
		if e == nil {
			spent, _ := b.Usage()
			if spent != total {
				t.Fatalf("spent %d want %d", spent, total)
			}
		}
	}
}

func TestOwnerMixedReconciliationAndPending(t *testing.T) {
	s, v := ownerCheckFixture(t)
	ctx := context.Background()
	for i := range v.Report.Dispositions {
		v.Report.Dispositions[i].Status = "DESCRIBED_UNMATCHED"
		v.Report.Dispositions[i].Reason = "NO_SEMANTIC_MEMBERSHIP"
	}
	v.Report.WithMembership = 0
	v.Report.WithoutMembership = 3
	for _, status := range []string{"UNTOUCHED", "OBSERVED_EMPTY", "UNAVAILABLE", "PREPARATION_FAILED"} {
		id := "data-only-" + status
		v.Report.Census = append(v.Report.Census, Lead{ID: id, Provenance: "data-only"})
		v.Report.Dispositions = append(v.Report.Dispositions, Disposition{id, status, "preserved"})
		v.Prepared = append(v.Prepared, publication.StoredObject{})
	}
	v.Cursor = len(v.Prepared)
	v.Report.UntouchedGaps = append(v.Report.UntouchedGaps, "data-only explicit gap")
	b := ownerBudget(t, 64<<20)
	ds, e := s.descriptions(ctx, v, b)
	if e != nil {
		t.Fatal(e)
	}
	native := make([]cat.NativeBoundDescriptionV3, len(ds))
	for i := range ds {
		native[i] = ds[i]
	}
	c, e := cat.BuildNativeV3(ctx, b, v.Interpreter, native, v.Policy.ReviewMode)
	if e != nil {
		t.Fatal(e)
	}
	first, e := ds[0].Evidence(ctx)
	if e != nil {
		t.Fatal(e)
	}
	second, e := ds[1].Evidence(ctx)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(cat.NativeDecisionV3{Predecessor: c.Revision(), Actor: v.Interpreter, Reason: "data-only", Affected: []string{}, Groups: []cat.NativeGroupV3{{ID: "data-group", Label: "data-only", Reason: "fixture", Members: []string{first.ID, second.ID}}}})
	c, e = cat.ApplyNativeDecisionV3(ctx, c, raw, false)
	if e != nil {
		t.Fatal(e)
	}
	expected := v.Report
	expected.Dispositions = append([]Disposition(nil), v.Report.Dispositions...)
	expected.WithMembership = 2
	expected.WithoutMembership = 1
	expected.CatalogRevision = c.Revision()
	for i := 0; i < 2; i++ {
		expected.Dispositions[i].Status = "DESCRIBED_WITH_MEMBERSHIP"
		expected.Dispositions[i].Reason = "SUPPLIED_SEMANTIC_PROPOSAL_REVIEWED_UNAUTHENTICATED"
	}
	report, _, e := reconcileInventory(ctx, b, v.Report, ds, c)
	if e != nil || !reflect.DeepEqual(report, expected) {
		t.Fatalf("complete mixed report differs: %v\ngot=%+v\nwant=%+v", e, report, expected)
	}
	// An outstanding preparation at Cursor is pending, not described or abstained.
	pending := v
	pending.Report.Dispositions = append([]Disposition(nil), v.Report.Dispositions...)
	pending.Report.Dispositions[2] = Disposition{v.Report.Census[2].ID, "PREPARED", "DESCRIBE_PENDING"}
	pending.Cursor = 2
	pending.Descriptions = v.Descriptions[:2]
	pending.Report.SuccessfulDescriptions = 2
	pending.Report.WithoutMembership = 2
	got, e := s.descriptions(ctx, pending, ownerBudget(t, 64<<20))
	if e != nil || len(got) != 2 || pending.Report.Dispositions[2].Status != "PREPARED" {
		t.Fatal("pending preparation misclassified", e, len(got))
	}
}

func TestOwnerSharedBodyDataIdentities(t *testing.T) {
	for _, distinct := range []bool{false, true} {
		t.Run(map[bool]string{false: "shared-unit-request", true: "distinct-unit-request"}[distinct], func(t *testing.T) {
			s, v := ownerCheckFixture(t)
			p := ownerPreparation(t, s, v.Prepared[0])
			d := ownerEvidence(t, s, v.Descriptions[0])
			originalBody := p.Unit.Body
			p.LeadID = v.Report.Census[1].ID
			p.Host.AttemptID += "-independent"
			p.Host.MessageID += "-independent"
			if distinct {
				p.Unit.ID += "-independent"
				p.Unit.Revision = ""
				p.Unit.SessionID = ""
				p.Unit.DocumentVersion = ""
				var e error
				p.Request, e = describerequest.FormatPrivateSourceRequestCandidate(p.Unit, 30000, 8<<20)
				if e != nil {
					t.Fatal(e)
				}
				p.Host.RequestRecordID = p.Request.RecordID
				p.Host.Pins.PromptSHA256 = "sha256:" + p.Request.Envelope.InputSHA256
				p.Host.Provenance.PacketID = p.Unit.ID
				p.Host.Provenance.RequestLineageIdentity = p.Request.Lineage.LineageIdentity
			}
			record, e := dw.ParseResponseRecordV2(d.Record)
			if e != nil {
				t.Fatal(e)
			}
			semantic, _ := json.Marshal(record.Semantic())
			record, e = dw.NewResponseRecordV2(semantic, p.Host)
			if e != nil {
				t.Fatal(e)
			}
			d.ID = record.ID()
			d.Record, e = record.Bytes()
			if e != nil {
				t.Fatal(e)
			}
			d.Unit = p.Unit
			d.Request = p.Request
			v.Prepared[1] = ownerStore(t, s, "inventory-evidence-", p)
			v.Descriptions[1] = ownerStore(t, s, "inventory-description-", d)
			if p.Unit.Body != originalBody {
				t.Fatal("shared body changed")
			}
			ds, e := s.descriptions(context.Background(), v, ownerBudget(t, 64<<20))
			if e != nil || len(ds) != 3 {
				t.Fatal("data-only independent identities refused", e)
			}
			t.Log("data-only formatter/response validation and owner reopen; NOT native admission or accepted-head proof")
		})
	}
}
func TestOwnerQuoteRegression(t *testing.T) {
	limits := dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}
	refs := make([]publication.StoredObject, 3)
	payloads := make([]uint64, 3)
	for i := range refs {
		refs[i] = publication.StoredObject{ID: strings.Repeat("a", 64), Bytes: 1024}
		payloads[i] = 65536 + 4096
	}
	q, e := quoteDurableCompletion(refs, 4096, 32768, limits, payloads)
	if e != nil {
		t.Fatal(e)
	}
	// Exact pre-change staged quotes, independently captured from the baseline
	// source overlay. Only final Describe and Review gain the owner traversal.
	before := []uint64{1939969, 1939969, 9002498, 8833793, 21097538, 8079360}
	added := uint64(3*1024 + 4096)
	for i := range before {
		want := before[i]
		if i == 2 || i == 4 {
			want += added
		}
		if q.Stages[i] != want {
			t.Fatalf("owner quote stage %d got=%d want=%d", i, q.Stages[i], want)
		}
	}
	total, ok := capacitySum(q.Stages...)
	if !ok {
		t.Fatal("scalar quote overflow")
	}
	t.Logf("scalar three-item quotes before=50893127 after=%d owner-per-pass=%d; excludes preparation/delivery", total, added)
	refs = append(refs, refs[0])
	payloads = append(payloads, payloads[0])
	if _, e = quoteDurableCompletion(refs, 4096, 32768, limits, payloads); e != nil {
		t.Fatal("four-item scalar quote fits without historical deep audit; not full admission", e)
	}
}
func TestOwnerRetainedQuoteEnvelope(t *testing.T) {
	dir := os.Getenv("LSP_TRACE_OWNER_FIXTURE")
	if dir == "" {
		t.Skip("external retained data-only fixture not configured")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "inventory-run-2e8d5a62e7e7f611898d6a188ea322e591d2654f42e4f4490934c87dbaa012ef.json"))
	if e != nil {
		t.Fatal(e)
	}
	var envelope struct{ Content durableInventoryHead }
	if json.Unmarshal(raw, &envelope) != nil {
		t.Fatal("retained scalar head")
	}
	h := envelope.Content
	p := h.Completion
	refs := make([]publication.StoredObject, len(p.Inputs))
	for i, n := range p.Inputs {
		refs[i] = publication.StoredObject{ID: strings.Repeat("a", 64), Bytes: n}
	}
	for _, kind := range []string{"initial-max-adapter", "retained-staged-adapter"} {
		var adapters []int
		if kind == "retained-staged-adapter" {
			adapters = p.Adapters
		}
		q, e := quoteDurableCompletionBounds(refs, p.InitialStateBytes, p.Manifest, p.Limits, p.Payloads, adapters)
		if e != nil {
			t.Logf("%s: quote refused before effects", kind)
			continue
		}
		sum, ok := capacitySum(q.Stages...)
		if !ok {
			t.Fatal("quote cap")
		}
		t.Logf("%s: stages=%v quote=%d plus retained preparation=%d lower-bound=%d limit=%d (excludes delivery/escrow; not lifecycle admission)", kind, q.Stages, sum, p.PreparationUsed, sum+p.PreparationUsed, durableRunLimit)
		if kind == "initial-max-adapter" {
			// Scalar projection only. Initial delivery index upper is conservative;
			// exact stored-index accounting is reported separately by the fixture audit.
			initial := durableInventoryHead{Phase: "Describe", Sequence: 1, Completion: q}
			escrow, packets, err := successorEscrow(initial)
			if err != nil {
				t.Fatal(err)
			}
			delivery, currentPackets, err := chunkDeliveryQuote(p.Inputs[0], int(chunkIndexUpper(p.Inputs[0])), 0)
			if err != nil {
				t.Fatal(err)
			}
			total := p.PreparationUsed + sum + delivery + escrow
			t.Logf("initial full upper-index projection: preparation=%d completion=%d current-delivery=%d successor-escrow=%d total=%d work=%d packets=%d byte-limit=%d; no fresh lifecycle acceptance", p.PreparationUsed, sum, delivery, escrow, total, total*32, uint64(len(q.Stages))+packets+currentPackets, durableRunLimit)
		}
	}
}

func TestOwnerTraversalCardinality(t *testing.T) {
	s, v := ownerCheckFixture(t)
	for _, name := range []string{"lead", "cursor", "extra-description", "pending", "unprepared"} {
		t.Run(name, func(t *testing.T) {
			c := v
			c.Report.Dispositions = append([]Disposition(nil), v.Report.Dispositions...)
			c.Prepared = append([]publication.StoredObject(nil), v.Prepared...)
			switch name {
			case "lead":
				c.Report.Dispositions[0].LeadID = "wrong"
			case "cursor":
				c.Cursor = 0
			case "extra-description":
				c.Descriptions = append(append([]publication.StoredObject(nil), v.Descriptions...), v.Descriptions[0])
			case "pending":
				c.Report.Dispositions[0].Status = "PREPARED"
			case "unprepared":
				c.Prepared[0] = publication.StoredObject{}
			}
			if _, e := s.descriptions(context.Background(), c, ownerBudget(t, 64<<20)); e != ErrDurableInventory {
				t.Fatalf("owner traversal guard: %v", e)
			}
		})
	}
}
