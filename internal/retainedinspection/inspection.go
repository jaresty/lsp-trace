// Package retainedinspection owns the additive operation-41 retained projection contract.
package retainedinspection

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	hi "lsp-trace/internal/hydratedinspection"
)

const (
	Mode                        = "RETAINED_SOURCE_PROJECTION"
	InputSchemaID               = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-inspect-hydrated.v2.schema.json"
	InputSchemaV3ID             = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-inspect-hydrated.v3.schema.json"
	ArtifactEnvelopeSchemaID    = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-inspect-hydrated-artifact.v2.schema.json"
	ArtifactEnvelopeSchemaV3ID  = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-inspect-hydrated-artifact.v3.schema.json"
	DomainErrorEnvelopeSchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-inspect-hydrated-domain-error.v2.schema.json"
	SourceProjectionSchemaID    = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.source-projection.v2.schema.json"
	SourceProjectionSchemaV3ID  = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.source-projection.v3.schema.json"
	SourceSnapshotSchemaID      = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph-v5-source-snapshot.v2.schema.json"
	maxRequestBytes             = hi.MaxRequestBytes
)

type Key struct {
	GraphSubjectID  string `json:"graph_subject_id"`
	LogicalSourceID string `json:"logical_source_id"`
}
type Selection struct {
	Target     Key   `json:"target"`
	Selections []Key `json:"selections"`
}
type ProjectionLimits struct {
	MaxSourceBytes   uint64 `json:"max_source_bytes"`
	MaxRanges        uint64 `json:"max_ranges"`
	MaxObjects       uint64 `json:"max_objects"`
	MaxWork          uint64 `json:"max_work"`
	MaxResponseBytes uint64 `json:"max_response_bytes"`
}
type Paging struct {
	MaxPageBytes     uint64 `json:"max_page_bytes"`
	MaxPages         uint64 `json:"max_pages"`
	MaxResponseBytes uint64 `json:"max_response_bytes"`
	Cursor           string `json:"cursor,omitempty"`
}
type Projection struct {
	Body            string           `json:"body"`
	PrivacyPolicyID string           `json:"privacy_policy_id"`
	Limits          ProjectionLimits `json:"limits"`
	Paging          *Paging          `json:"paging,omitempty"`
}
type ResolveLimits struct {
	MaxDistinctObjects   uint64 `json:"max_distinct_objects"`
	MaxUniqueSourceBytes uint64 `json:"max_unique_source_bytes"`
	MaxLogicalSelections uint64 `json:"max_logical_selections"`
}
type PublicationSnapshot struct {
	Selector             string `json:"selector"`
	ArtifactDigest       string `json:"artifact_digest"`
	ArtifactByteLength   uint64 `json:"artifact_byte_length"`
	ArtifactSchemaID     string `json:"artifact_schema_id"`
	PublicationMechanism string `json:"publication_mechanism"`
	Generation           string `json:"generation"`
	VerificationSelector string `json:"verification_selector"`
}
type ContentAddressedSnapshot struct {
	ID                 string `json:"id"`
	ArtifactByteLength uint64 `json:"artifact_byte_length"`
	ArtifactSchemaID   string `json:"artifact_schema_id"`
	Generation         string `json:"generation"`
}
type Evidence struct {
	InlineSnapshotV2           string                    `json:"inline_snapshot_v2,omitempty"`
	PublicationSnapshotV2      *PublicationSnapshot      `json:"publication_snapshot_v2,omitempty"`
	ContentAddressedSnapshotV2 *ContentAddressedSnapshot `json:"content_addressed_snapshot_v2,omitempty"`
}
type Request struct {
	Mode                   string        `json:"mode"`
	RetainedSourceEvidence Evidence      `json:"retained_source_evidence"`
	Selection              Selection     `json:"selection"`
	Projection             Projection    `json:"projection"`
	ResolveLimits          ResolveLimits `json:"resolve_limits"`
}
type Decoded struct {
	Legacy     *hi.Request `json:"-"`
	Projection *Request    `json:"-"`
}

