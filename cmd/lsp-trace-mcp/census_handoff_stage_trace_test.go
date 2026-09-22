package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/publication"
)

type failingTraceWriter struct {
	calls *int
}

func (f failingTraceWriter) Write([]byte) (int, error) {
	if f.calls != nil {
		*f.calls = *f.calls + 1
	}
	return 0, errors.New("malicious trace sink failure")
}

func TestCensusHandoffStageTraceNormalization(t *testing.T) {
	cases := []struct {
		name  string
		stage censusprogramc.Stage
		want  string
	}{
		{"publication", censusprogramc.StagePublication, "PUBLICATION_PRECONDITION"},
		{"reconciliation", censusprogramc.StageReconciliation, "RECONCILIATION"},
		{"composition", censusprogramc.StageComposition, "COMPOSITION"},
		{"admission", censusprogramc.StageAdmission, "ADMISSION"},
		{"computation", censusprogramc.StageComputation, "COMPUTATION"},
		{"representative", censusprogramc.StageRepresentative, "REPRESENTATIVE_QUALIFICATION"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fmt.Errorf("outer: %w", &censusprogramc.Failure{Stage: tc.stage, Err: errors.New("secret sentinel")})
			if got := string(handoffFailureStage(err)); got != tc.want {
				t.Fatalf("ASSERT_HANDOFF_STAGE_EXTRACTION_%s: got=%q want=%q", tc.name, got, tc.want)
			}
		})
	}
	var typedNil *censusprogramc.Failure
	for name, err := range map[string]error{
		"nil":       nil,
		"missing":   errors.New("secret sentinel missing"),
		"typed-nil": typedNil,
		"unknown":   &censusprogramc.Failure{Stage: censusprogramc.Stage("SECRET_STAGE"), Err: errors.New("secret sentinel")},
	} {
		t.Run(name, func(t *testing.T) {
			if got := handoffFailureStage(err); got != "" {
				t.Fatalf("ASSERT_HANDOFF_STAGE_UNKNOWN_NORMALIZES: got=%q", got)
			}
		})
	}
}

func TestCensusHandoffStageTraceUnknownDoesNotLeak(t *testing.T) {
	previous := descriptorTraceOutput
	t.Cleanup(func() { descriptorTraceOutput = previous })
	var output bytes.Buffer
	descriptorTraceOutput = &output
	t.Setenv(runtimeTracePathEnv, t.TempDir()+"/trace")
	censusHandoffStageTrace(context.Background(), "SECRET_STAGE")
	if output.String() != "CENSUS_HANDOFF_STAGE stage=UNKNOWN\n" {
		t.Fatalf("ASSERT_HANDOFF_STAGE_TRACE_UNKNOWN_ALLOWLIST: %q", output.String())
	}
}

