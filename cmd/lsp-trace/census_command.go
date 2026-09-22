package main

import (
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/publication"
)

type censusCLIResult = censusresult.Result
type censusCLIFileAccounting = censusresult.FileAccounting
type censusCLISymbolAccounting = censusresult.SymbolAccounting
type censusCLIPublicationReceipt = censusresult.PublicationReceipt
type censusFailureStage = censusresult.Stage
type censusFailureCode = censusresult.Code
type censusCLIDiagnostic = censusresult.Diagnostic

const (
	censusResultSchemaVersion     = censusresult.SchemaVersion
	censusDiagnosticSchemaVersion = censusresult.DiagnosticSchemaVersion
	censusStageSyntax             = censusresult.StageSyntax
	censusStageConfig             = censusresult.StageConfig
	censusStageDiscovery          = censusresult.StageDiscovery
	censusStageAcquisition        = censusresult.StageAcquisition
	censusStageAssembly           = censusresult.StageAssembly
	censusStagePublication        = censusresult.StagePublication
	censusStageCommitted          = censusresult.StageCommitted
	censusCodeInvalidSyntax       = censusresult.CodeInvalidSyntax
	censusCodeInvalidConfig       = censusresult.CodeInvalidConfig
	censusCodeDiscoveryFailed     = censusresult.CodeDiscoveryFailed
	censusCodeAcquisitionFailed   = censusresult.CodeAcquisitionFailed
	censusCodeAssemblyFailed      = censusresult.CodeAssemblyFailed
	censusCodePublicationFailed   = censusresult.CodePublicationFailed
	censusCodeCommittedDegraded   = censusresult.CodeCommittedDegraded
)

func buildCensusCLIResult(p censusacquisition.Projection, receipt publication.BoundFileReceipt) (censusCLIResult, error) {
	return censusresult.Build(p, censusresult.PublicationEvidence{Selector: receipt.FinalSelector, Digest: receipt.Digest, ByteLength: receipt.ByteLength, VerificationStatus: receipt.VerificationStatus, DirectorySyncStatus: receipt.DirectorySyncStatus, CloseStatus: receipt.CloseStatus})
}
func validateCensusCLIResult(r censusCLIResult) error          { return censusresult.Validate(r) }
func marshalCensusCLIResult(r censusCLIResult) ([]byte, error) { return censusresult.Marshal(r) }
func buildCensusCLIDiagnostic(stage censusFailureStage, batchOrdinal *int) (censusCLIDiagnostic, error) {
	return censusresult.NewDiagnostic(stage, batchOrdinal)
}
func marshalCensusCLIDiagnostic(d censusCLIDiagnostic) ([]byte, error) {
	return censusresult.MarshalDiagnostic(d)
}

type censusCLIOptions struct {
	Sources, Includes, Excludes                                                []string
	Workspace, Server, Profile, ConfigPath, CatalogConfigPath, PublicationRoot string
	ServerArgs                                                                 []string
	DownDepth, UpDepth, MaxNodes, MaxBatchTargets                              int
	Timeout, RequestTimeout                                                    time.Duration
	Resume, StopAfter                                                          string
	Machine, Help, Catalog                                                     bool
}
type censusStringFlags []string

func (s *censusStringFlags) String() string     { return strings.Join(*s, ",") }
func (s *censusStringFlags) Set(v string) error { *s = append(*s, v); return nil }

