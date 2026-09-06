package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/observationadapter"
)

func TestProductionMCPB05TwentyFourAttemptQualification(t *testing.T) {
	const assertion = "ASSERT_B05_FRAME6_REAL_MCP_TWENTY_FOUR_ATTEMPTS"
	provider := os.Getenv("LSP_TRACE_EXTERNAL_PROVIDER_PATH")
	if provider == "" {
		t.Skip("LSP_TRACE_EXTERNAL_PROVIDER_PATH is required")
	}
	if runtime.GOOS != "darwin" {
		t.Fatalf("%s: LocalDarwinSupervisor required", assertion)
	}
	provider, _ = filepath.EvalSymlinks(provider)
	if !filepath.IsAbs(provider) {
		t.Fatalf("%s: independently installed absolute provider required", assertion)
	}
	root := conformanceRepositoryRoot(t)
	if rel, e := filepath.Rel(root, provider); e == nil && !strings.HasPrefix(rel, "..") {
		t.Fatalf("%s: repository-local provider", assertion)
	}
	seeds := []struct {
		id, relation, path string
		want               int
	}{
		{"binds-argument-positive", "BINDS_ARGUMENT", "qualification/external-provider/component.gts", 1}, {"binds-argument-negative", "BINDS_ARGUMENT", "qualification/external-provider/binds-argument-negative.gts", 0},
		{"passes-callback-positive", "PASSES_CALLBACK", "qualification/external-provider/passes-callback-positive.gts", 1}, {"passes-callback-negative", "PASSES_CALLBACK", "qualification/external-provider/passes-callback-negative.gts", 0},
		{"invokes-task-positive", "INVOKES_TASK", "providers/ember-glint/fixtures/invokes-task-positive.gts", 1}, {"invokes-task-negative", "INVOKES_TASK", "providers/ember-glint/fixtures/invokes-task-negative.gts", 0},
		{"triggers-reload-positive", "TRIGGERS_RELOAD", "providers/ember-glint/fixtures/triggers-reload-positive.ts", 1}, {"triggers-reload-negative", "TRIGGERS_RELOAD", "providers/ember-glint/fixtures/triggers-reload-negative.ts", 0},
		{"updates-state-positive", "UPDATES_STATE", "providers/ember-glint/fixtures/updates-state-positive.ts", 2}, {"updates-state-negative", "UPDATES_STATE", "providers/ember-glint/fixtures/updates-state-negative.ts", 0},
		{"renders-from-positive", "RENDERS_FROM", "providers/ember-glint/fixtures/renders-from-positive.json", 1}, {"renders-from-negative", "RENDERS_FROM", "providers/ember-glint/fixtures/renders-from-negative.json", 0},
	}
	workspace := t.TempDir()
	workspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		t.Fatalf("%s: canonicalize workspace: %v", assertion, err)
	}
	for _, s := range seeds {
		raw, e := os.ReadFile(filepath.Join(root, s.path))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(workspace, s.id+filepath.Ext(s.path)), raw, 0644); e != nil {
			t.Fatal(e)
		}
	}
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = workspace
		c.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("%s: git %v: %v %s", assertion, args, e, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("config", "user.name", "lsp-trace qualification")
	git("config", "user.email", "qualification@example.invalid")
	git("add", ".")
	git("commit", "-qm", "frozen b05 seeds")
	commit := git("rev-parse", "HEAD")
	if path := os.Getenv("B05_FRAME6_COMMIT_FILE"); path != "" {
		if err := os.WriteFile(path, []byte(commit+"\n"), 0o644); err != nil {
			t.Fatalf("%s: retain workspace commit: %v", assertion, err)
		}
	}
	mcpBinary := buildMCPBinary(t)
	fakeLSP := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "b05-frame6", "profile": map[string]any{"trust_domain": "b05-frame6", "workspace": workspace, "profile": "fake-lsp", "environment_reference": "qualification"}, "execution": map[string]any{"path": fakeLSP, "directory": workspace}}}, "providers": []any{map[string]any{"schema_version": "lsp-trace.bootstrap-provider.v1", "identity": "ember-glint@1", "version": "1", "protocol": map[string]any{"name": observationadapter.ProtocolName, "version": observationadapter.ProtocolVersion}, "execution": map[string]any{"path": provider, "directory": filepath.Dir(provider)}, "executable_available": true, "conformance_verified": true, "capabilities": map[string]any{"relations": []string{"BINDS_ARGUMENT", "PASSES_CALLBACK", "INVOKES_TASK", "TRIGGERS_RELOAD", "UPDATES_STATE", "RENDERS_FROM"}, "languages": []string{"glimmer-js"}, "frameworks": []string{"ember"}}, "limits": map[string]any{"request_bytes": 1048576, "response_bytes": 1048576, "protocol_messages": 1, "stderr_bytes": 4096, "wall_time_ms": 30000, "termination_grace_ms": 1000}}}}
	requests := []map[string]any{callRequest(1, "lsp_session_v1_list", map[string]any{})}
	id := 2
	for _, s := range seeds {
		for _, op := range []string{"incoming", "slice"} {
			uri := "file://" + filepath.Join(workspace, s.id+filepath.Ext(s.path))
			q := map[string]any{"session_id": "b05-frame6", "generation": 1, "uri": uri, "line": 0, "character": 0, "relations": []string{s.relation}, "providers": []string{"ember-glint@1"}, "languages": []string{"glimmer-js"}, "frameworks": []string{"ember"}, "workspace_revision": map[string]any{"kind": "git", "commit": commit, "custody": "CALLER_ASSERTED"}, "fail_on_unknown_revision": true, "max_nodes": 100, "timeout_ms": 30000, "request_timeout_ms": 30000}
			if op == "slice" {
				q["start_mode"] = "at"
				q["up_depth"] = 1
				q["down_depth"] = 1
			} else {
				q["max_depth"] = 2
			}
			requests = append(requests, callRequest(id, "lsp_trace_v1_"+op, q), callRequest(id+1, "lsp_trace_v1_"+op, q))
			id += 2
		}
	}
	responses, e := runMCPProcessForAcceptance(mcpBinary, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, requests)
	if e != nil {
		t.Fatalf("%s: %v", assertion, e)
	}
	if len(responses) != 49 {
		t.Fatalf("%s: responses=%d", assertion, len(responses))
	}
	n := 0
	for i := 1; i < len(responses); i += 2 {
		first := decodeProcessCall(t, responses[i])
		second := decodeProcessCall(t, responses[i+1])
		seed := seeds[n/2]
		if first.env["operation_status"] == "SUCCEEDED" {
			a, b := inlineArtifactBytes(t, first.env), inlineArtifactBytes(t, second.env)
			if len(a) == 0 || !bytes.Equal(a, b) {
				t.Fatalf("%s: artifact replay %d differs", assertion, n)
			}
			if err := graph.ValidateNormalizedRelationsJSON(a); err != nil {
				t.Fatalf("ASSERT_B05_FRAME6_GRAPH_V4_SCHEMA_%s: %v", seed.id, err)
			}
			var result graph.NormalizedRelations
			if err := json.Unmarshal(a, &result); err != nil {
				t.Fatalf("ASSERT_B05_FRAME6_GRAPH_V4_DECODE_%s: %v", seed.id, err)
			}
			if err := validateB05Frame6Relations(result.Relations, seed.relation, seed.want); err != nil {
				t.Fatalf("ASSERT_B05_FRAME6_EXACT_RELATION_%s: %v; got=%+v", seed.id, err, result.Relations)
			}
			if seed.id == "updates-state-positive" {
				if err := validateB05Frame6UpdatesState(result.Relations); err != nil {
					t.Fatalf("ASSERT_B05_FRAME6_UPDATES_STATE_DECLARATION_SEMANTICS: %v; got=%+v", err, result.Relations)
				}
			}
			if seed.id == "binds-argument-positive" {
				from := strings.TrimPrefix(result.Relations[0].From, "path:")
				source, err := os.ReadFile(filepath.Join(workspace, seed.id+filepath.Ext(seed.path)))
				if err != nil || from == result.Relations[0].From || !bytes.Contains(source, []byte(from)) {
					t.Fatalf("ASSERT_B05_FRAME6_BINDS_ARGUMENT_SOURCE_SEMANTICS: source=%q from=%s err=%v", source, result.Relations[0].From, err)
				}
			}
		} else {
			if strings.HasSuffix(seed.id, "-positive") {
				t.Fatalf("ASSERT_B05_FRAME6_POSITIVE_MUST_RESOLVE_%s: envelope=%v", seed.id, first.env)
			}
			if first.env["outcome"] != "DOMAIN_ERROR" || first.env["operation_status"] != "FAILED" || first.env["code"] == "" || first.env["code"] != second.env["code"] {
				t.Fatalf("%s: attempt %d lacks deterministic explicit domain envelope: first=%v second=%v", assertion, n, first.env, second.env)
			}
			if _, ok := first.env["content"]; ok {
				t.Fatalf("%s: blocked attempt %d returned empty-success content", assertion, n)
			}
		}
		t.Logf("PASS ASSERT_B05_FRAME6_ATTEMPT_%02d", n+1)
		n++
	}
	if n != 24 {
		t.Fatalf("%s: got %d", assertion, n)
	}
	fmt.Fprintf(os.Stderr, "PASS %s commit=%s attempts=%d\n", assertion, commit, n)
}

