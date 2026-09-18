package liveprojection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"lsp-trace/internal/sourceprojection"
	"lsp-trace/sessionruntime"
)

type DocumentSymbolRequester interface {
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
}

type DisplayResolutionLimits struct {
	MaxWork        int
	MaxMessages    int
	MaxBytes       int64
	RequestTimeout time.Duration
}

type documentSymbol struct {
	Name           string                 `json:"name"`
	Range          sourceprojection.Range `json:"range"`
	SelectionRange sourceprojection.Range `json:"selectionRange"`
	Location       *struct {
		URI   string                 `json:"uri"`
		Range sourceprojection.Range `json:"range"`
	} `json:"location,omitempty"`
	Children []documentSymbol `json:"children,omitempty"`
}

// ResolveDisplayRanges replaces only Candidate.Range with a server-reported
// full symbol range. EvidenceRange, ItemRange, and SelectionRange remain exact.
func ResolveDisplayRanges(ctx context.Context, requester DocumentSymbolRequester, sessionID string, generation uint64, candidates []sourceprojection.Candidate, limits DisplayResolutionLimits) ([]sourceprojection.Candidate, error) {
	if requester == nil || limits.MaxWork <= 0 || limits.MaxMessages <= 0 || limits.MaxBytes <= 0 || limits.RequestTimeout <= 0 {
		return nil, errors.New("liveprojection: invalid display resolution configuration")
	}
	byURI := map[string][]int{}
	for i := range candidates {
		byURI[candidates[i].LogicalSourceID] = append(byURI[candidates[i].LogicalSourceID], i)
	}
	uris := make([]string, 0, len(byURI))
	for uri := range byURI {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	if len(uris) > limits.MaxWork {
		return nil, fmt.Errorf("liveprojection: display resolution work %d exceeds limit %d", len(uris), limits.MaxWork)
	}

	out := append([]sourceprojection.Candidate(nil), candidates...)
	for _, uri := range uris {
		params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
		response := requester.RoundTrip(ctx, sessionruntime.RoundTripRequest{SessionID: sessionID, Generation: generation, Method: "textDocument/documentSymbol", Params: params, Deadline: time.Now().Add(limits.RequestTimeout), MaxMessages: limits.MaxMessages, MaxBytes: limits.MaxBytes})
		if response.Failure != "" || response.ServerError != nil {
			return nil, fmt.Errorf("liveprojection: documentSymbol unavailable for %q", uri)
		}
		symbols, err := decodeDocumentSymbols(response.Result)
		if err != nil {
			return nil, fmt.Errorf("liveprojection: documentSymbol invalid for %q: %w", uri, err)
		}
		for _, index := range byURI[uri] {
			candidate := out[index]
			var matches []sourceprojection.Range
			for _, symbol := range symbols {
				if candidate.Role == "ENDPOINT" {
					if symbol.SelectionRange == candidate.SelectionRange {
						matches = append(matches, symbol.Range)
					}
				} else if contains(symbol.Range, candidate.EvidenceRange) {
					matches = append(matches, symbol.Range)
				}
			}
			if candidate.Role == "ENDPOINT" && len(matches) == 0 {
				for _, symbol := range symbols {
					if contains(symbol.Range, candidate.EvidenceRange) {
						matches = append(matches, symbol.Range)
					}
				}
			}
			if candidate.Role == "ENDPOINT" && len(matches) == 0 {
				continue
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("liveprojection: display range unavailable for unit=%q role=%q uri=%q evidence=%+v selection=%+v", candidate.UnitID, candidate.Role, candidate.LogicalSourceID, candidate.EvidenceRange, candidate.SelectionRange)
			}
			sort.Slice(matches, func(i, j int) bool { return rangeSize(matches[i]) < rangeSize(matches[j]) })
			if len(matches) > 1 && rangeSize(matches[0]) == rangeSize(matches[1]) && matches[0] != matches[1] {
				return nil, fmt.Errorf("liveprojection: display range ambiguous for unit=%q role=%q uri=%q evidence=%+v selection=%+v", candidate.UnitID, candidate.Role, candidate.LogicalSourceID, candidate.EvidenceRange, candidate.SelectionRange)
			}
			out[index].Range = matches[0]
			out[index].DisplayProvenance = "SERVER_REPORTED_DOCUMENT_SYMBOL"
		}
	}
	return out, nil
}

func decodeDocumentSymbols(raw json.RawMessage) ([]documentSymbol, error) {
	var roots []documentSymbol
	if err := json.Unmarshal(raw, &roots); err != nil {
		return nil, err
	}
	var flat []documentSymbol
	var walk func([]documentSymbol)
	walk = func(items []documentSymbol) {
		for _, item := range items {
			if item.Location != nil {
				item.Range = item.Location.Range
				item.SelectionRange = item.Location.Range
			}
			flat = append(flat, item)
			walk(item.Children)
		}
	}
	walk(roots)
	return flat, nil
}

func contains(outer, inner sourceprojection.Range) bool {
	return positionLE(outer.Start, inner.Start) && positionLE(inner.End, outer.End)
}
func positionLE(a, b sourceprojection.Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character <= b.Character)
}
func rangeSize(r sourceprojection.Range) uint64 {
	return (uint64(r.End.Line-r.Start.Line) << 32) + uint64(r.End.Character) + uint64(^r.Start.Character)
}
