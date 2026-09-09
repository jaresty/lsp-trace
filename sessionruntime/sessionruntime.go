// Package sessionruntime provides transport-neutral orchestration over the
// session algebra, resolved runtime identity, managed processes, and LSP wire.
// It neither registers nor advertises a command surface.
package sessionruntime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/session"
	"lsp-trace/internal/source"
)

type Limits struct {
	MaxSessions, MaxRequests, MaxChildren, MaxCancels, MaxTombstones, MaxObservations int
	// MaxOperations bounds retained lifecycle operation records. Zero defaults to MaxObservations.
	MaxOperations int
}

type OperationState string

const (
	OperationPending  OperationState = "PENDING"
	OperationComplete OperationState = "COMPLETE"
	OperationFailed   OperationState = "FAILED"
)

type OperationSnapshot struct {
	ID         string
	SessionID  string
	CallerID   string
	Generation uint64
	Restart    bool
	State      OperationState
	Failure    session.Failure
}

type ReadinessState string

const (
	ReadinessPending ReadinessState = "PENDING"
	ReadinessReady   ReadinessState = "READY"
	ReadinessFailed  ReadinessState = "FAILED"
)

type SessionMetadata struct {
	PositionEncoding      string
	CallHierarchySupport  bool
	DocumentSymbolSupport bool
	ProviderName          string
	ProviderVersion       string
}

type ReadinessSnapshot struct {
	ID                  string
	AttemptID           manageddiagnostic.StartupAttemptID
	DiagnosticOperation DiagnosticOperationHandle `json:"-"`
	SessionID           string
	Generation          uint64
	State               ReadinessState
	Failure             session.Failure
	Metadata            SessionMetadata
	Duration            time.Duration
	RequestMessages     int
	RequestBytes        int64
	ResponseMessages    int
	ResponseBytes       int64
	ThermalPhase        string
}

type readinessOperation struct {
	snapshot   ReadinessSnapshot
	started    time.Time
	done       chan struct{}
	diagnostic *manageddiagnostic.EventCollector
}

type Child interface {
	Teardown(context.Context) managedprocess.TeardownObservation
	Close() managedprocess.ResourceObservation
}

// wireChild resources must be interruptible by Close/Teardown, without acquiring
// Manager.mu. Starter implementations own that primitive contract.
type wireChild interface {
	Child
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
}

type Starter interface {
	Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation)
}

// ManagedStarter adapts the existing managed-process foundation to the narrow
// runtime seam. Production callers construct Manager with the sealed gate.
type ManagedStarter struct{ Manager *managedprocess.Manager }

func (s ManagedStarter) Start(ctx context.Context, spec managedprocess.Spec) (Child, managedprocess.StartObservation) {
	if s.Manager == nil {
		return nil, managedprocess.StartObservation{Kind: managedprocess.StartUnavailable, Reason: "containment unavailable"}
	}
	return s.Manager.Start(ctx, spec)
}

type Config struct {
	Limits           Limits
	Wire             lspwire.Limits
	Starter          Starter
	ReadinessTimeout time.Duration
	// Now is an optional monotonic clock seam for deterministic runtime observations.
	Now func() time.Time
	// Diagnostics is an optional internal-only exact-generation store.
	Diagnostics           *manageddiagnostic.Store
	SeedRevisionAuthority seedbinding.RevisionAuthority
	// Unexported seams keep deterministic fixtures inside this package; callers
	// cannot provide bytes that appear directly in a production attempt ID.
	startupAttemptEntropy func(uint64) []byte
	startupAttemptRandom  io.Reader
}
type StartRequest struct {
	Profile          runtimeprofile.Profile
	SeedBinding      *seedbinding.Manifest
	ProviderIdentity seedbinding.ProviderIdentity
	Process          managedprocess.Spec
	LanguageID       string
	Deadline         time.Time
}
type StartResult struct {
	AttemptID            manageddiagnostic.StartupAttemptID
	PublicDetail         string
	SessionID            string
	Generation           uint64
	DiagnosticGeneration DiagnosticGenerationHandle `json:"-"`
	State                session.State
	Failure              session.Failure
	Start                managedprocess.StartObservation
}
type Census struct{ Sessions, Generations, Requests, Children, Cancels, Tombstones, Observations, Operations, Workers int }
type Observation struct {
	Sequence, Generation uint64
	SessionID, Kind      string
	State                session.State
	Failure              session.Failure
}
type Record struct {
	SessionID  string
	Profile    runtimeprofile.Profile
	Generation uint64
	State      session.State
	Started    time.Time
}
type Request struct {
	Key      lspwire.RequestKey
	Deadline time.Time
	Terminal session.Failure
}

// RoundTripRequest describes one transport-neutral JSON-RPC transaction against
// an exact session generation.
type RoundTripRequest struct {
	SessionID   string
	Generation  uint64
	Method      string
	Params      json.RawMessage
	Deadline    time.Time
	MaxMessages int
	MaxBytes    int64
	// Diagnostic join values must be opaque safe identities, never labels or paths.
	DiagnosticCallerID string
	DiagnosticTargetID string
	DiagnosticSequence uint64 // scoped to the acquisition owner, not globally allocated
	DiagnosticObserver func(manageddiagnostic.Record)
}

// RoundTripResult is the immutable terminal observation of one transaction.
type DocumentRequest struct {
	SessionID, URI, LanguageID string
	Generation                 uint64
	// CaptureSupply returns an owned observation of this call's successful
	// notification write. Omitted mode retains its historical behavior.
	CaptureSupply bool `json:",omitempty"`
}

// MaxDocumentSupplyBytes bounds opt-in notification evidence. It does not bound
// legacy document synchronization, which preserves its existing behavior.
const MaxDocumentSupplyBytes = 1 << 20

const DocumentSupplyUnavailable session.Failure = "DOCUMENT_SUPPLY_UNAVAILABLE"

// DocumentSupply is historical supply evidence, not proof of server consumption,
// analyzed-source identity, or a currently active generation. Public values can
// be fabricated; offline consistency checks cannot authenticate their origin.
// The caller owns Content and Params. Manager retains neither buffer.
type DocumentSupply struct {
	Classification  string
	SessionID       string
	Generation      uint64
	URI             string
	DocumentVersion int
	Method          string
	Content         []byte
	Params          json.RawMessage
}

type DocumentResult struct {
	URI, LanguageID     string
	Version             int
	Failure             session.Failure
	DiagnosticOperation DiagnosticOperationHandle `json:"-"`
	// Supply is nil for omitted capture, unchanged/cached documents, and errors.
	// A cached digest is never promoted to evidence of a new notification.
	Supply *DocumentSupply `json:",omitempty"`
}

type openDocument struct {
	languageID string
	version    int
	digest     [32]byte
}

const LanguageIDUnavailable session.Failure = "LANGUAGE_ID_UNAVAILABLE"

