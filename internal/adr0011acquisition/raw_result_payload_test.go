package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func TestRawResultPayloadCheckpoint(t *testing.T) {
	for _, token := range []string{"null", "[]"} {
		t.Run(token, func(t *testing.T) {
			root := privatePublicationRoot(t)
			f, p, b := ownerReadFixture(t, "textDocument/references")
			body := []byte(`{ "jsonrpc" : "2.0", "id" : 31, "result" : ` + token + ` }`)
			f.response = []byte("Content-Length: " + strconvI(len(body)) + "\r\n\r\n" + string(body))
			p.Result = []byte(token)
			p.Read.FrameBytes = int64(len(f.response))
			p.Read.FrameSHA256 = privateDigest(f.response)
			f.pair = p
			var ok bool
			f.resultSpan, ok = exactOwnedSpan(f.response, "result", p.Method, p.Key.ID)
			if !ok {
				t.Fatal("fixture")
			}
			ref, err := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
			if err != nil || ref.stage != "VERIFIED" {
				t.Fatalf("owner-read: %+v %v", ref, err)
			}
			payload, err := publishRawResultPayload(root, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
			if err != nil || payload.stage != "VERIFIED" || payload.digest != privateDigest([]byte(token)) || payload.digest == privateDigest(body) {
				t.Fatalf("ASSERT_RAW_RESULT_EXACT: %+v %v", payload, err)
			}
			got, err := publication.ReadVerifiedBoundFile(root, payload.selector, 1048576)
			if err != nil || !bytes.Equal(got, []byte(token)) {
				t.Fatal("ASSERT_RAW_RESULT_READBACK")
			}
			if again, e := publishRawResultPayload(root, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil); e == nil || again.stage != "ABSENT" {
				t.Fatalf("ASSERT_RAW_RESULT_NO_REPLACE: %+v %v", again, e)
			}
		})
	}
}

func TestRawResultPayloadUnverified(t *testing.T) {
	root := privatePublicationRoot(t)
	f, p, b := ownerReadFixture(t, "textDocument/references")
	ref, err := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
	if err != nil {
		t.Fatal(err)
	}
	altered := func(r *publication.Root, s string, n int64) ([]byte, error) {
		if strings.Contains(s, "raw-payload") {
			return nil, errors.New("secret source")
		}
		return publication.ReadVerifiedBoundFile(r, s, n)
	}
	outcome, err := publishRawResultPayload(root, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, altered)
	if err == nil || strings.Contains(err.Error(), "secret") || outcome.stage != "COMMITTED_UNVERIFIED" || outcome.digest != privateDigest([]byte("null")) || outcome.selector == "" {
		t.Fatalf("ASSERT_RAW_RESULT_COMMITTED_UNVERIFIED: %+v %v", outcome, err)
	}
}

func TestRawResultPayloadRejectsChangedEvidence(t *testing.T) {
	root := privatePublicationRoot(t)
	f, p, b := ownerReadFixture(t, "textDocument/references")
	ref, err := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
	if err != nil {
		t.Fatal(err)
	}
	original, err := publication.ReadVerifiedBoundFile(root, ref.selector, ownerReadLimit)
	if err != nil {
		t.Fatal(err)
	}
	var record ownerReadRecord
	if json.Unmarshal(original, &record) != nil {
		t.Fatal("record")
	}
	for name, reader := range map[string]func(*publication.Root, string, int64) ([]byte, error){
		"deleted-payload-body": func(r *publication.Root, s string, n int64) ([]byte, error) {
			if s == record.ReadSelector {
				return nil, errRawResultPayload
			}
			return publication.ReadVerifiedBoundFile(r, s, n)
		},
		"modified-full-response": func(r *publication.Root, s string, n int64) ([]byte, error) {
			v, e := publication.ReadVerifiedBoundFile(r, s, n)
			if s == record.ReadSelector && e == nil {
				v = append([]byte(nil), v...)
				v[0] = '!'
			}
			return v, e
		},
		"changed-offset": func(r *publication.Root, s string, n int64) ([]byte, error) {
			v, e := publication.ReadVerifiedBoundFile(r, s, n)
			if s == ref.selector && e == nil {
				v = bytes.Replace(v, []byte(`"result_offset":`+fmt.Sprint(record.ResultOffset)), []byte(`"result_offset":0`), 1)
			}
			return v, e
		},
	} {
		t.Run(name, func(t *testing.T) {
			out, e := publishRawResultPayload(root, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, reader)
			if e == nil || out.stage != "ABSENT" {
				t.Fatalf("ASSERT_RAW_RESULT_REJECT_%s: %+v %v", name, out, e)
			}
		})
	}
	oversized := bytes.Repeat([]byte{' '}, rawResultPayloadLimit+1)
	slice := privateBodySlice{body: oversized, value: oversized, offset: 0, length: len(oversized), bodyDigest: privateDigest(oversized), valueDigest: privateDigest(oversized)}
	if replayPrivateBodySlice(oversized, slice, "result", p.Method, p.Key.ID, oversized) {
		t.Fatal("ASSERT_RAW_RESULT_OVER_LIMIT")
	}
}

func strconvI(n int) string { return fmt.Sprint(n) }
