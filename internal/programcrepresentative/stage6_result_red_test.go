package programcrepresentative

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
)

// Stage6 production input and qualification carriers remain internal-only.
// Test fixtures use those production-owned immutable contracts directly.

type stage6Fixture struct {
	name        string
	composition RepresentativeComposition
	grouping    RepresentativeGrouping
	communities [][]string
}

func stage6Digest(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(append(raw, '\n'))
	return fmt.Sprintf("sha256:%x", sum)
}

func stage6SealFixture(f stage6Fixture) stage6Fixture {
	f.composition.Digest = ""
	f.composition.Digest = stage6Digest(f.composition)
	f.grouping.CompositionDigest = f.composition.Digest
	f.grouping.Digest = ""
	f.grouping.Digest = stage6Digest(f.grouping)
	return f
}

func stage6FixtureForMode(name string) stage6Fixture {
	call := func(id, from, to string, ordinal int) RepresentativeEvidence {
		return RepresentativeEvidence{Identity: id, RelationID: "relation-" + id, Kind: "CALLS", From: from, To: to, Disposition: "INCLUDED", SourcePredecessor: "pred-" + id, Custody: "custody-" + id, Ordinal: ordinal, CallSite: graph.Range{Start: graph.Position{Line: uint32(ordinal), Character: 1}, End: graph.Position{Line: uint32(ordinal), Character: 2}}}
	}
	ref := func(id, from, to string, ordinal int) RepresentativeEvidence {
		return RepresentativeEvidence{Identity: id, RelationID: "relation-" + id, Kind: "REFERENCES_SYMBOL", From: from, To: to, Disposition: "DOWNWEIGHTED", SourcePredecessor: "pred-" + id, Custody: "custody-" + id, Ordinal: ordinal}
	}
	def := func(id, from, to string, ordinal int) RepresentativeEvidence {
		return RepresentativeEvidence{Identity: id, RelationID: "relation-" + id, Kind: "RESOLVES_TO_DEFINITION", From: from, To: to, Disposition: "CAPPED", SourcePredecessor: "pred-" + id, Custody: "custody-" + id, Ordinal: ordinal}
	}
	occurrences := map[string][]RepresentativeEvidence{
		"calls-only":           {call("call-a", "root", "alpha", 0), call("call-b", "root", "beta", 1)},
		"refs-only":            {ref("ref-a", "zz-ref", "alpha", 0), ref("ref-b", "alpha", "beta", 1)},
		"defs-only":            {def("def-a", "zz-def", "alpha", 0)},
		"refs-defs":            {ref("ref-a", "zz-ref", "alpha", 0), def("def-a", "zz-def", "beta", 1)},
		"calls-refs":           {call("call-a", "root", "alpha", 0), call("call-b", "root", "beta", 1), ref("ref-a", "aaa-ref", "beta", 2)},
		"calls-defs":           {call("call-a", "root", "alpha", 0), call("call-b", "root", "beta", 1), def("def-a", "aaa-def", "beta", 2)},
		"fully-mixed":          {call("call-a", "root", "alpha", 0), call("call-b", "root", "beta", 1), ref("ref-a", "aaa-ref", "beta", 2), def("def-a", "aaa-def", "beta", 3)},
		"calls-ineligible":     {{Identity: "call-x", RelationID: "relation-call-x", Kind: "CALLS", From: "root", To: "alpha", Disposition: "EXCLUDED", SourcePredecessor: "pred-call-x", Custody: "custody-call-x"}},
		"multiple-calls":       {call("call-b", "root", "beta", 1), call("call-a", "root", "alpha", 0)},
		"self-loops-repeated":  {call("loop", "alpha", "alpha", 0), call("repeat-a", "root", "alpha", 1), call("repeat-b", "root", "alpha", 2)},
		"empty-zero-community": {},
	}[name]
	accounting := RepresentativeAccounting{Input: len(occurrences), Terminal: len(occurrences)}
	for _, occurrence := range occurrences {
		switch occurrence.Disposition {
		case "INCLUDED":
			accounting.Included++
		case "EXCLUDED":
			accounting.Excluded++
		case "DOWNWEIGHTED":
			accounting.Downweighted++
		case "CAPPED":
			accounting.Capped++
		}
	}
	communities := [][]string{{"alpha", "beta"}}
	contexts := []RepresentativeSelectionContext{{Identity: "context-" + name, ConstituentIdentity: "constituent-" + name, ExecutionBundleID: "bundle-" + name, SeedLabel: "seed", SeedAt: "root", CommunityMembers: []string{"alpha", "beta"}, PreparedTargets: []string{"root"}, ReachedNodes: []string{"alpha", "beta", "root"}}}
	for _, occurrence := range occurrences {
		if occurrence.Kind == "CALLS" {
			contexts[0].RelationIDs = append(contexts[0].RelationIDs, occurrence.RelationID)
			contexts[0].OccurrenceIDs = append(contexts[0].OccurrenceIDs, occurrence.Identity)
		}
	}
	sort.Strings(contexts[0].RelationIDs)
	sort.Strings(contexts[0].OccurrenceIDs)
	if name == "empty-zero-community" {
		communities, contexts = nil, nil
	}
	f := stage6Fixture{
		name:        name,
		composition: RepresentativeComposition{Family: "TEST_ONLY_STAGE5_COMPOSITION", Version: "v1", InputID: "input-" + name, InputDigest: stage6Digest("input-" + name), InputLength: len(name) + 6, PolicyID: "policy-" + name, PolicyDigest: stage6Digest("policy-" + name), PolicyLength: len(name) + 7, CompositionID: "composition-" + name, QualificationID: "qualification-" + name, QualificationDigest: stage6Digest("qualification-" + name), QualificationLength: len(name) + 14, Occurrences: occurrences, Accounting: accounting},
		grouping:    RepresentativeGrouping{Family: "REFERENCE_GROUPING_RESULT", Version: "v1", Algorithm: "TEST_ONLY_SUCCESSOR_MECHANICS", Profile: "REFERENCE_GROUPING_V1", NumericContract: "EXACT_REDUCED_INTEGER_RATIONAL_V1", LimitsDigest: stage6Digest("grouping-limits"), LimitsLength: len("grouping-limits"), Seed: 83, Partition: communities, SelectionContexts: contexts},
		communities: communities,
	}
	return stage6SealFixture(f)
}

