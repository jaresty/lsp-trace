package operation

import (
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/verification"
)

// NewVerifyRetainedCallsV2Handler verifies immutable publication custody before
// admitting the exact retained-calls/v2 family. Historical verifier contracts
// remain unchanged.
func NewVerifyRetainedCallsV2Handler(loader CustodyLoader) Handler {
	return func(ctx context.Context, request Request) (Result, *Failure) {
		var in struct {
			Input  json.RawMessage `json:"input"`
			Schema schemaRef       `json:"schema"`
		}
		if err := json.Unmarshal(request.Input, &in); err != nil {
			return verifyFailure(FailureInvalidInput, err)
		}
		if in.Schema.Family != retainedcalls.Family || (in.Schema.Version != "v2" && in.Schema.Version != "lsp-trace.retained-calls.v2") {
			return verifyFailure("INPUT_FAMILY_MISMATCH", fmt.Errorf("explicit retained-calls/v2 required"))
		}
		if loader == nil {
			return verifyFailure(FailureInternal, fmt.Errorf("custody loader required"))
		}
		material, failed := loader.Load(ctx, in.Input)
		if failed != nil {
			return Result{}, failed
		}
		if err := verification.VerifyReceipt(material.Artifact, material.Receipt); err != nil {
			return verifyFailure("INPUT_INVALID", err)
		}
		if _, err := retainedcalls.ValidateFor(material.Artifact, retainedcalls.Family, "v2"); err != nil {
			return verifyFailure("INPUT_INVALID", err)
		}
		return Result{Artifact: material.Artifact}, nil
	}
}
