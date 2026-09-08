package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/strictjson"
)

const (
	privateRequestDiagnosticVersion  = "lsp-trace.private-request-diagnostic.v1"
	maxPrivateRequestDiagnosticBytes = 128 << 10
)

var (
	privateDigestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	privateOpaqueIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,256}$`)
)

type privateManagerIdentity struct {
	AttemptID  string `json:"attempt_id"`
	SessionID  string `json:"session_id"`
	Generation uint64 `json:"generation"`
}
type privateIO struct {
	State string `json:"state"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}
type privateDeadlineObservation struct {
	Source       string `json:"source"`
	Milliseconds int64  `json:"milliseconds,omitempty"`
	Kind         string `json:"kind,omitempty"`
}
type privateElapsed struct {
	Milliseconds int64 `json:"milliseconds"`
	Bounded      bool  `json:"bounded"`
}
type privateProcess struct {
	State    string `json:"state"`
	ExitCode int    `json:"exit_code,omitempty"`
}
type privateSelectedOperation struct {
	Method               string                       `json:"method"`
	DispatchAttempted    bool                         `json:"dispatch_attempted"`
	Write                privateIO                    `json:"write"`
	Read                 privateIO                    `json:"read"`
	Classification       string                       `json:"classification"`
	DeadlineObservations []privateDeadlineObservation `json:"deadline_observations"`
	Elapsed              privateElapsed               `json:"elapsed"`
	Process              privateProcess               `json:"process"`
	Sequence             uint64                       `json:"sequence"`
}
type privateRetention struct {
	RetainedBytes  int64  `json:"retained_bytes"`
	Fallback       bool   `json:"fallback"`
	OmittedRecords uint64 `json:"omitted_records"`
	Truncated      bool   `json:"truncated"`
	MaxRecords     int    `json:"max_records"`
	MaxBytes       int    `json:"max_bytes"`
}
type privateArtifactIntegrity struct {
	Length int64  `json:"length"`
	SHA256 string `json:"sha256"`
}
type privateRequestDiagnostic struct {
	SchemaVersion     string                   `json:"schema_version"`
	Manager           privateManagerIdentity   `json:"manager"`
	SelectedOperation privateSelectedOperation `json:"selected_operation"`
	Retention         privateRetention         `json:"retention"`
	Artifact          privateArtifactIntegrity `json:"artifact"`
}

func decodePrivateRequestDiagnostic(raw []byte) (privateRequestDiagnostic, error) {
	var d privateRequestDiagnostic
	if len(raw) == 0 || len(raw) > maxPrivateRequestDiagnosticBytes {
		return d, fmt.Errorf("private diagnostic byte limit")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return d, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return d, fmt.Errorf("one private diagnostic object required")
	}
	if err := validatePrivateRequestDiagnostic(d); err != nil {
		return d, err
	}
	return d, nil
}

func validatePrivateRequestDiagnostic(d privateRequestDiagnostic) error {
	if d.SchemaVersion != privateRequestDiagnosticVersion || !privateOpaqueIDPattern.MatchString(d.Manager.SessionID) || d.Manager.Generation == 0 || d.SelectedOperation.Sequence == 0 {
		return fmt.Errorf("private diagnostic identity invalid")
	}
	if len(d.Manager.AttemptID) != 64 {
		return fmt.Errorf("attempt identity format invalid")
	}
	for _, c := range d.Manager.AttemptID {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return fmt.Errorf("attempt identity format invalid")
		}
	}
	if d.SelectedOperation.Method != "textDocument/prepareCallHierarchy" && d.SelectedOperation.Method != "callHierarchy/incomingCalls" && d.SelectedOperation.Method != "callHierarchy/outgoingCalls" {
		return fmt.Errorf("private diagnostic method invalid")
	}
	if !d.SelectedOperation.DispatchAttempted {
		return fmt.Errorf("selected operation was not dispatched")
	}
	validIO := func(v privateIO) bool {
		return (v.State == "attempted" || v.State == "complete" || v.State == "failed" || v.State == "unavailable") && v.Count >= 0 && v.Bytes >= 0
	}
	if !validIO(d.SelectedOperation.Write) || !validIO(d.SelectedOperation.Read) {
		return fmt.Errorf("private diagnostic IO invalid")
	}
	if d.SelectedOperation.Classification != "TIMEOUT_OBSERVED" && d.SelectedOperation.Classification != "FAILED_AT_LIMIT_TIMING_CONSISTENT" && d.SelectedOperation.Classification != "FAILED_UNKNOWN" {
		return fmt.Errorf("private diagnostic classification invalid")
	}
	authoritative := false
	for _, observation := range d.SelectedOperation.DeadlineObservations {
		if observation.Source != "configured_limit" && observation.Source != "observed_terminal" {
			return fmt.Errorf("deadline source invalid")
		}
		if observation.Milliseconds < 0 || (observation.Kind != "" && observation.Kind != "deadline_exceeded" && observation.Kind != "unknown") {
			return fmt.Errorf("deadline observation invalid")
		}
		if observation.Source == "observed_terminal" && observation.Kind == "deadline_exceeded" {
			authoritative = true
		}
	}
	if d.SelectedOperation.Classification == "TIMEOUT_OBSERVED" && !authoritative {
		return fmt.Errorf("timeout classification lacks authoritative evidence")
	}
	if d.SelectedOperation.Elapsed.Milliseconds < 0 || !d.SelectedOperation.Elapsed.Bounded {
		return fmt.Errorf("elapsed metadata invalid")
	}
	if d.SelectedOperation.Process.State != "running" && d.SelectedOperation.Process.State != "exited" && d.SelectedOperation.Process.State != "unknown" {
		return fmt.Errorf("process state invalid")
	}
	if d.Retention.RetainedBytes < 0 || d.Retention.OmittedRecords < 0 || d.Retention.MaxRecords <= 0 || d.Retention.MaxBytes <= 0 || d.Artifact.Length < 0 || !privateDigestPattern.MatchString(d.Artifact.SHA256) {
		return fmt.Errorf("private diagnostic accounting invalid")
	}
	return nil
}

