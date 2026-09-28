package adr0011requestkey

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/lspwire"
)

func TestCanonical(t *testing.T) {
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	if got := Encode(key); got != "lsp-trace.request-key.v1:g=7;id=31" {
		t.Fatalf("encode: %q", got)
	}
	if got, err := Parse(Encode(key)); err != nil || got != key {
		t.Fatalf("parse: %+v %v", got, err)
	}
}

func TestRejectNoncanonical(t *testing.T) {
	for _, input := range []string{
		"lsp-trace.request-key.v1:g=07;id=31", "lsp-trace.request-key.v1:id=31",
		"lsp-trace.request-key.v1:id=31;g=7", "lsp-trace.request-key.v1:g=7,id=31",
		"lsp-trace.request-key.v2:g=7;id=31", "lsp-trace.request-key.v1:g=7;id=+31",
		"lsp-trace.request-key.v1:g=7;id=031", "lsp-trace.request-key.v1:g=7;id=31 ",
		"lsp-trace.request-key.v1:g=18446744073709551616;id=31",
	} {
		if _, err := Parse(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestIdentityRejectsSubstitution(t *testing.T) {
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	text := Encode(key)
	cases := []struct {
		name                 string
		runtime              lspwire.RequestKey
		generation, wire     uint64
		request, response    json.RawMessage
		invocation, expected string
	}{
		{"match", key, 7, 31, json.RawMessage(`31`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"runtime-generation", lspwire.RequestKey{Generation: 8, ID: 31}, 7, 31, json.RawMessage(`31`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"enclosing-generation", key, 8, 31, json.RawMessage(`31`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"runtime-id", lspwire.RequestKey{Generation: 7, ID: 32}, 7, 31, json.RawMessage(`31`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"owner-wire-id", key, 7, 32, json.RawMessage(`31`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"request-id", key, 7, 31, json.RawMessage(`32`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"response-id", key, 7, 31, json.RawMessage(`31`), json.RawMessage(`32`), "inv-a", "inv-a"},
		{"string-request-id", key, 7, 31, json.RawMessage(`"31"`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"string-response-id", key, 7, 31, json.RawMessage(`31`), json.RawMessage(`"31"`), "inv-a", "inv-a"},
		{"noncanonical-request-id", key, 7, 31, json.RawMessage(`031`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"float-request-id", key, 7, 31, json.RawMessage(`31.0`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"signed-request-id", key, 7, 31, json.RawMessage(`+31`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"null-request-id", key, 7, 31, json.RawMessage(`null`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"overflow-request-id", key, 7, 31, json.RawMessage(`18446744073709551616`), json.RawMessage(`31`), "inv-a", "inv-a"},
		{"float-response-id", key, 7, 31, json.RawMessage(`31`), json.RawMessage(`31.0`), "inv-a", "inv-a"},
		{"cross-invocation", key, 7, 31, json.RawMessage(`31`), json.RawMessage(`31`), "inv-b", "inv-a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Identity(text, tc.runtime, tc.generation, tc.wire, tc.request, tc.response, tc.invocation, tc.expected)
			if (err == nil) != (tc.name == "match") {
				t.Fatalf("unexpected identity result: %v", err)
			}
		})
	}
}
