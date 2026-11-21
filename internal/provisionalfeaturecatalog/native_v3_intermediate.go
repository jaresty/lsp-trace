//go:build darwin || linux

package provisionalfeaturecatalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"lsp-trace/internal/publication"
)

const intermediateVersionV3 = "lsp-trace.intermediate-catalog.v1"
const intermediatePrefixV3 = "inventory-intermediate-catalog-"

// IntermediateCatalogV3 is a fixed internal candidate, not a configurable
// sealer or native grant. Production Build/Apply continue to use the full sealer.
// The durable controller must select descriptions from its own accepted state.
type IntermediateCatalogV3 struct {
	catalog  NativeCatalogV3
	evidence *intermediateEvidenceV3
}
type intermediateEvidenceV3 struct {
	values  []SelectedEvidenceV3
	encoded [][]byte
	refs    []publication.StoredObject
	bytes   int
}
type intermediateRecordV3 struct {
	Version  string
	Metadata NativeCatalogWireV3
	Evidence []publication.StoredObject
}

// BuildIntermediateNativeV3 shares all existing construction guards. Descriptions
// must be genuine bound objects; a DTO cannot select this representation.
func BuildIntermediateNativeV3(ctx context.Context, b NativeBudgetV3, actor string, ds []NativeBoundDescriptionV3, mode ReviewModeV3) (IntermediateCatalogV3, error) {
	var snapshot *intermediateEvidenceV3
	c, e := buildNativeWithSealV3(ctx, b, actor, ds, func(ctx context.Context, b NativeBudgetV3, w NativeCatalogWireV3, a string, corrected bool) (NativeCatalogV3, error) {
		// Existing seal quote covers initial encoding. Additional reservation below
		// covers snapshot ownership, reference hashing and indexing before that work.
		if b.Reserve(ctx, nativeSealQuoteV3(w)) != nil {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
		var err error
		snapshot, err = ownIntermediateEvidenceV3(ctx, b, w.Evidence)
		if err != nil {
			return NativeCatalogV3{}, err
		}
		w.Evidence = snapshot.values
		return sealIntermediateReservedV3(ctx, b, w, a, corrected, snapshot)
	}, mode)
	if e != nil {
		return IntermediateCatalogV3{}, e
	}
	return IntermediateCatalogV3{c, snapshot}, nil
}

func ownIntermediateEvidenceV3(ctx context.Context, b NativeBudgetV3, values []SelectedEvidenceV3) (*intermediateEvidenceV3, error) {
	if len(values) > 100 || b.Reserve(ctx, uint64(len(values))*4096+4096) != nil {
		return nil, ErrNativeCatalogV3
	}
	s := &intermediateEvidenceV3{values: make([]SelectedEvidenceV3, len(values)), encoded: make([][]byte, len(values)), refs: make([]publication.StoredObject, len(values))}
	for i, e := range values {
		raw, err := json.Marshal(e) // exactly once per evidence, under initial seal quote
		if err != nil || len(raw) > 8<<20 || s.bytes > (8<<20)-len(raw) {
			return nil, ErrNativeCatalogV3
		}
		if b.Reserve(ctx, 8*uint64(len(raw))+4096) != nil {
			return nil, ErrNativeCatalogV3
		}
		// Unit, Request, Selected and their source fields are value-only (comparable
		// structs). Record is the only mutable evidence payload; never alias it.
		e.Record = append(json.RawMessage(nil), e.Record...)
		s.values[i] = e
		s.encoded[i] = raw
		s.refs[i] = publication.StoredObject{ID: NativeDigestV3(raw)[7:], Bytes: len(raw)}
		s.bytes += len(raw)
	}
	return s, nil
}

func (c IntermediateCatalogV3) Catalog() NativeCatalogV3 { return c.catalog }
func (c IntermediateCatalogV3) Apply(ctx context.Context, raw []byte, review bool) (IntermediateCatalogV3, error) {
	if c.evidence == nil {
		return IntermediateCatalogV3{}, ErrNativeCatalogV3
	}
	next, e := applyNativeDecisionWithSeal(ctx, c.catalog, raw, review, func(ctx context.Context, b NativeBudgetV3, w NativeCatalogWireV3, a string, r bool) (NativeCatalogV3, error) {
		if b.Reserve(ctx, nativeSealQuoteV3(w)) != nil {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
		return sealIntermediateReservedV3(ctx, b, w, a, r, c.evidence)
	})
	if e != nil {
		return IntermediateCatalogV3{}, e
	}
	return IntermediateCatalogV3{next, c.evidence}, nil
}

// metadataParts delegates field order, escaping (including invalid UTF-8),
// omitempty and number encoding to the existing wire's encoding/json contract.
// It never marshals an evidence-bearing catalog and strips the result afterward.
func intermediatePartsV3(w NativeCatalogWireV3) ([]byte, []byte, error) {
	w.Evidence = nil
	raw, e := json.Marshal(w)
	marker := []byte(`"evidence":null`)
	at := bytes.Index(raw, marker)
	if e != nil || at < 0 || bytes.Count(raw, marker) != 1 {
		return nil, nil, ErrNativeCatalogV3
	}
	return raw[:at], raw[at+len(marker):], nil
}
func streamIntermediateV3(dst io.Writer, pre, post []byte, s *intermediateEvidenceV3) error {
	parts := [][]byte{pre, []byte(`"evidence":[`)}
	for _, p := range parts {
		if _, e := dst.Write(p); e != nil {
			return e
		}
	}
	for i, p := range s.encoded {
		if i > 0 {
			if _, e := io.WriteString(dst, ","); e != nil {
				return e
			}
		}
		if _, e := dst.Write(p); e != nil {
			return e
		}
	}
	if _, e := io.WriteString(dst, "]"); e != nil {
		return e
	}
	_, e := dst.Write(post)
	return e
}
func sealIntermediateReservedV3(ctx context.Context, b NativeBudgetV3, w NativeCatalogWireV3, a string, corrected bool, s *intermediateEvidenceV3) (NativeCatalogV3, error) {
	if s == nil || len(w.Evidence) != len(s.values) || len(s.values) > 0 && &w.Evidence[0] != &s.values[0] {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	// The snapshot is immutable, shared only by this closed implementation. Clone
	// mutable metadata as well; no decision/result slice can change sealed state.
	affected := make([]string, len(w.Affected))
	copy(affected, w.Affected)
	if w.Affected == nil {
		affected = nil
	}
	w.Affected = affected
	groups := make([]NativeGroupV3, len(w.Groups))
	copy(groups, w.Groups)
	if w.Groups == nil {
		groups = nil
	}
	w.Groups = groups
	for i := range w.Groups {
		if w.Groups[i].Members != nil {
			members := make([]string, len(w.Groups[i].Members))
			copy(members, w.Groups[i].Members)
			w.Groups[i].Members = members
		}
	}
	w.Revision = ""
	pre, post, e := intermediatePartsV3(w)
	// Exact full length: empty revision becomes the 71-byte ASCII digest. Do
	// not reject otherwise supported near-limit catalogs with arbitrary slack.
	fullLength := len(pre) + len(post) + s.bytes + len(`"evidence":[]`) + max(0, len(s.values)-1) + 71
	if e != nil || fullLength > 8<<20 {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	h := sha256.New()
	if e = streamIntermediateV3(h, pre, post, s); e != nil {
		return NativeCatalogV3{}, e
	}
	w.Revision = fmt.Sprintf("sha256:%x", h.Sum(nil))
	pre, post, e = intermediatePartsV3(w)
	if e != nil {
		return NativeCatalogV3{}, e
	}
	var raw bytes.Buffer
	raw.Grow(fullLength)
	if e = streamIntermediateV3(&raw, pre, post, s); e != nil || raw.Len() != fullLength || ctx.Err() != nil {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	return NativeCatalogV3{&nativeCatalogStateV3{wire: w, raw: raw.Bytes(), budget: b, interpreter: a, corrected: corrected}}, nil
}

func (c IntermediateCatalogV3) Store(ctx context.Context, root *publication.Root) (publication.StoredObject, error) {
	if c.evidence == nil || c.catalog.state == nil {
		return publication.StoredObject{}, ErrNativeCatalogV3
	}
	w := c.catalog.state.wire
	w.Evidence = nil
	// Metadata marshal/copy and compact store's hash/write/readback. Full length
	// is a deliberately conservative pre-effect bound; no optimistic cost cut.
	if c.catalog.state.budget.Reserve(ctx, 16*uint64(len(c.catalog.state.raw))+16384) != nil {
		return publication.StoredObject{}, ErrNativeCatalogV3
	}
	raw, e := json.Marshal(intermediateRecordV3{intermediateVersionV3, w, c.evidence.refs})
	if e != nil || len(raw) > 8<<20 {
		return publication.StoredObject{}, ErrNativeCatalogV3
	}
	return publication.StoreObject(root, intermediatePrefixV3, raw)
}

// Reopen resolves only this owner's immutable evidence snapshot, itself built
// from controller-selected descriptions. Record references cannot import or
// select any other evidence object. expectedFull is the separate semantic input
// selected by controller state; compact bytes never replace task Input identity.
func (owner IntermediateCatalogV3) Reopen(ctx context.Context, root *publication.Root, ref, expectedFull publication.StoredObject, corrected bool) (IntermediateCatalogV3, error) {
	if owner.evidence == nil || owner.catalog.state == nil || !ref.Valid() || !expectedFull.Valid() {
		return IntermediateCatalogV3{}, ErrNativeCatalogV3
	}
	s := owner.catalog.state
	if s.budget.Reserve(ctx, 8*uint64(ref.Bytes)+2*uint64(expectedFull.Bytes)+16384) != nil {
		return IntermediateCatalogV3{}, ErrNativeCatalogV3
	}
	raw, e := publication.ReadObject(root, intermediatePrefixV3, ref)
	var r intermediateRecordV3
	if e != nil || DecodeNativeV3(raw, &r) != nil || r.Version != intermediateVersionV3 || r.Metadata.Evidence != nil || len(r.Evidence) != len(owner.evidence.refs) {
		return IntermediateCatalogV3{}, ErrNativeCatalogV3
	}
	canonical, e := json.Marshal(r)
	if e != nil || !bytes.Equal(raw, canonical) {
		return IntermediateCatalogV3{}, ErrNativeCatalogV3
	}
	for i, ref := range r.Evidence {
		if ref != owner.evidence.refs[i] {
			return IntermediateCatalogV3{}, ErrNativeCatalogV3
		}
	}
	w := r.Metadata
	if w.Version != "lsp-trace.native-catalog.v3" || w.Authority != 0 || w.Accepted || w.Status != "PROVISIONAL" || w.Completeness != "UNKNOWN" || w.ReviewMode != s.wire.ReviewMode || len(w.Groups) > 100 {
		return IntermediateCatalogV3{}, ErrNativeCatalogV3
	}
	revision := w.Revision
	w.Evidence = owner.evidence.values
	if s.budget.Reserve(ctx, nativeSealQuoteV3(w)) != nil {
		return IntermediateCatalogV3{}, ErrNativeCatalogV3
	}
	c, e := sealIntermediateReservedV3(ctx, s.budget, w, s.interpreter, corrected, owner.evidence)
	if e != nil || c.Revision() != revision || len(c.state.raw) != expectedFull.Bytes || NativeDigestV3(c.state.raw) != "sha256:"+expectedFull.ID {
		return IntermediateCatalogV3{}, ErrNativeCatalogV3
	}
	return IntermediateCatalogV3{c, owner.evidence}, nil
}
