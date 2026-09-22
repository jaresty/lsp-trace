package describeworker

import (
	"bytes"
	"testing"
)

func validSemantic() SemanticResponse {
	return SemanticResponse{
		Verdict:                "SUPPORTED",
		TargetRole:             "target role",
		NearestOutwardConsumer: "consumer",
		ConsumerNeed:           "need",
		ProvidedBehavior:       "behavior",
		BoundaryContribution:   "boundary",
		Limitations:            []string{"bounded"},
		Citations:              []string{"C1"},
	}
}

func validBinding() IdentityBinding {
	return IdentityBinding{
		RequestRecordID: "request", MessageID: "message", AttemptID: "attempt-a",
		WorkerSHA256: digestA, ModelSHA256: digestB, LibrarySHA256: digestC,
		SandboxExecutableSHA256: digestD, SandboxProfileSHA256: digestE,
		RuntimeIdentity: "runtime-v1", AdapterIdentity: "adapter-v1", ModelIdentity: "model-v1",
		PromptSHA256: digestF, GrammarSHA256: digestA,
		Limits: Limits{TimeoutMS: 1000, MaxTokens: 64, ContextTokens: 1024, StdoutBytes: 4096, StderrBytes: 1024, WorkBytes: 8192, TempBytes: 8192},
	}
}

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	digestC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	digestD = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	digestE = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	digestF = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
)

func TestResponseAndInvocationCanonicalIdentity(t *testing.T) {
	binding := validBinding()
	response, err := NewResponseRecord(binding, validSemantic())
	if err != nil {
		t.Fatal(err)
	}
	invocation, err := NewInvocationRecord(binding, StatusSucceeded, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true, StdoutBytes: 10, StderrBytes: 3}, response)
	if err != nil {
		t.Fatal(err)
	}
	if response.Authority() != 0 || response.Accepted() || response.Completeness() != "UNKNOWN" {
		t.Fatal("ASSERT_FIXED_CLAIM_CEILING")
	}
	raw, err := response.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseResponseRecord(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID() != response.ID() {
		t.Fatal("ASSERT_RESPONSE_ID_REPLAY")
	}
	iraw, err := invocation.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	iparsed, err := ParseInvocationRecord(iraw)
	if err != nil {
		t.Fatal(err)
	}
	if iparsed.ID() != invocation.ID() {
		t.Fatal("ASSERT_INVOCATION_ID_REPLAY")
	}

	for name, bad := range map[string][]byte{
		"trailing":  append(append([]byte(nil), raw...), []byte("{}")...),
		"unknown":   bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"unknown":0,"schema_version":`), 1),
		"duplicate": bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"schema_version":"x","schema_version":`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseResponseRecord(bad); err == nil {
				t.Fatal("ASSERT_STRICT_RECORD_REJECTION")
			}
		})
	}
}

func TestFailedInvocationRequiresNoSemanticResponse(t *testing.T) {
	binding := validBinding()
	invocation, err := NewInvocationRecord(binding, StatusTimeout, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, ResponseRecord{})
	if err != nil {
		t.Fatalf("ASSERT_RESPONSE_FREE_FAILURE: %v", err)
	}
	if invocation.Response().ID() != "" || invocation.Binding().RequestRecordID != binding.RequestRecordID {
		t.Fatal("ASSERT_FAILURE_IDENTITY_BINDING")
	}
	raw, err := invocation.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseInvocationRecord(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID() != invocation.ID() || parsed.Response().ID() != "" {
		t.Fatal("ASSERT_FAILURE_IDENTITY_REPLAY")
	}
}

func TestRetryIdentityAndDefensiveCloning(t *testing.T) {
	binding := validBinding()
	semantic := validSemantic()
	r1, err := NewResponseRecord(binding, semantic)
	if err != nil {
		t.Fatal(err)
	}
	semantic.Limitations[0] = "mutated"
	semantic.Citations[0] = "mutated"
	raw1, _ := r1.Bytes()
	if bytes.Contains(raw1, []byte("mutated")) {
		t.Fatal("ASSERT_ALIAS_MUTATION_ISOLATED")
	}

	i1, err := NewInvocationRecord(binding, StatusSucceeded, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, r1)
	if err != nil {
		t.Fatal(err)
	}
	binding.AttemptID = "attempt-b"
	r2, err := NewResponseRecord(binding, validSemantic())
	if err != nil {
		t.Fatal(err)
	}
	i2, err := NewInvocationRecord(binding, StatusSucceeded, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, r2)
	if err != nil {
		t.Fatal(err)
	}
	if r1.ID() != r2.ID() {
		t.Fatal("ASSERT_SEMANTIC_ID_RETRY_STABLE")
	}
	if i1.ID() == i2.ID() {
		t.Fatal("ASSERT_INVOCATION_ID_RETRY_DISTINCT")
	}
}
