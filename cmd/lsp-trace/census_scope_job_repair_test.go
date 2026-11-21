//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/scopejob"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopeJobRepairBudgetJSON(t *testing.T) {
	raw, _ := json.Marshal(scopejob.DefaultBudget())
	good := string(raw)
	for _, tc := range []struct {
		name, text string
		valid      bool
	}{
		{"valid", good, true},
		{"reordered", ` { "depth":128, ` + strings.TrimSuffix(strings.TrimPrefix(good, "{"), `,"depth":128}`) + " } ", true},
		{"duplicate-first", `{"retained_bytes":1,` + good[1:], false},
		{"duplicate-last", good[:len(good)-1] + `,"retained_bytes":1}`, false},
		{"duplicate-same", `{"retained_bytes":2147483648,` + good[1:], false},
		{"duplicate-valid-first", `{"retained_bytes":4294967296,` + good[1:], false},
		{"duplicate-valid-last", good[:len(good)-1] + `,"retained_bytes":4294967296}`, false},
		{"null", strings.Replace(good, `"depth":128`, `"depth":null`, 1), false},
		{"omitted", strings.Replace(good, `,"depth":128`, "", 1), false},
		{"alias", strings.Replace(good, `"retained_bytes"`, `"RETAINED_BYTES"`, 1), false},
		{"mixed-alias", strings.Replace(good, `"retained_bytes"`, `"Retained_Bytes"`, 1), false},
		{"unknown", good[:len(good)-1] + `,"unknown":1}`, false},
		{"nested", strings.Replace(good, `"depth":128`, `"depth":{"depth":1,"depth":128}`, 1), false},
		{"multiple", good + good, false}, {"trailing", good + " junk", false},
		{"overflow", strings.Replace(good, `"retained_bytes":2147483648`, `"retained_bytes":18446744073709551616`, 1), false},
		{"fraction", strings.Replace(good, `"depth":128`, `"depth":1.5`, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			parent, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			store := filepath.Join(parent, "store")
			if e := os.Mkdir(store, 0700); e != nil {
				t.Fatal(e)
			}
			budget := filepath.Join(parent, "budget.json")
			if e := os.WriteFile(budget, []byte(tc.text), 0600); e != nil {
				t.Fatal(e)
			}
			var out, diag bytes.Buffer
			code := runCensusWithDependencies([]string{workspace, "--scope-job-candidate", "--profile", "gopls", "--publication-root", store, "--scope-job-budget", budget}, &out, &diag, censusRunnerDependencies{lookPath: func(string) (string, error) { t.Fatal("provider reached"); return "", nil }})
			if (code == 0) != tc.valid {
				t.Errorf("budget admission code=%d valid=%v diagnostics=%s", code, tc.valid, diag.String())
			}
			if !tc.valid {
				if names, e := os.ReadDir(store); e != nil || len(names) != 0 {
					t.Error("invalid budget created artifacts")
				}
				if out.Len() != 0 {
					t.Error("invalid budget emitted report")
				}
				if strings.TrimSpace(diag.String()) != `{"schema_version":"lsp-trace.census-diagnostic.v1","status":"FAILED","stage":"config","code":"INVALID_CONFIG","retry":true}` {
					t.Errorf("failure shape changed: %s", diag.String())
				}
			}
		})
	}
}
