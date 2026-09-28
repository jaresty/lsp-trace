package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/publication"
)

func TestPrivateRawResultManifestSynthetic(t *testing.T) {
	if privateDigest([]byte("null")) == privateDigest([]byte("[]")) {
		t.Fatal("ASSERT_RAW_MANIFEST_NULL_ARRAY_DISTINCT")
	}
	for _, token := range []string{"null", "[]"} {
		t.Run(token, func(t *testing.T) {
			root, x := responseReadFixture(t, token)
			response, err := publishResponseRead(root, x, nil)
			if err != nil {
				t.Fatal(err)
			}
			state, err := publishRawResult(root, response, x.Payload, x, nil)
			if err != nil || state.stage != "VERIFIED" || !replayRawResult(root, state, response, x.Payload, x, nil) {
				t.Fatalf("ASSERT_RAW_MANIFEST_VERIFIED: %+v %v", state, err)
			}
			raw, err := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
			if err != nil {
				t.Fatal(err)
			}
			contract, err := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimPrefix(privateDigest(contract), "sha256:") != reviewedSuccessorSchemaDigest {
				t.Fatal("ASSERT_RAW_MANIFEST_SCHEMA_PIN")
			}
			var schema any
			if err = json.Unmarshal(contract, &schema); err != nil {
				t.Fatal(err)
			}
			id := schema.(map[string]any)["$id"].(string)
			compiler := jsonschema.NewCompiler()
			compiler.AssertFormat()
			if err = compiler.AddResource(id, schema); err != nil {
				t.Fatal(err)
			}
			shape, err := compiler.Compile(id + "#/$defs/rawResult")
			if err != nil {
				t.Fatal(err)
			}
			var value any
			if json.Unmarshal(raw, &value) != nil || shape.Validate(value) != nil {
				t.Fatalf("ASSERT_RAW_MANIFEST_SCHEMA: %s", raw)
			}
			var record rawResultRecord
			if json.Unmarshal(raw, &record) != nil || record.PayloadDigest != privateDigest([]byte(token)) || record.PayloadByteLength != len(token) || record.ResponseReadRef.Digest != response.digest || record.Access != "OWNER_ONLY" {
				t.Fatalf("ASSERT_RAW_MANIFEST_EXACT: %s", raw)
			}
			for name, mutate := range map[string]func(*responseReadInputs, *privateBodyPublication, *privateBodyPublication){
				"missing-payload": func(y *responseReadInputs, r, p *privateBodyPublication) { p.stage = "ABSENT" },
				"changed-payload": func(y *responseReadInputs, r, p *privateBodyPublication) {
					p.digest = "sha256:" + strings.Repeat("0", 64)
				},
				"changed-response-ref": func(y *responseReadInputs, r, p *privateBodyPublication) {
					r.digest = "sha256:" + strings.Repeat("0", 64)
				},
				"wrong-length": func(y *responseReadInputs, r, p *privateBodyPublication) { p.byteCount++ },
				"wrong-schema": func(y *responseReadInputs, r, p *privateBodyPublication) {
					y.SchemaDigest = "sha256:" + strings.Repeat("0", 64)
				},
				"changed-raw-id": func(y *responseReadInputs, r, p *privateBodyPublication) {
					y.Frames.response = bytes.Replace(y.Frames.response, []byte(`"id":31`), []byte(`"id":32`), 1)
				},
			} {
				t.Run(name, func(t *testing.T) {
					y, r, p := x, response, x.Payload
					mutate(&y, &r, &p)
					if replayRawResult(root, state, r, p, y, nil) {
						t.Fatal("ASSERT_RAW_MANIFEST_REJECT_" + name)
					}
					got, e := publishRawResult(root, r, p, y, nil)
					if e == nil || got.stage != "ABSENT" {
						t.Fatalf("ASSERT_RAW_MANIFEST_PRECOMMIT: %+v %v", got, e)
					}
				})
			}
			missing := func(r *publication.Root, s string, n int64) ([]byte, error) {
				if s == x.Payload.selector {
					return nil, errors.New("missing")
				}
				return publication.ReadVerifiedBoundFile(r, s, n)
			}
			changed := func(r *publication.Root, s string, n int64) ([]byte, error) {
				if s == x.Payload.selector {
					return []byte("{}"), nil
				}
				return publication.ReadVerifiedBoundFile(r, s, n)
			}
			if replayRawResult(root, state, response, x.Payload, x, missing) || replayRawResult(root, state, response, x.Payload, x, changed) {
				t.Fatal("ASSERT_RAW_MANIFEST_PAYLOAD_REPLAY")
			}
			for name, altered := range map[string][]byte{
				"duplicate": bytes.Replace(raw, []byte(`"access":`), []byte(`"access":"OWNER_ONLY","access":`), 1),
				"extra":     bytes.Replace(raw, []byte(`"access":`), []byte(`"extra":1,"access":`), 1),
				"digest":    bytes.Replace(raw, []byte(record.PayloadDigest), []byte("sha256:"+strings.Repeat("0", 64)), 1),
				"length":    bytes.Replace(raw, []byte(`"payload_byte_length":`+stringNumber(len(token))), []byte(`"payload_byte_length":99`), 1),
			} {
				t.Run(name, func(t *testing.T) {
					d := rawResultDigest(altered)
					forged := privateBodyPublication{rawResultSelector(d), d, len(altered), "VERIFIED"}
					if _, e := publication.PublishBoundFile(root, forged.selector, altered, func([]byte) error { return nil }); e != nil {
						t.Fatal(e)
					}
					if replayRawResult(root, forged, response, x.Payload, x, nil) {
						t.Fatal("ASSERT_RAW_MANIFEST_FORGED_" + name)
					}
				})
			}
			again, e := publishRawResult(root, response, x.Payload, x, nil)
			if e == nil || again.stage != "COMMITTED_UNVERIFIED" {
				t.Fatalf("ASSERT_RAW_MANIFEST_NO_REPLACE: %+v %v", again, e)
			}
		})
	}
}
func stringNumber(n int) string { return string([]byte{'0' + byte(n)}) }
