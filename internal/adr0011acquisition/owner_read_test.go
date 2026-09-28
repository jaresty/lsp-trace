package adr0011acquisition

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func ownerReadFixture(t *testing.T, method string) (ownedFrames, sessionruntime.OwnedMethodPair, sessionruntime.OwnedDocumentBinding) {
	t.Helper()
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	binding := sessionruntime.OwnedDocumentBinding{URI: "file:///owned.go", Version: 1, SHA256: "sha256:source"}
	params := `{"textDocument":{"uri":"file:///owned.go"}}`
	if method == "textDocument/references" {
		params = `{"textDocument":{"uri":"file:///owned.go"},"position":{"line":1,"character":2},"context":{"includeDeclaration":false}}`
	}
	result := `[]`
	if method == "textDocument/references" {
		result = `null`
	}
	request := fmt.Sprintf(`{"jsonrpc":"2.0","id":31,"method":%q,"params":%s}`, method, params)
	response := fmt.Sprintf(`{"jsonrpc":"2.0","id":31,"result":%s}`, result)
	frame := func(s string) []byte { return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(s), s)) }
	f := ownedFrames{request: frame(request), response: frame(response), invocation: "owner-invocation"}
	p := sessionruntime.OwnedMethodPair{SessionID: "owned", Generation: 7, Key: key, Method: method, Source: &binding, Params: []byte(params), Result: []byte(result), Write: sessionruntime.RequestWriteObservation{SessionID: "owned", Generation: 7, Key: key, Method: method}, Read: sessionruntime.ResponseReadObservation{SessionID: "owned", Generation: 7, Key: key}}
	p.Write.FrameBytes = int64(len(f.request))
	p.Read.FrameBytes = int64(len(f.response))
	p.Write.FrameSHA256 = privateDigest(f.request)
	p.Read.FrameSHA256 = privateDigest(f.response)
	f.pair = p
	var ok bool
	f.paramsSpan, ok = exactOwnedSpan(f.request, "params", method, key.ID)
	if !ok {
		t.Fatal("fixture params")
	}
	f.resultSpan, ok = exactOwnedSpan(f.response, "result", method, key.ID)
	if !ok {
		t.Fatal("fixture result")
	}
	return f, p, binding
}

func TestOwnerReadSynthetic(t *testing.T) {
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/references"} {
		t.Run(method, func(t *testing.T) {
			root := privatePublicationRoot(t)
			f, p, b := ownerReadFixture(t, method)
			ref, err := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
			if err != nil || ref.stage != "VERIFIED" {
				t.Fatalf("ASSERT_OWNER_READ_VERIFIED: %+v %v", ref, err)
			}
			raw, err := publication.ReadVerifiedBoundFile(root, ref.selector, ownerReadLimit)
			if err != nil || !replayOwnerRead(root, raw, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
				t.Fatalf("ASSERT_OWNER_READ_REPLAY: %v", err)
			}
			for name, mutation := range map[string]func() bool{
				"wrong-key": func() bool {
					q := p
					q.Key.ID++
					return replayOwnerRead(root, raw, ref, q, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
				},
				"wrong-invocation": func() bool {
					return replayOwnerRead(root, raw, ref, p, b, "owned", 7, "other", reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
				},
				"missing-owner-read": func() bool {
					return replayOwnerRead(root, raw, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, func(r *publication.Root, s string, n int64) ([]byte, error) {
						if strings.Contains(s, "-owner-read-") {
							return nil, errOwnerRead
						}
						return publication.ReadVerifiedBoundFile(r, s, n)
					})
				},
				"missing-write": func() bool {
					return replayOwnerRead(root, raw, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, func(r *publication.Root, s string, n int64) ([]byte, error) {
						if strings.Contains(s, "-write-") {
							return nil, errOwnerRead
						}
						return publication.ReadVerifiedBoundFile(r, s, n)
					})
				},
				"substituted-read": func() bool {
					return replayOwnerRead(root, raw, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, func(r *publication.Root, s string, n int64) ([]byte, error) {
						v, e := publication.ReadVerifiedBoundFile(r, s, n)
						if strings.Contains(s, "-read-") {
							v = append([]byte(nil), v...)
							v[len(v)-1] = '!'
						}
						return v, e
					})
				},
				"off-by-one": func() bool {
					changed := bytes.Replace(raw, []byte(`"result_offset":`+fmt.Sprint(len(`{"jsonrpc":"2.0","id":31,"result":`))), []byte(`"result_offset":0`), 1)
					return replayOwnerRead(root, changed, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
				},
				"unknown": func() bool {
					changed := bytes.Replace(raw, []byte(`"wire_id":31`), []byte(`"wire_id":31,"unknown":1`), 1)
					return replayOwnerRead(root, changed, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
				},
				"duplicate": func() bool {
					changed := bytes.Replace(raw, []byte(`"wire_id":31`), []byte(`"wire_id":31,"wire_id":31`), 1)
					return replayOwnerRead(root, changed, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
				},
				"trailing": func() bool {
					return replayOwnerRead(root, append(append([]byte(nil), raw...), []byte(" {}")...), ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile)
				},
			} {
				t.Run(name, func(t *testing.T) {
					if mutation() {
						t.Fatalf("ASSERT_OWNER_READ_REJECT_%s", name)
					}
				})
			}
			if ref2, err := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil); err == nil || ref2.stage != "ABSENT" {
				t.Fatalf("ASSERT_OWNER_READ_NO_REPLACE: %+v %v", ref2, err)
			}
		})
	}
}

func TestOwnerReadPrecommitAndUnverified(t *testing.T) {
	f, p, b := ownerReadFixture(t, "textDocument/documentSymbol")
	root := privatePublicationRoot(t)
	if ref, err := publishOwnerRead(root, f, p, b, "owned", 7, "wrong", reviewedSuccessorSchemaDigest, nil); err == nil || ref.stage != "ABSENT" {
		t.Fatalf("ASSERT_OWNER_READ_PRECOMMIT: %+v %v", ref, err)
	}
	root = privatePublicationRoot(t)
	altered := func(r *publication.Root, s string, n int64) ([]byte, error) {
		v, e := publication.ReadVerifiedBoundFile(r, s, n)
		if strings.Contains(s, "-owner-read-") {
			v = append([]byte(nil), v...)
			v[0] = '!'
		}
		return v, e
	}
	if ref, err := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, altered); err == nil || ref.stage != "COMMITTED_UNVERIFIED" {
		t.Fatalf("ASSERT_OWNER_READ_UNVERIFIED: %+v %v", ref, err)
	}
}
