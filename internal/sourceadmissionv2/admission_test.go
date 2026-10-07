package sourceadmissionv2

import "testing"

func TestAdmissionV2NFCObjectDigestAndCopies(t *testing.T) {
	r := Admit([]SelectedSource{{Path: "src/é.go", Revision: "r", Bytes: []byte("x")}}, Limits{2, 10, 20})
	if r.Outcome != Complete {
		t.Fatal(r)
	}
	b := r.Binding
	old := b.AdmissionDigest
	b.Sources[0].ObjectDigest = Digest([]byte("other"))
	r2 := Admit(b.Sources, Limits{2, 20, 40})
	if r2.Outcome != InvalidSource || old == "" {
		t.Fatal("object binding")
	}
	if Admit([]SelectedSource{{Path: "src/e\u0301.go", Revision: "r", Bytes: []byte("x")}}, Limits{2, 10, 20}).Outcome != InvalidSource {
		t.Fatal("NFC")
	}
}
func TestAdmissionV2Duplicate(t *testing.T) {
	s := SelectedSource{Path: "a.go", Revision: "r", Bytes: []byte("x")}
	if Admit([]SelectedSource{s, s}, Limits{2, 10, 20}).Outcome != DuplicateSource {
		t.Fatal("duplicate")
	}
}
