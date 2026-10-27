package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/session"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

// Fixed qualification policy, independent of caller/public acquisition limits.
const scopeAdapterParams = 2 << 20
const scopeAdapterResponse = 2 << 20

// Independent fixed JSON-RPC envelope allowance, not extra result capacity.
const scopeAdapterResponseEnvelope = 1 << 10
const scopeAdapterMessages = 64
const scopeAdapterRequests = 256
const scopeAdapterCapture = 8 << 20
const scopeAdapterTimeout = 30 * time.Second
const scopeAdapterRequestTimeout = 5 * time.Second

// An observation of binding only. No wire representation, custody or receipt.
type scopeBindingObservation struct{ adapter *scopeRuntimeAdapter }

// Omission count only, not a completeness/custody receipt. Native callers must
// consult it rather than interpreting filtered symbols as closed source evidence.
func (b scopeBindingObservation) symbolOmissions() uint64 {
	if b.adapter == nil {
		return 0
	}
	b.adapter.mu.Lock()
	defer b.adapter.mu.Unlock()
	return b.adapter.omittedExternal
}

type scopeRuntimeAdapter struct {
	manager   *sessionruntime.Manager
	identity  censusacquisition.SessionIdentity
	guard     *sessionruntime.RetainedScopeGuardPrivate
	files     censusacquisition.StaticFiles
	workspace string
	metadata  sessionruntime.SessionMetadata
	deadline  time.Time
	// invocationMu spans Manager transport, accounting, and ALL typed validation.
	// No callback under it may reenter an adapter invocation.
	invocationMu             sync.Mutex
	mu                       sync.Mutex
	omittedExternal          uint64
	afterResponsePrivateTest func() // nil outside package-local barrier fixtures
	beforeInvokePrivateTest  func() // nil outside package-local lock-attempt fixtures
	requests, captured       int
	terminal                 error
}

