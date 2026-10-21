package programc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graphkernel"
	"lsp-trace/internal/programcadmission"
)

// This file is a causal RED contract. Every Stage7C type and function named by
// these tests is production-owned. Fixtures use existing Stage3-5 producers and
// independently declared wire DTOs; they never mint a Stage4 qualification or
// Stage5 composition/readiness assertion.

const (
	stage7CCouplingSchema    = "lsp-trace.relation-scoped-coupling.v1"
	stage7CInstabilitySchema = "lsp-trace.successor-instability.v1"
	stage7CNumericPolicy     = "EXACT_REDUCED_NONNEGATIVE_INTEGER_RATIONAL_V1"
)

type stage7CID struct {
	ID, Schema, SHA256 string
	ByteLength         int
}
type stage7CRational struct {
	Numerator, Denominator uint64
	ZeroDenominator        string
}
type stage7CFamily struct {
	Scope                                string
	Evidence                             []byte
	SHA256                               string
	ByteLength                           int
	Availability                         string
	Omissions                            []string
	Input, Included, Omitted, Duplicates uint64
	Terminal, Truncated                  bool
}
type stage7CCouplingNode struct {
	NodeID      string
	Ca, Ce      uint64
	Instability stage7CRational
}
type stage7CCouplingDTO struct {
	Schema, Scope, Completeness, NumericPolicy string
	Graph, Projection                          stage7CID
	Families                                   []stage7CFamily
	Nodes                                      []stage7CCouplingNode
	Input, Included, Omitted, Duplicates       uint64
	Terminal, Truncated                        bool
	Limits                                     RelationScopedCouplingLimits
	Authority                                  int
	Accepted                                   bool
	Claims                                     []string
	Digest                                     string
}
type stage7CRunDTO struct {
	Seed, Ordinal           uint64
	RunID, Status, ResultID string
	Result                  []byte
	ResultSHA256            string
	ResultByteLength        int
}
type stage7CPairDTO struct {
	LeftRunID, RightRunID          string
	Matched, Reassigned, Unmatched uint64
	CommunityDenominator           uint64
	NodeIntersection, NodeUnion    uint64
	Jaccard, UnmatchedRate         stage7CRational
	Stable                         bool
}
type stage7CInstabilityDTO struct {
	Schema, Algorithm, Profile, NumericPolicy  string
	Stage3, Stage4Policy, Stage4Input          stage7CID
	Stage4Qualification, Stage5Composition     stage7CID
	Stage5Readiness                            stage7CID
	Parameters                                 []byte
	ParametersSHA256                           string
	Runs                                       []stage7CRunDTO
	Pairs                                      []stage7CPairDTO
	ExpectedRuns, CompleteRuns                 uint64
	FailedRuns, IncompleteRuns, MissingRuns    uint64
	TerminalRuns                               uint64
	ExpectedPairs, CompletePairs, BlockedPairs uint64
	Outcome                                    string
	PublicationAllowed, PublicationBlocked     bool
	Limits                                     SuccessorInstabilityLimits
	Authority                                  int
	Accepted                                   bool
	Completeness, Digest                       string
}

func stage7CRawHash(raw []byte) string {
	h := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(h[:])
}
func stage7CJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}
func stage7CClone[T any](t *testing.T, v T) T {
	t.Helper()
	raw := stage7CJSON(t, v)
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func stage7CIDFor(id, schema string, raw []byte) stage7CID {
	return stage7CID{id, schema, stage7CRawHash(raw), len(raw)}
}

// stage7CRealPredecessors uses the repository's admitted TypedInput and
// referencegrouping builders. The qualification, composition, and readiness
// bytes are returned only by the prospective retained-artifact loader, which
// must parse and independently verify them; this test never fabricates them.
func stage7CRealPredecessors(t *testing.T, kinds []programcadmission.RelationKind) Stage7CPredecessors {
	t.Helper()
	fixture := typedComputeFixture(t, false)
	var typed programcadmission.TypedInput
	if len(kinds) > 0 {
		var prepareErr error
		typed, prepareErr = programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{
			Calls: fixture.calls.Admission, Methods: fixture.methods,
			EnableMethods: !reflect.DeepEqual(kinds, []programcadmission.RelationKind{programcadmission.TypedCalls}),
			Kinds:         append([]programcadmission.RelationKind(nil), kinds...),
		})
		if prepareErr != nil {
			t.Fatalf("BLOCKED_NOT_RED typed mode admission kinds=%v: %v", kinds, prepareErr)
		}
	}
	stage3Raw := stage7CJSON(t, typed.Occurrences)
	_, input := testStage5ReferenceGroupingInput(t)
	policy := arbitraryPreStage5Policy(t, "stage7c-exact-policy")
	if len(policy.CanonicalBytes()) == 0 || len(input.CanonicalBytes()) == 0 {
		t.Fatal("BLOCKED_NOT_RED empty real predecessor")
	}
	supplied := stage7CQualificationFixture(t, policy.ID(), policy.Digest(), policy.ByteLength(), input.ID(), input.Digest(), input.ByteLength(), 8)
	retained, err := LoadRetainedStage4AndStage5Predecessors(policy, input, stage3Raw, 8, supplied)
	if err != nil {
		t.Fatalf("BLOCKED_NOT_RED retained Stage4-5 fixture unavailable: %v", err)
	}
	q, c, r := supplied.Qualification.Receipt, supplied.Composition, supplied.Readiness
	return Stage7CPredecessors{
		Stage3Canonical: stage3Raw, Stage3SHA256: stage7CRawHash(stage3Raw), Stage3ByteLength: len(stage3Raw),
		Stage4PolicyCanonical: policy.CanonicalBytes(), Stage4PolicyID: policy.ID(), Stage4PolicySHA256: policy.Digest(), Stage4PolicyByteLength: policy.ByteLength(),
		Stage4InputCanonical: input.CanonicalBytes(), Stage4InputID: input.ID(), Stage4InputSHA256: input.Digest(), Stage4InputByteLength: input.ByteLength(),
		Stage4QualificationCanonical: retained.QualificationCanonicalBytes(), Stage4QualificationID: q.ID, Stage4QualificationSchema: q.Schema, Stage4QualificationSHA256: q.SHA256, Stage4QualificationByteLength: q.ByteLength,
		Stage5CompositionCanonical: retained.CompositionCanonicalBytes(), Stage5CompositionID: c.ID, Stage5CompositionSchema: c.Schema, Stage5CompositionSHA256: c.SHA256, Stage5CompositionByteLength: c.ByteLength,
		Stage5ReadinessCanonical: retained.ReadinessCanonicalBytes(), Stage5ReadinessID: r.ID, Stage5ReadinessSchema: r.Schema, Stage5ReadinessSHA256: r.SHA256, Stage5ReadinessByteLength: r.ByteLength,
		QualificationCustody: retained.QualificationCustody(), Epoch: 8,
	}
}

