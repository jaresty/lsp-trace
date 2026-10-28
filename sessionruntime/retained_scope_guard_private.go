package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/session"
	"lsp-trace/internal/strictjson"
)

// RetainedScopeRolePrivate labels the fixed private pilot inventory. Neither role
// grants semantic admission, endpoint custody, or occurrence authority.
type RetainedScopeRolePrivate string

const (
	RetainedScopeNominationPrivate  RetainedScopeRolePrivate = "NOMINATION"
	RetainedScopeContextOnlyPrivate RetainedScopeRolePrivate = "CONTEXT_ONLY"
)

// RetainedScopeMemberPrivate carries caller expectations only. Manager never
// opens or supplies a document using Expected: CaptureSupply must precede bind.
type RetainedScopeMemberPrivate struct {
	URI        string
	LanguageID string
	Expected   []byte
	Role       RetainedScopeRolePrivate
}

// RetainedScopeGuardPrivate is an opaque Manager-minted capability. Copying a Go
// pointer preserves it; copying the pointed-to struct creates an invalid token.
// It has no wire representation, trust flag, or caller-controlled enable switch.
type RetainedScopeGuardPrivate struct {
	manager            *Manager
	session            *runtimeSession
	generation         uint64
	leaseNonce         uint64
	leaseToken         *sessionLeaseToken
	members            []RetainedScopeMemberPrivate
	documents          map[string]openDocument
	diagnostics        []RetainedScopeDiagnosticPrivate
	diagnosticOverflow bool
	diagnosticOrdinal  uint64
	diagnosticBytes    int
}

// MarshalJSON deliberately refuses capability serialization, including a zero
// value. RoundTripRequest excludes the attachment independently.
func (*RetainedScopeGuardPrivate) MarshalJSON() ([]byte, error) {
	return nil, errors.New("private retained scope capability is not serializable")
}

