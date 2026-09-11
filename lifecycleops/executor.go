package lifecycleops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
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
	Detail     string `json:"detail,omitempty"`
}

type publicSession struct {
	SessionID         string        `json:"session_id"`
	Alias             string        `json:"alias"`
	Generation        uint64        `json:"generation"`
	State             session.State `json:"state"`
	Readiness         string        `json:"readiness"`
	Started           time.Time     `json:"started"`
	LanguageID        string        `json:"language_id"`
	ServerProfile     string        `json:"server_profile"`
	RelationProviders []string      `json:"relation_providers"`
}

type publicCensus struct {
	Sessions     int `json:"sessions"`
	Generations  int `json:"generations"`
	Requests     int `json:"requests"`
	Children     int `json:"children"`
	Cancels      int `json:"cancels"`
	Tombstones   int `json:"tombstones"`
	Observations int `json:"observations"`
	Operations   int `json:"operations"`
	Workers      int `json:"workers"`
}

type publicListSnapshot struct {
	Sessions   []publicSession    `json:"sessions"`
	Census     publicCensus       `json:"census"`
	Resolution *SessionResolution `json:"resolution,omitempty"`
}

func publicSessionFromRecord(record sessionruntime.Record) publicSession {
	return publicSession{
		SessionID: record.SessionID, Alias: record.Routing.Alias, Generation: record.Generation,
		State: record.State, Readiness: record.Routing.Readiness, Started: record.Started,
		LanguageID: record.Routing.LanguageID, ServerProfile: record.Routing.ServerProfile,
		RelationProviders: append([]string(nil), record.Routing.RelationProviders...),
	}
}

func publicListFromSnapshot(snapshot ListSnapshot) publicListSnapshot {
	sessions := make([]publicSession, len(snapshot.Sessions))
	for i, record := range snapshot.Sessions {
		sessions[i] = publicSessionFromRecord(record)
	}
	census := snapshot.Census
	return publicListSnapshot{
		Sessions: sessions,
		Census: publicCensus{
			Sessions: census.Sessions, Generations: census.Generations, Requests: census.Requests,
			Children: census.Children, Cancels: census.Cancels, Tombstones: census.Tombstones,
			Observations: census.Observations, Operations: census.Operations, Workers: census.Workers,
		},
		Resolution: snapshot.Resolution,
	}
}

// Execute validates a closed direct-dispatch request before selecting one of
// exactly four lifecycleops handlers.
func (e *Executor) Execute(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.service == nil {
		return operation.Result{}, lifecycleFailure(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	switch request.Name {
	case OperationList:
		var input struct {
			URI    string `json:"uri,omitempty"`
			Detail string `json:"detail,omitempty"`
		}
		if err := decodeClosed(request.Input, &input); err != nil {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, err)
		}
		if input.Detail != "" && input.Detail != "compact" && input.Detail != "full" {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, fmt.Errorf("detail must be compact or full"))
		}
		var result ListSnapshot
		if input.URI == "" {
			result = e.service.List()
		} else {
			var failure Failure
			result, failure = e.service.ListForURI(input.URI)
			if failure != FailureNone {
				return operation.Result{}, lifecycleFailure(string(failure), errors.New(string(failure)))
			}
		}
		if input.Detail == "full" {
			return operation.Result{Value: result}, nil
		}
		return operation.Result{Value: publicListFromSnapshot(result)}, nil
	case OperationStatus, OperationStop, OperationRestart:
		var input selectorRequest
		if err := decodeClosed(request.Input, &input); err != nil {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, err)
		}
		if input.SessionID == "" {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, fmt.Errorf("session_id is required"))
		}
		if input.Detail != "" && input.Detail != "compact" && input.Detail != "full" {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, fmt.Errorf("detail must be compact or full"))
		}
		if request.Name != OperationStatus && input.Detail != "" {
			return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, fmt.Errorf("detail is forbidden for lifecycle mutations"))
		}
		if request.Name == OperationStatus {
			if input.CallerID != "" {
				return operation.Result{}, lifecycleFailure(operation.FailureInvalidInput, fmt.Errorf("caller_id is forbidden for status"))
			}
			record, failure := e.service.Status(input.SessionID, input.Generation)
			if failure != FailureNone {
				return operation.Result{}, e.actionableFailure(ctx, input, failure)
			}
			if input.Detail == "full" {
				return operation.Result{Value: record}, nil
			}
			return operation.Result{Value: publicSessionFromRecord(record)}, nil
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
	case FailureSessionNotFound:
		aliasSet := map[string]struct{}{}
		for _, record := range e.service.List().Sessions {
			if record.Routing.Alias != "" {
				aliasSet[record.Routing.Alias] = struct{}{}
			}
		}
		aliases := make([]string, 0, len(aliasSet))
		for alias := range aliasSet {
			aliases = append(aliases, fmt.Sprintf("%q", alias))
		}
		sort.Strings(aliases)
		if len(aliases) > 0 {
			diagnostics = append(diagnostics, "available session aliases: "+strings.Join(aliases, ", "))
		}
		diagnostics = append(diagnostics, "sessions are host-provisioned; configure bootstrap and restart lsp-trace-mcp to add one")
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
