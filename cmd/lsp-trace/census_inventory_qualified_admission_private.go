//go:build darwin || linux

package main

import (
	"bytes"
	"encoding"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/inventoryscope"
	product "lsp-trace/internal/privatehostproductv3"
)

const censusInventoryQualifiedHeaderSchema = "private-connected-inventory-qualified-header.v1"

// This extension is complete preparation DATA. Neither its family nor its
// caller-declared universe is source authority or filesystem verification.
// Omission preserves the pre-extension legacy header's exact canonical bytes.
type censusInventoryQualifiedAdmission struct {
	Qualified           censussourceplan.QualifiedInputV1
	Envelope            censusQualifiedCLIEnvelopePrivate
	PreparationIdentity string
}

// One bounded validation/canonicalization path for every header reference.
// Validate the original caller envelope, then compare the FULL rebuilt qualified
// input. Never substitute AnalysisPlan.Identity for the qualified family digest.
func canonicalCensusInventoryHeader(h censusInventoryHeader) (censusInventoryHeader, []byte, error) {
	fail := func() (censusInventoryHeader, []byte, error) {
		return censusInventoryHeader{}, nil, errCensusInventoryPrivate
	}
	if h.Schema == "" && h.QualifiedAdmission == nil {
		h.Schema = "private-connected-inventory-header.v1"
	}
	marshal := censusInventoryMarshal
	if h.QualifiedAdmission != nil {
		marshal = censusInventoryQualifiedMarshal
	}
	raw, e := marshal(h, 4<<20)
	if e != nil || h.SourceRevision != censusInventorySourceRevision || h.Workspace == "" || h.Workflow == "" || len(h.Workflow) > 4096 {
		return fail()
	}
	if h.QualifiedAdmission == nil {
		if h.Schema != "private-connected-inventory-header.v1" || h.PlanIdentity != h.Plan.Identity || censussourceplan.Validate(h.Plan, 16) != nil {
			return fail()
		}
		return h, raw, nil
	}
	if h.Schema != censusInventoryQualifiedHeaderSchema {
		return fail()
	}
	x := h.QualifiedAdmission
	envRaw, e := json.Marshal(x.Envelope)
	if e != nil {
		return fail()
	}
	env, e := decodeCensusQualifiedCLIPrivate(envRaw, inventoryscope.LimitsV1{})
	if e != nil {
		return fail()
	}
	q, e := validateCensusQualifiedCLIPrivate(env)
	if e != nil || censussourceplan.ValidateQualifiedV1(x.Qualified, inventoryscope.LimitsV1{}) != nil {
		return fail()
	}
	rebuilt, e := json.Marshal(q)
	if e != nil {
		return fail()
	}
	supplied, e := json.Marshal(x.Qualified)
	if e != nil || !bytes.Equal(rebuilt, supplied) {
		return fail()
	}
	_, identity, e := censusQualifiedCLIIdentityPrivate(env, q)
	plan, e2 := json.Marshal(h.Plan)
	expected, e3 := json.Marshal(q.AnalysisPlan)
	if e != nil || e2 != nil || e3 != nil || !bytes.Equal(plan, expected) || x.PreparationIdentity != identity || h.PlanIdentity != q.QualifiedDigest || h.Workspace != q.Workspace || h.Scope != q.Scope.ScopeDigest || h.Provenance != env.Universe.Claim || env.Preparation.RevisionAssertion != h.SourceRevision || q.PartitionSize != 16 {
		return fail()
	}
	return h, raw, nil
}

// Expected binding remains data; the successor checks the actual admitted job.
func censusInventoryQualifiedNativeBinding(h censusInventoryHeader, digest string, q product.InventoryRequestV3) bool {
	_, raw, e := canonicalCensusInventoryHeader(h)
	if e != nil || censusInventoryDigest(raw) != digest {
		return false
	}
	// A legacy header does not erase an explicit source expectation. Bind
	// every nonempty expectation before either family's eligibility rule.
	for _, owner := range q.Inventory.NativeOwners {
		if owner.ExpectedSourceAuthorization != "" && owner.ExpectedSourceAuthorization != digest {
			return false
		}
	}
	if h.QualifiedAdmission == nil {
		return true
	}
	return len(q.Inventory.NativeOwners) == 1 && q.Inventory.NativeOwners[0].ExpectedSourceAuthorization == digest
}

