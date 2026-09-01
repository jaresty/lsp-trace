package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
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
	CallerProvenanceClass        string           `json:"caller_provenance_class"`
	ToolVersionProvenanceClass   string           `json:"tool_version_provenance_class,omitempty"`
	ServerVersionProvenanceClass string           `json:"server_version_provenance_class,omitempty"`
	ResolvedSeeds                []InvocationSeed `json:"resolved_seeds"`
	ResolvedSeedContentsDigest   string           `json:"resolved_seed_contents_digest,omitempty"`
	AggregateScope               string           `json:"aggregate_scope"`
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
	Name                                string    `json:"name"`
	EnvironmentNameProcessContextDigest string    `json:"environment_name_process_context_digest"`
	Redaction                           Redaction `json:"redaction"`
}
type ProcessContext struct {
	Environment                              []EnvironmentIdentity `json:"environment"`
	EffectiveEnvironmentProcessContextDigest string                `json:"effective_environment_process_context_digest"`
	EffectiveEnvironmentVariableCount        int                   `json:"effective_environment_variable_count"`
	WorkingDirectoryProcessContextDigest     string                `json:"working_directory_process_context_digest"`
	AmbientEnvironmentState                  string                `json:"ambient_environment_state"`
	Redaction                                Redaction             `json:"redaction"`
}
type SeedMembership struct {
	MembershipID      string `json:"membership_id"`
	ExecutionBundleID string `json:"execution_bundle_id,omitempty"`
	SeedLabel         string `json:"seed_label"`
	SeedAt            string `json:"seed_at"`
	EvidenceKind      string `json:"evidence_kind"`
	EndpointID        string `json:"endpoint_id"`
}
type ReplayArtifact struct {
	Kind                     string    `json:"kind"`
	Locator                  string    `json:"locator"`
	State                    string    `json:"state"`
	ReplayInputContentDigest string    `json:"replay_input_content_digest,omitempty"`
	Redaction                Redaction `json:"redaction"`
}
type ReplayInputManifest struct {
	ReplayInputManifestDigest string           `json:"replay_input_manifest_digest"`
	Artifacts                 []ReplayArtifact `json:"artifacts"`
}
type LocatorSource struct {
	NodeID         string `json:"node_id"`
	SelectionRange Range  `json:"selection_range"`
}
type LocatorDerivation struct {
	Method  string `json:"method"`
	Version string `json:"version"`
}
type LocatorAuthority struct {
	Class string `json:"class"`
	Tool  string `json:"tool"`
}
type LocatorSemantics struct {
	EstablishesRuntimeBehavior       bool `json:"establishes_runtime_behavior"`
	EstablishesFeatureCorrespondence bool `json:"establishes_feature_correspondence"`
}
type LocatorProvenance struct {
	Source     LocatorSource     `json:"source"`
	Derivation LocatorDerivation `json:"derivation"`
	Authority  LocatorAuthority  `json:"authority"`
	Semantics  LocatorSemantics  `json:"semantics"`
}
type PortableLocator struct {
	NodeID     string             `json:"node_id"`
	Locator    string             `json:"locator"`
	Provenance *LocatorProvenance `json:"provenance,omitempty"`
	Redaction  Redaction          `json:"redaction"`
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
	ExecutionBundleID     string                 `json:"execution_bundle_id,omitempty"`
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
	Slice                 *SliceEvidence         `json:"slice,omitempty"`
	Summary               summaryV3              `json:"summary"`
}
type semanticReceiptV3 struct {
	ReceiptVersion           string `json:"receipt_version"`
	SemanticCommitmentDigest string `json:"semantic_commitment_digest"`
	DigestScope              string `json:"digest_scope"`
}
type bundleV3 struct {
	semanticV3
	TraceReceipt semanticReceiptV3 `json:"trace_receipt"`
}

func sensitivityPolicy() SensitivityPolicy {
	return SensitivityPolicy{
		Covered:                           []string{"invocation_arguments", "explicit_environment_names", "workspace_source_output_paths", "opaque_node_data", "diagnostics", "captured_server_stderr", "trace_transcripts"},
		AutomaticRedaction:                false,
		AccessControlResponsibility:       "BUNDLE_CUSTODIAN",
		AmbientProcessEnvironmentRecorded: false,
	}
}

