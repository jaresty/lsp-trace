package main

import (
	"bytes"
	"strings"
	"testing"

	"lsp-trace/internal/seedformat"
)

func TestTraceExactTargetGrammarAndDefaults(t *testing.T) {
	base := []string{"--workspace", t.TempDir(), "--server", "server"}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing", base, "exactly one"},
		{"file-without-symbol", append(append([]string{}, base...), "--file", "main.go"), "exactly one"},
		{"symbol-without-file", append(append([]string{}, base...), "--symbol", "Run"), "exactly one"},
		{"mixed", append(append([]string{}, base...), "--file", "main.go", "--symbol", "Run", "--at", "main.go:1:1"), "mutually exclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseTrace(tc.args); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ASSERT_TRACE_EXCLUSIVE_TARGET_GRAMMAR: err=%v want=%q", err, tc.want)
			}
		})
	}
	cfg, err := parseTrace(append(append([]string{}, base...), "--at", "main.go:1:1", "--at", "other.go:2:3"))
	if err != nil || len(cfg.ats) != 2 || cfg.downDepth != 2 || cfg.upDepth != 2 {
		t.Fatalf("ASSERT_TRACE_REPEATABLE_AT_DEFAULTS: cfg=%+v err=%v", cfg, err)
	}
}

func TestTraceCanonicalSeedsV2SingleAndMultiTarget(t *testing.T) {
	workspace := t.TempDir()
	cfg, err := parseTrace([]string{"--workspace", workspace, "--server", "server", "--at", "main.go:1:1", "--at", "other.go:2:3"})
	if err != nil {
		t.Fatal(err)
	}
	file, raw, err := traceSeeds(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if file.SchemaVersion != seedformat.Version || file.CoordinateConvention != seedformat.CoordinateConvention || len(file.Seeds) != 2 {
		t.Fatalf("ASSERT_TRACE_CANONICAL_SEEDS_V2_MULTI_TARGET: file=%+v raw=%s", file, raw)
	}
	decoded, err := seedformat.Decode(raw, workspace)
	if err != nil || !bytes.Equal(raw, mustCanonicalTraceSeeds(t, decoded, workspace)) {
		t.Fatalf("ASSERT_TRACE_CANONICAL_SEEDS_V2_BYTE_STABLE: err=%v raw=%s", err, raw)
	}
	manifest, err := traceManifest(cfg, file)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Expansion.TopmostSiblings || len(manifest.RequiredTargets) != 1 || *manifest.Root.Locator.Line != 0 || *manifest.RequiredTargets[0].Locator.Line != 1 {
		t.Fatalf("ASSERT_TRACE_MULTI_TARGET_V5_ONE_BASED_NO_SIBLINGS: %+v", manifest)
	}
}

func mustCanonicalTraceSeeds(t *testing.T, file seedformat.File, workspace string) []byte {
	t.Helper()
	raw, err := seedformat.EncodeCanonical(file, workspace)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
