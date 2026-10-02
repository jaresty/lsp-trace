package adr0011methodresult

import (
	"bytes"
	"math"
	"strconv"
	"strings"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
)

// DefinitionResponseHold is an independently supplied private synthetic response
// original and its separately held WRITE binding. It is not a producer receipt;
// these fields can be forged by a same-process caller.
type DefinitionResponseHold struct {
	Session, Transaction, Method, RequestOwnerKey string
	Generation, RequestID, ResponseID             uint64
	ResponseOriginal                              []byte
	ResponseLength, RawResultLength               int
	ResponseSHA256, RawResultSHA256               string
}

// CheckB4DefinitionBridgeWithHeldResponse requires a separately held exact
// response original and WRITE binding before projecting candidates. A caller can
// forge both inputs: this is private synthetic content integrity, not proof of
// same-transaction server custody or producer authentication.
func CheckB4DefinitionBridgeWithHeldResponse(in DefinitionBridgeInput, held DefinitionResponseHold) DefinitionBridgeResult {
	out := DefinitionBridgeResult{Status: DefinitionBridgeCorrespondenceInvalid}
	w := in.Replay.Write
	if v5.CheckB4a(w) != nil || w.Method != "textDocument/definition" {
		return out
	}
	out.Status = DefinitionBridgeResponseInvalid
	if held.Session != w.Session || held.Generation != w.Generation ||
		held.Transaction != w.Transaction || held.Method != w.Method ||
		held.RequestOwnerKey != w.CompletedKey || held.RequestID == 0 ||
		held.RequestID > math.MaxInt64 || held.RequestID != held.ResponseID ||
		string(w.RequestID) != strconv.FormatUint(held.RequestID, 10) ||
		held.ResponseLength <= 0 || held.ResponseLength > 1<<20 ||
		held.RawResultLength <= 0 || held.RawResultLength > 1<<20 ||
		len(held.ResponseOriginal) != held.ResponseLength ||
		len(in.ResponseFrame) != held.ResponseLength ||
		!bytes.Equal(in.ResponseFrame, held.ResponseOriginal) ||
		len(held.ResponseSHA256) != 64 || len(held.RawResultSHA256) != 64 {
		return out
	}
	// Own one stable copy for both verification and downstream projection; the
	// separately held digest must match exact response bytes, not just the ID.
	original := append([]byte(nil), held.ResponseOriginal...)
	if rawSHA(original) != "sha256:"+held.ResponseSHA256 ||
		strings.ToLower(held.ResponseSHA256) != held.ResponseSHA256 {
		return out
	}
	verified := in
	verified.ResponseFrame = original
	result, ok := bridgeResponseResult(verified)
	if !ok || len(result) != held.RawResultLength ||
		rawSHA(result) != "sha256:"+held.RawResultSHA256 ||
		strings.ToLower(held.RawResultSHA256) != held.RawResultSHA256 {
		return out
	}
	return CheckB4DefinitionBridge(verified)
}