func stage7CQualificationFixture(t *testing.T, policyID, policyDigest string, policyLength int, inputID, inputDigest string, inputLength int, epoch uint64) RetainedStage4AndStage5Evidence {
	t.Helper()
	pc := qualificationCommitment7{policyID, policyDigest, policyLength}
	ic := qualificationCommitment7{inputID, inputDigest, inputLength}
	evRaw := stage7CJSON(t, struct{ Kind string }{"TEST_ONLY_STRUCTURALLY_ISOLATED_QUALIFICATION_EVIDENCE"})
	ev := ExactRetainedEvidence{"evidence-1", "test.stage7c.evidence.v1", stage7CRawHash(evRaw), len(evRaw), evRaw}
	ec := qualificationCommitment7{ev.ID, ev.SHA256, ev.ByteLength}
	reviewer := qualificationReviewer7{"reviewer-1", "test.stage7c.reviewers", "LSP_TRACE_TEST_MAINTAINER", "REVIEWER_ASSERTED", "ACCEPT"}
	rd := qualificationReceipt7{"test.stage7c.qualification.v1", "QUALIFIED", pc, ic, []qualificationCommitment7{ec}, []qualificationReviewer7{reviewer}, "LSP_TRACE_TEST_MAINTAINER", 1, epoch, epoch}
	raw := stage7CJSON(t, rd)
	receipt := ExactRetainedEvidence{"qualification-1", rd.Schema, stage7CRawHash(raw), len(raw), raw}
	rc := qualificationCommitment7{receipt.ID, receipt.SHA256, receipt.ByteLength}
	record := stage7CJSON(t, struct {
		Receipt, Policy, Input qualificationCommitment7
		Evidence               []qualificationCommitment7
		Reviewer               qualificationReviewer7
	}{rc, pc, ic, []qualificationCommitment7{ec}, reviewer})
	anchorDoc := qualificationAnchor7{reviewer.Identity, reviewer.IdentityDomain, reviewer.IdentityClass, reviewer.Custody, reviewer.Decision, stage7CRawHash(record)}
	anchorRaw := stage7CJSON(t, anchorDoc)
	anchor := ExactRetainedEvidence{reviewer.Identity, "test.stage7c.trust-anchor.v1", stage7CRawHash(anchorRaw), len(anchorRaw), anchorRaw}
	q := &QualificationCustody{receipt, []ExactRetainedEvidence{ev}, []ExactRetainedEvidence{anchor}, epoch}
	cd := stage5Link7{"test.stage7c.composition.v1", epoch, pc, ic, rc, nil}
	cRaw := stage7CJSON(t, cd)
	c := ExactRetainedEvidence{"composition-1", cd.Schema, stage7CRawHash(cRaw), len(cRaw), cRaw}
	cc := qualificationCommitment7{c.ID, c.SHA256, c.ByteLength}
	rdoc := stage5Link7{"test.stage7c.readiness.v1", epoch, pc, ic, rc, &cc}
	rRaw := stage7CJSON(t, rdoc)
	r := ExactRetainedEvidence{"readiness-1", rdoc.Schema, stage7CRawHash(rRaw), len(rRaw), rRaw}
	return RetainedStage4AndStage5Evidence{q, c, r}
}

