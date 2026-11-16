package inventoryscope

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/bits"
	"path"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

func validDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	for _, c := range s[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func validID(s string) bool {
	if len(s) == 0 || len(s) > 256 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._:/@+-", c)) {
			return false
		}
	}
	return true
}
func validRef(r ArtifactRefV1) bool { return validID(r.ArtifactID) && validDigest(r.Digest) }

// CanonicalRelativePathV1 is case-sensitive and lexical, independent of OS.
// Normalization, symlink resolution and case-folding are deliberately absent.
func CanonicalRelativePathV1(s string) bool {
	if s == "" || len(s) > 16384 || !utf8.ValidString(s) || strings.ContainsAny(s, "\x00\\:") || strings.HasPrefix(s, "/") || path.Clean(s) != s {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." {
			return false
		}
	}
	return true
}

func digestV1(domain string, v any) string {
	b, _ := json.Marshal(v) // All callers use closed, JSON-encodable structs/slices.
	h := sha256.New()
	h.Write([]byte("lsp-trace:" + domain + "\n"))
	h.Write(b)
	h.Write([]byte{'\n'})
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func CandidateIDV1(source, relative string) (string, error) {
	if !validID(source) || len(relative) > 16384 || !CanonicalRelativePathV1(relative) {
		return "", ErrInvalidV1
	}
	return digestV1("inventory-candidate/v1", struct{ Source, Path string }{source, relative}), nil
}

// scanBudget never clones the input. Subtraction prevents integer overflow.
// Its strings and collection-element work account precedes any sort/hash/copy.
func scanBudget(v reflect.Value, bytes, work *int) error {
	if *work == 0 {
		return ErrLimitV1
	}
	*work--
	switch v.Kind() {
	case reflect.String:
		s := v.String()
		if len(s) > *bytes {
			return ErrLimitV1
		}
		*bytes -= len(s)
		if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
			return ErrInvalidV1
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := scanBudget(v.Field(i), bytes, work); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if v.Len() > *work {
			return ErrLimitV1
		}
		for i := 0; i < v.Len(); i++ {
			if err := scanBudget(v.Index(i), bytes, work); err != nil {
				return err
			}
		}
	}
	return nil
}
func boundedV1(v any, l LimitsV1) error {
	return boundedWorkV1(v, l, l.MaxWork)
}

// boundedWorkV1 scans the caller-owned value against the work remaining after
// collection operations are prepaid. String bytes bound encoding/hash work;
// structural visits bound the closed schema's remaining encoding overhead.
func boundedWorkV1(v any, l LimitsV1, work int) error {
	b := l.MaxUTF8Bytes
	return scanBudget(reflect.ValueOf(v), &b, &work)
}

// chargeWorkV1 reserves without computing an overflowing product or sum. It
// never raises a caller limit, and failure leaves the remaining account intact.
func chargeWorkV1(work *int, n, factor int) bool {
	if n < 0 || factor < 0 || *work < 0 {
		return false
	}
	if factor == 0 {
		return true
	}
	if n > *work/factor {
		return false
	}
	*work -= n * factor
	return true
}

func chargeSortV1(work *int, n int) bool {
	return chargeWorkV1(work, n, bits.Len(uint(n))+1)
}
func boundInputV1(in InputV1, l LimitsV1) error {
	if len(in.Candidates) > l.MaxCandidates || len(in.Observations) > l.MaxObservations || len(in.Resolutions) > l.MaxResolutions {
		return ErrLimitV1
	}
	remaining := l.MaxReferences
	for _, r := range in.Resolutions {
		if len(r.ObservationRefs) > remaining {
			return ErrLimitV1
		}
		remaining -= len(r.ObservationRefs)
	}
	// Output evidence references are also prepaid (at most one declaration
	// reference per candidate plus every observation).
	if len(in.Candidates) > l.MaxReferences || len(in.Observations) > l.MaxReferences-len(in.Candidates) {
		return ErrLimitV1
	}
	work := l.MaxWork
	for _, n := range []int{len(in.Candidates), len(in.Observations), len(in.Resolutions), len(in.Declaration.ApplicationRoots), len(in.Declaration.Includes), len(in.Declaration.Excludes)} {
		if !chargeSortV1(&work, n) {
			return ErrLimitV1
		}
	}
	if !chargeWorkV1(&work, len(in.Candidates), len(in.Declaration.ApplicationRoots)) {
		return ErrLimitV1
	}
	for _, r := range in.Resolutions {
		if !chargeSortV1(&work, len(r.ObservationRefs)) {
			return ErrLimitV1
		}
	}
	b := l.MaxUTF8Bytes
	return scanBudget(reflect.ValueOf(in), &b, &work)
}
func sortedSetV1(in []string) ([]string, error) {
	out := append([]string{}, in...)
	sort.Strings(out)
	for i := 1; i < len(out); i++ {
		if out[i] == out[i-1] {
			return nil, ErrInvalidV1
		}
	}
	return out, nil
}

func ObservationIDV1(o ObservationV1, l LimitsV1) (string, error) {
	l, e := l.EffectiveV1()
	if e != nil {
		return "", e
	}
	if e = boundedV1(o, l); e != nil {
		return "", e
	}
	o.ObservationID = ""
	return digestV1("inventory-observation/v1", o), nil
}
func ResolutionIDV1(r ResolutionV1, l LimitsV1) (string, error) {
	l, e := l.EffectiveV1()
	if e != nil {
		return "", e
	}
	if len(r.ObservationRefs) > l.MaxReferences {
		return "", ErrLimitV1
	}
	work := l.MaxWork
	if !chargeSortV1(&work, len(r.ObservationRefs)) {
		return "", ErrLimitV1
	}
	if e = boundedWorkV1(r, l, work); e != nil {
		return "", e
	}
	r.ResolutionID = ""
	r.ObservationRefs, e = sortedSetV1(r.ObservationRefs)
	if e != nil {
		return "", e
	}
	return digestV1("inventory-resolution/v1", r), nil
}

// CanonicalManifestV1 returns fixed-field, sorted-set JSON followed by LF.
// ScopeDigest is included in these artifact bytes. The scope identity itself
// uses the same representation with ScopeDigest empty and a distinct domain.
func CanonicalManifestV1(m ManifestV1, l LimitsV1) ([]byte, error) {
	l, e := l.EffectiveV1()
	if e != nil {
		return nil, e
	}
	if len(m.Entries) > l.MaxCandidates {
		return nil, ErrLimitV1
	}
	work := l.MaxWork
	if !chargeSortV1(&work, len(m.Entries)) {
		return nil, ErrLimitV1
	}
	refs := l.MaxReferences
	for _, v := range m.Entries {
		if len(v.EvidenceRefs) > refs {
			return nil, ErrLimitV1
		}
		refs -= len(v.EvidenceRefs)
		if !chargeSortV1(&work, len(v.EvidenceRefs)) {
			return nil, ErrLimitV1
		}
	}
	if e = boundedWorkV1(m, l, work); e != nil {
		return nil, e
	}
	m.Entries = append([]EntryV1{}, m.Entries...)
	for i := range m.Entries {
		m.Entries[i].EvidenceRefs, e = sortedSetV1(m.Entries[i].EvidenceRefs)
		if e != nil {
			return nil, e
		}
	}
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].CandidateID < m.Entries[j].CandidateID })
	b, e := json.Marshal(m)
	if e != nil {
		return nil, ErrInvalidV1
	}
	if int64(len(b)) >= l.MaxInputBytes {
		return nil, ErrLimitV1
	}
	return append(b, '\n'), nil
}
