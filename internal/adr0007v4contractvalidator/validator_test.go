package adr0007v4contractvalidator

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConformanceFixtures(t *testing.T) {
	pos, _ := filepath.Glob(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "*.json"))
	if len(pos) != 8 {
		t.Fatalf("positive fixture count = %d, want 8", len(pos))
	}
	for _, p := range pos {
		if err := ValidateBundleFile(p); err != nil {
			t.Fatalf("positive %s rejected: %v", p, err)
		}
	}
	neg, _ := filepath.Glob(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "negative", "*.json"))
	if len(neg) != 48 {
		t.Fatalf("negative fixture count = %d, want 48", len(neg))
	}
	for _, p := range neg {
		bun, err := loadBundleForTest(p)
		if err != nil {
			t.Fatal(err)
		}
		err = ValidateBundle(bun)
		if err == nil {
			t.Fatalf("negative %s accepted", p)
		}
		var ve *VError
		if !errors.As(err, &ve) {
			t.Fatalf("negative %s returned non-VError %v", p, err)
		}
		if ve.Code != bun.WantErrorCode || ve.Path != bun.WantErrorPath {
			t.Fatalf("negative %s got %s %s want %s %s", p, ve.Code, ve.Path, bun.WantErrorCode, bun.WantErrorPath)
		}
	}
}

func TestSchemaWalkerDetectsMutation(t *testing.T) {
	bun, err := loadBundleForTest(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "complete.json"))
	if err != nil {
		t.Fatal(err)
	}
	bun.SchemaBytes = []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"mutated","required":[],"properties":{}}`)
	err = ValidateBundle(bun)
	if err == nil {
		t.Fatal("mutated schema accepted")
	}
	var ve *VError
	if !errors.As(err, &ve) || ve.Path != "/schema_bytes/$id" {
		t.Fatalf("schema mutation got %v", err)
	}
}

func loadBundleForTest(p string) (Bundle, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return Bundle{}, err
	}
	var bun Bundle
	if err := json.Unmarshal(b, &bun); err != nil {
		return Bundle{}, err
	}
	return bun, nil
}
