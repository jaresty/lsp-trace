package integratedconformance

import (
	"bytes"
	"testing"

	"lsp-trace/internal/describeworker"
)

func TestADR0007ResponseV2SchemaOwnershipIsAdditive(t *testing.T) {
	for _, id := range []string{describeworker.SemanticSchemaV2, describeworker.ResponseSchemaV2, describeworker.InvocationSchemaV2} {
		raw, err := describeworker.SchemaV2(id)
		if err != nil || !bytes.Contains(raw, []byte(`"$id":"`+id+`"`)) {
			t.Fatalf("ASSERT_ADR0007_RESPONSE_V2_SCHEMA_OWNED: id=%s err=%v", id, err)
		}
	}
}
