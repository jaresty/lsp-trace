// Package filter projects deterministic, non-adjudicative pairwise seed-evidence comparisons.
package filter

import (
	"encoding/json"
	"fmt"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
)

const VersionV1 = "lsp-trace.filter.v1"

type StringPartition struct{ Shared, LeftOnly, RightOnly []string }

func (p StringPartition) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Shared []string `json:"shared"`
		Left   []string `json:"left_only"`
		Right  []string `json:"right_only"`
	}{p.Shared, p.LeftOnly, p.RightOnly})
}

type IndexPartition struct{ Shared, LeftOnly, RightOnly []int }

func (p IndexPartition) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Shared []int `json:"shared"`
		Left   []int `json:"left_only"`
		Right  []int `json:"right_only"`
	}{p.Shared, p.LeftOnly, p.RightOnly})
}

type NamespaceAccounting struct {
	LeftReferenceCount      int `json:"left_reference_count"`
	RightReferenceCount     int `json:"right_reference_count"`
	SharedReferenceCount    int `json:"shared_reference_count"`
	LeftOnlyReferenceCount  int `json:"left_only_reference_count"`
	RightOnlyReferenceCount int `json:"right_only_reference_count"`
	PairUniverseCount       int `json:"pair_universe_count"`
}
type Seed struct {
	Label   string             `json:"label"`
	State   string             `json:"state"`
	Failure *graph.SeedFailure `json:"failure"`
}
type Projection struct {
	FilterSchemaVersion   string `json:"filter_schema_version"`
	ProjectionKind        string `json:"projection_kind"`
	Authority             string `json:"authority"`
	SupportContribution   int    `json:"support_contribution"`
	NativeSemanticsPolicy string `json:"native_semantics_policy"`
	InputIdentity         struct {
		InspectionExactBytesDigest         string `json:"inspection_exact_bytes_digest"`
		ArtifactSemanticCommitmentDigest   string `json:"artifact_semantic_commitment_digest"`
		ArtifactExactSerializedBytesDigest string `json:"artifact_exact_serialized_bytes_digest"`
		ExecutionBundleID                  string `json:"execution_bundle_id,omitempty"`
	} `json:"input_identity"`
	Operands struct {
		LeftSeedLabel  string `json:"left_seed_label"`
		RightSeedLabel string `json:"right_seed_label"`
	} `json:"operands"`
	Seeds      []Seed `json:"seeds"`
	Partitions struct {
		Nodes                  StringPartition `json:"nodes"`
		CallRelations          StringPartition `json:"call_relations"`
		DispatchRelationships  StringPartition `json:"dispatch_relationships"`
		SiblingCandidates      StringPartition `json:"sibling_candidates"`
		DiagnosticCorrelations IndexPartition  `json:"diagnostic_correlations"`
	} `json:"partitions"`
	GlobalBoundary struct {
		Truncated           bool   `json:"truncated"`
		TraversalComplete   bool   `json:"traversal_complete"`
		SourceGraphComplete string `json:"source_graph_complete"`
	} `json:"global_boundary"`
	Accounting struct {
		RequestedSeedCount                   int                 `json:"requested_seed_count"`
		SuccessfulSeedCount                  int                 `json:"successful_seed_count"`
		FailedSeedCount                      int                 `json:"failed_seed_count"`
		SuccessfulSeedWithMembershipCount    int                 `json:"successful_seed_with_membership_count"`
		SuccessfulSeedWithoutMembershipCount int                 `json:"successful_seed_without_membership_count"`
		Nodes                                NamespaceAccounting `json:"nodes"`
		CallRelations                        NamespaceAccounting `json:"call_relations"`
		DispatchRelationships                NamespaceAccounting `json:"dispatch_relationships"`
		SiblingCandidates                    NamespaceAccounting `json:"sibling_candidates"`
		DiagnosticCorrelations               NamespaceAccounting `json:"diagnostic_correlations"`
	} `json:"accounting"`
	ClaimCeiling struct {
		Supports       []string `json:"supports"`
		DoesNotSupport []string `json:"does_not_support"`
	} `json:"claim_ceiling"`
}
type inspection struct {
	ArtifactIdentity struct {
		ExecutionBundleID          string `json:"execution_bundle_id,omitempty"`
		SemanticCommitmentDigest   string `json:"semantic_commitment_digest"`
		ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
	} `json:"artifact_identity"`
	Records struct {
		Nodes                 []graph.Node                 `json:"nodes"`
		CallRelations         []graph.Edge                 `json:"call_relations"`
		DispatchRelationships []graph.DispatchRelationship `json:"dispatch_relationships"`
		SiblingCandidates     []graph.SiblingCandidate     `json:"sibling_candidates"`
		Diagnostics           []graph.Diagnostic           `json:"diagnostics"`
	} `json:"records"`
	Seeds      []inspectionSeed     `json:"seeds"`
	Accounting inspectionAccounting `json:"accounting"`
}
type inspectionSeed struct {
	PreparationStatus           string                 `json:"preparation_status"`
	Seed                        graph.SeedResult       `json:"seed"`
	SeedMemberships             []graph.SeedMembership `json:"seed_memberships"`
	NativeNodeIDs               []string               `json:"native_node_ids"`
	NativeCallRelationIDs       []string               `json:"native_call_relation_ids"`
	CorrelatedDiagnosticIndexes []int                  `json:"correlated_diagnostic_indexes"`
}
type inspectionAccounting struct {
	RequestedSeedCount                   int    `json:"requested_seed_count"`
	SuccessfulSeedCount                  int    `json:"successful_seed_count"`
	FailedSeedCount                      int    `json:"failed_seed_count"`
	SuccessfulSeedWithMembershipCount    int    `json:"successful_seed_with_membership_count"`
	SuccessfulSeedWithoutMembershipCount int    `json:"successful_seed_without_membership_count"`
	Truncated                            bool   `json:"truncated"`
	TraversalComplete                    bool   `json:"traversal_complete"`
	SourceGraphComplete                  string `json:"source_graph_complete"`
}

