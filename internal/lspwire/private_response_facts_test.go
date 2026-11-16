package lspwire

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestPrivateResponseFactsWire(t *testing.T) {
	cases := []struct {
		name, body string
		shape      ResponseValueKind
		code       ResponseCodeState
		value      int64
		result     ResponseValueKind
		assoc      ResponseAssociation
	}{
		{"zero", `{"jsonrpc":"2.0","id":7,"error":{"code":0,"message":"PRIVATE_SENTINEL","data":{"uri":"PRIVATE_SENTINEL"}}}`, ResponseObject, ResponseCodeValid, 0, ResponseAbsent, ResponseIDMatched},
		{"missing", `{"jsonrpc":"2.0","id":7,"error":{}}`, ResponseObject, ResponseCodeAbsent, 0, ResponseAbsent, ResponseIDMatched},
		{"null-code", `{"jsonrpc":"2.0","id":7,"error":{"code":null}}`, ResponseObject, ResponseCodeNull, 0, ResponseAbsent, ResponseIDMatched},
		{"standard", `{"jsonrpc":"2.0","id":7,"error":{"code":-32601}}`, ResponseObject, ResponseCodeValid, -32601, ResponseAbsent, ResponseIDMatched},
		{"unlisted", `{"jsonrpc":"2.0","id":7,"error":{"code":123}}`, ResponseObject, ResponseCodeValid, 123, ResponseAbsent, ResponseIDMatched},
		{"max", `{"jsonrpc":"2.0","id":7,"error":{"code":9223372036854775807}}`, ResponseObject, ResponseCodeValid, 9223372036854775807, ResponseAbsent, ResponseIDMatched},
		{"min", `{"jsonrpc":"2.0","id":7,"error":{"code":-9223372036854775808}}`, ResponseObject, ResponseCodeValid, -9223372036854775808, ResponseAbsent, ResponseIDMatched},
		{"bool", `{"jsonrpc":"2.0","id":7,"error":{"code":true}}`, ResponseObject, ResponseCodeWrongType, 0, ResponseAbsent, ResponseIDMatched},
		{"fraction", `{"jsonrpc":"2.0","id":7,"error":{"code":1.0}}`, ResponseObject, ResponseCodeNotInteger, 0, ResponseAbsent, ResponseIDMatched},
		{"exponent", `{"jsonrpc":"2.0","id":7,"error":{"code":1e0}}`, ResponseObject, ResponseCodeNotInteger, 0, ResponseAbsent, ResponseIDMatched},
		{"overflow", `{"jsonrpc":"2.0","id":7,"error":{"code":9223372036854775808}}`, ResponseObject, ResponseCodeOutOfRange, 0, ResponseAbsent, ResponseIDMatched},
		{"null-error", `{"jsonrpc":"2.0","id":7,"error":null,"result":null}`, ResponseNull, ResponseCodeAbsent, 0, ResponseNull, ResponseIDMatched},
		{"absent-error", `{"jsonrpc":"2.0","id":7,"result":[]}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseArray, ResponseIDMatched},
		{"scalar-error", `{"jsonrpc":"2.0","id":7,"error":true}`, ResponseBoolean, ResponseCodeAbsent, 0, ResponseAbsent, ResponseIDMatched},
		{"repeated", `{"jsonrpc":"2.0","id":7,"error":{"code":2},"error":{}}`, ResponseObject, ResponseCodeAbsent, 0, ResponseAbsent, ResponseIDMatched},
		{"unmatched", `{"jsonrpc":"2.0","id":8,"result":{}}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseObject, ResponseIDUnmatched},
		{"string-id", `{"jsonrpc":"2.0","id":"PRIVATE_SENTINEL","result":0}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNumber, ResponseIDInvalid},
		{"missing-id", `{"jsonrpc":"2.0","result":null}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNull, ResponseIDInvalid},
		{"null-id", `{"jsonrpc":"2.0","id":null,"result":null}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNull, ResponseIDInvalid},
		{"fraction-id", `{"jsonrpc":"2.0","id":7.0,"result":null}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNull, ResponseIDInvalid},
		{"negative-id", `{"jsonrpc":"2.0","id":-7,"result":null}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNull, ResponseIDInvalid},
		{"object-id", `{"jsonrpc":"2.0","id":{},"result":null}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNull, ResponseIDInvalid},
		{"overflow-id", `{"jsonrpc":"2.0","id":18446744073709551616,"result":null}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNull, ResponseIDInvalid},
		{"max-id", `{"jsonrpc":"2.0","id":18446744073709551615,"result":null}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNull, ResponseIDUnmatched},
		{"repeat-id", `{"jsonrpc":"2.0","id":8,"id":7,"result":null}`, ResponseAbsent, ResponseCodeAbsent, 0, ResponseNull, ResponseIDMatched},
		{"repeat-code", `{"jsonrpc":"2.0","id":7,"error":{"code":true,"code":0}}`, ResponseObject, ResponseCodeValid, 0, ResponseAbsent, ResponseIDMatched},
		{"repeat-null-code", `{"jsonrpc":"2.0","id":7,"error":{"code":4},"error":{"code":null}}`, ResponseObject, ResponseCodeNull, 0, ResponseAbsent, ResponseIDMatched},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wire := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(c.body), c.body))
			m, _, raw, n, f, e := ReadPrivateDiscoveryFrameFacts(context.Background(), bytes.NewReader(wire), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil }, 7)
			if f.State != ResponseValidJSON || f.Error != c.shape || f.CodeState != c.code || f.Code != c.value || f.Result != c.result || f.Association != c.assoc {
				t.Fatalf("P1 original wire projection: got=%+v", f)
			}
			legacy, _, _, ln, le := ReadPrivateDiscoveryFrame(context.Background(), bytes.NewReader(wire), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil })
			if (le == nil) != (e == nil) || le != e || ln != n {
				t.Fatal("P3 legacy outcome changed")
			}
			a, _ := json.Marshal(m)
			b, _ := json.Marshal(legacy)
			if !bytes.Equal(a, b) {
				t.Fatal("P3 legacy message changed")
			}
			if e != nil && (raw != nil || n != 0) {
				t.Fatal("P3 decode failure promoted to success")
			}
			encoded, err := json.Marshal(f)
			if err != nil || bytes.Contains(encoded, []byte("PRIVATE_SENTINEL")) {
				t.Fatal("P5 payload retention")
			}
		})
	}
}
func TestPrivateResponseFactsUnavailable(t *testing.T) {
	for _, body := range []string{`{"id":`, ``} {
		wire := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
		_, _, _, _, f, e := ReadPrivateDiscoveryFrameFacts(context.Background(), bytes.NewReader(wire), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil }, 7)
		if e == nil || f.State != ResponseInvalidJSON || f.Association != ResponseUnassociated {
			t.Fatal("P1 invalid JSON facts", f, e)
		}
	}
	_, _, _, _, f, e := ReadPrivateDiscoveryFrameFacts(context.Background(), bytes.NewReader(nil), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil }, 7)
	if e == nil || f != (ResponseFacts{}) {
		t.Fatal("P1 no frame is not observed absent", f, e)
	}
}
func TestPrivateResponseFactsStrict(t *testing.T) {
	for _, f := range []ResponseFacts{{State: 99}, {Code: 1}, {State: ResponseValidJSON, Code: 1}, {State: ResponseValidJSON, Error: ResponseObject, CodeState: 99}, {State: ResponseValidJSON, Association: ResponseIDMatched, ID: ResponseString}} {
		if _, e := json.Marshal(f); e == nil {
			t.Fatal("P5 invalid facts serialized")
		}
	}
}
