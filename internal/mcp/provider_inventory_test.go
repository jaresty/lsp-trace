package mcp

import (
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/provider"
)

func TestCapabilitiesPublishConfiguredProviderInventory(t *testing.T) {
	declaration := provider.Declaration{
		Identity: "alpha@1", Version: "1", Protocol: provider.ProtocolIdentity{Name: "provider-protocol", Version: "1"},
		Executable: filepath.Join(string(filepath.Separator), "host", "alpha"), ExecutableAvailable: true,
		Capabilities: provider.Capabilities{Relations: []string{"CALLS"}, Languages: []string{"language-a"}, Frameworks: []string{"framework-a"}},
		Limits:       provider.Limits{RequestBytes: 1, ResponseBytes: 1, ProtocolMessages: 1, StderrBytes: 1, WallTime: time.Millisecond, TerminationGrace: time.Millisecond},
	}
	provisioned, err := provider.Provision([]provider.Declaration{declaration})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistryWithProviderInventory(false, false, provider.NewConfiguredInventory(provisioned))
	capabilities := registry.Capabilities()
	entries, ok := capabilities["configured_providers"].([]provider.InventoryEntry)
	if !ok || len(entries) != 1 || entries[0].Identity != "alpha@1" || entries[0].Readiness != provider.ExecutableAvailable {
		t.Fatalf("ASSERT_CONFIGURED_PROVIDER_INVENTORY_CAPABILITIES: configured_providers=%#v", capabilities["configured_providers"])
	}
	entries[0].Identity = "mutated@1"
	fresh := registry.Capabilities()["configured_providers"].([]provider.InventoryEntry)
	if fresh[0].Identity != "alpha@1" {
		t.Fatalf("ASSERT_CONFIGURED_PROVIDER_PROCESS_LIFETIME_IMMUTABLE: configured_providers=%#v", fresh)
	}
}

func TestCapabilitiesWithoutProvidersRemainCompatible(t *testing.T) {
	capabilities := NewRegistry(false).Capabilities()
	entries, ok := capabilities["configured_providers"].([]provider.InventoryEntry)
	if !ok || len(entries) != 0 || capabilities["capabilities_version"] != "1" {
		t.Fatalf("ASSERT_PROVIDER_INVENTORY_COMPATIBILITY: capabilities=%#v", capabilities)
	}
}