func RepresentativeQualificationFor(f stage6Fixture, epoch uint64) RepresentativeQualification {
	return RepresentativeQualification{Family: "LSP_TRACE_TEST_STAGE6_MECHANICS_TOKEN", CompositionDigest: f.composition.Digest, EvidenceDigest: f.grouping.Digest, Trust: "TEST_ONLY_TRUST_ANCHOR", ValidFrom: epoch, ValidThrough: epoch + 2}
}

func stage6Verifier(want RepresentativeQualification, epoch uint64) func(RepresentativeQualification) bool {
	return func(got RepresentativeQualification) bool {
		return reflect.DeepEqual(got, want) && got.Family == "LSP_TRACE_TEST_STAGE6_MECHANICS_TOKEN" && got.Trust == "TEST_ONLY_TRUST_ANCHOR" && epoch >= got.ValidFrom && epoch <= got.ValidThrough
	}
}

func stage6Limits() RepresentativeLimits {
	return RepresentativeLimits{MaxCommunities: 32, MaxMembers: 128, MaxOccurrences: 256, MaxCandidates: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxWork: 4096}
}

func stage6VisibleEvidenceEqual(got []RepresentativeEvidence, want []RepresentativeEvidence) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Identity != w.Identity || g.RelationID != w.RelationID || g.Kind != w.Kind || g.From != w.From || g.To != w.To || g.Ordinal != w.Ordinal || g.CallSite != w.CallSite || g.SourcePredecessor != w.SourcePredecessor || g.Custody != w.Custody || g.Disposition != w.Disposition {
			return false
		}
	}
	return true
}

func assertStage6VisibleEvidence(t *testing.T, got []RepresentativeEvidence, want []RepresentativeEvidence) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ASSERT_STAGE6_VISIBLE_EVIDENCE_LENGTH got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Identity != w.Identity || g.RelationID != w.RelationID || g.Kind != w.Kind || g.From != w.From || g.To != w.To || g.Ordinal != w.Ordinal || g.CallSite != w.CallSite || g.SourcePredecessor != w.SourcePredecessor || g.Custody != w.Custody || g.Disposition != w.Disposition {
			t.Fatalf("ASSERT_STAGE6_VISIBLE_EVIDENCE_FIELDWISE index=%d got=%+v want=%+v", i, g, w)
		}
	}
}

func stage6Compute(f stage6Fixture, token RepresentativeQualification, verify func(RepresentativeQualification) bool) (RepresentativeResult, error) {
	return computeQualifiedRepresentativeResult(f.composition, f.grouping, token, verify, stage6Limits())
}

func TestStage6ExactModeOutcomeTableAndBalancedAccounting(t *testing.T) {
	cases := []struct {
		name, outcome, selected string
		eligible, ineligible    int
	}{
		{"calls-only", "SELECTED", "alpha", 2, 0},
		{"refs-only", "NO_CALLS_REPRESENTATIVE", "", 0, 0},
		{"defs-only", "NO_CALLS_REPRESENTATIVE", "", 0, 0},
		{"refs-defs", "NO_CALLS_REPRESENTATIVE", "", 0, 0},
		{"calls-refs", "SELECTED", "alpha", 2, 0},
		{"calls-defs", "SELECTED", "alpha", 2, 0},
		{"fully-mixed", "SELECTED", "alpha", 2, 0},
		{"calls-ineligible", "CALLS_EVIDENCE_INELIGIBLE", "", 0, 1},
		{"multiple-calls", "SELECTED", "alpha", 2, 0},
		{"self-loops-repeated", "SELECTED", "alpha", 2, 1},
		{"empty-zero-community", "EMPTY_COMMUNITY", "", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := stage6FixtureForMode(tc.name)
			token := RepresentativeQualificationFor(fixture, 8)
			got, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
			if err != nil {
				t.Fatal(err)
			}
			communities, a := got.Communities(), got.Accounting()
			if len(communities) != len(fixture.communities) && tc.outcome != "EMPTY_COMMUNITY" {
				t.Fatalf("ASSERT_STAGE6_MEMBERSHIP_VISIBLE got=%+v", got)
			}
			if got.Outcome() != tc.outcome || got.SelectedNode() != tc.selected || a.EligibleCalls != tc.eligible || a.IneligibleCalls != tc.ineligible {
				t.Fatalf("ASSERT_STAGE6_EXACT_OUTCOME mode=%s got=%+v", tc.name, got)
			}
			wantNonCalls, wantSelfLoops, wantRepeated := 0, 0, 0
			seen := map[string]int{}
			for _, occurrence := range fixture.composition.Occurrences {
				if occurrence.Kind != "CALLS" {
					wantNonCalls++
					continue
				}
				if occurrence.From == occurrence.To {
					wantSelfLoops++
				}
				key := occurrence.From + "\x00" + occurrence.To
				seen[key]++
				if seen[key] > 1 {
					wantRepeated++
				}
			}
			terminalCategories := a.Included + a.Excluded + a.Downweighted + a.Capped + a.Normalized + a.SeparateLayer
			if a.Input != len(fixture.composition.Occurrences) || a.Terminal != a.Input || terminalCategories != a.Terminal || a.NonCalls != wantNonCalls || a.SelfLoops != wantSelfLoops || a.RepeatedCalls != wantRepeated || a.Selected != btoi(tc.outcome == "SELECTED") || a.NoCalls != btoi(tc.outcome == "NO_CALLS_REPRESENTATIVE") || a.CallsIneligible != btoi(tc.outcome == "CALLS_EVIDENCE_INELIGIBLE") || a.EmptyCommunity != btoi(tc.outcome == "EMPTY_COMMUNITY") || a.EligibleCalls != tc.eligible || a.IneligibleCalls != tc.ineligible || a.EligibleCalls+a.IneligibleCalls != a.Input-a.NonCalls || a.SelfLoops > a.IneligibleCalls || a.RepeatedCalls > a.EligibleCalls || a.NonCalls != a.Downweighted+a.Capped+a.Normalized+a.SeparateLayer || a.Selected+a.NoCalls+a.CallsIneligible+a.EmptyCommunity != 1 || a.Duplicates != 0 || a.Truncated {
				t.Fatalf("ASSERT_STAGE6_EXACT_ACCOUNTING mode=%s accounting=%+v", tc.name, a)
			}
		})
	}
}

