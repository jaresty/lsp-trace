package adr0011methodresult

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
)

type transitionMemoryStore struct {
	data                   []byte
	fail, crash, uncertain bool
}

func (m *transitionMemoryStore) Store(_ referenceTransitionOwner, b []byte) error {
	if m.crash {
		panic("callback")
	}
	if m.fail {
		return errors.New("failed")
	}
	m.data = append([]byte(nil), b...)
	return nil
}
func (m *transitionMemoryStore) Read(_ referenceTransitionOwner) ([]byte, error) {
	if m.uncertain {
		return nil, errors.New("uncertain")
	}
	return append([]byte(nil), m.data...), nil
}
func testTransitionOwner() referenceTransitionOwner {
	return referenceTransitionOwner{"tx", "request", "invocation", "read", "raw", "sha256:raw"}
}
func TestReferenceTransitionStandaloneReceipt(t *testing.T) {
	owner := testTransitionOwner()
	store := &transitionMemoryStore{}
	s, e := newReferenceTransitionSink(owner, store)
	if e != nil {
		t.Fatal(e)
	}
	for _, event := range []referenceTransitionEvent{{"QUERY_BEGIN", -1}, {"ELEMENT_BEGIN", 0}, {"ELEMENT_TERMINAL_VALID_PENDING_ADMISSION", 0}, {"QUERY_TERMINAL_ITEMS", -1}} {
		if e = s.transition(owner, event.Kind, event.Ordinal); e != nil {
			t.Fatal(e)
		}
	}
	receipt, e := s.seal(owner)
	if e != nil || verifyReferenceTransitionReceipt(store, receipt) != nil {
		t.Fatalf("ASSERT_TRANSITION_VERIFIED_READBACK: %v", e)
	}
	store.data[0] ^= 1
	if verifyReferenceTransitionReceipt(store, receipt) == nil {
		t.Fatal("ASSERT_TRANSITION_TAMPER_DENIED")
	}
}
func TestReferenceTransitionRejectsIncompleteAndWrongOwner(t *testing.T) {
	owner := testTransitionOwner()
	s, _ := newReferenceTransitionSink(owner, &transitionMemoryStore{})
	if r, e := s.seal(owner); r != nil || e == nil {
		t.Fatal("ASSERT_TRANSITION_OMISSION_DENIED")
	}
	other := owner
	other.Invocation = "other"
	if s.transition(other, "QUERY_BEGIN", -1) == nil {
		t.Fatal("ASSERT_TRANSITION_CROSS_INVOCATION_DENIED")
	}
	if s.transition(owner, "QUERY_BEGIN", -1) == nil {
		t.Fatal("ASSERT_TRANSITION_POISONED")
	}
	for _, event := range []referenceTransitionEvent{{"ELEMENT_BEGIN", 0}, {"QUERY_BEGIN", -1}, {"ELEMENT_TERMINAL_VALID_PENDING_ADMISSION", 0}} {
		s, _ = newReferenceTransitionSink(owner, &transitionMemoryStore{})
		if event.Kind != "ELEMENT_BEGIN" {
			_ = s.transition(owner, "QUERY_BEGIN", -1)
		}
		if s.transition(owner, event.Kind, event.Ordinal) == nil {
			t.Fatalf("ASSERT_TRANSITION_REORDER_DUPLICATE_DENIED: %s", event.Kind)
		}
	}
}
func TestReferenceTransitionCallbackFailures(t *testing.T) {
	for _, mode := range []string{"fail", "crash", "uncertain"} {
		t.Run(mode, func(t *testing.T) {
			store := &transitionMemoryStore{fail: mode == "fail", crash: mode == "crash", uncertain: mode == "uncertain"}
			owner := testTransitionOwner()
			s, _ := newReferenceTransitionSink(owner, store)
			if s.transition(owner, "QUERY_BEGIN", -1) == nil {
				t.Fatal("ASSERT_TRANSITION_CALLBACK_FAILURE_DENIED")
			}
			if s.transition(owner, "QUERY_BEGIN", -1) == nil {
				t.Fatal("ASSERT_TRANSITION_CALLBACK_POISONED")
			}
			if r, e := s.seal(owner); r != nil || e == nil {
				t.Fatal("ASSERT_TRANSITION_NO_RECEIPT")
			}
		})
	}
}
func TestReferenceTransitionEmptyRejectsDanglingElement(t *testing.T) {
	owner := testTransitionOwner()
	store := &transitionMemoryStore{}
	s, err := newReferenceTransitionSink(owner, store)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.transition(owner, "QUERY_BEGIN", -1); err != nil {
		t.Fatal(err)
	}
	if err := s.transition(owner, "ELEMENT_BEGIN", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.transition(owner, "QUERY_TERMINAL_EMPTY", -1); err == nil {
		t.Fatal("ASSERT_TRANSITION_EMPTY_DANGLING_BEGIN_DENIED")
	}
	if receipt, err := s.seal(owner); receipt != nil || err == nil {
		t.Fatal("ASSERT_TRANSITION_DANGLING_NO_RECEIPT")
	}
}

func TestReferenceTransitionReplayEmptyRejectsDanglingElement(t *testing.T) {
	owner := testTransitionOwner()
	store := &transitionMemoryStore{}
	forged, err := json.Marshal(referenceTransitionRecord{Owner: owner, Events: []referenceTransitionEvent{
		{Kind: "QUERY_BEGIN", Ordinal: -1},
		{Kind: "ELEMENT_BEGIN", Ordinal: 0},
		{Kind: "QUERY_TERMINAL_EMPTY", Ordinal: -1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	store.data = forged
	digest := sha256.Sum256(forged)
	if err := verifyReferenceTransitionReceipt(store, &referenceTransitionReceipt{owner: owner, digest: hex.EncodeToString(digest[:]), count: 3}); err == nil {
		t.Fatal("ASSERT_TRANSITION_REPLAY_EMPTY_DANGLING_BEGIN_DENIED")
	}
}

func TestReferenceTransitionEmptyAndReadbackUncertainty(t *testing.T) {
	owner := testTransitionOwner()
	store := &transitionMemoryStore{}
	s, _ := newReferenceTransitionSink(owner, store)
	if e := s.transition(owner, "QUERY_BEGIN", -1); e != nil {
		t.Fatal(e)
	}
	if e := s.transition(owner, "QUERY_TERMINAL_EMPTY", -1); e != nil {
		t.Fatal(e)
	}
	r, e := s.seal(owner)
	if e != nil {
		t.Fatal(e)
	}
	store.uncertain = true
	if verifyReferenceTransitionReceipt(store, r) == nil {
		t.Fatal("ASSERT_TRANSITION_READBACK_UNCERTAINTY_DENIED")
	}
}
