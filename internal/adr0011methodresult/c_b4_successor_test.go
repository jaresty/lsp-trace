package adr0011methodresult

import (
	"bytes"
	"math"
	"strconv"

	"lsp-trace/internal/adr0011cobserve"
	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

// cObservedB4 is an ADDITIVE test-only mirror of the frozen private adapter.
// It does not replace CheckPrivateComposedB4Definition or change its source pin.
// Review its conditions against that exact historical implementation before
// using its entry notification to reason about the C synthetic guard.
func cObservedB4(observer adr0011cobserve.Observer, manager *sessionruntime.Manager, lease sessionruntime.B4DefinitionLease, selection sessionruntime.B4DefinitionSelectionKey, replay v5.B4bFullCandidateInput, queryOccurrenceID string, targetSources map[string][]byte) PrivateB4Decision {
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
	adr0011cobserve.Notify(observer, adr0011cobserve.B4EvaluationEntry)
	checked := CheckB4DefinitionBridge(DefinitionBridgeInput{Replay: replay,
		ResponseFrame: responseBody, QueryOccurrenceID: queryOccurrenceID, TargetSources: targetSources})
	out.Status, out.ChronologyTerminal, out.Candidates = checked.Status, checked.ChronologyTerminal, checked.Candidates
	return out
}
