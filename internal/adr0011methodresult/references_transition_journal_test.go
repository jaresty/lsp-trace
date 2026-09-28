package adr0011methodresult

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

func journalRoot(t *testing.T) (*publication.Root, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	return root, dir
}
func journalBytes(o referenceTransitionOwner, events ...referenceTransitionEvent) []byte {
	b, _ := json.Marshal(referenceTransitionRecord{o, events})
	return b
}
func TestDurableReferenceTransitionEmptyAndEqualLocations(t *testing.T) {
	for _, k := range []int{0, 2} {
		t.Run(string(rune('0'+k)), func(t *testing.T) {
			root, _ := journalRoot(t)
			o := testTransitionOwner()
			s, e := newDurableReferenceTransitionStore(root, o)
			if e != nil {
				t.Fatal(e)
			}
			sink, e := newReferenceTransitionSink(o, s)
			if e != nil {
				t.Fatal(e)
			}
			events := []referenceTransitionEvent{{"QUERY_BEGIN", -1}}
			for i := 0; i < k; i++ {
				events = append(events, referenceTransitionEvent{"ELEMENT_BEGIN", i}, referenceTransitionEvent{"ELEMENT_TERMINAL_VALID_PENDING_ADMISSION", i})
			}
			terminal := "QUERY_TERMINAL_ITEMS"
			if k == 0 {
				terminal = "QUERY_TERMINAL_EMPTY"
			}
			events = append(events, referenceTransitionEvent{terminal, -1})
			for _, v := range events {
				if e = sink.transition(o, v.Kind, v.Ordinal); e != nil {
					t.Fatal(e)
				}
			}
			r, e := sink.seal(o)
			if e != nil || verifyReferenceTransitionReceipt(s, r) != nil {
				t.Fatalf("receipt: %v", e)
			}
			raw, e := s.Read(o)
			if e != nil || ReplayReferenceTransitionJournal(root, raw, o) != nil {
				t.Fatalf("replay: %v", e)
			}
			raw[0] ^= 1
			again, e := s.Read(o)
			if e != nil || bytes.Equal(raw, again) {
				t.Fatal("read returned cached buffer")
			}
			if ReplayReferenceTransitionJournal(root, again, referenceTransitionOwner{Transaction: "wrong"}) == nil {
				t.Fatal("cross owner replay")
			}
		})
	}
}
func TestDurableReferenceTransitionRejectsMutationDuplicateAndPoison(t *testing.T) {
	root, _ := journalRoot(t)
	o := testTransitionOwner()
	s, _ := newDurableReferenceTransitionStore(root, o)
	first := journalBytes(o, referenceTransitionEvent{"QUERY_BEGIN", -1})
	if e := s.Store(o, first); e != nil {
		t.Fatal(e)
	}
	if s.Store(o, first) == nil {
		t.Fatal("duplicate accepted")
	}
	if _, e := s.Read(o); e == nil {
		t.Fatal("failed store did not poison")
	}
	for _, tc := range []struct {
		name   string
		events []referenceTransitionEvent
		owner  referenceTransitionOwner
	}{
		{"mutation", []referenceTransitionEvent{{"QUERY_BEGIN", -1}, {"ELEMENT_BEGIN", 0}, {"ELEMENT_TERMINAL_VALID_PENDING_ADMISSION", 1}}, o},
		{"skip", []referenceTransitionEvent{{"QUERY_BEGIN", -1}, {"ELEMENT_BEGIN", 0}, {"ELEMENT_TERMINAL_VALID_PENDING_ADMISSION", 0}, {"QUERY_TERMINAL_ITEMS", -1}}, o},
		{"owner", []referenceTransitionEvent{{"QUERY_BEGIN", -1}, {"ELEMENT_BEGIN", 0}}, referenceTransitionOwner{Transaction: "wrong"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := journalRoot(t)
			j, _ := newDurableReferenceTransitionStore(r, o)
			if e := j.Store(o, first); e != nil {
				t.Fatal(e)
			}
			if j.Store(tc.owner, journalBytes(tc.owner, tc.events...)) == nil {
				t.Fatal("invalid prefix accepted")
			}
			if _, e := j.Read(o); e == nil {
				t.Fatal("not poisoned")
			}
		})
	}
}
