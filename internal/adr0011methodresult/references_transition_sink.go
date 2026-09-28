package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// This private sink is not attached to the evaluator. Its receipts are not issuance.
var errReferenceTransition = errors.New("reference transition evidence unavailable")

const (
	referenceTransitionMaxEvents = 2002
	referenceTransitionMaxBytes  = 1 << 20
	referenceTransitionMaxWork   = 4096
)

type referenceTransitionOwner struct {
	Transaction, RequestKey, Invocation, ResponseRead, RawSelector, RawDigest string
}
type referenceTransitionEvent struct {
	Kind    string `json:"kind"`
	Ordinal int    `json:"ordinal"`
}
type referenceTransitionRecord struct {
	Owner  referenceTransitionOwner   `json:"owner"`
	Events []referenceTransitionEvent `json:"events"`
}

// Store and Read must be independent operations: Read cannot return the caller's
// in-memory buffer as a substitute for durable storage readback.
type referenceTransitionStore interface {
	Store(owner referenceTransitionOwner, data []byte) error
	Read(owner referenceTransitionOwner) ([]byte, error)
}
type referenceTransitionReceipt struct {
	owner  referenceTransitionOwner
	digest string
	count  int
}
type referenceTransitionSink struct {
	owner                   referenceTransitionOwner
	store                   referenceTransitionStore
	events                  []referenceTransitionEvent
	begun, terminal, failed bool
	next, work              int
}

func newReferenceTransitionSink(owner referenceTransitionOwner, store referenceTransitionStore) (*referenceTransitionSink, error) {
	if owner.Transaction == "" || owner.RequestKey == "" || owner.Invocation == "" || owner.ResponseRead == "" || owner.RawSelector == "" || owner.RawDigest == "" || store == nil {
		return nil, errReferenceTransition
	}
	return &referenceTransitionSink{owner: owner, store: store}, nil
}

