package adr0011methodtransport

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

// LocalWriteCorrespondence is an in-memory comparison of request coordinates
// against manager-local write evidence, not provider custody or a receipt.
type LocalWriteCorrespondence string

const (
	LocalWriteNotObserved LocalWriteCorrespondence = "NOT_OBSERVED"
	LocalWriteMatch       LocalWriteCorrespondence = "LOCAL_MATCH"
	LocalWriteConflict    LocalWriteCorrespondence = "CONFLICT"
)

// classifyLocalWrite checks only self-consistency of one runtime-supplied result
// and the copied request. Runtime remains an untrusted interface: even MATCH
// does not authenticate a provider or make a result eligible for admission.
func classifyLocalWrite(req Request, result sessionruntime.RoundTripResult) LocalWriteCorrespondence {
	write, present := result.CompletedRequestWrite()
	if !present {
		return LocalWriteNotObserved
	}
	if result.Key.ID == 0 || result.Key.Generation != req.Generation ||
		result.RequestMessages != 1 ||
		write.SessionID != req.SessionID || write.Generation != req.Generation ||
		write.Key != result.Key || write.Method != req.Method {
		return LocalWriteConflict
	}
	body, err := json.Marshal(lspwire.Message{
		JSONRPC: lspwire.Version,
		ID:      json.RawMessage(strconv.FormatUint(result.Key.ID, 10)),
		Method:  req.Method,
		Params:  req.Params,
	})
	if err != nil || result.RequestBytes != int64(len(body)) {
		return LocalWriteConflict
	}
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	if write.FrameBytes != int64(len(frame)) || write.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(frame)) {
		return LocalWriteConflict
	}
	return LocalWriteMatch
}
