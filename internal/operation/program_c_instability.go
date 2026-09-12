package operation

import (
	"context"
	"encoding/json"

	"lsp-trace/internal/programc"
)

type programCInstabilityInput struct {
	Input                string   `json:"input"`
	Seeds                []uint64 `json:"seeds"`
	AlgorithmVersion     string   `json:"algorithm_version"`
	ParametersSHA256     string   `json:"parameters_sha256"`
	ResourcePolicySHA256 string   `json:"resource_policy_sha256"`
}

func ProgramCInstabilityHandler(_ context.Context, request Request) (Result, *Failure) {
	var input programCInstabilityInput
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, &Failure{Code: FailureInvalidInput, Err: err}
	}
	artifact, err := programc.ComputeInstabilityV5([]byte(input.Input), input.Seeds, input.AlgorithmVersion, input.ParametersSHA256, input.ResourcePolicySHA256)
	if err != nil {
		return Result{}, &Failure{Code: FailureInvalidInput, Err: err}
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return Result{}, &Failure{Code: FailureInternal, Err: err}
	}
	return Result{Artifact: encoded, LogicalDigest: artifact.PolicySHA256}, nil
}
