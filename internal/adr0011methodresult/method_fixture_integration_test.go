package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// fixtureRuntime is an in-process fake, NOT a managed session or LSP wire peer.
type fixtureRuntime struct {
	metadata                 sessionruntime.SessionMetadata
	metadataFailure          session.Failure
	response                 sessionruntime.RoundTripResult
	metadataCalls, wireCalls int
	metadataID               string
	metadataGeneration       uint64
	wire                     sessionruntime.RoundTripRequest
}

func (f *fixtureRuntime) Metadata(id string, generation uint64) (sessionruntime.SessionMetadata, session.Failure) {
	f.metadataCalls++
	f.metadataID, f.metadataGeneration = id, generation
	return f.metadata, f.metadataFailure
}
func (f *fixtureRuntime) RoundTrip(_ context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	f.wireCalls++
	f.wire = req
	return f.response
}

func fixtureRequest(method string, include bool) transport.Request {
	params := `{"textDocument":{"uri":"file:///w/q.go"},"position":{"line":0,"character":1}`
	if method == transport.MethodReferences {
		if include {
			params += `,"context":{"includeDeclaration":true}`
		} else {
			params += `,"context":{"includeDeclaration":false}`
		}
	}
	params += `}`
	return transport.Request{SessionID: "exact-fixture", Generation: 7, Method: method, Params: json.RawMessage(params),
		Deadline: time.Now().Add(20 * time.Second), MaxMessages: 5, MaxBytes: 8192}
}

func fixtureResponse(raw string) sessionruntime.RoundTripResult {
	return sessionruntime.RoundTripResult{Result: json.RawMessage(raw), Messages: 1, Bytes: int64(len(raw) + 100)}
}

func TestADR0011InProcessFailureFixturesRemainTransportOnly(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		metadata     sessionruntime.SessionMetadata
		failure      session.Failure
		response     sessionruntime.RoundTripResult
		want         transport.Outcome
		calls        int
	}{
		{"D-01", transport.MethodDefinition, sessionruntime.SessionMetadata{}, "", fixtureResponse(`[]`), transport.OutcomeUnsupportedCapability, 0},
		{"R-01", transport.MethodReferences, sessionruntime.SessionMetadata{}, "", fixtureResponse(`[]`), transport.OutcomeUnsupportedCapability, 0},
		{"D-08", transport.MethodDefinition, sessionruntime.SessionMetadata{DefinitionSupport: true}, session.StaleGeneration, fixtureResponse(`[]`), transport.OutcomePreflightFailure, 0},
		{"R-08", transport.MethodReferences, sessionruntime.SessionMetadata{ReferencesSupport: true}, session.StaleGeneration, fixtureResponse(`[]`), transport.OutcomePreflightFailure, 0},
		{"D-09", transport.MethodDefinition, sessionruntime.SessionMetadata{DefinitionSupport: true}, "", sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`), Messages: 1, Bytes: 8193}, transport.OutcomeTransportFailure, 1},
		{"R-09", transport.MethodReferences, sessionruntime.SessionMetadata{ReferencesSupport: true}, "", sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`), Messages: 1, Bytes: 8193}, transport.OutcomeTransportFailure, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fixtureRuntime{metadata: tc.metadata, metadataFailure: tc.failure, response: tc.response}
			req := fixtureRequest(tc.method, true)
			wire := transport.New(f).Execute(context.Background(), req)
			parsed, parseFailure := Parse(wire, 3)
			obs, present := wire.Observation()
			if wire.Outcome() != tc.want || f.metadataCalls != 1 || f.metadataID != req.SessionID || f.metadataGeneration != req.Generation ||
				f.wireCalls != tc.calls || !present || parseFailure == nil || parseFailure.Code != FailureTransport ||
				parseFailure.Ordinal != -1 || len(parsed.Items) != 0 || parsed.Null || len(wire.Raw()) != 0 ||
				obs.RawResultBytes != 0 || obs.RawResultSHA256 != "" {
				t.Fatalf("ASSERT_ADR0011_WIRE_PARSE_FAILURE: case=%s outcome=%s calls=%d parse=%+v items=%d observation=%+v", tc.name, wire.Outcome(), f.wireCalls, parseFailure, len(parsed.Items), obs)
			}
			if tc.calls == 0 && obs.RawResultDisposition != transport.RawResultNotInvoked || tc.calls == 1 && obs.RawResultDisposition != transport.RawResultWithheld {
				t.Fatalf("ASSERT_ADR0011_WIRE_PARSE_FAILURE: case=%s rawDisposition=%s", tc.name, obs.RawResultDisposition)
			}
			t.Log("ASSERT_ADR0011_WIRE_PARSE_FAILURE: PASS")
		})
	}
}

