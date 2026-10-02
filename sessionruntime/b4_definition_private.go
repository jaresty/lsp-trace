package sessionruntime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

// B4DefinitionOwner names a private pre-write declaration. Labels alone do not
// authenticate their declarant, the server, or a response.
type B4DefinitionOwner struct {
	Transaction       string
	CompletedOwnerKey string
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
}

type privateB4Reservation struct {
	token     [32]byte
	selection B4DefinitionSelectionKey
	session   *runtimeSession
	maxCharge int64
	capture   PrivateB4DefinitionCapture
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
	// Copy the caller's params before admission. This bound covers reservation,
	// selected request/response/result copies and the output on first consumption.
	req.Params = append(json.RawMessage(nil), req.Params...)
	charge := 2 * (req.MaxBytes + req.CaptureDefinitionResponseFrameMaxBytes +
		req.CaptureMethodRequestFrameMaxBytes + int64(len(req.Params)) + 256)
	if charge > privateB4MaxBytes {
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	reservation := &privateB4Reservation{maxCharge: charge,
		selection: B4DefinitionSelectionKey{Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}}
	if _, err := io.ReadFull(rand.Reader, reservation.token[:]); err != nil || reservation.token == ([32]byte{}) {
		return RoundTripResult{Failure: session.ResourceExhausted}, B4DefinitionLease{}
	}
	result := m.roundTripWithPrivate(parent, req, nil, reservation)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.privateB4Leases[reservation.token] != reservation {
		return result, B4DefinitionLease{}
	}
	// A failed or unselected transaction never becomes a transferable lease.
	writeFrame, wrote := result.CompletedMethodRequestFrame()
	readFrame, read := result.CompletedDefinitionResponseFrame()
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
		ResultSHA256: privateB4Hash(result.Result),
	}
	// The reservation is intentionally conservative. Only owned copies survive
	// return; release of the reservation happens on first successful consumption.
	reservation.capture = capture
	return result, B4DefinitionLease{token: reservation.token}
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

func (m *Manager) releasePrivateB4Locked(reservation *privateB4Reservation) {
	if m.privateB4Leases[reservation.token] == reservation {
		delete(m.privateB4Leases, reservation.token)
		m.privateB4Bytes -= reservation.maxCharge
	}
}

// Called only under Manager.mu after successful old-generation teardown on
// completed STOP or RESTART. Repeated retirement cannot subtract an absent entry.
func (m *Manager) retirePrivateB4StoppedLocked(id string, generation uint64, stopped *runtimeSession) {
	if stopped == nil || stopped.record.Generation != generation {
		return
	}
	for _, reservation := range m.privateB4Leases {
		if reservation.session == stopped && reservation.selection.SessionID == id &&
			reservation.selection.Key.Generation == generation {
			m.releasePrivateB4Locked(reservation)
		}
	}
}

// ConsumePrivateB4Definition grants first use only for an exact manager-held
// selection; it never accepts caller-supplied frames or result bytes.
func (m *Manager) ConsumePrivateB4Definition(lease B4DefinitionLease, selection B4DefinitionSelectionKey) (PrivateB4DefinitionCapture, PrivateB4Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	reservation := m.privateB4Leases[lease.token]
	if lease == (B4DefinitionLease{}) || reservation == nil || reservation.selection != selection ||
		reservation.capture.ResponseFrame == nil || m.closed {
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	r := m.sessions[selection.SessionID]
	if r == nil || r != reservation.session || r.record.Generation != selection.Key.Generation || r.record.State != session.Ready {
		return PrivateB4DefinitionCapture{}, PrivateB4Unavailable
	}
	capture := reservation.capture
	m.releasePrivateB4Locked(reservation)
	return capture, PrivateB4Selected
}
