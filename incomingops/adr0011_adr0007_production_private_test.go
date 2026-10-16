package incomingops

import (
	"encoding/json"
	"errors"
	"testing"

	"lsp-trace/internal/operation"
)

func TestProductionPrivateSelectorNormalization(t *testing.T) {
	base := `{"session_id":"session-1","generation":1,"uri":"file:///workspace/calls.go","symbol":"Target","max_depth":1,"max_nodes":8,"timeout_ms":5000,"request_timeout_ms":2000}`
	explicit := `{"session_id":"session-1","generation":1,"uri":"file:///workspace/calls.go","symbol":"Target","relations":["CALLS"],"max_depth":1,"max_nodes":8,"timeout_ms":5000,"request_timeout_ms":2000}`
	omitted, omittedErr := NormalizeCallsOnlyPrivate(operation.Request{Name: OperationIncoming, Input: json.RawMessage(base)})
	if errors.Is(omittedErr, ErrBehaviorNotImplementedPrivate) {
		t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_INCOMINGOPS")
	}
	explicitCalls, explicitErr := NormalizeCallsOnlyPrivate(operation.Request{Name: OperationIncoming, Input: json.RawMessage(explicit)})
	if omittedErr != nil || explicitErr != nil {
		t.Fatalf("ASSERT_OMITTED_EXPLICIT_CALLS_ONLY_PRODUCTION_RESULT: errors=%v/%v", omittedErr, explicitErr)
	}
	if len(omitted) != 1 || omitted[0].Name != "CALLS" {
		t.Fatalf("ASSERT_OMITTED_CALLS_ONLY_EXACT_SINGLE: %+v", omitted)
	}
	if len(explicitCalls) != 1 || explicitCalls[0].Name != "CALLS" {
		t.Fatalf("ASSERT_EXPLICIT_CALLS_ONLY_EXACT_SINGLE: %+v", explicitCalls)
	}
	if omitted[0] != explicitCalls[0] {
		t.Fatalf("ASSERT_OMITTED_EXPLICIT_CALLS_ONLY_SAME_EXPANSION: omitted=%+v explicit=%+v", omitted, explicitCalls)
	}
}

func TestProductionPrivateSelectorClassification(t *testing.T) {
	cases := []struct {
		name  string
		facts SelectorFactsPrivate
		want  SelectorClassificationPrivate
	}{
		{name: "provider-only", facts: SelectorFactsPrivate{ProviderRelations: 1}, want: SelectorClassificationPrivate{Family: "PROVIDER_ONLY", Code: "PROVIDER_ONLY_SELECTOR"}},
		{name: "mixed-provider-generic", facts: SelectorFactsPrivate{ProviderRelations: 1, GenericRelations: 1}, want: SelectorClassificationPrivate{Family: "MIXED_PROVIDER_GENERIC", Code: "MIXED_PROVIDER_GENERIC_SELECTOR"}},
		{name: "generic", facts: SelectorFactsPrivate{GenericRelations: 1}, want: SelectorClassificationPrivate{Family: "GENERIC", Code: "GENERIC_SELECTOR"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClassifySelectorPrivate(tc.facts)
			if errors.Is(err, ErrBehaviorNotImplementedPrivate) {
				t.Fatal("ASSERT_SKELETON_ERR_BEHAVIOR_NOT_IMPLEMENTED_SELECTOR_CLASSIFICATION")
			}
			if err != nil {
				t.Fatalf("ASSERT_SELECTOR_CLASSIFICATION_PRODUCTION_RESULT[%s]: %v", tc.name, err)
			}
			if got.Family != tc.want.Family || got.Code != tc.want.Code {
				t.Fatalf("ASSERT_SELECTOR_CLASSIFICATION_EXACT_FAMILY_CODE[%s]: got=%+v want=%+v", tc.name, got, tc.want)
			}
		})
	}
}