// PrepareDocument resolves one effective language identity, synchronizes the
// workspace file, and retains the exact value used by the document generation.
// Context cancellation interrupts owned notification I/O and retires that exact
// generation before returning. Source filesystem operations remain synchronous:
// context is checked before and after reads, not enforced inside kernel reads.
// Supply and version are returned only after a confirmed complete write (or the
// historical unchanged-document cache hit, which returns no new Supply).
func (m *Manager) PrepareDocument(ctx context.Context, req DocumentRequest) DocumentResult {
	if failure := contextFailure(ctx); failure != "" {
		return DocumentResult{Failure: failure}
	}
	u, err := url.Parse(req.URI)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.Path == "" {
		return DocumentResult{Failure: LanguageIDUnavailable}
	}
	path := filepath.Clean(filepath.FromSlash(u.Path))
	m.mu.Lock()
	r := m.sessions[req.SessionID]
	if r == nil {
		m.mu.Unlock()
		return DocumentResult{Failure: session.SessionNotFound}
	}
	if req.Generation != r.record.Generation {
		m.mu.Unlock()
		return DocumentResult{Failure: session.StaleGeneration}
	}
	workspace := filepath.Clean(r.record.Profile.Workspace().String())
	configured := r.languageID
	retainedSource, retained := r.seedSources[req.URI]
	retainedSource = append([]byte(nil), retainedSource...)
	diagnosticGeneration, identity := r.diagnosticGeneration, r.identity
	m.mu.Unlock()
	rel, err := filepath.Rel(workspace, path)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return DocumentResult{Failure: LanguageIDUnavailable}
	}
	languageID := req.LanguageID
	if languageID == "" {
		languageID = configured
	}
	if languageID == "" {
		languageID = source.LanguageID(path)
	}
	if languageID == "" {
		return DocumentResult{Failure: LanguageIDUnavailable}
	}
	var text []byte
	if retained {
		text = retainedSource
	} else if req.CaptureSupply {
		// Bind scope to the host-owned workspace, not an arbitrary URI path.
		// Exact canonical file URI spelling rejects query/fragment/alias forms.
		if req.URI != (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String() {
			return DocumentResult{Failure: DocumentSupplyUnavailable}
		}
		root, openErr := os.OpenRoot(workspace)
		if openErr != nil {
			return DocumentResult{Failure: DocumentSupplyUnavailable}
		}
		text, err = source.ReadRegularInputBounded(root, filepath.ToSlash(rel), MaxDocumentSupplyBytes)
		closeErr := root.Close()
		if err != nil || closeErr != nil || !utf8.Valid(text) {
			return DocumentResult{Failure: DocumentSupplyUnavailable}
		}
	} else {
		text, err = os.ReadFile(path)
		if err != nil {
			return DocumentResult{Failure: LanguageIDUnavailable}
		}
	}
	if failure := contextFailure(ctx); failure != "" {
		return DocumentResult{Failure: failure}
	}
	digest := sha256.Sum256(text)
	diagnosticHandle, collector := m.newDiagnosticOperation(diagnosticGeneration, identity)
	m.describeDiagnosticOperation(diagnosticHandle, "textDocument/didOpen", req.URI, SessionMetadata{})
	finishDocument := func(result DocumentResult, terminal uint16) DocumentResult {
		result.DiagnosticOperation = diagnosticHandle
		m.completeDiagnosticOperation(diagnosticHandle, collector, terminal)
		return result
	}

	m.mu.Lock()
	r = m.sessions[req.SessionID]
	if r == nil || r.record.Generation != req.Generation {
		m.mu.Unlock()
		return finishDocument(DocumentResult{Failure: session.StaleGeneration}, diagnosticEventTerminalFailure)
	}
	if m.closed || r.record.State != session.Ready || r.protocolOwned {
		m.mu.Unlock()
		return finishDocument(DocumentResult{Failure: session.LifecycleConflict}, diagnosticEventTerminalFailure)
	}
	child, ok := r.process.(wireChild)
	if !ok {
		m.mu.Unlock()
		return finishDocument(DocumentResult{Failure: session.SessionPoisoned}, diagnosticEventTerminalFailure)
	}
	previous, opened := r.documents[req.URI]
	if opened && previous.languageID != languageID {
		m.mu.Unlock()
		return finishDocument(DocumentResult{Failure: session.LifecycleConflict}, diagnosticEventTerminalFailure)
	}
	if opened && previous.digest == digest {
		m.mu.Unlock()
		collector.Record(diagnosticEventCacheHit, int64(previous.version), true)
		return finishDocument(DocumentResult{URI: req.URI, LanguageID: languageID, Version: previous.version}, diagnosticEventTerminalResponse)
	}
	if m.workers >= m.limits.MaxChildren {
		m.mu.Unlock()
		return finishDocument(DocumentResult{Failure: session.ResourceExhausted}, diagnosticEventTerminalFailure)
	}
	r.protocolOwned = true
	m.workers++
	m.mu.Unlock()
	version := 1
	method := "textDocument/didOpen"
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": req.URI, "languageId": languageID, "version": version, "text": string(text)}})
	if opened {
		version = previous.version + 1
		method = "textDocument/didChange"
		params, _ = json.Marshal(map[string]any{"textDocument": map[string]any{"uri": req.URI, "version": version}, "contentChanges": []map[string]string{{"text": string(text)}}})
	}
	owner := &ownedTransport{child: child}
	collector.Record(diagnosticEventWriteAttempt, int64(len(params)), false)
	writeErr := owner.run(ctx, func() error {
		return lspwire.NewWriter(checkedWriter{child.Stdin()}, m.wire).Write(lspwire.Message{JSONRPC: lspwire.Version, Method: method, Params: params})
	})
	failure := contextFailure(ctx)
	if writeErr == nil {
		collector.Record(diagnosticEventWriteComplete, int64(len(params)), true)
	}
	if writeErr != nil || failure != "" {
		owner.retire()
		if failure == "" {
			failure = session.SessionPoisoned
		}
	}
	m.mu.Lock()
	m.workers--
	select {
	case m.workerDone <- struct{}{}:
	default:
	}
	// The lease excludes lifecycle replacement; keep the exact-generation guard
	// here as well so no late completion can mutate a replacement record.
	r = m.sessions[req.SessionID]
	if r == nil || r.record.Generation != req.Generation {
		m.mu.Unlock()
		return finishDocument(DocumentResult{Failure: session.StaleGeneration}, diagnosticEventTerminalFailure)
	}
	r.protocolOwned = false
	if failure != "" {
		r.record.State = session.Poisoned
		r.retired = owner
		m.observe(req.SessionID, req.Generation, "document-failed", r.record.State, failure)
		if m.diagnostics != nil {
			diagnostic := manageddiagnostic.RecordDocumentSupply(manageddiagnostic.DocumentSupplyObservation{SessionID: req.SessionID, Generation: req.Generation, Sequence: m.sequence, Completed: false, Method: method})
			diagnostic.Terminal = manageddiagnostic.TerminalProtocolError
			diagnostic.Write = manageddiagnostic.IOFacts{State: manageddiagnostic.IOFailed, Messages: 1}
			m.diagnostics.Record(diagnostic)
		}
		m.mu.Unlock()
		return finishDocument(DocumentResult{Failure: failure}, diagnosticEventTerminalFailure)
	}
	r.documents[req.URI] = openDocument{languageID: languageID, version: version, digest: digest}
	m.observe(req.SessionID, req.Generation, "document", r.record.State, "")
	if m.diagnostics != nil {
		diagnostic := manageddiagnostic.RecordDocumentSupply(manageddiagnostic.DocumentSupplyObservation{SessionID: req.SessionID, Generation: req.Generation, Sequence: m.sequence, Completed: true, Method: method})
		diagnostic.Write = manageddiagnostic.IOFacts{State: manageddiagnostic.IOComplete, Messages: 1}
		m.diagnostics.Record(diagnostic)
	}
	result := DocumentResult{URI: req.URI, LanguageID: languageID, Version: version}
	if req.CaptureSupply {
		result.Supply = &DocumentSupply{
			Classification: "LSP_SUPPLIED", SessionID: req.SessionID,
			Generation: req.Generation, URI: req.URI, DocumentVersion: version,
			Method: method, Content: append([]byte(nil), text...),
			Params: append(json.RawMessage(nil), params...),
		}
	}
	m.mu.Unlock()
	return finishDocument(result, diagnosticEventTerminalResponse)
}

func (m *Manager) SeedBindingRequested(sessionID string, generation uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[sessionID]
	return r != nil && r.record.Generation == generation && r.seedBinding != nil
}

// AdmitSeedBinding supplies the retained preflight bytes and validates their
// declaration identity on the same initialized generation before target resolution.
// Request limits are supplied by the acquisition owner and count against the same
// deterministic request/time budget as subsequent acquisition requests.
func (m *Manager) AdmitSeedBinding(ctx context.Context, sessionID string, generation uint64, deadline time.Time, maxMessages int, maxBytes int64) seedbinding.ValidationResult {
	m.mu.Lock()
	r := m.sessions[sessionID]
	if r == nil {
		m.mu.Unlock()
		return seedbinding.ValidationResult{Status: seedbinding.Invalid, PrivateDetail: string(session.SessionNotFound)}
	}
	if r.record.Generation != generation {
		m.mu.Unlock()
		return seedbinding.ValidationResult{Status: seedbinding.Invalid, PrivateDetail: string(session.StaleGeneration)}
	}
	if r.seedBinding == nil || r.seedAdmittedGeneration == generation {
		m.mu.Unlock()
		return seedbinding.ValidationResult{Status: seedbinding.Match}
	}
	manifest := *r.seedBinding
	encoding := r.metadata.PositionEncoding
	expectedProvider := r.providerIdentity
	observedProvider := expectedProvider
	observedProvider.Name, observedProvider.Version = r.metadata.ProviderName, r.metadata.ProviderVersion
	m.mu.Unlock()
	if err := seedbinding.VerifyProviderIdentity(expectedProvider, observedProvider); err != nil {
		return seedbinding.ValidationResult{Status: seedbinding.Mismatch, PrivateDetail: "selected provider identity mismatch before semantic request"}
	}
	if encoding == "" || encoding != manifest.Locator.Encoding {
		return seedbinding.ValidationResult{Status: seedbinding.Unavailable, PrivateDetail: "negotiated position encoding unavailable"}
	}
	document := m.PrepareDocument(ctx, DocumentRequest{SessionID: sessionID, Generation: generation, URI: manifest.Locator.URI, LanguageID: manifest.Validator.Language, CaptureSupply: true})
	if document.Failure != "" || document.Supply == nil {
		return seedbinding.ValidationResult{Status: seedbinding.Unavailable, PrivateDetail: "retained document supply unavailable"}
	}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": manifest.Locator.URI}})
	validation := seedbinding.ValidationResult{Status: seedbinding.Unavailable, PrivateDetail: "documentSymbol unavailable"}
	response := m.roundTrip(ctx, RoundTripRequest{SessionID: sessionID, Generation: generation, Method: "textDocument/documentSymbol", Params: params, Deadline: deadline, MaxMessages: maxMessages, MaxBytes: maxBytes, DiagnosticTargetID: manifest.ID}, func(raw json.RawMessage, rpcError *lspwire.RPCError) bool {
		if rpcError != nil {
			return false
		}
		validation = seedbinding.ValidateDocumentSymbols(raw, manifest, document.Supply.Content, encoding)
		return validation.Status == seedbinding.Match
	})
	if response.Failure != "" || response.ServerError != nil {
		return seedbinding.ValidationResult{Status: seedbinding.Unavailable, PrivateDetail: "documentSymbol unavailable"}
	}
	if validation.Status == seedbinding.Match {
		m.mu.Lock()
		if current := m.sessions[sessionID]; current != nil && current.record.Generation == generation && current.seedBinding != nil {
			current.seedAdmittedGeneration = generation
		} else {
			validation = seedbinding.ValidationResult{Status: seedbinding.Invalid, PrivateDetail: "generation changed during semantic admission"}
		}
		m.mu.Unlock()
	}
	return validation
}

