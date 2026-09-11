package programcpresentation

import (
	"testing"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/programctestfixture"
)

func exactFixture(t testing.TB) (programc.Outcome, programc.BoundaryArtifact) {
	t.Helper()
	o, failure := programc.Compute(programctestfixture.ValidV5(t), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	b, err := programc.ComputeBoundary(o, programc.BoundaryRequest{PageRankTopK: 2, HubTopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	return o, b
}

func TestBuildRejectsEveryBrokenPresentationJoinWithoutPartialArtifact(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*programc.Outcome, *programc.BoundaryArtifact)
	}{
		{"missing-node", func(o *programc.Outcome, _ *programc.BoundaryArtifact) {
			o.Projection.NodeIdentities = o.Projection.NodeIdentities[1:]
		}},
		{"duplicate-node", func(o *programc.Outcome, _ *programc.BoundaryArtifact) {
			o.Projection.NodeIdentities = append(o.Projection.NodeIdentities, o.Projection.NodeIdentities[0])
		}},
		{"foreign-node", func(o *programc.Outcome, _ *programc.BoundaryArtifact) {
			o.Projection.NodeIdentities[0] = "foreign-node"
		}},
		{"mismatched-node-index", func(o *programc.Outcome, _ *programc.BoundaryArtifact) {
			o.Projection.NodeIDs[o.Projection.NodeIdentities[0]] = 2
		}},
		{"missing-community", func(o *programc.Outcome, _ *programc.BoundaryArtifact) { o.Communities = o.Communities[1:] }},
		{"duplicate-community", func(o *programc.Outcome, _ *programc.BoundaryArtifact) {
			o.Communities = append(o.Communities, o.Communities[0])
		}},
		{"foreign-community-member", func(o *programc.Outcome, _ *programc.BoundaryArtifact) { o.Communities[0].Members[0] = "foreign-node" }},
		{"missing-boundary-community", func(_ *programc.Outcome, b *programc.BoundaryArtifact) { b.Communities = b.Communities[1:] }},
		{"duplicate-boundary-community-id", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			b.Communities = append(b.Communities, b.Communities[0])
		}},
		{"mismatched-boundary-members", func(_ *programc.Outcome, b *programc.BoundaryArtifact) { b.Communities[0].Members[0] = "foreign-node" }},
		{"foreign-score-node", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			b.HubCrossingNodes = append(b.HubCrossingNodes, programc.BoundaryNodeScore{NodeID: "foreign-node", Score: 1})
		}},
		{"duplicate-score-node", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			if len(b.HubCrossingNodes) > 0 {
				b.HubCrossingNodes = append(b.HubCrossingNodes, b.HubCrossingNodes[0])
			} else {
				b.HubCrossingNodes = []programc.BoundaryNodeScore{{NodeID: b.Communities[0].Members[0]}, {NodeID: b.Communities[0].Members[0]}}
			}
		}},
		{"foreign-articulation-node", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			b.ArticulationPoints = append(b.ArticulationPoints, "foreign-node")
		}},
		{"foreign-bridge-call", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			b.Bridges = append(b.Bridges, "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
		}},
		{"duplicate-bridge-call", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			if len(b.Bridges) == 0 {
				b.Bridges = []string{"x", "x"}
			} else {
				b.Bridges = append(b.Bridges, b.Bridges[0])
			}
		}},
		{"foreign-witness-call", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			if len(b.CrossingWitnesses) == 0 {
				b.CrossingWitnesses = []programc.CrossingWitness{{CommunityA: b.Communities[0].CommunityID, CommunityB: b.Communities[len(b.Communities)-1].CommunityID, OccurrenceID: "foreign-call", SourceNodeID: b.Communities[0].Members[0], TargetNodeID: b.Communities[len(b.Communities)-1].Members[0]}}
			} else {
				b.CrossingWitnesses[0].OccurrenceID = "foreign-call"
			}
		}},
		{"mismatched-witness-endpoint", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			if len(b.CrossingWitnesses) > 0 {
				b.CrossingWitnesses[0].SourceNodeID, b.CrossingWitnesses[0].TargetNodeID = b.CrossingWitnesses[0].TargetNodeID, b.CrossingWitnesses[0].SourceNodeID
			} else {
				b.CrossingWitnesses = []programc.CrossingWitness{{CommunityA: b.Communities[0].CommunityID, CommunityB: b.Communities[len(b.Communities)-1].CommunityID, OccurrenceID: "foreign-call", SourceNodeID: b.Communities[len(b.Communities)-1].Members[0], TargetNodeID: b.Communities[0].Members[0]}}
			}
		}},
		{"missing-call", func(o *programc.Outcome, _ *programc.BoundaryArtifact) {
			o.Projection.Occurrences = o.Projection.Occurrences[1:]
		}},
		{"duplicate-call", func(o *programc.Outcome, _ *programc.BoundaryArtifact) {
			o.Projection.Occurrences = append(o.Projection.Occurrences, o.Projection.Occurrences[0])
		}},
		{"foreign-call-node-index", func(o *programc.Outcome, _ *programc.BoundaryArtifact) {
			o.Projection.Occurrences[0].From = int64(len(o.Projection.NodeIdentities))
		}},
		{"mismatched-boundary-binding", func(_ *programc.Outcome, b *programc.BoundaryArtifact) {
			b.Bindings.SourceSHA256 = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		}},
		{"changed-claim-ceiling", func(_ *programc.Outcome, b *programc.BoundaryArtifact) { b.ClaimCeiling = "changed" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o, b := exactFixture(t)
			tc.mutate(&o, &b)
			deferred := true
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ASSERT_FAIL_CLOSED_NO_PANIC_%s: %v", tc.name, r)
				}
				if deferred {
					t.Fatalf("ASSERT_FAIL_CLOSED_JOIN_%s: mutation accepted", tc.name)
				}
			}()
			got, err := Build(o, b)
			if err == nil || got.SchemaVersion != "" {
				return
			}
			deferred = false
		})
	}
}
