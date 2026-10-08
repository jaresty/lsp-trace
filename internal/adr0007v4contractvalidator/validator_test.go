package adr0007v4contractvalidator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConformanceFixtures(t *testing.T) {
	pos, err := filepath.Glob(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 7 {
		t.Fatalf("positive fixture count = %d, want 7", len(pos))
	}
	for _, p := range pos {
		if err := ValidateFile(p); err != nil {
			t.Fatalf("positive %s rejected: %v", p, err)
		}
	}
	neg, err := filepath.Glob(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "negative", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(neg) < 8 {
		t.Fatalf("negative fixture count = %d, want at least 8", len(neg))
	}
	for _, p := range neg {
		if err := ValidateFile(p); err == nil {
			b, _ := os.ReadFile(p)
			t.Fatalf("negative %s accepted: %s", p, string(b))
		}
	}
}
