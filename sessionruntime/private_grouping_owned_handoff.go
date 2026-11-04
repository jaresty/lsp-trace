package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/adr0011querytarget"
)

var ErrPrivateGroupingOwned = errors.New("private grouping owned handoff refused")

// PrivateGroupingOwnedRequest selects methods, not authority. Revision remains a
// caller assertion. This private entry neither starts a session nor grants one.
type PrivateGroupingOwnedRequest struct {
	Session                              SessionLeaseRequest
	URI, OccurrenceID, Revision, Privacy string
	Line, Character                      uint32
	References, Definitions              bool
	Deadline                             time.Time
}

// PrivateGroupingOwnedHandoff cannot be constructed from mutable pair snapshots.
// Its lifetime is strictly the currently active Manager-owned session lease.
type PrivateGroupingOwnedHandoff struct{ capture *privateGroupingCapture }
type privateGroupingCapture struct {
	manager  *Manager
	token    *sessionLeaseToken
	request  PrivateGroupingOwnedRequest
	view     PrivateGroupingOwnedView
	consumed bool
}

// PrivateGroupingOwnedView is a borrowed diagnostic copy, not an issuance token.
// No API accepts this value as proof that a transaction ran.
type PrivateGroupingOwnedView struct {
	Query             adr0011querytarget.Query
	Target            adr0011querytarget.Candidate
	Source            []byte
	Supply            DocumentSupply
	Metadata          SessionMetadata
	Revision, Privacy string
	Workspace         string
	Deadline          time.Time
	Symbol            OwnedMethodPair
	Methods           []OwnedMethodPair
	Unavailable       []string
}

