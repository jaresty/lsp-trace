package operation

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

type recordingValidator struct {
	err    error
	events *[]string
}

func (v recordingValidator) ValidateOperationInput(name Name, _ json.RawMessage) error {
	*v.events = append(*v.events, "structural:"+string(name))
	return v.err
}

func completeHandlers(handler Handler) map[Name]Handler {
	return map[Name]Handler{
		Capabilities: handler,
		SchemaGet:    handler,
		Validate:     handler,
		Verify:       handler,
		Inspect:      handler,
		Filter:       handler,
	}
}

func TestNewRequiredHandlersRejectsMissingNilAndUnknownRegistrations(t *testing.T) {
	assertion := "P1_REQUIRED_HANDLER_CONSTRUCTION"
	handler := func(context.Context, Request) (Result, *Failure) { return Result{}, nil }

	for name, handlers := range map[string]map[Name]Handler{
		"missing": completeHandlers(handler),
		"nil":     completeHandlers(handler),
		"unknown": completeHandlers(handler),
	} {
		switch name {
		case "missing":
			delete(handlers, Filter)
		case "nil":
			handlers[Filter] = nil
		case "unknown":
			handlers[Name("future")] = handler
		}
		if _, err := NewRequiredHandlers(handlers); err == nil {
			t.Errorf("%s: %s registration accepted", assertion, name)
		}
	}

	got, err := NewRequiredHandlers(completeHandlers(handler))
	if err != nil || len(got) != 6 {
		t.Fatalf("%s: complete registration = (%v, %v)", assertion, got, err)
	}
}

func TestOfflineNormalizesHandlerFailure(t *testing.T) {
	assertion := "P3_STABLE_HANDLER_FAILURE_NORMALIZATION"
	cause := errors.New("semantic failure")
	handlerFailure := &Failure{Diagnostics: []string{"detail"}, Err: cause}
	executor := NewOffline(recordingValidator{events: &[]string{}}, map[Name]Handler{
		Inspect: func(context.Context, Request) (Result, *Failure) {
			return Result{Artifact: []byte("must not escape")}, handlerFailure
		},
	})

	got, failure := executor.Execute(context.Background(), Request{Name: Inspect, Input: json.RawMessage(`{}`)})
	if len(got.Artifact) != 0 || failure == nil || failure.Code != "INTERNAL" {
		t.Fatalf("%s: result = %#v, failure = %#v", assertion, got, failure)
	}
	if !reflect.DeepEqual(failure.Diagnostics, []string{"detail"}) || !errors.Is(failure, cause) {
		t.Errorf("%s: diagnostics/causality = %#v", assertion, failure)
	}
}

func TestOfflineDispatchesExactlySixTypedOperations(t *testing.T) {
	assertion := "P1_TYPED_SIX_OPERATION_DISPATCH"
	events := []string{}
	handlers := map[Name]Handler{}
	for _, name := range []Name{Capabilities, SchemaGet, Validate, Verify, Inspect, Filter} {
		name := name
		handlers[name] = func(_ context.Context, request Request) (Result, *Failure) {
			events = append(events, "semantic:"+string(request.Name))
			return Result{Value: string(name)}, nil
		}
	}
	executor := NewOffline(recordingValidator{events: &events}, handlers)
	for _, name := range []Name{Capabilities, SchemaGet, Validate, Verify, Inspect, Filter} {
		got, failure := executor.Execute(context.Background(), Request{Name: name, RequestID: "r", Input: json.RawMessage(`{}`)})
		if failure != nil || got.Value != string(name) {
			t.Errorf("%s: %s dispatch = (%v, %v)", assertion, name, got.Value, failure)
		}
	}
	if len(events) != 12 {
		t.Errorf("%s: event count = %d, want 12", assertion, len(events))
	}
}

