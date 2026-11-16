package censussourceplan

import (
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/inventoryscope"
)

func repair86QualifiedFixture(t *testing.T, workspace, uriRoot string, paths []string, membership inventoryscope.MembershipV1) ([]censusacquisition.SourceFile, inventoryscope.InputV1, inventoryscope.ManifestV1, inventoryscope.SelectionV1) {
	t.Helper()
	in := inventoryscope.InputV1{Schema: inventoryscope.SchemaV1, Declaration: inventoryscope.DeclarationV1{Subject: "controlled-repair", SourceID: "controlled-source", ConfigDigest: "sha256:" + strings.Repeat("a", 64), PolicyID: inventoryscope.PolicyConsensusV1}}
	files := []censusacquisition.SourceFile{}
	for i, p := range paths {
		id, err := inventoryscope.CandidateIDV1(in.Declaration.SourceID, p)
		if err != nil {
			t.Fatal(err)
		}
		in.Candidates = append(in.Candidates, inventoryscope.CandidateV1{OriginalOrdinal: i, CanonicalRelativePath: p, CandidateID: id, LanguageID: "typescript", DiscoveryProvenance: inventoryscope.ArtifactRefV1{ArtifactID: "controlled-discovery", Digest: in.Declaration.ConfigDigest}})
		if membership == inventoryscope.IncludedV1 {
			in.Declaration.Includes = append(in.Declaration.Includes, p)
		}
		if membership == inventoryscope.ExcludedV1 {
			in.Declaration.Excludes = append(in.Declaration.Excludes, p)
		}
		files = append(files, censusacquisition.SourceFile{Path: p, URI: (&url.URL{Scheme: "file", Path: uriRoot + "/" + p}).String(), LanguageID: "typescript", Disposition: census.FileUnreadable})
	}
	m, err := inventoryscope.QualifyV1(in, inventoryscope.LimitsV1{})
	if err != nil {
		t.Fatal(err)
	}
	return files, in, m, inventoryscope.SelectionV1{State: inventoryscope.CallerSelectedV1, ManifestDigest: m.ScopeDigest, CallerActionRef: "controlled-selection"}
}
func repair86PlanAssert(t *testing.T, assertion string, ok bool, detail string) {
	t.Helper()
	if !ok {
		t.Errorf("%s FAIL: %s", assertion, detail)
		return
	}
	t.Logf("%s PASS: %s", assertion, detail)
}
func TestRepair86QualifiedCaseDistinct(t *testing.T) {
	for _, root := range [][2]string{{"/controlled", "/controlled"}, {`C:\controlled`, "/C:/controlled"}, {"C:/controlled", "/C:/controlled"}, {"/", ""}, {`C:\`, "/C:"}} {
		t.Run(root[0], func(t *testing.T) {
			files, in, m, s := repair86QualifiedFixture(t, root[0], root[1], []string{"a.ts", "A.ts"}, inventoryscope.IncludedV1)
			q, err := BuildQualifiedV1(files, in, m, s, root[0], 1, inventoryscope.LimitsV1{})
			if err != nil {
				repair86PlanAssert(t, "qualified-case-distinct", false, fmt.Sprintf("workspace=%q legacyKey=%q err=%v", root[0], platformPathKey("A.ts"), err))
				return
			}
			ok := len(q.AnalysisPlan.Files) == 2 && len(q.AnalysisPlan.Partitions) == 2 && len(q.OriginalOrdinals) == 2
			if ok {
				ok = q.AnalysisPlan.Files[0] == files[1] && q.AnalysisPlan.Files[1] == files[0] && q.OriginalOrdinals[0] == 1 && q.OriginalOrdinals[1] == 0 && q.AnalysisPlan.Partitions[0].Members[0] == files[1] && q.AnalysisPlan.Partitions[1].Members[0] == files[0]
			}
			repair86PlanAssert(t, "qualified-case-distinct", ok && ValidateQualifiedV1(q, inventoryscope.LimitsV1{}) == nil, fmt.Sprintf("workspace=%q legacyKey=%q exact files/partitions/ordinals/recompute", root[0], platformPathKey("A.ts")))
		})
	}
}
func TestRepair86QualifiedExactURI(t *testing.T) {
	for _, membership := range []inventoryscope.MembershipV1{inventoryscope.IncludedV1, inventoryscope.ExcludedV1, inventoryscope.UnresolvedV1} {
		t.Run(string(membership), func(t *testing.T) {
			files, in, m, s := repair86QualifiedFixture(t, "/controlled", "/controlled", []string{"a.ts"}, membership)
			q, err := BuildQualifiedV1(files, in, m, s, "/controlled", 1, inventoryscope.LimitsV1{})
			if err != nil {
				t.Fatal(err)
			}
			// Complete originals are bound even when no member is INCLUDED.
			for _, uri := range []string{"file:///controlled/A.ts", "file:///Controlled/a.ts", "file:///foreign/a.ts", "file:///controlled/%61.ts", "file:///controlled/a.ts?x=1", "file:///controlled/../a.ts"} {
				changed := append([]censusacquisition.SourceFile{}, files...)
				changed[0].URI = uri
				_, err = BuildQualifiedV1(changed, in, m, s, "/controlled", 1, inventoryscope.LimitsV1{})
				repair86PlanAssert(t, "qualified-exact-uri", err != nil, fmt.Sprintf("membership=%s URI=%s legacyKey=%q err=%v", membership, uri, platformPathKey("A.ts"), err))
				forged := q
				forged.Originals = changed
				if len(q.AnalysisPlan.Files) > 0 {
					forged.AnalysisPlan, err = Build(changed, "/controlled", 1)
					if err != nil {
						continue
					} // legacy rejects this forged representation itself
				}
				forged.QualifiedDigest = qualifiedDigestV1(forged)
				repair86PlanAssert(t, "qualified-exact-uri", ValidateQualifiedV1(forged, inventoryscope.LimitsV1{}) != nil, "rehashed forged originals/analysis cannot bypass exact validation")
			}
		})
	}
}
func TestRepair86DriveExactURI(t *testing.T) {
	for _, root := range []string{`C:\controlled`, "C:/controlled"} {
		for _, membership := range []inventoryscope.MembershipV1{inventoryscope.IncludedV1, inventoryscope.ExcludedV1, inventoryscope.UnresolvedV1} {
			files, in, m, s := repair86QualifiedFixture(t, root, "/C:/controlled", []string{"a.ts"}, membership)
			q, err := BuildQualifiedV1(files, in, m, s, root, 1, inventoryscope.LimitsV1{})
			repair86PlanAssert(t, "qualified-exact-uri", err == nil && ValidateQualifiedV1(q, inventoryscope.LimitsV1{}) == nil, "exact drive-root spelling accepted and recomputed")
			for _, uri := range []string{"file:///C:/controlled/A.ts", "file:///c:/controlled/a.ts", "file:///C:/Controlled/a.ts", "file:///D:/controlled/a.ts"} {
				changed := append([]censusacquisition.SourceFile{}, files...)
				changed[0].URI = uri
				_, err = BuildQualifiedV1(changed, in, m, s, root, 1, inventoryscope.LimitsV1{})
				repair86PlanAssert(t, "qualified-exact-uri", err != nil, fmt.Sprintf("root=%q membership=%s mismatched URI=%s", root, membership, uri))
			}
		}
	}
}

func TestRepair86LegacyFoldControl(t *testing.T) {
	workspace, uriRoot := "/controlled", "/controlled"
	if runtime.GOOS == "windows" {
		workspace = `C:\controlled`
		uriRoot = "/C:/controlled"
	}
	files, _, _, _ := repair86QualifiedFixture(t, workspace, uriRoot, []string{"A.ts", "a.ts"}, inventoryscope.IncludedV1)
	folded := platformPathKey("A.ts") == platformPathKey("a.ts")
	_, duplicateErr := Build(files, workspace, 1)
	mismatch := append([]censusacquisition.SourceFile{}, files[:1]...)
	mismatch[0].Path = "a.ts"
	_, bindingErr := Build(mismatch, workspace, 1)
	if folded && (duplicateErr != ErrDuplicateIdentity || bindingErr != nil) {
		t.Fatalf("legacy Windows-fold control inconsistent: duplicate=%v mismatch=%v", duplicateErr, bindingErr)
	}
	if !folded && (duplicateErr != nil || bindingErr == nil) {
		t.Fatalf("legacy exact control inconsistent: duplicate=%v mismatch=%v", duplicateErr, bindingErr)
	}
	t.Logf("legacy-fold-control PASS: fold=%t host=%s; pure branch control, not native Windows execution", folded, runtime.GOOS)
}
