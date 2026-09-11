package operation

import (
	"context"
	"encoding/json"

	"lsp-trace/internal/programcpresentation"
)

type programCLeidenInput struct {
	Input        string `json:"input"`
	Seed         uint64 `json:"seed"`
	PageRankTopK int    `json:"pagerank_top_k"`
	HubTopK      int    `json:"hub_top_k"`
}

// ProgramCLeidenHandler is the transport-neutral adapter to the same certified
// handler and presentation schema used by the CLI.
func ProgramCLeidenHandler(_ context.Context, request Request) (Result, *Failure) {
	var input programCLeidenInput
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, &Failure{Code: FailureInvalidInput, Err: err}
	}
	artifact, err := programcpresentation.Handle(programcpresentation.Request{
		Input: []byte(input.Input), Seed: input.Seed,
		PageRankTopK: input.PageRankTopK, HubTopK: input.HubTopK,
	})
	if err != nil {
		return Result{}, &Failure{Code: FailureInvalidInput, Err: err}
	}
	encoded, err := programcpresentation.JSON(artifact)
	if err != nil {
		return Result{}, &Failure{Code: FailureInternal, Err: err}
	}
	return Result{Artifact: encoded, LogicalDigest: artifact.PartitionSHA256}, nil
}
