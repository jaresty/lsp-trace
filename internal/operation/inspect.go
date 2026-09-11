package operation

import (
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/ancillaryinspection"
	"lsp-trace/internal/inspection"
	"lsp-trace/internal/schema"
)

type inspectInput struct {
	Input    json.RawMessage `json:"input"`
	Selector struct {
		Seed     *string `json:"seed,omitempty"`
		AllSeeds bool    `json:"all_seeds,omitempty"`
	} `json:"selector"`
	Ancillary bool                       `json:"ancillary,omitempty"`
	Page      bool                       `json:"page,omitempty"`
	Cursor    string                     `json:"cursor,omitempty"`
	Policy    ancillaryinspection.Policy `json:"policy,omitempty"`
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
		if input.Ancillary {
			r := ancillaryinspection.Request{Input: input.Input, Ancillary: true, Page: input.Page, Cursor: input.Cursor, Policy: input.Policy}
			r.Selector.AllSeeds = input.Selector.AllSeeds
			view, err := ancillaryinspection.Inspect(r)
			if err != nil {
				return inspectFailure(err)
			}
			artifact, err := json.Marshal(view)
			if err != nil {
				return inspectFailure(err)
			}
			return Result{Value: view, Artifact: append(artifact, '\n')}, nil
		}
		if input.Page || input.Cursor != "" || input.Policy != (ancillaryinspection.Policy{}) {
			return inspectFailure(fmt.Errorf("ancillary pagination options require ancillary"))
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
