package graph

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const NormalizedRelationsSchemaVersion = "lsp-trace.graph.v4"
const NormalizedRelationsArtifactKind = "NORMALIZED_RELATIONS"
const normalizedRelationsSchemaID = "https://lsp-trace.dev/schemas/normalized-relations/v1/schema.json"

const (
	RelationCalls          = "CALLS"
	RelationBindsArgument  = "BINDS_ARGUMENT"
	RelationPassesCallback = "PASSES_CALLBACK"
	RelationInvokesTask    = "INVOKES_TASK"
	RelationTriggersReload = "TRIGGERS_RELOAD"
	RelationUpdatesState   = "UPDATES_STATE"
	RelationRendersFrom    = "RENDERS_FROM"

	EvidenceServerReported = "SERVER_REPORTED"
	EvidenceSourceAdapter  = "SOURCE_DERIVED_ADAPTER"
	EvidenceCallerAsserted = "CALLER_ASSERTED"
)

var relationKinds = map[string]struct{}{
	RelationCalls: {}, RelationBindsArgument: {}, RelationPassesCallback: {}, RelationInvokesTask: {},
	RelationTriggersReload: {}, RelationUpdatesState: {}, RelationRendersFrom: {},
}

//go:embed schemas/normalized-relations.v1.schema.json
var normalizedRelationsSchema []byte

type RelationAdapter struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RelationAnchor struct {
	URI      string `json:"uri"`
	Revision string `json:"revision"`
	Blob     string `json:"blob"`
	Range    Range  `json:"range"`
}

type Relation struct {
	RelationID                 string           `json:"relation_id"`
	Kind                       string           `json:"kind"`
	From                       string           `json:"from"`
	To                         string           `json:"to"`
	EvidenceClass              string           `json:"evidence_class"`
	Adapter                    *RelationAdapter `json:"adapter,omitempty"`
	Anchors                    []RelationAnchor `json:"anchors"`
	Confidence                 string           `json:"confidence"`
	Supports                   []string         `json:"supports"`
	DoesNotSupport             []string         `json:"does_not_support"`
	ContributingObservationIDs []string         `json:"contributing_observation_ids"`
}

// NormalizedRelations is a detached graph-v4 projection. Legacy Result bytes are unaffected.
type NormalizedRelations struct {
	SchemaVersion string                         `json:"schema_version"`
	ArtifactKind  string                         `json:"artifact_kind"`
	Relations     []Relation                     `json:"relations"`
	Provenance    *NormalizedRelationsProvenance `json:"provenance,omitempty"`
}

// NormalizedRelationsProvenance retains provider-neutral acceptance evidence.
// Raw fields preserve their owning typed contracts without importing provider or
// framework semantics into the graph package.
type NormalizedRelationsProvenance struct {
	Diagnostics    []Diagnostic    `json:"diagnostics,omitempty"`
	ProviderID     string          `json:"provider_id"`
	Provider       json.RawMessage `json:"provider"`
	Protocol       json.RawMessage `json:"protocol"`
	Adapter        json.RawMessage `json:"adapter"`
	Coverage       json.RawMessage `json:"coverage"`
	Bounds         json.RawMessage `json:"bounds"`
	Custody        json.RawMessage `json:"custody"`
	ObservationIDs []string        `json:"observation_ids"`
	LogicalDigest  string          `json:"logical_digest"`
	Receipt        json.RawMessage `json:"receipt"`
}

// NewRelation canonicalizes identity-bearing collections and assigns the relation identity.
func NewRelation(relation Relation) Relation {
	relation.Anchors = canonicalAnchors(relation.Anchors)
	relation.Supports = canonicalStrings(relation.Supports)
	relation.DoesNotSupport = canonicalStrings(relation.DoesNotSupport)
	relation.ContributingObservationIDs = canonicalStrings(relation.ContributingObservationIDs)
	relation.RelationID = relationIdentity(relation)
	return relation
}