func malformedCommittedCompletion(t *testing.T) censusCompletion {
	t.Helper()
	projection := mcpContinuationProjection(t)
	rootDir := t.TempDir()
	if err := os.Chmod(rootDir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	publisher := captureset.NewPublisher(root)
	metadata := programccompose.ExactMetadata{RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1"}
	completion := completeCensusProjectionWithContinuation(context.Background(), projection, publisher, publisher, metadata)
	if completion.continuation == nil {
		t.Fatal("ASSERT_REAL_COMMITTED_CUSTODY_CONSTRUCTED")
	}
	custody, err := cloneCensusCompletionCustody(*completion.continuation)
	if err != nil {
		t.Fatal(err)
	}
	custody.publication.Resolved[0].Bytes = []byte("actual secret sentinel")
	completion.continuation = &custody
	return completion
}

func freshOutcome(t *testing.T, completion censusCompletion, tracePath string, sink io.Writer) (censusContinuationMCPResult, error, string) {
	t.Helper()
	t.Setenv(runtimeTracePathEnv, tracePath)
	previous := descriptorTraceOutput
	descriptorTraceOutput = sink
	t.Cleanup(func() { descriptorTraceOutput = previous })
	result, err := (&productionCensusContinuationHost{}).Fresh(context.Background(), completion, "")
	return result, err, fmt.Sprintf("%T:%v", err, err)
}

func TestCensusHandoffStageTraceFreshInputCountOutcome(t *testing.T) {
	completion := committedSingleConstituentCompletion(t, 1)
	if err := assertSingleConstituentCompletion(completion); err != nil {
		t.Fatal(err)
	}
	custodyBefore, err := cloneCensusCompletionCustody(*completion.continuation)
	if err != nil {
		t.Fatal(err)
	}
	resultBefore := completion.Result

	var disabled bytes.Buffer
	baselineResult, baselineErr, baselineClass := freshOutcome(t, completion, "", &disabled)
	if disabled.Len() != 0 {
		t.Fatalf("ASSERT_FRESH_INPUT_COUNT_TRACE_DISABLED_SILENCE: %q", disabled.String())
	}
	if baselineErr == nil {
		t.Fatal("ASSERT_FRESH_INPUT_COUNT_PUBLIC_REJECTION")
	}
	if strings.Contains(baselineClass, "input count 1") {
		t.Fatalf("ASSERT_FRESH_INPUT_COUNT_PUBLIC_NON_DISCLOSURE: %q", baselineClass)
	}

	var enabled bytes.Buffer
	enabledResult, enabledErr, enabledClass := freshOutcome(t, completion, t.TempDir()+"/trace", &enabled)
	if enabledResult != baselineResult || enabledClass != baselineClass || (enabledErr == nil) != (baselineErr == nil) {
		t.Fatalf("ASSERT_FRESH_INPUT_COUNT_TRACE_ENABLED_PRIMARY_EQUALITY: baseline=%+v/%s enabled=%+v/%s", baselineResult, baselineClass, enabledResult, enabledClass)
	}
	if got := enabled.String(); strings.Count(got, "CENSUS_HANDOFF_STAGE stage=COMPOSITION\n") != 1 || !strings.Contains(got, "CENSUS_HANDOFF_BRANCH branch=INPUT_COUNT\n") {
		t.Fatalf("ASSERT_FRESH_INPUT_COUNT_COMPOSITION_BRANCH: %q", got)
	}
	if strings.Contains(enabled.String(), "input count 1") || strings.Contains(enabled.String(), "publication-session") {
		t.Fatalf("ASSERT_FRESH_INPUT_COUNT_PRIVATE_DIAGNOSTIC: %q", enabled.String())
	}

	calls := 0
	failedResult, failedErr, failedClass := freshOutcome(t, completion, t.TempDir()+"/trace", failingTraceWriter{calls: &calls})
	if calls == 0 {
		t.Fatal("ASSERT_FRESH_INPUT_COUNT_FAILING_SINK_INVOKED")
	}
	if failedResult != baselineResult || failedClass != baselineClass || (failedErr == nil) != (baselineErr == nil) {
		t.Fatalf("ASSERT_FRESH_INPUT_COUNT_SINK_FAILURE_PRIMARY_EQUALITY: baseline=%+v/%s failed=%+v/%s", baselineResult, baselineClass, failedResult, failedClass)
	}
	if completion.Result != resultBefore || !reflect.DeepEqual(completion.continuation, &custodyBefore) {
		t.Fatal("ASSERT_FRESH_INPUT_COUNT_COMMITTED_CUSTODY_UNCHANGED")
	}
}

func TestCensusHandoffStageTraceEmissionAndFreshOutcome(t *testing.T) {
	completion := malformedCommittedCompletion(t)
	_, originalHandoffErr := completion.BuildCommittedHandoff()
	if originalHandoffErr == nil || originalHandoffErr.Error() == "" {
		t.Fatalf("ASSERT_ORIGINAL_HANDOFF_ERROR_TEXT_AVAILABLE: %v", originalHandoffErr)
	}
	t.Logf("ASSERT_ORIGINAL_HANDOFF_ERROR_TEXT: %q", originalHandoffErr.Error())
	var disabled bytes.Buffer
	baselineResult, baselineErr, baselineClass := freshOutcome(t, completion, "", &disabled)
	if disabled.Len() != 0 {
		t.Fatalf("ASSERT_FRESH_STAGE_TRACE_DISABLED_SILENCE: %q", disabled.String())
	}

	var enabled bytes.Buffer
	enabledResult, enabledErr, enabledClass := freshOutcome(t, completion, t.TempDir()+"/trace", &enabled)
	if enabledResult != baselineResult || enabledClass != baselineClass || (enabledErr == nil) != (baselineErr == nil) {
		t.Fatalf("ASSERT_FRESH_STAGE_TRACE_ENABLED_PRIMARY_EQUALITY: baseline=%+v/%s enabled=%+v/%s", baselineResult, baselineClass, enabledResult, enabledClass)
	}
	if !strings.Contains(enabled.String(), "CENSUS_HANDOFF_STAGE stage=PUBLICATION_PRECONDITION") {
		t.Fatalf("ASSERT_FRESH_STAGE_TRACE_ACTUAL_WRAPPED_STAGE: %q", enabled.String())
	}
	if strings.Contains(enabled.String(), originalHandoffErr.Error()) {
		t.Fatalf("ASSERT_FRESH_STAGE_TRACE_NON_DISCLOSURE: leaked=%q trace=%q", originalHandoffErr.Error(), enabled.String())
	}
	if strings.Contains(baselineClass, originalHandoffErr.Error()) {
		t.Fatalf("ASSERT_FRESH_STAGE_PUBLIC_NON_DISCLOSURE: %q", baselineClass)
	}

	calls := 0
	failing := failingTraceWriter{calls: &calls}
	failedResult, failedErr, failedClass := freshOutcome(t, completion, t.TempDir()+"/trace", failing)
	if calls == 0 {
		t.Fatal("ASSERT_FRESH_STAGE_TRACE_FAILING_WRITER_INVOKED")
	}
	if failedResult != baselineResult || failedClass != baselineClass || (failedErr == nil) != (baselineErr == nil) {
		t.Fatalf("ASSERT_FRESH_STAGE_TRACE_SINK_FAILURE_PRIMARY_EQUALITY: baseline=%+v/%s failed=%+v/%s", baselineResult, baselineClass, failedResult, failedClass)
	}
}
