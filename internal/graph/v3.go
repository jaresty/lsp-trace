package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
)

const (
	SemanticDigestDomain = "lsp-trace:semantic-bundle:v3"
	SemanticDigestScope  = "CANONICAL_SEMANTIC_BUNDLE_WITHOUT_RECEIPT"
	ByteDigestDomain     = "lsp-trace:serialized-output-bytes:v1"
	ByteDigestScope      = "EXACT_SERIALIZED_OUTPUT_BYTES"
	ResolvedSeedsDomain  = "lsp-trace:resolved-seed-contents:v1"
)

type BundleIdentity struct {
	CallerProvenanceClass string           `json:"caller_provenance_class"`
	ResolvedSeeds         []InvocationSeed `json:"resolved_seeds"`
	AggregateFingerprint  string           `json:"aggregate_fingerprint,omitempty"`
	AggregateScope        string           `json:"aggregate_scope"`
}
type SensitivityPolicy struct {
	Covered                           []string `json:"covered"`
	AutomaticRedaction                bool     `json:"automatic_redaction"`
	AccessControlResponsibility       string   `json:"access_control_responsibility"`
	AmbientProcessEnvironmentRecorded bool     `json:"ambient_process_environment_recorded"`
}
type summaryV3 struct {
	NodeCount           int    `json:"node_count"`
	EdgeCount           int    `json:"edge_count"`
	TerminalCount       int    `json:"terminal_count"`
	CycleCount          int    `json:"cycle_count"`
	TraversalComplete   bool   `json:"traversal_complete"`
	SourceGraphComplete string `json:"source_graph_complete"`
	CompletenessScope   string `json:"completeness_scope"`
	Truncated           bool   `json:"truncated"`
}
type semanticV3 struct {
	SchemaVersion         string                 `json:"schema_version"`
	Tool                  ToolIdentity           `json:"tool"`
	Invocation            Invocation             `json:"invocation"`
	Identity              BundleIdentity         `json:"identity"`
	SensitivityPolicy     SensitivityPolicy      `json:"sensitivity_policy"`
	EvidenceSemantics     EvidenceSemantics      `json:"evidence_semantics"`
	Capabilities          Capabilities           `json:"capabilities"`
	CapabilityQuality     CapabilityQuality      `json:"capability_quality"`
	Targets               []string               `json:"targets"`
	Nodes                 []Node                 `json:"nodes"`
	Edges                 []Edge                 `json:"edges"`
	Terminals             []Boundary             `json:"terminals"`
	Frontier              []Boundary             `json:"frontier"`
	Diagnostics           []Diagnostic           `json:"diagnostics"`
	SiblingCandidates     []SiblingCandidate     `json:"sibling_candidates,omitempty"`
	DispatchRelationships []DispatchRelationship `json:"dispatch_relationships,omitempty"`
	Seeds                 []SeedResult           `json:"seeds,omitempty"`
	Summary               summaryV3              `json:"summary"`
}
type bundleV3 struct {
	semanticV3
	TraceReceipt TraceReceipt `json:"trace_receipt"`
}

func (r Result) marshalV3() ([]byte, error) {
	if err := r.ValidateReferences(); err != nil {
		return nil, err
	}
	inv := normalizedInvocation(r.Invocation)
	tool := r.Tool
	if tool.Name == "" {
		tool.Name = "lsp-trace"
	}
	if tool.Version == "" {
		tool.Version = Unknown
	}
	resolved := make([]InvocationSeed, 0)
	for _, s := range inv.Seeds {
		if s.ResolvedURI != "" && s.ContentSHA256 != "" {
			resolved = append(resolved, s)
		}
	}
	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].ResolvedURI != resolved[j].ResolvedURI {
			return resolved[i].ResolvedURI < resolved[j].ResolvedURI
		}
		return resolved[i].Label < resolved[j].Label
	})
	aggInput, _ := json.Marshal(resolved)
	identity := BundleIdentity{CallerProvenanceClass: "CALLER_ASSERTED", ResolvedSeeds: resolved, AggregateScope: "RESOLVED_SEED_CONTENTS"}
	if len(resolved) > 0 {
		identity.AggregateFingerprint = domainDigest(ResolvedSeedsDomain, aggInput)
	}
	sem := semanticV3{r.SchemaVersion, tool, inv, identity, SensitivityPolicy{[]string{"invocation_arguments", "explicit_environment_values", "workspace_source_output_paths", "opaque_node_data", "diagnostics", "captured_server_stderr", "trace_transcripts"}, false, "BUNDLE_CUSTODIAN", false}, evidenceSemantics(), r.Capabilities, r.CapabilityQuality, r.Targets, r.Nodes, r.Edges, r.Terminals, r.Frontier, r.Diagnostics, r.SiblingCandidates, r.DispatchRelationships, r.Seeds, summaryV3{r.Summary.NodeCount, r.Summary.EdgeCount, r.Summary.TerminalCount, r.Summary.CycleCount, r.Summary.Complete, Unknown, CompletenessScope, r.Summary.Truncated}}
	canonical, err := json.Marshal(sem)
	if err != nil {
		return nil, err
	}
	return json.Marshal(bundleV3{sem, TraceReceipt{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}})
}