type RoundTripResult struct {
	Key                 lspwire.RequestKey
	DiagnosticOperation DiagnosticOperationHandle `json:"-"`
	Result              json.RawMessage
	ServerError         *lspwire.RPCError
	Failure             session.Failure
	Messages            int
	Bytes               int64
	RequestMessages     int
	RequestBytes        int64
	Duration            time.Duration
	ThermalPhase        string
	Notifications       []lspwire.Message
	Responses           []lspwire.Message
	started             time.Time
	diagnostic          manageddiagnostic.RequestObservation
	diagnosticSink      func(manageddiagnostic.Record)
	eventCollector      *manageddiagnostic.EventCollector
}

// RoundTrip executes one complete protocol transaction while exclusively
// owning the exact generation's stdin/stdout stream. Concurrent transactions
// and lifecycle operations are rejected rather than queued.
func (m *Manager) RoundTrip(parent context.Context, req RoundTripRequest) RoundTripResult {
	return m.roundTrip(parent, req, nil)
}

// roundTrip is the single transaction path. classify is package-private so only
// manager-owned workflows can append semantic disposition to the same collector.
func (m *Manager) roundTrip(parent context.Context, req RoundTripRequest, classify func(json.RawMessage, *lspwire.RPCError) bool) RoundTripResult {
	if failure := contextFailure(parent); failure != "" {
		return RoundTripResult{Failure: failure}
	}
	m.mu.Lock()
	r := m.sessions[req.SessionID]
	if r == nil {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.SessionNotFound}
	}
	if req.Generation != r.record.Generation {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.StaleGeneration}
	}
	if r.record.State != session.Ready || r.protocolOwned {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.LifecycleConflict}
	}
	child, ok := r.process.(wireChild)
	if !ok {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.SessionPoisoned}
	}
	if len(r.requests) >= m.limits.MaxRequests || m.workers >= m.limits.MaxChildren {
		m.mu.Unlock()
		return RoundTripResult{Failure: session.ResourceExhausted}
	}
	key := r.pending.Begin(req.Generation)
	r.requests[key] = &Request{Key: key, Deadline: req.Deadline}
	diagnosticGeneration, identity := r.diagnosticGeneration, r.identity
	r.protocolOwned = true
	m.workers++
	m.observe(req.SessionID, req.Generation, "request", r.record.State, "")
	m.mu.Unlock()

	handle, collector := m.newDiagnosticOperation(diagnosticGeneration, identity)
	documentURI := ""
	var documentParams struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(req.Params, &documentParams) == nil {
		documentURI = documentParams.TextDocument.URI
	}
	m.describeDiagnosticOperation(handle, req.Method, documentURI, SessionMetadata{})
	started := m.now()
	result := RoundTripResult{Key: key, DiagnosticOperation: handle, ThermalPhase: "WARM", started: started, diagnosticSink: req.DiagnosticObserver, eventCollector: collector}
	maxMessages := req.MaxMessages
	if maxMessages <= 0 {
		maxMessages = 1
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = m.wire.MaxBodyBytes
		if maxBytes <= 0 {
			maxBytes = lspwire.DefaultLimits().MaxBodyBytes
		}
	}
	requestedDeadline := time.Duration(0)
	if !req.Deadline.IsZero() {
		requestedDeadline = req.Deadline.Sub(started)
		if requestedDeadline < 0 {
			requestedDeadline = 0
		}
	}
	effectiveDeadline := requestedDeadline
	if parentDeadline, ok := parent.Deadline(); ok {
		parentRemaining := parentDeadline.Sub(started)
		if parentRemaining < 0 {
			parentRemaining = 0
		}
		if req.Deadline.IsZero() || parentRemaining < effectiveDeadline {
			effectiveDeadline = parentRemaining
		}
	}
	result.diagnostic = manageddiagnostic.RequestObservation{SessionID: req.SessionID, Generation: req.Generation, Sequence: req.DiagnosticSequence, Method: req.Method, ProtocolID: key.ID, TargetID: req.DiagnosticTargetID, CallerID: req.DiagnosticCallerID, RequestedDeadline: requestedDeadline, EffectiveDeadline: effectiveDeadline, RequestedMaxBytes: req.MaxBytes, EffectiveMaxBytes: maxBytes, RequestedMaxMessages: req.MaxMessages, EffectiveMaxMessages: maxMessages, Clock: func() int64 { return m.now().UnixNano() }}
	if result.diagnostic.Sequence == 0 {
		result.diagnostic.Sequence = key.ID
	}
	ctx := parent
	cancel := func() {}
	if !req.Deadline.IsZero() {
		ctx, cancel = context.WithDeadline(parent, req.Deadline)
	}
	defer cancel()

	writer := lspwire.NewWriter(checkedWriter{child.Stdin()}, m.wire)
	id := json.RawMessage(strconv.FormatUint(key.ID, 10))
	requestMessage := lspwire.Message{JSONRPC: lspwire.Version, ID: id, Method: req.Method, Params: req.Params}
	requestBody, _ := json.Marshal(requestMessage)
	result.RequestMessages = 1
	result.RequestBytes = int64(len(requestBody))
	owner := &ownedTransport{child: child}
	collector.Record(diagnosticEventWriteAttempt, result.RequestBytes, false)
	if err := owner.run(ctx, func() error { return writer.Write(requestMessage) }); err != nil {
		failure := contextFailure(ctx)
		if failure == "" {
			failure = session.SessionPoisoned
		}
		return m.finishRoundTrip(req.SessionID, owner, result, failure, true)
	}
	collector.Record(diagnosticEventWriteComplete, result.RequestBytes, true)

	type readResult struct {
		message lspwire.Message
		err     error
	}
	reads := make(chan readResult, 1)
	reader := lspwire.NewReader(child.Stdout(), m.wire)
	for result.Messages < maxMessages {
		go func() { msg, err := reader.Read(); reads <- readResult{msg, err} }()
		select {
		case <-ctx.Done():
			// Preserve cooperative $/cancelRequest behavior, but never let that
			// notification become a second unbounded write after cancellation.
			cancelCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			var state lspwire.CancelState
			_ = owner.run(cancelCtx, func() error {
				var err error
				state, err = r.pending.Cancel(writer, key)
				return err
			})
			stopCancel()
			owner.retire()
			<-reads // Close/teardown interrupted the outstanding read; join it.
			if state == lspwire.CancelWritten {
				m.mu.Lock()
				r.cancels++
				m.observe(req.SessionID, req.Generation, "cancel", r.record.State, session.RequestCancelled)
				m.mu.Unlock()
			}
			failure := session.RequestCancelled
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				failure = session.RequestTimeout
			}
			return m.finishRoundTrip(req.SessionID, owner, result, failure, true)
		case read := <-reads:
			if read.err != nil {
				failure := session.SessionPoisoned
				if errors.Is(read.err, io.EOF) {
					failure = session.SessionCrashed
				}
				return m.finishRoundTrip(req.SessionID, owner, result, failure, true)
			}
			body, _ := json.Marshal(read.message)
			collector.Record(diagnosticEventReadComplete, int64(len(body)), true)
			result.Messages++
			result.Bytes += int64(len(body))
			if result.Bytes > maxBytes {
				return m.finishRoundTrip(req.SessionID, owner, result, session.ResourceExhausted, true)
			}
			if read.message.Kind() == lspwire.KindNotification || read.message.Kind() == lspwire.KindRequest {
				result.Notifications = append(result.Notifications, read.message)
				continue
			}
			responseID, err := strconv.ParseUint(string(read.message.ID), 10, 64)
			if err != nil || responseID != key.ID {
				collector.Record(diagnosticEventUnmatched, int64(responseID), false)
				result.Responses = append(result.Responses, read.message)
				continue
			}
			disposition := r.pending.Accept(lspwire.ResponseKey{Generation: req.Generation, ID: responseID})
			if disposition != lspwire.ResponseAccepted {
				collector.Record(diagnosticEventLate, int64(responseID), false)
				result.Responses = append(result.Responses, read.message)
				continue
			}
			collector.Record(diagnosticEventMatched, int64(responseID), true)
			result.Result, result.ServerError = append(json.RawMessage(nil), read.message.Result...), read.message.Error
			collector.Record(diagnosticEventResponseDecoded, int64(len(result.Result)), result.ServerError == nil)
			if classify != nil {
				if classify(result.Result, result.ServerError) {
					collector.Record(diagnosticEventSemanticMatched, int64(responseID), true)
				} else {
					collector.Record(diagnosticEventSemanticUnmatched, int64(responseID), false)
				}
			}
			return m.finishRoundTrip(req.SessionID, owner, result, "", false)
		}
	}
	return m.finishRoundTrip(req.SessionID, owner, result, session.ResourceExhausted, true)
}

