// Package retainedprojection deterministically selects exact retained source
// evidence. It plans immutable source-object resolution but performs no
// acquisition, object-store access, or source projection assembly.
package retainedprojection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/internal/v5sourcesnapshotv4"
	"lsp-trace/internal/v5sourcesnapshotv5"
	"lsp-trace/internal/v5sourcesnapshotv6"
)

const Ordering = "TARGET_FIRST_THEN_GRAPH_SUBJECT_AND_LOGICAL_SOURCE_LEXICOGRAPHIC"

type Code string

const (
	CodeAdmission           Code = "ADMISSION_FAILED"
	CodeInvalidRequest      Code = "INVALID_REQUEST"
	CodeMissingBinding      Code = "MISSING_SUBJECT_BINDING"
	CodeMissingDisplay      Code = "MISSING_DISPLAY_BINDING"
	CodeAmbiguousSelection  Code = "AMBIGUOUS_SELECTION"
	CodeReceiptMismatch     Code = "RECEIPT_MISMATCH"
	CodeSourceMismatch      Code = "SOURCE_MISMATCH"
	CodeInvalidRange        Code = "INVALID_RANGE"
	CodeIncompatibleRange   Code = "INCOMPATIBLE_RANGE"
	CodeUnsupportedEncoding Code = "UNSUPPORTED_ENCODING"
	CodeInvalidProvenance   Code = "INVALID_DISPLAY_PROVENANCE"
)

type Error struct {
	Code   Code
	Key    Key
	detail string
}

func (e *Error) Error() string {
	if e.Key == (Key{}) {
		return fmt.Sprintf("retainedprojection: %s: %s", e.Code, e.detail)
	}
	return fmt.Sprintf("retainedprojection: %s: %s\x00%s: %s", e.Code, e.Key.GraphSubjectID, e.Key.LogicalSourceID, e.detail)
}

func IsCode(err error, code Code) bool {
	var selection *Error
	if errors.As(err, &selection) && selection.Code == code {
		return true
	}
	var assembly *AssemblyError
	return errors.As(err, &assembly) && assembly.Code == code
}

func fail(code Code, key Key, detail string) error {
	return &Error{Code: code, Key: key, detail: detail}
}

type Key struct {
	GraphSubjectID  string `json:"graph_subject_id"`
	LogicalSourceID string `json:"logical_source_id"`
}

type RelationSelector struct {
	RelationID string
	Caller     Key
	Callee     Key
	Range      graph.Range
}

type Request struct {
	Target     Key
	Selections []Key
	Relations  []RelationSelector
}

// Admitted is opaque outside this package. Admit is the only supported way to
// construct it from an externally supplied V2 artifact.
type Admitted struct {
	artifact v5sourcesnapshotv2.Artifact
	parent   v5sourcesnapshot.Artifact
	v3       *v5sourcesnapshotv3.Artifact
	v4       *v5sourcesnapshotv4.Artifact
	v5       *v5sourcesnapshotv5.Artifact
	v6       *v5sourcesnapshotv6.Artifact
	raw      []byte
}

var v3AdmissionLimits = v5sourcesnapshotv3.Limits{MaxArtifactBytes: 64 << 20, MaxParentBytes: 32 << 20, MaxReceipts: 10000, MaxSourceBytes: 16 << 20, MaxTotalSourceBytes: 64 << 20, MaxBindings: 100000, MaxWork: 200000}
var v4AdmissionLimits = v5sourcesnapshotv4.Limits{MaxArtifactBytes: 96 << 20, MaxParentBytes: 64 << 20, MaxReceipts: 10000, MaxSourceBytes: 16 << 20, MaxTotalSourceBytes: 64 << 20, MaxBindings: 100000, MaxWork: 200000}
var v5AdmissionLimits = v5sourcesnapshotv5.Limits{MaxArtifactBytes: 128 << 20, MaxGraphBytes: 64 << 20, MaxReceipts: 10000, MaxSourceBytes: 16 << 20, MaxTotalSourceBytes: 64 << 20, MaxBindings: 100000, MaxOutcomes: 100000, MaxWork: 200000}
var v6AdmissionLimits = v5sourcesnapshotv6.Limits{MaxArtifactBytes: 128 << 20, MaxGraphBytes: 64 << 20, MaxReceipts: 10000, MaxSourceBytes: 16 << 20, MaxTotalSourceBytes: 64 << 20, MaxBindings: 100000, MaxOutcomes: 100000, MaxWork: 200000}

