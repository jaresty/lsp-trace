//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/inventoryscope"
)

func applicationQualifiedFixturePrivate(t *testing.T) censussourceplan.QualifiedInputV1 {
	t.Helper()
	in := inventoryscope.InputV1{Schema: inventoryscope.SchemaV1, Declaration: inventoryscope.DeclarationV1{Subject: "synthetic-app", SourceID: "controlled", ConfigDigest: "sha256:" + strings.Repeat("a", 64), PolicyID: inventoryscope.PolicyConsensusV1, Includes: []string{"main.ts", "gap.go"}, Excludes: []string{"excluded.txt"}}}
	files := []censusacquisition.SourceFile{{Path: "main.ts", URI: "file:///controlled/main.ts", LanguageID: "typescript"}, {Path: "excluded.txt", URI: "file:///controlled/excluded.txt"}, {Path: "unresolved.txt", URI: "file:///controlled/unresolved.txt"}, {Path: "gap.go", URI: "file:///controlled/gap.go", LanguageID: "go", Disposition: census.FileUnsupported}}
	for i, f := range files {
		id, _ := inventoryscope.CandidateIDV1(in.Declaration.SourceID, f.Path)
		in.Candidates = append(in.Candidates, inventoryscope.CandidateV1{OriginalOrdinal: i, CanonicalRelativePath: f.Path, CandidateID: id, LanguageID: f.LanguageID, DiscoveryProvenance: inventoryscope.ArtifactRefV1{ArtifactID: "controlled-enumeration", Digest: in.Declaration.ConfigDigest}})
	}
	m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := censussourceplan.BuildQualifiedV1(files, in, m, inventoryscope.SelectionV1{State: inventoryscope.CallerSelectedV1, ManifestDigest: m.ScopeDigest, CallerActionRef: "controlled-action"}, "/controlled", 2, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func applicationQualifiedOptionsPrivate() censusApplicationQualifiedOptionsPrivate {
	return censusApplicationQualifiedOptionsPrivate{RevisionAssertion: "controlled-revision", Policy: pilot.ScopeEffectivePolicyPrivateV2{MaxNominationFiles: 2}, MaxArtifactBytes: 1 << 20, MaxMetadataBytes: 48000}
}
func TestApplicationQualifiedPreparationPrivate(t *testing.T) {
	t.Run("OnlyEligibleIncludedFrozen", func(t *testing.T) {
		q := applicationQualifiedFixturePrivate(t)
		calls := 0
		freeze := func(_ context.Context, workspace, revision string, files []censusacquisition.SourceFile, _ pilot.ScopeEffectivePolicyPrivateV2) ([]byte, error) {
			calls++
			if len(files) != 1 || files[0].Path != "main.ts" || workspace != "/controlled" {
				t.Fatal("noneligible or nonincluded file frozen")
			}
			return []byte(`{"controlled_manifest":true}`), nil
		}
		p, raw, e := buildCensusApplicationQualifiedFullPlanWithFreezePrivate(context.Background(), q, applicationQualifiedOptionsPrivate(), inventoryscope.LimitsV1{}, freeze)
		if e != nil {
			t.Fatal(e)
		}
		if calls != 1 || len(p.AnalysisEligible) != 1 || p.AnalysisEligible[0] != 0 || len(p.AnalysisGaps) != 1 || p.AnalysisGaps[0].OriginalOrdinal != 3 || p.ScopeLedger.IncludedCount != 2 {
			t.Fatal("analysis gap changed qualification or freeze selection")
		}
		if len(p.UntouchedRemainder) != 2 || p.UntouchedRemainder[0].OriginalOrdinal != 1 || p.UntouchedRemainder[1].OriginalOrdinal != 2 || p.UntouchedRemainder[0].Entry.Membership != inventoryscope.ExcludedV1 || p.UntouchedRemainder[1].Entry.Membership != inventoryscope.UnresolvedV1 {
			t.Fatal("untouched remainder was not preserved")
		}
		if p.OriginalDenominatorCount != 4 || p.OriginalDenominatorDigest != q.Scope.OriginalDenominatorDigest || p.InputFamily != censusApplicationQualifiedFamilyPrivate || p.ExecutionDisposition != "UNFUNDED_CONNECTED_EXECUTION_PENDING" || len(raw) > 48000 || !json.Valid(raw) {
			t.Fatal("preparation envelope lost fresh family or denominator")
		}
		if p.SourceAuthorizationRequirementFingerprint == q.AnalysisPlan.Identity || p.SourceAuthorizationRequirementFingerprint == q.QualifiedDigest || len(p.SourceAuthorizationRequirementFingerprint) != 71 {
			t.Fatal("source authorization requirement must be fresh and nonoperative")
		}
		for _, origin := range p.Origins {
			b := p.Artifacts[origin.ManifestRef.ArtifactID]
			if censusApplicationQualifiedArtifactDigestPrivate(b) != origin.ManifestRef.Digest {
				t.Fatal("frozen artifact reference mismatch")
			}
		}
		if censusApplicationQualifiedArtifactDigestPrivate(p.Artifacts[p.ScopeLedgerRef.ArtifactID]) != p.ScopeLedgerRef.Digest {
			t.Fatal("scope ledger artifact reference mismatch")
		}
		t.Log("included-only preparation and untouched remainder: PASS")
	})
	t.Run("RefuseBeforeFreeze", func(t *testing.T) {
		q := applicationQualifiedFixturePrivate(t)
		q.Selection.ManifestDigest = q.Scope.ConfigDigest
		calls := 0
		freeze := func(context.Context, string, string, []censusacquisition.SourceFile, pilot.ScopeEffectivePolicyPrivateV2) ([]byte, error) {
			calls++
			return []byte(`{}`), nil
		}
		if _, _, e := buildCensusApplicationQualifiedFullPlanWithFreezePrivate(context.Background(), q, applicationQualifiedOptionsPrivate(), inventoryscope.LimitsV1{}, freeze); e == nil || calls != 0 {
			t.Fatal("invalid qualification reached freeze")
		}
		t.Log("qualified admission before source access: PASS")
	})
	t.Run("BoundsBeforeFreeze", func(t *testing.T) {
		q := applicationQualifiedFixturePrivate(t)
		o := applicationQualifiedOptionsPrivate()
		o.MaxArtifactBytes = 1
		calls := 0
		freeze := func(context.Context, string, string, []censusacquisition.SourceFile, pilot.ScopeEffectivePolicyPrivateV2) ([]byte, error) {
			calls++
			return []byte(`{}`), nil
		}
		if _, _, e := buildCensusApplicationQualifiedFullPlanWithFreezePrivate(context.Background(), q, o, inventoryscope.LimitsV1{}, freeze); e == nil || calls != 0 {
			t.Fatal("artifact bound not enforced before freeze")
		}
		t.Log("bounded preparation artifacts: PASS")
	})
	t.Run("NoWork", func(t *testing.T) {
		q := applicationQualifiedFixturePrivate(t)
		in := q.Input
		in.Declaration.Includes = nil
		m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		s := q.Selection
		s.ManifestDigest = m.ScopeDigest
		q, e = censussourceplan.BuildQualifiedV1(q.Originals, in, m, s, q.Workspace, q.PartitionSize, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		calls := 0
		freeze := func(context.Context, string, string, []censusacquisition.SourceFile, pilot.ScopeEffectivePolicyPrivateV2) ([]byte, error) {
			calls++
			return []byte(`{}`), nil
		}
		p, _, e := buildCensusApplicationQualifiedFullPlanWithFreezePrivate(context.Background(), q, applicationQualifiedOptionsPrivate(), inventoryscope.LimitsV1{}, freeze)
		if e != nil || calls != 0 || len(p.Origins) != 0 || len(p.UntouchedRemainder) != 4 {
			t.Fatal("no-work preparation froze source or lost remainder")
		}
		t.Log("no-work preparation remains nonexecutable: PASS")
	})
	t.Run("Synthetic1099", func(t *testing.T) {
		q := applicationQualifiedFixturePrivate(t)
		in := q.Input
		in.Candidates = nil
		in.Declaration.Includes = nil
		in.Declaration.Excludes = nil
		files := []censusacquisition.SourceFile{}
		for i := 0; i < 1099; i++ {
			path := fmt.Sprintf("controlled/candidate-%04d.txt", i)
			id, _ := inventoryscope.CandidateIDV1(in.Declaration.SourceID, path)
			in.Candidates = append(in.Candidates, inventoryscope.CandidateV1{OriginalOrdinal: i, CanonicalRelativePath: path, CandidateID: id, DiscoveryProvenance: inventoryscope.ArtifactRefV1{ArtifactID: "controlled-1099", Digest: in.Declaration.ConfigDigest}})
			files = append(files, censusacquisition.SourceFile{Path: path, URI: "file:///controlled/" + path})
		}
		m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		s := q.Selection
		s.ManifestDigest = m.ScopeDigest
		q, e = censussourceplan.BuildQualifiedV1(files, in, m, s, q.Workspace, 8, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		o := applicationQualifiedOptionsPrivate()
		o.MaxArtifactBytes = 32 << 20
		calls := 0
		freeze := func(context.Context, string, string, []censusacquisition.SourceFile, pilot.ScopeEffectivePolicyPrivateV2) ([]byte, error) {
			calls++
			return []byte(`{}`), nil
		}
		p, raw, e := buildCensusApplicationQualifiedFullPlanWithFreezePrivate(context.Background(), q, o, inventoryscope.LimitsV1{}, freeze)
		if e != nil || calls != 0 || len(p.ScopeLedger.Entries) != 1099 || p.OriginalDenominatorCount != 1099 || len(p.UntouchedRemainder) != 1099 || len(raw) > 48000 {
			t.Fatal("synthetic 1099 accounting or compact metadata failed", e)
		}
		t.Log("synthetic 1099 complete without actual scope choice: PASS")
	})
	t.Run("FreezePartitionBounds", func(t *testing.T) {
		q := applicationQualifiedFixturePrivate(t)
		files := q.Originals
		files[3].Disposition = ""
		var e error
		q, e = censussourceplan.BuildQualifiedV1(files, q.Input, q.Scope, q.Selection, q.Workspace, 2, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		o := applicationQualifiedOptionsPrivate()
		o.Policy.MaxNominationFiles = 1
		calls := 0
		freeze := func(_ context.Context, _, _ string, files []censusacquisition.SourceFile, _ pilot.ScopeEffectivePolicyPrivateV2) ([]byte, error) {
			calls++
			if len(files) != 1 {
				t.Fatal("freeze nomination bound exceeded")
			}
			return json.Marshal(struct{ Path string }{files[0].Path})
		}
		p, _, e := buildCensusApplicationQualifiedFullPlanWithFreezePrivate(context.Background(), q, o, inventoryscope.LimitsV1{}, freeze)
		if e != nil || calls != 2 || len(p.Origins) != 2 || len(p.AnalysisEligible) != 2 {
			t.Fatal("qualified partition did not respect actual freeze bound", e)
		}
		t.Log("actual freeze nomination bound preserved: PASS")
	})
}
