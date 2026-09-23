package adr0011methodtransport_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type observationRuntime struct {
	metadata                 sessionruntime.SessionMetadata
	failure                  session.Failure
	result                   sessionruntime.RoundTripResult
	metadataCalls, wireCalls int
	wireParams               json.RawMessage
}

func (r *observationRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	r.metadataCalls++
	return r.metadata, r.failure
}
func (r *observationRuntime) RoundTrip(_ context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	r.wireCalls++
	r.wireParams = append(json.RawMessage(nil), req.Params...)
	return r.result
}

func observed(t *testing.T, result any) (reflect.Value, bool) {
	t.Helper()
	method := reflect.ValueOf(result).MethodByName("Observation")
	if !method.IsValid() {
		t.Fatal("ASSERT_TRANSACTION_OBSERVATION: accessor absent")
	}
	values := method.Call(nil)
	if len(values) != 2 || values[0].Kind() != reflect.Struct || values[1].Kind() != reflect.Bool {
		t.Fatal("ASSERT_TRANSACTION_OBSERVATION: accessor must return a value struct and presence bool")
	}
	return values[0], values[1].Bool()
}
func observedField(t *testing.T, value reflect.Value, name string) any {
	t.Helper()
	field := value.FieldByName(name)
	if !field.IsValid() || !field.CanInterface() {
		t.Fatalf("ASSERT_TRANSACTION_OBSERVATION: missing field %s", name)
	}
	return field.Interface()
}

func TestUnqualifiedObservationDistinguishesExactRequests(t *testing.T) {
	const first = ` {"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}} `
	const uriChanged = `{"textDocument":{"uri":"file:///w/b.go"},"position":{"line":0,"character":0}}`
	const positionChanged = `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":1,"character":0}}`
	const compact = `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}}`
	deadline := time.Now().Add(time.Second)
	var digests []string
	for _, tc := range []struct {
		raw        string
		generation uint64
		uri        string
		line       uint32
	}{
		{first, 1, "file:///w/a.go", 0},
		{uriChanged, 1, "file:///w/b.go", 0},
		{positionChanged, 1, "file:///w/a.go", 1},
		{first, 2, "file:///w/a.go", 0},
		{compact, 1, "file:///w/a.go", 0},
	} {
		runtime := &observationRuntime{
			metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, PositionEncoding: "utf-16", ProviderName: "reported-provider", ProviderVersion: "reported-version"},
			result:   sessionruntime.RoundTripResult{Key: lspwire.RequestKey{Generation: 1, ID: 77}, RequestMessages: 1, RequestBytes: 123, Messages: 2, Bytes: 234, Result: json.RawMessage(`[]`)},
		}
		got := transport.New(runtime).Execute(context.Background(), transport.Request{
			SessionID: "exact", Generation: tc.generation, Method: transport.MethodDefinition,
			Params: json.RawMessage(tc.raw), Deadline: deadline, MaxMessages: 5, MaxBytes: 8192,
		})
		obs, present := observed(t, got)
		digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(tc.raw)))
		if !present || got.Outcome() != transport.OutcomeTransportSuccess || runtime.metadataCalls != 1 || runtime.wireCalls != 1 || string(runtime.wireParams) != tc.raw ||
			observedField(t, obs, "ParamsSHA256") != digest || observedField(t, obs, "ParamsBytes") != len(tc.raw) ||
			observedField(t, obs, "DeclaredSessionID") != "exact" || observedField(t, obs, "DeclaredGeneration") != tc.generation || observedField(t, obs, "DeclaredMethod") != transport.MethodDefinition ||
			observedField(t, obs, "DeclaredQueryURI") != tc.uri || observedField(t, obs, "DeclaredLine") != tc.line || observedField(t, obs, "DeclaredCharacter") != uint32(0) ||
			observedField(t, obs, "DeclaredDeadline") != deadline || observedField(t, obs, "DeclaredMaxMessages") != 5 || observedField(t, obs, "DeclaredMaxBytes") != int64(8192) ||
			observedField(t, obs, "MetadataObserved") != true || observedField(t, obs, "ReportedMethodAdvertised") != true ||
			observedField(t, obs, "ReportedPositionEncoding") != "utf-16" || observedField(t, obs, "ReportedProviderName") != "reported-provider" || observedField(t, obs, "ReportedProviderVersion") != "reported-version" ||
			observedField(t, obs, "RoundTripCalled") != true || observedField(t, obs, "ReportedKey") != (lspwire.RequestKey{Generation: 1, ID: 77}) ||
			observedField(t, obs, "ReportedRequestMessages") != 1 || observedField(t, obs, "ReportedRequestBytes") != int64(123) ||
			observedField(t, obs, "ReportedResponseMessages") != 2 || observedField(t, obs, "ReportedResponseBytes") != int64(234) {
			t.Fatalf("ASSERT_TRANSACTION_OBSERVATION: present=%t outcome=%s metadata=%d wire=%d reportedCalled=%v paramsEqual=%t digest=%v want=%s", present, got.Outcome(), runtime.metadataCalls, runtime.wireCalls, observedField(t, obs, "RoundTripCalled"), string(runtime.wireParams) == tc.raw, observedField(t, obs, "ParamsSHA256"), digest)
		}
		if observedField(t, obs, "ReportedKey") == (lspwire.RequestKey{Generation: tc.generation, ID: 77}) && tc.generation == 2 {
			t.Fatal("ASSERT_TRANSACTION_OBSERVATION: mismatched reported generation was silently repaired")
		}
		body, err := json.Marshal(obs.Interface())
		if err != nil || strings.Contains(string(body), `"textDocument"`) || strings.Contains(string(body), `"result"`) || strings.Contains(string(body), `"server_error"`) {
			t.Fatalf("ASSERT_TRANSACTION_OBSERVATION: raw parameter/result material in observation: marshalErr=%v", err)
		}
		digests = append(digests, digest)
		t.Log("ASSERT_TRANSACTION_OBSERVATION: PASS distinct declared request and reported wire")
	}
	if digests[0] == digests[1] || digests[0] == digests[2] || digests[0] != digests[3] || digests[0] == digests[4] {
		t.Fatal("ASSERT_TRANSACTION_OBSERVATION: URI, position, generation, or whitespace substitution was not separated")
	}
}

