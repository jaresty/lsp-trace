package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
)

const filterUsage = "usage: lsp-trace filter INSPECTION --compare-seeds LABEL --compare-seeds LABEL [--json]"

type filterStringPartition struct{ Shared, LeftOnly, RightOnly []string }

func (p filterStringPartition) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Shared []string `json:"shared"`
		Left   []string `json:"left_only"`
		Right  []string `json:"right_only"`
	}{p.Shared, p.LeftOnly, p.RightOnly})
}

type filterIndexPartition struct{ Shared, LeftOnly, RightOnly []int }

func (p filterIndexPartition) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Shared []int `json:"shared"`
		Left   []int `json:"left_only"`
		Right  []int `json:"right_only"`
	}{p.Shared, p.LeftOnly, p.RightOnly})
}

type filterNamespaceAccounting struct {
	LeftReferenceCount      int `json:"left_reference_count"`
	RightReferenceCount     int `json:"right_reference_count"`
	SharedReferenceCount    int `json:"shared_reference_count"`
	LeftOnlyReferenceCount  int `json:"left_only_reference_count"`
	RightOnlyReferenceCount int `json:"right_only_reference_count"`
	PairUniverseCount       int `json:"pair_universe_count"`
}

type filterSeed struct {
	Label   string             `json:"label"`
	State   string             `json:"state"`
	Failure *graph.SeedFailure `json:"failure"`
}

type filterProjection struct {
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
	Seeds      []filterSeed `json:"seeds"`
	Partitions struct {
		Nodes                  filterStringPartition `json:"nodes"`
		CallRelations          filterStringPartition `json:"call_relations"`
		DispatchRelationships  filterStringPartition `json:"dispatch_relationships"`
		SiblingCandidates      filterStringPartition `json:"sibling_candidates"`
		DiagnosticCorrelations filterIndexPartition  `json:"diagnostic_correlations"`
	} `json:"partitions"`
	GlobalBoundary struct {
		Truncated           bool   `json:"truncated"`
		TraversalComplete   bool   `json:"traversal_complete"`
		SourceGraphComplete string `json:"source_graph_complete"`
	} `json:"global_boundary"`
	Accounting struct {
		RequestedSeedCount                   int                       `json:"requested_seed_count"`
		SuccessfulSeedCount                  int                       `json:"successful_seed_count"`
		FailedSeedCount                      int                       `json:"failed_seed_count"`
		SuccessfulSeedWithMembershipCount    int                       `json:"successful_seed_with_membership_count"`
		SuccessfulSeedWithoutMembershipCount int                       `json:"successful_seed_without_membership_count"`
		Nodes                                filterNamespaceAccounting `json:"nodes"`
		CallRelations                        filterNamespaceAccounting `json:"call_relations"`
		DispatchRelationships                filterNamespaceAccounting `json:"dispatch_relationships"`
		SiblingCandidates                    filterNamespaceAccounting `json:"sibling_candidates"`
		DiagnosticCorrelations               filterNamespaceAccounting `json:"diagnostic_correlations"`
	} `json:"accounting"`
	ClaimCeiling struct {
		Supports       []string `json:"supports"`
		DoesNotSupport []string `json:"does_not_support"`
	} `json:"claim_ceiling"`
}

func runFilter(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(stdout, filterUsage)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, filterUsage)
		return 1
	}
	input := args[0]
	fs := flag.NewFlagSet("filter", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, filterUsage) }
	var labels stringsFlag
	fs.Var(&labels, "compare-seeds", "seed label to compare; repeat exactly twice")
	_ = fs.Bool("json", false, "emit JSON (currently the only format)")
	if err := fs.Parse(args[1:]); err != nil {
		fs.Usage()
		return 1
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return 1
	}
	if len(labels) != 2 {
		fmt.Fprintln(stderr, "filter: exactly two --compare-seeds values are required")
		return 1
	}
	if labels[0] == "" || labels[1] == "" {
		fmt.Fprintln(stderr, "filter: compared seed labels must be nonempty")
		return 1
	}
	if labels[0] == labels[1] {
		fmt.Fprintln(stderr, "filter: compared seed labels must be distinct")
		return 1
	}
	data, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	projection, err := projectPairwiseFilter(data, labels[0], labels[1])
	if err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	if _, err := schema.ValidateFor(encoded, schema.FamilyFilter, "v1"); err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	return 0
}

