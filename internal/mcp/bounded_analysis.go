package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/boundedanalysis"
)

// Preflight the new family's wire body before the shared recursive duplicate
// decoder. This shallow typed probe does not materialize the input value tree.
func preflightBoundedWire(raw []byte) error {
	var header struct {
		Params struct {
			Name      string `json:"name"`
			Arguments struct {
				Schema struct {
					Family string `json:"family"`
				} `json:"schema"`
			} `json:"arguments"`
		} `json:"params"`
	}
	if json.Unmarshal(raw, &header) != nil {
		return nil
	}
	name := header.Params.Name
	if name == "lsp_trace_v1_bounded_retained_analysis" || name == "lsp_trace_bounded_retained_analysis" || name == "lsp_trace_v1_bounded_retained_metrics" || name == "lsp_trace_bounded_retained_metrics" || ((name == "lsp_trace_v1_validate" || name == "lsp_trace_validate") && (header.Params.Arguments.Schema.Family == boundedanalysis.Family || header.Params.Arguments.Schema.Family == "bounded-retained-metrics")) {
		return boundedanalysis.Preflight(raw, 4*1024*1024)
	}
	return nil
}

// The new family preserves the exact JSON value bytes of an object input as
// inline text before generic map transports can round numbers or compact it.
// Historical operations retain their existing argument decoding behavior.
func decodeBoundedParams(raw json.RawMessage, dst *callParams) (bool, error) {
	var header struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return false, nil
	}
	selected := header.Name == "lsp_trace_v1_bounded_retained_analysis" || header.Name == "lsp_trace_bounded_retained_analysis" || header.Name == "lsp_trace_v1_bounded_retained_metrics" || header.Name == "lsp_trace_bounded_retained_metrics"
	if header.Name == "lsp_trace_v1_validate" || header.Name == "lsp_trace_validate" {
		var h struct {
			Schema struct {
				Family string `json:"family"`
			} `json:"schema"`
		}
		_ = json.Unmarshal(header.Arguments, &h)
		selected = h.Schema.Family == boundedanalysis.Family || h.Schema.Family == "bounded-retained-metrics"
	}
	if !selected {
		return false, nil
	}
	if err := boundedanalysis.Preflight(raw, boundedanalysis.MaxBytes); err != nil {
		return true, err
	}
	if err := decodeClosed(raw, &header, "name", "arguments"); err != nil {
		return true, err
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(header.Arguments, &values); err != nil {
		return true, err
	}
	if values == nil {
		return true, fmt.Errorf("arguments must be an object")
	}
	dst.Name = header.Name
	dst.Arguments = map[string]any{}
	for k, v := range values {
		if k == "input" && len(bytes.TrimSpace(v)) > 0 && bytes.TrimSpace(v)[0] == '{' {
			dst.Arguments[k] = string(v)
			continue
		}
		d := json.NewDecoder(bytes.NewReader(v))
		d.UseNumber()
		var value any
		if err := d.Decode(&value); err != nil {
			return true, err
		}
		dst.Arguments[k] = value
	}
	return true, nil
}
