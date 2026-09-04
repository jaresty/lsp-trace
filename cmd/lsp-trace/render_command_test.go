package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderCommandDirectSelectorParityAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "artifact.json")
	if err := os.WriteFile(artifact, []byte(presentationFixture), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	var directOut, selectorOut, stderr bytes.Buffer
	old := renderLoadArtifact
	renderLoadArtifact = func(path string) ([]byte, error) { return os.ReadFile(artifact) }
	defer func() { renderLoadArtifact = old }()
	if code := runRender([]string{artifact, "--format", "summary"}, &directOut, &stderr); code != 0 {
		t.Errorf("FAIL ASSERT_RENDER_ACCEPTS_ARTIFACT_OR_SELECTOR: code=%d stderr=%s", code, stderr.String())
	}
	stderr.Reset()
	if code := runRender([]string{"selector.json", "--format", "summary"}, &selectorOut, &stderr); code != 0 {
		t.Errorf("FAIL ASSERT_RENDER_ACCEPTS_ARTIFACT_OR_SELECTOR: selector code=%d stderr=%s", code, stderr.String())
	}
	if directOut.String() != selectorOut.String() {
		t.Errorf("FAIL ASSERT_RENDER_SELECTOR_PARITY: direct=%q selector=%q", directOut.String(), selectorOut.String())
	}
	after, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("FAIL ASSERT_RENDER_READ_ONLY: artifact bytes changed")
	}
}

func TestRenderCommandRejectsMode(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := runRender([]string{"artifact.json", "--format", "wrong"}, &out, &stderr); code == 0 {
		t.Error("FAIL ASSERT_RENDER_MODE_ERROR: invalid mode accepted")
	}
}

const presentationFixture = `{"schema_version":"lsp-trace.graph.v3","invocation":{"workspace_uri":"file:///work/repo","limits":{"max_depth":2,"max_nodes":8,"timeout_ms":1000}},"nodes":[],"edges":[],"terminals":[],"frontier":[],"summary":{"node_count":0,"edge_count":0,"terminal_count":0,"cycle_count":0,"traversal_complete":true,"source_graph_complete":"UNKNOWN","completeness_scope":"SERVER_REPORTED_CALL_HIERARCHY","truncated":false}}`
