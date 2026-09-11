package programcpresentation

import (
	"encoding/json"
	"testing"
)

func TestStrictSchemaAndSemanticMutationCorpus(t *testing.T) {
	o, b := exactFixture(t)
	a, err := Build(o, b)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := JSON(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(valid); err != nil {
		t.Fatalf("ASSERT_VALID_PRESENTATION_ADMITTED: %v", err)
	}
	mutations := []struct {
		name string
		edit func(map[string]any)
	}{
		{"unknown-root-field", func(v map[string]any) { v["unknown"] = true }},
		{"unknown-nested-field", func(v map[string]any) { v["policy"].(map[string]any)["unknown"] = true }},
		{"foreign-node-fk", func(v map[string]any) {
			v["cross_community_calls"].([]any)[0].(map[string]any)["caller"].(map[string]any)["node_id"] = "foreign-node"
		}},
		{"community-order", func(v map[string]any) { x := v["communities"].([]any); x[0], x[1] = x[1], x[0] }},
		{"coordinate-convention", func(v map[string]any) {
			v["communities"].([]any)[0].(map[string]any)["members"].([]any)[0].(map[string]any)["location"].(map[string]any)["start_line"] = float64(0)
		}},
		{"duplicate-node-id", func(v map[string]any) {
			members := v["communities"].([]any)[0].(map[string]any)["members"].([]any)
			v["communities"].([]any)[0].(map[string]any)["members"] = append(members, members[0])
		}},
		{"changed-profile-identity", func(v map[string]any) { v["profile_id"] = "changed" }},
		{"changed-community-identity", func(v map[string]any) {
			v["communities"].([]any)[0].(map[string]any)["community_id"] = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		}},
		{"changed-claim-ceiling", func(v map[string]any) { v["claim_ceiling"] = "changed" }},
		{"changed-policy-identity", func(v map[string]any) { v["policy"].(map[string]any)["id"] = "changed" }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			var v map[string]any
			if err := json.Unmarshal(valid, &v); err != nil {
				t.Fatal(err)
			}
			tc.edit(v)
			bad, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateJSON(bad); err == nil {
				t.Fatalf("ASSERT_STRICT_MUTATION_REJECTED_%s", tc.name)
			}
		})
	}
}
