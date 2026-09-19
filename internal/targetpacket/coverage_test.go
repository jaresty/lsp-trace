package targetpacket

import (
	"reflect"
	"testing"
)

type coverageCase struct {
	property, name string
	fn             func(*testing.T)
}

// TestCoverageManifest is a compile-time reference registry for the RED matrix.
func TestCoverageManifest(t *testing.T) {
	cases := []coverageCase{
		{"A Snapshot", "TestSnapshotDuplicateOrdinal", TestSnapshotDuplicateOrdinal}, {"A Snapshot", "TestSnapshotIdentityMismatch", TestSnapshotIdentityMismatch}, {"A Snapshot", "TestSnapshotOrdinalOutside", TestSnapshotOrdinalOutside}, {"A Snapshot", "TestSnapshotCensusGraphDigestMismatch", TestSnapshotCensusGraphDigestMismatch}, {"A Snapshot", "TestSnapshotCensusGraphLengthMismatch", TestSnapshotCensusGraphLengthMismatch}, {"A Snapshot", "TestSnapshotUnusedValidExtra", TestSnapshotUnusedValidExtra}, {"A Snapshot", "TestSnapshotTwoNominationsReuseOneSuccess", TestSnapshotTwoNominationsReuseOneSuccess},
		{"B Mapping", "TestMappingZeroDisplayBinding", TestMappingZeroDisplayBinding}, {"B Mapping", "TestMappingDuplicateIdenticalDisplayRow", TestMappingDuplicateIdenticalDisplayRow}, {"B Mapping", "TestMappingSameNodeDistinctLogicalURIs", TestMappingSameNodeDistinctLogicalURIs},
		{"C Lookup", "TestLookupNil", TestLookupNil}, {"C Lookup", "TestLookupErrorSafe", TestLookupErrorSafe}, {"C Lookup", "TestLookupIdentityMismatch", TestLookupIdentityMismatch}, {"C Lookup", "TestLookupByteLengthMismatch", TestLookupByteLengthMismatch}, {"C Lookup", "TestLookupDigestMismatch", TestLookupDigestMismatch},
		{"D Limits", "TestResolveLimitFields", TestResolveLimitFields}, {"D Limits", "TestPolicyLimits", TestPolicyLimits}, {"D Limits", "TestMaxResponseBytesTooSmall", TestMaxResponseBytesTooSmall},
		{"E Determinism", "TestDeterminismAndAliases", TestDeterminismAndAliases}, {"E Determinism", "TestUnresolvedDeterminismUsesFullLineage", TestUnresolvedDeterminismUsesFullLineage}, {"F States", "TestStatesAndLineageClone", TestStatesAndLineageClone}, {"G Immutability", "TestCensusDeepImmutability", TestCensusDeepImmutability}, {"H Validator", "TestValidatorTamperMatrix", TestValidatorTamperMatrix}, {"H Validator", "TestValidatorClosedPacketConsistency", TestValidatorClosedPacketConsistency}, {"H Validator", "TestValidatorSyntaxAndReplay", TestValidatorSyntaxAndReplay}, {"I Representative invariants", "TestRepresentativeBuildInvariants", TestRepresentativeBuildInvariants}, {"I Success schema", "TestSuccessSchemaConcrete", TestSuccessSchemaConcrete}, {"J Error hygiene", "TestFailureErrorHygiene", TestFailureErrorHygiene},
	}
	seenName := map[string]bool{}
	seenFn := map[uintptr]bool{}
	for _, c := range cases {
		if c.property == "" || c.name == "" || c.fn == nil {
			t.Fatalf("invalid manifest entry %#v", c)
		}
		fn := reflect.ValueOf(c.fn).Pointer()
		if seenName[c.name] || seenFn[fn] {
			t.Fatalf("duplicate manifest entry %s", c.name)
		}
		seenName[c.name], seenFn[fn] = true, true
	}
	if len(cases) != 28 {
		t.Fatalf("coverage case count %d", len(cases))
	}
}
