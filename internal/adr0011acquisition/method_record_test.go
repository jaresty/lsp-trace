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

func TestPrivateMethodRecordSynthetic(t *testing.T) {
	root, ri := responseReadFixture(t, "[]")
	response, e := publishResponseRead(root, ri, nil)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := publishRawResult(root, response, ri.Payload, ri, nil)
	if e != nil {
		t.Fatal(e)
	}
	x := methodRecordInputs{ri, response, raw}
	state, e := publishMethodRecord(root, x, nil)
	if e != nil || state.stage != "VERIFIED" || !replayMethodRecord(root, state, x, nil) {
		t.Fatalf("ASSERT_METHOD_VERIFIED: %+v %v", state, e)
	}
	b, e := publication.ReadVerifiedBoundFile(root, state.selector, sourceRecordLimit)
	if e != nil {
		t.Fatal(e)
	}
	expected, ok := methodRecordExpected(root, x, publication.ReadVerifiedBoundFile)
	canonical, e := methodRecordBytes(expected)
	if !ok || e != nil || !bytes.Equal(b, canonical) || b[len(b)-1] != '\n' || bytes.HasSuffix(b, []byte("\n\n")) || state.digest != privateDigest(b) || state.selector != methodRecordSelector(state.digest) || bytes.Contains(b, []byte("ResultBase64")) {
		t.Fatal("ASSERT_METHOD_EXACT_LF_DIGEST")
	}
	contract, e := os.ReadFile("../../docs/qualification/schemas/adr0011-references-issuance-records.request-key-v1.proposed.schema.json")
	if e != nil {
		t.Fatal(e)
	}
	if strings.TrimPrefix(privateDigest(contract), "sha256:") != reviewedSuccessorSchemaDigest {
		t.Fatal("ASSERT_METHOD_SCHEMA_PIN")
	}
	var schema, value any
	if json.Unmarshal(contract, &schema) != nil || json.Unmarshal(b, &value) != nil {
		t.Fatal("ASSERT_METHOD_JSON")
	}
	id := schema.(map[string]any)["$id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if e := compiler.AddResource(id, schema); e != nil {
		t.Fatal(e)
	}
	shape, e := compiler.Compile(id + "#/$defs/methodRecord")
	if e != nil || shape.Validate(value) != nil {
		t.Fatalf("ASSERT_METHOD_SCHEMA: %v %s", e, b)
	}
	if expected.TargetRef.Digest != ri.Target.digest || expected.SourceRef.Digest != ri.TargetInputs.Source.digest || expected.RevisionRef.Digest != ri.TargetInputs.Revision.digest || expected.ResponseReadRef.Digest != response.digest || expected.RawResultRef.Digest != raw.digest {
		t.Fatal("ASSERT_METHOD_REFS")
	}
	for name, mutate := range map[string]func(*methodRecordInputs){
		"source": func(y *methodRecordInputs) {
			y.ResponseInputs.TargetInputs.Source.digest = "sha256:" + strings.Repeat("0", 64)
		},
		"revision": func(y *methodRecordInputs) {
			y.ResponseInputs.TargetInputs.Revision.digest = "sha256:" + strings.Repeat("0", 64)
		},
		"target":      func(y *methodRecordInputs) { y.ResponseInputs.Target.digest = "sha256:" + strings.Repeat("0", 64) },
		"request-key": func(y *methodRecordInputs) { y.ResponseInputs.Pair.Key.ID++ },
		"format": func(y *methodRecordInputs) {
			y.ResponseInputs.Frames.request = bytes.Replace(y.ResponseInputs.Frames.request, []byte(`"jsonrpc":"2.0"`), []byte(`"jsonrpc": "2.0"`), 1)
		},
		"payload":    func(y *methodRecordInputs) { y.ResponseInputs.Payload.stage = "ABSENT" },
		"raw":        func(y *methodRecordInputs) { y.RawResult.digest = "sha256:" + strings.Repeat("0", 64) },
		"session":    func(y *methodRecordInputs) { y.ResponseInputs.Pair.SessionID = "other" },
		"generation": func(y *methodRecordInputs) { y.ResponseInputs.Pair.Generation++ },
		"invocation": func(y *methodRecordInputs) { y.ResponseInputs.Frames.invocation = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			y := x
			mutate(&y)
			if replayMethodRecord(root, state, y, nil) {
				t.Fatal("ASSERT_METHOD_SUBSTITUTION")
			}
			got, e := publishMethodRecord(root, y, nil)
			if e == nil || got.stage != "ABSENT" {
				t.Fatalf("ASSERT_METHOD_PRECOMMIT %+v %v", got, e)
			}
		})
	}
	for name, altered := range map[string][]byte{
		"duplicate": bytes.Replace(b, []byte(`"Method":`), []byte(`"Method":"textDocument/references","Method":`), 1),
		"unknown":   bytes.Replace(b, []byte(`"Method":`), []byte(`"unknown":1,"Method":`), 1),
		"no-LF":     bytes.TrimSuffix(b, []byte("\n")),
		"wrong-ref": bytes.Replace(b, []byte(raw.digest), []byte("sha256:"+strings.Repeat("0", 64)), 1),
	} {
		t.Run(name, func(t *testing.T) {
			d := privateDigest(altered)
			forged := privateBodyPublication{methodRecordSelector(d), d, len(altered), "VERIFIED"}
			if _, e := publication.PublishBoundFile(root, forged.selector, altered, func([]byte) error { return nil }); e != nil {
				t.Fatal(e)
			}
			if replayMethodRecord(root, forged, x, nil) {
				t.Fatal("ASSERT_METHOD_FORGERY")
			}
		})
	}
	missing := func(r *publication.Root, s string, n int64) ([]byte, error) {
		if s == ri.Payload.selector {
			return nil, errors.New("missing payload")
		}
		return publication.ReadVerifiedBoundFile(r, s, n)
	}
	if replayMethodRecord(root, state, x, missing) {
		t.Fatal("ASSERT_METHOD_MISSING_PAYLOAD")
	}
	again, e := publishMethodRecord(root, x, nil)
	if e == nil || again.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_METHOD_COLLISION %+v %v", again, e)
	}
}
