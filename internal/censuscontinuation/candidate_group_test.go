package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programctestfixture"
	"lsp-trace/internal/sourceprojection"
)

const (
	fixturePartition = "sha256:6d4c2e16bfb292d29e7649d89118354e3a3e95c7ec00c0ba35ac9b8d3cf6104a"
	fixtureMemberA   = "3cdff2e81db031666b6d6d4d13711ddc2749722ac804ef79b103463ebfdda93c"
	fixtureMemberB   = "4a4c524d563487befdff3720d59fb28740155453ebe6625b782f7612ef9e445f"
	fixtureSingleton = "020b42e6f4356621d0dbfd50da9ee760ede90980ce036960fd05dafa7113c80b"
)

func fixtureCandidateInput(t testing.TB) CandidateGroupInput {
	t.Helper()
	o, failure := programc.Compute(programctestfixture.ValidV5(t), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	b, err := programc.ComputeBoundary(o, programc.BoundaryRequest{PageRankTopK: 2, HubTopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	return CandidateGroupInput{
		Outcome: o, Boundary: b, CommunityMembers: []string{fixtureMemberA, fixtureMemberB},
		RepresentativeID: fixtureMemberA, Bounds: FrozenCandidateGroupBounds(),
		Interpretation: HostInterpretation{Status: InterpretationUnresolved, HostID: "caller-host", ModelID: "caller-model", ContextID: "caller-context", Attempts: 1},
	}
}

func TestCandidateGroupFixtureIdentityAndCustody(t *testing.T) {
	in := fixtureCandidateInput(t)
	if len(in.Outcome.Communities) != 2 || in.Outcome.LogicalDigest != fixturePartition {
		t.Fatalf("fixture partition drift: communities=%d digest=%s", len(in.Outcome.Communities), in.Outcome.LogicalDigest)
	}
	if !reflect.DeepEqual(in.Outcome.Communities[1].Members, []string{fixtureMemberA, fixtureMemberB}) {
		t.Fatalf("selected community drift: %#v", in.Outcome.Communities)
	}
	if !reflect.DeepEqual(in.Outcome.Communities[0].Members, []string{fixtureSingleton}) {
		t.Fatalf("singleton drift: %#v", in.Outcome.Communities)
	}

	a, err := BuildCandidateGroup(in)
	if err != nil {
		t.Fatal(err)
	}
	if a.PartitionDigest != fixturePartition || a.ProfileID != programc.ProfileID || a.ProfileDigest != programc.ProfileDigest || a.Seed != 19 || !reflect.DeepEqual(a.MemberIDs(), []string{fixtureMemberA, fixtureMemberB}) {
		t.Fatalf("identity mismatch: %#v", a)
	}
	if got := []string{a.Members[0].Name, a.Members[1].Name}; !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("admitted graph join mismatch: %#v", got)
	}
	for _, member := range a.Members {
		if member.Source.Status != SourceUnavailable || member.Source.Reason == "" || member.Source.CustodyAvailable || len(member.Source.Citations) != 0 {
			t.Fatalf("source accounting mismatch: %#v", member)
		}
	}
	if a.SourceAccounting != (CandidateGroupSourceAccounting{AttemptedMembers: 2, TerminalMembers: 2, UnavailableMembers: 2}) {
		t.Fatalf("source accounting mismatch: %#v", a.SourceAccounting)
	}
	if a.Interpretation.Status != InterpretationUnresolved || len(a.Interpretation.Citations) != 0 || len(a.Interpretation.Claims) != 0 || a.Interpretation.Description != "" {
		t.Fatalf("interpretation ceiling mismatch: %#v", a.Interpretation)
	}
	if a.Authority != 0 || a.Accepted || a.Completeness != CompletenessUnknown || a.Representative.ID != fixtureMemberA || a.Representative.Role != "NAVIGATION_ONLY" {
		t.Fatalf("authority/representative mismatch: %#v", a)
	}
	if a.Boundary.Accounting.IntraOccurrences != 1 || a.Boundary.Accounting.CrossingOccurrences != 1 || len(a.InternalCalls) != 1 || len(a.CrossingCalls) != 1 || len(a.Boundary.CrossingWitnesses) != 1 || len(a.Boundary.Bridges) != 2 || len(a.Boundary.ArticulationPoints) != 1 || len(a.Boundary.HighCentralityCrossingNodes) == 0 || len(a.Boundary.HubCrossingNodes) == 0 {
		t.Fatalf("boundary evidence mismatch: %#v internal=%#v crossing=%#v", a.Boundary, a.InternalCalls, a.CrossingCalls)
	}
	raw, err := a.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	again, err := BuildCandidateGroup(in)
	if err != nil {
		t.Fatal(err)
	}
	rawAgain, _ := again.Bytes()
	if a.ID() == "" || a.ID() != again.ID() || !bytes.Equal(raw, rawAgain) {
		t.Fatal("candidate artifact is not deterministic")
	}
	if bytes.Contains(raw, []byte(programctestfixture.OpaqueMarker)) || bytes.Contains(raw, []byte(programctestfixture.SourceBodyMarker)) {
		t.Fatal("opaque marker leaked")
	}
}

func TestCandidateGroupRejectsBoundaryFieldMutationAfterReseal(t *testing.T) {
	base, err := BuildCandidateGroup(fixtureCandidateInput(t))
	if err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		mutate func(*programc.BoundaryArtifact)
	}{
		{"policy", func(b *programc.BoundaryArtifact) { b.Policy.ID += "-mutated" }},
		{"bindings", func(b *programc.BoundaryArtifact) { b.Bindings.SessionID += "-mutated" }},
		{"request", func(b *programc.BoundaryArtifact) { b.Request.PageRankTopK++ }},
		{"rank-policy", func(b *programc.BoundaryArtifact) { b.RankPolicies[0] += "-mutated" }},
		{"claim-ceiling", func(b *programc.BoundaryArtifact) { b.ClaimCeiling += " mutated" }},
		{"digest", func(b *programc.BoundaryArtifact) { b.Digest += "0" }},
		{"outcome", func(b *programc.BoundaryArtifact) { b.Outcome = "MUTATED" }},
		{"accounting", func(b *programc.BoundaryArtifact) { b.Accounting.AdmittedNodes++ }},
		{"community", func(b *programc.BoundaryArtifact) { b.Communities[0].Members[0] += "-mutated" }},
		{"witness", func(b *programc.BoundaryArtifact) { b.CrossingWitnesses[0].OccurrenceID += "-mutated" }},
		{"bridge", func(b *programc.BoundaryArtifact) { b.Bridges[0] += "-mutated" }},
		{"articulation", func(b *programc.BoundaryArtifact) { b.ArticulationPoints[0] += "-mutated" }},
		{"pagerank", func(b *programc.BoundaryArtifact) { b.PageRank.Iterations++ }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			a := base
			a.Boundary = base.Boundary
			a.Boundary.RankPolicies = append([]string(nil), base.Boundary.RankPolicies...)
			a.Boundary.Communities = append([]programc.BoundaryCommunity(nil), base.Boundary.Communities...)
			for i := range a.Boundary.Communities {
				a.Boundary.Communities[i].Members = append([]string(nil), base.Boundary.Communities[i].Members...)
			}
			a.Boundary.CrossingWitnesses = append([]programc.CrossingWitness(nil), base.Boundary.CrossingWitnesses...)
			a.Boundary.Bridges = append([]string(nil), base.Boundary.Bridges...)
			a.Boundary.ArticulationPoints = append([]string(nil), base.Boundary.ArticulationPoints...)
			tc.mutate(&a.Boundary)
			a.ArtifactID = sealCandidateGroup(a)
			if _, err := a.Bytes(); err == nil {
				t.Fatalf("ASSERT_CANDIDATE_BOUNDARY_MUTATION_REJECTED[%s]", tc.name)
			}
		})
	}
}

