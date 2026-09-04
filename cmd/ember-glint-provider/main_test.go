package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionCommandRequiresStdio(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "--stdio") {
		t.Fatalf("error=%v", err)
	}
}
func TestProductionCommandHonestlyAdvertisesNoUnqualifiedKinds(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app.gts")
	if err := os.WriteFile(source, []byte("<template></template>"), 0600); err != nil {
		t.Fatal(err)
	}
	req := map[string]any{"schema_version": "lsp-trace.provider-collector-request.v1", "provider_id": "ember-glint@1", "adapter_id": "host-adapter@1", "session": map[string]any{"session_id": "s", "generation": 1}, "seed": map[string]any{"uri": "file://" + source}, "relations": []string{"RENDERS_FROM"}, "document_custody": map[string]any{"original_uri": "file://" + source, "workspace_revision": "rev-1", "fail_on_unknown_revision": true}, "limits": map[string]any{"max_nodes": 10, "max_messages": 1, "max_bytes": 4096, "timeout_ms": 1000, "request_timeout_ms": 1000}}
	body, _ := json.Marshal(req)
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	var out bytes.Buffer
	if err := run([]string{"--stdio"}, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(out.String(), "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("unframed output %q", out.String())
	}
	var env struct {
		Provider struct{ Name, Version string }
		Coverage struct {
			Status    string   `json:"status"`
			Qualified []string `json:"qualified_relations"`
		}
		Observations []any
	}
	if err := json.Unmarshal([]byte(parts[1]), &env); err != nil {
		t.Fatal(err)
	}
	if env.Provider.Name != "ember-glint" || env.Provider.Version != "1" || env.Coverage.Status != "unsupported" || len(env.Coverage.Qualified) != 0 || len(env.Observations) != 0 {
		t.Fatalf("dishonest envelope: %+v", env)
	}
}