func Admit(raw []byte) (Admitted, error) {
	// V6 is normalized only from immutable retained bytes. Historical dispatch
	// below is unchanged and never consults workspace or session state.
	if version, err := v5sourcesnapshotv6.Validate(raw, v6AdmissionLimits); err == nil && version == v5sourcesnapshotv6.Version {
		var envelope v5sourcesnapshotv6.Artifact
		if json.Unmarshal(raw, &envelope) != nil {
			return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained artifact")
		}
		parent := v5sourcesnapshot.Artifact{GraphV5Bytes: append([]byte(nil), envelope.GraphV5Bytes...), GraphV5Digest: envelope.GraphV5Digest, PositionEncoding: envelope.PositionEncoding}
		artifact := v5sourcesnapshotv2.Artifact{}
		v3 := v5sourcesnapshotv3.Artifact{Receipts: make([]v5sourcesnapshotv3.Receipt, 0, len(envelope.Receipts)), Bindings: make([]v5sourcesnapshotv3.Binding, 0, len(envelope.RelationBindings))}
		for _, receipt := range envelope.Receipts {
			parent.Receipts = append(parent.Receipts, v5sourcesnapshot.Receipt{ID: receipt.ID, URI: receipt.URI, ContentDigest: receipt.ContentDigest, Content: append([]byte(nil), receipt.Content...), CanonicalReceipt: append([]byte(nil), receipt.CanonicalReceipt...)})
			v3.Receipts = append(v3.Receipts, v5sourcesnapshotv3.Receipt{ID: receipt.ID, URI: receipt.URI, ContentDigest: receipt.ContentDigest, Content: append([]byte(nil), receipt.Content...), CanonicalReceipt: append([]byte(nil), receipt.CanonicalReceipt...)})
		}
		for _, binding := range envelope.EndpointBindings {
			artifact.DisplayBindings = append(artifact.DisplayBindings, v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: binding.GraphSubjectID, LogicalSourceID: binding.LogicalSourceID, DisplayRange: binding.DisplayRange, DisplayRangePolicy: binding.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: binding.DisplayProvenanceKind, Method: binding.DisplayMethod}, ReceiptID: binding.ReceiptID, SourceDigest: binding.SourceDigest, PositionEncoding: binding.PositionEncoding, Status: v5sourcesnapshotv4.Status, Custody: v5sourcesnapshotv4.Custody})
			parent.Bindings = append(parent.Bindings,
				v5sourcesnapshot.Binding{RelationID: "v6-endpoint", EndpointRole: "DECLARATION", RangeRole: "DECLARATION_RANGE", Pointer: "/endpoint_bindings", NodeID: binding.GraphSubjectID, URI: binding.LogicalSourceID, Range: binding.ItemRange, SourceDigest: binding.SourceDigest, ReceiptIDs: []string{binding.ReceiptID}, Status: "RETAINED_BYTES"},
				v5sourcesnapshot.Binding{RelationID: "v6-endpoint", EndpointRole: "DECLARATION", RangeRole: "SELECTION_RANGE", Pointer: "/endpoint_bindings", NodeID: binding.GraphSubjectID, URI: binding.LogicalSourceID, Range: binding.SelectionRange, SourceDigest: binding.SourceDigest, ReceiptIDs: []string{binding.ReceiptID}, Status: "RETAINED_BYTES"})
		}
		for _, binding := range envelope.RelationBindings {
			v3.Bindings = append(v3.Bindings, v5sourcesnapshotv3.Binding{OccurrenceID: binding.OccurrenceID, RelationID: binding.RelationID, Direction: v5sourcesnapshotv3.Direction, CallerNodeID: binding.CallerNodeID, CalleeNodeID: binding.CalleeNodeID, CallerURI: binding.CallerLogicalSourceID, Range: binding.Range, CanonicalOrdinal: binding.CanonicalOrdinal, ReceiptID: binding.ReceiptID, SourceDigest: binding.SourceDigest, Provenance: v5sourcesnapshotv3.Provenance, Status: v5sourcesnapshotv3.Status, Custody: v5sourcesnapshotv3.Custody, Completeness: v5sourcesnapshotv3.Completeness})
		}
		return Admitted{artifact: artifact, parent: parent, v3: &v3, v6: &envelope, raw: append([]byte(nil), raw...)}, nil
	}

	// V5, V4, and V3 dispatch is explicit. Every path validates exact retained
	// bytes and normalizes only validated custody; none reads workspace state.
	if version, err := v5sourcesnapshotv5.Validate(raw, v5AdmissionLimits); err == nil && version == v5sourcesnapshotv5.Version {
		var envelope v5sourcesnapshotv5.Artifact
		if json.Unmarshal(raw, &envelope) != nil {
			return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained artifact")
		}
		parent := v5sourcesnapshot.Artifact{GraphV5Bytes: append([]byte(nil), envelope.GraphV5Bytes...), GraphV5Digest: envelope.GraphV5Digest, PositionEncoding: envelope.PositionEncoding}
		artifact := v5sourcesnapshotv2.Artifact{}
		v3 := v5sourcesnapshotv3.Artifact{Receipts: make([]v5sourcesnapshotv3.Receipt, 0, len(envelope.Receipts)), Bindings: make([]v5sourcesnapshotv3.Binding, 0, len(envelope.RelationBindings))}
		for _, receipt := range envelope.Receipts {
			parent.Receipts = append(parent.Receipts, v5sourcesnapshot.Receipt{ID: receipt.ID, URI: receipt.URI, ContentDigest: receipt.ContentDigest, Content: append([]byte(nil), receipt.Content...), CanonicalReceipt: append([]byte(nil), receipt.CanonicalReceipt...)})
			v3.Receipts = append(v3.Receipts, v5sourcesnapshotv3.Receipt{ID: receipt.ID, URI: receipt.URI, ContentDigest: receipt.ContentDigest, Content: append([]byte(nil), receipt.Content...), CanonicalReceipt: append([]byte(nil), receipt.CanonicalReceipt...)})
		}
		for _, binding := range envelope.EndpointBindings {
			artifact.DisplayBindings = append(artifact.DisplayBindings, v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: binding.GraphSubjectID, LogicalSourceID: binding.LogicalSourceID, DisplayRange: binding.DisplayRange, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: binding.ReceiptID, SourceDigest: binding.SourceDigest, PositionEncoding: envelope.PositionEncoding, Status: v5sourcesnapshotv4.Status, Custody: v5sourcesnapshotv4.Custody})
			parent.Bindings = append(parent.Bindings,
				v5sourcesnapshot.Binding{RelationID: "v5-endpoint", EndpointRole: "DECLARATION", RangeRole: "DECLARATION_RANGE", Pointer: "/endpoint_bindings", NodeID: binding.GraphSubjectID, URI: binding.LogicalSourceID, Range: binding.DisplayRange, SourceDigest: binding.SourceDigest, ReceiptIDs: []string{binding.ReceiptID}, Status: "RETAINED_BYTES"},
				v5sourcesnapshot.Binding{RelationID: "v5-endpoint", EndpointRole: "DECLARATION", RangeRole: "SELECTION_RANGE", Pointer: "/endpoint_bindings", NodeID: binding.GraphSubjectID, URI: binding.LogicalSourceID, Range: binding.DisplayRange, SourceDigest: binding.SourceDigest, ReceiptIDs: []string{binding.ReceiptID}, Status: "RETAINED_BYTES"})
		}
		for _, binding := range envelope.RelationBindings {
			v3.Bindings = append(v3.Bindings, v5sourcesnapshotv3.Binding{OccurrenceID: binding.OccurrenceID, RelationID: binding.RelationID, Direction: v5sourcesnapshotv3.Direction, CallerNodeID: binding.CallerNodeID, CalleeNodeID: binding.CalleeNodeID, CallerURI: binding.CallerLogicalSourceID, Range: binding.Range, CanonicalOrdinal: binding.CanonicalOrdinal, ReceiptID: binding.ReceiptID, SourceDigest: binding.SourceDigest, Provenance: v5sourcesnapshotv3.Provenance, Status: v5sourcesnapshotv3.Status, Custody: v5sourcesnapshotv3.Custody, Completeness: v5sourcesnapshotv3.Completeness})
		}
		return Admitted{artifact: artifact, parent: parent, v3: &v3, v5: &envelope, raw: append([]byte(nil), raw...)}, nil
	}

	if version, err := v5sourcesnapshotv4.Validate(raw, v4AdmissionLimits); err == nil && version == v5sourcesnapshotv4.Version {
		var envelope v5sourcesnapshotv4.Artifact
		var v3 v5sourcesnapshotv3.Artifact
		var artifact v5sourcesnapshotv2.Artifact
		var parent v5sourcesnapshot.Artifact
		if json.Unmarshal(raw, &envelope) != nil || json.Unmarshal(envelope.ParentSnapshot, &v3) != nil || json.Unmarshal(v3.ParentSnapshot, &artifact) != nil || json.Unmarshal(artifact.ParentSnapshot, &parent) != nil {
			return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained parent")
		}
		return Admitted{artifact: artifact, parent: parent, v3: &v3, v4: &envelope, raw: append([]byte(nil), raw...)}, nil
	}
	if version, err := v5sourcesnapshotv3.Validate(raw, v3AdmissionLimits); err == nil && version == v5sourcesnapshotv3.Version {
		var envelope v5sourcesnapshotv3.Artifact
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained artifact")
		}
		var artifact v5sourcesnapshotv2.Artifact
		if err := json.Unmarshal(envelope.ParentSnapshot, &artifact); err != nil {
			return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained parent")
		}
		var parent v5sourcesnapshot.Artifact
		if err := json.Unmarshal(artifact.ParentSnapshot, &parent); err != nil {
			return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained parent")
		}
		return Admitted{artifact: artifact, parent: parent, v3: &envelope, raw: append([]byte(nil), raw...)}, nil
	}
	if _, err := v5sourcesnapshotv2.Validate(raw); err != nil {
		return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained artifact")
	}
	var artifact v5sourcesnapshotv2.Artifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained artifact")
	}
	var parent v5sourcesnapshot.Artifact
	if err := json.Unmarshal(artifact.ParentSnapshot, &parent); err != nil {
		return Admitted{}, fail(CodeAdmission, Key{}, "invalid retained parent")
	}
	return Admitted{artifact: artifact, parent: parent, raw: append([]byte(nil), raw...)}, nil
}