func (r Result) marshalV3() ([]byte, error) {
	// Producer and verifier must project from the same canonical graph state.
	r.Canonicalize()
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
	identity := BundleIdentity{CallerProvenanceClass: "CALLER_ASSERTED", ToolVersionProvenanceClass: "CALLER_ASSERTED", ServerVersionProvenanceClass: "CALLER_ASSERTED", ResolvedSeeds: resolved, AggregateScope: "RESOLVED_SEED_CONTENTS"}
	if len(resolved) > 0 {
		identity.ResolvedSeedContentsDigest = domainDigest(ResolvedSeedsDomain, aggInput)
	}
	processContext := projectProcessContext(inv.Server.Environment, inv.EffectiveEnvironment, inv.WorkingDirectory)
	inv.Server.Environment = nil
	executionBundleID := semanticExecutionBundleID(inv)
	edges, siblings, dispatches := projectExecutionBundleRelations(executionBundleID, r.Edges, r.SiblingCandidates, r.DispatchRelationships)
	receipt := r.evidenceReceipt(inv.Provenance.SourceRevision)
	if err := validateProducerSeedRelations(r.Seeds); err != nil {
		return nil, err
	}
	if err := validateSeedJoins(inv.Seeds, r.Seeds, r.SiblingCandidates, r.DispatchRelationships, receipt); err != nil {
		return nil, err
	}
	sem := semanticV3{
		SchemaVersion: r.SchemaVersion, ExecutionBundleID: executionBundleID, Tool: tool, Invocation: inv, Identity: identity,
		SensitivityPolicy: sensitivityPolicy(),
		ProcessContext:    processContext, EvidenceSemantics: evidenceSemantics(), EvidenceReceipt: receipt,
		SeedMemberships: projectSeedMemberships(executionBundleID, inv.Seeds, r.Seeds, edges, siblings, dispatches, inv.Provenance.SourceRevision), ReplayInputManifest: projectReplayManifest(inv, r.Diagnostics), PortableLocators: projectPortableLocators(r.Nodes),
		Capabilities: r.Capabilities, CapabilityQuality: r.CapabilityQuality, Targets: r.Targets, Nodes: r.Nodes, Edges: edges,
		Terminals: r.Terminals, Frontier: r.Frontier, Diagnostics: r.Diagnostics, SiblingCandidates: siblings,
		DispatchRelationships: dispatches, Seeds: r.Seeds, Slice: r.Slice,
		Summary: summaryV3{r.Summary.NodeCount, r.Summary.EdgeCount, r.Summary.TerminalCount, r.Summary.CycleCount, r.Summary.Complete, Unknown, CompletenessScope, r.Summary.Truncated},
	}
	canonical, err := json.Marshal(sem)
	if err != nil {
		return nil, err
	}
	return json.Marshal(bundleV3{sem, semanticReceiptV3{"lsp-trace.semantic-receipt.v1", domainDigest(SemanticDigestDomain, canonical), SemanticDigestScope}})
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
		Environment:                              make([]EnvironmentIdentity, 0, len(names)),
		EffectiveEnvironmentProcessContextDigest: domainDigest("lsp-trace:effective-environment:v1", encoded),
		EffectiveEnvironmentVariableCount:        len(effective),
		WorkingDirectoryProcessContextDigest:     domainDigest("lsp-trace:working-directory:v1", []byte(filepath.ToSlash(filepath.Clean(workingDirectory)))),
		AmbientEnvironmentState:                  "IDENTIFIED_NOT_EMBEDDED",
		Redaction:                                Redaction{State: "REDACTED", Reason: "EFFECTIVE_PROCESS_CONTEXT_NOT_EMBEDDED"},
	}
	for _, name := range names {
		out.Environment = append(out.Environment, EnvironmentIdentity{Name: name, EnvironmentNameProcessContextDigest: domainDigest("lsp-trace:environment-name:v1", []byte(name)), Redaction: Redaction{State: "REDACTED", Reason: "SECRET_SAFE_ENVIRONMENT_IDENTITY"}})
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
	if context.AmbientEnvironmentState != "IDENTIFIED_NOT_EMBEDDED" || context.EffectiveEnvironmentVariableCount < 0 || !strings.HasPrefix(context.EffectiveEnvironmentProcessContextDigest, "sha256:") || !strings.HasPrefix(context.WorkingDirectoryProcessContextDigest, "sha256:") || context.Redaction != (Redaction{State: "REDACTED", Reason: "EFFECTIVE_PROCESS_CONTEXT_NOT_EMBEDDED"}) {
		return fmt.Errorf("process context mismatch")
	}
	last := ""
	for _, identity := range context.Environment {
		if identity.Name == "" || identity.EnvironmentNameProcessContextDigest != domainDigest("lsp-trace:environment-name:v1", []byte(identity.Name)) || identity.Redaction != (Redaction{State: "REDACTED", Reason: "SECRET_SAFE_ENVIRONMENT_IDENTITY"}) {
			return fmt.Errorf("process context mismatch")
		}
		if last != "" && identity.Name <= last {
			if identity.Name == last {
				return fmt.Errorf("duplicate explicit environment name %q", identity.Name)
			}
			return fmt.Errorf("process context mismatch")
		}
		last = identity.Name
	}
	return nil
}

