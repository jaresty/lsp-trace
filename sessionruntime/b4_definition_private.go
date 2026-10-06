package sessionruntime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path/filepath"
	"sync/atomic"
	"unsafe"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

// B4DefinitionOwner names a private pre-write declaration. Labels alone do not
// authenticate their declarant, the server, or a response.
type B4DefinitionOwner struct {
	Transaction       string
	CompletedOwnerKey string
	TargetSources     []B4DefinitionSourceLease
}

// B4DefinitionSourceReference identifies an exact Manager-held prepared document.
// It has no source-content, digest, or caller acquisition-identity fields.
type B4DefinitionSourceReference struct {
	SessionID       string
	Generation      uint64
	URI             string
	DocumentVersion int
}

// B4DefinitionSourceLease is opaque, Manager-local, and single-transfer.
type B4DefinitionSourceLease struct{ state *privateB4SourceLease }

// B4DefinitionTargetSource is output-only Manager custody metadata. It records
// exact supplied bytes without claiming that the language server analyzed them.
// AcquisitionID is retained for compatibility only: it is a Manager-generated
// opaque issuance/correlation identity, not caller acquisition provenance.
type B4DefinitionTargetSource struct {
	URI, AcquisitionID, SHA256 string
	SessionID                  string
	Generation                 uint64
	DocumentVersion            int
	Bytes                      []byte
}

// B4DefinitionSelectionKey identifies the complete manager-owned transaction;
// a numeric wire ID by itself is not a selection key.
type B4DefinitionSelectionKey struct {
	SessionID         string
	Key               lspwire.RequestKey
	Transaction       string
	CompletedOwnerKey string
}

// B4DefinitionLease is a non-serializable, manager-local bearer capability.
type B4DefinitionLease struct{ token [32]byte }

type PrivateB4Status string

const (
	PrivateB4Unavailable            PrivateB4Status = "PRIVATE_B4_UNAVAILABLE"
	PrivateB4Selected               PrivateB4Status = "PRIVATE_B4_SELECTED"
	privateB4MaxSlots                               = 4
	privateB4MaxBytes               int64           = 16 << 20
	privateB4MaxSourceDocuments                     = 256
	privateB4MaxSourceDocumentBytes                 = 4 << 20
)

type privateB4SourceDocumentKey struct {
	URI             string
	SessionID       string
	Generation      uint64
	DocumentVersion int
	SHA256          string
}

type privateB4SourceAccountingSnapshot struct {
	DocumentsAcquired int
	AdmittedLengths   map[privateB4SourceDocumentKey]int
}

// PrivateB4DefinitionCapture is output-only; no caller-provided response is accepted.
type PrivateB4DefinitionCapture struct {
	SessionID           string
	Key                 lspwire.RequestKey
	Transaction         string
	CompletedOwnerKey   string
	Method              string
	RequestFrame        []byte
	RequestParams       []byte
	ResponseFrame       []byte
	Result              []byte
	RequestFrameSHA256  string
	ResponseFrameSHA256 string
	ResultSHA256        string
	QueryOccurrenceID   string
	TargetSources       []B4DefinitionTargetSource
}

// privateB4ReservationC16 is a distinct versioned representation. It is never
// embedded in privateB4Reservation, preserving the frozen legacy layout.
type privateB4ReservationC16 struct {
	legacy  *privateB4Reservation
	profile *privateB4AccountC16
}

type privateB4Transaction struct {
	legacy *privateB4Reservation
	c16    *privateB4ReservationC16
}

type privateB4Reservation struct {
	token                 [32]byte
	selection             B4DefinitionSelectionKey
	session               *runtimeSession
	maxCharge             int64
	slot                  int
	sourceLeases          []B4DefinitionSourceLease
	history               *privateReadinessHistory
	historyMetadata       *lspwire.ImmutableHistoryMetadata
	historyBorrowed       bool
	capture               PrivateB4DefinitionCapture
	decodeEntry           func()
	retainEntry           func()
	method                string
	nonCustodial          bool
	documentsAcquired     int
	admittedLengths       map[privateB4SourceDocumentKey]int
	explicitBytes         *privateB4ByteAccountV2
	explicitLeases        [privateB4ByteSlots]privateB4ByteLeaseV2
	explicitLeaseCount    int
	requestFrameCapture   privateB4ByteLeaseV2
	requestFrameRetained  privateB4ByteLeaseV2
	requestFrameCaller    privateB4ByteLeaseV2
	sourceDescriptor      privateB4DescriptorLeaseV2
	responseFrameOwner    *privateB4ResponseFrameOwner
	resultOwner           *privateB4ResultOwner
	ownedMethodPair       *privateB4OwnedMethodPairOwner
	retainedNotifications *privateB4RetainedNotificationOwner
	serverError           *privateB4ServerErrorOwner
	retainedResponses     *privateB4RetainedResponseOwner
}

func (r *privateB4Reservation) reservePrivateB4ExplicitBytes(capacities []uint64) session.Failure {
	if r.explicitBytes == nil {
		account, failure := newPrivateB4ByteAccountV2()
		if failure != "" {
			return failure
		}
		r.explicitBytes = account
	}
	remaining := r.explicitLeases[r.explicitLeaseCount:]
	count, failure := r.explicitBytes.reserveMany(capacities, remaining)
	if failure != "" {
		return failure
	}
	r.explicitLeaseCount += count
	return ""
}

