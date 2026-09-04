package main

import (
	"fmt"
	"time"

	"lsp-trace/internal/provider"
)

const bootstrapProviderSchemaVersionV1 = "lsp-trace.bootstrap-provider.v1"

type bootstrapProviderDeclaration struct {
	SchemaVersion       string                     `json:"schema_version"`
	Identity            string                     `json:"identity"`
	Version             string                     `json:"version"`
	Protocol            bootstrapProviderProtocol  `json:"protocol"`
	Execution           bootstrapProviderExecution `json:"execution"`
	Capabilities        bootstrapCapabilities      `json:"capabilities"`
	ExecutableAvailable bool                       `json:"executable_available,omitempty"`
	ConformanceVerified bool                       `json:"conformance_verified,omitempty"`
	Limits              bootstrapProviderLimits    `json:"limits"`
}

type bootstrapProviderProtocol struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type bootstrapProviderExecution struct {
	Path        string   `json:"path"`
	Arguments   []string `json:"arguments,omitempty"`
	Directory   string   `json:"directory,omitempty"`
	Environment []string `json:"environment,omitempty"`
}

type bootstrapCapabilities struct {
	Relations  []string `json:"relations"`
	Languages  []string `json:"languages"`
	Frameworks []string `json:"frameworks"`
}

type bootstrapProviderLimits struct {
	RequestBytes      int `json:"request_bytes"`
	ResponseBytes     int `json:"response_bytes"`
	ProtocolMessages  int `json:"protocol_messages"`
	StderrBytes       int `json:"stderr_bytes"`
	WallTimeMillis    int `json:"wall_time_ms"`
	TerminationMillis int `json:"termination_grace_ms"`
}

func (config bootstrapConfig) providerDeclarations() ([]provider.Declaration, error) {
	declarations := make([]provider.Declaration, len(config.Providers))
	for i, configured := range config.Providers {
		if configured.SchemaVersion != bootstrapProviderSchemaVersionV1 {
			return nil, fmt.Errorf("bootstrap provider %d schema_version must be %q", i, bootstrapProviderSchemaVersionV1)
		}
		declarations[i] = provider.Declaration{
			Identity:            configured.Identity,
			Version:             configured.Version,
			Protocol:            provider.ProtocolIdentity{Name: configured.Protocol.Name, Version: configured.Protocol.Version},
			Executable:          configured.Execution.Path,
			Arguments:           append([]string(nil), configured.Execution.Arguments...),
			Directory:           configured.Execution.Directory,
			Environment:         append([]string(nil), configured.Execution.Environment...),
			ExecutableAvailable: configured.ExecutableAvailable,
			ConformanceVerified: configured.ConformanceVerified,
			Capabilities: provider.Capabilities{
				Relations:  append([]string(nil), configured.Capabilities.Relations...),
				Languages:  append([]string(nil), configured.Capabilities.Languages...),
				Frameworks: append([]string(nil), configured.Capabilities.Frameworks...),
			},
			Limits: provider.Limits{
				RequestBytes:     configured.Limits.RequestBytes,
				ResponseBytes:    configured.Limits.ResponseBytes,
				ProtocolMessages: configured.Limits.ProtocolMessages,
				StderrBytes:      configured.Limits.StderrBytes,
				WallTime:         time.Duration(configured.Limits.WallTimeMillis) * time.Millisecond,
				TerminationGrace: time.Duration(configured.Limits.TerminationMillis) * time.Millisecond,
			},
		}
	}
	return declarations, nil
}

func (config bootstrapConfig) provisionProviders() (provider.Provisioned, error) {
	declarations, err := config.providerDeclarations()
	if err != nil {
		return provider.Provisioned{}, err
	}
	provisioned, err := provider.Provision(declarations)
	if err != nil {
		return provider.Provisioned{}, fmt.Errorf("bootstrap providers: %w", err)
	}
	return provisioned, nil
}

func validateBootstrapProviders(config bootstrapConfig) error {
	_, err := config.provisionProviders()
	return err
}