func semanticExecutionBundleID(inv Invocation) string {
	canonical := inv
	canonical.Seeds = append([]InvocationSeed(nil), inv.Seeds...)
	sort.Slice(canonical.Seeds, func(i, j int) bool {
		a, b := canonical.Seeds[i], canonical.Seeds[j]
		if a.ResolvedURI != b.ResolvedURI {
			return a.ResolvedURI < b.ResolvedURI
		}
		if a.At != b.At {
			return a.At < b.At
		}
		return a.Label < b.Label
	})
	encoded, _ := json.Marshal(canonical)
	return domainDigest("lsp-trace:semantic-execution-bundle:v1", encoded)
}

func projectExecutionBundleRelations(bundleID string, edges []Edge, siblings []SiblingCandidate, dispatches []DispatchRelationship) ([]Edge, []SiblingCandidate, []DispatchRelationship) {
	edges = append([]Edge(nil), edges...)
	for i := range edges {
		edges[i].ExecutionBundleID = bundleID
	}
	siblings = append([]SiblingCandidate(nil), siblings...)
	for i := range siblings {
		siblings[i].ExecutionBundleID = bundleID
	}
	dispatches = append([]DispatchRelationship(nil), dispatches...)
	for i := range dispatches {
		dispatches[i].ExecutionBundleID = bundleID
	}
	return edges, siblings, dispatches
}

func projectSeedMemberships(bundleID string, invocationSeeds []InvocationSeed, results []SeedResult, _ []Edge, siblings []SiblingCandidate, dispatches []DispatchRelationship, _ string) []SeedMembership {
	byLabel := make(map[string]InvocationSeed, len(invocationSeeds))
	for _, seed := range invocationSeeds {
		byLabel[seed.Label] = seed
	}
	out := make([]SeedMembership, 0)
	for _, result := range results {
		seed := byLabel[result.Label]
		for _, endpoint := range result.PreparedTargetIDs {
			out = append(out, newSeedMembership(bundleID, seed, "PREPARED_TARGET", endpoint))
		}
		for _, endpoint := range result.ReachedNodeIDs {
			out = append(out, newSeedMembership(bundleID, seed, "REACHED_NODE", endpoint))
		}
		for _, relationID := range result.ReachedRelationIDs {
			out = append(out, newSeedMembership(bundleID, seed, "CALL_RELATION", relationID))
		}
	}
	for _, sibling := range siblings {
		for _, label := range sibling.SeedLabels {
			out = append(out, newSeedMembership(bundleID, byLabel[label], "SIBLING_CANDIDATE", sibling.RelationID))
		}
	}
	for _, dispatch := range dispatches {
		for _, label := range dispatch.SeedLabels {
			out = append(out, newSeedMembership(bundleID, byLabel[label], "DISPATCH_ASSOCIATION", dispatch.RelationID))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MembershipID < out[j].MembershipID })
	return out
}