func TestCandidateGroupCorrespondenceRejectsEveryIdentityMutation(t *testing.T) {
	r := sourceprojection.Range{End: sourceprojection.Position{Line: 1}}
	base := CandidateGroupMember{ID: "program-c", Name: "Symbol", Source: MemberSourceOutcome{SourceID: "sha256:document", ItemRange: &r, SelectionRange: &r, Correspondence: &MemberCorrespondence{GraphSubjectID: "graph-subject", V6EndpointGraphSubjectID: "program-c", ProgramCNodeID: "program-c", SymbolName: "Symbol", URI: "file:///source.go", ItemRange: r, SelectionRange: r, FixtureDigest: "sha256:fixture", DocumentDigest: "sha256:document", SessionID: "session", Generation: 1, DocumentVersion: "version", PositionEncoding: "utf-16", SourceResponseIdentity: "receipt"}}}
	mutations := []struct {
		name   string
		mutate func(*MemberCorrespondence)
	}{
		{"graph-subject", func(c *MemberCorrespondence) { c.GraphSubjectID = "program-c" }},
		{"v6-endpoint", func(c *MemberCorrespondence) { c.V6EndpointGraphSubjectID = "foreign" }},
		{"program-c", func(c *MemberCorrespondence) { c.ProgramCNodeID = "swapped" }},
		{"name", func(c *MemberCorrespondence) { c.SymbolName = "substituted" }},
		{"uri", func(c *MemberCorrespondence) { c.URI = "" }},
		{"item-range", func(c *MemberCorrespondence) { c.ItemRange.End.Line++ }},
		{"selection-range", func(c *MemberCorrespondence) { c.SelectionRange.End.Line++ }},
		{"fixture-digest", func(c *MemberCorrespondence) { c.FixtureDigest = "" }},
		{"document-digest", func(c *MemberCorrespondence) { c.DocumentDigest = "foreign" }},
		{"session", func(c *MemberCorrespondence) { c.SessionID = "" }},
		{"generation", func(c *MemberCorrespondence) { c.Generation = 0 }},
		{"version", func(c *MemberCorrespondence) { c.DocumentVersion = "" }},
		{"encoding", func(c *MemberCorrespondence) { c.PositionEncoding = "" }},
		{"response", func(c *MemberCorrespondence) { c.SourceResponseIdentity = "" }},
	}
	if !validCorrespondence(base, map[string]bool{}) {
		t.Fatal("ASSERT_CANDIDATE_CORRESPONDENCE_VALID")
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			c := *base.Source.Correspondence
			m.Source.Correspondence = &c
			tc.mutate(&c)
			if tc.name == "graph-subject" {
				if validCorrespondence(m, map[string]bool{"foreign": true}) {
					t.Fatal("ASSERT_CANDIDATE_CORRESPONDENCE_MUTATION_REJECTED")
				}
				return
			}
			if validCorrespondence(m, map[string]bool{}) {
				t.Fatal("ASSERT_CANDIDATE_CORRESPONDENCE_MUTATION_REJECTED")
			}
		})
	}
	if validCorrespondence(base, map[string]bool{"graph-subject": true}) {
		t.Fatal("ASSERT_CANDIDATE_CORRESPONDENCE_BIJECTION_REJECTED")
	}
}

