//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

type privatePrepareHostileError struct{}

func (privatePrepareHostileError) Error() string { panic("raw secret Error invoked") }
func (privatePrepareHostileError) Is(error) bool { panic("opaque Is invoked") }
func (privatePrepareHostileError) Unwrap() error { panic("opaque Unwrap invoked") }

type privatePrepareDiagnosticWriter struct {
	bytes.Buffer
	calls, attempted int
	mode             string
	before           func()
}

func (w *privatePrepareDiagnosticWriter) Write(p []byte) (int, error) {
	w.calls++
	w.attempted += len(p)
	if w.before != nil {
		w.before()
	}
	if w.mode == "short" {
		return len(p) - 1, nil
	}
	if w.mode == "error" {
		return 0, privatePrepareHostileError{}
	}
	return w.Buffer.Write(p)
}

func privatePrepareAssertFixedReport(t *testing.T, raw []byte) {
	t.Helper()
	reason := "(?:NONE|DEADLINE|CANCELED|REFUSED|UNAVAILABLE|EMPTY|UNJOINED|COMMIT_UNCERTAIN)"
	stage := "(?:NONE|RESERVE|REPLAY|WINDOW|START|READINESS|CAPTURE|FINISH|OBSERVE|RPREP|PERSIST|PACKET)"
	class := "(?:CONFIG_REFUSED|FRESH_ADMISSION_REFUSED|ROOT_REFUSED|CANONICAL_OWNER_REFUSED|PROFILE_REFUSED|STARTER_REFUSED|MANAGER_REFUSED|OUTPUT_FAILED_POSSIBLY_PARTIAL|REFUSED_OR_CONSUMED_PENDING_UNCERTAIN)"
	pattern := "^private Prepare: " + class + "\\nprivate Prepare diagnostic\\.v1 stage=" + stage + " reason=" + reason
	for _, name := range []string{" deferred_finish=", " shutdown=", " data_close=", " root_close=", " canceled=", " deadline="} {
		pattern += name + reason
	}
	pattern += " lease_boundary=(?:UNOBSERVED|NOT_APPLICABLE|OWNER_REJECTED|ADMISSION_REJECTED|CONTEXT_DONE|RESULT_REPLACED|RESULT_RETURNED|UNKNOWN)"
	pattern += " lease_result_observed=(?:YES|NO|UNKNOWN) inner_result=(?:UNOBSERVED|OK|KNOWN_UNJOINED|KNOWN_CONFLICT|CANCELED|DEADLINE|UNKNOWN)"
	pattern += " callback_sample=(?:UNOBSERVED|CALLER_POST_RETURN_LOCK|UNKNOWN) callback_entered=(?:YES|NO|UNKNOWN)"
	checkpoint := "(?:UNOBSERVED|OK|UNKNOWN|DEADLINE|CANCELED|KNOWN_UNJOINED|UNAVAILABLE|EMPTY|REFUSED|NOT_APPLICABLE)"
	pattern += " capture_result=(?:UNOBSERVED|OK|UNKNOWN|PRECHECK_REFUSED|RESOURCE_ENVELOPE_REFUSED|ENVELOPE_CONTEXT_REFUSED|ACTIVE_LEASE_REFUSED|METADATA_REFUSED|PREPARATION_UNKNOWN_FAILURE|PREPARATION_SUPPLY_INVALID|SOURCE_BOUND_REFUSED|TARGET_SELECTION_REFUSED|TARGET_BOUND_REFUSED|FINAL_LEASE_REFUSED|FINAL_BOUND_REFUSED|QUERY_GUARD_REFUSED|SYMBOL_CONTEXT_CANCELED|SYMBOL_CONTEXT_DEADLINE|SYMBOL_TOOL_NOT_IMPLEMENTED|SYMBOL_PROCESS_CONTAINMENT_UNAVAILABLE|SYMBOL_LIVE_LSP_DISABLED|SYMBOL_SESSION_NOT_FOUND|SYMBOL_LIST_CURSOR_INVALID|SYMBOL_STALE_GENERATION|SYMBOL_LIFECYCLE_CONFLICT|SYMBOL_SESSION_REAP_INCOMPLETE|SYMBOL_SESSION_STOPPING|SYMBOL_SESSION_POISONED|SYMBOL_RESOURCE_EXHAUSTED|SYMBOL_SPAWN_FAILED|SYMBOL_PIPE_SETUP_FAILED|SYMBOL_INITIALIZATION_FAILED|SYMBOL_INITIALIZATION_TIMEOUT|SYMBOL_SESSION_CRASHED|SYMBOL_REQUEST_CANCELLED|SYMBOL_REQUEST_TIMEOUT|SYMBOL_UNKNOWN_FAILURE|SYMBOL_SERVER_ERROR|SYMBOL_MISSING_PAIR|SYMBOL_INVALID_PAIR|RELATION_CONTEXT_CANCELED|RELATION_CONTEXT_DEADLINE|RELATION_TOOL_NOT_IMPLEMENTED|RELATION_PROCESS_CONTAINMENT_UNAVAILABLE|RELATION_LIVE_LSP_DISABLED|RELATION_SESSION_NOT_FOUND|RELATION_LIST_CURSOR_INVALID|RELATION_STALE_GENERATION|RELATION_LIFECYCLE_CONFLICT|RELATION_SESSION_REAP_INCOMPLETE|RELATION_SESSION_STOPPING|RELATION_SESSION_POISONED|RELATION_RESOURCE_EXHAUSTED|RELATION_SPAWN_FAILED|RELATION_PIPE_SETUP_FAILED|RELATION_INITIALIZATION_FAILED|RELATION_INITIALIZATION_TIMEOUT|RELATION_SESSION_CRASHED|RELATION_REQUEST_CANCELLED|RELATION_REQUEST_TIMEOUT|RELATION_UNKNOWN_FAILURE|RELATION_SERVER_ERROR|RELATION_MISSING_PAIR|RELATION_INVALID_PAIR|PREPARATION_TOOL_NOT_IMPLEMENTED|PREPARATION_PROCESS_CONTAINMENT_UNAVAILABLE|PREPARATION_LIVE_LSP_DISABLED|PREPARATION_SESSION_NOT_FOUND|PREPARATION_LIST_CURSOR_INVALID|PREPARATION_STALE_GENERATION|PREPARATION_LIFECYCLE_CONFLICT|PREPARATION_SESSION_REAP_INCOMPLETE|PREPARATION_SESSION_STOPPING|PREPARATION_SESSION_POISONED|PREPARATION_RESOURCE_EXHAUSTED|PREPARATION_SPAWN_FAILED|PREPARATION_PIPE_SETUP_FAILED|PREPARATION_INITIALIZATION_FAILED|PREPARATION_INITIALIZATION_TIMEOUT|PREPARATION_SESSION_CRASHED|PREPARATION_REQUEST_CANCELLED|PREPARATION_REQUEST_TIMEOUT|PREPARATION_DOCUMENT_SUPPLY_UNAVAILABLE|PREPARATION_LANGUAGE_ID_UNAVAILABLE|PREPARATION_DOCUMENT_URI_UNAVAILABLE|PREPARATION_DOCUMENT_OUTSIDE_WORKSPACE|PREPARATION_DOCUMENT_SOURCE_UNAVAILABLE|PREPARATION_WINDOW_UNJOINED)"
	for _, name := range []string{" issuance_result=", " calls_result="} {
		pattern += name + checkpoint
	}
	if !regexp.MustCompile(pattern + "\\n$").Match(raw) {
		t.Fatal("ASSERT_DIAG_REPORT_PRIVACY: FAIL non-allowlisted report")
	}
}

