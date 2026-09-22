package censuscontinuation

import (
	"bytes"
	"strings"
	"testing"

	"lsp-trace/internal/describeworker"
)

func TestWorkerHistoryV2CanonicalEmptyArrays(t *testing.T) {
	raw, err := encodeWorkerHistoryV2(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schema_version":"lsp-trace.describe-records.v2","invocations":[],"responses":[]}`
	if string(raw) != want {
		t.Fatalf("ASSERT_WORKER_V2_HISTORY_CANONICAL_EMPTY: got %s", raw)
	}
	invocations, responses, err := decodeWorkerHistoryV2(raw)
	if err != nil || invocations == nil || responses == nil || len(invocations) != 0 || len(responses) != 0 {
		t.Fatalf("ASSERT_WORKER_V2_HISTORY_EMPTY_ROUND_TRIP: invocations=%v responses=%v err=%v", invocations, responses, err)
	}
}

func TestWorkerHistoryV2FailureCustodyRoundTripAndTamper(t *testing.T) {
	digest := "sha256:" + strings.Repeat("b", 64)
	binding := describeworker.IdentityBinding{RequestRecordID: "request", MessageID: "message", AttemptID: "attempt", WorkerSHA256: digest, ModelSHA256: digest, LibrarySHA256: digest, SandboxExecutableSHA256: digest, SandboxProfileSHA256: digest, RuntimeIdentity: "runtime", AdapterIdentity: "adapter", ModelIdentity: "model", PromptSHA256: digest, GrammarSHA256: digest, Limits: describeworker.Limits{TimeoutMS: 1, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 16, StderrBytes: 16, WorkBytes: 1, TempBytes: 1}}
	stdout, stderr := []byte("invalid output"), []byte("diagnostic")
	invocation, err := describeworker.NewFailedInvocationRecordV2(binding, describeworker.StatusOutputInvalid, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true, StdoutBytes: len(stdout), StderrBytes: len(stderr), StdoutTruncated: true}, stdout, stderr, "SEMANTIC_VALUE_INVALID")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeWorkerHistoryV2([]describeworker.InvocationRecordV2{invocation}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, responses, err := decodeWorkerHistoryV2(raw)
	if err != nil || len(got) != 1 || len(responses) != 0 || got[0].FailureSubcode() != "SEMANTIC_VALUE_INVALID" || got[0].StdoutSHA256() != invocation.StdoutSHA256() || got[0].StderrSHA256() != invocation.StderrSHA256() || !got[0].Receipt().StdoutTruncated {
		t.Fatalf("ASSERT_WORKER_V2_HISTORY_FAILURE_CUSTODY: got=%+v err=%v", got, err)
	}
	tampered := bytes.Replace(raw, []byte(invocation.StdoutSHA256()), []byte("sha256:"+strings.Repeat("a", 64)), 1)
	if _, _, err := decodeWorkerHistoryV2(tampered); err == nil {
		t.Fatal("ASSERT_WORKER_V2_HISTORY_FAILURE_TAMPER_REJECTED")
	}
}

func TestWorkerHistoryV2BindsInvocationAndResponse(t *testing.T) {
	digest := "sha256:" + strings.Repeat("b", 64)
	host := describeworker.ResponseHostV2{RequestRecordID: "request", MessageID: "message", AttemptID: "attempt", Consumer: describeworker.ConsumerIdentityV2{Resolution: describeworker.ConsumerUnresolvedV2, AlternativeID: "OUTWARD_CONSUMER_UNRESOLVED"}, Pins: describeworker.HostPinsV2{WorkerSHA256: digest, ModelSHA256: digest, GrammarSHA256: digest, PromptSHA256: digest}, Provenance: describeworker.HostProvenanceV2{PacketID: "packet", RequestLineageIdentity: "lineage"}, Custody: describeworker.HostCustodyV2{GraphDigest: digest, CaptureID: "capture"}}
	response, err := describeworker.NewResponseRecordV2([]byte(`{"verdict":"COMPLETE","target_role":"target","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"behavior","consumer_relative":false},"boundary_contribution":"boundary","limitations":[]}`), host)
	if err != nil {
		t.Fatal(err)
	}
	binding := describeworker.IdentityBinding{RequestRecordID: host.RequestRecordID, MessageID: host.MessageID, AttemptID: host.AttemptID, WorkerSHA256: digest, ModelSHA256: digest, LibrarySHA256: digest, SandboxExecutableSHA256: digest, SandboxProfileSHA256: digest, RuntimeIdentity: "runtime", AdapterIdentity: "adapter", ModelIdentity: "model", PromptSHA256: digest, GrammarSHA256: digest, Limits: describeworker.Limits{TimeoutMS: 1, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 1, StderrBytes: 1, WorkBytes: 1, TempBytes: 1}}
	invocation, err := describeworker.NewInvocationRecordV2(binding, describeworker.StatusSucceeded, describeworker.ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true, StdoutBytes: response.RawStdoutLength()}, response)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeWorkerHistoryV2([]describeworker.InvocationRecordV2{invocation}, []describeworker.ResponseRecordV2{response})
	if err != nil {
		t.Fatal(err)
	}
	invocations, responses, err := decodeWorkerHistoryV2(raw)
	if err != nil || len(invocations) != 1 || len(responses) != 1 || invocations[0].Response().ID() != responses[0].ID() {
		t.Fatalf("ASSERT_WORKER_V2_HISTORY_BINDING: %v", err)
	}
	if _, _, err := decodeWorkerHistoryV2(bytes.Replace(raw, []byte(response.ID()), []byte("sha256:"+strings.Repeat("a", 64)), 1)); err == nil {
		t.Fatal("ASSERT_WORKER_V2_HISTORY_TAMPER_REJECTED")
	}
	v1 := bytes.Replace(raw, []byte(describeworker.InvocationSchemaV2), []byte(describeworker.InvocationSchema), 1)
	if _, _, err := decodeWorkerHistoryV2(v1); err == nil {
		t.Fatal("ASSERT_WORKER_V2_HISTORY_CROSS_VERSION_REJECTED")
	}
}
