package sessionruntime

import (
	"context"
	"encoding/json"
	"testing"
)

type capabilityPreparer struct{ doc DocumentResult }

func (p capabilityPreparer) PrepareDocument(context.Context, DocumentRequest) DocumentResult {
	return cloneDocumentResult(p.doc)
}

func TestPreparedDocumentCapabilityCannotBeForgedReusedOrCrossed(t *testing.T) {
	req := DocumentRequest{SessionID: "s", Generation: 7, URI: "file:///w/a.go", LanguageID: "go", CaptureSupply: true}
	doc := DocumentResult{URI: req.URI, LanguageID: "go", Version: 3, Supply: &DocumentSupply{SessionID: "s", Generation: 7, URI: req.URI, DocumentVersion: 3, Method: "textDocument/didOpen", Content: []byte("package a\n"), Params: json.RawMessage(`{"textDocument":{"uri":"file:///w/a.go"}}`)}}
	_, capability, err := PrepareDocumentForOperation(context.Background(), capabilityPreparer{doc}, req, "request-a")
	if err != nil {
		t.Fatal(err)
	}
	binding := capability.Binding()
	if _, err := ConsumePreparedDocumentCapability(PreparedDocumentCapability{}, binding); err == nil {
		t.Fatal("ASSERT_PREPARED_CAPABILITY_ZERO_VALUE_FORGERY_REJECTED")
	}
	crossed := binding
	crossed.RequestID = "request-b"
	if _, err := ConsumePreparedDocumentCapability(capability, crossed); err == nil {
		t.Fatal("ASSERT_PREPARED_CAPABILITY_CROSS_OPERATION_REJECTED")
	}
	if _, err := ConsumePreparedDocumentCapability(capability, binding); err != nil {
		t.Fatalf("ASSERT_PREPARED_CAPABILITY_EXACT_USE_ACCEPTED: %v", err)
	}
	if _, err := ConsumePreparedDocumentCapability(capability, binding); err == nil {
		t.Fatal("ASSERT_PREPARED_CAPABILITY_REPLAY_REJECTED")
	}
}
