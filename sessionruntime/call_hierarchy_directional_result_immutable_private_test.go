package sessionruntime

import (
	"crypto/sha256"
	"reflect"
	"testing"

	"lsp-trace/internal/directionalraw"
)

func TestDirectionalWireReceiptPrivateMetadataIsolation(t *testing.T) {
	request, response, result := []byte("request"), []byte("response"), []byte("result")
	receipt := DirectionalWireReceiptPrivate{
		method: "callHierarchy/incomingCalls",
		metadata: directionalraw.CompletionMetadataPrivate{
			RequestIDPrivate: 7, GenerationPrivate: 9,
			RequestLengthPrivate: uint64(len(request)), ResponseLengthPrivate: uint64(len(response)), ResultLengthPrivate: uint64(len(result)),
			RequestSHA256Private: sha256.Sum256(request), ResponseSHA256Private: sha256.Sum256(response), ResultSHA256Private: sha256.Sum256(result),
		},
		observations: []directionalraw.Observation{{Method: "callHierarchy/incomingCalls"}},
	}

	observations := receipt.ObservationsPrivate()
	observations[0].Method = "mutated"
	if receipt.MethodPrivate() != "callHierarchy/incomingCalls" || receipt.RequestIDPrivate() != 7 || receipt.GenerationPrivate() != 9 ||
		receipt.RequestLengthPrivate() != uint64(len(request)) || receipt.ResponseLengthPrivate() != uint64(len(response)) || receipt.ResultLengthPrivate() != uint64(len(result)) ||
		receipt.RequestSHA256Private() != sha256.Sum256(request) || receipt.ResponseSHA256Private() != sha256.Sum256(response) || receipt.ResultSHA256Private() != sha256.Sum256(result) ||
		receipt.ObservationsPrivate()[0].Method != "callHierarchy/incomingCalls" {
		t.Fatal("ASSERT_DIRECTIONAL_WIRE_RECEIPT_PRIVATE_METADATA_ISOLATION")
	}
	typeOf := reflect.TypeOf(receipt)
	for i := 0; i < typeOf.NumField(); i++ {
		if typeOf.Field(i).Type.Kind() == reflect.Slice && typeOf.Field(i).Type.Elem().Kind() == reflect.Uint8 {
			t.Fatal("ASSERT_DIRECTIONAL_WIRE_RECEIPT_PRIVATE_NO_RAW_BYTE_FIELD")
		}
	}
}
