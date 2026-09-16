package retainedprojection

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

func TestSelectTargetFirstCanonicalAndIdentityComplete(t *testing.T) {
	admitted := fixture()
	request := Request{
		Target: Key{GraphSubjectID: "target", LogicalSourceID: "file:///target.go"},
		Selections: []Key{
			{GraphSubjectID: "z", LogicalSourceID: "file:///z.go"},
			{GraphSubjectID: "target", LogicalSourceID: "file:///target.go"},
			{GraphSubjectID: "a", LogicalSourceID: "file:///a.go"},
		},
	}
	plan, err := Select(admitted, request)
	if err != nil {
		t.Fatalf("ASSERT_SELECT_ACCEPTS_ADMITTED_V2: %v", err)
	}
	wantKeys := []Key{
		{GraphSubjectID: "target", LogicalSourceID: "file:///target.go"},
		{GraphSubjectID: "a", LogicalSourceID: "file:///a.go"},
		{GraphSubjectID: "z", LogicalSourceID: "file:///z.go"},
	}
	gotKeys := make([]Key, len(plan.Selections))
	for i := range plan.Selections {
		gotKeys[i] = plan.Selections[i].Key
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("ASSERT_SELECT_TARGET_FIRST_LOGICAL_ORDER: got=%v want=%v", gotKeys, wantKeys)
	}
	selected := plan.Selections[0]
	if selected.ReceiptID != "receipt-target" || selected.Source.Digest != digestT || selected.Source.ByteLength != 6 {
		t.Fatalf("ASSERT_SELECT_EXACT_RESOLVER_IDENTITY: %+v", selected)
	}
	if selected.PositionEncoding != "utf-16" {
		t.Fatalf("ASSERT_SELECT_ENCODING_RETAINED: %q", selected.PositionEncoding)
	}
	if selected.ItemRange == nil || selected.SelectionRange == nil || selected.CallSiteRange != nil {
		t.Fatalf("ASSERT_SELECT_RANGE_ROLES_DISTINCT: %+v", selected)
	}
	if selected.DisplayRange == *selected.ItemRange {
		t.Fatalf("ASSERT_SELECT_DISPLAY_NOT_CONFLATED: display=%+v item=%+v", selected.DisplayRange, *selected.ItemRange)
	}
	if selected.DisplayProvenance.Kind != v5sourcesnapshotv2.ProvenanceKind || selected.DisplayProvenance.Method != v5sourcesnapshotv2.ProvenanceMethod {
		t.Fatalf("ASSERT_SELECT_EXPLICIT_DISPLAY_PROVENANCE: %+v", selected.DisplayProvenance)
	}
	if len(selected.EvidenceRanges) != 2 || selected.EvidenceRanges[0].Pointer == selected.EvidenceRanges[1].Pointer {
		t.Fatalf("ASSERT_SELECT_EXACT_RETAINED_EVIDENCE_RANGES: %+v", selected.EvidenceRanges)
	}
}

func TestSelectPermutationInvariantPlanBytes(t *testing.T) {
	admitted := fixture()
	keys := []Key{{"target", "file:///target.go"}, {"a", "file:///a.go"}, {"z", "file:///z.go"}}
	permutations := [][]Key{
		{keys[0], keys[1], keys[2]}, {keys[2], keys[0], keys[1]}, {keys[1], keys[2], keys[0]},
	}
	var baseline []byte
	for i, selections := range permutations {
		plan, err := Select(admitted, Request{Target: keys[0], Selections: selections})
		if err != nil {
			t.Fatalf("ASSERT_SELECT_PERMUTATION_ACCEPTED[%d]: %v", i, err)
		}
		raw, err := plan.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			baseline = raw
		} else if string(raw) != string(baseline) {
			t.Fatalf("ASSERT_SELECT_PERMUTATION_IDENTICAL_BYTES[%d]:\n%s\n%s", i, baseline, raw)
		}
	}
}

func TestSelectTerminalFailuresReturnZeroPlan(t *testing.T) {
	base := fixture()
	tests := []struct {
		name   string
		code   Code
		mutate func(*Admitted, *Request)
	}{
		{"missing-subject", CodeMissingBinding, func(_ *Admitted, r *Request) { r.Selections[1].GraphSubjectID = "missing" }},
		{"missing-display", CodeMissingDisplay, func(a *Admitted, _ *Request) { a.artifact.DisplayBindings = a.artifact.DisplayBindings[1:] }},
		{"duplicate-selection", CodeAmbiguousSelection, func(_ *Admitted, r *Request) { r.Selections = append(r.Selections, r.Selections[0]) }},
		{"duplicate-display", CodeAmbiguousSelection, func(a *Admitted, _ *Request) {
			a.artifact.DisplayBindings = append(a.artifact.DisplayBindings, a.artifact.DisplayBindings[0])
		}},
		{"receipt-mismatch", CodeReceiptMismatch, func(a *Admitted, _ *Request) { a.artifact.DisplayBindings[0].ReceiptID = "receipt-a" }},
		{"source-mismatch", CodeSourceMismatch, func(a *Admitted, _ *Request) { a.artifact.DisplayBindings[0].SourceDigest = digestA }},
		{"inverted-range", CodeInvalidRange, func(a *Admitted, _ *Request) { a.artifact.DisplayBindings[0].DisplayRange = rg(3, 0, 2, 0) }},
		{"incompatible-range", CodeIncompatibleRange, func(a *Admitted, _ *Request) { a.parent.Bindings[0].Range = rg(9, 0, 10, 0) }},
		{"unsupported-encoding", CodeUnsupportedEncoding, func(a *Admitted, _ *Request) { a.artifact.DisplayBindings[0].PositionEncoding = "utf-7" }},
		{"invalid-provenance", CodeInvalidProvenance, func(a *Admitted, _ *Request) { a.artifact.DisplayBindings[0].Provenance.Method = "callHierarchy" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			admitted := cloneAdmitted(t, base)
			request := Request{Target: Key{"target", "file:///target.go"}, Selections: []Key{{"target", "file:///target.go"}, {"a", "file:///a.go"}}}
			tc.mutate(&admitted, &request)
			plan, err := Select(admitted, request)
			if !reflect.DeepEqual(plan, Plan{}) {
				t.Fatalf("ASSERT_SELECT_NO_PARTIAL_PLAN_%s: %+v", tc.code, plan)
			}
			var typed *Error
			if !errors.As(err, &typed) || typed.Code != tc.code {
				t.Fatalf("ASSERT_SELECT_TYPED_TERMINAL_%s: %T %v", tc.code, err, err)
			}
		})
	}
}

