package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

const (
	SchemaVersionV1   = "lsp-trace.graph.v1"
	SchemaVersionV2   = "lsp-trace.graph.v2"
	SchemaVersionV3   = "lsp-trace.graph.v3"
	SchemaVersion     = SchemaVersionV3
	CompletenessScope = "SERVER_REPORTED_CALL_HIERARCHY"
	Unknown           = "UNKNOWN"
)

type Reason string

const (
	NoIncomingCalls          Reason = "NO_INCOMING_CALLS"
	ServerReportedNoIncoming Reason = "SERVER_REPORTED_NO_INCOMING_CALLS"
	PrepareReturnedNoItem    Reason = "PREPARE_RETURNED_NO_ITEM"
	IncomingReturnedNull     Reason = "INCOMING_RETURNED_NULL"
	ExternalURI              Reason = "EXTERNAL_URI"
	UnsupportedCallHierarchy Reason = "UNSUPPORTED_CALL_HIERARCHY"
	ServerError              Reason = "SERVER_ERROR"
	InvalidServerResponse    Reason = "INVALID_SERVER_RESPONSE"
	RequestTimeout           Reason = "REQUEST_TIMEOUT"
	GlobalTimeout            Reason = "GLOBAL_TIMEOUT"
	Cancelled                Reason = "CANCELLED"
	MaxDepth                 Reason = "MAX_DEPTH"
	MaxNodes                 Reason = "MAX_NODES"
	NodeIDCollision          Reason = "NODE_ID_COLLISION"
)

