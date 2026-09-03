package operation

import (
	"context"
	"encoding/json"
	"errors"

	semanticfilter "lsp-trace/internal/filter"
)

var (
	errFilterPairRequired = errors.New("filter requires exactly two compare_seeds values")
	errFilterSeedEmpty    = errors.New("filter compare_seeds values must be nonempty")
	errFilterSeedDistinct = errors.New("filter compare_seeds values must be distinct")
)

type filterOperationInput struct {
	Input  json.RawMessage `json:"input"`
	Filter struct {
		CompareSeeds []string `json:"compare_seeds"`
	} `json:"filter"`
}

// NewFilterHandler adapts already structurally admitted filter input to the
// accepted pairwise filter core. It owns no validation, transport, or process
// state and preserves the core projection without semantic rewriting.
func NewFilterHandler() Handler {
	return func(_ context.Context, request Request) (Result, *Failure) {
		var input filterOperationInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return filterFailure(err)
		}
		labels := input.Filter.CompareSeeds
		if len(labels) != 2 {
			return filterFailure(errFilterPairRequired)
		}
		if labels[0] == "" || labels[1] == "" {
			return filterFailure(errFilterSeedEmpty)
		}
		if labels[0] == labels[1] {
			return filterFailure(errFilterSeedDistinct)
		}

		inspectionBytes, err := admittedInputBytes(input.Input)
		if err != nil {
			return filterFailure(err)
		}
		projection, err := semanticfilter.ProjectPairwise(inspectionBytes, labels[0], labels[1])
		if err != nil {
			return filterFailure(err)
		}
		artifact, err := json.Marshal(projection)
		if err != nil {
			return Result{}, &Failure{Code: "INTERNAL", Err: err}
		}
		artifact = append(artifact, '\n')
		return Result{Value: projection, Artifact: artifact}, nil
	}
}

func admittedInputBytes(raw json.RawMessage) ([]byte, error) {
	var encoded string
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return nil, err
		}
		return []byte(encoded), nil
	}
	return append([]byte(nil), raw...), nil
}

func filterFailure(err error) (Result, *Failure) {
	return Result{}, &Failure{Code: "INVALID_INPUT", Err: err}
}
