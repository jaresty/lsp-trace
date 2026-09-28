package adr0011closure

import (
	"crypto/sha256"
	"encoding/hex"

	"lsp-trace/internal/publication"
)

// Predecessor digests are raw SHA-256 of the independently resolved bytes,
// unlike the domain-separated closure identity. Neither hash authenticates a
// producer. The supplied role verifier must establish semantic and owner
// validity under a separately qualified custody policy.
const maxPredecessorBytes = 1 << 20

type predecessorResolver func(selector string, limit int) ([]byte, error)
type predecessorVerifier func(raw []byte, owner string) error

// replayWithPredecessors remains inert: this private test-owned conjunction
// cannot issue a query terminal or an occurrence.
func replayWithPredecessors(root *publication.Root, ref reference, owner, role string, expected []reference, resolve predecessorResolver, verify map[string]predecessorVerifier) error {
	if err := replay(root, ref, owner, role, expected); err != nil {
		return err
	}
	return resolvePredecessors(owner, expected, resolve, verify)
}

// resolvePredecessors is private, inert evidence checking: it neither issues
// a closure nor retains predecessor bytes in one. The resolver must enforce
// its limit during acquisition, not merely after returning.
func resolvePredecessors(owner string, expected []reference, resolve predecessorResolver, verify map[string]predecessorVerifier) error {
	if !token(owner) || resolve == nil || len(expected) == 0 || len(expected) > maxDependencies {
		return errClosure
	}
	seen := make(map[string]struct{}, len(expected))
	for _, ref := range expected {
		if !validRef(ref) {
			return errClosure
		}
		key := ref.Role + "\x00" + ref.Selector
		if _, exists := seen[key]; exists {
			return errClosure
		}
		// A selector cannot represent two independent roles or digests.
		if _, exists := seen[ref.Selector]; exists {
			return errClosure
		}
		seen[key] = struct{}{}
		seen[ref.Selector] = struct{}{}
		check := verify[ref.Role]
		if check == nil {
			return errClosure
		}
		raw, err := resolve(ref.Selector, maxPredecessorBytes)
		if err != nil || len(raw) == 0 || len(raw) > maxPredecessorBytes {
			return errClosure
		}
		sum := sha256.Sum256(raw)
		if ref.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
			return errClosure
		}
		if check(raw, owner) != nil {
			return errClosure
		}
	}
	return nil
}