func TestCandidateGroupDescriptionCitationCoverageMutations(t *testing.T) {
	r := sourceprojection.Range{Start: sourceprojection.Position{Line: 1}, End: sourceprojection.Position{Line: 3}}
	inside := sourceprojection.Range{Start: sourceprojection.Position{Line: 1}, End: sourceprojection.Position{Line: 2}}
	members := []CandidateGroupMember{{ID: "program-c", Name: "Symbol", Source: MemberSourceOutcome{CitationID: "citation", EvidenceRange: r, DisplayRange: r, Correspondence: &MemberCorrespondence{URI: "file:///source.go"}}}}
	call := CandidateGroupCall{OccurrenceID: "occurrence", RelationID: "relation", SourceNodeID: "program-c", TargetNodeID: "callee", CallSite: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 2}}}
	base := Citation{SourceID: "citation", LogicalSourceID: "file:///source.go", Range: inside, Claim: "supported statement", RelationID: call.RelationID, OccurrenceID: call.OccurrenceID, CallerProgramCNodeID: call.SourceNodeID, CalleeProgramCNodeID: call.TargetNodeID}
	set := map[string]bool{"citation": true}
	calls := map[string]CandidateGroupCall{call.OccurrenceID: call}
	if !validDescriptionCitation(base, base.Claim, members, set, calls) {
		t.Fatal("ASSERT_CANDIDATE_DESCRIPTION_CITATION_VALID")
	}
	mutations := []struct {
		name   string
		mutate func(*Citation)
	}{
		{"missing", func(c *Citation) { c.SourceID = "" }}, {"foreign", func(c *Citation) { c.SourceID = "foreign" }},
		{"swapped-uri", func(c *Citation) { c.LogicalSourceID = "file:///foreign.go" }}, {"missing-claim", func(c *Citation) { c.Claim = "" }},
		{"foreign-claim", func(c *Citation) { c.Claim = "unsupported" }}, {"out-of-range", func(c *Citation) { c.Range.End.Line = 4 }},
		{"malformed", func(c *Citation) { c.Range.Start.Line = 3; c.Range.End.Line = 2 }},
		{"relation", func(c *Citation) { c.RelationID = "foreign" }}, {"occurrence", func(c *Citation) { c.OccurrenceID = "foreign" }},
		{"caller", func(c *Citation) { c.CallerProgramCNodeID = "foreign" }}, {"callee", func(c *Citation) { c.CalleeProgramCNodeID = "foreign" }},
		{"swapped-range", func(c *Citation) { c.Range.Start.Line++ }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.mutate(&c)
			if validDescriptionCitation(c, base.Claim, members, set, calls) {
				t.Fatal("ASSERT_CANDIDATE_DESCRIPTION_CITATION_MUTATION_REJECTED")
			}
		})
	}
	unsupported := []CandidateGroupMember{{Source: MemberSourceOutcome{CitationID: "other", EvidenceRange: r, DisplayRange: r, Correspondence: &MemberCorrespondence{URI: base.LogicalSourceID}}}}
	if validDescriptionCitation(base, base.Claim, unsupported, set, calls) {
		t.Fatal("ASSERT_CANDIDATE_DESCRIPTION_UNSUPPORTED_MEMBER_REJECTED")
	}
}

func TestCandidateGroupPageRankWorkAtBoundAndPlusOne(t *testing.T) {
	bounds := FrozenCandidateGroupBounds()
	if err := validatePageRankWork(bounds.MaxPageRankWork, bounds.MaxPageRankWork); err != nil {
		t.Fatalf("ASSERT_CANDIDATE_PAGERANK_WORK_AT_BOUND: %v", err)
	}
	if err := validatePageRankWork(bounds.MaxPageRankWork+1, bounds.MaxPageRankWork); err == nil {
		t.Fatal("ASSERT_CANDIDATE_PAGERANK_WORK_PLUS_ONE_REJECTED")
	}
}

func TestCandidateGroupFrozenBoundsAreIdentityAndCheckedBeforeInterpretation(t *testing.T) {
	base := fixtureCandidateInput(t)
	mutations := []func(*CandidateGroupBounds){
		func(b *CandidateGroupBounds) { b.MaxCommunities++ }, func(b *CandidateGroupBounds) { b.MaxMembersPerCommunity++ },
		func(b *CandidateGroupBounds) { b.MaxDistinctMembers++ }, func(b *CandidateGroupBounds) { b.MaxCallOccurrences++ },
		func(b *CandidateGroupBounds) { b.MaxSourceBytesPerMember++ }, func(b *CandidateGroupBounds) { b.MaxTotalUniqueSourceBytes++ },
		func(b *CandidateGroupBounds) { b.MaxSourceRanges++ }, func(b *CandidateGroupBounds) { b.MaxSourceObjects++ },
		func(b *CandidateGroupBounds) { b.MaxWorkUnits++ }, func(b *CandidateGroupBounds) { b.MaxPageRankWork++ },
		func(b *CandidateGroupBounds) { b.MaxResponseBytes++ }, func(b *CandidateGroupBounds) { b.MaxArtifactBytes++ },
		func(b *CandidateGroupBounds) { b.MaxCitations++ }, func(b *CandidateGroupBounds) { b.MaxDescriptionUTF8Bytes++ },
		func(b *CandidateGroupBounds) { b.MaxDescriptions++ }, func(b *CandidateGroupBounds) { b.MaxHostAttempts++ },
	}
	for i, mutate := range mutations {
		in := base
		mutate(&in.Bounds)
		in.Interpretation.Status = "INVALID_IF_REACHED"
		a, err := BuildCandidateGroup(in)
		if err == nil || err.Error() != "candidate group bounds mismatch" || a.ID() != "" {
			t.Fatalf("bound %d not rejected before interpretation: artifact=%#v err=%v", i, a, err)
		}
	}
}

func TestCandidateGroupPrivateV2ProfileDigestPinned(t *testing.T) {
	if got := CandidateGroupPrivateV2Profile().Digest(); got != CandidateGroupPrivateV2ProfileDigest {
		t.Fatalf("ASSERT_CANDIDATE_PROFILE_V2_DIGEST_PIN: got=%s want=%s", got, CandidateGroupPrivateV2ProfileDigest)
	}
}

