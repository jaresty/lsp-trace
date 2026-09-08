package mcp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCurrentCanonicalToolDocumentationMatchesRegistry(t *testing.T) {
	tools := NewRegistry(false).Advertised()
	count := strconv.Itoa(len(tools))
	for _, path := range []string{
		filepath.Join("..", "..", "README.md"),
		filepath.Join("..", "..", "docs", "retained-calls.md"),
		filepath.Join("..", "..", "cmd", "lsp-trace", "SKILL.md"),
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "exactly "+count) {
			t.Fatalf("ASSERT_CURRENT_MCP_TOOL_COUNT_DOCS: %s does not state exactly %s", path, count)
		}
		position := -1
		for _, tool := range tools {
			next := strings.Index(text[position+1:], "`"+tool.Name+"`")
			if next < 0 {
				t.Fatalf("ASSERT_CURRENT_MCP_TOOL_LIST_DOCS: %s missing %s", path, tool.Name)
			}
			position += next + 1
		}
	}
}
