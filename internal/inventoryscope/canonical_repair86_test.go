package inventoryscope

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"sort"
	"testing"
)

// These deliberately independent shape counts do not call production budget
// functions: ResolutionV1 has root + six fields; ManifestV1 has root + thirteen
// fields, and each EntryV1 has root + five fields. Slice elements add visits.
func repair86SortCost(n int) int { return n * (bits.Len(uint(n)) + 1) }
func repair86Refs(n int) []string {
	refs := []string{}
	for i := n - 1; i >= 0; i-- {
		refs = append(refs, fmt.Sprintf("ref-%03d", i))
	}
	return refs
}
func repair86Resolution(n int) ResolutionV1 {
	return ResolutionV1{ResolutionID: "ignored-self", CandidateID: "candidate", ObservationRefs: repair86Refs(n), Rule: ResolutionIntentFirstV1, Outcome: IncludedV1, CallerActionRef: "controlled-action"}
}
func repair86Manifest(entries, refs int) ManifestV1 {
	m := ManifestV1{Schema: SchemaV1, Entries: []EntryV1{}, IncludedCount: entries, OriginalDenominatorCount: entries}
	for i := entries - 1; i >= 0; i-- {
		m.Entries = append(m.Entries, EntryV1{CandidateID: fmt.Sprintf("candidate-%03d", i), Membership: IncludedV1, Reason: ReasonCallerIncludedV1, EvidenceRefs: repair86Refs(refs)})
	}
	return m
}
func repair86Assert(t *testing.T, assertion string, ok bool, detail string) {
	t.Helper()
	if !ok {
		t.Errorf("%s FAIL: %s", assertion, detail)
		return
	}
	t.Logf("%s PASS: %s", assertion, detail)
}

func TestRepair86WorkArithmetic(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		work, n, factor, want int
		ok                    bool
	}{
		{20, 3, 6, 2, true}, {17, 3, 6, 17, false},
		{maxInt, maxInt, 2, maxInt, false}, {maxInt, maxInt, maxInt, maxInt, false},
		{maxInt, maxInt, 1, 0, true}, {0, maxInt, 0, 0, true},
		{20, -1, 2, 20, false}, {20, 1, -2, 20, false}, {-1, 0, 0, -1, false},
	} {
		work := tc.work
		ok := chargeWorkV1(&work, tc.n, tc.factor)
		repair86Assert(t, "resolution-work-prepaid", ok == tc.ok && work == tc.want, fmt.Sprintf("overflow-safe charge work=%d n=%d factor=%d accepted=%t remaining=%d", tc.work, tc.n, tc.factor, ok, work))
	}
	work := maxInt
	repair86Assert(t, "manifest-work-prepaid", !chargeSortV1(&work, maxInt) && work == maxInt, "overflowing sort charge rejected without changing account")
}

func TestRepair86UTF8Prepayment(t *testing.T) {
	r := repair86Resolution(1)
	chars := len(r.ResolutionID) + len(r.CandidateID) + len(r.Rule) + len(r.Outcome) + len(r.CallerActionRef) + len(r.ObservationRefs[0])
	_, err := ResolutionIDV1(r, LimitsV1{MaxUTF8Bytes: chars - 1})
	repair86Assert(t, "resolution-work-prepaid", errors.Is(err, ErrLimitV1), "hash string bytes must be prepaid")
	_, err = ResolutionIDV1(r, LimitsV1{MaxUTF8Bytes: chars})
	repair86Assert(t, "resolution-work-prepaid", err == nil, "exact hash string-byte boundary accepted")
	m := repair86Manifest(1, 1)
	chars = len(m.Schema) + len(m.Entries[0].CandidateID) + len(m.Entries[0].Membership) + len(m.Entries[0].Reason) + len(m.Entries[0].EvidenceRefs[0])
	_, err = CanonicalManifestV1(m, LimitsV1{MaxUTF8Bytes: chars - 1})
	repair86Assert(t, "manifest-work-prepaid", errors.Is(err, ErrLimitV1), "encoding string bytes must be prepaid")
	_, err = CanonicalManifestV1(m, LimitsV1{MaxUTF8Bytes: chars})
	repair86Assert(t, "manifest-work-prepaid", err == nil, "exact encoding string-byte boundary accepted")
}