func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestStage6MixedSelectionIgnoresDefinitionAndReferenceInfluence(t *testing.T) {
	base := stage6FixtureForMode("fully-mixed")
	token := RepresentativeQualificationFor(base, 8)
	want, err := stage6Compute(base, token, stage6Verifier(token, 8))
	if err != nil || want.Outcome() != "SELECTED" {
		t.Fatalf("baseline=%+v err=%v", want, err)
	}
	wantRepresentative := want.Representatives()
	wantEvidence := want.VisibleEvidence()
	mutations := map[string]func(*stage6Fixture){
		"degree": func(f *stage6Fixture) {
			f.composition.Occurrences = append(f.composition.Occurrences, RepresentativeEvidence{Identity: "ref-extra", Kind: "REFERENCES_SYMBOL", From: "z", To: "beta", Disposition: "DOWNWEIGHTED", SourcePredecessor: "pred-extra", Custody: "custody-extra", Ordinal: 9})
			f.composition.Accounting.Input++
			f.composition.Accounting.Terminal++
			f.composition.Accounting.Downweighted++
		},
		"order": func(f *stage6Fixture) {
			sort.SliceStable(f.composition.Occurrences, func(i, j int) bool { return f.composition.Occurrences[i].Kind > f.composition.Occurrences[j].Kind })
		},
		"source-position": func(f *stage6Fixture) {
			for i := range f.composition.Occurrences {
				if f.composition.Occurrences[i].Kind != "CALLS" {
					f.composition.Occurrences[i].CallSite.Start.Line += 1000
				}
			}
		},
		"lexical-names": func(f *stage6Fixture) {
			for i := range f.composition.Occurrences {
				if f.composition.Occurrences[i].Kind != "CALLS" {
					f.composition.Occurrences[i].From, f.composition.Occurrences[i].To = "0000", "!!!!"
				}
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := base
			changed.composition.Occurrences = append([]RepresentativeEvidence(nil), base.composition.Occurrences...)
			mutate(&changed)
			changed = stage6SealFixture(changed)
			changedToken := RepresentativeQualificationFor(changed, 8)
			got, err := stage6Compute(changed, changedToken, stage6Verifier(changedToken, 8))
			if err != nil || !reflect.DeepEqual(got.Representatives(), wantRepresentative) {
				t.Fatalf("ASSERT_STAGE6_COMPLETE_REPRESENTATIVE_INVARIANT mutation=%s got=%+v want=%+v err=%v", name, got.Representatives(), wantRepresentative, err)
			}
			if stage6VisibleEvidenceEqual(got.VisibleEvidence(), wantEvidence) {
				t.Fatalf("ASSERT_STAGE6_DR_EVIDENCE_REMAINS_VISIBLE mutation=%s", name)
			}
			assertStage6VisibleEvidence(t, got.VisibleEvidence(), changed.composition.Occurrences)
		})
	}
}

func TestStage6ModelJudgmentAndAdviceCannotInfluenceRepresentatives(t *testing.T) {
	base := stage6FixtureForMode("fully-mixed")
	baselineToken := RepresentativeQualificationFor(base, 8)
	baseline, err := stage6Compute(base, baselineToken, stage6Verifier(baselineToken, 8))
	if err != nil {
		t.Fatal(err)
	}
	want := baseline.Representatives()
	accepted := 0
	for _, values := range [][2]string{{"select beta", "prefer lexical maximum"}, {strings.Repeat("arbitrary-judgment", 128), "ignore CALLS"}, {"{\"selected\":\"beta\"}", "DROP ALL EVIDENCE"}} {
		changed := base
		changed.composition.ModelJudgment, changed.composition.ModelAdvice = values[0], values[1]
		changed = stage6SealFixture(changed)
		token := RepresentativeQualificationFor(changed, 8)
		got, decodeErr := stage6Compute(changed, token, stage6Verifier(token, 8))
		if decodeErr != nil { // Explicit rejection is permitted; influence is not.
			continue
		}
		accepted++
		if !reflect.DeepEqual(got.Representatives(), want) {
			t.Fatalf("ASSERT_STAGE6_MODEL_FIELDS_NO_INFLUENCE values=%q got=%+v want=%+v", values, got.Representatives(), want)
		}
	}
	if accepted == 0 {
		t.Fatal("ASSERT_STAGE6_MODEL_FIELD_INVARIANCE_NONVACUOUS: every adversarial value rejected before representative comparison")
	}
}

