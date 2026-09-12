// Package census defines unstable internal planning types for a future census
// integration. It performs no discovery, acquisition, session, or publication.
package census

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/discoveryfilter"
	"lsp-trace/internal/publication"
)

const (
	DefaultDownDepth      = 1
	DefaultUpDepth        = 0
	DefaultMaxNodes       = 10000
	DefaultTimeout        = 5 * time.Minute
	DefaultRequestTimeout = 30 * time.Second
	BatchPolicyIdentity   = "canonical-seed-v2-bytes+census-ordinal/maximal-63"
)

type OutputPreference string

const (
	OutputHuman OutputPreference = "human"
	OutputJSON  OutputPreference = "json"
)

type Limits struct {
	MaxNodes       int
	Timeout        time.Duration
	RequestTimeout time.Duration
}

type Config struct {
	SourceRoots     []string
	Workspace       string
	Includes        []string
	Excludes        []string
	DownDepth       int
	UpDepth         int
	Limits          Limits
	PublicationRoot string
	Output          OutputPreference
}

func DefaultConfig(workspace string) Config {
	return Config{
		SourceRoots: []string{"."},
		Workspace:   workspace,
		DownDepth:   DefaultDownDepth,
		UpDepth:     DefaultUpDepth,
		Limits:      Limits{MaxNodes: DefaultMaxNodes, Timeout: DefaultTimeout, RequestTimeout: DefaultRequestTimeout},
		Output:      OutputHuman,
	}
}

// ValidateConfig checks syntax and fixed bounds only. Source and workspace
// filesystem resolution belongs to the future discovery integration boundary.
// The publication root is different: it is required and opened now so callers
// receive a pinned private capability rather than ambient pathname authority.
func ValidateConfig(cfg Config) (*publication.Root, error) {
	if cfg.Workspace == "" || filepath.Clean(cfg.Workspace) != cfg.Workspace {
		return nil, errors.New("workspace must be non-empty and lexically clean")
	}
	if len(cfg.SourceRoots) == 0 {
		return nil, errors.New("at least one source root is required")
	}
	for _, source := range cfg.SourceRoots {
		if err := validateSourceSpelling(source); err != nil {
			return nil, err
		}
	}
	if err := discoveryfilter.ValidatePatterns(cfg.Includes, cfg.Excludes); err != nil {
		return nil, err
	}
	if cfg.DownDepth < 0 || cfg.UpDepth < 0 {
		return nil, errors.New("depths must be non-negative")
	}
	if cfg.Limits.MaxNodes < 0 || cfg.Limits.Timeout < 0 || cfg.Limits.RequestTimeout <= 0 {
		return nil, errors.New("invalid census limits")
	}
	if cfg.Output != OutputHuman && cfg.Output != OutputJSON {
		return nil, fmt.Errorf("unsupported census output preference %q", cfg.Output)
	}
	root, err := publication.OpenRoot(cfg.PublicationRoot)
	if err != nil {
		return nil, err
	}
	if err := root.ValidatePrivate(); err != nil {
		root.Close()
		return nil, err
	}
	return root, nil
}

func validateSourceSpelling(source string) error {
	if source == "" || filepath.IsAbs(source) || filepath.Clean(source) != source {
		return fmt.Errorf("source root %q must be a non-empty clean workspace-relative path", source)
	}
	if source != "." && (source == ".." || strings.HasPrefix(source, ".."+string(filepath.Separator))) {
		return fmt.Errorf("source root %q escapes the workspace", source)
	}
	return nil
}

type FileDisposition string

const (
	FileSelected             FileDisposition = "selected"
	FileExcluded             FileDisposition = "excluded"
	FileForbidden            FileDisposition = "forbidden"
	FileUnreadable           FileDisposition = "unreadable"
	FileUnsupported          FileDisposition = "unsupported"
	FileDocumentSymbolFailed FileDisposition = "document-symbol-failed"
	FileOmitted              FileDisposition = "omitted"
	FileIncomplete           FileDisposition = "incomplete"
)

type SymbolDisposition string

const (
	SymbolSelected          SymbolDisposition = "selected"
	SymbolUnsupported       SymbolDisposition = "unsupported"
	SymbolPreparationFailed SymbolDisposition = "preparation-failed"
	SymbolNonCallable       SymbolDisposition = "non-callable"
	SymbolOmitted           SymbolDisposition = "omitted"
	SymbolIncomplete        SymbolDisposition = "incomplete"
)

type FileEntry struct {
	Ordinal     int
	Disposition FileDisposition
}

type SymbolEntry struct {
	Ordinal     int
	Disposition SymbolDisposition
}

type Accounting struct {
	FileDenominator   int
	Files             []FileEntry
	SymbolDenominator int
	Symbols           []SymbolEntry
}

func (a Accounting) Validate() error {
	if err := validateClosed(a.FileDenominator, len(a.Files), func(i int) (int, bool) {
		e := a.Files[i]
		return e.Ordinal, validFileDisposition(e.Disposition)
	}); err != nil {
		return fmt.Errorf("file accounting: %w", err)
	}
	if err := validateClosed(a.SymbolDenominator, len(a.Symbols), func(i int) (int, bool) {
		e := a.Symbols[i]
		return e.Ordinal, validSymbolDisposition(e.Disposition)
	}); err != nil {
		return fmt.Errorf("symbol accounting: %w", err)
	}
	return nil
}

func validateClosed(denominator, count int, entry func(int) (int, bool)) error {
	if denominator < 0 || count != denominator {
		return errors.New("denominator does not reconcile")
	}
	seen := make([]bool, denominator)
	for i := 0; i < count; i++ {
		ordinal, valid := entry(i)
		if !valid || ordinal < 0 || ordinal >= denominator || seen[ordinal] {
			return errors.New("invalid or duplicate terminal disposition")
		}
		seen[ordinal] = true
	}
	return nil
}

func validFileDisposition(d FileDisposition) bool {
	switch d {
	case FileSelected, FileExcluded, FileForbidden, FileUnreadable, FileUnsupported, FileDocumentSymbolFailed, FileOmitted, FileIncomplete:
		return true
	default:
		return false
	}
}

func validSymbolDisposition(d SymbolDisposition) bool {
	switch d {
	case SymbolSelected, SymbolUnsupported, SymbolPreparationFailed, SymbolNonCallable, SymbolOmitted, SymbolIncomplete:
		return true
	default:
		return false
	}
}

type Batch struct {
	Ordinal int
	Targets []captureset.Target
}

type BatchPlan struct {
	PolicyIdentity string
	Batches        []Batch
}

func PlanTargets(targets []captureset.Target) (BatchPlan, error) {
	if err := captureset.ValidatePlanningTargets(targets); err != nil {
		return BatchPlan{}, err
	}
	ordered := captureset.OrderTargets(targets)
	assignments := captureset.PlanBatches(len(ordered))
	batches := make([]Batch, len(assignments))
	for i, assignment := range assignments {
		end := assignment.TargetStart + assignment.TargetCount
		batches[i] = Batch{Ordinal: assignment.Ordinal, Targets: append([]captureset.Target(nil), ordered[assignment.TargetStart:end]...)}
	}
	return BatchPlan{PolicyIdentity: BatchPolicyIdentity, Batches: batches}, nil
}
