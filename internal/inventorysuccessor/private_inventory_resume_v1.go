//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/describerequest"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

const InventoryResumeHeadVersionV1 = "lsp-trace.durable-inventory-head.v7"
const InventoryResumeConfigVersionV1 = "lsp-trace.inventory-owned-config.v5"

// InventoryResumeProfileV1 composes two unchanged native/selected component
// quotes under a new initial parent enclosure. Component ParentLimit fields
// retain their historical meanings; this profile alone declares the new H.
type InventoryResumeProfileV1 struct {
	Schema                                                             string
	Batches                                                            int
	ParentLimit, NativeControl, SelectedControl, Admission, Checkpoint uint64
	CumulativeNative                                                   [14]uint64
}

func NewInventoryResumeProfileV1() InventoryResumeProfileV1 {
	p := InventoryResumeProfileV1{Schema: "lsp-trace.inventory-resume-profile.v1", Batches: 2, ParentLimit: cat.InventoryResumeLimitV1, NativeControl: cat.InventoryNativeControlBytesV2, SelectedControl: cat.InventorySelectedControlBytesV1, Admission: cat.InventoryAdmissionBytesV4, Checkpoint: cat.InventoryCheckpointBytesV1}
	n, s := NewInventoryNativeProfileV1(), NewInventorySelectedProfileV1()
	for i := range p.CumulativeNative {
		sum := n.Allocation[i] + s.Allocation[i]
		p.CumulativeNative[i] = 2 * sum
		if sum < n.Allocation[i] || p.CumulativeNative[i] < sum {
			panic("closed resume allocation overflow")
		}
	}
	return p
}
func (p InventoryResumeProfileV1) Valid() bool { return p == NewInventoryResumeProfileV1() }
func inventoryResumeProfileMatchesV1(schema string, p *InventoryResumeProfileV1) bool {
	if schema == InventorySemanticHeadVersionV1 {
		return p != nil && *p == NewInventorySemanticProfileV1()
	}
	if schema == InventoryReferenceHeadVersionV1 {
		return p != nil && *p == NewInventoryReferenceProfileV1()
	}
	if schema == InventoryConnectedHeadVersionV1 {
		return p != nil && *p == NewInventoryConnectedProfileV1()
	}
	if InventoryResumableHeadV1(schema) {
		return p != nil && p.Valid()
	}
	return p == nil
}
func inventoryParentWorkLimitV1(schema string) uint64 {
	if InventoryResumableHeadV1(schema) {
		return inventoryParentLimitV1(schema) * 32
	}
	return 1 << 32
}

// InventoryCheckpointV1 contains verified accounting and immutable descriptors,
// never an execution capability. Only a new parent CAS can fund the next batch.
type InventoryCheckpointV1 struct {
	Native              InventoryNativeClaimV1
	Selected            InventorySelectedClaimV1
	Admission, Evidence publication.StoredObject
	Used, OutputBytes   uint64
}

func InventoryNamespaceV1(kind, run string, ordinal int) string {
	return fmt.Sprintf("inventory-%s-%s-%06d", kind, run, ordinal)
}
func InventoryNativeControlForHeadV1(schema string) uint64 {
	if InventoryResumableHeadV1(schema) {
		return cat.InventoryNativeControlBytesV2
	}
	return InventoryNativeControlBytesV1
}
func InventorySelectedControlForHeadV1(schema string) uint64 {
	n := cat.InventorySelectedControlBytesV1 + cat.InventoryAdmissionBytesV4
	if InventoryReferenceHead(schema) {
		n += cat.InventoryReferenceRetentionV1
	}
	if InventoryResumableHeadV1(schema) {
		n += cat.InventoryCheckpointBytesV1
	}
	return n
}
func (c InventoryCheckpointV1) Valid(run string) bool {
	return c.Native.Schema == "lsp-trace.inventory-native-claim.v2" && c.Native.Valid() && c.Native.RunID == run && c.Native.Ordinal == 0 && c.Native.Outcome == "OBSERVED" && c.Selected.Schema == "lsp-trace.inventory-selected-claim.v2" && c.Selected.Valid() && c.Selected.RunID == run && c.Selected.Ordinal == 0 && c.Selected.Outcome == "OBSERVED" && c.Selected.Deadline == c.Native.Binding.Deadline.UnixNano() && c.Admission.Valid() && c.Admission.Bytes <= 16384 && c.Evidence.Valid() && c.Evidence.Bytes <= 8<<20 && c.OutputBytes == uint64(c.Evidence.Bytes) && c.Used > 0 && c.Used <= cat.InventoryResumeLimitV1
}

