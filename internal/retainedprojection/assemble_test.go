package retainedprojection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

type retainedTestBinding struct {
	Artifact string `json:"artifact"`
	Custody  string `json:"custody"`
}

func retainedIdentity(raw []byte) sourceobject.Identity {
	sum := sha256.Sum256(raw)
	return sourceobject.Identity{Digest: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: uint64(len(raw))}
}

func retainedRange(sl, sc, el, ec uint32) graph.Range {
	return graph.Range{Start: graph.Position{Line: sl, Character: sc}, End: graph.Position{Line: el, Character: ec}}
}

func retainedResolved() ResolveResult {
	raw := []byte("alpha beta\n")
	id := retainedIdentity(raw)
	mk := func(ordinal int, role, subject, uri string, display, item, selection graph.Range) ResolvedSelection {
		itemCopy, selectionCopy := item, selection
		return ResolvedSelection{Selection: Selection{
			Ordinal: ordinal, Role: role, Key: Key{GraphSubjectID: subject, LogicalSourceID: uri}, Source: id,
			PositionEncoding: "utf-16", ItemRange: &itemCopy, SelectionRange: &selectionCopy,
			DisplayRange: display, DisplayProvenance: v5sourcesnapshotv2.Provenance{Kind: "SERVER_REPORTED_DOCUMENT_SYMBOL", Method: "textDocument/documentSymbol"}, DisplayRangePolicy: "FULL_DEFINITION",
		}, Bytes: append([]byte(nil), raw...)}
	}
	return ResolveResult{Selections: []ResolvedSelection{
		mk(0, "TARGET", "target-subject", "file:///z.go", retainedRange(0, 0, 0, 10), retainedRange(0, 1, 0, 9), retainedRange(0, 2, 0, 7)),
		mk(1, "ADDITIONAL", "a-subject", "file:///a.go", retainedRange(0, 0, 0, 5), retainedRange(0, 0, 0, 5), retainedRange(0, 1, 0, 4)),
		mk(2, "ADDITIONAL", "b-subject", "file:///a.go", retainedRange(0, 6, 0, 10), retainedRange(0, 6, 0, 10), retainedRange(0, 7, 0, 9)),
		mk(3, "ADDITIONAL", "c-subject", "file:///b.go", retainedRange(0, 0, 0, 5), retainedRange(0, 0, 0, 5), retainedRange(0, 1, 0, 4)),
	}}
}

func retainedPolicy(body bool) sourceprojection.Policy {
	return sourceprojection.Policy{PolicyID: "retained-public", BodyRequested: body, MaxBytes: 1 << 20, MaxRanges: 32, MaxObjects: 32, MaxWork: 32, EnforceLimits: true}
}

func TestAssembleV2BoundedEndpointMappingDedupeAndCustody(t *testing.T) {
	const assertion = "ASSERT_RETAINED_ENDPOINT_MAPPING_DEDUPE_CUSTODY"
	resolved := retainedResolved()
	binding := retainedTestBinding{Artifact: "sha256:artifact", Custody: "caller-value"}
	got, err := AssembleV2Bounded(resolved, retainedPolicy(true), binding, "retained-public", 1<<20)
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	if got.CustodyMode != "RETAINED" || !reflect.DeepEqual(got.CustodyBinding, binding) {
		t.Fatalf("%s: custody=%q binding=%+v", assertion, got.CustodyMode, got.CustodyBinding)
	}
	if got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.GraphFactsAdded != 0 {
		t.Fatalf("%s: authority tuple=%d/%q/%d", assertion, got.Authority, got.SourceGraphComplete, got.GraphFactsAdded)
	}
	if want := []string{"file:///z.go", "file:///a.go", "file:///b.go"}; !reflect.DeepEqual(got.DocumentSelection.SelectedURIs, want) {
		t.Fatalf("%s: uris=%v", assertion, got.DocumentSelection.SelectedURIs)
	}
	if len(got.DocumentBindings) != 3 || got.DocumentBindings[0].DocumentVersion != 0 || got.DocumentAccounting.Candidates != 3 || got.DocumentAccounting.TotalAcquiredBytes != len(resolved.Selections[0].Bytes) {
		t.Fatalf("%s: bindings=%+v accounting=%+v", assertion, got.DocumentBindings, got.DocumentAccounting)
	}
	if len(got.Units) != 4 || len(got.Citations) != 4 {
		t.Fatalf("%s: units=%d citations=%d", assertion, len(got.Units), len(got.Citations))
	}
	wantBodies := map[string]string{"target-subject": "alpha beta", "a-subject": "alpha", "b-subject": "beta", "c-subject": "alpha"}
	for i, unit := range got.Units {
		if unit.Role != "ENDPOINT" || unit.RelationProvenance != "" || unit.PrivacyClassification != "PUBLIC" || unit.ItemRange == nil || unit.SelectionRange == nil {
			t.Fatalf("%s[%d]: %+v", assertion, i, unit)
		}
		if unit.EvidenceRange != *unit.SelectionRange {
			t.Fatalf("%s[%d]: evidence=%+v selection=%+v", assertion, i, unit.EvidenceRange, unit.SelectionRange)
		}
		if unit.BodyDisposition != "RETURNED" || unit.Body != wantBodies[unit.GraphSubjectID] {
			t.Fatalf("ASSERT_RETAINED_BODY_INCLUDE_EXACT[%d]: subject=%q disposition=%q body=%q want=%q", i, unit.GraphSubjectID, unit.BodyDisposition, unit.Body, wantBodies[unit.GraphSubjectID])
		}
	}
	if got.Units[0].DisplayRange == got.Units[0].EvidenceRange || *got.Units[0].ItemRange == got.Units[0].EvidenceRange {
		t.Fatalf("%s: distinct ranges collapsed: %+v", assertion, got.Units[0])
	}
}