func (r Request) Check() error {
	if r.Mode != Mode {
		return errors.New("invalid projection mode")
	}
	carriers := 0
	if r.RetainedSourceEvidence.InlineSnapshotV2 != "" {
		carriers++
	}
	if r.RetainedSourceEvidence.PublicationSnapshotV2 != nil {
		carriers++
	}
	if r.RetainedSourceEvidence.ContentAddressedSnapshotV2 != nil {
		carriers++
	}
	if carriers != 1 {
		return errors.New("exactly one retained source evidence carrier required")
	}
	if len(r.Selection.Selections) < 1 || len(r.Selection.Selections) > 10000 || !validKey(r.Selection.Target) {
		return errors.New("target and selections required")
	}
	seen, target := map[Key]bool{}, false
	for _, k := range r.Selection.Selections {
		if !validKey(k) {
			return errors.New("invalid selection key")
		}
		if seen[k] {
			return errors.New("duplicate selection key")
		}
		seen[k] = true
		target = target || k == r.Selection.Target
	}
	if !target {
		return errors.New("target must be selected")
	}
	if r.Projection.Body != "OMIT" && r.Projection.Body != "INCLUDE" {
		return errors.New("invalid projection body")
	}
	if !canonicalDigest(r.Projection.PrivacyPolicyID) {
		return errors.New("canonical privacy policy digest required")
	}
	l := r.Projection.Limits
	if l.MaxSourceBytes > 16<<20 || l.MaxRanges > 10000 || l.MaxObjects > 10000 || l.MaxWork > 512<<20 || l.MaxResponseBytes < 1 || l.MaxResponseBytes > 16<<20 {
		return errors.New("projection limit out of range")
	}
	q := r.ResolveLimits
	if q.MaxDistinctObjects < 1 || q.MaxDistinctObjects > 10000 || q.MaxUniqueSourceBytes < 1 || q.MaxUniqueSourceBytes > 64<<20 || q.MaxLogicalSelections < 1 || q.MaxLogicalSelections > 10000 {
		return errors.New("resolve limit out of range")
	}
	if p := r.Projection.Paging; p != nil && (p.MaxPageBytes < 1 || p.MaxPageBytes > 1<<20 || p.MaxPages < 1 || p.MaxPages > 10000 || p.MaxResponseBytes < 1 || p.MaxResponseBytes > 64<<20 || len(p.Cursor) > 2048) {
		return errors.New("paging limit out of range")
	}
	return nil
}
func validKey(k Key) bool {
	if k.GraphSubjectID == "" || k.LogicalSourceID == "" {
		return false
	}
	u, e := url.Parse(k.LogicalSourceID)
	return e == nil && u.IsAbs()
}
func canonicalDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") || strings.ToLower(s) != s {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil
}

func Decode(raw []byte) (Decoded, error) {
	if err := ValidateInputJSONV3(raw); err != nil {
		return Decoded{}, err
	}
	var tag struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &tag); err != nil {
		return Decoded{}, err
	}
	if tag.Mode == Mode {
		var r Request
		if err := json.Unmarshal(raw, &r); err != nil {
			return Decoded{}, err
		}
		if err := r.Check(); err != nil {
			return Decoded{}, err
		}
		return Decoded{Projection: &r}, nil
	}
	r := hi.DefaultRequest()
	if err := json.Unmarshal(raw, &r); err != nil {
		return Decoded{}, err
	}
	return Decoded{Legacy: &r}, nil
}

