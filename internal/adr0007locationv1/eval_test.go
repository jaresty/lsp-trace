package adr0007locationv1

import (
	"lsp-trace/internal/sourceadmissionv1"
	"testing"
)

type ctl struct{ c, d bool }

func (x ctl) Cancelled() bool        { return x.c }
func (x ctl) DeadlineExceeded() bool { return x.d }
func fixture() (Request, *Admission, Options) {
	a := sourceadmissionv1.Admit([]sourceadmissionv1.SelectedSource{{Path: "a.go", Revision: "r", Bytes: []byte("x")}}, sourceadmissionv1.Limits{MaxSources: 2, MaxSourceBytes: 10, MaxTotalBytes: 10}).Binding
	s := a.Sources[0]
	q := Request{ID: "q", Relation: Intersects, AdmissionDigest: a.AdmissionDigest, TopK: 1, PolicyDigest: "p", Selector: Selector{Kind: RangeUnion, Union: []PathRanges{{Path: "a.go", Ranges: []Range{{Position{1, 0}, Position{2, 0}}}}}}, Members: []Member{{ID: "m", Path: "a.go", Revision: "r", FileDigest: s.FileDigest, ObjectDigest: s.ObjectDigest, Ranges: []Range{{Position{1, 1}, Position{3, 0}}}, Available: true, PolicyAllowed: true}}}
	return q, a, Options{Limits: Limits{10000, 10, 10, 10, 10, 10, 100, 10000, 20}, ExpectedPolicyDigest: "p"}
}
func TestEvaluateComplete(t *testing.T) {
	q, a, o := fixture()
	r := Evaluate(q, a, o)
	if r.Outcome != Complete || len(r.Members) != 1 || r.Members[0].Outcome != Eligible || r.Accounting.Input != 1 || r.Accounting.Ranked != 1 {
		t.Fatalf("%+v", r)
	}
}
func TestExactWorkBoundaryAndPlusOne(t *testing.T) {
	q, a, o := fixture()
	r := Evaluate(q, a, o)
	o.Limits.MaxWork = r.Accounting.Work
	if Evaluate(q, a, o).Outcome != Complete {
		t.Fatal("boundary")
	}
	o.Limits.MaxWork--
	if Evaluate(q, a, o).Outcome != ResourceLimit {
		t.Fatal("plus one")
	}
}
func TestNoPartialCancellation(t *testing.T) {
	q, a, o := fixture()
	o.Control = ctl{c: true}
	r := Evaluate(q, a, o)
	if r.Outcome != Cancelled || len(r.Members) != 0 {
		t.Fatal(r)
	}
}
func TestAdjacencyIsNotIntersection(t *testing.T) {
	q, a, o := fixture()
	q.Selector.Union[0].Ranges[0] = Range{Position{0, 0}, Position{1, 1}}
	q.Members[0].Ranges = []Range{{Position{1, 1}, Position{2, 0}}}
	if Evaluate(q, a, o).Members[0].Outcome != Ineligible {
		t.Fatal("adjacent")
	}
}
