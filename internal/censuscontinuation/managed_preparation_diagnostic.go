package censuscontinuation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
)

const (
	DefaultManagedPreparationDiagnosticMaxBytes   int64 = 1 << 20
	DefaultManagedPreparationDiagnosticMaxRecords       = 1000
)

type ManagedPreparationDiagnosticFileRecorder struct {
	path       string
	maxBytes   int64
	maxRecords int
	mu         sync.Mutex
}

func safeDiagnosticParent(path string) error {
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return errors.New("managed preparation diagnostic parent unavailable")
	}
	return nil
}

func NewManagedPreparationDiagnosticFileRecorder(path string, maxBytes int64, maxRecords int) (*ManagedPreparationDiagnosticFileRecorder, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || maxBytes <= 0 || maxBytes >= math.MaxInt64 || maxRecords <= 0 {
		return nil, errors.New("invalid managed preparation diagnostic recorder configuration")
	}
	if err := safeDiagnosticParent(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, errors.New("managed preparation diagnostic storage unavailable")
	}
	if err := f.Close(); err != nil {
		return nil, errors.New("managed preparation diagnostic storage unavailable")
	}
	if _, err := readDiagnosticLedger(path, maxBytes, maxRecords, true); err != nil {
		return nil, err
	}
	return &ManagedPreparationDiagnosticFileRecorder{path: path, maxBytes: maxBytes, maxRecords: maxRecords}, nil
}

func (r *ManagedPreparationDiagnosticFileRecorder) RecordManagedPreparationDiagnostic(record ManagedPreparationDiagnostic) error {
	if r == nil {
		return errors.New("managed preparation diagnostic recorder unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := safeDiagnosticParent(r.path); err != nil {
		return err
	}
	if err := validateManagedPreparationDiagnostic(record); err != nil {
		return err
	}
	existing, err := readDiagnosticLedger(r.path, r.maxBytes, r.maxRecords, true)
	if err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return errors.New("managed preparation diagnostic encoding failed")
	}
	line = append(line, '\n')
	if int64(len(existing)) > r.maxBytes || int64(len(line)) > r.maxBytes-int64(len(existing)) {
		return errors.New("managed preparation diagnostic byte limit")
	}
	if len(existing) > 0 && bytes.Count(existing, []byte{'\n'}) >= r.maxRecords {
		return errors.New("managed preparation diagnostic record limit")
	}
	before, err := os.Lstat(r.path)
	if err != nil || !before.Mode().IsRegular() || before.Mode().Perm() != 0o600 {
		return errors.New("managed preparation diagnostic storage unavailable")
	}
	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return errors.New("managed preparation diagnostic storage unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0o600 || !os.SameFile(before, opened) {
		return errors.New("managed preparation diagnostic storage unavailable")
	}
	if n, err := f.Write(line); err != nil || n != len(line) {
		return errors.New("managed preparation diagnostic storage unavailable")
	}
	if err := f.Sync(); err != nil {
		return errors.New("managed preparation diagnostic storage unavailable")
	}
	return nil
}

func boundedRegularFile(path string, maxBytes int64, allowEmpty bool) ([]byte, error) {
	if maxBytes <= 0 || maxBytes >= math.MaxInt64 {
		return nil, errors.New("managed preparation diagnostic unavailable")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || info.Size() > maxBytes || (!allowEmpty && info.Size() == 0) {
		return nil, errors.New("managed preparation diagnostic unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("managed preparation diagnostic unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0o600 || !os.SameFile(info, opened) {
		return nil, errors.New("managed preparation diagnostic unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil || int64(len(raw)) != info.Size() || int64(len(raw)) > maxBytes {
		return nil, errors.New("managed preparation diagnostic unavailable")
	}
	return raw, nil
}

func readDiagnosticLedger(path string, maxBytes int64, maxRecords int, allowEmpty bool) ([]byte, error) {
	raw, err := boundedRegularFile(path, maxBytes, allowEmpty)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return raw, nil
	}
	if raw[len(raw)-1] != '\n' {
		return nil, errors.New("managed preparation diagnostic ledger incomplete")
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	if len(lines) > maxRecords {
		return nil, errors.New("managed preparation diagnostic record limit")
	}
	for _, line := range lines {
		if _, err := ParseManagedPreparationDiagnosticLine(line); err != nil {
			return nil, errors.New("managed preparation diagnostic ledger invalid")
		}
	}
	return raw, nil
}

func ReadLastManagedPreparationDiagnostic(path string) (ManagedPreparationDiagnostic, error) {
	var zero ManagedPreparationDiagnostic
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return zero, errors.New("invalid managed preparation diagnostic path")
	}
	if err := safeDiagnosticParent(path); err != nil {
		return zero, err
	}
	raw, err := readDiagnosticLedger(path, DefaultManagedPreparationDiagnosticMaxBytes, DefaultManagedPreparationDiagnosticMaxRecords, false)
	if err != nil {
		return zero, err
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	return ParseManagedPreparationDiagnosticLine(lines[len(lines)-1])
}

func ParseManagedPreparationDiagnosticLine(line []byte) (ManagedPreparationDiagnostic, error) {
	var record ManagedPreparationDiagnostic
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, errors.New("invalid managed preparation diagnostic record")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return record, errors.New("invalid managed preparation diagnostic record")
	}
	canonical, err := json.Marshal(record)
	if err != nil || string(canonical) != string(line) {
		return record, errors.New("noncanonical managed preparation diagnostic record")
	}
	if err := validateManagedPreparationDiagnostic(record); err != nil {
		return record, err
	}
	return record, nil
}

func validateManagedPreparationDiagnostic(record ManagedPreparationDiagnostic) error {
	if !validManagedPreparationFailure(record.Failure) || record.Attempted < 1 || record.Succeeded < 0 || record.Succeeded >= record.Attempted || record.Planned < record.Attempted || record.FailingOrdinal < 0 || record.FailingOrdinal >= record.Planned || record.FailingOrdinal != record.Attempted-1 {
		return errors.New("managed preparation diagnostic integrity mismatch")
	}
	return nil
}

func validManagedPreparationFailure(f ManagedDocumentPreparationFailure) bool {
	switch f {
	case ManagedDocumentPreparationContextCancelled, ManagedDocumentPreparationContextDeadline, ManagedDocumentPreparationStaleGeneration, ManagedDocumentPreparationLifecycleConflict, ManagedDocumentPreparationResourceExhausted, ManagedDocumentPreparationSessionPoisoned, ManagedDocumentPreparationSupplyUnavailable, ManagedDocumentPreparationURIUnavailable, ManagedDocumentPreparationOutsideWorkspace, ManagedDocumentPreparationSourceUnavailable, ManagedDocumentPreparationLanguageIDUnavailable, ManagedDocumentPreparationSupplyMissing, ManagedDocumentPreparationUnknown:
		return true
	default:
		return false
	}
}

func (d ManagedPreparationDiagnostic) String() string {
	return fmt.Sprintf("failure=%s attempted=%d succeeded=%d planned=%d failing_ordinal=%d", d.Failure, d.Attempted, d.Succeeded, d.Planned, d.FailingOrdinal)
}