// CapturePrivateGroupingOwned requires an existing exact active lease. It uses
// the ordinary bounded native transaction machinery, without enabling C16/C17
// public flags or accepting a caller-owned result as custody.
func (m *Manager) CapturePrivateGroupingOwned(ctx context.Context, req PrivateGroupingOwnedRequest) (PrivateGroupingOwnedHandoff, error) {
	fail := func() (PrivateGroupingOwnedHandoff, error) {
		return PrivateGroupingOwnedHandoff{}, ErrPrivateGroupingOwned
	}
	if !privateGroupingOwnedRequestFieldsValid(req) {
		return fail()
	}
	if m == nil || ctx == nil || req.URI == "" || req.OccurrenceID == "" || req.Revision == "" || strings.TrimSpace(req.Revision) != req.Revision || req.Privacy == "" || (!req.References && !req.Definitions) || req.Deadline.IsZero() || !time.Now().Before(req.Deadline) || req.Deadline.After(time.Now().Add(15*time.Second)) {
		return fail()
	}
	if f := m.CheckActiveSessionLeasePrivateV2(ctx, req.Session); f != "" {
		return fail()
	}
	metadata, f := m.Metadata(req.Session.SessionID, req.Session.Generation)
	if f != "" || !metadata.DocumentSymbolSupport || metadata.ProviderName == "" || metadata.ProviderVersion == "" || (metadata.PositionEncoding != "utf-8" && metadata.PositionEncoding != "utf-16" && metadata.PositionEncoding != "utf-32") {
		return fail()
	}
	// Empty language delegates to the existing managed document resolver; it is
	// not a Go/profile restriction and does not infer any semantic relation.
	language, _ := m.SessionLanguageID(req.Session.SessionID, req.Session.Generation)
	for _, field := range []string{metadata.PositionEncoding, metadata.ProviderName, metadata.ProviderVersion, metadata.ServerCommand, language} {
		if field != "" && !PrivateGroupingFieldValid(field) {
			return fail()
		}
	}
	d := m.PrepareDocument(ctx, DocumentRequest{SessionID: req.Session.SessionID, Generation: req.Session.Generation, URI: req.URI, LanguageID: language, CaptureSupply: true, PreparedNoFollow: true})
	if d.Failure != "" || d.Supply == nil || d.Version <= 0 || d.Supply.URI != req.URI || d.Supply.DocumentVersion != d.Version || d.Supply.SessionID != req.Session.SessionID || d.Supply.Generation != req.Session.Generation {
		return PrivateGroupingOwnedHandoff{}, fmt.Errorf("%w: document preparation: %s", ErrPrivateGroupingOwned, d.Failure)
	}
	// Charge actual supplied payload lengths before this adapter makes derived
	// source copies or sends semantic requests. PrepareDocument retains its own
	// existing source-read/supply bound; no physical RAM claim is made here.
	preflight := PrivateGroupingOwnedView{Source: d.Supply.Content, Supply: *d.Supply, Metadata: metadata, Revision: req.Revision, Privacy: req.Privacy, Workspace: req.Session.Workspace}
	if _, ok := PrivateGroupingQueryEncodedBound(preflight); !ok {
		return fail()
	}
	source := append([]byte(nil), d.Supply.Content...)
	binding := OwnedDocumentBinding{URI: req.URI, Version: d.Version, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(source))}
	query := adr0011querytarget.Query{OccurrenceID: req.OccurrenceID, URI: req.URI, Encoding: metadata.PositionEncoding, DocumentVersion: strconv.Itoa(d.Version), SourceDigest: binding.SHA256, SessionID: req.Session.SessionID, Generation: req.Session.Generation, Line: req.Line, Character: req.Character}
	transact := func(method string, params []byte) (OwnedMethodPair, error) {
		wire := RoundTripRequest{SessionID: req.Session.SessionID, Generation: req.Session.Generation, Method: method, Params: params, Deadline: req.Deadline, MaxMessages: 4, MaxBytes: 65536, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding, ADR0011PrivateP2: method != "textDocument/documentSymbol"}
		result := m.RoundTrip(ctx, wire)
		if ctx.Err() != nil || result.Failure != "" || result.ServerError != nil || result.ownedMethodPair == nil {
			return OwnedMethodPair{}, ErrPrivateGroupingOwned
		}
		// Read the retained private pair, not the caller-visible Result snapshot.
		pair, ok := result.CompletedOwnedMethodPair()
		if !ok || !validOwnedMethodPair(wire, result, pair.Result) || pair.Source == nil || *pair.Source != binding {
			return OwnedMethodPair{}, ErrPrivateGroupingOwned
		}
		return pair, nil
	}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": req.URI}})
	symbol, err := transact("textDocument/documentSymbol", params)
	if err != nil {
		return PrivateGroupingOwnedHandoff{}, fmt.Errorf("%w: documentSymbol transaction", err)
	}
	target, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(query, symbol.Result)
	if err != nil {
		return PrivateGroupingOwnedHandoff{}, fmt.Errorf("%w: canonical query target", err)
	}
	view := PrivateGroupingOwnedView{Query: query, Target: target, Source: source, Supply: *d.Supply, Metadata: metadata, Revision: req.Revision, Privacy: req.Privacy, Workspace: req.Session.Workspace, Deadline: req.Deadline, Symbol: symbol}
	if _, ok := PrivateGroupingQueryEncodedBound(view); !ok {
		return fail()
	}
	view.Supply.Content = append([]byte(nil), d.Supply.Content...)
	view.Supply.Params = append(json.RawMessage(nil), d.Supply.Params...)
	for _, method := range []struct {
		name                string
		selected, available bool
	}{{"textDocument/references", req.References, metadata.ReferencesSupport}, {"textDocument/definition", req.Definitions, metadata.DefinitionSupport}} {
		if !method.selected {
			continue
		}
		if !method.available {
			view.Unavailable = append(view.Unavailable, method.name)
			continue
		}
		fields := map[string]any{"textDocument": map[string]string{"uri": req.URI}, "position": map[string]uint32{"line": req.Line, "character": req.Character}}
		if method.name == "textDocument/references" {
			fields["context"] = map[string]bool{"includeDeclaration": false}
		}
		params, _ = json.Marshal(fields)
		pair, err := transact(method.name, params)
		if err != nil {
			return fail()
		}
		view.Methods = append(view.Methods, pair)
	}
	if f := m.CheckActiveSessionLeasePrivateV2(ctx, req.Session); f != "" {
		return fail()
	}
	if _, ok := PrivateGroupingQueryEncodedBound(view); !ok {
		return fail()
	}
	token, _ := ctx.Value(sessionLeaseContextKey{}).(*sessionLeaseToken)
	return PrivateGroupingOwnedHandoff{&privateGroupingCapture{manager: m, token: token, request: req, view: view}}, nil
}