type EvidenceRange struct {
	RelationID   string      `json:"relation_id"`
	EndpointRole string      `json:"endpoint_role"`
	RangeRole    string      `json:"range_role"`
	Pointer      string      `json:"pointer"`
	Range        graph.Range `json:"range"`
}

type Selection struct {
	Ordinal            int                           `json:"ordinal"`
	Role               string                        `json:"role"`
	Key                Key                           `json:"key"`
	ReceiptID          string                        `json:"receipt_id"`
	Source             sourceobject.Identity         `json:"source_identity"`
	PositionEncoding   string                        `json:"position_encoding"`
	EvidenceRanges     []EvidenceRange               `json:"evidence_ranges"`
	ItemRange          *graph.Range                  `json:"item_range,omitempty"`
	SelectionRange     *graph.Range                  `json:"selection_range,omitempty"`
	CallSiteRange      *graph.Range                  `json:"call_site_range,omitempty"`
	DisplayRange       graph.Range                   `json:"display_range"`
	DisplayProvenance  v5sourcesnapshotv2.Provenance `json:"display_provenance"`
	DisplayRangePolicy string                        `json:"display_range_policy"`
}

type RelationSelection struct {
	Ordinal          int                   `json:"ordinal"`
	RelationID       string                `json:"relation_id"`
	OccurrenceID     string                `json:"occurrence_id"`
	Caller           Key                   `json:"caller"`
	Callee           Key                   `json:"callee"`
	ReceiptID        string                `json:"receipt_id"`
	Source           sourceobject.Identity `json:"source_identity"`
	PositionEncoding string                `json:"position_encoding"`
	Range            graph.Range           `json:"range"`
	Direction        string                `json:"direction"`
	Provenance       string                `json:"provenance"`
}

