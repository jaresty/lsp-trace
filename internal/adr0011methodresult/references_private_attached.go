package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"

	"lsp-trace/internal/adr0011c18"
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

// RunPrivateAttachedReferencesContext observes only the coarse method-result
// boundary and otherwise uses the same implementation as the legacy entry point.
func RunPrivateAttachedReferencesContext(ctx context.Context, root *publication.Root, key lspwire.RequestKey, raw json.RawMessage, identity PrivateTransitionIdentity) (*ReferenceEvaluation, []byte, error) {
	adr0011c18.Notify(ctx, adr0011c18.PointMethodResultEntered)
	defer adr0011c18.Notify(ctx, adr0011c18.PointMethodResultReturned)
	_, receipt := adr0011c18.BeginSuffix(ctx)
	return runPrivateAttachedReferencesC18(root, key, raw, identity, nil, receipt)
}

type c18ReferenceTransitionStore struct {
	referenceTransitionStore
	receipt   *adr0011c18.Receipt
	retention bool
}

// Store gates the first durable write. The sink's immediate equality Read is
// part of this atomic retention operation; PointReadback separately gates the
// later independent final read and every replay read that follows it.
func (s *c18ReferenceTransitionStore) Store(owner referenceTransitionOwner, data []byte) error {
	if !s.retention {
		if !s.receipt.ReachSuffix(adr0011c18.PointRetention) {
			return errReferenceTransition
		}
		s.retention = true
	}
	return s.referenceTransitionStore.Store(owner, data)
}

// trace is package-private fault injection; production always supplies nil.
func runPrivateAttachedReferences(root *publication.Root, key lspwire.RequestKey, raw json.RawMessage, identity PrivateTransitionIdentity, trace publication.BoundFileTrace) (*ReferenceEvaluation, []byte, error) {
	return runPrivateAttachedReferencesC18(root, key, raw, identity, trace, nil)
}

func runPrivateAttachedReferencesC18(root *publication.Root, key lspwire.RequestKey, raw json.RawMessage, identity PrivateTransitionIdentity, trace publication.BoundFileTrace, receipt *adr0011c18.Receipt) (*ReferenceEvaluation, []byte, error) {
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
	var transitionStore referenceTransitionStore = store
	if receipt != nil {
		transitionStore = &c18ReferenceTransitionStore{referenceTransitionStore: store, receipt: receipt}
	}
	sink, err := newReferenceTransitionSink(owner, transitionStore)
	if err != nil {
		return nil, nil, err
	}
	if receipt != nil && !receipt.ReachSuffix(adr0011c18.PointObjectEvent) {
		return nil, nil, errReferenceTransition
	}
	evaluation, _, err := evaluateCompleteReferencesAttached(key, raw, owner, sink)
	if err != nil {
		return nil, nil, err
	}
	if receipt != nil && !receipt.ReachSuffix(adr0011c18.PointReadback) {
		return nil, nil, errReferenceTransition
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