func newSeedMembership(bundleID string, seed InvocationSeed, kind, endpoint string) SeedMembership {
	payload, _ := json.Marshal([]string{seed.Label, seed.At, seed.ResolvedURI, kind, endpoint})
	return SeedMembership{MembershipID: domainDigest("lsp-trace:seed-membership:v1", payload), ExecutionBundleID: bundleID, SeedLabel: seed.Label, SeedAt: seed.At, EvidenceKind: kind, EndpointID: endpoint}
}

func projectReplayManifest(inv Invocation, diagnostics []Diagnostic) ReplayInputManifest {
	artifacts := make([]ReplayArtifact, 0, len(inv.Seeds)+2)
	for _, seed := range inv.Seeds {
		state := "ABSENT"
		if seed.ContentSHA256 != "" {
			state = "PRESENT"
		}
		artifacts = append(artifacts, ReplayArtifact{Kind: "SOURCE_ARTIFACT", Locator: canonicalURI(seed.ResolvedURI), State: state, ReplayInputContentDigest: seed.ContentSHA256, Redaction: Redaction{State: "VISIBLE", Reason: "REPLAY_IDENTITY"}})
	}
	traceState, traceDigest := "ABSENT", ""
	if inv.Trace.Enabled && inv.Trace.ContentSHA256 != "" {
		traceState, traceDigest = "PRESENT", inv.Trace.ContentSHA256
	}
	artifacts = append(artifacts, ReplayArtifact{Kind: "PROTOCOL_TRANSCRIPT", Locator: "lsp-trace://protocol-transcript", State: traceState, ReplayInputContentDigest: traceDigest, Redaction: Redaction{State: "REDACTED", Reason: "PROTOCOL_CONTENT_NOT_EMBEDDED"}})
	stderrState, stderrDigest := "ABSENT", ""
	for _, diagnostic := range diagnostics {
		if diagnostic.Phase == "server-stderr" {
			stderrState = "PRESENT"
			stderrDigest = domainDigest("lsp-trace:server-stderr:v1", []byte(diagnostic.Message))
			break
		}
	}
	artifacts = append(artifacts, ReplayArtifact{Kind: "SERVER_STDERR", Locator: "lsp-trace://server-stderr", State: stderrState, ReplayInputContentDigest: stderrDigest, Redaction: Redaction{State: "REDACTED", Reason: "STDERR_CONTENT_NOT_EMBEDDED"}})
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].Kind != artifacts[j].Kind {
			return artifacts[i].Kind < artifacts[j].Kind
		}
		return artifacts[i].Locator < artifacts[j].Locator
	})
	encoded, _ := json.Marshal(artifacts)
	return ReplayInputManifest{ReplayInputManifestDigest: domainDigest("lsp-trace:replay-input-manifest:v1", encoded), Artifacts: artifacts}
}

