package retainedcalls

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRetainedV2LeafNamesAreNotCarrierKeys(t *testing.T) {
	for _, raw := range []string{`{"name":"data","kind":12}`, `{"name":"graph_bytes"}`, `{"name":"canonical_receipt"}`, `{"data":{"input_bytes":"not-base64","deep":[[[0]]]}}`} {
		if err := scanLeafV2([]byte(raw), 1024); err != nil {
			t.Fatalf("ASSERT_LEAF_NAME_OPAQUE: %s: %v", raw, err)
		}
	}
	t.Log("ASSERT_LEAF_NAME_OPAQUE: PASS")
}
func TestRetainedV2DecodedCarrierDepth(t *testing.T) {
	for _, depth := range []int{64, 65} {
		leaf := strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth)
		raw := []byte(`{"canonical_receipt":"` + base64.StdEncoding.EncodeToString([]byte(leaf)) + `"}`)
		err := preflightExportV2(raw, 1<<20)
		if (err == nil) != (depth == 64) {
			t.Fatalf("ASSERT_DECODED_DEPTH: depth=%d err=%v", depth, err)
		}
	}
}
