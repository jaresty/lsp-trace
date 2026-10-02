package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

// ownedFrames remain in this test-owned invocation; they are not an issuance receipt.
type ownedFrames struct {
	invocation             string
	request, response      []byte
	paramsSpan, resultSpan ownedSpan
	pair                   sessionruntime.OwnedMethodPair
}

func exactFrameBody(frame []byte) ([]byte, bool) {
	if len(frame) == 0 || len(frame) > 2<<20 {
		return nil, false
	}
	separator := bytes.Index(frame, []byte("\r\n\r\n"))
	if separator < 0 {
		return nil, false
	}
	header := frame[:separator]
	if !bytes.HasPrefix(header, []byte("Content-Length: ")) || bytes.Contains(header, []byte("\r\n")) {
		return nil, false
	}
	length, err := strconv.Atoi(string(header[len("Content-Length: "):]))
	if err != nil || length <= 0 || length != len(frame)-separator-4 {
		return nil, false
	}
	reader := lspwire.NewReader(bytes.NewReader(frame), lspwire.Limits{MaxBodyBytes: 2 << 20, MaxHeaderBytes: 64 << 10})
	if _, err = reader.Read(); err != nil {
		return nil, false
	}
	return frame[separator+4:], true
}

func exactJSONField(body []byte, field string) ([]byte, bool) {
	if strictjson.RejectDuplicates(body) != nil {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return nil, false
	}
	value, ok := fields[field]
	return value, ok && len(value) > 0
}

func verifiedOwnedFrames(result sessionruntime.RoundTripResult, pair sessionruntime.OwnedMethodPair, method string, params []byte, binding sessionruntime.OwnedDocumentBinding, sessionID string, generation uint64, capBytes int64) (ownedFrames, bool) {
	if result.Failure != "" || result.ServerError != nil || pair.SessionID != sessionID || pair.Generation != generation || pair.Key != result.Key || pair.Key.ID == 0 || pair.Key.Generation != generation || pair.Method != method || pair.Source == nil || *pair.Source != binding || binding.URI == "" || binding.Version <= 0 || binding.SHA256 == "" || !bytes.Equal(pair.Params, params) || !bytes.Equal(pair.Result, result.Result) || pair.Write.SessionID != sessionID || pair.Read.SessionID != sessionID || pair.Write.Generation != generation || pair.Read.Generation != generation || pair.Write.Key != pair.Key || pair.Read.Key != pair.Key || pair.Write.Method != method || capBytes <= 0 || capBytes > 2<<20 {
		return ownedFrames{}, false
	}
	var request, response []byte
	var requestOK, responseOK bool
	switch method {
	case "textDocument/documentSymbol":
		request, requestOK = result.CompletedDocumentSymbolRequestFrame()
		response, responseOK = result.CompletedDocumentSymbolResponseFrame()
	case "textDocument/references":
		request, requestOK = result.CompletedMethodRequestFrame()
		response, responseOK = result.CompletedReferencesResponseFrame()
	default:
		return ownedFrames{}, false
	}
	if !requestOK || !responseOK || len(request) == 0 || len(response) == 0 || int64(len(request)) > capBytes || int64(len(response)) > capBytes || pair.Write.FrameBytes != int64(len(request)) || pair.Read.FrameBytes != int64(len(response)) || pair.Write.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(request)) || pair.Read.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(response)) {
		return ownedFrames{}, false
	}
	requestBody, ok := exactFrameBody(request)
	if !ok {
		return ownedFrames{}, false
	}
	responseBody, ok := exactFrameBody(response)
	if !ok {
		return ownedFrames{}, false
	}
	paramsSpan, ok := exactOwnedSpan(request, "params", method, pair.Key.ID)
	if !ok || !bytes.Equal(paramsSpan.Value, pair.Params) {
		return ownedFrames{}, false
	}
	resultSpan, ok := exactOwnedSpan(response, "result", method, pair.Key.ID)
	if !ok || !bytes.Equal(resultSpan.Value, pair.Result) {
		return ownedFrames{}, false
	}
	writeSlice, ok := privateBodySliceFromFrame(request, paramsSpan)
	if !ok || !replayPrivateBodySlice(append([]byte(nil), requestBody...), writeSlice, "params", method, pair.Key.ID, pair.Params) {
		return ownedFrames{}, false
	}
	readSlice, ok := privateBodySliceFromFrame(response, resultSpan)
	if !ok || !replayPrivateBodySlice(append([]byte(nil), responseBody...), readSlice, "result", method, pair.Key.ID, pair.Result) {
		return ownedFrames{}, false
	}
	var req, resp lspwire.Message
	if json.Unmarshal(requestBody, &req) != nil || json.Unmarshal(responseBody, &resp) != nil || req.JSONRPC != lspwire.Version || resp.JSONRPC != lspwire.Version || req.Kind() != lspwire.KindRequest || resp.Kind() != lspwire.KindSuccessResponse || req.Method != method || !bytes.Equal(req.ID, resp.ID) || string(req.ID) != strconv.FormatUint(pair.Key.ID, 10) {
		return ownedFrames{}, false
	}
	return ownedFrames{request: append([]byte(nil), request...), response: append([]byte(nil), response...), paramsSpan: paramsSpan, resultSpan: resultSpan, pair: pair}, true
}
