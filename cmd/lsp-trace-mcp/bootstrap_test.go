package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/seedbinding"
	"lsp-trace/sessionruntime"
)

func TestBootstrapConfigIsStrictAndHostOwned(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_CONFIG_REJECTS_UNDECLARED_AUTHORITY"
	t.Log("ASSERTION: " + assertion)
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "bootstrap.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	t.Run("unknown field", func(t *testing.T) {
		body := `{"version":1,"processes":[{"profile":{"trust_domain":"test","workspace":"/workspace","profile":"go","environment_reference":"local"},"execution":{"path":"/server","directory":"/workspace"},"mcp_selected_command":"forbidden"}]}`
		if _, err := loadBootstrapConfig(write(t, body)); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("%s: err=%v", assertion, err)
		}
	})
	t.Run("trailing value", func(t *testing.T) {
		body := `{"version":1,"processes":[{"profile":{"trust_domain":"test","workspace":"/workspace","profile":"go","environment_reference":"local"},"execution":{"path":"/server","directory":"/workspace"}}]} {}`
		if _, err := loadBootstrapConfig(write(t, body)); err == nil || !strings.Contains(err.Error(), "one JSON value") {
			t.Fatalf("%s: err=%v", assertion, err)
		}
	})
	t.Run("relative execution", func(t *testing.T) {
		body := `{"version":1,"processes":[{"profile":{"trust_domain":"test","workspace":"/workspace","profile":"go","environment_reference":"local"},"execution":{"path":"server","directory":"/workspace"}}]}`
		if _, err := loadBootstrapConfig(write(t, body)); err == nil || !strings.Contains(err.Error(), "must be absolute") {
			t.Fatalf("%s: err=%v", assertion, err)
		}
	})
	t.Log("PASS " + assertion)
}

func TestBootstrapSeedBindingUsesManagedProviderWithoutExternalValidator(t *testing.T) {
	workspace := t.TempDir()
	manifest := &seedbinding.Manifest{SchemaVersion: seedbinding.VersionV2, ID: "seed", SourceRevision: "rev", Validator: seedbinding.ValidatorIdentity{Language: "csharp", Authority: "MANAGED_LSP", Name: "csharp-ls", Version: "host-pinned"}}
	receipt := &seedbinding.HostCustodyReceipt{Authenticated: true, Repository: workspace, SourceRevision: "rev", TargetPath: "a.cs", TargetSourceSHA256: strings.Repeat("a", 64), SeedManifestSHA256: strings.Repeat("b", 64)}
	process := bootstrapProcessConfig{Profile: bootstrapProfileIdentity{TrustDomain: "test", Workspace: workspace, Profile: "csharp", EnvironmentReference: "host"}, Execution: managedExecutionAuthority{Path: "/managed-provider", Directory: workspace}, SeedBinding: manifest, SeedCustodyReceipt: receipt}
	config := bootstrapConfig{Version: 1, Processes: []bootstrapProcessConfig{process}}
	raw, _ := json.Marshal(config)
	path := filepath.Join(t.TempDir(), "bootstrap.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadBootstrapConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareBootstrap(loaded)
	if err != nil || prepared[0].seedBinding != loaded.Processes[0].SeedBinding || seedAuthoritiesFromConfig(loaded) == nil {
		t.Fatalf("ASSERT_BOOTSTRAP_SEED_MANIFEST_AND_HOST_AUTHORITY_EXACT: prepared=%+v err=%v", prepared, err)
	}
	withoutReceipt := loaded
	withoutReceipt.Processes[0].SeedCustodyReceipt = nil
	raw, _ = json.Marshal(withoutReceipt)
	_ = os.WriteFile(path, raw, 0600)
	if _, err := loadBootstrapConfig(path); err == nil {
		t.Fatal("ASSERT_SEED_V2_WITHOUT_HOST_RECEIPT_FAILS_CLOSED")
	}
	loaded.Processes[0].SeedBinding.SchemaVersion = "unknown"
	raw, _ = json.Marshal(loaded)
	_ = os.WriteFile(path, raw, 0600)
	if _, err := loadBootstrapConfig(path); err == nil {
		t.Fatal("ASSERT_BOOTSTRAP_SEED_BINDING_VERSION_REJECTED")
	}
}

func TestBootstrapHostOwnedAliases(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_HOST_ALIAS_STRICT_UNIQUE"
	workspace := t.TempDir()
	write := func(body string) string {
		path := filepath.Join(t.TempDir(), "bootstrap.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	body := `{"version":1,"processes":[{"alias":"primary","profile":{"trust_domain":"test","workspace":"` + workspace + `","profile":"go","environment_reference":"local"},"execution":{"path":"/server","directory":"` + workspace + `"}}]}`
	config, err := loadBootstrapConfig(write(body))
	if err != nil || len(config.Processes) != 1 {
		t.Fatalf("%s: valid alias err=%v config=%+v", assertion, err, config)
	}
	config.Processes = append(config.Processes, config.Processes[0])
	config.Processes[1].Profile.Profile = "other"
	if _, err := prepareBootstrap(config); err == nil || !strings.Contains(err.Error(), "alias") {
		t.Fatalf("%s: duplicate alias err=%v", assertion, err)
	}
}

func TestPrepareBootstrapRejectsDuplicateSessionIdentity(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_DUPLICATE_SESSION_IDENTITY_REJECTED"
	workspace := t.TempDir()
	process := bootstrapProcessConfig{
		Profile:   bootstrapProfileIdentity{TrustDomain: "bootstrap", Workspace: workspace, Profile: "fake", EnvironmentReference: "hermetic"},
		Execution: managedExecutionAuthority{Path: "/fake-lsp", Directory: workspace},
	}
	_, err := prepareBootstrap(bootstrapConfig{Version: 1, Processes: []bootstrapProcessConfig{process, process}})
	if err == nil || !strings.Contains(err.Error(), "duplicates session identity") {
		t.Fatalf("%s: err=%v", assertion, err)
	}
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
		invalid.Profile.Profile = "missing"
		invalid.Execution.Path = filepath.Join(workspace, "missing-lsp")
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