func TestCandidateGroupPrivateV2ProfileFreezeAndMutationMatrix(t *testing.T) {
	v1, v2 := FrozenCandidateGroupProfile(), CandidateGroupPrivateV2Profile()
	if v2.ProfileID != CandidateGroupPrivateV2ProfileID || v2.Bounds.MaxPageRankWork != 1664 {
		t.Fatalf("ASSERT_CANDIDATE_PROFILE_V2_ID_LIMIT: %+v", v2)
	}
	want := v1.Bounds
	want.MaxPageRankWork = 1664
	if v2.Bounds != want || v2.ValidationPolicyID != v1.ValidationPolicyID || v2.ValidationPolicyVersion != v1.ValidationPolicyVersion {
		t.Fatalf("ASSERT_CANDIDATE_PROFILE_V2_SOLE_LIMIT_CHANGE: v1=%+v v2=%+v", v1, v2)
	}
	baseDigest := v2.Digest()
	mutations := []func(*CandidateGroupProfile){
		func(p *CandidateGroupProfile) { p.ProfileID += "-mutated" },
		func(p *CandidateGroupProfile) { p.ValidationPolicyID += "-mutated" },
		func(p *CandidateGroupProfile) { p.ValidationPolicyVersion += "-mutated" },
		func(p *CandidateGroupProfile) { p.Bounds.MaxCommunities++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxMembersPerCommunity++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxDistinctMembers++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxCallOccurrences++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxSourceBytesPerMember++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxTotalUniqueSourceBytes++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxSourceRanges++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxSourceObjects++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxWorkUnits++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxPageRankWork++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxResponseBytes++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxArtifactBytes++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxCitations++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxDescriptionUTF8Bytes++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxDescriptions++ },
		func(p *CandidateGroupProfile) { p.Bounds.MaxHostAttempts++ },
	}
	for i, mutate := range mutations {
		changed := v2
		mutate(&changed)
		if changed.Digest() == baseDigest || validCandidateGroupProfile(changed) {
			t.Fatalf("ASSERT_CANDIDATE_PROFILE_V2_MUTATION_REJECTED[%d]", i)
		}
	}
}

func TestCandidateGroupArtifactIdentityBindsResourceProfile(t *testing.T) {
	v1Input := fixtureCandidateInput(t)
	v1, err := BuildCandidateGroup(v1Input)
	if err != nil {
		t.Fatal(err)
	}
	v2Input := fixtureCandidateInput(t)
	v2Input.ResourceProfile = CandidateGroupPrivateV2Profile()
	v2Input.Bounds = v2Input.ResourceProfile.Bounds
	v2, err := BuildCandidateGroup(v2Input)
	if err != nil {
		t.Fatal(err)
	}
	if v1.ID() == v2.ID() || v1.ResourceProfileID == v2.ResourceProfileID || v1.ResourceProfileDigest == v2.ResourceProfileDigest {
		t.Fatal("ASSERT_CANDIDATE_ARTIFACT_PROFILE_BOUND_IDENTITY")
	}
	substituted := v2
	substituted.ResourceProfileID = CandidateGroupPrivateV1ProfileID
	substituted.ArtifactID = sealCandidateGroup(substituted)
	if _, err := substituted.Bytes(); err == nil {
		t.Fatal("ASSERT_CANDIDATE_ARTIFACT_PROFILE_SUBSTITUTION_REJECTED")
	}
}

func privateGroupingFixture(t *testing.T) (*MemoryStore, string, privateProvisionalArtifact) {
	t.Helper()
	store, admissionSelector, _ := privateProvisionalFixture(t, []string{privateCallerOutcomeSuccess})
	selector, err := PreparePrivateProvisionalCandidateSelector(context.Background(), store, admissionSelector)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, err := parsePrivateProvisionalCandidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return store, selector, artifact
}

func privateGroupingSubmission(t *testing.T, groups []privateGroupingSubmissionGroup, alternatives []privateGroupingSubmissionAlternative) []byte {
	t.Helper()
	raw, err := json.Marshal(privateGroupingSubmissionWire{SchemaVersion: privateGroupingSubmissionSchema, Groups: groups, Alternatives: alternatives})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPrivateGroupingHypothesisKindsOverlapAlternativesAndAccounting(t *testing.T) {
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	ids := []string{predecessor.wire.Candidates[0].CandidateID, predecessor.wire.Candidates[1].CandidateID}
	sort.Strings(ids)
	groups := []privateGroupingSubmissionGroup{
		{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: ids, Label: " exact label ", Claim: "claim", Basis: "basis", RejectionConditions: []string{"condition-a"}},
		{ClientGroupKey: "b", Kind: privateGroupingSemantic, CandidateIDs: []string{ids[0]}, Label: "other", Claim: "other claim", Basis: "other basis", RejectionConditions: []string{"condition-b"}},
		{ClientGroupKey: "c", Kind: privateGroupingSingleton, CandidateIDs: []string{ids[1]}, RejectionConditions: []string{}},
	}
	submission := privateGroupingSubmission(t, groups, []privateGroupingSubmissionAlternative{{LeftClientGroupKey: "a", RightClientGroupKey: "b"}})
	selector, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, submission)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, err := parsePrivateGroupingArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	w := artifact.wire
	if w.Authority != 0 || w.Accepted || w.Completeness != CompletenessUnknown || w.FeatureIdentity != "UNRESOLVED" || w.SemanticQualification != "NOT_PERFORMED" || w.AutomaticProgressionAllowed || w.Groups[0].Label != " exact label " {
		t.Fatalf("fixed fields or semantic preservation: %+v", w)
	}
	if w.Accounting.CandidateCount != 2 || w.Accounting.GroupCount != 3 || w.Accounting.MembershipCount != 4 || w.Accounting.DistinctGroupedCandidateCount != 2 || w.Accounting.UngroupedCandidateCount != 0 || len(w.UngroupedCandidateIDs) != 0 {
		t.Fatalf("accounting: %+v ungrouped=%v", w.Accounting, w.UngroupedCandidateIDs)
	}
	if bytes.Contains(raw, predecessor.wire.Descriptions[0].Outcome) || bytes.Contains(raw, []byte(`"outcome"`)) {
		t.Fatal("outcome payload copied")
	}
	second, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, submission)
	if err != nil || second != selector {
		t.Fatalf("replay selector=%q want=%q err=%v", second, selector, err)
	}
}

