package adr0007locationv2

import (
	"fmt"
	"lsp-trace/internal/sourceadmissionv2"
)

type Condition struct {
	Schema             string         `json:"schema"`
	CaseID             string         `json:"caseId"`
	AdmissionAvailable bool           `json:"admissionAvailable"`
	Cancel             bool           `json:"cancel"`
	Deadline           bool           `json:"deadline"`
	Limits             LimitsArtifact `json:"limits"`
}
type LimitsArtifact struct {
	Schema             string `json:"schema"`
	MaxRequestBytes    uint64 `json:"maxRequestBytes"`
	MaxMembers         uint64 `json:"maxMembers"`
	MaxPaths           uint64 `json:"maxPaths"`
	MaxSelectorRanges  uint64 `json:"maxSelectorRanges"`
	MaxMemberRanges    uint64 `json:"maxMemberRanges"`
	MaxPrefixExpansion uint64 `json:"maxPrefixExpansion"`
	MaxWitnesses       uint64 `json:"maxWitnesses"`
	MaxSourceBytes     uint64 `json:"maxSourceBytes"`
	MaxOutputBytes     uint64 `json:"maxOutputBytes"`
	MaxWork            uint64 `json:"maxWork"`
}
type FixtureEnvelope struct {
	Schema    string   `json:"schema"`
	CaseID    string   `json:"caseId"`
	Ordinal   int      `json:"ordinal"`
	Name      string   `json:"name"`
	Artifacts []string `json:"artifacts"`
}
type DesignCase struct {
	Name      string
	Condition Condition
	Request   Request
	Admission *Admission
	Expected  Result
}
type fixtureControl struct{ c, d bool }