func TestAssembleV2BoundedMetadataOnlyDeterminismAndExactBoundary(t *testing.T) {
	const assertion = "ASSERT_RETAINED_METADATA_DETERMINISM_BOUNDARY"
	resolved := retainedResolved()
	binding := retainedTestBinding{Custody: "opaque"}
	got, err := AssembleV2Bounded(resolved, retainedPolicy(false), binding, "retained-public", 1<<20)
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	for _, unit := range got.Units {
		if unit.Body != "" || unit.BodyDisposition != "NOT_REQUESTED" {
			t.Fatalf("%s: unit=%+v", assertion, unit)
		}
	}
	again, err := AssembleV2Bounded(resolved, retainedPolicy(false), binding, "retained-public", 1<<20)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("%s: deterministic=%v err=%v", assertion, reflect.DeepEqual(got, again), err)
	}
	raw, _ := json.Marshal(got)
	if _, err := AssembleV2Bounded(resolved, retainedPolicy(false), binding, "retained-public", len(raw)); err != nil {
		t.Fatalf("%s: exact rejected: %v", assertion, err)
	}
	limited, err := AssembleV2Bounded(resolved, retainedPolicy(false), binding, "retained-public", len(raw)-1)
	if err == nil || !reflect.DeepEqual(limited, reflect.Zero(reflect.TypeOf(limited)).Interface()) || !IsCode(err, CodeAssemblyFailed) {
		t.Fatalf("%s: limited=%+v err=%v", assertion, limited, err)
	}
}

func TestAssembleV2BoundedRejectsInvalidInputAtomically(t *testing.T) {
	const assertion = "ASSERT_RETAINED_INVALID_ATOMIC_TYPED"
	base := retainedResolved()
	cases := []struct {
		name   string
		mutate func(*ResolveResult, *sourceprojection.Policy, *string, *int)
		code   Code
	}{
		{"empty", func(r *ResolveResult, _ *sourceprojection.Policy, _ *string, _ *int) { *r = ResolveResult{} }, CodeInvalidResolveResult},
		{"nil-item", func(r *ResolveResult, _ *sourceprojection.Policy, _ *string, _ *int) {
			r.Selections[0].Selection.ItemRange = nil
		}, CodeInvalidResolveResult},
		{"encoding", func(r *ResolveResult, _ *sourceprojection.Policy, _ *string, _ *int) {
			r.Selections[0].Selection.PositionEncoding = "utf-8"
		}, CodeUnsupportedEncoding},
		{"policy-empty", func(_ *ResolveResult, _ *sourceprojection.Policy, id *string, _ *int) { *id = "" }, CodeInvalidRequest},
		{"policy-mismatch", func(_ *ResolveResult, _ *sourceprojection.Policy, id *string, _ *int) { *id = "other" }, CodeInvalidRequest},
		{"bound", func(_ *ResolveResult, _ *sourceprojection.Policy, _ *string, n *int) { *n = 0 }, CodeInvalidRequest},
		{"negative-max-bytes", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) { p.MaxBytes = -1 }, CodeInvalidRequest},
		{"negative-max-ranges", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) { p.MaxRanges = -1 }, CodeInvalidRequest},
		{"negative-max-objects", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) { p.MaxObjects = -1 }, CodeInvalidRequest},
		{"negative-max-work", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) { p.MaxWork = -1 }, CodeInvalidRequest},
		{"negative-max-bytes-unenforced", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) {
			p.EnforceLimits = false
			p.MaxBytes = -1
		}, CodeInvalidRequest},
		{"negative-max-ranges-unenforced", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) {
			p.EnforceLimits = false
			p.MaxRanges = -1
		}, CodeInvalidRequest},
		{"negative-max-objects-unenforced", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) {
			p.EnforceLimits = false
			p.MaxObjects = -1
		}, CodeInvalidRequest},
		{"negative-max-work-unenforced", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) {
			p.EnforceLimits = false
			p.MaxWork = -1
		}, CodeInvalidRequest},
		{"digest", func(r *ResolveResult, _ *sourceprojection.Policy, _ *string, _ *int) { r.Selections[0].Bytes[0] = 'X' }, CodeSourceMismatch},
		{"logical-mismatch", func(r *ResolveResult, _ *sourceprojection.Policy, _ *string, _ *int) {
			r.Selections[2].Selection.Source = retainedIdentity([]byte("different"))
		}, CodeSourceMismatch},
		{"inverted-range", func(r *ResolveResult, _ *sourceprojection.Policy, _ *string, _ *int) {
			r.Selections[0].Selection.DisplayRange = retainedRange(1, 0, 0, 0)
		}, CodeIncompatibleRange},
		{"noncanonical-order", func(r *ResolveResult, _ *sourceprojection.Policy, _ *string, _ *int) {
			r.Selections[1], r.Selections[2] = r.Selections[2], r.Selections[1]
			r.Selections[1].Selection.Ordinal, r.Selections[2].Selection.Ordinal = 1, 2
		}, CodeInvalidResolveResult},
		{"projection", func(_ *ResolveResult, p *sourceprojection.Policy, _ *string, _ *int) { p.MaxWork = 0 }, CodeProjectionFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := cloneResolvedForTest(base)
			p := retainedPolicy(true)
			id := p.PolicyID
			n := 1 << 20
			tc.mutate(&r, &p, &id, &n)
			got, err := AssembleV2Bounded(r, p, retainedTestBinding{}, id, n)
			var typed *AssemblyError
			if err == nil || !IsCode(err, tc.code) || !errors.As(err, &typed) || !reflect.DeepEqual(got, reflect.Zero(reflect.TypeOf(got)).Interface()) {
				t.Fatalf("%s/%s: got=%+v err=%T %v", assertion, tc.name, got, err, err)
			}
			if tc.code == CodeProjectionFailed && (typed.Cause == nil || !errors.Is(err, typed.Cause)) {
				t.Fatalf("%s/%s: cause is not unwrap-visible: %+v", assertion, tc.name, typed)
			}
			if typed.Key != (Key{}) && !strings.Contains(err.Error(), typed.Key.GraphSubjectID+"\x00"+typed.Key.LogicalSourceID) {
				t.Fatalf("%s/%s: key not deterministically formatted: %v", assertion, tc.name, err)
			}
		})
	}
}

