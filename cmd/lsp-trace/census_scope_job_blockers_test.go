//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	ai "lsp-trace/internal/applicationinventory"
	"os"
	"path/filepath"
	"testing"
)

func TestScopeJobBlockersStorePrecedence(t *testing.T) {
	private := func() string {
		p, e := filepath.EvalSymlinks(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		if e = os.Chmod(p, 0700); e != nil {
			t.Fatal(e)
		}
		return p
	}
	home, workspace, explicit, env := private(), private(), private(), private()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv(censusDurableStoreEnv, env)
	for _, mode := range []string{"explicit", "environment", "default"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{workspace, "--scope-job-candidate", "--profile", "gopls"}
			if mode == "explicit" {
				args = append(args, "--publication-root", explicit)
			}
			if mode == "default" {
				t.Setenv(censusDurableStoreEnv, "")
			}
			var out, diag bytes.Buffer
			if code := runCensusWithDependencies(args, &out, &diag, censusRunnerDependencies{}); code != 0 {
				t.Fatalf("start code=%d diag=%s", code, diag.String())
			}
			var r ai.InventoryJobStatus
			if e := json.Unmarshal(out.Bytes(), &r); e != nil {
				t.Fatal(e)
			}
			var receipt struct {
				JobID           string   `json:"job_id"`
				StatusOperation []string `json:"status_operation"`
			}
			if e := json.Unmarshal(diag.Bytes(), &receipt); e != nil || receipt.JobID != r.RunID {
				t.Fatalf("receipt %s", diag.String())
			}
			selected := ""
			if mode == "explicit" {
				selected = explicit
			}
			root, path, e := openSelectedCensusStore("", selected)
			if e != nil {
				t.Fatal(e)
			}
			root.Close()
			if mode == "explicit" && path != explicit || mode == "environment" && path != env || mode == "default" && !censusPathWithin(home, path) {
				t.Fatalf("wrong store %q", path)
			}
			if _, e = os.Stat(filepath.Join(path, "inventory-scope-"+r.RunID)); e != nil {
				t.Fatal(e)
			}
			out.Reset()
			diag.Reset()
			if code := runCensusWithDependencies(receipt.StatusOperation[1:], &out, &diag, censusRunnerDependencies{}); code != 0 {
				t.Fatalf("status code=%d diag=%s", code, diag.String())
			}
		})
	}
}
