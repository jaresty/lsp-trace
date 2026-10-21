package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuspreflight"
	"lsp-trace/internal/source"
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

func TestCensusEnumerationPreflightHasZeroRuntimeDependencies(t *testing.T) {
	args := []string{"--workspace", "/work", "--source", ".", "--machine", "--enumeration-preflight", "--prune-dir", "vendor"}
	for _, tc := range []struct {
		name  string
		files []censusacquisition.SourceFile
		err   error
		code  int
	}{
		{"success", []censusacquisition.SourceFile{{Path: "a.go"}, {Path: "tmp/x.go"}}, nil, 0},
		{"limit", nil, &source.LimitError{Category: "MAX_WORK", Observed: 200001, Limit: 200000}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runCensusWithDependencies(args, &stdout, &stderr, censusRunnerDependencies{enumeratePreflight: func(censusCLIOptions) ([]censusacquisition.SourceFile, error) { return tc.files, tc.err }})
			if code != tc.code {
				t.Fatalf("ASSERT_PREFLIGHT_CODE: %d stdout=%s stderr=%s", code, stdout.Bytes(), stderr.Bytes())
			}
			if tc.err == nil {
				var result censuspreflight.Result
				if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &result); err != nil || result.Authority != 0 || result.Completeness != "UNKNOWN" || result.Terminal != "COMPLETE" || result.AbsenceInference || result.Counts.Selected != 2 || result.Limits.MaxWork != 200000 {
					t.Fatalf("ASSERT_PREFLIGHT_RESULT: %+v err=%v", result, err)
				}
			} else {
				var diagnostic censuspreflight.Diagnostic
				if err := json.Unmarshal(bytes.TrimSpace(stderr.Bytes()), &diagnostic); err != nil || diagnostic.Category != "MAX_WORK" || diagnostic.Observed != 200001 || diagnostic.Limit != 200000 {
					t.Fatalf("ASSERT_PREFLIGHT_LIMIT: %+v err=%v", diagnostic, err)
				}
			}
		})
	}
}

func TestCensusPublicUsageMatchesPruneAndPreflightParserGrammar(t *testing.T) {
	const grammar = "[--source PATH...] [--include PATTERN...] [--exclude PATTERN...] [--prune-dir DIR...]"
	const preflight = "(--workspace PATH [--source PATH...] [--include PATTERN...] [--exclude PATTERN...] [--prune-dir DIR...] --enumeration-preflight --machine)"
	for name, usage := range map[string]string{"command": censusUsage, "top-level": usageText} {
		if !strings.Contains(usage, grammar) || !strings.Contains(usage, preflight) {
			t.Fatalf("ASSERT_CENSUS_PUBLIC_USAGE_PARITY_%s: %s", name, usage)
		}
	}
}

func TestCensusSelectedPathsDigestFixedVectorAndOrdering(t *testing.T) {
	const want = "sha256:5d08aff1569702b204f1eedd045ef0c62f6173adf0aa6204758bd44aec4a884e"
	if got := censuspreflight.DigestSelected([]string{"z.go", "a.go"}); got != want {
		t.Fatalf("ASSERT_CENSUS_SELECTED_DIGEST_VECTOR: got=%s want=%s", got, want)
	}
	if got := censuspreflight.DigestSelected([]string{"a.go", "z.go"}); got != want {
		t.Fatalf("ASSERT_CENSUS_SELECTED_DIGEST_SORTED: got=%s want=%s", got, want)
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