func TestPrivateSourcePrepareDescribeLeaseUnknownEnums(t *testing.T) {
	v := inv.PrivatePrepareLeaseObservationV1{Boundary: sessionruntime.PrivateLeaseBoundary(255), ResultObserved: inv.PrivatePrepareObservedV1(255), InnerResult: sessionruntime.PrivateLeaseResult(255), CallbackSample: inv.PrivatePrepareCallbackSampleV1(255), CallbackEntered: inv.PrivatePrepareObservedV1(255), CaptureResult: sessionruntime.PrivateGroupingCaptureResult(255), IssuanceResult: inv.PrivatePrepareCheckpointV1(255), CallsResult: inv.PrivatePrepareCheckpointV1(255)}
	raw := string(privatePrepareAppendLeaseReport(nil, v))
	if strings.Count(raw, "=UNKNOWN") != 8 || strings.Contains(raw, "255") || len(raw) > 4096 {
		t.Fatal("ASSERT_LEASE_UNKNOWN_ENUM_PRIVACY: FAIL", raw)
	}
	t.Log("ASSERT_LEASE_UNKNOWN_ENUM_PRIVACY: PASS")
}

func TestPrivateSourcePrepareDescribeLeaseObservationFields(t *testing.T) {
	t.Log("PROCEDURE: existing terminal formatter with no scoped lease sample; all attribution remains explicitly unobserved")
	writer := &privatePrepareDiagnosticWriter{}
	privatePrepareFailureReport(writer, "CONFIG_REFUSED", privatePrepareHostileError{})
	want := " lease_boundary=UNOBSERVED lease_result_observed=UNKNOWN inner_result=UNOBSERVED callback_sample=UNOBSERVED callback_entered=UNKNOWN capture_result=UNOBSERVED issuance_result=UNOBSERVED calls_result=UNOBSERVED\n"
	if !strings.HasSuffix(writer.String(), want) {
		t.Fatal("ASSERT_LEASE_FIELDS_DEFAULT: FAIL", writer.String())
	}
	t.Log("ASSERT_LEASE_FIELDS_DEFAULT: PASS")
}

