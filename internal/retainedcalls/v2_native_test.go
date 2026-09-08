package retainedcalls

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"testing"
)

func TestRetainedV2NativeProvenancePreservation(t *testing.T) {
	input, original := fixtureV2(t, acquisition.Slice, "")
	e := readExportV2(t, input)
	p, err := ReconstructV2(e.Tables)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(p.Result.Graph)
	want, _ := json.Marshal(original.Graph)
	if !bytes.Equal(got, want) {
		t.Fatal("ASSERT_NON_CALLS_FIELDS: projection lost")
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(want, &fields); err != nil {
		t.Fatal(err)
	}
	count := 0
	for name, value := range fields {
		switch name {
		case "nodes", "edges", "seed_memberships", "evidence_receipt":
			continue
		}
		count++
		found := false
		for _, row := range e.Tables.NativeFields {
			if row.Name == name && bytes.Equal(row.Value, value) {
				found = true
			}
		}
		if !found {
			t.Fatal("ASSERT_NON_CALLS_FIELDS", name)
		}
	}
	if count != len(e.Tables.NativeFields) {
		t.Fatal("ASSERT_NATIVE_FIELD_CENSUS")
	}
	// This native relation is representable by graph-v3, but not admitted by the
	// coordinator's deterministic replay. Export must not broaden that boundary.
	original.Graph.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: original.Request.Root.Locator.URI, SeedLabel: "root", SeedLabels: []string{"root"}, Candidate: original.Graph.Nodes[1]}}
	if err = acquisition.ValidateResult(original); err == nil {
		t.Fatal("ASSERT_NATIVE_ACQUISITION_BOUNDARY")
	}
	t.Log("ASSERT_NON_CALLS_FIELDS: PASS")
}
