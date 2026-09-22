package captureset

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

const (
	DefaultFailureLedgerMaxBytes   int64 = 1 << 20
	DefaultFailureLedgerMaxRecords       = 1024
)

type FailureLedgerConfig struct {
	Path       string
	MaxBytes   int64
	MaxRecords int
}

type FailureLedgerRecord struct {
	Stage               string `json:"stage"`
	Reason              string `json:"reason"`
	CandidateSHA256     string `json:"candidate_sha256"`
	CandidateByteLength uint64 `json:"candidate_byte_length"`
	CodecCategory       string `json:"codec_category"`
	CodecLimit          uint64 `json:"codec_limit,omitempty"`
	RootSource          string `json:"root_source"`
	TargetExists        *bool  `json:"target_exists,omitempty"`
	TargetEqual         *bool  `json:"target_equal,omitempty"`
	TempBytesCommitted  bool   `json:"temp_bytes_committed"`
	FinalBytesCommitted bool   `json:"final_bytes_committed"`
}

type FailureLedger struct {
	mu         sync.Mutex
	path       string
	maxBytes   int64
	maxRecords int
	records    int
	write      func(FailureLedgerRecord) error
}

type FailureLedgerWriteFailure struct {
	Stage, Reason string
	cause         error
}

func (f *FailureLedgerWriteFailure) Error() string { return "failure ledger write failed" }
func (f *FailureLedgerWriteFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.cause
}

func OpenFailureLedger(config FailureLedgerConfig) (*FailureLedger, error) {
	if config.Path == "" || !filepath.IsAbs(config.Path) || filepath.Clean(config.Path) != config.Path {
		return nil, errors.New("failure ledger path invalid")
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = DefaultFailureLedgerMaxBytes
	}
	if config.MaxRecords <= 0 {
		config.MaxRecords = DefaultFailureLedgerMaxRecords
	}
	parent := filepath.Dir(config.Path)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return nil, errors.New("failure ledger parent must be owner-only directory")
	}
	f, err := os.OpenFile(config.Path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm() != 0o600 || fi.Size() > config.MaxBytes {
		return nil, errors.New("failure ledger file invalid")
	}
	raw, err := os.ReadFile(config.Path)
	if err != nil {
		return nil, err
	}
	records := bytes.Count(raw, []byte{'\n'})
	if records > config.MaxRecords {
		return nil, errors.New("failure ledger record cap exceeded")
	}
	l := &FailureLedger{path: config.Path, maxBytes: config.MaxBytes, maxRecords: config.MaxRecords, records: records}
	l.write = l.append
	return l, nil
}

var ledgerDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var ledgerStages = map[string]bool{"ROOT_OPEN": true, "PRIVATE_VALIDATION": true, "ENCODE": true, "CANONICALIZE": true, "TEMP_WRITE": true, "FSYNC": true, "NO_REPLACE": true, "VERIFY": true, "RECEIPT": true, "CLEANUP": true, "INTERNAL": true}
var ledgerReasons = map[string]bool{"INVALID_REQUEST": true, "PRIVATE_INVALID": true, "MANIFEST_REJECTED": true, "CONSTITUENT_CARDINALITY": true, "CONSTITUENT_REJECTED": true, "DUPLICATE_CONSTITUENT": true, "CONSTITUENT_ASSOCIATION": true, "CONSTITUENT_VERIFY": true, "BUNDLE_ENCODE": true, "CANONICALIZATION_FAILED": true, "STAT_FAILED": true, "EXISTS": true, "FAILED": true, "UNSUPPORTED": true, "NOT_EQUAL": true, "INJECTED_BEFORE_INSTALL": true}

func (l *FailureLedger) Record(f *PublicationFailure) error {
	if l == nil || f == nil {
		return nil
	}
	if !ledgerStages[f.Stage] || !ledgerReasons[f.Reason] || !ledgerDigestPattern.MatchString(f.CandidateSHA256) || f.CodecCategory != "CAPTURE_SET_V1" || f.RootSource != "HOST_PUBLICATION_ROOT" {
		return errors.New("failure ledger record invalid")
	}
	return l.write(FailureLedgerRecord{Stage: f.Stage, Reason: f.Reason, CandidateSHA256: f.CandidateSHA256, CandidateByteLength: f.CandidateByteLength, CodecCategory: f.CodecCategory, CodecLimit: f.CodecLimit, RootSource: f.RootSource, TargetExists: f.TargetExists, TargetEqual: f.TargetEqual, TempBytesCommitted: f.TempBytesCommitted, FinalBytesCommitted: f.FinalBytesCommitted})
}

func (l *FailureLedger) append(record FailureLedgerRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.records >= l.maxRecords {
		return errors.New("failure ledger record cap exceeded")
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	info, err := os.Lstat(l.path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size()+int64(len(raw)) > l.maxBytes {
		return errors.New("failure ledger byte cap exceeded")
	}
	f, err := os.OpenFile(l.path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		l.records++
	}
	return err
}
