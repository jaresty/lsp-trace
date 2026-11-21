package sessionruntime

import (
	"context"
	"crypto/sha256"
	"fmt"
	methodsyntax "lsp-trace/internal/adr0011methodsyntax"
	"lsp-trace/internal/adr0011querytarget"
	"strconv"
	"time"
)

// Scalar descriptors are built under the original per-request retention quote.
// They are selectors, not evidence custody or downstream processing permission.
type PrivateDiscoveryGroupingQuery struct {
	Method, URI     string
	Line, Character uint32
	ObservedEmpty   bool
}
type privateDiscoveryGroupingOriginal struct{ view PrivateGroupingOwnedView }

func (o PrivateDiscoveryOwnedMethods) GroupingSession() SessionLeaseRequest {
	if !o.Available() {
		return SessionLeaseRequest{}
	}
	return SessionLeaseRequest{SessionID: o.state.sessionID, Generation: o.state.generation, Workspace: o.state.workspace}
}
func (o PrivateDiscoveryOwnedMethods) GroupingQueryCount() int {
	if !o.Available() {
		return 0
	}
	return len(o.state.queries)
}
func (o PrivateDiscoveryOwnedMethods) GroupingQuery(index int) (PrivateDiscoveryGroupingQuery, bool) {
	if !o.Available() || index < 0 || index >= 256 {
		return PrivateDiscoveryGroupingQuery{}, false
	}
	if index < 128 {
		if index >= len(o.state.queries) {
			return PrivateDiscoveryGroupingQuery{}, false
		}
		return o.state.queries[index], true
	}
	i := index - 128
	if i >= len(o.state.calls) {
		return PrivateDiscoveryGroupingQuery{}, false
	}
	q := o.state.calls[i].view.Request
	if q.Line == nil || q.Character == nil {
		return PrivateDiscoveryGroupingQuery{}, false
	}
	return PrivateDiscoveryGroupingQuery{Method: "CALLS", URI: q.URI, Line: *q.Line, Character: *q.Character}, true
}

// ConsumeDiscoveryGroupingOwned transfers one historical query from actual
// runtime originals. It neither reconstructs a live lease nor accepts pair DTOs.
// The fixed inventory producer must reserve conversion work before calling it.
// Failure retires the slot; copies of o share that retirement.
func (o PrivateDiscoveryOwnedMethods) ConsumeDiscoveryGroupingOwned(ctx context.Context, index int, occurrence, revision, privacy string, consume func(PrivateGroupingOwnedView) error) (PrivateGroupingSourceEvidence, error) {
	fail := func(stage string) (PrivateGroupingSourceEvidence, error) {
		return PrivateGroupingSourceEvidence{}, fmt.Errorf("%w: retained %s", ErrPrivateGroupingOwned, stage)
	}
	descriptor, ok := o.GroupingQuery(index)
	if ctx == nil || ctx.Err() != nil || !ok || consume == nil || !PrivateGroupingFieldValid(occurrence) || !PrivateGroupingFieldValid(revision) || !PrivateGroupingFieldValid(privacy) {
		return fail("precheck")
	}
	if descriptor.Method != "CALLS" && descriptor.Method != "textDocument/definition" && descriptor.Method != "textDocument/references" {
		return fail("method")
	}
	s := o.state
	s.nativeMu.Lock()
	if s.groupingTaken[index] {
		s.nativeMu.Unlock()
		return fail("spent")
	}
	s.groupingTaken[index] = true
	s.nativeMu.Unlock()
	supply := s.sources[descriptor.URI]
	if supply == nil || supply.SessionID != s.sessionID || supply.Generation != s.generation || supply.DocumentVersion <= 0 || s.metadata.ProviderName == "" || s.metadata.ProviderVersion == "" || !s.metadata.DocumentSymbolSupport {
		return fail("supply-metadata")
	}
	binding := OwnedDocumentBinding{URI: descriptor.URI, Version: supply.DocumentVersion, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(supply.Content))}
	var symbol OwnedMethodPair
	found := 0
	for _, p := range s.pairs {
		if p.Method == "textDocument/documentSymbol" && p.Source != nil && *p.Source == binding {
			symbol = p
			found++
		}
	}
	if found != 1 {
		return fail("symbol-binding")
	}
	query := adr0011querytarget.Query{OccurrenceID: occurrence, URI: descriptor.URI, Encoding: s.metadata.PositionEncoding, DocumentVersion: strconv.Itoa(binding.Version), SourceDigest: binding.SHA256, SessionID: s.sessionID, Generation: s.generation, Line: descriptor.Line, Character: descriptor.Character}
	target, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(query, symbol.Result)
	if err != nil {
		if descriptor.Method != "textDocument/definition" || index >= len(s.pairs) {
			return fail("target-selection")
		}
		target, err = SelectPrivateDefinitionOccurrenceV1(query, symbol.Result, s.pairs[index])
		if err != nil {
			return fail("occurrence-selection")
		}
		// Returned targets must remain inside the retained source universe. This
		// does not relabel the containing declaration as the definition target.
		parsed, failure := methodsyntax.ParseRaw(descriptor.Method, s.pairs[index].Result, 256)
		if failure != nil {
			return fail("definition-result")
		}
		for _, item := range parsed.Items {
			if s.sources[item.URI] == nil {
				return fail("definition-target-source")
			}
		}
	}
	v := PrivateGroupingOwnedView{Query: query, Target: target, Source: supply.Content, Supply: *supply, Metadata: s.metadata, Revision: revision, Privacy: privacy, Workspace: s.workspace, Deadline: s.deadline, Symbol: symbol}
	if index < 128 {
		pair := s.pairs[index]
		if pair.Source == nil || *pair.Source != binding || pair.SessionID != s.sessionID || pair.Generation != s.generation || pair.Method != descriptor.Method {
			return fail("method-binding")
		}
		v.Methods = []OwnedMethodPair{pair}
	}
	if _, ok := PrivateGroupingQueryEncodedBound(v); !ok {
		return fail("bounds")
	}
	original := PrivateGroupingSourceEvidence{discovery: &privateDiscoveryGroupingOriginal{view: v}}
	// Borrow only defensive copies. A callback cannot alter the retained source.
	borrowed, err := original.Snapshot(PrivateGroupingQueryMaxBytes)
	if err != nil {
		return fail("snapshot")
	}
	if err := consume(borrowed); err != nil {
		return PrivateGroupingSourceEvidence{}, err
	}
	if ctx.Err() != nil || !time.Now().Before(s.deadline) {
		return fail("liveness")
	}
	return original, nil
}