func TestPrivateGroupingHypothesisPartialCoverageAndStrictSubmission(t *testing.T) {
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	id := predecessor.wire.Candidates[0].CandidateID
	valid := privateGroupingSubmission(t, []privateGroupingSubmissionGroup{{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: []string{id}, Label: "label", Claim: "claim", Basis: "basis", RejectionConditions: []string{"reject"}}}, []privateGroupingSubmissionAlternative{})
	selector, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, valid)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, _ := parsePrivateGroupingArtifact(raw)
	if artifact.wire.Accounting.UngroupedCandidateCount != 1 || len(artifact.wire.UngroupedCandidateIDs) != 1 {
		t.Fatalf("partial coverage: %+v", artifact.wire.Accounting)
	}
	cases := map[string][]byte{
		"nil":          nil,
		"null":         []byte("null"),
		"empty-groups": privateGroupingSubmission(t, []privateGroupingSubmissionGroup{}, []privateGroupingSubmissionAlternative{}),
		"unknown":      bytes.Replace(valid, []byte(`"groups":`), []byte(`"caller_id":"forbidden","groups":`), 1),
		"whitespace":   append([]byte(" "), valid...),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, candidate); err == nil || got != "" {
				t.Fatalf("selector=%q err=%v", got, err)
			}
		})
	}
}

func TestPrivateGroupingHypothesisRejectsCanonicalAndAlternativeViolations(t *testing.T) {
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	ids := []string{predecessor.wire.Candidates[0].CandidateID, predecessor.wire.Candidates[1].CandidateID}
	sort.Strings(ids)
	base := func() []privateGroupingSubmissionGroup {
		return []privateGroupingSubmissionGroup{
			{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: []string{ids[0]}, Label: "a", Claim: "a", Basis: "a", RejectionConditions: []string{"a"}},
			{ClientGroupKey: "b", Kind: privateGroupingSemantic, CandidateIDs: []string{ids[1]}, Label: "b", Claim: "b", Basis: "b", RejectionConditions: []string{"b"}},
		}
	}
	for name, mutate := range map[string]func(*[]privateGroupingSubmissionGroup, *[]privateGroupingSubmissionAlternative){
		"group-order": func(g *[]privateGroupingSubmissionGroup, _ *[]privateGroupingSubmissionAlternative) {
			(*g)[0], (*g)[1] = (*g)[1], (*g)[0]
		},
		"member-order": func(g *[]privateGroupingSubmissionGroup, _ *[]privateGroupingSubmissionAlternative) {
			(*g)[0].CandidateIDs = []string{ids[1], ids[0]}
		},
		"duplicate-condition": func(g *[]privateGroupingSubmissionGroup, _ *[]privateGroupingSubmissionAlternative) {
			(*g)[0].RejectionConditions = []string{"a", "a"}
		},
		"reverse-alternative": func(_ *[]privateGroupingSubmissionGroup, a *[]privateGroupingSubmissionAlternative) {
			*a = []privateGroupingSubmissionAlternative{{LeftClientGroupKey: "b", RightClientGroupKey: "a"}}
		},
		"singleton-endpoint": func(g *[]privateGroupingSubmissionGroup, a *[]privateGroupingSubmissionAlternative) {
			(*g)[1] = privateGroupingSubmissionGroup{ClientGroupKey: "b", Kind: privateGroupingSingleton, CandidateIDs: []string{ids[1]}, RejectionConditions: []string{}}
			*a = []privateGroupingSubmissionAlternative{{LeftClientGroupKey: "a", RightClientGroupKey: "b"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			groups, alternatives := base(), []privateGroupingSubmissionAlternative{{LeftClientGroupKey: "a", RightClientGroupKey: "b"}}
			mutate(&groups, &alternatives)
			if got, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, privateGroupingSubmission(t, groups, alternatives)); err == nil || got != "" {
				t.Fatalf("selector=%q err=%v", got, err)
			}
		})
	}
}

func TestPrivateGroupingHypothesisSemanticBoundsAndAuthorityMutationMatrix(t *testing.T) {
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	id := predecessor.wire.Candidates[0].CandidateID
	group := privateGroupingSubmissionGroup{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: []string{id}, Label: strings.Repeat("x", privateGroupingMaxSemanticBytes), Claim: "claim", Basis: "basis", RejectionConditions: make([]string, privateGroupingMaxConditions)}
	for i := range group.RejectionConditions {
		group.RejectionConditions[i] = fmt.Sprintf("condition-%03d", i)
	}
	valid := privateGroupingSubmission(t, []privateGroupingSubmissionGroup{group}, []privateGroupingSubmissionAlternative{})
	selector, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, valid)
	if err != nil {
		t.Fatal(err)
	}
	tooLong := group
	tooLong.Label += "x"
	tooMany := group
	tooMany.RejectionConditions = append(append([]string{}, group.RejectionConditions...), "condition-overflow")
	for name, submission := range map[string][]byte{
		"semantic-plus-one":   privateGroupingSubmission(t, []privateGroupingSubmissionGroup{tooLong}, []privateGroupingSubmissionAlternative{}),
		"conditions-plus-one": privateGroupingSubmission(t, []privateGroupingSubmissionGroup{tooMany}, []privateGroupingSubmissionAlternative{}),
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, submission); err == nil || got != "" {
				t.Fatalf("selector=%q err=%v", got, err)
			}
		})
	}
	raw, _ := store.Get(context.Background(), selector)
	base, err := parsePrivateGroupingArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*privateGroupingArtifactWire){
		"authority":              func(w *privateGroupingArtifactWire) { w.Authority = 1 },
		"accepted":               func(w *privateGroupingArtifactWire) { w.Accepted = true },
		"completeness":           func(w *privateGroupingArtifactWire) { w.Completeness = "COMPLETE" },
		"feature-identity":       func(w *privateGroupingArtifactWire) { w.FeatureIdentity = "ACCEPTED" },
		"semantic-qualification": func(w *privateGroupingArtifactWire) { w.SemanticQualification = "PERFORMED" },
		"progression":            func(w *privateGroupingArtifactWire) { w.AutomaticProgressionAllowed = true },
		"review":                 func(w *privateGroupingArtifactWire) { w.ReviewStatus = "PERFORMED" },
		"challenge":              func(w *privateGroupingArtifactWire) { w.ChallengeStatus = "PERFORMED" },
		"adjudication":           func(w *privateGroupingArtifactWire) { w.AdjudicationStatus = "PERFORMED" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			w := base.wire
			mutate(&w)
			w.ArtifactID = identityJSON(privateGroupingArtifactDomain, w, func(v *privateGroupingArtifactWire) { v.ArtifactID = "" })
			mutated, _ := json.Marshal(w)
			if _, err := parsePrivateGroupingArtifact(mutated); err == nil {
				t.Fatal("accepted authority mutation")
			}
		})
	}
}

