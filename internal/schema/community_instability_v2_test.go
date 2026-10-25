package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCommunityInstabilityV1SchemaSHA256Pinned(t *testing.T) {
	raw, err := os.ReadFile("schemas/lsp-trace.community-instability.v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got, want := hex.EncodeToString(sum[:]), "8c581786a7b3371e42623ab4b8b6bd6594f1d0458b5b7092fcffdc23d5dbcef6"; got != want {
		t.Fatalf("ASSERT_COMMUNITY_INSTABILITY_V1_SCHEMA_SHA256 got=%s want=%s", got, want)
	}
}

func TestCommunityInstabilityV3SchemaSHA256Pinned(t *testing.T) {
	raw, err := os.ReadFile("schemas/lsp-trace.community-instability.v3.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got, want := hex.EncodeToString(sum[:]), "ab28b3dfd77b8e19838e8563329339ce0cd287a781ad7e31990a545476330db0"; got != want {
		t.Fatalf("ASSERT_COMMUNITY_INSTABILITY_V3_SCHEMA_SHA256 got=%s want=%s", got, want)
	}
}

func TestCommunityInstabilityRegisteredExactlyV1V2V3(t *testing.T) {
	versions := RegisteredFamilies()[FamilyCommunityInstability]
	if !reflect.DeepEqual(versions, []string{"v1", "v2", "v3"}) {
		t.Fatalf("ASSERT_COMMUNITY_INSTABILITY_EXACT_REGISTRATION: %v", versions)
	}
}

func TestCommunityInstabilityV2SchemaStrictCopyDelta(t *testing.T) {
	v1Raw, err := BytesFor(FamilyCommunityInstability, "v1")
	if err != nil {
		t.Fatal(err)
	}
	v2Raw, err := BytesFor(FamilyCommunityInstability, "v2")
	if err != nil {
		t.Fatal(err)
	}
	var v1, v2 map[string]any
	if err := json.Unmarshal(v1Raw, &v1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(v2Raw, &v2); err != nil {
		t.Fatal(err)
	}
	v1["$id"] = v2["$id"]
	v1Properties := v1["properties"].(map[string]any)
	v2Properties := v2["properties"].(map[string]any)
	v1Properties["schema_version"] = v2Properties["schema_version"]
	v1Properties["claim_ceiling"] = v2Properties["claim_ceiling"]
	if !reflect.DeepEqual(v1, v2) {
		t.Fatal("ASSERT_COMMUNITY_INSTABILITY_V2_ONLY_THREE_FIELDS_DIFFER")
	}
}
