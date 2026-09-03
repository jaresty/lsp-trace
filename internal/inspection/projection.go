package inspection

import (
	"encoding/json"
	"fmt"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
)

type Summary struct {
	NodeCount           int    `json:"node_count"`
	EdgeCount           int    `json:"edge_count"`
	TerminalCount       int    `json:"terminal_count"`
	CycleCount          int    `json:"cycle_count"`
	TraversalComplete   bool   `json:"traversal_complete"`
	SourceGraphComplete string `json:"source_graph_complete"`
	CompletenessScope   string `json:"completeness_scope"`
	Truncated           bool   `json:"truncated"`
}

type Bundle struct {
	SchemaVersion         string                       `json:"schema_version"`
	ExecutionBundleID     string                       `json:"execution_bundle_id,omitempty"`
	Invocation            graph.Invocation             `json:"invocation"`
	Nodes                 []graph.Node                 `json:"nodes"`
	Edges                 []graph.Edge                 `json:"edges"`
	DispatchRelationships []graph.DispatchRelationship `json:"dispatch_relationships"`
	SiblingCandidates     []graph.SiblingCandidate     `json:"sibling_candidates"`
	Terminals             []graph.Boundary             `json:"terminals"`
	Frontier              []graph.Boundary             `json:"frontier"`
	Diagnostics           []graph.Diagnostic           `json:"diagnostics"`
	Seeds                 []graph.SeedResult           `json:"seeds"`
	SeedMemberships       []graph.SeedMembership       `json:"seed_memberships"`
	Summary               Summary                      `json:"summary"`
	TraceReceipt          struct {
		SemanticCommitmentDigest string `json:"semantic_commitment_digest"`
	} `json:"trace_receipt"`
}

type Projection struct {
	InspectionSchemaVersion string `json:"inspection_schema_version"`
	ProjectionKind          string `json:"projection_kind"`
	Authority               string `json:"authority"`
	ArtifactIdentity        struct {
		ExecutionBundleID          string `json:"execution_bundle_id,omitempty"`
		SemanticCommitmentDigest   string `json:"semantic_commitment_digest"`
		ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
	} `json:"artifact_identity"`
	PreparationStatus string                 `json:"preparation_status"`
	Seed              graph.SeedResult       `json:"seed"`
	SeedMemberships   []graph.SeedMembership `json:"seed_memberships"`
	Nodes             []graph.Node           `json:"nodes"`
	Relations         []graph.Edge           `json:"relations"`
	Global            struct {
		Summary     Summary            `json:"summary"`
		Terminals   []graph.Boundary   `json:"terminals"`
		Frontier    []graph.Boundary   `json:"frontier"`
		Diagnostics []graph.Diagnostic `json:"diagnostics"`
	} `json:"global"`
	DiagnosticsOnReachedNodes struct {
		Authority   string             `json:"authority"`
		Diagnostics []graph.Diagnostic `json:"diagnostics"`
	} `json:"diagnostics_on_reached_nodes"`
}

type AllSeed struct {
	PreparationStatus           string                 `json:"preparation_status"`
	Seed                        graph.SeedResult       `json:"seed"`
	SeedMemberships             []graph.SeedMembership `json:"seed_memberships"`
	NativeNodeIDs               []string               `json:"native_node_ids"`
	NativeCallRelationIDs       []string               `json:"native_call_relation_ids"`
	DiscoveryNominationIDs      []string               `json:"discovery_nomination_ids"`
	CorrelatedDiagnosticIndexes []int                  `json:"correlated_diagnostic_indexes"`
}

type Records struct {
	Nodes                 []graph.Node                 `json:"nodes"`
	CallRelations         []graph.Edge                 `json:"call_relations"`
	DispatchRelationships []graph.DispatchRelationship `json:"dispatch_relationships"`
	SiblingCandidates     []graph.SiblingCandidate     `json:"sibling_candidates"`
	Diagnostics           []graph.Diagnostic           `json:"diagnostics"`
	Terminals             []graph.Boundary             `json:"terminals"`
	Frontier              []graph.Boundary             `json:"frontier"`
}

