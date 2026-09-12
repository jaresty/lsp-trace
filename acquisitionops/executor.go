// Package acquisitionops is the ordinary public managed acquisition boundary.
// It deliberately exposes no custody authority, seed bytes, or prepared-source
// capability. Trusted routes live under internal/acquisitionorchestration.
package acquisitionops

import (
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/operation"
	"lsp-trace/sessionruntime"
)

const (
	Slice           = acquisitionengine.Slice
	Incoming        = acquisitionengine.Incoming
	SliceV3         = acquisitionengine.SliceV3
	IncomingV3      = acquisitionengine.IncomingV3
	ManifestVersion = acquisitionengine.ManifestVersion
	MaxInputBytes   = acquisitionengine.MaxInputBytes
)

type Runtime = acquisitionengine.Runtime
type CustodyReceipt = acquisitionengine.CustodyReceipt
type Target = acquisitionengine.Target
type Limits = acquisitionengine.Limits
type Expansion = acquisitionengine.Expansion
type Manifest = acquisitionengine.Manifest
type GroupOptions = acquisitionengine.GroupOptions
type Input = acquisitionengine.Input

func DecodeManifest(raw []byte, mode operation.Name) (Manifest, error) {
	return acquisitionengine.DecodeManifest(raw, mode)
}

type Executor struct{ runtime Runtime }

func NewExecutor(r Runtime) *Executor { return &Executor{runtime: r} }

func (e *Executor) Execute(ctx context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: fmt.Errorf("managed runtime required")}
	}
	return acquisitionengine.ExecuteOrdinary(ctx, e.runtime, op)
}

// These unexported helpers preserve package-local regression coverage while the
// production implementations remain owned by the internal engine.
func validatePreparedDocument(sessionID string, generation uint64, locator acquisition.Locator, prepared sessionruntime.DocumentResult) error {
	if prepared.Supply == nil {
		return fmt.Errorf("prepared document supply unavailable")
	}
	supply := prepared.Supply
	if supply.SessionID != sessionID || supply.Generation != generation || supply.URI != prepared.URI || prepared.URI != locator.URI || supply.DocumentVersion != prepared.Version ||
		(prepared.LanguageID != "" && locator.LanguageID != "" && prepared.LanguageID != locator.LanguageID) {
		return fmt.Errorf("prepared document identity mismatch")
	}
	return nil
}

func cloneSiblingCandidatesForV5(source []graph.SiblingCandidate) ([]graph.SiblingCandidate, error) {
	raw, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("clone sibling candidates for v5: marshal: %w", err)
	}
	var cloned []graph.SiblingCandidate
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, fmt.Errorf("clone sibling candidates for v5: decode: %w", err)
	}
	return cloned, nil
}

func cloneGraphForV5(source graph.Result) (graph.Result, error) {
	raw, err := json.Marshal(source)
	if err != nil {
		return graph.Result{}, fmt.Errorf("clone graph for v5: marshal: %w", err)
	}
	cloned, err := graph.DecodeNativeV3(raw)
	if err != nil {
		return graph.Result{}, fmt.Errorf("clone graph for v5: decode: %w", err)
	}
	return cloned, nil
}
