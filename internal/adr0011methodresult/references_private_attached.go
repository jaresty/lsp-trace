package adr0011methodresult

import (
	"bytes"
	"encoding/json"

	transport "lsp-trace/internal/adr0011methodtransport"

	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/publication"
)

// PrivateTransitionIdentity binds one exact raw response to its private owner.
// This bridge does not issue successor events or expose a CLI selector.
type PrivateTransitionIdentity struct {
	Transaction, RequestKey, Invocation, ResponseRead, RawSelector, RawDigest string
}

// RunPrivateAttachedReferences evaluates one complete raw result against fresh,
// immutable journal prefixes. Neither output is available on any failure.
func RunPrivateAttachedReferences(root *publication.Root, key lspwire.RequestKey, raw json.RawMessage, identity PrivateTransitionIdentity) (*ReferenceEvaluation, []byte, error) {
	return runPrivateAttachedReferences(root, key, raw, identity, nil)
}

// trace is package-private fault injection; production always supplies nil.
func runPrivateAttachedReferences(root *publication.Root, key lspwire.RequestKey, raw json.RawMessage, identity PrivateTransitionIdentity, trace publication.BoundFileTrace) (*ReferenceEvaluation, []byte, error) {
	owner := referenceTransitionOwner{
		Transaction: identity.Transaction, RequestKey: identity.RequestKey,
		Invocation: identity.Invocation, ResponseRead: identity.ResponseRead,
		RawSelector: identity.RawSelector, RawDigest: identity.RawDigest,
	}
	if !validReferenceTransitionOwner(owner) || owner.RequestKey != adr0011requestkey.Encode(key) || owner.RawDigest != chainDigest(raw) {
		return nil, nil, errReferenceTransition
	}
	store, err := newDurableReferenceTransitionStore(root, owner)
	if err != nil {
		return nil, nil, err
	}
	store.trace = trace
	sink, err := newReferenceTransitionSink(owner, store)
	if err != nil {
		return nil, nil, err
	}
	evaluation, _, err := evaluateCompleteReferencesAttached(key, raw, owner, sink)
	if err != nil {
		return nil, nil, err
	}
	final, err := store.Read(owner)
	if err != nil {
		return nil, nil, err
	}
	if err := ReplayReferenceTransitionJournal(root, final, owner); err != nil {
		return nil, nil, err
	}
	return evaluation, append([]byte(nil), final...), nil
}

// ReplayPrivateAttachedJournal is a pure read-only replay of the independent
// raw result and every persisted transition prefix. It never reconstructs a
// journal from returned parser items or publishes one.
func ReplayPrivateAttachedJournal(root *publication.Root, raw []byte, key lspwire.RequestKey, identity PrivateTransitionIdentity, finalBytes []byte) error {
	owner := referenceTransitionOwner{identity.Transaction, identity.RequestKey, identity.Invocation, identity.ResponseRead, identity.RawSelector, identity.RawDigest}
	if !validReferenceTransitionOwner(owner) || owner.RequestKey != adr0011requestkey.Encode(key) || owner.RawDigest != chainDigest(raw) {
		return errReferenceTransition
	}
	if err := ReplayReferenceTransitionJournal(root, finalBytes, owner); err != nil {
		return err
	}
	// The result grammar accepts only null or an array of Location members.
	// parseRawUntrusted checks each nested member for duplicate keys and bounds
	// the count; unlike a reconstructed evaluation, this inspects supplied bytes.
	parsed, failure := parseRawUntrusted(transport.MethodReferences, raw, 1000)
	if failure != nil {
		return errReferenceTransition
	}
	trimmed := bytes.TrimSpace(raw)
	if !bytes.Equal(trimmed, []byte("null")) && (len(trimmed) == 0 || trimmed[0] != '[') {
		return errReferenceTransition
	}
	events, err := decodeReferenceTransitionPrefix(finalBytes, owner)
	if err != nil || len(events) != 2+2*len(parsed.Items) || events[0] != (referenceTransitionEvent{"QUERY_BEGIN", -1}) {
		return errReferenceTransition
	}
	for ordinal, item := range parsed.Items {
		if item.Ordinal != ordinal || item.Kind != Location || events[1+2*ordinal] != (referenceTransitionEvent{"ELEMENT_BEGIN", ordinal}) || events[2+2*ordinal] != (referenceTransitionEvent{"ELEMENT_TERMINAL_VALID_PENDING_ADMISSION", ordinal}) {
			return errReferenceTransition
		}
	}
	terminal := "QUERY_TERMINAL_ITEMS"
	if len(parsed.Items) == 0 {
		terminal = "QUERY_TERMINAL_EMPTY"
	}
	if events[len(events)-1] != (referenceTransitionEvent{terminal, -1}) {
		return errReferenceTransition
	}
	return nil
}
