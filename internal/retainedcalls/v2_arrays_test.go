package retainedcalls

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/acquisition"
	"testing"
)

func TestRetainedV2TypedEmptyArrays(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "missing")
	e := readExportV2(t, input)
	raw, _ := json.Marshal(e.Tables.Acquisition)
	for _, key := range []string{"required_targets", "request_ids", "expansions", "layers", "frontier_ids", "successful_empty_ids", "start_ids", "nodes", "group_ids", "occurrence_ids"} {
		if bytes.Contains(raw, []byte(`"`+key+`":null`)) {
			t.Errorf("ASSERT_TYPED_EMPTY_ARRAYS: %s is null", key)
		}
	}
	if !t.Failed() {
		t.Log("ASSERT_TYPED_EMPTY_ARRAYS: PASS")
	}
}