func (m *Manager) finishRoundTrip(id string, owner *ownedTransport, result RoundTripResult, failure session.Failure, poison bool) RoundTripResult {
	if poison {
		owner.retire()
	}
	m.mu.Lock()
	if r := m.sessions[id]; r != nil && r.record.Generation == result.Key.Generation {
		delete(r.requests, result.Key)
		r.protocolOwned = false
		if poison {
			r.record.State = session.Poisoned
			r.retired = owner
		}
		m.observe(id, result.Key.Generation, "response", r.record.State, failure)
	}
	result.Failure = failure
	result.Duration = m.now().Sub(result.started)
	terminalCode := diagnosticEventTerminalResponse
	if failure == session.RequestTimeout {
		terminalCode = diagnosticEventTerminalDeadline
	} else if failure != "" {
		terminalCode = diagnosticEventTerminalFailure
	}
	m.completeDiagnosticOperationLocked(result.DiagnosticOperation, result.eventCollector, terminalCode)
	terminal := manageddiagnostic.TerminalResponseReceived
	readState, writeState := manageddiagnostic.IOComplete, manageddiagnostic.IOComplete
	if failure != "" {
		terminal, readState = manageddiagnostic.TerminalProtocolError, manageddiagnostic.IOFailed
		switch failure {
		case session.RequestTimeout:
			terminal = manageddiagnostic.TerminalDeadlineExceeded
		case session.RequestCancelled:
			terminal = manageddiagnostic.TerminalCancelled
		case session.SessionCrashed:
			terminal = manageddiagnostic.TerminalTransportClosed
		}
	}
	diagnostic := manageddiagnostic.RecordRequest(result.diagnostic, func() manageddiagnostic.RequestOutcome {
		return manageddiagnostic.RequestOutcome{Terminal: terminal, ReadState: readState, WriteState: writeState, RequestBytes: result.RequestBytes, ResponseBytes: result.Bytes, ResponseMessages: result.Messages}
	})
	if result.ServerError != nil {
		diagnostic.NumericRPCCode = manageddiagnostic.Fact[int]{Status: manageddiagnostic.Observed, Value: result.ServerError.Code}
		diagnostic.Terminal = manageddiagnostic.TerminalProtocolError
	}
	if result.Duration < 0 {
		result.Duration = 0
	}
	m.workers--
	select {
	case m.workerDone <- struct{}{}:
	default:
	}
	m.mu.Unlock()
	if m.diagnostics != nil {
		m.diagnostics.Record(diagnostic)
	}
	if result.diagnosticSink != nil {
		result.diagnosticSink(diagnostic.Clone())
	}
	return result
}

type runtimeSession struct {
	record                 Record
	attemptID              manageddiagnostic.StartupAttemptID
	process                Child
	retired                *ownedTransport // joined exact-child retirement, never a replacement lookup
	spec                   managedprocess.Spec
	pending                *lspwire.Pending
	requests               map[lspwire.RequestKey]*Request
	cancels                int
	protocolOwned          bool
	lifecycleOwned         bool
	metadata               SessionMetadata
	languageID             string
	documents              map[string]openDocument
	seedSources            map[string][]byte
	seedBinding            *seedbinding.Manifest
	providerIdentity       seedbinding.ProviderIdentity
	seedAdmittedGeneration uint64
	identity               managedprocess.Identity
	diagnosticGeneration   DiagnosticGenerationHandle
}

type Manager struct {
	mu                    sync.Mutex
	limits                Limits
	wire                  lspwire.Limits
	starter               Starter
	algebra               *session.Manager
	sessions              map[string]*runtimeSession
	operations            map[string]OperationSnapshot
	operationIDs          []string
	readiness             map[string]*readinessOperation
	readinessIDs          map[string]string
	observations          []Observation
	sequence              uint64
	readinessSeq          uint64
	readinessTimeout      time.Duration
	now                   func() time.Time
	workers               int
	workerDone            chan struct{}
	closed                bool
	diagnostics           *manageddiagnostic.Store
	seedRevisionAuthority seedbinding.RevisionAuthority
	startupAttemptNonce   [16]byte
	startupAttemptEntropy func(uint64) []byte
	startupAttemptSeq     uint64
	diagnosticSequence    uint64
	diagnosticEvictions   uint64
	diagnosticOperations  map[DiagnosticOperationHandle]diagnosticOperation
	diagnosticOrder       []DiagnosticOperationHandle
}

func New(c Config) (*Manager, error) {
	l := c.Limits
	if l.MaxSessions <= 0 || l.MaxRequests <= 0 || l.MaxChildren <= 0 || l.MaxCancels <= 0 || l.MaxTombstones <= 0 || l.MaxObservations <= 0 {
		return nil, errors.New("sessionruntime: all limits must be positive")
	}
	if c.Starter == nil {
		return nil, errors.New("sessionruntime: starter is required")
	}
	a, err := session.NewManager(session.ManagerConfig{MaxSessions: l.MaxSessions})
	if err != nil {
		return nil, err
	}
	if l.MaxOperations <= 0 {
		l.MaxOperations = l.MaxObservations
	}
	readinessTimeout := c.ReadinessTimeout
	if readinessTimeout <= 0 {
		readinessTimeout = time.Second
	}
	now := c.Now
	if now == nil {
		now = time.Now
	}
	random := c.startupAttemptRandom
	if random == nil {
		random = rand.Reader
	}
	var managerNonce [16]byte
	if _, err := io.ReadFull(random, managerNonce[:]); err != nil {
		return nil, errors.New("sessionruntime: startup attempt identity unavailable")
	}
	return &Manager{limits: l, wire: c.Wire, starter: c.Starter, algebra: a, sessions: make(map[string]*runtimeSession), operations: make(map[string]OperationSnapshot), readiness: make(map[string]*readinessOperation), readinessIDs: make(map[string]string), readinessTimeout: readinessTimeout, now: now, workerDone: make(chan struct{}, 1), diagnostics: c.Diagnostics, seedRevisionAuthority: c.SeedRevisionAuthority, startupAttemptNonce: managerNonce, startupAttemptEntropy: c.startupAttemptEntropy, diagnosticOperations: make(map[DiagnosticOperationHandle]diagnosticOperation)}, nil
}