type Plan struct {
	Ordering   string              `json:"ordering"`
	Target     Key                 `json:"target"`
	Selections []Selection         `json:"selections"`
	Relations  []RelationSelection `json:"relations,omitempty"`
	binding    [32]byte
}

func (p Plan) Bytes() ([]byte, error) { return json.Marshal(p) }

const (
	RetainedCustody                 = "RETAINED"
	ImmutableSourceObjectIdentityV1 = "IMMUTABLE_SOURCE_OBJECT_IDENTITY_V1"
	GraphProvenanceV5SchemaID       = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph-provenance.v5.schema.json"

	// PlanSealDomainV1 defines the exact private plan-seal preimage:
	// domain UTF-8 bytes, NUL, raw 32-byte SHA-256 of the exact admitted V2
	// bytes, NUL, then the exact canonical Plan.Bytes bytes.
	PlanSealDomainV1 = "lsp-trace.retained-projection-plan-seal.v1"
)

type RetainedCustodyBinding struct {
	Custody         string `json:"custody"`
	GraphSchemaID   string `json:"graph_schema_id"`
	GraphDigest     string `json:"graph_digest"`
	GraphByteLength uint64 `json:"graph_byte_length"`
	CaptureID       string `json:"capture_id"`
	ManifestID      string `json:"manifest_id"`
	ResolverKind    string `json:"resolver_kind"`
}

