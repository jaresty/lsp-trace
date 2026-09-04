package presentation

import (
	"strings"
	"testing"
)

const fixture = `{
 "schema_version":"lsp-trace.graph.v3",
 "invocation":{"workspace_uri":"file:///work/repo","limits":{"max_depth":2,"max_nodes":8,"timeout_ms":1000}},
 "nodes":[
  {"id":"b","name":"Beta","uri":"file:///outside/b.go","range":{"start":{"line":4,"character":1},"end":{"line":4,"character":4}},"selection_range":{"start":{"line":4,"character":1},"end":{"line":4,"character":4}}},
  {"id":"a","name":"A & B","uri":"file:///work/repo/a.go","range":{"start":{"line":1,"character":2},"end":{"line":1,"character":5}},"selection_range":{"start":{"line":1,"character":2},"end":{"line":1,"character":5}}}
 ],
 "edges":[{"caller_node_id":"a","callee_node_id":"b","call_sites":[{"start":{"line":2,"character":3},"end":{"line":2,"character":6}}]}],
 "terminals":[{"node_id":"b","reason":"MAX_DEPTH"}],
 "frontier":[{"node_id":"a","reason":"REQUEST_TIMEOUT"}],
 "summary":{"node_count":2,"edge_count":1,"terminal_count":1,"cycle_count":0,"traversal_complete":false,"source_graph_complete":"UNKNOWN","completeness_scope":"SERVER_REPORTED_CALL_HIERARCHY","truncated":true}
}`

func mustRender(t *testing.T, format Format, detail Detail) string {
	t.Helper()
	got, err := Render([]byte(fixture), Options{Format: format, Detail: detail})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRenderSummaryAndTruncation(t *testing.T) {
	got := mustRender(t, FormatSummary, DetailCompact)
	for _, assertion := range []struct{ name, want string }{
		{"ASSERT_SUMMARY_AUTHORITY", "presentation only"}, {"ASSERT_SUMMARY_COUNTS", "nodes: 2"},
		{"ASSERT_TRUNCATION_CAUSES", "MAX_DEPTH"}, {"ASSERT_PARAMETER_SUGGESTION", "--max-depth"},
		{"ASSERT_TRUNCATION_UNSUGGESTED_CAUSE", "REQUEST_TIMEOUT"}, {"ASSERT_NO_UNSAFE_SUGGESTION", "--request-timeout"},
	} {
		if !strings.Contains(got, assertion.want) {
			t.Errorf("FAIL %s: missing %q in %q", assertion.name, assertion.want, got)
		} else {
			t.Logf("PASS %s", assertion.name)
		}
	}
}

func TestRenderTreePathsDetailAndDeterminism(t *testing.T) {
	got := mustRender(t, FormatTree, DetailFull)
	checks := []struct{ name, want string }{
		{"ASSERT_TREE_RELATIVE_PATH", "a.go:2:3"}, {"ASSERT_EXTERNAL_URI_PRESERVED", "file:///outside/b.go"},
		{"ASSERT_TREE_EDGE", "A & B"}, {"ASSERT_FULL_CALL_SITE", "3:4"}, {"ASSERT_FULL_BOUNDARY", "MAX_DEPTH"},
	}
	for _, c := range checks {
		if !strings.Contains(got, c.want) {
			t.Errorf("FAIL %s: missing %q in %q", c.name, c.want, got)
		} else {
			t.Logf("PASS %s", c.name)
		}
	}
	if again := mustRender(t, FormatTree, DetailFull); got != again {
		t.Errorf("FAIL ASSERT_DETERMINISTIC")
	} else {
		t.Log("PASS ASSERT_DETERMINISTIC")
	}
}

func TestRenderMermaidAndValidation(t *testing.T) {
	got := mustRender(t, FormatMermaid, DetailCompact)
	if !strings.Contains(got, "flowchart TD") || !strings.Contains(got, "A &#38; B") {
		t.Errorf("FAIL ASSERT_MERMAID_PROJECTION: %q", got)
	} else if strings.Index(got, "n_a[") > strings.Index(got, "n_b[") {
		t.Errorf("FAIL ASSERT_DETERMINISTIC: node IDs not in canonical order: %q", got)
	} else {
		t.Log("PASS ASSERT_MERMAID_PROJECTION")
		t.Log("PASS ASSERT_DETERMINISTIC")
	}
	if _, err := Render([]byte(fixture), Options{Format: "bad", Detail: DetailCompact}); err == nil {
		t.Error("FAIL ASSERT_FORMAT_VALIDATION")
	} else {
		t.Log("PASS ASSERT_FORMAT_VALIDATION")
	}
	if _, err := Render([]byte(fixture), Options{Format: FormatSummary, Detail: "bad"}); err == nil {
		t.Error("FAIL ASSERT_DETAIL_VALIDATION")
	} else {
		t.Log("PASS ASSERT_DETAIL_VALIDATION")
	}
}
