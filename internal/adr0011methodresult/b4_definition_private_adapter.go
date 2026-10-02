package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

// PrivateB4Decision remains unadmitted and unauthenticated even if a later
// manager-selected definition result can be projected into private candidates.
type PrivateB4Decision struct {
	Status             DefinitionBridgeStatus
	ChronologyTerminal string
	Candidates         []DefinitionCandidate
	Authority          int
	Accepted           bool
	Completeness       string
	ClaimCeiling       string
}

// CheckPrivateComposedB4Definition consumes one manager-selected response. Replay
// originals remain independently supplied and untrusted; only manager-held bytes
// can enter the bridge. Neither a caller response nor a forgeable hold is accepted.
func CheckPrivateComposedB4Definition(manager *sessionruntime.Manager, lease sessionruntime.B4DefinitionLease, selection sessionruntime.B4DefinitionSelectionKey, replay v5.B4bFullCandidateInput, queryOccurrenceID string, targetSources map[string][]byte) PrivateB4Decision {
	out := PrivateB4Decision{Status: DefinitionBridgeCorrespondenceInvalid, Authority: 0,
		Accepted: false, Completeness: "UNKNOWN", ClaimCeiling: "NO_PRODUCER_AUTHENTICATION"}
	if manager == nil || selection.SessionID == "" || selection.Key.Generation == 0 || selection.Key.ID == 0 ||
		selection.Transaction == "" || selection.CompletedOwnerKey == "" {
		return out
	}
	capture, status := manager.ConsumePrivateB4Definition(lease, selection)
	if status != sessionruntime.PrivateB4Selected || capture.SessionID != selection.SessionID || capture.Key != selection.Key ||
		capture.Transaction != selection.Transaction || capture.CompletedOwnerKey != selection.CompletedOwnerKey ||
		capture.Method != "textDocument/definition" || replay.Write.Method != capture.Method ||
		replay.Write.Session != capture.SessionID || replay.Write.Generation != capture.Key.Generation ||
		replay.Write.Transaction != capture.Transaction || replay.Write.CompletedKey != capture.CompletedOwnerKey ||
		!replay.Write.WriteCompleted || !bytes.Equal(replay.Write.RequestFrame, capture.RequestFrame) ||
		!bytes.Equal(replay.Write.RequestParams, capture.RequestParams) ||
		!privateCaptureHash(capture.RequestFrame, capture.RequestFrameSHA256) ||
		!privateCaptureHash(capture.ResponseFrame, capture.ResponseFrameSHA256) ||
		!privateCaptureHash(capture.Result, capture.ResultSHA256) {
		return out
	}
	// The typed request ID must agree as bytes and value with the selected key.
	id, err := strconv.ParseUint(string(replay.Write.RequestID), 10, 64)
	if err != nil || id == 0 || id > math.MaxInt64 || id != capture.Key.ID ||
		!bytes.Equal(replay.Write.RequestID, []byte(strconv.FormatUint(id, 10))) {
		return out
	}
	request, requestBody, ok := privateB4Frame(capture.RequestFrame)
	if !ok || request.Kind() != lspwire.KindRequest || request.Method != capture.Method ||
		!bytes.Equal(request.ID, replay.Write.RequestID) || !bytes.Equal(request.Params, capture.RequestParams) ||
		len(requestBody) == 0 {
		return out
	}
	response, responseBody, ok := privateB4Frame(capture.ResponseFrame)
	if !ok || response.Kind() != lspwire.KindSuccessResponse || !bytes.Equal(response.ID, replay.Write.RequestID) ||
		!bytes.Equal(response.Result, capture.Result) || len(responseBody) == 0 {
		return out
	}
	// The existing bridge builds its canonical candidate and balanced ledger
	// internally from these verified, manager-selected bytes. It also rejects
	// malformed members and unsafe target ranges before projecting anything.
	checked := CheckB4DefinitionBridge(DefinitionBridgeInput{Replay: replay,
		ResponseFrame: responseBody, QueryOccurrenceID: queryOccurrenceID, TargetSources: targetSources})
	out.Status, out.ChronologyTerminal, out.Candidates = checked.Status, checked.ChronologyTerminal, checked.Candidates
	return out
}

func privateCaptureHash(raw []byte, digest string) bool {
	return raw != nil && digest == fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
}

// Read exactly one bounded frame; retain the original JSON body for strict
// bridge validation rather than re-marshalling a permissively decoded message.
func privateB4Frame(frame []byte) (lspwire.Message, []byte, bool) {
	if len(frame) == 0 || len(frame) > 2<<20 {
		return lspwire.Message{}, nil, false
	}
	_, body, found := bytes.Cut(frame, []byte("\r\n\r\n"))
	if !found || len(body) == 0 || !json.Valid(body) {
		return lspwire.Message{}, nil, false
	}
	reader := lspwire.NewReader(bytes.NewReader(frame), lspwire.Limits{MaxBodyBytes: 2 << 20, MaxHeaderBytes: 64 << 10})
	message, _, exact, err := reader.ReadWithExactFrame(int64(len(frame)))
	if err != nil || !bytes.Equal(exact, frame) {
		return lspwire.Message{}, nil, false
	}
	return message, body, true
}
