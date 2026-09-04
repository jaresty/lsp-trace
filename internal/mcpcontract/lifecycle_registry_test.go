package mcpcontract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
)

func TestLifecycleContractProjectsClosedOperationRegistry(t *testing.T) {
	const assertion = "ASSERT_LIFECYCLE_PROJECTION_REGISTRY_IS_CLOSED_AND_IMMUTABLE"
	t.Log("ASSERTION: " + assertion)

	contract, err := LoadLifecycleContract()
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	if contract.CandidateStatus != "UNACCEPTED_NOT_RUNTIME" || contract.ContractVersion != "1" || contract.Extends != "stage1-manifest.v1" {
		t.Fatalf("%s: identity=%+v", assertion, contract)
	}
	for _, state := range []AvailabilityState{NotImplementedState, ContainmentUnavailableState, RuntimeDisabledState, EnabledState} {
		first, err := contract.Project(state)
		if err != nil {
			t.Fatalf("%s[%s]: %v", assertion, state, err)
		}
		if len(first) != 4 {
			t.Fatalf("%s[%s]: tools=%d", assertion, state, len(first))
		}
		for _, tool := range first {
			if tool.Availability != string(state) || tool.Advertised != (state == EnabledState) || len(tool.EnvelopeSchemaIDs) == 0 {
				t.Errorf("%s[%s/%s]: %+v", assertion, state, tool.Name, tool)
			}
		}
		first[0].Aliases[0] = "mutated"
		second, _ := contract.Project(state)
		if second[0].Aliases[0] == "mutated" {
			t.Fatalf("%s[%s]: projection aliases storage", assertion, state)
		}
	}
	if _, err := contract.Project(AvailabilityState("ANALYTICS")); err == nil || !strings.Contains(err.Error(), "unknown lifecycle availability") {
		t.Fatalf("%s: invalid state accepted: %v", assertion, err)
	}
}

func TestLifecycleOperationInputValidatorUsesProjectedRegistry(t *testing.T) {
	const assertion = "ASSERT_LIFECYCLE_OPERATION_VALIDATOR_STRUCTURAL_BOUNDARY"
	t.Log("ASSERTION: " + assertion)

	validator, err := NewLifecycleOperationInputValidator(EnabledState)
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	cases := []struct {
		name  operation.Name
		input string
	}{
		{operation.Name("session_list"), `{}`},
		{operation.Name("session_status"), `{"selector":{"session_id":"s"},"generation":1}`},
		{operation.Name("session_stop"), `{"selector":{"session_id":"s"},"generation":1}`},
		{operation.Name("session_restart"), `{"selector":{"session_id":"s"},"generation":1}`},
	}
	for _, tc := range cases {
		if err := validator.ValidateOperationInput(tc.name, json.RawMessage(tc.input)); err != nil {
			t.Errorf("%s[%s]: %v", assertion, tc.name, err)
		}
	}
	if err := validator.ValidateOperationInput(operation.Name("session_status"), json.RawMessage(`{"session_id":"s","analytics":true}`)); err == nil {
		t.Fatalf("%s: analytics field accepted", assertion)
	}
	if err := validator.ValidateOperationInput(operation.Name("analytics"), json.RawMessage(`{}`)); err == nil {
		t.Fatalf("%s: unregistered operation accepted", assertion)
	}

	got := validator.Operations()
	want := []operation.Name{"session_list", "session_restart", "session_status", "session_stop"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: operations=%v want=%v", assertion, got, want)
	}
	got[0] = "mutated"
	if reflect.DeepEqual(got, validator.Operations()) {
		t.Fatalf("%s: operations storage shared", assertion)
	}
}
