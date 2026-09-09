package manageddiagnostic

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const StartupDiagnosticsSchemaVersion = "managed-startup-diagnostics/v1"
const StartupDiagnosticsMaxRecords = 64
const StartupDiagnosticsMaxBytes = 65536

// StartupDiagnosticSink is a private CLI-owned destination. Root is caller-
// approved; Selector is interpreted only beneath Root.
type StartupDiagnosticSink struct {
	Root, Selector string
	// afterRootLstat is a package-private deterministic race seam used only by tests.
	afterRootLstat func()
}
type StartupDiagnosticReceipt struct {
	Status string `json:"status"`
	Digest string `json:"digest,omitempty"`
	Bytes  int    `json:"bytes,omitempty"`
}

type startupDiagnosticDocument struct {
	SchemaVersion string                    `json:"schema_version"`
	Attempt       startupAttemptProjection  `json:"attempt"`
	RecordsStatus string                    `json:"records_status"`
	Records       []startupRecordProjection `json:"records"`
	Accounting    startupAccounting         `json:"accounting"`
}
type startupAttemptProjection struct {
	Status      AttemptQueryStatus `json:"status"`
	AttemptID   StartupAttemptID   `json:"attempt_id,omitempty"`
	Outcome     StartupOutcome     `json:"outcome,omitempty"`
	Reason      string             `json:"reason,omitempty"`
	Admission   *StartupAdmission  `json:"admission,omitempty"`
	Stderr      Stderr             `json:"stderr"`
	ProcessExit ProcessExit        `json:"process_exit"`
}
type startupRecordProjection struct {
	Sequence    uint64      `json:"sequence"`
	Phase       Phase       `json:"phase"`
	Terminal    Terminal    `json:"terminal"`
	Read        IOFacts     `json:"read"`
	Write       IOFacts     `json:"write"`
	Limits      Limits      `json:"limits"`
	Stderr      Stderr      `json:"stderr"`
	ProcessExit ProcessExit `json:"process_exit"`
}
type startupAccounting struct {
	RequestedRecordLimit int    `json:"requested_record_limit"`
	EffectiveRecordLimit int    `json:"effective_record_limit"`
	RequestedByteLimit   int    `json:"requested_byte_limit"`
	EffectiveByteLimit   int    `json:"effective_byte_limit"`
	RetainedRecordCount  int    `json:"retained_record_count"`
	RetainedBytes        int    `json:"retained_bytes"`
	EvictedRecordCount   uint64 `json:"evicted_record_count"`
	OmittedRecordCount   int    `json:"omitted_record_count"`
	Truncated            bool   `json:"truncated"`
}

func (s StartupDiagnosticSink) Finalize(attempt StartupAttemptQuery, generation QueryResult) (StartupDiagnosticReceipt, error) {
	doc, err := projectStartupDiagnostics(attempt, generation)
	if err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	raw = append(raw, '\n')
	if len(raw) > StartupDiagnosticsMaxBytes {
		return StartupDiagnosticReceipt{Status: "omitted"}, errors.New("startup diagnostic byte limit exceeded")
	}
	doc.Accounting.RetainedBytes = len(raw)
	for i := 0; i < 4; i++ {
		raw, _ = json.Marshal(doc)
		raw = append(raw, '\n')
		if doc.Accounting.RetainedBytes == len(raw) {
			break
		}
		doc.Accounting.RetainedBytes = len(raw)
	}
	if len(raw) > StartupDiagnosticsMaxBytes || doc.Accounting.RetainedBytes != len(raw) {
		return StartupDiagnosticReceipt{Status: "omitted"}, errors.New("startup diagnostic exact byte accounting failed")
	}
	if err := ValidateStartupDiagnostics(raw); err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	if err := hardenedPublish(s.Root, s.Selector, raw, ValidateStartupDiagnostics, s.afterRootLstat); err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	sum := sha256.Sum256(raw)
	return StartupDiagnosticReceipt{Status: "available", Digest: "sha256:" + hex.EncodeToString(sum[:]), Bytes: len(raw)}, nil
}

// PublishHardened writes already-finalized private diagnostic bytes using the
// same fail-closed publication path as StartupDiagnosticSink.
func PublishHardened(root, selector string, raw []byte, validate func([]byte) error) error {
	return hardenedPublish(root, selector, raw, validate, nil)
}

