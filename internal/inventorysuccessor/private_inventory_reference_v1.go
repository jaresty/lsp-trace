//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	ms "lsp-trace/internal/adr0011methodsyntax"
	qt "lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/describerequest"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
	"reflect"
	"strconv"
)

const InventoryReferenceHeadVersionV1 = "lsp-trace.durable-inventory-head.v9"
const InventoryReferenceConfigVersionV1 = "lsp-trace.inventory-owned-config.v7"

func InventoryConnectedHead(schema string) bool {
	return schema == InventoryConnectedHeadVersionV1 || schema == InventoryReferenceHeadVersionV1
}
func NewInventoryReferenceProfileV1() InventoryResumeProfileV1 {
	p := NewInventoryConnectedProfileV1()
	p.Schema = "lsp-trace.inventory-connected-profile.v2"
	p.ParentLimit = cat.InventoryReferenceLimitV1
	p.Admission = cat.InventoryReferenceAdmissionV1
	return p
}

// Data-only historical observation. No constructor accepts it as an original.
// Query -> reported reference location is request/result direction, NOT a call,
// feature-membership edge, definition edge or proof of target source capture.
type InventoryReferenceWitnessV1 struct {
	Schema, Digest, RunID, LeadID, UnitID, RequestID, InputID string
	Query                                                     qt.Query
	Pair                                                      sessionruntime.OwnedMethodPair
	Direction, TargetSourceAvailability, Completeness         string
	Null                                                      bool
	Truncated                                                 bool
	ReturnedCount                                             int
	Targets                                                   []ms.Item
}

const referenceWitnessSchema = "lsp-trace.inventory-original-references.v1"

func referenceDigest(w InventoryReferenceWitnessV1) string {
	w.Digest = ""
	b, _ := json.Marshal(w)
	return fmt.Sprintf("sha256:%x", sha256.Sum256(b))
}

// Called only on the selected producer original, after reserving retention work.
// The opaque original is not re-opened from disk or renewed by this observation.
func (o PrivateProducerOriginals) InventoryReferencesV1(ctx context.Context, budget *cat.CallerBudgetV3, run, lead string, u tp.PrivateSourceUnit, request, input string) (InventoryReferenceWitnessV1, error) {
	bad := func() (InventoryReferenceWitnessV1, error) {
		return InventoryReferenceWitnessV1{}, ErrPrivateSelectedV2
	}
	if o.state == nil || ctx == nil || ctx.Err() != nil || o.InputID() != input || budget.Reserve(ctx, cat.InventoryReferenceRetentionV1) != nil {
		return bad()
	}
	n, ok := o.state.inventoryEvidence.EncodedBound()
	if !ok || n > cat.InventoryReferenceOriginalBytesV1 {
		return bad()
	}
	v, e := o.state.inventoryEvidence.Snapshot(cat.InventoryReferenceOriginalBytesV1)
	if e != nil || len(v.Methods) != 1 || len(v.Unavailable) != 0 {
		return bad()
	}
	if v.Query.SourceDigest != fmt.Sprintf("sha256:%x", sha256.Sum256(v.Source)) || v.Query.URI != u.URI || v.Query.SessionID != u.SessionID || v.Query.Generation != u.Generation || v.Query.SourceDigest != u.SourceDigest {
		return bad()
	}
	p := v.Methods[0]
	if len(p.Params) > 1024 || len(p.Result) > 4096 {
		return bad()
	}
	parsed, f := ms.ParseRaw(p.Method, p.Result, 8)
	if f != nil {
		return bad()
	}
	w := InventoryReferenceWitnessV1{Schema: referenceWitnessSchema, RunID: run, LeadID: lead, UnitID: u.ID, RequestID: request, InputID: input, Query: v.Query, Pair: p, Direction: "QUERY_TO_REPORTED_REFERENCE_LOCATION", TargetSourceAvailability: "NOT_CAPTURED_BY_THIS_WITNESS", Completeness: "RAW_RESPONSE_ONLY_PROVIDER_COMPLETENESS_UNKNOWN", Null: parsed.Null, Targets: parsed.Items, ReturnedCount: len(parsed.Items)}
	w.Digest = referenceDigest(w)
	if ValidateInventoryReferencesV1(w, run, lead, u, request, input) != nil {
		return bad()
	}
	return w, nil
}

