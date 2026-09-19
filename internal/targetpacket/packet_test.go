package targetpacket

import (
	"errors"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/sourceprojection"
	"reflect"
	"testing"
)

func structPolicy() sourceprojection.Policy {
	return sourceprojection.Policy{PolicyID: "p", BodyRequested: true, MaxBytes: 1024, MaxRanges: 10, MaxObjects: 10, MaxWork: 100, EnforceLimits: true}
}
func censusWith(n censusprogramc.Representative) censusprogramc.Result {
	return censusprogramc.Result{CensusID: "census", Admission: programcadmission.Result{Artifact: programcadmission.Artifact{Constituents: []programcadmission.ConstituentReference{{Identity: "constituent", GraphSHA256: "sha256:g", GraphByteLength: 1}}}}, Representatives: censusprogramc.RepresentativeSelection{Nominations: []censusprogramc.Representative{n}}}
}
func selected() censusprogramc.Representative {
	return censusprogramc.Representative{CensusID: "census", ConstituentIdentity: "constituent", ConstituentOrdinal: 0, SelectionState: "SELECTED", SelectedNode: "n"}
}
func TestASSERT_P1_NON_SELECTED_REJECTED(t *testing.T) {
	n := selected()
	n.SelectionState = "CANDIDATE"
	_, err := Build(Request{Census: censusWith(n), Policy: structPolicy()})
	if err == nil {
		t.Fatal("ASSERT_P1_NON_SELECTED_REJECTED")
	}
}
func TestASSERT_P1_FOREIGN_AND_ORDINAL_REJECTED(t *testing.T) {
	for _, n := range []censusprogramc.Representative{func() censusprogramc.Representative { x := selected(); x.CensusID = "foreign"; return x }(), func() censusprogramc.Representative { x := selected(); x.ConstituentOrdinal = 999; return x }()} {
		_, err := Build(Request{Census: censusWith(n), Policy: structPolicy()})
		if err == nil {
			t.Fatal("ASSERT_P1_FOREIGN_AND_ORDINAL_REJECTED")
		}
	}
}
func TestASSERT_P7_EMPTY_UNRESOLVED_EXPLICIT(t *testing.T) {
	empty, err := Build(Request{Census: censusprogramc.Result{}})
	if err != nil || empty.State != StateEmpty {
		t.Fatal("ASSERT_P7_EMPTY")
	}
	u := selected()
	r, err := Build(Request{Census: censusprogramc.Result{Representatives: censusprogramc.RepresentativeSelection{Unresolved: []censusprogramc.Representative{u}}}})
	if err != nil || r.State != StateUnresolved || r.UnresolvedCount != 1 {
		t.Fatal("ASSERT_P7_UNRESOLVED")
	}
}
func TestASSERT_P9_CENSUS_UNCHANGED_ON_FAILURE(t *testing.T) {
	n := selected()
	c := censusWith(n)
	before := c
	before.Representatives.Nominations = append([]censusprogramc.Representative(nil), c.Representatives.Nominations...)
	before.Admission.Artifact.Constituents = append([]programcadmission.ConstituentReference(nil), c.Admission.Artifact.Constituents...)
	_, _ = Build(Request{Census: c, Policy: structPolicy()})
	if !reflect.DeepEqual(c, before) {
		t.Fatal("ASSERT_P9_CENSUS_UNCHANGED")
	}
}
func TestASSERT_P8_CANONICAL_VALIDATION(t *testing.T) {
	out, err := Build(validRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	p := out.Packets[0]
	raw, err := EncodeCanonical(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Validate(raw); err != nil {
		t.Fatal(err)
	}
	if _, err = Validate(append(raw, raw...)); err == nil {
		t.Fatal("ASSERT_P8_TRAILING_JSON_REJECTED")
	}
	if _, err = Validate([]byte(`{"packet_id":"x","packet_id":"y"}`)); err == nil {
		t.Fatal("ASSERT_P8_DUPLICATE_KEY_REJECTED")
	}
}
func TestASSERT_P8_FAILURE_UNWRAPS_SAFE_CODE(t *testing.T) {
	_, err := Build(Request{Census: censusWith(selected()), Policy: structPolicy()})
	var f *Failure
	if !errors.As(err, &f) || f.Stage != StageInput {
		t.Fatal("ASSERT_P8_FAILURE_STAGE")
	}
}
