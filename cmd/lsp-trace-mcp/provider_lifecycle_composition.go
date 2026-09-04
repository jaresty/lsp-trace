package main

import (
	"errors"

	"lsp-trace/internal/provider"
)

type providerLifecycleFactory interface {
	NewRuntime(*provider.Registry) provider.Executor
	NewCollector(provider.Executor, provider.AdmissionResolver, provider.SemanticAdapter) (productionRelationCollector, error)
}

type productionProviderLifecycleFactory struct{}

func (productionProviderLifecycleFactory) NewRuntime(registry *provider.Registry) provider.Executor {
	return provider.NewRuntime(registry)
}

func (productionProviderLifecycleFactory) NewCollector(runtime provider.Executor, admitter provider.AdmissionResolver, adapter provider.SemanticAdapter) (productionRelationCollector, error) {
	return provider.NewCollector(runtime, admitter, adapter)
}

func composeMCPProviderLifecycle(
	selected *hostSelectorRuntime,
	registry *provider.Registry,
	admitter provider.AdmissionResolver,
	adapter provider.SemanticAdapter,
	composeExecutors func(*hostSelectorRuntime),
) (*hostSelectorRuntime, error) {
	return composeMCPProviderLifecycleWith(selected, registry, admitter, adapter, productionProviderLifecycleFactory{}, composeExecutors)
}

func composeMCPProviderLifecycleWith(
	selected *hostSelectorRuntime,
	registry *provider.Registry,
	admitter provider.AdmissionResolver,
	adapter provider.SemanticAdapter,
	factory providerLifecycleFactory,
	composeExecutors func(*hostSelectorRuntime),
) (*hostSelectorRuntime, error) {
	if selected == nil || registry == nil || admitter == nil || adapter == nil || factory == nil || composeExecutors == nil {
		return nil, errors.New("provider lifecycle composition requires host selector runtime, registry, admission resolver, semantic adapter, factory, and executor composer")
	}
	providerRuntime := factory.NewRuntime(registry)
	collector, err := factory.NewCollector(providerRuntime, admitter, adapter)
	if err != nil {
		return nil, err
	}
	attachProductionRelationCollector(selected, collector)
	composeExecutors(selected)
	return selected, nil
}
