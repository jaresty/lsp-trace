package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lsp"
)

func TestTopLevelHelpSucceedsOnStdout(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		stdout, stderr, code := captureRun(t, []string{arg})
		if code != 0 || stderr != "" || !strings.Contains(stdout, "usage:\n") || !strings.Contains(stdout, "lsp-trace trace") {
			t.Fatalf("ASSERT_TOP_LEVEL_HELP_SUCCESS: arg=%s code=%d stdout=%q stderr=%q", arg, code, stdout, stderr)
		}
	}
}

func TestInfoIsDeterministicBoundedAndPrivate(t *testing.T) {
	const secret = "INFO_MUST_NOT_LEAK_4f3c2a"
	t.Setenv("LSP_TRACE_BOOTSTRAP_CONFIG", secret)
	t.Setenv("LSP_TRACE_PRIVATE_PATH", secret)
	first, stderr, code := captureRun(t, []string{"info"})
	second, secondStderr, secondCode := captureRun(t, []string{"info"})
	if code != 0 || secondCode != 0 || stderr != "" || secondStderr != "" {
		t.Fatalf("ASSERT_INFO_OFFLINE_SUCCESS: codes=%d,%d stderr=%q,%q", code, secondCode, stderr, secondStderr)
	}
	if first != second {
		t.Fatalf("ASSERT_INFO_DETERMINISTIC: first=%q second=%q", first, second)
	}
	if len(first) > 16*1024 {
		t.Fatalf("ASSERT_INFO_BOUNDED_16K: bytes=%d", len(first))
	}
	var got struct {
		Version               string              `json:"version"`
		BuildRevision         string              `json:"build_revision"`
		Schemas               map[string][]string `json:"schemas"`
		DefaultMCPToolProfile string              `json:"default_mcp_tool_profile"`
		AdvertisedToolCount   int                 `json:"advertised_tool_count"`
		DispatchableCount     int                 `json:"dispatchable_operation_count"`
		InlineByteLimit       uint64              `json:"inline_byte_limit"`
	}
	if err := json.Unmarshal([]byte(first), &got); err != nil {
		t.Fatalf("ASSERT_INFO_JSON: %v output=%q", err, first)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(first), &fields); err != nil || len(fields) != 7 {
		t.Fatalf("ASSERT_INFO_CLOSED_SEVEN_FIELDS: fields=%v err=%v", fields, err)
	}
	if got.Version == "" || got.BuildRevision == "" {
		t.Fatalf("ASSERT_INFO_BUILD_IDENTITY_BOUNDED: %#v", got)
	}
	if got.DefaultMCPToolProfile != "full" || got.AdvertisedToolCount != 35 || got.DispatchableCount != 35 || got.InlineByteLimit != 1048576 {
		t.Fatalf("ASSERT_INFO_REGISTRY_AUTHORITY: %#v", got)
	}
	if !reflect.DeepEqual(got.Schemas["graph"], []string{"v1", "v2", "v3", "v4", "v5"}) || len(got.Schemas) < 10 {
		t.Fatalf("ASSERT_INFO_SCHEMA_REGISTRY: %#v", got.Schemas)
	}
	if strings.Contains(first, secret) {
		t.Fatalf("ASSERT_INFO_ENVIRONMENT_VALUE_NOT_READ: %s", first)
	}
	lower := strings.ToLower(first)
	for _, forbidden := range []string{"bootstrap", "config", "path", "environment", "session", "provider", "host", "authority"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("ASSERT_INFO_PRIVACY_EXCLUDES_%s: %s", strings.ToUpper(forbidden), first)
		}
	}
}

func TestVersionSucceedsWithBuildIdentityOnStdout(t *testing.T) {
	for _, arg := range []string{"--version", "version"} {
		stdout, stderr, code := captureRun(t, []string{arg})
		if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "lsp-trace ") || !strings.Contains(stdout, " revision=") || !strings.Contains(stdout, " modified=") {
			t.Fatalf("ASSERT_VERSION_BUILD_IDENTITY: arg=%s code=%d stdout=%q stderr=%q", arg, code, stdout, stderr)
		}
	}
}

func TestSliceHelpSucceedsOnStdout(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		stdout, stderr, code := captureRun(t, []string{"slice", arg})
		if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage of slice:") {
			t.Fatalf("ASSERT_SLICE_HELP_SUCCESS: arg=%s code=%d stdout=%q stderr=%q", arg, code, stdout, stderr)
		}
	}
}

func TestSliceHelpDistinguishesSelectorsAndPublication(t *testing.T) {
	stdout, stderr, code := captureRun(t, []string{"slice", "--help"})
	for _, want := range []string{
		"repeatable source file or directory",
		"exact target selector by document symbol",
		"exact target selector by PATH:LINE:COLUMN",
		"output destination; caller-provided publication location",
		"artifact selector is the product-generated reference to an immutable artifact",
	} {
		if code != 0 || stderr != "" || !strings.Contains(stdout, want) {
			t.Fatalf("ASSERT_SLICE_SELECTOR_TERMINOLOGY: missing=%q code=%d stdout=%q stderr=%q", want, code, stdout, stderr)
		}
	}
}

func TestTopLevelUsageAdvertisesFilterAndSchemaFamilies(t *testing.T) {
	stdout, stderr, code := captureRun(t, nil)
	if code != 1 || stdout != "" {
		t.Fatalf("ASSERT_TOP_LEVEL_FAMILY_USAGE: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{
		"lsp-trace filter INSPECTION --compare-seeds LABEL --compare-seeds LABEL [--json]",
		"lsp-trace schema get --family graph|inspect|filter|passage-verification --version VERSION",
		"lsp-trace validate --family graph|inspect|filter|passage-verification --version VERSION PATH|-",
		"lsp-trace execute --request-id ID --input PATH|-",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("ASSERT_TOP_LEVEL_FAMILY_USAGE: missing %q in %q", want, stderr)
		}
	}
}

