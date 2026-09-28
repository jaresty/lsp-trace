package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/sessionruntime"
)

// reviewedSuccessorSchemaDigest is owner policy, never selected from a claimant.
const reviewedSuccessorSchemaDigest = "86bb214e419f7c69b80ff5dfbdf0f0760c495c9bf0f88d06bdc227b77920fc8e"

// verifyOriginalRequestKey uses the retained complete Manager frames, not a
// reconstructed message or the pair's converted key as its wire evidence.
func verifyOriginalRequestKey(frames ownedFrames, expected sessionruntime.OwnedMethodPair, sessionID string, generation uint64, expectedInvocation, schemaDigest string) bool {
	if expectedInvocation == "" || frames.invocation == "" || schemaDigest != reviewedSuccessorSchemaDigest || sessionID == "" || expected.SessionID != sessionID || frames.pair.SessionID != sessionID || expected.Generation != generation || frames.pair.Generation != generation || expected.Key != frames.pair.Key || expected.Write.Key != expected.Key || expected.Read.Key != expected.Key || expected.Write.SessionID != sessionID || expected.Read.SessionID != sessionID || expected.Write.Generation != generation || expected.Read.Generation != generation || frames.pair.Write != expected.Write || frames.pair.Read != expected.Read || expected.Source == nil || frames.pair.Source == nil || *expected.Source != *frames.pair.Source || !bytes.Equal(expected.Params, frames.pair.Params) || !bytes.Equal(expected.Result, frames.pair.Result) || expected.Write.FrameBytes != int64(len(frames.request)) || expected.Read.FrameBytes != int64(len(frames.response)) || expected.Write.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(frames.request)) || expected.Read.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(frames.response)) {
		return false
	}
	requestBody, ok := exactFrameBody(frames.request)
	if !ok {
		return false
	}
	responseBody, ok := exactFrameBody(frames.response)
	if !ok {
		return false
	}
	requestID, ok := exactJSONField(requestBody, "id")
	if !ok {
		return false
	}
	responseID, ok := exactJSONField(responseBody, "id")
	if !ok {
		return false
	}
	return adr0011requestkey.Identity(adr0011requestkey.Encode(expected.Key), frames.pair.Key, generation, expected.Write.Key.ID, requestID, responseID, frames.invocation, expectedInvocation) == nil
}