func (m *Manager) Start(ctx context.Context, req StartRequest) (result StartResult) {
	seedSources := map[string][]byte{}
	if req.SeedBinding != nil {
		outcome := seedbinding.ValidateMechanical(ctx, req.Profile.Workspace().String(), *req.SeedBinding, m.seedRevisionAuthority)
		if outcome.Status != seedbinding.Match {
			return StartResult{Failure: session.Failure(outcome.Terminal), PublicDetail: outcome.Terminal}
		}
		seedSources[req.SeedBinding.Locator.URI] = append([]byte(nil), outcome.Source...)
	}
	attemptID, attemptSequence, attemptStarted := m.beginStartupAttempt()
	defer func() {
		result.AttemptID = attemptID
		if result.SessionID != "" && result.Generation != 0 {
			m.mu.Lock()
			if current := m.sessions[result.SessionID]; current != nil && current.record.Generation == result.Generation && current.attemptID == attemptID {
				result.DiagnosticGeneration = current.diagnosticGeneration
			}
			m.mu.Unlock()
		}
		if !m.finishStartupAttempt(attemptID, attemptSequence, attemptStarted, result) {
			panic("sessionruntime: startup attempt retention invariant violated")
		}
	}()
	if !req.Deadline.IsZero() && !time.Now().Before(req.Deadline) {
		return StartResult{Failure: session.RequestTimeout}
	}
	if err := ctx.Err(); err != nil {
		return StartResult{Failure: session.RequestCancelled}
	}
	id := req.Profile.SessionKey().String()
	m.mu.Lock()
	if current := m.sessions[id]; current != nil {
		result := StartResult{SessionID: id, Generation: current.record.Generation, State: current.record.State}
		m.mu.Unlock()
		return result
	}
	if len(m.sessions) >= m.limits.MaxSessions {
		m.mu.Unlock()
		return StartResult{SessionID: id, Failure: session.ResourceExhausted}
	}
	m.mu.Unlock()

	// Identity is observed from the exact immutable request spec before process
	// creation; only fixed-size digests and scalar status/count values survive.
	identity := managedprocess.ObserveIdentity(req.Process, req.Profile.ProfileName(), req.Profile.Workspace().String())
	// Start is deliberately before admission. The production managedprocess gate
	// returns UNAVAILABLE before command construction, pipes, or process creation.
	child, observed := m.starter.Start(ctx, req.Process)
	if observed.Kind == managedprocess.StartUnavailable {
		return StartResult{SessionID: id, Failure: session.ProcessContainmentUnavailable, Start: observed}
	}
	if observed.Kind != managedprocess.StartStarted || child == nil {
		return StartResult{SessionID: id, Failure: session.SpawnFailure, Start: observed}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions[id] != nil || len(m.sessions) >= m.limits.MaxSessions {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		return StartResult{SessionID: id, Failure: session.LifecycleConflict, Start: observed}
	}
	m.algebra.RegisterLifecycle(id, 1, session.Initializing, false)
	if admission := m.algebra.Admit(id, 0); admission.Kind != session.AdmissionFree {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		return StartResult{SessionID: id, Failure: session.ResourceExhausted, Start: observed}
	}
	r := Record{SessionID: id, Profile: req.Profile, Generation: 1, State: session.Initializing, Started: time.Now()}
	var retainedBinding *seedbinding.Manifest
	if req.SeedBinding != nil {
		copy := *req.SeedBinding
		retainedBinding = &copy
	}
	diagnosticGeneration := m.newDiagnosticGeneration(attemptID, id, 1)
	m.sessions[id] = &runtimeSession{record: r, attemptID: attemptID, process: child, spec: req.Process, pending: lspwire.NewPending(m.limits.MaxTombstones), requests: make(map[lspwire.RequestKey]*Request), languageID: req.LanguageID, documents: make(map[string]openDocument), seedSources: seedSources, seedBinding: retainedBinding, providerIdentity: req.ProviderIdentity, identity: identity, diagnosticGeneration: diagnosticGeneration}
	m.observe(id, 1, "startup", session.Initializing, "")
	return StartResult{SessionID: id, Generation: 1, State: session.Initializing, Start: observed}
}

func (m *Manager) BeginReadiness(ctx context.Context, id string, generation uint64, deadline time.Time) ReadinessSnapshot {
	m.mu.Lock()
	r := m.sessions[id]
	if r == nil {
		m.mu.Unlock()
		return ReadinessSnapshot{SessionID: id, Generation: generation, State: ReadinessFailed, Failure: session.SessionNotFound}
	}
	if generation != r.record.Generation {
		m.mu.Unlock()
		return ReadinessSnapshot{SessionID: id, Generation: generation, State: ReadinessFailed, Failure: session.StaleGeneration}
	}
	if existing := m.readinessIDs[id+":"+strconv.FormatUint(generation, 10)]; existing != "" {
		snapshot := m.readiness[existing].snapshot
		m.mu.Unlock()
		return snapshot
	}
	if r.protocolOwned {
		m.mu.Unlock()
		return ReadinessSnapshot{SessionID: id, Generation: generation, State: ReadinessFailed, Failure: session.LifecycleConflict}
	}
	protocol, ok := r.process.(wireChild)
	if !ok {
		m.mu.Unlock()
		return ReadinessSnapshot{SessionID: id, Generation: generation, State: ReadinessFailed, Failure: session.InitializationFailure}
	}
	workspace := r.record.Profile.Workspace().String()
	m.readinessSeq++
	opID := "readiness-" + strconv.FormatUint(m.readinessSeq, 10)
	op := &readinessOperation{snapshot: ReadinessSnapshot{ID: opID, AttemptID: r.attemptID, SessionID: id, Generation: generation, State: ReadinessPending, ThermalPhase: "COLD"}, started: m.now(), done: make(chan struct{})}
	m.readiness[opID] = op
	m.readinessIDs[id+":"+strconv.FormatUint(generation, 10)] = opID
	r.protocolOwned = true
	m.workers++
	diagnosticGeneration, identity := r.diagnosticGeneration, r.identity
	m.mu.Unlock()
	handle, collector := m.newDiagnosticOperation(diagnosticGeneration, identity)
	m.mu.Lock()
	op.snapshot.DiagnosticOperation = handle
	op.diagnostic = collector
	snapshot := op.snapshot
	m.mu.Unlock()
	go m.runReadiness(ctx, deadline, protocol, opID, generation, workspace)
	return snapshot
}

func (m *Manager) runReadiness(parent context.Context, deadline time.Time, child wireChild, opID string, generation uint64, workspace string) {
	ctx := parent
	cancel := func() {}
	if deadline.IsZero() {
		ctx, cancel = context.WithTimeout(parent, m.readinessTimeout)
	} else {
		ctx, cancel = context.WithDeadline(parent, deadline)
	}
	defer cancel()

	key := lspwire.NewPending(1).Begin(generation)
	id := strconv.FormatUint(key.ID, 10)
	writer := lspwire.NewWriter(child.Stdin(), m.wire)
	workspaceURI := (&url.URL{Scheme: "file", Path: workspace}).String()
	params, _ := json.Marshal(struct {
		ProcessID        any    `json:"processId"`
		RootURI          string `json:"rootUri"`
		WorkspaceFolders []struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"workspaceFolders"`
		Capabilities struct {
			TextDocument struct {
				CallHierarchy struct {
					DynamicRegistration bool `json:"dynamicRegistration"`
				} `json:"callHierarchy"`
			} `json:"textDocument"`
		} `json:"capabilities"`
	}{ProcessID: nil, RootURI: workspaceURI, WorkspaceFolders: []struct {
		URI  string `json:"uri"`
		Name string `json:"name"`
	}{{URI: workspaceURI, Name: "workspace"}}})
	initializeMessage := lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(id), Method: "initialize", Params: params}
	initializeBody, _ := json.Marshal(initializeMessage)
	m.recordReadinessEvent(opID, diagnosticEventWriteAttempt, int64(len(initializeBody)), false)
	m.recordReadinessRequest(opID, int64(len(initializeBody)))
	if err := writer.Write(initializeMessage); err != nil {
		m.abortReadinessDiagnostic(child, opID, session.InitializationFailure, manageddiagnostic.PhaseInitializeWrite, manageddiagnostic.Fact[manageddiagnostic.Substep]{Status: manageddiagnostic.Unavailable}, manageddiagnostic.TerminalProtocolError, "initialize-write-failed")
		return
	}
	m.recordReadinessEvent(opID, diagnosticEventWriteComplete, int64(len(initializeBody)), true)
	type readinessResult struct {
		metadata SessionMetadata
		err      error
	}
	response := make(chan readinessResult, 1)
	go func() {
		reader := lspwire.NewReader(child.Stdout(), m.wire)
		for messages := 0; messages < m.limits.MaxObservations; messages++ {
			message, err := reader.Read()
			if err != nil {
				response <- readinessResult{err: err}
				return
			}
			body, _ := json.Marshal(message)
			m.recordReadinessResponse(opID, int64(len(body)))
			m.recordReadinessEvent(opID, diagnosticEventReadComplete, int64(len(body)), true)
			if message.Kind() == lspwire.KindNotification {
				continue
			}
			if message.Kind() != lspwire.KindSuccessResponse || string(message.ID) != id || message.Error != nil || len(message.Result) == 0 {
				response <- readinessResult{err: errors.New("sessionruntime: invalid readiness response")}
				return
			}
			metadata := SessionMetadata{}
			var initialized struct {
				ServerInfo struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"serverInfo"`
				Capabilities struct {
					PositionEncoding       string          `json:"positionEncoding"`
					CallHierarchyProvider  json.RawMessage `json:"callHierarchyProvider"`
					DocumentSymbolProvider json.RawMessage `json:"documentSymbolProvider"`
				} `json:"capabilities"`
			}
			if err := json.Unmarshal(message.Result, &initialized); err != nil {
				response <- readinessResult{err: errors.New("sessionruntime: malformed initialize result")}
				return
			}
			metadata.PositionEncoding = initialized.Capabilities.PositionEncoding
			metadata.ProviderName, metadata.ProviderVersion = initialized.ServerInfo.Name, initialized.ServerInfo.Version
			if metadata.PositionEncoding == "" {
				metadata.PositionEncoding = "utf-16"
			}
			provider := initialized.Capabilities.CallHierarchyProvider
			metadata.CallHierarchySupport = string(provider) == "true" || (len(provider) > 0 && string(provider) != "false" && string(provider) != "null")
			documentSymbols := initialized.Capabilities.DocumentSymbolProvider
			metadata.DocumentSymbolSupport = string(documentSymbols) == "true" || (len(documentSymbols) > 0 && string(documentSymbols) != "false" && string(documentSymbols) != "null")
			response <- readinessResult{metadata: metadata}
			return
		}
		response <- readinessResult{err: errors.New("sessionruntime: readiness response limit exceeded")}
	}()
	select {
	case observed := <-response:
		if observed.err != nil {
			terminal, reason := manageddiagnostic.TerminalProtocolError, "initialize-response-invalid"
			if errors.Is(observed.err, io.EOF) {
				terminal, reason = manageddiagnostic.TerminalTransportClosed, "transport-closed"
			}
			m.abortReadinessDiagnostic(child, opID, session.InitializationFailure, manageddiagnostic.PhaseInitializeResponse, manageddiagnostic.Fact[manageddiagnostic.Substep]{Status: manageddiagnostic.Unavailable}, terminal, reason)
			return
		}
		initializedMessage := lspwire.Message{JSONRPC: lspwire.Version, Method: "initialized", Params: json.RawMessage(`{}`)}
		initializedBody, _ := json.Marshal(initializedMessage)
		m.recordReadinessRequest(opID, int64(len(initializedBody)))
		if err := writer.Write(initializedMessage); err != nil {
			m.abortReadinessDiagnostic(child, opID, session.InitializationFailure, manageddiagnostic.PhaseInitializeWrite, manageddiagnostic.Fact[manageddiagnostic.Substep]{Status: manageddiagnostic.Observed, Value: manageddiagnostic.SubstepInitializedNotification}, manageddiagnostic.TerminalProtocolError, "initialized-notification-write-failed")
			return
		}
		m.recordReadinessEvent(opID, diagnosticEventInitialized, int64(len(initializedBody)), true)
		m.recordSuccessfulReadiness(opID, int64(len(initializeBody)), int64(len(initializedBody)), observed.metadata)
		m.describeReadinessDiagnostic(opID, observed.metadata)
		m.terminalReadinessEvent(opID, diagnosticEventTerminalResponse)
		m.finishReadiness(opID, ReadinessReady, "", observed.metadata)
	case <-ctx.Done():
		// Retire the readiness-owned transport before closing its diagnostics, then
		// join the reader so every shutdown read observation precedes the terminal.
		_ = child.Teardown(context.Background())
		_ = child.Close()
		late := <-response
		if late.err != nil {
			m.recordReadinessEvent(opID, diagnosticEventLate, 0, false)
		} else {
			m.recordReadinessEvent(opID, diagnosticEventLate, 0, true)
		}
		failure := session.RequestCancelled
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			failure = session.InitializationTimeout
		}
		terminal := manageddiagnostic.TerminalCancelled
		if failure == session.InitializationTimeout {
			terminal = manageddiagnostic.TerminalDeadlineExceeded
		}
		m.abortReadinessDiagnostic(child, opID, failure, manageddiagnostic.PhaseInitializeResponse, manageddiagnostic.Fact[manageddiagnostic.Substep]{Status: manageddiagnostic.Unavailable}, terminal, "initialize-wait-ended")
	}
}

