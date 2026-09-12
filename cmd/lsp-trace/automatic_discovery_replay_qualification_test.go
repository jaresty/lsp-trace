package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestAutomaticDiscoveryRetainedSeedReplayQualification is an opt-in release
// qualification guard. It intentionally exercises only public CLI inputs and
// retained outputs so it can land independently of discovery filtering and
// capture-set production.
//
// Automatic discovery retains canonical lsp-trace.seeds.v2 bytes, whose
// unified replay input is --seed-file. --seed-manifest remains the legacy
// acquisition-manifest input. Discovery and replay must produce the same exact
// V5 graph while replay must not inherit discovery custody.
func TestAutomaticDiscoveryRetainedSeedReplayQualification(t *testing.T) {
	const assertion = "ASSERT_AUTOMATIC_DISCOVERY_EXACT_RETAINED_SEED_REPLAY_EQUIVALENCE"
	if os.Getenv("LSP_TRACE_QUALIFY_DISCOVERY_REPLAY") != "1" {
		t.Skip(`BLOCKED_UNIFIED_SEED_FILE_REPLAY_ADMISSION: set LSP_TRACE_QUALIFY_DISCOVERY_REPLAY=1; current RED is flag provided but not defined: -seed-file`)
	}
	if runtime.GOOS != "darwin" {
		t.Skip("managed process CLI uses Darwin supervisor")
	}

	cli := buildStartupSinkCLI(t)
	fake := filepath.Join(t.TempDir(), "fake-lsp")
	if out, err := exec.Command("go", "build", "-o", fake, "../fake-lsp").CombinedOutput(); err != nil {
		t.Fatalf("%s: build fake LSP: %v: %s", assertion, err, out)
	}
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("leaf\n\npeer\n\nNested\nhidden\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(startArgs []string, grouped bool, output string) ([]byte, []byte) {
		t.Helper()
		args := []string{"slice", "--production-v5", "--workspace", workspace, "--server", fake, "--server-env", "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=hierarchical", "--language-id", "go"}
		args = append(args, startArgs...)
		if grouped {
			args = append(args, "--output", output, "--group-by", "leiden", "--community-seed", "19", "--pagerank-top-k", "2", "--hub-top-k", "2")
		}
		cmd := exec.Command(cli, args...)
		cmd.Env = append(os.Environ(), "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL=hierarchical")
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s: args=%v: %v: stderr=%s", assertion, args, err, stderr.String())
		}
		expectedStderr := ""
		if len(startArgs) > 0 && startArgs[0] == "--from-file" {
			expectedStderr = "automatic discovery accounting: files_enumerated=1 files_selected=1 files_excluded=0 files_unsupported=0 files_document_supply_failed=0 files_document_symbol_failed=0 files_processed=1 files_incomplete=0 symbols_enumerated=5 symbols_selected=5 symbols_excluded=0 symbols_unsupported=0 symbols_preparation_failed=0 symbols_prepared=5 symbols_incomplete=0; operational census only; does not claim endpoint or source completeness\n"
		}
		if stderr.String() != expectedStderr {
			t.Fatalf("%s: unexpected stderr for args=%v: got=%q want=%q", assertion, args, stderr.String(), expectedStderr)
		}
		return stdout.Bytes(), readQualificationArtifact(t, grouped, output, stdout.Bytes())
	}

	discoveryOut, discoveryArtifactBytes := run([]string{"--from-file", "main.go"}, false, "")
	_ = discoveryOut
	var discovery qualificationV5Artifact
	if err := json.Unmarshal(discoveryArtifactBytes, &discovery); err != nil {
		t.Fatalf("%s: decode discovery V5: %v", assertion, err)
	}
	if discovery.SeedSpec == nil {
		t.Fatalf("ASSERT_DISCOVERY_RETAINS_CANONICAL_SEEDS_V2: seed_spec omitted")
	}
	retainedSeeds, err := base64.StdEncoding.DecodeString(discovery.SeedSpec.Bytes)
	if err != nil {
		t.Fatalf("ASSERT_DISCOVERY_RETAINS_CANONICAL_SEEDS_V2: %v", err)
	}
	manifestPath := filepath.Join(t.TempDir(), "retained-seeds.v2.json")
	if err := os.WriteFile(manifestPath, retainedSeeds, 0o600); err != nil {
		t.Fatal(err)
	}

	_, replayArtifactBytes := run([]string{"--seed-file", manifestPath}, false, "")
	assertEquivalentQualificationV5(t, discoveryArtifactBytes, replayArtifactBytes)

	discoveryGraph := filepath.Join(t.TempDir(), "discovery-v5.json")
	replayGraph := filepath.Join(t.TempDir(), "replay-v5.json")
	discoveryGrouped, discoveryGroupedArtifact := run([]string{"--from-file", "main.go"}, true, discoveryGraph)
	replayGrouped, replayGroupedArtifact := run([]string{"--seed-file", manifestPath}, true, replayGraph)
	assertEquivalentQualificationV5(t, discoveryGroupedArtifact, replayGroupedArtifact)
	if !bytes.Equal(discoveryGrouped, replayGrouped) {
		t.Fatalf("ASSERT_DISCOVERY_REPLAY_GROUPED_LEIDEN_EXACT: discovery=%q replay=%q", discoveryGrouped, replayGrouped)
	}
}

