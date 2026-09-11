package gonumleiden

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	FixtureInventorySchema = "lsp-trace.private.program-c.a-06-fixture-inventory.v1"
	FixtureGraphSchema     = "lsp-trace.private.program-c.graph-fixture.v1"
	ProjectionDigest       = "sha256:f4fb309c6e849b5a8e6057f355b1db6ee3c53414c3c430b67be17f68fe9e97f9"
)

var requiredFixtureClasses = []string{"adversarial-input-order", "directed", "disconnected", "high-degree-hub", "singleton", "weighted"}

type CandidateSpec struct {
	Name     string `json:"name"`
	Module   string `json:"module"`
	Revision string `json:"revision,omitempty"`
	Version  string `json:"version"`
}

type Provenance struct {
	Kind         string `json:"kind"`
	SourcePath   string `json:"source_path"`
	SourceSHA256 string `json:"source_sha256"`
	Derivation   string `json:"derivation"`
}

type FileBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  string `json:"bytes"`
}

type ProjectionBinding struct {
	Name          string `json:"name"`
	LogicalDigest string `json:"logical_digest"`
	Manifest      string `json:"manifest"`
}

type FixtureRecord struct {
	ID                        string            `json:"id"`
	Provenance                Provenance        `json:"provenance"`
	Graph                     FileBinding       `json:"graph"`
	Projection                ProjectionBinding `json:"projection"`
	NodeCount                 int               `json:"node_count"`
	DirectedWeightedEdgeCount int               `json:"directed_weighted_edge_count"`
	Directed                  bool              `json:"directed"`
	Weighted                  bool              `json:"weighted"`
	WeightSemantics           string            `json:"weight_semantics"`
	CoveredClasses            []string          `json:"covered_classes"`
}

type Schedule struct {
	BaseRepeats  int    `json:"base_repeats_per_candidate_fixture_seed"`
	Permutations int    `json:"deterministic_input_permutations_per_candidate_fixture_seed"`
	Permutation  string `json:"permutation"`
}

type InventoryLimits struct {
	WallSeconds int   `json:"wall_seconds_per_run"`
	RSSBytes    int64 `json:"aggregate_process_tree_rss_bytes_per_run"`
	MaxNodes    int   `json:"maximum_nodes_per_fixture"`
	MaxEdges    int   `json:"maximum_directed_weighted_edges_per_fixture"`
}

type FixtureInventory struct {
	SchemaVersion   string          `json:"schema_version"`
	InventoryID     string          `json:"inventory_id"`
	CandidateScope  []CandidateSpec `json:"candidate_scope"`
	Seeds           []uint64        `json:"seed_inventory"`
	Schedule        Schedule        `json:"schedule"`
	Limits          InventoryLimits `json:"limits"`
	RequiredClasses []string        `json:"required_classes"`
	Fixtures        []FixtureRecord `json:"fixtures"`
	ClaimCeiling    string          `json:"claim_ceiling"`
}

type QualifiedFixture struct {
	Record  FixtureRecord `json:"record"`
	Request Fixture       `json:"-"`
}

type graphEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight"`
}

type graphFixture struct {
	SchemaVersion string      `json:"schema_version"`
	Directed      bool        `json:"directed"`
	Weighted      bool        `json:"weighted"`
	Nodes         []string    `json:"nodes"`
	Edges         []graphEdge `json:"edges"`
}

func safeRead(root, relative string) ([]byte, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return nil, Fail("DERIVATION_REJECTED", "invalid repository path")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, Fail("DERIVATION_REJECTED", "invalid repository path")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, Fail("DERIVATION_REJECTED", err.Error())
	}
	rootAbs, err = filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, Fail("DERIVATION_REJECTED", err.Error())
	}
	path := filepath.Join(rootAbs, clean)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, Fail("DERIVATION_REJECTED", err.Error())
	}
	if resolved != rootAbs && !strings.HasPrefix(resolved, rootAbs+string(filepath.Separator)) {
		return nil, Fail("DERIVATION_REJECTED", "repository path escape")
	}
	st, err := os.Stat(resolved)
	if err != nil || !st.Mode().IsRegular() {
		return nil, Fail("DERIVATION_REJECTED", "repository path is not a regular file")
	}
	b, err := os.ReadFile(resolved)
	if err != nil {
		return nil, Fail("DERIVATION_REJECTED", err.Error())
	}
	return b, nil
}

func parseGraphFixture(f Fixture) (graphFixture, error) {
	var graph graphFixture
	if Digest([]byte(f.GraphBytes)) != f.GraphSHA256 {
		return graph, Fail("INPUT_DIGEST_MISMATCH", f.ID)
	}
	if err := json.Unmarshal([]byte(f.GraphBytes), &graph); err != nil {
		return graph, Fail("DERIVATION_REJECTED", err.Error())
	}
	if graph.SchemaVersion != FixtureGraphSchema || !graph.Directed || !graph.Weighted {
		return graph, Fail("DERIVATION_REJECTED", "directed weighted graph semantics")
	}
	if len(graph.Nodes) != f.NodeCount || len(graph.Edges) != f.EdgeCount || len(graph.Nodes) == 0 {
		return graph, Fail("DERIVATION_REJECTED", "graph count metadata")
	}
	if len(graph.Nodes) > MaxNodes || len(graph.Edges) > MaxEdges {
		return graph, Fail("CAP_BREACH", "fixture cardinality")
	}
	nodes := make(map[string]bool, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node == "" || nodes[node] {
			return graph, Fail("DERIVATION_REJECTED", "node identity")
		}
		nodes[node] = true
	}
	seen := make(map[[2]string]bool, len(graph.Edges))
	for _, edge := range graph.Edges {
		key := [2]string{edge.From, edge.To}
		if !nodes[edge.From] || !nodes[edge.To] || edge.Weight <= 0 || seen[key] {
			return graph, Fail("DERIVATION_REJECTED", "invalid or duplicate directed weighted edge")
		}
		seen[key] = true
	}
	return graph, nil
}

