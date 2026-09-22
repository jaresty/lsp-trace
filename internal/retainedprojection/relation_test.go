package retainedprojection

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

func relationFixture(t *testing.T) (Admitted, Request, RelationSelection) {
	t.Helper()
	a := fixture()
	caller := Key{"a", "file:///a.go"}
	callee := Key{"target", "file:///target.go"}
	r := rg(2, 3, 2, 8)
	receipt := v5sourcesnapshotv3.Receipt{ID: "relation-receipt", URI: caller.LogicalSourceID, ContentDigest: digestA, Content: []byte("aaa")}
	binding := v5sourcesnapshotv3.Binding{OccurrenceID: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", RelationID: "calls-a-target", Direction: v5sourcesnapshotv3.Direction, CallerNodeID: caller.GraphSubjectID, CalleeNodeID: callee.GraphSubjectID, CallerURI: caller.LogicalSourceID, Range: r, CanonicalOrdinal: 0, ReceiptID: receipt.ID, SourceDigest: receipt.ContentDigest, Provenance: v5sourcesnapshotv3.Provenance, Status: v5sourcesnapshotv3.Status, Custody: v5sourcesnapshotv3.Custody, Authority: 0, Accepted: false, Completeness: v5sourcesnapshotv3.Completeness}
	a.v3 = &v5sourcesnapshotv3.Artifact{Receipts: []v5sourcesnapshotv3.Receipt{receipt}, Bindings: []v5sourcesnapshotv3.Binding{binding}}
	a.raw = []byte("exact-v3-artifact")
	a.parent.GraphV5Bytes = []byte("graph")
	a.parent.GraphV5Digest = custodyDigest(a.parent.GraphV5Bytes)
	request := Request{Target: callee, Selections: []Key{callee, caller}, Relations: []RelationSelector{{RelationID: binding.RelationID, Caller: caller, Callee: callee, Range: r}}}
	selection := RelationSelection{Ordinal: 0, RelationID: binding.RelationID, OccurrenceID: binding.OccurrenceID, Caller: caller, Callee: callee, ReceiptID: receipt.ID, Source: sourceobject.Identity{Digest: digestA, ByteLength: 3}, PositionEncoding: "utf-16", Range: r, Direction: binding.Direction, Provenance: binding.Provenance}
	return a, request, selection
}

func TestRelationV2CompatibilityAndFailClosedSelector(t *testing.T) {
	typeOf := reflect.TypeOf(RelationSelector{})
	wantFields := []string{"RelationID", "Caller", "Callee", "Range"}
	if typeOf.NumField() != len(wantFields) {
		t.Fatalf("ASSERT_RELATION_SELECTOR_CALLER_AUTHORITY_FIELDS: fields=%d", typeOf.NumField())
	}
	for i, want := range wantFields {
		if typeOf.Field(i).Name != want {
			t.Fatalf("ASSERT_RELATION_SELECTOR_CALLER_AUTHORITY_FIELDS[%d]: got=%s want=%s", i, typeOf.Field(i).Name, want)
		}
	}
	raw := validCustodyArtifact(t)
	a, err := Admit(raw)
	if err != nil {
		t.Fatal(err)
	}
	key := a.DisplayKeys()[0]
	request := Request{Target: key, Selections: []Key{key}}
	plan, err := Select(a, request)
	if err != nil {
		t.Fatal(err)
	}
	got, err := plan.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		Ordering   string      `json:"ordering"`
		Target     Key         `json:"target"`
		Selections []Selection `json:"selections"`
	}
	legacy.Ordering, legacy.Target, legacy.Selections = plan.Ordering, plan.Target, plan.Selections
	want, _ := json.Marshal(legacy)
	if !bytes.Equal(got, want) || bytes.Contains(got, []byte(`"relations"`)) {
		t.Fatalf("ASSERT_RELATION_V2_ENDPOINT_BYTES_COMPATIBLE: got=%s want=%s", got, want)
	}
	request.Relations = []RelationSelector{{RelationID: "r", Caller: key, Callee: key, Range: rg(0, 0, 0, 1)}}
	if p, err := Select(a, request); !reflect.DeepEqual(p, Plan{}) || !IsCode(err, CodeInvalidRequest) || strings.Contains(err.Error(), string(raw)) {
		t.Fatalf("ASSERT_RELATION_V2_SELECTOR_FAILS_CLOSED_SOURCE_SAFE: plan=%+v err=%v", p, err)
	}
}