func (x fixtureControl) Cancelled() bool        { return x.c }
func (x fixtureControl) DeadlineExceeded() bool { return x.d }
func DefaultLimits() Limits {
	return Limits{1 << 20, 64, 64, 128, 256, 64, 256, 1 << 20, 1 << 20, 1 << 20}
}
func limitsArtifact(l Limits) LimitsArtifact {
	return LimitsArtifact{"lsp-trace.adr0007.location.limits.v2", l.MaxRequestBytes, l.MaxMembers, l.MaxPaths, l.MaxSelectorRanges, l.MaxMemberRanges, l.MaxPrefixExpansion, l.MaxWitnesses, l.MaxSourceBytes, l.MaxOutputBytes, l.MaxWork}
}
func BuildDesignCases() []DesignCase {
	names := []string{"exact-file-intersects", "exact-file-ineligible", "range-contained-by", "range-contains", "partial-intersection", "adjacent-not-intersection", "range-union", "prefix-frozen-expansion", "invalid-selector", "non-nfc-path", "empty-range", "stale-admission", "admission-unavailable", "unavailable-member", "duplicate-member", "invalid-member", "policy-filtered", "complete-denominator-topk", "member-limit", "prefix-limit", "witness-limit", "exact-work-boundary", "work-plus-one", "cancel-deadline"}
	out := make([]DesignCase, 24)
	for i, n := range names {
		out[i] = baseCase(i+1, n)
	}
	out[1].Request.Members[0].Ranges = []Range{{Position{5, 0}, Position{6, 0}}}
	out[2].Request.Relation = ContainedBy
	out[3].Request.Relation = Contains
	out[3].Request.Selector.Union[0].Ranges = []Range{{Position{1, 2}, Position{1, 3}}}
	out[4].Request.Selector.Union[0].Ranges = []Range{{Position{1, 0}, Position{1, 2}}}
	out[5].Request.Selector.Union[0].Ranges = []Range{{Position{0, 0}, Position{1, 0}}}
	out[5].Request.Members[0].Ranges = []Range{{Position{1, 0}, Position{2, 0}}}
	out[6].Request.Selector.Union = append(out[6].Request.Selector.Union, PathRanges{Path: "src/b.go", Ranges: []Range{{Position{0, 0}, Position{1, 0}}}})
	out[6].Request.Members = append(out[6].Request.Members, candidate(out[6].Admission.Sources[1], "m2", Range{Position{0, 0}, Position{1, 0}}, 2))
	out[7].Request.Selector = Selector{Kind: PathPrefix, Path: "src", FrozenPaths: []string{"src/a.go", "src/b.go"}}
	out[8].Request.Selector = Selector{Kind: ExactFile, Path: "src/a.go", Union: []PathRanges{{Path: "src/a.go", Ranges: []Range{{Position{0, 0}, Position{1, 0}}}}}}
	out[9].Request.Selector.Union[0].Path = "src/e\u0301.go"
	out[10].Request.Selector.Union[0].Ranges = []Range{{Position{1, 0}, Position{1, 0}}}
	out[11].Request.AdmissionDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	out[12].Condition.AdmissionAvailable = false
	out[13].Request.Members[0].Available = false
	out[14].Request.Members = append(out[14].Request.Members, out[14].Request.Members[0])
	out[15].Request.Members[0].Path = "missing.go"
	out[16].Request.Members[0].PolicyAllowed = false
	all := []Candidate{}
	for j := 0; j < 6; j++ {
		m := candidate(out[17].Admission.Sources[0], fmt.Sprintf("m%d", j), Range{Position{1, 0}, Position{2, 0}}, int64(10-j))
		all = append(all, m)
	}
	all[1].Ranges = []Range{{Position{5, 0}, Position{6, 0}}}
	all[2].Available = false
	all[3].Path = "missing.go"
	all[4].ID = all[0].ID
	all[5].PolicyAllowed = false
	out[17].Request.Members = all
	out[17].Request.TopK = 1
	out[18].Request.Members = append(out[18].Request.Members, candidate(out[18].Admission.Sources[1], "m2", Range{Position{0, 0}, Position{1, 0}}, 2))
	out[18].Condition.Limits.MaxMembers = 1
	out[19].Request.Selector = Selector{Kind: PathPrefix, Path: "src", FrozenPaths: []string{"src/a.go", "src/b.go"}}
	out[19].Condition.Limits.MaxPrefixExpansion = 1
	out[20].Request.Selector.Union[0].Ranges = []Range{{Position{0, 0}, Position{3, 0}}, {Position{0, 0}, Position{4, 0}}}
	out[20].Condition.Limits.MaxWitnesses = 1
	for i := range out {
		evaluateCase(&out[i])
	}
	exact := out[21].Expected.Counters.Work
	out[21].Condition.Limits.MaxWork = exact
	evaluateCase(&out[21])
	out[22].Condition.Limits.MaxWork = exact - 1
	evaluateCase(&out[22])
	out[23].Condition.Cancel = true
	out[23].Condition.Deadline = true
	evaluateCase(&out[23])
	return out
}
func baseCase(ord int, name string) DesignCase {
	ar := sourceadmissionv2.Admit([]sourceadmissionv2.SelectedSource{{Path: "src/a.go", Revision: "rev-1", Bytes: []byte("alpha\n")}, {Path: "src/b.go", Revision: "rev-1", Bytes: []byte("beta\n")}}, sourceadmissionv2.Limits{MaxSources: 4, MaxSourceBytes: 100, MaxTotalBytes: 200})
	a := ar.Binding
	m := candidate(a.Sources[0], "m1", Range{Position{1, 0}, Position{2, 0}}, 1)
	l := DefaultLimits()
	id := fmt.Sprintf("%02d", ord)
	q := Request{Schema: Schema, ID: id, Relation: Intersects, Selector: Selector{Kind: RangeUnion, Union: []PathRanges{{Path: "src/a.go", Ranges: []Range{{Position{0, 0}, Position{3, 0}}}}}}, AdmissionDigest: a.AdmissionDigest, TopK: 10, Members: []Candidate{m}, PolicyDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	return DesignCase{name, Condition{"lsp-trace.adr0007.location.condition.v2", id, true, false, false, limitsArtifact(l)}, q, a, Result{}}
}
func candidate(s sourceadmissionv2.SelectedSource, id string, r Range, score int64) Candidate {
	return Candidate{id, s.Path, s.Revision, s.FileDigest, s.ObjectDigest, []Range{r}, true, true, score}
}
func evaluateCase(c *DesignCase) {
	l := Limits{c.Condition.Limits.MaxRequestBytes, c.Condition.Limits.MaxMembers, c.Condition.Limits.MaxPaths, c.Condition.Limits.MaxSelectorRanges, c.Condition.Limits.MaxMemberRanges, c.Condition.Limits.MaxPrefixExpansion, c.Condition.Limits.MaxWitnesses, c.Condition.Limits.MaxSourceBytes, c.Condition.Limits.MaxOutputBytes, c.Condition.Limits.MaxWork}
	var a *Admission
	if c.Condition.AdmissionAvailable {
		a = c.Admission
	}
	c.Expected = Evaluate(c.Request, a, Options{Limits: l, Control: fixtureControl{c.Condition.Cancel, c.Condition.Deadline}, ExpectedPolicyDigest: c.Request.PolicyDigest})
}
