package mcpcontract

import "testing"

const structuralContextRegexLocatorInputID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-structural-context.v5.schema.json"

func TestStructuralContextRegexLocatorContract(t *testing.T) {
	base := `"session_id":"project","generation":1,"down_depth":0,"up_depth":0,"max_nodes":10,"timeout_ms":1000,"request_timeout_ms":1000,"analysis":{"kind":"NEIGHBORHOOD"}`
	validLocator := `"regex_locator":{"uri":"file:///workspace/main.go","pattern":"\\bfunc\\s+Target\\b","match_index":1,"capture_group":0,"expected_document_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","limits":{"max_document_bytes":1048576,"max_matches":100,"max_pattern_bytes":4096,"max_work":1000000}}`

	cases := []struct {
		name      string
		assertion string
		input     string
		valid     bool
	}{
		{"valid", "ASSERT_REGEX_LOCATOR_VALID", `{` + base + `,` + validLocator + `}`, true},
		{"exclusive-symbol", "ASSERT_REGEX_LOCATOR_EXCLUSIVE_SYMBOL", `{` + base + `,"symbol":"Target",` + validLocator + `}`, false},
		{"exclusive-position", "ASSERT_REGEX_LOCATOR_EXCLUSIVE_POSITION", `{` + base + `,"uri":"file:///workspace/main.go","line":0,"character":0,` + validLocator + `}`, false},
		{"requires-uri", "ASSERT_REGEX_LOCATOR_REQUIRES_URI", `{` + base + `,"regex_locator":{"pattern":"Target","match_index":0,"limits":{"max_document_bytes":1,"max_matches":1,"max_pattern_bytes":1,"max_work":1}}}`, false},
		{"requires-pattern", "ASSERT_REGEX_LOCATOR_REQUIRES_PATTERN", `{` + base + `,"regex_locator":{"uri":"file:///workspace/main.go","match_index":0,"limits":{"max_document_bytes":1,"max_matches":1,"max_pattern_bytes":1,"max_work":1}}}`, false},
		{"nonempty-pattern", "ASSERT_REGEX_LOCATOR_NONEMPTY_PATTERN", `{` + base + `,"regex_locator":{"uri":"file:///workspace/main.go","pattern":"","match_index":0,"limits":{"max_document_bytes":1,"max_matches":1,"max_pattern_bytes":1,"max_work":1}}}`, false},
		{"zero-based-index", "ASSERT_REGEX_LOCATOR_ZERO_BASED_INDEX", `{` + base + `,"regex_locator":{"uri":"file:///workspace/main.go","pattern":"Target","match_index":-1,"limits":{"max_document_bytes":1,"max_matches":1,"max_pattern_bytes":1,"max_work":1}}}`, false},
		{"capture-bound", "ASSERT_REGEX_LOCATOR_CAPTURE_BOUND", `{` + base + `,"regex_locator":{"uri":"file:///workspace/main.go","pattern":"Target","match_index":0,"capture_group":33,"limits":{"max_document_bytes":1,"max_matches":1,"max_pattern_bytes":1,"max_work":1}}}`, false},
		{"digest-shape", "ASSERT_REGEX_LOCATOR_DIGEST_SHAPE", `{` + base + `,"regex_locator":{"uri":"file:///workspace/main.go","pattern":"Target","match_index":0,"expected_document_digest":"sha256:nope","limits":{"max_document_bytes":1,"max_matches":1,"max_pattern_bytes":1,"max_work":1}}}`, false},
		{"requires-limits", "ASSERT_REGEX_LOCATOR_REQUIRES_LIMITS", `{` + base + `,"regex_locator":{"uri":"file:///workspace/main.go","pattern":"Target","match_index":0}}`, false},
		{"positive-limits", "ASSERT_REGEX_LOCATOR_POSITIVE_LIMITS", `{` + base + `,"regex_locator":{"uri":"file:///workspace/main.go","pattern":"Target","match_index":0,"limits":{"max_document_bytes":0,"max_matches":0,"max_pattern_bytes":0,"max_work":0}}}`, false},
		{"unknown-field", "ASSERT_REGEX_LOCATOR_UNKNOWN_FIELD", `{` + base + `,"regex_locator":{"uri":"file:///workspace/main.go","pattern":"Target","match_index":0,"fallback":true,"limits":{"max_document_bytes":1,"max_matches":1,"max_pattern_bytes":1,"max_work":1}}}`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateJSON(structuralContextRegexLocatorInputID, []byte(tc.input))
			if tc.valid && err != nil {
				t.Fatalf("%s: valid request rejected: %v", tc.assertion, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("%s: invalid request accepted", tc.assertion)
			}
		})
	}
}
