package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"lsp-trace/internal/operation"
)

const (
	executionSchemaVersion = "lsp-trace.execution.v1"
	executionOperation     = operation.Name("execute")

	executionExitSuccess          = 0
	executionExitInvalidInput     = 1
	executionExitOperationFailure = 2
	executionExitCancelled        = 130
)

type executionSuccess struct {
	SchemaVersion string         `json:"schema_version"`
	State         string         `json:"state"`
	Operation     operation.Name `json:"operation"`
	RequestID     string         `json:"request_id"`
	Result        any            `json:"result,omitempty"`
	Artifact      []byte         `json:"artifact,omitempty"`
	LogicalDigest string         `json:"logical_digest,omitempty"`
}

type executionError struct {
	SchemaVersion string         `json:"schema_version"`
	State         string         `json:"state"`
	Operation     operation.Name `json:"operation"`
	RequestID     string         `json:"request_id,omitempty"`
	Code          string         `json:"code"`
	Diagnostics   []string       `json:"diagnostics"`
}

type unavailableExecutionExecutor struct{}

func (unavailableExecutionExecutor) Execute(context.Context, operation.Request) (operation.Result, *operation.Failure) {
	return operation.Result{}, &operation.Failure{Code: operation.FailureNotImplemented, Err: operation.ErrNotImplemented}
}

// registeredExecutionExecutor is replaced by the concrete-operation
// registration seam. Keeping the fallback transport-shaped avoids assigning
// any execution semantics to the CLI.
var registeredExecutionExecutor operation.Executor = unavailableExecutionExecutor{}

func runExecution(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, cancel := signalContext()
	defer cancel()
	return runExecutionWithExecutor(ctx, args, stdin, stdout, stderr, registeredExecutionExecutor)
}

var signalContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runExecutionWithExecutor(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, executor operation.Executor) int {
	requestID, inputPath, parseFailure := parseExecutionArgs(args)
	if parseFailure != nil {
		return writeExecutionJSON(stdout, stderr, executionError{
			SchemaVersion: executionSchemaVersion,
			State:         "error",
			Operation:     executionOperation,
			RequestID:     requestID,
			Code:          operation.FailureInvalidInput,
			Diagnostics:   []string{parseFailure.Error()},
		}, executionExitInvalidInput)
	}

	input, err := readExecutionInput(inputPath, stdin)
	if err != nil {
		return writeExecutionJSON(stdout, stderr, executionError{
			SchemaVersion: executionSchemaVersion,
			State:         "error",
			Operation:     executionOperation,
			RequestID:     requestID,
			Code:          operation.FailureInvalidInput,
			Diagnostics:   []string{err.Error()},
		}, executionExitInvalidInput)
	}

	result, failure := executor.Execute(ctx, operation.Request{
		Name:      executionOperation,
		RequestID: requestID,
		Input:     input,
	})
	if failure != nil {
		normalized := operation.NormalizeFailure(failure)
		code := executionExitOperationFailure
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(normalized.Err, context.Canceled) {
			code = executionExitCancelled
		}
		return writeExecutionJSON(stdout, stderr, executionError{
			SchemaVersion: executionSchemaVersion,
			State:         "error",
			Operation:     executionOperation,
			RequestID:     requestID,
			Code:          normalized.Code,
			Diagnostics:   append([]string(nil), normalized.Diagnostics...),
		}, code)
	}

	return writeExecutionJSON(stdout, stderr, executionSuccess{
		SchemaVersion: executionSchemaVersion,
		State:         "ok",
		Operation:     executionOperation,
		RequestID:     requestID,
		Result:        result.Value,
		Artifact:      append([]byte(nil), result.Artifact...),
		LogicalDigest: result.LogicalDigest,
	}, executionExitSuccess)
}

func parseExecutionArgs(args []string) (requestID, inputPath string, err error) {
	fs := flag.NewFlagSet("execute", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&requestID, "request-id", "", "canonical request identity")
	fs.StringVar(&inputPath, "input", "", "request JSON path or - for stdin")
	if parseErr := fs.Parse(args); parseErr != nil {
		return requestID, inputPath, fmt.Errorf("invalid execution arguments: %w", parseErr)
	}
	if fs.NArg() != 0 {
		return requestID, inputPath, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if requestID == "" {
		return requestID, inputPath, errors.New("--request-id is required")
	}
	if inputPath == "" {
		return requestID, inputPath, errors.New("--input is required")
	}
	return requestID, inputPath, nil
}

func readExecutionInput(path string, stdin io.Reader) (json.RawMessage, error) {
	var (
		data []byte
		err  error
	)
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read execution input: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 || !json.Valid(data) {
		return nil, errors.New("execution input must be one valid JSON value")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&value); err != nil {
		return nil, errors.New("execution input must be one valid JSON value")
	}
	if decoder.Decode(&value) != io.EOF {
		return nil, errors.New("execution input must be one valid JSON value")
	}
	return append(json.RawMessage(nil), data...), nil
}

func writeExecutionJSON(stdout, stderr io.Writer, value any, code int) int {
	encoded, err := json.Marshal(value)
	if err != nil {
		fmt.Fprintln(stderr, "execute: encode response")
		return executionExitInvalidInput
	}
	encoded = append(encoded, '\n')
	written, err := stdout.Write(encoded)
	if err != nil || written != len(encoded) {
		fmt.Fprintln(stderr, "execute: write response")
		return executionExitInvalidInput
	}
	return code
}
