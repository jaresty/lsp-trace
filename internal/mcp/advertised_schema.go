package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-trace/internal/mcpcontract"
)

const maxAdvertisementReferenceDepth = 16

// selfContainedAdvertisementSchema resolves external schema-resource references
// for MCP tools/list consumers that receive only one inputSchema object. Canonical
// schema bytes and runtime validation remain unchanged.
func selfContainedAdvertisementSchema(schema map[string]any) (map[string]any, error) {
	value, err := expandAdvertisementReferences(schema, 0, map[string]bool{})
	if err != nil {
		return nil, err
	}
	out, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("advertised input schema root is %T", value)
	}
	return out, nil
}

func expandAdvertisementReferences(value any, depth int, active map[string]bool) (any, error) {
	if depth > maxAdvertisementReferenceDepth {
		return nil, fmt.Errorf("advertised input schema reference depth exceeds %d", maxAdvertisementReferenceDepth)
	}
	switch typed := value.(type) {
	case map[string]any:
		if ref, ok := typed["$ref"].(string); ok && isExternalSchemaReference(ref) {
			if active[ref] {
				return nil, fmt.Errorf("advertised input schema reference cycle at %q", ref)
			}
			raw, err := mcpcontract.SchemaJSON(ref)
			if err != nil {
				return nil, fmt.Errorf("resolve advertised input schema reference %q: %w", ref, err)
			}
			var resolved any
			if err := json.Unmarshal(raw, &resolved); err != nil {
				return nil, fmt.Errorf("decode advertised input schema reference %q: %w", ref, err)
			}
			active[ref] = true
			expanded, err := expandAdvertisementReferences(resolved, depth+1, active)
			delete(active, ref)
			if err != nil {
				return nil, err
			}
			return expanded, nil
		}
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			expanded, err := expandAdvertisementReferences(child, depth, active)
			if err != nil {
				return nil, err
			}
			out[key] = expanded
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			expanded, err := expandAdvertisementReferences(child, depth, active)
			if err != nil {
				return nil, err
			}
			out[i] = expanded
		}
		return out, nil
	default:
		return value, nil
	}
}

func isExternalSchemaReference(ref string) bool {
	return strings.HasPrefix(ref, "https://jaresty.github.io/lsp-trace/") && !strings.Contains(ref, "#")
}