func (m *Manager) describeReadinessDiagnostic(id string, metadata SessionMetadata) {
	m.mu.Lock()
	op := m.readiness[id]
	var handle DiagnosticOperationHandle
	if op != nil {
		handle = op.snapshot.DiagnosticOperation
	}
	m.mu.Unlock()
	m.describeDiagnosticOperation(handle, "initialize", "", metadata)
}

func (m *Manager) abortReadinessDiagnostic(child Child, id string, failure session.Failure, phase manageddiagnostic.Phase, substep manageddiagnostic.Fact[manageddiagnostic.Substep], terminal manageddiagnostic.Terminal, reason string) {
	m.mu.Lock()
	op := m.readiness[id]
	var sid string
	var generation uint64
	if op != nil {
		sid, generation = op.snapshot.SessionID, op.snapshot.Generation
	}
	m.sequence++
	sequence := m.sequence
	m.mu.Unlock()
	preExit := false
	if observable, ok := child.(interface {
		Observe() managedprocess.SurvivorObservation
	}); ok {
		preExit = observable.Observe().Kind == managedprocess.SurvivorDead
	}
	teardown := child.Teardown(context.Background())
	_ = child.Close()
	m.terminalReadinessEvent(id, diagnosticEventTerminalFailure)
	m.finishReadiness(id, ReadinessFailed, failure, SessionMetadata{})
	if m.diagnostics != nil && sid != "" {
		exit := manageddiagnostic.ProcessExit{Status: manageddiagnostic.Unavailable}
		if preExit {
			terminal = manageddiagnostic.TerminalProcessExited
			exit = manageddiagnostic.ProcessExit{Status: manageddiagnostic.Observed, ExitCode: manageddiagnostic.Fact[int]{Status: manageddiagnostic.Observed, Value: teardown.Death.ExitCode}, ObservedBeforeCleanup: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Observed, Value: true}, CleanupInduced: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Observed, Value: false}}
		}
		r := manageddiagnostic.Record{SessionID: sid, Generation: generation, Sequence: sequence, Phase: phase, Substep: substep, Terminal: terminal, Reason: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: reason}, Request: manageddiagnostic.RequestFacts{Method: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: "initialize"}, TargetID: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Unavailable}, CallerID: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Unavailable}, OwnerSequence: manageddiagnostic.Fact[uint64]{Status: manageddiagnostic.Observed, Value: sequence}, ProtocolID: manageddiagnostic.Fact[uint64]{Status: manageddiagnostic.Observed, Value: 1}}, Read: manageddiagnostic.IOFacts{State: manageddiagnostic.IOFailed}, Write: manageddiagnostic.IOFacts{State: manageddiagnostic.IOAttempted}, CallHierarchy: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Unavailable}, DocumentSupplyCompleted: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Unavailable}, ProcessExit: exit, Stderr: manageddiagnostic.Stderr{Status: manageddiagnostic.Withheld, ObservedByteCount: manageddiagnostic.Fact[int64]{Status: manageddiagnostic.Observed, Value: int64(len(teardown.Death.Stderr.Bytes))}, Truncated: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Observed, Value: teardown.Death.Stderr.Truncated}}}
		m.diagnostics.Record(r)
	}
}

func (m *Manager) abortReadiness(child Child, id string, failure session.Failure) {
	_ = child.Teardown(context.Background())
	_ = child.Close()
	m.finishReadiness(id, ReadinessFailed, failure, SessionMetadata{})
}

func (m *Manager) recordReadinessEvent(id string, code uint16, count int64, flag bool) {
	m.mu.Lock()
	op := m.readiness[id]
	var collector *manageddiagnostic.EventCollector
	if op != nil {
		collector = op.diagnostic
	}
	m.mu.Unlock()
	collector.Record(code, count, flag)
}

func (m *Manager) terminalReadinessEvent(id string, code uint16) {
	m.mu.Lock()
	op := m.readiness[id]
	var collector *manageddiagnostic.EventCollector
	if op != nil {
		collector = op.diagnostic
	}
	m.mu.Unlock()
	if op != nil {
		m.completeDiagnosticOperation(op.snapshot.DiagnosticOperation, collector, code)
	}
}

func (m *Manager) recordReadinessRequest(id string, bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.readiness[id]; op != nil && op.snapshot.State == ReadinessPending {
		op.snapshot.RequestMessages++
		op.snapshot.RequestBytes += bytes
	}
}

func (m *Manager) recordReadinessResponse(id string, bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.readiness[id]; op != nil && op.snapshot.State == ReadinessPending {
		op.snapshot.ResponseMessages++
		op.snapshot.ResponseBytes += bytes
	}
}

func (m *Manager) recordSuccessfulReadiness(id string, initializeBytes, initializedBytes int64, metadata SessionMetadata) {
	m.mu.Lock()
	op := m.readiness[id]
	if op == nil {
		m.mu.Unlock()
		return
	}
	s := op.snapshot
	m.mu.Unlock()
	if m.diagnostics == nil {
		return
	}
	r := manageddiagnostic.Record{SessionID: s.SessionID, Generation: s.Generation, Sequence: 1, Phase: manageddiagnostic.PhaseReadinessComplete, Substep: manageddiagnostic.Fact[manageddiagnostic.Substep]{Status: manageddiagnostic.Observed, Value: manageddiagnostic.SubstepInitializedNotification}, Terminal: manageddiagnostic.TerminalResponseReceived, Reason: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: "readiness-complete"}, Request: manageddiagnostic.RequestFacts{Method: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: "initialize"}, OwnerSequence: manageddiagnostic.Fact[uint64]{Status: manageddiagnostic.Observed, Value: 1}}, Read: manageddiagnostic.IOFacts{State: manageddiagnostic.IOComplete, Messages: s.ResponseMessages, Bytes: s.ResponseBytes}, Write: manageddiagnostic.IOFacts{State: manageddiagnostic.IOComplete, Messages: 2, Bytes: initializeBytes + initializedBytes}, CallHierarchy: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Observed, Value: metadata.CallHierarchySupport}, DocumentSupplyCompleted: manageddiagnostic.Fact[bool]{Status: manageddiagnostic.Unavailable}, ProcessExit: manageddiagnostic.ProcessExit{Status: manageddiagnostic.Unavailable}, Stderr: manageddiagnostic.Stderr{Status: manageddiagnostic.Withheld}}
	m.diagnostics.Record(r)
}

