package main

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/incomingops"
	"lsp-trace/internal/provider"
	"lsp-trace/sliceops"
)

type productionRelationCollector interface {
	CollectRelations(context.Context, []string, json.RawMessage) (provider.Result, error)
}

var _ incomingops.RelationCollector = (*hostSelectorRuntime)(nil)
var _ sliceops.RelationCollector = (*hostSelectorRuntime)(nil)

func (r *hostSelectorRuntime) CollectRelations(ctx context.Context, relations []string, raw json.RawMessage) (provider.Result, error) {
	if len(relations) == 0 {
		return provider.Result{}, errors.New("production relation collector requires non-empty relations")
	}
	if r == nil || r.relationCollector == nil {
		return provider.Result{}, errors.New("production relation collector is not provisioned")
	}
	return r.relationCollector.CollectRelations(ctx, append([]string(nil), relations...), append(json.RawMessage(nil), raw...))
}

func attachProductionRelationCollector(runtime *hostSelectorRuntime, collector productionRelationCollector) *hostSelectorRuntime {
	if runtime != nil {
		runtime.relationCollector = collector
	}
	return runtime
}
