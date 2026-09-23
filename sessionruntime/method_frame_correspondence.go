package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"lsp-trace/internal/lspwire"
)

// ErrMethodFrameCorrespondence has fixed text to avoid reflecting raw frames,
// caller parameters, provider messages, or paths into a failure.
var ErrMethodFrameCorrespondence = errors.New("method frame correspondence failed")

// This is a private verification-work ceiling, not an occurrence-admission or
// retention policy. It bounds raw replay and reconstructed request work here;
// the manager's capture cap is independently caller-selected.
const maxMethodFrameCorrespondenceBytes = 2 << 20

// VerifyMethodFrameCorrespondence checks local byte agreement of one opted-in
// accepted D/R transaction. It does not authenticate the acquisition owner or
// provider, establish immutable custody, select terminal members, or admit an
// occurrence. All inputs, including manager-local observations, are replayable
// by a caller and therefore cannot form an owner-verified receipt.
func VerifyMethodFrameCorrespondence(req RoundTripRequest, result RoundTripResult) error {
	cap := int64(0)
	switch req.Method {
	case "textDocument/definition":
		cap = req.CaptureDefinitionResponseFrameMaxBytes
	case "textDocument/references":
		cap = req.CaptureReferencesResponseFrameMaxBytes
	default:
		return ErrMethodFrameCorrespondence
	}
	if req.SessionID == "" || req.Generation == 0 || req.MaxMessages <= 0 || req.MaxBytes <= 0 ||
		cap <= 0 || cap > maxMethodFrameCorrespondenceBytes || len(req.Params) > maxMethodFrameCorrespondenceBytes ||
		result.Failure != "" || result.ServerError != nil || result.Key.Generation != req.Generation || result.Key.ID == 0 ||
		result.Messages <= 0 || result.Messages > req.MaxMessages || result.Bytes <= 0 || result.Bytes > req.MaxBytes ||
		result.RequestMessages != 1 {
		return ErrMethodFrameCorrespondence
	}
	write, wrote := result.CompletedRequestWrite()
	read, accepted := result.CompletedResponseRead()
	raw, captured := result.CompletedMethodResponseFrame()
	if !wrote || !accepted || !captured || len(raw) == 0 || int64(len(raw)) > cap ||
		write.SessionID != req.SessionID || read.SessionID != req.SessionID ||
		write.Generation != req.Generation || read.Generation != req.Generation ||
		write.Key != result.Key || read.Key != result.Key || write.Method != req.Method {
		return ErrMethodFrameCorrespondence
	}

	request := lspwire.Message{
		JSONRPC: lspwire.Version, ID: json.RawMessage(strconv.FormatUint(result.Key.ID, 10)),
		Method: req.Method, Params: req.Params,
	}
	body, err := json.Marshal(request)
	if err != nil || len(body) == 0 || int64(len(body)) != result.RequestBytes {
		return ErrMethodFrameCorrespondence
	}
	limits := lspwire.Limits{MaxBodyBytes: maxMethodFrameCorrespondenceBytes, MaxHeaderBytes: 64 << 10}
	var framed bytes.Buffer
	if err := lspwire.NewWriter(&framed, limits).Write(request); err != nil ||
		framed.Len() > maxMethodFrameCorrespondenceBytes || write.FrameBytes != int64(framed.Len()) ||
		write.FrameSHA256 != methodCorrespondenceHash(framed.Bytes()) {
		return ErrMethodFrameCorrespondence
	}

	decoded, observation, exact, err := lspwire.NewReader(bytes.NewReader(raw), limits).ReadWithExactFrame(int64(len(raw)))
	if err != nil || observation.FrameBytes != int64(len(raw)) ||
		observation.FrameBytes != read.FrameBytes || observation.FrameSHA256 != read.FrameSHA256 ||
		!bytes.Equal(exact, raw) || decoded.Kind() != lspwire.KindSuccessResponse ||
		!bytes.Equal(decoded.Result, result.Result) {
		return ErrMethodFrameCorrespondence
	}
	id, err := strconv.ParseUint(string(decoded.ID), 10, 64)
	if err != nil || id != result.Key.ID {
		return ErrMethodFrameCorrespondence
	}
	return nil
}

func methodCorrespondenceHash(frame []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(frame))
}