func (m *Manager) finishReadiness(id string, state ReadinessState, failure session.Failure, metadata SessionMetadata) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op := m.readiness[id]
	if op == nil || op.snapshot.State != ReadinessPending {
		return
	}
	op.snapshot.State, op.snapshot.Failure, op.snapshot.Metadata = state, failure, metadata
	op.snapshot.Duration = m.now().Sub(op.started)
	if op.snapshot.Duration < 0 {
		op.snapshot.Duration = 0
	}
	if r := m.sessions[op.snapshot.SessionID]; r != nil && r.record.Generation == op.snapshot.Generation {
		r.metadata = metadata
		r.protocolOwned = false
		if state == ReadinessReady {
			result := m.algebra.ObserveInitialization(op.snapshot.SessionID, op.snapshot.Generation, true)
			r.record.State = result.State
			m.observe(op.snapshot.SessionID, op.snapshot.Generation, "readiness", result.State, result.Failure)
		} else {
			result := m.algebra.ObserveInitialization(op.snapshot.SessionID, op.snapshot.Generation, false)
			r.record.State = result.State
			m.observe(op.snapshot.SessionID, op.snapshot.Generation, "readiness", result.State, failure)
		}
	}
	close(op.done)
	m.workers--
	select {
	case m.workerDone <- struct{}{}:
	default:
	}
}

// GetStartupAttempt returns only the exact retained startup-attempt observation.
func (m *Manager) GetStartupAttempt(id manageddiagnostic.StartupAttemptID) manageddiagnostic.StartupAttemptQuery {
	if m.diagnostics == nil {
		return manageddiagnostic.StartupAttemptQuery{Status: manageddiagnostic.AttemptUnavailable}
	}
	return m.diagnostics.StartupAttempt(id)
}

// Diagnostics returns only records retained for the exact identity and generation.
func (m *Manager) Diagnostics(sessionID string, generation uint64) manageddiagnostic.QueryResult {
	if m.diagnostics == nil {
		return manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}
	}
	return m.diagnostics.Query(sessionID, generation)
}

// RecordCapabilityCheck is a typed internal hook for operation owners outside runtime.
func (m *Manager) RecordCapabilityCheck(observation manageddiagnostic.CapabilityObservation) bool {
	if m.diagnostics == nil {
		return false
	}
	return m.diagnostics.Record(manageddiagnostic.RecordCapabilityCheck(observation))
}

func (m *Manager) Readiness(id string) (ReadinessSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op, ok := m.readiness[id]
	if !ok {
		return ReadinessSnapshot{}, false
	}
	return op.snapshot, true
}

func (m *Manager) WaitReadiness(ctx context.Context, id string) (ReadinessSnapshot, bool) {
	m.mu.Lock()
	op, ok := m.readiness[id]
	if !ok {
		m.mu.Unlock()
		return ReadinessSnapshot{}, false
	}
	done := op.done
	snapshot := op.snapshot
	m.mu.Unlock()
	if snapshot.State != ReadinessPending {
		return snapshot, true
	}
	select {
	case <-done:
		return m.Readiness(id)
	case <-ctx.Done():
		return snapshot, true
	}
}

func (m *Manager) ObserveInitialization(id string, generation uint64, complete bool) session.LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return session.LifecycleResult{Failure: session.SessionNotFound}
	}
	result := m.algebra.ObserveInitialization(id, generation, complete)
	r.record.State = result.State
	kind := "initialization"
	if !complete {
		kind = "poison"
	}
	m.observe(id, generation, kind, result.State, result.Failure)
	return result
}

func (m *Manager) BeginRequest(id string, generation uint64, deadline time.Time) (lspwire.RequestKey, session.Failure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return lspwire.RequestKey{}, session.SessionNotFound
	}
	if generation != r.record.Generation {
		return lspwire.RequestKey{}, session.StaleGeneration
	}
	if len(r.requests) >= m.limits.MaxRequests {
		return lspwire.RequestKey{}, session.ResourceExhausted
	}
	key := r.pending.Begin(generation)
	r.requests[key] = &Request{Key: key, Deadline: deadline}
	m.observe(id, generation, "request", r.record.State, "")
	return key, ""
}

func (m *Manager) CancelRequest(id string, key lspwire.RequestKey) (lspwire.CancelState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil || key.Generation != r.record.Generation {
		return lspwire.CancelNotPending, nil
	}
	if r.protocolOwned {
		return lspwire.CancelNotPending, errors.New("sessionruntime: protocol stream exclusively owned; cancel the operation context")
	}
	child, ok := r.process.(wireChild)
	if !ok {
		return lspwire.CancelNotPending, errors.New("sessionruntime: active child has no LSP wire")
	}
	state, err := r.pending.Cancel(lspwire.NewWriter(child.Stdin(), m.wire), key)
	if state == lspwire.CancelWritten {
		r.cancels++
		m.observe(id, key.Generation, "cancel", r.record.State, session.RequestCancelled)
	}
	return state, err
}

func (m *Manager) CompleteResponse(id string, key lspwire.ResponseKey) lspwire.ResponseDisposition {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return lspwire.ResponseUnknown
	}
	d := r.pending.Accept(key)
	if d == lspwire.ResponseAccepted {
		delete(r.requests, key)
		m.observe(id, key.Generation, "response", r.record.State, "")
	}
	return d
}

func (m *Manager) Expire(now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, r := range m.sessions {
		for key, request := range r.requests {
			if request.Terminal == "" && !request.Deadline.IsZero() && !now.Before(request.Deadline) {
				request.Terminal = session.RequestTimeout
				delete(r.requests, key)
				n++
				m.observe(id, key.Generation, "deadline", r.record.State, session.RequestTimeout)
			}
		}
	}
	return n
}

func (m *Manager) ObserveCrash(id string, generation uint64) session.Failure {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return session.SessionNotFound
	}
	if generation != r.record.Generation {
		return session.StaleGeneration
	}
	r.record.State = session.Crashed
	m.observe(id, generation, "crash", session.Crashed, session.SessionCrashed)
	return ""
}

func (m *Manager) Poison(id string, generation uint64) session.LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return session.LifecycleResult{Failure: session.SessionNotFound}
	}
	result := m.algebra.ObservePoison(id, generation)
	r.record.State = result.State
	m.observe(id, generation, "poison", result.State, result.Failure)
	return result
}

func (m *Manager) Stop(ctx context.Context, id, caller string) session.LifecycleResult {
	return m.terminate(ctx, id, caller, false)
}

func (m *Manager) Restart(ctx context.Context, id, caller string) session.LifecycleResult {
	return m.terminate(ctx, id, caller, true)
}

func (m *Manager) terminate(_ context.Context, id, caller string, restart bool) session.LifecycleResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return session.LifecycleResult{Failure: session.SessionNotFound}
	}
	if m.closed || (r.protocolOwned && !r.lifecycleOwned) {
		return session.LifecycleResult{State: r.record.State, Generation: r.record.Generation, Failure: session.LifecycleConflict}
	}
	if readinessID := m.readinessIDs[id+":"+strconv.FormatUint(r.record.Generation, 10)]; readinessID != "" {
		if readiness := m.readiness[readinessID]; readiness != nil && readiness.snapshot.State == ReadinessPending {
			return session.LifecycleResult{State: r.record.State, Generation: r.record.Generation, Failure: session.LifecycleConflict}
		}
	}
	op := session.LifecycleStop
	kind := "teardown"
	if restart {
		op, kind = session.LifecycleRestart, "restart"
	}
	for i := len(m.operationIDs) - 1; i >= 0; i-- {
		existing := m.operations[m.operationIDs[i]]
		if existing.SessionID != id || existing.Generation != r.record.Generation || existing.Restart != restart {
			continue
		}
		if existing.CallerID == caller {
			return session.LifecycleResult{State: r.record.State, Generation: existing.Generation, IntentID: existing.ID, Replayed: true}
		}
		if existing.State == OperationPending {
			// Joining an accepted operation consumes no new runtime capacity. Let the
			// algebra bound and record the distinct caller against that exact intent.
			return m.algebra.Lifecycle(session.LifecycleRequest{SessionID: id, Generation: r.record.Generation, Operation: op, CallerID: caller, ChildRisk: true, HasWork: len(r.requests) > 0})
		}
	}
	// Runtime capacity is reserved before the session algebra can install or join
	// an intent. Holding m.mu makes the cross-layer admission indivisible to other
	// runtime callers. Non-new algebra results release the provisional worker slot.
	if m.workers >= m.limits.MaxChildren || !m.canReserveOperation() {
		return session.LifecycleResult{Failure: session.ResourceExhausted}
	}
	m.workers++
	intent := m.algebra.Lifecycle(session.LifecycleRequest{SessionID: id, Generation: r.record.Generation, Operation: op, CallerID: caller, ChildRisk: true, HasWork: len(r.requests) > 0})
	if intent.Failure != "" || intent.Noop || intent.Joined || intent.Replayed {
		m.workers--
		return intent
	}
	if !m.reserveOperation(intent.IntentID) {
		// canReserveOperation and reserveOperation run under the same lock, so this
		// is defensive only: no concurrent runtime mutation can consume capacity.
		m.workers--
		return session.LifecycleResult{Failure: session.ResourceExhausted}
	}
	snapshot := OperationSnapshot{ID: intent.IntentID, SessionID: id, CallerID: caller, Generation: r.record.Generation, Restart: restart, State: OperationPending}
	m.operations[snapshot.ID] = snapshot
	m.operationIDs = append(m.operationIDs, snapshot.ID)
	r.protocolOwned, r.lifecycleOwned = true, true
	m.observe(id, snapshot.Generation, kind, intent.State, "")
	go m.runLifecycle(snapshot, r.process, r.pending, r.spec, r.retired)
	return intent
}

