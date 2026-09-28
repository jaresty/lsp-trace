package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

const referenceTransitionJournalRole = "references-transition-journal-v1"

// durableReferenceTransitionStore is one-shot per owner. A failed attempt is
// terminal even if the no-replace installation committed before verification.
type durableReferenceTransitionStore struct {
	root     *publication.Root
	owner    referenceTransitionOwner
	selector string
	events   []referenceTransitionEvent
	poisoned bool
	trace    publication.BoundFileTrace
}

func newDurableReferenceTransitionStore(root *publication.Root, owner referenceTransitionOwner) (*durableReferenceTransitionStore, error) {
	if root == nil || !validReferenceTransitionOwner(owner) {
		return nil, errReferenceTransition
	}
	return &durableReferenceTransitionStore{root: root, owner: owner}, nil
}

func validReferenceTransitionOwner(o referenceTransitionOwner) bool {
	return o.Transaction != "" && o.RequestKey != "" && o.Invocation != "" && o.ResponseRead != "" && o.RawSelector != "" && o.RawDigest != ""
}

func referenceTransitionJournalSelector(raw []byte) string {
	h := sha256.New()
	h.Write([]byte(referenceTransitionJournalRole))
	h.Write([]byte{0})
	h.Write(raw)
	return fmt.Sprintf("adr0011-%s-%s.json", referenceTransitionJournalRole, hex.EncodeToString(h.Sum(nil)))
}

func decodeReferenceTransitionPrefix(raw []byte, owner referenceTransitionOwner) ([]referenceTransitionEvent, error) {
	if !validReferenceTransitionOwner(owner) || len(raw) == 0 || len(raw) > referenceTransitionMaxBytes || strictjson.RejectDuplicates(raw) != nil {
		return nil, errReferenceTransition
	}
	var r referenceTransitionRecord
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || r.Owner != owner || len(r.Events) == 0 || len(r.Events) > referenceTransitionMaxEvents {
		return nil, errReferenceTransition
	}
	canonical, e := json.Marshal(r)
	if e != nil || !bytes.Equal(raw, canonical) {
		return nil, errReferenceTransition
	}
	begun, terminal, next := false, false, 0
	for i, v := range r.Events {
		if terminal {
			return nil, errReferenceTransition
		}
		switch v.Kind {
		case "QUERY_BEGIN":
			if begun || v.Ordinal != -1 || i != 0 {
				return nil, errReferenceTransition
			}
			begun = true
		case "ELEMENT_BEGIN":
			if !begun || v.Ordinal != next || next >= 1000 || i == 0 || (i > 0 && r.Events[i-1].Kind == "ELEMENT_BEGIN") {
				return nil, errReferenceTransition
			}
		case "ELEMENT_TERMINAL_VALID_PENDING_ADMISSION":
			if !begun || v.Ordinal != next || i == 0 || r.Events[i-1] != (referenceTransitionEvent{"ELEMENT_BEGIN", next}) {
				return nil, errReferenceTransition
			}
			next++
		case "QUERY_TERMINAL_EMPTY":
			if !begun || next != 0 || v.Ordinal != -1 || i == 0 || r.Events[i-1].Kind != "QUERY_BEGIN" {
				return nil, errReferenceTransition
			}
			terminal = true
		case "QUERY_TERMINAL_ITEMS":
			if !begun || next == 0 || v.Ordinal != -1 || i == 0 || r.Events[i-1].Kind != "ELEMENT_TERMINAL_VALID_PENDING_ADMISSION" {
				return nil, errReferenceTransition
			}
			terminal = true
		default:
			return nil, errReferenceTransition
		}
	}
	return r.Events, nil
}

