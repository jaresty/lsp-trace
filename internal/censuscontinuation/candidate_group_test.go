package censuscontinuation

import (
	"bytes"
	"reflect"
	"strings"
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
