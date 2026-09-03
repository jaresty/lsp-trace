package operation

import (
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/inspection"
	"lsp-trace/internal/schema"
)

type inspectInput struct {
	Input    json.RawMessage `json:"input"`
	Selector struct {
		Seed     *string `json:"seed,omitempty"`
		AllSeeds bool    `json:"all_seeds,omitempty"`
	} `json:"selector"`
}

// NewInspectHandler adapts structurally admitted graph bytes to the accepted
// inspection core without taking authority over projection semantics.
func NewInspectHandler() Handler {
	return func(_ context.Context, request Request) (Result, *Failure) {
		var input inspectInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return inspectFailure(err)
		}
		graphBytes, err := admittedInspectionInputBytes(input.Input)
		if err != nil {
			return inspectFailure(err)
		}

		var projection any
		switch {
		case input.Selector.Seed != nil && !input.Selector.AllSeeds:
			projection, err = inspection.ProjectSeed(graphBytes, *input.Selector.Seed)
		case input.Selector.Seed == nil && input.Selector.AllSeeds:
			projection, err = inspection.ProjectAllSeeds(graphBytes)
		default:
			err = fmt.Errorf("inspection selector must choose exactly one of seed or all_seeds")
		}
		if err != nil {
			return inspectFailure(err)
		}
		artifact, err := json.Marshal(projection)
		if err != nil {
			return inspectFailure(err)
		}
		if err := schema.ValidateInspection(artifact); err != nil {
			return inspectFailure(err)
		}
		artifact = append(artifact, '\n')
		return Result{Value: projection, Artifact: artifact}, nil
	}
}

func admittedInspectionInputBytes(input json.RawMessage) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("inspection input is required")
	}
	if input[0] != '"' {
		return input, nil
	}
	var encoded string
	if err := json.Unmarshal(input, &encoded); err != nil {
		return nil, err
	}
	return []byte(encoded), nil
}

func inspectFailure(err error) (Result, *Failure) {
	return Result{}, &Failure{Code: "INSPECTION_FAILED", Err: err}
}
