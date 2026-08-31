package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sort"
	"strings"
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
type Redaction struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}
type EnvironmentIdentity struct {
	Name      string    `json:"name"`
	Identity  string    `json:"identity"`
	Redaction Redaction `json:"redaction"`
}
type ProcessContext struct {
	Environment                       []EnvironmentIdentity `json:"environment"`
	EffectiveEnvironmentIdentity      string                `json:"effective_environment_identity"`
	EffectiveEnvironmentVariableCount int                   `json:"effective_environment_variable_count"`
	WorkingDirectoryIdentity          string                `json:"working_directory_identity"`
	AmbientEnvironmentState           string                `json:"ambient_environment_state"`
	Redaction                         Redaction             `json:"redaction"`
}
type SeedMembership struct {
	MembershipID string `json:"membership_id"`
	SeedLabel    string `json:"seed_label"`
	SeedAt       string `json:"seed_at"`
	EvidenceKind string `json:"evidence_kind"`
	EndpointID   string `json:"endpoint_id"`
}
type ReplayArtifact struct {
	Kind      string    `json:"kind"`
	Locator   string    `json:"locator"`
	State     string    `json:"state"`
	Digest    string    `json:"digest,omitempty"`
	Redaction Redaction `json:"redaction"`
}
type ReplayInputManifest struct {
	ManifestID string           `json:"manifest_id"`
	Artifacts  []ReplayArtifact `json:"artifacts"`
}
type PortableLocator struct {
	NodeID    string    `json:"node_id"`
	Locator   string    `json:"locator"`
	Redaction Redaction `json:"redaction"`
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
	ProcessContext        ProcessContext         `json:"process_context"`
	EvidenceSemantics     EvidenceSemantics      `json:"evidence_semantics"`
	EvidenceReceipt       *EvidenceReceipt       `json:"evidence_receipt,omitempty"`
	SeedMemberships       []SeedMembership       `json:"seed_memberships"`
	ReplayInputManifest   ReplayInputManifest    `json:"replay_input_manifest"`
	PortableLocators      []PortableLocator      `json:"portable_locators"`
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
	processContext := projectProcessContext(inv.Server.Environment, inv.EffectiveEnvironment, inv.WorkingDirectory)
	inv.Server.Environment = nil
	sem := semanticV3{
		SchemaVersion: r.SchemaVersion, Tool: tool, Invocation: inv, Identity: identity,
		SensitivityPolicy: SensitivityPolicy{[]string{"invocation_arguments", "explicit_environment_names", "workspace_source_output_paths", "opaque_node_data", "diagnostics", "captured_server_stderr", "trace_transcripts"}, true, "BUNDLE_CUSTODIAN", false},
		ProcessContext:    processContext, EvidenceSemantics: evidenceSemantics(), EvidenceReceipt: r.evidenceReceipt(inv.Provenance.SourceRevision),
		SeedMemberships: projectSeedMemberships(inv.Seeds, r.Seeds, r.SiblingCandidates, r.DispatchRelationships, inv.Provenance.SourceRevision), ReplayInputManifest: projectReplayManifest(inv, r.Diagnostics), PortableLocators: projectPortableLocators(r.Nodes),
		Capabilities: r.Capabilities, CapabilityQuality: r.CapabilityQuality, Targets: r.Targets, Nodes: r.Nodes, Edges: r.Edges,
		Terminals: r.Terminals, Frontier: r.Frontier, Diagnostics: r.Diagnostics, SiblingCandidates: r.SiblingCandidates,
		DispatchRelationships: r.DispatchRelationships, Seeds: r.Seeds,
		Summary: summaryV3{r.Summary.NodeCount, r.Summary.EdgeCount, r.Summary.TerminalCount, r.Summary.CycleCount, r.Summary.Complete, Unknown, CompletenessScope, r.Summary.Truncated},
	}
	canonical, err := json.Marshal(sem)
	if err != nil {
		return nil, err
	}
	return json.Marshal(bundleV3{sem, TraceReceipt{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}})
}

func projectProcessContext(environment map[string]string, effectiveEnvironment []string, workingDirectory string) ProcessContext {
	names := make([]string, 0, len(environment))
	for name := range environment {
		names = append(names, name)
	}
	sort.Strings(names)
	effective := normalizedEnvironment(effectiveEnvironment)
	encoded, _ := json.Marshal(effective)
	out := ProcessContext{
		Environment:                       make([]EnvironmentIdentity, 0, len(names)),
		EffectiveEnvironmentIdentity:      domainDigest("lsp-trace:effective-environment:v1", encoded),
		EffectiveEnvironmentVariableCount: len(effective),
		WorkingDirectoryIdentity:          domainDigest("lsp-trace:working-directory:v1", []byte(workingDirectory)),
		AmbientEnvironmentState:           "IDENTIFIED_NOT_EMBEDDED",
		Redaction:                         Redaction{State: "REDACTED", Reason: "EFFECTIVE_PROCESS_CONTEXT_NOT_EMBEDDED"},
	}
	for _, name := range names {
		out.Environment = append(out.Environment, EnvironmentIdentity{Name: name, Identity: domainDigest("lsp-trace:environment-name:v1", []byte(name)), Redaction: Redaction{State: "REDACTED", Reason: "SECRET_SAFE_ENVIRONMENT_IDENTITY"}})
	}
	return out
}