func ProjectPairwise(data []byte, leftLabel, rightLabel string) (Projection, error) {
	var header struct {
		InspectionSchemaVersion string `json:"inspection_schema_version"`
		ProjectionKind          string `json:"projection_kind"`
		Authority               string `json:"authority"`
		SchemaVersion           string `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil || header.InspectionSchemaVersion != schema.InspectionVersionV1 || header.ProjectionKind != "ALL_SEEDS" || header.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || header.SchemaVersion != "" {
		return Projection{}, fmt.Errorf("input must be %s ALL_SEEDS", schema.InspectionVersionV1)
	}
	if err := schema.ValidateAllSeedInspection(data); err != nil {
		return Projection{}, err
	}
	var in inspection
	if err := json.Unmarshal(data, &in); err != nil {
		return Projection{}, err
	}
	var left, right *inspectionSeed
	for i := range in.Seeds {
		if in.Seeds[i].Seed.Label == leftLabel {
			left = &in.Seeds[i]
		}
		if in.Seeds[i].Seed.Label == rightLabel {
			right = &in.Seeds[i]
		}
	}
	if left == nil {
		return Projection{}, fmt.Errorf("seed label %q not found", leftLabel)
	}
	if right == nil {
		return Projection{}, fmt.Errorf("seed label %q not found", rightLabel)
	}
	out := Projection{FilterSchemaVersion: VersionV1, ProjectionKind: "SEED_EVIDENCE_COMPARISON", Authority: "TOOL_DERIVED_SET_PROJECTION", NativeSemanticsPolicy: "PRESERVE_WITHOUT_AUTHORITY_UPGRADE"}
	out.InputIdentity.InspectionExactBytesDigest = graph.ExactBytesDigest(data)
	out.InputIdentity.ArtifactSemanticCommitmentDigest = in.ArtifactIdentity.SemanticCommitmentDigest
	out.InputIdentity.ArtifactExactSerializedBytesDigest = in.ArtifactIdentity.ExactSerializedBytesDigest
	out.InputIdentity.ExecutionBundleID = in.ArtifactIdentity.ExecutionBundleID
	out.Operands.LeftSeedLabel, out.Operands.RightSeedLabel = leftLabel, rightLabel
	out.Seeds = []Seed{seedSummary(*left), seedSummary(*right)}
	out.Partitions.Nodes = partitionStrings(schema.ReferenceNode, nodeGlobals(in), left.NativeNodeIDs, right.NativeNodeIDs)
	out.Partitions.CallRelations = partitionStrings(schema.ReferenceCallRelation, callGlobals(in), left.NativeCallRelationIDs, right.NativeCallRelationIDs)
	ld, ls := membershipRefs(left.SeedMemberships)
	rd, rs := membershipRefs(right.SeedMemberships)
	out.Partitions.DispatchRelationships = partitionStrings(schema.ReferenceDispatchRelationship, dispatchGlobals(in), ld, rd)
	out.Partitions.SiblingCandidates = partitionStrings(schema.ReferenceSiblingCandidate, siblingGlobals(in), ls, rs)
	out.Partitions.DiagnosticCorrelations = partitionIndexes(len(in.Records.Diagnostics), left.CorrelatedDiagnosticIndexes, right.CorrelatedDiagnosticIndexes)
	out.GlobalBoundary.Truncated, out.GlobalBoundary.TraversalComplete, out.GlobalBoundary.SourceGraphComplete = in.Accounting.Truncated, in.Accounting.TraversalComplete, in.Accounting.SourceGraphComplete
	out.Accounting.RequestedSeedCount, out.Accounting.SuccessfulSeedCount, out.Accounting.FailedSeedCount = in.Accounting.RequestedSeedCount, in.Accounting.SuccessfulSeedCount, in.Accounting.FailedSeedCount
	out.Accounting.SuccessfulSeedWithMembershipCount, out.Accounting.SuccessfulSeedWithoutMembershipCount = in.Accounting.SuccessfulSeedWithMembershipCount, in.Accounting.SuccessfulSeedWithoutMembershipCount
	out.Accounting.Nodes = accountStrings(out.Partitions.Nodes, len(left.NativeNodeIDs), len(right.NativeNodeIDs))
	out.Accounting.CallRelations = accountStrings(out.Partitions.CallRelations, len(left.NativeCallRelationIDs), len(right.NativeCallRelationIDs))
	out.Accounting.DispatchRelationships = accountStrings(out.Partitions.DispatchRelationships, len(ld), len(rd))
	out.Accounting.SiblingCandidates = accountStrings(out.Partitions.SiblingCandidates, len(ls), len(rs))
	out.Accounting.DiagnosticCorrelations = accountIndexes(out.Partitions.DiagnosticCorrelations, len(left.CorrelatedDiagnosticIndexes), len(right.CorrelatedDiagnosticIndexes))
	out.ClaimCeiling.Supports = []string{"EXACT_REFERENCE_INTERSECTION", "EXACT_REFERENCE_DIFFERENCE"}
	out.ClaimCeiling.DoesNotSupport = []string{"SHARED_FEATURE_PURPOSE", "DISTINCT_FEATURE_PURPOSE", "FEATURE_IDENTITY", "WORKFLOW_IDENTITY", "MERGE_OR_SPLIT_DISPOSITION", "INDEPENDENT_OBSERVATION", "EVIDENTIARY_SUPPORT", "CONFIDENCE", "COVERAGE", "RUNTIME_BEHAVIOR", "ACCEPTANCE"}
	return out, nil
}
func seedSummary(s inspectionSeed) Seed {
	state := "SUCCESSFUL_WITH_EVIDENCE"
	if s.PreparationStatus == "FAILED" {
		state = "FAILED"
	} else {
		d, b := membershipRefs(s.SeedMemberships)
		if len(s.NativeNodeIDs)+len(s.NativeCallRelationIDs)+len(d)+len(b)+len(s.CorrelatedDiagnosticIndexes) == 0 {
			state = "SUCCESSFUL_EMPTY"
		}
	}
	return Seed{s.Seed.Label, state, s.Seed.Failure}
}
func membershipRefs(ms []graph.SeedMembership) (d, s []string) {
	for _, m := range ms {
		if m.EvidenceKind == "DISPATCH_ASSOCIATION" {
			d = append(d, m.EndpointID)
		}
		if m.EvidenceKind == "SIBLING_CANDIDATE" {
			s = append(s, m.EndpointID)
		}
	}
	return
}
func nodeGlobals(in inspection) []string {
	o := make([]string, len(in.Records.Nodes))
	for i, v := range in.Records.Nodes {
		o[i] = v.ID
	}
	return o
}
func callGlobals(in inspection) []string {
	o := make([]string, len(in.Records.CallRelations))
	for i, v := range in.Records.CallRelations {
		o[i] = v.RelationID
	}
	return o
}
func dispatchGlobals(in inspection) []string {
	o := make([]string, len(in.Records.DispatchRelationships))
	for i, v := range in.Records.DispatchRelationships {
		o[i] = v.RelationID
	}
	return o
}
func siblingGlobals(in inspection) []string {
	o := make([]string, len(in.Records.SiblingCandidates))
	for i, v := range in.Records.SiblingCandidates {
		o[i] = v.RelationID
	}
	return o
}
func partitionStrings(ns schema.ReferenceNamespace, g, l, r []string) StringPartition {
	gs := make([]schema.TypedReference, len(g))
	for i, v := range g {
		gs[i] = schema.TypedReference{Namespace: ns, Value: v, Ordinal: i}
	}
	lk := make([]schema.TypedReferenceKey, len(l))
	for i, v := range l {
		lk[i] = schema.TypedReferenceKey{Namespace: ns, Value: v}
	}
	rk := make([]schema.TypedReferenceKey, len(r))
	for i, v := range r {
		rk[i] = schema.TypedReferenceKey{Namespace: ns, Value: v}
	}
	p := schema.PartitionTypedReferences(gs, lk, rk)
	o := StringPartition{[]string{}, []string{}, []string{}}
	for _, v := range p.Shared {
		o.Shared = append(o.Shared, v.Value)
	}
	for _, v := range p.LeftOnly {
		o.LeftOnly = append(o.LeftOnly, v.Value)
	}
	for _, v := range p.RightOnly {
		o.RightOnly = append(o.RightOnly, v.Value)
	}
	return o
}
func partitionIndexes(total int, l, r []int) IndexPartition {
	lm, rm := map[int]bool{}, map[int]bool{}
	for _, v := range l {
		lm[v] = true
	}
	for _, v := range r {
		rm[v] = true
	}
	o := IndexPartition{[]int{}, []int{}, []int{}}
	for i := 0; i < total; i++ {
		if lm[i] && rm[i] {
			o.Shared = append(o.Shared, i)
		} else if lm[i] {
			o.LeftOnly = append(o.LeftOnly, i)
		} else if rm[i] {
			o.RightOnly = append(o.RightOnly, i)
		}
	}
	return o
}
func accountStrings(p StringPartition, l, r int) NamespaceAccounting {
	return NamespaceAccounting{l, r, len(p.Shared), len(p.LeftOnly), len(p.RightOnly), len(p.Shared) + len(p.LeftOnly) + len(p.RightOnly)}
}
func accountIndexes(p IndexPartition, l, r int) NamespaceAccounting {
	return NamespaceAccounting{l, r, len(p.Shared), len(p.LeftOnly), len(p.RightOnly), len(p.Shared) + len(p.LeftOnly) + len(p.RightOnly)}
}
