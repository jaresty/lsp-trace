package adr0011genericv4proposal

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestV4HeldResponseRejectsMethodAndMismatchedTypedID(t *testing.T) {
	wire := func(v any) []byte {
		t.Helper()
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append([]byte("Content-Length: "+strconv.Itoa(len(b))+"\r\n\r\n"), b...)
	}
	for _, method := range []string{"textDocument/references", "textDocument/definition"} {
		init := testExchange(t, "initialize", 1, 0, 1, nil, true)
		base := testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", method, "go")}, true)
		good := []heldCapabilityExchange{init, base}
		if got := stateAt(good, stream(good), 5, method, nil, "go", "CLAIMANT"); got.status != "SUPPORTED" {
			t.Errorf("%s baseline %+v", method, got)
		}
		for _, tc := range []struct {
			name string
			body map[string]any
		}{
			{"response request method", map[string]any{"jsonrpc": "2.0", "id": 2, "method": "client/registerCapability", "result": nil}},
			{"mismatched ID", map[string]any{"jsonrpc": "2.0", "id": 3, "result": nil}},
			{"mismatched ID type", map[string]any{"jsonrpc": "2.0", "id": "2", "result": nil}},
			{"boolean ID", map[string]any{"jsonrpc": "2.0", "id": true, "result": nil}},
			{"null error", map[string]any{"jsonrpc": "2.0", "id": 2, "error": nil}},
			{"boolean error code", map[string]any{"jsonrpc": "2.0", "id": 2, "error": map[string]any{"code": true, "message": "failed"}}},
			{"missing error message", map[string]any{"jsonrpc": "2.0", "id": 2, "error": map[string]any{"code": -32603}}},
		} {
			changed := base
			response := *base.response
			response.raw = wire(tc.body)
			changed.response = &response
			held := []heldCapabilityExchange{init, changed}
			if got := stateAt(held, stream(held), 5, method, nil, "go", "CLAIMANT"); got.status != "INVALID_CHRONOLOGY" {
				t.Errorf("%s/%s not rejected: %+v", method, tc.name, got)
			}
		}
	}
}