// NormalizeRelations projects only server-reported legacy call edges as CALLS.
func NormalizeRelations(result Result) NormalizedRelations {
	nodes := make(map[string]Node, len(result.Nodes))
	for _, node := range result.Nodes {
		nodes[node.ID] = node
	}
	relations := make([]Relation, 0, len(result.Edges))
	for _, edge := range result.Edges {
		anchors := make([]RelationAnchor, 0, len(edge.CallSites))
		uri := nodes[edge.CallerNodeID].URI
		for _, callSite := range edge.CallSites {
			anchors = append(anchors, RelationAnchor{URI: canonicalRelationURI(uri), Revision: Unknown, Blob: Unknown, Range: callSite})
		}
		observationPayload, _ := json.Marshal(struct {
			Kind    string           `json:"kind"`
			From    string           `json:"from"`
			To      string           `json:"to"`
			Anchors []RelationAnchor `json:"anchors"`
		}{RelationCalls, edge.CallerNodeID, edge.CalleeNodeID, canonicalAnchors(anchors)})
		relations = append(relations, NewRelation(Relation{
			Kind: RelationCalls, From: edge.CallerNodeID, To: edge.CalleeNodeID,
			EvidenceClass: EvidenceServerReported, Anchors: anchors, Confidence: "EXACT",
			Supports:                   []string{"source_dependency_relation"},
			DoesNotSupport:             []string{"runtime_execution", "whole_source_completeness"},
			ContributingObservationIDs: []string{digest("lsp-trace:observation:server-call:v1", observationPayload)},
		}))
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].RelationID < relations[j].RelationID })
	return NormalizedRelations{SchemaVersion: NormalizedRelationsSchemaVersion, ArtifactKind: NormalizedRelationsArtifactKind, Relations: relations}
}

func relationIdentity(relation Relation) string {
	identity := struct {
		SchemaVersion string           `json:"schema_version"`
		Kind          string           `json:"kind"`
		From          string           `json:"from"`
		To            string           `json:"to"`
		Anchors       []RelationAnchor `json:"anchors"`
		EvidenceClass string           `json:"evidence_class"`
		Adapter       *RelationAdapter `json:"adapter,omitempty"`
		Observations  []string         `json:"contributing_observation_ids"`
	}{NormalizedRelationsSchemaVersion, relation.Kind, relation.From, relation.To, canonicalAnchors(relation.Anchors), relation.EvidenceClass, relation.Adapter, canonicalStrings(relation.ContributingObservationIDs)}
	encoded, _ := json.Marshal(identity)
	return digest("lsp-trace:normalized-relation:v1", encoded)
}

func digest(domain string, payload []byte) string {
	sum := sha256.Sum256(append(append([]byte(domain), 0), payload...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func canonicalAnchors(in []RelationAnchor) []RelationAnchor {
	out := append([]RelationAnchor(nil), in...)
	for i := range out {
		out[i].URI = canonicalRelationURI(out[i].URI)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		if a.Revision != b.Revision {
			return a.Revision < b.Revision
		}
		if a.Blob != b.Blob {
			return a.Blob < b.Blob
		}
		return lessRange(a.Range, b.Range)
	})
	return out
}

func canonicalStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	if len(out) == 0 {
		return out
	}
	unique := out[:1]
	for _, value := range out[1:] {
		if value != unique[len(unique)-1] {
			unique = append(unique, value)
		}
	}
	return unique
}

func canonicalRelationURI(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return u.String()
}

func NormalizedRelationsSchemaJSON() []byte { return append([]byte(nil), normalizedRelationsSchema...) }

func ValidateNormalizedRelationsJSON(data []byte) error {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(normalizedRelationsSchema))
	if err != nil {
		return fmt.Errorf("decode normalized relations schema: %w", err)
	}
	if err := compiler.AddResource(normalizedRelationsSchemaID, document); err != nil {
		return fmt.Errorf("register normalized relations schema: %w", err)
	}
	compiled, err := compiler.Compile(normalizedRelationsSchemaID)
	if err != nil {
		return fmt.Errorf("compile normalized relations schema: %w", err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid normalized relations JSON: %w", err)
	}
	if err := compiled.Validate(value); err != nil {
		return fmt.Errorf("normalized relations schema validation: %w", err)
	}
	var artifact NormalizedRelations
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return fmt.Errorf("decode normalized relations: %w", err)
	}
	seen := map[string]struct{}{}
	for _, relation := range artifact.Relations {
		if err := validateRelation(relation); err != nil {
			return err
		}
		if _, duplicate := seen[relation.RelationID]; duplicate {
			return fmt.Errorf("duplicate relation id %q", relation.RelationID)
		}
		seen[relation.RelationID] = struct{}{}
	}
	return nil
}

