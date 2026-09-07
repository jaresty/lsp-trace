package integratedconformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RawMessage is intentional: maps cannot retain the duplicate-member witness.
func TestCustodyRepairRawInput(t *testing.T) {
	h := newOperationalHarness(t)
	for _, mode := range []string{"direct", "cli", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			for _, fixture := range []string{
				`{"operational":{"require_authenticated":true},"operational":OP}`,
				`{"operational":{"require_authenticated":true,"require_authenticated":false,"source_root":SRC,"inputs":[{"path":"input.go","class":"SOURCE"}]}}`,
				`{"operational":{"source_root":SRC,"inputs":[{"path":"missing","path":"input.go","class":"SOURCE"}]}}`,
				`{"operational":{"require_authenticated":true,"require_\u0061uthenticated":false,"source_root":SRC,"inputs":[{"path":"input.go","class":"SOURCE"}]}}`,
				`{"Source":"ignored","operational":OP}`,
				`{"Source":"","operational":OP}`,
				`{"SOURCE":"","operational":OP}`,
				`{"source":"","operational":OP}`,
				`{"Operational":OP}`,
				`{"operational":{"Source_root":SRC,"inputs":[{"path":"input.go","class":"SOURCE"}]}}`,
				`{"operational":{"source_root":SRC,"inputs":[{"Path":"input.go","class":"SOURCE"}]}}`,
			} {
				t.Run(fixture, func(t *testing.T) {
					input := operationalRequest(t)
					op := input["operational"].(map[string]any)
					delete(op, "require_authenticated") // exact reported duplicate-object fixture
					opRaw, _ := json.Marshal(op)
					sourceRaw, _ := json.Marshal(op["source_root"])
					rootRaw, _ := json.Marshal(input["root"])
					raw := strings.ReplaceAll(strings.ReplaceAll(fixture, "OP", string(opRaw)), "SRC", string(sourceRaw))
					raw = `{"root":` + string(rootRaw) + `,` + raw[1:]
					got := h.run(mode, json.RawMessage(raw), "", nil)
					if !got.Failed {
						t.Errorf("ASSERT_RAW_REJECTION: accepted %s", raw)
					}
					if strings.Contains(fixture, `"require_authenticated":true`) || strings.Contains(fixture, `"path":"missing"`) {
						if !strings.Contains(got.Output, "duplicate JSON member") {
							t.Errorf("not rejected at raw duplicate boundary: %s", got.Output)
						}
					}
					entries, err := os.ReadDir(input["root"].(string))
					if err != nil {
						t.Fatal(err)
					}
					if len(entries) != 0 {
						t.Errorf("ASSERT_NO_PUBLICATION: %v", entries)
					}
				})
			}
			for _, legacy := range []string{"", "source", "Source"} {
				input := operationalRequest(t)
				if legacy != "" {
					delete(input, "operational")
					input[legacy] = "legacy"
				}
				got := h.run(mode, input, "", nil)
				if got.Failed {
					t.Fatalf("ordinary control: %s", got.Output)
				}
				if _, err := os.Stat(filepath.Join(input["root"].(string), "receipt.json")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
