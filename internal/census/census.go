// Package census defines the pre-start planning boundary for the census command.
package census

import (
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/discoveryfilter"
	"lsp-trace/internal/publication"
)

const ResultVersion = "lsp-trace.census-plan.v1"

type OutputFormat string

const (
	OutputHuman OutputFormat = "human"
	OutputJSON  OutputFormat = "json"
)

type Config struct {
	WorkspaceRoot   string
	PublicationRoot string
	Includes        []string
	Excludes        []string
	Format          OutputFormat
}

type FileAccounting struct {
	Enumerated int `json:"enumerated"`
	Selected   int `json:"selected"`
	Excluded   int `json:"excluded"`
}

type SymbolAccounting struct {
	Enumerated int `json:"enumerated"`
	Selected   int `json:"selected"`
	Prepared   int `json:"prepared"`
}

type Accounting struct {
	Targets int              `json:"targets"`
	Batches int              `json:"batches"`
	Files   FileAccounting   `json:"files"`
	Symbols SymbolAccounting `json:"symbols"`
}

type Result struct {
	Status     string
	Accounting Accounting
	Batches    []Batch
}

type Batch struct {
	Ordinal int
	Targets []captureset.Target
}

// MachineOutput intentionally contains counts and fixed policy values only.
type MachineOutput struct {
	SchemaVersion   string     `json:"schema_version"`
	Status          string     `json:"status"`
	MaxBatchTargets int        `json:"max_batch_targets"`
	Accounting      Accounting `json:"accounting"`
}

// HumanOutput intentionally contains no paths, selectors, seeds, or source data.
type HumanOutput struct {
	Status          string
	MaxBatchTargets int
	TargetCount     int
	BatchCount      int
}

// Validate performs all checks and pins the publication root before later integration.
func Validate(cfg Config) (io.Closer, error) {
	if cfg.Format != OutputHuman && cfg.Format != OutputJSON {
		return nil, fmt.Errorf("unsupported census output format %q", cfg.Format)
	}
	if err := discoveryfilter.ValidatePatterns(cfg.Includes, cfg.Excludes); err != nil {
		return nil, err
	}
	root, err := publication.OpenRoot(cfg.PublicationRoot)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(cfg.PublicationRoot)
	if err != nil {
		root.Close()
		return nil, err
	}
	if info.Mode().Perm()&0o077 != 0 {
		root.Close()
		return nil, fmt.Errorf("publication root must be private (group/other permissions are forbidden)")
	}
	return root, nil
}

func BatchTargets(targets []captureset.Target) []Batch {
	ordered := captureset.OrderTargets(targets)
	plan := captureset.PlanBatches(len(ordered))
	batches := make([]Batch, len(plan))
	for i, batch := range plan {
		batches[i] = Batch{Ordinal: batch.Ordinal, Targets: append([]captureset.Target(nil), ordered[batch.TargetStart:batch.TargetStart+batch.TargetCount]...)}
	}
	return batches
}

func NewReadyResult(targets []captureset.Target) Result {
	batches := BatchTargets(targets)
	return Result{Status: "READY", Accounting: Accounting{Targets: len(targets), Batches: len(batches)}, Batches: batches}
}

func ProjectMachine(result Result) MachineOutput {
	return MachineOutput{SchemaVersion: ResultVersion, Status: result.Status, MaxBatchTargets: captureset.MaxBatchTargets, Accounting: result.Accounting}
}

func ProjectHuman(result Result) HumanOutput {
	return HumanOutput{Status: result.Status, MaxBatchTargets: captureset.MaxBatchTargets, TargetCount: result.Accounting.Targets, BatchCount: result.Accounting.Batches}
}