func (m *Manager) canReserveOperation() bool {
	if len(m.operations) < m.limits.MaxOperations {
		return true
	}
	for _, id := range m.operationIDs {
		if m.operations[id].State != OperationPending {
			return true
		}
	}
	return false
}

func (m *Manager) reserveOperation(id string) bool {
	if _, exists := m.operations[id]; exists {
		return true
	}
	for len(m.operations) >= m.limits.MaxOperations {
		removed := false
		for i, candidate := range m.operationIDs {
			if m.operations[candidate].State != OperationPending {
				delete(m.operations, candidate)
				m.operationIDs = append(m.operationIDs[:i], m.operationIDs[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			return false
		}
	}
	return true
}

func (m *Manager) runLifecycle(operation OperationSnapshot, child Child, pending *lspwire.Pending, spec managedprocess.Spec, retired *ownedTransport) {
	shutdownComplete := true
	var teardown managedprocess.TeardownObservation
	var resources managedprocess.ResourceObservation
	if retired != nil {
		// There is no live protocol stream left to shut down. Reuse confirmed
		// forced-retirement observations; this is not a shutdown acknowledgment.
		teardown, resources = retired.teardown, retired.resources
	} else {
		if protocol, ok := child.(wireChild); ok {
			shutdownComplete = m.gracefulShutdown(protocol, pending, operation.Generation)
		}
		teardown = child.Teardown(context.Background())
		resources = child.Close()
	}
	death := teardown.Death.Reap.Kind == managedprocess.ReapComplete
	observed := session.LifecycleCompletion{ShutdownComplete: shutdownComplete, UnsafeIOAbsent: resources.Kind == managedprocess.ResourcesClosed, TerminateSucceeded: death, DeathObserved: death, NoContainedSurvivors: death, StderrDrainComplete: true, Reaped: death, InitializationPending: true}

	m.mu.Lock()
	completed := m.algebra.CompleteLifecycleObserved(operation.SessionID, operation.ID, observed)
	r := m.sessions[operation.SessionID]
	m.observe(operation.SessionID, operation.Generation, "teardown", completed.State, completed.Failure)
	if completed.Failure != "" || !death || resources.Kind != managedprocess.ResourcesClosed {
		operation.State, operation.Failure = OperationFailed, completed.Failure
		if operation.Failure == "" {
			operation.Failure = session.SessionReapIncomplete
		}
		if r != nil {
			r.record.State = session.Poisoned
			r.protocolOwned, r.lifecycleOwned = false, false
		}
		m.finishOperation(operation)
		m.mu.Unlock()
		return
	}
	if !operation.Restart {
		if !m.algebra.ReleaseStopped(operation.SessionID, operation.Generation) {
			operation.State, operation.Failure = OperationFailed, session.LifecycleConflict
			if r != nil {
				r.record.State = session.Poisoned
				r.protocolOwned, r.lifecycleOwned = false, false
			}
			m.finishOperation(operation)
			m.mu.Unlock()
			return
		}
		delete(m.sessions, operation.SessionID)
		operation.State = OperationComplete
		m.finishOperation(operation)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	attemptID, attemptSequence, attemptStarted := m.beginStartupAttempt()
	identity := managedprocess.ObserveIdentity(spec, r.record.Profile.ProfileName(), r.record.Profile.Workspace().String())
	next, start := m.starter.Start(context.Background(), spec)
	m.mu.Lock()
	if start.Kind != managedprocess.StartStarted || next == nil || r == nil {
		operation.State, operation.Failure = OperationFailed, session.SpawnFailure
		m.finishStartupAttempt(attemptID, attemptSequence, attemptStarted, StartResult{SessionID: operation.SessionID, Failure: session.SpawnFailure, Start: start})
		if r != nil {
			r.record.State = session.Poisoned
		}
		m.observe(operation.SessionID, completed.Generation, "poison", session.Poisoned, session.SpawnFailure)
		m.finishOperation(operation)
		m.mu.Unlock()
		return
	}
	r.process, r.record.Generation, r.record.State = next, completed.Generation, session.Initializing
	r.attemptID = attemptID
	r.identity = identity
	r.diagnosticGeneration = m.newDiagnosticGeneration(attemptID, operation.SessionID, completed.Generation)
	m.finishStartupAttempt(attemptID, attemptSequence, attemptStarted, StartResult{SessionID: operation.SessionID, Generation: completed.Generation, State: session.Initializing, Start: start})
	r.retired = nil
	r.pending, r.requests, r.documents, r.cancels, r.protocolOwned, r.lifecycleOwned = lspwire.NewPending(m.limits.MaxTombstones), make(map[lspwire.RequestKey]*Request), make(map[string]openDocument), 0, false, false
	r.seedAdmittedGeneration = 0
	m.observe(operation.SessionID, completed.Generation, "startup", session.Starting, "")
	m.observe(operation.SessionID, completed.Generation, "initialization", session.Initializing, "")
	m.mu.Unlock()
	if _, ok := next.(wireChild); ok {
		readiness := m.BeginReadiness(context.Background(), operation.SessionID, completed.Generation, time.Time{})
		observed, _ := m.WaitReadiness(context.Background(), readiness.ID)
		if observed.State != ReadinessReady {
			operation.State, operation.Failure = OperationFailed, observed.Failure
		} else {
			operation.State = OperationComplete
		}
	} else {
		operation.State = OperationComplete
	}
	m.mu.Lock()
	m.finishOperation(operation)
	m.mu.Unlock()
}

func (m *Manager) gracefulShutdown(child wireChild, pending *lspwire.Pending, generation uint64) bool {
	key := pending.Begin(generation)
	id := strconv.FormatUint(key.ID, 10)
	writer := lspwire.NewWriter(child.Stdin(), m.wire)
	if err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(id), Method: "shutdown"}); err != nil {
		return false
	}
	response := make(chan error, 1)
	go func() {
		message, err := lspwire.NewReader(child.Stdout(), m.wire).Read()
		if err == nil && (message.Kind() != lspwire.KindSuccessResponse || string(message.ID) != id || pending.Accept(key) != lspwire.ResponseAccepted) {
			err = errors.New("sessionruntime: invalid shutdown response")
		}
		response <- err
	}()
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case err := <-response:
		if err != nil {
			return false
		}
	case <-timer.C:
		return false
	}
	return writer.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "exit"}) == nil
}

func (m *Manager) finishOperation(operation OperationSnapshot) {
	m.operations[operation.ID] = operation
	m.workers--
	select {
	case m.workerDone <- struct{}{}:
	default:
	}
}

func (m *Manager) Operation(id string) (OperationSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot, ok := m.operations[id]
	return snapshot, ok
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	for m.workers > 0 {
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.workerDone:
		}
		m.mu.Lock()
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) Census() Census {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := Census{Sessions: len(m.sessions), Generations: len(m.sessions), Children: len(m.sessions), Observations: len(m.observations), Operations: len(m.operations), Workers: m.workers}
	for _, r := range m.sessions {
		c.Requests += len(r.requests)
		c.Cancels += r.cancels
		c.Tombstones += r.pending.TombstoneCount()
	}
	return c
}
func (m *Manager) Metadata(id string, generation uint64) (SessionMetadata, session.Failure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[id]
	if r == nil {
		return SessionMetadata{}, session.SessionNotFound
	}
	if generation != r.record.Generation {
		return SessionMetadata{}, session.StaleGeneration
	}
	if r.record.State != session.Ready || r.protocolOwned {
		return SessionMetadata{}, session.LifecycleConflict
	}
	return r.metadata, ""
}

func (m *Manager) Records() []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Record, 0, len(m.sessions))
	for _, r := range m.sessions {
		out = append(out, r.record)
	}
	return out
}
func (m *Manager) Observations() []Observation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Observation(nil), m.observations...)
}
func (m *Manager) observe(id string, generation uint64, kind string, state session.State, failure session.Failure) {
	m.sequence++
	m.observations = append(m.observations, Observation{Sequence: m.sequence, SessionID: id, Generation: generation, Kind: kind, State: state, Failure: failure})
	if over := len(m.observations) - m.limits.MaxObservations; over > 0 {
		copy(m.observations, m.observations[over:])
		m.observations = m.observations[:m.limits.MaxObservations]
	}
}
