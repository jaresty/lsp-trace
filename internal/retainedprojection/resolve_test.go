package retainedprojection

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceobject"
)

type lookupFunc func(sourceobject.Identity) (sourceobject.Object, error)

func (f lookupFunc) Get(id sourceobject.Identity) (sourceobject.Object, error) { return f(id) }

func object(raw string) sourceobject.Object {
	b := []byte(raw)
	sum := sha256.Sum256(b)
	id := sourceobject.Identity{Digest: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: uint64(len(b))}
	return sourceobject.Object{Identity: id, Bytes: b}
}

func resolvePlan(objects ...sourceobject.Object) Plan {
	selections := make([]Selection, len(objects))
	for i, o := range objects {
		selections[i] = Selection{Ordinal: i, Role: "ADDITIONAL", Key: Key{GraphSubjectID: string(rune('a' + i)), LogicalSourceID: "file:///source.go"}, Source: o.Identity}
	}
	if len(selections) > 0 {
		selections[0].Role = "TARGET"
	}
	p := Plan{Ordering: Ordering, Selections: selections}
	if len(selections) > 0 {
		p.Target = selections[0].Key
	}
	return p
}

func generousLimits() ResolveLimits {
	return ResolveLimits{MaxDistinctObjects: 16, MaxUniqueSourceBytes: 1 << 20, MaxLogicalSelections: 16}
}

func TestResolveExactOnceDuplicateReuseAndOrder(t *testing.T) {
	a, b := object("alpha"), object("bravo")
	p := resolvePlan(a, b, a)
	p.Selections[2].Key = Key{GraphSubjectID: "duplicate-a", LogicalSourceID: "file:///source.go"}
	calls := map[sourceobject.Identity]int{}
	lookup := lookupFunc(func(id sourceobject.Identity) (sourceobject.Object, error) {
		calls[id]++
		if id == a.Identity {
			return a, nil
		}
		if id == b.Identity {
			return b, nil
		}
		return sourceobject.Object{}, errors.New("unexpected identity")
	})
	got, err := Resolve(p, lookup, generousLimits())
	if err != nil {
		t.Fatalf("ASSERT_RESOLVE_DUPLICATE_REUSE_ORDER: %v", err)
	}
	if calls[a.Identity] != 1 || calls[b.Identity] != 1 {
		t.Fatalf("ASSERT_RESOLVE_ONE_GET_PER_DISTINCT_IDENTITY: calls=%v", calls)
	}
	want := [][]byte{[]byte("alpha"), []byte("bravo"), []byte("alpha")}
	if len(got.Selections) != 3 {
		t.Fatalf("ASSERT_RESOLVE_PRESERVES_LOGICAL_DUPLICATES: got=%d", len(got.Selections))
	}
	for i := range want {
		if got.Selections[i].Selection.Key != p.Selections[i].Key || !bytes.Equal(got.Selections[i].Bytes, want[i]) {
			t.Fatalf("ASSERT_RESOLVE_PLAN_SELECTION_ORDER[%d]: got=%+v", i, got.Selections[i])
		}
	}
}

func TestResolveDefensiveCopies(t *testing.T) {
	o := object("mutable")
	p := resolvePlan(o, o)
	p.Selections[1].Key.GraphSubjectID = "duplicate"
	storeBytes := o.Bytes
	got, err := Resolve(p, lookupFunc(func(sourceobject.Identity) (sourceobject.Object, error) { return o, nil }), generousLimits())
	if err != nil {
		t.Fatalf("ASSERT_RESOLVE_COPY_OWNERSHIP: %v", err)
	}
	got.Selections[0].Bytes[0] = 'X'
	if string(storeBytes) != "mutable" || string(got.Selections[1].Bytes) != "mutable" {
		t.Fatalf("ASSERT_RESOLVE_NO_MUTABLE_ALIASES: store=%q duplicate=%q", storeBytes, got.Selections[1].Bytes)
	}
}

