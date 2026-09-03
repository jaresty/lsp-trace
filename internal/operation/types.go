// Package operation defines transport-independent invocation contracts for
// lsp-trace operations. It deliberately owns neither MCP framing nor schemas.
package operation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Name is a canonical operation identity independent of any transport alias.
type Name string

const (
	Capabilities Name = "capabilities"
	SchemaGet    Name = "schema_get"
	Validate     Name = "validate"
	Verify       Name = "verify"
	Inspect      Name = "inspect"
	Filter       Name = "filter"
)

var ErrNotImplemented = errors.New("operation not implemented")

const (
	FailureInvalidInput   = "INVALID_INPUT"
	FailureNotImplemented = "NOT_IMPLEMENTED"
	FailureInternal       = "INTERNAL"
)

// Request carries structurally validated operation input. Input remains raw so
// the schema-contract owner, rather than this package, controls its definition.
type Request struct {
	Name      Name
	RequestID string
	Input     json.RawMessage
}

// Result is the transport-independent operation result. Artifact contains the
// exact authoritative bytes produced by existing CLI semantics.
type Result struct {
	Value    any
	Artifact []byte
}

// Failure classifies an operation failure without imposing an MCP envelope.
type Failure struct {
	Code        string
	Diagnostics []string
	Err         error
}

func (f *Failure) Error() string {
	if f == nil {
		return ""
	}
	if f.Err != nil {
		return fmt.Sprintf("%s: %v", f.Code, f.Err)
	}
	return f.Code
}

func (f *Failure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.Err
}

// NormalizeFailure returns a stable, independently owned failure. Existing
// codes, diagnostics, and causal errors are preserved exactly.
func NormalizeFailure(failure *Failure) *Failure {
	if failure == nil {
		return nil
	}
	code := failure.Code
	if code == "" {
		code = FailureInternal
	}
	return &Failure{
		Code:        code,
		Diagnostics: append([]string(nil), failure.Diagnostics...),
		Err:         failure.Err,
	}
}

// StructuralValidator is supplied by the schema-contract layer. It must return
// before a semantic handler is selected.
type StructuralValidator interface {
	ValidateOperationInput(Name, json.RawMessage) error
}

// Handler performs one already structurally validated offline operation.
type Handler func(context.Context, Request) (Result, *Failure)

var requiredNames = [...]Name{Capabilities, SchemaGet, Validate, Verify, Inspect, Filter}

// NewRequiredHandlers validates exactly the canonical offline handler set.
func NewRequiredHandlers(handlers map[Name]Handler) (map[Name]Handler, error) {
	if len(handlers) != len(requiredNames) {
		return nil, fmt.Errorf("required handler count: got %d want %d", len(handlers), len(requiredNames))
	}
	for _, name := range requiredNames {
		if handler, ok := handlers[name]; !ok || handler == nil {
			return nil, fmt.Errorf("required handler %q is missing", name)
		}
	}
	return handlers, nil
}

// Executor exposes the shared typed operation API.
type Executor interface {
	Execute(context.Context, Request) (Result, *Failure)
}
