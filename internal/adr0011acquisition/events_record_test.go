package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/publication"
)

const eventsLocation = `{"uri":"file:///a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`

func TestPrivateEventsSynthetic(t *testing.T) {
	contract, err := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if json.Unmarshal(contract, &schema) != nil {
		t.Fatal("schema")
	}
	id := schema.(map[string]any)["$id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err = compiler.AddResource(id, schema); err != nil {
		t.Fatal(err)
	}
	shape, err := compiler.Compile(id + "#/$defs/events")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"null", "[]", "[" + eventsLocation + "," + eventsLocation + "]"} {
		t.Run(token, func(t *testing.T) {
			root, x := responseReadFixture(t, token)
			response, e := publishResponseRead(root, x, nil)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := publishRawResult(root, response, x.Payload, x, nil)
			if e != nil {
				t.Fatal(e)
			}
			scanner, e := observeRawScanner(root, x.Payload, nil)
			if e != nil {
				t.Fatal(e)
			}
			expected, ok := responseReadExpected(root, x, publication.ReadVerifiedBoundFile)
			if !ok {
				t.Fatal("response")
			}
			identity := adr0011methodresult.PrivateTransitionIdentity{Transaction: expected.TransactionID, RequestKey: expected.RequestKey, Invocation: expected.InvocationID, ResponseRead: response.selector, RawSelector: raw.selector, RawDigest: privateDigest([]byte(token))}
			evaluation, journal, e := adr0011methodresult.RunPrivateAttachedReferences(root, x.Pair.Key, []byte(token), identity)
			if e != nil || evaluation == nil {
				t.Fatal("attached", e)
			}
			state, e := publishEvents(root, response, raw, scanner, evaluation, x, expected.TransactionID, []byte(token), journal, nil, nil)
			if e != nil || state.stage != "VERIFIED" || !replayEvents(root, state, response, raw, scanner, x, expected.TransactionID, []byte(token), journal, nil) {
				t.Fatalf("ASSERT_EVENTS_VERIFIED: %+v %v", state, e)
			}
			b, e := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
			if e != nil {
				t.Fatal(e)
			}
			var value any
			if json.Unmarshal(b, &value) != nil || shape.Validate(value) != nil {
				t.Fatalf("ASSERT_EVENTS_SCHEMA: %s %v", b, shape.Validate(value))
			}
			for name, invalid := range map[string][]byte{
				"missing-field":        bytes.Replace(b, []byte(`"schema_version":"REFERENCES_EVALUATOR_EVENTS_V1",`), nil, 1),
				"unknown-field":        bytes.Replace(b, []byte(`"events":`), []byte(`"extra":1,"events":`), 1),
				"null-element-ordinal": bytes.Replace(b, []byte(`"kind":"ELEMENT_BEGIN","ordinal":0`), []byte(`"kind":"ELEMENT_BEGIN","ordinal":null`), 1),
			} {
				if bytes.Equal(invalid, b) {
					continue
				}
				var rejected any
				if json.Unmarshal(invalid, &rejected) != nil || shape.Validate(rejected) == nil {
					t.Fatalf("ASSERT_EVENTS_SCHEMA_REJECT_%s", name)
				}
			}
			var record eventsRecord
			if json.Unmarshal(b, &record) != nil || len(record.Events) != 2+2*scanner.knownE || record.Events[0].Ordinal != nil || record.Events[len(record.Events)-1].Ordinal != nil || record.Events[len(record.Events)-1].TerminalDisposition != evaluation.Disposition {
				t.Fatal("ASSERT_EVENTS_GRAMMAR")
			}
			for name, altered := range map[string][]byte{
				"duplicate": bytes.Replace(b, []byte(`"events":`), []byte(`"events":[],"events":`), 1),
				"unknown":   bytes.Replace(b, []byte(`"events":`), []byte(`"unknown":1,"events":`), 1),
				"wrong-ref": bytes.Replace(b, []byte(raw.digest), []byte("sha256:"+strings.Repeat("0", 64)), 1),
			} {
				t.Run(name, func(t *testing.T) {
					d := eventsDigest(altered)
					forged := privateBodyPublication{eventsSelector(d), d, len(altered), "VERIFIED"}
					if _, e := publication.PublishBoundFile(root, forged.selector, altered, func([]byte) error { return nil }); e != nil {
						t.Fatal(e)
					}
					if replayEvents(root, forged, response, raw, scanner, x, expected.TransactionID, []byte(token), journal, nil) {
						t.Fatal("ASSERT_EVENTS_FORGERY")
					}
				})
			}
			if replayEvents(root, state, response, raw, scanner, x, expected.TransactionID, []byte(token), []byte("{}"), nil) || replayEvents(root, state, response, raw, scanner, x, expected.TransactionID, []byte("[]changed"), journal, nil) {
				t.Fatal("ASSERT_EVENTS_EXTERNAL_REPLAY")
			}
			changed := scanner
			changed.knownE++
			if replayEvents(root, state, response, raw, changed, x, expected.TransactionID, []byte(token), journal, nil) {
				t.Fatal("ASSERT_EVENTS_SCANNER_SWAP")
			}
			again, e := publishEvents(root, response, raw, scanner, evaluation, x, expected.TransactionID, []byte(token), journal, nil, nil)
			if e == nil || again.stage != "COMMITTED_UNVERIFIED" {
				t.Fatalf("ASSERT_EVENTS_NO_REPLACE: %+v %v", again, e)
			}
		})
	}
}
