package sessionruntime

import (
	"context"
	"encoding/json"

	"lsp-trace/internal/lsp"
)

// These are data-only structural observations inside the SAME closed discovery
// enclosure. They do not expose Manager, original method-pair custody, native
// targets, or a grouping/Describe handoff. Every call uses the original deadline
// and cumulative request/message/byte/work/retention balances.
type privateDiscoveryRelationPosition struct {
	TextDocument lsp.TextDocumentIdentifier        `json:"textDocument"`
	Position     lsp.Position                      `json:"position"`
	Context      *privateDiscoveryReferenceContext `json:"context,omitempty"`
}
type privateDiscoveryReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

func (s *PrivateDiscoverySession) References(ctx context.Context, uri string, position lsp.Position) ([]lsp.Location, error) {
	if !s.admitURI(uri, "", true) || !s.client.metadata.ReferencesSupport {
		return nil, ErrPrivateDiscovery
	}
	var out []lsp.Location
	p := privateDiscoveryRelationPosition{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: position, Context: &privateDiscoveryReferenceContext{}}
	err := s.client.call(s.diagnosticContext(ctx, uri), "textDocument/references", p, &out)
	return out, err
}

// Definition deliberately preserves the server's null/Location/LocationLink
// wire shape as data. Callers must not turn unavailable/failed into observed-empty.
func (s *PrivateDiscoverySession) Definition(ctx context.Context, uri string, position lsp.Position) (json.RawMessage, error) {
	if !s.admitURI(uri, "", true) || !s.client.metadata.DefinitionSupport {
		return nil, ErrPrivateDiscovery
	}
	var out json.RawMessage
	p := privateDiscoveryRelationPosition{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: position}
	err := s.client.call(s.diagnosticContext(ctx, uri), "textDocument/definition", p, &out)
	return out, err
}

func boundedPrivateDiscoveryItem(i lsp.CallHierarchyItem) bool {
	// Bound escaped JSON BEFORE allocation; Data is already JSON and Tags are
	// fixed-width integers. This stays below call's existing 64KiB parameter cap.
	return len(i.URI) > 0 && len(i.URI) <= 4096 && len(i.URI)+len(i.Name)+len(i.Detail)+len(i.Data) <= 8192 && len(i.Tags) <= 32
}
func (s *PrivateDiscoverySession) OutgoingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, error) {
	if !s.admitURI(item.URI, "", true) || !s.client.metadata.CallHierarchySupport || !boundedPrivateDiscoveryItem(item) {
		return nil, ErrPrivateDiscovery
	}
	var out []lsp.CallHierarchyOutgoingCall
	if s.client.w.ownedMethods != nil && s.client.w.ownedMethods.directions != 0 {
		err := s.retainedCalls(ctx, item, "callHierarchy/outgoingCalls", &out)
		return out, err
	}
	err := s.client.call(s.diagnosticContext(ctx, item.URI), "callHierarchy/outgoingCalls", lsp.CallHierarchyOutgoingCallsParams{Item: item}, &out)
	return out, err
}
func (s *PrivateDiscoverySession) IncomingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, error) {
	if !s.admitURI(item.URI, "", true) || !s.client.metadata.CallHierarchySupport || !boundedPrivateDiscoveryItem(item) {
		return nil, ErrPrivateDiscovery
	}
	var out []lsp.CallHierarchyIncomingCall
	if s.client.w.ownedMethods != nil && s.client.w.ownedMethods.directions != 0 {
		err := s.retainedCalls(ctx, item, "callHierarchy/incomingCalls", &out)
		return out, err
	}
	err := s.client.call(s.diagnosticContext(ctx, item.URI), "callHierarchy/incomingCalls", lsp.CallHierarchyIncomingCallsParams{Item: item}, &out)
	return out, err
}
