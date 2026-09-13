package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/publication"
)

const (
	censusResultSchemaVersion     = "lsp-trace.census-result.v1"
	censusDiagnosticSchemaVersion = "lsp-trace.census-diagnostic.v1"
)

// censusCLIResult is the closed public projection of an already verified and
// published census. It deliberately has no source, provider, path, stderr, or
// native graph byte fields.
type censusCLIResult struct {
	SchemaVersion          string                      `json:"schema_version"`
	Status                 string                      `json:"status"`
	CensusID               string                      `json:"census_id"`
	CaptureSetID           string                      `json:"capture_set_id"`
	SessionID              string                      `json:"session_id"`
	Generation             uint64                      `json:"generation"`
	TargetCount            int                         `json:"target_count"`
	BatchCount             int                         `json:"batch_count"`
	FileAccounting         censusCLIFileAccounting     `json:"file_accounting"`
	SymbolAccounting       censusCLISymbolAccounting   `json:"symbol_accounting"`
	Authority              int                         `json:"authority"`
	SourceGraphComplete    string                      `json:"source_graph_complete"`
	NativeAggregateCustody bool                        `json:"native_aggregate_custody"`
	CrossCaptureCalls      []string                    `json:"cross_capture_calls"`
	LeidenAdmissible       bool                        `json:"leiden_admissible"`
	Publication            censusCLIPublicationReceipt `json:"publication"`
}

type censusCLIFileAccounting struct {
	Denominator          int `json:"denominator"`
	Selected             int `json:"selected"`
	Excluded             int `json:"excluded"`
	Forbidden            int `json:"forbidden"`
	Unreadable           int `json:"unreadable"`
	Unsupported          int `json:"unsupported"`
	DocumentSymbolFailed int `json:"document_symbol_failed"`
	Omitted              int `json:"omitted"`
	Incomplete           int `json:"incomplete"`
}

type censusCLISymbolAccounting struct {
	Denominator       int `json:"denominator"`
	Selected          int `json:"selected"`
	Unsupported       int `json:"unsupported"`
	PreparationFailed int `json:"preparation_failed"`
	PrepareMissing    int `json:"prepare_missing"`
	NonCallable       int `json:"non_callable"`
	Omitted           int `json:"omitted"`
	Incomplete        int `json:"incomplete"`
}

type censusCLIPublicationReceipt struct {
	Selector            string `json:"selector"`
	Digest              string `json:"digest"`
	ByteLength          uint64 `json:"byte_length"`
	VerificationStatus  string `json:"verification_status"`
	DirectorySyncStatus string `json:"directory_sync_status"`
	CloseStatus         string `json:"close_status"`
}

func buildCensusCLIResult(p censusacquisition.Projection, receipt publication.BoundFileReceipt) (censusCLIResult, error) {
	m := p.Manifest
	if err := validateCensusLedger(m.FileLedger); err != nil {
		return censusCLIResult{}, fmt.Errorf("file accounting: %w", err)
	}
	if err := validateCensusLedger(m.SymbolLedger); err != nil {
		return censusCLIResult{}, fmt.Errorf("symbol accounting: %w", err)
	}
	r := censusCLIResult{
		SchemaVersion: censusResultSchemaVersion, Status: "SUCCEEDED",
		CensusID: p.CensusID, CaptureSetID: m.LogicalDigest,
		SessionID: p.Session.SessionID, Generation: p.Session.Generation,
		TargetCount: len(m.Targets), BatchCount: len(m.Batches),
		FileAccounting: projectCensusFileAccounting(m.FileLedger), SymbolAccounting: projectCensusSymbolAccounting(m.SymbolLedger),
		Authority: 0, SourceGraphComplete: "UNKNOWN", NativeAggregateCustody: false,
		CrossCaptureCalls: []string{}, LeidenAdmissible: false,
		Publication: censusCLIPublicationReceipt{Selector: receipt.FinalSelector, Digest: receipt.Digest, ByteLength: receipt.ByteLength, VerificationStatus: receipt.VerificationStatus, DirectorySyncStatus: receipt.DirectorySyncStatus, CloseStatus: receipt.CloseStatus},
	}
	if err := validateCensusCLIResult(r); err != nil {
		return censusCLIResult{}, err
	}
	return r, nil
}

