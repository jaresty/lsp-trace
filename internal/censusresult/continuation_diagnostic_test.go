package censusresult

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestContinuationDiagnosticCausesAreDistinctClosedAndPrivateSafe(t *testing.T) {
	cases := []struct {
		cause ContinuationCause
		want  ContinuationCode
	}{
		{ContinuationHostConstructionFailed, CodeContinuationHostConstructionFailed},
		{ContinuationDescriptorUnavailable, CodeContinuationDescriptorUnavailable},
		{ContinuationStopCheckpointMismatch, CodeContinuationStopCheckpointMismatch},
		{ContinuationPublicationFailed, CodeContinuationPublicationFailed},
	}
	seen := map[ContinuationCode]bool{}
	for _, tc := range cases {
		d, err := NewContinuationDiagnostic(tc.cause, ContinuationDiagnosticContext{
			ConstructionStage: "ROOT_OPEN", ConstructionReason: "ROOT_OPEN_FAILED",
			PublicSelector: "g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json",
			ExpectedStage:  "REQUESTS_RENDERED", ExpectedStatus: "RUNNING",
			ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "RUNNING",
		})
		if err != nil {
			t.Fatalf("ASSERT_CONTINUATION_CAUSES_DISTINCT[%s]: %v", tc.cause, err)
		}
		if d.Code != tc.want || seen[d.Code] {
			t.Fatalf("ASSERT_CONTINUATION_CAUSES_DISTINCT[%s]: code=%s", tc.cause, d.Code)
		}
		seen[d.Code] = true
		if !d.CensusCommitPreserved || d.Phase == "" || d.Stage == "" || d.FailedField == "" || d.Invariant == "" || d.RetryAction == "" || d.CallerAction == "" || d.Guidance == "" {
			t.Fatalf("ASSERT_CONTINUATION_DIAGNOSTIC_ACTIONABLE_FIELDS[%s]: %+v", tc.cause, d)
		}
		raw, err := MarshalContinuationDiagnostic(d)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"/private/", "private host detail", "worker_pin", "publication_root", "raw_error"} {
			if bytes.Contains(raw, []byte(forbidden)) {
				t.Fatalf("ASSERT_CONTINUATION_DIAGNOSTIC_PRIVATE_SAFE_CLOSED[%s]: %s", tc.cause, raw)
			}
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		if tc.cause != ContinuationStopCheckpointMismatch {
			for _, field := range []string{"expected_stage", "expected_status", "observed_stage", "observed_status"} {
				if _, ok := decoded[field]; ok {
					t.Fatalf("ASSERT_CONTINUATION_DIAGNOSTIC_PRIVATE_SAFE_CLOSED[%s]: unexpected %s", tc.cause, field)
				}
			}
		}
	}
}

func TestContinuationCaptureGenericDiagnosticIsTypedPrivateSafeAndActionable(t *testing.T) {
	ctx := ContinuationDiagnosticContext{ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "FAILED_CAPTURE", ResourceCategory: "availability", ResourceField: "source_preparation", CaptureInvariant: "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT", CaptureCallerAction: "RETRY_WITH_MANAGED_SOURCE_SUPPLY", CaptureRecovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}
	d, err := NewContinuationDiagnostic(ContinuationCaptureFailed, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Code != CodeContinuationCaptureFailed || d.FailedField != "source_preparation" || d.ResourceCategory != "availability" || d.ResourceObserved != 0 || d.ResourceLimit != 0 || d.CallerAction != "RETRY_WITH_MANAGED_SOURCE_SUPPLY" {
		t.Fatalf("ASSERT_GENERIC_CAPTURE_PUBLIC_DIAGNOSTIC: %+v", d)
	}
	raw, err := MarshalContinuationDiagnostic(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private", "source.go", "managed document preparation unavailable"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("ASSERT_GENERIC_CAPTURE_PUBLIC_DIAGNOSTIC_PRIVATE_SAFE forbidden=%q raw=%s", forbidden, raw)
		}
	}
}