func TestADR0011InProcessSuccessFixturesKeepRequestAndMemberOrder(t *testing.T) {
	for _, tc := range []struct {
		name, method, raw string
		include           bool
		count             int
		null              bool
		kind              Kind
	}{
		{"D-02", transport.MethodDefinition, `[]`, false, 0, false, ""},
		{"R-02", transport.MethodReferences, `[]`, true, 0, false, ""},
		{"D-03", transport.MethodDefinition, `null`, false, 0, true, ""},
		{"D-04", transport.MethodDefinition, `[` + loc + `,` + loc + `]`, false, 2, false, Location},
		{"D-05", transport.MethodDefinition, `[` + link + `,` + link + `]`, false, 2, false, LocationLink},
		{"R-03", transport.MethodReferences, `[` + loc + `]`, true, 1, false, Location},
		{"R-04", transport.MethodReferences, `[` + loc + `]`, false, 1, false, Location},
		{"R-05", transport.MethodReferences, `[` + loc + `,` + loc + `]`, false, 2, false, Location},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fixtureRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}, response: fixtureResponse(tc.raw)}
			req := fixtureRequest(tc.method, tc.include)
			wire := transport.New(f).Execute(context.Background(), req)
			parsed, failed := Parse(wire, 3)
			obs, present := wire.Observation()
			if tc.count == 2 && (failed != nil || len(parsed.Items) != 2) {
				t.Fatalf("ASSERT_ADR0011_MEMBER_MULTIPLICITY: case=%s items=%+v failure=%+v", tc.name, parsed.Items, failed)
			}
			if wire.Outcome() != transport.OutcomeTransportSuccess || failed != nil || f.metadataCalls != 1 || f.wireCalls != 1 ||
				f.wire.Method != tc.method || f.wire.SessionID != req.SessionID || f.wire.Generation != req.Generation ||
				string(f.wire.Params) != string(req.Params) || string(wire.Raw()) != tc.raw || !present ||
				obs.RawResultDisposition != transport.RawResultRetained || obs.RawResultBytes != len(tc.raw) ||
				obs.DeclaredQueryURI != "file:///w/q.go" || obs.DeclaredLine != 0 || obs.DeclaredCharacter != 1 ||
				len(parsed.Items) != tc.count || parsed.Null != tc.null {
				t.Fatalf("ASSERT_ADR0011_WIRE_PARSE_SUCCESS: case=%s outcome=%s failed=%+v calls=%d obs=%+v parsed=%+v", tc.name, wire.Outcome(), failed, f.wireCalls, obs, parsed)
			}
			if tc.method == transport.MethodReferences && (!obs.IncludeDeclarationPresent || obs.DeclaredIncludeDeclaration != tc.include) ||
				tc.method == transport.MethodDefinition && obs.IncludeDeclarationPresent {
				t.Fatalf("ASSERT_ADR0011_REQUEST_CONTEXT: case=%s observation=%+v", tc.name, obs)
			}
			for i, item := range parsed.Items {
				if item.Ordinal != i || item.Kind != tc.kind || item.URI != "file:///w/a.go" {
					t.Fatalf("ASSERT_ADR0011_MEMBER_ORDER: case=%s item=%+v", tc.name, item)
				}
				if item.Kind == LocationLink && (item.TargetRange == nil || item.OriginSelectionRange == nil) {
					t.Fatalf("ASSERT_ADR0011_MEMBER_ORDER: link fields lost: %+v", item)
				}
			}
			if tc.count == 2 && (parsed.Items[0].Range != parsed.Items[1].Range || parsed.Items[0].Ordinal == parsed.Items[1].Ordinal) {
				t.Fatalf("ASSERT_ADR0011_MEMBER_MULTIPLICITY: case=%s items=%+v", tc.name, parsed.Items)
			}
			t.Log("ASSERT_ADR0011_WIRE_PARSE_SUCCESS: PASS")
			if tc.count == 2 {
				t.Log("ASSERT_ADR0011_MEMBER_MULTIPLICITY: PASS")
			}
		})
	}
}