func projectPairwiseFilter(data []byte, leftLabel, rightLabel string) (filterProjection, error) {
	var header struct {
		InspectionSchemaVersion string `json:"inspection_schema_version"`
		ProjectionKind          string `json:"projection_kind"`
		Authority               string `json:"authority"`
		SchemaVersion           string `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil || header.InspectionSchemaVersion != schema.InspectionVersionV1 || header.ProjectionKind != "ALL_SEEDS" || header.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || header.SchemaVersion != "" {
		return filterProjection{}, fmt.Errorf("input must be %s ALL_SEEDS", schema.InspectionVersionV1)
	}
	if err := schema.ValidateAllSeedInspection(data); err != nil {
		return filterProjection{}, err
	}
	var in inspectAllProjection
	if err := json.Unmarshal(data, &in); err != nil {
		return filterProjection{}, err
	}
	var left, right *inspectAllSeed
	for i := range in.Seeds {
		if in.Seeds[i].Seed.Label == leftLabel {
			left = &in.Seeds[i]
		}
		if in.Seeds[i].Seed.Label == rightLabel {
			right = &in.Seeds[i]
		}
	}
	if left == nil {
		return filterProjection{}, fmt.Errorf("seed label %q not found", leftLabel)
	}
	if right == nil {
		return filterProjection{}, fmt.Errorf("seed label %q not found", rightLabel)
	}
	out := filterProjection{FilterSchemaVersion: "lsp-trace.filter.v1", ProjectionKind: "SEED_EVIDENCE_COMPARISON", Authority: "TOOL_DERIVED_SET_PROJECTION", NativeSemanticsPolicy: "PRESERVE_WITHOUT_AUTHORITY_UPGRADE"}
	out.InputIdentity.InspectionExactBytesDigest = graph.ExactBytesDigest(data)
	out.InputIdentity.ArtifactSemanticCommitmentDigest = in.ArtifactIdentity.SemanticCommitmentDigest
	out.InputIdentity.ArtifactExactSerializedBytesDigest = in.ArtifactIdentity.ExactSerializedBytesDigest
	out.InputIdentity.ExecutionBundleID = in.ArtifactIdentity.ExecutionBundleID
	out.Operands.LeftSeedLabel, out.Operands.RightSeedLabel = leftLabel, rightLabel
	out.Seeds = []filterSeed{filterSeedSummary(*left), filterSeedSummary(*right)}
	out.Partitions.Nodes = partitionStringRefs(schema.ReferenceNode, nodeGlobals(in), left.NativeNodeIDs, right.NativeNodeIDs)
	out.Partitions.CallRelations = partitionStringRefs(schema.ReferenceCallRelation, callGlobals(in), left.NativeCallRelationIDs, right.NativeCallRelationIDs)
	leftDispatch, leftSibling := membershipRefs(left.SeedMemberships)
	rightDispatch, rightSibling := membershipRefs(right.SeedMemberships)
	out.Partitions.DispatchRelationships = partitionStringRefs(schema.ReferenceDispatchRelationship, dispatchGlobals(in), leftDispatch, rightDispatch)
	out.Partitions.SiblingCandidates = partitionStringRefs(schema.ReferenceSiblingCandidate, siblingGlobals(in), leftSibling, rightSibling)
	out.Partitions.DiagnosticCorrelations = partitionDiagnosticRefs(len(in.Records.Diagnostics), left.CorrelatedDiagnosticIndexes, right.CorrelatedDiagnosticIndexes)
	out.GlobalBoundary.Truncated, out.GlobalBoundary.TraversalComplete, out.GlobalBoundary.SourceGraphComplete = in.Accounting.Truncated, in.Accounting.TraversalComplete, in.Accounting.SourceGraphComplete
	out.Accounting.RequestedSeedCount, out.Accounting.SuccessfulSeedCount, out.Accounting.FailedSeedCount = in.Accounting.RequestedSeedCount, in.Accounting.SuccessfulSeedCount, in.Accounting.FailedSeedCount
	out.Accounting.SuccessfulSeedWithMembershipCount, out.Accounting.SuccessfulSeedWithoutMembershipCount = in.Accounting.SuccessfulSeedWithMembershipCount, in.Accounting.SuccessfulSeedWithoutMembershipCount
	out.Accounting.Nodes = accountStrings(out.Partitions.Nodes, len(left.NativeNodeIDs), len(right.NativeNodeIDs))
	out.Accounting.CallRelations = accountStrings(out.Partitions.CallRelations, len(left.NativeCallRelationIDs), len(right.NativeCallRelationIDs))
	out.Accounting.DispatchRelationships = accountStrings(out.Partitions.DispatchRelationships, len(leftDispatch), len(rightDispatch))
	out.Accounting.SiblingCandidates = accountStrings(out.Partitions.SiblingCandidates, len(leftSibling), len(rightSibling))
	out.Accounting.DiagnosticCorrelations = accountIndexes(out.Partitions.DiagnosticCorrelations, len(left.CorrelatedDiagnosticIndexes), len(right.CorrelatedDiagnosticIndexes))
	out.ClaimCeiling.Supports = []string{"EXACT_REFERENCE_INTERSECTION", "EXACT_REFERENCE_DIFFERENCE"}
	out.ClaimCeiling.DoesNotSupport = []string{"SHARED_FEATURE_PURPOSE", "DISTINCT_FEATURE_PURPOSE", "FEATURE_IDENTITY", "WORKFLOW_IDENTITY", "MERGE_OR_SPLIT_DISPOSITION", "INDEPENDENT_OBSERVATION", "EVIDENTIARY_SUPPORT", "CONFIDENCE", "COVERAGE", "RUNTIME_BEHAVIOR", "ACCEPTANCE"}
	return out, nil
}

func filterSeedSummary(seed inspectAllSeed) filterSeed {
	state := "SUCCESSFUL_WITH_EVIDENCE"
	if seed.PreparationStatus == "FAILED" {
		state = "FAILED"
	} else {
		dispatch, sibling := membershipRefs(seed.SeedMemberships)
		if len(seed.NativeNodeIDs)+len(seed.NativeCallRelationIDs)+len(dispatch)+len(sibling)+len(seed.CorrelatedDiagnosticIndexes) == 0 {
			state = "SUCCESSFUL_EMPTY"
		}
	}
	return filterSeed{seed.Seed.Label, state, seed.Seed.Failure}
}
func nodeGlobals(in inspectAllProjection) []string {
	out := make([]string, len(in.Records.Nodes))
	for i, v := range in.Records.Nodes {
		out[i] = v.ID
	}
	return out
}
func callGlobals(in inspectAllProjection) []string {
	out := make([]string, len(in.Records.CallRelations))
	for i, v := range in.Records.CallRelations {
		out[i] = v.RelationID
	}
	return out
}
func dispatchGlobals(in inspectAllProjection) []string {
	out := make([]string, len(in.Records.DispatchRelationships))
	for i, v := range in.Records.DispatchRelationships {
		out[i] = v.RelationID
	}
	return out
}
func siblingGlobals(in inspectAllProjection) []string {
	out := make([]string, len(in.Records.SiblingCandidates))
	for i, v := range in.Records.SiblingCandidates {
		out[i] = v.RelationID
	}
	return out
}
func membershipRefs(ms []graph.SeedMembership) (dispatch, sibling []string) {
	for _, m := range ms {
		if m.EvidenceKind == "DISPATCH_ASSOCIATION" {
			dispatch = append(dispatch, m.EndpointID)
		}
		if m.EvidenceKind == "SIBLING_CANDIDATE" {
			sibling = append(sibling, m.EndpointID)
		}
	}
	return
}
func partitionStringRefs(ns schema.ReferenceNamespace, global, left, right []string) filterStringPartition {
	g := make([]schema.TypedReference, len(global))
	for i, v := range global {
		g[i] = schema.TypedReference{Namespace: ns, Value: v, Ordinal: i}
	}
	lk := make([]schema.TypedReferenceKey, len(left))
	for i, v := range left {
		lk[i] = schema.TypedReferenceKey{Namespace: ns, Value: v}
	}
	rk := make([]schema.TypedReferenceKey, len(right))
	for i, v := range right {
		rk[i] = schema.TypedReferenceKey{Namespace: ns, Value: v}
	}
	p := schema.PartitionTypedReferences(g, lk, rk)
	out := filterStringPartition{[]string{}, []string{}, []string{}}
	for _, v := range p.Shared {
		out.Shared = append(out.Shared, v.Value)
	}
	for _, v := range p.LeftOnly {
		out.LeftOnly = append(out.LeftOnly, v.Value)
	}
	for _, v := range p.RightOnly {
		out.RightOnly = append(out.RightOnly, v.Value)
	}
	return out
}
func partitionDiagnosticRefs(total int, left, right []int) filterIndexPartition {
	l, r := map[int]bool{}, map[int]bool{}
	for _, v := range left {
		l[v] = true
	}
	for _, v := range right {
		r[v] = true
	}
	out := filterIndexPartition{[]int{}, []int{}, []int{}}
	for i := 0; i < total; i++ {
		if l[i] && r[i] {
			out.Shared = append(out.Shared, i)
		} else if l[i] {
			out.LeftOnly = append(out.LeftOnly, i)
		} else if r[i] {
			out.RightOnly = append(out.RightOnly, i)
		}
	}
	return out
}
func accountStrings(p filterStringPartition, l, r int) filterNamespaceAccounting {
	return filterNamespaceAccounting{l, r, len(p.Shared), len(p.LeftOnly), len(p.RightOnly), len(p.Shared) + len(p.LeftOnly) + len(p.RightOnly)}
}
func accountIndexes(p filterIndexPartition, l, r int) filterNamespaceAccounting {
	return filterNamespaceAccounting{l, r, len(p.Shared), len(p.LeftOnly), len(p.RightOnly), len(p.Shared) + len(p.LeftOnly) + len(p.RightOnly)}
}
