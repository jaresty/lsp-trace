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

type inspectSummary struct {
	NodeCount           int    `json:"node_count"`
	EdgeCount           int    `json:"edge_count"`
	TerminalCount       int    `json:"terminal_count"`
	CycleCount          int    `json:"cycle_count"`
	TraversalComplete   bool   `json:"traversal_complete"`
	SourceGraphComplete string `json:"source_graph_complete"`
	CompletenessScope   string `json:"completeness_scope"`
	Truncated           bool   `json:"truncated"`
}

type inspectBundle struct {
	SchemaVersion     string                 `json:"schema_version"`
	ExecutionBundleID string                 `json:"execution_bundle_id,omitempty"`
	Nodes             []graph.Node           `json:"nodes"`
	Edges             []graph.Edge           `json:"edges"`
	Terminals         []graph.Boundary       `json:"terminals"`
	Frontier          []graph.Boundary       `json:"frontier"`
	Diagnostics       []graph.Diagnostic     `json:"diagnostics"`
	Seeds             []graph.SeedResult     `json:"seeds"`
	SeedMemberships   []graph.SeedMembership `json:"seed_memberships"`
	Summary           inspectSummary         `json:"summary"`
	TraceReceipt      struct {
		SemanticCommitmentDigest string `json:"semantic_commitment_digest"`
	} `json:"trace_receipt"`
}

type inspectProjection struct {
	ProjectionKind   string `json:"projection_kind"`
	Authority        string `json:"authority"`
	ArtifactIdentity struct {
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
		Summary     inspectSummary     `json:"summary"`
		Terminals   []graph.Boundary   `json:"terminals"`
		Frontier    []graph.Boundary   `json:"frontier"`
		Diagnostics []graph.Diagnostic `json:"diagnostics"`
	} `json:"global"`
	DiagnosticsOnReachedNodes struct {
		Authority   string             `json:"authority"`
		Diagnostics []graph.Diagnostic `json:"diagnostics"`
	} `json:"diagnostics_on_reached_nodes"`
}

func runInspect(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL [--json]")
		return 1
	}
	input := args[0]
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	seedLabel := fs.String("seed", "", "existing seed label")
	_ = fs.Bool("json", false, "emit JSON (currently the only format)")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if *seedLabel == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: lsp-trace inspect SELECTOR_OR_ARTIFACT --seed LABEL [--json]")
		return 1
	}
	data, err := loadInspectArtifact(input)
	if err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	projection, err := projectSeedInspection(data, *seedLabel)
	if err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	encoded = append(encoded, '\n')
	if _, err := stdout.Write(encoded); err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	return 0
}

func loadInspectArtifact(path string) ([]byte, error) {
	input, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var header struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(input, &header); err != nil {
		return nil, fmt.Errorf("malformed input: %w", err)
	}
	if header.SchemaVersion != "" {
		if header.SchemaVersion != graph.SchemaVersionV3 {
			return nil, fmt.Errorf("inspection requires %s", graph.SchemaVersionV3)
		}
		if _, err := schema.Validate(input, "v3"); err != nil {
			return nil, err
		}
		return input, nil
	}

	artifact, _, err := loadCustodiedGeneration(path)
	if err != nil {
		return nil, err
	}
	if _, err := schema.Validate(artifact, "v3"); err != nil {
		return nil, err
	}
	return artifact, nil
}

func projectSeedInspection(data []byte, label string) (inspectProjection, error) {
	var bundle inspectBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return inspectProjection{}, err
	}
	var selected *graph.SeedResult
	for i := range bundle.Seeds {
		if bundle.Seeds[i].Label == label {
			selected = &bundle.Seeds[i]
			break
		}
	}
	if selected == nil {
		return inspectProjection{}, fmt.Errorf("seed label %q not found", label)
	}

	out := inspectProjection{ProjectionKind: "SEED_INSPECTION", Authority: "NON_AUTHORITATIVE_DERIVED_VIEW", Seed: *selected}
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
	out.Global.Frontier = bundle.Frontier
	out.Global.Diagnostics = bundle.Diagnostics
	out.DiagnosticsOnReachedNodes.Authority = "TOOL_DERIVED_NODE_CORRELATION"
	out.DiagnosticsOnReachedNodes.Diagnostics = make([]graph.Diagnostic, 0)
	for _, diagnostic := range bundle.Diagnostics {
		if diagnostic.NodeID != "" && reachedNodes[diagnostic.NodeID] {
			out.DiagnosticsOnReachedNodes.Diagnostics = append(out.DiagnosticsOnReachedNodes.Diagnostics, diagnostic)
		}
	}
	return out, nil
}
