package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

type continuationMCPHostFixture struct {
	freshCalls, resumeCalls int
	freshStop, resumeStop   string
	result                  censusContinuationMCPResult
	err                     error
}

func (f *continuationMCPHostFixture) Fresh(_ context.Context, _ censusCompletion, stop string) (censusContinuationMCPResult, error) {
	f.freshCalls++
	f.freshStop = stop
	return f.result, f.err
}

func (f *continuationMCPHostFixture) Resume(_ context.Context, _, stop string) (censusContinuationMCPResult, error) {
	f.resumeCalls++
	f.resumeStop = stop
	return f.result, f.err
}

func TestCensusContinuationFreshSelectsV2AndRunsCensusOnce(t *testing.T) {
	census := privateCensusSuccessFixture()
	fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(census)}}
	descriptor := "g-" + strings.Repeat("a", 64) + ".selector.json"
	host := &continuationMCPHostFixture{result: censusContinuationMCPResult{Census: &census, Descriptor: descriptor, Status: "DEGRADED"}}
	binding := newPrivateCensusMCPBinding(fixture, host)
	request := operation.Request{
		Name:      operation.Census,
		RequestID: "continuation-fresh",
		Input:     []byte(`{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG"}}`),
	}

	got, err := binding.callCanonical(context.Background(), request)
	if err != nil {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_FRESH_V2_DECODED: %v", err)
	}
	if len(fixture.calls) != 1 || host.freshCalls != 1 {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_FRESH_CORE_ONCE: census=%d continuation=%d", len(fixture.calls), host.freshCalls)
	}
	var envelope map[string]any
	if err := json.Unmarshal(got.Structured, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["envelope_schema_id"] != mcpcontract.FutureCensusCompositeSuccessID || envelope["operation_status"] != "SUCCEEDED" {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_FRESH_SUCCESSOR_ENVELOPE: %v", envelope)
	}
}

func TestCensusContinuationHostUnprovisionedAfterCommitIsTypedAndPrivate(t *testing.T) {
	census := privateCensusSuccessFixture()
	fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(census)}}
	binding := newPrivateCensusMCPBinding(fixture)
	request := operation.Request{Name: operation.Census, RequestID: "continuation-host-unprovisioned", Input: []byte(`{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","stop_after":"DESCRIBE_REQUESTS"}}`)}

	direct, directErr := binding.callDirect(context.Background(), request)
	canonical, canonicalErr := binding.callCanonical(context.Background(), request)
	if directErr != nil || canonicalErr != nil {
		t.Fatalf("ASSERT_CONTINUATION_HOST_UNPROVISIONED_TYPED_NOT_TRANSPORT_ERROR: direct=%v canonical=%v", directErr, canonicalErr)
	}
	if string(direct.Structured) != string(canonical.Structured) {
		t.Fatalf("ASSERT_CONTINUATION_HOST_UNPROVISIONED_DIRECT_GATEWAY_PARITY: direct=%s canonical=%s", direct.Structured, canonical.Structured)
	}
	if err := mcpcontract.ValidateFutureCensusEnvelopeExclusive(direct.Structured); err != nil {
		t.Fatalf("ASSERT_CONTINUATION_HOST_UNPROVISIONED_SCHEMA_VALID: %v envelope=%s", err, direct.Structured)
	}
	var envelope map[string]any
	if err := json.Unmarshal(direct.Structured, &envelope); err != nil {
		t.Fatal(err)
	}
	result, _ := envelope["result"].(map[string]any)
	if envelope["outcome"] != "COMMITTED_DEGRADED" || envelope["operation_status"] != "SUCCEEDED" || envelope["isError"] != false || result["stage"] != "committed-degradation" || result["code"] != "CONTINUATION_HOST_UNAVAILABLE" || result["retry"] != false {
		t.Fatalf("ASSERT_CONTINUATION_HOST_UNPROVISIONED_CLOSED_POST_COMMIT_DIAGNOSTIC: %v", envelope)
	}
	detail, _ := result["detail"].(string)
	for _, required := range []string{"continuation host capability", "bootstrap continuation configuration", "census commit is preserved", "provision", "reconnect", "resubmit", "do not retry on this connection"} {
		if !strings.Contains(strings.ToLower(detail), required) {
			t.Errorf("ASSERT_CONTINUATION_HOST_UNPROVISIONED_ACTIONABLE_DETAIL missing=%q detail=%q", required, detail)
		}
	}
	for _, forbidden := range []string{"/", "\\", "sha256:", "worker", "model", "pin"} {
		if strings.Contains(strings.ToLower(detail), forbidden) {
			t.Errorf("ASSERT_CONTINUATION_HOST_UNPROVISIONED_NO_PRIVATE_PATHS_OR_PINS forbidden=%q detail=%q", forbidden, detail)
		}
	}
	if len(fixture.calls) != 2 {
		t.Fatalf("ASSERT_CONTINUATION_HOST_UNPROVISIONED_CENSUS_COMMITTED_BEFORE_DIAGNOSTIC: calls=%d", len(fixture.calls))
	}
}