func TestStage7CCouplingCompleteConnectedDTOAndHistoricalCalls(t *testing.T) {
	p := stage7CRealPredecessors(t, []programcadmission.RelationKind{programcadmission.TypedCalls})
	in := NewRelationScopedCouplingInputFromPredecessors(RelationScopeCalls, p, RelationScopedCouplingLimits{MaxInputBytes: 1 << 20, MaxNodes: 16, MaxRelations: 16, MaxWork: 256})
	got, failure := ComputeRelationScopedCouplingArtifact(in)
	if failure != nil {
		t.Fatal(failure)
	}
	var graph struct {
		Nodes []string
		Edges [][2]string
	}
	if err := json.Unmarshal(got.GraphCanonicalBytes(), &graph); err != nil {
		t.Fatal(err)
	}
	arcs := make([]graphkernel.Arc, len(graph.Edges))
	for i, e := range graph.Edges {
		arcs[i] = graphkernel.Arc{From: e[0], To: e[1], Weight: 1}
	}
	g, err := graphkernel.NewDirectedWeighted(graph.Nodes, arcs)
	if err != nil {
		t.Fatal(err)
	}
	historical := g.RobertMartinCoupling()
	wantNodes := make([]stage7CCouplingNode, len(got.Nodes()))
	for i, n := range got.Nodes() {
		wantNodes[i] = stage7CCouplingNode{n.NodeID, n.Ca, n.Ce, stage7CRational{n.Instability.Numerator, n.Instability.Denominator, n.Instability.ZeroDenominator}}
	}
	if !got.ReproducesHistoricalRobertMartinCoupling(historical) {
		t.Fatal("ASSERT_STAGE7C_CONNECTED_HISTORICAL_CALLS")
	}
	want := stage7CCouplingDTO{
		Schema: stage7CCouplingSchema, Scope: "CALLS", Graph: stage7CIDFor(got.GraphID(), got.GraphSchema(), got.GraphCanonicalBytes()), Projection: stage7CIDFor(got.ProjectionID(), got.ProjectionSchema(), got.ProjectionCanonicalBytes()),
		Families: []stage7CFamily{{Scope: "CALLS", Evidence: got.FamilyCanonicalBytes(RelationScopeCalls), SHA256: stage7CRawHash(got.FamilyCanonicalBytes(RelationScopeCalls)), ByteLength: len(got.FamilyCanonicalBytes(RelationScopeCalls)), Availability: "AVAILABLE", Input: uint64(len(graph.Edges)), Included: uint64(len(graph.Edges)), Terminal: true}},
		Nodes:    wantNodes, Input: uint64(len(graph.Edges)), Included: uint64(len(graph.Edges)), Terminal: true, Completeness: "UNKNOWN", NumericPolicy: stage7CNumericPolicy, Limits: in.Limits, Authority: 0, Accepted: false,
		Claims: []string{"STRUCTURAL_RELATION_SCOPED_COUPLING_ONLY", "NO_SEMANTIC_AUTHORITY", "NO_FEATURE_ACCEPTANCE"}, Digest: got.Digest(),
	}
	if dto := got.DTO(); !bytes.Equal(stage7CJSON(t, dto), stage7CJSON(t, want)) {
		t.Fatalf("ASSERT_STAGE7C_COMPLETE_COUPLING_DTO\n got=%#v\nwant=%#v", dto, want)
	}
	if raw := got.CanonicalBytes(); !bytes.Equal(raw, stage7CJSON(t, want)) {
		t.Fatalf("ASSERT_STAGE7C_COUPLING_LITERAL_GOLDEN\n%s", raw)
	}
}

func TestStage7CCouplingScopesCombinedQualificationAndZeroDenominator(t *testing.T) {
	cases := []struct {
		name  string
		scope RelationScope
		kinds []programcadmission.RelationKind
	}{
		{"calls", RelationScopeCalls, []programcadmission.RelationKind{programcadmission.TypedCalls}},
		{"references", RelationScopeReferencesSymbol, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}},
		{"definitions", RelationScopeResolvesToDefinition, []programcadmission.RelationKind{programcadmission.TypedResolvesToDefinition}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := stage7CRealPredecessors(t, tc.kinds)
			in := NewRelationScopedCouplingInputFromPredecessors(tc.scope, p, RelationScopedCouplingLimits{1 << 20, 32, 32, 512})
			got, f := ComputeRelationScopedCouplingArtifact(in)
			if f != nil {
				t.Fatal(f)
			}
			if got.Scope() != tc.scope || got.Authority() != 0 || got.Accepted() || got.Completeness() != "UNKNOWN" {
				t.Fatalf("ASSERT_STAGE7C_SCOPE_CEILING %+v", got)
			}
			for _, n := range got.Nodes() {
				if n.Ca+n.Ce == 0 {
					if n.Instability.Denominator != 0 || n.Instability.ZeroDenominator != ZeroDenominatorTyped {
						t.Fatal("ASSERT_STAGE7C_TYPED_ZERO")
					}
				} else if n.Instability.Numerator != n.Ce || n.Instability.Denominator != n.Ca+n.Ce {
					t.Fatal("ASSERT_STAGE7C_EXACT_CE_OVER_TOTAL")
				}
			}
		})
	}
	p := stage7CRealPredecessors(t, []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition})
	for name, mutate := range map[string]func(*Stage7CPredecessors){
		"qualification-not-issued": func(x *Stage7CPredecessors) {
			x.QualificationCustody = nil
			x.Stage4QualificationCanonical = nil
			x.Stage4QualificationID = ""
			x.Stage4QualificationSchema = ""
			x.Stage4QualificationSHA256 = ""
			x.Stage4QualificationByteLength = 0
		},
		"qualification-absent": func(x *Stage7CPredecessors) { x.Stage4QualificationCanonical = nil },
		"qualification-stale":  func(x *Stage7CPredecessors) { x.Epoch++ },
		"composition-absent":   func(x *Stage7CPredecessors) { x.Stage5CompositionCanonical = nil },
		"readiness-absent":     func(x *Stage7CPredecessors) { x.Stage5ReadinessCanonical = nil },
	} {
		t.Run(name, func(t *testing.T) {
			q := p.DeepCopy()
			mutate(&q)
			in := NewRelationScopedCouplingInputFromPredecessors(RelationScopeCombined, q, RelationScopedCouplingLimits{1 << 20, 32, 32, 512})
			got, f := ComputeRelationScopedCouplingArtifact(in)
			if f == nil || f.Code != CombinedQualificationUnavailable || !reflect.DeepEqual(got, RelationScopedCouplingArtifact{}) {
				t.Fatalf("ASSERT_STAGE7C_COMBINED_GATE got=%+v failure=%+v", got, f)
			}
		})
	}
}