func validateB05Frame6Relations(relations []graph.Relation, kind string, want int) error {
	if len(relations) != want {
		return fmt.Errorf("want=%s/%d got count=%d", kind, want, len(relations))
	}
	for i, relation := range relations {
		if relation.Kind != kind {
			return fmt.Errorf("relation[%d] want kind=%s got=%s", i, kind, relation.Kind)
		}
	}
	return nil
}

func validateB05Frame6UpdatesState(relations []graph.Relation) error {
	if err := validateB05Frame6Relations(relations, graph.RelationUpdatesState, 2); err != nil {
		return err
	}
	producer := relations[0].From
	if !strings.HasPrefix(producer, "UPDATES_STATE:state-producer:") {
		return fmt.Errorf("producer is not declaration-derived: %s", producer)
	}
	targets := map[string]struct{}{}
	anchors := map[graph.Range]struct{}{}
	for i, relation := range relations {
		if relation.From != producer {
			return fmt.Errorf("relation[%d] producer=%s differs from %s", i, relation.From, producer)
		}
		if !strings.HasPrefix(relation.To, "UPDATES_STATE:state-value:") {
			return fmt.Errorf("relation[%d] target is not declaration-derived STATE_VALUE: %s", i, relation.To)
		}
		if len(relation.Anchors) != 1 {
			return fmt.Errorf("relation[%d] want one write anchor got=%d", i, len(relation.Anchors))
		}
		targets[relation.To] = struct{}{}
		anchors[relation.Anchors[0].Range] = struct{}{}
	}
	if len(targets) != 2 {
		return fmt.Errorf("want two distinct STATE_VALUE targets got=%d", len(targets))
	}
	if len(anchors) != 2 {
		return fmt.Errorf("want two distinct write anchors got=%d", len(anchors))
	}
	return nil
}

func TestB05Frame6ExactMultiplicityGuard(t *testing.T) {
	relation := graph.Relation{Kind: graph.RelationUpdatesState}
	for name, relations := range map[string][]graph.Relation{
		"missing second": {relation},
		"spurious third": {relation, relation, relation},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateB05Frame6Relations(relations, graph.RelationUpdatesState, 2); err == nil {
				t.Fatal("ASSERT_B05_FRAME6_EXACT_MULTIPLICITY_GUARD: malformed multiplicity accepted")
			}
		})
	}
}
