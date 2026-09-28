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

func TestPrivateScannerRecordSynthetic(t *testing.T) {
	contract, err := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if err := json.Unmarshal(contract, &schema); err != nil {
		t.Fatal(err)
	}
	id := schema.(map[string]any)["$id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(id, schema); err != nil {
		t.Fatal(err)
	}
	shape, err := compiler.Compile(id + "#/$defs/scanner")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, token, form string
		count             int
	}{
		{"null", "null", "NULL", 0}, {"empty", "[]", "ARRAY", 0},
		{"duplicates", `[{"uri":"file:///a"},{"uri":"file:///a"}]`, "ARRAY", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, x := responseReadFixture(t, tc.token)
			response, e := publishResponseRead(root, x, nil)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := publishRawResult(root, response, x.Payload, x, nil)
			if e != nil {
				t.Fatal(e)
			}
			state, e := publishScannerRecord(root, response, raw, x.Payload, x, nil, nil)
			if e != nil || state.stage != "VERIFIED" || !replayScannerRecord(root, state, response, raw, x.Payload, x, nil, nil) {
				t.Fatalf("ASSERT_SCANNER_VERIFIED: %+v %v", state, e)
			}
			b, e := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
			if e != nil {
				t.Fatal(e)
			}
			var v any
			if json.Unmarshal(b, &v) != nil || shape.Validate(v) != nil {
				t.Fatalf("ASSERT_SCANNER_SCHEMA: %s", b)
			}
			var record scannerRecord
			if json.Unmarshal(b, &record) != nil || record.TopLevelForm != tc.form || record.KnownE != tc.count || record.WorkUnits != len(tc.token) || record.ScannerImplementationDigest != scannerSourcePin {
				t.Fatalf("ASSERT_SCANNER_COUNT: %s", b)
			}
			source, e := scannerSourceBytes()
			if e != nil {
				t.Fatal(e)
			}
			badSource := func() ([]byte, error) { return append(append([]byte(nil), source...), 0), nil }
			missingSource := func() ([]byte, error) { return nil, errors.New("missing") }
			oversizedSource := func() ([]byte, error) { return make([]byte, scannerSourceLimit+1), nil }
			for name, reader := range map[string]func() ([]byte, error){"altered-pin": badSource, "missing-pin": missingSource, "oversized-pin": oversizedSource} {
				t.Run(name, func(t *testing.T) {
					if replayScannerRecord(root, state, response, raw, x.Payload, x, nil, reader) {
						t.Fatal("ASSERT_SCANNER_PIN_REPLAY")
					}
					got, e := publishScannerRecord(root, response, raw, x.Payload, x, nil, reader)
					if e == nil || got.stage != "ABSENT" {
						t.Fatalf("ASSERT_SCANNER_PIN_PREINSTALL: %+v %v", got, e)
					}
				})
			}
			for name, mutate := range map[string]func(*privateBodyPublication, *privateBodyPublication, *privateBodyPublication, *responseReadInputs){
				"response-ref": func(r, a, p *privateBodyPublication, y *responseReadInputs) {
					r.digest = "sha256:" + strings.Repeat("0", 64)
				},
				"raw-ref": func(r, a, p *privateBodyPublication, y *responseReadInputs) {
					a.digest = "sha256:" + strings.Repeat("0", 64)
				},
				"unverified-raw":      func(r, a, p *privateBodyPublication, y *responseReadInputs) { a.stage = "COMMITTED_UNVERIFIED" },
				"unverified-response": func(r, a, p *privateBodyPublication, y *responseReadInputs) { r.stage = "COMMITTED_UNVERIFIED" },
				"payload": func(r, a, p *privateBodyPublication, y *responseReadInputs) {
					p.digest = "sha256:" + strings.Repeat("0", 64)
				},
				"frame": func(r, a, p *privateBodyPublication, y *responseReadInputs) {
					y.Frames.response = bytes.Replace(y.Frames.response, []byte(`"id":31`), []byte(`"id":32`), 1)
				},
			} {
				t.Run(name, func(t *testing.T) {
					r, a, p, y := response, raw, x.Payload, x
					mutate(&r, &a, &p, &y)
					if replayScannerRecord(root, state, r, a, p, y, nil, nil) {
						t.Fatal("ASSERT_SCANNER_SUBSTITUTION_REPLAY")
					}
					got, e := publishScannerRecord(root, r, a, p, y, nil, nil)
					if e == nil || got.stage != "ABSENT" {
						t.Fatalf("ASSERT_SCANNER_SUBSTITUTION_PREINSTALL: %+v %v", got, e)
					}
				})
			}
			changed := func(r *publication.Root, s string, n int64) ([]byte, error) {
				if s == x.Payload.selector {
					return []byte("{}"), nil
				}
				return publication.ReadVerifiedBoundFile(r, s, n)
			}
			if replayScannerRecord(root, state, response, raw, x.Payload, x, changed, nil) {
				t.Fatal("ASSERT_SCANNER_RETAINED_RAW")
			}
			for name, altered := range map[string][]byte{
				"duplicate": bytes.Replace(b, []byte(`"known_e":`), []byte(`"known_e":0,"known_e":`), 1),
				"extra":     bytes.Replace(b, []byte(`"known_e":`), []byte(`"extra":1,"known_e":`), 1),
				"pin":       bytes.Replace(b, []byte(scannerSourcePin), []byte("sha256:"+strings.Repeat("0", 64)), 1),
				"count":     bytes.Replace(b, []byte(`"known_e":`+stringNumber(tc.count)), []byte(`"known_e":99`), 1),
			} {
				t.Run(name, func(t *testing.T) {
					d := scannerRecordDigest(altered)
					forged := privateBodyPublication{scannerRecordSelector(d), d, len(altered), "VERIFIED"}
					if _, e := publication.PublishBoundFile(root, forged.selector, altered, func([]byte) error { return nil }); e != nil {
						t.Fatal(e)
					}
					if replayScannerRecord(root, forged, response, raw, x.Payload, x, nil, nil) {
						t.Fatal("ASSERT_SCANNER_FORGED")
					}
				})
			}
			again, e := publishScannerRecord(root, response, raw, x.Payload, x, nil, nil)
			if e == nil || again.stage != "COMMITTED_UNVERIFIED" {
				t.Fatalf("ASSERT_SCANNER_NO_REPLACE: %+v %v", again, e)
			}
		})
	}
	for _, token := range []string{"{", `{"bad":1}`, `[1,]`, strings.Repeat(" ", rawResultPayloadLimit+1)} {
		observed := scanRawTopLevel([]byte(token))
		if observed.form != "MALFORMED" {
			t.Fatalf("ASSERT_SCANNER_MALFORMED: %q %+v", token[:min(len(token), 32)], observed)
		}
	}
}
