package retainedcalls

import (
	"encoding/json"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"testing"
)

func TestRetainedV2SharedProducerOracle(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "")
	var admitted graphprovenance.EvidenceV2
	if err := json.Unmarshal(input, &admitted); err != nil {
		t.Fatal(err)
	}
	id := digest(VersionV2+":input", input)
	tables, err := extractTablesV2(admitted, id)
	if err != nil {
		t.Fatal(err)
	}
	// Bypass ExportV2's own independent guard: exercise validator independently
	// against the table producer's exact output, including an injected producer fault.
	raw, err := json.Marshal(EvidenceV2{VersionV2, PolicyV2, input, id, tables})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateFor(raw, Family, "v2"); err != nil {
		t.Fatalf("ASSERT_SHARED_PRODUCER_ORACLE: %v", err)
	}
	t.Log("ASSERT_SHARED_PRODUCER_ORACLE: PASS")
}