func LoadFixtureInventory(root, relative string) (FixtureInventory, []QualifiedFixture, error) {
	var inventory FixtureInventory
	raw, err := safeRead(root, relative)
	if err != nil {
		return inventory, nil, err
	}
	if err := json.Unmarshal(raw, &inventory); err != nil {
		return inventory, nil, Fail("DERIVATION_REJECTED", err.Error())
	}
	expectedCandidates := []CandidateSpec{
		{Name: "Gonum Louvain", Module: "gonum.org/v1/gonum", Version: "v0.17.0"},
		{Name: "Gonum Leiden", Module: "gonum.org/v1/gonum", Revision: "69ca49f456a7a38cf370131834a2178d9aae17fe", Version: "v0.17.1-0.20260426204603-69ca49f456a7"},
	}
	gotCandidates, _ := json.Marshal(inventory.CandidateScope)
	wantCandidates, _ := json.Marshal(expectedCandidates)
	if inventory.SchemaVersion != FixtureInventorySchema || inventory.InventoryID == "" || string(gotCandidates) != string(wantCandidates) {
		return inventory, nil, Fail("DERIVATION_REJECTED", "inventory identity or candidate scope")
	}
	if len(inventory.Seeds) != 2 || inventory.Seeds[0] != 1 || inventory.Seeds[1] != 2 || inventory.Schedule.BaseRepeats != Runs || inventory.Schedule.Permutations != Permutations {
		return inventory, nil, Fail("SCHEDULE_INVALID", "seed or repeat inventory")
	}
	if inventory.Schedule.Permutation != "reverse exact node insertion order and exact directed-edge insertion order" {
		return inventory, nil, Fail("SCHEDULE_INVALID", "input permutation")
	}
	if inventory.Limits != (InventoryLimits{WallSeconds: 60, RSSBytes: 512 << 20, MaxNodes: MaxNodes, MaxEdges: MaxEdges}) {
		return inventory, nil, Fail("CAP_BREACH", "inventory limits")
	}
	classes := append([]string(nil), inventory.RequiredClasses...)
	sort.Strings(classes)
	if fmt.Sprint(classes) != fmt.Sprint(requiredFixtureClasses) || len(inventory.Fixtures) == 0 || len(inventory.Fixtures) > 6 {
		return inventory, nil, Fail("DERIVATION_REJECTED", "fixture class inventory")
	}
	covered := make(map[string]bool)
	ids := make(map[string]bool)
	qualified := make([]QualifiedFixture, 0, len(inventory.Fixtures))
	for _, record := range inventory.Fixtures {
		if record.ID == "" || ids[record.ID] || record.Provenance.Kind == "" || record.Provenance.Derivation == "" || record.Graph.Bytes != "exact UTF-8 file bytes including final LF; no normalization before hashing" || record.Projection != (ProjectionBinding{Name: ProfileName, LogicalDigest: ProjectionDigest, Manifest: "qualification/program-c/projection-profiles.v1.json"}) || !record.Directed || !record.Weighted || record.WeightSemantics != "positive integer admitted CALL occurrence count" {
			return inventory, nil, Fail("DERIVATION_REJECTED", "fixture metadata")
		}
		ids[record.ID] = true
		source, err := safeRead(root, record.Provenance.SourcePath)
		if err != nil || Digest(source) != record.Provenance.SourceSHA256 {
			return inventory, nil, Fail("INPUT_DIGEST_MISMATCH", record.ID+" provenance")
		}
		graphBytes, err := safeRead(root, record.Graph.Path)
		if err != nil || Digest(graphBytes) != record.Graph.SHA256 {
			return inventory, nil, Fail("INPUT_DIGEST_MISMATCH", record.ID+" graph")
		}
		fixture := Fixture{ID: record.ID, GraphSHA256: record.Graph.SHA256, GraphBytes: string(graphBytes), NodeCount: record.NodeCount, EdgeCount: record.DirectedWeightedEdgeCount}
		if _, err := parseGraphFixture(fixture); err != nil {
			return inventory, nil, err
		}
		for _, class := range record.CoveredClasses {
			covered[class] = true
		}
		qualified = append(qualified, QualifiedFixture{Record: record, Request: fixture})
	}
	for _, class := range requiredFixtureClasses {
		if !covered[class] {
			return inventory, nil, Fail("DERIVATION_REJECTED", "missing fixture class "+class)
		}
	}
	return inventory, qualified, nil
}
