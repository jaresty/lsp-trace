package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
)

type fakeMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type fakeItem struct {
	Name           string          `json:"name"`
	Kind           int             `json:"kind"`
	URI            string          `json:"uri"`
	Range          fakeRange       `json:"range"`
	SelectionRange fakeRange       `json:"selectionRange"`
	Data           json.RawMessage `json:"data,omitempty"`
}
type fakePosition struct{ Line, Character uint32 }
type fakeRange struct{ Start, End fakePosition }

func TestFakeLanguageServerProcess(t *testing.T) {
	if os.Getenv("LSP_TRACE_FAKE_SERVER") == "" {
		return
	}
	if err := serveFake(os.Getenv("LSP_TRACE_FAKE_SCENARIO"), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	os.Exit(0)
}

func TestSubprocessLifecycleAndHierarchyShapes(t *testing.T) {
	for _, scenario := range []string{"linear", "branch", "diamond", "cycle"} {
		t.Run(scenario, func(t *testing.T) {
			result, code := executeFake(t, scenario, 500*time.Millisecond)
			if code != 0 || !result.Summary.Complete {
				t.Fatalf("ASSERT_SUBPROCESS_%s_COMPLETE: code=%d summary=%#v diagnostics=%#v", strings.ToUpper(scenario), code, result.Summary, result.Diagnostics)
			}
			want := map[string][2]int{"linear": {2, 1}, "branch": {3, 2}, "diamond": {4, 4}, "cycle": {2, 2}}[scenario]
			if result.Summary.NodeCount != want[0] || result.Summary.EdgeCount != want[1] {
				t.Fatalf("ASSERT_SUBPROCESS_%s_SHAPE: nodes=%d edges=%d want=%d/%d", strings.ToUpper(scenario), result.Summary.NodeCount, result.Summary.EdgeCount, want[0], want[1])
			}
		})
	}
}

func TestSubprocessSliceRelaysServerStderrWhenInitializeEndsWithEOF(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice-stderr-exit-initialize", "--from-file", "main.go", "--down-depth", "1", "--up-depth", "1", "--request-timeout", "5s", "--timeout", "10s"}
	stdout, stderr, code := captureRun(t, args)
	const causal = "workspace requires Elixir ~> 1.16.2"
	if code != 1 || stdout != "" || strings.Count(stderr, causal) != 1 || strings.Count(stderr, "EOF") != 1 {
		t.Fatalf("ASSERT_SLICE_INITIALIZE_SERVER_STDERR_RELAY: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestSubprocessSliceRetainsServerStderrOnOutgoingFailure(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice-outgoing-error", "--from-file", "main.go", "--down-depth", "1", "--up-depth", "1", "--request-timeout", "500ms", "--timeout", "2s"}
	stdout, stderr, _ := captureRun(t, args)
	const causal = "outgoing worker compilation failed"
	var got struct {
		Diagnostics []graph.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("ASSERT_SLICE_OUTGOING_STDERR_JSON: %v stdout=%q stderr=%q", err, stdout, stderr)
	}
	found := false
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Phase == "server-stderr" && strings.Contains(diagnostic.Message, causal) {
			found = true
		}
	}
	if !found || strings.Count(stderr, causal) != 1 {
		t.Fatalf("ASSERT_SLICE_OUTGOING_SERVER_STDERR_RETAINED_RELAYED: diagnostics=%#v stderr=%q", got.Diagnostics, stderr)
	}
}

func TestSubprocessSliceComposesOutgoingFrontierWithIncomingTraversal(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice", "--from-file", "main.go", "--down-depth", "1", "--up-depth", "2", "--request-timeout", "500ms", "--timeout", "2s"}
	stdout, stderr, code := captureRun(t, args)
	var got struct {
		Nodes []graph.Node        `json:"nodes"`
		Edges []graph.Edge        `json:"edges"`
		Slice graph.SliceEvidence `json:"slice"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("ASSERT_SLICE_COMMAND_JSON: %v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if code != 0 || len(got.Nodes) != 3 || len(got.Edges) != 2 || len(got.Slice.Layers) != 2 || len(got.Slice.FrontierNodeIDs) != 1 || len(got.Slice.OutgoingRelationIDs) != 1 {
		t.Fatalf("ASSERT_SLICE_COMMAND_COMPOSES_DIRECTIONS: code=%d nodes=%d edges=%d slice=%#v stderr=%s", code, len(got.Nodes), len(got.Edges), got.Slice, stderr)
	}
}

func TestSubprocessSliceStartsUpwardFromEarlyLeaves(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, scenario string) (int, struct {
		Nodes []graph.Node       `json:"nodes"`
		Seeds []graph.SeedResult `json:"seeds"`
		Slice struct {
			FrontierNodeIDs         []string `json:"frontier_node_ids"`
			OutgoingTerminalNodeIDs []string `json:"outgoing_terminal_node_ids"`
			UpwardStartNodeIDs      []string `json:"upward_start_node_ids"`
		} `json:"slice"`
	}) {
		t.Helper()
		args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=" + scenario, "--from-file", "main.go", "--down-depth", "2", "--up-depth", "1", "--request-timeout", "500ms", "--timeout", "2s"}
		stdout, stderr, code := captureRun(t, args)
		var got struct {
			Nodes []graph.Node       `json:"nodes"`
			Seeds []graph.SeedResult `json:"seeds"`
			Slice struct {
				FrontierNodeIDs         []string `json:"frontier_node_ids"`
				OutgoingTerminalNodeIDs []string `json:"outgoing_terminal_node_ids"`
				UpwardStartNodeIDs      []string `json:"upward_start_node_ids"`
			} `json:"slice"`
		}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("ASSERT_SLICE_EARLY_LEAF_JSON: %v stdout=%q stderr=%q", err, stdout, stderr)
		}
		return code, got
	}

	t.Run("leaf-recovers-callers", func(t *testing.T) {
		code, got := run(t, "slice-leaf")
		if code != 2 || len(got.Nodes) != 5 || len(got.Slice.FrontierNodeIDs) != 0 || len(got.Slice.OutgoingTerminalNodeIDs) != 1 || len(got.Slice.UpwardStartNodeIDs) != 1 {
			t.Fatalf("ASSERT_SLICE_EARLY_LEAF_UPWARD_START: code=%d nodes=%d slice=%#v", code, len(got.Nodes), got.Slice)
		}
		if len(got.Seeds) != 1 || len(got.Seeds[0].ReachedNodeIDs) != 5 || len(got.Seeds[0].ReachedRelationIDs) != 4 {
			t.Fatalf("ASSERT_SLICE_SINGLE_SEED_MEMBERSHIP_POPULATED: %#v", got.Seeds)
		}
	})
	t.Run("shallow-and-deep-deduplicate-shared-caller", func(t *testing.T) {
		code, got := run(t, "slice-shallow-deep")
		if code != 2 || len(got.Nodes) != 5 || len(got.Slice.FrontierNodeIDs) != 1 || len(got.Slice.OutgoingTerminalNodeIDs) != 1 || len(got.Slice.UpwardStartNodeIDs) != 2 {
			t.Fatalf("ASSERT_SLICE_SHALLOW_DEEP_UPWARD_UNION: code=%d nodes=%d slice=%#v", code, len(got.Nodes), got.Slice)
		}
		if got.Slice.UpwardStartNodeIDs[0] >= got.Slice.UpwardStartNodeIDs[1] {
			t.Fatalf("ASSERT_SLICE_UPWARD_START_NATIVE_ID_ORDER: %#v", got.Slice.UpwardStartNodeIDs)
		}
	})
	t.Run("outgoing-error-is-not-leaf", func(t *testing.T) {
		code, got := run(t, "slice-outgoing-error")
		if len(got.Slice.OutgoingTerminalNodeIDs) != 0 || len(got.Slice.UpwardStartNodeIDs) != 0 {
			t.Fatalf("ASSERT_SLICE_OUTGOING_FAILURE_NOT_LEAF: code=%d slice=%#v", code, got.Slice)
		}
	})
}

func TestSubprocessSlicePopulatesPerSeedCausalMemberships(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, scenario string) (string, struct {
		Nodes []graph.Node       `json:"nodes"`
		Edges []graph.Edge       `json:"edges"`
		Seeds []graph.SeedResult `json:"seeds"`
	}) {
		t.Helper()
		args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=" + scenario, "--at", "main.go:1:1", "--at", "main.go:2:1", "--down-depth", "1", "--up-depth", "1", "--request-timeout", "500ms", "--timeout", "2s"}
		stdout, stderr, code := captureRun(t, args)
		if code != 0 && code != 2 {
			t.Fatalf("ASSERT_SLICE_MEMBERSHIP_EXECUTION: code=%d stderr=%q", code, stderr)
		}
		var got struct {
			Nodes []graph.Node       `json:"nodes"`
			Edges []graph.Edge       `json:"edges"`
			Seeds []graph.SeedResult `json:"seeds"`
		}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("ASSERT_SLICE_MEMBERSHIP_JSON: %v stdout=%q stderr=%q", err, stdout, stderr)
		}
		return stdout, got
	}

	t.Run("disconnected", func(t *testing.T) {
		first, got := run(t, "slice-membership-disconnected")
		second, _ := run(t, "slice-membership-disconnected")
		if first != second {
			t.Fatalf("ASSERT_SLICE_MEMBERSHIP_DETERMINISTIC_BYTES")
		}
		if len(got.Seeds) != 2 || len(got.Nodes) != 4 || len(got.Edges) != 2 || len(got.Seeds[0].ReachedNodeIDs) != 2 || len(got.Seeds[1].ReachedNodeIDs) != 2 || len(got.Seeds[0].ReachedRelationIDs) != 1 || len(got.Seeds[1].ReachedRelationIDs) != 1 {
			t.Fatalf("ASSERT_SLICE_DISCONNECTED_SEED_MEMBERSHIPS: %#v", got)
		}
		if got.Seeds[0].ReachedNodeIDs[0] == got.Seeds[1].ReachedNodeIDs[0] || got.Seeds[0].ReachedRelationIDs[0] == got.Seeds[1].ReachedRelationIDs[0] {
			t.Fatalf("ASSERT_SLICE_DISCONNECTED_MEMBERSHIPS_DISJOINT: %#v", got.Seeds)
		}
	})
	t.Run("converging-and-upward", func(t *testing.T) {
		_, got := run(t, "slice-membership-converging")
		if len(got.Seeds) != 2 || len(got.Nodes) != 4 || len(got.Edges) != 3 {
			t.Fatalf("ASSERT_SLICE_CONVERGING_UNION_SHAPE: %#v", got)
		}
		for _, seed := range got.Seeds {
			if len(seed.ReachedNodeIDs) != 3 || len(seed.ReachedRelationIDs) != 2 {
				t.Fatalf("ASSERT_SLICE_CONVERGING_SHARED_UPWARD_MEMBERSHIP: %#v", got.Seeds)
			}
		}
	})
	t.Run("failed-seed", func(t *testing.T) {
		_, got := run(t, "slice-membership-failed")
		if len(got.Seeds) != 2 || got.Seeds[0].Failure != nil || len(got.Seeds[0].ReachedNodeIDs) == 0 || got.Seeds[1].Failure == nil || len(got.Seeds[1].ReachedNodeIDs) != 0 || len(got.Seeds[1].ReachedRelationIDs) != 0 {
			t.Fatalf("ASSERT_SLICE_FAILED_SEED_EMPTY_MEMBERSHIP: %#v", got.Seeds)
		}
	})
}

func TestWriteSliceSeedFailuresReportsEveryFailureOnly(t *testing.T) {
	var out strings.Builder
	writeSliceSeedFailures(&out, []graph.SeedResult{
		{Label: "ok"},
		{Label: "first", Failure: &graph.SeedFailure{Phase: "slice-prepare", Message: "no item"}},
		{Label: "second", Failure: &graph.SeedFailure{Phase: "source", Message: "missing file"}},
	})
	const want = "slice seed \"first\" failed during slice-prepare: no item\nslice seed \"second\" failed during source: missing file\n"
	if out.String() != want {
		t.Fatalf("ASSERT_SLICE_EVERY_FAILED_SEED_ONLY: want=%q got=%q", want, out.String())
	}
}

func TestSubprocessSliceReportsFailedSeedLabelAndCause(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\nfunc second() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice-membership-failed", "--at", "main.go:1:1", "--at", "main.go:2:1", "--down-depth", "1", "--up-depth", "1", "--request-timeout", "500ms", "--timeout", "2s"}
	_, stderr, code := captureRun(t, args)
	if code != 2 {
		t.Fatalf("ASSERT_SLICE_FAILED_SEED_REPORT_EXIT: code=%d stderr=%q", code, stderr)
	}
	const want = "slice seed \"seed-2\" failed during slice-prepare: json-rpc error -32004: seed-b prepare failed\n"
	if strings.Count(stderr, want) != 1 {
		t.Fatalf("ASSERT_SLICE_FAILED_SEED_LABEL_AND_CAUSE: want=%q stderr=%q", want, stderr)
	}
}

func TestSubprocessSliceSummarizesNoisyDiagnosticsWithoutDroppingEvidence(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice-noisy", "--from-file", "main.go", "--down-depth", "1", "--up-depth", "1", "--request-timeout", "500ms", "--timeout", "2s"}
	stdout, stderr, code := captureRun(t, args)
	var got struct {
		Diagnostics []graph.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("ASSERT_SLICE_NOISY_JSON: %v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if code != 2 {
		t.Fatalf("ASSERT_SLICE_NOISY_EXIT_PRESERVED: code=%d stderr=%q", code, stderr)
	}
	if strings.Count(stderr, "slice-prepare: skipped 2 non-callable document symbols") != 1 {
		t.Errorf("ASSERT_SLICE_NONCALLABLE_STDERR_SUMMARY: stderr=%q", stderr)
	}
	if strings.Count(stderr, "traverse: SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE (2 occurrences)") != 1 {
		t.Errorf("ASSERT_SLICE_RANGE_STDERR_SUMMARY: stderr=%q", stderr)
	}
	if len(got.Diagnostics) != 4 {
		t.Fatalf("ASSERT_SLICE_DIAGNOSTIC_EVIDENCE_RETAINED: diagnostics=%#v", got.Diagnostics)
	}
}

func TestSubprocessSlicePublishesAttributableOutsideCallerRanges(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\nfunc second() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	selector := filepath.Join(t.TempDir(), "selector.json")
	args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice-range-warning", "--at", "main.go:1:1", "--at", "main.go:2:1", "--down-depth", "1", "--up-depth", "2", "--request-timeout", "500ms", "--timeout", "2s", "--output", selector}
	stdout, stderr, code := captureRun(t, args)
	if code != 0 || stdout != "" {
		t.Fatalf("ASSERT_SLICE_RANGE_WARNING_PUBLISH_EXIT: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	artifact, err := readSelectedArtifact(selector)
	if err != nil {
		t.Fatalf("ASSERT_SLICE_RANGE_WARNING_SELECTOR_PUBLISHED: %v", err)
	}
	if _, err := schema.Validate(artifact, "v3"); err != nil {
		t.Fatalf("ASSERT_SLICE_RANGE_WARNING_SCHEMA_VALID: %v", err)
	}
	var got struct {
		Diagnostics []graph.Diagnostic `json:"diagnostics"`
		Edges       []graph.Edge       `json:"edges"`
		Seeds       []graph.SeedResult `json:"seeds"`
		Summary     struct {
			TraversalComplete   bool   `json:"traversal_complete"`
			SourceGraphComplete string `json:"source_graph_complete"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(artifact, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Edges) != 2 || len(got.Diagnostics) != 2 || len(got.Seeds) != 2 || !got.Summary.TraversalComplete || got.Summary.SourceGraphComplete != graph.Unknown {
		t.Fatalf("ASSERT_SLICE_RANGE_WARNING_EVIDENCE_RETAINED: edges=%#v diagnostics=%#v seeds=%#v summary=%#v", got.Edges, got.Diagnostics, got.Seeds, got.Summary)
	}
	globalRelations := map[string]bool{}
	for _, edge := range got.Edges {
		globalRelations[edge.RelationID] = true
	}
	reachedRelations := map[string]bool{}
	for _, seed := range got.Seeds {
		if seed.Failure != nil || len(seed.ReachedRelationIDs) != 1 {
			t.Fatalf("ASSERT_SLICE_RANGE_WARNING_SEED_MEMBERSHIP: %#v", got.Seeds)
		}
		for _, relationID := range seed.ReachedRelationIDs {
			reachedRelations[relationID] = true
		}
	}
	if len(globalRelations) != len(reachedRelations) {
		t.Fatalf("ASSERT_SLICE_RANGE_WARNING_RELATION_UNION: global=%#v reached=%#v", globalRelations, reachedRelations)
	}
	if strings.Count(stderr, "traverse: SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE (2 occurrences)") != 1 {
		t.Fatalf("ASSERT_SLICE_RANGE_WARNING_DIAGNOSTIC_SUMMARY: %q", stderr)
	}
	verifyOut, verifyErr, verifyCode := captureRun(t, []string{"verify", selector})
	if verifyCode != 0 || verifyOut != "verified integrity and custody\n" || verifyErr != "" {
		t.Fatalf("ASSERT_SLICE_RANGE_WARNING_VERIFY: code=%d stdout=%q stderr=%q", verifyCode, verifyOut, verifyErr)
	}
	stdoutArgs := args[:len(args)-2]
	first, firstErr, firstCode := captureRun(t, stdoutArgs)
	second, secondErr, secondCode := captureRun(t, stdoutArgs)
	if firstCode != 0 || secondCode != 0 || first != second || firstErr != secondErr {
		t.Fatalf("ASSERT_SLICE_RANGE_WARNING_DETERMINISTIC_REPLAY: firstCode=%d secondCode=%d bytesEqual=%v firstErr=%q secondErr=%q", firstCode, secondCode, first == second, firstErr, secondErr)
	}
}

func TestSubprocessSliceRejectsUnattributableIncomingRelation(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	selector := filepath.Join(t.TempDir(), "selector.json")
	args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice-range-unattributable", "--at", "main.go:1:1", "--down-depth", "1", "--up-depth", "2", "--request-timeout", "500ms", "--timeout", "2s", "--output", selector}
	stdout, stderr, code := captureRun(t, args)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "dangling boundary node id") {
		t.Fatalf("ASSERT_SLICE_UNATTRIBUTABLE_RELATION_REJECTED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(selector); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_SLICE_UNATTRIBUTABLE_SELECTOR_ABSENT: err=%v", err)
	}
}

func TestSubprocessSliceExplicitStartModesPreserveSeedLabels(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	seedFile := filepath.Join(t.TempDir(), "seeds.json")
	if err := os.WriteFile(seedFile, []byte(`{"seeds":[{"label":"named-start","at":"main.go:1:1"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, startMode string
		mode            []string
		label           string
	}{
		{name: "at", startMode: "at", mode: []string{"--at", "main.go:1:1"}, label: "seed-1"},
		{name: "seed-file", startMode: "seed_file", mode: []string{"--seed-file", seedFile}, label: "named-start"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"slice", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=slice", "--down-depth", "1", "--up-depth", "2", "--request-timeout", "500ms", "--timeout", "2s"}
			args = append(args, tc.mode...)
			stdout, stderr, code := captureRun(t, args)
			var got struct {
				Invocation struct {
					Seeds []graph.InvocationSeed `json:"seeds"`
				} `json:"invocation"`
				Seeds []struct {
					Label    string   `json:"label"`
					Prepared []string `json:"prepared_target_ids"`
				} `json:"seeds"`
				Slice graph.SliceEvidence `json:"slice"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("ASSERT_SLICE_EXPLICIT_JSON: %v stdout=%q stderr=%q", err, stdout, stderr)
			}
			if code != 0 || len(got.Invocation.Seeds) != 1 || got.Invocation.Seeds[0].Label != tc.label || len(got.Seeds) != 1 || got.Seeds[0].Label != tc.label || len(got.Seeds[0].Prepared) != 1 || len(got.Slice.StartingNodeIDs) != 1 || got.Slice.StartMode != tc.startMode {
				t.Fatalf("ASSERT_SLICE_EXPLICIT_SEED_PROVENANCE: code=%d invocation=%#v seeds=%#v slice=%#v stderr=%s", code, got.Invocation.Seeds, got.Seeds, got.Slice, stderr)
			}
		})
	}
}