func stage7CInstabilityFixture(p Stage7CPredecessors, seeds []uint64, threshold SuccessorThreshold, l SuccessorInstabilityLimits) SuccessorInstabilityInput {
	in := NewSuccessorInstabilityInputFromPredecessors(p, seeds, threshold, l)
	_, nodes := stage7CEdges(p.Stage3Canonical, RelationScopeCombined)
	partition := make([][]string, len(nodes))
	for i, n := range nodes {
		partition[i] = []string{n}
	}
	raw := mustJSON7(partition)
	for _, seed := range seeds {
		runID := fmt.Sprintf("run-%d", seed)
		in.Runs = append(in.Runs, SuccessorRun{Seed: seed, Ordinal: seed, RunID: runID, Status: SuccessorRunComplete, ResultID: runID + "-result", Result: clone7(raw), ResultSHA256: hash7(raw), ResultByteLength: len(raw)})
	}
	return in
}

func TestStage7CSuccessorExactMatchingAllPairsThresholdAndPublication(t *testing.T) {
	p := stage7CRealPredecessors(t, []programcadmission.RelationKind{programcadmission.TypedCalls})
	in := stage7CInstabilityFixture(p, []uint64{3, 5, 7}, SuccessorThreshold{Numerator: 1, Denominator: 2, Boundary: ThresholdInclusive}, SuccessorInstabilityLimits{MaxInputBytes: 1 << 20, MaxRuns: 16, MaxPairs: 120, MaxCommunities: 64, MaxNodes: 256, MaxWork: 1 << 20})
	got, f := ComputeSuccessorInstabilityArtifact(in)
	if f != nil {
		t.Fatal(f)
	}
	if got.RunAccounting() != (SuccessorRunAccounting{Expected: 3, Complete: 3, Terminal: 3}) || got.PairAccounting() != (SuccessorPairAccounting{Expected: 3, Complete: 3}) {
		t.Fatalf("ASSERT_STAGE7C_EXACT_N_AND_N_CHOOSE_2 runs=%+v pairs=%+v", got.RunAccounting(), got.PairAccounting())
	}
	n := uint64(len(got.Runs()[0].ResultCanonicalBytes()))
	_ = n
	var partition [][]string
	if err := json.Unmarshal(got.Runs()[0].ResultCanonicalBytes(), &partition); err != nil {
		t.Fatal(err)
	}
	count := uint64(len(partition))
	wantPairs := []stage7CPairDTO{{LeftRunID: "run-3", RightRunID: "run-5", Matched: count, CommunityDenominator: count, NodeIntersection: count, NodeUnion: count, Jaccard: stage7CRational{1, 1, ""}, UnmatchedRate: stage7CRational{0, 1, ""}, Stable: true}, {LeftRunID: "run-3", RightRunID: "run-7", Matched: count, CommunityDenominator: count, NodeIntersection: count, NodeUnion: count, Jaccard: stage7CRational{1, 1, ""}, UnmatchedRate: stage7CRational{0, 1, ""}, Stable: true}, {LeftRunID: "run-5", RightRunID: "run-7", Matched: count, CommunityDenominator: count, NodeIntersection: count, NodeUnion: count, Jaccard: stage7CRational{1, 1, ""}, UnmatchedRate: stage7CRational{0, 1, ""}, Stable: true}}
	if !bytes.Equal(stage7CJSON(t, got.DTO().Pairs), stage7CJSON(t, wantPairs)) || !got.PublicationAllowed() || got.PublicationBlocked() || got.Outcome() != SuccessorReadinessReady {
		t.Fatalf("ASSERT_STAGE7C_EXACT_MATCH_AND_PUBLICATION %+v", got.DTO())
	}
	if !bytes.Equal(got.CanonicalBytes(), stage7CJSON(t, got.DTO())) {
		t.Fatal("ASSERT_STAGE7C_INSTABILITY_CONNECTED_LITERAL_GOLDEN")
	}

	matching := []struct {
		name string
		pair SuccessorPartitionPair
		want SuccessorPartitionComparison
	}{
		{"label-invariant", SuccessorPartitionPair{Left: [][]string{{"a", "b"}, {"c"}}, Right: [][]string{{"c"}, {"b", "a"}}}, SuccessorPartitionComparison{Matched: 2, CommunityDenominator: 2, NodeIntersection: 3, NodeUnion: 3, Jaccard: ExactRational{1, 1}, UnmatchedRate: ExactRational{0, 1}}},
		{"split", SuccessorPartitionPair{Left: [][]string{{"a", "b", "c"}}, Right: [][]string{{"a"}, {"b", "c"}}}, SuccessorPartitionComparison{Matched: 1, Reassigned: 3, Unmatched: 1, CommunityDenominator: 2, NodeIntersection: 3, NodeUnion: 3, Jaccard: ExactRational{1, 1}, UnmatchedRate: ExactRational{1, 2}}},
		{"unequal-union", SuccessorPartitionPair{Left: [][]string{{"a"}}, Right: [][]string{{"a"}, {"missing"}}}, SuccessorPartitionComparison{Matched: 1, Unmatched: 1, CommunityDenominator: 2, NodeIntersection: 1, NodeUnion: 2, Jaccard: ExactRational{1, 2}, UnmatchedRate: ExactRational{1, 2}}},
		{"tie-lexical", SuccessorPartitionPair{Left: [][]string{{"a"}, {"b"}}, Right: [][]string{{"a", "b"}, {"c"}}}, SuccessorPartitionComparison{Matched: 1, Reassigned: 2, Unmatched: 2, CommunityDenominator: 2, NodeIntersection: 2, NodeUnion: 3, Jaccard: ExactRational{2, 3}, UnmatchedRate: ExactRational{1, 1}}},
	}
	for _, tc := range matching {
		t.Run(tc.name, func(t *testing.T) {
			got, f := CompareSuccessorPartitions(tc.pair)
			if f != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ASSERT_STAGE7C_EXACT_MATCH got=%+v want=%+v failure=%+v", got, tc.want, f)
			}
		})
	}
	if !EvaluateSuccessorThreshold(ExactRational{1, 2}, SuccessorThreshold{1, 2, ThresholdInclusive}) || EvaluateSuccessorThreshold(ExactRational{1, 2}, SuccessorThreshold{1, 2, ThresholdExclusive}) {
		t.Fatal("ASSERT_STAGE7C_THRESHOLD_BOUNDARY")
	}
	max := ^uint64(0)
	if !EvaluateSuccessorThreshold(ExactRational{max, max}, SuccessorThreshold{max, max, ThresholdInclusive}) || EvaluateSuccessorThreshold(ExactRational{max, max - 1}, SuccessorThreshold{max, max, ThresholdInclusive}) {
		t.Fatal("ASSERT_STAGE7C_THRESHOLD_CHECKED_128_BIT_COMPARISON")
	}
}