type Accounting struct {
	RequestedSeedCount                     int    `json:"requested_seed_count"`
	SuccessfulSeedCount                    int    `json:"successful_seed_count"`
	FailedSeedCount                        int    `json:"failed_seed_count"`
	SuccessfulSeedWithMembershipCount      int    `json:"successful_seed_with_membership_count"`
	SuccessfulSeedWithoutMembershipCount   int    `json:"successful_seed_without_membership_count"`
	GlobalNodeRecordCount                  int    `json:"global_node_record_count"`
	GlobalCallRelationRecordCount          int    `json:"global_call_relation_record_count"`
	GlobalDispatchRelationshipRecordCount  int    `json:"global_dispatch_relationship_record_count"`
	GlobalSiblingCandidateRecordCount      int    `json:"global_sibling_candidate_record_count"`
	GlobalDiagnosticRecordCount            int    `json:"global_diagnostic_record_count"`
	GlobalTerminalRecordCount              int    `json:"global_terminal_record_count"`
	GlobalFrontierRecordCount              int    `json:"global_frontier_record_count"`
	SeedMembershipRecordCount              int    `json:"seed_membership_record_count"`
	SeedNodeReferenceCount                 int    `json:"seed_node_reference_count"`
	SeedCallRelationReferenceCount         int    `json:"seed_call_relation_reference_count"`
	SeedDiscoveryNominationReferenceCount  int    `json:"seed_discovery_nomination_reference_count"`
	SeedCorrelatedDiagnosticReferenceCount int    `json:"seed_correlated_diagnostic_reference_count"`
	Truncated                              bool   `json:"truncated"`
	TraversalComplete                      bool   `json:"traversal_complete"`
	SourceGraphComplete                    string `json:"source_graph_complete"`
}

type AllProjection struct {
	InspectionSchemaVersion string `json:"inspection_schema_version"`
	ProjectionKind          string `json:"projection_kind"`
	Authority               string `json:"authority"`
	ArtifactIdentity        struct {
		ExecutionBundleID          string `json:"execution_bundle_id,omitempty"`
		SemanticCommitmentDigest   string `json:"semantic_commitment_digest"`
		ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
	} `json:"artifact_identity"`
	Records    Records    `json:"records"`
	Seeds      []AllSeed  `json:"seeds"`
	Accounting Accounting `json:"accounting"`
}