func (r *privateB4Reservation) reservePrivateB4RequestFrame(capacity uint64) session.Failure {
	var leases [3]privateB4ByteLeaseV2
	if _, failure := r.explicitBytes.reserveMany([]uint64{capacity, capacity, capacity}, leases[:]); failure != "" {
		return failure
	}
	r.requestFrameCapture, r.requestFrameRetained, r.requestFrameCaller = leases[0], leases[1], leases[2]
	return ""
}

func (r *privateB4Reservation) releasePrivateB4RequestFrameTemporary() {
	r.requestFrameCapture.release()
	r.requestFrameRetained.release()
}

func (r *privateB4Reservation) releasePrivateB4ExplicitBytes() {
	r.releasePrivateB4RequestFrameTemporary()
	r.requestFrameCaller.release()
	for i := 0; i < r.explicitLeaseCount; i++ {
		r.explicitLeases[i].release()
		r.explicitLeases[i] = privateB4ByteLeaseV2{}
	}
	r.explicitLeaseCount = 0
}

func (r *privateB4Reservation) admitPrivateB4SourceAccounting(key privateB4SourceDocumentKey, length int) session.Failure {
	if length < 0 || length > privateB4MaxSourceDocumentBytes {
		return session.ResourceExhausted
	}
	if admitted, ok := r.admittedLengths[key]; ok {
		if admitted != length {
			return session.ResourceExhausted
		}
		return ""
	}
	if r.documentsAcquired >= privateB4MaxSourceDocuments {
		return session.ResourceExhausted
	}
	if r.admittedLengths == nil {
		r.admittedLengths = make(map[privateB4SourceDocumentKey]int, privateB4MaxSourceDocuments)
	}
	r.admittedLengths[key] = length
	r.documentsAcquired++
	return ""
}

// privateB4SourceAccountingSnapshot is criterion-specific test evidence. It is
// Manager-private, contains no source bytes, and is not publication authority.
func (r *privateB4Reservation) privateB4SourceAccountingSnapshot() privateB4SourceAccountingSnapshot {
	lengths := make(map[privateB4SourceDocumentKey]int, len(r.admittedLengths))
	for key, length := range r.admittedLengths {
		lengths[key] = length
	}
	return privateB4SourceAccountingSnapshot{DocumentsAcquired: r.documentsAcquired, AdmittedLengths: lengths}
}

func (r *privateB4Reservation) releasePrivateB4SourceStorage() {
	// Detach reservation ownership without mutating captures already returned to
	// the caller. Those captures own their existing copies; copies do not recharge.
	r.capture.TargetSources = nil
	r.sourceLeases = nil
}

type privateB4ResponseCapture struct{ reservation *privateB4Reservation }

type privateB4SuccessorAllocationOwner struct{ account *privateB4ByteAccountV2 }
type privateB4SuccessorAllocationLease struct{ lease privateB4ByteLeaseV2 }

func (o privateB4SuccessorAllocationOwner) ReserveSuccessorAllocation(role lspwire.SuccessorAllocationRole, capacity uint64) (lspwire.SuccessorAllocationLease, error) {
	lease, failure := o.account.reserve(capacity)
	if failure != "" {
		return nil, fmt.Errorf("%s: role=%d capacity=%d", failure, role, capacity)
	}
	return &privateB4SuccessorAllocationLease{lease: lease}, nil
}

func (l *privateB4SuccessorAllocationLease) Release() { l.lease.release() }

type privateB4TransportReader struct{ io.Reader }

func (r privateB4TransportReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		err = fmt.Errorf("%w: %v", errPrivateB4Transport, err)
	}
	return n, err
}

func (c privateB4ResponseCapture) ObserveOriginalFrame(frame []byte) {
	c.reservation.capture.ResponseFrame = frame
	c.observeEntries()
}

func (c privateB4ResponseCapture) ObserveOriginalFrameOwned(frame []byte, lease lspwire.SuccessorAllocationLease) {
	if c.reservation.responseFrameOwner != nil {
		c.reservation.capture.ResponseFrame = nil
		c.reservation.responseFrameOwner.releaseManager()
	}
	owner := &privateB4ResponseFrameOwner{state: privateB4ResponseFrameOpen, managerHeld: true, bytes: frame, charge: lease}
	c.reservation.responseFrameOwner = owner
	c.reservation.capture.ResponseFrame = owner.bytes
	c.observeEntries()
}

func (c privateB4ResponseCapture) observeEntries() {
	if c.reservation.retainEntry != nil {
		c.reservation.retainEntry()
	}
	if c.reservation.decodeEntry != nil {
		c.reservation.decodeEntry()
	}
}

func (c privateB4ResponseCapture) ObserveOriginalResult(result []byte) {
	c.reservation.capture.Result = result
}

func (c privateB4ResponseCapture) ObserveOriginalResultOwned(result []byte, lease lspwire.SuccessorAllocationLease) {
	if previous := c.reservation.resultOwner; previous != nil {
		c.reservation.capture.Result = nil
		previous.releaseManager()
	}
	owner := &privateB4ResultOwner{state: privateB4ResultOpen, managerHeld: true, bytes: result, charge: lease}
	c.reservation.resultOwner = owner
	c.reservation.capture.Result = owner.bytes
}

