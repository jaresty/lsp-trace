package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
)

func providerMessageCompositionRequest(s StartResult) RoundTripRequest {
	return RoundTripRequest{
		SessionID: s.SessionID, Generation: s.Generation,
		Method: "textDocument/documentSymbol", Params: json.RawMessage(`{"textDocument":{"uri":"file:///fixture.go"}}`),
		Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096,
		CaptureProviderMessageComposition: true,
	}
}

func TestProviderMessageCompositionAcceptedSuccessAndDisabledParity(t *testing.T) {
	m, s, _ := roundTripManager(t, "success")
	request := providerMessageCompositionRequest(s)
	got := m.RoundTrip(context.Background(), request)
	observation, ok := got.ProviderMessageComposition()
	if !ok {
		t.Fatal("ASSERT_PROVIDER_MESSAGE_COMPOSITION_PRESENT")
	}
	if observation.LogShowTelemetryNotifications != 1 || observation.UnmatchedLateInvalidResponses != 1 || observation.AcceptedTerminalResponses != 1 || observation.Outcome != ProviderMessageOutcomeAcceptedTerminalResponse {
		t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_SUCCESS: %+v", observation)
	}
	if got.Failure != "" || got.Messages != 3 || len(got.Notifications) != 1 || len(got.Responses) != 1 {
		t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_EXISTING_RESULT: %+v", got)
	}

	m2, s2, _ := roundTripManager(t, "references-empty")
	disabled := providerMessageCompositionRequest(s2)
	disabled.CaptureProviderMessageComposition = false
	without := m2.RoundTrip(context.Background(), disabled)
	if _, ok := without.ProviderMessageComposition(); ok {
		t.Fatal("ASSERT_PROVIDER_MESSAGE_COMPOSITION_DISABLED_ABSENT")
	}
}

func TestProviderMessageCompositionAcceptedServerError(t *testing.T) {
	m, s, _ := roundTripManager(t, "server-error")
	request := providerMessageCompositionRequest(s)
	request.MaxMessages = 1
	got := m.RoundTrip(context.Background(), request)
	observation, ok := got.ProviderMessageComposition()
	if !ok || got.ServerError == nil || observation.AcceptedTerminalResponses != 1 || observation.Outcome != ProviderMessageOutcomeAcceptedTerminalResponse {
		t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_SERVER_ERROR: result=%+v observation=%+v ok=%v", got, observation, ok)
	}
}

func TestProviderMessageCompositionInvalidProtocolAndPrivacy(t *testing.T) {
	m, s, _ := roundTripManager(t, "malformed")
	request := providerMessageCompositionRequest(s)
	request.MaxMessages = 1
	got := m.RoundTrip(context.Background(), request)
	observation, ok := got.ProviderMessageComposition()
	if !ok || observation.InvalidProtocolMessages != 1 || observation.Outcome != ProviderMessageOutcomeInvalidProtocolMessage {
		t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_INVALID_PROTOCOL: result=%+v observation=%+v ok=%v", got, observation, ok)
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(ProviderMessageComposition{}), reflect.TypeOf(observation)} {
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			switch name {
			case "Method", "ID", "Payload", "Error", "URI", "Path", "Hash", "Generation":
				t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_PRIVACY: forbidden field %s", name)
			}
		}
	}
	raw, err := json.Marshal(got)
	observationRaw, observationErr := json.Marshal(observation)
	if err != nil || observationErr != nil || string(raw) == "" || containsProviderCompositionJSON(raw) || string(observationRaw) != "{}" {
		t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_JSON_OMITTED: result=%s observation=%s err=%v observationErr=%v", raw, observationRaw, err, observationErr)
	}
}

func TestProviderMessageCompositionExactBucketsAndInvalidAllowlist(t *testing.T) {
	var accumulator providerMessageCompositionAccumulator
	accumulator.enabled = true
	for _, method := range []string{"$/progress", "window/logMessage", "window/showMessage", "telemetry/event", "$/progress/near", "window/logMessage/near"} {
		accumulator.dispatch(lspwire.Message{JSONRPC: lspwire.Version, Method: method}, false)
	}
	accumulator.dispatch(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`7`), Method: "workspace/configuration"}, false)
	accumulator.dispatch(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`8`), Result: json.RawMessage(`null`)}, false)
	accumulator.dispatch(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`9`), Error: &lspwire.RPCError{Code: -32603}}, true)
	if accumulator.ProgressNotifications != 1 || accumulator.LogShowTelemetryNotifications != 3 || accumulator.OtherNotifications != 2 || accumulator.ServerRequests != 1 || accumulator.UnmatchedLateInvalidResponses != 1 || accumulator.AcceptedTerminalResponses != 1 {
		t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_EXACT_BUCKETS: %+v", accumulator.ProviderMessageComposition)
	}

	for _, err := range []error{lspwire.ErrMalformedJSON, lspwire.ErrWrongVersion, lspwire.ErrInvalidMessage, lspwire.ErrInvalidContentLength, lspwire.ErrDuplicateContentLength, lspwire.ErrMissingContentLength} {
		if !isProviderInvalidProtocol(err) || !isProviderInvalidProtocol(errors.Join(errors.New("wrapped"), err)) {
			t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_INVALID_ALLOWLIST: %v", err)
		}
	}
	for _, err := range []error{io.EOF, io.ErrUnexpectedEOF, lspwire.ErrFrameTooLarge, lspwire.ErrHeaderTooLarge, lspwire.ErrOriginalFrameTooLarge, lspwire.ErrCumulativeOriginalFramesTooLarge, errors.New("transport-unavailable"), context.Canceled, context.DeadlineExceeded} {
		if isProviderInvalidProtocol(err) {
			t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_INVALID_EXCLUSION: %v", err)
		}
	}
}

func TestProviderMessageCompositionCounterOverflowIsClosed(t *testing.T) {
	accumulator := providerMessageCompositionAccumulator{enabled: true}
	accumulator.ProgressNotifications = math.MaxUint64
	accumulator.dispatch(lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress"}, false)
	observation, ok := accumulator.finish("")
	if !ok || observation.ProgressNotifications != math.MaxUint64 || observation.Outcome != ProviderMessageOutcomeOtherRuntimeFailure {
		t.Fatalf("ASSERT_PROVIDER_MESSAGE_COMPOSITION_OVERFLOW: %+v ok=%v", observation, ok)
	}
}

func containsProviderCompositionJSON(raw []byte) bool {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return true
	}
	_, found := value["provider_message_composition"]
	return found
}