// DisplayKeys returns the (graph_subject_id, logical_source_id) keys of the
// admitted artifact's display bindings, in artifact order. It exposes only the
// identity pair — no ranges, provenance, or bytes — so callers can map a
// selected node to its logical source without reaching into the artifact.
func (a Admitted) DisplayKeys() []Key {
	if a.v6 != nil {
		keys := make([]Key, 0, len(a.v6.EndpointBindings))
		for _, binding := range a.v6.EndpointBindings {
			keys = append(keys, Key{GraphSubjectID: binding.GraphSubjectID, LogicalSourceID: binding.LogicalSourceID})
		}
		return keys
	}
	if a.v5 != nil {
		keys := make([]Key, 0, len(a.v5.EndpointBindings))
		for _, binding := range a.v5.EndpointBindings {
			keys = append(keys, Key{GraphSubjectID: binding.GraphSubjectID, LogicalSourceID: binding.LogicalSourceID})
		}
		return keys
	}
	if a.v4 != nil {
		keys := make([]Key, 0, len(a.v4.EndpointBindings))
		for _, binding := range a.v4.EndpointBindings {
			keys = append(keys, Key{GraphSubjectID: binding.GraphSubjectID, LogicalSourceID: binding.LogicalSourceID})
		}
		return keys
	}
	keys := make([]Key, 0, len(a.artifact.DisplayBindings))
	for _, binding := range a.artifact.DisplayBindings {
		keys = append(keys, Key{GraphSubjectID: binding.GraphSubjectID, LogicalSourceID: binding.LogicalSourceID})
	}
	return keys
}

// ValidateGraphCustody checks that exact graph-provenance.v5 envelope bytes
// match the schema identity, digest, and length declared by the binding.
func ValidateGraphCustody(binding RetainedCustodyBinding, graphBytes []byte) error {
	if binding.GraphSchemaID != GraphProvenanceV5SchemaID {
		return errors.New("graph custody schema mismatch")
	}
	sum := sha256.Sum256(graphBytes)
	if binding.GraphDigest != "sha256:"+hex.EncodeToString(sum[:]) {
		return errors.New("graph custody digest mismatch")
	}
	if binding.GraphByteLength != uint64(len(graphBytes)) {
		return errors.New("graph custody byte length mismatch")
	}
	if _, err := graphprovenance.ValidateFor(graphBytes, graphprovenance.Family, "v5"); err != nil {
		return fmt.Errorf("graph custody envelope: %w", err)
	}
	return nil
}

// CustodyBinding preserves the historical graph.v5 schema label for compatibility.
// That label describes the embedded graph object, not exact outer-envelope custody.
func (a Admitted) CustodyBinding(plan Plan) (RetainedCustodyBinding, error) {
	zero := RetainedCustodyBinding{}
	if len(a.raw) == 0 || len(a.parent.GraphV5Bytes) == 0 || a.parent.GraphV5Digest == "" {
		return zero, fail(CodeAdmission, Key{}, "valid admitted artifact required")
	}
	planBytes, err := plan.Bytes()
	if err != nil {
		return zero, fail(CodeInvalidPlan, Key{}, err.Error())
	}
	if plan.binding == ([32]byte{}) || plan.binding != planSeal(a.raw, planBytes) {
		return zero, fail(CodeInvalidPlan, Key{}, "canonical plan returned by Select required")
	}
	graphSum := sha256.Sum256(a.parent.GraphV5Bytes)
	if a.parent.GraphV5Digest != "sha256:"+hex.EncodeToString(graphSum[:]) {
		return zero, fail(CodeAdmission, Key{}, "embedded graph digest mismatch")
	}
	captureSum := sha256.Sum256(a.raw)
	manifestSum := sha256.Sum256(planBytes)
	return RetainedCustodyBinding{
		Custody: RetainedCustody, GraphSchemaID: graphprovenance.GraphV5SchemaID,
		GraphDigest: a.parent.GraphV5Digest, GraphByteLength: uint64(len(a.parent.GraphV5Bytes)),
		CaptureID:    "sha256:" + hex.EncodeToString(captureSum[:]),
		ManifestID:   "sha256:" + hex.EncodeToString(manifestSum[:]),
		ResolverKind: ImmutableSourceObjectIdentityV1,
	}, nil
}

// GraphProvenanceCustodyBinding explicitly binds the exact outer
// graph-provenance.v5 envelope while preserving the historical source and plan
// seals produced by CustodyBinding.
func (a Admitted) GraphProvenanceCustodyBinding(plan Plan) (RetainedCustodyBinding, error) {
	binding, err := a.CustodyBinding(plan)
	if err != nil {
		return RetainedCustodyBinding{}, err
	}
	binding.GraphSchemaID = GraphProvenanceV5SchemaID
	return binding, nil
}