func normalizedEnvironment(environment []string) []string {
	values := map[string]string{}
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name != "" {
			values[name] = value
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name+"="+values[name])
	}
	return out
}

func validateProcessContext(context ProcessContext) error {
	if context.AmbientEnvironmentState != "IDENTIFIED_NOT_EMBEDDED" || context.EffectiveEnvironmentVariableCount < 0 || !strings.HasPrefix(context.EffectiveEnvironmentIdentity, "sha256:") || !strings.HasPrefix(context.WorkingDirectoryIdentity, "sha256:") || context.Redaction != (Redaction{State: "REDACTED", Reason: "EFFECTIVE_PROCESS_CONTEXT_NOT_EMBEDDED"}) {
		return fmt.Errorf("process context mismatch")
	}
	last := ""
	for _, identity := range context.Environment {
		if identity.Name == "" || identity.Name < last || identity.Identity != domainDigest("lsp-trace:environment-name:v1", []byte(identity.Name)) || identity.Redaction != (Redaction{State: "REDACTED", Reason: "SECRET_SAFE_ENVIRONMENT_IDENTITY"}) {
			return fmt.Errorf("process context mismatch")
		}
		last = identity.Name
	}
	return nil
}

func projectSeedMemberships(invocationSeeds []InvocationSeed, results []SeedResult, siblings []SiblingCandidate, dispatches []DispatchRelationship, sourceRevision string) []SeedMembership {
	byLabel := make(map[string]InvocationSeed, len(invocationSeeds))
	for _, seed := range invocationSeeds {
		byLabel[seed.Label] = seed
	}
	out := make([]SeedMembership, 0)
	for _, result := range results {
		seed := byLabel[result.Label]
		for _, endpoint := range result.PreparedTargetIDs {
			out = append(out, newSeedMembership(seed, "PREPARED_TARGET", endpoint))
		}
		for _, endpoint := range result.ReachedNodeIDs {
			out = append(out, newSeedMembership(seed, "REACHED_NODE", endpoint))
		}
		for _, edge := range result.ReachedEdges {
			relation := newEvidenceRelation("CALL_RELATION", "CALLER_TO_CALLEE", edge.CallerNodeID+"->"+edge.CalleeNodeID, sourceRevision, "", "", "", "", "", edge.CallerNodeID, edge.CalleeNodeID)
			out = append(out, newSeedMembership(seed, "CALL_RELATION", relation.RelationID))
		}
	}
	for _, sibling := range siblings {
		out = append(out, newSeedMembership(byLabel[sibling.SeedLabel], "SIBLING_CANDIDATE", sibling.Candidate.ID))
	}
	for _, dispatch := range dispatches {
		endpoint := dispatch.Interface.ID + "->" + dispatch.Implementation.ID
		out = append(out, newSeedMembership(byLabel[dispatch.SeedLabel], "DISPATCH_ASSOCIATION", endpoint))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MembershipID < out[j].MembershipID })
	return out
}

func newSeedMembership(seed InvocationSeed, kind, endpoint string) SeedMembership {
	payload, _ := json.Marshal([]string{seed.Label, seed.At, seed.ResolvedURI, kind, endpoint})
	return SeedMembership{MembershipID: domainDigest("lsp-trace:seed-membership:v1", payload), SeedLabel: seed.Label, SeedAt: seed.At, EvidenceKind: kind, EndpointID: endpoint}
}