func TestResolveSelectionMetadataIsDeeplyIsolated(t *testing.T) {
	o := object("metadata")
	p := resolvePlan(o, o)
	p.Selections[1].Key.GraphSubjectID = "duplicate-source"
	item := graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 2}}
	selection := graph.Range{Start: graph.Position{Line: 1, Character: 1}, End: graph.Position{Line: 1, Character: 2}}
	callSite := graph.Range{Start: graph.Position{Line: 3}, End: graph.Position{Line: 4}}
	for i := range p.Selections {
		itemCopy, selectionCopy, callSiteCopy := item, selection, callSite
		p.Selections[i].EvidenceRanges = []EvidenceRange{{RelationID: "relation", Range: itemCopy}}
		p.Selections[i].ItemRange = &itemCopy
		p.Selections[i].SelectionRange = &selectionCopy
		p.Selections[i].CallSiteRange = &callSiteCopy
	}
	got, err := Resolve(p, lookupFunc(func(sourceobject.Identity) (sourceobject.Object, error) { return o, nil }), generousLimits())
	if err != nil {
		t.Fatalf("ASSERT_RESOLVE_METADATA_CLONE_SETUP: %v", err)
	}

	p.Selections[0].EvidenceRanges[0].RelationID = "input-mutated"
	p.Selections[0].ItemRange.Start.Line = 99
	p.Selections[0].SelectionRange.Start.Line = 99
	p.Selections[0].CallSiteRange.Start.Line = 99
	if got.Selections[0].Selection.EvidenceRanges[0].RelationID != "relation" || got.Selections[0].Selection.ItemRange.Start.Line != 1 || got.Selections[0].Selection.SelectionRange.Start.Line != 1 || got.Selections[0].Selection.CallSiteRange.Start.Line != 3 {
		t.Fatalf("ASSERT_RESOLVE_METADATA_NO_INPUT_ALIAS: %+v", got.Selections[0].Selection)
	}

	got.Selections[0].Selection.EvidenceRanges[0].RelationID = "output-mutated"
	got.Selections[0].Selection.ItemRange.Start.Line = 77
	got.Selections[0].Selection.SelectionRange.Start.Line = 77
	got.Selections[0].Selection.CallSiteRange.Start.Line = 77
	if got.Selections[1].Selection.EvidenceRanges[0].RelationID != "relation" || got.Selections[1].Selection.ItemRange.Start.Line != 1 || got.Selections[1].Selection.SelectionRange.Start.Line != 1 || got.Selections[1].Selection.CallSiteRange.Start.Line != 3 {
		t.Fatalf("ASSERT_RESOLVE_METADATA_NO_OUTPUT_ALIAS: %+v", got.Selections[1].Selection)
	}
	if p.Selections[1].EvidenceRanges[0].RelationID != "relation" || p.Selections[1].ItemRange.Start.Line != 1 || p.Selections[1].SelectionRange.Start.Line != 1 || p.Selections[1].CallSiteRange.Start.Line != 3 {
		t.Fatalf("ASSERT_RESOLVE_METADATA_OUTPUT_CANNOT_MUTATE_INPUT: %+v", p.Selections[1])
	}
}

func TestResolveRejectsNonCanonicalPlanKeysBeforeLookup(t *testing.T) {
	a, b, c := object("a"), object("b"), object("c")
	base := resolvePlan(a, b, c)
	base.Selections[0].Key = Key{"target", "file:///target.go"}
	base.Target = base.Selections[0].Key
	base.Selections[1].Key = Key{"a", "file:///a.go"}
	base.Selections[2].Key = Key{"b", "file:///b.go"}

	tests := []struct {
		name   string
		mutate func(*Plan)
	}{
		{"out-of-order-additional", func(p *Plan) {
			p.Selections[1], p.Selections[2] = p.Selections[2], p.Selections[1]
			p.Selections[1].Ordinal, p.Selections[2].Ordinal = 1, 2
		}},
		{"duplicate-additional", func(p *Plan) { p.Selections[2].Key = p.Selections[1].Key }},
		{"duplicate-target", func(p *Plan) { p.Selections[2].Key = p.Target }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			p.Selections = append([]Selection(nil), base.Selections...)
			tc.mutate(&p)
			calls := 0
			got, err := Resolve(p, lookupFunc(func(sourceobject.Identity) (sourceobject.Object, error) {
				calls++
				return sourceobject.Object{}, errors.New("must not lookup")
			}), generousLimits())
			if !reflect.DeepEqual(got, ResolveResult{}) || !IsCode(err, CodeInvalidPlan) || calls != 0 {
				t.Fatalf("ASSERT_RESOLVE_CANONICAL_KEYS_%s_PRELOOKUP: got=%+v calls=%d err=%T %v", tc.name, got, calls, err, err)
			}
		})
	}
}

func TestResolveRejectsIdentityAndByteMismatchesAtomically(t *testing.T) {
	good, other := object("good"), object("other")
	tests := []struct {
		name     string
		code     Code
		returned sourceobject.Object
	}{
		{"returned-identity", CodeReturnedIdentityMismatch, sourceobject.Object{Identity: other.Identity, Bytes: append([]byte(nil), good.Bytes...)}},
		{"actual-length", CodeResolvedLengthMismatch, sourceobject.Object{Identity: good.Identity, Bytes: []byte("bad")}},
		{"digest", CodeResolvedDigestMismatch, sourceobject.Object{Identity: good.Identity, Bytes: []byte("evil")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(resolvePlan(good), lookupFunc(func(sourceobject.Identity) (sourceobject.Object, error) { return tc.returned, nil }), generousLimits())
			if !reflect.DeepEqual(got, ResolveResult{}) || !IsCode(err, tc.code) {
				t.Fatalf("ASSERT_RESOLVE_TYPED_%s_ATOMIC_ZERO: got=%+v err=%T %v", tc.code, got, err, err)
			}
		})
	}
}

