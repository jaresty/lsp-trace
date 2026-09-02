package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeProfileConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func withProfileDiscovery(t *testing.T) (workspace, userConfig string) {
	t.Helper()
	workspace = t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	userConfig = filepath.Join(xdg, "lsp-trace", "config.toml")
	return workspace, userConfig
}

func TestProfileDefaultDiscoveryAndFieldPrecedence(t *testing.T) {
	workspace, user := withProfileDiscovery(t)
	writeProfileConfig(t, user, `[profiles.go]
command = "user-server"
args = ["user-arg"]
env = ["TOKEN"]
language_ids = ["go"]
`)
	writeProfileConfig(t, filepath.Join(workspace, ".lsp-trace.toml"), `[profiles.go]
command = "project-server"
args = ["project-arg"]
`)
	t.Setenv("TOKEN", "super-secret")
	cfg, err := parse([]string{"--workspace", workspace, "--profile", "go", "--server-arg", "cli-arg", "--at", "main.go:1:1"})
	if err != nil {
		t.Fatalf("ASSERT_PROFILE_DISCOVERY_PRECEDENCE: %v", err)
	}
	if cfg.command != "project-server" || !reflect.DeepEqual([]string(cfg.args), []string{"cli-arg"}) || cfg.languageID != "go" || !reflect.DeepEqual([]string(cfg.env), []string{"TOKEN=super-secret"}) {
		t.Fatalf("ASSERT_PROFILE_DISCOVERY_PRECEDENCE: cfg=%#v", cfg)
	}
}

func TestProfileExplicitConfigReplacesDiscovery(t *testing.T) {
	workspace, user := withProfileDiscovery(t)
	writeProfileConfig(t, user, `[profiles.p]
command = "user"
`)
	writeProfileConfig(t, filepath.Join(workspace, ".lsp-trace.toml"), `[profiles.p]
command = "project"
`)
	explicit := filepath.Join(t.TempDir(), "explicit.toml")
	writeProfileConfig(t, explicit, `[profiles.p]
command = "explicit"
`)
	cfg, err := parse([]string{"--workspace", workspace, "--config", explicit, "--profile", "p", "--at", "main.go:1:1"})
	if err != nil || cfg.command != "explicit" {
		t.Fatalf("ASSERT_EXPLICIT_CONFIG_REPLACEMENT: command=%q err=%v", cfg.command, err)
	}
}

func TestProfileStrictDecodeAndLookupFailures(t *testing.T) {
	workspace, _ := withProfileDiscovery(t)
	cases := []struct{ name, body, profile, want string }{
		{"unknown", "[profiles.p]\ncommand=\"s\"\nunknown=true\n", "p", "unknown"},
		{"malformed", "[profiles.p\ncommand=\"s\"\n", "p", "TOML"},
		{"missing", "[profiles.other]\ncommand=\"s\"\n", "p", "profile"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			writeProfileConfig(t, path, tc.body)
			_, err := parse([]string{"--workspace", workspace, "--config", path, "--profile", tc.profile, "--at", "main.go:1:1"})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("ASSERT_PROFILE_FAIL_CLOSED_%s: err=%v", tc.name, err)
			}
		})
	}
}

func TestProfileEnvironmentReferencesAndSecretNonPersistence(t *testing.T) {
	workspace, _ := withProfileDiscovery(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	writeProfileConfig(t, path, `[profiles.p]
command = "server"
env = ["TOKEN", "AUTH=${AUTH}"]
`)
	t.Setenv("TOKEN", "token-secret")
	t.Setenv("AUTH", "auth-secret")
	cfg, err := parse([]string{"--workspace", workspace, "--config", path, "--profile", "p", "--at", "main.go:1:1"})
	if err != nil {
		t.Fatalf("ASSERT_PROFILE_ENV_REFERENCES: %v", err)
	}
	if !reflect.DeepEqual([]string(cfg.env), []string{"TOKEN=token-secret", "AUTH=auth-secret"}) {
		t.Fatalf("ASSERT_PROFILE_ENV_REFERENCES: env=%q", cfg.env)
	}
	result, _ := execute(t.Context(), cfg)
	encoded := resultJSON(t, result)
	if strings.Contains(encoded, "token-secret") || strings.Contains(encoded, "auth-secret") {
		t.Fatalf("ASSERT_PROFILE_SECRET_NON_PERSISTENCE: %s", encoded)
	}

	t.Setenv("AUTH", "")
	if err := os.Unsetenv("AUTH"); err != nil {
		t.Fatal(err)
	}
	_, err = parse([]string{"--workspace", workspace, "--config", path, "--profile", "p", "--at", "main.go:1:1"})
	if err == nil || !strings.Contains(err.Error(), "AUTH") {
		t.Fatalf("ASSERT_PROFILE_MISSING_ENV: %v", err)
	}

	writeProfileConfig(t, path, `[profiles.p]
command = "server"
env = ["TOKEN=plaintext"]
`)
	_, err = parse([]string{"--workspace", workspace, "--config", path, "--profile", "p", "--at", "main.go:1:1"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "environment") {
		t.Fatalf("ASSERT_PROFILE_PLAINTEXT_SECRET_REJECTED: %v", err)
	}
}

func resultJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestProfileIncomingSliceParityAndExplicitSelection(t *testing.T) {
	workspace, _ := withProfileDiscovery(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	writeProfileConfig(t, path, `[profiles.go]
command = "profile-server"
args = ["--stdio"]
language_ids = ["go", "gomod"]
`)
	incoming, err := parse([]string{"--workspace", workspace, "--config", path, "--profile", "go", "--server", "override", "--at", "main.go:1:1"})
	if err != nil {
		t.Fatalf("ASSERT_PROFILE_INCOMING_PARITY: %v", err)
	}
	slice, err := parseSlice([]string{"--workspace", workspace, "--config", path, "--profile", "go", "--server", "override", "--from-file", "main.go"})
	if err != nil {
		t.Fatalf("ASSERT_PROFILE_SLICE_PARITY: %v", err)
	}
	if incoming.command != slice.command || !reflect.DeepEqual([]string(incoming.args), []string(slice.args)) || incoming.languageID != slice.languageID {
		t.Fatalf("ASSERT_PROFILE_INCOMING_SLICE_PARITY: incoming=%#v slice=%#v", incoming, slice)
	}

	_, err = parse([]string{"--workspace", workspace, "--at", "main.go:1:1"})
	if err == nil || !strings.Contains(err.Error(), "--server") {
		t.Fatalf("ASSERT_NO_AUTOMATIC_PROFILE_SELECTION: %v", err)
	}
}

func TestLegacyFlagsRemainUnchangedWithoutProfile(t *testing.T) {
	workspace, _ := withProfileDiscovery(t)
	writeProfileConfig(t, filepath.Join(workspace, ".lsp-trace.toml"), "not valid toml")
	cfg, err := parse([]string{"--workspace", workspace, "--server", "legacy", "--server-arg", "x", "--server-env", "A=1", "--language-id", "go", "--at", "main.go:1:1"})
	if err != nil || cfg.command != "legacy" || cfg.languageID != "go" || !reflect.DeepEqual([]string(cfg.args), []string{"x"}) || !reflect.DeepEqual([]string(cfg.env), []string{"A=1"}) {
		t.Fatalf("ASSERT_LEGACY_FLAGS_UNCHANGED: cfg=%#v err=%v", cfg, err)
	}
}
