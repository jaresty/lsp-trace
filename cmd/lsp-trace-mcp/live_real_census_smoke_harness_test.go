package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censusdiagnostic"
)

func TestLiveRealRetainedStoresOutliveChildCleanup(t *testing.T) {
	evidence := filepath.Join(t.TempDir(), "evidence")
	if err := os.Mkdir(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	var publication, continuation string
	t.Run("allocate-and-write", func(t *testing.T) {
		var err error
		publication, continuation, err = allocateLiveRealRetainedStores(evidence, "publication", "continuation")
		if err != nil {
			t.Fatal(err)
		}
		if publication == continuation || filepath.Dir(publication) != evidence || filepath.Dir(continuation) != evidence {
			t.Fatal("stores are not distinct children of evidence root")
		}
		for _, store := range []string{publication, continuation} {
			if err := os.WriteFile(filepath.Join(store, "nested", "object"), []byte("unreachable"), 0600); err == nil {
				t.Fatal("expected nested parent creation to be explicit")
			}
			if err := os.Mkdir(filepath.Join(store, "nested"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(store, "nested", "object"), []byte(filepath.Base(store)+"-bytes"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	})
	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{"publication", filepath.Join(publication, "nested", "object"), "publication-bytes"},
		{"continuation", filepath.Join(continuation, "nested", "object"), "continuation-bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := os.ReadFile(tc.path)
			if err != nil || string(got) != tc.want {
				t.Fatalf("retained bytes=%q err=%v", got, err)
			}
		})
	}
}

func TestLiveRealRetainedStoresRejectUnsafeOrExistingDestinations(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "valid")
	if err := os.Mkdir(valid, 0700); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		setup func(string) error
	}{
		{"invalid-root", func(string) error { return nil }},
		{"symlink-root", func(path string) error { return os.Symlink(valid, path) }},
		{"existing-publication", func(path string) error { return os.Mkdir(filepath.Join(path, "publication"), 0700) }},
		{"symlink-publication", func(path string) error {
			return os.Symlink(filepath.Join(path, "target"), filepath.Join(path, "publication"))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name)
			if tc.name == "invalid-root" {
				path = filepath.Join(root, "missing")
			} else if tc.name == "symlink-root" {
				if err := tc.setup(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			} else if err := tc.setup(path); err != nil {
				t.Fatal(err)
			}
			if _, _, err := allocateLiveRealRetainedStores(path, "publication", "continuation"); err == nil {
				t.Fatal("expected unsafe or existing destination refusal")
			}
		})
	}
	if _, _, err := allocateLiveRealRetainedStores(valid, "publication-pass", "continuation-pass"); err != nil {
		t.Fatalf("secure fresh destinations rejected: %v", err)
	}
}

func TestLiveRealCandidateArgsUseRegisteredFlags(t *testing.T) {
	args := liveRealCandidateArgs("bootstrap.json", "publication", "acquisition.ndjson")
	joined := strings.Join(args, " ")
	for _, unsupported := range []string{"--continuation-root", "--managed-preparation-diagnostic-path"} {
		if strings.Contains(joined, unsupported) {
			t.Fatalf("unsupported flag %q present in candidate args", unsupported)
		}
	}
	for _, registered := range []string{"--bootstrap-config", "--publication-root", "--acquisition-diagnostic-path"} {
		if !strings.Contains(joined, registered) {
			t.Fatalf("registered flag %q missing", registered)
		}
	}
}

func TestLiveRealSemanticTemplateVector(t *testing.T) {
	path := t.TempDir() + "/REQUEST.template.json"
	if err := os.WriteFile(path, []byte(`{"session_id":"REPLACE_FROM_SINGLE_POST_RESTART_SESSION_LIST","generation":0,"sources":["internal/censuscontinuation/pipeline.go","internal/censuscontinuation/capture.go"],"down_depth":1,"up_depth":0,"max_nodes":10000,"batch_targets":16,"timeout_ms":60000,"request_timeout_ms":30000,"continuation":{"kind":"ADR_0007_FEATURE_CATALOG","stop_after":"DESCRIBE_REQUESTS"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, got, err := loadLiveRealTemplate(path, "live", 7)
	if err != nil {
		t.Fatal(err)
	}
	if got != liveRealPinnedSemantic {
		t.Fatalf("got %s want %s", got, liveRealPinnedSemantic)
	}
}

func TestLiveRealClosedFailureClassification(t *testing.T) {
	if got := liveRealClosedFailureClass("flag provided but not defined: -x", 2, "startup"); got != "STARTUP_FLAG_REJECTED" {
		t.Fatalf("got %q", got)
	}
	if got := liveRealClosedFailureClass("", 0, "before-session-list"); got != "SERVE_EOF_BEFORE_SESSION_LIST" {
		t.Fatalf("got %q", got)
	}
	if got := liveRealClosedFailureClass("", 1, "census"); got != "PROCESS_EXIT_NONZERO" {
		t.Fatalf("got %q", got)
	}
}

type liveRealContinuation struct {
	Kind      string `json:"kind"`
	StopAfter string `json:"stop_after"`
}

type liveRealTemplate struct {
	SessionID        string               `json:"session_id"`
	Generation       uint64               `json:"generation"`
	Sources          []string             `json:"sources"`
	DownDepth        uint64               `json:"down_depth"`
	UpDepth          uint64               `json:"up_depth"`
	MaxNodes         uint64               `json:"max_nodes"`
	BatchTargets     uint64               `json:"batch_targets"`
	TimeoutMS        uint64               `json:"timeout_ms"`
	RequestTimeoutMS uint64               `json:"request_timeout_ms"`
	Continuation     liveRealContinuation `json:"continuation"`
}

const liveRealPinnedSemantic = "sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9"

func liveRealStagedRequest(v liveRealTemplate) map[string]any {
	return map[string]any{"sources": v.Sources, "down_depth": v.DownDepth, "up_depth": v.UpDepth, "max_nodes": v.MaxNodes, "batch_targets": v.BatchTargets, "timeout_ms": v.TimeoutMS, "request_timeout_ms": v.RequestTimeoutMS, "continuation": v.Continuation}
}

func loadLiveRealTemplate(path string, session string, generation uint64) (liveRealTemplate, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return liveRealTemplate{}, "", err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var v liveRealTemplate
	if err := dec.Decode(&v); err != nil {
		return liveRealTemplate{}, "", err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return liveRealTemplate{}, "", errors.New("trailing template JSON")
	}
	if v.SessionID != "REPLACE_FROM_SINGLE_POST_RESTART_SESSION_LIST" || v.Generation != 0 || v.Continuation.Kind != "ADR_0007_FEATURE_CATALOG" || v.Continuation.StopAfter != "DESCRIBE_REQUESTS" {
		return liveRealTemplate{}, "", errors.New("template routing mismatch")
	}
	v.SessionID, v.Generation = session, generation
	semantic := map[string]any{"sources": v.Sources, "down_depth": v.DownDepth, "up_depth": v.UpDepth, "max_nodes": v.MaxNodes, "batch_targets": v.BatchTargets, "timeout_ms": v.TimeoutMS, "request_timeout_ms": v.RequestTimeoutMS, "continuation": v.Continuation}
	canonical, err := json.Marshal(semantic)
	if err != nil {
		return liveRealTemplate{}, "", err
	}
	d := sha256.Sum256(canonical)
	return v, "sha256:" + hex.EncodeToString(d[:]), nil
}

type liveRealSend func(int, string, map[string]any) (map[string]any, error)
type liveRealPhaseError struct {
	phase string
	err   error
}

func (e liveRealPhaseError) Error() string { return e.phase + ": " + e.err.Error() }
func (e liveRealPhaseError) Unwrap() error { return e.err }

type liveRealPhaseResult struct {
	response map[string]any
	semantic string
}
type liveRealRunInput struct {
	evidenceDir, tracePath, diagnosticPath, candidateDigest, semanticFingerprint string
}
type liveRealRunResult struct {
	err                                                                                   error
	response                                                                              map[string]any
	semantic, runtimeFingerprint, terminal, failureClass, failedField, failedInvariant    string
	exitCode, traceBytes                                                                  int
	exitStatus                                                                            string
	traceDigest, diagnosticCategory, diagnosticOperationCategory, diagnosticOperationCode string
	diagnosticOrdinalPresent                                                              bool
	diagnosticOrdinal                                                                     int
	diagnosticFailedField, diagnosticFailedInvariant                                      string
	diagnosticExpectedCount, diagnosticObservedCount                                      int
	diagnosticExpectedCountPresent, diagnosticObservedCountPresent                        bool
	publicResponseDigest, publicResponseBytes                                             string
	diagnosticAccounting                                                                  string
	cleanupStatus, cleanupExitCode                                                        string
}

const liveRealFinalizationTimeout = 750 * time.Millisecond

func runLiveRealAfterStart(cmd *exec.Cmd, ctx context.Context, in io.WriteCloser, reader *bufio.Reader, stderr interface{ String() string }, phase func(liveRealSend) (liveRealPhaseResult, error), input liveRealRunInput) liveRealRunResult {
	result := liveRealRunResult{failureClass: "UNKNOWN_CLOSED", failedField: "request", failedInvariant: "STAGED_REQUEST_NORMALIZES_TO_INTERNAL_RECEIPT"}
	enc := json.NewEncoder(in)
	send := func(id int, name string, args map[string]any) (map[string]any, error) {
		if err := enc.Encode(callRequest(id, name, args)); err != nil {
			return nil, err
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var v map[string]any
		if err := json.Unmarshal(line, &v); err != nil {
			return nil, err
		}
		return v, nil
	}
	var phaseErr error
	var pr liveRealPhaseResult
	if phase != nil {
		pr, phaseErr = phase(send)
	}
	// Capture the phase response before classifying process termination. A successful
	// paused terminal is still a valid result even when the serving process then exits.
	result.response, result.semantic = pr.response, pr.semantic
	_ = in.Close()
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		result.failureClass = "HARNESS_TIMEOUT"
		_ = cmd.Process.Kill()
		waitErr = <-waitCh
	case <-time.After(liveRealFinalizationTimeout):
		result.failureClass = "HARNESS_TIMEOUT"
		_ = cmd.Process.Kill()
		waitErr = <-waitCh
	}
	result.exitCode = exitCode(waitErr)
	if result.exitCode >= 0 {
		result.exitStatus = fmt.Sprintf("exit-%d", result.exitCode)
	} else {
		result.exitStatus = "unknown"
	}
	if waitErr == nil {
		result.cleanupStatus = "EXITED"
	} else if ctx.Err() != nil {
		result.cleanupStatus, result.cleanupExitCode = "TIMEOUT_KILLED", "unknown"
	} else {
		result.cleanupStatus, result.cleanupExitCode = "EXITED_NONZERO", fmt.Sprintf("%d", result.exitCode)
	}
	if ctx.Err() != nil {
		result.failureClass = "HARNESS_TIMEOUT"
		phaseErr = ctx.Err()
	}
	if waitErr != nil && phaseErr == nil {
		result.failureClass = liveRealClosedFailureClass(stderr.String(), exitCode(waitErr), "exit")
		phaseErr = fmt.Errorf("candidate exited")
	}
	var pe liveRealPhaseError
	if errors.As(phaseErr, &pe) {
		if pe.phase == "census-terminal" {
			result.failureClass = "REQUEST_ADMISSION_REJECTED"
			if pr.response != nil {
				result.failedInvariant = liveRealCensusAdmissionInvariant(pr.response)
				result.failedField = liveRealCensusInvariantField(result.failedInvariant)
			}
		} else if pe.phase == "census-acquisition" {
			result.failureClass = "ACQUISITION_FAILED"
			result.failedField = "acquisition"
			result.failedInvariant = "SYMBOL_ACQUISITION_COMPLETES"
		} else if pe.phase == "session-list" {
			result.failureClass = "SESSION_READINESS_FAILED"
			result.failedField = "session-list"
			result.failedInvariant = "EXACT_WORKSPACE_READY_SESSION_PRESENT"
		} else {
			result.failureClass = liveRealClosedFailureClass(stderr.String(), exitCode(waitErr), pe.phase)
		}
		result.err = fmt.Errorf("%s: %w", pe.phase, pe.err)
	} else {
		result.err = phaseErr
		if waitErr != nil && result.failureClass == "UNKNOWN_CLOSED" {
			result.failureClass = liveRealClosedFailureClass(stderr.String(), exitCode(waitErr), "exit")
		}
	}
	if result.failureClass == "UNKNOWN_CLOSED" && result.response == nil && result.exitCode <= 0 {
		result.failureClass = "SERVE_EOF_BEFORE_CENSUS"
	}
	if result.response != nil {
		result.terminal = terminalStatus(mustJSONNoFatal(result.response))
	}
	if result.err == nil && result.terminal == "SUCCEEDED/PAUSED/DESCRIBE_REQUESTS" {
		result.failureClass = "SUCCESS"
		result.failedField, result.failedInvariant = "", ""
	} else if result.failureClass == "UNKNOWN_CLOSED" && result.err == nil {
		result.failureClass = "SUCCESS"
	}
	if result.response != nil {
		public := mustJSONNoFatal(result.response)
		result.publicResponseDigest = digestBytes(public)
		result.publicResponseBytes = string(public)
	}
	result.runtimeFingerprint = requestReceiptFingerprint(result.response)
	if input.diagnosticPath != "" {
		if st, e := os.Stat(input.diagnosticPath); e == nil && st.Size() > 0 {
			d, e := censusdiagnostic.ReadLast(input.diagnosticPath)
			if e != nil {
				result.diagnosticAccounting = "READ_FAILURE"
			} else {
				result.diagnosticCategory, result.diagnosticOperationCategory, result.diagnosticOperationCode = d.Category, d.OperationCategory, d.OperationCode
				result.diagnosticFailedField, result.diagnosticFailedInvariant = d.FailedField, d.FailedInvariant
				if d.Ordinal != nil {
					result.diagnosticOrdinalPresent, result.diagnosticOrdinal = true, *d.Ordinal
				}
				if d.ExpectedCount != nil {
					result.diagnosticExpectedCountPresent, result.diagnosticExpectedCount = true, *d.ExpectedCount
				}
				if d.ObservedCount != nil {
					result.diagnosticObservedCountPresent, result.diagnosticObservedCount = true, *d.ObservedCount
				}
				if d.FailedField != "" {
					result.failedField, result.failedInvariant = d.FailedField, d.FailedInvariant
				}
			}
		} else if e != nil && !os.IsNotExist(e) {
			result.diagnosticAccounting = "STAT_FAILURE"
		}
	}
	if input.tracePath != "" {
		result.traceDigest, result.traceBytes = inspectLiveRealTrace(input.tracePath)
	}
	if result.failureClass == "SUCCESS" && result.err == nil {
		result.failedField, result.failedInvariant = "", ""
	}
	if input.evidenceDir != "" {
		if e := persistLiveRealEvidence(input, result); e != nil && result.err == nil {
			result.err = errors.New("evidence finalization failed")
		}
	}
	return result
}

func inspectLiveRealTrace(path string) (string, int) {
	i, e := os.Lstat(path)
	if e != nil || !i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm() != 0600 || i.Size() == 0 || i.Size() > 64<<20 {
		return "", 0
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", 0
	}
	d := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(d[:]), len(b)
}
func persistLiveRealEvidence(in liveRealRunInput, r liveRealRunResult) error {
	if err := os.Chmod(in.evidenceDir, 0700); err != nil {
		return err
	}
	s := map[string]any{"candidate_sha256": in.candidateDigest, "qualification_semantic_fingerprint": in.semanticFingerprint, "runtime_request_fingerprint": r.runtimeFingerprint, "terminal": r.terminal, "failure_class": r.failureClass, "failed_field": r.failedField, "failed_invariant": r.failedInvariant, "exit_status": r.exitStatus, "exit_code": r.exitCode, "diagnostic_category": r.diagnosticCategory, "diagnostic_operation_category": r.diagnosticOperationCategory, "diagnostic_operation_code": r.diagnosticOperationCode, "diagnostic_ordinal_present": r.diagnosticOrdinalPresent, "diagnostic_ordinal": r.diagnosticOrdinal, "diagnostic_failed_field": r.diagnosticFailedField, "diagnostic_failed_invariant": r.diagnosticFailedInvariant, "diagnostic_expected_count_present": r.diagnosticExpectedCountPresent, "diagnostic_expected_count": r.diagnosticExpectedCount, "diagnostic_observed_count_present": r.diagnosticObservedCountPresent, "diagnostic_observed_count": r.diagnosticObservedCount, "diagnostic_accounting": r.diagnosticAccounting, "cleanup_status": r.cleanupStatus, "cleanup_exit_code": r.cleanupExitCode, "trace_sha256": r.traceDigest, "trace_bytes": r.traceBytes, "public_response_sha256": r.publicResponseDigest, "public_response_bytes": len(r.publicResponseBytes), "worker_count": 0, "model_invocation_count": 0}
	b, _ := json.Marshal(s)
	if err := os.WriteFile(filepath.Join(in.evidenceDir, "summary.json"), b, 0600); err != nil {
		return err
	}
	files := map[string]string{"summary.json": digestBytes(b)}
	if r.publicResponseBytes != "" {
		raw := []byte(r.publicResponseBytes)
		if err := os.WriteFile(filepath.Join(in.evidenceDir, "public-response.json"), raw, 0600); err != nil {
			return err
		}
		files["public-response.json"] = digestBytes(raw)
	}
	if r.traceDigest != "" {
		raw, e := os.ReadFile(in.tracePath)
		if e != nil {
			return e
		}
		if err := os.WriteFile(filepath.Join(in.evidenceDir, "runtime.trace"), raw, 0600); err != nil {
			return err
		}
		files["runtime.trace"] = digestBytes(raw)
	}
	m, _ := json.Marshal(files)
	return os.WriteFile(filepath.Join(in.evidenceDir, "manifest.json"), m, 0600)
}
func digestBytes(b []byte) string { d := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(d[:]) }
func exitCode(err error) int {
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	return -1
}
func mustJSONNoFatal(v any) []byte { b, _ := json.Marshal(v); return b }

func decodeDirectEnvelope(response map[string]any) (map[string]any, error) {
	outer, ok := response["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("result missing")
	}
	content, ok := outer["content"].([]any)
	if !ok || len(content) != 1 {
		return nil, fmt.Errorf("content invalid")
	}
	item, ok := content[0].(map[string]any)
	if !ok || len(item) != 2 || item["type"] != "text" {
		return nil, fmt.Errorf("content item invalid")
	}
	text, ok := item["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text missing")
	}
	var domain map[string]any
	if err := json.Unmarshal([]byte(text), &domain); err != nil {
		return nil, err
	}
	structured, ok := outer["structuredContent"].(map[string]any)
	if !ok || !reflect.DeepEqual(domain, structured) {
		return nil, fmt.Errorf("structured content parity mismatch")
	}
	outerError, _ := outer["isError"].(bool)
	innerError, _ := domain["isError"].(bool)
	if outerError != innerError {
		return nil, fmt.Errorf("isError mismatch")
	}
	return domain, nil
}

func exactReadySessionValue(response map[string]any, workspace string) (string, uint64, error) {
	decoded, err := decodeDirectEnvelope(response)
	if err != nil {
		return "", 0, err
	}
	rawResult, ok := decoded["result"].(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("session domain result missing")
	}
	if _, ok := rawResult["Sessions"]; !ok {
		return "", 0, fmt.Errorf("session domain Sessions missing")
	}
	if _, wrongCase := rawResult["sessions"]; wrongCase {
		return "", 0, fmt.Errorf("session domain casing invalid")
	}
	var domain struct {
		Tool            string `json:"tool"`
		Outcome         string `json:"outcome"`
		OperationStatus string `json:"operation_status"`
		IsError         bool   `json:"isError"`
		Result          struct {
			Sessions []struct {
				SessionID        string `json:"SessionID"`
				Generation       uint64 `json:"Generation"`
				State            string `json:"State"`
				PositionEncoding string `json:"position_encoding"`
				Routing          struct {
					WorkspaceRoot string `json:"workspace_root"`
					Readiness     string `json:"readiness"`
					Generation    uint64 `json:"generation"`
				} `json:"routing"`
			} `json:"Sessions"`
			Census *struct {
				Workers int `json:"Workers"`
			} `json:"Census"`
		} `json:"result"`
	}
	encoded, err := json.Marshal(decoded)
	if err != nil || json.Unmarshal(encoded, &domain) != nil {
		return "", 0, fmt.Errorf("session domain decode")
	}

	if domain.Tool != "lsp_session_v1_list" || domain.Outcome != "COMPLETE" || domain.OperationStatus != "SUCCEEDED" || domain.IsError {
		return "", 0, fmt.Errorf("session domain status invalid")
	}
	var id string
	var generation uint64
	count := 0
	for _, s := range domain.Result.Sessions {
		if s.Routing.WorkspaceRoot != workspace || s.State != "READY" || s.Routing.Readiness != "READY" {
			continue
		}
		if s.SessionID == "" || s.Generation == 0 || s.Routing.Generation != s.Generation || (s.PositionEncoding != "" && s.PositionEncoding != "utf-16") {
			return "", 0, fmt.Errorf("matching session invalid")
		}
		id, generation, count = s.SessionID, s.Generation, count+1
	}
	if domain.Result.Census != nil && domain.Result.Census.Workers != 0 {
		return "", 0, fmt.Errorf("active workers")
	}
	if count != 1 {
		return "", 0, fmt.Errorf("exact workspace ready session unavailable")
	}
	return id, generation, nil
}

func TestLiveRealExactReadySessionFixtures(t *testing.T) {
	workspace := "/Users/schwa/dev/lsp-trace"
	session := func(id, ws, state, readiness string, generation, routingGeneration uint64) map[string]any {
		return map[string]any{"SessionID": id, "Generation": generation, "State": state, "position_encoding": "utf-16", "routing": map[string]any{"workspace_root": ws, "readiness": readiness, "generation": routingGeneration}}
	}
	wrap := func(domain map[string]any) map[string]any {
		b, _ := json.Marshal(domain)
		var structured map[string]any
		_ = json.Unmarshal(b, &structured)
		return map[string]any{"result": map[string]any{"isError": domain["isError"], "structuredContent": structured, "content": []any{map[string]any{"type": "text", "text": string(b)}}}}
	}
	domain := func(sessions []any, workers int) map[string]any {
		return map[string]any{"tool": "lsp_session_v1_list", "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "result": map[string]any{"Sessions": sessions, "Census": map[string]any{"Workers": workers}}}
	}
	goodSessions := []any{session("other", "/other", "READY", "READY", 1, 1), session("target", workspace, "READY", "READY", 1, 1)}
	badStatus := domain(goodSessions, 0)
	badStatus["operation_status"] = "FAILED"
	lowercase := map[string]any{"tool": "lsp_session_v1_list", "outcome": "COMPLETE", "operation_status": "SUCCEEDED", "isError": false, "result": map[string]any{"sessions": goodSessions}}
	for _, tt := range []struct {
		name     string
		response map[string]any
		want     bool
	}{
		{"multiple sessions exact match", wrap(domain(goodSessions, 0)), true},
		{"not ready", wrap(domain([]any{session("target", workspace, "STARTING", "READY", 1, 1)}, 0)), false},
		{"generation disagreement", wrap(domain([]any{session("target", workspace, "READY", "READY", 2, 1)}, 0)), false},
		{"duplicate target", wrap(domain([]any{session("a", workspace, "READY", "READY", 1, 1), session("b", workspace, "READY", "READY", 1, 1)}, 0)), false},
		{"active workers", wrap(domain(goodSessions, 1)), false},
		{"lowercase domain keys", wrap(lowercase), false},
		{"direct structured result", map[string]any{"result": map[string]any{"sessions": goodSessions}}, false},
		{"multiple content", map[string]any{"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "{}"}, map[string]any{"type": "text", "text": "{}"}}}}, false},
		{"nontext content", map[string]any{"result": map[string]any{"content": []any{map[string]any{"type": "image", "text": "{}"}}}}, false},
		{"failed domain status", wrap(badStatus), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := exactReadySessionValue(tt.response, workspace)
			if (err == nil) != tt.want {
				t.Fatalf("err=%v want=%v", err, tt.want)
			}
		})
	}
}

func TestLiveRealDirectCensusEnvelope(t *testing.T) {
	wrap := func(domain map[string]any) map[string]any {
		b, _ := json.Marshal(domain)
		var structured map[string]any
		_ = json.Unmarshal(b, &structured)
		return map[string]any{"result": map[string]any{"isError": domain["isError"], "structuredContent": structured, "content": []any{map[string]any{"type": "text", "text": string(b)}}}}
	}
	acquisition := map[string]any{"tool": "lsp_trace_v1_census", "outcome": "DOMAIN_ERROR", "operation_status": "FAILED", "isError": true, "error": map[string]any{"code": "ACQUISITION_FAILED", "stage": "acquisition"}, "request_receipt": map[string]any{"fingerprint": "sha256:receipt"}}
	decoded, err := decodeDirectEnvelope(wrap(acquisition))
	if err != nil {
		t.Fatal(err)
	}
	if got := liveRealCensusFailureCode(decoded); got != "ACQUISITION_FAILED" {
		t.Fatalf("failure code %q", got)
	}
	if got := requestReceiptFingerprint(decoded); got != "sha256:receipt" {
		t.Fatalf("receipt %q", got)
	}
	if status, stage, ok := liveRealCensusAdmission(decoded); ok || status != "FAILED" || stage != "acquisition" {
		t.Fatalf("terminal %q/%q ok=%v", status, stage, ok)
	}
	pausedResult := map[string]any{
		"schema_version": "lsp-trace.census-feature-catalog-result.v2",
		"catalog": map[string]any{
			"kind": "ADR_0007_FEATURE_CATALOG", "status": "PAUSED", "authority": 0,
			"accepted": false, "completeness": "UNKNOWN", "request_count": 3, "preparation_count": 2,
			"checkpoint_selector": "g-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.selector.json", "composite_selector": "g-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.selector.json", "catalog_selector": "g-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.selector.json", "resume_guidance": liveRealResumeGuidance},
		"census_identity": map[string]any{"selector": "g-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.selector.json", "digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "byte_length": 1},
	}
	paused := map[string]any{"tool": "lsp_trace_v1_census", "envelope_schema_id": "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-feature-catalog-result.v2.schema.json", "outcome": "PAUSED", "operation_status": "SUCCEEDED", "isError": false, "result": pausedResult}
	decoded, err = decodeDirectEnvelope(wrap(paused))
	if err != nil {
		t.Fatal(err)
	}
	if status, stage, ok := liveRealCensusAdmission(decoded); !ok || status != "SUCCEEDED" || stage != "describe-requests" {
		t.Fatalf("paused admission %q/%q ok=%v", status, stage, ok)
	}
	if got := requestReceiptFingerprint(decoded); got != "" {
		t.Fatalf("absent paused receipt %q", got)
	}
	if got := terminalStatus(mustJSONNoFatal(decoded)); got != "SUCCEEDED/PAUSED/DESCRIBE_REQUESTS" {
		t.Fatalf("terminal status %q", got)
	}
	for name, mutate := range map[string]func(map[string]any){
		"wrong schema":                 func(r map[string]any) { r["schema_version"] = "wrong" },
		"wrong catalog status":         func(r map[string]any) { r["catalog"].(map[string]any)["status"] = "COMPLETE" },
		"wrong authority":              func(r map[string]any) { r["catalog"].(map[string]any)["authority"] = 1 },
		"wrong accepted":               func(r map[string]any) { r["catalog"].(map[string]any)["accepted"] = true },
		"wrong completeness":           func(r map[string]any) { r["catalog"].(map[string]any)["completeness"] = "KNOWN" },
		"missing resume guidance":      func(r map[string]any) { delete(r["catalog"].(map[string]any), "resume_guidance") },
		"wrong resume guidance":        func(r map[string]any) { r["catalog"].(map[string]any)["resume_guidance"] = "wrong" },
		"missing preparation count":    func(r map[string]any) { delete(r["catalog"].(map[string]any), "preparation_count") },
		"negative preparation count":   func(r map[string]any) { r["catalog"].(map[string]any)["preparation_count"] = -1 },
		"preparation exceeds requests": func(r map[string]any) { r["catalog"].(map[string]any)["preparation_count"] = 4 },
		"fractional preparation count": func(r map[string]any) { r["catalog"].(map[string]any)["preparation_count"] = 0.5 },
		"missing request count":        func(r map[string]any) { delete(r["catalog"].(map[string]any), "request_count") },
		"zero request count":           func(r map[string]any) { r["catalog"].(map[string]any)["request_count"] = 0 },
		"fractional request count":     func(r map[string]any) { r["catalog"].(map[string]any)["request_count"] = 1.5 },
		"uppercase selector": func(r map[string]any) {
			r["catalog"].(map[string]any)["catalog_selector"] = "G-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.selector.json"
		},
		"short selector":                func(r map[string]any) { r["catalog"].(map[string]any)["catalog_selector"] = "g-0123.selector.json" },
		"malformed identity selector":   func(r map[string]any) { r["census_identity"].(map[string]any)["selector"] = "bad.selector.json" },
		"identity wrong type":           func(r map[string]any) { r["census_identity"] = "identity" },
		"malformed identity digest":     func(r map[string]any) { r["census_identity"].(map[string]any)["digest"] = "sha256:bad" },
		"zero identity length":          func(r map[string]any) { r["census_identity"].(map[string]any)["byte_length"] = 0 },
		"fractional identity length":    func(r map[string]any) { r["census_identity"].(map[string]any)["byte_length"] = 1.5 },
		"null census absent identity":   func(r map[string]any) { delete(r, "census_identity"); r["census"] = nil },
		"both census and identity":      func(r map[string]any) { r["census"] = map[string]any{} },
		"census absent identity valid":  func(r map[string]any) { delete(r, "census") },
		"valid census without identity": func(r map[string]any) { delete(r, "census_identity"); r["census"] = map[string]any{"Workers": 0} },
	} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(pausedResult)
			copyResult := map[string]any{}
			if err := json.Unmarshal(raw, &copyResult); err != nil {
				t.Fatal(err)
			}
			mutate(copyResult)
			candidate := map[string]any{"tool": "lsp_trace_v1_census", "envelope_schema_id": "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-feature-catalog-result.v2.schema.json", "outcome": "PAUSED", "operation_status": "SUCCEEDED", "isError": false, "result": copyResult}
			got, err := decodeDirectEnvelope(wrap(candidate))
			if err != nil {
				t.Fatal(err)
			}
			_, _, ok := liveRealCensusAdmission(got)
			if strings.Contains(name, "valid") {
				if !ok {
					t.Fatal("valid census variant rejected")
				}
			} else if ok {
				t.Fatal("malformed paused result admitted")
			}
		})
	}
	missingEnvelopeSchema := map[string]any{}
	for k, v := range paused {
		missingEnvelopeSchema[k] = v
	}
	delete(missingEnvelopeSchema, "envelope_schema_id")
	missingDecoded, err := decodeDirectEnvelope(wrap(missingEnvelopeSchema))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := liveRealCensusAdmission(missingDecoded); ok {
		t.Fatal("missing envelope schema admitted")
	}
	mismatch := wrap(acquisition)
	mismatch["result"].(map[string]any)["structuredContent"].(map[string]any)["outcome"] = "COMPLETE"
	if _, err := decodeDirectEnvelope(mismatch); err == nil {
		t.Fatal("wrapper parity mismatch accepted")
	}
}

func TestLiveRealFinalizerEOF(t *testing.T) {
	testLiveRealFinalizerProcess(t, "eof", "SERVE_EOF_BEFORE_CENSUS")
}
func TestLiveRealFinalizerNonzero(t *testing.T) {
	testLiveRealFinalizerProcess(t, "nonzero", "PROCESS_EXIT_NONZERO")
}
func TestLiveRealFinalizerSuccessfulPausedResponse(t *testing.T) {
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.Mkdir(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "trace")
	if err := os.WriteFile(trace, []byte("valid trace"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", "read line; exit 0")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	response := map[string]any{"operation_status": "SUCCEEDED", "outcome": "PAUSED", "isError": false, "stage": "DESCRIBE_REQUESTS", "result": map[string]any{"stage": "DESCRIBE_REQUESTS"}}
	result := runLiveRealAfterStart(cmd, context.Background(), in, bufio.NewReader(out), &stderr, func(liveRealSend) (liveRealPhaseResult, error) {
		return liveRealPhaseResult{response: response, semantic: "semantic"}, nil
	}, liveRealRunInput{evidenceDir: evidence, tracePath: trace, candidateDigest: "candidate", semanticFingerprint: "semantic"})
	if result.err != nil || result.failureClass != "SUCCESS" || result.failedField != "" || result.failedInvariant != "" {
		t.Fatalf("successful paused finalization: err=%v class=%q field=%q invariant=%q", result.err, result.failureClass, result.failedField, result.failedInvariant)
	}
	if result.terminal != "SUCCEEDED/PAUSED/DESCRIBE_REQUESTS" || result.response == nil {
		t.Fatalf("terminal/response not retained: terminal=%q response=%v", result.terminal, result.response)
	}
	for _, name := range []string{"summary.json", "public-response.json", "manifest.json"} {
		if info, err := os.Stat(filepath.Join(evidence, name)); err != nil || info.Size() == 0 {
			t.Fatalf("missing retained %s: %v", name, err)
		}
	}
	var summary map[string]any
	b, _ := os.ReadFile(filepath.Join(evidence, "summary.json"))
	if err := json.Unmarshal(b, &summary); err != nil || summary["failure_class"] != "SUCCESS" || summary["terminal"] != result.terminal || summary["worker_count"] != float64(0) || summary["model_invocation_count"] != float64(0) {
		t.Fatalf("bad retained summary: %v %s", err, b)
	}
}

func TestLiveRealFinalizerTimeout(t *testing.T) {
	testLiveRealFinalizerProcess(t, "timeout", "HARNESS_TIMEOUT")
}

func testLiveRealFinalizerProcess(t *testing.T, mode, wantClass string) {
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence")
	if err := os.Mkdir(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(dir, "trace")
	if err := os.WriteFile(trace, []byte("valid trace"), 0600); err != nil {
		t.Fatal(err)
	}
	script := "read line; "
	switch mode {
	case "eof":
		script += "exit 0"
	case "nonzero":
		script += "echo secret-stderr >&2; exit 7"
	default:
		script += "sleep 2"
	}
	cmd := exec.Command("/bin/sh", "-c", script)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	result := runLiveRealAfterStart(cmd, ctx, in, bufio.NewReader(out), &stderr, func(send liveRealSend) (liveRealPhaseResult, error) {
		_, err := send(1, "test", map[string]any{"secret_path": "/private/secret"})
		return liveRealPhaseResult{semantic: "test-semantic"}, err
	}, liveRealRunInput{evidenceDir: evidence, tracePath: trace, candidateDigest: "candidate", semanticFingerprint: "semantic"})
	if result.failureClass != wantClass {
		t.Fatalf("failure class %q, want %q", result.failureClass, wantClass)
	}
	for _, name := range []string{"summary.json", "manifest.json", "runtime.trace"} {
		info, err := os.Stat(filepath.Join(evidence, name))
		if err != nil || info.Mode().Perm() != 0600 || info.Size() == 0 {
			t.Fatalf("bad evidence %s: %v", name, err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(evidence, "summary.json"))
	if bytes.Contains(b, []byte("secret")) || bytes.Contains(b, []byte("/private")) || bytes.Contains(stderr.Bytes(), []byte("secret")) && mode != "nonzero" {
		t.Fatal("privacy leak")
	}
	if result.traceDigest == "" {
		t.Fatal("trace not retained")
	}
}

func TestLiveRealPublicResponsePersistence(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "evidence"), 0700); err != nil {
		t.Fatal(err)
	}
	response := map[string]any{"operation_status": "SUCCEEDED", "outcome": "PAUSED", "isError": false, "error": map[string]any{"code": "INVALID", "stage": "census"}}
	r := liveRealRunResult{response: response, failureClass: "REQUEST_ADMISSION_REJECTED", failedField: "catalog", failedInvariant: "CATALOG_CONTRACT"}
	raw := mustJSONNoFatal(response)
	r.publicResponseBytes, r.publicResponseDigest = string(raw), digestBytes(raw)
	in := liveRealRunInput{evidenceDir: filepath.Join(dir, "evidence"), candidateDigest: "candidate", semanticFingerprint: "semantic"}
	if err := persistLiveRealEvidence(in, r); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(in.evidenceDir, "public-response.json")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("public response custody: %v mode=%v", err, info.Mode().Perm())
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("public response bytes mismatch: %v", err)
	}
	if digestBytes(got) != r.publicResponseDigest {
		t.Fatalf("response digest mismatch: got=%s want=%s", digestBytes(got), r.publicResponseDigest)
	}
	var manifest map[string]string
	manifestRaw, _ := os.ReadFile(filepath.Join(in.evidenceDir, "manifest.json"))
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil || manifest["public-response.json"] != r.publicResponseDigest {
		t.Fatalf("manifest response digest mismatch: %v %v", err, manifest)
	}
	var summary map[string]any
	summaryRaw, _ := os.ReadFile(filepath.Join(in.evidenceDir, "summary.json"))
	if err := json.Unmarshal(summaryRaw, &summary); err != nil || summary["public_response_sha256"] != r.publicResponseDigest {
		t.Fatalf("summary response digest mismatch: %v %v", err, summary)
	}
}

func TestLiveRealInvariantFieldsClosed(t *testing.T) {
	for invariant, field := range map[string]string{"PAUSED_ENVELOPE": "envelope", "ENVELOPE_SCHEMA": "envelope", "RESULT_OBJECT": "result", "RESULT_SCHEMA": "result", "REQUEST_COUNT": "result", "CATALOG_CONTRACT": "catalog", "SELECTOR_GRAMMAR": "catalog", "SELECTOR_EQUALITY": "catalog", "PREPARATION_COUNT_BOUNDED": "catalog", "CENSUS_XOR_IDENTITY": "census_identity", "IDENTITY_BINDING": "census_identity"} {
		if got := liveRealCensusInvariantField(invariant); got != field {
			t.Fatalf("%s field=%s want=%s", invariant, got, field)
		}
	}
}

func TestLiveRealStagedRequestShape(t *testing.T) {
	v := liveRealTemplate{Sources: []string{"a"}, DownDepth: 1, UpDepth: 2, MaxNodes: 3, BatchTargets: 4, TimeoutMS: 5, RequestTimeoutMS: 6, Continuation: liveRealContinuation{Kind: "K", StopAfter: "S"}}
	b, err := json.Marshal(liveRealStagedRequest(v))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	c, ok := got["continuation"].(map[string]any)
	if !ok || c["kind"] != "K" || c["stop_after"] != "S" {
		t.Fatal("nested continuation missing")
	}
	if _, ok := got["stop_after"]; ok {
		t.Fatal("stop_after escaped top level")
	}
	for _, k := range []string{"sources", "down_depth", "up_depth", "max_nodes", "batch_targets", "timeout_ms", "request_timeout_ms", "continuation"} {
		if _, ok := got[k]; !ok {
			t.Fatalf("missing %s", k)
		}
	}
}

func liveRealClosedFailureClass(stderr string, exitCode int, phase string) string {
	if strings.Contains(stderr, "flag provided but not defined") || strings.Contains(stderr, "unknown flag") {
		return "STARTUP_FLAG_REJECTED"
	}
	if strings.HasPrefix(phase, "before-session-list") {
		return "SERVE_EOF_BEFORE_SESSION_LIST"
	}
	if strings.HasPrefix(phase, "before-census") {
		return "SERVE_EOF_BEFORE_CENSUS"
	}
	if exitCode != 0 {
		return "PROCESS_EXIT_NONZERO"
	}
	return "UNKNOWN_CLOSED"
}