// Transition records before the evaluator advances. Any callback error or panic
// poisons this sink permanently; callers must not advance on its error.
func (s *referenceTransitionSink) transition(owner referenceTransitionOwner, kind string, ordinal int) (err error) {
	if s == nil {
		return errReferenceTransition
	}
	if s.failed || s.terminal || s.owner != owner {
		s.failed = true
		return errReferenceTransition
	}
	defer func() {
		if recover() != nil {
			err = errReferenceTransition
		}
		if err != nil {
			s.failed = true
		}
	}()
	switch kind {
	case "QUERY_BEGIN":
		if s.begun || ordinal != -1 {
			return errReferenceTransition
		}
		s.begun = true
	case "ELEMENT_BEGIN":
		if !s.begun || ordinal != s.next || s.next >= 1000 || (len(s.events) > 0 && s.events[len(s.events)-1].Kind == "ELEMENT_BEGIN") {
			return errReferenceTransition
		}
	case "ELEMENT_TERMINAL_VALID_PENDING_ADMISSION":
		if !s.begun || ordinal != s.next || len(s.events) == 0 || s.events[len(s.events)-1] != (referenceTransitionEvent{Kind: "ELEMENT_BEGIN", Ordinal: ordinal}) {
			return errReferenceTransition
		}
		s.next++
	case "QUERY_TERMINAL_ITEMS":
		if !s.begun || s.next == 0 || ordinal != -1 || len(s.events) == 0 || s.events[len(s.events)-1].Kind != "ELEMENT_TERMINAL_VALID_PENDING_ADMISSION" {
			return errReferenceTransition
		}
		s.terminal = true
	case "QUERY_TERMINAL_EMPTY":
		if !s.begun || s.next != 0 || ordinal != -1 || len(s.events) == 0 || s.events[len(s.events)-1].Kind != "QUERY_BEGIN" {
			return errReferenceTransition
		}
		s.terminal = true
	default:
		return errReferenceTransition
	}
	s.work++
	if s.work > referenceTransitionMaxWork || len(s.events) >= referenceTransitionMaxEvents {
		return errReferenceTransition
	}
	candidate := append(append([]referenceTransitionEvent(nil), s.events...), referenceTransitionEvent{kind, ordinal})
	encoded, e := json.Marshal(referenceTransitionRecord{s.owner, candidate})
	if e != nil || len(encoded) > referenceTransitionMaxBytes {
		return errReferenceTransition
	}
	if e = s.store.Store(s.owner, encoded); e != nil {
		return fmt.Errorf("%w: store: %v", errReferenceTransition, e)
	}
	read, e := s.store.Read(s.owner)
	if e != nil || !bytes.Equal(encoded, read) {
		return errReferenceTransition
	}
	s.events = candidate
	return nil
}
func (s *referenceTransitionSink) seal(owner referenceTransitionOwner) (receipt *referenceTransitionReceipt, err error) {
	if s == nil || s.failed || !s.terminal || owner != s.owner {
		return nil, errReferenceTransition
	}
	defer func() {
		if recover() != nil {
			s.failed = true
			receipt = nil
			err = errReferenceTransition
		}
	}()
	encoded, e := json.Marshal(referenceTransitionRecord{s.owner, s.events})
	if e != nil {
		s.failed = true
		return nil, errReferenceTransition
	}
	read, e := s.store.Read(owner)
	if e != nil || !bytes.Equal(read, encoded) {
		s.failed = true
		return nil, errReferenceTransition
	}
	digest := sha256.Sum256(read)
	return &referenceTransitionReceipt{owner: owner, digest: hex.EncodeToString(digest[:]), count: len(s.events)}, nil
}
func verifyReferenceTransitionReceipt(store referenceTransitionStore, receipt *referenceTransitionReceipt) (err error) {
	if store == nil || receipt == nil || receipt.count < 2 || receipt.count > referenceTransitionMaxEvents {
		return errReferenceTransition
	}
	defer func() {
		if recover() != nil {
			err = errReferenceTransition
		}
	}()
	raw, e := store.Read(receipt.owner)
	if e != nil || len(raw) > referenceTransitionMaxBytes {
		return errReferenceTransition
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != receipt.digest {
		return errReferenceTransition
	}
	var record referenceTransitionRecord
	if json.Unmarshal(raw, &record) != nil || record.Owner != receipt.owner || len(record.Events) != receipt.count {
		return errReferenceTransition
	}
	// Replay the event grammar without writing to the store.
	begun, terminal, next := false, false, 0
	for i, v := range record.Events {
		if terminal {
			return errReferenceTransition
		}
		switch v.Kind {
		case "QUERY_BEGIN":
			if begun || v.Ordinal != -1 {
				return errReferenceTransition
			}
			begun = true
		case "ELEMENT_BEGIN":
			if !begun || v.Ordinal != next || next >= 1000 || (i > 0 && record.Events[i-1].Kind == "ELEMENT_BEGIN") {
				return errReferenceTransition
			}
		case "ELEMENT_TERMINAL_VALID_PENDING_ADMISSION":
			if !begun || v.Ordinal != next || i == 0 || record.Events[i-1] != (referenceTransitionEvent{"ELEMENT_BEGIN", next}) {
				return errReferenceTransition
			}
			next++
		case "QUERY_TERMINAL_EMPTY":
			if !begun || next != 0 || v.Ordinal != -1 || i == 0 || record.Events[i-1].Kind != "QUERY_BEGIN" {
				return errReferenceTransition
			}
			terminal = true
		case "QUERY_TERMINAL_ITEMS":
			if !begun || next == 0 || v.Ordinal != -1 || i == 0 || record.Events[i-1].Kind != "ELEMENT_TERMINAL_VALID_PENDING_ADMISSION" {
				return errReferenceTransition
			}
			terminal = true
		default:
			return errReferenceTransition
		}
	}
	if !terminal {
		return errReferenceTransition
	}
	return nil
}