type Limits struct {
	MaxDepth  int   `json:"max_depth"`
	MaxNodes  int   `json:"max_nodes"`
	TimeoutMS int64 `json:"timeout_ms"`
}
type Target struct {
	URI    string `json:"uri"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}
type ServerInvocation struct {
	Command     string            `json:"command"`
	Arguments   []string          `json:"arguments"`
	Environment map[string]string `json:"environment,omitempty"`
}
type InvocationProvenance struct {
	InvocationID   string `json:"invocation_id"`
	Caller         string `json:"caller"`
	Source         string `json:"source"`
	SourceRevision string `json:"source_revision"`
	ServerVersion  string `json:"server_version"`
	Timestamp      string `json:"timestamp"`
}

type ToolIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InvocationSeed struct {
	Label         string `json:"label"`
	At            string `json:"at"`
	ResolvedURI   string `json:"resolved_uri,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
	LanguageID    string `json:"language_id"`
}
type ExpansionConfig struct {
	TopmostSiblings bool `json:"topmost_siblings"`
	DispatchFamily  bool `json:"dispatch_family"`
}
type TraceConfig struct {
	Enabled       bool   `json:"enabled"`
	Path          string `json:"path,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
}
type Invocation struct {
	WorkspaceURI         string               `json:"workspace_uri"`
	WorkingDirectory     string               `json:"-"`
	EffectiveEnvironment []string             `json:"-"`
	Target               Target               `json:"target"`
	Server               ServerInvocation     `json:"server"`
	Limits               Limits               `json:"limits"`
	RequestTimeoutMS     int64                `json:"request_timeout_ms"`
	Concurrency          int                  `json:"concurrency"`
	LanguageID           string               `json:"language_id"`
	Expansion            ExpansionConfig      `json:"expansion"`
	Trace                TraceConfig          `json:"trace"`
	OutputMode           string               `json:"output_mode"`
	OutputPath           string               `json:"output_path"`
	Seeds                []InvocationSeed     `json:"seeds"`
	Provenance           InvocationProvenance `json:"provenance,omitempty"`
}
type Capabilities struct {
	CallHierarchyProvider bool `json:"call_hierarchy_provider"`
}
type Boundary struct {
	NodeID     string `json:"node_id,omitempty"`
	Reason     Reason `json:"reason"`
	Message    string `json:"message,omitempty"`
	Provenance string `json:"provenance,omitempty"`
}
type DiagnosticCategory string

const (
	UnresolvedCall DiagnosticCategory = "UNRESOLVED_CALL"
	DynamicCall    DiagnosticCategory = "DYNAMIC_CALL"
)

type Diagnostic struct {
	Phase    string             `json:"phase"`
	Method   string             `json:"method,omitempty"`
	NodeID   string             `json:"node_id,omitempty"`
	Category DiagnosticCategory `json:"category,omitempty"`
	Message  string             `json:"message"`
}
type CapabilityQuality struct {
	Advertised               bool   `json:"advertised"`
	PrepareSucceeded         bool   `json:"prepare_succeeded"`
	IncomingRequestSuccesses int    `json:"incoming_request_successes"`
	IncomingEdges            int    `json:"incoming_edges"`
	CrossFileEdges           int    `json:"cross_file_edges"`
	CrossModuleEdges         string `json:"cross_module_edges"`
	UnresolvedCalls          int    `json:"unresolved_calls"`
	DynamicCalls             int    `json:"dynamic_calls"`
}

type Summary struct {
	NodeCount     int  `json:"node_count"`
	EdgeCount     int  `json:"edge_count"`
	TerminalCount int  `json:"terminal_count"`
	CycleCount    int  `json:"cycle_count"`
	Complete      bool `json:"complete"`
	Truncated     bool `json:"truncated"`
}

type SeedFailure struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

type SeedResult struct {
	Label             string       `json:"label"`
	Requested         Target       `json:"requested_position"`
	PreparedTargetIDs []string     `json:"prepared_target_ids"`
	ReachedNodeIDs    []string     `json:"reached_node_ids"`
	ReachedEdges      []Edge       `json:"-"`
	Failure           *SeedFailure `json:"failure,omitempty"`
}

type SiblingCandidate struct {
	SeedURI   string `json:"seed_uri"`
	SeedLabel string `json:"seed_label,omitempty"`
	Candidate Node   `json:"candidate"`
}

type DispatchRelationship struct {
	SeedLabel      string `json:"seed_label"`
	Interface      Node   `json:"interface"`
	Implementation Node   `json:"implementation"`
}

type EvidenceRelation struct {
	RelationID           string `json:"relation_id"`
	RelationKind         string `json:"relation_kind"`
	EvidenceClass        string `json:"evidence_class"`
	EvidenceRole         string `json:"evidence_role"`
	Direction            string `json:"direction"`
	Locator              string `json:"locator"`
	SourceRevision       string `json:"source_revision"`
	SupportContribution  int    `json:"support_contribution"`
	SeedURI              string `json:"seed_uri,omitempty"`
	SeedLabel            string `json:"seed_label,omitempty"`
	CandidateNodeID      string `json:"candidate_node_id,omitempty"`
	InterfaceNodeID      string `json:"interface_node_id,omitempty"`
	ImplementationNodeID string `json:"implementation_node_id,omitempty"`
	CallerNodeID         string `json:"caller_node_id,omitempty"`
	CalleeNodeID         string `json:"callee_node_id,omitempty"`
}

type EvidenceReceipt struct {
	SupportTotal int                `json:"support_total"`
	Relations    []EvidenceRelation `json:"relations"`
}

type ClaimCeiling struct {
	EvidenceClass       string   `json:"evidence_class"`
	Supports            []string `json:"supports"`
	DoesNotSupport      []string `json:"does_not_support"`
	SupportContribution int      `json:"support_contribution"`
}

type EvidenceSemantics struct {
	CallEdges          ClaimCeiling `json:"call_edges"`
	DiscoveryRelations ClaimCeiling `json:"discovery_relations"`
}

type TraceReceipt struct {
	ReceiptVersion string `json:"receipt_version"`
	ContentDigest  string `json:"content_digest"`
	DigestScope    string `json:"digest_scope"`
}

type Result struct {
	SchemaVersion         string                 `json:"schema_version"`
	Invocation            Invocation             `json:"invocation"`
	Capabilities          Capabilities           `json:"capabilities"`
	Targets               []string               `json:"targets"`
	Nodes                 []Node                 `json:"nodes"`
	Edges                 []Edge                 `json:"edges"`
	Terminals             []Boundary             `json:"terminals"`
	Frontier              []Boundary             `json:"frontier"`
	Diagnostics           []Diagnostic           `json:"diagnostics"`
	Summary               Summary                `json:"summary"`
	CapabilityQuality     CapabilityQuality      `json:"capability_quality,omitempty"`
	SiblingCandidates     []SiblingCandidate     `json:"sibling_candidates,omitempty"`
	DispatchRelationships []DispatchRelationship `json:"dispatch_relationships,omitempty"`
	Seeds                 []SeedResult           `json:"-"`
	Tool                  ToolIdentity           `json:"-"`
}

func (r Result) MarshalJSON() ([]byte, error) {
	type legacyBoundary struct {
		NodeID  string `json:"node_id,omitempty"`
		Reason  Reason `json:"reason"`
		Message string `json:"message,omitempty"`
	}
	type legacyDiagnostic struct {
		Phase   string `json:"phase"`
		Method  string `json:"method,omitempty"`
		NodeID  string `json:"node_id,omitempty"`
		Message string `json:"message"`
	}
	if r.SchemaVersion == SchemaVersionV3 {
		return r.marshalV3()
	}
	if r.SchemaVersion == SchemaVersionV1 {
		project := func(in []Boundary) []legacyBoundary {
			if in == nil {
				return nil
			}
			out := make([]legacyBoundary, len(in))
			for i, b := range in {
				reason := b.Reason
				if reason == ServerReportedNoIncoming {
					reason = NoIncomingCalls
				}
				out[i] = legacyBoundary{b.NodeID, reason, b.Message}
			}
			return out
		}
		projectDiagnostics := func(in []Diagnostic) []legacyDiagnostic {
			if in == nil {
				return nil
			}
			out := make([]legacyDiagnostic, len(in))
			for i, d := range in {
				out[i] = legacyDiagnostic{d.Phase, d.Method, d.NodeID, d.Message}
			}
			return out
		}
		legacyInvocation := r.Invocation
		legacyInvocation.Provenance = InvocationProvenance{}
		return json.Marshal(struct {
			SchemaVersion string `json:"schema_version"`
			Invocation    struct {
				WorkspaceURI string           `json:"workspace_uri"`
				Target       Target           `json:"target"`
				Server       ServerInvocation `json:"server"`
				Limits       Limits           `json:"limits"`
			} `json:"invocation"`
			Capabilities Capabilities       `json:"capabilities"`
			Targets      []string           `json:"targets"`
			Nodes        []Node             `json:"nodes"`
			Edges        []Edge             `json:"edges"`
			Terminals    []legacyBoundary   `json:"terminals"`
			Frontier     []legacyBoundary   `json:"frontier"`
			Diagnostics  []legacyDiagnostic `json:"diagnostics"`
			Summary      Summary            `json:"summary"`
		}{r.SchemaVersion, struct {
			WorkspaceURI string           `json:"workspace_uri"`
			Target       Target           `json:"target"`
			Server       ServerInvocation `json:"server"`
			Limits       Limits           `json:"limits"`
		}{legacyInvocation.WorkspaceURI, legacyInvocation.Target, legacyInvocation.Server, legacyInvocation.Limits}, r.Capabilities, r.Targets, r.Nodes, r.Edges, project(r.Terminals), project(r.Frontier), projectDiagnostics(r.Diagnostics), r.Summary})
	}
	invocation := normalizedInvocation(r.Invocation)
	tool := r.Tool
	if tool.Name == "" {
		tool.Name = "lsp-trace"
	}
	if tool.Version == "" {
		tool.Version = Unknown
	}
	type summaryV2 struct {
		NodeCount           int    `json:"node_count"`
		EdgeCount           int    `json:"edge_count"`
		TerminalCount       int    `json:"terminal_count"`
		CycleCount          int    `json:"cycle_count"`
		TraversalComplete   bool   `json:"traversal_complete"`
		SourceGraphComplete string `json:"source_graph_complete"`
		CompletenessScope   string `json:"completeness_scope"`
		Truncated           bool   `json:"truncated"`
	}
	type invocationV2 struct {
		WorkspaceURI string `json:"workspace_uri"`
		Target       Target `json:"target"`
		Server       struct {
			Command   string   `json:"command"`
			Arguments []string `json:"arguments"`
		} `json:"server"`
		Limits     Limits               `json:"limits"`
		Provenance InvocationProvenance `json:"provenance,omitempty"`
	}
	type resultV2 struct {
		SchemaVersion         string                 `json:"schema_version"`
		Tool                  ToolIdentity           `json:"tool"`
		Invocation            invocationV2           `json:"invocation"`
		EvidenceSemantics     EvidenceSemantics      `json:"evidence_semantics"`
		TraceReceipt          *TraceReceipt          `json:"trace_receipt,omitempty"`
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
		EvidenceReceipt       *EvidenceReceipt       `json:"evidence_receipt,omitempty"`
		Seeds                 []SeedResult           `json:"seeds,omitempty"`
		Summary               summaryV2              `json:"summary"`
	}
	legacyServer := struct {
		Command   string   `json:"command"`
		Arguments []string `json:"arguments"`
	}{invocation.Server.Command, invocation.Server.Arguments}
	legacyInvocation := invocationV2{invocation.WorkspaceURI, invocation.Target, legacyServer, invocation.Limits, invocation.Provenance}
	payload := resultV2{
		SchemaVersion: r.SchemaVersion, Tool: tool, Invocation: legacyInvocation,
		EvidenceSemantics: evidenceSemantics(),
		Capabilities:      r.Capabilities, CapabilityQuality: r.CapabilityQuality,
		Targets: r.Targets, Nodes: r.Nodes, Edges: r.Edges, Terminals: r.Terminals,
		Frontier: r.Frontier, Diagnostics: r.Diagnostics,
		SiblingCandidates: r.SiblingCandidates, DispatchRelationships: r.DispatchRelationships,
		EvidenceReceipt: r.evidenceReceipt(invocation.Provenance.SourceRevision), Seeds: r.Seeds,
		Summary: summaryV2{r.Summary.NodeCount, r.Summary.EdgeCount, r.Summary.TerminalCount, r.Summary.CycleCount, r.Summary.Complete, Unknown, CompletenessScope, r.Summary.Truncated},
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payload.TraceReceipt = &TraceReceipt{
		ReceiptVersion: "lsp-trace.receipt.v1",
		ContentDigest:  domainDigest("lsp-trace:trace-receipt:v1", canonical),
		DigestScope:    "CANONICAL_GRAPH_WITHOUT_TRACE_RECEIPT",
	}
	return json.Marshal(payload)
}

func normalizedInvocation(invocation Invocation) Invocation {
	values := []*string{
		&invocation.Provenance.InvocationID,
		&invocation.Provenance.Caller,
		&invocation.Provenance.Source,
		&invocation.Provenance.SourceRevision,
		&invocation.Provenance.ServerVersion,
		&invocation.Provenance.Timestamp,
	}
	for _, value := range values {
		if *value == "" {
			*value = Unknown
		}
	}
	return invocation
}

func evidenceSemantics() EvidenceSemantics {
	unsupported := []string{"runtime_execution", "feature_identity", "whole_source_completeness", "independent_source_confirmation"}
	return EvidenceSemantics{
		CallEdges: ClaimCeiling{
			EvidenceClass:       "SERVER_REPORTED_CALL_HIERARCHY",
			Supports:            []string{"server_reported_caller_callee_relation"},
			DoesNotSupport:      unsupported,
			SupportContribution: 1,
		},
		DiscoveryRelations: ClaimCeiling{
			EvidenceClass:       "DISCOVERY_NOMINATION",
			Supports:            []string{"candidate_for_separate_investigation"},
			DoesNotSupport:      []string{"caller_callee_relation", "runtime_execution", "feature_identity", "whole_source_completeness"},
			SupportContribution: 0,
		},
	}
}

func (r Result) evidenceReceipt(sourceRevision string) *EvidenceReceipt {
	relations := make([]EvidenceRelation, 0, len(r.Edges)+len(r.SiblingCandidates)+len(r.DispatchRelationships))
	for _, edge := range r.Edges {
		relations = append(relations, newEvidenceRelation("CALL_RELATION", "CALLER_TO_CALLEE", edge.CallerNodeID+"->"+edge.CalleeNodeID, sourceRevision, "", "", "", "", "", edge.CallerNodeID, edge.CalleeNodeID))
	}
	for _, candidate := range r.SiblingCandidates {
		relations = append(relations, newEvidenceRelation("SIBLING_CANDIDATE", "DISCOVERY", candidate.SeedURI, sourceRevision, candidate.SeedURI, candidate.SeedLabel, candidate.Candidate.ID, "", "", "", ""))
	}
	for _, relationship := range r.DispatchRelationships {
		relations = append(relations, newEvidenceRelation("DISPATCH_ASSOCIATION", "ASSOCIATION", relationship.SeedLabel, sourceRevision, "", relationship.SeedLabel, "", relationship.Interface.ID, relationship.Implementation.ID, "", ""))
	}
	if len(relations) == 0 {
		return nil
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].RelationID < relations[j].RelationID })
	out := relations[:1]
	for _, relation := range relations[1:] {
		if relation.RelationID != out[len(out)-1].RelationID {
			out = append(out, relation)
		}
	}
	supportTotal := 0
	for _, relation := range out {
		supportTotal += relation.SupportContribution
	}
	return &EvidenceReceipt{SupportTotal: supportTotal, Relations: out}
}

func newEvidenceRelation(kind, direction, locator, sourceRevision, seedURI, seedLabel, candidateID, interfaceID, implementationID, callerID, calleeID string) EvidenceRelation {
	identityInput := struct {
		Version          string `json:"version"`
		EvidenceClass    string `json:"evidence_class"`
		RelationKind     string `json:"relation_kind"`
		SourceRevision   string `json:"source_revision"`
		Direction        string `json:"direction"`
		Locator          string `json:"locator"`
		SeedURI          string `json:"seed_uri"`
		SeedLabel        string `json:"seed_label"`
		CandidateID      string `json:"candidate_node_id"`
		InterfaceID      string `json:"interface_node_id"`
		ImplementationID string `json:"implementation_node_id"`
		CallerID         string `json:"caller_node_id"`
		CalleeID         string `json:"callee_node_id"`
	}{"lsp-trace.evidence-relation.v2", evidenceClassForKind(kind), kind, sourceRevision, direction, locator, seedURI, seedLabel, candidateID, interfaceID, implementationID, callerID, calleeID}
	encoded, err := json.Marshal(identityInput)
	if err != nil {
		panic(err)
	}
	return EvidenceRelation{
		RelationID:   domainDigest("lsp-trace:evidence-relation:v2", encoded),
		RelationKind: kind, EvidenceClass: evidenceClassForKind(kind), EvidenceRole: evidenceRoleForKind(kind),
		Direction: direction, Locator: locator, SourceRevision: sourceRevision, SupportContribution: supportForKind(kind),
		SeedURI: seedURI, SeedLabel: seedLabel, CandidateNodeID: candidateID, InterfaceNodeID: interfaceID, ImplementationNodeID: implementationID,
		CallerNodeID: callerID, CalleeNodeID: calleeID,
	}
}

func evidenceClassForKind(kind string) string {
	if kind == "CALL_RELATION" {
		return "SERVER_REPORTED_CALL_HIERARCHY"
	}
	return "DISCOVERY_NOMINATION"
}
func evidenceRoleForKind(kind string) string {
	if kind == "CALL_RELATION" {
		return "CALL_SUPPORT"
	}
	return "DISCOVERY_ONLY"
}
func supportForKind(kind string) int {
	if kind == "CALL_RELATION" {
		return 1
	}
	return 0
}

func domainDigest(domain string, payload []byte) string {
	input := append(append([]byte(domain), 0), payload...)
	sum := sha256.Sum256(input)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (r *Result) Canonicalize() {
	for i := range r.Edges {
		r.Edges[i].CallSites = mergeRanges(nil, r.Edges[i].CallSites)
	}
	sort.Slice(r.Nodes, func(i, j int) bool {
		a, b := r.Nodes[i], r.Nodes[j]
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		if a.SelectionRange != b.SelectionRange {
			return lessRange(a.SelectionRange, b.SelectionRange)
		}
		if a.Range != b.Range {
			return lessRange(a.Range, b.Range)
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Detail != b.Detail {
			return a.Detail < b.Detail
		}
		return a.ID < b.ID
	})
	sort.Slice(r.Edges, func(i, j int) bool {
		a, b := r.Edges[i], r.Edges[j]
		if a.CallerNodeID != b.CallerNodeID {
			return a.CallerNodeID < b.CallerNodeID
		}
		return a.CalleeNodeID < b.CalleeNodeID
	})
	sort.Strings(r.Targets)
	r.Targets = uniqueStrings(r.Targets)
	sort.Slice(r.SiblingCandidates, func(i, j int) bool {
		a, b := r.SiblingCandidates[i], r.SiblingCandidates[j]
		if a.SeedURI != b.SeedURI {
			return a.SeedURI < b.SeedURI
		}
		return a.Candidate.ID < b.Candidate.ID
	})
	sort.Slice(r.DispatchRelationships, func(i, j int) bool {
		a, b := r.DispatchRelationships[i], r.DispatchRelationships[j]
		if a.SeedLabel != b.SeedLabel {
			return a.SeedLabel < b.SeedLabel
		}
		if a.Interface.ID != b.Interface.ID {
			return a.Interface.ID < b.Interface.ID
		}
		return a.Implementation.ID < b.Implementation.ID
	})
	if len(r.DispatchRelationships) > 0 {
		out := r.DispatchRelationships[:1]
		for _, relationship := range r.DispatchRelationships[1:] {
			last := out[len(out)-1]
			if relationship.SeedLabel != last.SeedLabel || relationship.Interface.ID != last.Interface.ID || relationship.Implementation.ID != last.Implementation.ID {
				out = append(out, relationship)
			}
		}
		r.DispatchRelationships = out
	}
	if len(r.SiblingCandidates) > 0 {
		out := r.SiblingCandidates[:1]
		for _, candidate := range r.SiblingCandidates[1:] {
			last := out[len(out)-1]
			if candidate.SeedURI != last.SeedURI || candidate.Candidate.ID != last.Candidate.ID {
				out = append(out, candidate)
			}
		}
		r.SiblingCandidates = out
	}
	for i := range r.Seeds {
		sort.Strings(r.Seeds[i].PreparedTargetIDs)
		r.Seeds[i].PreparedTargetIDs = uniqueStrings(r.Seeds[i].PreparedTargetIDs)
		sort.Strings(r.Seeds[i].ReachedNodeIDs)
		r.Seeds[i].ReachedNodeIDs = uniqueStrings(r.Seeds[i].ReachedNodeIDs)
	}
	sort.Slice(r.Seeds, func(i, j int) bool { return r.Seeds[i].Label < r.Seeds[j].Label })
	sort.Slice(r.Terminals, func(i, j int) bool { return lessBoundary(r.Terminals[i], r.Terminals[j]) })
	sort.Slice(r.Frontier, func(i, j int) bool { return lessBoundary(r.Frontier[i], r.Frontier[j]) })
	sort.Slice(r.Diagnostics, func(i, j int) bool {
		a, b := r.Diagnostics[i], r.Diagnostics[j]
		if a.Phase != b.Phase {
			return a.Phase < b.Phase
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.NodeID != b.NodeID {
			return a.NodeID < b.NodeID
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		return a.Message < b.Message
	})
	r.Summary.NodeCount = len(r.Nodes)
	r.Summary.EdgeCount = len(r.Edges)
	r.Summary.TerminalCount = len(r.Terminals)
	r.Summary.CycleCount = cycleCount(r.Nodes, r.Edges)
	for i := range r.Terminals {
		if r.Terminals[i].Provenance == "" {
			r.Terminals[i].Provenance = boundaryProvenance(r.Terminals[i].Reason)
		}
	}
	for i := range r.Frontier {
		if r.Frontier[i].Provenance == "" {
			r.Frontier[i].Provenance = "CLIENT_DERIVED"
		}
	}
	if r.SchemaVersion != SchemaVersionV1 {
		r.CapabilityQuality.IncomingEdges = len(r.Edges)
		r.CapabilityQuality.UnresolvedCalls = 0
		r.CapabilityQuality.DynamicCalls = 0
		for _, diagnostic := range r.Diagnostics {
			switch diagnostic.Category {
			case UnresolvedCall:
				r.CapabilityQuality.UnresolvedCalls++
			case DynamicCall:
				r.CapabilityQuality.DynamicCalls++
			}
		}
		if r.CapabilityQuality.UnresolvedCalls > 0 {
			r.Summary.Complete = false
		}
		if r.CapabilityQuality.CrossModuleEdges == "" {
			r.CapabilityQuality.CrossModuleEdges = Unknown
		}
	}
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := in[:1]
	for _, value := range in[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

func MergeResults(results ...Result) Result {
	out := Result{Summary: Summary{Complete: true}, CapabilityQuality: CapabilityQuality{CrossModuleEdges: Unknown}}
	nodes := map[string]Node{}
	for _, result := range results {
		if out.SchemaVersion == "" && result.SchemaVersion != "" {
			out.SchemaVersion = result.SchemaVersion
		}
		out.Capabilities.CallHierarchyProvider = out.Capabilities.CallHierarchyProvider || result.Capabilities.CallHierarchyProvider
		out.CapabilityQuality.Advertised = out.CapabilityQuality.Advertised || result.CapabilityQuality.Advertised
		out.CapabilityQuality.PrepareSucceeded = out.CapabilityQuality.PrepareSucceeded || result.CapabilityQuality.PrepareSucceeded
		out.CapabilityQuality.IncomingRequestSuccesses += result.CapabilityQuality.IncomingRequestSuccesses
		out.CapabilityQuality.CrossFileEdges += result.CapabilityQuality.CrossFileEdges
		out.Summary.Complete = out.Summary.Complete && result.Summary.Complete
		out.Summary.Truncated = out.Summary.Truncated || result.Summary.Truncated
		out.Targets = append(out.Targets, result.Targets...)
		out.Seeds = append(out.Seeds, result.Seeds...)
		out.SiblingCandidates = append(out.SiblingCandidates, result.SiblingCandidates...)
		out.DispatchRelationships = append(out.DispatchRelationships, result.DispatchRelationships...)
		out.Terminals = append(out.Terminals, result.Terminals...)
		out.Frontier = append(out.Frontier, result.Frontier...)
		out.Diagnostics = append(out.Diagnostics, result.Diagnostics...)
		for _, node := range result.Nodes {
			if existing, ok := nodes[node.ID]; !ok || SameNodeIdentity(existing, node) {
				nodes[node.ID] = node
			} else {
				out.Terminals = append(out.Terminals, Boundary{NodeID: node.ID, Reason: NodeIDCollision})
				out.Summary.Complete = false
			}
		}
		for _, edge := range result.Edges {
			out.Edges = MergeEdge(out.Edges, edge)
		}
	}
	for _, node := range nodes {
		out.Nodes = append(out.Nodes, node)
	}
	out.Terminals = uniqueBoundaries(out.Terminals)
	out.Frontier = uniqueBoundaries(out.Frontier)
	out.Diagnostics = uniqueDiagnostics(out.Diagnostics)
	if out.SchemaVersion == "" {
		out.SchemaVersion = SchemaVersion
	}
	out.Canonicalize()
	return out
}

func uniqueBoundaries(in []Boundary) []Boundary {
	seen := map[Boundary]struct{}{}
	out := make([]Boundary, 0, len(in))
	for _, item := range in {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func uniqueDiagnostics(in []Diagnostic) []Diagnostic {
	seen := map[Diagnostic]struct{}{}
	out := make([]Diagnostic, 0, len(in))
	for _, item := range in {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func boundaryProvenance(reason Reason) string {
	if reason == ServerReportedNoIncoming || reason == IncomingReturnedNull {
		return "SERVER_REPORTED"
	}
	return "CLIENT_DERIVED"
}

func lessBoundary(a, b Boundary) bool {
	if a.NodeID != b.NodeID {
		return a.NodeID < b.NodeID
	}
	if a.Reason != b.Reason {
		return a.Reason < b.Reason
	}
	return a.Message < b.Message
}

func cycleCount(nodes []Node, edges []Edge) int {
	adj := make(map[string][]string, len(nodes))
	selfLoop := make(map[string]bool)
	for _, edge := range edges {
		adj[edge.CallerNodeID] = append(adj[edge.CallerNodeID], edge.CalleeNodeID)
		if edge.CallerNodeID == edge.CalleeNodeID {
			selfLoop[edge.CallerNodeID] = true
		}
	}
	index := 0
	indices := make(map[string]int, len(nodes))
	lowlink := make(map[string]int, len(nodes))
	onStack := make(map[string]bool, len(nodes))
	stack := make([]string, 0, len(nodes))
	cycles := 0
	var visit func(string)
	visit = func(v string) {
		indices[v], lowlink[v] = index, index
		index++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range adj[v] {
			if _, ok := indices[w]; !ok {
				visit(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] && indices[w] < lowlink[v] {
				lowlink[v] = indices[w]
			}
		}
		if lowlink[v] != indices[v] {
			return
		}
		size := 0
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			size++
			if w == v {
				break
			}
		}
		if size > 1 || selfLoop[v] {
			cycles++
		}
	}
	for _, node := range nodes {
		if _, ok := indices[node.ID]; !ok {
			visit(node.ID)
		}
	}
	return cycles
}
