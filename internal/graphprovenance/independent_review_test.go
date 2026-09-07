package graphprovenance

import (
	"context"
	"encoding/json"
	"lsp-trace/internal/graph"
	"testing"
)

// Imported from the independent 20a9c02 review.
func TestIndependentLayerCensusCompleteness(t *testing.T) {
	root, raw, seed := fixture(t)
	out, err := Capture(context.Background(), raw, root, seed, "s", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFor(out, Family, "v1"); err != nil {
		t.Fatal(err)
	}
	var e Evidence
	if err := json.Unmarshal(out, &e); err != nil {
		t.Fatal(err)
	}
	for _, b := range e.Bindings {
		if b.Pointer == "/slice/layers/0/node_ids/0" {
			return
		}
	}
	t.Fatal("VALIDATOR_ACCEPTED_MISSING_MANDATORY_LAYER_NODE_REFERENCE: /slice/layers/0/node_ids/0")
}

func TestIndependentRejectMultipleSeeds(t *testing.T) {
	root, raw, seed := fixture(t)
	var g graph.Result
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	var native struct {
		Seeds []graph.SeedResult   `json:"seeds"`
		Slice *graph.SliceEvidence `json:"slice"`
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	g.Seeds, g.Slice = native.Seeds, native.Slice
	other := g.Invocation.Seeds[0]
	other.Label = "second"
	g.Invocation.Seeds = append(g.Invocation.Seeds, other)
	result := g.Seeds[0]
	result.Label = "second"
	g.Seeds = append(g.Seeds, result)
	g.Canonicalize()
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Capture(context.Background(), raw, root, seed, "s", 1, nil)
	if err == nil {
		if _, e := ValidateFor(out, Family, "v1"); e != nil {
			t.Fatal(e)
		}
		t.Fatal("VALIDATOR_ACCEPTED_MULTISEED_GRAPH_OUTSIDE_SINGLE_AT_CONTRACT")
	}
}
