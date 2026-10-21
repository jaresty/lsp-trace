package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestCensusPruneDirectoriesCanonicalizeToMinimalAntichain(t *testing.T) {
	o, err := parseCensusCLIOptions([]string{
		"census", "--workspace", "/work", "--source", ".", "--server", "gopls", "--publication-root", "/private",
		"--prune-dir", "vendor/generated", "--prune-dir", "vendor", "--prune-dir", "tmp", "--prune-dir", "vendor",
	})
	if err != nil {
		t.Fatalf("ASSERT_CENSUS_PRUNE_CANONICAL_PARSE: %v", err)
	}
	if want := []string{"tmp", "vendor"}; !reflect.DeepEqual(o.PrunedDirectories, want) {
		t.Fatalf("ASSERT_CENSUS_PRUNE_MINIMAL_ANTICHAIN: got=%v want=%v", o.PrunedDirectories, want)
	}
}

func TestCensusPruneDirectoryRejectsNonCanonicalPaths(t *testing.T) {
	bad := []string{"", "/abs", `a\\b`, ".", "..", "a/./b", "a/../b", "a//b", "C:/drive", "../escape", "a\x00b"}
	for _, value := range bad {
		t.Run(strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
			_, err := parseCensusCLIOptions([]string{
				"census", "--workspace", "/work", "--source", ".", "--server", "gopls", "--publication-root", "/private", "--prune-dir", value,
			})
			if err == nil {
				t.Fatalf("ASSERT_CENSUS_PRUNE_REJECTED: %q", value)
			}
		})
	}
}

func TestCensusEnumerationPreflightSyntaxAndIncompatibilities(t *testing.T) {
	valid := []string{"census", "--workspace", "/work", "--source", ".", "--machine", "--enumeration-preflight"}
	if o, err := parseCensusCLIOptions(valid); err != nil || !o.EnumerationPreflight {
		t.Fatalf("ASSERT_CENSUS_ENUMERATION_PREFLIGHT_VALID: options=%+v err=%v", o, err)
	}
	for _, extra := range [][]string{
		{"--catalog", "--catalog-config", "/host.json"}, {"--stop-after", "describe-requests"}, {"--server", "gopls"}, {"--profile", "go"}, {"--config", "/profiles.toml"}, {"--publication-root", "/private"}, {"--private-enumeration-projection"}, {"--private-acquisition-diagnostic-path", "/tmp/d"},
	} {
		args := append(append([]string{}, valid...), extra...)
		if _, err := parseCensusCLIOptions(args); err == nil {
			t.Fatalf("ASSERT_CENSUS_ENUMERATION_PREFLIGHT_REJECTS_CONTROL: %v", extra)
		}
	}
	if _, err := parseCensusCLIOptions([]string{"census", "--workspace", "/work", "--source", ".", "--enumeration-preflight"}); err == nil {
		t.Fatal("ASSERT_CENSUS_ENUMERATION_PREFLIGHT_REQUIRES_MACHINE")
	}
}