// ConsumePrivateGroupingOwned atomically retires the opaque handle before its
// callback. Protocol ownership prevents source replacement during issuance;
// cancellation/lifecycle invalidation still prevents successful return. Failure
// never refunds or revives the handle. No hard CPU interruption is claimed.
func (m *Manager) ConsumePrivateGroupingOwned(ctx context.Context, h PrivateGroupingOwnedHandoff, consume func(PrivateGroupingOwnedView) error) error {
	if m == nil || ctx == nil || h.capture == nil || consume == nil {
		return ErrPrivateGroupingOwned
	}
	c := h.capture
	m.mu.Lock()
	r := m.sessions[c.request.Session.SessionID]
	token, _ := ctx.Value(sessionLeaseContextKey{}).(*sessionLeaseToken)
	valid := c.manager == m && !c.consumed && c.token != nil && token == c.token && r != nil && r.record.Generation == c.request.Session.Generation && m.validSessionLeaseLocked(ctx, r) && !r.protocolOwned && !r.lifecycleOwned && ctx.Err() == nil && time.Now().Before(c.request.Deadline)
	if valid {
		_, valid = PrivateGroupingQueryEncodedBound(c.view)
	}
	if valid {
		for _, pair := range append([]OwnedMethodPair{c.view.Symbol}, c.view.Methods...) {
			req := RoundTripRequest{SessionID: pair.SessionID, Generation: pair.Generation, Method: pair.Method, Params: pair.Params, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: pair.Source, MaxMessages: 4, MaxBytes: 65536}
			if !matchesOwnedDocument(req, r.documents) {
				valid = false
				break
			}
		}
	}
	if !valid {
		m.mu.Unlock()
		return ErrPrivateGroupingOwned
	}
	c.consumed = true
	r.protocolOwned = true
	m.mu.Unlock()
	defer func() { m.mu.Lock(); r.protocolOwned = false; m.mu.Unlock() }()
	// The consumer gets independent copies, never the retained backing slices.
	view := c.view
	view.Source = append([]byte(nil), view.Source...)
	view.Supply.Content = append([]byte(nil), view.Supply.Content...)
	view.Supply.Params = append(json.RawMessage(nil), view.Supply.Params...)
	view.Symbol = clonePrivateGroupingPair(view.Symbol)
	view.Unavailable = append([]string(nil), view.Unavailable...)
	view.Methods = make([]OwnedMethodPair, len(c.view.Methods))
	for i, p := range c.view.Methods {
		view.Methods[i] = clonePrivateGroupingPair(p)
	}
	if err := consume(view); err != nil {
		return err
	}
	m.mu.Lock()
	live := ctx.Err() == nil && m.validSessionLeaseLocked(ctx, r) && !m.closed && time.Now().Before(c.request.Deadline)
	m.mu.Unlock()
	if !live {
		return ErrPrivateGroupingOwned
	}
	return nil
}

// Fixed private logical encoding contract, not a campaign grant or RAM bound.
const PrivateGroupingMaxFieldBytes = 4096
const PrivateGroupingMaxKinds = 3
const PrivateGroupingQueryMaxBytes uint64 = 1500000

func PrivateGroupingFieldValid(s string) bool {
	return len(s) > 0 && len(s) <= PrivateGroupingMaxFieldBytes && utf8.ValidString(s)
}
func privateGroupingOwnedRequestFieldsValid(req PrivateGroupingOwnedRequest) bool {
	for _, s := range []string{req.Session.SessionID, req.Session.Workspace, req.URI, req.OccurrenceID, req.Revision, req.Privacy} {
		if !PrivateGroupingFieldValid(s) {
			return false
		}
	}
	return req.Session.Generation != 0
}

