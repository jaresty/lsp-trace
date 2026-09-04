package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"lsp-trace/internal/provider"
)

type compositionCollector struct{ calls int }

func (c *compositionCollector) CollectRelations(context.Context, []string, json.RawMessage) (json.RawMessage, error) {
	c.calls++
	return json.RawMessage(`{"provider":"ok"}`), nil
}

type compositionFactory struct {
	runtimeCalls   int
	collectorCalls int
	collector      productionRelationCollector
}

func (f *compositionFactory) NewRuntime(*provider.Registry) provider.Executor {
	f.runtimeCalls++
	return compositionExecutor{}
}

func (f *compositionFactory) NewCollector(provider.Executor, provider.AdmissionResolver, provider.SemanticAdapter) (productionRelationCollector, error) {
	f.collectorCalls++
	if f.collector == nil {
		return nil, errors.New("missing test collector")
	}
	return f.collector, nil
}

type compositionExecutor struct{}

func (compositionExecutor) Execute(context.Context, string, json.RawMessage, provider.Limits) provider.Receipt {
	return provider.Receipt{}
}

type compositionAdmitter struct{}

func (compositionAdmitter) Admit(context.Context, provider.Selection) (provider.Admission, error) {
	return provider.Admission{}, nil
}

type compositionAdapter struct{}

func (compositionAdapter) Adapt(context.Context, provider.StrictCollectorRequest, provider.Receipt) (json.RawMessage, error) {
	return nil, nil
}

func composeLifecycleFixture(t *testing.T) (*hostSelectorRuntime, *compositionFactory, *compositionCollector, bool) {
	t.Helper()
	collector := &compositionCollector{}
	factory := &compositionFactory{collector: collector}
	attachedBeforeExecutors := false
	selected, err := composeMCPProviderLifecycleWith(
		newHostSelectorRuntime(nil, nil),
		provider.NewRegistry(),
		compositionAdmitter{},
		compositionAdapter{},
		factory,
		func(runtime *hostSelectorRuntime) { attachedBeforeExecutors = runtime.relationCollector == collector },
	)
	if err != nil {
		t.Fatal(err)
	}
	return selected, factory, collector, attachedBeforeExecutors
}

func TestMCPProviderLifecycleComposition(t *testing.T) {
	t.Run("production wrapper", func(t *testing.T) {
		attachedBeforeExecutors := false
		selected, err := composeMCPProviderLifecycle(newHostSelectorRuntime(nil, nil), provider.NewRegistry(), compositionAdmitter{}, compositionAdapter{}, func(runtime *hostSelectorRuntime) {
			attachedBeforeExecutors = runtime.relationCollector != nil
		})
		if err != nil || selected == nil || selected.relationCollector == nil || !attachedBeforeExecutors {
			t.Fatalf("ASSERT_MCP_PROVIDER_LIFECYCLE_PRODUCTION_COMPOSITION: selected=%v attached=%v err=%v", selected != nil, attachedBeforeExecutors, err)
		}
		t.Log("PASS ASSERT_MCP_PROVIDER_LIFECYCLE_PRODUCTION_COMPOSITION")
	})

	t.Run("exactly one", func(t *testing.T) {
		selected, factory, collector, _ := composeLifecycleFixture(t)
		if factory.runtimeCalls != 1 || factory.collectorCalls != 1 || selected.relationCollector != collector {
			t.Fatalf("ASSERT_MCP_PROVIDER_LIFECYCLE_EXACTLY_ONE: runtime_calls=%d collector_calls=%d attached=%v", factory.runtimeCalls, factory.collectorCalls, selected.relationCollector == collector)
		}
		t.Log("PASS ASSERT_MCP_PROVIDER_LIFECYCLE_EXACTLY_ONE")
	})

	t.Run("attached before executors", func(t *testing.T) {
		_, _, _, attachedBeforeExecutors := composeLifecycleFixture(t)
		if !attachedBeforeExecutors {
			t.Fatal("ASSERT_MCP_PROVIDER_LIFECYCLE_ATTACHED_BEFORE_EXECUTORS: attached=false")
		}
		t.Log("PASS ASSERT_MCP_PROVIDER_LIFECYCLE_ATTACHED_BEFORE_EXECUTORS")
	})

	t.Run("omitted relations", func(t *testing.T) {
		selected, _, collector, _ := composeLifecycleFixture(t)
		artifact, err := selected.CollectRelations(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil || artifact != nil || collector.calls != 0 {
			t.Fatalf("ASSERT_MCP_OMITTED_RELATIONS_BYPASS_COLLECTOR: calls=%d artifact=%s err=%v", collector.calls, artifact, err)
		}
		t.Log("PASS ASSERT_MCP_OMITTED_RELATIONS_BYPASS_COLLECTOR")
	})
}
