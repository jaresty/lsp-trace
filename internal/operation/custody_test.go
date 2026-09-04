package operation

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

var custodyStages = []CustodyStage{StageDiscovery, StageReceipt, StageManifest, StageSnapshot, StageAdmission, StagePublication}

func custodyHandlers(events *[]CustodyStage) map[CustodyStage]CustodyStageHandler {
	handlers := make(map[CustodyStage]CustodyStageHandler, len(custodyStages))
	for _, stage := range custodyStages {
		stage := stage
		handlers[stage] = func(_ context.Context, _ CustodyRequest, _ CustodyResponse) (StageResult, error) {
			*events = append(*events, stage)
			result := StageResult{Artifact: json.RawMessage(`{"stage":"` + stage + `","values":{"b":2,"a":1}}`)}
			if stage == StageAdmission {
				admitted := true
				result.Admitted = &admitted
			}
			return result, nil
		}
	}
	return handlers
}

func successfulCustodyOperation(events *[]CustodyStage) *CustodyOperation {
	operation, err := NewCustodyOperation(custodyHandlers(events))
	if err != nil {
		panic(err)
	}
	return operation
}

func TestCustodyOperationCanonicalRegistration(t *testing.T) {
	const assertion = "P1_CANONICAL_CUSTODY_OPERATION_NAME"
	if CustodyExecute != Name("execute") {
		t.Fatalf("%s: got %q", assertion, CustodyExecute)
	}
}

func TestCustodyOperationRegistrationIsClosedAndImmutable(t *testing.T) {
	t.Run("complete", func(t *testing.T) {
		const assertion = "P2A_COMPLETE_STAGE_REGISTRATION"
		handlers := custodyHandlers(&[]CustodyStage{})
		for _, stage := range custodyStages {
			incomplete := make(map[CustodyStage]CustodyStageHandler, len(handlers)-1)
			for candidate, handler := range handlers {
				if candidate != stage {
					incomplete[candidate] = handler
				}
			}
			if _, err := NewCustodyOperation(incomplete); err == nil {
				t.Fatalf("%s: accepted missing stage %q", assertion, stage)
			}
		}
		handlers[StageManifest] = nil
		if _, err := NewCustodyOperation(handlers); err == nil {
			t.Fatalf("%s: accepted nil stage handler", assertion)
		}
	})

	t.Run("immutable", func(t *testing.T) {
		const assertion = "P2B_CALLER_MUTATION_CANNOT_CHANGE_AVAILABILITY"
		events := []CustodyStage{}
		handlers := custodyHandlers(&events)
		operation, err := NewCustodyOperation(handlers)
		if err != nil {
			t.Fatalf("%s: construct: %v", assertion, err)
		}
		delete(handlers, StageManifest)
		response, failure := operation.ExecuteCustody(context.Background(), CustodyRequest{OperationID: "op-1", Input: json.RawMessage(`{}`)})
		if failure != nil || len(events) != len(custodyStages) {
			t.Fatalf("%s: events=%v lifecycle=%v failure=%v", assertion, events, response.Lifecycle, failure)
		}
	})
}

func TestCustodyCanonicalModelsAndTransportNeutrality(t *testing.T) {
	const assertion = "P1_CANONICAL_REQUEST_RESPONSE_AND_P6_TRANSPORT_NEUTRAL"
	requestType := reflect.TypeOf(CustodyRequest{})
	responseType := reflect.TypeOf(CustodyResponse{})
	for _, typ := range []reflect.Type{requestType, responseType} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name + " " + typ.Field(i).Tag.Get("json"))
			if strings.Contains(name, "cli") || strings.Contains(name, "mcp") || strings.Contains(name, "transport") || strings.Contains(name, "exit") {
				t.Fatalf("%s: transport-local field %q", assertion, name)
			}
		}
	}
	events := []CustodyStage{}
	response, failure := successfulCustodyOperation(&events).ExecuteCustody(context.Background(), CustodyRequest{OperationID: "op-1", Input: json.RawMessage(`{"source":"x"}`)})
	if failure != nil || response.OperationID != "op-1" {
		t.Fatalf("%s: response=%#v failure=%v", assertion, response, failure)
	}
}

func TestCustodyExecutesFixedSequenceAndReportsLifecycle(t *testing.T) {
	const assertion = "P3A_FIXED_STAGE_SEQUENCE"
	events := []CustodyStage{}
	response, failure := successfulCustodyOperation(&events).ExecuteCustody(context.Background(), CustodyRequest{OperationID: "op-1", Input: json.RawMessage(`{"source":"x"}`)})
	if failure != nil {
		t.Fatalf("%s: failure=%v", assertion, failure)
	}
	if !reflect.DeepEqual(events, custodyStages) {
		t.Fatalf("%s: events=%v want=%v", assertion, events, custodyStages)
	}
	if len(response.Lifecycle) != len(custodyStages) {
		t.Fatalf("%s: lifecycle=%#v", assertion, response.Lifecycle)
	}
	for i, record := range response.Lifecycle {
		if record.Stage != custodyStages[i] || record.Availability != AvailabilityAvailable || record.State != LifecycleSucceeded {
			t.Errorf("%s: record[%d]=%#v", assertion, i, record)
		}
	}
}