// parseCensusCLIOptions parses only syntax and closed preflight constraints. It
// performs no filesystem, environment, profile, session, or publication work.
func parseCensusCLIOptions(args []string) (censusCLIOptions, error) {
	clean := args
	if len(clean) > 0 && clean[0] == "census" {
		clean = clean[1:]
	}
	machineCount := censusMachineFlagCount(clean)
	o := censusCLIOptions{Sources: censusrequest.DefaultSources(), DownDepth: int(censusrequest.DefaultDownDepth), UpDepth: int(censusrequest.DefaultUpDepth), MaxNodes: int(censusrequest.DefaultMaxNodes), MaxBatchTargets: int(censusrequest.DefaultBatchTargets), Timeout: time.Duration(censusrequest.DefaultTimeoutMS) * time.Millisecond, RequestTimeout: time.Duration(censusrequest.DefaultRequestTimeoutMS) * time.Millisecond}
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
	fs.StringVar(&o.ConfigPath, "config", "", "acquisition profile configuration")
	fs.StringVar(&o.CatalogConfigPath, "catalog-config", "", "host-owned catalog continuation configuration")
	fs.StringVar(&o.PublicationRoot, "publication-root", "", "private publication root")
	fs.BoolVar(&o.Catalog, "catalog", false, "continue to the provisional feature catalog")
	fs.StringVar(&o.Resume, "resume", "", "resume an opaque catalog continuation selector")
	fs.StringVar(&o.StopAfter, "stop-after", "", "stop catalog continuation after describe-requests")
	fs.IntVar(&o.DownDepth, "down-depth", o.DownDepth, "")
	fs.IntVar(&o.UpDepth, "up-depth", o.UpDepth, "")
	fs.IntVar(&o.MaxNodes, "max-nodes", o.MaxNodes, "graph traversal node limit")
	fs.IntVar(&o.MaxBatchTargets, "batch-targets", o.MaxBatchTargets, "maximum census targets per acquisition batch (1..63)")
	fs.DurationVar(&o.Timeout, "timeout", o.Timeout, "")
	fs.DurationVar(&o.RequestTimeout, "request-timeout", o.RequestTimeout, "")
	fs.BoolVar(&o.Machine, "machine", false, "emit machine-readable JSON")
	fs.BoolVar(&o.Help, "help", false, "show census help")
	fs.BoolVar(&o.Help, "h", false, "show census help")
	if err := fs.Parse(clean); err != nil {
		return o, errors.New("invalid census syntax")
	}
	if machineCount > 1 {
		return o, errors.New("duplicate machine flag")
	}
	if len(src) > 0 {
		o.Sources = []string(src)
	}
	o.Includes, o.Excludes, o.ServerArgs = []string(inc), []string(exc), []string(serverArgs)
	if fs.NArg() != 0 {
		return o, errors.New("census accepts no positional arguments")
	}
	if o.Resume != "" {
		o.Catalog = true
	} else if o.Catalog && !censusFlagPresent(clean, "batch-targets") {
		o.MaxBatchTargets = 16
	}
	if err := validateCensusCLIOptions(o, clean); err != nil {
		return o, err
	}
	return o, nil
}
func censusFlagPresent(args []string, wanted string) bool {
	for _, arg := range args {
		if strings.TrimPrefix(strings.SplitN(arg, "=", 2)[0], "--") == wanted {
			return true
		}
	}
	return false
}

func censusMachineFlagCount(args []string) int {
	count := 0
	for i := 0; i < len(args); i++ {
		if args[i] == "--server-arg" {
			i++
			continue
		}
		if args[i] == "--machine" || strings.HasPrefix(args[i], "--machine=") {
			count++
		}
	}
	return count
}

func validateCensusCLIOptions(o censusCLIOptions, args []string) error {
	if o.Help {
		return nil
	}
	if o.StopAfter != "" {
		if o.StopAfter != "describe-requests" {
			return errors.New("stop-after must be describe-requests")
		}
		if !o.Catalog && o.Resume == "" {
			return errors.New("stop-after requires catalog or resume")
		}
	}
	if o.Resume != "" {
		if !continuationhost.IsPublicDescriptorSelector(o.Resume) {
			return errors.New("resume requires a canonical opaque selector")
		}
		if o.CatalogConfigPath == "" || o.PublicationRoot == "" {
			return errors.New("resume requires catalog config and publication root")
		}
		forbidden := map[string]bool{"source": true, "include": true, "exclude": true, "server": true, "server-arg": true, "profile": true, "config": true, "down-depth": true, "up-depth": true, "max-nodes": true, "batch-targets": true, "timeout": true, "request-timeout": true}
		for _, arg := range args {
			name := strings.TrimPrefix(strings.SplitN(arg, "=", 2)[0], "--")
			if forbidden[name] {
				return errors.New("resume rejects census acquisition controls")
			}
		}
		return nil
	}
	if o.Workspace == "" || len(o.Sources) == 0 || o.PublicationRoot == "" {
		return errors.New("workspace, source, and publication root are required")
	}
	if o.Catalog && o.CatalogConfigPath == "" {
		return errors.New("catalog requires catalog config and publication root")
	}
	if !o.Catalog && o.CatalogConfigPath != "" {
		return errors.New("catalog config requires catalog or resume")
	}
	if o.Server == "" && o.Profile == "" {
		return errors.New("server or profile is required")
	}
	if o.ConfigPath != "" && o.Profile == "" {
		return errors.New("config requires profile")
	}
	if o.DownDepth < 0 || o.UpDepth < 0 || o.MaxNodes < 1 || o.MaxNodes > 10000 || o.MaxBatchTargets < 1 || o.MaxBatchTargets > 63 || o.Timeout <= 0 || o.RequestTimeout <= 0 || o.RequestTimeout > o.Timeout {
		return errors.New("invalid census bounds")
	}
	for _, s := range o.Sources {
		if s == "" || filepath.IsAbs(s) || !filepath.IsLocal(s) || filepath.Clean(s) != s {
			return errors.New("source must be a canonical workspace-relative path")
		}
	}
	return nil
}
