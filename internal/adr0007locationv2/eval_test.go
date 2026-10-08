package adr0007locationv2

import (
	"lsp-trace/internal/sourceadmissionv2"
	"testing"
)

type ctl struct{ c, d bool }

func (x ctl) Cancelled() bool        { return x.c }
func (x ctl) DeadlineExceeded() bool { return x.d }
func fixture() (Request, *Admission, Options) {
	a := sourceadmissionv2.Admit([]sourceadmissionv2.SelectedSource{{Path: "src/a.go", Revision: "r", Bytes: []byte("x")}}, sourceadmissionv2.Limits{MaxSources: 2, MaxSourceBytes: 10, MaxTotalBytes: 20}).Binding
	s := a.Sources[0]
	q := Request{Schema: Schema, ID: "q", Relation: Intersects, Selector: Selector{Kind: RangeUnion, Union: []PathRanges{{Path: s.Path, Ranges: []Range{{Position{1, 0}, Position{2, 0}}}}}}, AdmissionDigest: a.AdmissionDigest, TopK: 1, PolicyDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Members: []Candidate{{ID: "m", Path: s.Path, Revision: s.Revision, FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest, Ranges: []Range{{Position{1, 1}, Position{3, 0}}}, Available: true, PolicyAllowed: true, Score: 1}}}
	o := Options{Limits: Limits{100000, 10, 10, 10, 10, 10, 10, 1000, 100000, 100000}, ExpectedPolicyDigest: q.PolicyDigest}
	return q, a, o
}
func TestEvaluateV2ExactAccounting(t *testing.T) {
	q, a, o := fixture()
	r := Evaluate(q, a, o)
	if r.Outcome != Complete || len(r.Members) != 1 || r.Counters.Input != 1 || r.Counters.Eligible != 1 || r.Counters.Witnesses != 1 || r.Counters.Eligible+r.Counters.Ineligible+r.Counters.UnavailableLocation+r.Counters.InvalidLocation+r.Counters.DuplicateMember+r.Counters.FilteredByPolicy != r.Counters.Input {
		t.Fatalf("%+v", r)
	}
}
func TestPrefixMustEqualAdmittedExpansion(t *testing.T) {
	q, a, o := fixture()
	q.Selector = Selector{Kind: PathPrefix, Path: "src", FrozenPaths: []string{"src/a.go", "src/missing.go"}}
	if Evaluate(q, a, o).Outcome != InvalidSelector {
		t.Fatal("prefix")
	}
}
func TestLoopCancellationNoPartial(t *testing.T) {
	q, a, o := fixture()
	o.Control = ctl{c: true}
	r := Evaluate(q, a, o)
	if r.Outcome != Cancelled || len(r.Members) != 0 {
		t.Fatal(r)
	}
}
func TestWorkRefusesBeforeOutput(t *testing.T) {
	q, a, o := fixture()
	o.Limits.MaxWork = 1
	if Evaluate(q, a, o).Outcome != ResourceLimit {
		t.Fatal("work")
	}
}
