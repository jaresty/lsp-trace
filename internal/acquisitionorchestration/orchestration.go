// Package acquisitionorchestration is the only production authority-minting
// layer. Each entry point names one trusted route and immediately spends the
// resulting capability in acquisitionengine.
package acquisitionorchestration

import (
	"context"
	"fmt"

	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/sessionruntime"
)

const (
	RouteExplicitTrace  = "explicit-trace"
	RouteSeedFile       = "canonical-seed-file"
	RouteLegacyManifest = "legacy-seed-manifest"
	RouteAutomaticFile  = "automatic-from-file"
)

type Runtime = acquisitionengine.Runtime

type routePolicy struct {
	callerLocal    bool
	retainSeedSpec bool
	prepareSource  bool
}

func policyForRoute(route string) (routePolicy, bool) {
	switch route {
	case RouteExplicitTrace:
		return routePolicy{callerLocal: true, prepareSource: true}, true
	case RouteSeedFile:
		return routePolicy{callerLocal: true}, true
	case RouteAutomaticFile:
		return routePolicy{callerLocal: true, retainSeedSpec: true}, true
	case RouteLegacyManifest:
		return routePolicy{}, true
	default:
		return routePolicy{}, false
	}
}

type documentRuntime interface {
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
}

func failure(code string, err error) (operation.Result, *operation.Failure) {
	return operation.Result{}, &operation.Failure{Code: code, Err: err}
}

func canonicalBinding(runtime Runtime, op operation.Request, route string) (operation.Request, acquisitionengine.Input, acquisitionauthority.Binding, error) {
	in, canonical, err := acquisitionengine.CanonicalInput(op.Input)
	if err != nil {
		return op, in, acquisitionauthority.Binding{}, err
	}
	op.Input = canonical
	workspace := ""
	for _, record := range runtime.Records() {
		if record.SessionID == in.SessionID && record.Generation == in.Generation {
			if workspace != "" {
				return op, in, acquisitionauthority.Binding{}, fmt.Errorf("ambiguous host workspace")
			}
			workspace = record.Profile.Workspace().String()
		}
	}
	if workspace == "" {
		return op, in, acquisitionauthority.Binding{}, fmt.Errorf("host workspace unavailable")
	}
	return op, in, acquisitionauthority.Binding{
		Operation: op.Name, Route: route, RequestID: op.RequestID,
		Workspace: workspace, SessionID: in.SessionID, Generation: in.Generation,
		Input: canonical, PublicationRoot: op.PublicationRoot, ArtifactStore: op.ArtifactStore,
	}, nil
}

func executeCallerSeeds(ctx context.Context, runtime Runtime, op operation.Request, route string, seedSpec []byte) (operation.Result, *operation.Failure) {
	if runtime == nil {
		return failure(operation.FailureInternal, fmt.Errorf("managed runtime required"))
	}
	policy, ok := policyForRoute(route)
	if !ok || !policy.callerLocal {
		return failure(operation.FailureInternal, fmt.Errorf("invalid caller-local acquisition route"))
	}
	op, in, binding, err := canonicalBinding(runtime, op, route)
	if err != nil {
		return failure(operation.FailureInvalidInput, err)
	}
	authority, err := acquisitionauthority.MintSeedAuthority(binding, seedSpec, seedbinding.CallerAssertedLocal, false, "", policy.retainSeedSpec)
	if err != nil {
		return failure(operation.FailureInvalidInput, err)
	}
	var capability *acquisitionauthority.PreparedSourceCapability
	if policy.prepareSource {
		preparer, ok := runtime.(documentRuntime)
		if !ok {
			return failure(operation.FailureInternal, fmt.Errorf("managed document supply unavailable"))
		}
		locator := in.SeedManifest.Root.Locator
		doc, prepared, prepareErr := acquisitionauthority.PrepareSource(ctx, preparer, sessionruntime.DocumentRequest{SessionID: in.SessionID, Generation: in.Generation, URI: locator.URI, LanguageID: locator.LanguageID, CaptureSupply: true}, binding)
		if doc.Failure != "" {
			return failure(string(doc.Failure), nil)
		}
		if prepareErr != nil {
			return failure(string(sessionruntime.DocumentSupplyUnavailable), prepareErr)
		}
		capability = &prepared
	}
	return acquisitionengine.ExecuteAuthorized(ctx, runtime, op, route, authority, capability, binding)
}

// ExecuteExplicitTrace preserves caller-local custody and source supply for MCP
// trace and the CLI trace facade without retaining replayable seed bytes.
func ExecuteExplicitTrace(ctx context.Context, runtime Runtime, op operation.Request, seedSpec []byte) (operation.Result, *operation.Failure) {
	return executeCallerSeeds(ctx, runtime, op, RouteExplicitTrace, seedSpec)
}

