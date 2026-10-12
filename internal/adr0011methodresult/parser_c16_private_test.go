package adr0011methodresult

import (
	"errors"
	"testing"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/adr0011work"
)

var errC16Admission = errors.New("c16 object admission refused")

type c16RecordingAdmitter struct {
	calls       int
	callbacks   int
	failAt      int
	callbackErr error
}

func (a *c16RecordingAdmitter) WithObjectAdmission(fn func() error) error {
	a.calls++
	if a.failAt != 0 && a.calls == a.failAt {
		return errC16Admission
	}
	a.callbacks++
	if err := fn(); err != nil {
		return err
	}
	return a.callbackErr
}

func TestPrivateC16ParserMalformedNeverAdmitted(t *testing.T) {
	a := &c16RecordingAdmitter{}
	result, failure := parseRawUntrustedWithAdmission(transport.MethodDefinition,
		[]byte(`[{"uri":"file:///ok.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}},{"uri":"file:///bad.go"}]`), 1000, a)
	if failure == nil || failure.Code != FailureMalformed || failure.Ordinal != 1 || len(result.Items) != 0 {
		t.Fatalf("ASSERT_C16_MALFORMED_RESULT failure=%+v items=%d", failure, len(result.Items))
	}
	if a.calls != 0 || a.callbacks != 0 {
		t.Fatalf("ASSERT_C16_MALFORMED_NEVER_ADMITTED calls=%d callbacks=%d", a.calls, a.callbacks)
	}
}

func TestPrivateC16ParserValidCandidatesAdmittedByOrdinal(t *testing.T) {
	a := &c16RecordingAdmitter{}
	raw := []byte(`[{"uri":"file:///same.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}},{"uri":"file:///same.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`)
	result, failure := parseRawUntrustedWithAdmission(transport.MethodDefinition, raw, 1000, a)
	if failure != nil || len(result.Items) != 2 {
		t.Fatalf("ASSERT_C16_VALID_RESULT failure=%+v items=%d", failure, len(result.Items))
	}
	if a.calls != 2 || a.callbacks != 2 || result.Items[0].Ordinal != 0 || result.Items[1].Ordinal != 1 {
		t.Fatalf("ASSERT_C16_DISTINCT_ORDINAL_ADMISSION calls=%d callbacks=%d items=%+v", a.calls, a.callbacks, result.Items)
	}
}

func TestPrivateC16ParserRefusalPrecedesCallbackAndReturnsNoItems(t *testing.T) {
	a := &c16RecordingAdmitter{failAt: 2}
	raw := []byte(`[{"uri":"file:///one.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}},{"uri":"file:///two.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`)
	result, failure := parseRawUntrustedWithAdmission(transport.MethodDefinition, raw, 1000, a)
	if failure == nil || failure.Code != FailureResource || failure.Ordinal != 1 || len(result.Items) != 0 {
		t.Fatalf("ASSERT_C16_REFUSAL_ATOMIC failure=%+v items=%d", failure, len(result.Items))
	}
	if a.calls != 2 || a.callbacks != 1 {
		t.Fatalf("ASSERT_C16_REFUSAL_BEFORE_CALLBACK calls=%d callbacks=%d", a.calls, a.callbacks)
	}
}

func TestPrivateC16ParserCallbackFailureReturnsNoItems(t *testing.T) {
	a := &c16RecordingAdmitter{callbackErr: errC16Admission}
	raw := []byte(`{"uri":"file:///one.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`)
	result, failure := parseRawUntrustedWithAdmission(transport.MethodDefinition, raw, 1000, a)
	if failure == nil || failure.Code != FailureResource || failure.Ordinal != 0 || len(result.Items) != 0 {
		t.Fatalf("ASSERT_C16_CALLBACK_ROLLBACK failure=%+v items=%d", failure, len(result.Items))
	}
	if a.calls != 1 || a.callbacks != 1 {
		t.Fatalf("ASSERT_C16_CALLBACK_EXECUTED calls=%d callbacks=%d", a.calls, a.callbacks)
	}
}

