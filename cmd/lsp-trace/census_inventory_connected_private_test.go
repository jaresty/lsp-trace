package main

import (
	"bytes"
	"lsp-trace/sessionruntime"
	"strings"
	"testing"
)

func TestCensusInventoryConnectedPrivateExclusiveFlags(t *testing.T) {
	for _, ordinary := range [][]string{{"-workspace", "/synthetic"}, {"--server", "never-started"}} {
		args := append([]string{"--private-inventory-bridge-config", "/synthetic/nonexistent/connected.json", "--machine"}, ordinary...)
		var out, diagnostic bytes.Buffer
		code := runCensusWithDependencies(args, &out, &diagnostic, censusRunnerDependencies{})
		if code == 0 || !strings.Contains(diagnostic.String(), "SYNTAX") {
			t.Fatal("ASSERT_CONNECTED_PRIVATE_FLAGS_EXCLUSIVE", ordinary, code, diagnostic.String())
		}
	}
	t.Log("ASSERT_CONNECTED_PRIVATE_FLAGS_EXCLUSIVE PASS")
}

// This invokes the actual CLI entry; no live process or native owner is created.
func TestCensusInventoryConnectedPrivateEntry(t *testing.T) {
	var out, diagnostic bytes.Buffer
	starts := 0
	deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) {
		starts++
		t.Error("ASSERT_CONNECTED_NO_CONSTRUCTOR_BEFORE_ADMISSION")
		return nil, nil
	}}
	code := runCensusWithDependencies([]string{"--private-inventory-bridge-config", "/synthetic/nonexistent/connected.json", "--machine"}, &out, &diagnostic, deps)
	if code == 0 || !strings.Contains(diagnostic.String(), "CONFIG") || strings.Contains(diagnostic.String(), "SYNTAX") || starts != 0 {
		t.Fatalf("ASSERT_CONNECTED_ENTRY_REACHES_ADMISSION: code=%d starts=%d diagnostic=%s", code, starts, diagnostic.String())
	}
	t.Log("ASSERT_CONNECTED_ENTRY_REACHES_ADMISSION PASS")
}
