package integratedconformance

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionExecutionWiring(t *testing.T) {
	root := repositoryRoot(t)
	request := func() map[string]any {
		return map[string]any{"root": t.TempDir(), "source": "package fixture\nfunc Production() {}\n"}
	}

	t.Run("ASSERT_PRODUCTION_CLI_CANONICAL_CUSTODY", func(t *testing.T) {
		input, err := json.Marshal(request())
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "run", "./cmd/lsp-trace", "execute", "--request-id", "production-cli", "--input", "-")
		cmd.Dir, cmd.Stdin = root, bytes.NewReader(input)
		out, err := cmd.CombinedOutput()
		if err != nil || !bytes.Contains(out, []byte(`"state":"ok"`)) || !bytes.Contains(out, []byte(`"operation":"execute"`)) || !bytes.Contains(out, []byte(`"artifact":`)) {
			t.Fatalf("ASSERT_PRODUCTION_CLI_CANONICAL_CUSTODY: err=%v output=%s", err, out)
		}
	})

	t.Run("ASSERT_PRODUCTION_MCP_CANONICAL_CUSTODY", func(t *testing.T) {
		params, _ := json.Marshal(map[string]any{"name": "lsp_trace_v1_execute", "arguments": map[string]any{"request": request()}})
		line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
		cmd := exec.Command("go", "run", "./cmd/lsp-trace-mcp")
		cmd.Dir, cmd.Stdin = root, bytes.NewReader(append(line, '\n'))
		out, err := cmd.CombinedOutput()
		if err != nil || !bytes.Contains(out, []byte(`"operation_status":"SUCCEEDED"`)) || !bytes.Contains(out, []byte(`"content":`)) || bytes.Contains(out, []byte("unknown offline operation")) {
			t.Fatalf("ASSERT_PRODUCTION_MCP_CANONICAL_CUSTODY: err=%v output=%s", err, out)
		}
	})

	t.Run("ASSERT_PRODUCTION_FAILS_CLOSED", func(t *testing.T) {
		publication := filepath.Join(t.TempDir(), "publication")
		cmd := exec.Command("go", "run", "./cmd/lsp-trace", "execute", "--request-id", "invalid", "--input", "-")
		cmd.Dir, cmd.Stdin = root, strings.NewReader(`{"root":"","source":"x"}`)
		out, err := cmd.CombinedOutput()
		if err == nil || !bytes.Contains(out, []byte(`"state":"error"`)) {
			t.Fatalf("ASSERT_PRODUCTION_FAILS_CLOSED: err=%v output=%s", err, out)
		}
		if _, statErr := os.Stat(publication); !os.IsNotExist(statErr) {
			t.Fatalf("ASSERT_PRODUCTION_FAILS_CLOSED: publication exists: %v", statErr)
		}
	})
}
