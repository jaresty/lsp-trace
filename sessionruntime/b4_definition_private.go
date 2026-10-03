package sessionruntime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sync/atomic"

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
	PrivateB4Unavailable PrivateB4Status = "PRIVATE_B4_UNAVAILABLE"
	PrivateB4Selected    PrivateB4Status = "PRIVATE_B4_SELECTED"
	privateB4MaxSlots                    = 4
	privateB4MaxBytes    int64           = 16 << 20
)

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

type privateB4Reservation struct {
	token        [32]byte
	selection    B4DefinitionSelectionKey
	session      *runtimeSession
	maxCharge    int64
	slot         int
	sourceLeases []B4DefinitionSourceLease
	capture      PrivateB4DefinitionCapture
	decodeEntry  func()
	retainEntry  func()
}

type privateB4ResponseCapture struct{ reservation *privateB4Reservation }

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
	if c.reservation.retainEntry != nil {
		c.reservation.retainEntry()
	}
	if c.reservation.decodeEntry != nil {
		c.reservation.decodeEntry()
	}
}

func privateB4SuccessorOptions(reservation *privateB4Reservation) lspwire.SuccessorIngressOptions {
	return lspwire.SuccessorIngressOptions{
		OwnerID: "sessionruntime.private-b4",
		Capture: privateB4ResponseCapture{reservation: reservation},
		Limits: lspwire.SuccessorIngressLimits{
			HeaderBytes: 65536, FrameBytes: 2097152, ConsumptionBytes: 8388608, AcquisitionBytes: 8392705,
			MaxReadBytes: 4096, PrefetchBytes: 4096, HistoryAcquiredBytes: 8388608, HistoryOutstandingBytes: 8388608,
		},
	}
}

type privateB4Slot struct {
	occupied    bool
	token       [32]byte
	reservation *privateB4Reservation
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
	var identity [32]byte
	if _, err := io.ReadFull(rand.Reader, identity[:]); err != nil || identity == ([32]byte{}) {
		return B4DefinitionSourceLease{}, PrivateB4Unavailable
	}
	content := append([]byte(nil), document.supply.Content...)
	state := &privateB4SourceLease{manager: m, session: r, source: B4DefinitionTargetSource{
		URI: ref.URI, AcquisitionID: "sha256:" + fmtDigest(identity), SHA256: privateB4Hash(content),
		SessionID: ref.SessionID, Generation: ref.Generation, DocumentVersion: ref.DocumentVersion, Bytes: content,
	}}
	return B4DefinitionSourceLease{state: state}, PrivateB4Selected
}

