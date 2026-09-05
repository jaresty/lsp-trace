package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/observationadapter"
)

func TestProductionMCPB05TwentyAttemptQualification(t *testing.T) {
	const assertion = "ASSERT_B05_FRAME6_REAL_MCP_TWENTY_ATTEMPTS"
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
	seeds := []struct{ id, relation, path string }{
		{"passes-callback-positive", "PASSES_CALLBACK", "qualification/external-provider/passes-callback-positive.gts"}, {"passes-callback-negative", "PASSES_CALLBACK", "qualification/external-provider/passes-callback-negative.gts"},
		{"invokes-task-positive", "INVOKES_TASK", "providers/ember-glint/fixtures/invokes-task-positive.gts"}, {"invokes-task-negative", "INVOKES_TASK", "providers/ember-glint/fixtures/invokes-task-negative.gts"},
		{"triggers-reload-positive", "TRIGGERS_RELOAD", "providers/ember-glint/fixtures/triggers-reload-positive.ts"}, {"triggers-reload-negative", "TRIGGERS_RELOAD", "providers/ember-glint/fixtures/triggers-reload-negative.ts"},
		{"updates-state-positive", "UPDATES_STATE", "providers/ember-glint/fixtures/updates-state-positive.ts"}, {"updates-state-negative", "UPDATES_STATE", "providers/ember-glint/fixtures/updates-state-negative.ts"},
		{"renders-from-positive", "RENDERS_FROM", "providers/ember-glint/fixtures/renders-from-positive.json"}, {"renders-from-negative", "RENDERS_FROM", "providers/ember-glint/fixtures/renders-from-negative.json"},
	}
	workspace := t.TempDir()
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
	config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "b05-frame6", "profile": map[string]any{"trust_domain": "b05-frame6", "workspace": workspace, "profile": "fake-lsp", "environment_reference": "qualification"}, "execution": map[string]any{"path": fakeLSP, "directory": workspace}}}, "providers": []any{map[string]any{"schema_version": "lsp-trace.bootstrap-provider.v1", "identity": "ember-glint@1", "version": "1", "protocol": map[string]any{"name": observationadapter.ProtocolName, "version": observationadapter.ProtocolVersion}, "execution": map[string]any{"path": provider, "directory": filepath.Dir(provider)}, "executable_available": true, "conformance_verified": true, "capabilities": map[string]any{"relations": []string{"PASSES_CALLBACK", "UPDATES_STATE", "RENDERS_FROM"}, "languages": []string{"glimmer-js"}, "frameworks": []string{"ember"}}, "limits": map[string]any{"request_bytes": 1048576, "response_bytes": 1048576, "protocol_messages": 1, "stderr_bytes": 4096, "wall_time_ms": 30000, "termination_grace_ms": 1000}}}}
	requests := []map[string]any{callRequest(1, "lsp_session_v1_list", map[string]any{})}
	id := 2
	for _, s := range seeds {
		for _, op := range []string{"incoming", "slice"} {
			uri := "file://" + filepath.Join(workspace, s.id+filepath.Ext(s.path))
			q := map[string]any{"session_id": "b05-frame6", "generation": 1, "uri": uri, "line": 0, "character": 0, "relations": []string{s.relation}, "providers": []string{"ember-glint@1"}, "languages": []string{"glimmer-js"}, "frameworks": []string{"ember"}, "workspace_revision": map[string]any{"kind": "git", "commit": commit, "custody": "CALLER_ASSERTED"}, "fail_on_unknown_revision": true, "max_nodes": 100, "max_messages": 1, "max_bytes": 1048576, "timeout_ms": 30000, "request_timeout_ms": 30000}
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
	if len(responses) != 41 {
		t.Fatalf("%s: responses=%d", assertion, len(responses))
	}
	n := 0
	for i := 1; i < len(responses); i += 2 {
		first := decodeProcessCall(t, responses[i])
		second := decodeProcessCall(t, responses[i+1])
		if first.env["operation_status"] == "SUCCEEDED" {
			a, b := inlineArtifactBytes(t, first.env), inlineArtifactBytes(t, second.env)
			if len(a) == 0 || !bytes.Equal(a, b) {
				t.Fatalf("%s: artifact replay %d differs", assertion, n)
			}
		} else {
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
	if n != 20 {
		t.Fatalf("%s: got %d", assertion, n)
	}
	fmt.Fprintf(os.Stderr, "PASS %s commit=%s attempts=%d\n", assertion, commit, n)
}
