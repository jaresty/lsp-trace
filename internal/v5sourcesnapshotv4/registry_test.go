package v5sourcesnapshotv4

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"

	"lsp-trace/internal/schema"
)

func TestV4RegisteredAdditivelyAndClosed(t *testing.T) {
	families := schema.RegisteredFamilies()
	versions := families[schema.FamilyGraphV5SourceSnapshot]
	want := []string{"v1", "v2", "v3", "v4", "v5", "v6"}
	if len(versions) != len(want) {
		t.Fatalf("ASSERT_PUBLICATION_V4_REGISTERED_ADDITIVELY: %v", versions)
	}
	for i := range want {
		if versions[i] != want[i] {
			t.Fatalf("ASSERT_PUBLICATION_V4_REGISTERED_ADDITIVELY: %v", versions)
		}
	}
	historical := map[string]string{
		"v1": "1597702004047a81fa44b885598a768c32ef6d10fd8739ec6ad862c781bdd14a",
		"v2": "05c8f82839f2e9d426ab0a1a86fe6bdb2eb2c12cc9d9def7b8a4d59a217d43a0",
		"v3": "4754f3b763f7c8acf977d335338ce6a52370ed819fde02aa3ebb8024826311a6",
		"v4": "9c2f2fb3d1fbc94ca074f6459b6f8d270fb4b911b950e14a79207579c1f2cb2b",
		"v5": "a1a70c1b8020325de85aa3a4bf096b0485a6d48f22e0df35d48a6a3a2d90f9ed",
	}
	for version, digest := range historical {
		raw, err := schema.BytesFor(schema.FamilyGraphV5SourceSnapshot, version)
		got := fmt.Sprintf("%x", sha256.Sum256(raw))
		if err != nil || got != digest {
			t.Fatalf("ASSERT_V1_V2_V3_V4_V5_GOLDENS_UNCHANGED: %s got=%s want=%s err=%v", version, got, digest, err)
		}
	}
	if raw, err := schema.BytesFor(schema.FamilyGraphV5SourceSnapshot, "v6"); err != nil || len(raw) == 0 {
		t.Fatalf("ASSERT_PUBLICATION_V6_REGISTERED_ADDITIVELY: %v", err)
	}
	v4, _ := schema.BytesFor(schema.FamilyGraphV5SourceSnapshot, "v4")
	if !bytes.Contains(v4, []byte(`"additionalProperties":false`)) {
		t.Fatal("ASSERT_V4_CLOSED_SCHEMA")
	}
}
