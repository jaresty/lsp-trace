package sessionruntime

import (
	"bytes"
	"encoding/json"

	"lsp-trace/internal/lspwire"
)

// OwnedMethodPair is an in-memory, manager-local observation of one keyed
// definition/references transaction. It is not a published method receipt,
// provider authentication, terminal ledger, or admitted occurrence.
type OwnedMethodPair struct {
	SessionID  string
	Generation uint64
	Key        lspwire.RequestKey
	Method     string
	Params     json.RawMessage
	Result     json.RawMessage
	Write      RequestWriteObservation
	Read       ResponseReadObservation
}

// CompletedOwnedMethodPair returns a copy only for an opted-in, bounded,
// successful managed transaction. A zero value cannot issue D/R evidence.
func (r RoundTripResult) CompletedOwnedMethodPair() (OwnedMethodPair, bool) {
	if r.Failure != "" || r.ServerError != nil || r.ownedMethodPair == nil {
		return OwnedMethodPair{}, false
	}
	p := *r.ownedMethodPair
	p.Params = append(json.RawMessage(nil), p.Params...)
	p.Result = append(json.RawMessage(nil), p.Result...)
	return p, true
}

func eligibleOwnedMethodRequest(req RoundTripRequest) bool {
	return req.CaptureOwnedMethodPair && (req.Method == "textDocument/definition" || req.Method == "textDocument/references") &&
		req.SessionID != "" && req.Generation != 0 && req.MaxMessages >= 1 && req.MaxMessages <= 64 &&
		req.MaxBytes >= 1 && req.MaxBytes <= 1<<20 && len(req.Params) >= 1 && len(req.Params) <= 64<<10
}

func buildOwnedMethodPair(req RoundTripRequest, r RoundTripResult, params, raw json.RawMessage) *OwnedMethodPair {
	if !eligibleOwnedMethodRequest(req) || r.ServerError != nil || r.Key.Generation != req.Generation || r.Key.ID == 0 ||
		r.RequestMessages != 1 || r.RequestBytes < 1 || r.RequestBytes > req.MaxBytes ||
		r.Messages < 1 || r.Messages > req.MaxMessages || r.Bytes < 1 || r.Bytes > req.MaxBytes ||
		len(raw) < 1 || int64(len(raw)) > req.MaxBytes || !bytes.Equal(raw, r.Result) ||
		r.requestWrite == nil || r.responseRead == nil || r.requestWrite.FrameBytes < 1 || r.responseRead.FrameBytes < 1 ||
		r.requestWrite.SessionID != req.SessionID || r.responseRead.SessionID != req.SessionID ||
		r.requestWrite.Generation != req.Generation || r.responseRead.Generation != req.Generation ||
		r.requestWrite.Key != r.Key || r.responseRead.Key != r.Key || r.requestWrite.Method != req.Method {
		return nil
	}
	return &OwnedMethodPair{SessionID: req.SessionID, Generation: req.Generation, Key: r.Key, Method: req.Method,
		Params: append(json.RawMessage(nil), params...), Result: append(json.RawMessage(nil), raw...),
		Write: *r.requestWrite, Read: *r.responseRead}
}
