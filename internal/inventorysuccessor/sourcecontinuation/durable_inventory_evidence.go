//go:build darwin || linux

package sourcecontinuation

import (
	"context"
	"encoding/json"
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
}

// RetainInventoryEvidence consumes the original owner's one-shot durable handoff.
// It does not start a semantic window. All originals are retained while the
// original bounded native owner is still present; later tasks have no native state.
func RetainInventoryEvidence(ctx context.Context, root *publication.Root, run, lead string, r PrivateSelectedResultV2, b CallerDescribeBindingV3) (publication.StoredObject, uint64, error) {
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
	p := retainedInventoryEvidence{"lsp-trace.inventory-evidence.v1", run, lead, v.selected, r.state.accounting, v.unit, v.request, v.handoff, v.checkpoint, b.Interpreter, v.host, b.Limits}
	raw, e := json.Marshal(p)
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
	if e != nil || durableStrict(raw, &p) != nil || p.Schema != "lsp-trace.inventory-evidence.v1" || p.RunID != run || p.LeadID != lead || p.Accounting.Head == "" || p.Accounting.Event != "OBSERVE" || p.Accounting.Disposition != "OBSERVED" || p.Accounting.Windows != 3 || p.Handoff == "" || p.Checkpoint == "" || !p.Limits.Valid() || !selectedSelectorBoundV2(p.Selected) || describerequest.ValidateRecord(p.Request) != nil || p.Unit.ID != p.Request.Envelope.AdmissionID || p.Host.RequestRecordID != p.Request.RecordID || p.Host.Pins.PromptSHA256 != "sha256:"+p.Request.Envelope.InputSHA256 {
		return p, ErrDurableDescribeV1
	}
	return p, nil
}
func CompleteInventoryEvidence(ctx context.Context, root *publication.Root, ref publication.StoredObject, run, lead, actor string, deadline time.Time, budget *CallerBudgetV3, answer []byte) (BoundSelectedDescriptionV3, error) {
	p, e := readInventoryEvidence(ctx, root, ref, run, lead, budget)
	if e != nil || p.Interpreter != actor {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	b := CallerDescribeBindingV3{AttemptID: p.Host.AttemptID, Interpreter: p.Interpreter, Pins: p.Host.Pins, Deadline: deadline, Limits: p.Limits, Budget: budget}
	a := SelectedDescribeV3{&selectedDescribeStateV3{selected: p.Selected, unit: p.Unit, request: p.Request, handoff: p.Handoff, checkpoint: p.Checkpoint, binding: b, host: p.Host}}
	return CompleteSelectedDescribeV3(ctx, a, answer)
}

// ReopenInventoryDescription restores data-only semantic custody from the owned
// store, never native admission and never caller-supplied DTO bytes.
func ReopenInventoryDescription(ctx context.Context, root *publication.Root, ref publication.StoredObject, budget *CallerBudgetV3) (BoundSelectedDescriptionV3, error) {
	if !ref.Valid() || budget.Reserve(ctx, uint64(ref.Bytes)*6+16384) != nil {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	raw, e := publication.ReadObject(root, "inventory-description-", ref)
	var p SelectedEvidenceV3
	if e != nil || durableStrict(raw, &p) != nil || p.ID == "" || p.HandoffID == "" || p.CheckpointID == "" || p.Unit.ID != p.Request.Envelope.AdmissionID || describerequest.ValidateRecord(p.Request) != nil {
		return BoundSelectedDescriptionV3{}, ErrDurableDescribeV1
	}
	return BoundSelectedDescriptionV3{&boundSelectedStateV3{evidence: p, raw: raw, budget: budget}}, nil
}