// Equal returned locations cannot identify the symbol named by a references
// query. This is an in-process fake-runtime counterexample, not a target receipt.
func TestADR0011ReferencesResultCannotIdentifyDistinctQueries(t *testing.T) {
	raw := `[` + loc + `,` + loc + `]`
	first := fixtureRequest(transport.MethodReferences, false)
	second := fixtureRequest(transport.MethodReferences, false)
	second.Params = bytes.Replace(second.Params, []byte(`"uri":"file:///w/q.go"`), []byte(`"uri":"file:///w/r.go"`), 1)
	second.Params = bytes.Replace(second.Params, []byte(`"character":1`), []byte(`"character":2`), 1)
	if bytes.Equal(first.Params, second.Params) {
		t.Fatal("ASSERT_ADR0011_REFERENCES_RESULT_TARGET_NONIDENTIFIABILITY: queries not distinct")
	}

	left := &fixtureRuntime{metadata: sessionruntime.SessionMetadata{ReferencesSupport: true}, response: fixtureResponse(raw)}
	right := &fixtureRuntime{metadata: sessionruntime.SessionMetadata{ReferencesSupport: true}, response: fixtureResponse(raw)}
	leftResult := transport.New(left).Execute(context.Background(), first)
	rightResult := transport.New(right).Execute(context.Background(), second)
	leftItems, leftFailure := Parse(leftResult, 3)
	rightItems, rightFailure := Parse(rightResult, 3)
	leftObservation, leftPresent := leftResult.Observation()
	rightObservation, rightPresent := rightResult.Observation()
	if leftResult.Outcome() != transport.OutcomeTransportSuccess || rightResult.Outcome() != transport.OutcomeTransportSuccess ||
		leftFailure != nil || rightFailure != nil || len(leftItems.Items) != 2 || len(rightItems.Items) != 2 ||
		!bytes.Equal(leftResult.Raw(), rightResult.Raw()) || !reflect.DeepEqual(leftItems, rightItems) ||
		!leftPresent || !rightPresent || left.wireCalls != 1 || right.wireCalls != 1 ||
		!bytes.Equal(left.wire.Params, first.Params) || !bytes.Equal(right.wire.Params, second.Params) ||
		leftObservation.DeclaredQueryURI != "file:///w/q.go" || rightObservation.DeclaredQueryURI != "file:///w/r.go" ||
		leftObservation.DeclaredCharacter != 1 || rightObservation.DeclaredCharacter != 2 ||
		leftObservation.ParamsSHA256 == rightObservation.ParamsSHA256 ||
		leftItems.Items[0].Ordinal != 0 || leftItems.Items[1].Ordinal != 1 {
		t.Fatalf("ASSERT_ADR0011_REFERENCES_RESULT_TARGET_NONIDENTIFIABILITY: outcomes=%s/%s failures=%v/%v items=%d/%d declared=%s:%d/%s:%d", leftResult.Outcome(), rightResult.Outcome(), leftFailure, rightFailure, len(leftItems.Items), len(rightItems.Items), leftObservation.DeclaredQueryURI, leftObservation.DeclaredCharacter, rightObservation.DeclaredQueryURI, rightObservation.DeclaredCharacter)
	}
	t.Log("ASSERT_ADR0011_REFERENCES_RESULT_TARGET_NONIDENTIFIABILITY: PASS (same returned locations, distinct declared queries; no target identity)")
}

func TestADR0011InProcessMalformedAndResourceFixturesEmitNoItems(t *testing.T) {
	badSecond := `[` + loc + `,{"uri":"file:///w/b.go","uri":"file:///w/c.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}]`
	for _, tc := range []struct {
		name, method, raw string
		limit             int
		code              Code
		ordinal           int
	}{
		{"D-07", transport.MethodDefinition, badSecond, 3, FailureMalformed, 1},
		{"R-07", transport.MethodReferences, badSecond, 3, FailureMalformed, 1},
		{"D-09-candidates", transport.MethodDefinition, `[` + loc + `,` + loc + `]`, 1, FailureResource, 1},
		{"R-09-candidates", transport.MethodReferences, `[` + loc + `,` + loc + `]`, 1, FailureResource, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fixtureRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}, response: fixtureResponse(tc.raw)}
			wire := transport.New(f).Execute(context.Background(), fixtureRequest(tc.method, true))
			parsed, failed := Parse(wire, tc.limit)
			obs, present := wire.Observation()
			if wire.Outcome() != transport.OutcomeTransportSuccess || !present || f.wireCalls != 1 ||
				obs.RawResultDisposition != transport.RawResultRetained || failed == nil || failed.Code != tc.code || failed.Ordinal != tc.ordinal ||
				len(parsed.Items) != 0 || parsed.Null || string(wire.Raw()) != tc.raw {
				t.Fatalf("ASSERT_ADR0011_PARSE_NO_PARTIAL: case=%s outcome=%s failure=%+v obs=%+v parsed=%+v", tc.name, wire.Outcome(), failed, obs, parsed)
			}
			t.Log("ASSERT_ADR0011_PARSE_NO_PARTIAL: PASS")
		})
	}
}
