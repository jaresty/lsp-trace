package censusresult

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/publication"
)

const SchemaVersion = "lsp-trace.census-result.v1"

const (
	DirectorySyncComplete               = "COMPLETE"
	DirectorySyncFailed                 = "FAILED"
	DirectorySyncNotAttemptedPostCommit = "NOT_ATTEMPTED_POST_COMMIT"
	CloseComplete                       = "COMPLETE"
	CloseFailed                         = "FAILED"
)

type Result struct {
	SchemaVersion          string             `json:"schema_version"`
	Status                 string             `json:"status"`
	CensusID               string             `json:"census_id"`
	CaptureSetID           string             `json:"capture_set_id"`
	SessionID              string             `json:"session_id"`
	Generation             uint64             `json:"generation"`
	TargetCount            int                `json:"target_count"`
	BatchCount             int                `json:"batch_count"`
	FileAccounting         FileAccounting     `json:"file_accounting"`
	SymbolAccounting       SymbolAccounting   `json:"symbol_accounting"`
	Authority              int                `json:"authority"`
	SourceGraphComplete    string             `json:"source_graph_complete"`
	NativeAggregateCustody bool               `json:"native_aggregate_custody"`
	CrossCaptureCalls      []string           `json:"cross_capture_calls"`
	LeidenAdmissible       bool               `json:"leiden_admissible"`
	Publication            PublicationReceipt `json:"publication"`
}

type FileAccounting struct {
	Denominator          int `json:"denominator"`
	Processed            int `json:"processed"`
	Excluded             int `json:"excluded"`
	Forbidden            int `json:"forbidden"`
	Unreadable           int `json:"unreadable"`
	Unsupported          int `json:"unsupported"`
	DocumentSymbolFailed int `json:"document_symbol_failed"`
	Omitted              int `json:"omitted"`
	Incomplete           int `json:"incomplete"`
}

type SymbolAccounting struct {
	Denominator       int `json:"denominator"`
	Prepared          int `json:"prepared"`
	Unsupported       int `json:"unsupported"`
	PreparationFailed int `json:"preparation_failed"`
	PrepareMissing    int `json:"prepare_missing"`
	NonCallable       int `json:"non_callable"`
	Omitted           int `json:"omitted"`
	Incomplete        int `json:"incomplete"`
}

type PublicationReceipt struct {
	Selector            string `json:"selector"`
	Digest              string `json:"digest"`
	ByteLength          uint64 `json:"byte_length"`
	VerificationStatus  string `json:"verification_status"`
	DirectorySyncStatus string `json:"directory_sync_status"`
	CloseStatus         string `json:"close_status"`
}

type PublicationEvidence struct {
	Selector, Digest, VerificationStatus, DirectorySyncStatus, CloseStatus string
	ByteLength                                                             uint64
}

func Build(p censusacquisition.Projection, receipt PublicationEvidence) (Result, error) {
	m := p.Manifest
	if err := validateLedger(m.FileLedger); err != nil {
		return Result{}, fmt.Errorf("file accounting: %w", err)
	}
	if err := validateLedger(m.SymbolLedger); err != nil {
		return Result{}, fmt.Errorf("symbol accounting: %w", err)
	}
	dir, err := normalizeDirectorySync(receipt.DirectorySyncStatus)
	if err != nil {
		return Result{}, err
	}
	closeStatus, err := normalizeClose(receipt.CloseStatus)
	if err != nil {
		return Result{}, err
	}
	r := Result{SchemaVersion: SchemaVersion, Status: "SUCCEEDED", CensusID: p.CensusID, CaptureSetID: m.LogicalDigest, SessionID: p.Session.SessionID, Generation: p.Session.Generation, TargetCount: len(m.Targets), BatchCount: len(m.Batches), FileAccounting: projectFiles(m.FileLedger), SymbolAccounting: projectSymbols(m.SymbolLedger), Authority: 0, SourceGraphComplete: "UNKNOWN", CrossCaptureCalls: []string{}, Publication: PublicationReceipt{Selector: receipt.Selector, Digest: receipt.Digest, ByteLength: receipt.ByteLength, VerificationStatus: receipt.VerificationStatus, DirectorySyncStatus: dir, CloseStatus: closeStatus}}
	if err := Validate(r); err != nil {
		return Result{}, err
	}
	return r, nil
}

func normalizeDirectorySync(s string) (string, error) {
	switch s {
	case publication.DirectorySyncComplete, DirectorySyncComplete:
		return DirectorySyncComplete, nil
	case publication.DirectorySyncFailed, DirectorySyncFailed:
		return DirectorySyncFailed, nil
	case publication.DirectorySyncNotAttemptedPostCommit:
		return DirectorySyncNotAttemptedPostCommit, nil
	}
	return "", errors.New("invalid census directory sync status")
}
func normalizeClose(s string) (string, error) {
	switch s {
	case publication.CloseComplete, CloseComplete:
		return CloseComplete, nil
	case publication.CloseFailed, CloseFailed:
		return CloseFailed, nil
	}
	return "", errors.New("invalid census close status")
}