func validArgs(workspace string) []string {
	return []string{"--workspace", workspace, "--server", "server", "--at", "main.go:1:1"}
}

func TestParseRejectsTrailingArguments(t *testing.T) {
	_, err := parse(append(validArgs(t.TempDir()), "unexpected"))
	if err == nil || !strings.Contains(err.Error(), "unexpected positional") {
		t.Fatalf("parse error = %v, want unexpected positional argument", err)
	}
}

func TestParseValidatesFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"required", nil, "required"},
		{"negative limit", append(validArgs(t.TempDir()), "--max-depth", "-1"), "non-negative"},
		{"zero request timeout", append(validArgs(t.TempDir()), "--request-timeout", "0"), "request-timeout must be greater than zero"},
		{"bad env missing equals", append(validArgs(t.TempDir()), "--server-env", "KEY"), "invalid --server-env"},
		{"bad env empty key", append(validArgs(t.TempDir()), "--server-env", "=value"), "invalid --server-env"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parse error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestRunPackagesCallerSuppliedInvocationProvenance(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(validArgs(workspace),
		"--provenance-invocation-id", "run-123",
		"--provenance-caller", "audit-agent",
		"--provenance-source", "review-request",
		"--provenance-source-revision", "commit-abc",
		"--provenance-server-version", "server-1.2.3",
		"--provenance-timestamp", "2026-08-31T19:00:00Z",
		"--provenance-tool-version", "v0.3.0",
	)
	stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
	var receipt struct {
		Tool struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"tool"`
		Invocation struct {
			WorkspaceURI string `json:"workspace_uri"`
			Server       struct {
				Command string `json:"command"`
			} `json:"server"`
			Provenance struct {
				InvocationID   string `json:"invocation_id"`
				Caller         string `json:"caller"`
				Source         string `json:"source"`
				SourceRevision string `json:"source_revision"`
				ServerVersion  string `json:"server_version"`
				Timestamp      string `json:"timestamp"`
			} `json:"provenance"`
		} `json:"invocation"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("ASSERT_RECEIPT_STDOUT_JSON: %v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if receipt.Tool.Name != "lsp-trace" || receipt.Tool.Version != "v0.3.0" {
		t.Fatalf("ASSERT_RECEIPT_TOOL_IDENTITY: tool=%#v stdout=%s", receipt.Tool, stdout)
	}
	if receipt.Invocation.WorkspaceURI == "" || receipt.Invocation.Server.Command != "server" {
		t.Fatalf("ASSERT_RECEIPT_INVOCATION_PARAMETERS: invocation=%#v", receipt.Invocation)
	}
	if got := receipt.Invocation.Provenance; got.InvocationID != "run-123" || got.Caller != "audit-agent" || got.Source != "review-request" || got.SourceRevision != "commit-abc" || got.ServerVersion != "server-1.2.3" || got.Timestamp != "2026-08-31T19:00:00Z" {
		t.Fatalf("ASSERT_CALLER_SUPPLIED_PROVENANCE: provenance=%#v", got)
	}
	if code != 1 || !strings.Contains(stderr, "spawn:") {
		t.Fatalf("ASSERT_RECEIPT_STREAM_CONTRACT: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
}

func TestRunUsesUnknownForOmittedInvocationProvenance(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ := captureRun(t, append([]string{"incoming"}, validArgs(workspace)...))
	var receipt struct {
		Invocation struct {
			Provenance map[string]string `json:"provenance"`
		} `json:"invocation"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("ASSERT_UNKNOWN_PROVENANCE_STDOUT_JSON: %v stdout=%q", err, stdout)
	}
	for _, key := range []string{"invocation_id", "caller", "source", "source_revision", "server_version", "timestamp"} {
		if receipt.Invocation.Provenance[key] != "UNKNOWN" {
			t.Fatalf("ASSERT_OMITTED_PROVENANCE_UNKNOWN: key=%s provenance=%#v", key, receipt.Invocation.Provenance)
		}
	}
}

func TestParseAcceptsTopmostSiblingsOptIn(t *testing.T) {
	cfg, err := parse(append(validArgs(t.TempDir()), "--expand-topmost-siblings"))
	if err != nil || !cfg.topmostSiblings {
		t.Fatalf("ASSERT_TOPMOST_SIBLINGS_CLI_OPT_IN: enabled=%t err=%v", cfg.topmostSiblings, err)
	}
}

func TestUsageKeepsLegacyVisibleAndAdvertisesEmbeddedSkill(t *testing.T) {
	if strings.Count(usageText, "lsp-trace census") != 1 {
		t.Fatalf("ASSERT_USAGE_ADVERTISES_CENSUS_ONCE: %q", usageText)
	}
	if !strings.Contains(usageText, "lsp-trace incoming ") || !strings.Contains(usageText, "lsp-trace slice ") {
		t.Fatalf("ASSERT_USAGE_KEEPS_LEGACY_VISIBLE_UNTIL_PARITY: %q", usageText)
	}
	if !strings.Contains(usageText, "lsp-trace trace") || !strings.Contains(usageText, "lsp-trace inspect SELECTOR_OR_ARTIFACT (--seed LABEL | --all-seeds)") || !strings.Contains(usageText, "lsp-trace skill get") {
		t.Fatalf("ASSERT_USAGE_ADVERTISES_PRIMARY_COMMANDS: %q", usageText)
	}
	for _, want := range []string{
		"target selector identifies a symbol or position",
		"output destination is a caller-provided publication location",
		"artifact selector is a product-generated reference to an immutable artifact",
	} {
		if !strings.Contains(usageText, want) {
			t.Fatalf("ASSERT_USAGE_SELECTOR_TERMINOLOGY: missing=%q usage=%q", want, usageText)
		}
	}
}

func TestSkillDispatcherPreservesGetAndSelectsBothSkills(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runSkill([]string{"get"}, &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.String() != embeddedSkill {
		t.Fatalf("ASSERT_SKILL_GET_BACKWARD_COMPATIBLE: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, name := range []string{"lsp-trace", "lsp-trace-feature-inventory"} {
		files, err := embeddedSkillFiles(name)
		if err != nil || len(files) < 2 || len(files["SKILL.md"]) == 0 {
			t.Fatalf("ASSERT_SKILL_DISPATCH: name=%s files=%d err=%v", name, len(files), err)
		}
	}
	for _, args := range [][]string{{"list"}, {"get", "unknown", "destination"}, {"get", "lsp-trace"}} {
		stdout.Reset()
		stderr.Reset()
		if code := runSkill(args, &stdout, &stderr); code == 0 || stdout.Len() != 0 {
			t.Fatalf("ASSERT_SKILL_REJECTS_UNSUPPORTED: args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestSkillExportManifestsAndBytesMatchSourceEmbeddedAndExport(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name       string
		sourceRoot string
		manifest   []string
	}{
		{"lsp-trace", ".", []string{
			"SKILL.md",
			"references/evidence-boundaries.md",
			"references/live-tracing.md",
			"references/offline-evidence.md",
			"references/transport-routing.md",
		}},
		{"lsp-trace-feature-inventory", "../../.pi/skills/lsp-trace-feature-inventory", []string{
			"SKILL.md",
			"references/adjudication-and-acceptance.md",
			"references/preparation-and-grouping.md",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sort.Strings(tc.manifest)
			embedded, err := embeddedSkillFiles(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			gotManifest := make([]string, 0, len(embedded))
			for relative := range embedded {
				gotManifest = append(gotManifest, relative)
			}
			sort.Strings(gotManifest)
			if !reflect.DeepEqual(gotManifest, tc.manifest) {
				t.Fatalf("ASSERT_SKILL_EMBEDDED_COMPLETE_SORTED_MANIFEST: got=%v want=%v", gotManifest, tc.manifest)
			}

			destination := filepath.Join(root, tc.name)
			var stdout, stderr bytes.Buffer
			if code := runSkill([]string{"get", tc.name, destination}, &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.String() != "" {
				t.Fatalf("ASSERT_SKILL_EXPORT_STDOUT_GRAMMAR: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			exportedManifest := fileManifest(t, destination)
			if !reflect.DeepEqual(exportedManifest, tc.manifest) {
				t.Fatalf("ASSERT_SKILL_EXPORT_COMPLETE_SORTED_MANIFEST: got=%v want=%v", exportedManifest, tc.manifest)
			}
			for _, relative := range tc.manifest {
				source, err := os.ReadFile(filepath.Join(tc.sourceRoot, filepath.FromSlash(relative)))
				if err != nil {
					t.Fatal(err)
				}
				exported, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(relative)))
				if err != nil || !bytes.Equal(source, embedded[relative]) || !bytes.Equal(source, exported) {
					t.Fatalf("ASSERT_SKILL_SOURCE_EMBEDDED_EXPORT_BYTE_PARITY: file=%s err=%v", relative, err)
				}
				info, err := os.Stat(filepath.Join(destination, filepath.FromSlash(relative)))
				if err != nil || info.Mode().Perm()&0o077 != 0 {
					t.Fatalf("ASSERT_SKILL_EXPORT_PRIVATE_FILE_MODE: file=%s mode=%v err=%v", relative, info.Mode(), err)
				}
			}
		})
	}
}

func fileManifest(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

func TestSkillExportDestinationValidationAndSymlinkAncestry(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "existing")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	fileParent := filepath.Join(root, "file")
	if err := os.WriteFile(fileParent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{
		"", ".", "..", string(filepath.Separator),
		root + string(filepath.Separator) + ".." + string(filepath.Separator) + "escaped",
		existing, filepath.Join(fileParent, "skill"), filepath.Join(root, "missing", "skill"),
	} {
		var stdout, stderr bytes.Buffer
		if code := runSkill([]string{"get", "lsp-trace", destination}, &stdout, &stderr); code == 0 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("ASSERT_SKILL_EXPORT_REJECTS_UNSAFE_DESTINATION: destination=%q code=%d stdout=%q stderr=%q", destination, code, stdout.String(), stderr.String())
		}
	}

	realParent := filepath.Join(root, "real")
	if err := os.Mkdir(realParent, 0o700); err != nil {
		t.Fatal(err)
	}
	symlinkParent := filepath.Join(root, "link")
	if err := os.Symlink(realParent, symlinkParent); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runSkill([]string{"get", "lsp-trace", filepath.Join(symlinkParent, "skill")}, &stdout, &stderr); code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("ASSERT_SKILL_EXPORT_ALLOWS_SYMLINK_ANCESTRY: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestMaterializeSkillRejectsUnsafeEmbeddedPathsExactly(t *testing.T) {
	for _, relative := range []string{"", ".", "../x", "a/../x", "/x", "a//x", "./x", "a/./x"} {
		destination := filepath.Join(t.TempDir(), "skill")
		if err := materializeSkill(destination, map[string][]byte{relative: []byte("x")}); err == nil {
			t.Fatalf("ASSERT_SKILL_REJECTS_UNSAFE_EMBEDDED_PATH: path=%q", relative)
		}
		if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("ASSERT_SKILL_UNSAFE_PATH_LEAVES_NO_FINAL: path=%q err=%v", relative, err)
		}
	}
	if err := materializeSkill(filepath.Join(t.TempDir(), "skill"), map[string][]byte{"guide..md": []byte("ok")}); err != nil {
		t.Fatalf("ASSERT_SKILL_PATH_VALIDATION_NOT_DOT_SUBSTRING_HEURISTIC: %v", err)
	}
}

func TestMaterializeSkillIsTransactionalPrivateAndNoOverwrite(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "skill")
	if err := materializeSkill(destination, map[string][]byte{"file": []byte("original"), "file/child": []byte("impossible")}); err == nil {
		t.Fatal("ASSERT_SKILL_TRANSACTION_FAILURE_REQUIRED")
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ASSERT_SKILL_TRANSACTION_LEAVES_NO_FINAL: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("ASSERT_SKILL_TRANSACTION_CLEANS_ONLY_STAGING: entries=%v err=%v", entries, err)
	}

	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(destination, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := materializeSkill(destination, map[string][]byte{"SKILL.md": []byte("replace")}); err == nil {
		t.Fatal("ASSERT_SKILL_NO_OVERWRITE_REJECTION_REQUIRED")
	}
	got, err := os.ReadFile(marker)
	if err != nil || string(got) != "keep" {
		t.Fatalf("ASSERT_SKILL_EXISTING_FINAL_UNCHANGED: got=%q err=%v", got, err)
	}
}

func TestMaterializeSkillPublicationRaceNeverReplacesCompetitor(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "skill")
	originalPublish := publishSkillDirectory
	t.Cleanup(func() { publishSkillDirectory = originalPublish })
	var competitorInfo os.FileInfo
	publishSkillDirectory = func(parentHandle, containerHandle, payloadHandle *os.File, container, payload, final string) error {
		competitor := filepath.Join(parent, final)
		if err := os.Mkdir(competitor, 0o700); err != nil {
			return err
		}
		var err error
		competitorInfo, err = os.Stat(competitor)
		if err != nil {
			return err
		}
		return originalPublish(parentHandle, containerHandle, payloadHandle, container, payload, final)
	}

	err := materializeSkill(destination, map[string][]byte{"SKILL.md": []byte("publisher")})
	if err == nil || err.Error() != "skill destination already exists" {
		t.Fatalf("ASSERT_SKILL_PUBLICATION_RACE_NORMALIZED_EXISTS: err=%v", err)
	}
	finalInfo, statErr := os.Stat(destination)
	if statErr != nil || competitorInfo == nil || !os.SameFile(competitorInfo, finalInfo) {
		t.Fatalf("ASSERT_SKILL_PUBLICATION_RACE_NEVER_REPLACES: competitor=%v final=%v err=%v", competitorInfo, finalInfo, statErr)
	}
}

func TestMaterializeSkillMovedAndSubstitutedContainerCannotSubstitutePayloadOrDeleteCompetitor(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "skill")
	moved := filepath.Join(parent, "moved-container")
	originalHook := afterSkillContainerPinned
	t.Cleanup(func() { afterSkillContainerPinned = originalHook })
	var substitute string
	afterSkillContainerPinned = func(_ *os.File, container string) error {
		if err := os.Rename(filepath.Join(parent, container), moved); err != nil {
			return err
		}
		substitute = filepath.Join(parent, container)
		if err := os.MkdirAll(filepath.Join(substitute, "payload"), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(substitute, "payload", "SKILL.md"), []byte("competitor"), 0o600); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(substitute, "competitor"), []byte("keep"), 0o600)
	}

	err := materializeSkill(destination, map[string][]byte{"SKILL.md": []byte("publisher")})
	if got, readErr := os.ReadFile(filepath.Join(substitute, "competitor")); readErr != nil || string(got) != "keep" {
		t.Fatalf("ASSERT_SKILL_SUBSTITUTED_CONTAINER_NEVER_DELETED: got=%q err=%v publish=%v", got, readErr, err)
	}
	if err == nil {
		if got, readErr := os.ReadFile(filepath.Join(destination, "SKILL.md")); readErr != nil || string(got) != "publisher" {
			t.Fatalf("ASSERT_SKILL_PINNED_CONTAINER_PAYLOAD_PUBLISHED: got=%q err=%v", got, readErr)
		}
	} else if _, statErr := os.Lstat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("ASSERT_SKILL_CONTAINER_SUBSTITUTION_FAILS_SAFE: publish=%v final=%v", err, statErr)
	}
}

func TestParseAcceptsDispatchFamilyOptIn(t *testing.T) {
	cfg, err := parse(append(validArgs(t.TempDir()), "--expand-dispatch-family"))
	if err != nil || !cfg.expandDispatchFamily {
		t.Fatalf("ASSERT_DISPATCH_FAMILY_CLI_OPT_IN: enabled=%t err=%v", cfg.expandDispatchFamily, err)
	}
}

type dispatchIntegrationClient struct{}

func (dispatchIntegrationClient) SupportsTypeHierarchy() bool { return true }
func (dispatchIntegrationClient) PrepareTypeHierarchy(context.Context, lsp.PrepareTypeHierarchyParams) ([]lsp.TypeHierarchyItem, error) {
	return []lsp.TypeHierarchyItem{{Name: "Contract", URI: "file:///contract", SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}}, nil
}
func (dispatchIntegrationClient) Subtypes(_ context.Context, item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error) {
	if item.Name != "Contract" {
		return nil, nil
	}
	return []lsp.TypeHierarchyItem{{Name: "Implementation", URI: "file:///implementation", SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}}, nil
}

func TestResolveDispatchRelationshipsPreservesSeedAndSeparateNodes(t *testing.T) {
	relationships, diagnostics := resolveDispatchRelationships(context.Background(), dispatchIntegrationClient{}, lsp.PrepareTypeHierarchyParams{}, "entry")
	if len(relationships) != 1 || relationships[0].SeedLabel != "entry" || relationships[0].Interface.Name != "Contract" || relationships[0].Implementation.Name != "Implementation" || len(diagnostics) != 0 {
		t.Fatalf("ASSERT_DISPATCH_INTEGRATION_RELATIONSHIP: relationships=%#v diagnostics=%#v", relationships, diagnostics)
	}
}

func TestParseConcurrencyContract(t *testing.T) {
	if _, err := parse(append(validArgs(t.TempDir()), "--concurrency", "1")); err != nil {
		t.Fatalf("concurrency 1: %v", err)
	}
	for _, value := range []string{"0", "2", "-1"} {
		t.Run(value, func(t *testing.T) {
			_, err := parse(append(validArgs(t.TempDir()), "--concurrency", value))
			if err == nil || !strings.Contains(err.Error(), "--concurrency must be 1") {
				t.Fatalf("parse error = %v, want concurrency validation", err)
			}
		})
	}
}

func TestParseLogLevelContract(t *testing.T) {
	for _, level := range []string{"error", "warn", "info", "debug"} {
		t.Run(level, func(t *testing.T) {
			if _, err := parse(append(validArgs(t.TempDir()), "--log-level", level)); err != nil {
				t.Fatalf("valid log level: %v", err)
			}
		})
	}
	_, err := parse(append(validArgs(t.TempDir()), "--log-level", "trace"))
	if err == nil || !strings.Contains(err.Error(), "invalid --log-level") {
		t.Fatalf("parse error = %v, want log-level validation", err)
	}
}

func TestParseAcceptsRepeatableFlags(t *testing.T) {
	args := append(validArgs(t.TempDir()), "--server-arg", "--stdio", "--server-arg", "x", "--server-env", "A=1", "--server-env", "B=")
	cfg, err := parse(args)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.args, ","); got != "--stdio,x" {
		t.Fatalf("args = %q", got)
	}
	if got := strings.Join(cfg.env, ","); got != "A=1,B=" {
		t.Fatalf("env = %q", got)
	}
	if cfg.requestTimeout != 30*time.Second {
		t.Fatalf("request timeout = %s", cfg.requestTimeout)
	}
}

func TestParseRejectsDuplicateServerEnvNames(t *testing.T) {
	for _, declarations := range [][]string{{"TOKEN=one", "TOKEN=one"}, {"TOKEN=one", "TOKEN=two"}} {
		args := append(validArgs(t.TempDir()), "--server-env", declarations[0], "--server-env", declarations[1])
		stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
		if code != 1 || stdout != "" || strings.Contains(stderr, "deprecated") || !strings.HasSuffix(strings.TrimSpace(stderr), `duplicate --server-env name "TOKEN"`) {
			t.Fatalf("ASSERT_DUPLICATE_SERVER_ENV_PARSE_REJECTION: declarations=%v code=%d stdout=%q stderr=%q", declarations, code, stdout, stderr)
		}
	}
}

func TestParseAcceptsDistinctServerEnvNames(t *testing.T) {
	if _, err := parse(append(validArgs(t.TempDir()), "--server-env", "TOKEN=one", "--server-env", "OTHER=two")); err != nil {
		t.Fatalf("ASSERT_DISTINCT_SERVER_ENV_NAMES_PASS: %v", err)
	}
}

func TestParseAcceptsRepeatedAtAndSeedFile(t *testing.T) {
	workspace := t.TempDir()
	seedFile := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(seedFile, []byte(`{"seeds":[{"label":"interface","at":"main.go:1:1"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--workspace", workspace, "--server", "server", "--at", "main.go:1:1", "--at", "main.go:2:1", "--seed-file", seedFile}
	if _, err := parse(args); err != nil {
		t.Fatalf("ASSERT_REPEATABLE_AT_ACCEPTED: %v", err)
	}
}

func TestParseRejectsZeroSeedsAndInvalidOrDuplicateLabels(t *testing.T) {
	workspace := t.TempDir()
	base := []string{"--workspace", workspace, "--server", "server"}
	if _, err := parse(base); err == nil || !strings.Contains(err.Error(), "seed") {
		t.Fatalf("ASSERT_SEED_FILE_VALIDATION: zero seeds error=%v", err)
	}
	for name, body := range map[string]string{
		"invalid":   `{"seeds":[{"label":"","at":"main.go:1:1"}]}`,
		"duplicate": `{"seeds":[{"label":"same","at":"main.go:1:1"},{"label":"same","at":"main.go:2:1"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "seeds.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := parse(append(base, "--seed-file", path)); err == nil || !strings.Contains(err.Error(), "label") {
				t.Fatalf("ASSERT_SEED_FILE_VALIDATION: %s error=%v", name, err)
			}
		})
	}
}

func TestParseAt(t *testing.T) {
	path, line, col, err := parseAt("C:\\src\\file.go:12:34")
	if err != nil || path != "C:\\src\\file.go" || line != 12 || col != 34 {
		t.Fatalf("parseAt = %q,%d,%d,%v", path, line, col, err)
	}
	for _, input := range []string{"file.go", "file.go:x:1", "file.go:1:0", ":1:1"} {
		t.Run(input, func(t *testing.T) {
			if _, _, _, err := parseAt(input); err == nil {
				t.Fatalf("parseAt(%q) succeeded", input)
			}
		})
	}
}

func TestRunUsageAndParseErrorsUseStderrOnly(t *testing.T) {
	for _, args := range [][]string{nil, {"outgoing"}, {"incoming", "--workspace", "x"}} {
		stdout, stderr, code := captureRun(t, args)
		if code != 1 || stdout != "" || stderr == "" {
			t.Fatalf("run(%v) stdout=%q stderr=%q code=%d", args, stdout, stderr, code)
		}
	}
}

func TestRuntimeHelperServer(t *testing.T) {
	if os.Getenv("LSP_TRACE_RUNTIME_HELPER") == "" {
		return
	}
	fmt.Fprintln(os.Stderr, "runtime helper diagnostic")
	r := bufio.NewReader(os.Stdin)
	for {
		body, err := readRuntimeMessage(r)
		if err != nil {
			return
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(body, &msg) != nil || len(msg.ID) == 0 {
			continue
		}
		switch msg.Method {
		case "initialize":
			if os.Getenv("LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE") == "1" {
				return
			}
			writeRuntimeResponse(msg.ID, map[string]any{"capabilities": map[string]any{"callHierarchyProvider": true}})
		case "textDocument/documentSymbol":
			writeRuntimeResponse(msg.ID, []any{})
		case "textDocument/prepareCallHierarchy":
			time.Sleep(3 * time.Second)
			writeRuntimeResponse(msg.ID, []any{})
		case "shutdown":
			writeRuntimeResponse(msg.ID, nil)
			return
		}
	}
}

func readRuntimeMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(name, "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing content length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeRuntimeResponse(id json.RawMessage, result any) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func assertMixedFailureSeeds(t *testing.T, resultJSON []byte, globalPhase string) {
	t.Helper()
	var got struct {
		SchemaVersion string `json:"schema_version"`
		Invocation    struct {
			Seeds []struct {
				Label string `json:"label"`
			} `json:"seeds"`
		} `json:"invocation"`
		Seeds []struct {
			Label   string `json:"label"`
			Failure *struct {
				Phase string `json:"phase"`
			} `json:"failure"`
		} `json:"seeds"`
		Summary struct {
			TraversalComplete bool `json:"traversal_complete"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(resultJSON, &got); err != nil {
		t.Fatalf("ASSERT_MIXED_FAILURE_SERIALIZES: %v: %s", err, resultJSON)
	}
	phases := map[string]string{}
	resultLabels := map[string]struct{}{}
	for _, seed := range got.Seeds {
		if _, duplicate := resultLabels[seed.Label]; duplicate {
			t.Fatalf("ASSERT_MIXED_PREFLIGHT_GLOBAL_UNIQUE_RESULT_LABELS: %#v", got.Seeds)
		}
		resultLabels[seed.Label] = struct{}{}
		if seed.Failure != nil {
			phases[seed.Label] = seed.Failure.Phase
		}
	}
	invocationLabels := map[string]struct{}{}
	for _, seed := range got.Invocation.Seeds {
		invocationLabels[seed.Label] = struct{}{}
	}
	if got.SchemaVersion != "lsp-trace.graph.v3" || len(got.Invocation.Seeds) != 2 || len(got.Seeds) != 2 || !reflect.DeepEqual(invocationLabels, resultLabels) || phases["bad"] != "source" || phases["good"] != globalPhase || got.Summary.TraversalComplete {
		t.Fatalf("ASSERT_MIXED_PREFLIGHT_GLOBAL_ONE_RESULT_PER_SEED: schema=%q invocation=%#v seeds=%#v phases=%#v complete=%v", got.SchemaVersion, got.Invocation.Seeds, got.Seeds, phases, got.Summary.TraversalComplete)
	}
}

func TestRunMixedSourceAndSpawnFailurePreservesEverySeed(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ := captureRun(t, []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--seed-file", writeMixedSeedFile(t)})
	assertMixedFailureSeeds(t, []byte(stdout), "spawn")
}

func TestRunMixedSourceAndTraceOpenFailurePreservesEverySeed(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, _, _ := captureRun(t, []string{"incoming", "--workspace", workspace, "--server", "server", "--seed-file", writeMixedSeedFile(t), "--trace-lsp", t.TempDir()})
	assertMixedFailureSeeds(t, []byte(stdout), "trace")
}

func writeMixedSeedFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(path, []byte(`{"seeds":[{"label":"bad","at":"missing.go:1:1"},{"label":"good","at":"good.go:1:1"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertPublishedMixedFailure(t *testing.T, args []string, globalPhase string) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "bundle.json")
	stdout, stderr, code := captureRun(t, append(args, "--output", output))
	if code == 0 || stdout != "" {
		t.Fatalf("ASSERT_MIXED_FAILURE_PUBLISH_EXIT: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	artifact, err := readSelectedArtifact(output)
	if err != nil {
		t.Fatal(err)
	}
	assertMixedFailureSeeds(t, artifact, globalPhase)
	verifyOut, verifyErr, verifyCode := captureRun(t, []string{"verify", output})
	if verifyCode != 0 || verifyOut != "verified integrity and custody\n" || verifyErr != "" {
		t.Fatalf("ASSERT_MIXED_FAILURE_PUBLISH_VERIFY: code=%d stdout=%q stderr=%q", verifyCode, verifyOut, verifyErr)
	}
}

func TestMixedMissingSourceAndSpawnPreservesEverySeedResult(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assertPublishedMixedFailure(t, []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--seed-file", writeMixedSeedFile(t)}, "spawn")
}

func TestMixedMissingSourceAndTraceOpenPreservesEverySeedResult(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assertPublishedMixedFailure(t, []string{"incoming", "--workspace", workspace, "--server", "server", "--seed-file", writeMixedSeedFile(t), "--trace-lsp", t.TempDir()}, "trace")
}

func TestMixedMissingSourceAndInitializePreservesEverySeedResult(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"incoming", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=TestRuntimeHelperServer", "--server-env", "LSP_TRACE_RUNTIME_HELPER=1", "--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1", "--seed-file", writeMixedSeedFile(t), "--request-timeout", "1s"}
	assertPublishedMixedFailure(t, args, "initialize")
}

func TestRunInitializeFailureIsIncomplete(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(validArgs(workspace),
		"--server", os.Args[0],
		"--server-arg", "-test.run=TestRuntimeHelperServer",
		"--server-env", "LSP_TRACE_RUNTIME_HELPER=1",
		"--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1",
		"--request-timeout", "1s",
	)
	stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
	var result struct {
		Summary struct {
			TraversalComplete bool `json:"traversal_complete"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not graph JSON: %v; stdout=%q stderr=%q", err, stdout, stderr)
	}
	if code != 1 || result.Summary.TraversalComplete {
		t.Fatalf("ASSERT_INITIALIZE_FAILURE_INCOMPLETE: code=%d complete=%t stdout=%s stderr=%s", code, result.Summary.TraversalComplete, stdout, stderr)
	}
}

func TestRunMixedSourceAndInitializeFailurePreservesEverySeed(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "good.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"incoming", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=TestRuntimeHelperServer", "--server-env", "LSP_TRACE_RUNTIME_HELPER=1", "--server-env", "LSP_TRACE_RUNTIME_HELPER_EXIT_INITIALIZE=1", "--seed-file", writeMixedSeedFile(t), "--request-timeout", "1s"}
	stdout, _, _ := captureRun(t, args)
	assertMixedFailureSeeds(t, []byte(stdout), "initialize")
}

func TestRunRequestTimeoutTraceStderrAndExitPolicy(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(t.TempDir(), "protocol.jsonl")
	args := append(validArgs(workspace),
		"--server", os.Args[0],
		"--server-arg", "-test.run=TestRuntimeHelperServer",
		"--server-env", "LSP_TRACE_RUNTIME_HELPER=1",
		"--request-timeout", "1s",
		"--trace-lsp", tracePath,
		"--log-level", "debug",
	)
	stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
	if code != 2 {
		t.Fatalf("code = %d, want structured-incomplete exit 2; stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "exit code 2 is not an invocation failure") || !strings.Contains(stderr, "inspect summary.traversal_complete and each seed.failure") {
		t.Fatalf("ASSERT_STRUCTURED_INCOMPLETE_GUIDANCE: stderr=%q", stderr)
	}
	var result struct {
		Terminals []struct {
			Reason string `json:"reason"`
		} `json:"terminals"`
		Diagnostics []struct {
			Phase   string `json:"phase"`
			Message string `json:"message"`
		} `json:"diagnostics"`
		Summary struct {
			Complete bool `json:"complete"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not one graph JSON document: %v; %q", err, stdout)
	}
	if result.Summary.Complete || len(result.Terminals) == 0 || result.Terminals[0].Reason != "REQUEST_TIMEOUT" {
		t.Fatalf("result = %+v, want REQUEST_TIMEOUT incomplete graph", result)
	}
	if !strings.Contains(stderr, "runtime helper diagnostic") {
		t.Fatalf("stderr = %q, want captured server diagnostic", stderr)
	}
	foundServerStderr := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Phase == "server-stderr" && strings.Contains(diagnostic.Message, "runtime helper diagnostic") {
			foundServerStderr = true
		}
	}
	if !foundServerStderr {
		t.Fatalf("diagnostics = %+v, want retained server stderr", result.Diagnostics)
	}
	trace, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(trace), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("trace lines = %d, want sent and received events", len(lines))
	}
	for _, line := range lines {
		var event struct {
			Sequence  uint64          `json:"sequence"`
			Direction string          `json:"direction"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &event); err != nil || event.Sequence == 0 || event.Direction == "" || !json.Valid(event.Payload) {
			t.Fatalf("invalid trace event %q: %+v, %v", line, event, err)
		}
	}
}

func TestRunOutputOpenFailureUsesStderrOnly(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(target, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(validArgs(workspace), "--output", filepath.Join(workspace, "missing", "result.json"))
	stdout, stderr, code := captureRun(t, append([]string{"incoming"}, args...))
	if code != 1 || !json.Valid([]byte(stdout)) || !strings.Contains(stderr, "publish:") {
		t.Fatalf("ASSERT_OUTPUT_PUBLICATION_FAILURE_RETENTION: stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestCaptureRunDrainsLargeStdout(t *testing.T) {
	old := embeddedSkill
	embeddedSkill = strings.Repeat("large-output\n", 1<<17)
	t.Cleanup(func() { embeddedSkill = old })
	stdout, stderr, code := captureRun(t, []string{"skill", "get"})
	if code != 0 || stderr != "" || stdout != embeddedSkill {
		t.Fatalf("ASSERT_P4_LARGE_STREAM_CAPTURE: code=%d stdout=%d want=%d stderr=%q", code, len(stdout), len(embeddedSkill), stderr)
	}
}

func TestSliceHelpSaysZeroUpDepthDisablesTraversal(t *testing.T) {
	stdout, stderr, code := captureRun(t, []string{"slice", "--help"})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "incoming traversal depth; 0 disables") || strings.Contains(stdout, "incoming traversal depth; 0 unlimited") {
		t.Fatalf("ASSERT_SLICE_UP_DEPTH_ZERO_HELP: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestParseSliceUsesSymmetricDepthFlagsAndExclusiveStartModes(t *testing.T) {
	base := []string{"--workspace", "/w", "--server", "server", "--down-depth", "2", "--up-depth", "7"}
	t.Run("from file", func(t *testing.T) {
		cfg, err := parseSlice(append(append([]string{}, base...), "--from-file", "a.go"))
		if err != nil || cfg.downDepth != 2 || cfg.upDepth != 7 || cfg.fromFile != "a.go" {
			t.Errorf("ASSERT_SLICE_SYMMETRIC_DEPTH_FLAGS: cfg=%#v err=%v", cfg, err)
		}
	})
	t.Run("repeatable at", func(t *testing.T) {
		cfg, err := parseSlice(append(append([]string{}, base...), "--at", "a.go:1:1", "--at", "b.go:2:3"))
		if err != nil || len(cfg.ats) != 2 {
			t.Errorf("ASSERT_SLICE_REPEATABLE_AT_MODE: cfg=%#v err=%v", cfg, err)
		}
	})
	t.Run("seed file", func(t *testing.T) {
		cfg, err := parseSlice(append(append([]string{}, base...), "--seed-file", "seeds.json"))
		if err != nil || cfg.seedFile != "seeds.json" {
			t.Errorf("ASSERT_SLICE_SEED_FILE_MODE: cfg=%#v err=%v", cfg, err)
		}
	})
	t.Run("exclusive", func(t *testing.T) {
		if _, err := parseSlice(append(append([]string{}, base...), "--from-file", "a.go", "--at", "a.go:1:1")); err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Errorf("ASSERT_SLICE_START_MODES_EXCLUSIVE: %v", err)
		}
	})
	t.Run("managed symbol from file", func(t *testing.T) {
		args := append(append([]string{}, base...), "--graph-provenance", "--from-file", "a.go", "--symbol", "Target")
		cfg, err := parseSlice(args)
		if err != nil || cfg.fromFile != "a.go" || cfg.symbol != "Target" {
			t.Errorf("ASSERT_MANAGED_CLI_FILE_SYMBOL_SELECTOR_PARITY: cfg=%#v err=%v", cfg, err)
		}
	})
	t.Run("managed selector failures", func(t *testing.T) {
		for _, extra := range [][]string{
			{"--graph-provenance", "--from-file", "a.go"},
			{"--graph-provenance", "--symbol", "Target"},
			{"--graph-provenance", "--from-file", "a.go", "--symbol", "Target", "--at", "a.go:1:1"},
		} {
			_, err := parseSlice(append(append([]string{}, base...), extra...))
			for _, want := range []string{
				"exactly one managed target selector",
				"--at PATH:LINE:COLUMN",
				"--from-file PATH --symbol NAME",
			} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("ASSERT_MANAGED_CLI_SELECTOR_FAILS_CLOSED: args=%v missing=%q err=%v", extra, want, err)
				}
			}
		}
	})
	t.Run("no max depth", func(t *testing.T) {
		if _, err := parseSlice(append(append([]string{}, base...), "--max-depth", "7", "--from-file", "a.go")); err == nil {
			t.Error("ASSERT_SLICE_MAX_DEPTH_NOT_EXPOSED: accepted --max-depth")
		}
	})
}

func TestSchemaGetAndValidateCommands(t *testing.T) {
	for _, version := range []string{"v1", "v2", "v3"} {
		version := version
		t.Run("schema get "+version, func(t *testing.T) {
			stdout, stderr, code := captureRun(t, []string{"schema", "get", "--schema", version})
			full := "lsp-trace.graph." + version
			if code != 0 || stderr != "" || !strings.Contains(stdout, `"$schema": "https://json-schema.org/draft/2020-12/schema"`) || !strings.Contains(stdout, `"const": "`+full+`"`) {
				t.Errorf("ASSERT_SCHEMA_GET_%s: code=%d stdout=%q stderr=%q", strings.ToUpper(version), code, stdout, stderr)
			}
		})
	}

	valid := filepath.Join(t.TempDir(), "valid-v1.json")
	validJSON := `{"schema_version":"lsp-trace.graph.v1","invocation":{},"capabilities":{},"targets":[],"nodes":[],"edges":[],"terminals":[],"frontier":[],"diagnostics":[],"summary":{}}`
	if err := os.WriteFile(valid, []byte(validJSON), 0600); err != nil {
		t.Fatal(err)
	}
	t.Run("validate autodetect", func(t *testing.T) {
		stdout, stderr, code := captureRun(t, []string{"validate", valid})
		if code != 0 || stdout != "valid lsp-trace.graph.v1\n" || stderr != "" {
			t.Errorf("ASSERT_SCHEMA_VALIDATE_AUTODETECT: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
	t.Run("validate mismatch", func(t *testing.T) {
		stdout, stderr, code := captureRun(t, []string{"validate", "--schema", "v2", valid})
		if code == 0 || stdout != "" || !strings.Contains(stderr, "schema version mismatch") {
			t.Errorf("ASSERT_SCHEMA_VALIDATE_EXPLICIT_MISMATCH: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
	t.Run("validate stdin", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := runValidate([]string{"-"}, strings.NewReader(validJSON), &stdout, &stderr)
		if code != 0 || stdout.String() != "valid lsp-trace.graph.v1\n" || stderr.String() != "" {
			t.Errorf("ASSERT_SCHEMA_VALIDATE_STDIN: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	invalid := filepath.Join(t.TempDir(), "invalid-v1.json")
	if err := os.WriteFile(invalid, []byte(`{"schema_version":"lsp-trace.graph.v1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Run("validate required field", func(t *testing.T) {
		stdout, stderr, code := captureRun(t, []string{"validate", invalid})
		if code == 0 || stdout != "" || !strings.Contains(stderr, "schema validation") || !strings.Contains(stderr, "invocation") {
			t.Errorf("ASSERT_SCHEMA_VALIDATE_REQUIRED_FIELD: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
}

func captureRun(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW
	var out, stderr bytes.Buffer
	var drains sync.WaitGroup
	drains.Add(2)
	go func() { defer drains.Done(); _, _ = out.ReadFrom(outR) }()
	go func() { defer drains.Done(); _, _ = stderr.ReadFrom(errR) }()
	code := run(args)
	_ = outW.Close()
	_ = errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	drains.Wait()
	return out.String(), stderr.String(), code
}