func TestContinuationCaptureLimitDiagnosticIsTypedPrivateSafeAndActionable(t *testing.T) {
	ctx := ContinuationDiagnosticContext{ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "FAILED_CAPTURE", ResourceCategory: "input", ResourceObserved: 135, ResourceLimit: 100, ResourceField: "bindings", CaptureInvariant: "OBSERVED_MUST_NOT_EXCEED_LIMIT", CaptureCallerAction: "INCREASE_BOUNDED_CAPTURE_LIMIT", CaptureRecovery: "RESTART_FROM_PRESERVED_CENSUS_COMMIT"}
	d, err := NewContinuationDiagnostic(ContinuationCaptureFailed, ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalContinuationDiagnostic(d)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, required := range []string{`"code":"CONTINUATION_CAPTURE_FAILED"`, `"resource_category":"input"`, `"resource_observed":135`, `"resource_limit":100`, `"failed_field":"bindings"`, `"invariant":"OBSERVED_MUST_NOT_EXCEED_LIMIT"`, `"caller_action":"INCREASE_BOUNDED_CAPTURE_LIMIT"`, `"recovery":"RESTART_FROM_PRESERVED_CENSUS_COMMIT"`, `"census_commit_preserved":true`} {
		if !strings.Contains(got, required) {
			t.Fatalf("ASSERT_CAPTURE_LIMIT_PUBLIC_DIAGNOSTIC missing=%q got=%s", required, got)
		}
	}
	for _, forbidden := range []string{"sha256:", "/private/", "descriptor"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("ASSERT_CAPTURE_LIMIT_PUBLIC_DIAGNOSTIC_PRIVATE_SAFE forbidden=%q got=%s", forbidden, got)
		}
	}
	for _, mutate := range []func(*ContinuationDiagnosticContext){func(c *ContinuationDiagnosticContext) { c.ResourceField = "raw_path" }, func(c *ContinuationDiagnosticContext) { c.ResourceCategory = "source_path" }, func(c *ContinuationDiagnosticContext) { c.CaptureRecovery = "READ_RAW" }} {
		bad := ctx
		mutate(&bad)
		if _, err := NewContinuationDiagnostic(ContinuationCaptureFailed, bad); err == nil {
			t.Fatal("ASSERT_CAPTURE_DIAGNOSTIC_CLOSED_ENUMS")
		}
	}
}

func TestContinuationDiagnosticGuidanceAndStrictJSON(t *testing.T) {
	for _, cause := range []ContinuationCause{ContinuationHostConstructionFailed, ContinuationDescriptorUnavailable, ContinuationStopCheckpointMismatch, ContinuationPublicationFailed} {
		d, err := NewContinuationDiagnostic(cause, ContinuationDiagnosticContext{ConstructionStage: "ROOT_OPEN", ConstructionReason: "ROOT_OPEN_FAILED", PublicSelector: "g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.selector.json", ExpectedStage: "REQUESTS_RENDERED", ExpectedStatus: "RUNNING", ObservedStage: "PROGRAM_C_COMPUTED", ObservedStatus: "RUNNING"})
		if err != nil {
			t.Fatal(err)
		}
		joined := d.RetryAction + " " + d.CallerAction + " " + d.Guidance
		if !strings.Contains(joined, "census commit is preserved") || (!strings.Contains(joined, "resume") && !strings.Contains(joined, "provision")) {
			t.Fatalf("ASSERT_CONTINUATION_DIAGNOSTIC_ACTIONABLE_FIELDS[%s]: %q", cause, joined)
		}
		raw, _ := MarshalContinuationDiagnostic(d)
		var value map[string]any
		_ = json.Unmarshal(raw, &value)
		value["unknown"] = true
		unknown, _ := json.Marshal(value)
		if _, err := DecodeContinuationDiagnostic(unknown); err == nil {
			t.Fatalf("ASSERT_CONTINUATION_DIAGNOSTIC_STRICT_JSON[%s/unknown]", cause)
		}
		duplicate := append(bytes.TrimSpace(raw), []byte(` {}`)...)
		if _, err := DecodeContinuationDiagnostic(duplicate); err == nil {
			t.Fatalf("ASSERT_CONTINUATION_DIAGNOSTIC_STRICT_JSON[%s/trailing]", cause)
		}
	}
}