func TestAssembleV2BoundedRejectsDisplayProvenanceSubstitutionAtomically(t *testing.T) {
	cases := []struct {
		name      string
		assertion string
		mutate    func(*Selection)
	}{
		{"kind", "ASSERT_RETAINED_PROVENANCE_KIND_SUBSTITUTION", func(s *Selection) { s.DisplayProvenance.Kind = "OTHER" }},
		{"method", "ASSERT_RETAINED_PROVENANCE_METHOD_SUBSTITUTION", func(s *Selection) { s.DisplayProvenance.Method = "other" }},
		{"policy", "ASSERT_RETAINED_DISPLAY_POLICY_SUBSTITUTION", func(s *Selection) { s.DisplayRangePolicy = "OTHER" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved := retainedResolved()
			tc.mutate(&resolved.Selections[0].Selection)
			got, err := AssembleV2Bounded(resolved, retainedPolicy(false), retainedTestBinding{}, "retained-public", 1<<20)
			if err == nil || !IsCode(err, CodeInvalidProvenance) || !reflect.DeepEqual(got, reflect.Zero(reflect.TypeOf(got)).Interface()) {
				t.Fatalf("%s: got=%+v err=%T %v", tc.assertion, got, err, err)
			}
		})
	}
}

func cloneResolvedForTest(in ResolveResult) ResolveResult {
	raw, _ := json.Marshal(in)
	var out ResolveResult
	_ = json.Unmarshal(raw, &out)
	return out
}

func TestAssembleV2BoundedIDsAreIdentityComplete(t *testing.T) {
	const assertion = "ASSERT_RETAINED_IDENTITY_COMPLETE_IDS"
	a := retainedResolved()
	b := cloneResolvedForTest(a)
	first, err := AssembleV2Bounded(a, retainedPolicy(false), retainedTestBinding{}, "retained-public", 1<<20)
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	b.Selections[1].Selection.DisplayRange = retainedRange(0, 0, 0, 4)
	b.Selections[1].Selection.ItemRange = func() *graph.Range { v := retainedRange(0, 0, 0, 4); return &v }()
	b.Selections[1].Selection.SelectionRange = func() *graph.Range { v := retainedRange(0, 1, 0, 3); return &v }()
	second, err := AssembleV2Bounded(b, retainedPolicy(false), retainedTestBinding{}, "retained-public", 1<<20)
	if err != nil {
		t.Fatalf("%s changed: %v", assertion, err)
	}
	firstIDs, secondIDs := map[string]string{}, map[string]string{}
	for _, unit := range first.Units {
		firstIDs[unit.GraphSubjectID] = unit.UnitID
	}
	for _, unit := range second.Units {
		secondIDs[unit.GraphSubjectID] = unit.UnitID
	}
	if firstIDs["target-subject"] != secondIDs["target-subject"] || firstIDs["a-subject"] == secondIDs["a-subject"] || !strings.HasPrefix(firstIDs["target-subject"], "sha256:") {
		t.Fatalf("%s: first=%+v second=%+v", assertion, firstIDs, secondIDs)
	}
}
