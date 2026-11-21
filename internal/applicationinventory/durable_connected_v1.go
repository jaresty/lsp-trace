//go:build darwin || linux

package applicationinventory

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/describerequest"
	dw "lsp-trace/internal/describeworker"
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
	"reflect"
	"time"
)

// ConnectedAdmissionConfigV1 selects a NEW initial profile. It cannot upgrade
// v7. Four 30s semantic leases do not impose an overall job deadline. The
// response ceilings are deliberately narrower than the historical grammar.
func ConnectedAdmissionConfigV1(enumeration scopejob.Config, admission OwnedInventoryConfig) (OwnedInventoryConfig, error) {
	c, e := ResumableAdmissionConfigV1(enumeration, admission)
	if e != nil {
		return c, e
	}
	c.Schema = is.InventoryConnectedConfigVersionV1
	c.BudgetBytes = cat.InventoryConnectedLimitV1
	p := is.NewInventoryConnectedProfileV1()
	c.ResumeProfile = &p
	c.ResponseLimits = dw.SuppliedResponseLimits{MaxAdapterBytes: cat.InventoryConnectedAdapterV1, MaxSemanticBytes: cat.InventoryConnectedSemanticV1}
	c.TaskWindow = 30 * time.Second
	if !connectedConfigValid(c) {
		return OwnedInventoryConfig{}, ErrDurableInventory
	}
	return c, nil
}
func connectedConfigValid(c OwnedInventoryConfig) bool {
	if c.Schema != is.InventoryConnectedConfigVersionV1 || c.BudgetBytes != cat.InventoryConnectedLimitV1 || c.ResumeProfile == nil || *c.ResumeProfile != is.NewInventoryConnectedProfileV1() || c.Interpreter == c.Policy.Reviewer || c.ResponseLimits.MaxAdapterBytes > cat.InventoryConnectedAdapterV1 || c.ResponseLimits.MaxSemanticBytes > cat.InventoryConnectedSemanticV1 {
		return false
	}
	c.Schema = is.InventoryResumeConfigVersionV1
	c.BudgetBytes = cat.InventoryResumeLimitV1
	p := is.NewInventoryResumeProfileV1()
	c.ResumeProfile = &p
	return resumeConfigValidV1(c)
}

// Immutable original acquisition head, not a serialized capability. It binds
// the exact two-admission predecessor and its cumulative spending into Seal.
type connectedClosureV1 struct {
	Head publication.StoredObject
	Used uint64
}

func connectedAcquisitionValid(h durableInventoryHead) bool {
	if !is.InventoryConnectedHead(h.Schema) || h.Limit != h.connectedLimit() {
		return false
	}
	if h.Phase == "Creating" {
		return ownedCreationValid(h)
	}
	if h.Phase == "Sealing" {
		if h.Closure == nil || !h.Closure.Head.Valid() || h.Closure.Head.Bytes > 16384 || h.Status != "CLAIMED" || !validDurableID(h.TaskID) || h.Deadline <= 0 || h.Used != h.Closure.Used+h.connectedSealBytes() || h.Packets != 9 {
			return false
		}
		h.Used = h.Closure.Used
		h.Work = h.Used * 32
		h.Packets--
		h.Phase = "Admitted"
		h.Status = "PARTIAL"
		h.TaskID = ""
		h.Deadline = 0
		h.Closure = nil
	} else if h.Closure != nil {
		return false
	}
	h.Limit = h.acquisitionLimit()
	if h.Schema != is.InventoryReferenceHeadVersionV1 {
		h.Schema = is.InventoryResumeHeadVersionV1
	}
	return resumeHeadValidV1(h)
}
func connectedSemanticValid(h durableInventoryHead) bool {
	if h.Native != nil || h.Selected != nil || h.Checkpoint != nil || h.Packets < 9+uint64(h.CompletionCursor) || len(h.Completion.Inputs) != 2 || len(h.Completion.Payloads) != 2 {
		return false
	}
	for i, n := range h.Completion.Inputs {
		if n < 1 || n > cat.InventoryConnectedEvidenceV1 || h.Completion.Payloads[i] == 0 || h.Completion.Payloads[i] > cat.InventoryConnectedPayloadV1 {
			return false
		}
	}
	return is.InventoryConnectedHead(h.Schema) && h.Limit == h.connectedLimit() && h.Closure != nil && h.Closure.Head.Valid() && h.Closure.Head.Bytes <= 16384 && h.Closure.Used <= h.acquisitionLimit() && h.Completion.PreparationUsed == h.Closure.Used+h.connectedSealBytes() && len(h.Completion.Inputs) == 2 && h.Completion.InitialStateBytes <= cat.InventoryConnectedStateV1 && h.Completion.Manifest <= h.manifestLimit() && h.Completion.Limits.MaxAdapterBytes <= cat.InventoryConnectedAdapterV1 && h.Completion.Limits.MaxSemanticBytes <= cat.InventoryConnectedSemanticV1
}
func (h durableInventoryHead) workLimit() uint64 {
	if is.InventoryConnectedHead(h.Schema) {
		return h.connectedLimit() * 32
	}
	return 1 << 32
}
func (h durableInventoryHead) runLimit() uint64 {
	if is.InventoryConnectedHead(h.Schema) {
		return h.connectedLimit()
	}
	return durableRunLimit
}
func (h durableInventoryHead) independentTaskLeases() bool {
	return h.Schema == ownedHeadVersion || is.InventoryConnectedHead(h.Schema)
}
func (h durableInventoryHead) sliceBudget(deadline time.Time, amount, prior uint64, semantic time.Duration) (*cat.CallerBudgetV3, error) {
	if h.Schema == is.InventoryReferenceHeadVersionV1 {
		return cat.NewInventoryReferenceBudgetV1(deadline, amount, prior, semantic)
	}
	if h.Schema == is.InventoryConnectedHeadVersionV1 {
		return cat.NewInventoryConnectedBudgetV1(deadline, amount, prior, semantic)
	}
	return cat.NewReservedCallerBudgetV3(deadline, amount, prior, semantic)
}