func TestStage7CSuccessorRunAccountingAndEveryPublicationBlocker(t *testing.T) {
	base := stage7CInstabilityFixture(stage7CRealPredecessors(t, []programcadmission.RelationKind{programcadmission.TypedCalls}), []uint64{3, 5, 7}, SuccessorThreshold{1, 2, ThresholdInclusive}, SuccessorInstabilityLimits{1 << 20, 16, 120, 64, 256, 1 << 20})
	cases := []struct {
		name, code string
		mutate     func(*SuccessorInstabilityInput)
		runs       SuccessorRunAccounting
		pairs      SuccessorPairAccounting
	}{
		{"failed", SuccessorReadinessPublicationBlocked, func(x *SuccessorInstabilityInput) {
			x.Runs[0].Status = SuccessorRunFailed
			x.Runs[0].Result = nil
			x.Runs[0].ResultID = ""
			x.Runs[0].ResultSHA256 = ""
			x.Runs[0].ResultByteLength = 0
		}, SuccessorRunAccounting{Expected: 3, Complete: 2, Failed: 1, Terminal: 3}, SuccessorPairAccounting{Expected: 3, Blocked: 2, Complete: 1}},
		{"incomplete", SuccessorReadinessPublicationBlocked, func(x *SuccessorInstabilityInput) {
			x.Runs[0].Status = SuccessorRunIncomplete
			x.Runs[0].Result = nil
			x.Runs[0].ResultID = ""
			x.Runs[0].ResultSHA256 = ""
			x.Runs[0].ResultByteLength = 0
		}, SuccessorRunAccounting{Expected: 3, Complete: 2, Incomplete: 1, Terminal: 2}, SuccessorPairAccounting{Expected: 3, Blocked: 2, Complete: 1}},
		{"missing", SuccessorReadinessPublicationBlocked, func(x *SuccessorInstabilityInput) { x.Runs = x.Runs[:2] }, SuccessorRunAccounting{Expected: 3, Complete: 2, Missing: 1, Terminal: 2}, SuccessorPairAccounting{Expected: 3, Blocked: 2, Complete: 1}},
		{"threshold", SuccessorReadinessPublicationBlocked, func(x *SuccessorInstabilityInput) { x.Threshold = SuccessorThreshold{0, 1, ThresholdExclusive} }, SuccessorRunAccounting{Expected: 3, Complete: 3, Terminal: 3}, SuccessorPairAccounting{Expected: 3, Complete: 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base.DeepCopy()
			tc.mutate(&in)
			got, f := ComputeSuccessorInstabilityArtifact(in)
			if f != nil {
				t.Fatal(f)
			}
			if got.Outcome() != tc.code || got.PublicationAllowed() || !got.PublicationBlocked() || got.RunAccounting() != tc.runs || got.PairAccounting() != tc.pairs {
				t.Fatalf("ASSERT_STAGE7C_PUBLICATION_BLOCKER got=%+v", got.DTO())
			}
		})
	}
	t.Run("expired-qualification", func(t *testing.T) {
		in := base.DeepCopy()
		in.Predecessors.Epoch++
		got, f := ComputeSuccessorInstabilityArtifact(in)
		if f == nil || f.Code != CombinedQualificationUnavailable || !reflect.DeepEqual(got, SuccessorInstabilityArtifact{}) {
			t.Fatalf("ASSERT_STAGE7C_EXPIRED_QUALIFICATION got=%+v failure=%+v", got, f)
		}
	})
}