func resealPrivateGroupingWire(w *privateGroupingArtifactWire) {
	for i := range w.Groups {
		w.Groups[i].GroupID = identityJSON(privateGroupingGroupDomain, w.Groups[i], func(v *privateGroupingGroup) { v.GroupID = "" })
	}
	groupIDs := map[string]string{}
	for _, group := range w.Groups {
		groupIDs[group.ClientGroupKey] = group.GroupID
	}
	for i := range w.Alternatives {
		w.Alternatives[i].LeftGroupID = groupIDs[w.Alternatives[i].LeftClientGroupKey]
		w.Alternatives[i].RightGroupID = groupIDs[w.Alternatives[i].RightClientGroupKey]
		w.Alternatives[i].AlternativeID = identityJSON(privateGroupingAlternativeDomain, w.Alternatives[i], func(v *privateGroupingAlternative) { v.AlternativeID = "" })
	}
	w.ArtifactID = identityJSON(privateGroupingArtifactDomain, *w, func(v *privateGroupingArtifactWire) { v.ArtifactID = "" })
}

func clonePrivateGroupingWire(t *testing.T, wire privateGroupingArtifactWire) privateGroupingArtifactWire {
	t.Helper()
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var clone privateGroupingArtifactWire
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestPrivateGroupingRetainedStructuralMutationMatrix(t *testing.T) {
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	ids := []string{predecessor.wire.Candidates[0].CandidateID, predecessor.wire.Candidates[1].CandidateID}
	sort.Strings(ids)
	groups := []privateGroupingSubmissionGroup{
		{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: []string{ids[0]}, Label: "a", Claim: "a", Basis: "a", RejectionConditions: []string{"a"}},
		{ClientGroupKey: "b", Kind: privateGroupingSemantic, CandidateIDs: []string{ids[1]}, Label: "b", Claim: "b", Basis: "b", RejectionConditions: []string{"b"}},
	}
	selector, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, privateGroupingSubmission(t, groups, []privateGroupingSubmissionAlternative{{LeftClientGroupKey: "a", RightClientGroupKey: "b"}}))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, err := parsePrivateGroupingArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*privateGroupingArtifactWire){
		"snapshot-order": func(w *privateGroupingArtifactWire) {
			w.CandidateSnapshots[0], w.CandidateSnapshots[1] = w.CandidateSnapshots[1], w.CandidateSnapshots[0]
		},
		"snapshot-ordinal": func(w *privateGroupingArtifactWire) { w.CandidateSnapshots[0].Ordinal++ },
		"snapshot-description": func(w *privateGroupingArtifactWire) {
			w.CandidateSnapshots[0].DescriptionID = w.CandidateSnapshots[1].DescriptionID
		},
		"snapshot-outcome":          func(w *privateGroupingArtifactWire) { w.CandidateSnapshots[0].OutcomeDigest = "not-a-digest" },
		"semantic-empty-label":      func(w *privateGroupingArtifactWire) { w.Groups[0].Label = "" },
		"semantic-empty-conditions": func(w *privateGroupingArtifactWire) { w.Groups[0].RejectionConditions = []string{} },
		"singleton-with-semantics":  func(w *privateGroupingArtifactWire) { w.Groups[0].Kind = privateGroupingSingleton },
		"candidate-order":           func(w *privateGroupingArtifactWire) { w.Groups[0].CandidateIDs = []string{ids[1], ids[0]} },
		"alternative-order": func(w *privateGroupingArtifactWire) {
			w.Alternatives[0].LeftClientGroupKey, w.Alternatives[0].RightClientGroupKey = "b", "a"
		},
		"alternative-duplicate": func(w *privateGroupingArtifactWire) { w.Alternatives = append(w.Alternatives, w.Alternatives[0]) },
		"alternative-singleton-endpoint": func(w *privateGroupingArtifactWire) {
			w.Groups[1] = privateGroupingGroup{ClientGroupKey: "b", Kind: privateGroupingSingleton, CandidateIDs: []string{ids[1]}, RejectionConditions: []string{}}
		},
		"complement":     func(w *privateGroupingArtifactWire) { w.UngroupedCandidateIDs = []string{ids[0]} },
		"accounting":     func(w *privateGroupingArtifactWire) { w.Accounting.MembershipCount++ },
		"profile-digest": func(w *privateGroupingArtifactWire) { w.ProfileDigest = digestOf([]byte("foreign")) },
		"profile-bound":  func(w *privateGroupingArtifactWire) { w.Bounds.MaxGroups++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			wire := clonePrivateGroupingWire(t, artifact.wire)
			mutate(&wire)
			resealPrivateGroupingWire(&wire)
			mutated, _ := json.Marshal(wire)
			if _, err := parsePrivateGroupingArtifact(mutated); err == nil {
				t.Fatal("accepted retained structural mutation")
			}
		})
	}
	for name, mutate := range map[string]func(*privateGroupingArtifactWire){
		"group-id-substitution":       func(w *privateGroupingArtifactWire) { w.Groups[0].GroupID = w.Groups[1].GroupID },
		"alternative-id-substitution": func(w *privateGroupingArtifactWire) { w.Alternatives[0].AlternativeID = w.Groups[0].GroupID },
	} {
		t.Run(name, func(t *testing.T) {
			wire := clonePrivateGroupingWire(t, artifact.wire)
			mutate(&wire)
			wire.ArtifactID = identityJSON(privateGroupingArtifactDomain, wire, func(v *privateGroupingArtifactWire) { v.ArtifactID = "" })
			mutated, _ := json.Marshal(wire)
			if _, err := parsePrivateGroupingArtifact(mutated); err == nil {
				t.Fatal("accepted retained identity substitution")
			}
		})
	}
}