func obj(p map[string]any, req ...string) map[string]any {
	if req == nil {
		req = []string{}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": p, "required": req}
}
func integer(min, max uint64) map[string]any {
	return map[string]any{"type": "integer", "minimum": min, "maximum": max}
}
func digest() map[string]any {
	return map[string]any{"type": "string", "pattern": "^sha256:[0-9a-f]{64}$"}
}
func generation() map[string]any {
	return map[string]any{"type": "string", "pattern": "^g-[0-9a-f]{64}$"}
}
func localRef(name string) map[string]any { return map[string]any{"$ref": "#/$defs/" + name} }
func cloneV1() map[string]any {
	var v map[string]any
	_ = json.Unmarshal(hi.InputSchema(), &v)
	delete(v, "$id")
	delete(v, "$schema")
	v["not"] = map[string]any{"required": []string{"mode"}}
	return v
}
func InputSchema() []byte {
	key := obj(map[string]any{"graph_subject_id": map[string]any{"type": "string", "minLength": 1}, "logical_source_id": map[string]any{"type": "string", "minLength": 1, "format": "uri"}}, "graph_subject_id", "logical_source_id")
	pub := obj(map[string]any{"selector": map[string]any{"type": "string", "minLength": 1}, "artifact_digest": localRef("d"), "artifact_byte_length": integer(1, 192<<20), "artifact_schema_id": map[string]any{"const": SourceSnapshotSchemaID}, "publication_mechanism": map[string]any{"const": "atomic_no_replace_with_verified_generation"}, "generation": localRef("g"), "verification_selector": map[string]any{"type": "string", "minLength": 1}}, "selector", "artifact_digest", "artifact_byte_length", "artifact_schema_id", "publication_mechanism", "generation", "verification_selector")
	ca := obj(map[string]any{"id": localRef("d"), "artifact_byte_length": integer(1, 192<<20), "artifact_schema_id": map[string]any{"const": SourceSnapshotSchemaID}, "generation": localRef("g")}, "id", "artifact_byte_length", "artifact_schema_id", "generation")
	evidence := obj(map[string]any{"inline_snapshot_v2": map[string]any{"type": "string", "minLength": 1, "maxLength": 1 << 20}, "publication_snapshot_v2": pub, "content_addressed_snapshot_v2": ca})
	delete(evidence, "required")
	evidence["oneOf"] = []any{map[string]any{"required": []string{"inline_snapshot_v2"}}, map[string]any{"required": []string{"publication_snapshot_v2"}}, map[string]any{"required": []string{"content_addressed_snapshot_v2"}}}
	selection := obj(map[string]any{"target": localRef("k"), "selections": map[string]any{"type": "array", "minItems": 1, "maxItems": 10000, "items": localRef("k")}}, "target", "selections")
	limits := obj(map[string]any{"max_source_bytes": integer(0, 16<<20), "max_ranges": integer(0, 10000), "max_objects": integer(0, 10000), "max_work": integer(0, 512<<20), "max_response_bytes": integer(1, 16<<20)}, "max_source_bytes", "max_ranges", "max_objects", "max_work", "max_response_bytes")
	projection := obj(map[string]any{"body": map[string]any{"enum": []string{"OMIT", "INCLUDE"}}, "privacy_policy_id": digest(), "limits": limits}, "body", "privacy_policy_id", "limits")
	resolve := obj(map[string]any{"max_distinct_objects": integer(1, 10000), "max_unique_source_bytes": integer(1, 64<<20), "max_logical_selections": integer(1, 10000)}, "max_distinct_objects", "max_unique_source_bytes", "max_logical_selections")
	branch := obj(map[string]any{"mode": map[string]any{"const": Mode}, "retained_source_evidence": evidence, "selection": selection, "projection": projection, "resolve_limits": resolve}, "mode", "retained_source_evidence", "selection", "projection", "resolve_limits")
	s := map[string]any{"$id": InputSchemaID, "type": "object", "$defs": map[string]any{"d": digest(), "g": generation(), "k": key}, "oneOf": []any{cloneV1(), branch}}
	b, _ := json.Marshal(s)
	return b
}

var states = []string{"ADMISSION_FAILED", "INVALID_REQUEST", "MISSING_SUBJECT_BINDING", "MISSING_DISPLAY_BINDING", "AMBIGUOUS_SELECTION", "RECEIPT_MISMATCH", "SOURCE_MISMATCH", "INVALID_RANGE", "INCOMPATIBLE_RANGE", "UNSUPPORTED_ENCODING", "INVALID_DISPLAY_PROVENANCE", "INVALID_PLAN", "RETURNED_IDENTITY_MISMATCH", "RESOLVED_LENGTH_MISMATCH", "RESOLVED_DIGEST_MISMATCH", "RESOLVE_DISTINCT_OBJECT_LIMIT", "RESOLVE_UNIQUE_SOURCE_BYTES_LIMIT", "RESOLVE_LOGICAL_SELECTION_LIMIT", "INVALID_RESOLVE_RESULT", "PROJECTION_FAILED", "ASSEMBLY_FAILED", "INVALID_IDENTITY", "MISSING", "CORRUPT", "POLICY", "PERMISSION", "LIMIT"}

// ErrorStates returns a defensive copy of the closed public error-state vocabulary.
func ErrorStates() []string { return append([]string(nil), states...) }

func ArtifactEnvelopeSchema() []byte {
	id := ArtifactEnvelopeSchemaID
	s := obj(map[string]any{"envelope_version": map[string]any{"const": "1"}, "envelope_schema_id": map[string]any{"const": id}, "tool": map[string]any{"const": "lsp_trace_v1_inspect_hydrated"}, "request_id": map[string]any{"type": "string", "minLength": 1}, "outcome": map[string]any{"const": "COMPLETE"}, "operation_status": map[string]any{"const": "SUCCEEDED"}, "isError": map[string]any{"const": false}, "artifact_schema_id": map[string]any{"const": SourceProjectionSchemaID}, "content": map[string]any{"type": "string"}}, "envelope_version", "envelope_schema_id", "tool", "request_id", "outcome", "operation_status", "isError", "artifact_schema_id", "content")
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["$id"] = id
	b, _ := json.Marshal(s)
	return b
}
func InputSchemaV3() []byte {
	var s map[string]any
	_ = json.Unmarshal(InputSchema(), &s)
	s["$id"] = InputSchemaV3ID
	branch := s["oneOf"].([]any)[1].(map[string]any)
	projection := branch["properties"].(map[string]any)["projection"].(map[string]any)
	paging := obj(map[string]any{
		"max_page_bytes": integer(1, 1<<20), "max_pages": integer(1, 10000),
		"max_response_bytes": integer(1, 64<<20), "cursor": map[string]any{"type": "string", "minLength": 1, "maxLength": 2048},
	}, "max_page_bytes", "max_pages", "max_response_bytes")
	projection["properties"].(map[string]any)["paging"] = paging
	b, _ := json.Marshal(s)
	return b
}

func ArtifactEnvelopeSchemaV3() []byte {
	var s map[string]any
	_ = json.Unmarshal(ArtifactEnvelopeSchema(), &s)
	s["$id"] = ArtifactEnvelopeSchemaV3ID
	p := s["properties"].(map[string]any)
	p["envelope_schema_id"] = map[string]any{"const": ArtifactEnvelopeSchemaV3ID}
	p["artifact_schema_id"] = map[string]any{"const": SourceProjectionSchemaV3ID}
	b, _ := json.Marshal(s)
	return b
}

func DomainErrorEnvelopeSchema() []byte {
	id := DomainErrorEnvelopeSchemaID
	diag := map[string]any{"type": "array", "maxItems": 64, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}}
	key := obj(map[string]any{"graph_subject_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}, "logical_source_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 4096, "format": "uri"}}, "graph_subject_id", "logical_source_id")
	s := obj(map[string]any{"envelope_version": map[string]any{"const": "1"}, "envelope_schema_id": map[string]any{"const": id}, "tool": map[string]any{"const": "lsp_trace_v1_inspect_hydrated"}, "request_id": map[string]any{"type": "string", "minLength": 1}, "outcome": map[string]any{"const": "DOMAIN_ERROR"}, "operation_status": map[string]any{"const": "FAILED"}, "isError": map[string]any{"const": true}, "phase": map[string]any{"enum": []string{"DECODE", "INGRESS", "ADMIT", "SELECT", "RESOLVE", "PROJECT", "ASSEMBLE"}}, "state": map[string]any{"enum": states}, "selection_key": key, "diagnostics": diag}, "envelope_version", "envelope_schema_id", "tool", "request_id", "outcome", "operation_status", "isError", "phase", "state")
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["$id"] = id
	b, _ := json.Marshal(s)
	return b
}