var sha256Digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validateCensusCLIResult(r censusCLIResult) error {
	if r.SchemaVersion != censusResultSchemaVersion || r.Status != "SUCCEEDED" || strings.TrimSpace(r.CensusID) == "" || strings.TrimSpace(r.CaptureSetID) == "" || strings.TrimSpace(r.SessionID) == "" || r.Generation == 0 {
		return errors.New("invalid census result identity")
	}
	if r.TargetCount < 1 || r.BatchCount < 1 || r.BatchCount != (r.TargetCount+62)/63 {
		return errors.New("invalid census target or batch accounting")
	}
	f := r.FileAccounting
	fileCounts := []int{f.Selected, f.Excluded, f.Forbidden, f.Unreadable, f.Unsupported, f.DocumentSymbolFailed, f.Omitted, f.Incomplete}
	if f.Denominator < 0 || hasNegativeCensusCount(fileCounts) || sumCensusCounts(fileCounts) != f.Denominator {
		return errors.New("file accounting does not reconcile")
	}
	s := r.SymbolAccounting
	symbolCounts := []int{s.Selected, s.Unsupported, s.PreparationFailed, s.PrepareMissing, s.NonCallable, s.Omitted, s.Incomplete}
	if s.Denominator < 0 || hasNegativeCensusCount(symbolCounts) || sumCensusCounts(symbolCounts) != s.Denominator {
		return errors.New("symbol accounting does not reconcile")
	}
	if r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.NativeAggregateCustody || r.CrossCaptureCalls == nil || len(r.CrossCaptureCalls) != 0 || r.LeidenAdmissible {
		return errors.New("invalid census authority ceiling")
	}
	p := r.Publication
	if !safeCensusSelector(p.Selector) || !sha256Digest.MatchString(p.Digest) || p.ByteLength == 0 {
		return errors.New("invalid census publication identity")
	}
	if p.VerificationStatus != "VERIFIED" && p.VerificationStatus != "COMMITTED_VERIFICATION_FAILED" {
		return errors.New("invalid census verification status")
	}
	if p.DirectorySyncStatus != publication.DirectorySyncComplete && p.DirectorySyncStatus != publication.DirectorySyncFailed && p.DirectorySyncStatus != publication.DirectorySyncNotAttemptedPostCommit {
		return errors.New("invalid census directory sync status")
	}
	if p.CloseStatus != publication.CloseComplete && p.CloseStatus != publication.CloseFailed {
		return errors.New("invalid census close status")
	}
	return nil
}

func hasNegativeCensusCount(counts []int) bool {
	for _, count := range counts {
		if count < 0 {
			return true
		}
	}
	return false
}
func sumCensusCounts(counts []int) (total int) {
	for _, count := range counts {
		total += count
	}
	return total
}

func projectCensusFileAccounting(l captureset.Ledger) (a censusCLIFileAccounting) {
	a.Denominator = l.Denominator
	for _, e := range l.Entries {
		switch e.Disposition {
		case "selected":
			a.Selected++
		case "excluded":
			a.Excluded++
		case "forbidden":
			a.Forbidden++
		case "unreadable":
			a.Unreadable++
		case "unsupported":
			a.Unsupported++
		case "document-symbol-failed":
			a.DocumentSymbolFailed++
		case "omitted":
			a.Omitted++
		case "incomplete":
			a.Incomplete++
		}
	}
	return
}
func projectCensusSymbolAccounting(l captureset.Ledger) (a censusCLISymbolAccounting) {
	a.Denominator = l.Denominator
	for _, e := range l.Entries {
		switch e.Disposition {
		case "selected":
			a.Selected++
		case "unsupported":
			a.Unsupported++
		case "preparation-failed":
			a.PreparationFailed++
		case "prepare-missing":
			a.PrepareMissing++
		case "non-callable":
			a.NonCallable++
		case "omitted":
			a.Omitted++
		case "incomplete":
			a.Incomplete++
		}
	}
	return
}