// Constructor accepts only the read-only restart result, never raw expectations.
// The supplied context must remain inside the existing Manager lease lifetime.
func newScopeRuntimeAdapter(ctx context.Context, restart scopeRestartResult, manager *sessionruntime.Manager, identity censusacquisition.SessionIdentity) (*scopeRuntimeAdapter, scopeBindingObservation, error) {
	fail := func(err error) (*scopeRuntimeAdapter, scopeBindingObservation, error) {
		return nil, scopeBindingObservation{}, err
	}
	if ctx == nil || manager == nil || restart.state == nil || identity.Validate() != nil {
		return fail(ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	deadline := time.Now().Add(scopeAdapterTimeout)
	ctx, constructorCancel := context.WithDeadline(ctx, deadline)
	defer constructorCancel()
	// Revalidate opaque state through retained invariants before any preparation.
	// Check opaque size/invariant bounds before making detached copies.
	scope := restart.state.scope
	plan, err := deriveScopeRetentionPlan(scope, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
	if err != nil {
		return fail(err)
	}
	if err = validateScopeRetentionPlan(plan, scopeRetentionByteCeiling, scopeRetentionWorkCeiling); err != nil {
		return fail(err)
	}
	head, err := parseScopeBundleHead(restart.state.head)
	if err != nil || head.State != "RETAINED_INPUT_BUNDLE" || head.Index != len(plan.state.objects)+1 || head.Generation != uint64(2*head.Index) || !bytes.Equal(plan.metadataBytes(), restart.state.metadata) || len(restart.state.linked) != head.Index || head.Binding.Scope != Digest(scope.ManifestBytes()) || head.Binding.Metadata != Digest(restart.state.metadata) {
		return fail(ErrInvalid)
	}
	if err = validateScopeRestartForAdapter(ctx, restart, plan, head); err != nil {
		return fail(err)
	}
	var manifest scopeManifest
	if scopeBundleStrict(scope.ManifestBytes(), &manifest) != nil {
		return fail(ErrInvalid)
	}
	metadata, failure := manager.Metadata(identity.SessionID, identity.Generation)
	if failure != "" {
		return fail(scopeAdapterFailure(failure, ""))
	}
	a := &scopeRuntimeAdapter{manager: manager, identity: identity, workspace: manifest.Root, metadata: metadata, deadline: deadline}
	members := make([]sessionruntime.RetainedScopeMemberPrivate, 0, 6)
	supplyCtx, cancel := context.WithDeadline(ctx, a.deadline)
	defer cancel()
	for _, file := range scope.Files() {
		// Both independent returned supply and Manager-owned supply must agree. Bind
		// below rechecks Manager's independent copy under its own lock.
		result := manager.PrepareDocument(supplyCtx, sessionruntime.DocumentRequest{SessionID: identity.SessionID, Generation: identity.Generation, URI: file.URI, LanguageID: "go", CaptureSupply: true})
		if result.Failure != "" {
			return fail(scopeAdapterFailure(result.Failure, result.LifecycleSubcause))
		}
		if !scopeAdapterSupplyMatches(result, file, identity) {
			return fail(scopeAdapterFailure(sessionruntime.DocumentSupplyUnavailable, ""))
		}
		role := sessionruntime.RetainedScopeRolePrivate(file.Role)
		members = append(members, sessionruntime.RetainedScopeMemberPrivate{URI: file.URI, LanguageID: "go", Expected: file.Bytes, Role: role})
		if role == sessionruntime.RetainedScopeNominationPrivate {
			a.files = append(a.files, censusacquisition.SourceFile{Path: file.Path, URI: file.URI, LanguageID: "go"})
		}
	}
	a.guard, failure = manager.BindRetainedScopePrivate(supplyCtx, identity.SessionID, identity.Generation, members)
	if failure != "" {
		return fail(scopeAdapterFailure(failure, ""))
	}
	return a, scopeBindingObservation{a}, nil
}

// Revalidate the observed prefix entirely in memory. No root capability, write
// operation, filesystem lookup or receipt publication is installed in this seam.
func validateScopeRestartForAdapter(ctx context.Context, restart scopeRestartResult, plan scopeRetentionPlan, head scopeBundleHead) error {
	verifier := &scopeBundleOwner{plan: plan}
	receipts := make(map[string][]byte, len(restart.state.linked))
	for _, raw := range restart.state.linked {
		if len(raw) == 0 || len(raw) > scopeBundleWireLimit {
			return ErrInvalid
		}
		id := Digest(raw)
		if _, exists := receipts[id]; exists {
			return ErrInvalid
		}
		receipts[id] = raw
	}
	verifier.ops.getReceipt = func(c context.Context, _ *publication.ContentRetentionRoot, id string) ([]byte, error) {
		if err := c.Err(); err != nil {
			return nil, err
		}
		raw, ok := receipts[id]
		if !ok {
			return nil, ErrInvalid
		}
		return raw, nil
	}
	verifier.ops.get = func(c context.Context, _ *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
		if err := c.Err(); err != nil {
			return nil, err
		}
		for i := 0; i <= len(plan.state.objects); i++ {
			obj := verifier.item(i)
			if obj.digest == id && int64(len(obj.raw)) == n {
				return obj.raw, nil
			}
		}
		return nil, ErrInvalid
	}
	var observed [][]byte
	_, err := verifier.verifyPrefix(ctx, nil, nil, nil, head.Binding, head, restart.state.head, &observed)
	return err
}

func scopeAdapterSupplyMatches(r sessionruntime.DocumentResult, f scopeObservedFile, id censusacquisition.SessionIdentity) bool {
	s := r.Supply
	if r.URI != f.URI || r.LanguageID != "go" || r.Version < 1 || s == nil || s.Classification != "LSP_SUPPLIED" || s.URI != f.URI || s.SessionID != id.SessionID || s.Generation != id.Generation || s.DocumentVersion != r.Version || !bytes.Equal(s.Content, f.Bytes) || strictjson.RejectDuplicates(s.Params) != nil {
		return false
	}
	var p struct {
		TextDocument struct {
			URI        string  `json:"uri"`
			LanguageID string  `json:"languageId"`
			Version    int     `json:"version"`
			Text       *string `json:"text"`
		} `json:"textDocument"`
		Changes []struct {
			Text *string `json:"text"`
		} `json:"contentChanges"`
	}
	if json.Unmarshal(s.Params, &p) != nil || p.TextDocument.URI != f.URI || p.TextDocument.Version != r.Version {
		return false
	}
	switch s.Method {
	case "textDocument/didOpen":
		return p.TextDocument.LanguageID == "go" && p.TextDocument.Text != nil && bytes.Equal([]byte(*p.TextDocument.Text), f.Bytes)
	case "textDocument/didChange":
		return len(p.Changes) == 1 && p.Changes[0].Text != nil && bytes.Equal([]byte(*p.Changes[0].Text), f.Bytes)
	}
	return false
}

func scopeAdapterFailure(f session.Failure, sub sessionruntime.LifecycleSubcause) error {
	var category error
	switch f {
	case session.SessionPoisoned:
		category = censusacquisition.ErrSessionPoisoned
	case session.LifecycleConflict:
		category = censusacquisition.ErrLifecycleConflict
	case session.SessionNotFound, session.StaleGeneration:
		category = censusacquisition.ErrSessionUnavailable
	}
	runtimeError := scopeRuntimeFailurePrivate{Failure: f}
	if category == nil {
		return runtimeError
	}
	return &censusacquisition.DocumentSupplyFailure{Cause: errors.Join(runtimeError, category), LifecycleSubcause: censusacquisition.LifecycleSubcause(sub)}
}

// Supply is a frozen nomination check, never preparation, even on a cache hit.
func (a *scopeRuntimeAdapter) Supply(ctx context.Context, file censusacquisition.SourceFile) error {
	if a == nil || ctx == nil {
		return ErrInvalid
	}
	found := false
	for _, f := range a.files {
		if file == f {
			found = true
			break
		}
	}
	if !found {
		return ErrInvalid
	}
	return a.check(ctx)
}
func (a *scopeRuntimeAdapter) check(ctx context.Context) error {
	if a == nil || ctx == nil {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	terminal := a.terminal
	a.mu.Unlock()
	if terminal != nil {
		return terminal
	}
	if time.Now().After(a.deadline) {
		return context.DeadlineExceeded
	}
	if f := a.manager.CheckRetainedScopePrivate(ctx, a.identity.SessionID, a.identity.Generation, a.guard); f != "" {
		return scopeAdapterFailure(f, "")
	}
	return nil
}
func (a *scopeRuntimeAdapter) SupportsDocumentSymbols() bool { return a.metadata.DocumentSymbolSupport }
func (a *scopeRuntimeAdapter) SupportsCallHierarchy() bool   { return a.metadata.CallHierarchySupport }

type scopeDiscoveryOmissionKey struct{}

func (a *scopeRuntimeAdapter) Discover(ctx context.Context, id censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	if id != a.identity {
		return censusacquisition.Discovery{}, ErrInvalid
	}
	if err := a.check(ctx); err != nil {
		return censusacquisition.Discovery{}, err
	}
	ctx, cancel := context.WithDeadline(ctx, a.deadline)
	defer cancel()
	var invocationOmissions atomic.Uint64
	ctx = context.WithValue(ctx, scopeDiscoveryOmissionKey{}, &invocationOmissions)
	out, err := (censusacquisition.DiscoveryAdapter{Workspace: a.workspace, Files: append(censusacquisition.StaticFiles(nil), a.files...), Supplier: a, Client: a, Limits: censusacquisition.DiscoveryLimits{MaxFiles: 4, MaxSymbols: 128, MaxDepth: 64, MaxStringBytes: 1 << 20}}).Discover(ctx, id)
	omitted := invocationOmissions.Load()
	if omitted != 0 {
		out.Complete = false
		out.Closure = censusacquisition.DiscoveryClosureUnknown
		observation := censusacquisition.DiscoveryObservation{Dimension: censusacquisition.DiscoveryTargetSourceOmissions, Known: true, Omitted: &omitted}
		replaced := false
		for i := range out.Observations {
			if out.Observations[i].Dimension == observation.Dimension {
				out.Observations[i] = observation
				replaced = true
				break
			}
		}
		if !replaced {
			out.Observations = append(out.Observations, observation)
		}
	}
	return out, err
}

// All four methods share this Manager-owned guarded request path. No alternate
// ordinary/directional request can omit the attachment. No capture opt-in is set.
// invoke is the admission boundary for direct and native calls. Responses remain
// charged on decode failure, and any post-response error permanently latches.
func (a *scopeRuntimeAdapter) invoke(fn func() error) error {
	if a == nil {
		return ErrInvalid
	}
	if a.beforeInvokePrivateTest != nil {
		a.beforeInvokePrivateTest()
	}
	a.invocationMu.Lock()
	defer a.invocationMu.Unlock()
	a.mu.Lock()
	before, terminal := a.captured, a.terminal
	a.mu.Unlock()
	if terminal != nil {
		return terminal
	}
	err := fn()
	a.mu.Lock()
	if err != nil && a.captured != before && a.terminal == nil {
		a.terminal = err
	}
	a.mu.Unlock()
	return err
}

func (a *scopeRuntimeAdapter) roundTrip(ctx context.Context, method string, params json.RawMessage) (raw json.RawMessage, err error) {
	err = a.invoke(func() error { var e error; raw, e = a.roundTripLocked(ctx, method, params); return e })
	return raw, err
}

// Only invoke's callback may call this method; it must not recursively lock.
func (a *scopeRuntimeAdapter) roundTripLocked(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	bounds, err := a.wireBoundsLocked(ctx, scopeAdapterMessages, scopeAdapterWireBytes, scopeAdapterResponseEnvelope)
	if err != nil {
		return nil, err
	}
	return a.roundTripWithBoundsLocked(ctx, method, params, bounds)
}

// The effective wire limits are fixed before Manager admission. The enclosing
// invocation lock continues through every direct/native response validator.
func (a *scopeRuntimeAdapter) roundTripWithBoundsLocked(ctx context.Context, method string, params json.RawMessage, bounds scopeEffectiveWireBounds) (json.RawMessage, error) {
	if a == nil || ctx == nil {
		return nil, ErrInvalid
	}
	if _, requested := directionalraw.FromContext(ctx); requested && directionalraw.Allowed(method) {
		return nil, errors.New("directional custody unavailable for retained scope adapter")
	}
	if len(params) == 0 || len(params) > scopeAdapterParams || bounds.messages <= 0 || bounds.messages > scopeAdapterMessages || bounds.bytes < scopeMinimumResponseBody || bounds.bytes > scopeAdapterWireBytes || bounds.deadline.IsZero() {
		return nil, ErrInvalid
	}
	if err := a.check(ctx); err != nil {
		return nil, err
	}
	a.mu.Lock()
	if a.terminal != nil {
		err := a.terminal
		a.mu.Unlock()
		return nil, err
	}
	if a.requests >= scopeAdapterRequests || a.captured >= scopeAdapterCapture {
		a.mu.Unlock()
		return nil, ErrInvalid
	}
	a.requests++
	a.mu.Unlock()
	ctx, cancel := context.WithDeadline(ctx, bounds.deadline)
	defer cancel()
	r := a.manager.RoundTrip(ctx, sessionruntime.RoundTripRequest{SessionID: a.identity.SessionID, Generation: a.identity.Generation, RetainedScopePrivate: a.guard, Method: method, Params: params, Deadline: bounds.deadline, MaxMessages: bounds.messages, MaxBytes: int64(bounds.bytes)})
	if a.afterResponsePrivateTest != nil && r.Failure == "" {
		a.afterResponsePrivateTest()
	}
	var err error
	if r.Failure != "" {
		err = scopeAdapterFailure(r.Failure, "")
	} else if r.ServerError != nil {
		err = fmt.Errorf("private scope json-rpc error %d", r.ServerError.Code)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.terminal = err
		return nil, err
	}
	if len(r.Result) > scopeAdapterResponse || len(r.Result) > scopeAdapterCapture-a.captured {
		a.terminal = ErrInvalid
		return nil, ErrInvalid
	}
	a.captured += len(r.Result)
	// Validate before any custom decoder or native WireClient can normalize it.
	var p struct {
		TextDocument lsp.TextDocumentIdentifier `json:"textDocument"`
	}
	if method == "textDocument/documentSymbol" {
		_ = json.Unmarshal(params, &p)
	}
	_, omitted, validationErr := validateScopeResponse(method, r.Result, p.TextDocument.URI)
	if validationErr != nil {
		a.terminal = validationErr
		return nil, validationErr
	}
	a.omittedExternal += uint64(omitted)
	if counter, ok := ctx.Value(scopeDiscoveryOmissionKey{}).(*atomic.Uint64); ok {
		counter.Add(uint64(omitted))
	}
	return bytes.Clone(r.Result), nil
}
func (a *scopeRuntimeAdapter) call(ctx context.Context, method string, params, out any) (null bool, err error) {
	err = a.invoke(func() error { var e error; null, e = a.callLocked(ctx, method, params, out); return e })
	return null, err
}
func (a *scopeRuntimeAdapter) callLocked(ctx context.Context, method string, params, out any) (bool, error) {
	p, err := json.Marshal(params)
	if err != nil {
		return false, err
	}
	raw, err := a.roundTripLocked(ctx, method, p)
	if err != nil {
		return false, err
	}
	if strictjson.RejectDuplicates(raw) != nil {
		return false, ErrInvalid
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true, nil
	}
	if method == "textDocument/documentSymbol" {
		request := params.(lsp.DocumentSymbolParams)
		symbols, _, e := validateScopeResponse(method, raw, request.TextDocument.URI)
		if e != nil {
			return false, e
		}
		*out.(*[]lsp.DocumentSymbol) = symbols
		return false, nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return false, err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return false, ErrInvalid
	}
	return false, nil
}
func (a *scopeRuntimeAdapter) DocumentSymbols(ctx context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	var out []lsp.DocumentSymbol
	_, err := a.call(ctx, "textDocument/documentSymbol", p, &out)
	return out, err
}
func (a *scopeRuntimeAdapter) PrepareCallHierarchy(ctx context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	var out []lsp.CallHierarchyItem
	_, err := a.call(ctx, "textDocument/prepareCallHierarchy", p, &out)
	return out, err
}
func (a *scopeRuntimeAdapter) IncomingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	var out []lsp.CallHierarchyIncomingCall
	null, err := a.call(ctx, "callHierarchy/incomingCalls", lsp.CallHierarchyIncomingCallsParams{Item: item}, &out)
	return out, null, err
}
func (a *scopeRuntimeAdapter) OutgoingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	var out []lsp.CallHierarchyOutgoingCall
	null, err := a.call(ctx, "callHierarchy/outgoingCalls", lsp.CallHierarchyOutgoingCallsParams{Item: item}, &out)
	return out, null, err
}

// Native acquisition defaults to guarded ordinary transport. Directional custody
// remains unavailable unless the package-private native opt-in is selected.
func (a *scopeRuntimeAdapter) nativeClient() acquisition.Client {
	return a.nativeClientPrivate(false)
}

// Explicit package-local opt-in only. The acquisition producer owns the real
// reservation in ctx and its terminal release; this adapter never creates one.
func (a *scopeRuntimeAdapter) nativeDirectionalClientPrivate() acquisition.Client {
	return a.nativeClientPrivate(true)
}

func (a *scopeRuntimeAdapter) nativeSupplyPrivate(ctx context.Context, id acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
	if id.SessionID != a.identity.SessionID || id.Generation != a.identity.Generation {
		return acquisition.Supply{}, ErrInvalid
	}
	for _, f := range a.files {
		if f.URI == l.URI && (l.LanguageID == "" || l.LanguageID == f.LanguageID) {
			if err := a.Supply(ctx, f); err != nil {
				return acquisition.Supply{}, err
			}
			return acquisition.Supply{URI: f.URI, LanguageID: f.LanguageID}, nil
		}
	}
	return acquisition.Supply{}, ErrInvalid
}

func (a *scopeRuntimeAdapter) nativeClientPrivate(directional bool) acquisition.Client {
	// This per-client normalization cache is only accessed under invocationMu.
	// It preserves all supported symbol fields after WireClient has independently
	// completed its own strict validation, without invoking lsp's custom decoder.
	var normalizedSymbols []lsp.DocumentSymbol
	c := acquisition.NewWireClient(func(ctx context.Context, r acquisition.WireRequest) (json.RawMessage, error) {
		raw, err := a.nativeWireRoundTripLocked(ctx, r)
		if err == nil && r.Method == "textDocument/documentSymbol" {
			var p lsp.DocumentSymbolParams
			if json.Unmarshal(r.Params, &p) != nil {
				return nil, ErrInvalid
			}
			normalizedSymbols, _, err = validateScopeResponse(r.Method, raw, p.TextDocument.URI)
		}
		return raw, err
	})
	if directional {
		c.SetDirectionalPrivate(a.nativeDirectionalRoundTripLocked)
	}
	return acquisition.WithDocumentSupply(scopeNativeClient{adapter: a, wire: c, symbols: func() []lsp.DocumentSymbol { return normalizedSymbols }}, a.nativeSupplyPrivate)
}