var once sync.Once
var compiled *jsonschema.Schema
var compileErr error
var onceV3 sync.Once
var compiledV3 *jsonschema.Schema
var compileErrV3 error

func ValidateInputJSONV3(raw []byte) error {
	if len(raw) == 0 || len(raw) > maxRequestBytes {
		return errors.New("request byte limit")
	}
	onceV3.Do(func() {
		c := jsonschema.NewCompiler()
		doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(InputSchemaV3()))
		if e == nil {
			e = c.AddResource(InputSchemaV3ID, doc)
		}
		if e == nil {
			compiledV3, e = c.Compile(InputSchemaV3ID)
		}
		compileErrV3 = e
	})
	if compileErrV3 != nil {
		return compileErrV3
	}
	doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if e != nil {
		return e
	}
	if e = compiledV3.Validate(doc); e != nil {
		return fmt.Errorf("schema validation: %w", e)
	}
	return nil
}

func ValidateInputJSON(raw []byte) error {
	if len(raw) == 0 || len(raw) > maxRequestBytes {
		return errors.New("request byte limit")
	}
	once.Do(func() {
		c := jsonschema.NewCompiler()
		doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(InputSchema()))
		if e == nil {
			e = c.AddResource(InputSchemaID, doc)
		}
		if e == nil {
			compiled, e = c.Compile(InputSchemaID)
		}
		compileErr = e
	})
	if compileErr != nil {
		return compileErr
	}
	doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if e != nil {
		return e
	}
	if e = compiled.Validate(doc); e != nil {
		return fmt.Errorf("schema validation: %w", e)
	}
	return nil
}