func TestOfflineNamedEntryPointsUseTypedDispatch(t *testing.T) {
	assertion := "P1_NAMED_SIX_OPERATION_API"
	events := []string{}
	handlers := map[Name]Handler{}
	for _, name := range []Name{Capabilities, SchemaGet, Validate, Verify, Inspect, Filter} {
		name := name
		handlers[name] = func(context.Context, Request) (Result, *Failure) { return Result{Value: name}, nil }
	}
	executor := NewOffline(recordingValidator{events: &events}, handlers)
	calls := []struct {
		name Name
		call Handler
	}{
		{Capabilities, executor.ExecuteCapabilities}, {SchemaGet, executor.ExecuteSchemaGet},
		{Validate, executor.ExecuteValidate}, {Verify, executor.ExecuteVerify},
		{Inspect, executor.ExecuteInspect}, {Filter, executor.ExecuteFilter},
	}
	for _, tc := range calls {
		got, failure := tc.call(context.Background(), Request{Name: tc.name, Input: json.RawMessage(`{}`)})
		if failure != nil || got.Value != tc.name {
			t.Errorf("%s: %s = (%v, %v)", assertion, tc.name, got.Value, failure)
		}
	}
}

func TestOfflineStructuralFailurePrecedesAndSuppressesSemantics(t *testing.T) {
	assertionClass := "P2_STRUCTURAL_ERROR_CLASSIFICATION"
	assertionOrder := "P2_STRUCTURAL_BEFORE_SEMANTIC"
	events := []string{}
	structuralErr := errors.New("closed input rejected")
	handler := func(context.Context, Request) (Result, *Failure) {
		events = append(events, "semantic")
		return Result{}, nil
	}
	executor := NewOffline(recordingValidator{err: structuralErr, events: &events}, map[Name]Handler{Inspect: handler})
	_, failure := executor.Execute(context.Background(), Request{Name: Inspect, Input: json.RawMessage(`{"extra":true}`)})
	if failure == nil || failure.Code != "INVALID_INPUT" || !errors.Is(failure, structuralErr) {
		t.Errorf("%s: failure = %#v", assertionClass, failure)
	}
	if want := []string{"structural:inspect"}; !reflect.DeepEqual(events, want) {
		t.Errorf("%s: events = %v, want %v", assertionOrder, events, want)
	}
}

func TestOfflinePreservesSemanticArtifactAndAuthority(t *testing.T) {
	assertionParity := "P1_CLI_SEMANTIC_PARITY"
	assertionBoundary := "P3_AUTHORITY_CUSTODY_UNCHANGED"
	events := []string{}
	artifact := []byte("{\"authority\":\"NON_AUTHORITATIVE_DERIVED_VIEW\"}\n")
	value := map[string]any{"authority": "NON_AUTHORITATIVE_DERIVED_VIEW", "custody": "INTEGRITY_AND_CUSTODY_ONLY"}
	executor := NewOffline(recordingValidator{events: &events}, map[Name]Handler{
		Inspect: func(context.Context, Request) (Result, *Failure) {
			return Result{Value: value, Artifact: artifact}, nil
		},
	})
	got, failure := executor.Execute(context.Background(), Request{Name: Inspect, Input: json.RawMessage(`{}`)})
	if failure != nil || !reflect.DeepEqual(got.Artifact, artifact) {
		t.Errorf("%s: result = %#v, failure = %v", assertionParity, got, failure)
	}
	if !reflect.DeepEqual(got.Value, value) {
		t.Errorf("%s: value = %#v, want %#v", assertionBoundary, got.Value, value)
	}
}

func TestOfflineHasNoImplicitLiveExecutionPath(t *testing.T) {
	assertion := "P3_NO_LANGUAGE_SERVER_START"
	events := []string{}
	executor := NewOffline(recordingValidator{events: &events}, map[Name]Handler{
		SchemaGet: func(context.Context, Request) (Result, *Failure) {
			events = append(events, "offline-handler")
			return Result{}, nil
		},
	})
	_, failure := executor.Execute(context.Background(), Request{Name: SchemaGet, Input: json.RawMessage(`{}`)})
	if failure != nil {
		t.Fatalf("%s: %v", assertion, failure)
	}
	if want := []string{"structural:schema_get", "offline-handler"}; !reflect.DeepEqual(events, want) {
		t.Errorf("%s: events = %v, want %v", assertion, events, want)
	}
}
