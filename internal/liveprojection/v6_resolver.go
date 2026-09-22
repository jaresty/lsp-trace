package liveprojection

import (
	"context"
	"errors"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/v5sourcesnapshotv6"
)

// FullDefinitionResolver adapts the managed documentSymbol resolver to V6
// endpoint capture. ResolveDisplayRanges remains the sole containment policy.
type FullDefinitionResolver struct {
	Requester DocumentSymbolRequester
	Limits    DisplayResolutionLimits
}

func (r FullDefinitionResolver) ResolveFullDefinition(ctx context.Context, in v5sourcesnapshotv6.ResolveRequest) (v5sourcesnapshotv6.ResolveResult, error) {
	candidate := sourceprojection.Candidate{
		UnitID: in.GraphSubjectID, Role: "ENDPOINT", LogicalSourceID: in.URI,
		Range: toProjectionRange(in.ItemRange), EvidenceRange: toProjectionRange(in.EvidenceRange),
		ItemRange: toProjectionRange(in.ItemRange), SelectionRange: toProjectionRange(in.SelectionRange),
	}
	resolved, err := ResolveDisplayRanges(ctx, r.Requester, in.SessionID, in.Generation, []sourceprojection.Candidate{candidate}, r.Limits)
	if err != nil || len(resolved) != 1 || resolved[0].DisplayProvenance != v5sourcesnapshotv6.ProvenanceKind {
		if err == nil {
			err = errors.New("liveprojection: full definition display unavailable")
		}
		return v5sourcesnapshotv6.ResolveResult{}, err
	}
	return v5sourcesnapshotv6.ResolveResult{
		DisplayRange: toGraphRange(resolved[0].Range), ItemRange: in.ItemRange, SelectionRange: in.SelectionRange,
		ProvenanceKind: v5sourcesnapshotv6.ProvenanceKind, Method: v5sourcesnapshotv6.ProvenanceMethod,
		DocumentDigest: in.DocumentDigest, DocumentByteLength: uint64(len(in.Bytes)), DocumentVersion: in.DocumentVersion,
	}, nil
}

func toProjectionRange(r graph.Range) sourceprojection.Range {
	return sourceprojection.Range{Start: sourceprojection.Position{Line: r.Start.Line, Character: r.Start.Character}, End: sourceprojection.Position{Line: r.End.Line, Character: r.End.Character}}
}
func toGraphRange(r sourceprojection.Range) graph.Range {
	return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
}

var _ v5sourcesnapshotv6.FullDefinitionResolver = FullDefinitionResolver{}
