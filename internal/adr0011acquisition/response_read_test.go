package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/publication"
)

func responseReadFixture(t *testing.T, token string) (*publication.Root, responseReadInputs) {
	t.Helper()
	root, targetInputs := targetRecordFixture(t)
	target, e := publishTargetRecord(root, targetInputs, nil)
	if e != nil {
		t.Fatal(e)
	}
	f, p, b := ownerReadFixture(t, "textDocument/references")
	b.SHA256 = targetInputs.Binding.SHA256
	p.Source = &b
	params := bytes.Replace(p.Params, []byte(`"character":2`), []byte(`"character":4`), 1)
	p.Params = params
	request := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":31,"method":"textDocument/references","params":%s}`, params))
	f.request = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(request), request))
	p.Write.FrameBytes = int64(len(f.request))
	p.Write.FrameSHA256 = privateDigest(f.request)
	response := []byte(`{"jsonrpc":"2.0","id":31,"result":` + token + `}`)
	f.response = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(response), response))
	p.Read.FrameBytes = int64(len(f.response))
	p.Read.FrameSHA256 = privateDigest(f.response)
	p.Result = []byte(token)
	f.pair = p
	var ok bool
	f.paramsSpan, ok = exactOwnedSpan(f.request, "params", p.Method, p.Key.ID)
	if !ok {
		t.Fatal("params")
	}
	f.resultSpan, ok = exactOwnedSpan(f.response, "result", p.Method, p.Key.ID)
	if !ok {
		t.Fatal("result")
	}
	owner, e := publishOwnerRead(root, f, p, b, p.SessionID, p.Generation, f.invocation, reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	payload, e := publishRawResultPayload(root, owner, p, b, p.SessionID, p.Generation, f.invocation, reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	return root, responseReadInputs{targetInputs, target, f, p, b, targetInputs.Query, owner, payload, reviewedSuccessorSchemaDigest}
}
func TestPrivateResponseReadSynthetic(t *testing.T) {
	for _, token := range []string{"null", "[]"} {
		t.Run(token, func(t *testing.T) {
			root, x := responseReadFixture(t, token)
			state, e := publishResponseRead(root, x, nil)
			if e != nil || state.stage != "VERIFIED" {
				t.Fatalf("ASSERT_RESPONSE_READ_VERIFIED: %+v %v", state, e)
			}
			raw, e := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
			if e != nil || !replayResponseRead(root, state, x, nil) {
				t.Fatal("ASSERT_RESPONSE_READ_REPLAY", e)
			}
			var record responseReadRecord
			if json.Unmarshal(raw, &record) != nil || record.RawResultDigest != privateDigest([]byte(token)) || record.RawResultByteLength != len(token) || record.ResultPresence != "PRESENT" || record.TargetRef.Digest != x.Target.digest {
				t.Fatalf("ASSERT_RESPONSE_READ_EXACT: %s", raw)
			}
			contract, e := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
			if e != nil {
				t.Fatal(e)
			}
			var schema any
			if json.Unmarshal(contract, &schema) != nil {
				t.Fatal("schema")
			}
			id := schema.(map[string]any)["$id"].(string)
			compiler := jsonschema.NewCompiler()
			compiler.AssertFormat()
			if e = compiler.AddResource(id, schema); e != nil {
				t.Fatal(e)
			}
			shape, e := compiler.Compile(id + "#/$defs/responseRead")
			if e != nil {
				t.Fatal(e)
			}
			var value any
			if json.Unmarshal(raw, &value) != nil || shape.Validate(value) != nil {
				t.Fatalf("ASSERT_RESPONSE_READ_SCHEMA: %s %v", raw, shape.Validate(value))
			}
			for name, mutate := range map[string]func(*responseReadInputs){
				"raw-id": func(y *responseReadInputs) {
					y.Frames.request = bytes.Replace(y.Frames.request, []byte(`"id":31`), []byte(`"id":32`), 1)
				},
				"write-whitespace": func(y *responseReadInputs) {
					reframeResponseTest(t, &y.Frames.request, &y.Pair.Write.FrameBytes, &y.Pair.Write.FrameSHA256)
					y.Frames.pair.Write = y.Pair.Write
				},
				"read-whitespace": func(y *responseReadInputs) {
					reframeResponseTest(t, &y.Frames.response, &y.Pair.Read.FrameBytes, &y.Pair.Read.FrameSHA256)
					y.Frames.pair.Read = y.Pair.Read
				},
				"point": func(y *responseReadInputs) { y.Query.Character++ },
				"context": func(y *responseReadInputs) {
					y.Frames.request = bytes.Replace(y.Frames.request, []byte(`"includeDeclaration":false`), []byte(`"includeDeclaration":true`), 1)
				},
				"result-token": func(y *responseReadInputs) {
					y.Frames.response = bytes.Replace(y.Frames.response, []byte(`"result":`+token), []byte(`"result":{}`), 1)
				}, "invocation": func(y *responseReadInputs) { y.Frames.invocation = "other" }, "generation": func(y *responseReadInputs) { y.Pair.Generation++ }, "target": func(y *responseReadInputs) { y.Target.stage = "ABSENT" }, "payload": func(y *responseReadInputs) { y.Payload.stage = "ABSENT" }, "body": func(y *responseReadInputs) { y.OwnerRead.stage = "ABSENT" }, "schema": func(y *responseReadInputs) { y.SchemaDigest = "sha256:" + strings.Repeat("0", 64) },
			} {
				t.Run(name, func(t *testing.T) {
					y := x
					mutate(&y)
					if replayResponseRead(root, state, y, nil) {
						t.Fatal("ASSERT_RESPONSE_READ_REJECT_" + name)
					}
				})
			}
			missing := func(r *publication.Root, s string, n int64) ([]byte, error) {
				if s == x.Payload.selector {
					return nil, errors.New("missing")
				}
				return publication.ReadVerifiedBoundFile(r, s, n)
			}
			if replayResponseRead(root, state, x, missing) {
				t.Fatal("ASSERT_RESPONSE_READ_MISSING_PAYLOAD")
			}
			for name, changed := range map[string][]byte{"duplicate": bytes.Replace(raw, []byte(`"method":`), []byte(`"method":"textDocument/references","method":`), 1), "extra": bytes.Replace(raw, []byte(`"method":`), []byte(`"unexpected":1,"method":`), 1), "transaction": bytes.Replace(raw, []byte(record.TransactionID), []byte("sha256:"+strings.Repeat("0", 64)), 1)} {
				t.Run(name, func(t *testing.T) {
					v := privateBodyPublication{selector: responseReadSelector(responseReadDigest(changed)), digest: responseReadDigest(changed), byteCount: len(changed), stage: "VERIFIED"}
					if _, e := publication.PublishBoundFile(root, v.selector, changed, func([]byte) error { return nil }); e != nil {
						t.Fatal(e)
					}
					if replayResponseRead(root, v, x, nil) {
						t.Fatal("ASSERT_RESPONSE_READ_RECORD_REJECT_" + name)
					}
				})
			}
			again, e := publishResponseRead(root, x, nil)
			if e == nil || again.stage != "COMMITTED_UNVERIFIED" {
				t.Fatalf("ASSERT_RESPONSE_READ_NO_REPLACE: %+v %v", again, e)
			}
		})
	}
}
func reframeResponseTest(t *testing.T, frame *[]byte, n *int64, digest *string) {
	t.Helper()
	body, ok := exactFrameBody(*frame)
	if !ok {
		t.Fatal("frame")
	}
	body = append([]byte{' '}, body...)
	*frame = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
	*n = int64(len(*frame))
	*digest = privateDigest(*frame)
}
