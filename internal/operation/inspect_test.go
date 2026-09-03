package operation

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/inspection"
)

const (
	assertInspectSeed      = "P_INSPECT_EXPLICIT_SEED_CORE_PARITY"
	assertInspectAll       = "P_INSPECT_ALL_SEEDS_CORE_PARITY"
	assertInspectBytes     = "P_INSPECT_DETERMINISTIC_EXACT_BYTES"
	assertInspectError     = "P_INSPECT_CORE_ERROR_UNCHANGED"
	assertInspectAuthority = "P_INSPECT_AUTHORITY_UNCHANGED"
)

func inspectionGraphBytes(t *testing.T) []byte {
	t.Helper()
	bundle := inspection.Bundle{
		SchemaVersion: "lsp-trace.graph.v3",
		Invocation:    graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "chosen"}, {Label: "other"}}},
		Nodes:         []graph.Node{}, Edges: []graph.Edge{}, DispatchRelationships: []graph.DispatchRelationship{},
		SiblingCandidates: []graph.SiblingCandidate{}, Terminals: []graph.Boundary{}, Frontier: []graph.Boundary{},
		Diagnostics: []graph.Diagnostic{}, SeedMemberships: []graph.SeedMembership{},
		Seeds:   []graph.SeedResult{{Label: "chosen", PreparedTargetIDs: []string{}, ReachedNodeIDs: []string{}, ReachedRelationIDs: []string{}}, {Label: "other", PreparedTargetIDs: []string{}, ReachedNodeIDs: []string{}, ReachedRelationIDs: []string{}}},
		Summary: inspection.Summary{TraversalComplete: true, SourceGraphComplete: "COMPLETE", CompletenessScope: "BOUNDED_TRAVERSAL"},
	}
	bundle.TraceReceipt.SemanticCommitmentDigest = "sha256:semantic"
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func inspectRequest(t *testing.T, input []byte, selector any) Request {
	t.Helper()
	data, err := json.Marshal(map[string]any{"input": string(input), "selector": selector})
	if err != nil {
		t.Fatal(err)
	}
	return Request{Name: Inspect, Input: data}
}

func TestInspectHandlerPreservesSeedAndAllSeedCoreResults(t *testing.T) {
	data := inspectionGraphBytes(t)
	handler := NewInspectHandler()
	cases := []struct {
		name, assertion string
		selector        any
		want            any
	}{
		{"seed", assertInspectSeed, map[string]any{"seed": "chosen"}, func() any { v, _ := inspection.ProjectSeed(data, "chosen"); return v }()},
		{"all", assertInspectAll, map[string]any{"all_seeds": true}, func() any { v, _ := inspection.ProjectAllSeeds(data); return v }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, failure := handler(context.Background(), inspectRequest(t, data, tc.selector))
			if failure != nil || !reflect.DeepEqual(got.Value, tc.want) {
				t.Fatalf("%s: result=%#v failure=%v", tc.assertion, got, failure)
			}
			wantArtifact, err := json.Marshal(tc.want)
			if err != nil {
				t.Fatal(err)
			}
			wantArtifact = append(wantArtifact, '\n')
			if !reflect.DeepEqual(got.Artifact, wantArtifact) {
				t.Fatalf("%s: artifact=%q want exact CLI bytes=%q", assertInspectBytes, got.Artifact, wantArtifact)
			}
		})
	}
}

func TestInspectHandlerPreservesErrorAndAuthority(t *testing.T) {
	handler := NewInspectHandler()
	data := inspectionGraphBytes(t)
	got, failure := handler(context.Background(), inspectRequest(t, data, map[string]any{"seed": "missing"}))
	if failure == nil || failure.Err == nil || failure.Err.Error() != `seed label "missing" not found` {
		t.Fatalf("%s: result=%#v failure=%#v", assertInspectError, got, failure)
	}
	got, failure = handler(context.Background(), inspectRequest(t, data, map[string]any{"seed": "chosen"}))
	if failure != nil {
		t.Fatalf("%s: %v", assertInspectAuthority, failure)
	}
	projection := got.Value.(inspection.Projection)
	if projection.Authority != "NON_AUTHORITATIVE_DERIVED_VIEW" || projection.DiagnosticsOnReachedNodes.Authority != "TOOL_DERIVED_NODE_CORRELATION" {
		t.Fatalf("%s: authority=%q diagnostics_authority=%q", assertInspectAuthority, projection.Authority, projection.DiagnosticsOnReachedNodes.Authority)
	}
}

func TestInspectHandlerDeterministicConcurrentBytes(t *testing.T) {
	handler := NewInspectHandler()
	request := inspectRequest(t, inspectionGraphBytes(t), map[string]any{"all_seeds": true})
	const count = 16
	artifacts := make([][]byte, count)
	failures := make([]*Failure, count)
	var wg sync.WaitGroup
	for i := range artifacts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, failure := handler(context.Background(), request)
			artifacts[i], failures[i] = result.Artifact, failure
		}(i)
	}
	wg.Wait()
	for i := 1; i < count; i++ {
		if failures[i] != nil || !reflect.DeepEqual(artifacts[0], artifacts[i]) {
			t.Fatalf("%s: run %d bytes=%q failure=%v; first=%q", assertInspectBytes, i, artifacts[i], failures[i], artifacts[0])
		}
	}
}