func projectPortableLocators(nodes []Node) []PortableLocator {
	out := make([]PortableLocator, 0, len(nodes))
	for _, node := range nodes {
		r := node.SelectionRange
		locator := fmt.Sprintf("%s#L%d:%d-L%d:%d", canonicalURI(node.URI), r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
		out = append(out, PortableLocator{
			NodeID:  node.ID,
			Locator: locator,
			Provenance: &LocatorProvenance{
				Source:     LocatorSource{NodeID: node.ID, SelectionRange: node.SelectionRange},
				Derivation: LocatorDerivation{Method: "CANONICAL_URI_WITH_SELECTION_RANGE", Version: "1"},
				Authority:  LocatorAuthority{Class: "TOOL_DERIVED", Tool: "lsp-trace"},
				Semantics:  LocatorSemantics{},
			},
			Redaction: Redaction{State: "VISIBLE", Reason: "CANONICAL_SOURCE_LOCATION"},
		})
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
		if NewNode(n.Item).ID != n.ID {
			return fmt.Errorf("invalid canonical node id %q", n.ID)
		}
		if _, ok := ids[n.ID]; ok {
			return fmt.Errorf("duplicate canonical node %q", n.ID)
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
	edges := map[string]struct{}{}
	relationIDs := map[string]struct{}{}
	for _, e := range r.Edges {
		if e.RelationID != canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", e.CallerNodeID+"->"+e.CalleeNodeID, "", "", "", e.CallerNodeID, e.CalleeNodeID) {
			return fmt.Errorf("invalid canonical call relation id %q", e.RelationID)
		}
		if err := need("edge caller", e.CallerNodeID); err != nil {
			return err
		}
		if err := need("edge callee", e.CalleeNodeID); err != nil {
			return err
		}
		canonicalSites := mergeRanges(nil, e.CallSites)
		encoded, _ := json.Marshal(canonicalSites)
		key := e.CallerNodeID + "\x00" + e.CalleeNodeID + "\x00" + string(encoded)
		if _, ok := edges[key]; ok {
			return fmt.Errorf("duplicate canonical edge %s->%s", e.CallerNodeID, e.CalleeNodeID)
		}
		edges[key] = struct{}{}
		relationIDs[e.RelationID] = struct{}{}
	}
	if r.Slice != nil {
		for _, id := range r.Slice.StartingNodeIDs {
			if err := need("slice starting", id); err != nil {
				return err
			}
		}
		for i, layer := range r.Slice.Layers {
			if layer.Depth != i {
				return fmt.Errorf("slice layers must be contiguous from depth zero")
			}
			for _, id := range layer.NodeIDs {
				if err := need("slice layer", id); err != nil {
					return err
				}
			}
		}
		for _, id := range r.Slice.FrontierNodeIDs {
			if err := need("slice frontier", id); err != nil {
				return err
			}
		}
		for _, id := range r.Slice.OutgoingTerminalNodeIDs {
			if err := need("slice outgoing terminal", id); err != nil {
				return err
			}
		}
		for _, id := range r.Slice.UpwardStartNodeIDs {
			if err := need("slice upward start", id); err != nil {
				return err
			}
		}
		if r.Slice.DownDepth < 0 {
			return fmt.Errorf("slice down depth must be non-negative")
		}
		if r.Slice.DownDepth < len(r.Slice.Layers) {
			if !reflect.DeepEqual(r.Slice.FrontierNodeIDs, r.Slice.Layers[r.Slice.DownDepth].NodeIDs) {
				return fmt.Errorf("slice frontier does not equal down-depth layer")
			}
		} else if len(r.Slice.FrontierNodeIDs) != 0 {
			return fmt.Errorf("slice frontier must be empty when down-depth layer was not reached")
		}
		if r.Slice.OutgoingTerminalNodeIDs != nil || r.Slice.UpwardStartNodeIDs != nil {
			upwardDuplicate := false
			for i := 1; i < len(r.Slice.UpwardStartNodeIDs); i++ {
				if r.Slice.UpwardStartNodeIDs[i-1] == r.Slice.UpwardStartNodeIDs[i] {
					upwardDuplicate = true
					break
				}
			}
			if !sort.StringsAreSorted(r.Slice.UpwardStartNodeIDs) || upwardDuplicate {
				return fmt.Errorf("slice upward starts must be sorted and unique")
			}
			upwardSet := map[string]struct{}{}
			for _, id := range append(append([]string{}, r.Slice.FrontierNodeIDs...), r.Slice.OutgoingTerminalNodeIDs...) {
				upwardSet[id] = struct{}{}
			}
			expectedUpward := make([]string, 0, len(upwardSet))
			for id := range upwardSet {
				expectedUpward = append(expectedUpward, id)
			}
			sort.Strings(expectedUpward)
			if !reflect.DeepEqual(r.Slice.UpwardStartNodeIDs, expectedUpward) {
				return fmt.Errorf("slice upward starts do not equal frontier and outgoing-terminal union")
			}
			membershipNodes := map[string]struct{}{}
			membershipRelations := map[string]struct{}{}
			for _, seed := range r.Seeds {
				if seed.Failure != nil {
					if len(seed.ReachedNodeIDs) != 0 || len(seed.ReachedRelationIDs) != 0 {
						return fmt.Errorf("failed slice seed has non-empty membership %q", seed.Label)
					}
					continue
				}
				for _, id := range seed.ReachedNodeIDs {
					membershipNodes[id] = struct{}{}
				}
				for _, id := range seed.ReachedRelationIDs {
					membershipRelations[id] = struct{}{}
				}
			}
			if len(membershipNodes) != len(ids) {
				return fmt.Errorf("slice seed memberships do not cover union nodes")
			}
			for id := range ids {
				if _, ok := membershipNodes[id]; !ok {
					return fmt.Errorf("slice seed memberships do not cover union nodes")
				}
			}
			if len(membershipRelations) != len(relationIDs) {
				return fmt.Errorf("slice seed memberships do not cover union relations")
			}
			for id := range relationIDs {
				if _, ok := membershipRelations[id]; !ok {
					return fmt.Errorf("slice seed memberships do not cover union relations")
				}
			}
		}
		for _, id := range r.Slice.OutgoingRelationIDs {
			if _, ok := relationIDs[id]; !ok {
				return fmt.Errorf("dangling slice outgoing relation id %q", id)
			}
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
		if c.RelationID != canonicalRelationID("SIBLING_CANDIDATE", "DISCOVERY", c.Candidate.ID, c.Candidate.ID, "", "", "", "") {
			return fmt.Errorf("invalid canonical sibling relation id %q", c.RelationID)
		}
		if err := checkEmbedded("sibling candidate", c.Candidate); err != nil {
			return err
		}
	}
	for _, d := range r.DispatchRelationships {
		if d.RelationID != canonicalRelationID("DISPATCH_ASSOCIATION", "ASSOCIATION", d.Interface.ID+"->"+d.Implementation.ID, "", d.Interface.ID, d.Implementation.ID, "", "") {
			return fmt.Errorf("invalid canonical dispatch relation id %q", d.RelationID)
		}
		if err := checkEmbedded("dispatch interface", d.Interface); err != nil {
			return err
		}
		if err := checkEmbedded("dispatch implementation", d.Implementation); err != nil {
			return err
		}
	}
	return nil
}

func validateEvidenceKinds(receipt *EvidenceReceipt, memberships []SeedMembership) error {
	if receipt != nil {
		for _, relation := range receipt.Relations {
			switch relation.RelationKind {
			case "CALL_RELATION", "DISPATCH_ASSOCIATION", "SIBLING_CANDIDATE":
			default:
				return fmt.Errorf("unknown native relation kind %q", relation.RelationKind)
			}
		}
	}
	for _, membership := range memberships {
		switch membership.EvidenceKind {
		case "PREPARED_TARGET", "REACHED_NODE", "CALL_RELATION", "DISPATCH_ASSOCIATION", "SIBLING_CANDIDATE":
		default:
			return fmt.Errorf("unknown derived evidence kind %q", membership.EvidenceKind)
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
	if err := validateEvidenceKinds(b.EvidenceReceipt, b.SeedMemberships); err != nil {
		return err
	}
	if b.ExecutionBundleID != "" {
		expectedBundleID := semanticExecutionBundleID(b.Invocation)
		if b.ExecutionBundleID != expectedBundleID {
			return fmt.Errorf("execution bundle identity mismatch")
		}
		for _, relation := range b.Edges {
			if relation.ExecutionBundleID != expectedBundleID {
				return fmt.Errorf("execution bundle relation mismatch")
			}
		}
		for _, relation := range b.SiblingCandidates {
			if relation.ExecutionBundleID != expectedBundleID {
				return fmt.Errorf("execution bundle relation mismatch")
			}
		}
		for _, relation := range b.DispatchRelationships {
			if relation.ExecutionBundleID != expectedBundleID {
				return fmt.Errorf("execution bundle relation mismatch")
			}
		}
		for _, membership := range b.SeedMemberships {
			if membership.ExecutionBundleID != expectedBundleID {
				return fmt.Errorf("execution bundle membership mismatch")
			}
		}
	}
	verified := Result{
		SchemaVersion: b.SchemaVersion, Targets: b.Targets, Nodes: b.Nodes, Edges: b.Edges,
		Terminals: b.Terminals, Frontier: b.Frontier, Diagnostics: b.Diagnostics, Seeds: b.Seeds,
		SiblingCandidates: b.SiblingCandidates, DispatchRelationships: b.DispatchRelationships, Slice: b.Slice,
		Summary:      Summary{Complete: true, Truncated: b.Summary.Truncated},
		Capabilities: b.Capabilities, CapabilityQuality: b.CapabilityQuality,
	}
	for _, membership := range b.SeedMemberships {
		for i := range verified.SiblingCandidates {
			if membership.EvidenceKind == "SIBLING_CANDIDATE" && membership.EndpointID == verified.SiblingCandidates[i].RelationID {
				verified.SiblingCandidates[i].SeedLabels = append(verified.SiblingCandidates[i].SeedLabels, membership.SeedLabel)
			}
		}
		for i := range verified.DispatchRelationships {
			if membership.EvidenceKind == "DISPATCH_ASSOCIATION" && membership.EndpointID == verified.DispatchRelationships[i].RelationID {
				verified.DispatchRelationships[i].SeedLabels = append(verified.DispatchRelationships[i].SeedLabels, membership.SeedLabel)
			}
		}
	}
	if err := verified.ValidateReferences(); err != nil {
		return err
	}
	verified.Canonicalize()
	if err := validateProcessContext(b.ProcessContext); err != nil {
		return err
	}
	if !reflect.DeepEqual(b.SensitivityPolicy, sensitivityPolicy()) {
		return fmt.Errorf("sensitivity policy mismatch")
	}
	if !reflect.DeepEqual(b.EvidenceSemantics, evidenceSemantics()) {
		return fmt.Errorf("evidence semantics mismatch")
	}
	expectedReceipt := verified.evidenceReceipt(b.Invocation.Provenance.SourceRevision)
	if err := validateSeedJoins(b.Invocation.Seeds, b.Seeds, verified.SiblingCandidates, verified.DispatchRelationships, expectedReceipt); err != nil {
		return err
	}
	expectedMemberships := projectSeedMemberships(b.ExecutionBundleID, b.Invocation.Seeds, b.Seeds, b.Edges, verified.SiblingCandidates, verified.DispatchRelationships, b.Invocation.Provenance.SourceRevision)
	expectedManifest := projectReplayManifest(b.Invocation, b.Diagnostics)
	expectedLocators := projectPortableLocators(b.Nodes)
	historicalLocators := len(b.PortableLocators) > 0
	for _, locator := range b.PortableLocators {
		if locator.Provenance != nil {
			historicalLocators = false
			break
		}
	}
	if historicalLocators {
		for i := range expectedLocators {
			expectedLocators[i].Provenance = nil
		}
	}
	if !reflect.DeepEqual(b.EvidenceReceipt, expectedReceipt) {
		return fmt.Errorf("replay identity mismatch: evidence receipt")
	}
	if !reflect.DeepEqual(b.SeedMemberships, expectedMemberships) {
		return fmt.Errorf("replay identity mismatch: seed memberships")
	}
	if !reflect.DeepEqual(b.ReplayInputManifest, expectedManifest) {
		return fmt.Errorf("replay identity mismatch: input manifest")
	}
	if !reflect.DeepEqual(b.PortableLocators, expectedLocators) {
		return fmt.Errorf("replay identity mismatch: portable locators")
	}
	if b.Summary.NodeCount != verified.Summary.NodeCount ||
		b.Summary.TraversalComplete != verified.Summary.Complete ||
		b.Summary.EdgeCount != verified.Summary.EdgeCount ||
		b.Summary.TerminalCount != verified.Summary.TerminalCount ||
		b.Summary.CycleCount != verified.Summary.CycleCount ||
		b.CapabilityQuality.Advertised != verified.CapabilityQuality.Advertised ||
		b.CapabilityQuality.PrepareSucceeded != verified.CapabilityQuality.PrepareSucceeded ||
		b.CapabilityQuality.IncomingRequestSuccesses != verified.CapabilityQuality.IncomingRequestSuccesses ||
		b.CapabilityQuality.IncomingEdges != verified.CapabilityQuality.IncomingEdges ||
		b.CapabilityQuality.CrossFileEdges != verified.CapabilityQuality.CrossFileEdges ||
		b.CapabilityQuality.UnresolvedCalls != verified.CapabilityQuality.UnresolvedCalls ||
		b.CapabilityQuality.DynamicCalls != verified.CapabilityQuality.DynamicCalls ||
		b.CapabilityQuality.CrossModuleEdges != verified.CapabilityQuality.CrossModuleEdges {
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
	if b.Identity.CallerProvenanceClass != "CALLER_ASSERTED" ||
		(b.Identity.ToolVersionProvenanceClass != "" && b.Identity.ToolVersionProvenanceClass != "CALLER_ASSERTED") ||
		(b.Identity.ServerVersionProvenanceClass != "" && b.Identity.ServerVersionProvenanceClass != "CALLER_ASSERTED") ||
		b.Identity.AggregateScope != "RESOLVED_SEED_CONTENTS" || b.Identity.ResolvedSeedContentsDigest != aggregate || !reflect.DeepEqual(b.Identity.ResolvedSeeds, resolved) {
		return fmt.Errorf("bundle identity mismatch")
	}
	receipt := b.TraceReceipt
	b.TraceReceipt = semanticReceiptV3{}
	canonical, err := json.Marshal(b.semanticV3)
	if err != nil {
		return err
	}
	if receipt.ReceiptVersion != "lsp-trace.semantic-receipt.v1" || receipt.DigestScope != SemanticDigestScope || receipt.SemanticCommitmentDigest != domainDigest(SemanticDigestDomain, canonical) {
		return fmt.Errorf("embedded semantic receipt mismatch")
	}
	return nil
}

func validateProducerSeedRelations(results []SeedResult) error {
	for _, result := range results {
		if result.ReachedEdges == nil {
			continue
		}
		expected := make([]string, 0, len(result.ReachedEdges))
		for _, edge := range result.ReachedEdges {
			expected = append(expected, canonicalRelationID("CALL_RELATION", "CALLER_TO_CALLEE", edge.CallerNodeID+"->"+edge.CalleeNodeID, "", "", "", edge.CallerNodeID, edge.CalleeNodeID))
		}
		sort.Strings(expected)
		expected = uniqueStrings(expected)
		if !reflect.DeepEqual(result.ReachedRelationIDs, expected) {
			return fmt.Errorf("seed join mismatch: exact call relations for %q", result.Label)
		}
	}
	return nil
}

func validateSeedJoins(invocation []InvocationSeed, results []SeedResult, siblings []SiblingCandidate, dispatches []DispatchRelationship, receipt *EvidenceReceipt) error {
	invocationLabels := make(map[string]struct{}, len(invocation))
	for _, seed := range invocation {
		if seed.Label == "" {
			return fmt.Errorf("seed join mismatch: empty invocation label")
		}
		if _, duplicate := invocationLabels[seed.Label]; duplicate {
			return fmt.Errorf("seed join mismatch: duplicate invocation label %q", seed.Label)
		}
		invocationLabels[seed.Label] = struct{}{}
	}
	resultLabels := make(map[string]struct{}, len(results))
	relations := map[string]EvidenceRelation{}
	if receipt != nil {
		for _, relation := range receipt.Relations {
			relations[relation.RelationID] = relation
		}
	}
	for _, result := range results {
		if _, exists := invocationLabels[result.Label]; !exists {
			return fmt.Errorf("seed join mismatch: result label %q has no invocation seed", result.Label)
		}
		if _, duplicate := resultLabels[result.Label]; duplicate {
			return fmt.Errorf("seed join mismatch: duplicate result label %q", result.Label)
		}
		resultLabels[result.Label] = struct{}{}
		for _, relationID := range result.ReachedRelationIDs {
			relation, exists := relations[relationID]
			if !exists || relation.RelationKind != "CALL_RELATION" {
				return fmt.Errorf("seed join mismatch: unknown call relation %q", relationID)
			}
		}
	}
	if len(results) != len(invocation) {
		return fmt.Errorf("seed join mismatch: invocation/result cardinality")
	}
	for label := range invocationLabels {
		if _, exists := resultLabels[label]; !exists {
			return fmt.Errorf("seed join mismatch: missing result for %q", label)
		}
	}
	for _, sibling := range siblings {
		for _, label := range sibling.SeedLabels {
			if _, exists := invocationLabels[label]; !exists {
				return fmt.Errorf("seed join mismatch: sibling label %q", label)
			}
		}
	}
	for _, dispatch := range dispatches {
		for _, label := range dispatch.SeedLabels {
			if _, exists := invocationLabels[label]; !exists {
				return fmt.Errorf("seed join mismatch: dispatch label %q", label)
			}
		}
	}
	return nil
}

func ExactBytesDigest(data []byte) string { return domainDigest(ByteDigestDomain, data) }
