package communityregister

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/programcpresentation"
	"lsp-trace/internal/programctestfixture"
	"lsp-trace/internal/schema"
)

func fixturePartition(t *testing.T, graph []byte, seed uint64) []byte {
	t.Helper()
	a, err := programcpresentation.Handle(programcpresentation.Request{Input: graph, Seed: seed, PageRankTopK: 2, HubTopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	b, err := programcpresentation.JSON(a)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestRegisterSchemaMutationAndPartitionRejections(t *testing.T) {
	graph := programctestfixture.ValidV5(t)
	p := fixturePartition(t, graph, 19)
	r, err := Aggregate(graph, p)
	if err != nil {
		t.Fatal(err)
	}
	good, err := JSON(r)
	if err != nil {
		t.Fatal(err)
	}
	var mutated map[string]any
	_ = json.Unmarshal(good, &mutated)
	mutated["authority"] = 1
	bad, _ := json.Marshal(mutated)
	if _, err = schema.ValidateFor(bad, schema.FamilyTechnicalCommunityRegister, "v1"); err == nil {
		t.Fatal("ASSERT_REGISTER_SCHEMA_MUTATION_REJECTED")
	}
	if _, err = Aggregate(graph, p, p); err == nil {
		t.Fatal("ASSERT_DUPLICATE_PARTITION_REJECTED")
	}
	var partition map[string]any
	_ = json.Unmarshal(p, &partition)
	cs := partition["communities"].([]any)
	partition["communities"] = cs[:len(cs)-1]
	incomplete, _ := json.Marshal(partition)
	if _, err = Aggregate(graph, incomplete); err == nil {
		t.Fatal("ASSERT_INCOMPLETE_PARTITION_REJECTED")
	}
}
