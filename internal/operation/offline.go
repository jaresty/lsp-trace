package operation

import (
	"context"
)

// Offline contains immutable Stage 1 offline handlers.
type Offline struct {
	validator StructuralValidator
	handlers  map[Name]Handler
}

// NewOffline constructs an offline executor from schema-owned validation and
// operation handlers.
func NewOffline(validator StructuralValidator, handlers map[Name]Handler) *Offline {
	return &Offline{validator: validator, handlers: handlers}
}

func (o *Offline) executeAs(ctx context.Context, name Name, request Request) (Result, *Failure) {
	request.Name = name
	return o.Execute(ctx, request)
}

func (o *Offline) ExecuteCapabilities(ctx context.Context, request Request) (Result, *Failure) {
	return o.executeAs(ctx, Capabilities, request)
}
func (o *Offline) ExecuteSchemaGet(ctx context.Context, request Request) (Result, *Failure) {
	return o.executeAs(ctx, SchemaGet, request)
}
func (o *Offline) ExecuteValidate(ctx context.Context, request Request) (Result, *Failure) {
	return o.executeAs(ctx, Validate, request)
}
func (o *Offline) ExecuteVerify(ctx context.Context, request Request) (Result, *Failure) {
	return o.executeAs(ctx, Verify, request)
}
func (o *Offline) ExecuteInspect(ctx context.Context, request Request) (Result, *Failure) {
	return o.executeAs(ctx, Inspect, request)
}
func (o *Offline) ExecuteFilter(ctx context.Context, request Request) (Result, *Failure) {
	return o.executeAs(ctx, Filter, request)
}

// Execute validates closed input before selecting or invoking semantic work.
func (o *Offline) Execute(ctx context.Context, request Request) (Result, *Failure) {
	if o == nil || o.validator == nil {
		return Result{}, &Failure{Code: FailureNotImplemented, Err: ErrNotImplemented}
	}
	if err := o.validator.ValidateOperationInput(request.Name, request.Input); err != nil {
		return Result{}, &Failure{Code: FailureInvalidInput, Err: err}
	}
	handler, ok := o.handlers[request.Name]
	if !ok || handler == nil {
		return Result{}, &Failure{Code: FailureNotImplemented, Err: ErrNotImplemented}
	}
	result, failure := handler(ctx, request)
	if failure != nil {
		return Result{}, NormalizeFailure(failure)
	}
	return result, nil
}
