package main

import (
	"lsp-trace/internal/provider"
	"lsp-trace/sessionruntime"
)

type providerLifecycleFactory interface {
	NewRuntime(*provider.Registry) provider.Executor
	NewCollector(provider.Executor, provider.AdmissionResolver, provider.SemanticAdapter) (productionRelationCollector, error)
}

func composeMCPProviderLifecycleWith(
	manager *sessionruntime.Manager,
	registry *provider.Registry,
	admitter provider.AdmissionResolver,
	adapter provider.SemanticAdapter,
	factory providerLifecycleFactory,
	composeExecutors func(*hostSelectorRuntime),
) (*hostSelectorRuntime, error) {
	selected := newHostSelectorRuntime(manager, nil)
	composeExecutors(selected)
	providerRuntime := factory.NewRuntime(registry)
	factory.NewRuntime(registry)
	collector, err := factory.NewCollector(providerRuntime, admitter, adapter)
	if err != nil {
		return nil, err
	}
	return attachProductionRelationCollector(selected, collector), nil
}