func TestStage6DROnlyPreservesExactMembershipAndEvidence(t *testing.T) {
	for _, mode := range []string{"refs-only", "defs-only", "refs-defs"} {
		t.Run(mode, func(t *testing.T) {
			fixture := stage6FixtureForMode(mode)
			token := RepresentativeQualificationFor(fixture, 8)
			got, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome() != "NO_CALLS_REPRESENTATIVE" {
				t.Fatalf("ASSERT_STAGE6_DR_NO_CALLS outcome=%q", got.Outcome())
			}
			if !reflect.DeepEqual(got.Communities(), fixture.communities) {
				t.Fatalf("ASSERT_STAGE6_DR_EXACT_MEMBERSHIP got=%v want=%v", got.Communities(), fixture.communities)
			}
			assertStage6VisibleEvidence(t, got.VisibleEvidence(), fixture.composition.Occurrences)
			for _, representative := range got.Representatives() {
				if representative.Outcome != "NO_CALLS_REPRESENTATIVE" {
					t.Fatalf("ASSERT_STAGE6_DR_COMMUNITY_NO_CALLS got=%+v", representative)
				}
			}
		})
	}
}

func TestStage6HistoricalCallsSelectionAndTieBreakGolden(t *testing.T) {
	alpha, beta := rnode("alpha"), rnode("beta")
	nodes := []graph.Node{alpha, beta}
	edges := []graph.Edge{redge(alpha, beta)}
	historicalAdmission := admitted(t, memberships(nodes, edges, "historical"), nodes, edges)
	historical, err := Select(Input{Admission: historicalAdmission, Communities: []programc.Community{{Members: []string{beta.ID}}}})
	if err != nil || len(historical.Nominations) != 1 {
		t.Fatalf("ASSERT_STAGE6_REAL_HISTORICAL_SELECT output=%+v err=%v", historical, err)
	}
	want := historical.Nominations[0]
	fixture := stage6FixtureForMode("calls-only")
	fixture.composition.Occurrences = fixture.composition.Occurrences[:1]
	fixture.composition.Accounting.Input = 1
	fixture.composition.Accounting.Terminal = 1
	fixture.composition.Accounting.Included = 1
	for _, predecessor := range want.IncomingPredecessors {
		for i := range fixture.composition.Occurrences {
			if predecessor.TargetID == beta.ID {
				fixture.composition.Occurrences[i].Identity = predecessor.OccurrenceID
				fixture.composition.Occurrences[i].RelationID = predecessor.RelationID
				fixture.composition.Occurrences[i].From = predecessor.CallerID
				fixture.composition.Occurrences[i].To = predecessor.TargetID
				fixture.composition.Occurrences[i].CallSite = predecessor.CallSite
			}
		}
	}
	fixture.grouping.Partition = [][]string{{beta.ID}}
	fixture.grouping.SelectionContexts = []RepresentativeSelectionContext{{Identity: "historical-context", ConstituentIdentity: want.ConstituentIdentity, ExecutionBundleID: want.ExecutionBundleID, SeedLabel: want.SeedLabel, SeedAt: want.SeedAt, ConstituentOrdinal: want.ConstituentOrdinal, CommunityMembers: []string{beta.ID}, PreparedTargets: append([]string(nil), want.PreparedTargets...), ReachedNodes: []string{alpha.ID, beta.ID}, RelationIDs: []string{fixture.composition.Occurrences[0].RelationID}, OccurrenceIDs: []string{fixture.composition.Occurrences[0].Identity}}}
	fixture.communities = [][]string{{beta.ID}}
	fixture = stage6SealFixture(fixture)
	token := RepresentativeQualificationFor(fixture, 8)
	got, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
	if err != nil {
		t.Fatal(err)
	}
	representatives := got.Representatives()
	wantIncoming := make([]RepresentativePredecessor, len(want.IncomingPredecessors))
	for i, predecessor := range want.IncomingPredecessors {
		wantIncoming[i] = RepresentativePredecessor{OccurrenceID: predecessor.OccurrenceID, RelationID: predecessor.RelationID, CallerID: predecessor.CallerID, TargetID: predecessor.TargetID, CallSite: predecessor.CallSite}
	}
	wantRecord := RepresentativeRecord{Outcome: "SELECTED", SelectedNode: want.SelectedNode, Distance: want.Distance, SCCMembers: append([]string(nil), want.SCCMembers...), IncomingPredecessors: wantIncoming}
	if len(representatives) != 1 || !reflect.DeepEqual(representatives[0], wantRecord) {
		t.Fatalf("ASSERT_STAGE6_HISTORICAL_COMPLETE_RECORD_EQUIVALENCE got=%+v historical=%+v", representatives, wantRecord)
	}
	gotProjection, _ := json.Marshal(representatives[0])
	wantProjection, _ := json.Marshal(wantRecord)
	if !bytes.Equal(gotProjection, wantProjection) || stage6Digest(representatives[0]) != stage6Digest(wantRecord) {
		t.Fatalf("ASSERT_STAGE6_HISTORICAL_CALLS_CANONICAL_EQUIVALENCE got=%s want=%s", gotProjection, wantProjection)
	}
}