func privateB4SuccessorOptions(reservation *privateB4Reservation) lspwire.SuccessorIngressOptions {
	complete := uint64(1048576)
	if reservation.method == "textDocument/references" {
		complete = 1572864
	}
	options := lspwire.SuccessorIngressOptions{
		OwnerID:              "sessionruntime.private-b4",
		History:              reservation.historyMetadata,
		Capture:              privateB4ResponseCapture{reservation: reservation},
		GovernServerRequests: !reservation.nonCustodial,
		Limits: lspwire.SuccessorIngressLimits{
			HeaderBytes: 65536, FrameBytes: 2097152, ConsumptionBytes: 8388608, AcquisitionBytes: 8392705,
			MaxReadBytes: 4096, PrefetchBytes: 4096, HistoryAcquiredBytes: 8388608, HistoryOutstandingBytes: 8388608,
			MessageAttempts: 64, CompleteMessageBytes: complete, ResultTokenBytes: 524288,
		},
	}
	if reservation.explicitBytes != nil && !reservation.nonCustodial {
		options.AllocationOwner = privateB4SuccessorAllocationOwner{account: reservation.explicitBytes}
	}
	return options
}

type privateB4Slot struct {
	occupied    bool
	token       [32]byte
	reservation *privateB4Reservation
	c16         *privateB4ReservationC16
}

const (
	privateB4SourceHeld uint32 = iota
	privateB4SourceTransferred
	privateB4SourceReleased
)

type privateB4SourceLease struct {
	manager *Manager
	session *runtimeSession
	source  B4DefinitionTargetSource
	state   atomic.Uint32
	ingress privateB4SourceIngressLease
}

// PreparePrivateB4DefinitionSource issues a capability only from the exact
// successful supply retained in the Manager's current open-document record.
func (m *Manager) PreparePrivateB4DefinitionSource(ref B4DefinitionSourceReference) (B4DefinitionSourceLease, PrivateB4Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.sessions[ref.SessionID]
	if r == nil || r.record.Generation != ref.Generation || r.record.State != session.Ready || m.closed {
		return B4DefinitionSourceLease{}, PrivateB4Unavailable
	}
	u, err := url.Parse(ref.URI)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.Path == "" || u.RawQuery != "" || u.Fragment != "" {
		return B4DefinitionSourceLease{}, PrivateB4Unavailable
	}
	path := filepath.Clean(filepath.FromSlash(u.Path))
	workspace := filepath.Clean(r.record.Profile.Workspace().String())
	rel, relErr := filepath.Rel(workspace, path)
	canonical := ref.URI == (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	inside := relErr == nil && rel != ".." && !filepath.IsAbs(rel) && !(len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator))
	document, ok := r.documents[ref.URI]
	if !canonical || !inside || !ok || document.version != ref.DocumentVersion || document.supply == nil ||
		document.supply.SessionID != ref.SessionID || document.supply.Generation != ref.Generation ||
		document.supply.URI != ref.URI || document.supply.DocumentVersion != ref.DocumentVersion ||
		document.digest != sha256.Sum256(document.supply.Content) {
		return B4DefinitionSourceLease{}, PrivateB4Unavailable
	}
	if m.privateB4SourceIngress == nil {
		var failure session.Failure
		m.privateB4SourceIngress, failure = newPrivateB4SourceIngressOwner()
		if failure != "" {
			return B4DefinitionSourceLease{}, PrivateB4Unavailable
		}
	}
	ingress, failure := m.privateB4SourceIngress.reserve(privateB4SourceIngressStateBytes, nil)
	if failure != "" {
		return B4DefinitionSourceLease{}, PrivateB4Unavailable
	}
	var identity [32]byte
	if _, err := io.ReadFull(rand.Reader, identity[:]); err != nil || identity == ([32]byte{}) {
		ingress.release()
		return B4DefinitionSourceLease{}, PrivateB4Unavailable
	}
	content := append([]byte(nil), document.supply.Content...)
	state := &privateB4SourceLease{manager: m, session: r, source: B4DefinitionTargetSource{
		URI: ref.URI, AcquisitionID: "sha256:" + fmtDigest(identity), SHA256: privateB4Hash(content),
		SessionID: ref.SessionID, Generation: ref.Generation, DocumentVersion: ref.DocumentVersion, Bytes: content,
	}, ingress: ingress}
	state.ingress.entry().state = state
	return B4DefinitionSourceLease{state: state}, PrivateB4Selected
}

func (m *Manager) ReleasePrivateB4DefinitionSource(lease B4DefinitionSourceLease) PrivateB4Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lease.state == nil || lease.state.manager != m || !lease.state.state.CompareAndSwap(privateB4SourceHeld, privateB4SourceReleased) {
		return PrivateB4Unavailable
	}
	entry := lease.state.ingress.entry()
	if entry == nil || entry.state != lease.state {
		lease.state.state.Store(privateB4SourceHeld)
		return PrivateB4Unavailable
	}
	lease.state.ingress.release()
	lease.state.source.Bytes = nil
	return PrivateB4Selected
}