func Select(admitted Admitted, request Request) (Plan, error) {
	zero := Plan{}
	if request.Target.GraphSubjectID == "" || request.Target.LogicalSourceID == "" || len(request.Selections) == 0 {
		return zero, fail(CodeInvalidRequest, Key{}, "non-empty target and selections required")
	}
	keys := append([]Key(nil), request.Selections...)
	sort.Slice(keys, func(i, j int) bool { return lessKey(keys[i], keys[j]) })
	for i, key := range keys {
		if key.GraphSubjectID == "" || key.LogicalSourceID == "" {
			return zero, fail(CodeInvalidRequest, key, "selection key fields required")
		}
		if i > 0 && key == keys[i-1] {
			return zero, fail(CodeAmbiguousSelection, key, "duplicate requested selection key")
		}
	}
	targetAt := -1
	for i := range keys {
		if keys[i] == request.Target {
			targetAt = i
			break
		}
	}
	if targetAt < 0 {
		return zero, fail(CodeInvalidRequest, request.Target, "target must be selected")
	}
	keys = append([]Key{request.Target}, append(keys[:targetAt], keys[targetAt+1:]...)...)

	displays := map[Key][]v5sourcesnapshotv2.DisplayBinding{}
	parentBindings := map[Key][]v5sourcesnapshot.Binding{}
	receipts := map[string][]v5sourcesnapshot.Receipt{}
	if admitted.v4 != nil {
		for _, b := range admitted.v4.EndpointBindings {
			key := Key{b.GraphSubjectID, b.LogicalSourceID}
			display := v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: b.GraphSubjectID, LogicalSourceID: b.LogicalSourceID, DisplayRange: b.DisplayRange, DisplayRangePolicy: b.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: b.Provenance.Kind, Method: b.Provenance.Method}, ReceiptID: b.ReceiptID, SourceDigest: b.SourceDigest, PositionEncoding: b.PositionEncoding, Status: b.Status, Custody: b.Custody}
			displays[key] = appendUniqueDisplay(displays[key], display)
			parentBindings[key] = []v5sourcesnapshot.Binding{
				{RelationID: "v4-endpoint", EndpointRole: "DECLARATION", RangeRole: "DECLARATION_RANGE", Pointer: "/v4/endpoint_bindings", NodeID: b.GraphSubjectID, URI: b.LogicalSourceID, Range: b.DisplayRange, SourceDigest: b.SourceDigest, ReceiptIDs: []string{b.ReceiptID}, Status: "RETAINED_BYTES"},
				{RelationID: "v4-endpoint", EndpointRole: "DECLARATION", RangeRole: "SELECTION_RANGE", Pointer: "/v4/endpoint_bindings", NodeID: b.GraphSubjectID, URI: b.LogicalSourceID, Range: b.DisplayRange, SourceDigest: b.SourceDigest, ReceiptIDs: []string{b.ReceiptID}, Status: "RETAINED_BYTES"},
			}
		}
		for _, r := range admitted.v4.EndpointReceipts {
			receipt := v5sourcesnapshot.Receipt{ID: r.ID, URI: r.URI, ContentDigest: r.ContentDigest, Content: append([]byte(nil), r.Content...), CanonicalReceipt: append([]byte(nil), r.CanonicalReceipt...)}
			receipts[r.ID] = append(receipts[r.ID], receipt)
		}
	} else {
		for _, binding := range admitted.artifact.DisplayBindings {
			key := Key{binding.GraphSubjectID, binding.LogicalSourceID}
			displays[key] = appendUniqueDisplay(displays[key], binding)
		}
		for _, binding := range admitted.parent.Bindings {
			key := Key{binding.NodeID, binding.URI}
			parentBindings[key] = append(parentBindings[key], binding)
		}
		for _, receipt := range admitted.parent.Receipts {
			receipts[receipt.ID] = append(receipts[receipt.ID], receipt)
		}
	}

	selections := make([]Selection, 0, len(keys))
	for ordinal, key := range keys {
		bindings := append([]v5sourcesnapshot.Binding(nil), parentBindings[key]...)
		if len(bindings) == 0 {
			return zero, fail(CodeMissingBinding, key, "no retained subject binding")
		}
		displayRows := displays[key]
		if len(displayRows) == 0 {
			return zero, fail(CodeMissingDisplay, key, "no display binding")
		}
		if len(displayRows) != 1 {
			return zero, fail(CodeAmbiguousSelection, key, "multiple display bindings")
		}
		display := displayRows[0]
		if !validEncoding(display.PositionEncoding) {
			return zero, fail(CodeUnsupportedEncoding, key, "unsupported position encoding")
		}
		if !validRange(display.DisplayRange) {
			return zero, fail(CodeInvalidRange, key, "invalid display range")
		}
		if display.Provenance.Kind != v5sourcesnapshotv2.ProvenanceKind || display.Provenance.Method != v5sourcesnapshotv2.ProvenanceMethod || display.DisplayRangePolicy != v5sourcesnapshotv2.DisplayRangePolicy {
			return zero, fail(CodeInvalidProvenance, key, "display provenance or policy substitution")
		}
		sort.Slice(bindings, func(i, j int) bool {
			a, b := bindings[i], bindings[j]
			if a.RelationID != b.RelationID {
				return a.RelationID < b.RelationID
			}
			if a.EndpointRole != b.EndpointRole {
				return a.EndpointRole < b.EndpointRole
			}
			if a.RangeRole != b.RangeRole {
				return a.RangeRole < b.RangeRole
			}
			return a.Pointer < b.Pointer
		})
		rows := receipts[display.ReceiptID]
		if len(rows) != 1 || rows[0].URI != key.LogicalSourceID || !includesReceipt(bindings, display.ReceiptID) {
			return zero, fail(CodeReceiptMismatch, key, "display and retained evidence do not bind one receipt")
		}
		receipt := rows[0]
		if display.SourceDigest != receipt.ContentDigest || !allDigest(bindings, display.SourceDigest) {
			return zero, fail(CodeSourceMismatch, key, "display, receipt, and retained evidence digests differ")
		}
		if display.PositionEncoding != admitted.parent.PositionEncoding {
			return zero, fail(CodeUnsupportedEncoding, key, "position encoding differs from retained parent")
		}
		selection := Selection{Ordinal: ordinal, Role: "ADDITIONAL", Key: key, ReceiptID: display.ReceiptID, Source: sourceobject.Identity{Digest: display.SourceDigest, ByteLength: uint64(len(receipt.Content))}, PositionEncoding: display.PositionEncoding, EvidenceRanges: make([]EvidenceRange, 0, len(bindings)), DisplayRange: display.DisplayRange, DisplayProvenance: display.Provenance, DisplayRangePolicy: display.DisplayRangePolicy}
		if ordinal == 0 {
			selection.Role = "TARGET"
		}
		for _, binding := range bindings {
			if !validRange(binding.Range) {
				return zero, fail(CodeInvalidRange, key, "invalid retained evidence range")
			}
			selection.EvidenceRanges = append(selection.EvidenceRanges, EvidenceRange{binding.RelationID, binding.EndpointRole, binding.RangeRole, binding.Pointer, binding.Range})
			switch binding.RangeRole {
			case "DECLARATION_RANGE":
				if err := setUniqueRange(&selection.ItemRange, binding.Range); err != nil {
					return zero, fail(CodeAmbiguousSelection, key, "conflicting item ranges")
				}
			case "SELECTION_RANGE":
				if err := setUniqueRange(&selection.SelectionRange, binding.Range); err != nil {
					return zero, fail(CodeAmbiguousSelection, key, "conflicting selection ranges")
				}
			case "CALL_SITE_RANGE":
				if err := setUniqueRange(&selection.CallSiteRange, binding.Range); err != nil {
					return zero, fail(CodeAmbiguousSelection, key, "conflicting call-site ranges")
				}
			}
		}
		if selection.ItemRange == nil || selection.SelectionRange == nil {
			return zero, fail(CodeMissingBinding, key, "item and selection ranges required")
		}
		if !contains(*selection.ItemRange, *selection.SelectionRange) || !contains(selection.DisplayRange, *selection.ItemRange) {
			return zero, fail(CodeIncompatibleRange, key, "selection, item, and display ranges are incompatible")
		}
		selections = append(selections, selection)
	}
	relations, err := selectRelations(admitted, request.Relations, selections)
	if err != nil {
		return zero, err
	}
	plan := Plan{Ordering: Ordering, Target: request.Target, Selections: selections, Relations: relations}
	planBytes, err := plan.Bytes()
	if err != nil {
		return zero, fail(CodeInvalidPlan, Key{}, err.Error())
	}
	plan.binding = planSeal(admitted.raw, planBytes)
	return plan, nil
}

