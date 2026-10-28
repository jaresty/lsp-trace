package retainedpartialprogramc

import "testing"

func TestCloneOutcomePrivateBindingBytesAreIndependent(t *testing.T) {
	in := Outcome{Source: SourceBinding{raw: []byte("input"), native: []byte("native"), manifest: []byte("manifest")}}
	out := CloneOutcome(in)
	out.Source.raw[0] = '!'
	out.Source.native[0] = '!'
	out.Source.manifest[0] = '!'
	if string(in.Source.raw) != "input" || string(in.Source.native) != "native" || string(in.Source.manifest) != "manifest" {
		t.Fatal("cloned source binding shares backing bytes")
	}
	in.Source.raw[1] = '?'
	in.Source.native[1] = '?'
	in.Source.manifest[1] = '?'
	if out.Source.raw[1] == '?' || out.Source.native[1] == '?' || out.Source.manifest[1] == '?' {
		t.Fatal("original mutation reaches cloned source binding")
	}
	zero := CloneOutcome(Outcome{})
	if zero.Source.raw != nil || zero.Source.native != nil || zero.Source.manifest != nil {
		t.Fatal("nil binding bytes changed shape")
	}
}
