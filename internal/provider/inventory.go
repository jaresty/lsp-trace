package provider

// Readiness is the closed host-declared readiness state of a configured provider.
type Readiness string

const (
	Declared            Readiness = "DECLARED"
	ExecutableAvailable Readiness = "EXECUTABLE_AVAILABLE"
	ConformanceVerified Readiness = "CONFORMANCE_VERIFIED"
	Ready               Readiness = "READY"
)

// InventoryEntry is immutable declarative provider metadata. Configuration,
// executable availability, and conformance evidence remain separate from the
// readiness value derived from them.
type InventoryEntry struct {
	Identity            string           `json:"identity"`
	Version             string           `json:"version"`
	Protocol            ProtocolIdentity `json:"protocol"`
	Capabilities        Capabilities     `json:"capabilities"`
	Configured          bool             `json:"configured"`
	ExecutableAvailable bool             `json:"executable_available"`
	ConformanceVerified bool             `json:"conformance_verified"`
	Readiness           Readiness        `json:"readiness"`
}

// ConfiguredInventory is a process-lifetime snapshot of host declarations.
type ConfiguredInventory struct{ entries []InventoryEntry }

func NewConfiguredInventory(provisioned Provisioned) ConfiguredInventory {
	entries := make([]InventoryEntry, len(provisioned.Declarations))
	for i, declaration := range provisioned.Declarations {
		entries[i] = InventoryEntry{
			Identity: declaration.Identity, Version: declaration.Version, Protocol: declaration.Protocol,
			Capabilities: cloneCapabilities(declaration.Capabilities), Configured: true,
			ExecutableAvailable: declaration.ExecutableAvailable,
			ConformanceVerified: declaration.ConformanceVerified,
			Readiness:           readiness(declaration.ExecutableAvailable, declaration.ConformanceVerified),
		}
	}
	return ConfiguredInventory{entries: entries}
}

func (inventory ConfiguredInventory) Entries() []InventoryEntry {
	entries := make([]InventoryEntry, len(inventory.entries))
	for i, entry := range inventory.entries {
		entries[i] = entry
		entries[i].Capabilities = cloneCapabilities(entry.Capabilities)
	}
	return entries
}

func readiness(executableAvailable, conformanceVerified bool) Readiness {
	switch {
	case executableAvailable && conformanceVerified:
		return Ready
	case executableAvailable:
		return ExecutableAvailable
	case conformanceVerified:
		return ConformanceVerified
	default:
		return Declared
	}
}

func cloneCapabilities(capabilities Capabilities) Capabilities {
	return Capabilities{
		Relations:  append([]string(nil), capabilities.Relations...),
		Languages:  append([]string(nil), capabilities.Languages...),
		Frameworks: append([]string(nil), capabilities.Frameworks...),
	}
}