func classifyPrivateFailure(r manageddiagnostic.Record) string {
	if r.Terminal == manageddiagnostic.TerminalDeadlineExceeded {
		return "TIMEOUT_OBSERVED"
	}
	if r.Timing.Elapsed.Status == manageddiagnostic.Observed && r.Limits.EffectiveDeadlineNS.Status == manageddiagnostic.Observed && r.Limits.EffectiveDeadlineNS.Value > 0 && r.Timing.Elapsed.Value >= r.Limits.EffectiveDeadlineNS.Value {
		return "FAILED_AT_LIMIT_TIMING_CONSISTENT"
	}
	return "FAILED_UNKNOWN"
}

func projectPrivateRequestDiagnostic(attemptID, sessionID string, generation uint64, query manageddiagnostic.QueryResult, public []byte, maxRecords, maxBytes int) ([]byte, error) {
	var selected *manageddiagnostic.Record
	var retained int64
	for i := range query.Records {
		r := query.Records[i]
		encoded, _ := json.Marshal(r)
		retained += int64(len(encoded))
		if r.Phase == manageddiagnostic.PhaseRequestDispatch && r.Terminal != manageddiagnostic.TerminalResponseReceived {
			copy := r
			selected = &copy
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no failed request diagnostic retained")
	}
	r := *selected
	method := r.Request.Method.Value
	if method != "textDocument/prepareCallHierarchy" && method != "callHierarchy/incomingCalls" && method != "callHierarchy/outgoingCalls" {
		return nil, fmt.Errorf("failed request method is not exportable")
	}
	terminalKind := "unknown"
	if r.Terminal == manageddiagnostic.TerminalDeadlineExceeded {
		terminalKind = "deadline_exceeded"
	}
	limitMS := r.Limits.EffectiveDeadlineNS.Value / 1e6
	elapsedMS := r.Timing.Elapsed.Value / 1e6
	process := privateProcess{State: "unknown"}
	if r.ProcessExit.Status == manageddiagnostic.Observed {
		process.State = "exited"
		if r.ProcessExit.ExitCode.Status == manageddiagnostic.Observed {
			process.ExitCode = r.ProcessExit.ExitCode.Value
		}
	}
	sum := sha256.Sum256(public)
	d := privateRequestDiagnostic{SchemaVersion: privateRequestDiagnosticVersion, Manager: privateManagerIdentity{attemptID, sessionID, generation}, SelectedOperation: privateSelectedOperation{Method: method, DispatchAttempted: r.Write.State != manageddiagnostic.IOUnavailable, Write: privateIO{string(r.Write.State), r.Write.Messages, r.Write.Bytes}, Read: privateIO{string(r.Read.State), r.Read.Messages, r.Read.Bytes}, Classification: classifyPrivateFailure(r), DeadlineObservations: []privateDeadlineObservation{{Source: "configured_limit", Milliseconds: limitMS}, {Source: "observed_terminal", Kind: terminalKind}}, Elapsed: privateElapsed{Milliseconds: elapsedMS, Bounded: true}, Process: process, Sequence: r.Sequence}, Retention: privateRetention{RetainedBytes: retained, Fallback: false, OmittedRecords: query.EvictedRecords, Truncated: query.EvictedRecords > 0, MaxRecords: maxRecords, MaxBytes: maxBytes}, Artifact: privateArtifactIntegrity{Length: int64(len(public)), SHA256: fmt.Sprintf("sha256:%x", sum)}}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	if _, err := decodePrivateRequestDiagnostic(raw); err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func publishPrivateRequestDiagnostic(rootPath, selector string, raw []byte) error {
	if !filepath.IsAbs(rootPath) {
		return fmt.Errorf("absolute private diagnostic root required")
	}
	if _, err := safeStartupSelector(selector); err != nil {
		return fmt.Errorf("unsafe private diagnostic selector")
	}
	if _, err := decodePrivateRequestDiagnostic(bytes.TrimSpace(raw)); err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := writeRootFile(root, selector, raw); err != nil {
		return err
	}
	return syncRoot(root)
}

func runPrivateRequestDiagnosticValidation(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace validate-private-request-diagnostics PATH")
		return 1
	}
	raw, err := os.ReadFile(args[0])
	if err == nil {
		_, err = decodePrivateRequestDiagnostic(bytes.TrimSpace(raw))
	}
	if err != nil {
		fmt.Fprintln(stderr, "private request diagnostics invalid")
		return 1
	}
	fmt.Fprintln(stdout, "valid private request diagnostics")
	return 0
}
