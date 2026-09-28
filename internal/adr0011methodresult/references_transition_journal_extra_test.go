package adr0011methodresult

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

func TestDurableReferenceTransitionCollisionAndMissingPrefix(t *testing.T) {
	o := testTransitionOwner()
	first := journalBytes(o, referenceTransitionEvent{"QUERY_BEGIN", -1})
	t.Run("collision", func(t *testing.T) {
		root, dir := journalRoot(t)
		selector := referenceTransitionJournalSelector(first)
		if e := os.WriteFile(filepath.Join(dir, selector), []byte("collision"), 0600); e != nil {
			t.Fatal(e)
		}
		s, _ := newDurableReferenceTransitionStore(root, o)
		if s.Store(o, first) == nil {
			t.Fatal("collision accepted")
		}
		if _, e := s.Read(o); e == nil {
			t.Fatal("collision not poisoned")
		}
		raw, e := os.ReadFile(filepath.Join(dir, selector))
		if e != nil || !bytes.Equal(raw, []byte("collision")) {
			t.Fatal("collision replaced")
		}
	})
	t.Run("postcommit readback", func(t *testing.T) {
		root, dir := journalRoot(t)
		s, _ := newDurableReferenceTransitionStore(root, o)
		s.trace = func(event publication.BoundFileTraceEvent) {
			if event.Stage == "RECEIPT" {
				_ = os.Remove(filepath.Join(dir, referenceTransitionJournalSelector(first)))
			}
		}
		if s.Store(o, first) == nil {
			t.Fatal("postcommit loss accepted")
		}
		if _, e := s.Read(o); e == nil {
			t.Fatal("postcommit loss did not poison")
		}
	})
	t.Run("missing earlier prefix", func(t *testing.T) {
		root, dir := journalRoot(t)
		s, _ := newDurableReferenceTransitionStore(root, o)
		sink, _ := newReferenceTransitionSink(o, s)
		for _, v := range []referenceTransitionEvent{{"QUERY_BEGIN", -1}, {"QUERY_TERMINAL_EMPTY", -1}} {
			if e := sink.transition(o, v.Kind, v.Ordinal); e != nil {
				t.Fatal(e)
			}
		}
		last, e := s.Read(o)
		if e != nil {
			t.Fatal(e)
		}
		if e := os.Remove(filepath.Join(dir, referenceTransitionJournalSelector(first))); e != nil {
			t.Fatal(e)
		}
		if ReplayReferenceTransitionJournal(root, last, o) == nil {
			t.Fatal("missing prefix accepted")
		}
	})
	t.Run("read mismatch", func(t *testing.T) {
		root, dir := journalRoot(t)
		s, _ := newDurableReferenceTransitionStore(root, o)
		if e := s.Store(o, first); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, s.selector), []byte("bad"), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := s.Read(o); e == nil {
			t.Fatal("mismatched durable read accepted")
		}
		if s.Store(o, journalBytes(o, referenceTransitionEvent{"QUERY_BEGIN", -1}, referenceTransitionEvent{"QUERY_TERMINAL_EMPTY", -1})) == nil {
			t.Fatal("read failure not poisoned")
		}
	})
}
