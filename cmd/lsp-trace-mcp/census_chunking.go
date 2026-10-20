package main

import (
	"bytes"
	"context"
	"errors"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/censuschunkingruntime"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

type privateCensusChunkCompletion struct {
	terminalSelector string
	terminalIndex    []byte
}

func clonePrivateCensusChunkCompletion(in *privateCensusChunkCompletion) (*privateCensusChunkCompletion, error) {
	if in == nil || in.terminalSelector == "" || len(in.terminalIndex) == 0 {
		return nil, errors.New("private census chunk completion missing")
	}
	index, err := censuschunking.ParseTerminalIndex(in.terminalIndex)
	if err != nil || index.ImmutableSelector != in.terminalSelector || index.Authority != 0 || index.Accepted || index.SourceGraphComplete != "UNKNOWN" || index.CrossChunkCalls != "NOT_COMPUTED" || index.AggregateCapture != "NOT_MATERIALIZED" {
		return nil, errors.New("private census chunk completion invalid")
	}
	return &privateCensusChunkCompletion{terminalSelector: in.terminalSelector, terminalIndex: bytes.Clone(in.terminalIndex)}, nil
}

func privateCensusChunkRoute(activated bool, discovery censusacquisition.Discovery) (bool, error) {
	if !activated {
		return false, nil
	}
	compatibility, err := censuschunking.CompatibilityForTargetCount(len(discovery.Targets))
	if err != nil {
		return false, err
	}
	return !compatibility, nil
}

func (r *censusRuntime) runPrivateCensusChunking(ctx context.Context, prepared censusRuntimeResult, root *publication.Root) (*privateCensusChunkCompletion, error) {
	input, projection, err := censuschunkingruntime.Translate(prepared.discovery, prepared.admitted.requestReceipt)
	if err != nil {
		return nil, err
	}
	projectionBytes, err := censusenumerationprojection.EncodeCanonical(projection)
	if err != nil {
		return nil, err
	}
	built, err := censuschunking.Build(input, projectionBytes)
	if err != nil || built.Compatibility {
		return nil, errors.Join(err, errors.New("private census chunk plan unavailable"))
	}
	if r == nil || r.runtime == nil || r.runtime.Manager == nil || root == nil {
		return nil, errors.New("private census chunk runtime unavailable")
	}
	if prepared.discovery.Session.SessionID != prepared.admitted.sessionID || prepared.discovery.Session.Generation != prepared.admitted.generation || prepared.discovery.Workspace != prepared.admitted.workspace {
		return nil, errors.New("private census chunk identity changed before lease")
	}
	lease := sessionruntime.SessionLeaseRequest{SessionID: prepared.admitted.sessionID, Generation: prepared.admitted.generation, Workspace: prepared.admitted.workspace}
	var result censuschunking.CoordinatorResult
	withLease := r.chunkLease
	if withLease == nil {
		withLease = r.runtime.Manager.WithReadySessionLease
	}
	coordinate := r.chunkCoordinate
	if coordinate == nil {
		coordinate = censuschunking.Coordinate
	}
	readTerminal := r.chunkReadTerminal
	if readTerminal == nil {
		readTerminal = func(root *publication.Root, selector string, maxBytes int64) ([]byte, error) {
			return root.ReadSelector(selector, maxBytes)
		}
	}
	err = withLease(ctx, lease, func(leaseCtx context.Context) error {
		receipt, publishErr := censusenumerationprojection.Publish(root, projection)
		if publishErr != nil {
			return publishErr
		}
		input.GlobalProjection = censuschunking.ArtifactRef{Selector: receipt.Selector, SHA256: receipt.Digest, ByteLength: receipt.ByteLength}
		built, buildErr := censuschunking.Build(input, projectionBytes)
		if buildErr != nil || built.Compatibility {
			return errors.Join(buildErr, errors.New("private census chunk plan changed under lease"))
		}
		store, storeErr := censuschunkingruntime.NewCoordinatorStore(root)
		if storeErr != nil {
			return storeErr
		}
		heads, headsErr := censuschunkingruntime.NewHeadStore(root)
		if headsErr != nil {
			return headsErr
		}
		limits := censusRuntimeAcquisitionLimits(prepared.options)
		core := censusacquisition.Core{Discoverer: fixedCensusRuntimeDiscovery{discovery: prepared.discovery}, Acquirer: censusRuntimeBatchAcquirer{runtime: r.runtime, admitted: prepared.admitted, limits: limits, execute: r.batchExecute}, Planning: &censusacquisition.PlanningConfig{DownDepth: int(prepared.options.downDepth), UpDepth: int(prepared.options.upDepth), MaxBatchTargets: int(prepared.options.maxBatchTargets)}}
		held, heldErr := censuschunkingruntime.NewHeldSession(censuschunking.ExpectedSession{SessionID: prepared.admitted.sessionID, Generation: prepared.admitted.generation, WorkspaceURI: input.WorkspaceURI}, core)
		if heldErr != nil {
			return heldErr
		}
		projections := make([]censuschunking.Projection, len(built.Chunks))
		sidecars := make([]censuschunking.Sidecar, len(built.Chunks))
		for i := range built.Chunks {
			projections[i], sidecars[i] = built.Chunks[i].Projection, built.Chunks[i].Sidecar
		}
		result, err = coordinate(leaseCtx, censuschunking.CoordinatorRequest{Input: input, Plan: built.Plan, Index: built.Index, Projections: projections, Sidecars: sidecars, Store: store, Heads: heads, Sessions: held})
		return err
	})
	if err != nil {
		return nil, err
	}
	raw, err := readTerminal(root, result.TerminalSelector, censuschunking.MaxCanonicalBytes)
	if err != nil {
		return nil, err
	}
	index, err := censuschunking.ParseTerminalIndex(bytes.Clone(raw))
	if err != nil {
		return nil, err
	}
	return clonePrivateCensusChunkCompletion(&privateCensusChunkCompletion{terminalSelector: index.ImmutableSelector, terminalIndex: raw})
}
