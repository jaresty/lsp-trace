package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCLICensusCatalogUsesCoherentBoundedCaptureDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"continuation":{"publication_root":"/private","max_object_bytes":67108864}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	contract, err := censusCatalogContract(path)
	if err != nil {
		t.Fatal(err)
	}
	limits := contract.CaptureLimits()
	if limits.MaxArtifactBytes != 64<<20 || limits.MaxParentBytes != 64<<20 || limits.MaxReceipts != 1000 || limits.MaxBindings != 1000 || limits.MaxSourceBytes != 1<<20 || limits.MaxTotalSourceBytes != 8<<20 || limits.MaxWork != 10000 {
		t.Fatalf("ASSERT_CLI_CAPTURE_DEFAULT_PARITY: %+v", limits)
	}
}

func TestCensusBatchTargetsGrammarDefaultsAndResumeExclusion(t *testing.T) {
	base := []string{"census", "--workspace", "/work", "--publication-root", "/private", "--server", "gopls"}
	for _, tc := range []struct {
		name        string
		args        []string
		want        int
		wantCatalog bool
		ok          bool
	}{
		{name: "historical_omitted", args: base, want: 63, ok: true},
		{name: "historical_explicit", args: append(append([]string{}, base...), "--batch-targets", "16"), want: 16, ok: true},
		{name: "catalog_omitted", args: append(append([]string{}, base...), "--catalog", "--catalog-config", "/host.json"), want: 16, wantCatalog: true, ok: true},
		{name: "lower_bound", args: append(append([]string{}, base...), "--batch-targets", "0")},
		{name: "upper_bound", args: append(append([]string{}, base...), "--batch-targets", "64")},
		{name: "resume_forbidden", args: []string{"census", "--resume", "g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json", "--publication-root", "/private", "--catalog-config", "/host.json", "--batch-targets", "16"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCensusCLIOptions(tc.args)
			if (err == nil) != tc.ok {
				t.Fatalf("ASSERT_BATCH_TARGETS_CLI_VALIDATION: err=%v options=%+v", err, got)
			}
			if tc.ok {
				field := reflect.ValueOf(got).FieldByName("MaxBatchTargets")
				if !field.IsValid() || int(field.Int()) != tc.want || got.Catalog != tc.wantCatalog {
					t.Fatalf("ASSERT_BATCH_TARGETS_CLI_DEFAULT_TRANSFER: field_valid=%t catalog=%t want=%d", field.IsValid(), got.Catalog, tc.want)
				}
			}
		})
	}
}

func TestCensusStopAfterDescribeRequestsGrammar(t *testing.T) {
	base := []string{"census", "--workspace", "/work", "--publication-root", "/private", "--server", "gopls", "--catalog", "--catalog-config", "/host.json"}
	selector := "g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json"
	for _, tc := range []struct {
		name string
		args []string
		ok   bool
	}{
		{name: "fresh", args: append(append([]string{}, base...), "--stop-after", "describe-requests"), ok: true},
		{name: "unknown", args: append(append([]string{}, base...), "--stop-after", "other")},
		{name: "without_catalog", args: []string{"census", "--workspace", "/work", "--publication-root", "/private", "--server", "gopls", "--stop-after", "describe-requests"}},
		{name: "resume_same_stop", args: []string{"census", "--resume", selector, "--publication-root", "/private", "--catalog-config", "/host.json", "--stop-after", "describe-requests"}, ok: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCensusCLIOptions(tc.args)
			if (err == nil) != tc.ok {
				t.Fatalf("ASSERT_STOP_AFTER_DESCRIBE_REQUESTS_CLI_GRAMMAR: ok=%t err=%v options=%+v", tc.ok, err, got)
			}
			if tc.ok && got.StopAfter != "describe-requests" {
				t.Fatalf("ASSERT_STOP_AFTER_DESCRIBE_REQUESTS_CLI_CANONICAL: %q", got.StopAfter)
			}
		})
	}
}

func TestCensusCatalogGrammar(t *testing.T) {
	selector := "g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json"
	freshBase := []string{"census", "--workspace", "/work", "--publication-root", "/private", "--server", "gopls", "--catalog-config", "/host.json"}
	resumeBase := []string{"census", "--resume", selector, "--publication-root", "/private", "--catalog-config", "/host.json"}
	for _, tc := range []struct {
		name string
		args []string
		ok   bool
	}{
		{name: "fresh_catalog", args: append(append([]string{}, freshBase...), "--catalog"), ok: true},
		{name: "fresh_catalog_profile_config", args: append(append([]string{}, freshBase...), "--catalog", "--profile", "go", "--config", "/profiles.toml"), ok: true},
		{name: "fresh_catalog_missing_host_config", args: []string{"census", "--workspace", "/work", "--publication-root", "/private", "--server", "gopls", "--catalog"}},
		{name: "resume", args: append([]string{}, resumeBase...), ok: true},
		{name: "resume_implies_catalog", args: append(append([]string{}, resumeBase...), "--catalog"), ok: true},
		{name: "resume_config", args: append(append([]string{}, resumeBase...), "--config", "/profiles.toml")},
		{name: "resume_source", args: append(append([]string{}, resumeBase...), "--source", ".")},
		{name: "resume_include", args: append(append([]string{}, resumeBase...), "--include", "*.go")},
		{name: "resume_server", args: append(append([]string{}, resumeBase...), "--server", "gopls")},
		{name: "resume_profile", args: append(append([]string{}, resumeBase...), "--profile", "go")},
		{name: "resume_depth", args: append(append([]string{}, resumeBase...), "--down-depth", "2")},
		{name: "resume_missing_host_config", args: []string{"census", "--resume", selector, "--publication-root", "/private"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCensusCLIOptions(tc.args)
			if (err == nil) != tc.ok {
				t.Fatalf("ASSERT_CENSUS_CATALOG_GRAMMAR ok=%t err=%v options=%+v", tc.ok, err, got)
			}
		})
	}
}
