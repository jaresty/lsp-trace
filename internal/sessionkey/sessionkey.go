// Package sessionkey derives an opaque, immutable identity from resolved runtime
// identity. It deliberately carries no process command, credentials, or permissions.
package sessionkey

import (
	"crypto/sha256"
	"encoding/hex"
)

const domain = "lsp-trace/resolved-session-key/v1\x00"

// Material is trusted, resolved identity material. Its fields identify a runtime
// profile but cannot grant executable authority.
type Material struct {
	TrustDomain          string
	Workspace            string
	Profile              string
	EnvironmentReference string
	OptionsDigest        string
}

// Key is an immutable, opaque session identity.
type Key struct {
	digest [sha256.Size]byte
}

// Derive deterministically binds every resolved identity component using
// length-delimited fields, preventing boundary ambiguity.
func Derive(material Material) Key {
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	for _, field := range []string{
		material.TrustDomain,
		material.Workspace,
		material.Profile,
		material.EnvironmentReference,
		material.OptionsDigest,
	} {
		length := uint64(len(field))
		var encoded [8]byte
		for i := 7; i >= 0; i-- {
			encoded[i] = byte(length)
			length >>= 8
		}
		_, _ = h.Write(encoded[:])
		_, _ = h.Write([]byte(field))
	}
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return Key{digest: digest}
}

func (key Key) String() string {
	return "sk1:" + hex.EncodeToString(key.digest[:])
}

func (key Key) MarshalText() ([]byte, error) {
	return []byte(key.String()), nil
}
