package describeworker

import (
	"bytes"
	"testing"
)

func TestInvocationRecordV2CanonicalVersionBinding(t *testing.T) {
	host := hostV2()
	response, err := NewResponseRecordV2([]byte(validSemanticV2()), host)
	if err != nil {
		t.Fatal(err)
	}
	binding := bindingForHostV2(host)
	invocation, err := NewInvocationRecordV2(binding, StatusSucceeded, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true, StdoutBytes: response.RawStdoutLength()}, response)
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Response().ID() != response.ID() || invocation.Status() != StatusSucceeded || invocation.Binding().AttemptID != binding.AttemptID || invocation.Receipt().StdoutBytes != response.RawStdoutLength() {
		t.Fatal("ASSERT_INVOCATION_V2_RESPONSE_AND_ACCOUNTING_BINDING")
	}
	raw, err := invocation.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseInvocationRecordV2(raw)
	if err != nil || parsed.ID() != invocation.ID() || parsed.Response().RawStdoutDigest() != response.RawStdoutDigest() {
		t.Fatalf("ASSERT_INVOCATION_V2_EXACT_REPLAY: %v", err)
	}
	for name, bad := range map[string][]byte{
		"trailing":        append(append([]byte(nil), raw...), []byte("{}")...),
		"unknown":         bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"unknown":0,"schema_version":`), 1),
		"duplicate":       bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"schema_version":"x","schema_version":`), 1),
		"v1-substitution": bytes.Replace(raw, []byte(InvocationSchemaV2), []byte(InvocationSchema), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseInvocationRecordV2(bad); err == nil {
				t.Fatal("ASSERT_INVOCATION_V2_STRICT_REJECTION")
			}
		})
	}
	v1, err := NewResponseRecord(binding, validSemantic())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewInvocationRecordV2(binding, StatusSucceeded, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, ResponseRecordV2{}); err == nil {
		t.Fatal("ASSERT_INVOCATION_V2_REJECTS_MISSING_V2_RESPONSE")
	}
	_ = v1 // V1 has no conversion path into the V2 constructor by type.
}

func TestInvocationRecordV2TerminalFailureHasNoResponse(t *testing.T) {
	binding := validBinding()
	invocation, err := NewInvocationRecordV2(binding, StatusTimeout, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, ResponseRecordV2{})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Response().ID() != "" {
		t.Fatal("ASSERT_INVOCATION_V2_FAILURE_RESPONSE_ABSENT")
	}
	raw, _ := invocation.Bytes()
	parsed, err := ParseInvocationRecordV2(raw)
	if err != nil || parsed.Response().ID() != "" {
		t.Fatalf("ASSERT_INVOCATION_V2_FAILURE_REPLAY: %v", err)
	}
}

func TestInvocationRecordV2LeavesV1BytesStable(t *testing.T) {
	binding := validBinding()
	response, err := NewResponseRecord(binding, validSemantic())
	if err != nil {
		t.Fatal(err)
	}
	before, err := NewInvocationRecord(binding, StatusSucceeded, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, response)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := before.Bytes()

	host := hostV2()
	v2Response, err := NewResponseRecordV2([]byte(validSemanticV2()), host)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewInvocationRecordV2(bindingForHostV2(host), StatusSucceeded, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true, StdoutBytes: v2Response.RawStdoutLength()}, v2Response); err != nil {
		t.Fatal(err)
	}
	after, err := NewInvocationRecord(binding, StatusSucceeded, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, response)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := after.Bytes()
	if !bytes.Equal(got, want) {
		t.Fatalf("ASSERT_V1_INVOCATION_BYTES_STABLE\ngot=%s\nwant=%s", got, want)
	}
}

func bindingForHostV2(host ResponseHostV2) IdentityBinding {
	return IdentityBinding{
		RequestRecordID: host.RequestRecordID, MessageID: host.MessageID, AttemptID: host.AttemptID,
		WorkerSHA256: host.Pins.WorkerSHA256, ModelSHA256: host.Pins.ModelSHA256,
		LibrarySHA256: digestC, SandboxExecutableSHA256: digestD, SandboxProfileSHA256: digestE,
		RuntimeIdentity: "runtime-v1", AdapterIdentity: "adapter-v1", ModelIdentity: "model-v1",
		PromptSHA256: host.Pins.PromptSHA256, GrammarSHA256: host.Pins.GrammarSHA256,
		Limits: validBinding().Limits,
	}
}