func TestPrivateSourcePrepareDescribeCaptureVocabularyBound(t *testing.T) {
	// Exhaustive numeric domain; expected vocabulary is the independent literal
	// allowlist above, not a regex generated from production String output.
	for i := 0; i < 256; i++ {
		v := inv.PrivatePrepareLeaseObservationV1{CaptureResult: sessionruntime.PrivateGroupingCaptureResult(i)}
		raw := []byte("private Prepare: CONFIG_REFUSED\nprivate Prepare diagnostic.v1 stage=NONE reason=NONE deferred_finish=NONE shutdown=NONE data_close=NONE root_close=NONE canceled=NONE deadline=NONE")
		raw = privatePrepareAppendLeaseReport(raw, v)
		raw = append(raw, '\n')
		privatePrepareAssertFixedReport(t, raw)
	}
	maximum := func(value func(uint8) string) string {
		var longest string
		for i := 0; i < 256; i++ {
			s := value(uint8(i))
			if len(s) > len(longest) {
				longest = s
			}
		}
		return longest
	}
	stage := maximum(func(i uint8) string { return inv.PrivatePrepareStageV1(i).String() })
	reason := maximum(func(i uint8) string { return inv.PrivatePrepareReasonV1(i).String() })
	upper := "private Prepare: REFUSED_OR_CONSUMED_PENDING_UNCERTAIN\nprivate Prepare diagnostic.v1 stage=" + stage + " reason=" + reason
	for _, name := range []string{" deferred_finish=", " shutdown=", " data_close=", " root_close=", " canceled=", " deadline="} {
		upper += name + reason
	}
	fields := []struct {
		name  string
		value func(uint8) string
	}{
		{" lease_boundary=", func(i uint8) string { return sessionruntime.PrivateLeaseBoundary(i).String() }},
		{" lease_result_observed=", func(i uint8) string { return inv.PrivatePrepareObservedV1(i).String() }},
		{" inner_result=", func(i uint8) string { return sessionruntime.PrivateLeaseResult(i).String() }},
		{" callback_sample=", func(i uint8) string { return inv.PrivatePrepareCallbackSampleV1(i).String() }},
		{" callback_entered=", func(i uint8) string { return inv.PrivatePrepareObservedV1(i).String() }},
		{" capture_result=", func(i uint8) string { return sessionruntime.PrivateGroupingCaptureResult(i).String() }},
		{" issuance_result=", func(i uint8) string { return inv.PrivatePrepareCheckpointV1(i).String() }},
		{" calls_result=", func(i uint8) string { return inv.PrivatePrepareCheckpointV1(i).String() }},
	}
	for _, field := range fields {
		upper += field.name + maximum(field.value)
	}
	upper += "\n"
	privatePrepareAssertFixedReport(t, []byte(upper))
	if len(upper) != 567 || len(upper) > 4096 {
		t.Fatal("ASSERT_CAPTURE_REPORT_BOUND", len(upper))
	}
	t.Logf("ASSERT_CAPTURE_REPORT_BOUND: PASS exhaustive 256 values, 88 closed codes, %d-byte independent all-slot maximum; original one-Write/16-slot/256-step limits unchanged", len(upper))
}