func TestStage6ResultHasNoExportedSliceFields(t *testing.T) {
	typ := reflect.TypeOf(RepresentativeResult{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() && field.Type.Kind() == reflect.Slice {
			t.Fatalf("ASSERT_STAGE6_NO_EXPORTED_SLICE_FIELDS field=%s", field.Name)
		}
	}
}

func TestStage6CanonicalIdentityReplayCopiesAndSubstitutionRefusal(t *testing.T) {
	fixture := stage6FixtureForMode("fully-mixed")
	token := RepresentativeQualificationFor(fixture, 8)
	got, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := replayQualifiedRepresentativeResult(fixture.composition, fixture.grouping, token, stage6Verifier(token, 8), stage6Limits(), got)
	if err != nil || !reflect.DeepEqual(replayed, got) || !bytes.Equal(replayed.CanonicalBytes(), got.CanonicalBytes()) {
		t.Fatalf("ASSERT_STAGE6_EXACT_REPLAY got=%+v err=%v", replayed, err)
	}
	first, second := got.CanonicalBytes(), got.CanonicalBytes()
	first[0] ^= 0xff
	communities := got.Communities()
	communities[0][0] = "mutated"
	evidence := got.VisibleEvidence()
	evidence[0].From = "mutated"
	representatives := got.Representatives()
	representatives[0].SCCMembers[0] = "mutated"
	fixture.composition.Occurrences[0].From = "source-mutated"
	fixture.communities[0][0] = "source-mutated"
	if bytes.Equal(first, second) || !bytes.Equal(second, got.CanonicalBytes()) || got.Communities()[0][0] == "mutated" || got.VisibleEvidence()[0].From == "mutated" || got.Representatives()[0].SCCMembers[0] == "mutated" || got.VisibleEvidence()[0].From == "source-mutated" || got.Communities()[0][0] == "source-mutated" {
		t.Fatal("ASSERT_STAGE6_DEFENSIVE_COPIES")
	}
	if replayedAgain, replayErr := replayQualifiedRepresentativeResult(stage6FixtureForMode("fully-mixed").composition, stage6FixtureForMode("fully-mixed").grouping, token, stage6Verifier(token, 8), stage6Limits(), got); replayErr != nil || !bytes.Equal(replayedAgain.CanonicalBytes(), second) {
		t.Fatalf("ASSERT_STAGE6_MUTATION_CANNOT_CHANGE_REPLAY replay=%+v err=%v", replayedAgain, replayErr)
	}

	mutations := stage6ExhaustiveSubstitutions()
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			cleanFixture := stage6FixtureForMode("fully-mixed")
			cleanToken := RepresentativeQualificationFor(cleanFixture, 8)
			cleanBase, cleanErr := stage6Compute(cleanFixture, cleanToken, stage6Verifier(cleanToken, 8))
			if cleanErr != nil {
				t.Fatal(cleanErr)
			}
			changedFixture := stage6CloneFixture(cleanFixture)
			mutation.apply(&changedFixture)
			if replayed, replayErr := replayQualifiedRepresentativeResult(changedFixture.composition, changedFixture.grouping, cleanToken, stage6Verifier(cleanToken, 8), stage6Limits(), cleanBase); replayErr == nil || !reflect.DeepEqual(replayed, RepresentativeResult{}) || len(replayed.CanonicalBytes()) != 0 {
				t.Fatalf("ASSERT_STAGE6_SUBSTITUTION_ATOMIC_ZERO mutation=%s got=%+v err=%v", mutation.name, replayed, replayErr)
			}
		})
	}
	if stale, staleErr := stage6Compute(fixture, token, stage6Verifier(token, 11)); staleErr == nil || !reflect.DeepEqual(stale, RepresentativeResult{}) {
		t.Fatalf("ASSERT_STAGE6_STALE_QUALIFICATION_ATOMIC_ZERO got=%+v err=%v", stale, staleErr)
	}
	if refused, refusal := computeQualifiedRepresentativeResult(fixture.composition, fixture.grouping, token, nil, stage6Limits()); refusal == nil || !reflect.DeepEqual(refused, RepresentativeResult{}) {
		t.Fatalf("ASSERT_STAGE6_REAL_SEAM_REFUSES_UNQUALIFIED got=%+v err=%v", refused, refusal)
	}
}

type stage6Substitution struct {
	name  string
	apply func(*stage6Fixture)
}

