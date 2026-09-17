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

type Request struct {
	Target     Key
	Selections []Key
}

// Admitted is opaque outside this package. Admit is the only supported way to
// construct it from an externally supplied V2 artifact.
type Admitted struct {
	artifact v5sourcesnapshotv2.Artifact
	parent   v5sourcesnapshot.Artifact
	raw      []byte
}

func Admit(raw []byte) (Admitted, error) {
	if _, err := v5sourcesnapshotv2.Validate(raw); err != nil {
		return Admitted{}, fail(CodeAdmission, Key{}, err.Error())
	}
	var artifact v5sourcesnapshotv2.Artifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return Admitted{}, fail(CodeAdmission, Key{}, err.Error())
	}
	var parent v5sourcesnapshot.Artifact
	if err := json.Unmarshal(artifact.ParentSnapshot, &parent); err != nil {
		return Admitted{}, fail(CodeAdmission, Key{}, err.Error())
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

type Plan struct {
	Ordering   string      `json:"ordering"`
	Target     Key         `json:"target"`
	Selections []Selection `json:"selections"`
	binding    [32]byte
}

func (p Plan) Bytes() ([]byte, error) { return json.Marshal(p) }

const (
	RetainedCustody                 = "RETAINED"
	ImmutableSourceObjectIdentityV1 = "IMMUTABLE_SOURCE_OBJECT_IDENTITY_V1"

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
	for _, binding := range admitted.artifact.DisplayBindings {
		key := Key{binding.GraphSubjectID, binding.LogicalSourceID}
		displays[key] = append(displays[key], binding)
	}
	parentBindings := map[Key][]v5sourcesnapshot.Binding{}
	for _, binding := range admitted.parent.Bindings {
		key := Key{binding.NodeID, binding.URI}
		parentBindings[key] = append(parentBindings[key], binding)
	}
	receipts := map[string][]v5sourcesnapshot.Receipt{}
	for _, receipt := range admitted.parent.Receipts {
		receipts[receipt.ID] = append(receipts[receipt.ID], receipt)
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
	plan := Plan{Ordering: Ordering, Target: request.Target, Selections: selections}
	planBytes, err := plan.Bytes()
	if err != nil {
		return zero, fail(CodeInvalidPlan, Key{}, err.Error())
	}
	plan.binding = planSeal(admitted.raw, planBytes)
	return plan, nil
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
