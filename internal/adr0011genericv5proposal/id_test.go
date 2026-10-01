package adr0011genericv5proposal

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func idFrame(rawID string, response, rpcError bool) []byte {
	body := `{"jsonrpc":"2.0","id":` + rawID
	if response {
		if rpcError {
			body += `,"error":{"code":-32603,"message":"failed"}}`
		} else {
			body += `,"result":null}`
		}
	} else {
		body += `,"method":"initialize","params":{}}`
	}
	return append([]byte("Content-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
}

func TestV5IDCorrespondenceBeforeCapabilityState(t *testing.T) {
	for _, tc := range []struct {
		name, request, response string
		valid                   bool
		rpcError                bool
	}{
		{"true ↔ true", "true", "true", false, false},
		{"false ↔ false", "false", "false", false, false},
		{"null ↔ null", "null", "null", false, false},
		{"array ↔ array", "[]", "[]", false, false},
		{"object ↔ object", "{}", "{}", false, false},
		{"string ↔ number", `"1"`, "1", false, false},
		{"integer ↔ equivalent fraction", "1", "1.0", true, false},
		{"integer ↔ equivalent exponent", "1", "1e0", true, false},
		{"fraction ↔ equivalent exponent", "0.001", "1e-3", true, false},
		{"unequal numeric values", "1.01", "1.001", false, false},
		{"no IEEE-754 rounding", "9007199254740993", "9007199254740992", false, false},
		{"escaped ↔ literal equivalent string", `"a\u0062"`, `"ab"`, true, false},
		{"empty JSON string ID", `""`, `""`, true, false},
		{"escaped raw exactly 4096", `"` + strings.Repeat(`\u0061`, 682) + `aa"`, `"` + strings.Repeat(`\u0061`, 682) + `aa"`, true, false},
		{"escaped raw 4097", `"` + strings.Repeat(`\u0061`, 682) + `aaa"`, `"a"`, false, false},
		{"escaped raw 4202", `"` + strings.Repeat(`\u0061`, 700) + `"`, `"a"`, false, false},
		{"response escaped raw 4097", `"` + strings.Repeat("a", 685) + `"`, `"` + strings.Repeat(`\u0061`, 682) + `aaa"`, false, false},
		{"decoded exactly 1024", `"` + strings.Repeat("a", 1024) + `"`, `"` + strings.Repeat("a", 1024) + `"`, true, false},
		{"multibyte decoded exactly 1024", `"` + strings.Repeat("é", 512) + `"`, `"` + strings.Repeat("é", 512) + `"`, true, false},
		{"multibyte decoded 1026", `"` + strings.Repeat("é", 513) + `"`, `"` + strings.Repeat("é", 513) + `"`, false, false},
		{"decoded 1025", `"` + strings.Repeat("a", 1025) + `"`, `"` + strings.Repeat("a", 1025) + `"`, false, false},
		{"small decoded oversized escaped raw", `"` + strings.Repeat(`\u0061`, 683) + `"`, `"a"`, false, false},
		{"number raw exactly 4096", strings.Repeat("1", 4096), strings.Repeat("1", 4096), true, false},
		{"number raw 4097", strings.Repeat("1", 4097), "1", false, false},
		{"extreme exponent", "1e100001", "1", false, false},
		{"negative extreme exponent", "1e-100001", "1", false, false},
		{"bounded large exponent without expansion", "1e99999", "10e99998", true, false},
		{"malformed leading zero", "01", "1", false, false},
		{"malformed decimal", "1.", "1", false, false},
		{"non-JSON NaN", "NaN", "NaN", false, false},
		{"non-JSON Infinity", "Infinity", "Infinity", false, false},
		{"valid request ↔ error response equivalent decimal", "1e0", "1.0", true, true},
		{"error response with null ID", "1", "null", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, response := idFrame(tc.request, false, false), idFrame(tc.response, true, tc.rpcError)
			called := false
			status := CandidateState(request, response, func() string { called = true; return "SUPPORTED" })
			if tc.valid && (status != "SUPPORTED" || !called) || !tc.valid && (status != "INVALID_CHRONOLOGY" || called) {
				t.Errorf("%s: status=%s state_evaluated=%v valid=%v", tc.name, status, called, tc.valid)
			}
			if tc.valid {
				pair, err := ValidateFramedPair(request, response)
				if err != nil || !bytes.Equal(pair.Request.RawToken, []byte(tc.request)) || !bytes.Equal(pair.Response.RawToken, []byte(tc.response)) {
					t.Errorf("%s: original ID tokens lost or rejected: %v", tc.name, err)
				}
			}
		})
	}
}

func TestV5DuplicateDecodedIDAndMalformedResponse(t *testing.T) {
	request := idFrame("1", false, false)
	badDuplicate := `{"jsonrpc":"2.0","id":1,"\u0069d":1,"method":"initialize","params":{}}`
	duplicate := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(badDuplicate))), badDuplicate...)
	for _, tc := range []struct {
		name    string
		request []byte
		result  []byte
	}{
		{"duplicate decoded id", duplicate, idFrame("1", true, false)},
		{"response null error ID", request, idFrame("null", true, true)},
		{"response boolean ID", request, idFrame("true", true, false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			got := CandidateState(tc.request, tc.result, func() string { called = true; return "SUPPORTED" })
			if got != "INVALID_CHRONOLOGY" || called {
				t.Errorf("%s: got %s state_evaluated=%v", tc.name, got, called)
			}
		})
	}
}