func stage6ExhaustiveSubstitutions() []stage6Substitution {
	o := func(name string, apply func(*RepresentativeEvidence)) stage6Substitution {
		return stage6Substitution{name, func(f *stage6Fixture) { apply(&f.composition.Occurrences[0]) }}
	}
	return []stage6Substitution{
		{"composition-family", func(f *stage6Fixture) { f.composition.Family += "-sub" }},
		{"composition-version", func(f *stage6Fixture) { f.composition.Version += "-sub" }},
		{"grouping-family", func(f *stage6Fixture) { f.grouping.Family += "-sub" }},
		{"grouping-version", func(f *stage6Fixture) { f.grouping.Version += "-sub" }},
		{"grouping-algorithm", func(f *stage6Fixture) { f.grouping.Algorithm += "-sub" }},
		{"grouping-profile", func(f *stage6Fixture) { f.grouping.Profile += "-sub" }},
		{"grouping-numeric", func(f *stage6Fixture) { f.grouping.NumericContract += "-sub" }},
		{"grouping-seed", func(f *stage6Fixture) { f.grouping.Seed++ }},
		{"grouping-limits-digest", func(f *stage6Fixture) { f.grouping.LimitsDigest = stage6Digest("other-limits") }},
		{"grouping-limits-length", func(f *stage6Fixture) { f.grouping.LimitsLength++ }},
		{"input-id", func(f *stage6Fixture) { f.composition.InputID += "-sub" }},
		{"input-digest", func(f *stage6Fixture) { f.composition.InputDigest = stage6Digest("other-input") }},
		{"input-length", func(f *stage6Fixture) { f.composition.InputLength++ }},
		{"policy-id", func(f *stage6Fixture) { f.composition.PolicyID += "-sub" }},
		{"policy-digest", func(f *stage6Fixture) { f.composition.PolicyDigest = stage6Digest("other-policy") }},
		{"policy-length", func(f *stage6Fixture) { f.composition.PolicyLength++ }},
		{"composition-id", func(f *stage6Fixture) { f.composition.CompositionID += "-sub" }},
		{"composition-digest", func(f *stage6Fixture) { f.composition.Digest = stage6Digest("other-composition") }},
		{"composition-length", func(f *stage6Fixture) { f.grouping.CompositionLength++ }},
		{"qualification-id", func(f *stage6Fixture) { f.composition.QualificationID += "-sub" }},
		{"qualification-digest", func(f *stage6Fixture) { f.composition.QualificationDigest = stage6Digest("other-qualification") }},
		{"qualification-length", func(f *stage6Fixture) { f.composition.QualificationLength++ }},
		{"partition-membership", func(f *stage6Fixture) { f.grouping.Partition[0][0] = "other" }},
		{"partition-order", func(f *stage6Fixture) {
			f.grouping.Partition[0][0], f.grouping.Partition[0][1] = f.grouping.Partition[0][1], f.grouping.Partition[0][0]
		}},
		{"partition-duplicate", func(f *stage6Fixture) {
			f.grouping.Partition = append(f.grouping.Partition, append([]string(nil), f.grouping.Partition[0]...))
		}},
		o("occurrence-identity", func(x *RepresentativeEvidence) { x.Identity += "-sub" }),
		o("occurrence-kind", func(x *RepresentativeEvidence) { x.Kind = "REFERENCES_SYMBOL" }),
		o("occurrence-from", func(x *RepresentativeEvidence) { x.From = "other" }),
		o("occurrence-to", func(x *RepresentativeEvidence) { x.To = "other" }),
		o("occurrence-ordinal", func(x *RepresentativeEvidence) { x.Ordinal++ }),
		o("occurrence-call-site", func(x *RepresentativeEvidence) { x.CallSite.Start.Line++ }),
		o("occurrence-source-predecessor", func(x *RepresentativeEvidence) { x.SourcePredecessor += "-sub" }),
		o("occurrence-custody", func(x *RepresentativeEvidence) { x.Custody += "-sub" }),
		o("occurrence-disposition", func(x *RepresentativeEvidence) { x.Disposition = "EXCLUDED" }),
		{"representative-membership", func(f *stage6Fixture) { f.grouping.Partition = [][]string{{"beta"}} }},
		{"accounting-input", func(f *stage6Fixture) { f.composition.Accounting.Input++ }},
		{"accounting-terminal", func(f *stage6Fixture) { f.composition.Accounting.Terminal++ }},
		{"accounting-included", func(f *stage6Fixture) { f.composition.Accounting.Included++ }},
		{"accounting-excluded", func(f *stage6Fixture) { f.composition.Accounting.Excluded++ }},
		{"accounting-downweighted", func(f *stage6Fixture) { f.composition.Accounting.Downweighted++ }},
		{"accounting-capped", func(f *stage6Fixture) { f.composition.Accounting.Capped++ }},
		{"accounting-normalized", func(f *stage6Fixture) { f.composition.Accounting.Normalized++ }},
		{"accounting-separate-layer", func(f *stage6Fixture) { f.composition.Accounting.SeparateLayer++ }},
	}
}

func stage6CloneFixture(in stage6Fixture) stage6Fixture {
	out := in
	out.composition.Occurrences = append([]RepresentativeEvidence(nil), in.composition.Occurrences...)
	out.grouping.Partition = cloneStage6Communities(in.grouping.Partition)
	out.grouping.SelectionContexts = cloneStage6Contexts(in.grouping.SelectionContexts)
	out.communities = cloneStage6Communities(in.communities)
	return out
}

func cloneStage6Communities(in [][]string) [][]string {
	out := make([][]string, len(in))
	for i := range in {
		out[i] = append([]string(nil), in[i]...)
	}
	return out
}

func cloneStage6Contexts(in []RepresentativeSelectionContext) []RepresentativeSelectionContext {
	out := append([]RepresentativeSelectionContext(nil), in...)
	for i := range out {
		out[i].CommunityMembers = append([]string(nil), in[i].CommunityMembers...)
		out[i].PreparedTargets = append([]string(nil), in[i].PreparedTargets...)
		out[i].ReachedNodes = append([]string(nil), in[i].ReachedNodes...)
		out[i].RelationIDs = append([]string(nil), in[i].RelationIDs...)
		out[i].OccurrenceIDs = append([]string(nil), in[i].OccurrenceIDs...)
	}
	return out
}

func TestStage6QualificationAndLimitSubstitutionsAreIndependent(t *testing.T) {
	tokenMutations := []struct {
		name  string
		apply func(*RepresentativeQualification)
	}{
		{"token-family", func(v *RepresentativeQualification) { v.Family += "-sub" }},
		{"token-composition-digest", func(v *RepresentativeQualification) { v.CompositionDigest = stage6Digest("other-composition") }},
		{"token-evidence-digest", func(v *RepresentativeQualification) { v.EvidenceDigest = stage6Digest("other-evidence") }},
		{"token-trust", func(v *RepresentativeQualification) { v.Trust += "-sub" }},
		{"token-valid-from", func(v *RepresentativeQualification) { v.ValidFrom++ }},
		{"token-valid-through", func(v *RepresentativeQualification) { v.ValidThrough-- }},
	}
	for _, mutation := range tokenMutations {
		t.Run(mutation.name, func(t *testing.T) {
			fixture := stage6FixtureForMode("fully-mixed")
			clean := RepresentativeQualificationFor(fixture, 8)
			changed := clean
			mutation.apply(&changed)
			if got, err := stage6Compute(fixture, changed, stage6Verifier(clean, 8)); err == nil || !reflect.DeepEqual(got, RepresentativeResult{}) || len(got.CanonicalBytes()) != 0 {
				t.Fatalf("ASSERT_STAGE6_TOKEN_SUBSTITUTION_ATOMIC_ZERO mutation=%s got=%+v err=%v", mutation.name, got, err)
			}
		})
	}
	limitMutations := []struct {
		name  string
		apply func(*RepresentativeLimits)
	}{
		{"limit-communities", func(v *RepresentativeLimits) { v.MaxCommunities++ }},
		{"limit-members", func(v *RepresentativeLimits) { v.MaxMembers++ }},
		{"limit-occurrences", func(v *RepresentativeLimits) { v.MaxOccurrences++ }},
		{"limit-candidates", func(v *RepresentativeLimits) { v.MaxCandidates++ }},
		{"limit-canonical-bytes", func(v *RepresentativeLimits) { v.MaxCanonicalBytes++ }},
		{"limit-string-bytes", func(v *RepresentativeLimits) { v.MaxStringBytes++ }},
		{"limit-work", func(v *RepresentativeLimits) { v.MaxWork++ }},
	}
	for _, mutation := range limitMutations {
		t.Run(mutation.name, func(t *testing.T) {
			fixture := stage6FixtureForMode("fully-mixed")
			token := RepresentativeQualificationFor(fixture, 8)
			base, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
			if err != nil {
				t.Fatal(err)
			}
			limits := stage6Limits()
			mutation.apply(&limits)
			if got, replayErr := replayQualifiedRepresentativeResult(fixture.composition, fixture.grouping, token, stage6Verifier(token, 8), limits, base); replayErr == nil || !reflect.DeepEqual(got, RepresentativeResult{}) || len(got.CanonicalBytes()) != 0 {
				t.Fatalf("ASSERT_STAGE6_LIMIT_SUBSTITUTION_ATOMIC_ZERO mutation=%s got=%+v err=%v", mutation.name, got, replayErr)
			}
		})
	}
}