// MembersPrivate returns detached expectations, never Manager-owned byte slices.
func (g *RetainedScopeGuardPrivate) MembersPrivate() []RetainedScopeMemberPrivate {
	if g == nil {
		return nil
	}
	return cloneRetainedScopeMembersPrivate(g.members)
}
func cloneRetainedScopeMembersPrivate(members []RetainedScopeMemberPrivate) []RetainedScopeMemberPrivate {
	out := append([]RetainedScopeMemberPrivate(nil), members...)
	for i := range out {
		out[i].Expected = append([]byte(nil), out[i].Expected...)
	}
	return out
}
func canonicalRetainedScopeURIPrivate(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || strings.IndexByte(u.Path, 0) >= 0 {
		return false
	}
	path := filepath.Clean(filepath.FromSlash(u.Path))
	return filepath.IsAbs(path) && raw == (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}
func cloneRetainedScopeDocumentPrivate(d openDocument) openDocument {
	if d.supply != nil {
		s := *d.supply
		s.Content = append([]byte(nil), s.Content...)
		s.Params = append(json.RawMessage(nil), s.Params...)
		d.supply = &s
	}
	return d
}
func validRetainedScopeDocumentPrivate(d openDocument, member RetainedScopeMemberPrivate, id string, generation uint64) bool {
	s := d.supply
	if s == nil || d.version < 1 || d.languageID != member.LanguageID || s.Classification != "LSP_SUPPLIED" || s.SessionID != id || s.Generation != generation || s.URI != member.URI || s.DocumentVersion != d.version || (s.Method != "textDocument/didOpen" && s.Method != "textDocument/didChange") || !bytes.Equal(s.Content, member.Expected) || sha256.Sum256(s.Content) != d.digest {
		return false
	}
	// Supply has no language field: independently retained openDocument language
	// is authoritative; didOpen also records it in the exact notification params.
	if strictjson.RejectDuplicates(s.Params) != nil || !retainedScopeSupplyKeysPrivate(s) {
		return false
	}
	var p struct {
		TextDocument struct {
			URI        string  `json:"uri"`
			LanguageID string  `json:"languageId"`
			Version    int     `json:"version"`
			Text       *string `json:"text"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text *string `json:"text"`
		} `json:"contentChanges"`
	}
	if json.Unmarshal(s.Params, &p) != nil || p.TextDocument.URI != member.URI || p.TextDocument.Version != d.version {
		return false
	}
	if s.Method == "textDocument/didOpen" {
		return p.TextDocument.LanguageID == member.LanguageID && p.TextDocument.Text != nil && bytes.Equal([]byte(*p.TextDocument.Text), s.Content)
	}
	return len(p.ContentChanges) == 1 && p.ContentChanges[0].Text != nil && bytes.Equal([]byte(*p.ContentChanges[0].Text), s.Content)
}

// BindRetainedScopePrivate activates an irreversible-for-this-generation private
// equality boundary, not proof that a server analyzed these bytes. Exactly four
// nominations and two context-only members are required; no generic scope API.
func (m *Manager) BindRetainedScopePrivate(ctx context.Context, id string, generation uint64, members []RetainedScopeMemberPrivate) (*RetainedScopeGuardPrivate, session.Failure) {
	if ctx == nil {
		return nil, session.LifecycleConflict
	}
	if failure := contextFailure(ctx); failure != "" {
		return nil, failure
	}
	if len(members) != 6 {
		return nil, DocumentSupplyUnavailable
	}
	// Bound the snapshot before copying caller expectations. All identity and
	// role validation below uses this detached snapshot under Manager.mu.
	total := 0
	for _, member := range members {
		if len(member.Expected) > 2<<20 {
			return nil, DocumentSupplyUnavailable
		}
		total += len(member.Expected)
	}
	if total > 8<<20 {
		return nil, DocumentSupplyUnavailable
	}
	members = cloneRetainedScopeMembersPrivate(members)
	m.mu.Lock()
	defer m.mu.Unlock()
	if failure := contextFailure(ctx); failure != "" {
		return nil, failure
	}
	r := m.sessions[id]
	if r == nil {
		return nil, session.SessionNotFound
	}
	if generation != r.record.Generation {
		return nil, session.StaleGeneration
	}
	if m.closed || r.record.State != session.Ready || r.protocolOwned || r.lifecycleOwned || r.leaseClosing || !m.validSessionLeaseLocked(ctx, r) {
		return nil, session.LifecycleConflict
	}
	if r.retainedScopePrivate != nil || r.retainedScopePreparing != 0 {
		return nil, DocumentSupplyUnavailable
	}
	nominations, contexts := 0, 0
	seen := make(map[string]bool, 6)
	for _, member := range members {
		if !canonicalRetainedScopeURIPrivate(member.URI) || seen[member.URI] || member.LanguageID == "" || strings.TrimSpace(member.LanguageID) != member.LanguageID || !utf8.ValidString(member.LanguageID) || len(member.LanguageID) > 256 || !utf8.Valid(member.Expected) {
			return nil, DocumentSupplyUnavailable
		}
		seen[member.URI] = true
		switch member.Role {
		case RetainedScopeNominationPrivate:
			nominations++
		case RetainedScopeContextOnlyPrivate:
			contexts++
		default:
			return nil, DocumentSupplyUnavailable
		}
	}
	if nominations != 4 || contexts != 2 {
		return nil, DocumentSupplyUnavailable
	}
	g := &RetainedScopeGuardPrivate{manager: m, session: r, generation: generation, leaseNonce: r.leaseNonce, leaseToken: r.leaseToken, members: members, documents: make(map[string]openDocument, 6)}
	for _, member := range members {
		d, ok := r.documents[member.URI]
		if !ok || !validRetainedScopeDocumentPrivate(d, member, id, generation) {
			return nil, DocumentSupplyUnavailable
		}
		g.documents[member.URI] = cloneRetainedScopeDocumentPrivate(d)
	}
	r.retainedScopePrivate = g
	return g, ""
}

// CheckRetainedScopePrivate checks a bound capability without filesystem or wire
// work. It grants no custody and is used by the fixed private frozen supplier.
func (m *Manager) CheckRetainedScopePrivate(ctx context.Context, id string, generation uint64, g *RetainedScopeGuardPrivate) session.Failure {
	if ctx == nil {
		return session.LifecycleConflict
	}
	if f := contextFailure(ctx); f != "" {
		return f
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if f := contextFailure(ctx); f != "" {
		return f
	}
	r := m.sessions[id]
	if r == nil {
		return session.SessionNotFound
	}
	if generation != r.record.Generation {
		return session.StaleGeneration
	}
	if r.record.State == session.Poisoned {
		return session.SessionPoisoned
	}
	if m.closed || r.record.State != session.Ready || r.protocolOwned || r.lifecycleOwned || r.leaseClosing || !m.validSessionLeaseLocked(ctx, r) {
		return session.LifecycleConflict
	}
	if g == nil || len(g.members) != 6 {
		return DocumentSupplyUnavailable
	}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": g.members[0].URI}})
	req := RoundTripRequest{SessionID: id, Generation: generation, Method: "textDocument/documentSymbol", Params: params, RetainedScopePrivate: g}
	reason, class := m.retainedScopeMatchReasonPrivate(ctx, req, r)
	if reason != "ACCEPTED" {
		m.observeRetainedScopePrivate(g, r, "CHECK", "NONE", class, reason)
		return DocumentSupplyUnavailable
	}
	return ""
}

func retainedScopeMethodPrivate(method string) bool {
	switch method {
	case "textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
		return true
	}
	return false
}

const maxRetainedScopeParamsPrivate = 2 << 20

// This allocation-free gate must run before copying even an unrecognized token's
// Params. Identity validation still belongs to the locked admission check.
func eligibleRetainedScopeRequestPrivate(req RoundTripRequest) bool {
	return retainedScopeMethodPrivate(req.Method) && len(req.Params) > 0 && len(req.Params) <= maxRetainedScopeParamsPrivate
}

// Decode only canonical DTO keys, not encoding/json's case-insensitive struct
// field matching. Reject both alias-only and alias-plus-exact spellings (including
// Unicode case-fold lookalikes) rather than selecting one interpretation. Callers
// reject duplicate decoded JSON keys once for the complete input before this.
func retainedScopeObjectKeysPrivate(raw json.RawMessage, allowed ...string) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, false
	}
	for key := range object {
		exact := false
		for _, name := range allowed {
			if key == name {
				exact = true
				break
			}
		}
		if !exact {
			return nil, false
		}
	}
	return object, true
}

func retainedScopePositionKeysPrivate(raw json.RawMessage) bool {
	_, ok := retainedScopeObjectKeysPrivate(raw, "line", "character")
	return ok
}
func retainedScopeRangeKeysPrivate(raw json.RawMessage) bool {
	rangeObject, ok := retainedScopeObjectKeysPrivate(raw, "start", "end")
	if !ok {
		return false
	}
	for _, position := range rangeObject {
		if !retainedScopePositionKeysPrivate(position) {
			return false
		}
	}
	return true
}

func retainedScopeSupplyKeysPrivate(s *DocumentSupply) bool {
	rootKeys := []string{"textDocument"}
	documentKeys := []string{"uri", "languageId", "version", "text"}
	if s.Method == "textDocument/didChange" {
		rootKeys = append(rootKeys, "contentChanges")
		documentKeys = []string{"uri", "version"}
	}
	root, ok := retainedScopeObjectKeysPrivate(s.Params, rootKeys...)
	if !ok {
		return false
	}
	if _, ok := retainedScopeObjectKeysPrivate(root["textDocument"], documentKeys...); !ok {
		return false
	}
	if s.Method == "textDocument/didChange" {
		var changes []json.RawMessage
		if json.Unmarshal(root["contentChanges"], &changes) != nil || len(changes) != 1 {
			return false
		}
		// Manager supplies one full-text change, never caller-authored edits.
		if _, ok := retainedScopeObjectKeysPrivate(changes[0], "text"); !ok {
			return false
		}
	}
	return true
}

func retainedScopeRequestURIPrivate(req RoundTripRequest) (string, bool) {
	if !eligibleRetainedScopeRequestPrivate(req) || strictjson.RejectDuplicates(req.Params) != nil {
		return "", false
	}
	key := "textDocument"
	rootKeys := []string{"textDocument", "workDoneToken", "partialResultToken"}
	documentKeys := []string{"uri"}
	switch req.Method {
	case "textDocument/prepareCallHierarchy":
		rootKeys = []string{"textDocument", "position", "workDoneToken"}
	case "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
		key = "item"
		rootKeys = []string{"item", "workDoneToken", "partialResultToken"}
		documentKeys = []string{"name", "kind", "tags", "detail", "uri", "range", "selectionRange", "data"}
	}
	root, ok := retainedScopeObjectKeysPrivate(req.Params, rootKeys...)
	if !ok {
		return "", false
	}
	inner, ok := retainedScopeObjectKeysPrivate(root[key], documentKeys...)
	if !ok {
		return "", false
	}
	if position, present := root["position"]; present && !retainedScopePositionKeysPrivate(position) {
		return "", false
	}
	for _, name := range []string{"range", "selectionRange"} {
		if raw, present := inner[name]; present && !retainedScopeRangeKeysPrivate(raw) {
			return "", false
		}
	}
	// item.data is deliberately opaque. Do not reinterpret keys inside it as
	// protocol DTO fields, and never rebuild the item or drop its retained data.
	var uri string
	if json.Unmarshal(inner["uri"], &uri) != nil || !canonicalRetainedScopeURIPrivate(uri) {
		return "", false
	}
	return uri, true
}

// Called with Manager.mu held, before any pending request or write allocation.
func (m *Manager) matchesRetainedScopePrivate(ctx context.Context, req RoundTripRequest, r *runtimeSession) bool {
	reason, class := m.retainedScopeMatchReasonPrivate(ctx, req, r)
	method := req.Method
	if !retainedScopeMethodPrivate(method) {
		method = "OTHER"
	}
	m.observeRetainedScopePrivate(req.RetainedScopePrivate, r, "ADMISSION", method, class, reason)
	return reason == "ACCEPTED"
}

// Called only with Manager.mu held. Keep the latest bounded observations so a
// late refusal cannot disappear behind successful requests. Overflow discloses
// discarded prefix events. Successful Check calls are not attempted requests.
func (m *Manager) observeRetainedScopePrivate(g *RetainedScopeGuardPrivate, r *runtimeSession, phase, method, class, reason string) {
	if g == nil || r.retainedScopePrivate != g || g.manager != m || g.session != r {
		return
	}
	g.diagnosticOrdinal++
	event := RetainedScopeDiagnosticPrivate{phase, method, g.diagnosticOrdinal, class, reason}
	encoded, _ := json.Marshal(event)
	for len(g.diagnostics) > 0 && (len(g.diagnostics) >= 64 || g.diagnosticBytes+len(encoded)+1 > 8000) {
		old, _ := json.Marshal(g.diagnostics[0])
		g.diagnosticBytes -= len(old) + 1
		g.diagnostics = g.diagnostics[1:]
		g.diagnosticOverflow = true
	}
	g.diagnostics = append(g.diagnostics, event)
	g.diagnosticBytes += len(encoded) + 1
}

func (m *Manager) retainedScopeMatchReasonPrivate(ctx context.Context, req RoundTripRequest, r *runtimeSession) (string, string) {
	g := req.RetainedScopePrivate
	if g == nil {
		if r.retainedScopePrivate == nil || !retainedScopeMethodPrivate(req.Method) {
			return "ACCEPTED", "INVALID"
		}
		return "CAPABILITY_MISSING", "INVALID"
	}
	if r.retainedScopePrivate != g || g.manager != m || g.session != r || g.generation != req.Generation || g.leaseNonce != r.leaseNonce || g.leaseToken != r.leaseToken || r.lifecycleOwned || r.leaseClosing || m.closed || !m.validSessionLeaseLocked(ctx, r) {
		return "CAPABILITY_MISMATCH", "INVALID"
	}
	uri, ok := retainedScopeRequestURIPrivate(req)
	if !ok {
		return "REQUEST_DTO_INVALID", "INVALID"
	}
	if _, ok = g.documents[uri]; !ok {
		return "URI_OUTSIDE_FROZEN", "FROZEN_EXTERNAL"
	}
	class := "CONTEXT"
	for _, member := range g.members {
		if member.URI == uri && member.Role == RetainedScopeNominationPrivate {
			class = "NOMINATION"
		}
	}
	for _, member := range g.members {
		current, ok := r.documents[member.URI]
		bound := g.documents[member.URI]
		if !ok || !validRetainedScopeDocumentPrivate(current, member, req.SessionID, req.Generation) || current.version != bound.version || current.languageID != bound.languageID || current.digest != bound.digest || current.supply.Method != bound.supply.Method || !bytes.Equal(current.supply.Params, bound.supply.Params) || !bytes.Equal(current.supply.Content, bound.supply.Content) {
			return "FROZEN_STATE_MISMATCH", class
		}
	}
	return "ACCEPTED", class
}