func TestRepair86ResolutionWork(t *testing.T) {
	for _, n := range []int{0, 1, 2, 64} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			r := repair86Resolution(n)
			structural := 7 + n
			full := structural + repair86SortCost(n)
			for _, budget := range []int{1, structural - 1, full - 1} {
				_, err := ResolutionIDV1(r, LimitsV1{MaxWork: budget})
				repair86Assert(t, "resolution-work-prepaid", errors.Is(err, ErrLimitV1), fmt.Sprintf("n=%d budget=%d complete=%d err=%v", n, budget, full, err))
			}
			_, err := ResolutionIDV1(r, LimitsV1{MaxWork: full})
			repair86Assert(t, "resolution-work-prepaid", err == nil, fmt.Sprintf("n=%d exact=%d err=%v", n, full, err))
			if n > 0 {
				_, err = ResolutionIDV1(r, LimitsV1{MaxWork: structural})
				repair86Assert(t, "resolution-work-prepaid", errors.Is(err, ErrLimitV1), fmt.Sprintf("n=%d structural-only=%d sort=%d err=%v", n, structural, full-structural, err))
			}
		})
	}
}
func TestRepair86ManifestWork(t *testing.T) {
	for _, shape := range [][2]int{{0, 0}, {1, 0}, {1, 1}, {64, 1}, {1, 64}, {2, 64}} {
		t.Run(fmt.Sprintf("%dx%d", shape[0], shape[1]), func(t *testing.T) {
			n, k := shape[0], shape[1]
			m := repair86Manifest(n, k)
			structural := 14 + n*(6+k)
			full := structural + repair86SortCost(n) + n*repair86SortCost(k)
			for _, budget := range []int{1, structural - 1, full - 1} {
				_, err := CanonicalManifestV1(m, LimitsV1{MaxWork: budget})
				repair86Assert(t, "manifest-work-prepaid", errors.Is(err, ErrLimitV1), fmt.Sprintf("shape=%v budget=%d complete=%d err=%v", shape, budget, full, err))
			}
			_, err := CanonicalManifestV1(m, LimitsV1{MaxWork: full})
			repair86Assert(t, "manifest-work-prepaid", err == nil, fmt.Sprintf("shape=%v exact=%d err=%v", shape, full, err))
			if n > 0 {
				_, err = CanonicalManifestV1(m, LimitsV1{MaxWork: structural})
				repair86Assert(t, "manifest-work-prepaid", errors.Is(err, ErrLimitV1), fmt.Sprintf("shape=%v structural-only=%d sort=%d err=%v", shape, structural, full-structural, err))
				// Paying only either component must not suffice for the whole operation.
				if k > 0 {
					for _, partial := range []int{structural + repair86SortCost(n), structural + n*repair86SortCost(k)} {
						_, err = CanonicalManifestV1(m, LimitsV1{MaxWork: partial})
						repair86Assert(t, "manifest-work-prepaid", errors.Is(err, ErrLimitV1), fmt.Sprintf("shape=%v partial=%d complete=%d err=%v", shape, partial, full, err))
					}
				}
			}
		})
	}
}
func TestRepair86ResolutionBytes(t *testing.T) {
	for _, n := range []int{0, 1, 64} {
		r := repair86Resolution(n)
		reference := r
		reference.ResolutionID = ""
		reference.ObservationRefs = append([]string{}, r.ObservationRefs...)
		sort.Strings(reference.ObservationRefs)
		b, err := json.Marshal(reference)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(append(append([]byte("lsp-trace:inventory-resolution/v1\n"), b...), '\n'))
		want := "sha256:" + hex.EncodeToString(sum[:])
		for _, limits := range []LimitsV1{{}, {MaxWork: 7 + n + repair86SortCost(n)}} {
			got, err := ResolutionIDV1(r, limits)
			repair86Assert(t, "resolution-funded-bytes-preserved", err == nil && got == want, fmt.Sprintf("n=%d got=%s want=%s err=%v", n, got, want, err))
		}
	}
}
func TestRepair86ManifestBytes(t *testing.T) {
	for _, shape := range [][2]int{{0, 0}, {64, 1}, {1, 64}} {
		n, k := shape[0], shape[1]
		m := repair86Manifest(n, k)
		reference := m
		reference.Entries = append([]EntryV1{}, m.Entries...)
		for i := range reference.Entries {
			reference.Entries[i].EvidenceRefs = append([]string{}, m.Entries[i].EvidenceRefs...)
			sort.Strings(reference.Entries[i].EvidenceRefs)
		}
		sort.Slice(reference.Entries, func(i, j int) bool { return reference.Entries[i].CandidateID < reference.Entries[j].CandidateID })
		b, err := json.Marshal(reference)
		if err != nil {
			t.Fatal(err)
		}
		want := append(b, '\n')
		full := 14 + n*(6+k) + repair86SortCost(n) + n*repair86SortCost(k)
		for _, limits := range []LimitsV1{{}, {MaxWork: full}} {
			got, err := CanonicalManifestV1(m, limits)
			repair86Assert(t, "manifest-funded-bytes-preserved", err == nil && bytes.Equal(got, want), fmt.Sprintf("shape=%v bytes=%d digest=%x err=%v", shape, len(got), sha256.Sum256(got), err))
		}
	}
}