type objectsMaterializedAdmitter struct {
	account  *adr0011work.Account
	identity string
}

func (a objectsMaterializedAdmitter) WithObjectAdmission(materialize func() error) error {
	return a.account.WithObjectMaterialization(a.identity, materialize)
}

func TestObjectsMaterializedParserCountsConstructedItemsByOrdinalWithoutC16(t *testing.T) {
	account := adr0011work.New("parse-a")
	admitter := objectsMaterializedAdmitter{account: account, identity: "parse-a"}
	raw := []byte(`[{"uri":"file:///same.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}},{"uri":"file:///same.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`)
	result, failure := parseRawUntrustedWithAdmission(transport.MethodDefinition, raw, 1000, admitter)
	if failure != nil || len(result.Items) != 2 || result.Items[0].Ordinal != 0 || result.Items[1].Ordinal != 1 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_TWO_DISTINCT_ORDINALS failure=%+v items=%+v", failure, result.Items)
	}
	if got := account.Snapshot().ObjectsMaterialized; got != 2 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_EQUAL_CANDIDATES_COUNT_TWO got=%d", got)
	}
}

func TestObjectsMaterializedParserInvalidInputsCountZero(t *testing.T) {
	for name, raw := range map[string][]byte{
		"null":      []byte(`null`),
		"empty":     []byte(`[]`),
		"malformed": []byte(`[{"uri":"file:///bad.go"}]`),
	} {
		t.Run(name, func(t *testing.T) {
			account := adr0011work.New("parse-" + name)
			admitter := objectsMaterializedAdmitter{account: account, identity: "parse-" + name}
			_, _ = parseRawUntrustedWithAdmission(transport.MethodDefinition, raw, 1000, admitter)
			if got := account.Snapshot().ObjectsMaterialized; got != 0 {
				t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_INVALID_ZERO case=%s got=%d", name, got)
			}
		})
	}
}

func TestObjectsMaterializedParserLaterFailureDoesNotRefundAndReplayCountsAgain(t *testing.T) {
	account := adr0011work.New("parse-replay")
	admitter := objectsMaterializedAdmitter{account: account, identity: "parse-replay"}
	one := []byte(`{"uri":"file:///one.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`)
	if result, failure := parseRawUntrustedWithAdmission(transport.MethodDefinition, one, 1000, admitter); failure != nil || len(result.Items) != 1 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_FIRST_PARSE failure=%+v items=%d", failure, len(result.Items))
	}
	// A later consumer failure cannot erase the already constructed parser Item.
	laterFailure := errors.New("later publication failure")
	if laterFailure == nil || account.Snapshot().ObjectsMaterialized != 1 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_LATER_FAILURE_NO_REFUND snapshot=%+v", account.Snapshot())
	}
	if result, failure := parseRawUntrustedWithAdmission(transport.MethodDefinition, one, 1000, admitter); failure != nil || len(result.Items) != 1 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_REPLAY_PARSE failure=%+v items=%d", failure, len(result.Items))
	}
	if got := account.Snapshot().ObjectsMaterialized; got != 2 {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_REPLAY_COUNTS_AGAIN got=%d", got)
	}
}

func TestObjectsMaterializedParserDefaultOffParity(t *testing.T) {
	raw := []byte(`{"uri":"file:///one.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`)
	legacy, legacyFailure := parseRawUntrustedWithAdmission(transport.MethodDefinition, raw, 1000, nil)
	account := adr0011work.New("parse-parity")
	counted, countedFailure := parseRawUntrustedWithAdmission(transport.MethodDefinition, raw, 1000, objectsMaterializedAdmitter{account: account, identity: "parse-parity"})
	if legacyFailure != nil || countedFailure != nil || len(legacy.Items) != 1 || len(counted.Items) != 1 || legacy.Items[0] != counted.Items[0] {
		t.Fatalf("ASSERT_OBJECTS_MATERIALIZED_DEFAULT_OFF_PARITY legacy=%+v/%+v counted=%+v/%+v", legacy, legacyFailure, counted, countedFailure)
	}
}