func TestResolvePreservesSourceObjectCodes(t *testing.T) {
	o := object("source")
	codes := []sourceobject.Code{sourceobject.CodeMissing, sourceobject.CodePolicy, sourceobject.CodePermission, sourceobject.CodeLimit, sourceobject.CodeCorrupt, sourceobject.CodeInvalidIdentity}
	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			sourceErr := &sourceobject.Error{Code: code}
			got, err := Resolve(resolvePlan(o), lookupFunc(func(sourceobject.Identity) (sourceobject.Object, error) { return sourceobject.Object{}, sourceErr }), generousLimits())
			if !reflect.DeepEqual(got, ResolveResult{}) || !sourceobject.IsCode(err, code) {
				t.Fatalf("ASSERT_RESOLVE_PRESERVES_SOURCEOBJECT_%s: got=%+v err=%T %v", code, got, err, err)
			}
		})
	}
}

func TestResolveLimitsAreTypedAndAtomic(t *testing.T) {
	a, b := object("aaaa"), object("bb")
	pDuplicate := resolvePlan(a, a)
	pDuplicate.Selections[1].Key.GraphSubjectID = "duplicate"
	tests := []struct {
		name   string
		plan   Plan
		limits ResolveLimits
		code   Code
	}{
		{"distinct", resolvePlan(a, b), ResolveLimits{MaxDistinctObjects: 1, MaxUniqueSourceBytes: 100, MaxLogicalSelections: 10}, CodeResolveDistinctLimit},
		{"unique-bytes", resolvePlan(a, b), ResolveLimits{MaxDistinctObjects: 10, MaxUniqueSourceBytes: 5, MaxLogicalSelections: 10}, CodeResolveSourceBytesLimit},
		{"logical", pDuplicate, ResolveLimits{MaxDistinctObjects: 10, MaxUniqueSourceBytes: 100, MaxLogicalSelections: 1}, CodeResolveSelectionLimit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got, err := Resolve(tc.plan, lookupFunc(func(id sourceobject.Identity) (sourceobject.Object, error) {
				calls++
				if id == a.Identity {
					return a, nil
				}
				return b, nil
			}), tc.limits)
			if !reflect.DeepEqual(got, ResolveResult{}) || !IsCode(err, tc.code) {
				t.Fatalf("ASSERT_RESOLVE_LIMIT_%s_ATOMIC_ZERO: got=%+v err=%T %v", tc.code, got, err, err)
			}
			if calls != 0 {
				t.Fatalf("ASSERT_RESOLVE_LIMIT_%s_PRELOOKUP: calls=%d", tc.code, calls)
			}
		})
	}
}

func TestResolveMalformedPlansAreTypedAndPrelookup(t *testing.T) {
	o := object("valid")
	badDigest := resolvePlan(o)
	badDigest.Selections[0].Source.Digest = "SHA256:not-canonical"
	cases := []Plan{
		{},
		{Ordering: "wrong", Target: Key{"a", "file:///source.go"}, Selections: resolvePlan(o).Selections},
		func() Plan { p := resolvePlan(o); p.Selections[0].Ordinal = 1; return p }(),
		func() Plan { p := resolvePlan(o); p.Selections[0].Role = "ADDITIONAL"; return p }(),
		badDigest,
	}
	for i, p := range cases {
		calls := 0
		got, err := Resolve(p, lookupFunc(func(sourceobject.Identity) (sourceobject.Object, error) { calls++; return o, nil }), generousLimits())
		if !reflect.DeepEqual(got, ResolveResult{}) || !IsCode(err, CodeInvalidPlan) || calls != 0 {
			t.Fatalf("ASSERT_RESOLVE_INVALID_PLAN_ATOMIC_ZERO[%d]: got=%+v calls=%d err=%T %v", i, got, calls, err, err)
		}
	}
}

func TestResolveLateFailureReturnsZeroWithoutRefetch(t *testing.T) {
	a, b := object("first"), object("second")
	calls := map[sourceobject.Identity]int{}
	got, err := Resolve(resolvePlan(a, b), lookupFunc(func(id sourceobject.Identity) (sourceobject.Object, error) {
		calls[id]++
		if id == a.Identity {
			return a, nil
		}
		return sourceobject.Object{}, &sourceobject.Error{Code: sourceobject.CodeMissing}
	}), generousLimits())
	if !reflect.DeepEqual(got, ResolveResult{}) || !sourceobject.IsCode(err, sourceobject.CodeMissing) {
		t.Fatalf("ASSERT_RESOLVE_LATE_FAILURE_ATOMIC_ZERO: got=%+v err=%T %v", got, err, err)
	}
	if calls[a.Identity] != 1 || calls[b.Identity] != 1 {
		t.Fatalf("ASSERT_RESOLVE_LATE_FAILURE_EXACT_ONCE: calls=%v", calls)
	}
}
