package sessionruntime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/lsp"
)

func TestPrivateDiscoveryRelationsRejectBeforeWire(t *testing.T) {
	// No Manager/transport exists. An admission regression would dereference it
	// rather than silently issuing an out-of-scope or over-parameter-budget call.
	s := &PrivateDiscoverySession{members: []PrivateDiscoveryMember{{URI: "file:///inside.go", LanguageID: "go"}}, supplied: map[string]bool{"file:///inside.go": true}, client: privateDiscoveryClient{metadata: SessionMetadata{ReferencesSupport: true, DefinitionSupport: true, CallHierarchySupport: true}}}
	ctx := context.Background()
	if _, err := s.References(ctx, "file:///foreign.go", lsp.Position{}); err == nil {
		t.Fatal("foreign references admitted")
	}
	if _, err := s.Definition(ctx, "file:///foreign.go", lsp.Position{}); err == nil {
		t.Fatal("foreign definition admitted")
	}
	item := lsp.CallHierarchyItem{URI: "file:///inside.go", Name: strings.Repeat("n", 8193)}
	if _, err := s.OutgoingCalls(ctx, item); err == nil {
		t.Fatal("oversized outgoing item admitted")
	}
	if _, err := s.IncomingCalls(ctx, item); err == nil {
		t.Fatal("oversized incoming item admitted")
	}
	s.closed = true
	if _, err := s.References(ctx, "file:///inside.go", lsp.Position{}); err == nil {
		t.Fatal("closed references admitted")
	}
	if _, err := s.Definition(ctx, "file:///inside.go", lsp.Position{}); err == nil {
		t.Fatal("closed definition admitted")
	}
	t.Log("ASSERT_FOLDER_RELATION_SCOPE_AND_CUSTODY: PASS")
}

func TestPrivateDiscoveryRelationParameterBound(t *testing.T) {
	uri := "file:///inside.go"
	item := lsp.CallHierarchyItem{URI: uri, Name: strings.Repeat("n", 8192-len(uri))}
	if !boundedPrivateDiscoveryItem(item) {
		t.Fatal("at-bound rejected")
	}
	item.Name += "n"
	if boundedPrivateDiscoveryItem(item) {
		t.Fatal("over-bound admitted")
	}
	item.Name = "name"
	item.Data = json.RawMessage(`"` + strings.Repeat("<", 8192-len(uri)-len(item.Name)-2) + `"`)
	if !boundedPrivateDiscoveryItem(item) {
		t.Fatal("at-bound opaque data rejected")
	}
	raw, err := json.Marshal(lsp.CallHierarchyOutgoingCallsParams{Item: item})
	if err != nil || len(raw) > 64<<10 {
		t.Fatal("escaped item exceeded unchanged parameter quote", len(raw), err)
	}
	item.Data = append(item.Data, ' ')
	if boundedPrivateDiscoveryItem(item) {
		t.Fatal("over-bound opaque data admitted")
	}
	item.Data = nil
	item.Tags = make([]int, 33)
	if boundedPrivateDiscoveryItem(item) {
		t.Fatal("over-bound tag list admitted")
	}
	t.Log("ASSERT_FOLDER_RELATION_PARAMETER_BOUND: PASS")
}