func TestCensusContinuationPostConstructionFailureIsOperationalAndPrivate(t *testing.T) {
	census := privateCensusSuccessFixture()
	fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(census)}}
	host := &continuationMCPHostFixture{err: errors.New("private host detail /private/config.json")}
	binding := newPrivateCensusMCPBinding(fixture, host)
	request := operation.Request{Name: operation.Census, RequestID: "continuation-host-failure", Input: []byte(`{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","stop_after":"DESCRIBE_REQUESTS"}}`)}
	got, err := binding.callDirect(context.Background(), request)
	envelope := string(got.Structured)
	if err != nil || !strings.Contains(envelope, `"code":"CONTINUATION_PUBLICATION_FAILED"`) || strings.Contains(envelope, "CONTINUATION_HOST_CONSTRUCTION_FAILED") {
		t.Fatalf("ASSERT_POST_CONSTRUCTION_FAILURE_NOT_HOST_ASSEMBLY: err=%v envelope=%s", err, envelope)
	}
	for _, forbidden := range []string{"private host detail", "/private/", "config.json"} {
		if strings.Contains(envelope, forbidden) {
			t.Fatalf("ASSERT_POST_CONSTRUCTION_FAILURE_PUBLIC_PRIVATE_SAFE forbidden=%q envelope=%s", forbidden, envelope)
		}
	}
}

func TestCensusContinuationStopAfterDescribeRequestsDirectCanonicalParity(t *testing.T) {
	census := privateCensusSuccessFixture()
	fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(census)}}
	descriptor := "g-" + strings.Repeat("a", 64) + ".selector.json"
	host := &continuationMCPHostFixture{result: censusContinuationMCPResult{Census: &census, Descriptor: descriptor, Status: "PAUSED", RequestCount: 3, PreparationCount: 2}}
	binding := newPrivateCensusMCPBinding(fixture, host)
	request := operation.Request{Name: operation.Census, RequestID: "continuation-stop", Input: []byte(`{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","stop_after":"DESCRIBE_REQUESTS"}}`)}
	direct, err := binding.callDirect(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := binding.callCanonical(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if string(direct.Structured) != string(canonical.Structured) || host.freshStop != "DESCRIBE_REQUESTS" || !strings.Contains(string(direct.Structured), `"outcome":"PAUSED"`) || !strings.Contains(string(direct.Structured), `"request_count":3`) {
		t.Fatalf("ASSERT_STOP_AFTER_MCP_DIRECT_GATEWAY_PARITY: stop=%q direct=%s canonical=%s", host.freshStop, direct.Structured, canonical.Structured)
	}
}

func TestCensusContinuationResumeBranchesBeforeCensusRunner(t *testing.T) {
	fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(privateCensusSuccessFixture())}}
	binding := newPrivateCensusMCPBinding(fixture)
	request := operation.Request{
		Name:      operation.Census,
		RequestID: "continuation-resume",
		Input:     []byte(`{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json"}}`),
	}

	got, err := binding.callCanonical(context.Background(), request)
	if err != nil {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_RESUME_TYPED_ENVELOPE: %v", err)
	}
	if len(fixture.calls) != 0 {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_RESUME_CENSUS_ZERO: calls=%d", len(fixture.calls))
	}
	var envelope map[string]any
	if err := json.Unmarshal(got.Structured, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["envelope_schema_id"] == "" || envelope["isError"] != true {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_RESUME_EXCLUSIVE_TYPED_ERROR: %v", envelope)
	}
	if !strings.Contains(string(got.Structured), "RESUME_NEEDS_WORKSPACE_OR_UNSUPPORTED") {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_RESUME_SAFE_TYPED_REASON: %s", got.Structured)
	}
}