func TestStage7CStrictFailuresLimitsForgeryCopiesPermutationAndRace(t *testing.T) {
	p := stage7CRealPredecessors(t, []programcadmission.RelationKind{programcadmission.TypedCalls})
	ci := NewRelationScopedCouplingInputFromPredecessors(RelationScopeCalls, p, RelationScopedCouplingLimits{1 << 20, 16, 16, 256})
	si := stage7CInstabilityFixture(p, []uint64{3, 5, 7}, SuccessorThreshold{1, 2, ThresholdInclusive}, SuccessorInstabilityLimits{1 << 20, 16, 120, 64, 256, 1 << 20})
	c, _ := ComputeRelationScopedCouplingArtifact(ci)
	s, _ := ComputeSuccessorInstabilityArtifact(si)
	for family, tc := range map[string]struct {
		raw   []byte
		parse func([]byte) *Stage7CFailure
	}{
		"coupling":    {c.CanonicalBytes(), func(b []byte) *Stage7CFailure { _, f := ParseRelationScopedCouplingArtifact(b, ci.Limits); return f }},
		"instability": {s.CanonicalBytes(), func(b []byte) *Stage7CFailure { _, f := ParseSuccessorInstabilityArtifact(b, si.Limits); return f }},
	} {
		for name, bad := range map[string][]byte{"unknown": bytes.Replace(tc.raw, []byte("{"), []byte("{\"unknown\":true,"), 1), "duplicate": bytes.Replace(tc.raw, []byte("{"), []byte("{\"schema\":\"forged\","), 1), "trailing-value": append(append([]byte(nil), tc.raw...), []byte("{}\n")...), "no-lf": tc.raw[:len(tc.raw)-1], "extra-lf": append(append([]byte(nil), tc.raw...), '\n'), "crlf": append(append([]byte(nil), tc.raw[:len(tc.raw)-1]...), '\r', '\n')} {
			t.Run(family+"-"+name, func(t *testing.T) {
				if f := tc.parse(bad); f == nil || f.Message == "" || len(f.Details) == 0 {
					t.Fatalf("ASSERT_STAGE7C_EXACT_TYPED_PARSE_FAILURE %+v", f)
				}
			})
		}
	}
	for name, mutate := range map[string]func(*SuccessorInstabilityArtifact){
		"stage3": func(x *SuccessorInstabilityArtifact) { x.TestOnlyMutateStage3Length(1) }, "qualification": func(x *SuccessorInstabilityArtifact) {
			x.TestOnlyMutateQualificationDigest("sha256:" + strings.Repeat("f", 64))
		}, "composition": func(x *SuccessorInstabilityArtifact) {
			x.TestOnlyMutateCompositionDigest("sha256:" + strings.Repeat("e", 64))
		}, "readiness": func(x *SuccessorInstabilityArtifact) {
			x.TestOnlyMutateReadinessDigest("sha256:" + strings.Repeat("d", 64))
		}, "parameters": func(x *SuccessorInstabilityArtifact) { x.TestOnlyMutateParameters() }, "run-bytes": func(x *SuccessorInstabilityArtifact) { x.TestOnlyMutateRunBytes(0) }, "pair-order": func(x *SuccessorInstabilityArtifact) { x.TestOnlyReversePairs() }, "accounting": func(x *SuccessorInstabilityArtifact) { x.TestOnlyMutateAccounting() }, "publication": func(x *SuccessorInstabilityArtifact) { x.TestOnlyMutatePublication() }, "limits": func(x *SuccessorInstabilityArtifact) { x.TestOnlyMutateLimits() },
	} {
		t.Run("resealed-"+name, func(t *testing.T) {
			x := s.DeepCopy()
			mutate(&x)
			x.ResealForTest()
			if got, f := ParseSuccessorInstabilityArtifact(x.CanonicalBytes(), si.Limits); f == nil || !reflect.DeepEqual(got, SuccessorInstabilityArtifact{}) {
				t.Fatalf("ASSERT_STAGE7C_RESEALED_SEMANTIC_FORGERY got=%+v failure=%+v", got, f)
			}
		})
	}
	limits := []struct {
		name   string
		mutate func(*SuccessorInstabilityLimits)
	}{{"bytes", func(x *SuccessorInstabilityLimits) { x.MaxInputBytes = 1 }}, {"runs", func(x *SuccessorInstabilityLimits) { x.MaxRuns = 2 }}, {"pairs", func(x *SuccessorInstabilityLimits) { x.MaxPairs = 2 }}, {"communities", func(x *SuccessorInstabilityLimits) { x.MaxCommunities = 1 }}, {"nodes", func(x *SuccessorInstabilityLimits) { x.MaxNodes = 1 }}, {"work", func(x *SuccessorInstabilityLimits) { x.MaxWork = 1 }}}
	for _, tc := range limits {
		t.Run("limit-"+tc.name, func(t *testing.T) {
			in := si.DeepCopy()
			tc.mutate(&in.Limits)
			got, f := ComputeSuccessorInstabilityArtifact(in)
			if f == nil || f.Code != Stage7CLimitExceeded || !reflect.DeepEqual(got, SuccessorInstabilityArtifact{}) || f.Effects != 0 {
				t.Fatalf("ASSERT_STAGE7C_LIMIT_PRE_EFFECT_ATOMIC got=%+v failure=%+v", got, f)
			}
		})
	}
	larger := si.DeepCopy()
	larger.Limits.MaxRuns = ^uint64(0)
	if got, f := ComputeSuccessorInstabilityArtifact(larger); f != nil || got.AggregateInstability() != s.AggregateInstability() || !reflect.DeepEqual(got.Pairs(), s.Pairs()) || bytes.Equal(got.CanonicalBytes(), s.CanonicalBytes()) {
		t.Fatalf("ASSERT_STAGE7C_LARGER_LIMIT_PRESERVES_SEMANTICS_AND_BINDS_PROVENANCE failure=%+v", f)
	}
	copies := [][]byte{s.CanonicalBytes(), s.ParametersCanonicalBytes(), s.Stage3CanonicalBytes(), s.QualificationCanonicalBytes(), s.CompositionCanonicalBytes(), s.ReadinessCanonicalBytes(), s.Runs()[0].ResultCanonicalBytes()}
	for _, b := range copies {
		b[0] ^= 0xff
	}
	if !bytes.Equal(s.CanonicalBytes(), s.DeepCopy().CanonicalBytes()) {
		t.Fatal("ASSERT_STAGE7C_NESTED_DEFENSIVE_COPIES")
	}
	permuted := si.DeepCopy()
	sort.Slice(permuted.Runs, func(i, j int) bool { return permuted.Runs[i].RunID > permuted.Runs[j].RunID })
	again, f := ComputeSuccessorInstabilityArtifact(permuted)
	if f != nil || !bytes.Equal(again.CanonicalBytes(), s.CanonicalBytes()) {
		t.Fatal("ASSERT_STAGE7C_PERMUTATION_INVARIANCE")
	}
	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 64; j++ {
				_ = s.CanonicalBytes()
				_ = s.Runs()
				_ = s.Pairs()
				_ = s.Claims()
			}
		}()
	}
	wg.Wait()
}