func ValidateAllSeedAccounting(projection AllProjection) error {
	expected := Accounting{
		RequestedSeedCount:                    len(projection.Seeds),
		GlobalNodeRecordCount:                 len(projection.Records.Nodes),
		GlobalCallRelationRecordCount:         len(projection.Records.CallRelations),
		GlobalDispatchRelationshipRecordCount: len(projection.Records.DispatchRelationships),
		GlobalSiblingCandidateRecordCount:     len(projection.Records.SiblingCandidates),
		GlobalDiagnosticRecordCount:           len(projection.Records.Diagnostics),
		GlobalTerminalRecordCount:             len(projection.Records.Terminals),
		GlobalFrontierRecordCount:             len(projection.Records.Frontier),
	}
	for _, seed := range projection.Seeds {
		if seed.PreparationStatus == "FAILED" {
			expected.FailedSeedCount++
		} else {
			expected.SuccessfulSeedCount++
			if len(seed.SeedMemberships) == 0 {
				expected.SuccessfulSeedWithoutMembershipCount++
			} else {
				expected.SuccessfulSeedWithMembershipCount++
			}
		}
		expected.SeedMembershipRecordCount += len(seed.SeedMemberships)
		expected.SeedNodeReferenceCount += len(seed.NativeNodeIDs)
		expected.SeedCallRelationReferenceCount += len(seed.NativeCallRelationIDs)
		expected.SeedDiscoveryNominationReferenceCount += len(seed.DiscoveryNominationIDs)
		expected.SeedCorrelatedDiagnosticReferenceCount += len(seed.CorrelatedDiagnosticIndexes)
	}
	checks := []struct {
		field     string
		got, want int
	}{
		{"requested_seed_count", projection.Accounting.RequestedSeedCount, expected.RequestedSeedCount},
		{"successful_seed_count", projection.Accounting.SuccessfulSeedCount, expected.SuccessfulSeedCount},
		{"failed_seed_count", projection.Accounting.FailedSeedCount, expected.FailedSeedCount},
		{"successful_seed_with_membership_count", projection.Accounting.SuccessfulSeedWithMembershipCount, expected.SuccessfulSeedWithMembershipCount},
		{"successful_seed_without_membership_count", projection.Accounting.SuccessfulSeedWithoutMembershipCount, expected.SuccessfulSeedWithoutMembershipCount},
		{"global_node_record_count", projection.Accounting.GlobalNodeRecordCount, expected.GlobalNodeRecordCount},
		{"global_call_relation_record_count", projection.Accounting.GlobalCallRelationRecordCount, expected.GlobalCallRelationRecordCount},
		{"global_dispatch_relationship_record_count", projection.Accounting.GlobalDispatchRelationshipRecordCount, expected.GlobalDispatchRelationshipRecordCount},
		{"global_sibling_candidate_record_count", projection.Accounting.GlobalSiblingCandidateRecordCount, expected.GlobalSiblingCandidateRecordCount},
		{"global_diagnostic_record_count", projection.Accounting.GlobalDiagnosticRecordCount, expected.GlobalDiagnosticRecordCount},
		{"global_terminal_record_count", projection.Accounting.GlobalTerminalRecordCount, expected.GlobalTerminalRecordCount},
		{"global_frontier_record_count", projection.Accounting.GlobalFrontierRecordCount, expected.GlobalFrontierRecordCount},
		{"seed_membership_record_count", projection.Accounting.SeedMembershipRecordCount, expected.SeedMembershipRecordCount},
		{"seed_node_reference_count", projection.Accounting.SeedNodeReferenceCount, expected.SeedNodeReferenceCount},
		{"seed_call_relation_reference_count", projection.Accounting.SeedCallRelationReferenceCount, expected.SeedCallRelationReferenceCount},
		{"seed_discovery_nomination_reference_count", projection.Accounting.SeedDiscoveryNominationReferenceCount, expected.SeedDiscoveryNominationReferenceCount},
		{"seed_correlated_diagnostic_reference_count", projection.Accounting.SeedCorrelatedDiagnosticReferenceCount, expected.SeedCorrelatedDiagnosticReferenceCount},
	}
	for _, check := range checks {
		if check.got != check.want {
			return fmt.Errorf("%s: got %d, want %d", check.field, check.got, check.want)
		}
	}
	return nil
}

