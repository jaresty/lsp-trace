package programcadmission

import (
	"encoding/json"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/sessionruntime"
)

// privateBindingBudget is a closed-shape upper bound, not an encoder or an
// admission path. It reads only the sealed owner's existing values. All arithmetic
// is ceiling-checked before multiplying/adding; traversal stops on exhaustion.
// The companion closed-shape test pins every transitive field, tag and method set.
type privateBindingBudget struct {
	total uint64
	ok    bool
}

func (b *privateBindingBudget) add(n uint64) bool {
	if !b.ok {
		return false
	}
	b.total, b.ok = sessionruntime.PrivateGroupingCheckedEncodedSize(b.total, n, 1, 0)
	return b.ok
}
func (b *privateBindingBudget) field(key string) { b.add(uint64(len(key)) + 4) }
func (b *privateBindingBudget) str(s string) {
	if !b.add(2) {
		return
	}
	for i := 0; i < len(s); i++ {
		n := uint64(1)
		c := s[i]
		if c == '"' || c == '\\' {
			n = 2
		} else if c < 0x20 || c == '<' || c == '>' || c == '&' || c >= 0x80 {
			n = 6
		}
		if !b.add(n) {
			return
		}
	}
}
func (b *privateBindingBudget) raw(raw json.RawMessage) {
	if !b.ok {
		return
	}
	if raw == nil {
		b.add(4)
		return
	}
	// RawMessage's MarshalJSON is compacted with HTML escaping. At most six
	// output bytes per input byte; whitespace only shrinks. Reserve before Valid
	// so scanner work/storage is bounded, including invalid/deep input.
	var ok bool
	b.total, ok = sessionruntime.PrivateGroupingCheckedEncodedSize(b.total, uint64(len(raw)), 6, 0)
	b.ok = ok
	if b.ok && !json.Valid(raw) {
		b.ok = false
	}
}
func (b *privateBindingBudget) bytes(raw []byte) {
	if !b.ok {
		return
	}
	n := uint64(len(raw))
	// Bound n before n+2 and multiplication, even for adversarial helper inputs.
	if n > sessionruntime.PrivateGroupingQueryMaxBytes {
		b.ok = false
		return
	}
	encoded := 2 + 4*((n+2)/3)
	if encoded < 4 {
		encoded = 4
	} // null, or clone-normalized non-nil empty
	b.add(encoded)
}

func privateCompositeBindingBound(source CompositeSourceBinding) (uint64, bool) {
	b := privateBindingBudget{total: 1, ok: true} // strict raw comparison + appended newline
	b.binding(source)
	if !b.ok {
		return 0, false
	}
	return b.total, true
}

func (b *privateBindingBudget) binding(v CompositeSourceBinding) {
	if !b.add(2) {
		return
	}
	b.field("CompositeID")
	b.str(v.CompositeID)
	b.field("CompositeOutputSHA256")
	b.str(v.CompositeOutputSHA256)
	b.field("ClaimCeiling")
	b.str(v.ClaimCeiling)
	b.field("Authority")
	b.add(20)
	b.field("SourceGraphComplete")
	b.str(v.SourceGraphComplete)
	b.field("Compatibility")
	b.compatibility(v.Compatibility)
	b.field("Constituents")
	if !b.add(4) {
		return
	}
	for _, item := range v.Constituents {
		if !b.add(1) {
			return
		}
		b.constituent(item)
	}
	b.field("Completeness")
	b.completeness(v.Completeness)
}

func (b *privateBindingBudget) compatibility(v CompatibilityReference) {
	if !b.add(2) {
		return
	}
	b.field("WorkspaceURI")
	b.str(v.WorkspaceURI)
	b.field("SourceRevision")
	b.str(v.SourceRevision)
	b.field("PositionEncoding")
	b.str(v.PositionEncoding)
	b.field("RevisionCustody")
	b.str(v.RevisionCustody)
	b.field("AcquisitionSemantics")
	b.str(v.AcquisitionSemantics)
	b.field("PrivacyPolicy")
	b.str(v.PrivacyPolicy)
	b.field("ServerCommand")
	b.str(v.ServerCommand)
	b.field("ServerVersion")
	b.str(v.ServerVersion)
	b.field("LanguageID")
	b.str(v.LanguageID)
	b.field("EvidenceSemantics")
	b.raw(v.EvidenceSemantics)
	b.field("SensitivityPolicy")
	b.raw(v.SensitivityPolicy)
}

func (b *privateBindingBudget) completeness(v CompositeCompleteness) {
	if !b.add(2) {
		return
	}
	b.field("AllTraversalComplete")
	b.add(5)
	b.field("AnyTruncated")
	b.add(5)
	b.field("WholeWorkspace")
	b.add(5)
	b.field("PerInput")
	if !b.add(4) {
		return
	}
	for _, item := range v.PerInput {
		if !b.add(1) {
			return
		}
		b.raw(item)
	}
}

