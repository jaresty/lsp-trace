package referencegrouping

import (
	"lsp-trace/internal/programcadmission"
	"reflect"
	"testing"
)

func TestPrivateAllModeMetadataOpaqueBoundedCopy(t *testing.T) {
	in := evaluationAdmittedFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
	got, err := ReadAllModeMetadataPrivate(in)
	if err != nil || got.Input.ID != in.ID() || got.Input.ByteLength != in.ByteLength() || got.Contributions != 2 || got.Custody != 4 {
		t.Fatal("ASSERT_ALLMODE_METADATA_EXACT", got, err)
	}
	got.EligibleKinds[0] = "foreign"
	again, err := ReadAllModeMetadataPrivate(in)
	if err != nil || again.EligibleKinds[0] != "CALLS" {
		t.Fatal("ASSERT_ALLMODE_METADATA_COPY")
	}
	if _, err = ReadAllModeMetadataPrivate(AdmittedInput{}); err == nil {
		t.Fatal("ASSERT_ALLMODE_METADATA_ZERO_REJECTED")
	}
	if _, _, _, err = ProjectionCountsPrivate(EvaluationProjectionPrivate{}); err == nil {
		t.Fatal("ASSERT_ALLMODE_METADATA_ZERO_PROJECTION")
	}
	t.Log("ASSERT_ALLMODE_METADATA_EXACT: PASS")
	t.Log("ASSERT_ALLMODE_METADATA_COPY: PASS")
	t.Log("ASSERT_ALLMODE_METADATA_ZERO_REJECTED: PASS")
	// Same eligible kinds are retained even when a policy excludes every active
	// occurrence; modes must not be inferred from nonzero numerical edges.
	policy := stage5Policy(t, map[string]GroupingPolicyRule{"REFERENCES_SYMBOL": {Action: "EXCLUDE", RationaleCode: "CONTROLLED_EXCLUDED", Weight: Rational{0, 1}}})
	p, err := ProjectEvaluationPrivate(in, policy, stage5Limits())
	if err != nil {
		t.Fatal(err)
	}
	n, o, c, err := ProjectionCountsPrivate(p)
	if err != nil || n != 2 || o != 2 || c != 4 || len(p.PairContributions()) != 0 || !reflect.DeepEqual(again.EligibleKinds, []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}) {
		t.Fatal("ASSERT_ALLMODE_METADATA_EMPTY_NOT_KIND_ABSENCE")
	}
	t.Log("ASSERT_ALLMODE_METADATA_EMPTY_NOT_KIND_ABSENCE: PASS")
}
