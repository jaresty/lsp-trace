package inventorysuccessor

import (
	"errors"
	"reflect"
	"slices"
	"unicode/utf8"

	cc "lsp-trace/internal/censuscontinuation"
	rg "lsp-trace/internal/referencegrouping"
	tp "lsp-trace/internal/targetpacket"
)

// rdDescriptorInputV2 is a bounded projection of one scheduled RD nomination and
// one of its source intents. All fields are DATA ONLY. Neither agreement, hashes,
// nor descriptor possession proves that any original producer or nomination exists.
// Kinds and KindCount use the original PrivateManagedInputBindingV2 metadata
// types. Slot.Mode is the census mode, NOT the native ACQUIRE/ISSUE/REPLAY mode.
// Slot.SeedNamespace is retained separately: nominations/intents omit it.
//
// No original RD nomination is supplied by this unit. A future native join must
// provide independently trusted expected values, the exact opaque handoff and
// original source/definition handles, and whole-record Resume validation. Source
// and definition IDs are deliberately not invented here. This cannot recover an
// RD target from R, admit a source, mint custody, or replace opaque Resume.
type rdDescriptorInputV2 struct {
	Slot       cc.PrivateAllModeSlot
	Nomination tp.PrivateNomination
	Intent     tp.PrivateSourceIntent
	Kinds      [3]string
	KindCount  uint8
}

// Unexported immutable-by-interface comparison snapshot, not a capability. No
// serializer or native integration caller is provided. Same-package code is not
// a security boundary; valid records only describe internally coherent values.
type rdDescriptorV2 struct {
	value rdDescriptorInputV2
	valid bool
}

var errRDDescriptorV2 = errors.New("private RD descriptor comparison refused")

func newRDDescriptorV2(v rdDescriptorInputV2) (rdDescriptorV2, error) {
	if !validRDDescriptorInputV2(v) {
		return rdDescriptorV2{}, errRDDescriptorV2
	}
	v.Nomination.Members = slices.Clone(v.Nomination.Members)
	v.Nomination.Endpoints = slices.Clone(v.Nomination.Endpoints)
	return rdDescriptorV2{value: v, valid: true}, nil
}

func validRDDescriptorInputV2(v rdDescriptorInputV2) bool {
	// Require the original count, distinct actual kinds and zero unused tail BEFORE
	// normalizing any copy. A deduplicated/sorted set is insufficient.
	if v.KindCount != 2 || v.Kinds[2] != "" ||
		!((v.Kinds[0] == "REFERENCES_SYMBOL" && v.Kinds[1] == "RESOLVES_TO_DEFINITION") ||
			(v.Kinds[1] == "REFERENCES_SYMBOL" && v.Kinds[0] == "RESOLVES_TO_DEFINITION")) {
		return false
	}
	n, s := v.Nomination, v.Slot
	// 64 is the existing private all-mode schedule ceiling; no new schedule or
	// budget is authorized. Seed is compared exactly, not rewritten to seed7.
	if s.Mode != "RD" || s.Ordinal < 0 || s.Ordinal >= 64 || n.Mode != s.Mode || n.Slot != s.Ordinal || n.Seed != s.Seed || n.RunID != s.RunID || n.Community < 0 ||
		n.Status != "PROVISIONAL_STRUCTURAL_HYPOTHESIS" || n.ClaimCeiling != rg.EvaluationCeilingPrivate ||
		n.RejectionCondition != "SPLIT_OR_WITHDRAW_IF_ADMITTED_SOURCE_REVIEW_CONTRADICTS_MEMBERSHIP" || n.Gap != "" {
		return false
	}
	for _, x := range []string{s.SeedNamespace, s.RunID, n.ID, n.InputID, n.CaseID, n.PolicyID, n.ResultID, n.Lineage} {
		if !rdDescriptorStringV2(x, false) {
			return false
		}
	}
	// This descriptor selects an actual endpoint intent, not an empty/gap intent.
	// Both nil and empty slices are invalid here; neither is silently repaired.
	if len(n.Members) == 0 || len(n.Members) > rg.EvaluationMaxNodesPrivate || len(n.Endpoints) == 0 || len(n.Endpoints) > 2*rg.EvaluationMaxOccurrencesPrivate+rg.EvaluationMaxNodesPrivate {
		return false
	}
	for _, m := range n.Members {
		if !rdDescriptorStringV2(m, false) {
			return false
		}
	}
	for _, e := range n.Endpoints {
		if e.Kind != "REFERENCES_SYMBOL" && e.Kind != "RESOLVES_TO_DEFINITION" {
			return false
		}
		for _, x := range []string{e.Node, e.Kind, e.Occurrence, e.Role, e.Predecessor, e.Custody, e.Disposition} {
			if !rdDescriptorStringV2(x, true) {
				return false
			}
		}
	}
	// Reuse the existing pure intent contract for membership, local ordinal,
	// role, bounds and exact nomination/lineage/endpoint projection. Its detailed
	// errors never escape. Optional predecessor/custody/disposition stay optional.
	intents, err := tp.BuildPrivateSourceIntents(n, 2*rg.EvaluationMaxOccurrencesPrivate+rg.EvaluationMaxNodesPrivate)
	if err != nil {
		return false
	}
	matches := 0
	for _, i := range intents {
		if i == v.Intent {
			matches++
		}
	}
	return matches == 1
}

func rdDescriptorStringV2(s string, optional bool) bool {
	return (optional || s != "") && len(s) <= rg.EvaluationMaxStringPrivate && utf8.ValidString(s)
}

// matches is exact DATA correspondence, including original producer kind order,
// seed namespace, every nomination field and selected endpoint intent. Comparing
// two caller-supplied values is not independent validation or native admission.
func (d rdDescriptorV2) matches(v rdDescriptorInputV2) bool {
	return d.valid && validRDDescriptorInputV2(v) && reflect.DeepEqual(d.value, v)
}

// comparisonKinds normalizes a fixed-size COPY only. The original order remains
// bound separately in value.Kinds for later exact replay comparisons.
func (d rdDescriptorV2) comparisonKinds() [2]string {
	if !d.valid {
		return [2]string{}
	}
	k := [2]string{d.value.Kinds[0], d.value.Kinds[1]}
	if k[1] < k[0] {
		k[0], k[1] = k[1], k[0]
	}
	return k
}

func (d rdDescriptorV2) snapshot() rdDescriptorInputV2 {
	if !d.valid {
		return rdDescriptorInputV2{}
	}
	v := d.value
	v.Nomination.Members = slices.Clone(v.Nomination.Members)
	v.Nomination.Endpoints = slices.Clone(v.Nomination.Endpoints)
	return v
}
