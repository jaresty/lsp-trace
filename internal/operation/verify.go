package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/verification"
)

// CustodyMaterial carries exact bytes acquired by an adapter-owned custody loader.
type CustodyMaterial struct {
	Artifact []byte
	Receipt  []byte
}

// CustodyLoader acquires retained bytes without granting the operation handler
// filesystem or publication authority.
type CustodyLoader interface {
	Load(context.Context, json.RawMessage) (CustodyMaterial, *Failure)
}

type verifyInput struct {
	Input json.RawMessage `json:"input"`
}

// NewVerifyHandler adapts explicitly injected custody loading to the accepted
// exact-byte receipt and semantic verification cores. It performs no filesystem
// access and has no publication authority.
func NewVerifyHandler(loader CustodyLoader) Handler {
	return func(ctx context.Context, request Request) (Result, *Failure) {
		var input verifyInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return verifyFailure(FailureInvalidInput, err)
		}
		if len(input.Input) == 0 || bytes.Equal(input.Input, []byte("null")) {
			return verifyFailure(FailureInvalidInput, fmt.Errorf("verification input is required"))
		}
		if loader == nil {
			return verifyFailure(FailureInternal, fmt.Errorf("custody loader is required"))
		}
		material, failure := loader.Load(ctx, input.Input)
		if failure != nil {
			return Result{}, failure
		}
		if err := verification.VerifyReceipt(material.Artifact, material.Receipt); err != nil {
			return verifyFailure("VERIFICATION_FAILED", err)
		}
		if err := graph.ValidateSemanticBundle(bytes.TrimSpace(material.Artifact)); err != nil {
			return verifyFailure("VERIFICATION_FAILED", err)
		}
		return Result{Artifact: material.Artifact}, nil
	}
}

func verifyFailure(code string, err error) (Result, *Failure) {
	return Result{}, &Failure{Code: code, Diagnostics: []string{err.Error()}, Err: err}
}