func TestPrivateSourcePrepareDescribeDiagnosticBudget(t *testing.T) {
	t.Log("PROCEDURE: terminal failure reporting with hostile opaque cause and oversized graph; bounded one Write including short/error writers")
	for _, mode := range []string{"healthy-writer", "short", "error", "huge-graph", "output-short"} {
		t.Run(mode, func(t *testing.T) {
			writer := &privatePrepareDiagnosticWriter{mode: mode}
			var out bytes.Buffer
			var cause error = privatePrepareHostileError{}
			if mode == "huge-graph" {
				nodes := make([]error, 4096)
				for i := range nodes {
					nodes[i] = privatePrepareHostileError{}
				}
				cause = errors.Join(nodes...)
			}
			code := 1
			if mode == "output-short" {
				code = finishPrivateSourcePrepareDescribe(context.Background(), time.Now().Add(time.Second), inv.StagedPrepareDescribeV1{Packet: []byte("already-paid-packet")}, nil, readyShortWriter{}, writer)
			} else {
				code = finishPrivateSourcePrepareDescribe(context.Background(), time.Now().Add(time.Second), inv.StagedPrepareDescribeV1{}, cause, &out, writer)
			}
			if code != 1 || writer.calls != 1 || writer.attempted > 4096 || writer.attempted == 0 || out.Len() != 0 {
				t.Fatal("ASSERT_DIAG_REPORT_CAP: FAIL", mode, writer.calls, writer.attempted)
			}
			if mode != "short" && mode != "error" {
				privatePrepareAssertFixedReport(t, writer.Bytes())
				if !strings.Contains(writer.String(), "diagnostic.v1 stage=NONE") || strings.Contains(writer.String(), "secret") {
					t.Fatal("ASSERT_DIAG_REPORT_PRIVACY: FAIL")
				}
			}
		})
	}
	writer := &privatePrepareDiagnosticWriter{}
	privatePrepareFailureReport(writer, strings.Repeat("raw-secret", 1<<17), errors.New(strings.Repeat("raw-secret", 1<<17)))
	privatePrepareAssertFixedReport(t, writer.Bytes())
	if writer.calls != 1 || writer.attempted > 4096 || strings.Contains(writer.String(), "raw-secret") {
		t.Fatal("ASSERT_DIAG_REPORT_PRIVACY: FAIL large")
	}
	t.Log("ASSERT_DIAG_REPORT_CAP: PASS; ASSERT_DIAG_REPORT_PRIVACY: PASS")
}

func TestPrivateSourcePrepareDescribeDiagnosticEarlyCleanup(t *testing.T) {
	file, e := os.CreateTemp(t.TempDir(), "owned-early-data")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := &privatePrepareDiagnosticWriter{}
	writer.before = func() {
		if _, e := file.Stat(); e == nil {
			t.Fatal("ASSERT_DIAG_REPORT_AFTER_CLOSE: FAIL")
		}
	}
	var out bytes.Buffer
	code := executePrivateSourcePrepareDescribe(ctx, privateSourcePrepareDescribeConfig{}, &out, writer, file)
	if code != 1 || writer.calls != 1 || out.Len() != 0 || !strings.Contains(writer.String(), "canceled=CANCELED") {
		t.Fatal("ASSERT_DIAG_REPORT_AFTER_CLOSE: FAIL")
	}
	t.Log("ASSERT_DIAG_REPORT_AFTER_CLOSE: PASS")
}

