package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectHelpIncludesHydratedOptions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runInspect([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("help wrote stdout: %q", stdout.String())
	}
	for _, want := range []string{"-hydrated", "-retained-projection-v2", "-node", "-relation", "-seed", "-publication-root", "-artifact-store", "-private-root", "-enable-private-paths"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("help omits %q: %q", want, stderr.String())
		}
	}
}

func TestHydratedCLIRetainedProjectionV2RequiresExplicitModeAndReachesSharedFailure(t *testing.T) {
	request := `{"mode":"RETAINED_SOURCE_PROJECTION","retained_source_evidence":{"inline_snapshot_v2":"{}"},"selection":{"target":{"graph_subject_id":"node","logical_source_id":"file:///source.go"},"selections":[{"graph_subject_id":"node","logical_source_id":"file:///source.go"}]},"projection":{"body":"OMIT","privacy_policy_id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","limits":{"max_source_bytes":1024,"max_ranges":4,"max_objects":4,"max_work":4,"max_response_bytes":4096}},"resolve_limits":{"max_distinct_objects":4,"max_unique_source_bytes":1024,"max_logical_selections":4}}`
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, []byte(request), 0600); err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()
	if err := os.Chmod(store, 0700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runInspect([]string{path, "--hydrated", "--retained-projection-v2", "--artifact-store", store, "--json"}, &stdout, &stderr)
	if code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "retained source projection failed in ADMIT: ADMISSION_FAILED") {
		t.Fatalf("ASSERT_CLI_RETAINED_V2_SHARED_HANDLER: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestHydratedCLIRejectsBeforeIO(t *testing.T) {
	for _, args := range [][]string{
		{"absent.json", "--retained-projection-v2", "--artifact-store", "/request-must-not-select-or-activate-a-root", "--json"},
		{"absent.json", "--hydrated", "--all-seeds"},
		{"absent.json", "--hydrated", "--seed", "seed"},
		{"absent.json", "--hydrated", "--seed="},
		{"absent.json", "--node", "x", "--all-seeds"},
		{"absent.json", "--hydrated=false", "--all-seeds"},
		{"absent.json", "--hydrated", "--page"},
		{"absent.json", "--hydrated", "--cursor", "bad"},
		{"absent.json", "--hydrated", "--max-work", "-1"},
		{"absent.json", "--hydrated", "--max-page-bytes", "4095"},
		{"absent.json", "--hydrated", "--position-encoding", "guess"},
		{"absent.json", "--hydrated", "--node", strings.Repeat("x", 1025)},
	} {
		var out, err bytes.Buffer
		if code := runInspect(args, &out, &err); code == 0 || out.Len() != 0 || !strings.Contains(err.String(), "INVALID_INPUT") || strings.Contains(err.String(), "no such file") {
			t.Fatalf("PUBLIC_CLI_PREFLIGHT FAIL: %q code=%d out=%s err=%s", args, code, out.String(), err.String())
		}
	}
	t.Log("PUBLIC_CLI_PREFLIGHT PASS")
}
func TestHydratedCLIAcceptsLargeRegularArtifactWithoutWeakeningInlineTransport(t *testing.T) {
	raw, err := os.ReadFile("../../internal/hydratedevidence/testdata/focused-fr20.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) >= 1<<20+1 {
		t.Fatal("fixture unexpectedly exceeds padding target")
	}
	raw = append(raw, bytes.Repeat([]byte(" "), 1<<20+1-len(raw))...)
	path := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{{"--json"}, nil} {
		args := []string{path, "--hydrated", "--node", "unknown"}
		args = append(args, extra...)
		var stdout, stderr bytes.Buffer
		if code := runInspect(args, &stdout, &stderr); code != 0 || stdout.Len() == 0 {
			t.Fatalf("ASSERT_HYDRATED_CLI_LARGE_REGULAR_ARTIFACT: args=%v code=%d stderr=%s", args, code, stderr.String())
		}
	}
}

func TestHydratedCLIPrivatePathsDisabledByDefault(t *testing.T) {
	var out, stderr bytes.Buffer
	privateRoot := t.TempDir()
	args := []string{"artifact.json", "--hydrated", "--private-root", privateRoot, "--artifact-schema-id", "schema", "--artifact-digest", "sha256:" + strings.Repeat("a", 64), "--artifact-generation", "g-" + strings.Repeat("a", 64), "--artifact-byte-length", "2", "--node", "unknown", "--json"}
	if code := runInspect(args, &out, &stderr); code == 0 || out.Len() != 0 || !strings.Contains(stderr.String(), "PATH_DISABLED") || strings.Contains(stderr.String(), privateRoot) {
		t.Fatalf("ASSERT_HYDRATED_PRIVATE_PATH_DEFAULT_DISABLED: code=%d out=%s err=%s", code, out.String(), stderr.String())
	}
}

func TestHydratedCLIExplicitInputSafety(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.json")
	raw, err := os.ReadFile("../../internal/hydratedevidence/testdata/focused-fr20.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err = os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{dir, link} {
		var out, stderr bytes.Buffer
		if runInspect([]string{input, "--hydrated", "--node", "unknown", "--json"}, &out, &stderr) == 0 || out.Len() != 0 {
			t.Fatal("PUBLIC_CLI_INPUT_SAFETY FAIL", input)
		}
	}
	var out, stderr bytes.Buffer
	if runInspect([]string{source, "--hydrated", "--max-input-bytes", "100", "--json"}, &out, &stderr) == 0 || out.Len() != 0 {
		t.Fatal("PUBLIC_CLI_INPUT_SAFETY FAIL: byte limit")
	}
	t.Log("PUBLIC_CLI_INPUT_SAFETY PASS")
}
