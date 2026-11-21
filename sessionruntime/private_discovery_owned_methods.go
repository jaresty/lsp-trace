package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// PrivateDiscoveryOwnedMethods retains runtime-produced method originals and
// optional actual directional originals. It is NOT native/grouping admission.
// No public fields, JSON decoder,
// or constructor can turn caller observations into this process-local object.
// Release occurs only after the enclosing runtime has joined successfully.
// The accounting owner imposes its additional Complete/readback barrier.
type PrivateDiscoveryOwnedMethods struct{ state *privateDiscoveryOwnedMethods }
type privateDiscoveryOwnedMethods struct {
	nativeMu             sync.Mutex
	nativeTaken          [128]bool
	groupingTaken        [256]bool
	queries              []PrivateDiscoveryGroupingQuery
	metadata             SessionMetadata
	sessionID, workspace string
	generation           uint64
	deadline             time.Time
	sources              map[string]*DocumentSupply
	pairs                []OwnedMethodPair
	directions           PrivateCallDirections
	calls                []privateDiscoveryOwnedCall
}

// Available reports retained originals, not admission/authority or completeness.
func (o PrivateDiscoveryOwnedMethods) Available() bool {
	return o.state != nil && time.Now().Before(o.state.deadline) && len(o.state.pairs) > 0
}

func (w *privateDiscoveryWindow) prepayOwnedMethods(enabled bool, directions PrivateCallDirections) bool {
	if !enabled {
		return true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, 64 << 10}, privateDiscoveryCharge{&w.remaining.Work, 64 << 10}) {
		return false
	}
	if directions != 0 && !w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, 64 << 10}, privateDiscoveryCharge{&w.remaining.Work, 64 << 10}) {
		return false
	}
	deadline, _ := w.ctx.Deadline()
	w.ownedMethods = &privateDiscoveryOwnedMethods{directions: directions, deadline: deadline, sources: make(map[string]*DocumentSupply, 16), pairs: make([]OwnedMethodPair, 0, 128)}
	return true
}

func (w *privateDiscoveryWindow) ownSupply(s *DocumentSupply) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ownedMethods != nil && !w.ownedReleased && s != nil {
		w.ownedMethods.sources[s.URI] = s
	}
}

func (w *privateDiscoveryWindow) bindOwnedContext(metadata SessionMetadata, sessionID string, generation uint64, workspace string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ownedMethods != nil && !w.ownedReleased {
		w.ownedMethods.metadata, w.ownedMethods.sessionID, w.ownedMethods.generation, w.ownedMethods.workspace = metadata, sessionID, generation, workspace
	}
}

// Freeze a detached private index after cleanup. Late callback work cannot
// append to a released result. Copies are covered by each pre-effect quote.
func (w *privateDiscoveryWindow) takeOwnedMethods() PrivateDiscoveryOwnedMethods {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ownedReleased || w.ownedMethods == nil {
		return PrivateDiscoveryOwnedMethods{}
	}
	w.ownedReleased = true
	v := &privateDiscoveryOwnedMethods{queries: append([]PrivateDiscoveryGroupingQuery(nil), w.ownedMethods.queries...), directions: w.ownedMethods.directions, calls: append([]privateDiscoveryOwnedCall(nil), w.ownedMethods.calls...), metadata: w.ownedMethods.metadata, sessionID: w.ownedMethods.sessionID, generation: w.ownedMethods.generation, workspace: w.ownedMethods.workspace, deadline: w.ownedMethods.deadline, sources: make(map[string]*DocumentSupply, len(w.ownedMethods.sources)), pairs: append([]OwnedMethodPair(nil), w.ownedMethods.pairs...)}
	for uri, s := range w.ownedMethods.sources {
		v.sources[uri] = s
	}
	return PrivateDiscoveryOwnedMethods{state: v}
}

// quoteOwnedMethod charges rejecting as well as successful retention before the
// request effect. The existing sourceLimit already paid for the supplied source
// and its copies. This debit covers pair/source checks and bounded pair copies;
// it is carved from, not added to, the original retained/work ceilings.
func (w *privateDiscoveryWindow) quoteOwnedMethod(req *RoundTripRequest, uri string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ownedReleased {
		return false
	}
	if w.ownedMethods == nil {
		return true
	}
	switch req.Method {
	case "textDocument/documentSymbol", "textDocument/definition", "textDocument/references":
	default:
		return true // CALLS must use its separate existing directional owner.
	}
	s := w.ownedMethods.sources[uri]
	if s == nil || len(w.ownedMethods.pairs) >= 128 || req.MaxBytes < 1 || req.MaxBytes > 1<<20 {
		return false
	}
	b := uint64(req.MaxBytes) + uint64(len(req.Params)) + uint64(len(s.Content)) + 4096
	if !w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, 8 * b}, privateDiscoveryCharge{&w.remaining.Work, 64 * b}) {
		return false
	}
	req.CaptureOwnedMethodPair = true
	// Use the actual supplied bytes/version; Manager independently checks this
	// against its private document record before producing the original pair.
	req.ExpectedOwnedDocument = &OwnedDocumentBinding{URI: uri, Version: s.DocumentVersion, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(s.Content))}
	return true
}

func (w *privateDiscoveryWindow) retainOwnedMethod(req RoundTripRequest, result RoundTripResult) bool {
	if !req.CaptureOwnedMethodPair {
		return true
	}
	pair, ok := result.CompletedOwnedMethodPair()
	if !ok || !validOwnedMethodPair(req, result, pair.Result) || pair.Source == nil || *pair.Source != *req.ExpectedOwnedDocument {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ownedReleased || w.ownedMethods == nil || len(w.ownedMethods.pairs) >= 128 {
		return false
	}
	var p struct {
		Position struct{ Line, Character uint32 } `json:"position"`
	}
	if json.Unmarshal(pair.Params, &p) != nil {
		return false
	}
	trimmed := bytes.TrimSpace(pair.Result)
	w.ownedMethods.queries = append(w.ownedMethods.queries, PrivateDiscoveryGroupingQuery{Method: pair.Method, URI: pair.Source.URI, Line: p.Position.Line, Character: p.Position.Character, ObservedEmpty: bytes.Equal(trimmed, []byte("[]")) || bytes.Equal(trimmed, []byte("null"))})
	w.ownedMethods.pairs = append(w.ownedMethods.pairs, pair)
	return true
}
