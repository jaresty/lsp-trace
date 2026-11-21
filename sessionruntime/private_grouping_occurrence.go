package sessionruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	methodsyntax "lsp-trace/internal/adr0011methodsyntax"
	qt "lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/strictjson"
)

// SelectPrivateDefinitionOccurrenceV1 is a pure revalidation of historical
// evidence, not a custody issuer. A containing symbol is context, not the
// definition target and not proof that the occurrence is a call.
func SelectPrivateDefinitionOccurrenceV1(q qt.Query, symbols []byte, pair OwnedMethodPair) (qt.Candidate, error) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct{ Line, Character *uint32 } `json:"position"`
	}
	if pair.Method != "textDocument/definition" || pair.SessionID != q.SessionID || pair.Generation != q.Generation || pair.Source == nil || pair.Source.URI != q.URI || pair.Source.SHA256 != q.SourceDigest || fmt.Sprint(pair.Source.Version) != q.DocumentVersion || strictjson.RejectDuplicates(pair.Params) != nil || json.Unmarshal(pair.Params, &p) != nil || p.TextDocument.URI != q.URI || p.Position.Line == nil || p.Position.Character == nil || *p.Position.Line != q.Line || *p.Position.Character != q.Character {
		return qt.Candidate{}, ErrPrivateGroupingOwned
	}
	result, failure := methodsyntax.ParseRaw(pair.Method, pair.Result, 256)
	if failure != nil || len(result.Items) == 0 {
		return qt.Candidate{}, ErrPrivateGroupingOwned
	}
	return qt.SelectOccurrenceContainerV1(q, symbols)
}

// ReselectPrivateGroupingTargetV1 preserves declaration-only selection unless
// the retained view explicitly carries the separate occurrence representation.
func ReselectPrivateGroupingTargetV1(v PrivateGroupingOwnedView) (qt.Candidate, error) {
	if v.Target.Version != qt.OccurrenceCandidateVersionV1 {
		return qt.SelectDocumentSymbolCandidateV1(v.Query, v.Symbol.Result)
	}
	if len(v.Methods) != 1 {
		return qt.Candidate{}, ErrPrivateGroupingOwned
	}
	return SelectPrivateDefinitionOccurrenceV1(v.Query, v.Symbol.Result, v.Methods[0])
}

// MatchesGroupingNativeV1 is checked under paid selected conversion. It binds
// the original prepared call item to the actual containing declaration, rather
// than assuming that a common URI makes every query a callable occurrence.
func (o PrivateDiscoveryOwnedMethods) MatchesGroupingNativeV1(index int, v PrivateGroupingOwnedView) bool {
	if !o.Available() || index < 0 || index >= len(o.state.calls) {
		return false
	}
	call := o.state.calls[index].view
	if len(call.Requests) != 3 || call.Request.URI != v.Query.URI || call.Session.SessionID != v.Query.SessionID || call.Session.Generation != v.Query.Generation || call.Session.Workspace != v.Workspace || call.Metadata.PositionEncoding != v.Query.Encoding || call.Supply.URI != v.Query.URI || call.Supply.SessionID != v.Query.SessionID || call.Supply.Generation != v.Query.Generation || call.Supply.DocumentVersion != v.Supply.DocumentVersion || !bytes.Equal(call.Supply.Content, v.Source) {
		return false
	}
	var items []lsp.CallHierarchyItem
	if json.Unmarshal(call.Requests[0].Result, &items) != nil || len(items) != 1 {
		return false
	}
	item := items[0]
	r := func(x lsp.Range) qt.Range {
		return qt.Range{Start: qt.Position{Line: x.Start.Line, Character: x.Start.Character}, End: qt.Position{Line: x.End.Line, Character: x.End.Character}}
	}
	return item.URI == v.Query.URI && item.Name == v.Target.SymbolName && int(item.Kind) == v.Target.SymbolKind && r(item.Range) == v.Target.DisplayRange && r(item.SelectionRange) == v.Target.SelectionRange
}
