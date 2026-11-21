//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"testing"
)

func TestCensusDurableAssemblyDiagnosticIsSingleJSON(t *testing.T) {
	home := folderInventoryTemp(t)
	t.Setenv("HOME", home)
	t.Setenv(censusDurableStoreEnv, "")
	t.Setenv("LSP_TRACE_CALLER_FD", "")
	t.Setenv(censusDurableCallerEnv, "malformed compatibility declaration")
	workspace := folderInventoryTemp(t)
	for name, body := range map[string]string{"go.mod": "module controlled.example/checkout\n\ngo 1.26\n", "checkout.go": "package checkout\nfunc Checkout() int { return price() }\n", "price.go": "package checkout\nfunc price() int { return 7 }\n"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	wire := newFolderInventoryWire(t, workspace)
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { return "/controlled/provider", nil }
	deps.newStarter = func() (sessionruntime.Starter, error) { return terminalStarter{child: wire}, nil }
	var out, diagnostic bytes.Buffer
	code := runCensusWithDependencies([]string{workspace}, &out, &diagnostic, deps)
	d := checkOperationBytes(t, diagnostic.Bytes())
	if code != 1 || out.Len() != 0 || d.Operation != "start" || d.Action != "STOP" || d.Retry {
		t.Fatalf("assembly failure is one structured diagnostic: code=%d out=%s diagnostic=%s", code, out.String(), diagnostic.String())
	}
}
