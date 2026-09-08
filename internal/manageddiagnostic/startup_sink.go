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
type StartupDiagnosticSink struct{ Root, Selector string }
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
	selector, err := safeStartupSelector(s.Selector)
	if err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	rootInfo, err := os.Stat(s.Root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode().Perm()&0222 == 0 || rootInfo.Mode().Perm()&0077 != 0 {
		return StartupDiagnosticReceipt{Status: "omitted"}, errors.New("startup diagnostic root must be a private writable directory")
	}
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
	// RetainedBytes changes its own encoding length; converge within a tiny fixed bound.
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
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	defer root.Close()
	if dir := filepath.Dir(selector); dir != "." {
		if err := root.MkdirAll(dir, 0700); err != nil {
			return StartupDiagnosticReceipt{Status: "omitted"}, err
		}
	}
	var nonce [8]byte
	if _, err = io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	tmp := selector + ".tmp-" + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = root.Remove(tmp)
		}
	}()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	if err = root.Link(tmp, selector); err != nil {
		return StartupDiagnosticReceipt{Status: "omitted"}, err
	}
	ok = true
	_ = root.Remove(tmp)
	sum := sha256.Sum256(raw)
	return StartupDiagnosticReceipt{Status: "available", Digest: "sha256:" + hex.EncodeToString(sum[:]), Bytes: len(raw)}, nil
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
	if d.Accounting.RetainedBytes != len(raw) || d.Accounting.RetainedRecordCount != len(d.Records) || len(d.Records) > StartupDiagnosticsMaxRecords {
		return errors.New("startup diagnostic accounting mismatch")
	}
	if d.Attempt.Status == AttemptAvailable {
		if d.Attempt.AttemptID == "" || len(d.Attempt.AttemptID) != 64 {
			return errors.New("startup diagnostic attempt identity")
		}
		if d.Attempt.Outcome == StartupFailed && d.Attempt.Admission != nil {
			return errors.New("failed startup diagnostic admission")
		}
	} else if d.Attempt.Status != AttemptUnavailable && d.Attempt.Status != AttemptEvicted {
		return errors.New("startup diagnostic attempt status")
	}
	if d.RecordsStatus != "available" && d.RecordsStatus != "unavailable" && d.RecordsStatus != "evicted" && d.RecordsStatus != "omitted" {
		return errors.New("startup diagnostic records status")
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