func appendUniqueDisplay(rows []v5sourcesnapshotv2.DisplayBinding, candidate v5sourcesnapshotv2.DisplayBinding) []v5sourcesnapshotv2.DisplayBinding {
	for _, row := range rows {
		if row == candidate {
			return rows
		}
	}
	return append(rows, candidate)
}

func selectRelations(admitted Admitted, requested []RelationSelector, endpoints []Selection) ([]RelationSelection, error) {
	if len(requested) == 0 {
		return nil, nil
	}
	if admitted.v3 == nil {
		return nil, fail(CodeInvalidRequest, Key{}, "relation selections require V3 artifact")
	}
	selected := make(map[Key]struct{}, len(endpoints))
	for _, s := range endpoints {
		selected[s.Key] = struct{}{}
	}
	receipts := make(map[string]v5sourcesnapshotv3.Receipt, len(admitted.v3.Receipts))
	for _, r := range admitted.v3.Receipts {
		receipts[r.ID] = r
	}
	out := make([]RelationSelection, 0, len(requested))
	seen := map[string]bool{}
	for _, r := range requested {
		if r.RelationID == "" || r.Caller == (Key{}) || r.Callee == (Key{}) || !validRange(r.Range) {
			return nil, fail(CodeInvalidRequest, r.Caller, "invalid relation selector")
		}
		if _, ok := selected[r.Caller]; !ok {
			return nil, fail(CodeMissingBinding, r.Caller, "relation caller must be selected")
		}
		if _, ok := selected[r.Callee]; !ok {
			return nil, fail(CodeMissingBinding, r.Callee, "relation callee must be selected")
		}
		var matches []v5sourcesnapshotv3.Binding
		for _, b := range admitted.v3.Bindings {
			if b.RelationID == r.RelationID && b.Direction == v5sourcesnapshotv3.Direction && b.CallerNodeID == r.Caller.GraphSubjectID && b.CalleeNodeID == r.Callee.GraphSubjectID && b.CallerURI == r.Caller.LogicalSourceID && b.Range == r.Range {
				matches = append(matches, b)
			}
		}
		if len(matches) != 1 {
			return nil, fail(CodeAmbiguousSelection, r.Caller, "relation selector must bind exactly one retained occurrence")
		}
		b := matches[0]
		receipt, ok := receipts[b.ReceiptID]
		if !ok || receipt.URI != b.CallerURI || receipt.ContentDigest != b.SourceDigest {
			return nil, fail(CodeReceiptMismatch, r.Caller, "relation receipt/source mismatch")
		}
		if seen[b.OccurrenceID] {
			return nil, fail(CodeAmbiguousSelection, r.Caller, "duplicate relation selector")
		}
		seen[b.OccurrenceID] = true
		out = append(out, RelationSelection{RelationID: b.RelationID, OccurrenceID: b.OccurrenceID, Caller: r.Caller, Callee: r.Callee, ReceiptID: b.ReceiptID, Source: sourceobject.Identity{Digest: b.SourceDigest, ByteLength: uint64(len(receipt.Content))}, PositionEncoding: admitted.parent.PositionEncoding, Range: b.Range, Direction: b.Direction, Provenance: b.Provenance})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OccurrenceID < out[j].OccurrenceID })
	for i := range out {
		out[i].Ordinal = i
	}
	return out, nil
}

