package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

const selectedSymbol = `[{"name":"chosen","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":3,"character":0}},"selectionRange":{"start":{"line":1,"character":2},"end":{"line":1,"character":8}}}]`

func targetFixture(t *testing.T, token string) (*publication.Root, privateBodyPublication, adr0011querytarget.Query, sessionruntime.OwnedMethodPair, sessionruntime.OwnedDocumentBinding, func(*publication.Root, string, int64) ([]byte, error)) {
	t.Helper()
	root := privatePublicationRoot(t)
	f, p, b := ownerReadFixture(t, "textDocument/documentSymbol")
	b.SHA256 = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("source")))
	p.Source = &b
	body := []byte(`{ "jsonrpc" : "2.0", "id" : 31, "result" : ` + token + ` }`)
	f.response = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
	p.Result = []byte(token)
	p.Read.FrameBytes = int64(len(f.response))
	p.Read.FrameSHA256 = privateDigest(f.response)
	f.pair = p
	var ok bool
	f.resultSpan, ok = exactOwnedSpan(f.response, "result", p.Method, p.Key.ID)
	if !ok {
		t.Fatal("fixture span")
	}
	owner, e := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
	if e != nil {
		t.Fatal(e)
	}
	q := adr0011querytarget.Query{OccurrenceID: "owned-1", URI: b.URI, Encoding: "utf-16", DocumentVersion: "1", SourceDigest: b.SHA256, SessionID: "owned", Generation: 7, Line: 1, Character: 4}
	return root, owner, q, p, b, publication.ReadVerifiedBoundFile
}
func TestTargetResultExactAndReplay(t *testing.T) {
	root, owner, q, p, b, reader := targetFixture(t, selectedSymbol)
	pub, e := publishTargetResult(root, owner, p, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, q, reader)
	if e != nil || pub.stage != "VERIFIED" {
		t.Fatalf("ASSERT_TARGET_RESULT_EXACT: %+v %v", pub, e)
	}
	raw, e := reader(root, pub.selector, ownerReadLimit)
	if e != nil {
		t.Fatal(e)
	}
	var record targetResultRecord
	if json.Unmarshal(raw, &record) != nil {
		t.Fatal("record")
	}
	payload, e := reader(root, record.PayloadSelector, 1048576)
	if e != nil || !bytes.Equal(payload, []byte(selectedSymbol)) || record.PayloadDigest != privateDigest(payload) || record.OwnerReadRef.Selector != owner.selector {
		t.Fatal("ASSERT_TARGET_RESULT_EXACT_BYTES")
	}
	if !replayTargetResult(root, pub, owner, p, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, q, reader) {
		t.Fatal("ASSERT_TARGET_RESULT_REPLAY")
	}
	for name, alter := range map[string]func(*publication.Root, string, int64) ([]byte, error){
		"missing-result": func(r *publication.Root, s string, n int64) ([]byte, error) {
			if s == record.PayloadSelector {
				return nil, errors.New("missing")
			}
			return reader(r, s, n)
		},
		"replaced-result": func(r *publication.Root, s string, n int64) ([]byte, error) {
			v, e := reader(r, s, n)
			if s == record.PayloadSelector && e == nil {
				v = append([]byte(nil), v...)
				v[0] = '!'
			}
			return v, e
		},
		"cross-method": func(r *publication.Root, s string, n int64) ([]byte, error) {
			if s == record.PayloadSelector {
				return []byte(`null`), nil
			}
			return reader(r, s, n)
		},
		"changed-offset": func(r *publication.Root, s string, n int64) ([]byte, error) {
			v, e := reader(r, s, n)
			if s == owner.selector && e == nil {
				var o ownerReadRecord
				_ = json.Unmarshal(v, &o)
				v = bytes.Replace(v, []byte(fmt.Sprintf(`"result_offset":%d`, o.ResultOffset)), []byte(`"result_offset":0`), 1)
			}
			return v, e
		},
	} {
		t.Run(name, func(t *testing.T) {
			if replayTargetResult(root, pub, owner, p, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, q, alter) {
				t.Fatalf("ASSERT_TARGET_RESULT_REJECT_%s", name)
			}
		})
	}
	wrongMethod := p
	wrongMethod.Method = "textDocument/references"
	if replayTargetResult(root, pub, owner, wrongMethod, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, q, reader) {
		t.Fatal("ASSERT_TARGET_RESULT_CROSS_METHOD")
	}
	wrong := q
	wrong.Character = 20
	if replayTargetResult(root, pub, owner, p, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, wrong, reader) {
		t.Fatal("ASSERT_TARGET_RESULT_QUERY_MISMATCH")
	}
	wrong = q
	wrong.SourceDigest = "sha256:wrong"
	if replayTargetResult(root, pub, owner, p, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, wrong, reader) {
		t.Fatal("ASSERT_TARGET_RESULT_SOURCE_MISMATCH")
	}
	other := p
	other.Result = []byte(`null`)
	if out, e := publishTargetResult(root, owner, other, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, q, reader); e == nil || out.stage != "ABSENT" {
		t.Fatalf("ASSERT_TARGET_RESULT_PRECOMMIT_SUBSTITUTION: %+v %v", out, e)
	}
	if second, e := publishTargetResult(root, owner, p, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, q, reader); e == nil || second.stage != "ABSENT" {
		t.Fatalf("ASSERT_TARGET_RESULT_NO_REPLACE: %+v %v", second, e)
	}
}
func TestTargetResultRejectsInvalidSelectionAndUnverified(t *testing.T) {
	for _, token := range []string{"null", "[]", `[` + selectedSymbol[1:len(selectedSymbol)-1] + `,` + selectedSymbol[1:len(selectedSymbol)-1] + `]`} {
		t.Run(token, func(t *testing.T) {
			root, owner, q, p, b, reader := targetFixture(t, token)
			if out, e := publishTargetResult(root, owner, p, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, q, reader); e == nil || out.stage != "ABSENT" {
				t.Fatalf("ASSERT_TARGET_RESULT_SELECTION_FAIL: %+v %v", out, e)
			}
		})
	}
	root, owner, q, p, b, reader := targetFixture(t, selectedSymbol)
	failPayload := func(r *publication.Root, s string, n int64) ([]byte, error) {
		if strings.Contains(s, "-target-result-v1-") {
			return nil, errTargetResult
		}
		return reader(r, s, n)
	}
	if out, e := publishTargetResult(root, owner, p, b, "owned", 7, "owner-invocation", reviewedSuccessorSchemaDigest, q, failPayload); e == nil || out.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_TARGET_RESULT_COMMITTED_UNVERIFIED: %+v %v", out, e)
	}
	root2, owner2, q2, p2, b2, reader2 := targetFixture(t, selectedSymbol)
	if out, e := publishTargetResult(root2, owner2, p2, b2, "owned", 7, "wrong", reviewedSuccessorSchemaDigest, q2, reader2); e == nil || out.stage != "ABSENT" {
		t.Fatalf("ASSERT_TARGET_RESULT_PRECOMMIT: %+v %v", out, e)
	}
}
