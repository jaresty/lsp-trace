package programccompose

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/graph"
)

func TestConstituentRetainsLocalCanonicalSeedMemberships(t *testing.T) {
	a, b := node("a"), node("b")
	x := capture(t, "x", []graph.Node{a, b}, []graph.Edge{edge(a, b, 1)}, true, false)
	y := capture(t, "y", []graph.Node{b}, nil, true, false)
	one, err := Compose([]Input{x, y})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Compose([]Input{y, x})
	if err != nil {
		t.Fatal(err)
	}
	if string(one.Bytes) != string(two.Bytes) {
		t.Fatal("ASSERT_SEED_MEMBERSHIP_REPLAY_DETERMINISTIC")
	}
	if len(one.Artifact.Constituents) != 2 {
		t.Fatal("ASSERT_TWO_CONSTITUENTS")
	}
	var first, second []graph.SeedMembership
	if err := json.Unmarshal(one.Artifact.Constituents[0].SeedMemberships, &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(one.Artifact.Constituents[1].SeedMemberships, &second); err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(second) == 0 || string(one.Artifact.Constituents[0].SeedMemberships) == string(one.Artifact.Constituents[1].SeedMemberships) {
		t.Fatal("ASSERT_CONSTITUENT_LOCAL_MEMBERSHIPS_NOT_UNIONED")
	}
	kinds := map[string]bool{}
	for _, membership := range first {
		kinds[membership.EvidenceKind] = membership.EndpointID != ""
	}
	for _, kind := range []string{"PREPARED_TARGET", "REACHED_NODE", "CALL_RELATION"} {
		if !kinds[kind] {
			t.Fatalf("ASSERT_ENDPOINT_IDENTITY_RETAINED kind=%s", kind)
		}
	}

	for name, alter := range map[string]func([]graph.SeedMembership) []graph.SeedMembership{
		"removal": func(in []graph.SeedMembership) []graph.SeedMembership { return in[:len(in)-1] },
		"reordering": func(in []graph.SeedMembership) []graph.SeedMembership {
			out := append([]graph.SeedMembership(nil), in...)
			out[0], out[1] = out[1], out[0]
			return out
		},
		"mutation": func(in []graph.SeedMembership) []graph.SeedMembership {
			out := append([]graph.SeedMembership(nil), in...)
			out[0].EndpointID = "other-endpoint"
			return out
		},
	} {
		t.Run(name, func(t *testing.T) {
			mutated := one.Artifact
			mutated.Constituents = append([]Constituent(nil), one.Artifact.Constituents...)
			memberships := alter(append([]graph.SeedMembership(nil), first...))
			mutated.Constituents[0].SeedMemberships, _ = json.Marshal(memberships)
			mutated.CompositeID, mutated.OutputSHA256 = "", ""
			pre, _ := json.Marshal(mutated)
			mutated.CompositeID = digest("lsp-trace:program-c-compose:identity:v1", pre)
			pre, _ = json.Marshal(mutated)
			mutated.OutputSHA256 = digest("lsp-trace:program-c-compose:output:v1", pre)
			raw, _ := json.Marshal(mutated)
			if _, err := Validate(append(raw, '\n')); err == nil {
				t.Fatal("ASSERT_SEED_MEMBERSHIP_MUTATION_REPLAY_REJECTED")
			}
		})
	}
}