func censusInventoryPartitionAvailable(h censusInventoryHeader, ordinal int) bool {
	if ordinal < 0 || ordinal >= len(h.Plan.Partitions) {
		return false
	}
	if h.QualifiedAdmission == nil {
		return true
	}
	if h.QualifiedAdmission.Qualified.NoWork {
		return false
	}
	for _, f := range h.Plan.Partitions[ordinal].Members {
		if f.Disposition != "" {
			return false
		}
	}
	return len(h.Plan.Partitions[ordinal].Members) > 0
}

// This codec is deliberately limited to the CURRENT closed header graph. It
// only counts; encoding/json remains the sole encoder. No callbacks, globals,
// custom marshalers, time.Time, embedded-field dominance, maps or byte slices
// belong to that graph. Unknown types/tags fail before whole-body allocation.
func censusInventoryQualifiedStruct(t reflect.Type) bool {
	switch t {
	case reflect.TypeFor[censusInventoryHeader](),
		reflect.TypeFor[censusInventoryQualifiedAdmission](),
		reflect.TypeFor[censusQualifiedCLIEnvelopePrivate](),
		reflect.TypeFor[censusQualifiedUniversePrivate](),
		reflect.TypeFor[censusApplicationQualifiedOptionsPrivate](),
		reflect.TypeFor[censussourceplan.Plan](),
		reflect.TypeFor[censussourceplan.Partition](),
		reflect.TypeFor[censussourceplan.QualifiedInputV1](),
		reflect.TypeFor[censusacquisition.SourceFile](),
		reflect.TypeFor[censusacquisition.DiscoveryLimits](),
		reflect.TypeFor[censusacquisition.PlanningConfig](),
		reflect.TypeFor[pilot.ScopeEffectivePolicyPrivateV2](),
		reflect.TypeFor[inventoryscope.InputV1](),
		reflect.TypeFor[inventoryscope.DeclarationV1](),
		reflect.TypeFor[inventoryscope.CandidateV1](),
		reflect.TypeFor[inventoryscope.ArtifactRefV1](),
		reflect.TypeFor[inventoryscope.ObservationV1](),
		reflect.TypeFor[inventoryscope.ResolutionV1](),
		reflect.TypeFor[inventoryscope.ManifestV1](),
		reflect.TypeFor[inventoryscope.EntryV1](),
		reflect.TypeFor[inventoryscope.SelectionV1]():
		return true
	}
	return false
}

type censusInventoryQualifiedField struct {
	name  string
	index int
	omit  bool
}

type censusInventoryQualifiedSizer struct {
	left   int
	fields map[reflect.Type][]censusInventoryQualifiedField
}

// Schema closure is local and acyclic. Even nil/omitted fields must have a
// supported type: no untrusted encoder or IsZero method is ever invoked.
func censusInventoryQualifiedSchema() (map[reflect.Type][]censusInventoryQualifiedField, bool) {
	fields := make(map[reflect.Type][]censusInventoryQualifiedField)
	active := make(map[reflect.Type]bool)
	var visit func(reflect.Type, int) bool
	visit = func(t reflect.Type, depth int) bool {
		if depth > 32 || active[t] {
			return false
		}
		if _, ok := fields[t]; ok {
			return true
		}
		for _, encoder := range []reflect.Type{reflect.TypeFor[json.Marshaler](), reflect.TypeFor[encoding.TextMarshaler]()} {
			if t.Implements(encoder) || reflect.PointerTo(t).Implements(encoder) {
				return false
			}
		}
		active[t] = true
		defer delete(active, t)
		switch t.Kind() {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Uint8:
		case reflect.Pointer, reflect.Slice:
			if !visit(t.Elem(), depth+1) {
				return false
			}
		case reflect.Struct:
			if !censusInventoryQualifiedStruct(t) {
				return false
			}
			names := make(map[string]bool)
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if f.Anonymous || f.PkgPath != "" {
					return false
				}
				tag := strings.Split(f.Tag.Get("json"), ",")
				name := tag[0]
				if name == "" {
					name = f.Name
				}
				// Current tags are plain names with at most omitempty. In
				// particular there is no omitzero/custom IsZero case to guess.
				omit := len(tag) == 2 && tag[1] == "omitempty"
				if len(tag) > 2 || (len(tag) == 2 && !omit) || name == "-" || names[name] {
					return false
				}
				for _, c := range name {
					if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
						return false
					}
				}
				names[name] = true
				if !visit(f.Type, depth+1) {
					return false
				}
				fields[t] = append(fields[t], censusInventoryQualifiedField{name, i, omit})
			}
		default:
			return false
		}
		if _, ok := fields[t]; !ok {
			fields[t] = nil
		}
		return true
	}
	return fields, visit(reflect.TypeFor[censusInventoryHeader](), 0)
}