// RoundTripPrivateB4 opts into a bounded private definition path. Its reservation
// is made under Manager.mu before Pending.Begin and before the actual WRITE.
// Ordinary RoundTrip and other methods never enter this path.
func (m *Manager) RoundTripPrivateB4(parent context.Context, req RoundTripRequest, owner B4DefinitionOwner) (RoundTripResult, B4DefinitionLease) {
	if req.Method != "textDocument/definition" || req.SessionID == "" || req.Generation == 0 ||
		owner.Transaction == "" || owner.CompletedOwnerKey == "" || req.MaxMessages <= 0 ||
		req.MaxBytes <= 0 || req.MaxBytes > 4<<20 || len(req.Params) == 0 || len(req.Params) > maxMethodFrameCorrespondenceBytes ||
		req.CaptureDefinitionResponseFrameMaxBytes <= 0 || req.CaptureDefinitionResponseFrameMaxBytes > maxMethodFrameCorrespondenceBytes ||
		req.CaptureMethodRequestFrameMaxBytes <= 0 || req.CaptureMethodRequestFrameMaxBytes > maxMethodFrameCorrespondenceBytes {
		return RoundTripResult{Failure: session.ToolNotImplemented}, B4DefinitionLease{}
	}
	account, accountFailure := newPrivateB4ByteAccountV2()
	if accountFailure != "" {
		return RoundTripResult{Failure: accountFailure}, B4DefinitionLease{}
	}
	reservation := &privateB4Reservation{
		selection:     B4DefinitionSelectionKey{Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey},
		explicitBytes: account,
	}
	transaction := &privateB4Transaction{legacy: reservation}
	if req.EnablePrivateC16ObjectAccounting {
		profile, failure := newPrivateB4AccountC16WithBytes(account)
		if failure != "" {
			account.requestTerminalRelease()
			return RoundTripResult{Failure: failure}, B4DefinitionLease{}
		}
		transaction.c16 = &privateB4ReservationC16{legacy: reservation, profile: profile}
	}
	terminalManaged := false
	defer func() {
		if !terminalManaged {
			reservation.sourceDescriptor.release()
			reservation.releasePrivateB4ExplicitBytes()
			if transaction.c16 != nil {
				_ = transaction.c16.profile.requestTerminalReleaseC16(context.Background())
			} else {
				account.requestTerminalRelease()
			}
		}
	}()
	if m.b4ParamsHook != nil {
		m.b4ParamsHook(reservation.explicitBytes)
	}
	// Refusal precedes the independent Manager-owned params allocation.
	params, paramsLease, failure := copyPrivateB4Params(reservation.explicitBytes, req.Params)
	if failure != "" {
		return RoundTripResult{Failure: failure}, B4DefinitionLease{}
	}
	defer paramsLease.release()
	req.Params = params
	// Reserve persistent handle-slice and capture-params backing before either
	// allocation. Source bytes are resolved only from exact live Manager-held
	// prepared document records while Manager.mu is held by admission.
	handleSize := uint64(unsafe.Sizeof(B4DefinitionSourceLease{}))
	if uint64(len(owner.TargetSources)) > math.MaxUint64/handleSize ||
		reservation.reservePrivateB4ExplicitBytes([]uint64{uint64(len(owner.TargetSources)) * handleSize, uint64(len(req.Params))}) != "" {
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	reservation.sourceLeases = make([]B4DefinitionSourceLease, len(owner.TargetSources))
	copy(reservation.sourceLeases, owner.TargetSources)
	if failure := reservation.reservePrivateB4RequestFrame(uint64(req.CaptureMethodRequestFrameMaxBytes)); failure != "" {
		return RoundTripResult{Failure: failure}, B4DefinitionLease{}
	}
	if _, err := io.ReadFull(rand.Reader, reservation.token[:]); err != nil || reservation.token == ([32]byte{}) {
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	req.ADR0011PrivateOwnedMethodPairLeaseV1 = req.CaptureOwnedMethodPair
	req.ADR0011PrivateNotificationLeaseV1 = true
	req.ADR0011PrivateServerErrorLeaseV1 = true
	req.ADR0011PrivateResponseLeaseV1 = true
	result := m.roundTripWithPrivateTransaction(parent, req, nil, transaction)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.privateB4ReservationLocked(reservation.token) != reservation {
		return result, B4DefinitionLease{}
	}
	terminalManaged = true
	if result.Failure == "" && result.privateB4ServerError != nil {
		result.privateB4ServerError.bindRetirement(m, reservation)
		return result, B4DefinitionLease{}
	}
	// A failed or unselected transaction never becomes a transferable lease.
	writeFrame, wrote := result.CompletedMethodRequestFrame()
	readFrame, read := result.CompletedDefinitionResponseFrame()
	if reservation.capture.ResponseFrame != nil {
		readFrame = reservation.capture.ResponseFrame
	}
	write, wroteObservation := result.CompletedRequestWrite()
	readObservation, readObserved := result.CompletedResponseRead()
	r := m.sessions[req.SessionID]
	privateResult := result.Result
	if reservation.resultOwner != nil {
		privateResult = reservation.capture.Result
		result.Result = privateResult
	}
	invalid := result.Failure != "" || result.ServerError != nil || !wrote || !read || !wroteObservation || !readObserved ||
		result.Key != reservation.selection.Key || write.SessionID != req.SessionID || write.Key != result.Key ||
		readObservation.SessionID != req.SessionID || readObservation.Key != result.Key ||
		VerifyMethodFrameCorrespondence(req, result) != nil || r == nil || r != reservation.session ||
		r.record.Generation != req.Generation || r.record.State != session.Ready || m.closed
	if reservation.resultOwner != nil {
		result.Result = nil
	}
	if invalid {
		if result.privateB4OwnedMethodPair != nil {
			(OwnedMethodPairLease{owner: result.privateB4OwnedMethodPair}).Release()
			result.privateB4OwnedMethodPair = nil
			reservation.ownedMethodPair = nil
		}
		if result.privateB4RetainedNotifications != nil {
			(RetainedNotificationLease{owner: result.privateB4RetainedNotifications}).Release()
			result.privateB4RetainedNotifications = nil
			reservation.retainedNotifications = nil
		}
		if result.privateB4ServerError != nil {
			(ServerErrorLease{owner: result.privateB4ServerError}).Release()
			result.privateB4ServerError = nil
			reservation.serverError = nil
		}
		if result.privateB4RetainedResponses != nil {
			(RetainedResponseLease{owner: result.privateB4RetainedResponses}).Release()
			result.privateB4RetainedResponses = nil
			reservation.retainedResponses = nil
		}
		m.releasePrivateB4Locked(reservation)
		return result, B4DefinitionLease{}
	}
	capture := PrivateB4DefinitionCapture{
		SessionID: req.SessionID, Key: result.Key, Transaction: owner.Transaction,
		CompletedOwnerKey: owner.CompletedOwnerKey, Method: req.Method,
		RequestFrame: writeFrame, RequestParams: append([]byte(nil), req.Params...),
		ResponseFrame: readFrame, Result: nil,
		RequestFrameSHA256: privateB4Hash(writeFrame), ResponseFrameSHA256: privateB4Hash(readFrame),
		ResultSHA256: privateB4Hash(privateResult), TargetSources: reservation.capture.TargetSources,
	}
	capture.QueryOccurrenceID = privateB4QueryOccurrenceID(capture)
	if !reservation.requestFrameCapture.shrinkExact(uint64(len(writeFrame))) ||
		!reservation.requestFrameRetained.shrinkExact(uint64(len(writeFrame))) ||
		!reservation.requestFrameCaller.shrinkExact(uint64(len(writeFrame))) {
		m.releasePrivateB4Locked(reservation)
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	reservation.releasePrivateB4RequestFrameTemporary()
	callerCharge := reservation.requestFrameCaller.transfer()
	if !callerCharge.lease.active {
		m.releasePrivateB4Locked(reservation)
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	reservation.requestFrameCaller = privateB4ByteLeaseV2{}
	result.privateB4RequestFrame = &privateB4RequestFrameOwner{state: privateB4RequestFrameOpen, bytes: writeFrame, charge: callerCharge}
	result.methodRequestFrame = nil
	if reservation.responseFrameOwner == nil || !reservation.responseFrameOwner.acquireCaller() {
		requestLease := B4RequestFrameLease{owner: result.privateB4RequestFrame}
		requestLease.Release()
		m.releasePrivateB4Locked(reservation)
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	result.privateB4ResponseFrame = reservation.responseFrameOwner
	if reservation.resultOwner == nil || !reservation.resultOwner.acquireCaller() {
		requestLease := B4RequestFrameLease{owner: result.privateB4RequestFrame}
		requestLease.Release()
		responseLease := B4ResponseFrameLease{owner: result.privateB4ResponseFrame}
		responseLease.Release()
		m.releasePrivateB4Locked(reservation)
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	result.privateB4Result = reservation.resultOwner
	// The reservation is intentionally conservative. Only owned copies survive
	// return; release of the reservation happens on first successful consumption.
	reservation.capture = capture
	return result, B4DefinitionLease{token: reservation.token}
}

func privateB4QueryOccurrenceID(c PrivateB4DefinitionCapture) string {
	h := sha256.New()
	put := func(b []byte) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(b)))
		_, _ = h.Write(n[:])
		_, _ = h.Write(b)
	}
	put([]byte(c.SessionID))
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], c.Key.Generation)
	put(n[:])
	binary.BigEndian.PutUint64(n[:], c.Key.ID)
	put(n[:])
	put([]byte(c.Method))
	put(c.RequestParams)
	put([]byte(c.Transaction))
	put([]byte(c.CompletedOwnerKey))
	return "sha256:" + fmtDigest(*(*[32]byte)(h.Sum(nil)))
}

func privateB4Hash(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + fmtDigest(h)
}

// fmtDigest avoids exporting the capture as part of a diagnostic message.
func fmtDigest(h [32]byte) string {
	const digits = "0123456789abcdef"
	var out [64]byte
	for i, b := range h {
		out[2*i], out[2*i+1] = digits[b>>4], digits[b&15]
	}
	return string(out[:])
}

func (m *Manager) admitPrivateB4SourcesLocked(req RoundTripRequest, reservation *privateB4Reservation) session.Failure {
	emptySlot := -1
	for i := range m.privateB4Leases {
		slot := &m.privateB4Leases[i]
		if slot.occupied {
			if slot.token == reservation.token {
				return session.ResourceExhausted
			}
			continue
		}
		if emptySlot < 0 {
			emptySlot = i
		}
	}
	if emptySlot < 0 {
		return session.ResourceExhausted
	}
	type admittedSource struct {
		key    privateB4SourceDocumentKey
		source B4DefinitionTargetSource
	}
	seenLease := make(map[*privateB4SourceLease]struct{}, len(reservation.sourceLeases))
	seenKey := make(map[privateB4SourceDocumentKey]struct{}, len(reservation.sourceLeases))
	admitted := make([]admittedSource, 0, len(reservation.sourceLeases))
	sourceBytes := 0
	for _, lease := range reservation.sourceLeases {
		s := lease.state
		if s == nil || s.manager != m || s.session != m.sessions[req.SessionID] || s.source.SessionID != req.SessionID ||
			s.source.Generation != req.Generation || s.state.Load() != privateB4SourceHeld || s.source.SHA256 != privateB4Hash(s.source.Bytes) {
			return session.ToolNotImplemented
		}
		document, ok := s.session.documents[s.source.URI]
		if !ok || document.version != s.source.DocumentVersion || document.supply == nil ||
			document.digest != sha256.Sum256(s.source.Bytes) || document.digest != sha256.Sum256(document.supply.Content) {
			return session.ToolNotImplemented
		}
		seenLease[s] = struct{}{}
		key := privateB4SourceDocumentKey{URI: s.source.URI, SessionID: s.source.SessionID, Generation: s.source.Generation,
			DocumentVersion: s.source.DocumentVersion, SHA256: s.source.SHA256}
		if _, ok := seenKey[key]; ok {
			continue
		}
		if len(s.source.Bytes) > privateB4MaxSourceDocumentBytes || len(admitted) >= privateB4MaxSourceDocuments {
			return session.ResourceExhausted
		}
		seenKey[key] = struct{}{}
		admitted = append(admitted, admittedSource{key: key, source: s.source})
		sourceBytes += len(s.source.Bytes)
	}
	charge := 2 * (req.MaxBytes + req.CaptureDefinitionResponseFrameMaxBytes + req.CaptureMethodRequestFrameMaxBytes + int64(len(req.Params)+sourceBytes) + 256)
	if charge > privateB4MaxBytes || charge > privateB4MaxBytes-m.privateB4Bytes {
		return session.ResourceExhausted
	}
	newDocuments := 0
	for _, source := range admitted {
		if length, ok := reservation.admittedLengths[source.key]; ok {
			if length != len(source.source.Bytes) {
				return session.ResourceExhausted
			}
		} else {
			newDocuments++
		}
	}
	if newDocuments > privateB4MaxSourceDocuments-reservation.documentsAcquired {
		return session.ResourceExhausted
	}
	var sourceCapacities [privateB4MaxSourceDocuments]uint64
	for i := range admitted {
		sourceCapacities[i] = uint64(len(admitted[i].source.Bytes))
	}
	if failure := reservation.reservePrivateB4ExplicitBytes(sourceCapacities[:len(admitted)]); failure != "" {
		return failure
	}
	for _, source := range admitted {
		if failure := reservation.admitPrivateB4SourceAccounting(source.key, len(source.source.Bytes)); failure != "" {
			panic("private B4 source-accounting preflight diverged")
		}
	}
	owned := make([]B4DefinitionTargetSource, len(admitted))
	for i, source := range admitted {
		owned[i] = source.source
		owned[i].Bytes = append([]byte(nil), source.source.Bytes...)
	}
	reservation.maxCharge = charge
	reservation.slot = emptySlot
	reservation.selection.SessionID = req.SessionID
	reservation.selection.Key.Generation = req.Generation
	reservation.session = m.sessions[req.SessionID]
	reservation.capture.TargetSources = owned
	descriptor, failure := m.privateB4SourceIngress.transferStates(seenLease, reservation.explicitBytes)
	if failure != "" {
		return failure
	}
	reservation.sourceDescriptor = descriptor
	for source := range seenLease {
		source.state.Store(privateB4SourceTransferred)
	}
	return ""
}

// installPrivateB4Locked is called only after admitPrivateB4SourcesLocked has
// preflighted every refusal and transferred every source lease under Manager.mu.
// It deliberately has no failure result: once transfer begins, installation cannot
// refuse and therefore cannot strand transferred source capabilities.
func (m *Manager) installPrivateB4Locked(_ RoundTripRequest, _ *runtimeSession, reservation *privateB4Reservation) {
	m.privateB4Leases[reservation.slot] = privateB4Slot{occupied: true, token: reservation.token, reservation: reservation}
	m.privateB4Bytes += reservation.maxCharge
}

func (m *Manager) installPrivateB4TransactionLocked(_ RoundTripRequest, _ *runtimeSession, transaction *privateB4Transaction) {
	reservation := transaction.legacy
	m.privateB4Leases[reservation.slot] = privateB4Slot{occupied: true, token: reservation.token, reservation: reservation, c16: transaction.c16}
	m.privateB4Bytes += reservation.maxCharge
}

func (m *Manager) privateB4ReservationLocked(token [32]byte) *privateB4Reservation {
	for i := range m.privateB4Leases {
		slot := &m.privateB4Leases[i]
		if slot.occupied && slot.token == token {
			return slot.reservation
		}
	}
	return nil
}

func (m *Manager) privateB4LeaseCountLocked() int {
	count := 0
	for i := range m.privateB4Leases {
		if m.privateB4Leases[i].occupied {
			count++
		}
	}
	return count
}

func (m *Manager) releasePrivateB4ServerErrorReservation(reservation *privateB4Reservation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releasePrivateB4Locked(reservation)
}

func (m *Manager) releasePrivateB4Locked(reservation *privateB4Reservation) {
	slot := &m.privateB4Leases[reservation.slot]
	if slot.occupied && slot.token == reservation.token && slot.reservation == reservation {
		if slot.c16 != nil {
			_ = slot.c16.profile.requestTerminalReleaseC16(context.Background())
		}
		if reservation.historyBorrowed && reservation.history != nil {
			reservation.history.releaseBorrowerLocked()
			reservation.historyBorrowed = false
		}
		for _, lease := range reservation.sourceLeases {
			if lease.state != nil && lease.state.manager == m && lease.state.state.CompareAndSwap(privateB4SourceTransferred, privateB4SourceReleased) {
				lease.state.source.Bytes = nil
			}
		}
		reservation.releasePrivateB4SourceStorage()
		reservation.sourceDescriptor.release()
		if reservation.responseFrameOwner != nil {
			reservation.capture.ResponseFrame = nil
			reservation.responseFrameOwner.releaseManager()
		}
		if reservation.resultOwner != nil {
			reservation.capture.Result = nil
			reservation.resultOwner.releaseManager()
		}
		reservation.releasePrivateB4ExplicitBytes()
		reservation.explicitBytes.requestTerminalRelease()
		*slot = privateB4Slot{}
		m.privateB4Bytes -= reservation.maxCharge
	}
}

func (m *Manager) hasPrivateB4CallerLeaseJoinLocked(r *runtimeSession) bool {
	if r == nil {
		return false
	}
	for i := range m.privateB4Leases {
		reservation := m.privateB4Leases[i].reservation
		if m.privateB4Leases[i].occupied && reservation != nil && reservation.session == r && (reservation.ownedMethodPair != nil || reservation.retainedNotifications != nil || reservation.serverError != nil || reservation.retainedResponses != nil) {
			return true
		}
	}
	return false
}

type privateB4RetirementJoin struct {
	pairOwners         [privateB4MaxSlots]*privateB4OwnedMethodPairOwner
	pairCount          int
	notificationOwners [privateB4MaxSlots]*privateB4RetainedNotificationOwner
	notificationCount  int
	serverErrorOwners  [privateB4MaxSlots]*privateB4ServerErrorOwner
	serverErrorCount   int
	responseOwners     [privateB4MaxSlots]*privateB4RetainedResponseOwner
	responseCount      int
	manager            *Manager
	c16Reservations    [privateB4MaxSlots]*privateB4Reservation
	c16Profiles        [privateB4MaxSlots]*privateB4ReservationC16
	c16Count           int
}

func (j *privateB4RetirementJoin) wait() {
	if j == nil {
		return
	}
	for i := 0; i < j.pairCount; i++ {
		j.pairOwners[i].waitReleased()
	}
	for i := 0; i < j.notificationCount; i++ {
		j.notificationOwners[i].waitReleased()
	}
	for i := 0; i < j.serverErrorCount; i++ {
		j.serverErrorOwners[i].waitReleased()
	}
	for i := 0; i < j.responseCount; i++ {
		j.responseOwners[i].waitReleased()
	}
	for i := 0; i < j.c16Count; i++ {
		reservation := j.c16Reservations[i]
		c16 := j.c16Profiles[i]
		if c16 != nil {
			_ = c16.profile.requestTerminalReleaseC16(context.Background())
		}
		j.manager.mu.Lock()
		j.manager.releasePrivateB4Locked(reservation)
		j.manager.mu.Unlock()
	}
}

// Called only under Manager.mu after successful old-generation teardown on
// completed STOP or RESTART. Repeated retirement cannot subtract an absent entry.
func (m *Manager) retirePrivateB4StoppedLocked(id string, generation uint64, stopped *runtimeSession) privateB4RetirementJoin {
	var join privateB4RetirementJoin
	join.manager = m
	if stopped == nil || stopped.record.Generation != generation {
		return join
	}
	stopped.readinessHistory.retireLocked()
	for i := range m.privateB4Leases {
		reservation := m.privateB4Leases[i].reservation
		if m.privateB4Leases[i].occupied && reservation.session == stopped && reservation.selection.SessionID == id && reservation.selection.Key.Generation == generation {
			if reservation.ownedMethodPair != nil {
				reservation.ownedMethodPair.requestTerminal()
				join.pairOwners[join.pairCount] = reservation.ownedMethodPair
				join.pairCount++
			}
			if reservation.retainedNotifications != nil {
				reservation.retainedNotifications.requestTerminal()
				join.notificationOwners[join.notificationCount] = reservation.retainedNotifications
				join.notificationCount++
			}
			if reservation.serverError != nil {
				reservation.serverError.requestTerminal()
				join.serverErrorOwners[join.serverErrorCount] = reservation.serverError
				join.serverErrorCount++
			}
			if reservation.retainedResponses != nil {
				reservation.retainedResponses.requestTerminal()
				join.responseOwners[join.responseCount] = reservation.retainedResponses
				join.responseCount++
			}
			if m.privateB4Leases[i].c16 != nil {
				join.c16Reservations[join.c16Count] = reservation
				join.c16Profiles[join.c16Count] = m.privateB4Leases[i].c16
				join.c16Count++
			} else {
				m.releasePrivateB4Locked(reservation)
			}
		}
	}
	return join
}

func (m *Manager) privateB4CaptureLocked(lease B4DefinitionLease, selection B4DefinitionSelectionKey) (*privateB4Reservation, PrivateB4DefinitionCapture, PrivateB4Status) {
	reservation := m.privateB4ReservationLocked(lease.token)
	if lease == (B4DefinitionLease{}) || reservation == nil || reservation.selection != selection ||
		reservation.capture.ResponseFrame == nil || m.closed {
		return nil, PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	r := m.sessions[selection.SessionID]
	if r == nil || r != reservation.session || r.record.Generation != selection.Key.Generation || r.record.State != session.Ready {
		return nil, PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	capture := reservation.capture
	var capacities [3 + privateB4MaxSourceDocuments]uint64
	capacities[0] = uint64(len(capture.RequestFrame))
	capacities[1] = uint64(len(capture.RequestParams))
	capacities[2] = uint64(len(capture.ResponseFrame))
	for i := range capture.TargetSources {
		capacities[3+i] = uint64(len(capture.TargetSources[i].Bytes))
	}
	if failure := reservation.reservePrivateB4ExplicitBytes(capacities[:3+len(capture.TargetSources)]); failure != "" {
		return nil, PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	capture.RequestFrame = append([]byte(nil), capture.RequestFrame...)
	capture.RequestParams = append([]byte(nil), capture.RequestParams...)
	capture.ResponseFrame = append([]byte(nil), capture.ResponseFrame...)
	capture.Result = nil
	capture.TargetSources = append([]B4DefinitionTargetSource(nil), capture.TargetSources...)
	for i := range capture.TargetSources {
		capture.TargetSources[i].Bytes = append([]byte(nil), capture.TargetSources[i].Bytes...)
	}
	return reservation, capture, PrivateB4Selected
}

// PrivateB4DefinitionBorrow is valid only for the dynamic extent of a borrowed
// callback. Result is read-only and must not be retained or mutated.
type PrivateB4DefinitionBorrow struct {
	Capture PrivateB4DefinitionCapture
	Result  []byte

	c16 *privateB4ReservationC16
}

// WithObjectAdmission admits one parser-owned C16 materialization. Copies of
// an enabled borrow share the same versioned profile.
func (b PrivateB4DefinitionBorrow) WithObjectAdmission(materialize func() error) error {
	if b.c16 == nil || b.c16.profile == nil {
		return ErrPrivateB4C16NotEnabled
	}
	return b.c16.profile.WithObjectAdmission(materialize)
}

// PreparePrivateB4DefinitionBorrowed presents the manager-owned lexical result
// without consuming the lease or copying the result backing.
func (m *Manager) PreparePrivateB4DefinitionBorrowed(lease B4DefinitionLease, selection B4DefinitionSelectionKey, prepare func(PrivateB4DefinitionBorrow) bool) (PrivateB4DefinitionCapture, PrivateB4Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	reservation, capture, status := m.privateB4CaptureLocked(lease, selection)
	if status != PrivateB4Selected || reservation.resultOwner == nil || prepare == nil {
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	result, ok := reservation.resultOwner.managerBytes()
	if !ok || !prepare(PrivateB4DefinitionBorrow{Capture: capture, Result: result}) {
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	return capture, PrivateB4Selected
}

// CommitPrivateB4DefinitionBorrowed preserves the atomic Manager.mu
// selection/prepare/publication interval without copying the manager-owned
// lexical result token.
func (m *Manager) CommitPrivateB4DefinitionBorrowed(lease B4DefinitionLease, selection B4DefinitionSelectionKey, prepare func(PrivateB4DefinitionBorrow) bool, publish func(PrivateB4DefinitionBorrow)) (PrivateB4DefinitionCapture, PrivateB4Status) {
	m.mu.Lock()
	reservation, capture, status := m.privateB4CaptureLocked(lease, selection)
	if status != PrivateB4Selected || reservation.resultOwner == nil || prepare == nil || publish == nil {
		m.mu.Unlock()
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	result, ok := reservation.resultOwner.managerBytes()
	if !ok {
		m.mu.Unlock()
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	c16 := m.privateB4Leases[reservation.slot].c16
	borrow := PrivateB4DefinitionBorrow{Capture: capture, Result: result, c16: c16}
	if c16 == nil {
		defer m.mu.Unlock()
		if !prepare(borrow) {
			return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
		}
		publish(borrow)
		m.releasePrivateB4Locked(reservation)
		return capture, PrivateB4Selected
	}
	if c16.profile.beginBorrow() != nil {
		m.mu.Unlock()
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	m.mu.Unlock()
	prepared := false
	func() { defer c16.profile.endBorrow(); prepared = prepare(borrow) }()
	if !prepared {
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	m.mu.Lock()
	if m.privateB4ReservationLocked(lease.token) != reservation {
		m.mu.Unlock()
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	m.releasePrivateB4Locked(reservation)
	m.mu.Unlock()
	publish(borrow)
	return capture, PrivateB4Selected
}

func (m *Manager) ConsumePrivateB4DefinitionBorrowed(lease B4DefinitionLease, selection B4DefinitionSelectionKey, consume func(PrivateB4DefinitionBorrow) bool) (PrivateB4DefinitionCapture, PrivateB4Status) {
	return m.CommitPrivateB4DefinitionBorrowed(lease, selection, consume, func(PrivateB4DefinitionBorrow) {})
}
