package adr0011methodresult

import (
	"bytes"
	"errors"

	"lsp-trace/internal/publication"
)

// CandidateExpectation is supplied independently of retained bytes. It is a
// consistency check, not a producer authentication or D/R admission decision.
type CandidateExpectation struct {
	ID, SessionID, Method, QueryURI, ParamsSHA256 string
	Generation, KeyID                             uint64
	QueryLine, QueryCharacter                     uint32
}

// PublishPrivateCandidate retains a replayable query/result candidate under a
// caller-owned private root, with no-replace publication. This is NOT a method
// receipt. Exact LSP frames are neither required nor retained here. The owner
// must explicitly review and remove the file; no timed deletion is implied.
func PublishPrivateCandidate(root *publication.Root, selector string, raw []byte) (*publication.BoundFileReceipt, error) {
	if _, err := VerifyCanonicalCandidate(raw); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, ErrInvalidCanonicalCandidate
	}
	return publication.PublishBoundFile(root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return ErrInvalidCanonicalCandidate
		}
		_, err := VerifyCanonicalCandidate(got)
		return err
	})
}

// ReadPrivateCandidate rechecks private custody, canonical replay and an
// independently supplied exact query identity. Its return value remains an
// unadmitted candidate, never an occurrence, ledger terminal or Leiden input.
func ReadPrivateCandidate(root *publication.Root, selector string, expected CandidateExpectation) (CanonicalCandidate, error) {
	bad := func() (CanonicalCandidate, error) { return CanonicalCandidate{}, ErrInvalidCanonicalCandidate }
	if expected.ID == "" || expected.SessionID == "" || expected.Method == "" || expected.QueryURI == "" || expected.ParamsSHA256 == "" || expected.Generation == 0 || expected.KeyID == 0 {
		return bad()
	}
	raw, err := publication.ReadVerifiedBoundFile(root, selector, maxCandidateBytes)
	if err != nil {
		return CanonicalCandidate{}, errors.Join(ErrInvalidCanonicalCandidate, err)
	}
	c, err := VerifyCanonicalCandidate(raw)
	if err != nil || c.ID != expected.ID || c.SessionID != expected.SessionID || c.Method != expected.Method || c.QueryURI != expected.QueryURI || c.ParamsSHA256 != expected.ParamsSHA256 || c.Generation != expected.Generation || c.KeyID != expected.KeyID || c.QueryLine != expected.QueryLine || c.QueryCharacter != expected.QueryCharacter {
		return bad()
	}
	return c, nil
}
