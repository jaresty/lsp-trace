package programc

import (
	"reflect"
	"testing"

	"lsp-trace/internal/programctestfixture"
)

func TestCandidateGroupSeed19FixtureIdentity(t *testing.T) {
	o, failure := Compute(programctestfixture.ValidV5(t), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	want := []Community{
		{Members: []string{"020b42e6f4356621d0dbfd50da9ee760ede90980ce036960fd05dafa7113c80b"}},
		{Members: []string{"3cdff2e81db031666b6d6d4d13711ddc2749722ac804ef79b103463ebfdda93c", "4a4c524d563487befdff3720d59fb28740155453ebe6625b782f7612ef9e445f"}},
	}
	if o.Seed != 19 || o.ProfileID != ProfileID || o.ProfileDigest != ProfileDigest || o.LogicalDigest != "sha256:6d4c2e16bfb292d29e7649d89118354e3a3e95c7ec00c0ba35ac9b8d3cf6104a" || !reflect.DeepEqual(o.Communities, want) {
		t.Fatalf("candidate-group fixture identity drift: seed=%d profile=%s digest=%s communities=%#v", o.Seed, o.ProfileID, o.LogicalDigest, o.Communities)
	}
}