func validateRelation(relation Relation) error {
	if _, ok := relationKinds[relation.Kind]; !ok {
		return fmt.Errorf("unknown normalized relation kind %q", relation.Kind)
	}
	if relation.From == "" || relation.To == "" {
		return fmt.Errorf("normalized relation has empty endpoint")
	}
	if relation.EvidenceClass != EvidenceServerReported && relation.EvidenceClass != EvidenceSourceAdapter && relation.EvidenceClass != EvidenceCallerAsserted {
		return fmt.Errorf("unknown evidence class %q", relation.EvidenceClass)
	}
	if relation.Kind == RelationCalls && relation.EvidenceClass != EvidenceServerReported {
		return fmt.Errorf("CALLS must be SERVER_REPORTED")
	}
	if relation.EvidenceClass == EvidenceSourceAdapter && (relation.Adapter == nil || relation.Adapter.Name == "" || relation.Adapter.Version == "") {
		return fmt.Errorf("source-derived relation requires named versioned adapter")
	}
	if relation.EvidenceClass != EvidenceSourceAdapter && relation.Adapter != nil {
		return fmt.Errorf("adapter is only valid for source-derived evidence")
	}
	if len(relation.Anchors) == 0 || len(relation.ContributingObservationIDs) == 0 {
		return fmt.Errorf("normalized relation requires anchors and contributing observations")
	}
	if relation.Confidence != "EXACT" {
		return fmt.Errorf("unsupported confidence %q", relation.Confidence)
	}
	for _, anchor := range relation.Anchors {
		if anchor.URI == "" || anchor.Revision == "" || anchor.Blob == "" {
			return fmt.Errorf("relation anchor lacks custody")
		}
		if err := ValidateRange(anchor.Range); err != nil {
			return err
		}
	}
	if !hasString(relation.Supports, "source_dependency_relation") || !hasString(relation.DoesNotSupport, "runtime_execution") || !hasString(relation.DoesNotSupport, "whole_source_completeness") {
		return fmt.Errorf("relation support/non-entailment contract mismatch")
	}
	if relation.Kind == RelationPassesCallback && !hasString(relation.DoesNotSupport, "callback_invocation") {
		return fmt.Errorf("PASSES_CALLBACK must disclaim callback_invocation")
	}
	if relation.Kind == RelationUpdatesState && !hasString(relation.DoesNotSupport, "render_occurrence") {
		return fmt.Errorf("UPDATES_STATE must disclaim render_occurrence")
	}
	if relation.Kind == RelationRendersFrom && !hasString(relation.DoesNotSupport, "repaint") {
		return fmt.Errorf("RENDERS_FROM must disclaim repaint")
	}
	if relation.RelationID != relationIdentity(relation) {
		return fmt.Errorf("invalid deterministic relation id %q", relation.RelationID)
	}
	if !equalStrings(relation.ContributingObservationIDs, canonicalStrings(relation.ContributingObservationIDs)) || !equalAnchors(relation.Anchors, canonicalAnchors(relation.Anchors)) {
		return fmt.Errorf("relation identity inputs are not canonical")
	}
	return nil
}

func hasString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func equalAnchors(a, b []RelationAnchor) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