// Runs only after the funded Seal CAS. Reads actual immutable bytes and checks
// both admission bindings, chain, selected accounting and intrinsic request ID.
// The original completion receipts were created only after native root cleanup.
func (s DurableStore) verifyConnectedAdmissions(ctx context.Context, h durableInventoryHead, original []byte) ([]string, error) {
	bad := func() ([]string, error) { return nil, ErrDurableInventory }
	if h.Closure == nil || h.Closure.Head != ownedConfigReference(original) || h.Checkpoint == nil || h.Native == nil || h.Selected == nil || !h.Checkpoint.Valid(h.RunID) || !h.Native.Valid() || !h.Selected.Valid() || h.Native.Ordinal != 1 || h.Selected.Ordinal != 1 || h.Native.Outcome != "OBSERVED" || h.Selected.Outcome != "OBSERVED" || h.Selected.Deadline != h.Native.Binding.Deadline.UnixNano() {
		return bad()
	}
	if h.Checkpoint.Evidence.Bytes > cat.InventoryConnectedEvidenceV1 {
		return bad()
	}
	if is.VerifyInventoryCheckpointForHeadV1(ctx, s.Root, h.Schema, h.RunID, *h.Checkpoint) != nil {
		return bad()
	}
	// Replay the original frozen selection through both exact ordinal partitions.
	originalHead := h
	originalHead.Phase = "Admitted"
	originalHead.Status = "PARTIAL"
	originalHead.Used = h.Closure.Used
	originalHead.Work = originalHead.Used * 32
	originalHead.Packets--
	originalHead.Closure = nil
	originalHead.TaskID = ""
	originalHead.Deadline = 0
	gaps := []string{"PAID_SCOPE_17_MEMBERS; TWO_SELECTED_R_QUERY_SIDE_ADMISSIONS; NOT_FULL_D_R_CALL_COVERAGE", "STRUCTURAL_RELATIONSHIPS_ARE_NOT_FEATURE_IDENTITY", "DENOMINATOR_SHA256:" + h.Enumeration.Last, "ACQUISITION_CLOSURE_SHA256:" + h.Closure.Head.ID}
	for ordinal, n := range []*is.InventoryNativeClaimV1{&h.Checkpoint.Native, h.Native} {
		originalHead.Sequence = ordinal
		batch, e := s.nativeBatchAt(ctx, originalHead, "")
		if e != nil || !reflect.DeepEqual(batch.Members, n.Members) || batch.DenominatorDigest != n.Binding.DenominatorDigest || batch.PlanIdentity != n.Binding.PlanIdentity || batch.Policy != n.Policy || batch.Workspace != n.Binding.Workspace {
			return bad()
		}
		for _, member := range n.Members {
			gaps = append(gaps, "PAID_DISCOVERY_MEMBER:"+member.URI)
		}
		gaps = append(gaps, fmt.Sprintf("ENUMERATION_QUALIFIERS:%+v;UNKNOWN_REMAINDER=%v", batch.Counts, batch.UnknownRemainder))
	}
	last := h.Input
	retained := h.OutputBytes
	for i := 1; i >= 0; i-- {
		if !last.Valid() || last.Bytes > 16384 {
			return bad()
		}
		raw, e := publication.ReadObject(s.Root, "inventory-admission-", last)
		var a ownedAdmissionRecord
		if e != nil || strictInventory(raw, &a) != nil || a.Schema != h.admissionSchema() || a.RunID != h.RunID || a.Index != i || a.Binding.Index != i || a.LeadID != fmt.Sprintf("%s-%06d", h.RunID, i) || !a.Evidence.Valid() || a.Evidence.Bytes > cat.InventoryConnectedEvidenceV1 || a.Payload == 0 || a.Payload > cat.InventoryConnectedPayloadV1 || a.AdmittedEvidenceBytes != retained || uint64(a.Evidence.Bytes) > retained {
			return bad()
		}
		selected := h.Selected
		if i == 0 {
			selected = &h.Checkpoint.Selected
			if last != h.Checkpoint.Admission || a.Evidence != h.Checkpoint.Evidence {
				return bad()
			}
		} else if a.Previous != h.Checkpoint.Admission {
			return bad()
		}
		raw, e = publication.ReadObject(s.Root, "inventory-evidence-", a.Evidence)
		var v struct {
			Schema, RunID, LeadID, Handoff, Checkpoint string
			Accounting                                 is.CompletedSelectedAccountingV1
			Unit                                       struct{ ID string }
			Request                                    describerequest.Record
		}
		if e != nil || json.Unmarshal(raw, &v) != nil || v.Schema != h.evidenceSchema() || v.RunID != h.RunID || v.LeadID != a.LeadID || v.Handoff == "" || v.Checkpoint == "" || v.Accounting != selected.Accounting || v.Unit.ID != a.Binding.SourceUnitID || v.Request.RecordID != a.Binding.RequestID || v.Unit.ID != v.Request.Envelope.AdmissionID || describerequest.ValidateRecord(v.Request) != nil {
			return bad()
		}
		gaps = append(gaps, "ADMISSION_SHA256:"+last.ID+";EVIDENCE_SHA256:"+a.Evidence.ID)
		if h.Schema == is.InventoryReferenceHeadVersionV1 {
			w, e := is.ValidateInventoryReferenceEvidenceV1(raw, h.RunID, a.LeadID)
			if e != nil {
				return bad()
			}
			header := fmt.Sprintf("ORIGINAL_REFERENCES_ADMISSION:%s;EVIDENCE:%s;WITNESS:%s;METHOD:%s;REQUEST:%d;SESSION:%s;GENERATION:%d;QUERY:%s;SOURCE:%s;TARGET_SOURCE:%s;COMPLETENESS:%s", last.ID, a.Evidence.ID, w.Digest, w.Pair.Method, w.Pair.Key.ID, w.Pair.SessionID, w.Pair.Generation, w.Query.URI, w.Query.SourceDigest, w.TargetSourceAvailability, w.Completeness)
			if len(header) > 2048 {
				return bad()
			}
			gaps = append(gaps, header)
			for _, target := range w.Targets {
				rawTarget, e := json.Marshal(target)
				if e != nil {
					return bad()
				}
				line := "ORIGINAL_REFERENCE_TARGET;WITNESS:" + w.Digest + ";DIRECTION:" + w.Direction + ";LOCATION:" + string(rawTarget)
				if len(line) > 2048 {
					return bad()
				}
				gaps = append(gaps, line)
			}
		}
		retained -= uint64(a.Evidence.Bytes)
		last = a.Previous
	}
	if retained != 0 || last != (publication.StoredObject{}) || ctx.Err() != nil {
		return bad()
	}
	ref, e := publication.StoreObject(s.Root, "inventory-acquisition-closure-", original)
	if e != nil || ref != h.Closure.Head {
		return bad()
	}
	return gaps, nil
}
