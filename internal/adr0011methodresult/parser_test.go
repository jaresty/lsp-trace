package adr0011methodresult

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type parserRuntime struct {
	result sessionruntime.RoundTripResult
}

func (r parserRuntime) Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}, ""
}
func (r parserRuntime) RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	return r.result
}

func methodResult(t *testing.T, method, raw string) transport.Result {
	t.Helper()
	params := `{"textDocument":{"uri":"file:///w/q.go"},"position":{"line":0,"character":1}`
	if method == transport.MethodReferences {
		params += `,"context":{"includeDeclaration":true}`
	}
	params += `}`
	got := transport.New(parserRuntime{result: sessionruntime.RoundTripResult{
		Result: json.RawMessage(raw), Messages: 1, Bytes: int64(len(raw) + 100),
	}}).Execute(context.Background(), transport.Request{
		SessionID: "exact", Generation: 1, Method: method, Params: json.RawMessage(params),
		Deadline: time.Now().Add(time.Second), MaxMessages: 5, MaxBytes: 8192,
	})
	if got.Outcome() != transport.OutcomeTransportSuccess {
		t.Fatalf("test setup: transport outcome=%s: %s", got.Outcome(), got.FailureText())
	}
	return got
}

const loc = `{"uri":"file:///w/a.go","range":{"start":{"line":1,"character":2},"end":{"line":1,"character":5}}}`
const link = `{"targetUri":"file:///w/a.go","targetRange":{"start":{"line":1,"character":0},"end":{"line":5,"character":0}},"targetSelectionRange":{"start":{"line":2,"character":1},"end":{"line":2,"character":4}},"originSelectionRange":{"start":{"line":0,"character":1},"end":{"line":0,"character":2}}}`

func TestDefinitionNullIsZeroReturnedTargets(t *testing.T) {
	got, failure := Parse(methodResult(t, transport.MethodDefinition, `null`), 1)
	if failure != nil || !got.Null || len(got.Items) != 0 {
		t.Fatalf("ASSERT_DEFINITION_NULL_EMPTY: result=%+v failure=%v", got, failure)
	}
	t.Log("ASSERT_DEFINITION_NULL_EMPTY: PASS")
}

func TestDefinitionShapesRetainOrdinalsAndRanges(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		count     int
		kind      Kind
	}{
		{"empty array", `[]`, 0, ""},
		{"scalar location", loc, 1, Location},
		{"duplicate locations", `[` + loc + `,` + loc + `]`, 2, Location},
		{"links", `[` + link + `,` + link + `]`, 2, LocationLink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, failure := Parse(methodResult(t, transport.MethodDefinition, tc.raw), 3)
			if failure != nil || got.Null || len(got.Items) != tc.count {
				t.Fatalf("ASSERT_DEFINITION_SHAPE: result=%+v failure=%v", got, failure)
			}
			for i, item := range got.Items {
				if item.Ordinal != i || item.Kind != tc.kind || item.URI != "file:///w/a.go" {
					t.Fatalf("ASSERT_DEFINITION_ITEM_FIDELITY: %+v", item)
				}
				if item.Kind == Location && (item.Range.Start != (Position{1, 2}) || item.TargetRange != nil || item.OriginSelectionRange != nil) {
					t.Fatalf("ASSERT_LOCATION_RANGE: %+v", item)
				}
				if item.Kind == LocationLink && (item.Range.Start != (Position{2, 1}) || item.TargetRange == nil || item.TargetRange.Start != (Position{1, 0}) || item.OriginSelectionRange == nil || item.OriginSelectionRange.Start != (Position{0, 1})) {
					t.Fatalf("ASSERT_LINK_DISTINCT_RANGES: %+v", item)
				}
			}
		})
	}
}

