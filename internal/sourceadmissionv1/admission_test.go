package sourceadmissionv1

import "testing"

func TestAdmitExactAndDefensive(t *testing.T) {
	raw := []byte("x\n")
	r := Admit([]SelectedSource{{Path: "a.go", Revision: "r", Bytes: raw}}, Limits{2, 10, 20})
	if r.Outcome != Complete || r.Binding == nil {
		t.Fatal(r)
	}
	raw[0] = 'z'
	if string(r.Binding.Sources[0].Bytes) != "x\n" {
		t.Fatal("input alias")
	}
	c := Clone(r.Binding)
	c.Sources[0].Bytes[0] = 'q'
	if string(r.Binding.Sources[0].Bytes) != "x\n" {
		t.Fatal("clone alias")
	}
}
func TestAdmitRejectsUnsafeDuplicateAndLimit(t *testing.T) {
	l := Limits{2, 10, 20}
	if Admit([]SelectedSource{{Path: "../a", Revision: "r", Bytes: []byte("x")}}, l).Outcome != InvalidSource {
		t.Fatal("unsafe")
	}
	if Admit([]SelectedSource{{Path: "a", Revision: "r", Bytes: []byte("x")}, {Path: "a", Revision: "r", Bytes: []byte("y")}}, l).Outcome != DuplicateSource {
		t.Fatal("duplicate")
	}
	if Admit([]SelectedSource{{Path: "a", Revision: "r", Bytes: []byte("01234567890")}}, l).Outcome != InvalidSource {
		t.Fatal("limit")
	}
}