func (b *privateBindingBudget) constituent(v ConstituentReference) {
	if !b.add(2) {
		return
	}
	b.field("Identity")
	b.str(v.Identity)
	b.field("SHA256")
	b.str(v.SHA256)
	b.field("SchemaVersion")
	b.str(v.SchemaVersion)
	b.field("GraphSHA256")
	b.str(v.GraphSHA256)
	b.field("GraphSchemaID")
	b.str(v.GraphSchemaID)
	b.field("ByteLength")
	b.add(20)
	b.field("GraphByteLength")
	b.add(20)
	b.field("BytesBase64")
	b.str(v.BytesBase64)
	b.field("SessionID")
	b.str(v.SessionID)
	b.field("InvocationID")
	b.str(v.InvocationID)
	b.field("Generation")
	b.add(20)
	b.field("RevisionCustody")
	b.str(v.RevisionCustody)
	b.field("SourcePolicy")
	b.str(v.SourcePolicy)
	b.field("WorkspaceURI")
	b.str(v.WorkspaceURI)
	b.field("AnalyzedVersion")
	b.str(v.AnalyzedVersion)
	b.field("DependencyCompleteness")
	b.str(v.DependencyCompleteness)
	b.field("CaptureBudget")
	b.captureBudget(v.CaptureBudget)
	b.field("Supplies")
	if !b.add(4) {
		return
	}
	for _, item := range v.Supplies {
		if !b.add(1) {
			return
		}
		b.supply(item)
	}
	b.field("Captures")
	if !b.add(4) {
		return
	}
	for _, item := range v.Captures {
		if !b.add(1) {
			return
		}
		b.receipt(item)
	}
	b.field("Bindings")
	if !b.add(4) {
		return
	}
	for _, item := range v.Bindings {
		if !b.add(1) {
			return
		}
		b.sourceBinding(item)
	}
	b.field("Invocation")
	b.raw(v.Invocation)
	b.field("Seeds")
	b.raw(v.Seeds)
	b.field("SeedMemberships")
	b.raw(v.SeedMemberships)
	b.field("Frontier")
	b.raw(v.Frontier)
	b.field("Diagnostics")
	b.raw(v.Diagnostics)
	b.field("Summary")
	b.raw(v.Summary)
	b.field("Slice")
	b.raw(v.Slice)
}

func (b *privateBindingBudget) captureBudget(v graphprovenance.CaptureBudgetV2) {
	if !b.add(2) {
		return
	}
	b.field("root_status")
	b.str(v.RootStatus)
	b.field("attempts")
	b.add(20)
	b.field("charged_bytes")
	b.add(20)
}

func (b *privateBindingBudget) supply(v graphprovenance.SupplyReceiptV2) {
	if !b.add(2) {
		return
	}
	b.field("request_id")
	b.str(v.RequestID)
	b.field("status")
	b.str(v.Status)
	b.field("observation")
	b.observation(v.Observation)
	b.field("receipt")
	if v.Receipt == nil {
		b.add(4)
	} else {
		b.receipt(*v.Receipt)
	}
}

func (b *privateBindingBudget) observation(v acquisition.Supply) {
	if !b.add(2) {
		return
	}
	b.field("uri")
	b.str(v.URI)
	b.field("language_id")
	b.str(v.LanguageID)
	b.field("observation")
	if len(v.Observation) > 0 {
		b.raw(v.Observation)
	}
}

func (b *privateBindingBudget) receipt(v graphprovenance.Receipt) {
	if !b.add(2) {
		return
	}
	b.field("id")
	b.str(v.ID)
	b.field("uri")
	b.str(v.URI)
	b.field("path")
	b.str(v.Path)
	b.field("classification")
	b.str(v.Classification)
	b.field("analyzed_version")
	b.str(v.AnalyzedVersion)
	b.field("status")
	b.str(v.Status)
	b.field("content")
	b.bytes(v.Content)
	b.field("canonical_receipt")
	b.bytes(v.CanonicalReceipt)
	b.field("supply")
	if v.Supply == nil {
		b.add(4)
	} else {
		b.supplyMetadata(*v.Supply)
	}
}

func (b *privateBindingBudget) supplyMetadata(v graphprovenance.SupplyMetadata) {
	if !b.add(2) {
		return
	}
	b.field("session_id")
	b.str(v.SessionID)
	b.field("generation")
	b.add(20)
	b.field("document_version")
	b.add(20)
	b.field("method")
	b.str(v.Method)
	b.field("params")
	b.raw(v.Params)
}

func (b *privateBindingBudget) sourceBinding(v graphprovenance.BindingV2) {
	if !b.add(2) {
		return
	}
	b.field("pointer")
	b.str(v.Pointer)
	b.field("uri")
	b.str(v.URI)
	b.field("attribution")
	b.str(v.Attribution)
	b.field("anchor_status")
	b.str(v.AnchorStatus)
	b.field("receipt_ids")
	if !b.add(4) {
		return
	}
	for _, item := range v.ReceiptIDs {
		if !b.add(1) {
			return
		}
		b.str(item)
	}
}
