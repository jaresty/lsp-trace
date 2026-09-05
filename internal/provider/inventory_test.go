package provider

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfiguredProviderInventoryPublishesImmutableGenericFacts(t *testing.T) {
	const assertion = "ASSERT_CONFIGURED_PROVIDER_INVENTORY_CAPABILITIES"
	t.Log("ASSERTION: " + assertion)
	declaration := admissionDeclaration("alpha@1", "PASSES_CALLBACK")
	declaration.ExecutableAvailable = true
	declaration.ConformanceVerified = true
	provisioned := mustProvisionAdmission(t, declaration)

	inventory := NewConfiguredInventory(provisioned)
	got := inventory.Entries()
	if len(got) != 1 || got[0].Identity != declaration.Identity || got[0].Protocol != declaration.Protocol || !reflect.DeepEqual(got[0].Capabilities, declaration.Capabilities) {
		t.Fatalf("%s: entries=%+v", assertion, got)
	}
	if !got[0].Configured || !got[0].ExecutableAvailable || !got[0].ConformanceVerified || got[0].Readiness != Ready {
		t.Fatalf("ASSERT_CONFIGURED_PROVIDER_READINESS_CLOSED_AND_DISTINCT: entry=%+v", got[0])
	}

	got[0].Identity = "mutated@1"
	got[0].Capabilities.Relations[0] = "CALLS"
	if fresh := inventory.Entries()[0]; fresh.Identity != declaration.Identity || fresh.Capabilities.Relations[0] != "PASSES_CALLBACK" {
		t.Fatalf("ASSERT_CONFIGURED_PROVIDER_PROCESS_LIFETIME_IMMUTABLE: entry=%+v", fresh)
	}
}

func TestConfiguredProviderInventoryReadinessStates(t *testing.T) {
	for _, tc := range []struct {
		name, id                string
		executable, conformance bool
		want                    Readiness
	}{
		{"declared", "declared@1", false, false, Declared},
		{"executable", "executable@1", true, false, ExecutableAvailable},
		{"conformance", "conformance@1", false, true, ConformanceVerified},
		{"ready", "ready@1", true, true, Ready},
	} {
		t.Run(tc.name, func(t *testing.T) {
			declaration := admissionDeclaration(tc.id, "CALLS")
			declaration.Executable = filepath.Join(string(filepath.Separator), "host", tc.id)
			declaration.ExecutableAvailable = tc.executable
			declaration.ConformanceVerified = tc.conformance
			entry := NewConfiguredInventory(mustProvisionAdmission(t, declaration)).Entries()[0]
			if entry.Readiness != tc.want || entry.Configured != true || entry.ExecutableAvailable != tc.executable || entry.ConformanceVerified != tc.conformance {
				t.Fatalf("ASSERT_CONFIGURED_PROVIDER_READINESS_CLOSED_AND_DISTINCT: got=%+v want=%s", entry, tc.want)
			}
		})
	}
}

func TestConfiguredProviderInventoryDoesNotLaunch(t *testing.T) {
	directory := t.TempDir()
	marker := filepath.Join(directory, "started")
	executable := filepath.Join(directory, "must-not-be-launched")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\ntouch \""+marker+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	declaration := admissionDeclaration("missing@1", "CALLS")
	declaration.Executable = executable
	inventory := NewConfiguredInventory(mustProvisionAdmission(t, declaration))
	if got := inventory.Entries(); len(got) != 1 || got[0].ExecutableAvailable {
		t.Fatalf("ASSERT_CONFIGURED_PROVIDER_INVENTORY_NO_LAUNCH: entries=%+v", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_CONFIGURED_PROVIDER_INVENTORY_NO_LAUNCH: provider process side effect observed: %v", err)
	}
}