func projectReplayManifest(inv Invocation, diagnostics []Diagnostic) ReplayInputManifest {
	artifacts := make([]ReplayArtifact, 0, len(inv.Seeds)+2)
	for _, seed := range inv.Seeds {
		state := "ABSENT"
		if seed.ContentSHA256 != "" {
			state = "PRESENT"
		}
		artifacts = append(artifacts, ReplayArtifact{Kind: "SOURCE_ARTIFACT", Locator: canonicalURI(seed.ResolvedURI), State: state, Digest: seed.ContentSHA256, Redaction: Redaction{State: "VISIBLE", Reason: "REPLAY_IDENTITY"}})
	}
	traceState, traceDigest := "ABSENT", ""
	if inv.Trace.Enabled && inv.Trace.ContentSHA256 != "" {
		traceState, traceDigest = "PRESENT", inv.Trace.ContentSHA256
	}
	artifacts = append(artifacts, ReplayArtifact{Kind: "PROTOCOL_TRANSCRIPT", Locator: "lsp-trace://protocol-transcript", State: traceState, Digest: traceDigest, Redaction: Redaction{State: "REDACTED", Reason: "PROTOCOL_CONTENT_NOT_EMBEDDED"}})
	stderrState, stderrDigest := "ABSENT", ""
	for _, diagnostic := range diagnostics {
		if diagnostic.Phase == "server-stderr" {
			stderrState = "PRESENT"
			stderrDigest = domainDigest("lsp-trace:server-stderr:v1", []byte(diagnostic.Message))
			break
		}
	}
	artifacts = append(artifacts, ReplayArtifact{Kind: "SERVER_STDERR", Locator: "lsp-trace://server-stderr", State: stderrState, Digest: stderrDigest, Redaction: Redaction{State: "REDACTED", Reason: "STDERR_CONTENT_NOT_EMBEDDED"}})
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].Kind != artifacts[j].Kind {
			return artifacts[i].Kind < artifacts[j].Kind
		}
		return artifacts[i].Locator < artifacts[j].Locator
	})
	encoded, _ := json.Marshal(artifacts)
	return ReplayInputManifest{ManifestID: domainDigest("lsp-trace:replay-input-manifest:v1", encoded), Artifacts: artifacts}
}

func projectPortableLocators(nodes []Node) []PortableLocator {
	out := make([]PortableLocator, 0, len(nodes))
	for _, node := range nodes {
		r := node.SelectionRange
		locator := fmt.Sprintf("%s#L%d:%d-L%d:%d", canonicalURI(node.URI), r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
		out = append(out, PortableLocator{NodeID: node.ID, Locator: locator, Redaction: Redaction{State: "VISIBLE", Reason: "CANONICAL_SOURCE_LOCATION"}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out
}

func canonicalURI(raw string) string {
	if raw == "" {
		return "lsp-trace://absent"
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() {
		return "lsp-trace://invalid/" + domainDigest("lsp-trace:invalid-locator:v1", []byte(raw))
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return u.String()
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
	verified := Result{
		SchemaVersion: b.SchemaVersion, Targets: b.Targets, Nodes: b.Nodes, Edges: b.Edges,
		Terminals: b.Terminals, Frontier: b.Frontier, Diagnostics: b.Diagnostics, Seeds: b.Seeds,
		SiblingCandidates: b.SiblingCandidates, DispatchRelationships: b.DispatchRelationships,
		Summary:      Summary{Complete: b.Summary.TraversalComplete, Truncated: b.Summary.Truncated},
		Capabilities: b.Capabilities, CapabilityQuality: b.CapabilityQuality,
	}
	if err := verified.ValidateReferences(); err != nil {
		return err
	}
	verified.Canonicalize()
	if err := validateProcessContext(b.ProcessContext); err != nil {
		return err
	}
	expectedReceipt := verified.evidenceReceipt(b.Invocation.Provenance.SourceRevision)
	expectedMemberships := projectSeedMemberships(b.Invocation.Seeds, b.Seeds, b.SiblingCandidates, b.DispatchRelationships, b.Invocation.Provenance.SourceRevision)
	expectedManifest := projectReplayManifest(b.Invocation, b.Diagnostics)
	expectedLocators := projectPortableLocators(b.Nodes)
	if !reflect.DeepEqual(b.EvidenceReceipt, expectedReceipt) ||
		!reflect.DeepEqual(b.SeedMemberships, expectedMemberships) ||
		!reflect.DeepEqual(b.ReplayInputManifest, expectedManifest) ||
		!reflect.DeepEqual(b.PortableLocators, expectedLocators) {
		return fmt.Errorf("replay identity mismatch")
	}
	if b.Summary.NodeCount != verified.Summary.NodeCount ||
		b.Summary.EdgeCount != verified.Summary.EdgeCount ||
		b.Summary.TerminalCount != verified.Summary.TerminalCount ||
		b.Summary.CycleCount != verified.Summary.CycleCount ||
		b.CapabilityQuality.IncomingEdges != verified.CapabilityQuality.IncomingEdges ||
		b.CapabilityQuality.UnresolvedCalls != verified.CapabilityQuality.UnresolvedCalls ||
		b.CapabilityQuality.DynamicCalls != verified.CapabilityQuality.DynamicCalls ||
		(b.Summary.TraversalComplete && (b.Summary.Truncated || len(b.Frontier) > 0 || verified.CapabilityQuality.UnresolvedCalls > 0)) ||
		b.Capabilities.CallHierarchyProvider != b.CapabilityQuality.Advertised {
		return fmt.Errorf("derived semantic mismatch")
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