func ProjectSeed(data []byte, label string) (Projection, error) {
	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return Projection{}, err
	}
	var selected *graph.SeedResult
	for i := range bundle.Seeds {
		if bundle.Seeds[i].Label == label {
			selected = &bundle.Seeds[i]
			break
		}
	}
	if selected == nil {
		return Projection{}, fmt.Errorf("seed label %q not found", label)
	}

	out := Projection{InspectionSchemaVersion: schema.InspectionVersionV1, ProjectionKind: "SEED_INSPECTION", Authority: "NON_AUTHORITATIVE_DERIVED_VIEW", Seed: *selected}
	if out.Seed.PreparedTargetIDs == nil {
		out.Seed.PreparedTargetIDs = make([]string, 0)
	}
	if out.Seed.ReachedNodeIDs == nil {
		out.Seed.ReachedNodeIDs = make([]string, 0)
	}
	if out.Seed.ReachedRelationIDs == nil {
		out.Seed.ReachedRelationIDs = make([]string, 0)
	}
	out.ArtifactIdentity.ExecutionBundleID = bundle.ExecutionBundleID
	out.ArtifactIdentity.SemanticCommitmentDigest = bundle.TraceReceipt.SemanticCommitmentDigest
	out.ArtifactIdentity.ExactSerializedBytesDigest = graph.ExactBytesDigest(data)
	if selected.Failure != nil {
		out.PreparationStatus = "FAILED"
	} else {
		out.PreparationStatus = "SUCCEEDED"
	}
	reachedNodes := make(map[string]bool, len(selected.ReachedNodeIDs))
	for _, id := range selected.ReachedNodeIDs {
		reachedNodes[id] = true
	}
	reachedRelations := make(map[string]bool, len(selected.ReachedRelationIDs))
	for _, id := range selected.ReachedRelationIDs {
		reachedRelations[id] = true
	}
	out.SeedMemberships = make([]graph.SeedMembership, 0)
	for _, membership := range bundle.SeedMemberships {
		if membership.SeedLabel == label {
			out.SeedMemberships = append(out.SeedMemberships, membership)
		}
	}
	out.Nodes = make([]graph.Node, 0)
	for _, node := range bundle.Nodes {
		if reachedNodes[node.ID] {
			out.Nodes = append(out.Nodes, node)
		}
	}
	out.Relations = make([]graph.Edge, 0)
	for _, relation := range bundle.Edges {
		if reachedRelations[relation.RelationID] {
			out.Relations = append(out.Relations, relation)
		}
	}
	out.Global.Summary = bundle.Summary
	out.Global.Terminals = bundle.Terminals
	if out.Global.Terminals == nil {
		out.Global.Terminals = make([]graph.Boundary, 0)
	}
	out.Global.Frontier = bundle.Frontier
	if out.Global.Frontier == nil {
		out.Global.Frontier = make([]graph.Boundary, 0)
	}
	out.Global.Diagnostics = bundle.Diagnostics
	if out.Global.Diagnostics == nil {
		out.Global.Diagnostics = make([]graph.Diagnostic, 0)
	}
	out.DiagnosticsOnReachedNodes.Authority = "TOOL_DERIVED_NODE_CORRELATION"
	out.DiagnosticsOnReachedNodes.Diagnostics = make([]graph.Diagnostic, 0)
	for _, diagnostic := range bundle.Diagnostics {
		if diagnostic.NodeID != "" && reachedNodes[diagnostic.NodeID] {
			out.DiagnosticsOnReachedNodes.Diagnostics = append(out.DiagnosticsOnReachedNodes.Diagnostics, diagnostic)
		}
	}
	return out, nil
}

