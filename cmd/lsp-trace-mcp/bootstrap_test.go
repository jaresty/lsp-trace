package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/sessionruntime"
)

func TestBootstrapConfigIsStrictAndHostOwned(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_CONFIG_REJECTS_UNDECLARED_AUTHORITY"
	t.Log("ASSERTION: " + assertion)
	path := filepath.Join(t.TempDir(), "bootstrap.json")
	body := `{"version":1,"processes":[{"profile":{"trust_domain":"test","workspace":"/workspace","profile":"go","environment_reference":"local"},"execution":{"path":"server","directory":"/workspace"},"mcp_selected_command":"forbidden"}]}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBootstrapConfig(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("%s: err=%v", assertion, err)
	}
	t.Log("PASS " + assertion)
}

func TestBootstrapRollbackAndShutdownOwnEveryStartedSession(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("LOCAL_DARWIN_SUPERVISION_ONLY")
	}
	fake := buildBinary(t, "fake-lsp", "./cmd/fake-lsp")
	workspace := t.TempDir()
	valid := bootstrapProcessConfig{
		Profile:   bootstrapProfileIdentity{TrustDomain: "bootstrap", Workspace: workspace, Profile: "fake", EnvironmentReference: "hermetic"},
		Execution: managedExecutionAuthority{Path: fake, Directory: workspace},
	}

	t.Run("rollback", func(t *testing.T) {
		const assertion = "ASSERT_BOOTSTRAP_PARTIAL_FAILURE_ROLLS_BACK_STARTED_SESSION"
		t.Log("ASSERTION: " + assertion)
		_, manager, err := newServerRuntime(false)
		if err != nil {
			t.Fatal(err)
		}
		invalid := valid
		invalid.Profile.TrustDomain = ""
		if _, err := startBootstrap(context.Background(), manager, bootstrapConfig{Version: 1, Processes: []bootstrapProcessConfig{valid, invalid}}, 5*time.Second); err == nil {
			t.Fatalf("%s: startup unexpectedly succeeded", assertion)
		}
		assertBootstrapRecordsStopped(t, assertion, manager.Records())
		t.Log("PASS " + assertion)
	})

	t.Run("shutdown", func(t *testing.T) {
		const assertion = "ASSERT_BOOTSTRAP_NORMAL_SHUTDOWN_STOPS_STARTED_SESSION"
		t.Log("ASSERTION: " + assertion)
		_, manager, err := newServerRuntime(false)
		if err != nil {
			t.Fatal(err)
		}
		sessions, err := startBootstrap(context.Background(), manager, bootstrapConfig{Version: 1, Processes: []bootstrapProcessConfig{valid}}, 5*time.Second)
		if err != nil {
			t.Fatalf("%s: start=%v", assertion, err)
		}
		if err := stopBootstrap(context.Background(), manager, sessions); err != nil {
			t.Fatalf("%s: stop=%v", assertion, err)
		}
		assertBootstrapRecordsStopped(t, assertion, manager.Records())
		t.Log("PASS " + assertion)
	})
}

func assertBootstrapRecordsStopped(t *testing.T, assertion string, records []sessionruntime.Record) {
	t.Helper()
	if len(records) != 0 {
		t.Fatalf("%s: retained records=%+v", assertion, records)
	}
}