func (s *durableReferenceTransitionStore) Store(owner referenceTransitionOwner, raw []byte) (err error) {
	if s == nil {
		return errReferenceTransition
	}
	if s.poisoned {
		return errReferenceTransition
	}
	defer func() {
		if recover() != nil {
			err = errReferenceTransition
		}
		if err != nil {
			s.poisoned = true
		}
	}()
	if s.root == nil || owner != s.owner {
		return errReferenceTransition
	}
	events, e := decodeReferenceTransitionPrefix(raw, owner)
	if e != nil || len(events) != len(s.events)+1 || !equalReferenceEvents(events[:len(s.events)], s.events) {
		return errReferenceTransition
	}
	selector := referenceTransitionJournalSelector(raw)
	receipt, e := publication.PublishBoundFileWithTrace(s.root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return errReferenceTransition
		}
		return nil
	}, s.trace)
	digest := sha256.Sum256(raw)
	if e != nil || receipt == nil || receipt.FinalSelector != selector || receipt.Digest != "sha256:"+hex.EncodeToString(digest[:]) || receipt.ByteLength != uint64(len(raw)) || receipt.Mechanism != publication.BoundFileMechanism || !receipt.NamespaceAtomic || receipt.VerificationStatus != "VERIFIED" || receipt.DirectorySyncStatus != publication.DirectorySyncComplete || receipt.CloseStatus != publication.CloseComplete {
		return errReferenceTransition
	}
	got, e := publication.ReadVerifiedBoundFile(s.root, selector, referenceTransitionMaxBytes)
	if e != nil || !bytes.Equal(got, raw) {
		return errReferenceTransition
	}
	s.events = append([]referenceTransitionEvent(nil), events...)
	s.selector = selector
	return nil
}

func equalReferenceEvents(a, b []referenceTransitionEvent) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *durableReferenceTransitionStore) Read(owner referenceTransitionOwner) (raw []byte, err error) {
	if s == nil || s.poisoned || s.root == nil || s.selector == "" || owner != s.owner {
		return nil, errReferenceTransition
	}
	defer func() {
		if recover() != nil {
			raw = nil
			err = errReferenceTransition
		}
		if err != nil {
			s.poisoned = true
		}
	}()
	raw, err = publication.ReadVerifiedBoundFile(s.root, s.selector, referenceTransitionMaxBytes)
	if err != nil || referenceTransitionJournalSelector(raw) != s.selector {
		return nil, errReferenceTransition
	}
	events, e := decodeReferenceTransitionPrefix(raw, owner)
	if e != nil || !equalReferenceEvents(events, s.events) {
		return nil, errReferenceTransition
	}
	return raw, nil
}

// ReplayReferenceTransitionJournal checks every deterministic immutable prefix
// independently; the caller supplies observed final bytes, not a trusted index.
func ReplayReferenceTransitionJournal(root *publication.Root, finalBytes []byte, owner referenceTransitionOwner) error {
	if root == nil {
		return errReferenceTransition
	}
	events, e := decodeReferenceTransitionPrefix(finalBytes, owner)
	if e != nil || len(events) < 2 {
		return errReferenceTransition
	}
	for n := 1; n <= len(events); n++ {
		prefix, e := json.Marshal(referenceTransitionRecord{owner, events[:n]})
		if e != nil || len(prefix) > referenceTransitionMaxBytes {
			return errReferenceTransition
		}
		selector := referenceTransitionJournalSelector(prefix)
		got, e := publication.ReadVerifiedBoundFile(root, selector, referenceTransitionMaxBytes)
		if e != nil || !bytes.Equal(got, prefix) || referenceTransitionJournalSelector(got) != selector {
			return errReferenceTransition
		}
		parsed, e := decodeReferenceTransitionPrefix(got, owner)
		if e != nil || len(parsed) != n || !equalReferenceEvents(parsed, events[:n]) {
			return errReferenceTransition
		}
	}
	digest := sha256.Sum256(finalBytes)
	return verifyReferenceTransitionReceipt(referenceTransitionReplayRead{root: root, selector: referenceTransitionJournalSelector(finalBytes), owner: owner}, &referenceTransitionReceipt{owner: owner, digest: hex.EncodeToString(digest[:]), count: len(events)})
}

type referenceTransitionReplayRead struct {
	root     *publication.Root
	selector string
	owner    referenceTransitionOwner
}

func (r referenceTransitionReplayRead) Store(referenceTransitionOwner, []byte) error {
	return errReferenceTransition
}
func (r referenceTransitionReplayRead) Read(o referenceTransitionOwner) ([]byte, error) {
	if o != r.owner {
		return nil, errReferenceTransition
	}
	return publication.ReadVerifiedBoundFile(r.root, r.selector, referenceTransitionMaxBytes)
}