type qualificationV5Artifact struct {
	GraphV5       string `json:"graph_v5"`
	GraphV5SHA256 string `json:"graph_v5_sha256"`
	SeedSpec      *struct {
		Bytes string `json:"bytes"`
	} `json:"seed_spec"`
}

func readQualificationArtifact(t *testing.T, grouped bool, output string, stdout []byte) []byte {
	t.Helper()
	if !grouped {
		return append([]byte(nil), stdout...)
	}
	selectorRaw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var selector struct {
		Generation string `json:"generation"`
	}
	if err := json.Unmarshal(selectorRaw, &selector); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(output), selector.Generation, "artifact.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertEquivalentQualificationV5(t *testing.T, discoveryRaw, replayRaw []byte) {
	t.Helper()
	var discovery, replay qualificationV5Artifact
	if err := json.Unmarshal(discoveryRaw, &discovery); err != nil {
		t.Fatalf("ASSERT_DISCOVERY_V5_ARTIFACT_VALID: %v", err)
	}
	if err := json.Unmarshal(replayRaw, &replay); err != nil {
		t.Fatalf("ASSERT_REPLAY_V5_ARTIFACT_VALID: %v", err)
	}
	if replay.SeedSpec != nil {
		t.Fatalf("ASSERT_REPLAY_DOES_NOT_FORGE_DISCOVERY_CUSTODY: replay seed_spec=%+v", replay.SeedSpec)
	}
	if discovery.GraphV5SHA256 != replay.GraphV5SHA256 || discovery.GraphV5 != replay.GraphV5 {
		t.Fatalf("ASSERT_EXACT_TARGET_STATUS_GRAPH_V5_IDENTITY_AND_SERVER_CALLS: discovery_digest=%q replay_digest=%q", discovery.GraphV5SHA256, replay.GraphV5SHA256)
	}
	native, err := base64.StdEncoding.DecodeString(discovery.GraphV5)
	if err != nil {
		t.Fatal(err)
	}
	var graph struct {
		EvidenceSemantics struct {
			CallEdges struct {
				EvidenceClass string `json:"evidence_class"`
			} `json:"call_edges"`
		} `json:"evidence_semantics"`
	}
	if err := json.Unmarshal(native, &graph); err != nil {
		t.Fatalf("ASSERT_NATIVE_GRAPH_V5_VALID: %v", err)
	}
	if graph.EvidenceSemantics.CallEdges.EvidenceClass != "SERVER_REPORTED_CALL_HIERARCHY" {
		t.Fatalf("ASSERT_CALLS_REMAIN_SERVER_ONLY: evidence_class=%q", graph.EvidenceSemantics.CallEdges.EvidenceClass)
	}
}
