package schema

import (
	"bytes"
	"testing"
)

func TestCensusFeatureCatalogV2RegisteredAdditively(t *testing.T) {
	versions := RegisteredFamilies()[FamilyCensusFeatureCatalog]
	if len(versions) != 1 || versions[0] != "v2" {
		t.Fatalf("ASSERT_CENSUS_FEATURE_CATALOG_V2_REGISTERED: %v", versions)
	}
	raw, err := BytesFor(FamilyCensusFeatureCatalog, "v2")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"$id":"https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-feature-catalog-result.v2.schema.json"`)) {
		t.Fatal("ASSERT_CENSUS_FEATURE_CATALOG_V2_ID")
	}
}