// Verification proves stored internal consistency under the immutable admission
// digest, not provider authentication or resistance to replacing an entire store.
func ValidateInventoryReferencesV1(w InventoryReferenceWitnessV1, run, lead string, u tp.PrivateSourceUnit, request, input string) error {
	bad := ErrPrivateSelectedV2
	p, q := w.Pair, w.Query
	if len(run) != 64 || len(request) != 64 || len(input) != 71 || len(u.ID) != 71 || len(u.URI) > 512 || len(u.SessionID) > 256 || len(u.DocumentVersion) > 32 || len(u.SourceDigest) != 71 || len(q.OccurrenceID) != 71 || len(q.URI) > 512 || len(q.SessionID) > 256 || len(q.SourceDigest) != 71 || len(q.DocumentVersion) > 32 || len(q.Encoding) > 16 || len(w.Digest) != 71 || len(w.UnitID) != 71 || len(w.RequestID) != 64 || len(w.InputID) != 71 || len(w.RunID) != 64 || len(w.LeadID) != 71 || (lead != run+"-000000" && lead != run+"-000001") || !hashV2("sha256:"+run) || !hashV2("sha256:"+request) || !hashV2(input) || !hashV2(u.ID) || !hashV2(u.SourceDigest) {
		return bad
	}
	if w.Schema != referenceWitnessSchema || w.RunID != run || w.LeadID != lead || w.UnitID != u.ID || w.RequestID != request || w.InputID != input || w.Digest == "" || w.Direction != "QUERY_TO_REPORTED_REFERENCE_LOCATION" || w.TargetSourceAvailability != "NOT_CAPTURED_BY_THIS_WITNESS" || w.Completeness != "RAW_RESPONSE_ONLY_PROVIDER_COMPLETENESS_UNKNOWN" || len(w.Targets) > 8 || len(p.Params) > 1024 || len(p.Result) > 4096 || p.Method != "textDocument/references" || p.Source == nil || p.Key.ID == 0 || p.Key.Generation != p.Generation || p.SessionID != u.SessionID || p.Generation != u.Generation || p.Source.URI != u.URI || p.Source.SHA256 != u.SourceDigest || strconv.Itoa(p.Source.Version) != u.DocumentVersion || q.URI != u.URI || q.SessionID != u.SessionID || q.Generation != u.Generation || q.SourceDigest != u.SourceDigest || q.DocumentVersion != u.DocumentVersion || q.Encoding != u.PositionEncoding {
		return bad
	}
	if p.Write.Key != p.Key || p.Read.Key != p.Key || p.Write.Method != p.Method || p.Write.SessionID != p.SessionID || p.Read.SessionID != p.SessionID || p.Write.Generation != p.Generation || p.Read.Generation != p.Generation || p.Write.FrameBytes <= 0 || p.Read.FrameBytes <= 0 || !hashV2(p.Write.FrameSHA256) || !hashV2(p.Read.FrameSHA256) {
		return bad
	}
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct{ Line, Character uint32 } `json:"position"`
		Context  struct {
			IncludeDeclaration *bool `json:"includeDeclaration"`
		} `json:"context"`
	}
	if cat.DecodeNativeV3(p.Params, &params) != nil || params.TextDocument.URI != q.URI || params.Position.Line != q.Line || params.Position.Character != q.Character || params.Context.IncludeDeclaration == nil {
		return bad
	}
	parsed, f := ms.ParseRaw(p.Method, p.Result, 8)
	if f != nil || parsed.Null != w.Null || w.Truncated || w.ReturnedCount != len(parsed.Items) || !reflect.DeepEqual(parsed.Items, w.Targets) {
		return bad
	}
	for _, t := range w.Targets {
		if len(t.URI) == 0 || len(t.URI) > 512 {
			return bad
		}
	}
	raw, e := json.Marshal(w)
	if e != nil || len(raw) > cat.InventoryReferenceWitnessBytesV1 || w.Digest != referenceDigest(w) {
		return bad
	}
	return nil
}

// Used by independent checkpoint replay, Seal and semantic reopening. No grant.
func ValidateInventoryReferenceEvidenceV1(raw []byte, run, lead string) (InventoryReferenceWitnessV1, error) {
	var v struct {
		Schema, RunID, LeadID string
		Accounting            CompletedSelectedAccountingV1
		Unit                  tp.PrivateSourceUnit
		Request               describerequest.Record
		Selected              struct{ Intent tp.PrivateSourceIntent }
		OriginalReferences    *InventoryReferenceWitnessV1
	}
	var fields map[string]json.RawMessage
	if len(raw) > cat.InventoryConnectedEvidenceV1 || cat.DecodeNativeV3(raw, &fields) != nil || json.Unmarshal(raw, &v) != nil || v.Schema != "lsp-trace.inventory-evidence.v2" || v.RunID != run || v.LeadID != lead || v.OriginalReferences == nil || v.Selected.Intent.Endpoint.Kind != "REFERENCES_SYMBOL" || v.Selected.Intent.Endpoint.Role != "REFERENCED_SYMBOL" || v.Request.Envelope.AdmissionID != v.Unit.ID || describerequest.ValidateRecord(v.Request) != nil {
		return InventoryReferenceWitnessV1{}, ErrPrivateSelectedV2
	}
	w := *v.OriginalReferences
	if v.Selected.Intent.RunID != run || v.Accounting.URI != w.Query.URI || v.Accounting.OccurrenceID != w.Query.OccurrenceID || v.Accounting.Line != w.Query.Line || v.Accounting.Character != w.Query.Character || v.Accounting.Revision != v.Unit.Revision {
		return w, ErrPrivateSelectedV2
	}
	return w, ValidateInventoryReferencesV1(w, run, lead, v.Unit, v.Request.RecordID, v.Selected.Intent.InputID)
}