// PrivateGroupingCheckedEncodedSize is the arithmetic primitive for this one
// private encoding contract. Reject before multiplying or adding; overflow and
// exceeding the fixed ceiling have the same fail-closed outcome.
func PrivateGroupingCheckedEncodedSize(total, n, multiplier, extra uint64) (uint64, bool) {
	if total > PrivateGroupingQueryMaxBytes || extra > PrivateGroupingQueryMaxBytes-total {
		return 0, false
	}
	remaining := PrivateGroupingQueryMaxBytes - total - extra
	if multiplier != 0 && n > remaining/multiplier {
		return 0, false
	}
	return total + n*multiplier + extra, true
}
func privateGroupingEncodedString(total uint64, s string) (uint64, bool) {
	if s != "" && !PrivateGroupingFieldValid(s) {
		return 0, false
	}
	return PrivateGroupingCheckedEncodedSize(total, uint64(len(s)), 6, 2)
}
func privateGroupingEncodedBytes(total uint64, n int) (uint64, bool) {
	if n < 0 {
		return 0, false
	}
	groups := uint64(n) / 3
	if uint64(n)%3 != 0 {
		groups++
	}
	return PrivateGroupingCheckedEncodedSize(total, groups, 4, 4)
}

// PrivateGroupingQueryEncodedBound covers ONLY PrivateGroupingOwnedView's closed
// query encoding. 16 KiB covers fixed field names, separators, all scalar
// numbers/ranges/booleans and the deadline; all variable strings and byte fields
// are enumerated below. No payload copies, maps or marshal calls occur here.
func PrivateGroupingQueryEncodedBound(v PrivateGroupingOwnedView) (uint64, bool) {
	if len(v.Methods) > 2 || len(v.Unavailable) > 2 {
		return 0, false
	}
	total := uint64(16 << 10)
	var ok bool
	strings := []string{v.Query.OccurrenceID, v.Query.URI, v.Query.Encoding, v.Query.DocumentVersion, v.Query.SourceDigest, v.Query.SessionID,
		v.Target.Version, v.Target.ID, v.Target.SymbolID, v.Target.QueryOccurrenceID, v.Target.SymbolName, v.Target.QueryURI, v.Target.SessionID, v.Target.Encoding, v.Target.DocumentVersion, v.Target.SourceDigest, v.Target.DocumentSymbolResultSHA256,
		v.Supply.Classification, v.Supply.SessionID, v.Supply.URI, v.Supply.Method,
		v.Metadata.PositionEncoding, v.Metadata.ProviderName, v.Metadata.ProviderVersion, v.Metadata.ServerCommand, v.Revision, v.Privacy, v.Workspace, "CALLER_ASSERTED"}
	for _, s := range strings {
		total, ok = privateGroupingEncodedString(total, s)
		if !ok {
			return 0, false
		}
	}
	for _, s := range v.Unavailable {
		total, ok = privateGroupingEncodedString(total, s)
		if !ok {
			return 0, false
		}
	}
	for _, n := range []int{len(v.Source), len(v.Supply.Content), len(v.Supply.Params)} {
		total, ok = privateGroupingEncodedBytes(total, n)
		if !ok {
			return 0, false
		}
	}
	pairBound := func(p OwnedMethodPair) bool {
		for _, s := range []string{p.Method, p.SessionID, p.Write.SessionID, p.Write.Method, p.Write.FrameSHA256, p.Read.SessionID, p.Read.FrameSHA256} {
			total, ok = privateGroupingEncodedString(total, s)
			if !ok {
				return false
			}
		}
		if p.Source != nil {
			for _, s := range []string{p.Source.URI, p.Source.SHA256} {
				total, ok = privateGroupingEncodedString(total, s)
				if !ok {
					return false
				}
			}
		}
		for _, n := range []int{len(p.Params), len(p.Result)} {
			total, ok = privateGroupingEncodedBytes(total, n)
			if !ok {
				return false
			}
		}
		return true
	}
	if !pairBound(v.Symbol) {
		return 0, false
	}
	for _, p := range v.Methods {
		if !pairBound(p) {
			return 0, false
		}
	}
	return total, true
}

func clonePrivateGroupingPair(p OwnedMethodPair) OwnedMethodPair {
	p.Params = append(json.RawMessage(nil), p.Params...)
	p.Result = append(json.RawMessage(nil), p.Result...)
	if p.Source != nil {
		s := *p.Source
		p.Source = &s
	}
	return p
}