func ProjectAllSeeds(data []byte) (AllProjection, error) {
	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return AllProjection{}, err
	}
	out := AllProjection{InspectionSchemaVersion: schema.InspectionVersionV1, ProjectionKind: "ALL_SEEDS", Authority: "NON_AUTHORITATIVE_DERIVED_VIEW"}
	out.ArtifactIdentity.ExecutionBundleID = bundle.ExecutionBundleID
	out.ArtifactIdentity.SemanticCommitmentDigest = bundle.TraceReceipt.SemanticCommitmentDigest
	out.ArtifactIdentity.ExactSerializedBytesDigest = graph.ExactBytesDigest(data)
	out.Records = Records{
		Nodes: append([]graph.Node{}, bundle.Nodes...), CallRelations: append([]graph.Edge{}, bundle.Edges...),
		DispatchRelationships: append([]graph.DispatchRelationship{}, bundle.DispatchRelationships...),
		SiblingCandidates:     append([]graph.SiblingCandidate{}, bundle.SiblingCandidates...),
		Diagnostics:           append([]graph.Diagnostic{}, bundle.Diagnostics...), Terminals: append([]graph.Boundary{}, bundle.Terminals...), Frontier: append([]graph.Boundary{}, bundle.Frontier...),
	}

	results := make(map[string][]graph.SeedResult, len(bundle.Seeds))
	for _, result := range bundle.Seeds {
		results[result.Label] = append(results[result.Label], result)
	}
	out.Seeds = make([]AllSeed, 0, len(bundle.Invocation.Seeds))
	for _, requested := range bundle.Invocation.Seeds {
		matches := results[requested.Label]
		if len(matches) != 1 {
			return AllProjection{}, fmt.Errorf("seed label %q has %d results; expected exactly one", requested.Label, len(matches))
		}
		seed := matches[0]
		if seed.PreparedTargetIDs == nil {
			seed.PreparedTargetIDs = []string{}
		}
		if seed.ReachedNodeIDs == nil {
			seed.ReachedNodeIDs = []string{}
		}
		if seed.ReachedRelationIDs == nil {
			seed.ReachedRelationIDs = []string{}
		}
		entry := AllSeed{PreparationStatus: "SUCCEEDED", Seed: seed, SeedMemberships: []graph.SeedMembership{}, NativeNodeIDs: append([]string{}, seed.ReachedNodeIDs...), NativeCallRelationIDs: append([]string{}, seed.ReachedRelationIDs...), DiscoveryNominationIDs: []string{}, CorrelatedDiagnosticIndexes: []int{}}
		if seed.Failure != nil {
			entry.PreparationStatus = "FAILED"
			out.Accounting.FailedSeedCount++
		} else {
			out.Accounting.SuccessfulSeedCount++
		}
		for _, membership := range bundle.SeedMemberships {
			if membership.SeedLabel != requested.Label {
				continue
			}
			entry.SeedMemberships = append(entry.SeedMemberships, membership)
			if membership.EvidenceKind == "SIBLING_CANDIDATE" || membership.EvidenceKind == "DISPATCH_ASSOCIATION" {
				entry.DiscoveryNominationIDs = append(entry.DiscoveryNominationIDs, membership.EndpointID)
			}
		}
		if seed.Failure == nil {
			if len(entry.SeedMemberships) == 0 {
				out.Accounting.SuccessfulSeedWithoutMembershipCount++
			} else {
				out.Accounting.SuccessfulSeedWithMembershipCount++
			}
		}
		reached := make(map[string]bool, len(seed.ReachedNodeIDs))
		for _, id := range seed.ReachedNodeIDs {
			reached[id] = true
		}
		for i, diagnostic := range bundle.Diagnostics {
			if diagnostic.NodeID != "" && reached[diagnostic.NodeID] {
				entry.CorrelatedDiagnosticIndexes = append(entry.CorrelatedDiagnosticIndexes, i)
			}
		}
		out.Accounting.SeedMembershipRecordCount += len(entry.SeedMemberships)
		out.Accounting.SeedNodeReferenceCount += len(entry.NativeNodeIDs)
		out.Accounting.SeedCallRelationReferenceCount += len(entry.NativeCallRelationIDs)
		out.Accounting.SeedDiscoveryNominationReferenceCount += len(entry.DiscoveryNominationIDs)
		out.Accounting.SeedCorrelatedDiagnosticReferenceCount += len(entry.CorrelatedDiagnosticIndexes)
		out.Seeds = append(out.Seeds, entry)
	}
	out.Accounting.RequestedSeedCount = len(bundle.Invocation.Seeds)
	out.Accounting.GlobalNodeRecordCount = len(out.Records.Nodes)
	out.Accounting.GlobalCallRelationRecordCount = len(out.Records.CallRelations)
	out.Accounting.GlobalDispatchRelationshipRecordCount = len(out.Records.DispatchRelationships)
	out.Accounting.GlobalSiblingCandidateRecordCount = len(out.Records.SiblingCandidates)
	out.Accounting.GlobalDiagnosticRecordCount = len(out.Records.Diagnostics)
	out.Accounting.GlobalTerminalRecordCount = len(out.Records.Terminals)
	out.Accounting.GlobalFrontierRecordCount = len(out.Records.Frontier)
	out.Accounting.Truncated = bundle.Summary.Truncated
	out.Accounting.TraversalComplete = bundle.Summary.TraversalComplete
	out.Accounting.SourceGraphComplete = bundle.Summary.SourceGraphComplete
	if err := ValidateAllSeedAccounting(out); err != nil {
		return AllProjection{}, fmt.Errorf("inspection accounting: %w", err)
	}
	return out, nil
}