func TestCustodyLogicalDigestCanonicalizesEquivalentJSON(t *testing.T) {
	const assertion = "P4A_CANONICAL_LOGICAL_DIGEST"
	requestA := CustodyRequest{OperationID: "runtime", Input: json.RawMessage(`{"z":3,"a":1}`)}
	requestB := CustodyRequest{OperationID: "runtime", Input: json.RawMessage(`{"a":1,"z":3}`)}
	responseA, failureA := successfulCustodyOperation(&[]CustodyStage{}).ExecuteCustody(context.Background(), requestA)
	responseB, failureB := successfulCustodyOperation(&[]CustodyStage{}).ExecuteCustody(context.Background(), requestB)
	if failureA != nil || failureB != nil || responseA.LogicalDigest == "" || responseA.LogicalDigest != responseB.LogicalDigest {
		t.Fatalf("%s: digestA=%q digestB=%q failures=(%v,%v)", assertion, responseA.LogicalDigest, responseB.LogicalDigest, failureA, failureB)
	}
}

func TestCustodyLogicalDigestExcludesOperationIdentity(t *testing.T) {
	const assertion = "P4B_LOGICAL_DIGEST_EXCLUDES_OPERATION_IDENTITY"
	input := json.RawMessage(`{"a":1,"z":3}`)
	responseA, failureA := successfulCustodyOperation(&[]CustodyStage{}).ExecuteCustody(context.Background(), CustodyRequest{OperationID: "runtime-a", Input: input})
	responseB, failureB := successfulCustodyOperation(&[]CustodyStage{}).ExecuteCustody(context.Background(), CustodyRequest{OperationID: "runtime-b", Input: input})
	if failureA != nil || failureB != nil || responseA.LogicalDigest == "" || responseA.LogicalDigest != responseB.LogicalDigest {
		t.Fatalf("%s: digestA=%q digestB=%q failures=(%v,%v)", assertion, responseA.LogicalDigest, responseB.LogicalDigest, failureA, failureB)
	}
}

func TestCustodyTypedFailuresSuppressLaterStages(t *testing.T) {
	cause := errors.New("stage exploded")
	tests := []struct {
		name      string
		configure func(*CustodyOperation)
		wantCode  string
		wantStage CustodyStage
		wantCause error
	}{
		{"unavailable", func(o *CustodyOperation) { delete(o.Handlers, StageManifest) }, CustodyFailureStageUnavailable, StageManifest, nil},
		{"stage_failure", func(o *CustodyOperation) {
			o.Handlers[StageSnapshot] = func(context.Context, CustodyRequest, CustodyResponse) (StageResult, error) {
				return StageResult{}, cause
			}
		}, CustodyFailureStageFailed, StageSnapshot, cause},
		{"publication_failure", func(o *CustodyOperation) {
			o.Handlers[StagePublication] = func(context.Context, CustodyRequest, CustodyResponse) (StageResult, error) {
				return StageResult{}, cause
			}
		}, CustodyFailurePublicationFailed, StagePublication, cause},
		{"admission_rejected", func(o *CustodyOperation) {
			o.Handlers[StageAdmission] = func(context.Context, CustodyRequest, CustodyResponse) (StageResult, error) {
				admitted := false
				return StageResult{Admitted: &admitted}, nil
			}
		}, CustodyFailureAdmissionRejected, StageAdmission, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertion := "P3B_STABLE_FAILURE_LIFECYCLE_" + strings.ToUpper(tc.name)
			events := []CustodyStage{}
			o := successfulCustodyOperation(&events)
			tc.configure(o)
			response, failure := o.ExecuteCustody(context.Background(), CustodyRequest{OperationID: "op-1", Input: json.RawMessage(`{}`)})
			if failure == nil || failure.Code != tc.wantCode || failure.Stage != tc.wantStage || (tc.wantCause != nil && !errors.Is(failure, tc.wantCause)) {
				t.Fatalf("%s: response=%#v failure=%#v", assertion, response, failure)
			}
			failed := false
			for _, record := range response.Lifecycle {
				if record.Stage == tc.wantStage {
					failed = record.State == LifecycleFailed
				}
				if failed && record.Stage != tc.wantStage && record.State != LifecycleSkipped {
					t.Errorf("%s: later record=%#v", assertion, record)
				}
			}
		})
	}
}

func TestCustodyRejectsInvalidRequestBeforeStages(t *testing.T) {
	const assertion = "P4_INVALID_REQUEST_TYPED_AND_SUPPRESSED"
	events := []CustodyStage{}
	response, failure := successfulCustodyOperation(&events).ExecuteCustody(context.Background(), CustodyRequest{Input: json.RawMessage(`not-json`)})
	if failure == nil || failure.Code != CustodyFailureInvalidRequest || failure.Stage != StageDiscovery || len(events) != 0 {
		t.Fatalf("%s: response=%#v failure=%#v events=%v", assertion, response, failure, events)
	}
}