// ExecuteExplicitTracePrepared seals a real source preparation made before an
// exact symbol lookup, once the resulting acquisition operation is fully known.
func ExecuteExplicitTracePrepared(ctx context.Context, runtime Runtime, op operation.Request, seedSpec []byte, doc sessionruntime.DocumentResult) (operation.Result, *operation.Failure) {
	if runtime == nil {
		return failure(operation.FailureInternal, fmt.Errorf("managed runtime required"))
	}
	policy, ok := policyForRoute(RouteExplicitTrace)
	if !ok || !policy.callerLocal || !policy.prepareSource {
		return failure(operation.FailureInternal, fmt.Errorf("invalid explicit trace policy"))
	}
	op, in, binding, err := canonicalBinding(runtime, op, RouteExplicitTrace)
	if err != nil {
		return failure(operation.FailureInvalidInput, err)
	}
	authority, err := acquisitionauthority.MintSeedAuthority(binding, seedSpec, seedbinding.CallerAssertedLocal, false, "", policy.retainSeedSpec)
	if err != nil {
		return failure(operation.FailureInvalidInput, err)
	}
	locator := in.SeedManifest.Root.Locator
	req := sessionruntime.DocumentRequest{SessionID: in.SessionID, Generation: in.Generation, URI: locator.URI, LanguageID: locator.LanguageID, CaptureSupply: true}
	capability, err := acquisitionauthority.SealPreparedSource(doc, req, binding)
	if err != nil {
		return failure(string(sessionruntime.DocumentSupplyUnavailable), err)
	}
	return acquisitionengine.ExecuteAuthorized(ctx, runtime, op, RouteExplicitTrace, authority, &capability, binding)
}

// ExecuteSeedFile preserves caller-local custody without retaining replayed seed bytes.
func ExecuteSeedFile(ctx context.Context, runtime Runtime, op operation.Request, seedSpec []byte) (operation.Result, *operation.Failure) {
	return executeCallerSeeds(ctx, runtime, op, RouteSeedFile, seedSpec)
}

// ExecuteAutomaticFile preserves generated canonical Seeds V2 in V5.
func ExecuteAutomaticFile(ctx context.Context, runtime Runtime, op operation.Request, seedSpec []byte) (operation.Result, *operation.Failure) {
	return executeCallerSeeds(ctx, runtime, op, RouteAutomaticFile, seedSpec)
}

// ExecuteLegacyManifest preserves the runtime's established seed-binding custody
// but never carries or retains request seed bytes.
func ExecuteLegacyManifest(ctx context.Context, runtime Runtime, op operation.Request) (operation.Result, *operation.Failure) {
	if runtime == nil {
		return failure(operation.FailureInternal, fmt.Errorf("managed runtime required"))
	}
	policy, ok := policyForRoute(RouteLegacyManifest)
	if !ok || policy.callerLocal || policy.retainSeedSpec || policy.prepareSource {
		return failure(operation.FailureInternal, fmt.Errorf("invalid legacy manifest policy"))
	}
	op, in, binding, err := canonicalBinding(runtime, op, RouteLegacyManifest)
	if err != nil {
		return failure(operation.FailureInvalidInput, err)
	}
	provenanceRuntime, ok := runtime.(interface {
		SeedCustodyProvenance(string, uint64) (seedbinding.CustodyMode, bool)
	})
	if !ok {
		return failure("OUTPUT_VALIDATION_FAILED", fmt.Errorf("legacy manifest custody unavailable"))
	}
	provenance, found := provenanceRuntime.SeedCustodyProvenance(in.SessionID, in.Generation)
	if !found {
		return failure("OUTPUT_VALIDATION_FAILED", fmt.Errorf("legacy manifest custody unavailable"))
	}
	authenticated := provenance == seedbinding.VerifiedHost
	receipt := ""
	if authenticated {
		receiptRuntime, ok := runtime.(interface {
			SeedCustodyReceipt(string, uint64) (string, bool)
		})
		if !ok {
			return failure("OUTPUT_VALIDATION_FAILED", fmt.Errorf("verified host custody requires an exact host receipt identifier"))
		}
		var receiptFound bool
		receipt, receiptFound = receiptRuntime.SeedCustodyReceipt(in.SessionID, in.Generation)
		if !receiptFound || receipt == "" {
			return failure("OUTPUT_VALIDATION_FAILED", fmt.Errorf("verified host custody requires an exact host receipt identifier"))
		}
	}
	authority, err := acquisitionauthority.MintSeedAuthority(binding, nil, provenance, authenticated, receipt, false)
	if err != nil {
		return failure("OUTPUT_VALIDATION_FAILED", err)
	}
	return acquisitionengine.ExecuteAuthorized(ctx, runtime, op, RouteLegacyManifest, authority, nil, binding)
}
