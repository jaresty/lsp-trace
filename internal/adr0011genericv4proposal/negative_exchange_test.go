package adr0011genericv4proposal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"lsp-trace/internal/strictjson"
)

// Candidate-only oracle: it cannot establish ownership or completeness of the host frame stream.
func decodeCandidateFrame(raw []byte) (map[string]json.RawMessage, error) {
	header, body, ok := bytes.Cut(raw, []byte("\r\n\r\n"))
	if !ok || !bytes.HasPrefix(header, []byte("Content-Length: ")) {
		return nil, fmt.Errorf("missing frame header")
	}
	n, err := strconv.Atoi(string(bytes.TrimPrefix(header, []byte("Content-Length: "))))
	if err != nil || n != len(body) || strictjson.RejectDuplicates(body) != nil {
		return nil, fmt.Errorf("invalid complete frame")
	}
	var v map[string]json.RawMessage
	err = json.Unmarshal(body, &v)
	return v, err
}

type candidateFrame struct {
	direction string
	ordinal   int
	raw       []byte
}

// Only offline candidate fixtures call this. Production B4 stays paused.
func candidateEffective(request, response candidateFrame, observed []candidateFrame, method string, write int) (bool, error) {
	req, err := decodeCandidateFrame(request.raw)
	if err != nil {
		return false, err
	}
	var requestMethod string
	if err = json.Unmarshal(req["method"], &requestMethod); err != nil || requestMethod != method || len(req["params"]) == 0 || len(req["id"]) == 0 {
		return false, fmt.Errorf("request mirror")
	}
	wantRequest, wantResponse := "SERVER_TO_CLIENT", "CLIENT_TO_SERVER"
	if method == "initialize" {
		wantRequest, wantResponse = wantResponse, wantRequest
	}
	if request.direction != wantRequest || request.ordinal < 0 || response.direction != wantResponse || response.ordinal <= request.ordinal {
		return false, fmt.Errorf("direction or ordinal")
	}
	resp, err := decodeCandidateFrame(response.raw)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(bytes.TrimSpace(req["id"]), bytes.TrimSpace(resp["id"])) || len(resp["id"]) == 0 {
		return false, fmt.Errorf("response id")
	}
	_, result := resp["result"]
	_, rpcError := resp["error"]
	if result == rpcError {
		return false, fmt.Errorf("response result/error shape")
	}
	if len(observed) != 2 || observed[0].direction != request.direction || observed[0].ordinal != request.ordinal || !bytes.Equal(observed[0].raw, request.raw) || observed[1].direction != response.direction || observed[1].ordinal != response.ordinal || !bytes.Equal(observed[1].raw, response.raw) {
		return false, fmt.Errorf("unmatched or extra capability frame")
	}
	return result && response.ordinal < write, nil
}

func TestV4RejectedCapabilityExchangeCandidates(t *testing.T) {
	frame := func(v any) []byte {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte("Content-Length: "+strconv.Itoa(len(b))+"\r\n\r\n"), b...)
	}
	for _, method := range []string{"references", "definition"} {
		req := candidateFrame{"SERVER_TO_CLIENT", 1, frame(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "client/registerCapability", "params": map[string]any{"registrations": []any{map[string]any{"id": "r", "method": "textDocument/" + method}}}})}
		resp := candidateFrame{"CLIENT_TO_SERVER", 2, frame(map[string]any{"jsonrpc": "2.0", "id": 1, "result": nil})}
		if ok, err := candidateEffective(req, resp, []candidateFrame{req, resp}, "client/registerCapability", 10); err != nil || !ok {
			t.Fatalf("%s valid pair: %v", method, err)
		}
		late := resp
		late.ordinal = 11
		if ok, err := candidateEffective(req, late, []candidateFrame{req, late}, "client/registerCapability", 10); err != nil || ok {
			t.Fatalf("%s late success gained support: %v", method, err)
		}
		mismatch := resp
		mismatch.raw = frame(map[string]any{"jsonrpc": "2.0", "id": 2, "result": nil})
		reversed := resp
		reversed.ordinal = 0
		wrong := resp
		wrong.direction = "SERVER_TO_CLIENT"
		for _, tc := range []struct {
			name     string
			response candidateFrame
			stream   []candidateFrame
		}{
			{"mismatched ID", mismatch, []candidateFrame{req, mismatch}},
			{"reversed response", reversed, []candidateFrame{req, reversed}},
			{"wrong direction", wrong, []candidateFrame{req, wrong}},
			{"missing response", resp, []candidateFrame{req}},
			{"duplicate response", resp, []candidateFrame{req, resp, {resp.direction, 3, resp.raw}}},
		} {
			if ok, err := candidateEffective(req, tc.response, tc.stream, "client/registerCapability", 10); err == nil || ok {
				t.Errorf("%s/%s issued candidate without rejecting malformed exchange", method, tc.name)
			}
		}
		initialize := candidateFrame{"CLIENT_TO_SERVER", 1, frame(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})}
		initResult := candidateFrame{"SERVER_TO_CLIENT", 2, frame(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"capabilities": map[string]any{}}})}
		if ok, err := candidateEffective(initialize, initResult, []candidateFrame{initialize, initResult}, "initialize", 10); err != nil || !ok {
			t.Errorf("%s initialize: %v", method, err)
		}
		unregister := candidateFrame{"SERVER_TO_CLIENT", 1, frame(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "client/unregisterCapability", "params": map[string]any{"unregisterations": []any{map[string]any{"id": "r", "method": "textDocument/" + method}}}})}
		if ok, err := candidateEffective(unregister, resp, []candidateFrame{unregister, resp}, "client/unregisterCapability", 10); err != nil || !ok {
			t.Errorf("%s unregister response: %v", method, err)
		}
	}
}
