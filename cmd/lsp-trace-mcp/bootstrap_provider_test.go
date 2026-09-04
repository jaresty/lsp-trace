package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/provider"
)

type bootstrapProviderSource interface {
	providerDeclarations() ([]provider.Declaration, error)
	provisionProviders() (provider.Provisioned, error)
}

func writeBootstrapProviderConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bootstrap.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validBootstrapProviderJSON(executable, directory string) string {
	return `{
  "version": 1,
  "providers": [{
    "schema_version": "lsp-trace.bootstrap-provider.v1",
    "identity": "ember-relations@1",
    "version": "1.2.3",
    "protocol": {"name": "lsp-provider", "version": "1"},
    "execution": {"path": "` + executable + `", "arguments": ["--stdio"], "directory": "` + directory + `", "environment": ["MODE=test"]},
    "capabilities": {"relations": ["CALLS", "BINDS_ARGUMENT"], "languages": ["typescript"], "frameworks": ["ember"]},
    "limits": {"request_bytes": 1024, "response_bytes": 2048, "protocol_messages": 8, "stderr_bytes": 4096, "wall_time_ms": 5000, "termination_grace_ms": 250}
  }]
}`
}

func TestBootstrapProviderDeclarationConvertsDeterministically(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_DETERMINISTIC_PROVISION_INPUT"
	t.Log("ASSERTION: " + assertion)
	config, err := loadBootstrapConfig(writeBootstrapProviderConfig(t, validBootstrapProviderJSON("/opt/providers/ember", "/workspace")))
	if err != nil {
		t.Fatalf("%s: valid provider config rejected: %v", assertion, err)
	}
	source, ok := any(config).(bootstrapProviderSource)
	if !ok {
		t.Fatalf("%s: bootstrap config has no provider conversion surface", assertion)
	}
	provisioned, err := source.provisionProviders()
	if err != nil {
		t.Fatalf("%s: provision conversion failed: %v", assertion, err)
	}
	if len(provisioned.Declarations) != 1 {
		t.Fatalf("%s: declarations=%d", assertion, len(provisioned.Declarations))
	}
	got := provisioned.Declarations[0]
	if got.Identity != "ember-relations@1" || got.Version != "1.2.3" || got.Protocol != (provider.ProtocolIdentity{Name: "lsp-provider", Version: "1"}) || got.Executable != "/opt/providers/ember" || got.Directory != "/workspace" {
		t.Fatalf("%s: identity/protocol/execution mismatch: %+v", assertion, got)
	}
	if !reflect.DeepEqual(got.Arguments, []string{"--stdio"}) || !reflect.DeepEqual(got.Environment, []string{"MODE=test"}) || !reflect.DeepEqual(got.Capabilities.Relations, []string{"BINDS_ARGUMENT", "CALLS"}) {
		t.Fatalf("%s: copied/canonical fields mismatch: %+v", assertion, got)
	}
}

func TestBootstrapProviderDeclarationRejectsUnknownNestedField(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_DISALLOW_UNKNOWN_FIELDS"
	t.Log("ASSERTION: " + assertion)
	body := strings.Replace(validBootstrapProviderJSON("/provider", "/workspace"), `"version": "1"`, `"version": "1", "semantic_adapter": "forbidden"`, 1)
	if _, err := loadBootstrapConfig(writeBootstrapProviderConfig(t, body)); err == nil || !strings.Contains(err.Error(), `unknown field "semantic_adapter"`) {
		t.Fatalf("%s: err=%v", assertion, err)
	}
}

func TestBootstrapProviderDeclarationRejectsRelativePathsAndHostLimitOverflow(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_ABSOLUTE_AND_HOST_BOUNDED"
	t.Log("ASSERTION: " + assertion)
	for name, tc := range map[string]struct {
		body, want string
	}{
		"relative executable": {validBootstrapProviderJSON("provider", "/workspace"), "executable path must be absolute"},
		"relative directory":  {validBootstrapProviderJSON("/provider", "workspace"), "working directory must be absolute"},
		"limit overflow":      {strings.Replace(validBootstrapProviderJSON("/provider", "/workspace"), `"request_bytes": 1024`, `"request_bytes": 16777217`, 1), "within host maxima"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadBootstrapConfig(writeBootstrapProviderConfig(t, tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: err=%v want=%q", assertion, err, tc.want)
			}
		})
	}
}

func TestBootstrapProviderDeclarationRejectsUnstableContractFields(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_STABLE_VERSIONED_CONTRACT"
	t.Log("ASSERTION: " + assertion)
	valid := validBootstrapProviderJSON("/provider", "/workspace")
	for name, tc := range map[string]struct {
		body, want string
	}{
		"schema version":    {strings.Replace(valid, bootstrapProviderSchemaVersionV1, "lsp-trace.bootstrap-provider.v2", 1), "schema_version"},
		"identity":          {strings.Replace(valid, "ember-relations@1", "unstable identity", 1), "stable name@version"},
		"provider version":  {strings.Replace(valid, `"version": "1.2.3"`, `"version": ""`, 1), "provider version"},
		"protocol identity": {strings.Replace(valid, `"name": "lsp-provider"`, `"name": ""`, 1), "protocol identity/version"},
		"capabilities":      {strings.Replace(valid, `"languages": ["typescript"]`, `"languages": []`, 1), "must be non-empty"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadBootstrapConfig(writeBootstrapProviderConfig(t, tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: err=%v want=%q", assertion, err, tc.want)
			}
		})
	}
}

func TestBootstrapProviderDeclarationConversionCopiesMutableFields(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_PROVIDER_DECLARATION_IMMUTABLE_COPY"
	t.Log("ASSERTION: " + assertion)
	config, err := loadBootstrapConfig(writeBootstrapProviderConfig(t, validBootstrapProviderJSON("/provider", "/workspace")))
	if err != nil {
		t.Fatalf("%s: load: %v", assertion, err)
	}
	source, ok := any(config).(bootstrapProviderSource)
	if !ok {
		t.Fatalf("%s: bootstrap config has no provider conversion surface", assertion)
	}
	first, err := source.providerDeclarations()
	if err != nil {
		t.Fatalf("%s: first conversion: %v", assertion, err)
	}
	first[0].Arguments[0] = "changed"
	first[0].Environment[0] = "CHANGED=1"
	first[0].Capabilities.Relations[0] = "UPDATES_STATE"
	second, err := source.providerDeclarations()
	if err != nil {
		t.Fatalf("%s: second conversion: %v", assertion, err)
	}
	if second[0].Arguments[0] != "--stdio" || second[0].Environment[0] != "MODE=test" || second[0].Capabilities.Relations[0] != "CALLS" {
		t.Fatalf("%s: conversion retained mutable aliases: %+v", assertion, second[0])
	}
}
