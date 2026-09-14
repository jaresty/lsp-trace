package acquisitionorchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/session"
)

const privateCensusBatchRoute = "private-census-batch"

type PlannedBatchRequest struct {
	SessionID        string
	Generation       uint64
	RequestID        string
	Manifest         acquisitionengine.Manifest
	CanonicalSeedsV2 []byte
}

type PlannedBatchResult struct {
	SessionID  string
	Generation uint64
	RawV5      []byte
}

func ExecutePlannedBatch(ctx context.Context, runtime Runtime, request PlannedBatchRequest) (PlannedBatchResult, *operation.Failure) {
	fail := func(code string, err error) (PlannedBatchResult, *operation.Failure) {
		return PlannedBatchResult{}, &operation.Failure{Code: code, Err: err}
	}
	if runtime == nil {
		return fail(operation.FailureInternal, fmt.Errorf("managed runtime required"))
	}
	if request.SessionID == "" || request.Generation == 0 || request.RequestID == "" {
		return fail(operation.FailureInvalidInput, fmt.Errorf("exact planned batch identity required"))
	}
	if err := ctx.Err(); err != nil {
		return fail(string(session.RequestCancelled), err)
	}
	raw, err := json.Marshal(acquisitionengine.Input{SessionID: request.SessionID, Generation: request.Generation, SeedManifest: request.Manifest, OutputVersion: graphprovenance.VersionV5})
	if err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	op := operation.Request{Name: acquisitionengine.SliceV3, RequestID: request.RequestID, Input: raw, RetainedSeedSpec: bytes.Clone(request.CanonicalSeedsV2)}
	input, canonical, err := acquisitionengine.CanonicalInput(op.Input)
	if err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	op.Input = canonical
	if input.SessionID != request.SessionID || input.Generation != request.Generation {
		return fail(operation.FailureInvalidInput, fmt.Errorf("planned batch identity drift"))
	}
	workspace := ""
	for _, record := range runtime.Records() {
		if record.SessionID == request.SessionID && record.Generation == request.Generation {
			if workspace != "" {
				return fail(operation.FailureInvalidInput, fmt.Errorf("ambiguous host workspace"))
			}
			workspace = record.Profile.Workspace().String()
		}
	}
	if workspace == "" {
		return fail(operation.FailureInvalidInput, fmt.Errorf("host workspace unavailable"))
	}
	binding := acquisitionauthority.Binding{Operation: acquisitionengine.SliceV3, Route: privateCensusBatchRoute, RequestID: request.RequestID, Workspace: workspace, SessionID: request.SessionID, Generation: request.Generation, Input: canonical}
	authority, err := acquisitionauthority.MintSeedAuthority(binding, op.RetainedSeedSpec, seedbinding.CallerAssertedLocal, false, "", true)
	if err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	result, failure := acquisitionengine.ExecuteAuthorized(ctx, runtime, op, privateCensusBatchRoute, authority, nil, binding)
	if failure != nil {
		return PlannedBatchResult{}, failure
	}
	return PlannedBatchResult{SessionID: request.SessionID, Generation: request.Generation, RawV5: bytes.Clone(result.Artifact)}, nil
}
