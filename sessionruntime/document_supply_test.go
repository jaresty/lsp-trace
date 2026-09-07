package sessionruntime

import (
	"reflect"
	"testing"
)

func TestDocumentSupplyOptInSurface(t *testing.T) {
	field, ok := reflect.TypeOf(DocumentRequest{}).FieldByName("CaptureSupply")
	if !ok || field.Type.Kind() != reflect.Bool {
		t.Fatal("ASSERT_DOCUMENT_SUPPLY_OPT_IN: missing bool CaptureSupply; legacy callers must not retain source bytes")
	}
	field, ok = reflect.TypeOf(DocumentResult{}).FieldByName("Supply")
	if !ok || field.Type.Kind() != reflect.Pointer {
		t.Fatal("ASSERT_DOCUMENT_SUPPLY_OWNED_RESULT: missing optional owned supply observation")
	}
}