func (r Result) ValidateReferences() error {
	ids := map[string]struct{}{}
	for _, n := range r.Nodes {
		if _, ok := ids[n.ID]; ok {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		ids[n.ID] = struct{}{}
	}
	need := func(kind, id string) error {
		if id == "" {
			return nil
		}
		if _, ok := ids[id]; !ok {
			return fmt.Errorf("dangling %s node id %q", kind, id)
		}
		return nil
	}
	for _, id := range r.Targets {
		if err := need("target", id); err != nil {
			return err
		}
	}
	for _, e := range r.Edges {
		if err := need("edge caller", e.CallerNodeID); err != nil {
			return err
		}
		if err := need("edge callee", e.CalleeNodeID); err != nil {
			return err
		}
	}
	for _, b := range append(append([]Boundary{}, r.Terminals...), r.Frontier...) {
		if err := need("boundary", b.NodeID); err != nil {
			return err
		}
	}
	for _, s := range r.Seeds {
		for _, id := range append(append([]string{}, s.PreparedTargetIDs...), s.ReachedNodeIDs...) {
			if err := need("seed", id); err != nil {
				return err
			}
		}
	}
	// Discovery candidates and dispatch endpoints are self-contained full Nodes. Validate
	// their IDs against their embedded identity; they intentionally need not be call Nodes.
	checkEmbedded := func(kind string, n Node) error {
		if n.ID == "" || NewNode(n.Item).ID != n.ID {
			return fmt.Errorf("invalid embedded %s node id %q", kind, n.ID)
		}
		return nil
	}
	for _, c := range r.SiblingCandidates {
		if err := checkEmbedded("sibling candidate", c.Candidate); err != nil {
			return err
		}
	}
	for _, d := range r.DispatchRelationships {
		if err := checkEmbedded("dispatch interface", d.Interface); err != nil {
			return err
		}
		if err := checkEmbedded("dispatch implementation", d.Implementation); err != nil {
			return err
		}
	}
	return nil
}

func ValidateSemanticBundle(data []byte) error {
	var b bundleV3
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&b); err != nil {
		return fmt.Errorf("malformed bundle: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("malformed bundle: trailing JSON content")
	}
	if b.SchemaVersion != SchemaVersionV3 {
		return fmt.Errorf("verification requires %s", SchemaVersionV3)
	}
	if err := (Result{
		Targets: b.Targets, Nodes: b.Nodes, Edges: b.Edges, Terminals: b.Terminals,
		Frontier: b.Frontier, Seeds: b.Seeds, SiblingCandidates: b.SiblingCandidates,
		DispatchRelationships: b.DispatchRelationships,
	}).ValidateReferences(); err != nil {
		return err
	}
	resolved := make([]InvocationSeed, 0)
	for _, seed := range b.Invocation.Seeds {
		if seed.ResolvedURI != "" && seed.ContentSHA256 != "" {
			resolved = append(resolved, seed)
		}
	}
	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].ResolvedURI != resolved[j].ResolvedURI {
			return resolved[i].ResolvedURI < resolved[j].ResolvedURI
		}
		return resolved[i].Label < resolved[j].Label
	})
	aggregate := ""
	if len(resolved) > 0 {
		input, _ := json.Marshal(resolved)
		aggregate = domainDigest(ResolvedSeedsDomain, input)
	}
	if b.Identity.CallerProvenanceClass != "CALLER_ASSERTED" || b.Identity.AggregateScope != "RESOLVED_SEED_CONTENTS" || b.Identity.AggregateFingerprint != aggregate || !reflect.DeepEqual(b.Identity.ResolvedSeeds, resolved) {
		return fmt.Errorf("bundle identity mismatch")
	}
	receipt := b.TraceReceipt
	b.TraceReceipt = TraceReceipt{}
	canonical, err := json.Marshal(b.semanticV3)
	if err != nil {
		return err
	}
	if receipt.ReceiptVersion != "lsp-trace.semantic-receipt.v1" || receipt.DigestScope != SemanticDigestScope || receipt.ContentDigest != domainDigest(SemanticDigestDomain, canonical) {
		return fmt.Errorf("embedded semantic receipt mismatch")
	}
	return nil
}

func ExactBytesDigest(data []byte) string { return domainDigest(ByteDigestDomain, data) }