func TestPrivateGroupingClientKeyUTF8BoundaryAndGenericGroupLimit(t *testing.T) {
	if privateGroupingComputedProfileDigest() != privateGroupingProfileDigestPin || privateGroupingProfileDigest() != privateGroupingProfileDigestPin {
		t.Fatalf("profile digest drift: computed=%s pinned=%s", privateGroupingComputedProfileDigest(), privateGroupingProfileDigestPin)
	}
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	id := predecessor.wire.Candidates[0].CandidateID
	exact := strings.Repeat("é", 127) + "ab"
	if len([]byte(exact)) != privateGroupingMaxClientKeyBytes {
		t.Fatal("fixture byte boundary")
	}
	group := func(key string) privateGroupingSubmissionGroup {
		return privateGroupingSubmissionGroup{ClientGroupKey: key, Kind: privateGroupingSemantic, CandidateIDs: []string{id}, Label: "a", Claim: "a", Basis: "a", RejectionConditions: []string{"a"}}
	}
	if _, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, privateGroupingSubmission(t, []privateGroupingSubmissionGroup{group(exact)}, []privateGroupingSubmissionAlternative{})); err != nil {
		t.Fatalf("exact multibyte key rejected: %v", err)
	}
	if got, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, privateGroupingSubmission(t, []privateGroupingSubmissionGroup{group(exact + "c")}, []privateGroupingSubmissionAlternative{})); err == nil || got != "" {
		t.Fatalf("plus-one key selector=%q err=%v", got, err)
	}
	alternativeGroups := make([]privateGroupingSubmissionGroup, 92)
	for i := range alternativeGroups {
		alternativeGroups[i] = group(fmt.Sprintf("a-%03d", i))
	}
	allPairs := make([]privateGroupingSubmissionAlternative, 0, 4186)
	for left := 0; left < len(alternativeGroups); left++ {
		for right := left + 1; right < len(alternativeGroups); right++ {
			allPairs = append(allPairs, privateGroupingSubmissionAlternative{LeftClientGroupKey: alternativeGroups[left].ClientGroupKey, RightClientGroupKey: alternativeGroups[right].ClientGroupKey})
		}
	}
	if _, err := parsePrivateGroupingSubmission(privateGroupingSubmission(t, alternativeGroups, allPairs[:privateGroupingMaxAlternatives])); err != nil {
		t.Fatalf("generic alternative bound rejected: %v", err)
	}
	if _, err := parsePrivateGroupingSubmission(privateGroupingSubmission(t, alternativeGroups, allPairs[:privateGroupingMaxAlternatives+1])); err == nil {
		t.Fatal("generic alternative plus-one accepted")
	}
	groups := make([]privateGroupingSubmissionGroup, privateGroupingMaxGroups)
	for i := range groups {
		groups[i] = group(fmt.Sprintf("g-%04d", i))
	}
	atBound := privateGroupingSubmission(t, groups, []privateGroupingSubmissionAlternative{})
	if _, err := parsePrivateGroupingSubmission(atBound); err != nil {
		t.Fatalf("generic group bound rejected: %v", err)
	}
	groups = append(groups, group("g-overflow"))
	sort.Slice(groups, func(i, j int) bool { return groups[i].ClientGroupKey < groups[j].ClientGroupKey })
	if _, err := parsePrivateGroupingSubmission(privateGroupingSubmission(t, groups, []privateGroupingSubmissionAlternative{})); err == nil {
		t.Fatal("generic group plus-one accepted")
	}
	oversized := make([]byte, privateGroupingMaxBytes+1)
	if _, err := parsePrivateGroupingArtifact(oversized); err == nil {
		t.Fatal("artifact byte plus-one accepted")
	}
}

func TestPrivateGroupingMemberAndMembershipLimits(t *testing.T) {
	bounds := privateGroupingFrozenBounds()
	memberIDs := make([]string, bounds.MaxMembersPerGroup+1)
	for i := range memberIDs {
		memberIDs[i] = digestOf([]byte(fmt.Sprintf("member-%05d", i)))
	}
	sort.Strings(memberIDs)
	if err := validatePrivateGroupingGroup("a", privateGroupingSemantic, memberIDs[:bounds.MaxMembersPerGroup], "a", "a", "a", []string{"a"}, bounds); err != nil {
		t.Fatalf("member bound rejected: %v", err)
	}
	if err := validatePrivateGroupingGroup("a", privateGroupingSemantic, memberIDs, "a", "a", "a", []string{"a"}, bounds); err == nil {
		t.Fatal("member plus-one accepted")
	}

	store, candidateSelector, predecessor := privateGroupingFixture(t)
	id := predecessor.wire.Candidates[0].CandidateID
	selector, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, privateGroupingSubmission(t, []privateGroupingSubmissionGroup{{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: []string{id}, Label: "a", Claim: "a", Basis: "a", RejectionConditions: []string{"a"}}}, []privateGroupingSubmissionAlternative{}))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.Get(context.Background(), selector)
	artifact, _ := parsePrivateGroupingArtifact(raw)
	wire := clonePrivateGroupingWire(t, artifact.wire)
	five := append([]string{}, memberIDs[:5]...)
	wire.CandidateSnapshots = make([]privateGroupingSnapshot, len(five))
	for i, candidateID := range five {
		wire.CandidateSnapshots[i] = privateGroupingSnapshot{Ordinal: i, CandidateID: candidateID, DescriptionID: digestOf([]byte("description-" + candidateID)), OutcomeDigest: digestOf([]byte("outcome-" + candidateID)), OutcomeByteLength: 1}
	}
	wire.Groups = make([]privateGroupingGroup, bounds.MaxGroups)
	for i := range wire.Groups {
		wire.Groups[i] = privateGroupingGroup{ClientGroupKey: fmt.Sprintf("g-%04d", i), Kind: privateGroupingSemantic, CandidateIDs: append([]string{}, five[:4]...), Label: "a", Claim: "a", Basis: "a", RejectionConditions: []string{"a"}}
	}
	wire.Alternatives = []privateGroupingAlternative{}
	wire.UngroupedCandidateIDs = []string{five[4]}
	wire.Accounting = privateGroupingAccounting{CandidateCount: 5, GroupCount: bounds.MaxGroups, SemanticGroupCount: bounds.MaxGroups, MembershipCount: bounds.MaxMemberships, DistinctGroupedCandidateCount: 4, UngroupedCandidateCount: 1}
	resealPrivateGroupingWire(&wire)
	if err := (privateGroupingArtifact{wire: wire}).validate(); err != nil {
		t.Fatalf("membership bound rejected: %v", err)
	}
	wire.Groups[0].CandidateIDs = append([]string{}, five...)
	wire.UngroupedCandidateIDs = []string{}
	wire.Accounting.MembershipCount++
	wire.Accounting.DistinctGroupedCandidateCount = 5
	wire.Accounting.UngroupedCandidateCount = 0
	resealPrivateGroupingWire(&wire)
	if err := (privateGroupingArtifact{wire: wire}).validate(); err == nil {
		t.Fatal("membership plus-one accepted")
	}
}

