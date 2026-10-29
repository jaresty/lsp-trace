package acquisition

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// These private V2 metadata limits are not public acquisition defaults.
const (
	frozenQueryMembersPrivateV2    = 16
	frozenQueryURIBytesPrivateV2   = 8192
	frozenQueryTotalBytesPrivateV2 = 128 << 10
)

// FrozenQueryDeclarationPrivateV2 is immutable restrict-only query metadata.
// It grants no acquisition authorization, source custody, or Manager binding.
// Copies share immutable state; the zero value is invalid.
type FrozenQueryDeclarationPrivateV2 struct{ state *frozenQueryStatePrivateV2 }
type frozenQueryStatePrivateV2 struct {
	members []string
	seal    *frozenQuerySealPrivateV2
}
type frozenQuerySealPrivateV2 struct{ marker byte }

var frozenQuerySealV2 = &frozenQuerySealPrivateV2{marker: 1}

// NewFrozenQueryDeclarationPrivateV2 validates exact, already sorted canonical
// URIs. It does not normalize, sort, acquire files, or contact providers.
func NewFrozenQueryDeclarationPrivateV2(members []string) (FrozenQueryDeclarationPrivateV2, error) {
	if err := validateFrozenMembersPrivateV2(members); err != nil {
		return FrozenQueryDeclarationPrivateV2{}, err
	}
	owned := make([]string, len(members))
	for i, uri := range members {
		owned[i] = strings.Clone(uri)
	}
	return FrozenQueryDeclarationPrivateV2{state: &frozenQueryStatePrivateV2{members: owned, seal: frozenQuerySealV2}}, nil
}

func validateFrozenMembersPrivateV2(members []string) error {
	// Preflight all lengths before UTF-8 checks, parsing, copying, or maps.
	if len(members) < 1 || len(members) > frozenQueryMembersPrivateV2 {
		return errors.New("private V2 frozen query member count must be 1..16")
	}
	total := 0
	for _, uri := range members {
		if len(uri) > frozenQueryURIBytesPrivateV2 || len(uri) > frozenQueryTotalBytesPrivateV2-total {
			return errors.New("private V2 frozen query metadata byte limit exceeded")
		}
		total += len(uri)
	}
	for i, uri := range members {
		if !utf8.ValidString(uri) || !canonicalURI(uri) || (i > 0 && members[i-1] >= uri) {
			return errors.New("private V2 frozen query declaration must be sorted canonical unique URIs")
		}
	}
	return nil
}

func (d FrozenQueryDeclarationPrivateV2) validate() error {
	if d.state == nil || d.state.seal != frozenQuerySealV2 {
		return errors.New("invalid private V2 frozen query declaration")
	}
	return validateFrozenMembersPrivateV2(d.state.members)
}

// MembersPrivateV2 returns a detached copy for metadata reconstruction by a
// future validated retained V2 codec, not the V1 decoder or an execution grant.
func (d FrozenQueryDeclarationPrivateV2) MembersPrivateV2() []string {
	if d.validate() != nil {
		return nil
	}
	return append([]string(nil), d.state.members...)
}

func (d FrozenQueryDeclarationPrivateV2) contains(uri string) bool {
	if d.validate() != nil {
		return false
	}
	for _, member := range d.state.members {
		if member == uri {
			return true
		}
	}
	return false
}

// WithFrozenQueryDeclarationPrivateV2 returns a request copy restricted by d.
// It checks only metadata and target membership, not operation authorization or
// source binding. RequiredTargets and other ordinary Request fields retain
// their existing copy semantics.
func (r Request) WithFrozenQueryDeclarationPrivateV2(d FrozenQueryDeclarationPrivateV2) (Request, error) {
	if r.FrozenQueryURIsPrivate != [6]string{} {
		return Request{}, errors.New("private frozen query V1 and V2 declarations cannot be combined")
	}
	if err := d.validate(); err != nil {
		return Request{}, err
	}
	r.frozenQueryDeclarationPrivateV2 = &d
	if err := r.validateFrozenQueryPrivate(); err != nil {
		return Request{}, err
	}
	return r, nil
}