func (m *Manager) ReleasePrivateB4DefinitionSource(lease B4DefinitionSourceLease) PrivateB4Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lease.state == nil || lease.state.manager != m || !lease.state.state.CompareAndSwap(privateB4SourceHeld, privateB4SourceReleased) {
		return PrivateB4Unavailable
	}
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
	// Source bytes are resolved only from exact live Manager-held prepared
	// document records while Manager.mu is held by admission.
	req.Params = append(json.RawMessage(nil), req.Params...)
	reservation := &privateB4Reservation{sourceLeases: append([]B4DefinitionSourceLease(nil), owner.TargetSources...),
		selection: B4DefinitionSelectionKey{Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}}
	if _, err := io.ReadFull(rand.Reader, reservation.token[:]); err != nil || reservation.token == ([32]byte{}) {
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	result := m.roundTripWithPrivate(parent, req, nil, reservation)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.privateB4ReservationLocked(reservation.token) != reservation {
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
	if result.Failure != "" || result.ServerError != nil || !wrote || !read || !wroteObservation || !readObserved ||
		result.Key != reservation.selection.Key || write.SessionID != req.SessionID || write.Key != result.Key ||
		readObservation.SessionID != req.SessionID || readObservation.Key != result.Key ||
		VerifyMethodFrameCorrespondence(req, result) != nil || r == nil || r != reservation.session ||
		r.record.Generation != req.Generation || r.record.State != session.Ready || m.closed {
		m.releasePrivateB4Locked(reservation)
		return result, B4DefinitionLease{}
	}
	capture := PrivateB4DefinitionCapture{
		SessionID: req.SessionID, Key: result.Key, Transaction: owner.Transaction,
		CompletedOwnerKey: owner.CompletedOwnerKey, Method: req.Method,
		RequestFrame: writeFrame, RequestParams: append([]byte(nil), req.Params...),
		ResponseFrame: readFrame, Result: append([]byte(nil), result.Result...),
		RequestFrameSHA256: privateB4Hash(writeFrame), ResponseFrameSHA256: privateB4Hash(readFrame),
		ResultSHA256: privateB4Hash(result.Result), TargetSources: reservation.capture.TargetSources,
	}
	capture.QueryOccurrenceID = privateB4QueryOccurrenceID(capture)
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
	seenLease := make(map[*privateB4SourceLease]struct{}, len(reservation.sourceLeases))
	seenURI := make(map[string]struct{}, len(reservation.sourceLeases))
	owned := make([]B4DefinitionTargetSource, len(reservation.sourceLeases))
	sourceBytes := 0
	for i, lease := range reservation.sourceLeases {
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
		if _, ok := seenLease[s]; ok {
			return session.ToolNotImplemented
		}
		if _, ok := seenURI[s.source.URI]; ok {
			return session.ToolNotImplemented
		}
		seenLease[s], seenURI[s.source.URI] = struct{}{}, struct{}{}
		owned[i] = s.source
		owned[i].Bytes = append([]byte(nil), s.source.Bytes...)
		sourceBytes += len(s.source.Bytes)
	}
	charge := 2 * (req.MaxBytes + req.CaptureDefinitionResponseFrameMaxBytes + req.CaptureMethodRequestFrameMaxBytes + int64(len(req.Params)+sourceBytes) + 256)
	if charge > privateB4MaxBytes || charge > privateB4MaxBytes-m.privateB4Bytes {
		return session.ResourceExhausted
	}
	reservation.maxCharge = charge
	reservation.slot = emptySlot
	reservation.selection.SessionID = req.SessionID
	reservation.selection.Key.Generation = req.Generation
	reservation.session = m.sessions[req.SessionID]
	reservation.capture.TargetSources = owned
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

func (m *Manager) releasePrivateB4Locked(reservation *privateB4Reservation) {
	slot := &m.privateB4Leases[reservation.slot]
	if slot.occupied && slot.token == reservation.token && slot.reservation == reservation {
		*slot = privateB4Slot{}
		m.privateB4Bytes -= reservation.maxCharge
	}
}

// Called only under Manager.mu after successful old-generation teardown on
// completed STOP or RESTART. Repeated retirement cannot subtract an absent entry.
func (m *Manager) retirePrivateB4StoppedLocked(id string, generation uint64, stopped *runtimeSession) {
	if stopped == nil || stopped.record.Generation != generation {
		return
	}
	for i := range m.privateB4Leases {
		reservation := m.privateB4Leases[i].reservation
		if m.privateB4Leases[i].occupied && reservation.session == stopped && reservation.selection.SessionID == id && reservation.selection.Key.Generation == generation {
			m.releasePrivateB4Locked(reservation)
		}
	}
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
	capture.RequestFrame = append([]byte(nil), capture.RequestFrame...)
	capture.RequestParams = append([]byte(nil), capture.RequestParams...)
	capture.ResponseFrame = append([]byte(nil), capture.ResponseFrame...)
	capture.Result = append([]byte(nil), capture.Result...)
	capture.TargetSources = append([]B4DefinitionTargetSource(nil), capture.TargetSources...)
	for i := range capture.TargetSources {
		capture.TargetSources[i].Bytes = append([]byte(nil), capture.TargetSources[i].Bytes...)
	}
	return reservation, capture, PrivateB4Selected
}

// PreparePrivateB4Definition returns manager-owned copies without consuming the
// lease. It carries no authority to commit or label any bytes as retained.
func (m *Manager) PreparePrivateB4Definition(lease B4DefinitionLease, selection B4DefinitionSelectionKey) (PrivateB4DefinitionCapture, PrivateB4Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, capture, status := m.privateB4CaptureLocked(lease, selection)
	return capture, status
}

// CommitPrivateB4Definition holds Manager.mu while prepare acquires or reserves
// any dependent state in canonical order. A successful prepare must leave that
// state valid for the infallible fixed-storage publication callback. The lease is
// consumed only after publication completes.
func (m *Manager) CommitPrivateB4Definition(lease B4DefinitionLease, selection B4DefinitionSelectionKey, prepare func(PrivateB4DefinitionCapture) bool, publish func(PrivateB4DefinitionCapture)) (PrivateB4DefinitionCapture, PrivateB4Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	reservation, capture, status := m.privateB4CaptureLocked(lease, selection)
	if status != PrivateB4Selected || prepare == nil || publish == nil || !prepare(capture) {
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	publish(capture)
	m.releasePrivateB4Locked(reservation)
	return capture, PrivateB4Selected
}

// ConsumePrivateB4Definition preserves the original first-use operation.
func (m *Manager) ConsumePrivateB4Definition(lease B4DefinitionLease, selection B4DefinitionSelectionKey) (PrivateB4DefinitionCapture, PrivateB4Status) {
	return m.CommitPrivateB4Definition(lease, selection, func(PrivateB4DefinitionCapture) bool { return true }, func(PrivateB4DefinitionCapture) {})
}
