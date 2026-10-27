package programcrepresentative

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMechanicsV2AdapterContractGolden(t *testing.T) {
	contract, err := newAdapterContractV2("fixture-version")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	const wantIdentity = "sha256:a2e84e57305d03c7b320595c35039dc2f7b988ba8291e3cd661fafaa091a5a38"
	const wantSHA256 = "sha256:89e385d50b2b0b8f0ed3f15bf9e8e5d6cf15188baff6dc2e3e95496ff94ad85c"
	const wantLength = 1486
	if contract.SchemaVersion != AdapterContractSchemaVersionV2 || contract.Authority != 0 || contract.Accepted || contract.Identity != wantIdentity || rawDigest(canonical) != wantSHA256 || len(canonical) != wantLength {
		t.Fatalf("ASSERT_MECHANICS_V2_ADAPTER_CONTRACT_GOLDEN identity=%s sha256=%s length=%d canonical=%s", contract.Identity, rawDigest(canonical), len(canonical), canonical)
	}
}

func TestMechanicsV2SourceLimitsInclusiveAndPlusOne(t *testing.T) {
	if err := validateSourceLimitsV2(MaxSeedGroupsV2, MaxMembershipsV2, MaxMembershipsPerGroupV2, MaxGroupOccurrencesV2, MaxSumGroupOccurrencesV2); err != nil {
		t.Fatalf("ASSERT_MECHANICS_V2_ALL_SOURCE_LIMITS_EQUALITY err=%v", err)
	}
	tests := []struct {
		name                                                             string
		seedGroups, memberships, perGroup, perOccurrence, sumOccurrences int
	}{
		{"seed-groups", MaxSeedGroupsV2 + 1, 0, 0, 0, 0},
		{"memberships", 0, MaxMembershipsV2 + 1, 0, 0, 0},
		{"memberships-per-group", 0, 0, MaxMembershipsPerGroupV2 + 1, 0, 0},
		{"group-occurrences", 0, 0, 0, MaxGroupOccurrencesV2 + 1, 0},
		{"sum-group-occurrences", 0, 0, 0, 0, MaxSumGroupOccurrencesV2 + 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSourceLimitsV2(tc.seedGroups, tc.memberships, tc.perGroup, tc.perOccurrence, tc.sumOccurrences); err == nil {
				t.Fatal("ASSERT_MECHANICS_V2_SOURCE_LIMIT_PLUS_ONE")
			}
		})
	}
}

func TestMechanicsV2StringLimitInclusiveAndPlusOne(t *testing.T) {
	if !validateStringCapV2(strings.Repeat("x", MaxStringBytesV2)) {
		t.Fatal("ASSERT_MECHANICS_V2_STRING_LIMIT_EQUALITY")
	}
	if validateStringCapV2(strings.Repeat("x", MaxStringBytesV2+1)) {
		t.Fatal("ASSERT_MECHANICS_V2_STRING_LIMIT_PLUS_ONE")
	}
}