func TestReferencesNullAndExactRepeatedLocations(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		null  bool
		count int
	}{{`null`, true, 0}, {`[]`, false, 0}, {`[` + loc + `,` + loc + `]`, false, 2}} {
		got, failure := Parse(methodResult(t, transport.MethodReferences, tc.raw), 2)
		if failure != nil || got.Null != tc.null || len(got.Items) != tc.count {
			t.Fatalf("ASSERT_REFERENCES_SHAPE: raw=%s result=%+v failure=%v", tc.raw, got, failure)
		}
		for i, item := range got.Items {
			if item.Ordinal != i || item.Kind != Location || item.Range.End != (Position{1, 5}) {
				t.Fatalf("ASSERT_REFERENCE_OCCURRENCE_NOT_DEDUPED: %+v", item)
			}
		}
	}
}

func TestMalformedAndCandidateLimitFailWithoutItems(t *testing.T) {
	for _, tc := range []struct {
		name, method, raw string
		limit             int
		want              Code
	}{
		{"invalid json", transport.MethodDefinition, `[`, 2, FailureMalformed},
		{"trailing value", transport.MethodDefinition, `null null`, 2, FailureMalformed},
		{"trailing array value", transport.MethodDefinition, `[] null`, 2, FailureMalformed},
		{"duplicate nested key", transport.MethodDefinition, `{"uri":"file:///w/a.go","range":{"start":{"line":1,"line":2,"character":0},"end":{"line":1,"character":1}}}`, 2, FailureMalformed},
		{"escaped duplicate", transport.MethodDefinition, `{"uri":"file:///w/a.go","u\u0072i":"file:///w/b.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}`, 2, FailureMalformed},
		{"mixed definition array", transport.MethodDefinition, `[` + loc + `,` + link + `]`, 2, FailureMalformed},
		{"scalar link", transport.MethodDefinition, link, 2, FailureMalformed},
		{"scalar reference", transport.MethodReferences, loc, 2, FailureMalformed},
		{"reference link", transport.MethodReferences, `[` + link + `]`, 2, FailureMalformed},
		{"missing required URI", transport.MethodDefinition, `{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`, 2, FailureMalformed},
		{"null required range", transport.MethodDefinition, `{"uri":"x","range":null}`, 2, FailureMalformed},
		{"negative coordinate", transport.MethodDefinition, `{"uri":"x","range":{"start":{"line":-1,"character":0},"end":{"line":1,"character":0}}}`, 2, FailureMalformed},
		{"fractional coordinate", transport.MethodDefinition, `{"uri":"x","range":{"start":{"line":0.5,"character":0},"end":{"line":1,"character":0}}}`, 2, FailureMalformed},
		{"coordinate overflow", transport.MethodDefinition, `{"uri":"x","range":{"start":{"line":2147483648,"character":0},"end":{"line":2147483648,"character":1}}}`, 2, FailureMalformed},
		{"reversed range", transport.MethodDefinition, `{"uri":"x","range":{"start":{"line":1,"character":2},"end":{"line":1,"character":1}}}`, 2, FailureMalformed},
		{"link selection outside target", transport.MethodDefinition, `[{"targetUri":"x","targetRange":{"start":{"line":1,"character":0},"end":{"line":1,"character":2}},"targetSelectionRange":{"start":{"line":1,"character":3},"end":{"line":1,"character":4}}}]`, 2, FailureMalformed},
		{"candidate exhaustion", transport.MethodDefinition, `[` + loc + `,` + loc + `]`, 1, FailureResource},
		{"zero limit", transport.MethodDefinition, `null`, 0, FailureLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, failure := Parse(methodResult(t, tc.method, tc.raw), tc.limit)
			if failure == nil || failure.Code != tc.want || len(got.Items) != 0 || got.Null {
				t.Fatalf("ASSERT_TYPED_PARSE_FAILURE: result=%+v failure=%v want=%s", got, failure, tc.want)
			}
		})
	}
}