func validateCensusLedger(l captureset.Ledger) error {
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

func safeCensusSelector(s string) bool {
	return s != "" && !filepath.IsAbs(s) && filepath.IsLocal(s) && path.Clean(s) == s && !strings.ContainsRune(s, 0) && !strings.Contains(s, `\`)
}

func marshalCensusCLIResult(r censusCLIResult) ([]byte, error) {
	if err := validateCensusCLIResult(r); err != nil {
		return nil, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

type censusFailureStage string
type censusFailureCode string

const (
	censusStageSyntax           censusFailureStage = "syntax"
	censusStageConfig           censusFailureStage = "config"
	censusStageDiscovery        censusFailureStage = "discovery"
	censusStageAcquisition      censusFailureStage = "acquisition"
	censusStageAssembly         censusFailureStage = "assembly"
	censusStagePublication      censusFailureStage = "publication"
	censusStageCommitted        censusFailureStage = "committed-degradation"
	censusCodeInvalidSyntax     censusFailureCode  = "INVALID_SYNTAX"
	censusCodeInvalidConfig     censusFailureCode  = "INVALID_CONFIG"
	censusCodeDiscoveryFailed   censusFailureCode  = "DISCOVERY_FAILED"
	censusCodeAcquisitionFailed censusFailureCode  = "ACQUISITION_FAILED"
	censusCodeAssemblyFailed    censusFailureCode  = "ASSEMBLY_FAILED"
	censusCodePublicationFailed censusFailureCode  = "PUBLICATION_FAILED"
	censusCodeCommittedDegraded censusFailureCode  = "COMMITTED_DEGRADED"
)

type censusCLIDiagnostic struct {
	SchemaVersion string             `json:"schema_version"`
	Status        string             `json:"status"`
	Stage         censusFailureStage `json:"stage"`
	Code          censusFailureCode  `json:"code"`
	BatchOrdinal  *int               `json:"batch_ordinal,omitempty"`
	Retry         bool               `json:"retry"`
}

func buildCensusCLIDiagnostic(stage censusFailureStage, batchOrdinal *int) (censusCLIDiagnostic, error) {
	codes := map[censusFailureStage]censusFailureCode{censusStageSyntax: censusCodeInvalidSyntax, censusStageConfig: censusCodeInvalidConfig, censusStageDiscovery: censusCodeDiscoveryFailed, censusStageAcquisition: censusCodeAcquisitionFailed, censusStageAssembly: censusCodeAssemblyFailed, censusStagePublication: censusCodePublicationFailed, censusStageCommitted: censusCodeCommittedDegraded}
	code, ok := codes[stage]
	if !ok || batchOrdinal != nil && (*batchOrdinal < 0 || stage != censusStageAcquisition) {
		return censusCLIDiagnostic{}, errors.New("invalid census diagnostic")
	}
	d := censusCLIDiagnostic{SchemaVersion: censusDiagnosticSchemaVersion, Status: "FAILED", Stage: stage, Code: code, BatchOrdinal: batchOrdinal, Retry: stage != censusStageCommitted}
	if stage == censusStageCommitted {
		d.Status = "SUCCEEDED_DEGRADED"
	}
	return d, nil
}
func marshalCensusCLIDiagnostic(d censusCLIDiagnostic) ([]byte, error) {
	expected, err := buildCensusCLIDiagnostic(d.Stage, d.BatchOrdinal)
	if err != nil || d.SchemaVersion != expected.SchemaVersion || d.Status != expected.Status || d.Stage != expected.Stage || d.Code != expected.Code || d.Retry != expected.Retry {
		if err == nil {
			err = errors.New("invalid census diagnostic")
		}
		return nil, err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

type censusCLIOptions struct {
	Sources, Includes, Excludes                             []string
	Workspace, Server, Profile, ConfigPath, PublicationRoot string
	ServerArgs                                              []string
	DownDepth, UpDepth, MaxNodes                            int
	Timeout, RequestTimeout                                 time.Duration
	Machine, Help                                           bool
}
type censusStringFlags []string

func (s *censusStringFlags) String() string     { return strings.Join(*s, ",") }
func (s *censusStringFlags) Set(v string) error { *s = append(*s, v); return nil }

// parseCensusCLIOptions parses only syntax and closed preflight constraints. It
// performs no filesystem, environment, profile, session, or publication work.
func parseCensusCLIOptions(args []string) (censusCLIOptions, error) {
	clean, machine, state := extractMachineMode(args)
	if state != machineFlagOK {
		return censusCLIOptions{}, errors.New("invalid leading --machine grammar")
	}
	if len(clean) > 0 && clean[0] == "census" {
		clean = clean[1:]
	}
	o := censusCLIOptions{Sources: []string{"."}, DownDepth: census.DefaultDownDepth, UpDepth: census.DefaultUpDepth, MaxNodes: census.DefaultMaxNodes, Timeout: census.DefaultTimeout, RequestTimeout: census.DefaultRequestTimeout, Machine: machine}
	fs := flag.NewFlagSet("census", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var src, inc, exc, serverArgs censusStringFlags
	fs.Var(&src, "source", "workspace-relative source root")
	fs.Var(&inc, "include", "include pattern")
	fs.Var(&exc, "exclude", "exclude pattern")
	fs.StringVar(&o.Workspace, "workspace", "", "workspace")
	fs.StringVar(&o.Server, "server", "", "server")
	fs.Var(&serverArgs, "server-arg", "server argument")
	fs.StringVar(&o.Profile, "profile", "", "profile")
	fs.StringVar(&o.ConfigPath, "config", "", "config")
	fs.StringVar(&o.PublicationRoot, "publication-root", "", "private publication root")
	fs.IntVar(&o.DownDepth, "down-depth", o.DownDepth, "")
	fs.IntVar(&o.UpDepth, "up-depth", o.UpDepth, "")
	fs.IntVar(&o.MaxNodes, "max-nodes", o.MaxNodes, "")
	fs.DurationVar(&o.Timeout, "timeout", o.Timeout, "")
	fs.DurationVar(&o.RequestTimeout, "request-timeout", o.RequestTimeout, "")
	if err := fs.Parse(clean); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			o.Help = true
			return o, nil
		}
		return censusCLIOptions{}, errors.New("invalid census syntax")
	}
	if len(src) > 0 {
		o.Sources = []string(src)
	}
	o.Includes, o.Excludes, o.ServerArgs = []string(inc), []string(exc), []string(serverArgs)
	if fs.NArg() != 0 {
		return censusCLIOptions{}, errors.New("census accepts no positional arguments")
	}
	if err := validateCensusCLIOptions(o); err != nil {
		return censusCLIOptions{}, err
	}
	return o, nil
}
func validateCensusCLIOptions(o censusCLIOptions) error {
	if o.Help {
		return nil
	}
	if o.Workspace == "" || len(o.Sources) == 0 || o.PublicationRoot == "" {
		return errors.New("workspace, source, and publication root are required")
	}
	if (o.Server == "") == (o.Profile == "") {
		return errors.New("exactly one of server or profile is required")
	}
	if o.ConfigPath != "" && o.Profile == "" {
		return errors.New("config requires profile")
	}
	if o.Server == "" && len(o.ServerArgs) > 0 {
		return errors.New("server arguments require server")
	}
	if o.DownDepth < 0 || o.UpDepth < 0 || o.MaxNodes < 1 || o.MaxNodes > 10000 || o.Timeout <= 0 || o.RequestTimeout <= 0 || o.RequestTimeout > o.Timeout {
		return errors.New("invalid census bounds")
	}
	for _, s := range o.Sources {
		if s == "" || filepath.IsAbs(s) || !filepath.IsLocal(s) || filepath.Clean(s) != s {
			return errors.New("invalid source")
		}
	}
	return nil
}