func TestRelationSelectionDerivesExactOccurrenceAndRejectsSubstitutions(t *testing.T) {
	a, request, want := relationFixture(t)
	plan, err := Select(a, request)
	if err != nil || !reflect.DeepEqual(plan.Relations, []RelationSelection{want}) {
		t.Fatalf("ASSERT_RELATION_EXACT_DERIVED_SELECTION: got=%+v err=%v", plan.Relations, err)
	}
	baseSeal, baseBytes := plan.binding, mustPlanBytes(t, plan)
	mutations := []struct {
		name  string
		apply func(*Request)
	}{
		{"missing", func(r *Request) { r.Relations[0].RelationID = "missing" }},
		{"reversed", func(r *Request) {
			r.Relations[0].Caller, r.Relations[0].Callee = r.Relations[0].Callee, r.Relations[0].Caller
		}},
		{"foreign-uri", func(r *Request) { r.Relations[0].Caller.LogicalSourceID = "file:///foreign.go" }},
		{"substituted-range", func(r *Request) { r.Relations[0].Range = rg(9, 0, 9, 1) }},
		{"duplicate-selector", func(r *Request) { r.Relations = append(r.Relations, r.Relations[0]) }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			r := request
			r.Relations = append([]RelationSelector(nil), request.Relations...)
			tc.apply(&r)
			p, err := Select(a, r)
			if !reflect.DeepEqual(p, Plan{}) || err == nil {
				t.Fatalf("ASSERT_RELATION_SUBSTITUTION_%s_REJECTED: plan=%+v err=%v", tc.name, p, err)
			}
		})
	}
	ambiguous := a
	ambiguous.v3 = cloneV3(t, a.v3)
	ambiguous.v3.Bindings = append(ambiguous.v3.Bindings, ambiguous.v3.Bindings[0])
	ambiguous.v3.Bindings[1].OccurrenceID = strings.Replace(ambiguous.v3.Bindings[1].OccurrenceID, "d", "e", 1)
	if p, err := Select(ambiguous, request); !reflect.DeepEqual(p, Plan{}) || !IsCode(err, CodeAmbiguousSelection) {
		t.Fatalf("ASSERT_RELATION_AMBIGUOUS_REJECTED: %+v %v", p, err)
	}

	changed := a
	changed.v3 = cloneV3(t, a.v3)
	changed.v3.Bindings[0].OccurrenceID = strings.Replace(changed.v3.Bindings[0].OccurrenceID, "d", "e", 1)
	changedPlan, err := Select(changed, request)
	if err != nil || bytes.Equal(baseBytes, mustPlanBytes(t, changedPlan)) || baseSeal == changedPlan.binding {
		t.Fatalf("ASSERT_RELATION_OCCURRENCE_CHANGES_PLAN_AND_SEAL")
	}
	forged := plan
	forged.Relations = append([]RelationSelection(nil), plan.Relations...)
	forged.Relations[0] = changedPlan.Relations[0]
	if got, err := a.CustodyBinding(forged); got != (RetainedCustodyBinding{}) || !IsCode(err, CodeInvalidPlan) {
		t.Fatalf("ASSERT_RELATION_COORDINATED_POST_PLAN_SUBSTITUTION: got=%+v err=%v", got, err)
	}
}

func TestRelationSelectionPreservesParallelOccurrences(t *testing.T) {
	a, request, first := relationFixture(t)
	secondBinding := a.v3.Bindings[0]
	secondBinding.OccurrenceID = strings.Replace(secondBinding.OccurrenceID, "d", "e", 1)
	secondBinding.Range = rg(4, 1, 4, 6)
	secondBinding.CanonicalOrdinal = 1
	a.v3.Bindings = append(a.v3.Bindings, secondBinding)
	request.Relations = append(request.Relations, RelationSelector{RelationID: secondBinding.RelationID, Caller: request.Relations[0].Caller, Callee: request.Relations[0].Callee, Range: secondBinding.Range})

	plan, err := Select(a, request)
	if err != nil {
		t.Fatalf("ASSERT_RELATION_PARALLEL_OCCURRENCES_ACCEPTED: %v", err)
	}
	if len(plan.Relations) != 2 {
		t.Fatalf("ASSERT_RELATION_PARALLEL_OCCURRENCES_PRESERVED: got=%d", len(plan.Relations))
	}
	got := map[string]graph.Range{}
	for _, relation := range plan.Relations {
		got[relation.OccurrenceID] = relation.Range
	}
	if got[first.OccurrenceID] != first.Range || got[secondBinding.OccurrenceID] != secondBinding.Range {
		t.Fatalf("ASSERT_RELATION_PARALLEL_CALL_SITES_DISTINCT: %+v", plan.Relations)
	}
}

