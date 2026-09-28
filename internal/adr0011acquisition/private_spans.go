package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

// ownedSpan is private capture evidence, with offsets relative to the original frame.
// It is not an ownerRead receipt (whose proposed offsets are body-relative).
type ownedSpan struct {
	Offset, Length int
	FrameLength    int
	FrameDigest    string
	ValueDigest    string
	Value          []byte
}

func jsonSpace(b byte) bool { return b == ' ' || b == '\n' || b == '\r' || b == '\t' }

func exactOwnedSpan(frame []byte, field, method string, wireID uint64) (ownedSpan, bool) {
	body, ok := exactFrameBody(frame)
	if !ok || strictjson.RejectDuplicates(body) != nil {
		return ownedSpan{}, false
	}
	if (field != "params" && field != "result") || wireID == 0 || (method != "textDocument/documentSymbol" && method != "textDocument/references") {
		return ownedSpan{}, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	first, err := dec.Token()
	if err != nil || first != json.Delim('{') {
		return ownedSpan{}, false
	}
	seen := map[string]bool{}
	selectedStart, selectedEnd := -1, -1
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			return ownedSpan{}, false
		}
		key, ok := k.(string)
		if !ok || seen[key] {
			return ownedSpan{}, false
		}
		seen[key] = true
		switch key {
		case "jsonrpc", "id":
		case "method", "params":
			if field != "params" {
				return ownedSpan{}, false
			}
		case "result":
			if field != "result" {
				return ownedSpan{}, false
			}
		default:
			return ownedSpan{}, false
		}
		start := int(dec.InputOffset())
		for start < len(body) && jsonSpace(body[start]) {
			start++
		}
		if start >= len(body) || body[start] != ':' {
			return ownedSpan{}, false
		}
		start++
		for start < len(body) && jsonSpace(body[start]) {
			start++
		}
		var raw json.RawMessage
		if dec.Decode(&raw) != nil {
			return ownedSpan{}, false
		}
		end := start + len(raw)
		if start >= end || end > len(body) || !bytes.Equal(body[start:end], raw) {
			return ownedSpan{}, false
		}
		switch key {
		case "jsonrpc":
			if string(raw) != `"2.0"` {
				return ownedSpan{}, false
			}
		case "id":
			if string(raw) != strconv.FormatUint(wireID, 10) {
				return ownedSpan{}, false
			}
		case "method":
			var name string
			if json.Unmarshal(raw, &name) != nil || name != method {
				return ownedSpan{}, false
			}
		}
		if key == field {
			selectedStart, selectedEnd = start, end
		}
	}
	closing, err := dec.Token()
	if err != nil || closing != json.Delim('}') {
		return ownedSpan{}, false
	}
	if _, err = dec.Token(); err != io.EOF || !seen["jsonrpc"] || !seen["id"] || !seen[field] || selectedStart < 0 || (field == "params" && !seen["method"]) || (field == "result" && seen["method"]) {
		return ownedSpan{}, false
	}
	if selectedEnd-selectedStart > 1<<20 || (field == "params" && selectedEnd-selectedStart > 65536) {
		return ownedSpan{}, false
	}
	offset := len(frame) - len(body) + selectedStart
	value := append([]byte(nil), frame[offset:offset+selectedEnd-selectedStart]...)
	return ownedSpan{Offset: offset, Length: len(value), FrameLength: len(frame), FrameDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(frame)), ValueDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(value)), Value: value}, true
}

// replayOwnedSpan takes independently supplied frame bytes and an expected span.
func replayOwnedSpan(frame []byte, claimed ownedSpan, field, method string, wireID uint64) bool {
	actual, ok := exactOwnedSpan(frame, field, method, wireID)
	return ok && claimed.Offset == actual.Offset && claimed.Length == actual.Length && claimed.FrameLength == actual.FrameLength && claimed.FrameDigest == actual.FrameDigest && claimed.ValueDigest == actual.ValueDigest && bytes.Equal(claimed.Value, actual.Value)
}

// replayOwnedFrames checks a separately supplied immutable copy against independent
// expected manager pair and source identity. No raw bytes enter errors or publication.
func replayOwnedFrames(frames ownedFrames, expected sessionruntime.OwnedMethodPair, binding sessionruntime.OwnedDocumentBinding, sessionID string, generation uint64, capBytes int64) bool {
	p := expected
	if capBytes <= 0 || capBytes > 1<<20 || p.SessionID != sessionID || p.Generation != generation || p.Key.ID == 0 || p.Key.Generation != generation || p.Source == nil || *p.Source != binding || binding.URI == "" || binding.Version <= 0 || binding.SHA256 == "" || p.Write.SessionID != sessionID || p.Read.SessionID != sessionID || p.Write.Generation != generation || p.Read.Generation != generation || p.Write.Key != p.Key || p.Read.Key != p.Key || p.Write.Method != p.Method || p.Key != frames.pair.Key || p.Method != frames.pair.Method || frames.pair.SessionID != sessionID || frames.pair.Generation != generation || frames.pair.Source == nil || *frames.pair.Source != binding || frames.pair.Write != p.Write || frames.pair.Read != p.Read || !bytes.Equal(p.Params, frames.pair.Params) || !bytes.Equal(p.Result, frames.pair.Result) {
		return false
	}
	if int64(len(frames.request)) > capBytes || int64(len(frames.response)) > capBytes || p.Write.FrameBytes != int64(len(frames.request)) || p.Read.FrameBytes != int64(len(frames.response)) || p.Write.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(frames.request)) || p.Read.FrameSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(frames.response)) {
		return false
	}
	return replayOwnedSpan(frames.request, frames.paramsSpan, "params", p.Method, p.Key.ID) && replayOwnedSpan(frames.response, frames.resultSpan, "result", p.Method, p.Key.ID) && bytes.Equal(frames.paramsSpan.Value, p.Params) && bytes.Equal(frames.resultSpan.Value, p.Result)
}
