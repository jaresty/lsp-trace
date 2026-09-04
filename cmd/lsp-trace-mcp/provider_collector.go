package main

import (
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/incomingops"
	"lsp-trace/sliceops"
)

type productionRelationCollector interface {
	CollectRelations(context.Context, []string, json.RawMessage) (json.RawMessage, error)
}

var _ incomingops.RelationCollector = (*hostSelectorRuntime)(nil)
var _ sliceops.RelationCollector = (*hostSelectorRuntime)(nil)

func (r *hostSelectorRuntime) CollectRelations(ctx context.Context, relations []string, raw json.RawMessage) (json.RawMessage, error) {
	if len(relations) == 0 {
		return nil, nil
	}
	if r == nil || r.relationCollector == nil {
		return nil, errors.New("production relation collector is not provisioned")
	}
	return r.relationCollector.CollectRelations(ctx, append([]string(nil), relations...), append(json.RawMessage(nil), raw...))
}

func attachProductionRelationCollector(runtime *hostSelectorRuntime, collector productionRelationCollector) *hostSelectorRuntime {
	if runtime != nil {
		runtime.relationCollector = collector
	}
	return runtime
}