func hardenedPublish(rootPath, selector string, raw []byte, validate func([]byte) error, afterRootLstat func()) error {
	selector, err := safeStartupSelector(selector)
	if err != nil {
		return err
	}
	if validate == nil {
		return errors.New("private diagnostic validator required")
	}
	if err := validate(raw); err != nil {
		return err
	}
	pathInfo, err := os.Lstat(rootPath)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.IsDir() {
		return errors.New("diagnostic root must be a private writable directory")
	}
	if afterRootLstat != nil {
		afterRootLstat()
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	opened, err := root.Open(".")
	if err != nil {
		return errors.New("diagnostic root unavailable")
	}
	openedInfo, statErr := opened.Stat()
	closeErr := opened.Close()
	if statErr != nil || closeErr != nil || !openedInfo.IsDir() || !os.SameFile(pathInfo, openedInfo) || openedInfo.Mode().Perm()&0222 == 0 || openedInfo.Mode().Perm()&0077 != 0 {
		return errors.New("diagnostic root must be a private writable directory")
	}
	if dir := filepath.Dir(selector); dir != "." {
		if err := root.MkdirAll(dir, 0700); err != nil {
			return err
		}
		// Reject symlinks in every existing nested component.
		parts := strings.Split(dir, string(filepath.Separator))
		for i := range parts {
			component := filepath.Join(parts[:i+1]...)
			info, err := root.Lstat(component)
			if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
				return errors.New("diagnostic selector parent must be a private directory")
			}
		}
	}
	var nonce [8]byte
	if _, err = io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return err
	}
	tmp := selector + ".tmp-" + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			_ = root.Remove(tmp)
		}
	}()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr = f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	check, err := root.Open(tmp)
	if err != nil {
		return err
	}
	stored, readErr := io.ReadAll(check)
	info, statErr := check.Stat()
	closeErr = check.Close()
	if readErr != nil || statErr != nil || closeErr != nil || info.Mode().Perm() != 0600 || info.Mode().IsRegular() == false || info.Sys() == nil || !bytes.Equal(stored, raw) {
		return errors.New("diagnostic temporary file validation failed")
	}
	if err := validate(stored); err != nil {
		return err
	}
	if err = root.Link(tmp, selector); err != nil {
		return err
	}
	published = true
	if err = root.Remove(tmp); err != nil {
		return err
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	err = dir.Sync()
	closeErr = dir.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func safeStartupSelector(selector string) (string, error) {
	if selector == "" || filepath.IsAbs(selector) || filepath.Clean(selector) != selector || selector == "." || strings.HasPrefix(selector, ".."+string(filepath.Separator)) {
		return "", errors.New("unsafe startup diagnostic selector")
	}
	return selector, nil
}

func projectStartupDiagnostics(a StartupAttemptQuery, q QueryResult) (startupDiagnosticDocument, error) {
	d := startupDiagnosticDocument{SchemaVersion: StartupDiagnosticsSchemaVersion, Records: []startupRecordProjection{}, Accounting: startupAccounting{RequestedRecordLimit: StartupDiagnosticsMaxRecords, EffectiveRecordLimit: StartupDiagnosticsMaxRecords, RequestedByteLimit: StartupDiagnosticsMaxBytes, EffectiveByteLimit: StartupDiagnosticsMaxBytes}}
	d.Attempt.Status = a.Status
	if a.Status == "" {
		d.Attempt.Status = AttemptUnavailable
	}
	if a.Status == AttemptAvailable {
		if a.Record == nil || ValidateStartupAttempt(*a.Record) != nil {
			return d, errors.New("invalid startup attempt")
		}
		r := a.Record
		d.Attempt.AttemptID = r.AttemptID
		d.Attempt.Outcome = r.Outcome
		d.Attempt.Reason = r.Reason.Value
		d.Attempt.Admission = r.Admission
		d.Attempt.Stderr = r.Stderr
		d.Attempt.ProcessExit = r.ProcessExit
	}
	switch q.Status {
	case QueryAvailable, QueryUnavailable, QueryEvicted:
		d.RecordsStatus = string(q.Status)
	case "":
		d.RecordsStatus = "omitted"
	default:
		return d, errors.New("invalid diagnostic availability")
	}
	limit := len(q.Records)
	if limit > StartupDiagnosticsMaxRecords {
		d.Accounting.OmittedRecordCount = limit - StartupDiagnosticsMaxRecords
		limit = StartupDiagnosticsMaxRecords
		d.Accounting.Truncated = true
	}
	for _, r := range q.Records[:limit] {
		if Validate(r) != nil {
			return d, errors.New("invalid retained diagnostic record")
		}
		d.Records = append(d.Records, startupRecordProjection{r.Sequence, r.Phase, r.Terminal, r.Read, r.Write, r.Limits, r.Stderr, r.ProcessExit})
	}
	d.Accounting.RetainedRecordCount = len(d.Records)
	d.Accounting.EvictedRecordCount = q.EvictedRecords
	return d, nil
}

func validateProjectedAttempt(a startupAttemptProjection) error {
	switch a.Status {
	case AttemptAvailable:
		idRaw, err := hex.DecodeString(string(a.AttemptID))
		if err != nil || len(idRaw) != sha256.Size || string(a.AttemptID) != strings.ToLower(string(a.AttemptID)) {
			return errors.New("startup diagnostic attempt identity")
		}
		reason := Fact[string]{}
		if a.Reason != "" {
			reason = Fact[string]{Status: Observed, Value: a.Reason}
		}
		r := StartupAttemptRecord{SchemaVersion: StartupAttemptSchemaVersion, AttemptID: a.AttemptID, Sequence: 1, Outcome: a.Outcome, Reason: reason, Stderr: a.Stderr, ProcessExit: a.ProcessExit, Admission: a.Admission}
		if err := ValidateStartupAttempt(r); err != nil {
			return fmt.Errorf("startup diagnostic attempt: %w", err)
		}
	case AttemptUnavailable, AttemptEvicted, AttemptQueryStatus("omitted"):
		if a.AttemptID != "" || a.Outcome != "" || a.Reason != "" || a.Admission != nil || a.Stderr != (Stderr{}) || a.ProcessExit != (ProcessExit{}) {
			return errors.New("unavailable startup diagnostic attempt carries detail")
		}
	default:
		return errors.New("startup diagnostic attempt status")
	}
	return nil
}

func validateProjectedRecord(p startupRecordProjection) error {
	if p.Sequence == 0 {
		return errors.New("startup diagnostic record sequence")
	}
	rule, ok := diagnosticPhaseRules[p.Phase]
	if !ok || !rule.terminals[p.Terminal] {
		return errors.New("startup diagnostic record phase or terminal")
	}
	if !validIO(p.Read) || !validIO(p.Write) || !validStatus(p.Stderr.Status) || !validFact(p.Stderr.ObservedByteCount) || !validFact(p.Stderr.Truncated) || !validStatus(p.ProcessExit.Status) || !validFact(p.ProcessExit.ExitCode) || !validFact(p.ProcessExit.ObservedBeforeCleanup) || !validFact(p.ProcessExit.CleanupInduced) ||
		!validFact(p.Limits.RequestedDeadlineNS) || !validFact(p.Limits.EffectiveDeadlineNS) || !validFact(p.Limits.RequestedMaxBytes) || !validFact(p.Limits.EffectiveMaxBytes) || !validFact(p.Limits.RequestedMaxMessages) || !validFact(p.Limits.EffectiveMaxMessages) {
		return errors.New("startup diagnostic record hidden or invalid fact")
	}
	if p.Stderr.ByteCap < 0 || p.Stderr.ObservedByteCount.Value < 0 || p.Limits.RequestedDeadlineNS.Value < 0 || p.Limits.EffectiveDeadlineNS.Value < 0 || p.Limits.RequestedMaxBytes.Value < 0 || p.Limits.EffectiveMaxBytes.Value < 0 || p.Limits.RequestedMaxMessages.Value < 0 || p.Limits.EffectiveMaxMessages.Value < 0 {
		return errors.New("startup diagnostic record negative value")
	}
	if p.Stderr.Status != Observed && (p.Stderr.ByteCap != 0 || p.Stderr.ObservedByteCount.Status == Observed || p.Stderr.Truncated.Status == Observed) {
		return errors.New("startup diagnostic record unavailable stderr carries detail")
	}
	if p.ProcessExit.Status != Observed && (p.ProcessExit.ExitCode.Status == Observed || p.ProcessExit.ObservedBeforeCleanup.Status == Observed || p.ProcessExit.CleanupInduced.Status == Observed) {
		return errors.New("startup diagnostic record unavailable process exit carries detail")
	}
	if p.Terminal == TerminalProcessExited {
		if p.ProcessExit.Status != Observed || p.ProcessExit.ObservedBeforeCleanup.Status != Observed {
			return errors.New("startup diagnostic record process exit chronology")
		}
	} else if p.ProcessExit.Status == Observed {
		return errors.New("startup diagnostic record process exit mismatch")
	}
	if (p.Limits.RequestedDeadlineNS.Status == Observed && p.Limits.EffectiveDeadlineNS.Status != Observed) || (p.Limits.RequestedMaxBytes.Status == Observed && p.Limits.EffectiveMaxBytes.Status != Observed) || (p.Limits.RequestedMaxMessages.Status == Observed && p.Limits.EffectiveMaxMessages.Status != Observed) {
		return errors.New("startup diagnostic record effective limit missing")
	}
	matched := p.Terminal == TerminalResponseReceived || (rule.matchedResponse && p.Terminal == TerminalProtocolError)
	if matched && rule.io != "none" && rule.io != "write-only" && (!completedMessages(p.Write, 1) || !completedMessages(p.Read, 1)) {
		return errors.New("startup diagnostic record matched response IO")
	}
	switch rule.io {
	case "none":
		if p.Read.State != IOUnavailable || p.Write.State != IOUnavailable {
			return errors.New("startup diagnostic record unexpected IO")
		}
	case "write-only":
		if p.Read.State != IOUnavailable || p.Write.Messages < 1 || (p.Write.State != IOComplete && p.Write.State != IOFailed) {
			return errors.New("startup diagnostic record write IO")
		}
	}
	return nil
}

func ValidateStartupDiagnostics(raw []byte) error {
	if len(raw) == 0 || len(raw) > StartupDiagnosticsMaxBytes {
		return errors.New("startup diagnostic byte limit")
	}
	var d startupDiagnosticDocument
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing startup diagnostic data")
	}
	if d.SchemaVersion != StartupDiagnosticsSchemaVersion {
		return errors.New("unknown startup diagnostic schema")
	}
	if d.Accounting.RequestedRecordLimit != StartupDiagnosticsMaxRecords || d.Accounting.EffectiveRecordLimit != StartupDiagnosticsMaxRecords || d.Accounting.RequestedByteLimit != StartupDiagnosticsMaxBytes || d.Accounting.EffectiveByteLimit != StartupDiagnosticsMaxBytes {
		return errors.New("startup diagnostic limits mismatch")
	}
	if d.Accounting.RetainedBytes != len(raw) || d.Accounting.RetainedRecordCount != len(d.Records) || len(d.Records) > StartupDiagnosticsMaxRecords || d.Accounting.OmittedRecordCount < 0 {
		return errors.New("startup diagnostic accounting mismatch")
	}
	if !validRetainedStrings(string(d.Attempt.Status), string(d.Attempt.AttemptID), string(d.Attempt.Outcome), d.Attempt.Reason, string(d.RecordsStatus)) {
		return errors.New("startup diagnostic retained string limit")
	}
	if err := validateProjectedAttempt(d.Attempt); err != nil {
		return err
	}
	switch d.RecordsStatus {
	case "available":
		if d.Accounting.Truncated != (d.Accounting.OmittedRecordCount > 0) {
			return errors.New("startup diagnostic truncation mismatch")
		}
	case "evicted":
		if len(d.Records) != 0 || d.Accounting.EvictedRecordCount == 0 || d.Accounting.OmittedRecordCount != 0 || d.Accounting.Truncated {
			return errors.New("startup diagnostic evicted records mismatch")
		}
	case "unavailable", "omitted":
		if len(d.Records) != 0 || d.Accounting.EvictedRecordCount != 0 || d.Accounting.OmittedRecordCount != 0 || d.Accounting.Truncated {
			return errors.New("startup diagnostic unavailable records carry detail")
		}
	default:
		return errors.New("startup diagnostic records status")
	}
	var previous uint64
	for _, r := range d.Records {
		if r.Sequence <= previous {
			return errors.New("startup diagnostic record sequence order")
		}
		previous = r.Sequence
		if !validRetainedStrings(string(r.Phase), string(r.Terminal), string(r.Read.State), string(r.Write.State), string(r.Stderr.Status), string(r.ProcessExit.Status)) {
			return errors.New("startup diagnostic retained string limit")
		}
		if err := validateProjectedRecord(r); err != nil {
			return err
		}
	}
	return nil
}
func VerifyStartupDiagnostics(raw []byte, digest string, length int) error {
	if length != len(raw) {
		return errors.New("startup diagnostic length mismatch")
	}
	sum := sha256.Sum256(raw)
	if digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return fmt.Errorf("startup diagnostic digest mismatch")
	}
	return ValidateStartupDiagnostics(raw)
}
