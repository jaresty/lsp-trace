package adr0011methodresult

import (
	"errors"
	"testing"

	transport "lsp-trace/internal/adr0011methodtransport"
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