func TestSubprocessLabeledMultipleSeeds(t *testing.T) {
	for _, scenario := range []string{"multi-two", "multi-duplicate", "multi-mixed"} {
		t.Run(scenario, func(t *testing.T) {
			workspace := t.TempDir()
			if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
				t.Fatal(err)
			}
			seedFile := filepath.Join(t.TempDir(), "seeds.json")
			body := `{"seeds":[{"label":"interface","at":"main.go:1:1"},{"label":"implementation","at":"main.go:2:1"}]}`
			if err := os.WriteFile(seedFile, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"incoming", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=" + scenario, "--seed-file", seedFile, "--request-timeout", "500ms", "--timeout", "1s"}
			stdout, stderr, code := captureRun(t, args)
			if code != 0 && scenario != "multi-mixed" {
				t.Fatalf("ASSERT_REPEATABLE_AT_ACCEPTED: code=%d stderr=%s stdout=%s", code, stderr, stdout)
			}
			var got struct {
				Seeds []struct {
					Label     string       `json:"label"`
					Requested graph.Target `json:"requested_position"`
					Prepared  []string     `json:"prepared_target_ids"`
					Reached   []string     `json:"reached_node_ids"`
					Failure   any          `json:"failure"`
				} `json:"seeds"`
				Nodes   []graph.Node `json:"nodes"`
				Summary struct {
					TraversalComplete bool `json:"traversal_complete"`
				} `json:"summary"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("ASSERT_V2_SEED_PROVENANCE: %v stdout=%q stderr=%q", err, stdout, stderr)
			}
			byLabel := map[string]struct {
				Requested graph.Target
				Prepared  []string
				Reached   []string
				Failure   any
			}{}
			for _, seed := range got.Seeds {
				byLabel[seed.Label] = struct {
					Requested graph.Target
					Prepared  []string
					Reached   []string
					Failure   any
				}{seed.Requested, seed.Prepared, seed.Reached, seed.Failure}
			}
			_, hasInterface := byLabel["interface"]
			_, hasImplementation := byLabel["implementation"]
			if len(got.Seeds) != 2 || !hasInterface || !hasImplementation || byLabel["interface"].Requested.Line != 1 || len(byLabel["implementation"].Prepared) == 0 {
				t.Fatalf("ASSERT_V2_SEED_PROVENANCE: %#v", got.Seeds)
			}
			switch scenario {
			case "multi-duplicate":
				left, right := byLabel["interface"].Reached, byLabel["implementation"].Reached
				if len(got.Nodes) != 1 || len(left) != 1 || len(right) != 1 || left[0] != right[0] {
					t.Fatalf("ASSERT_DUPLICATE_SEEDS_COLLAPSE: nodes=%#v seeds=%#v", got.Nodes, got.Seeds)
				}
			case "multi-mixed":
				if code != 2 || got.Summary.TraversalComplete || len(got.Nodes) == 0 || byLabel["interface"].Failure == nil || len(byLabel["implementation"].Reached) == 0 {
					t.Fatalf("ASSERT_FAILED_SEED_INCOMPLETE_WITH_GRAPH: code=%d result=%#v stderr=%s", code, got, stderr)
				}
			default:
				if !got.Summary.TraversalComplete || len(got.Nodes) != 2 {
					t.Fatalf("ASSERT_FAILED_SEED_CONTINUES: %#v", got)
				}
			}
		})
	}
}

func TestSubprocessOpaqueDataAndShuffledOrdering(t *testing.T) {
	a, codeA := executeFake(t, "shuffle-forward", 500*time.Millisecond)
	b, codeB := executeFake(t, "shuffle-reverse", 500*time.Millisecond)
	if codeA != 0 || codeB != 0 {
		t.Fatalf("ASSERT_SUBPROCESS_SHUFFLE_SUCCESS: codes=%d/%d", codeA, codeB)
	}
	normalizedInvocation := func(result graph.Result) graph.Invocation {
		seeds := make([]graph.InvocationSeed, 0, len(result.Seeds))
		for _, seed := range result.Seeds {
			seeds = append(seeds, graph.InvocationSeed{Label: seed.Label, At: seed.Label + ":1:1"})
		}
		return graph.Invocation{Seeds: seeds}
	}
	a.Invocation, b.Invocation = normalizedInvocation(a), normalizedInvocation(b)
	for i := range a.Seeds {
		a.Seeds[i].Requested = graph.Target{}
	}
	for i := range b.Seeds {
		b.Seeds[i].Requested = graph.Target{}
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("ASSERT_SUBPROCESS_SHUFFLED_CANONICAL: forward=%s reverse=%s", ja, jb)
	}
	if !strings.Contains(string(ja), `"opaque":{"token":[1,"two"]}`) {
		t.Fatalf("ASSERT_SUBPROCESS_OPAQUE_DATA: %s", ja)
	}
}

func TestSubprocessFailureTraffic(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		code    int
		reason  graph.Reason
		phase   string
	}{
		{"null", 2 * time.Second, 0, graph.PrepareReturnedNoItem, ""},
		{"error", 2 * time.Second, 2, graph.ServerError, "traverse"},
		{"delay", 20 * time.Millisecond, 2, graph.RequestTimeout, ""},
		{"delay-initialize", 20 * time.Millisecond, 2, "", "initialize"},
		{"malformed", 2 * time.Second, 1, "", "initialize"},
		{"exit-early", 2 * time.Second, 1, "", "initialize"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, code := executeFake(t, tt.name, tt.timeout)
			if code != tt.code {
				t.Fatalf("ASSERT_SUBPROCESS_%s_EXIT: got=%d want=%d result=%#v", strings.ToUpper(tt.name), code, tt.code, r)
			}
			if tt.reason != "" && (len(r.Terminals) == 0 || r.Terminals[0].Reason != tt.reason) {
				t.Fatalf("ASSERT_SUBPROCESS_%s_REASON: %#v", strings.ToUpper(tt.name), r.Terminals)
			}
			if tt.phase != "" && (len(r.Diagnostics) == 0 || r.Diagnostics[0].Phase != tt.phase) {
				t.Fatalf("ASSERT_SUBPROCESS_%s_PHASE: %#v", strings.ToUpper(tt.name), r.Diagnostics)
			}
		})
	}
}

func executeFake(t *testing.T, scenario string, requestTimeout time.Duration) (graph.Result, int) {
	t.Helper()
	workspace := t.TempDir()
	target := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(target, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config{workspace: workspace, command: os.Args[0], at: "main.go:1:1", requestTimeout: requestTimeout, timeout: time.Second, maxDepth: 100, maxNodes: 100}
	cfg.args = []string{"-test.run=^TestFakeLanguageServerProcess$"}
	cfg.env = []string{"LSP_TRACE_FAKE_SERVER=1", "LSP_TRACE_FAKE_SCENARIO=" + scenario}
	return execute(context.Background(), cfg)
}

func serveFake(scenario string, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	for {
		m, err := readFake(r)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch m.Method {
		case "initialize":
			if scenario == "slice-stderr-exit-initialize" {
				fmt.Fprintln(os.Stderr, "workspace requires Elixir ~> 1.16.2")
				return nil
			}
			if scenario == "delay-initialize" {
				time.Sleep(80 * time.Millisecond)
			}
			if scenario == "exit-early" {
				return nil
			}
			if scenario == "malformed" {
				_, err = fmt.Fprint(out, "Content-Length: 5\r\n\r\n{")
				return err
			}
			capabilities := map[string]any{"callHierarchyProvider": true}
			if strings.HasPrefix(scenario, "slice") {
				capabilities["documentSymbolProvider"] = true
			}
			err = writeFake(out, m.ID, map[string]any{"capabilities": capabilities}, nil)
		case "initialized", "textDocument/didOpen":
		case "textDocument/documentSymbol":
			if scenario == "slice-noisy" {
				err = writeFake(out, m.ID, []fakeItem{item("start", 0), item("value-a", 1), item("value-b", 2)}, nil)
			} else if strings.HasPrefix(scenario, "slice") {
				err = writeFake(out, m.ID, []fakeItem{item("start", 0)}, nil)
			}
		case "textDocument/prepareCallHierarchy":
			if scenario == "null" {
				err = writeFake(out, m.ID, nil, nil)
				break
			}
			var p struct {
				Position fakePosition `json:"position"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if scenario == "multi-mixed" && p.Position.Line == 0 {
				err = writeFake(out, m.ID, nil, map[string]any{"code": -32002, "message": "seed prepare failure"})
				break
			}
			if scenario == "slice-noisy" && p.Position.Line > 0 {
				err = writeFake(out, m.ID, nil, map[string]any{"code": 0, "message": fmt.Sprintf("value-%c is not a function", 'a'+p.Position.Line-1)})
				break
			}
			if scenario == "slice-membership-failed" && p.Position.Line == 1 {
				err = writeFake(out, m.ID, nil, map[string]any{"code": -32004, "message": "seed-b prepare failed"})
				break
			}
			line := p.Position.Line
			name := "leaf"
			if strings.HasPrefix(scenario, "slice-membership") || strings.HasPrefix(scenario, "slice-range-") {
				name = fmt.Sprintf("seed-%c", 'a'+p.Position.Line)
			} else if strings.HasPrefix(scenario, "slice") {
				name = "start"
			}
			if scenario == "multi-two" && line > 0 {
				name = "second"
			}
			if scenario == "multi-duplicate" {
				line = 0
			}
			err = writeFake(out, m.ID, []fakeItem{item(name, line)}, nil)
		case "callHierarchy/outgoingCalls":
			var p struct {
				Item fakeItem `json:"item"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if scenario == "slice-outgoing-error" {
				fmt.Fprintln(os.Stderr, "outgoing worker compilation failed")
				err = writeFake(out, m.ID, nil, map[string]any{"code": -32003, "message": "outgoing failed"})
				break
			}
			calls := []map[string]any{}
			if (scenario == "slice" || scenario == "slice-noisy") && p.Item.Name == "start" {
				calls = append(calls, map[string]any{"to": item("leaf", 1), "fromRanges": []fakeRange{{Start: fakePosition{Line: 0, Character: 5}, End: fakePosition{Line: 0, Character: 9}}}})
			}
			if scenario == "slice-membership-disconnected" || scenario == "slice-membership-failed" {
				if p.Item.Name == "seed-a" || p.Item.Name == "seed-b" {
					calls = append(calls, map[string]any{"to": item("leaf-"+p.Item.Name[5:], p.Item.Range.Start.Line+2), "fromRanges": []fakeRange{}})
				}
			}
			if scenario == "slice-membership-converging" && (p.Item.Name == "seed-a" || p.Item.Name == "seed-b") {
				calls = append(calls, map[string]any{"to": item("shared", 3), "fromRanges": []fakeRange{}})
			}
			if scenario == "slice-shallow-deep" {
				switch p.Item.Name {
				case "start":
					calls = append(calls,
						map[string]any{"to": item("shallow", 1), "fromRanges": []fakeRange{}},
						map[string]any{"to": item("deep-1", 2), "fromRanges": []fakeRange{}},
					)
				case "deep-1":
					calls = append(calls, map[string]any{"to": item("deep-2", 3), "fromRanges": []fakeRange{}})
				}
			}
			err = writeFake(out, m.ID, calls, nil)
		case "callHierarchy/incomingCalls":
			var p struct {
				Item fakeItem `json:"item"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if scenario == "delay" {
				time.Sleep(80 * time.Millisecond)
			}
			if scenario == "error" {
				err = writeFake(out, m.ID, nil, map[string]any{"code": -32001, "message": "fake failure"})
				break
			}
			err = writeFake(out, m.ID, incoming(scenario, p.Item), nil)
		case "shutdown":
			err = writeFake(out, m.ID, nil, nil)
		case "exit":
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func item(name string, line uint32) fakeItem {
	r := fakeRange{Start: fakePosition{Line: line}, End: fakePosition{Line: line, Character: 20}}
	selection := fakeRange{Start: fakePosition{Line: line}, End: fakePosition{Line: line, Character: 4}}
	return fakeItem{Name: name, Kind: 12, URI: "file:///workspace/main.go", Range: r, SelectionRange: selection, Data: json.RawMessage(`{"opaque":{"token":[1,"two"]},"name":"` + name + `"}`)}
}
func incoming(scenario string, it fakeItem) []map[string]any {
	call := func(from fakeItem) map[string]any {
		return map[string]any{"from": from, "fromRanges": []fakeRange{{Start: fakePosition{Line: from.Range.Start.Line, Character: 5}, End: fakePosition{Line: from.Range.Start.Line, Character: 9}}}}
	}
	switch scenario {
	case "slice":
		if it.Name == "leaf" {
			return []map[string]any{call(item("root", 2))}
		}
	case "slice-noisy":
		if it.Name == "leaf" {
			outside := []fakeRange{{Start: fakePosition{Line: 99}, End: fakePosition{Line: 99, Character: 1}}}
			return []map[string]any{{"from": item("root-a", 2), "fromRanges": outside}, {"from": item("root-b", 3), "fromRanges": outside}}
		}
	case "slice-range-warning":
		if strings.HasPrefix(it.Name, "seed-") {
			outside := []fakeRange{{Start: fakePosition{Line: it.Range.Start.Line + 20}, End: fakePosition{Line: it.Range.Start.Line + 20, Character: 4}}}
			return []map[string]any{{"from": item("caller-"+it.Name[5:], it.Range.Start.Line+2), "fromRanges": outside}}
		}
	case "slice-range-unattributable":
		if strings.HasPrefix(it.Name, "seed-") {
			return []map[string]any{{"from": item("", it.Range.Start.Line+2), "fromRanges": []fakeRange{}}}
		}
	case "slice-leaf":
		if it.Name == "start" {
			return []map[string]any{call(item("caller-a", 1)), call(item("caller-b", 2)), call(item("caller-c", 3)), call(item("caller-d", 4))}
		}
	case "slice-shallow-deep":
		if it.Name == "shallow" || it.Name == "deep-2" {
			return []map[string]any{call(item("shared-caller", 4))}
		}
	case "slice-membership-converging":
		if it.Name == "shared" {
			return []map[string]any{call(item("shared-caller", 4))}
		}
	case "linear":
		if it.Name == "leaf" {
			return []map[string]any{call(item("root", 1))}
		}
	case "branch":
		if it.Name == "leaf" {
			return []map[string]any{call(item("a", 1)), call(item("b", 2))}
		}
	case "diamond", "shuffle-forward", "shuffle-reverse":
		if it.Name == "leaf" {
			x := []map[string]any{call(item("a", 1)), call(item("b", 2))}
			if scenario == "shuffle-reverse" {
				x[0], x[1] = x[1], x[0]
			}
			return x
		}
		if it.Name == "a" || it.Name == "b" {
			return []map[string]any{call(item("root", 3))}
		}
	case "cycle":
		if it.Name == "leaf" {
			return []map[string]any{call(item("root", 1))}
		}
		if it.Name == "root" {
			return []map[string]any{call(item("leaf", 0))}
		}
	}
	return []map[string]any{}
}
func writeFake(w io.Writer, id json.RawMessage, result any, rpcErr any) error {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result, "error": rpcErr})
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}
func readFake(r *bufio.Reader) (fakeMessage, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return fakeMessage{}, err
	}
	if !strings.HasPrefix(strings.ToLower(line), "content-length:") {
		return fakeMessage{}, fmt.Errorf("bad header %q", line)
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Content-Length:")))
	if err != nil {
		return fakeMessage{}, err
	}
	if line, err = r.ReadString('\n'); err != nil || strings.TrimSpace(line) != "" {
		return fakeMessage{}, fmt.Errorf("bad separator")
	}
	body := make([]byte, n)
	if _, err = io.ReadFull(r, body); err != nil {
		return fakeMessage{}, err
	}
	var m fakeMessage
	err = json.Unmarshal(body, &m)
	return m, err
}