func TestStage7CEightRealModesAndEmptyZero(t *testing.T) {
	modes := []struct {
		name  string
		kinds []programcadmission.RelationKind
	}{{"calls-only", []programcadmission.RelationKind{programcadmission.TypedCalls}}, {"refs-only", []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}, {"defs-only", []programcadmission.RelationKind{programcadmission.TypedResolvesToDefinition}}, {"refs-defs", []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}}, {"calls-refs", []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol}}, {"calls-defs", []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedResolvesToDefinition}}, {"all", []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}}, {"empty-zero", nil}}
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			p := stage7CRealPredecessors(t, tc.kinds)
			in := stage7CInstabilityFixture(p, []uint64{3, 5, 7}, SuccessorThreshold{1, 2, ThresholdInclusive}, SuccessorInstabilityLimits{1 << 20, 16, 120, 64, 256, 1 << 20})
			got, f := ComputeSuccessorInstabilityArtifact(in)
			if f != nil {
				t.Fatal(f)
			}
			if got.Mode() != tc.name || got.Authority() != 0 || got.Accepted() || got.Completeness() != "UNKNOWN" {
				t.Fatalf("ASSERT_STAGE7C_REAL_MODE %+v", got.DTO())
			}
			if tc.name == "calls-only" {
				if got.Outcome() != SuccessorReadinessReady || !got.PublicationAllowed() {
					t.Fatalf("ASSERT_STAGE7C_CALLS_MODE_READY %+v", got.DTO())
				}
			} else if got.Outcome() != SuccessorReadinessPublicationBlocked || got.PublicationAllowed() {
				t.Fatalf("ASSERT_STAGE7C_UNQUALIFIED_MODE_EXPLICITLY_BLOCKED mode=%s dto=%+v", tc.name, got.DTO())
			}
			if tc.name == "empty-zero" && (len(got.Pairs()) != 3 || got.AggregateInstability() != (ExactRational{0, 1})) {
				t.Fatal("ASSERT_STAGE7C_EMPTY_ZERO_EXACT")
			}
		})
	}
}

func TestStage7CHistoricalInstabilityV1Connected(t *testing.T) {
	const digest = "sha256:b44cef6e72f2e808202961b94f91a45d97e8bcaded8e0e91ebf6cda64a5bd67c"
	if InstabilityVersion != "lsp-trace.community-instability.v1" || InstabilityPolicyDigest() != digest {
		t.Fatal("ASSERT_STAGE7C_HISTORICAL_POLICY")
	}
	r := campaignFixture(t)
	historical, err := ComputeInstability(r)
	if err != nil {
		t.Fatal(err)
	}
	raw := stage7CJSON(t, historical)
	if err := ValidateInstability(raw, r); err != nil {
		t.Fatal(err)
	}
	p := stage7CRealPredecessors(t, []programcadmission.RelationKind{programcadmission.TypedCalls})
	seedInput := stage7CInstabilityFixture(p, r.DeclaredSeeds, SuccessorThreshold{1, 2, ThresholdInclusive}, SuccessorInstabilityLimits{1 << 20, 16, 120, 64, 256, 1 << 20})
	in := NewSuccessorInstabilityInputFromHistoricalV1(p, r, raw, InstabilityPolicyBytes(), digest, seedInput.Runs, SuccessorInstabilityLimits{1 << 20, 16, 120, 64, 256, 1 << 20})
	got, f := ComputeSuccessorInstabilityArtifact(in)
	if f != nil || !got.ReproducesHistoricalInstabilityV1(raw, digest) {
		t.Fatalf("ASSERT_STAGE7C_CONNECTED_HISTORICAL_INSTABILITY got=%+v failure=%+v", got, f)
	}
}