func TestUnqualifiedObservationSeparatesCapabilityAndPreflight(t *testing.T) {
	req := transport.Request{SessionID: "exact", Generation: 1, Method: transport.MethodReferences,
		Params:   json.RawMessage(`{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0},"context":{"includeDeclaration":false}}`),
		Deadline: time.Now().Add(time.Second), MaxMessages: 5, MaxBytes: 8192}
	runtime := &observationRuntime{metadata: sessionruntime.SessionMetadata{PositionEncoding: "utf-16"}}
	got := transport.New(runtime).Execute(context.Background(), req)
	obs, present := observed(t, got)
	if !present || got.Outcome() != transport.OutcomeUnsupportedCapability || runtime.metadataCalls != 1 || runtime.wireCalls != 0 || observedField(t, obs, "MetadataObserved") != true || observedField(t, obs, "ReportedMethodAdvertised") != false || observedField(t, obs, "RoundTripCalled") != false || observedField(t, obs, "DeclaredIncludeDeclaration") != false || observedField(t, obs, "IncludeDeclarationPresent") != true {
		t.Fatalf("ASSERT_TRANSACTION_OBSERVATION: unsupported capability presence=%t outcome=%s metadata=%d wire=%d", present, got.Outcome(), runtime.metadataCalls, runtime.wireCalls)
	}
	t.Log("ASSERT_TRANSACTION_OBSERVATION: PASS capability absent without wire")

	runtime = &observationRuntime{failure: session.StaleGeneration}
	got = transport.New(runtime).Execute(context.Background(), req)
	obs, present = observed(t, got)
	if !present || got.Outcome() != transport.OutcomePreflightFailure || runtime.metadataCalls != 1 || runtime.wireCalls != 0 || observedField(t, obs, "MetadataObserved") != false || observedField(t, obs, "RoundTripCalled") != false {
		t.Fatalf("ASSERT_TRANSACTION_OBSERVATION: metadata failure presence=%t outcome=%s metadata=%d wire=%d", present, got.Outcome(), runtime.metadataCalls, runtime.wireCalls)
	}
	t.Log("ASSERT_TRANSACTION_OBSERVATION: PASS metadata failure without capability claim")

	req.Generation = 0
	runtime = &observationRuntime{}
	got = transport.New(runtime).Execute(context.Background(), req)
	_, present = observed(t, got)
	if present || got.Outcome() != transport.OutcomePreflightFailure || runtime.metadataCalls != 0 || runtime.wireCalls != 0 {
		t.Fatalf("ASSERT_TRANSACTION_OBSERVATION: invalid preflight presence=%t outcome=%s metadata=%d wire=%d", present, got.Outcome(), runtime.metadataCalls, runtime.wireCalls)
	}
	t.Log("ASSERT_TRANSACTION_OBSERVATION: PASS invalid preflight has no observation")
}
