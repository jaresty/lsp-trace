package inventoryscope

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func fixtureV1(t *testing.T) InputV1 {
	t.Helper()
	in := InputV1{Schema: SchemaV1, Declaration: DeclarationV1{Subject: "synthetic", SourceID: "controlled-source", ConfigDigest: "sha256:" + strings.Repeat("a", 64), PolicyID: PolicyConsensusV1}, Candidates: []CandidateV1{}}
	for i, p := range []string{"app/main.go", ".evidence/helper.ts", "outside/data.txt"} {
		id, e := CandidateIDV1(in.Declaration.SourceID, p)
		if e != nil {
			t.Fatal(e)
		}
		in.Candidates = append(in.Candidates, CandidateV1{i, p, id, []string{"go", "typescript", "unknown"}[i], ArtifactRefV1{"discovery", in.Declaration.ConfigDigest}})
	}
	return in
}
func observationV1(t *testing.T, in InputV1, index int, kind ObservationKindV1, assertion MembershipV1, artifact string) ObservationV1 {
	t.Helper()
	o := ObservationV1{CandidateID: in.Candidates[index].CandidateID, Assertion: assertion, Kind: kind, ProducerIdentity: "synthetic-producer", ProducerVersion: "v1", Artifact: ArtifactRefV1{artifact, in.Declaration.ConfigDigest}, SourceID: in.Declaration.SourceID, ConfigDigest: in.Declaration.ConfigDigest}
	var e error
	o.ObservationID, e = ObservationIDV1(o, LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	return o
}
func entryV1(t *testing.T, m ManifestV1, id string) EntryV1 {
	t.Helper()
	for _, e := range m.Entries {
		if e.CandidateID == id {
			return e
		}
	}
	t.Fatal("missing entry")
	return EntryV1{}
}
func qualifyTestV1(t *testing.T, in InputV1) ManifestV1 {
	t.Helper()
	m, e := QualifyV1(in, LimitsV1{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func cloneInputTestV1(in InputV1) InputV1 {
	b, _ := json.Marshal(in)
	var out InputV1
	_ = json.Unmarshal(b, &out)
	return out
}

func TestResolutionSetDeterminismV1(t *testing.T) {
	in := fixtureV1(t)
	in.Declaration.PolicyID = PolicyIntentFirstV1
	for i := 0; i < 2; i++ {
		in.Declaration.Includes = append(in.Declaration.Includes, in.Candidates[i].CanonicalRelativePath)
		a := observationV1(t, in, i, BuildMembershipV1, ExcludedV1, "build")
		b := observationV1(t, in, i, ResourceGapV1, UnresolvedV1, "resource")
		in.Observations = append(in.Observations, a, b)
		r := ResolutionV1{CandidateID: in.Candidates[i].CandidateID, ObservationRefs: []string{a.ObservationID, b.ObservationID}, Rule: ResolutionIntentFirstV1, Outcome: IncludedV1, CallerActionRef: "explicit-resolution"}
		r.ResolutionID, _ = ResolutionIDV1(r, LimitsV1{})
		in.Resolutions = append(in.Resolutions, r)
	}
	m := qualifyTestV1(t, in)
	before, _ := CanonicalManifestV1(m, LimitsV1{})
	for i, j := 0, len(in.Observations)-1; i < j; i, j = i+1, j-1 {
		in.Observations[i], in.Observations[j] = in.Observations[j], in.Observations[i]
	}
	in.Resolutions[0], in.Resolutions[1] = in.Resolutions[1], in.Resolutions[0]
	for i := range in.Resolutions {
		r := &in.Resolutions[i]
		r.ObservationRefs[0], r.ObservationRefs[1] = r.ObservationRefs[1], r.ObservationRefs[0]
	}
	in.Candidates[0], in.Candidates[2] = in.Candidates[2], in.Candidates[0]
	in.Declaration.Includes[0], in.Declaration.Includes[1] = in.Declaration.Includes[1], in.Declaration.Includes[0]
	after, _ := CanonicalManifestV1(qualifyTestV1(t, in), LimitsV1{})
	if !bytes.Equal(before, after) {
		t.Fatal("resolution set permutation changed canonical scope")
	}
	t.Log("all evidence resolution sets deterministic: PASS")
}

func TestQualificationV1(t *testing.T) {
	t.Run("MissingUnresolved", func(t *testing.T) {
		in := fixtureV1(t)
		m := qualifyTestV1(t, in)
		if m.UnresolvedCount != 3 || m.IncludedCount != 0 || m.ExcludedCount != 0 {
			t.Fatal("missing observations must remain unresolved")
		}
		t.Log("missing observations must remain unresolved: PASS")
	})
	t.Run("CompleteAccounting", func(t *testing.T) {
		in := fixtureV1(t)
		m := qualifyTestV1(t, in)
		if len(m.Entries) != 3 || m.OriginalDenominatorCount != 3 {
			t.Fatal("complete original ledger required")
		}
		m.Entries = m.Entries[:2]
		if ValidateV1(in, m, LimitsV1{}) == nil {
			t.Fatal("missing manifest entry accepted")
		}
		t.Log("complete original ledger required: PASS")
	})
	t.Run("IdentityAdmission", func(t *testing.T) {
		for _, p := range []string{"a/./b", "../b", "/b", "a\\b", "C:/b", "a\x00b", "a//b", "a/../b", string([]byte{255})} {
			in := fixtureV1(t)
			in.Candidates[0].CanonicalRelativePath = p
			if _, e := QualifyV1(in, LimitsV1{}); e == nil {
				t.Fatal("aliased source identity accepted")
			}
		}
		in := fixtureV1(t)
		in.Candidates[1] = in.Candidates[0]
		if _, e := QualifyV1(in, LimitsV1{}); e == nil {
			t.Fatal("duplicate candidate accepted")
		}
		in = fixtureV1(t)
		in.Candidates[1].OriginalOrdinal = 3
		if _, e := QualifyV1(in, LimitsV1{}); e == nil {
			t.Fatal("noncontiguous original ordinal accepted")
		}
		in = fixtureV1(t)
		in.Candidates[0].CandidateID = in.Candidates[1].CandidateID
		if _, e := QualifyV1(in, LimitsV1{}); e == nil {
			t.Fatal("foreign candidate identity accepted")
		}
		t.Log("canonical identity admission: PASS")
	})
	t.Run("ConflictResolution", func(t *testing.T) {
		in := fixtureV1(t)
		in.Declaration.PolicyID = PolicyIntentFirstV1
		in.Declaration.Includes = []string{in.Candidates[0].CanonicalRelativePath}
		in.Observations = []ObservationV1{observationV1(t, in, 0, BuildMembershipV1, ExcludedV1, "build")}
		m := qualifyTestV1(t, in)
		if entryV1(t, m, in.Candidates[0].CandidateID).Membership != UnresolvedV1 {
			t.Fatal("conflict must remain unresolved without resolution")
		}
		r := ResolutionV1{CandidateID: in.Candidates[0].CandidateID, ObservationRefs: []string{in.Observations[0].ObservationID}, Rule: ResolutionIntentFirstV1, Outcome: IncludedV1, CallerActionRef: "explicit-action"}
		r.ResolutionID, _ = ResolutionIDV1(r, LimitsV1{})
		in.Resolutions = []ResolutionV1{r}
		m = qualifyTestV1(t, in)
		if entryV1(t, m, r.CandidateID).Membership != IncludedV1 || in.Observations[0].Assertion != ExcludedV1 {
			t.Fatal("typed intent resolution must preserve build fact")
		}
		in.Resolutions[0].ObservationRefs = nil
		in.Resolutions[0].ResolutionID, _ = ResolutionIDV1(in.Resolutions[0], LimitsV1{})
		if _, e := QualifyV1(in, LimitsV1{}); e == nil {
			t.Fatal("uncited conflict resolution accepted")
		}
		t.Log("conflict and explicit resolution: PASS")
	})
	t.Run("AnalysisNotMembership", func(t *testing.T) {
		for _, kind := range []ObservationKindV1{AnalysisFailureV1, UnreadableV1, UnsupportedLanguageV1, ResourceGapV1} {
			in := fixtureV1(t)
			in.Observations = []ObservationV1{observationV1(t, in, 0, kind, UnresolvedV1, "analysis")}
			m := qualifyTestV1(t, in)
			if entryV1(t, m, in.Candidates[0].CandidateID).Membership != UnresolvedV1 {
				t.Fatal("analysis gap changed membership")
			}
			in.Declaration.Includes = []string{in.Candidates[0].CanonicalRelativePath}
			m = qualifyTestV1(t, in)
			if entryV1(t, m, in.Candidates[0].CandidateID).Membership != IncludedV1 {
				t.Fatal("analysis gap excluded explicit inclusion")
			}
		}
		t.Log("analysis disposition is not membership: PASS")
	})
	t.Run("RootIntentNotLanguage", func(t *testing.T) {
		in := fixtureV1(t)
		in.Declaration.ApplicationRoots = []string{"app"}
		m := qualifyTestV1(t, in)
		if m.UnresolvedCount != 3 {
			t.Fatal("roots alone inferred membership")
		}
		in.Declaration.RootBoundary = true
		m = qualifyTestV1(t, in)
		if m.ExcludedCount != 2 || entryV1(t, m, in.Candidates[1].CandidateID).Reason != ReasonOutsideRootV1 {
			t.Fatal("explicit root boundary not accounted")
		}
		in.Declaration.RootBoundary = false
		in.Declaration.Excludes = []string{".evidence/helper.ts"}
		m = qualifyTestV1(t, in)
		if m.ExcludedCount != 1 || m.UnresolvedCount != 2 {
			t.Fatal("explicit hidden-path exclusion not preserved")
		}
		in = fixtureV1(t)
		in.Declaration.Includes = []string{in.Candidates[0].CanonicalRelativePath, in.Candidates[1].CanonicalRelativePath}
		m = qualifyTestV1(t, in)
		if m.IncludedCount != 2 {
			t.Fatal("language labels must not filter declared intent")
		}
		t.Log("declared intent is language agnostic: PASS")
	})
	t.Run("Bindings", func(t *testing.T) {
		in := fixtureV1(t)
		in.Observations = []ObservationV1{observationV1(t, in, 0, MembershipFactV1, IncludedV1, "member")}
		for _, change := range []func(*InputV1){func(v *InputV1) { v.Observations[0].SourceID = "foreign" }, func(v *InputV1) { v.Observations[0].ConfigDigest = "sha256:" + strings.Repeat("b", 64) }, func(v *InputV1) { v.Observations[0].CandidateID = "foreign" }, func(v *InputV1) { v.Observations = append(v.Observations, v.Observations[0]) }} {
			v := cloneInputTestV1(in)
			change(&v)
			if _, e := QualifyV1(v, LimitsV1{}); e == nil {
				t.Fatal("foreign or duplicate evidence accepted")
			}
		}
		t.Log("source config evidence binding: PASS")
	})
	t.Run("Determinism", func(t *testing.T) {
		in := fixtureV1(t)
		in.Declaration.ApplicationRoots = []string{"outside", "app"}
		in.Declaration.Includes = []string{in.Candidates[1].CanonicalRelativePath, in.Candidates[0].CanonicalRelativePath}
		in.Observations = []ObservationV1{observationV1(t, in, 0, MembershipFactV1, IncludedV1, "first"), observationV1(t, in, 1, BuildMembershipV1, IncludedV1, "second")}
		m := qualifyTestV1(t, in)
		a, _ := CanonicalManifestV1(m, LimitsV1{})
		in.Candidates[0], in.Candidates[2] = in.Candidates[2], in.Candidates[0]
		in.Observations[0], in.Observations[1] = in.Observations[1], in.Observations[0]
		in.Declaration.Includes[0], in.Declaration.Includes[1] = in.Declaration.Includes[1], in.Declaration.Includes[0]
		in.Declaration.ApplicationRoots[0], in.Declaration.ApplicationRoots[1] = in.Declaration.ApplicationRoots[1], in.Declaration.ApplicationRoots[0]
		n := qualifyTestV1(t, in)
		b, _ := CanonicalManifestV1(n, LimitsV1{})
		if !bytes.Equal(a, b) {
			t.Fatal("set permutation changed canonical scope")
		}
		t.Log("canonical scope deterministic: PASS")
	})
	t.Run("IdentityChanges", func(t *testing.T) {
		in := fixtureV1(t)
		m := qualifyTestV1(t, in)
		for _, change := range []func(*InputV1){func(v *InputV1) { v.Declaration.ConfigDigest = "sha256:" + strings.Repeat("b", 64) }, func(v *InputV1) { v.Declaration.PolicyID = PolicyIntentFirstV1 }, func(v *InputV1) { v.Declaration.Includes = []string{v.Candidates[0].CanonicalRelativePath} }, func(v *InputV1) {
			v.Declaration.SourceID = "new-source"
			for i := range v.Candidates {
				v.Candidates[i].CandidateID, _ = CandidateIDV1(v.Declaration.SourceID, v.Candidates[i].CanonicalRelativePath)
			}
		}, func(v *InputV1) {
			v.Observations = []ObservationV1{observationV1(t, *v, 0, AnalysisFailureV1, UnresolvedV1, "new-artifact")}
		}} {
			v := cloneInputTestV1(in)
			change(&v)
			if qualifyTestV1(t, v).ScopeDigest == m.ScopeDigest {
				t.Fatal("scope identity failed to bind changed facts")
			}
		}
		t.Log("scope digest binds all facts: PASS")
	})
	t.Run("Tamper", func(t *testing.T) {
		in := fixtureV1(t)
		m := qualifyTestV1(t, in)
		for _, change := range []func(*ManifestV1){func(v *ManifestV1) { v.IncludedCount++ }, func(v *ManifestV1) { v.ScopeDigest = v.ConfigDigest }, func(v *ManifestV1) { v.OriginalDenominatorDigest = v.ConfigDigest }, func(v *ManifestV1) { v.Entries[0].EvidenceRefs = []string{v.ConfigDigest} }, func(v *ManifestV1) { v.Entries[0].Membership = IncludedV1 }, func(v *ManifestV1) { v.Entries[0].CandidateID = "foreign" }} {
			b, _ := json.Marshal(m)
			var v ManifestV1
			_ = json.Unmarshal(b, &v)
			change(&v)
			if ValidateV1(in, v, LimitsV1{}) == nil {
				t.Fatal("tampered manifest accepted")
			}
		}
		t.Log("manifest fully recomputed: PASS")
	})
	t.Run("Limits", func(t *testing.T) {
		in := fixtureV1(t)
		for _, l := range []LimitsV1{{MaxCandidates: 2}, {MaxUTF8Bytes: 5}, {MaxWork: 5}, {MaxInputBytes: 10}, {MaxInputBytes: 1 << 40}, {MaxCandidates: -1}} {
			if _, e := QualifyV1(in, l); !errors.Is(e, ErrLimitV1) {
				t.Fatal("input limit not enforced")
			}
		}
		t.Log("preallocation budgets enforced: PASS")
	})
	t.Run("Selection", func(t *testing.T) {
		m := qualifyTestV1(t, fixtureV1(t))
		if ValidateSelectionV1(SelectionV1{CallerSelectedV1, m.ScopeDigest, "action"}, m) != nil {
			t.Fatal("explicit selection rejected")
		}
		for _, s := range []SelectionV1{{CallerSelectedV1, m.ScopeDigest, ""}, {CallerSelectedV1, m.ConfigDigest, "action"}, {ProposedV1, m.ScopeDigest, "action"}, {"GRANTED", m.ScopeDigest, "action"}} {
			if ValidateSelectionV1(s, m) == nil {
				t.Fatal("invalid selection accepted")
			}
		}
		t.Log("selection exact and nonauthorizing: PASS")
	})
	t.Run("StrictDecode", func(t *testing.T) {
		in := fixtureV1(t)
		b, _ := json.Marshal(in)
		if _, e := DecodeInputV1(bytes.NewReader(b), LimitsV1{}); e != nil {
			t.Fatal(e)
		}
		for _, raw := range [][]byte{append(b, []byte(" {}")...), []byte(`{"schema":"inventory-scope/v1","schema":"secret-payload"}`), []byte(`{"secret-payload":true}`), bytes.Replace(b, []byte(`"schema"`), []byte(`"Schema"`), 1), []byte{255}, []byte(`null`)} {
			if _, e := DecodeInputV1(bytes.NewReader(raw), LimitsV1{}); e == nil || strings.Contains(e.Error(), "secret-payload") {
				t.Fatal("strict decoder leaked or accepted invalid input")
			}
		}
		if _, e := DecodeInputV1(bytes.NewReader(b), LimitsV1{MaxInputBytes: int64(len(b) - 1)}); !errors.Is(e, ErrLimitV1) {
			t.Fatal("byte sentinel not enforced")
		}
		if _, e := DecodeInputV1(bytes.NewReader(b), LimitsV1{MaxCandidates: 2}); !errors.Is(e, ErrLimitV1) {
			t.Fatal("decode count cap not enforced")
		}
		if _, e := DecodeInputV1(bytes.NewReader(b), LimitsV1{MaxUTF8Bytes: 20}); !errors.Is(e, ErrLimitV1) {
			t.Fatal("decoded aggregate byte cap not enforced")
		}
		if _, e := DecodeInputV1(bytes.NewReader(b), LimitsV1{MaxWork: 5}); !errors.Is(e, ErrLimitV1) {
			t.Fatal("decoded work cap not enforced")
		}
		t.Log("strict bounded decoder: PASS")
	})
}