func TestPrivateSourcePrepareDescribeConfig(t *testing.T) {
	c := privateSourcePrepareDescribeConfig{Schema: "private-source-prepare-describe.v1", AccountingRoot: "/tmp/private-accounting", CanonicalJob: []byte("unissued declaration"), ExpectedHead: "sha256:" + strings.Repeat("1", 64), Profile: runtimeprofile.Selector{Workspace: "/tmp/source"}, Workspace: "/tmp/source", ProcessPath: "/not-executed", PacketLimit: 1 << 20}
	c.Persist = inv.PreparedDescribePersistRequestV1{RootPath: c.AccountingRoot, InitialHead: c.ExpectedHead, Deadline: time.Now().Add(time.Second)}
	for _, fault := range []string{"valid", "HOST", "OutputRoots", "Timing", "borrowedgrant", "duplicate", "oversize", "root-mismatch", "trailing"} {
		t.Run(fault, func(t *testing.T) {
			q := c
			if fault == "root-mismatch" {
				q.Persist.RootPath = "/foreign"
			}
			raw, _ := json.Marshal(q)
			switch fault {
			case "HOST", "OutputRoots", "Timing", "borrowedgrant":
				raw = append([]byte("{\""+fault+"\":true,"), raw[1:]...)
			case "duplicate":
				raw = append([]byte("{\"Schema\":\"private-source-prepare-describe.v1\","), raw[1:]...)
			case "oversize":
				raw = bytes.Repeat([]byte(" "), 65537)
			case "trailing":
				raw = append(raw, []byte("{}")...)
			}
			_, e := decodePrivateSourcePrepareDescribeConfig(raw)
			if (e == nil) != (fault == "valid") {
				t.Fatal("ASSERT_PREPARE_CONFIG_STRICT", fault, e)
			}
		})
	}
}
func TestPrivateSourcePrepareDescribeDispatchRefused(t *testing.T) {
	var out, diag bytes.Buffer
	if runPrivateHostApplication([]string{"source-prepare-describe"}, nil, &out, &diag) == 0 || out.Len() != 0 || !strings.Contains(diag.String(), "CONFIG_REFUSED") {
		t.Fatal("ASSERT_PREPARE_DISPATCH", diag.String())
	}
}
func TestPrivateSourcePrepareDescribeConfigFileRefused(t *testing.T) {
	for _, fault := range []string{"mode", "symlink", "fifo", "oversize", "unknown-control"} {
		t.Run(fault, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config")
			switch fault {
			case "fifo":
				if e := syscall.Mkfifo(p, 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				target := p + ".target"
				if e := os.WriteFile(target, []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(target, p); e != nil {
					t.Fatal(e)
				}
			default:
				raw := []byte(`{"HOST":{"OutputRoots":["not-opened"]}}`)
				mode := os.FileMode(0600)
				if fault == "mode" {
					mode = 0644
				}
				if fault == "oversize" {
					raw = bytes.Repeat([]byte("x"), 65537)
				}
				if e := os.WriteFile(p, raw, mode); e != nil {
					t.Fatal(e)
				}
			}
			var out, diag bytes.Buffer
			if runPrivateSourcePrepareDescribe([]string{"source-prepare-describe", p}, &out, &diag) == 0 || out.Len() != 0 || !strings.Contains(diag.String(), "CONFIG_REFUSED") {
				t.Fatal("ASSERT_PREPARE_CONFIG_FILE_ZERO", fault, diag.String())
			}
		})
	}
}
func TestPrivateSourcePrepareDescribeOutputFailure(t *testing.T) {
	// Writer-only unit check, NOT positive native pipeline qualification.
	for _, writer := range []interface{ Write([]byte) (int, error) }{readyShortWriter{}, readyErrorWriter{}} {
		var diag bytes.Buffer
		if finishPrivateSourcePrepareDescribe(context.Background(), time.Now().Add(time.Second), inv.StagedPrepareDescribeV1{Packet: []byte("writer test")}, nil, writer, &diag) == 0 || !strings.Contains(diag.String(), "POSSIBLY_PARTIAL") {
			t.Fatal("ASSERT_PREPARE_PARTIAL_OUTPUT_FAILED")
		}
	}
}