func (s *censusInventoryQualifiedSizer) take(n int) bool {
	if n < 0 || n > s.left {
		return false
	}
	s.left -= n
	return true
}

// Match encoding/json's default EscapeHTML=true string encoding byte-for-byte.
// Invalid UTF-8 consumes one input byte per replacement escape, not one rune.
func (s *censusInventoryQualifiedSizer) text(v string) bool {
	if !s.take(2) {
		return false
	}
	for i := 0; i < len(v); {
		c := v[i]
		if c < utf8.RuneSelf {
			n := 1
			switch c {
			case '"', '\\', '\b', '\f', '\n', '\r', '\t':
				n = 2
			case '<', '>', '&':
				n = 6
			default:
				if c < 0x20 {
					n = 6
				}
			}
			if !s.take(n) {
				return false
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(v[i:])
		n := size
		if (r == utf8.RuneError && size == 1) || r == '\u2028' || r == '\u2029' {
			n = 6
		}
		if !s.take(n) {
			return false
		}
		i += size
	}
	return true
}

func (s *censusInventoryQualifiedSizer) value(v reflect.Value, depth int) bool {
	if depth > 32 || !v.IsValid() {
		return false
	}
	info, ok := s.fields[v.Type()]
	if !ok {
		return false
	}
	switch v.Kind() {
	case reflect.String:
		return s.text(v.String())
	case reflect.Bool:
		if v.Bool() {
			return s.take(4)
		}
		return s.take(5)
	case reflect.Int:
		return s.take(len(strconv.FormatInt(v.Int(), 10)))
	case reflect.Uint8:
		return s.take(len(strconv.FormatUint(v.Uint(), 10)))
	case reflect.Pointer:
		if v.IsNil() {
			return s.take(4)
		}
		return s.value(v.Elem(), depth+1)
	case reflect.Slice:
		if v.Len() > 100000 {
			return false
		}
		if v.IsNil() {
			return s.take(4)
		}
		if !s.take(2) {
			return false
		}
		for i := 0; i < v.Len(); i++ {
			if i > 0 && !s.take(1) {
				return false
			}
			if !s.value(v.Index(i), depth+1) {
				return false
			}
		}
		return true
	case reflect.Struct:
		if !s.take(2) {
			return false
		}
		wrote := false
		for _, f := range info {
			x := v.Field(f.index)
			if f.omit {
				empty := false
				switch x.Kind() {
				case reflect.String, reflect.Slice:
					empty = x.Len() == 0
				case reflect.Pointer:
					empty = x.IsNil()
				case reflect.Bool:
					empty = !x.Bool()
				case reflect.Int:
					empty = x.Int() == 0
				case reflect.Uint8:
					empty = x.Uint() == 0
				}
				if empty {
					continue
				}
			}
			if wrote && !s.take(1) {
				return false
			}
			wrote = true
			if !s.text(f.name) || !s.take(1) || !s.value(x, depth+1) {
				return false
			}
		}
		return true
	}
	return false
}

func censusInventoryQualifiedSize(v any, limit int) (int, error) {
	if limit <= 0 || limit > 4<<20 {
		return 0, errCensusInventoryPrivate
	}
	fields, ok := censusInventoryQualifiedSchema()
	if !ok {
		return 0, errCensusInventoryPrivate
	}
	s := censusInventoryQualifiedSizer{left: limit, fields: fields}
	if !s.value(reflect.ValueOf(v), 0) {
		return 0, errCensusInventoryPrivate
	}
	return limit - s.left, nil
}

func censusInventoryQualifiedMarshal(v any, limit int) ([]byte, error) {
	h, ok := v.(censusInventoryHeader)
	if !ok || h.QualifiedAdmission == nil {
		return nil, errCensusInventoryPrivate
	}
	n, e := censusInventoryQualifiedSize(h, limit)
	if e != nil {
		return nil, e
	}
	// The only whole-body allocation follows successful exact size admission.
	raw, e := json.Marshal(h)
	if e != nil || len(raw) != n {
		return nil, errCensusInventoryPrivate
	}
	return raw, nil
}