func TestMalformedDuplicateReportsMemberOrdinal(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      int
	}{
		{"second member required field", `[` + loc + `,{"uri":"x","uri":"y","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}}]`, 1},
		{"second member nested extension", `[` + loc + `,{"uri":"x","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"extension":{"x":1,"x":2}}]`, 1},
		{"scalar member", `{"uri":"x","uri":"y","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}}`, 0},
		{"whole result trailing value", `null null`, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, failure := Parse(methodResult(t, transport.MethodDefinition, tc.raw), 2)
			if failure == nil || failure.Code != FailureMalformed || failure.Ordinal != tc.want || len(got.Items) != 0 || got.Null {
				ordinal := -2
				if failure != nil {
					ordinal = failure.Ordinal
				}
				t.Fatalf("ASSERT_DUPLICATE_MEMBER_ORDINAL: result=%+v failure=%+v gotOrdinal=%d wantOrdinal=%d", got, failure, ordinal, tc.want)
			}
		})
	}
}

func TestCandidateExhaustionReportsNextOrdinal(t *testing.T) {
	got, failure := Parse(methodResult(t, transport.MethodDefinition, `[`+loc+`,`+loc+`]`), 1)
	if failure == nil || failure.Code != FailureResource || failure.Ordinal != 1 || len(got.Items) != 0 {
		t.Fatalf("ASSERT_CANDIDATE_EXHAUSTION_ORDINAL: result=%+v failure=%+v", got, failure)
	}
}

func TestLaterMalformedMemberReturnsNoEarlierItems(t *testing.T) {
	bad := `[` + loc + `,{"uri":"file:///w/b.go","range":{"start":{"line":1,"character":2},"end":{"line":1,"character":1}}}]`
	got, failure := Parse(methodResult(t, transport.MethodDefinition, bad), 2)
	if failure == nil || failure.Code != FailureMalformed || failure.Ordinal != 1 || len(got.Items) != 0 {
		t.Fatalf("ASSERT_NO_PARTIAL_PARSE_ITEMS: result=%+v failure=%+v", got, failure)
	}
}

func TestUnknownExtensionCannotHideDuplicateKey(t *testing.T) {
	bad := `{"uri":"file:///w/a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"extension":{"x":1,"x":2}}`
	got, failure := Parse(methodResult(t, transport.MethodDefinition, bad), 1)
	if failure == nil || failure.Code != FailureMalformed || len(got.Items) != 0 {
		t.Fatalf("ASSERT_NESTED_EXTENSION_DUPLICATE_REJECTED: result=%+v failure=%+v", got, failure)
	}

	valid := `{"uri":"file:///w/a.go","range":{"start":{"line":2147483647,"character":0},"end":{"line":2147483647,"character":0}},"extension":{"x":1}}`
	got, failure = Parse(methodResult(t, transport.MethodDefinition, valid), 1)
	if failure != nil || len(got.Items) != 1 || got.Items[0].Range.Start != (Position{2147483647, 0}) || got.Items[0].Range.End != got.Items[0].Range.Start {
		t.Fatalf("ASSERT_EMPTY_MAXIMUM_RANGE_PRESERVED: result=%+v failure=%+v", got, failure)
	}
}

func TestTransportFailureIsNotAnEmptyMethodResult(t *testing.T) {
	got := transport.New(parserRuntime{result: sessionruntime.RoundTripResult{Failure: session.RequestTimeout}}).Execute(context.Background(), transport.Request{
		SessionID: "exact", Generation: 1, Method: transport.MethodDefinition,
		Params:   json.RawMessage(`{"textDocument":{"uri":"file:///w/q.go"},"position":{"line":0,"character":0}}`),
		Deadline: time.Now().Add(time.Second), MaxMessages: 5, MaxBytes: 8192,
	})
	parsed, failure := Parse(got, 1)
	if failure == nil || failure.Code != FailureTransport || len(parsed.Items) != 0 || parsed.Null {
		t.Fatalf("ASSERT_FAILED_TRANSPORT_NOT_EMPTY_RESULT: parsed=%+v failure=%v", parsed, failure)
	}
}
