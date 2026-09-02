package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
)

func inspectFixture(t *testing.T) ([]byte, string) {
	t.Helper()
	caller := graph.NewNode(graph.Item{Name: "caller", URI: "file:///w/caller.go"})
	callee := graph.NewNode(graph.Item{Name: "callee", URI: "file:///w/callee.go"})
	edges := graph.MergeEdge(nil, graph.Edge{CallerNodeID: caller.ID, CalleeNodeID: callee.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}}})
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV3,
		Invocation:    graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "chosen", At: "callee.go:1:1"}, {Label: "other", At: "other.go:1:1"}}},
		Nodes:         []graph.Node{caller, callee},
		Edges:         edges,
		Diagnostics:   []graph.Diagnostic{{Phase: "traverse", NodeID: callee.ID, Message: "on reached node"}, {Phase: "server", Message: "global only"}},
		Seeds: []graph.SeedResult{
			{Label: "chosen", Requested: graph.Target{URI: callee.URI, Line: 1, Column: 1}, PreparedTargetIDs: []string{callee.ID}, ReachedNodeIDs: []string{caller.ID, callee.ID}, ReachedRelationIDs: []string{edges[0].RelationID}, ReachedEdges: edges},
			{Label: "other", Requested: graph.Target{URI: "file:///w/other.go", Line: 1, Column: 1}, Failure: &graph.SeedFailure{Phase: "prepare", Message: "no item"}},
		},
		Summary: graph.Summary{Complete: true},
	}
	data, err := marshalResult(result, false)
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(artifact, data, 0600); err != nil {
		t.Fatal(err)
	}
	selector := filepath.Join(t.TempDir(), "selector.json")
	if err := publishBundle(selector, data); err != nil {
		t.Fatal(err)
	}
	return data, artifact + "\x00" + selector
}

func TestInspectSeedProjectsSameReadOnlyEvidenceFromArtifactAndSelector(t *testing.T) {
	original, paths := inspectFixture(t)
	artifact, selector, _ := strings.Cut(paths, "\x00")
	var outputs []string
	for _, input := range []string{artifact, selector} {
		stdout, stderr, code := captureRun(t, []string{"inspect", input, "--seed", "chosen", "--json"})
		if code != 0 || stderr != "" {
			t.Fatalf("ASSERT_INSPECT_ACCEPTS_ARTIFACT_OR_SELECTOR: input=%s code=%d stderr=%q", input, code, stderr)
		}
		var got struct {
			ProjectionKind   string `json:"projection_kind"`
			Authority        string `json:"authority"`
			ArtifactIdentity struct {
				SemanticCommitmentDigest   string `json:"semantic_commitment_digest"`
				ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
			} `json:"artifact_identity"`
			PreparationStatus string                 `json:"preparation_status"`
			Seed              graph.SeedResult       `json:"seed"`
			SeedMemberships   []graph.SeedMembership `json:"seed_memberships"`
			Nodes             []graph.Node           `json:"nodes"`
			Relations         []graph.Edge           `json:"relations"`
			Global            struct {
				Summary     map[string]any     `json:"summary"`
				Diagnostics []graph.Diagnostic `json:"diagnostics"`
			} `json:"global"`
			DiagnosticsOnReachedNodes struct {
				Authority   string             `json:"authority"`
				Diagnostics []graph.Diagnostic `json:"diagnostics"`
			} `json:"diagnostics_on_reached_nodes"`
		}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("ASSERT_INSPECT_JSON: %v stdout=%q", err, stdout)
		}
		if got.ProjectionKind != "SEED_INSPECTION" || got.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || got.PreparationStatus != "SUCCEEDED" || got.Seed.Label != "chosen" || strings.Contains(stdout, `"schema_version"`) || len(got.SeedMemberships) != 4 || len(got.Nodes) != 2 || len(got.Relations) != 1 || len(got.Global.Diagnostics) != 2 || len(got.DiagnosticsOnReachedNodes.Diagnostics) != 1 || got.DiagnosticsOnReachedNodes.Authority != "TOOL_DERIVED_NODE_CORRELATION" || got.ArtifactIdentity.SemanticCommitmentDigest == "" || got.ArtifactIdentity.ExactSerializedBytesDigest != graph.ExactBytesDigest(original) {
			t.Fatalf("ASSERT_INSPECT_EXACT_NATIVE_PROJECTION: %#v", got)
		}
		outputs = append(outputs, stdout)
	}
	if outputs[0] != outputs[1] {
		t.Fatalf("ASSERT_INSPECT_SELECTOR_ARTIFACT_EQUIVALENCE: artifact=%s selector=%s", outputs[0], outputs[1])
	}
	after, err := os.ReadFile(artifact)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("ASSERT_INSPECT_READ_ONLY: err=%v equal=%v", err, bytes.Equal(after, original))
	}
}

func TestInspectProjectsFailedSeedWithoutFabricatingMembership(t *testing.T) {
	_, paths := inspectFixture(t)
	artifact, _, _ := strings.Cut(paths, "\x00")
	stdout, stderr, code := captureRun(t, []string{"inspect", artifact, "--seed", "other"})
	var got struct {
		ProjectionKind    string           `json:"projection_kind"`
		Authority         string           `json:"authority"`
		PreparationStatus string           `json:"preparation_status"`
		Seed              graph.SeedResult `json:"seed"`
		Nodes             []graph.Node     `json:"nodes"`
		Relations         []graph.Edge     `json:"relations"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("ASSERT_INSPECT_FAILED_SEED_JSON: %v stdout=%q", err, stdout)
	}
	if code != 0 || stderr != "" || got.ProjectionKind != "SEED_INSPECTION" || got.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || got.PreparationStatus != "FAILED" || got.Seed.Failure == nil || got.Seed.Failure.Phase != "prepare" || got.Seed.Failure.Message != "no item" || len(got.Nodes) != 0 || len(got.Relations) != 0 {
		t.Fatalf("ASSERT_INSPECT_FAILED_SEED_STATE_WITHOUT_MEMBERSHIP: code=%d stderr=%q projection=%#v", code, stderr, got)
	}
}

func TestInspectRejectsUnknownSeedAndTamperedSelector(t *testing.T) {
	_, paths := inspectFixture(t)
	artifact, selector, _ := strings.Cut(paths, "\x00")
	stdout, stderr, code := captureRun(t, []string{"inspect", artifact, "--seed", "missing"})
	if code != 1 || stdout != "" || !strings.Contains(stderr, `inspect: seed label "missing" not found`) {
		t.Fatalf("ASSERT_INSPECT_UNKNOWN_SEED_FAILS_CLOSED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	selected, err := selectedGenerationFile(selector, generationArtifactName)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(selected, append(data, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = captureRun(t, []string{"inspect", selector, "--seed", "chosen"})
	if code != 1 || stdout != "" || !strings.Contains(stderr, "exact-byte integrity mismatch") {
		t.Fatalf("ASSERT_INSPECT_SELECTOR_CUSTODY_FAILS_CLOSED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
