package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/provider"
)

func TestAcquisitionDiagnosticFlagBootstrapValidationAndLifecycle(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n")
	t.Run("absent does not create recorder and publication root is independent", func(t *testing.T) {
		root := t.TempDir()
		var out, errOut bytes.Buffer
		if code := run([]string{"--publication-root", root}, input, &out, &errOut); code != 0 {
			t.Fatalf("run=%d stderr=%s", code, errOut.String())
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("publication root unexpectedly populated: %v", entries)
		}
	})
	t.Run("valid independent path is accepted and bound", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "diagnostic.ndjson")
		var out, errOut bytes.Buffer
		if code := run([]string{"--acquisition-diagnostic-path", path}, input, &out, &errOut); code != 0 {
			t.Fatalf("run=%d stderr=%s", code, errOut.String())
		}
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("ledger=%v info=%v", err, info)
		}
		server, _, err := newServerRuntimeWithSeedAuthoritiesAndProfileAndArtifactStore(false, provider.NewConfiguredInventory(provider.Provisioned{}), nil, nil, nil, nil, mcp.ToolProfileDefault)
		if err != nil {
			t.Fatal(err)
		}
		r, err := censusdiagnostic.NewRecorder(filepath.Join(dir, "bound.ndjson"), censusdiagnostic.DefaultMaxBytes, censusdiagnostic.DefaultMaxRecords)
		if err != nil {
			t.Fatal(err)
		}
		if err := bindCensusDiagnosticRecorder(server, r); err != nil {
			t.Fatal(err)
		}
	})
	for _, tc := range []struct {
		name string
		args func(string) []string
	}{
		{"relative", func(string) []string { return []string{"--acquisition-diagnostic-path", "relative.ndjson"} }},
		{"unclean", func(p string) []string {
			return []string{"--acquisition-diagnostic-path", filepath.Dir(p) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(p)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "diagnostic.ndjson")
			args := tc.args(path)
			if tc.name == "unclean" && (filepath.Clean(args[1]) == args[1] || !filepath.IsAbs(args[1])) {
				t.Fatalf("unclean fixture was normalized: %q", args[1])
			}
			var out, errOut bytes.Buffer
			if code := run(args, input, &out, &errOut); code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "acquisition diagnostic:") {
				t.Fatalf("run=%d stdout=%q stderr=%s", code, out.String(), errOut.String())
			}
		})
	}
	t.Run("invalid custody is rejected without repair", func(t *testing.T) {
		base := t.TempDir()
		target := filepath.Join(base, "target")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name  string
			path  string
			setup func()
		}{
			{"missing parent", filepath.Join(base, "missing", "x.ndjson"), func() {}},
			{"wrong parent mode", filepath.Join(base, "mode", "x.ndjson"), func() { _ = os.Mkdir(filepath.Dir(filepath.Join(base, "mode", "x.ndjson")), 0o755) }},
			{"symlink parent", filepath.Join(base, "parent", "x.ndjson"), func() { _ = os.Symlink(base, filepath.Join(base, "parent")) }},
			{"wrong file mode", filepath.Join(base, "file.ndjson"), func() { _ = os.WriteFile(filepath.Join(base, "file.ndjson"), nil, 0o644) }},
			{"symlink file", filepath.Join(base, "link.ndjson"), func() { _ = os.Symlink(target, filepath.Join(base, "link.ndjson")) }},
			{"nonregular file", filepath.Join(base, "dir.ndjson"), func() { _ = os.Mkdir(filepath.Join(base, "dir.ndjson"), 0o700) }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				tc.setup()
				var out, errOut bytes.Buffer
				if code := run([]string{"--acquisition-diagnostic-path", tc.path}, input, &out, &errOut); code == 0 || out.Len() != 0 {
					t.Fatalf("run=%d stdout=%q stderr=%s", code, out.String(), errOut.String())
				}
			})
		}
	})
	t.Run("sequential runs do not leak recorder state", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "diagnostic.ndjson")
		var out, errOut bytes.Buffer
		if code := run([]string{"--acquisition-diagnostic-path", path}, input, &out, &errOut); code != 0 {
			t.Fatal(code)
		}
		before, _ := os.ReadFile(path)
		out.Reset()
		errOut.Reset()
		if code := run(nil, input, &out, &errOut); code != 0 {
			t.Fatal(code)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("recorder state leaked across runs")
		}
	})
}
