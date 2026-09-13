package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

// censusDiscoveryFailure is the closed package-main error boundary for source
// census discovery. It intentionally carries no wrapped error or path.
type censusDiscoveryFailure struct {
	Stage censusFailureStage
	Code  censusFailureCode
}

func (e censusDiscoveryFailure) Error() string { return string(e.Stage) + ":" + string(e.Code) }

// initializedCensusDiscoverySession is the least-authority initialized-session
// surface needed by the production discovery adapter.
type initializedCensusDiscoverySession interface {
	SessionID() string
	Generation() uint64
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
}

type censusWorkspaceEnumerator struct {
	workspace string
	roots     []string
}

func (e censusWorkspaceEnumerator) Enumerate(ctx context.Context) ([]censusacquisition.SourceFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	roots, err := expandCensusSourceRoots(e.workspace, e.roots)
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return []censusacquisition.SourceFile{}, nil
	}
	return censusacquisition.EnumerateWorkspace(e.workspace, roots, source.LanguageID)
}

// source.Discover deliberately rejects an ambiguous dot input. At the parsed
// census boundary dot means all immediate workspace members; expanding it here
// preserves the shared enumerator's safety contract and deterministic order.
func expandCensusSourceRoots(workspace string, roots []string) ([]string, error) {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if filepath.Clean(root) != "." {
			out = append(out, root)
			continue
		}
		entries, err := os.ReadDir(workspace)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			out = append(out, entry.Name())
		}
	}
	sort.Slice(out, func(i, j int) bool { return filepath.ToSlash(out[i]) < filepath.ToSlash(out[j]) })
	return append([]string(nil), out...), nil
}

type censusRuntimeDocumentSupplier struct {
	runtime initializedCensusDiscoverySession
}

func (s censusRuntimeDocumentSupplier) Supply(ctx context.Context, file censusacquisition.SourceFile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.runtime.PrepareDocument(ctx, sessionruntime.DocumentRequest{
		URI: file.URI, LanguageID: file.LanguageID, CaptureSupply: true,
	})
	if result.Failure != "" || result.URI != file.URI || result.LanguageID != file.LanguageID {
		return errors.New("document supply failed")
	}
	// CaptureSupply forces the runtime to return an owned immutable observation
	// for a new notification. Cached unchanged documents are also successful.
	if result.Supply != nil {
		if result.Supply.URI != file.URI || result.Supply.SessionID != s.runtime.SessionID() || result.Supply.Generation != s.runtime.Generation() {
			return errors.New("document supply identity mismatch")
		}
		_ = append([]byte(nil), result.Supply.Content...)
	}
	return nil
}

type censusProductionDiscoveryClient struct {
	productionDiscoveryClient
	callHierarchy bool
}

func (c censusProductionDiscoveryClient) SupportsCallHierarchy() bool { return c.callHierarchy }

func newCensusDiscoveryAdapter(options censusCLIOptions, session initializedCensusDiscoverySession, client censusacquisition.DiscoveryClient) (censusacquisition.Discoverer, error) {
	if session == nil || client == nil || session.SessionID() == "" || session.Generation() == 0 {
		return nil, censusDiscoveryFailure{Stage: censusStageDiscovery, Code: censusCodeDiscoveryFailed}
	}
	workspace, err := filepath.Abs(options.Workspace)
	if err != nil {
		return nil, censusDiscoveryFailure{Stage: censusStageDiscovery, Code: censusCodeDiscoveryFailed}
	}
	workspace = filepath.Clean(workspace)
	info, err := os.Stat(workspace)
	if err != nil || !info.IsDir() {
		return nil, censusDiscoveryFailure{Stage: censusStageDiscovery, Code: censusCodeDiscoveryFailed}
	}
	roots := append([]string(nil), options.Sources...)
	if len(roots) == 0 {
		roots = []string{"."}
	}
	return censusacquisition.DiscoveryAdapter{
		Workspace: workspace,
		Filters: censusacquisition.Filters{
			Includes: append([]string(nil), options.Includes...),
			Excludes: append([]string(nil), options.Excludes...),
		},
		Files:    censusWorkspaceEnumerator{workspace: workspace, roots: roots},
		Supplier: censusRuntimeDocumentSupplier{runtime: session},
		Client:   client,
		Limits: censusacquisition.DiscoveryLimits{
			MaxFiles: options.MaxNodes, MaxSymbols: options.MaxNodes,
		},
	}, nil
}

// newInitializedCensusDiscoverer consumes only parsed options and an already
// initialized caller-owned runtime. It performs no profile or lifecycle work.
func newInitializedCensusDiscoverer(options censusCLIOptions, runtime *initializedAcquisitionRuntime, callHierarchy bool) (censusacquisition.Discoverer, error) {
	if runtime == nil || runtime.privateAcquisitionRuntime == nil {
		return nil, censusDiscoveryFailure{Stage: censusStageDiscovery, Code: censusCodeDiscoveryFailed}
	}
	requestTimeout := options.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = runtime.requestTimeout
	}
	if requestTimeout <= 0 {
		requestTimeout = 30 * time.Second
	}
	client := censusProductionDiscoveryClient{
		productionDiscoveryClient: productionDiscoveryClient{
			runtime: runtime.privateAcquisitionRuntime, sessionID: runtime.SessionID(),
			generation: runtime.Generation(), timeout: requestTimeout,
		},
		callHierarchy: callHierarchy,
	}
	return newCensusDiscoveryAdapter(options, runtime, client)
}

// runInitializedCensusDiscovery is the integrated private discovery API. All
// returned failures are closed typed stage/code values without wrapped paths.
func runInitializedCensusDiscovery(ctx context.Context, options censusCLIOptions, runtime *initializedAcquisitionRuntime, callHierarchy bool) (censusacquisition.Discovery, error) {
	discoverer, err := newInitializedCensusDiscoverer(options, runtime, callHierarchy)
	if err != nil {
		return censusacquisition.Discovery{}, err
	}
	discovery, err := discoverer.Discover(ctx, censusacquisition.SessionIdentity{SessionID: runtime.SessionID(), Generation: runtime.Generation()})
	if err != nil {
		return discovery, censusDiscoveryFailure{Stage: censusStageDiscovery, Code: censusCodeDiscoveryFailed}
	}
	return discovery, nil
}
