package lifecycleops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"lsp-trace/internal/operation"
)

const (
	OperationList    operation.Name = "session_list"
	OperationStatus  operation.Name = "session_status"
	OperationStop    operation.Name = "session_stop"
	OperationRestart operation.Name = "session_restart"
)

// Executor is the disabled transport-neutral lifecycle executor family. It is
// not registered or advertised by the production MCP server.
type Executor struct{ service *Service }

// NewExecutor constructs the lifecycle executor family without registering or
// advertising it on any transport.
func NewExecutor(service *Service) *Executor { return &Executor{service: service} }

type selectorRequest struct {
	SessionID  string `json:"session_id"`
	Generation uint64 `json:"generation"`
	CallerID   string `json:"caller_id,omitempty"`
}

// Execute validates a closed direct-dispatch request before selecting one of
// exactly four lifecycleops handlers.
func (e *Executor) Execute(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.service == nil {
		return operation.Result{}, lifecycleFailure(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	switch request.Name {
	case OperationList:
		var input struct{}
		if err := decodeClosed(request.Input, &input); err != nil {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, err)
		}
		return operation.Result{Value: e.service.List()}, nil
	case OperationStatus, OperationStop, OperationRestart:
		var input selectorRequest
		if err := decodeClosed(request.Input, &input); err != nil {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, err)
		}
		if input.SessionID == "" {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, fmt.Errorf("session_id is required"))
		}
		if request.Name == OperationStatus {
			if input.CallerID != "" {
				return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, fmt.Errorf("caller_id is forbidden for status"))
			}
			record, failure := e.service.Status(input.SessionID, input.Generation)
			if failure != FailureNone {
				return operation.Result{}, e.actionableFailure(ctx, input, failure)
			}
			return operation.Result{Value: record}, nil
		}
		if input.CallerID == "" {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, fmt.Errorf("caller_id is required"))
		}
		lifecycleRequest := LifecycleRequest{SessionID: input.SessionID, Generation: input.Generation, CallerID: input.CallerID}
		var acceptance Acceptance
		if request.Name == OperationStop {
			acceptance = e.service.Stop(ctx, lifecycleRequest)
		} else {
			acceptance = e.service.Restart(ctx, lifecycleRequest)
		}
		if acceptance.Failure != FailureNone {
			return operation.Result{}, e.actionableFailure(ctx, input, acceptance.Failure)
		}
		return operation.Result{Value: acceptance}, nil
	default:
		return operation.Result{}, lifecycleFailure(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
}

func decodeClosed(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("one JSON value required")
	}
	return nil
}

func (e *Executor) actionableFailure(ctx context.Context, input selectorRequest, failure Failure) *operation.Failure {
	diagnostics := []string(nil)
	switch failure {
	case FailureStaleGeneration:
		for _, record := range e.service.List().Sessions {
			if record.SessionID == input.SessionID {
				diagnostics = []string{
					fmt.Sprintf("current generation is %d", record.Generation),
					fmt.Sprintf("retry lsp_session_v1_status with session_id %q and generation %d", input.SessionID, record.Generation),
				}
				break
			}
		}
	case FailureReapIncomplete:
		diagnostics = []string{"host operator must inspect and reap the trusted local child before retrying"}
	case FailureCapacityExhausted:
		if ctx.Err() != nil {
			diagnostics = []string{
				"caller observation was cancelled; an accepted lifecycle intent may continue",
				"query lsp_session_v1_status for the current generation before retrying",
			}
		}
	}
	return &operation.Failure{Code: string(failure), Diagnostics: diagnostics}
}

func lifecycleFailure(code string, err error) *operation.Failure {
	return &operation.Failure{Code: code, Err: err}
}