func TestRelationResolveSharedLookupLimitsAndCopies(t *testing.T) {
	_, _, relation := relationFixture(t)
	o := object("aaa")
	relation.Source = o.Identity
	p := resolvePlan(o, o)
	p.Selections[0].Key = relation.Callee
	p.Selections[1].Key = relation.Caller
	p.Target = relation.Callee
	p.Relations = []RelationSelection{relation}
	calls := 0
	got, err := Resolve(p, lookupFunc(func(id sourceobject.Identity) (sourceobject.Object, error) { calls++; return o, nil }), generousLimits())
	if err != nil || calls != 1 || len(got.Relations) != 1 {
		t.Fatalf("ASSERT_RELATION_RESOLVE_SHARED_LOOKUP: calls=%d got=%+v err=%v", calls, got, err)
	}
	got.Relations[0].Bytes[0] = 'X'
	if string(got.Selections[0].Bytes) != "aaa" || string(o.Bytes) != "aaa" {
		t.Fatal("ASSERT_RELATION_RESOLVE_DEFENSIVE_BYTES")
	}
	limits := generousLimits()
	limits.MaxLogicalSelections = 1
	if zero, err := Resolve(p, lookupFunc(func(sourceobject.Identity) (sourceobject.Object, error) { t.Fatal("lookup after limit"); return o, nil }), limits); !reflect.DeepEqual(zero, ResolveResult{}) || !IsCode(err, CodeResolveSelectionLimit) {
		t.Fatalf("ASSERT_RELATION_RESOLVE_SHARED_LOGICAL_LIMIT: %+v %v", zero, err)
	}
	meta, err := ResolveMetadata(p, generousLimits())
	if err != nil || len(meta.Relations) != 1 || len(meta.Relations[0].Bytes) != 0 {
		t.Fatalf("ASSERT_RELATION_RESOLVE_METADATA: %+v %v", meta, err)
	}
}

func TestRelationAssemblyOneProjectionExactOutput(t *testing.T) {
	resolved := retainedResolved()
	caller, callee := resolved.Selections[1].Selection.Key, resolved.Selections[0].Selection.Key
	r := retainedRange(0, 1, 0, 4)
	relation := RelationSelection{RelationID: "calls", OccurrenceID: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", Caller: caller, Callee: callee, ReceiptID: "r", Source: resolved.Selections[1].Selection.Source, PositionEncoding: "utf-16", Range: r, Direction: v5sourcesnapshotv3.Direction, Provenance: v5sourcesnapshotv3.Provenance}
	resolved.Relations = []ResolvedRelation{{Selection: relation, Bytes: append([]byte(nil), resolved.Selections[1].Bytes...)}}
	got, err := AssembleV2Bounded(resolved, retainedPolicy(true), retainedTestBinding{}, "retained-public", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Units) != len(resolved.Selections)+1 || len(got.Citations) != len(got.Units) {
		t.Fatalf("ASSERT_RELATION_ASSEMBLY_SINGLE_PROJECTION_COUNTS: units=%d citations=%d", len(got.Units), len(got.Citations))
	}
	var found bool
	for _, u := range got.Units {
		if u.Role == "RELATION" {
			found = true
			if u.GraphSubjectID != relation.RelationID || u.OccurrenceID != relation.OccurrenceID || u.RelationProvenance != "SERVER_REPORTED" || u.EvidenceRange != projectionRangeRetained(r) || u.Body != "lph" {
				t.Fatalf("ASSERT_RELATION_ASSEMBLY_EXACT_UNIT: %+v", u)
			}
		}
	}
	if !found || got.Authority != 0 || got.SourceGraphComplete != "UNKNOWN" || got.GraphFactsAdded != 0 {
		t.Fatalf("ASSERT_RELATION_ASSEMBLY_AUTHORITY_COMPLETENESS: found=%v tuple=%d/%s/%d", found, got.Authority, got.SourceGraphComplete, got.GraphFactsAdded)
	}
	policy := retainedPolicy(false)
	policy.MaxObjects = 0
	zero, err := AssembleV2Bounded(resolved, policy, retainedTestBinding{}, "retained-public", 1<<20)
	if err != nil || len(zero.Units) != 0 || zero.Accounting.Selected != 0 || zero.Accounting.Omitted != len(resolved.Selections)+len(resolved.Relations) {
		t.Fatalf("ASSERT_RELATION_ASSEMBLY_ZERO_RESOURCE: %+v %v", zero, err)
	}
}

func mustPlanBytes(t *testing.T, p Plan) []byte {
	t.Helper()
	b, e := p.Bytes()
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func cloneV3(t *testing.T, in *v5sourcesnapshotv3.Artifact) *v5sourcesnapshotv3.Artifact {
	t.Helper()
	b, _ := json.Marshal(in)
	var out v5sourcesnapshotv3.Artifact
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

var _ = sourceprojection.Candidate{}