func TestSelectAmbiguityErrorPermutationInvariant(t *testing.T) {
	admitted := fixture()
	requests := []Request{
		{Target: Key{"target", "file:///target.go"}, Selections: []Key{{"target", "file:///target.go"}, {"a", "file:///a.go"}, {"a", "file:///a.go"}}},
		{Target: Key{"target", "file:///target.go"}, Selections: []Key{{"a", "file:///a.go"}, {"target", "file:///target.go"}, {"a", "file:///a.go"}}},
	}
	var baseline string
	for i, request := range requests {
		plan, err := Select(admitted, request)
		if !reflect.DeepEqual(plan, Plan{}) || err == nil {
			t.Fatalf("ASSERT_SELECT_AMBIGUITY_ZERO[%d]: plan=%+v err=%v", i, plan, err)
		}
		if i == 0 {
			baseline = err.Error()
		} else if err.Error() != baseline {
			t.Fatalf("ASSERT_SELECT_AMBIGUITY_IDENTICAL_ERROR: %q != %q", err, baseline)
		}
	}
}

func fixture() Admitted {
	parent := v5sourcesnapshot.Artifact{
		PositionEncoding: "utf-16",
		Receipts: []v5sourcesnapshot.Receipt{
			{ID: "receipt-target", URI: "file:///target.go", ContentDigest: digestT, Content: []byte("target")},
			{ID: "receipt-a", URI: "file:///a.go", ContentDigest: digestA, Content: []byte("aaa")},
			{ID: "receipt-z", URI: "file:///z.go", ContentDigest: digestZ, Content: []byte("zzzz")},
		},
	}
	for _, row := range []struct{ id, uri, receipt, digest string }{{"target", "file:///target.go", "receipt-target", digestT}, {"a", "file:///a.go", "receipt-a", digestA}, {"z", "file:///z.go", "receipt-z", digestZ}} {
		parent.Bindings = append(parent.Bindings,
			v5sourcesnapshot.Binding{RelationID: "relation-" + row.id, EndpointRole: "DECLARATION", RangeRole: "DECLARATION_RANGE", Pointer: "/" + row.id + "/range", NodeID: row.id, URI: row.uri, Range: rg(1, 0, 3, 0), SourceDigest: row.digest, ReceiptIDs: []string{row.receipt}, Status: "RETAINED_BYTES"},
			v5sourcesnapshot.Binding{RelationID: "relation-" + row.id, EndpointRole: "DECLARATION", RangeRole: "SELECTION_RANGE", Pointer: "/" + row.id + "/selection_range", NodeID: row.id, URI: row.uri, Range: rg(1, 5, 1, 8), SourceDigest: row.digest, ReceiptIDs: []string{row.receipt}, Status: "RETAINED_BYTES"},
		)
	}
	artifact := v5sourcesnapshotv2.Artifact{DisplayBindings: []v5sourcesnapshotv2.DisplayBinding{}}
	for _, row := range []struct{ id, uri, receipt, digest string }{{"target", "file:///target.go", "receipt-target", digestT}, {"a", "file:///a.go", "receipt-a", digestA}, {"z", "file:///z.go", "receipt-z", digestZ}} {
		artifact.DisplayBindings = append(artifact.DisplayBindings, v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: row.id, LogicalSourceID: row.uri, DisplayRange: rg(0, 0, 4, 0), DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: row.receipt, SourceDigest: row.digest, PositionEncoding: "utf-16", Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody})
	}
	return Admitted{artifact: artifact, parent: parent}
}

func cloneAdmitted(t *testing.T, in Admitted) Admitted {
	t.Helper()
	var out struct {
		Artifact v5sourcesnapshotv2.Artifact `json:"artifact"`
		Parent   v5sourcesnapshot.Artifact   `json:"parent"`
	}
	wrapped, _ := json.Marshal(struct {
		Artifact v5sourcesnapshotv2.Artifact `json:"artifact"`
		Parent   v5sourcesnapshot.Artifact   `json:"parent"`
	}{in.artifact, in.parent})
	if err := json.Unmarshal(wrapped, &out); err != nil {
		t.Fatal(err)
	}
	return Admitted{artifact: out.Artifact, parent: out.Parent}
}

func rg(sl, sc, el, ec uint32) graph.Range {
	return graph.Range{Start: graph.Position{Line: sl, Character: sc}, End: graph.Position{Line: el, Character: ec}}
}

const (
	digestT = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestA = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	digestZ = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)