func TestStage7CExplicitCustodyDerivedForgeryAndMissingRoundTrip(t *testing.T) {
	_, input := testStage5ReferenceGroupingInput(t)
	policy := arbitraryPreStage5Policy(t, "stage7c-custody-mutations")
	stage3 := stage7CJSON(t, []any{})
	base := stage7CQualificationFixture(t, policy.ID(), policy.Digest(), policy.ByteLength(), input.ID(), input.Digest(), input.ByteLength(), 8)
	for name, mutate := range map[string]func(*RetainedStage4AndStage5Evidence){
		"receipt":     func(x *RetainedStage4AndStage5Evidence) { x.Qualification.Receipt.Canonical[0] ^= 1 },
		"evidence":    func(x *RetainedStage4AndStage5Evidence) { x.Qualification.Evidence[0].Canonical[0] ^= 1 },
		"trust":       func(x *RetainedStage4AndStage5Evidence) { x.Qualification.TrustAnchors[0].Canonical[0] ^= 1 },
		"epoch":       func(x *RetainedStage4AndStage5Evidence) { x.Qualification.Epoch++ },
		"composition": func(x *RetainedStage4AndStage5Evidence) { x.Composition.Canonical[0] ^= 1 },
		"readiness":   func(x *RetainedStage4AndStage5Evidence) { x.Readiness.Canonical[0] ^= 1 },
	} {
		t.Run("custody-"+name, func(t *testing.T) {
			x := RetainedStage4AndStage5Evidence{cloneQualificationCustody7(base.Qualification), cloneEvidence7(base.Composition), cloneEvidence7(base.Readiness)}
			mutate(&x)
			if _, err := LoadRetainedStage4AndStage5Predecessors(policy, input, stage3, 8, x); err == nil {
				t.Fatal("ASSERT_STAGE7C_EXACT_CUSTODY_MUTATION_REJECTED")
			}
		})
	}
	p := stage7CRealPredecessors(t, []programcadmission.RelationKind{programcadmission.TypedCalls})
	ci := NewRelationScopedCouplingInputFromPredecessors(RelationScopeCalls, p, RelationScopedCouplingLimits{1 << 20, 16, 16, 256})
	c, f := ComputeRelationScopedCouplingArtifact(ci)
	if f != nil {
		t.Fatal(f)
	}
	for name, mutate := range map[string]func(*relationScopedCouplingDTO){"node": func(d *relationScopedCouplingDTO) { d.Nodes[0].Ce++ }, "family": func(d *relationScopedCouplingDTO) { d.Families[0].Included++ }, "accounting": func(d *relationScopedCouplingDTO) { d.Included++ }, "claims": func(d *relationScopedCouplingDTO) { d.Claims[0] = "forged" }, "graph-id": func(d *relationScopedCouplingDTO) { d.Graph.ID = "forged" }} {
		t.Run("coupling-resealed-"+name, func(t *testing.T) {
			d := c.DTO()
			mutate(&d)
			d.Digest = digestDTO7(d)
			if got, f := ParseRelationScopedCouplingArtifact(mustJSON7(d), ci.Limits); f == nil || !reflect.DeepEqual(got, RelationScopedCouplingArtifact{}) {
				t.Fatalf("ASSERT_STAGE7C_COUPLING_DERIVED_FORGERY got=%+v failure=%+v", got, f)
			}
		})
	}
	si := stage7CInstabilityFixture(p, []uint64{3, 5, 7}, SuccessorThreshold{1, 2, ThresholdInclusive}, SuccessorInstabilityLimits{1 << 20, 16, 120, 64, 256, 1 << 20})
	si.Runs = si.Runs[:2]
	s, f := ComputeSuccessorInstabilityArtifact(si)
	if f != nil {
		t.Fatal(f)
	}
	if s.RunAccounting() != (SuccessorRunAccounting{Expected: 3, Complete: 2, Missing: 1, Terminal: 2}) {
		t.Fatalf("ASSERT_STAGE7C_MISSING_DECLARED_SEED_ACCOUNTING %+v", s.RunAccounting())
	}
	parsed, f := ParseSuccessorInstabilityArtifact(s.CanonicalBytes(), si.Limits)
	if f != nil || parsed.RunAccounting() != s.RunAccounting() {
		t.Fatalf("ASSERT_STAGE7C_MISSING_CANONICAL_ROUNDTRIP parsed=%+v failure=%+v", parsed.RunAccounting(), f)
	}
}

func TestStage7COnlyAuthorizedTestFileAndExactOwners(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	cmd := exec.Command("git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range bytes.Split(out, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		if len(record) < 4 {
			t.Fatalf("ASSERT_STAGE7C_MALFORMED_PORCELAIN %q", record)
		}
		path := string(record[3:])
		allowed := map[string]bool{
			"internal/programc/stage7c_artifacts_red_test.go":            true,
			"internal/programc/stage7c_coupling.go":                      true,
			"internal/programc/stage7c_instability.go":                   true,
			"internal/programcrepresentative/source_context_red_test.go": true,
			"internal/programcrepresentative/stage7b_causal_red_test.go": true,
		}
		if !allowed[path] {
			t.Fatalf("ASSERT_STAGE7C_ONLY_AUTHORIZED_DIFF status=%q", record)
		}
	}
	for file, typ := range map[string]string{"stage7c_coupling.go": "RelationScopedCouplingArtifact", "stage7c_instability.go": "SuccessorInstabilityArtifact"} {
		path := filepath.Join(filepath.Dir(here), file)
		fset := token.NewFileSet()
		f, e := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if e != nil {
			t.Fatalf("ASSERT_STAGE7C_REQUIRED_PRODUCTION_OWNER file=%s err=%v", file, e)
		}
		found := false
		ast.Inspect(f, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == typ {
				found = true
			}
			return true
		})
		if !found {
			t.Fatalf("ASSERT_STAGE7C_EXACT_PRODUCTION_OWNER file=%s type=%s", file, typ)
		}
	}
	for _, productionFile := range []string{"stage7c_coupling.go", "stage7c_instability.go"} {
		path := filepath.Join(filepath.Dir(here), productionFile)
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, imported := range parsed.Imports {
			name := strings.ToLower(strings.Trim(imported.Path.Value, "\""))
			for _, forbidden := range []string{"cmd/", "internal/mcp", "adr0007", "stage7b"} {
				if strings.Contains(name, forbidden) {
					t.Fatalf("ASSERT_STAGE7C_FORBIDDEN_IMPORT file=%s import=%s", productionFile, name)
				}
			}
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				name := strings.ToLower(id.Name)
				for _, forbidden := range []string{"adr0007", "stage7b"} {
					if strings.Contains(name, forbidden) {
						t.Fatalf("ASSERT_STAGE7C_FORBIDDEN_API file=%s identifier=%s", productionFile, id.Name)
					}
				}
			}
			return true
		})
	}
	if reflect.TypeOf(RelationScopedCouplingArtifact{}).PkgPath() != reflect.TypeOf(SuccessorInstabilityArtifact{}).PkgPath() || reflect.TypeOf(RelationScopedCouplingArtifact{}).Name() == reflect.TypeOf(SuccessorInstabilityArtifact{}).Name() {
		t.Fatal("ASSERT_STAGE7C_TWO_DISTINCT_PRODUCTION_ARTIFACTS")
	}
}
