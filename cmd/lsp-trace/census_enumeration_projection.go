package main

import (
	"net/url"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/publication"
)

func publishCensusEnumerationProjection(discovery censusacquisition.Discovery, request censusrequest.Receipt, rootPath string) censusenumerationprojection.Receipt {
	files := make([]censusenumerationprojection.File, len(discovery.FileLedger.Entries))
	for i, entry := range discovery.FileLedger.Entries {
		files[i] = censusenumerationprojection.File{Ordinal: entry.Ordinal, Path: entry.Identity, Disposition: entry.Disposition}
	}
	symbols := make([]censusenumerationprojection.Symbol, len(discovery.EnumerationSymbols))
	for i, symbol := range discovery.EnumerationSymbols {
		filePath := files[symbol.FileOrdinal].Path
		start := censusenumerationprojection.Position{Line: symbol.SelectionStart.Line, Character: symbol.SelectionStart.Character}
		digest, _, err := censusenumerationprojection.SymbolIdentityDigest(filePath, start, symbol.Kind, symbol.Name, symbol.Ordinal)
		if err != nil {
			return censusenumerationprojection.Unavailable("ENCODE_FAILED")
		}
		symbols[i] = censusenumerationprojection.Symbol{Ordinal: symbol.Ordinal, FileOrdinal: symbol.FileOrdinal, SelectionStart: start, Kind: symbol.Kind, Disposition: string(symbol.Disposition), IdentityDigest: digest}
	}
	observations := make([]censusenumerationprojection.Observation, len(discovery.Observations))
	for i, observation := range discovery.Observations {
		state := "UNKNOWN"
		if observation.Known {
			state = "KNOWN"
		}
		observations[i] = censusenumerationprojection.Observation{Dimension: string(observation.Dimension), State: state, Observed: observation.Observed, Limit: observation.Limit, Omitted: observation.Omitted}
	}
	failures := make([]censusenumerationprojection.ProviderFailure, len(discovery.ProviderFailures))
	for i, failure := range discovery.ProviderFailures {
		failures[i] = censusenumerationprojection.ProviderFailure{Stage: failure.Stage, Code: failure.Code, FileOrdinal: failure.FileOrdinal, SymbolOrdinal: failure.SymbolOrdinal}
	}
	workspaceURI := (&url.URL{Scheme: "file", Path: discovery.Workspace}).String()
	projection, err := censusenumerationprojection.Build(censusenumerationprojection.Input{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, RequestFingerprint: request.Fingerprint, WorkspaceURI: workspaceURI, Files: files, Symbols: symbols, Observations: observations, ProviderFailures: failures, RequestDownDepth: request.Semantic.DownDepth, RequestUpDepth: request.Semantic.UpDepth, RequestMaxNodes: request.Semantic.MaxNodes, RequestBatchTargets: request.Semantic.BatchTargets, RequestTimeoutMS: request.Semantic.RequestTimeoutMS, TimeoutMS: request.Semantic.TimeoutMS})
	if err != nil {
		return censusenumerationprojection.Unavailable("ENCODE_FAILED")
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		return censusenumerationprojection.Unavailable("PUBLICATION_FAILED")
	}
	defer root.Close()
	receipt, err := censusenumerationprojection.Publish(root, projection)
	if err != nil {
		return censusenumerationprojection.Unavailable("PUBLICATION_FAILED")
	}
	return receipt
}
