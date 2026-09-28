package adr0011methodresult

import (
	"encoding/json"
	"errors"
	"lsp-trace/internal/lspwire"
	"testing"
)

const attachedLocation = `{"uri":"file:///one","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`

type attachedStore struct {
	transitionMemoryStore
	rejectAt  int
	writes    int
	wrongRead bool
}

func (s *attachedStore) Store(owner referenceTransitionOwner, b []byte) error {
	s.writes++
	if s.writes == s.rejectAt {
		return errors.New("rejected")
	}
	return s.transitionMemoryStore.Store(owner, b)
}
func (s *attachedStore) Read(owner referenceTransitionOwner) ([]byte, error) {
	if s.wrongRead {
		return []byte("wrong"), nil
	}
	return s.transitionMemoryStore.Read(owner)
}
func TestAttachedCompleteReferences(t *testing.T) {
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	for _, tc := range []struct {
		name, raw                    string
		reject                       int
		panicStore, wrongRead, cross bool
		wantSuccess                  bool
	}{
		{name: "two", raw: "[" + attachedLocation + "," + attachedLocation + "]", wantSuccess: true},
		{name: "empty", raw: "[]", wantSuccess: true},
		{name: "null", raw: "null", wantSuccess: true},
		{name: "begin-fails", raw: "[" + attachedLocation + "]", reject: 1},
		{name: "element-fails", raw: "[" + attachedLocation + "]", reject: 2},
		{name: "terminal-fails", raw: "[" + attachedLocation + "]", reject: 3},
		{name: "query-terminal-fails", raw: "[" + attachedLocation + "]", reject: 4},
		{name: "panic", raw: "[" + attachedLocation + "]", panicStore: true},
		{name: "readback", raw: "[" + attachedLocation + "]", wrongRead: true},
		{name: "cross-owner", raw: "[" + attachedLocation + "]", cross: true},
		{name: "malformed", raw: "[" + attachedLocation + `,{"uri":9}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner := testTransitionOwner()
			owner.RawDigest = chainDigest([]byte(tc.raw))
			owner.RequestKey = "lsp-trace.request-key.v1:g=7;id=31"
			store := &attachedStore{rejectAt: tc.reject, wrongRead: tc.wrongRead}
			store.crash = tc.panicStore
			sink, e := newReferenceTransitionSink(owner, store)
			if e != nil {
				t.Fatal(e)
			}
			if tc.cross {
				owner.Invocation = "different"
			}
			got, receipt, e := evaluateCompleteReferencesAttached(key, json.RawMessage(tc.raw), owner, sink)
			if tc.wantSuccess {
				if e != nil || receipt == nil || got == nil || got.T != 0 || got.A != 0 || got.P != len(got.Items) || verifyReferenceTransitionReceipt(store, receipt) != nil {
					t.Fatalf("ASSERT_ATTACHED_SUCCESS: got=%+v receipt=%+v err=%v", got, receipt, e)
				}
			} else if e == nil || got != nil || receipt != nil {
				t.Fatalf("ASSERT_ATTACHED_FAIL_CLOSED: got=%+v receipt=%+v err=%v", got, receipt, e)
			}
		})
	}
}