func planSeal(admittedRaw, planBytes []byte) [32]byte {
	captureDigest := sha256.Sum256(admittedRaw)
	preimage := make([]byte, 0, len(PlanSealDomainV1)+1+len(captureDigest)+1+len(planBytes))
	preimage = append(preimage, PlanSealDomainV1...)
	preimage = append(preimage, 0)
	preimage = append(preimage, captureDigest[:]...)
	preimage = append(preimage, 0)
	preimage = append(preimage, planBytes...)
	return sha256.Sum256(preimage)
}

func lessKey(a, b Key) bool {
	if a.GraphSubjectID != b.GraphSubjectID {
		return a.GraphSubjectID < b.GraphSubjectID
	}
	return a.LogicalSourceID < b.LogicalSourceID
}
func validEncoding(value string) bool {
	return value == "utf-8" || value == "utf-16" || value == "utf-32"
}
func validRange(value graph.Range) bool { return beforeOrEqual(value.Start, value.End) }
func beforeOrEqual(a, b graph.Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character <= b.Character
}
func contains(outer, inner graph.Range) bool {
	return beforeOrEqual(outer.Start, inner.Start) && beforeOrEqual(inner.End, outer.End)
}
func setUniqueRange(dst **graph.Range, value graph.Range) error {
	if *dst == nil {
		copied := value
		*dst = &copied
		return nil
	}
	if **dst != value {
		return errors.New("conflicting range")
	}
	return nil
}
func includesReceipt(bindings []v5sourcesnapshot.Binding, receiptID string) bool {
	for _, binding := range bindings {
		found := false
		for _, id := range binding.ReceiptIDs {
			if id == receiptID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func allDigest(bindings []v5sourcesnapshot.Binding, digest string) bool {
	for _, binding := range bindings {
		if binding.SourceDigest != digest {
			return false
		}
	}
	return true
}