func TestPrivateGroupingConcurrentReplay(t *testing.T) {
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	id := predecessor.wire.Candidates[0].CandidateID
	submission := privateGroupingSubmission(t, []privateGroupingSubmissionGroup{{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: []string{id}, Label: "a", Claim: "a", Basis: "a", RejectionConditions: []string{"a"}}}, []privateGroupingSubmissionAlternative{})
	const workers = 12
	selectors := make(chan string, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector, err := RetainPrivateGroupingHypothesisSelector(context.Background(), store, candidateSelector, submission)
			selectors <- selector
			errs <- err
		}()
	}
	wg.Wait()
	close(selectors)
	close(errs)
	var want string
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for selector := range selectors {
		if want == "" {
			want = selector
		} else if selector != want {
			t.Fatalf("selector=%q want=%q", selector, want)
		}
	}
}

func TestPrivateGroupingHypothesisPutReadbackCancellationAndMutation(t *testing.T) {
	store, candidateSelector, predecessor := privateGroupingFixture(t)
	id := predecessor.wire.Candidates[0].CandidateID
	submission := privateGroupingSubmission(t, []privateGroupingSubmissionGroup{{ClientGroupKey: "a", Kind: privateGroupingSemantic, CandidateIDs: []string{id}, Label: "a", Claim: "a", Basis: "a", RejectionConditions: []string{"a"}}}, []privateGroupingSubmissionAlternative{})
	for name, configure := range map[string]func(*privateProvisionalFaultStore){
		"put":      func(s *privateProvisionalFaultStore) { s.putErr = true },
		"selector": func(s *privateProvisionalFaultStore) { s.wrongPut = true },
		"readback": func(s *privateProvisionalFaultStore) { s.readbackErr = true },
		"mutation": func(s *privateProvisionalFaultStore) { s.mutateReadback = true },
	} {
		t.Run(name, func(t *testing.T) {
			fault := &privateProvisionalFaultStore{Store: store}
			configure(fault)
			if got, err := RetainPrivateGroupingHypothesisSelector(context.Background(), fault, candidateSelector, submission); err == nil || got != "" || fault.puts != 1 {
				t.Fatalf("selector=%q err=%v puts=%d", got, err, fault.puts)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	fault := &privateProvisionalFaultStore{Store: store, cancel: cancel}
	if got, err := RetainPrivateGroupingHypothesisSelector(ctx, fault, candidateSelector, submission); err == nil || got != "" || fault.puts != 1 || fault.gets[fault.last] != 0 {
		t.Fatalf("post-put cancellation selector=%q err=%v puts=%d readbacks=%d", got, err, fault.puts, fault.gets[fault.last])
	}
	changed := &privateProvisionalFaultStore{Store: store, changeSelector: candidateSelector, changeOnRead: 2}
	if got, err := RetainPrivateGroupingHypothesisSelector(context.Background(), changed, candidateSelector, submission); err == nil || got != "" || changed.puts != 0 {
		t.Fatalf("predecessor mutation selector=%q err=%v puts=%d", got, err, changed.puts)
	}
}

func TestCandidateGroupRejectsMutationAndFrozenLimitPlusOneWithoutPartialArtifact(t *testing.T) {
	base := fixtureCandidateInput(t)
	tests := []struct {
		name   string
		mutate func(*CandidateGroupInput)
	}{
		{"membership", func(in *CandidateGroupInput) { in.CommunityMembers[0] = fixtureSingleton }},
		{"host-membership", func(in *CandidateGroupInput) { in.Interpretation.MemberIDs = []string{fixtureSingleton} }},
		{"host-attempts", func(in *CandidateGroupInput) { in.Interpretation.Attempts = 2 }},
		{"citations", func(in *CandidateGroupInput) {
			in.Interpretation.Citations = make([]Citation, in.Bounds.MaxCitations+1)
		}},
		{"description-bytes", func(in *CandidateGroupInput) {
			in.Interpretation.Description = strings.Repeat("x", in.Bounds.MaxDescriptionUTF8Bytes+1)
		}},
		{"artifact-bytes", func(in *CandidateGroupInput) { in.Bounds.MaxArtifactBytes = 1 }},
		{"work", func(in *CandidateGroupInput) { in.Bounds.MaxWorkUnits = 1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			in.CommunityMembers = append([]string(nil), base.CommunityMembers...)
			tc.mutate(&in)
			a, err := BuildCandidateGroup(in)
			if err == nil || a.ID() != "" {
				t.Fatalf("accepted invalid input: %#v", a)
			}
		})
	}
}