var sha256Digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func Validate(r Result) error {
	if r.SchemaVersion != SchemaVersion || r.Status != "SUCCEEDED" || strings.TrimSpace(r.CensusID) == "" || strings.TrimSpace(r.CaptureSetID) == "" || strings.TrimSpace(r.SessionID) == "" || r.Generation == 0 {
		return errors.New("invalid census result identity")
	}
	if r.TargetCount < 1 || r.BatchCount < 1 || r.BatchCount != (r.TargetCount+62)/63 {
		return errors.New("invalid census target or batch accounting")
	}
	f := r.FileAccounting
	fc := []int{f.Processed, f.Excluded, f.Forbidden, f.Unreadable, f.Unsupported, f.DocumentSymbolFailed, f.Omitted, f.Incomplete}
	if f.Denominator < 0 || negative(fc) || sum(fc) != f.Denominator {
		return errors.New("file accounting does not reconcile")
	}
	s := r.SymbolAccounting
	sc := []int{s.Prepared, s.Unsupported, s.PreparationFailed, s.PrepareMissing, s.NonCallable, s.Omitted, s.Incomplete}
	if s.Denominator < 0 || negative(sc) || sum(sc) != s.Denominator {
		return errors.New("symbol accounting does not reconcile")
	}
	if r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.NativeAggregateCustody || r.CrossCaptureCalls == nil || len(r.CrossCaptureCalls) != 0 || r.LeidenAdmissible {
		return errors.New("invalid census authority ceiling")
	}
	p := r.Publication
	if !safeSelector(p.Selector) || !sha256Digest.MatchString(p.Digest) || p.ByteLength == 0 {
		return errors.New("invalid census publication identity")
	}
	if p.VerificationStatus != "VERIFIED" && p.VerificationStatus != "COMMITTED_VERIFICATION_FAILED" {
		return errors.New("invalid census verification status")
	}
	if p.DirectorySyncStatus != DirectorySyncComplete && p.DirectorySyncStatus != DirectorySyncFailed && p.DirectorySyncStatus != DirectorySyncNotAttemptedPostCommit {
		return errors.New("invalid census directory sync status")
	}
	if p.CloseStatus != CloseComplete && p.CloseStatus != CloseFailed {
		return errors.New("invalid census close status")
	}
	return nil
}
func Marshal(r Result) ([]byte, error) {
	if err := Validate(r); err != nil {
		return nil, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
func negative(v []int) bool {
	for _, n := range v {
		if n < 0 {
			return true
		}
	}
	return false
}
func sum(v []int) (n int) {
	for _, x := range v {
		n += x
	}
	return
}
func safeSelector(s string) bool {
	return s != "" && !filepath.IsAbs(s) && filepath.IsLocal(s) && path.Clean(s) == s && !strings.ContainsRune(s, 0) && !strings.Contains(s, `\`)
}
func validateLedger(l captureset.Ledger) error {
	if l.Denominator < 0 || len(l.Entries) != l.Denominator {
		return errors.New("denominator does not reconcile")
	}
	seen := make([]bool, l.Denominator)
	for _, e := range l.Entries {
		if e.Ordinal < 0 || e.Ordinal >= l.Denominator || seen[e.Ordinal] || e.Disposition == "" {
			return errors.New("invalid terminal disposition")
		}
		seen[e.Ordinal] = true
	}
	return nil
}
func projectFiles(l captureset.Ledger) (a FileAccounting) {
	a.Denominator = l.Denominator
	for _, e := range l.Entries {
		switch e.Disposition {
		case censusacquisition.FileProcessed:
			a.Processed++
		case string(census.FileExcluded):
			a.Excluded++
		case string(census.FileForbidden):
			a.Forbidden++
		case string(census.FileUnreadable):
			a.Unreadable++
		case string(census.FileUnsupported):
			a.Unsupported++
		case string(census.FileDocumentSymbolFailed):
			a.DocumentSymbolFailed++
		case string(census.FileOmitted):
			a.Omitted++
		case string(census.FileIncomplete):
			a.Incomplete++
		}
	}
	return
}
func projectSymbols(l captureset.Ledger) (a SymbolAccounting) {
	a.Denominator = l.Denominator
	for _, e := range l.Entries {
		switch e.Disposition {
		case censusacquisition.SymbolPrepared:
			a.Prepared++
		case string(census.SymbolUnsupported):
			a.Unsupported++
		case string(census.SymbolPreparationFailed):
			a.PreparationFailed++
		case string(census.SymbolPrepareMissing):
			a.PrepareMissing++
		case string(census.SymbolNonCallable):
			a.NonCallable++
		case string(census.SymbolOmitted):
			a.Omitted++
		case string(census.SymbolIncomplete):
			a.Incomplete++
		}
	}
	return
}
