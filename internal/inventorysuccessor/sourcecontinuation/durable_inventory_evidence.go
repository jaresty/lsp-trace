//go:build darwin || linux

package sourcecontinuation

import (
	"context"
	"encoding/json"
	"fmt"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"reflect"
	"time"

	"lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
	tp "lsp-trace/internal/targetpacket"
)

type retainedInventoryEvidence struct {
	Schema, RunID, LeadID            string
	Selected                         PrivateSelectedEndpointV2
	Accounting                       inv.CompletedSelectedAccountingV1
	Unit                             tp.PrivateSourceUnit
	Request                          describerequest.Record
	Handoff, Checkpoint, Interpreter string
	Host                             dw.ResponseHostV2
	Limits                           dw.SuppliedResponseLimits
	OriginalReferences               *inv.InventoryReferenceWitnessV1 `json:",omitempty"`
}

// RetainInventoryEvidence consumes the original owner's one-shot durable handoff.
// It does not start a semantic window. All originals are retained while the
// original bounded native owner is still present; later tasks have no native state.
func RetainInventoryEvidence(ctx context.Context, root *publication.Root, run, lead string, r PrivateSelectedResultV2, b CallerDescribeBindingV3) (publication.StoredObject, uint64, error) {
	return retainInventoryEvidence(ctx, root, run, lead, r, b, false)
}