func TestCensusContinuationFailedCaptureMapsBeforeDescriptorPublication(t *testing.T) {
	cp, err := censuscontinuation.NewCheckpoint(censuscontinuation.CheckpointInput{Stage: censuscontinuation.StageProgramCComputed, CensusID: "census:preserved", HandoffID: "sha256:" + strings.Repeat("a", 64), ContractID: "sha256:" + strings.Repeat("b", 64), ProfileID: "sha256:" + strings.Repeat("c", 64), Artifacts: []censuscontinuation.ArtifactRef{{Kind: "continuation_contract", ID: "sha256:" + strings.Repeat("d", 64), Digest: "sha256:" + strings.Repeat("d", 64), ByteLength: 1}, {Kind: "handoff", ID: "sha256:" + strings.Repeat("e", 64), Digest: "sha256:" + strings.Repeat("e", 64), ByteLength: 1}, {Kind: "program_c", ID: "sha256:" + strings.Repeat("f", 64), Digest: "sha256:" + strings.Repeat("f", 64), ByteLength: 25645747}}, Status: censuscontinuation.StatusFailedCapture, Diagnostics: []censuscontinuation.Diagnostic{{Code: "CAPTURE", Stage: censuscontinuation.StageProgramCComputed, Category: "input", Observed: 135, Limit: 100, FailedField: "bindings", Invariant: "OBSERVED_MUST_NOT_EXCEED_LIMIT", CallerAction: "INCREASE_BOUNDED_CAPTURE_LIMIT", Recovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := censuscontinuation.FailedCaptureResult(cp)
	if err != nil {
		t.Fatal(err)
	}
	host := &productionCensusContinuationHost{}
	_, err = host.publish(context.Background(), result)
	diagnostic, ok := censusresult.ContinuationDiagnosticFromError(err)
	if !ok || diagnostic.Code != censusresult.CodeContinuationCaptureFailed || diagnostic.ObservedStage != "PROGRAM_C_COMPUTED" || diagnostic.ObservedStatus != "FAILED_CAPTURE" || diagnostic.ResourceCategory != "input" || diagnostic.ResourceObserved != 135 || diagnostic.ResourceLimit != 100 || diagnostic.FailedField != "bindings" || diagnostic.Invariant != "OBSERVED_MUST_NOT_EXCEED_LIMIT" || diagnostic.CallerAction != "INCREASE_BOUNDED_CAPTURE_LIMIT" || diagnostic.Recovery != "RESTART_FROM_PRESERVED_CENSUS_COMMIT" || !diagnostic.CensusCommitPreserved {
		t.Fatalf("ASSERT_FAILED_CAPTURE_PRECEDES_DESCRIPTOR_PUBLICATION ok=%t diagnostic=%+v err=%v", ok, diagnostic, err)
	}
}

func TestCensusContinuationTerminalCheckpointStoreFailurePrecedesDescriptorFallback(t *testing.T) {
	host := &productionCensusContinuationHost{}
	_, err := host.publish(context.Background(), censuscontinuation.Result{Err: &censuscontinuation.TerminalCheckpointPersistError{}})
	diagnostic, ok := censusresult.ContinuationDiagnosticFromError(err)
	if !ok || diagnostic.Code != censusresult.CodeContinuationPublicationFailed {
		t.Fatalf("ASSERT_TERMINAL_CHECKPOINT_STORE_FAILURE_PRECEDES_DESCRIPTOR_FALLBACK ok=%t diagnostic=%+v err=%v", ok, diagnostic, err)
	}
}

func TestCensusContinuationTypedCausesDirectCanonicalParity(t *testing.T) {
	selector := "g-" + strings.Repeat("a", 64) + ".selector.json"
	cases := []struct {
		name  string
		cause censusresult.ContinuationCause
		code  string
		ctx   censusresult.ContinuationDiagnosticContext
	}{
		{name: "host_construction", cause: censusresult.ContinuationHostConstructionFailed, code: "CONTINUATION_HOST_CONSTRUCTION_FAILED", ctx: censusresult.ContinuationDiagnosticContext{ConstructionStage: "HOST_ASSEMBLY", ConstructionReason: "HOST_ASSEMBLY_FAILED"}},
		{name: "descriptor", cause: censusresult.ContinuationDescriptorUnavailable, code: "CONTINUATION_DESCRIPTOR_UNAVAILABLE", ctx: censusresult.ContinuationDiagnosticContext{PublicSelector: selector}},
		{name: "checkpoint", cause: censusresult.ContinuationStopCheckpointMismatch, code: "CONTINUATION_STOP_CHECKPOINT_MISMATCH", ctx: censusresult.ContinuationDiagnosticContext{PublicSelector: selector, ExpectedStage: "REQUESTS_RENDERED", ExpectedStatus: "RUNNING", ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "RUNNING"}},
		{name: "publication", cause: censusresult.ContinuationPublicationFailed, code: "CONTINUATION_PUBLICATION_FAILED"},
		{name: "capture", cause: censusresult.ContinuationCaptureFailed, code: "CONTINUATION_CAPTURE_FAILED", ctx: censusresult.ContinuationDiagnosticContext{ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "FAILED_CAPTURE", ResourceCategory: "input", ResourceObserved: 135, ResourceLimit: 100, ResourceField: "bindings", CaptureInvariant: "OBSERVED_MUST_NOT_EXCEED_LIMIT", CaptureCallerAction: "INCREASE_BOUNDED_CAPTURE_LIMIT", CaptureRecovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}},
		{name: "capture_availability", cause: censusresult.ContinuationCaptureFailed, code: "CONTINUATION_CAPTURE_FAILED", ctx: censusresult.ContinuationDiagnosticContext{ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "FAILED_CAPTURE", ResourceCategory: "availability", ResourceField: "source_preparation", CaptureInvariant: "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT", CaptureCallerAction: "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME", CaptureRecovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}},
		{name: "capture_internal_resolver", cause: censusresult.ContinuationCaptureFailed, code: "CONTINUATION_CAPTURE_FAILED", ctx: censusresult.ContinuationDiagnosticContext{ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "FAILED_CAPTURE", ResourceCategory: "internal", ResourceField: "resolver", CaptureInvariant: "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT", CaptureCallerAction: "RETRY_OR_REPORT_CAPTURE_IMPLEMENTATION", CaptureRecovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}},
		{name: "capture_internal_serialization", cause: censusresult.ContinuationCaptureFailed, code: "CONTINUATION_CAPTURE_FAILED", ctx: censusresult.ContinuationDiagnosticContext{ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "FAILED_CAPTURE", ResourceCategory: "internal", ResourceField: "serialization", CaptureInvariant: "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT", CaptureCallerAction: "REPORT_CAPTURE_IMPLEMENTATION", CaptureRecovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}},
		{name: "capture_internal_runtime_injection", cause: censusresult.ContinuationCaptureFailed, code: "CONTINUATION_CAPTURE_FAILED", ctx: censusresult.ContinuationDiagnosticContext{ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "FAILED_CAPTURE", ResourceCategory: "internal", ResourceField: "runtime_injection", CaptureInvariant: "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT", CaptureCallerAction: "FIX_CAPTURE_RUNTIME_INJECTION", CaptureRecovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}},
		{name: "capture_internal_capture", cause: censusresult.ContinuationCaptureFailed, code: "CONTINUATION_CAPTURE_FAILED", ctx: censusresult.ContinuationDiagnosticContext{ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "FAILED_CAPTURE", ResourceCategory: "internal", ResourceField: "capture", CaptureInvariant: "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT", CaptureCallerAction: "RETRY_OR_REPORT_CAPTURE_IMPLEMENTATION", CaptureRecovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failure := censusresult.NewContinuationFailure(tc.cause, tc.ctx)
			host := &continuationMCPHostFixture{err: failure}
			fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(privateCensusSuccessFixture())}}
			binding := newPrivateCensusMCPBinding(fixture, host)
			input := []byte(`{"session_id":"s","generation":1,"sources":["."],"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","stop_after":"DESCRIBE_REQUESTS"}}`)
			if tc.cause == censusresult.ContinuationDescriptorUnavailable || tc.cause == censusresult.ContinuationStopCheckpointMismatch {
				input = []byte(`{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"` + selector + `","stop_after":"DESCRIBE_REQUESTS"}}`)
			}
			request := operation.Request{Name: operation.Census, RequestID: "cause-" + tc.name, Input: input}
			direct, directErr := binding.callDirect(context.Background(), request)
			canonical, canonicalErr := binding.callCanonical(context.Background(), request)
			if directErr != nil || canonicalErr != nil || string(direct.Structured) != string(canonical.Structured) {
				t.Fatalf("ASSERT_CONTINUATION_DIAGNOSTIC_TRANSPORT_PARITY[%s]: direct=%v canonical=%v", tc.name, directErr, canonicalErr)
			}
			var envelope map[string]any
			if err := json.Unmarshal(direct.Structured, &envelope); err != nil {
				t.Fatal(err)
			}
			result := envelope["result"].(map[string]any)
			if result["code"] != tc.code || result["census_commit_preserved"] != true || envelope["outcome"] != "COMMITTED_DEGRADED" {
				t.Fatalf("ASSERT_CONTINUATION_CAUSES_DISTINCT[%s]: %s", tc.name, direct.Structured)
			}
			if result["schema_version"] != "lsp-trace.census-continuation-diagnostic.v2" || result["recovery_selector_state"] != "RECOVERY_SELECTOR_UNAVAILABLE" || result["authority"] != float64(0) {
				t.Fatalf("ASSERT_CONTINUATION_V2_RECOVERY_CONTRACT[%s]: %s", tc.name, direct.Structured)
			}
			if tc.cause == censusresult.ContinuationCaptureFailed {
				if result["last_successful_stage"] != tc.ctx.ObservedStage || result["terminal_stage"] != tc.ctx.ObservedStage || result["terminal_status"] != tc.ctx.ObservedStatus {
					t.Fatalf("ASSERT_CONTINUATION_V2_STAGE_PROGRESSION[%s]: %s", tc.name, direct.Structured)
				}
				if result["preserved_census_selector"] != privateCensusSuccessFixture().Publication.Selector {
					t.Fatalf("ASSERT_CONTINUATION_V2_PRESERVED_CENSUS_SELECTOR[%s]: %s", tc.name, direct.Structured)
				}
				if _, invented := result["recovery_selector"]; invented {
					t.Fatalf("ASSERT_CONTINUATION_V2_NO_INVENTED_RECOVERY_SELECTOR[%s]: %s", tc.name, direct.Structured)
				}
			}
		})
	}
}

func TestCensusContinuationResumeHostOnceCensusZero(t *testing.T) {
	fixture := &privateCensusRunnerFixture{completion: censusCompletion{Result: ptrCensusResult(privateCensusSuccessFixture())}}
	descriptor := "g-" + strings.Repeat("b", 64) + ".selector.json"
	host := &continuationMCPHostFixture{result: censusContinuationMCPResult{CensusSelector: descriptor, CensusDigest: "sha256:" + strings.Repeat("c", 64), CensusByteLength: uint64(len(descriptor)), Descriptor: descriptor, Status: "COMPLETE"}}
	binding := newPrivateCensusMCPBinding(fixture, host)
	request := operation.Request{Name: operation.Census, RequestID: "continuation-resume-success", Input: []byte(`{"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","resume_selector":"g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json"}}`)}
	got, err := binding.callCanonical(context.Background(), request)
	if err != nil {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_RESUME_SUCCESS: %v", err)
	}
	if len(fixture.calls) != 0 || host.resumeCalls != 1 || host.freshCalls != 0 {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_RESUME_EXECUTOR_SESSION_CORE_ZERO: census=%d resume=%d fresh=%d", len(fixture.calls), host.resumeCalls, host.freshCalls)
	}
	if err := mcpcontract.ValidateFutureCensusCompositeEnvelopeV2(got.Structured); err != nil {
		t.Fatalf("ASSERT_CENSUS_CONTINUATION_RESUME_SUCCESSOR_VALID: %v", err)
	}
}