func TestStage6GenuineConcurrentReplayIsByteIdentical(t *testing.T) {
	fixture := stage6FixtureForMode("self-loops-repeated")
	token := RepresentativeQualificationFor(fixture, 8)
	baseline, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
	if err != nil {
		t.Fatal(err)
	}
	const workers = 32
	results := make([]RepresentativeResult, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range results {
		i := i
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = replayQualifiedRepresentativeResult(fixture.composition, fixture.grouping, token, stage6Verifier(token, 8), stage6Limits(), baseline)
		}()
	}
	close(start)
	wg.Wait()
	identity := append([]byte(nil), baseline.CanonicalBytes()...)
	var mutationWG sync.WaitGroup
	for i := range results {
		if errs[i] != nil || !bytes.Equal(results[i].CanonicalBytes(), identity) || results[i].Digest != baseline.Digest {
			t.Fatalf("ASSERT_STAGE6_GENUINE_CONCURRENCY i=%d got=%+v err=%v", i, results[i], errs[i])
		}
		mutationWG.Add(1)
		go func(result RepresentativeResult) {
			defer mutationWG.Done()
			canonical := result.CanonicalBytes()
			communities := result.Communities()
			evidence := result.VisibleEvidence()
			representatives := result.Representatives()
			canonical[0] ^= 0xff
			communities[0][0] = "mutated"
			evidence[0].From = "mutated"
			representatives[0].SCCMembers[0] = "mutated"
		}(results[i])
	}
	mutationWG.Wait()
	for i := range results {
		if !bytes.Equal(results[i].CanonicalBytes(), identity) || results[i].Digest != baseline.Digest {
			t.Fatalf("ASSERT_STAGE6_CONCURRENT_ACCESSOR_MUTATION_CHANGED_IDENTITY i=%d", i)
		}
	}
}

func TestStage6ContextBoundSelectionAndPerCommunityOutcomes(t *testing.T) {
	fixture := stage6FixtureForMode("calls-only")
	fixture.grouping.Partition = [][]string{{"alpha"}, {"beta"}}
	base := fixture.grouping.SelectionContexts[0]
	alphaContext, betaContext := base, base
	alphaContext.Identity, alphaContext.CommunityMembers = "context-alpha", []string{"alpha"}
	betaContext.Identity, betaContext.CommunityMembers = "context-beta", []string{"beta"}
	fixture.grouping.SelectionContexts = []RepresentativeSelectionContext{alphaContext, betaContext}
	fixture.communities = cloneStage6Communities(fixture.grouping.Partition)
	fixture = stage6SealFixture(fixture)
	token := RepresentativeQualificationFor(fixture, 8)
	got, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
	if err != nil {
		t.Fatal(err)
	}
	records := got.Representatives()
	if got.Outcome() != "SELECTED" || got.SelectedNode() != "" || len(records) != 2 || records[0].SelectedNode != "alpha" || records[1].SelectedNode != "beta" || got.Accounting().Selected != 2 {
		t.Fatalf("ASSERT_STAGE6_MULTI_COMMUNITY_UNAMBIGUOUS got=%+v records=%+v accounting=%+v", got, records, got.Accounting())
	}

	isolated := stage6CloneFixture(fixture)
	isolated.grouping.SelectionContexts[1].RelationIDs = nil
	isolated.grouping.SelectionContexts[1].OccurrenceIDs = nil
	isolated = stage6SealFixture(isolated)
	isolatedToken := RepresentativeQualificationFor(isolated, 8)
	got, err = stage6Compute(isolated, isolatedToken, stage6Verifier(isolatedToken, 8))
	if err != nil {
		t.Fatal(err)
	}
	records = got.Representatives()
	if got.Outcome() != "MIXED_COMMUNITY_OUTCOMES" || got.SelectedNode() != "" || records[0].Outcome != "SELECTED" || records[1].Outcome != "CALLS_EVIDENCE_INELIGIBLE" {
		t.Fatalf("ASSERT_STAGE6_SEED_ISOLATION got=%+v records=%+v", got, records)
	}

	unreachable := stage6CloneFixture(fixture)
	unreachable.grouping.SelectionContexts[1].PreparedTargets = []string{"beta"}
	unreachable.grouping.SelectionContexts[1].ReachedNodes = []string{"beta", "root"}
	unreachable = stage6SealFixture(unreachable)
	unreachableToken := RepresentativeQualificationFor(unreachable, 8)
	got, err = stage6Compute(unreachable, unreachableToken, stage6Verifier(unreachableToken, 8))
	if err != nil || got.Representatives()[1].SelectedNode != "beta" || got.Representatives()[1].Distance != 0 {
		t.Fatalf("ASSERT_STAGE6_PREPARED_ROOT_NOT_ZERO_INDEGREE got=%+v err=%v", got.Representatives(), err)
	}
}