// VerifyInventoryCheckpointV1 checks actual immutable admission/evidence bytes.
// Its return is a verification result, not permission. No owner is reconstructed.
func VerifyInventoryCheckpointV1(ctx context.Context, root *publication.Root, run string, c InventoryCheckpointV1) error {
	return verifyInventoryCheckpoint(ctx, root, run, c, false)
}
func VerifyInventoryCheckpointForHeadV1(ctx context.Context, root *publication.Root, schema, run string, c InventoryCheckpointV1) error {
	return verifyInventoryCheckpoint(ctx, root, run, c, InventoryReferenceHead(schema))
}
func verifyInventoryCheckpoint(ctx context.Context, root *publication.Root, run string, c InventoryCheckpointV1, references bool) error {
	admissionSchema, evidenceSchema := "lsp-trace.inventory-admission.v2", "lsp-trace.inventory-evidence.v1"
	if references {
		admissionSchema = "lsp-trace.inventory-admission.v3"
		evidenceSchema = "lsp-trace.inventory-evidence.v2"
	}
	if ctx == nil || ctx.Err() != nil || !c.Valid(run) {
		return ErrPrivateDiscoveryOperationV2
	}
	raw, e := publication.ReadObject(root, "inventory-admission-", c.Admission)
	var a struct {
		Schema, RunID, LeadID string
		Index                 int
		Previous, Evidence    publication.StoredObject
		Binding               struct {
			Index                               int
			Provenance, RequestID, SourceUnitID string
		}
		Payload, AdmittedEvidenceBytes uint64
	}
	if e != nil || cat.DecodeNativeV3(raw, &a) != nil || a.Schema != admissionSchema || a.RunID != run || a.Index != 0 || a.Binding.Index != 0 || a.Previous != (publication.StoredObject{}) || a.Evidence != c.Evidence || a.AdmittedEvidenceBytes != c.OutputBytes || a.LeadID != fmt.Sprintf("%s-000000", run) || a.Binding.RequestID == "" || a.Binding.SourceUnitID == "" {
		return ErrPrivateDiscoveryOperationV2
	}
	evidence, e := publication.ReadObject(root, "inventory-evidence-", c.Evidence)
	var v struct {
		Schema, RunID, LeadID, Handoff, Checkpoint string
		Accounting                                 CompletedSelectedAccountingV1
		Unit                                       struct{ ID, SourceDigest string }
		Request                                    describerequest.Record
	}
	if e != nil || json.Unmarshal(evidence, &v) != nil || v.Schema != evidenceSchema || v.RunID != run || v.LeadID != a.LeadID || v.Accounting != c.Selected.Accounting || v.Handoff == "" || v.Checkpoint == "" || v.Unit.ID != a.Binding.SourceUnitID || v.Request.RecordID != a.Binding.RequestID || v.Unit.ID != v.Request.Envelope.AdmissionID || describerequest.ValidateRecord(v.Request) != nil {
		return ErrPrivateDiscoveryOperationV2
	}
	if references {
		if _, e := ValidateInventoryReferenceEvidenceV1(evidence, run, a.LeadID); e != nil {
			return e
		}
	}
	return ctx.Err()
}
