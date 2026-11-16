package censussourceplan

import (
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/inventoryscope"
)

func qualifiedFixtureV1(t *testing.T) QualifiedInputV1 {
	t.Helper()
	in := inventoryscope.InputV1{Schema: inventoryscope.SchemaV1, Declaration: inventoryscope.DeclarationV1{Subject: "synthetic", SourceID: "synthetic-source", ConfigDigest: "sha256:" + strings.Repeat("a", 64), PolicyID: inventoryscope.PolicyConsensusV1, Includes: []string{"b.ts", "a.go"}, Excludes: []string{"excluded.txt"}}}
	files := []censusacquisition.SourceFile{{Path: "b.ts", URI: "file:///controlled/b.ts", LanguageID: "typescript"}, {Path: "a.go", URI: "file:///controlled/a.go", LanguageID: "go", Disposition: census.FileUnreadable}, {Path: "excluded.txt", URI: "file:///controlled/excluded.txt"}, {Path: "unresolved.txt", URI: "file:///controlled/unresolved.txt"}}
	for i, f := range files {
		id, _ := inventoryscope.CandidateIDV1(in.Declaration.SourceID, f.Path)
		in.Candidates = append(in.Candidates, inventoryscope.CandidateV1{OriginalOrdinal: i, CanonicalRelativePath: f.Path, CandidateID: id, LanguageID: f.LanguageID, DiscoveryProvenance: inventoryscope.ArtifactRefV1{ArtifactID: "synthetic-discovery", Digest: in.Declaration.ConfigDigest}})
	}
	m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := BuildQualifiedV1(files, in, m, inventoryscope.SelectionV1{State: inventoryscope.CallerSelectedV1, ManifestDigest: m.ScopeDigest, CallerActionRef: "explicit-selection"}, "/controlled", 1, inventoryscope.LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func cloneQualifiedTestV1(q QualifiedInputV1) QualifiedInputV1 {
	b, _ := json.Marshal(q)
	var v QualifiedInputV1
	_ = json.Unmarshal(b, &v)
	return v
}
func TestQualifiedV1(t *testing.T) {
	t.Run("CanonicalInputArtifact", func(t *testing.T) {
		q := qualifiedFixtureV1(t)
		in := cloneQualifiedTestV1(q).Input
		in.Candidates[0], in.Candidates[3] = in.Candidates[3], in.Candidates[0]
		in.Declaration.Includes[0], in.Declaration.Includes[1] = in.Declaration.Includes[1], in.Declaration.Includes[0]
		in.Observations = nil
		in.Resolutions = nil
		in.Declaration.ApplicationRoots = nil
		m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		other, e := BuildQualifiedV1(q.Originals, in, m, q.Selection, q.Workspace, q.PartitionSize, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		a, _ := json.Marshal(q)
		b, _ := json.Marshal(other)
		if string(a) != string(b) || q.QualifiedDigest != other.QualifiedDigest {
			t.Fatal("qualified artifact changed under set permutation")
		}
		t.Log("qualified artifact set ordering canonical: PASS")
	})
	t.Run("OnlyIncluded", func(t *testing.T) {
		q := qualifiedFixtureV1(t)
		if len(q.AnalysisPlan.Files) != 2 || q.AnalysisPlan.Files[0].Path != "a.go" || q.AnalysisPlan.Files[0].Disposition != census.FileUnreadable || len(q.AnalysisPlan.Partitions) != 2 || q.OriginalOrdinals[0] != 1 || q.OriginalOrdinals[1] != 0 || q.Scope.UnresolvedCount != 1 || q.Scope.ExcludedCount != 1 {
			t.Fatal("qualified plan must preserve only included members and original ordinals")
		}
		if ValidateQualifiedV1(q, inventoryscope.LimitsV1{}) != nil {
			t.Fatal("valid qualified plan rejected")
		}
		t.Log("qualified members and original ordinals: PASS")
	})
	t.Run("NewFamily", func(t *testing.T) {
		q := qualifiedFixtureV1(t)
		if q.QualifiedDigest == q.AnalysisPlan.Identity || q.InputFamily != QualifiedFamilyV1 {
			t.Fatal("legacy identity reused for qualified scope")
		}
		v := cloneQualifiedTestV1(q)
		v.QualifiedDigest = v.AnalysisPlan.Identity
		if ValidateQualifiedV1(v, inventoryscope.LimitsV1{}) == nil {
			t.Fatal("old analysis digest accepted")
		}
		v = cloneQualifiedTestV1(q)
		v.InputFamily = "census-source-plan/v1"
		if ValidateQualifiedV1(v, inventoryscope.LimitsV1{}) == nil {
			t.Fatal("old family accepted")
		}
		t.Log("qualified identity is a distinct family: PASS")
	})
	t.Run("Tamper", func(t *testing.T) {
		q := qualifiedFixtureV1(t)
		for _, change := range []func(*QualifiedInputV1){func(v *QualifiedInputV1) { v.OriginalOrdinals[0] = 0 }, func(v *QualifiedInputV1) { v.AnalysisPlan.Partitions[0].Members[0] = v.Originals[2] }, func(v *QualifiedInputV1) { v.AnalysisPlan.Files[0] = v.Originals[2] }, func(v *QualifiedInputV1) { v.Originals[2].URI = "file:///controlled/foreign.txt" }, func(v *QualifiedInputV1) { v.Scope.IncludedCount++ }, func(v *QualifiedInputV1) { v.Selection.ManifestDigest = v.Scope.ConfigDigest }, func(v *QualifiedInputV1) {
			v.Selection.State = inventoryscope.ProposedV1
			v.Selection.CallerActionRef = ""
		}, func(v *QualifiedInputV1) { v.Originals = v.Originals[:3] }, func(v *QualifiedInputV1) { v.Workspace = "/controlled\x00" }} {
			v := cloneQualifiedTestV1(q)
			change(&v)
			if ValidateQualifiedV1(v, inventoryscope.LimitsV1{}) == nil {
				t.Fatal("qualified tamper accepted")
			}
		}
		t.Log("qualified bindings fully recomputed: PASS")
	})
	t.Run("EmptyIncluded", func(t *testing.T) {
		q := qualifiedFixtureV1(t)
		in := q.Input
		in.Declaration.Includes = nil
		m, e := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		s := q.Selection
		s.ManifestDigest = m.ScopeDigest
		q, e = BuildQualifiedV1(q.Originals, in, m, s, q.Workspace, q.PartitionSize, inventoryscope.LimitsV1{})
		if e != nil {
			t.Fatal(e)
		}
		if !q.NoWork || len(q.AnalysisPlan.Files) != 0 || len(q.AnalysisPlan.Partitions) != 0 || q.AnalysisPlan.Identity != "" || ValidateQualifiedV1(q, inventoryscope.LimitsV1{}) != nil {
			t.Fatal("empty included scope must be explicit no work")
		}
		t.Log("empty included scope has no executable partitions: PASS")
	})
}