// V9 only: same one-shot original, not a second retention or refreshed admission.
func RetainInventoryReferenceEvidenceV1(ctx context.Context, root *publication.Root, run, lead string, r PrivateSelectedResultV2, b CallerDescribeBindingV3) (publication.StoredObject, uint64, error) {
	return retainInventoryEvidence(ctx, root, run, lead, r, b, true)
}
func retainInventoryEvidence(ctx context.Context, root *publication.Root, run, lead string, r PrivateSelectedResultV2, b CallerDescribeBindingV3, references bool) (publication.StoredObject, uint64, error) {
	if !durableID(run) || !smallSelectedV3(lead) || r.state == nil || !r.state.durableIssued.CompareAndSwap(false, true) {
		return publication.StoredObject{}, 0, ErrDurableDescribeV1
	}
	a, e := PrepareSelectedDescribeV3(ctx, r, b)
	if e != nil {
		return publication.StoredObject{}, 0, e
	}
	v := a.state
	recordUpper, e := inventoryRecordUpper(ctx, v.host, b.Limits.MaxAdapterBytes, b.Budget)
	if e != nil {
		return publication.StoredObject{}, 0, e
	}
	payload, ok := sourceResponseAdd(uint64(len(v.unit.Body)), uint64(len(v.request.Envelope.Prompt)), recordUpper)
	if !ok || payload > 8<<20 {
		return publication.StoredObject{}, 0, ErrDurableDescribeV1
	}
	p := retainedInventoryEvidence{"lsp-trace.inventory-evidence.v1", run, lead, v.selected, r.state.accounting, v.unit, v.request, v.handoff, v.checkpoint, b.Interpreter, v.host, b.Limits, nil}
	if references {
		w, err := r.state.original.InventoryReferencesV1(ctx, b.Budget, run, lead, v.unit, v.request.RecordID, v.selected.Intent.InputID)
		if err != nil {
			return publication.StoredObject{}, 0, err
		}
		p.Schema = "lsp-trace.inventory-evidence.v2"
		p.OriginalReferences = &w
		wr, _ := json.Marshal(w)
		payload += uint64(len(wr)) + 1024 // later actual admission-chain provenance
	}
	raw, e := json.Marshal(p)
	if references && (len(raw) > 32<<10 || payload > 64<<10) {
		return publication.StoredObject{}, 0, ErrDurableDescribeV1
	}
	if e != nil || b.Budget.Reserve(ctx, uint64(len(raw))*8+16384) != nil {
		return publication.StoredObject{}, 0, ErrDurableDescribeV1
	}
	ref, e := publication.StoreObject(root, "inventory-evidence-", raw)
	if e != nil {
		return publication.StoredObject{}, 0, e
	}
	encoded, e := publication.ReadObject(root, "inventory-evidence-", ref)
	var restored retainedInventoryEvidence
	if e != nil || durableStrict(encoded, &restored) != nil || !reflect.DeepEqual(p, restored) {
		return publication.StoredObject{}, 0, ErrDurableDescribeV1
	}
	return ref, payload, nil
}
func readInventoryEvidence(ctx context.Context, root *publication.Root, ref publication.StoredObject, run, lead string, budget *CallerBudgetV3) (retainedInventoryEvidence, error) {
	var p retainedInventoryEvidence
	if !ref.Valid() || budget.Reserve(ctx, uint64(ref.Bytes)*6+16384) != nil {
		return p, ErrDurableDescribeV1
	}
	raw, e := publication.ReadObject(root, "inventory-evidence-", ref)
	if e != nil || durableStrict(raw, &p) != nil || (p.Schema != "lsp-trace.inventory-evidence.v1" && p.Schema != "lsp-trace.inventory-evidence.v2") || p.RunID != run || p.LeadID != lead || p.Accounting.Head == "" || p.Accounting.Event != "OBSERVE" || p.Accounting.Disposition != "OBSERVED" || p.Accounting.Windows != 3 || p.Handoff == "" || p.Checkpoint == "" || !p.Limits.Valid() || !selectedSelectorBoundV2(p.Selected) || describerequest.ValidateRecord(p.Request) != nil || p.Unit.ID != p.Request.Envelope.AdmissionID || p.Host.RequestRecordID != p.Request.RecordID || p.Host.Pins.PromptSHA256 != "sha256:"+p.Request.Envelope.InputSHA256 {
		return p, ErrDurableDescribeV1
	}
	if p.Schema == "lsp-trace.inventory-evidence.v1" {
		if p.OriginalReferences != nil {
			return p, ErrDurableDescribeV1
		}
	} else {
		if budget.Reserve(ctx, 32*8192) != nil {
			return p, ErrDurableDescribeV1
		}
		if _, e := inv.ValidateInventoryReferenceEvidenceV1(raw, run, lead); e != nil {
			return p, e
		}
	}
	return p, nil
}
func CompleteInventoryEvidence(ctx context.Context, root *publication.Root, ref publication.StoredObject, run, lead, actor string, deadline time.Time, budget *CallerBudgetV3, answer []byte) (BoundSelectedDescriptionV3, error) {
	return completeInventoryEvidence(ctx, root, ref, publication.StoredObject{}, run, lead, actor, deadline, budget, answer)
}
func CompleteInventoryReferenceEvidenceV1(ctx context.Context, root *publication.Root, ref, admission publication.StoredObject, run, lead, actor string, deadline time.Time, budget *CallerBudgetV3, answer []byte) (BoundSelectedDescriptionV3, error) {
	return completeInventoryEvidence(ctx, root, ref, admission, run, lead, actor, deadline, budget, answer)
}
func completeInventoryEvidence(ctx context.Context, root *publication.Root, ref, admission publication.StoredObject, run, lead, actor string, deadline time.Time, budget *CallerBudgetV3, answer []byte) (BoundSelectedDescriptionV3, error) {
	p, e := readInventoryEvidence(ctx, root, ref, run, lead, budget)
	if e != nil || p.Interpreter != actor {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	var provenance *cat.NativeInventoryAdmissionV1
	if p.OriginalReferences != nil {
		if !admission.Valid() || admission.Bytes > 16384 || budget.Reserve(ctx, 32*16384+32768) != nil {
			return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
		}
		raw, e := publication.ReadObject(root, "inventory-admission-", admission)
		var n struct {
			Schema, RunID, LeadID string
			Index                 int
			Previous, Evidence    publication.StoredObject
			Binding               struct {
				Index                               int
				Provenance, RequestID, SourceUnitID string
			}
			Payload, AdmittedEvidenceBytes uint64
		}
		if e != nil || durableStrict(raw, &n) != nil || n.Schema != "lsp-trace.inventory-admission.v3" || n.RunID != run || n.LeadID != lead || n.Index < 0 || n.Index > 1 || n.Binding.Index != n.Index || lead != fmt.Sprintf("%s-%06d", run, n.Index) || n.Evidence != ref || n.Binding.RequestID != p.Request.RecordID || n.Binding.SourceUnitID != p.Unit.ID {
			return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
		}
		provenance = &cat.NativeInventoryAdmissionV1{RunID: run, LeadID: lead, AdmissionSHA256: "sha256:" + admission.ID, EvidenceSHA256: "sha256:" + ref.ID, WitnessSHA256: p.OriginalReferences.Digest}
	} else if admission != (publication.StoredObject{}) {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	b := CallerDescribeBindingV3{AttemptID: p.Host.AttemptID, Interpreter: p.Interpreter, Pins: p.Host.Pins, Deadline: deadline, Limits: p.Limits, Budget: budget}
	a := SelectedDescribeV3{&selectedDescribeStateV3{selected: p.Selected, unit: p.Unit, request: p.Request, handoff: p.Handoff, checkpoint: p.Checkpoint, binding: b, host: p.Host}}
	d, e := CompleteSelectedDescribeV3(ctx, a, answer)
	if e != nil || p.OriginalReferences == nil {
		return d, e
	}
	if budget.Reserve(ctx, 4*uint64(len(d.state.raw))+32*8192) != nil {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	d.state.evidence.InventoryAdmission = provenance
	d.state.evidence.OriginalReferences, e = json.Marshal(p.OriginalReferences)
	if e != nil {
		return BoundSelectedDescriptionV3{}, e
	}
	d.state.raw, e = json.Marshal(d.state.evidence)
	if len(d.state.raw) > ref.Bytes+len(answer)+2048 {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	return d, e
}

// ReopenInventoryDescription restores data-only semantic custody from the owned
// store, never native admission and never caller-supplied DTO bytes.
func ReopenInventoryDescription(ctx context.Context, root *publication.Root, ref publication.StoredObject, budget *CallerBudgetV3) (BoundSelectedDescriptionV3, error) {
	return reopenInventoryDescription(ctx, root, ref, budget, "", "", nil)
}
func ReopenInventoryDescriptionForRunV1(ctx context.Context, root *publication.Root, ref publication.StoredObject, budget *CallerBudgetV3, run, lead string) (BoundSelectedDescriptionV3, error) {
	return reopenInventoryDescription(ctx, root, ref, budget, run, lead, nil)
}
func ReopenInventoryReferenceDescriptionV1(ctx context.Context, root *publication.Root, ref publication.StoredObject, budget *CallerBudgetV3, run, lead string, admission, evidence publication.StoredObject) (BoundSelectedDescriptionV3, error) {
	if !admission.Valid() || !evidence.Valid() {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	expected := &cat.NativeInventoryAdmissionV1{RunID: run, LeadID: lead, AdmissionSHA256: "sha256:" + admission.ID, EvidenceSHA256: "sha256:" + evidence.ID}
	return reopenInventoryDescription(ctx, root, ref, budget, run, lead, expected)
}
func reopenInventoryDescription(ctx context.Context, root *publication.Root, ref publication.StoredObject, budget *CallerBudgetV3, run, lead string, expected *cat.NativeInventoryAdmissionV1) (BoundSelectedDescriptionV3, error) {
	if !ref.Valid() || budget.Reserve(ctx, uint64(ref.Bytes)*6+16384) != nil {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	raw, e := publication.ReadObject(root, "inventory-description-", ref)
	var p SelectedEvidenceV3
	if e != nil || durableStrict(raw, &p) != nil || p.ID == "" || p.HandoffID == "" || p.CheckpointID == "" || p.Unit.ID != p.Request.Envelope.AdmissionID || describerequest.ValidateRecord(p.Request) != nil {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	if expected != nil && (len(p.OriginalReferences) == 0 || p.InventoryAdmission == nil || p.InventoryAdmission.RunID != expected.RunID || p.InventoryAdmission.LeadID != expected.LeadID || p.InventoryAdmission.AdmissionSHA256 != expected.AdmissionSHA256 || p.InventoryAdmission.EvidenceSHA256 != expected.EvidenceSHA256) {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	if len(p.OriginalReferences) > 0 {
		if budget.Reserve(ctx, 32*8192) != nil {
			return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
		}
		var w inv.InventoryReferenceWitnessV1
		m := p.InventoryAdmission
		if m == nil || !inventoryProvenanceSHA(m.AdmissionSHA256) || !inventoryProvenanceSHA(m.EvidenceSHA256) {
			return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
		}
		if durableStrict(p.OriginalReferences, &w) != nil || (run != "" && (w.RunID != run || w.LeadID != lead)) || p.Selected.RunID != w.RunID || m.RunID != w.RunID || m.LeadID != w.LeadID || m.WitnessSHA256 != w.Digest || inv.ValidateInventoryReferencesV1(w, w.RunID, w.LeadID, p.Unit, p.Request.RecordID, p.OriginalInputID) != nil {
			return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
		}
	}
	if len(p.OriginalReferences) == 0 && p.InventoryAdmission != nil {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	return BoundSelectedDescriptionV3{&boundSelectedStateV3{evidence: p, raw: raw, budget: budget}}, nil
}
func inventoryProvenanceSHA(s string) bool {
	return len(s) == 71 && s[:7] == "sha256:" && durableID(s[7:])
}