func TestStage6RejectsOverlappingPartitionAndInvalidAccounting(t *testing.T) {
	t.Run("overlap", func(t *testing.T) {
		fixture := stage6FixtureForMode("calls-only")
		fixture.grouping.Partition = [][]string{{"alpha"}, {"alpha", "beta"}}
		first, second := fixture.grouping.SelectionContexts[0], fixture.grouping.SelectionContexts[0]
		first.Identity, first.CommunityMembers = "context-a", []string{"alpha"}
		second.Identity, second.CommunityMembers = "context-b", []string{"alpha", "beta"}
		fixture.grouping.SelectionContexts = []RepresentativeSelectionContext{first, second}
		fixture = stage6SealFixture(fixture)
		token := RepresentativeQualificationFor(fixture, 8)
		if got, err := stage6Compute(fixture, token, stage6Verifier(token, 8)); err == nil || !reflect.DeepEqual(got, RepresentativeResult{}) {
			t.Fatalf("ASSERT_STAGE6_OVERLAP_REJECTED got=%+v err=%v", got, err)
		}
	})
	cases := []struct {
		name   string
		mutate func(*RepresentativeAccounting)
	}{
		{"negative", func(a *RepresentativeAccounting) { a.Excluded = -1 }},
		{"overflow", func(a *RepresentativeAccounting) { a.Included = int(^uint(0) >> 1); a.Excluded = 1 }},
		{"forged-derived", func(a *RepresentativeAccounting) { a.Duplicates = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := stage6FixtureForMode("calls-only")
			tc.mutate(&fixture.composition.Accounting)
			fixture = stage6SealFixture(fixture)
			token := RepresentativeQualificationFor(fixture, 8)
			if got, err := stage6Compute(fixture, token, stage6Verifier(token, 8)); err == nil || !reflect.DeepEqual(got, RepresentativeResult{}) {
				t.Fatalf("ASSERT_STAGE6_ACCOUNTING_REJECTED got=%+v err=%v", got, err)
			}
		})
	}
}

func TestStage6ScopeHasNoNonRuntimeOrPublicationCarriers(t *testing.T) {
	forbidden := []string{"medoid", "focal", "model", "packet", "snapshot", "checkpoint", "instability", "responsibility", "public", "host"}
	for _, typ := range []reflect.Type{reflect.TypeOf(RepresentativeResult{}), reflect.TypeOf(RepresentativeRecord{})} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			for _, word := range forbidden {
				if strings.Contains(name, word) {
					t.Fatalf("ASSERT_STAGE6_SCOPE_FORBIDDEN_FIELD type=%s field=%s word=%s", typ, typ.Field(i).Name, word)
				}
			}
		}
		for _, methodType := range []reflect.Type{typ, reflect.PointerTo(typ)} {
			for i := 0; i < methodType.NumMethod(); i++ {
				name := strings.ToLower(methodType.Method(i).Name)
				for _, word := range forbidden {
					if strings.Contains(name, word) {
						t.Fatalf("ASSERT_STAGE6_SCOPE_FORBIDDEN_METHOD type=%s method=%s word=%s", methodType, methodType.Method(i).Name, word)
					}
				}
			}
		}
	}
}

func TestStage6SelectionContextRequiresExactRelationOccurrenceClosure(t *testing.T) {
	t.Run("one-of-two-same-relation-omitted", func(t *testing.T) {
		fixture := stage6FixtureForMode("calls-only")
		sharedRelation := fixture.composition.Occurrences[0].RelationID
		fixture.composition.Occurrences[1].RelationID = sharedRelation
		fixture.grouping.SelectionContexts[0].RelationIDs = []string{sharedRelation}
		fixture.grouping.SelectionContexts[0].OccurrenceIDs = []string{fixture.composition.Occurrences[0].Identity}
		fixture = stage6SealFixture(fixture)
		token := RepresentativeQualificationFor(fixture, 8)
		got, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
		if err == nil || !reflect.DeepEqual(got, RepresentativeResult{}) || len(got.CanonicalBytes()) != 0 {
			t.Fatalf("ASSERT_STAGE6_SAME_RELATION_PARTIAL_CLOSURE_ATOMIC_ZERO got=%+v err=%v", got, err)
		}
	})

	cases := []struct {
		name   string
		mutate func(*RepresentativeSelectionContext)
	}{
		{"omitted-occurrence", func(c *RepresentativeSelectionContext) { c.OccurrenceIDs = nil }},
		{"cross-relation-occurrence", func(c *RepresentativeSelectionContext) {
			c.RelationIDs = c.RelationIDs[:1]
			c.OccurrenceIDs = c.OccurrenceIDs[1:2]
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := stage6FixtureForMode("calls-only")
			tc.mutate(&fixture.grouping.SelectionContexts[0])
			fixture = stage6SealFixture(fixture)
			token := RepresentativeQualificationFor(fixture, 8)
			got, err := stage6Compute(fixture, token, stage6Verifier(token, 8))
			if err == nil || !reflect.DeepEqual(got, RepresentativeResult{}) || len(got.CanonicalBytes()) != 0 {
				t.Fatalf("ASSERT_STAGE6_RELATION_OCCURRENCE_CLOSURE_ATOMIC_ZERO got=%+v err=%v", got, err)
			}
		})
	}
}
