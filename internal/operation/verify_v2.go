package operation

import (
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/verification"
)

const VerifyV2 Name = "verify_v2"

// NewVerifyV2Handler admits only the explicitly selected graph-provenance v2
// family. The v1 verifier and its historical input/error contract are unchanged.
func NewVerifyV2Handler(loader CustodyLoader) Handler {
	return func(ctx context.Context, request Request) (Result, *Failure) {
		var in struct {
			Input  json.RawMessage `json:"input"`
			Schema schemaRef       `json:"schema"`
		}
		if err := json.Unmarshal(request.Input, &in); err != nil {
			return verifyFailure(FailureInvalidInput, err)
		}
		if in.Schema.Family != graphprovenance.Family || (in.Schema.Version != "v2" && in.Schema.Version != "lsp-trace.graph-provenance.v2") {
			return verifyFailure("INPUT_FAMILY_MISMATCH", fmt.Errorf("explicit graph-provenance/v2 required"))
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
		if _, err := graphprovenance.ValidateFor(material.Artifact, in.Schema.Family, in.Schema.Version); err != nil {
			return verifyFailure("INPUT_INVALID", err)
		}
		return Result{Artifact: material.Artifact}, nil
	}
}
