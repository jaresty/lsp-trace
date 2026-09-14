package main

import (
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusresult"
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
	o := censusCLIOptions{Machine: machine}
	if state != machineFlagOK {
		return o, errors.New("invalid leading --machine grammar")
	}
	if len(clean) > 0 && clean[0] == "census" {
		clean = clean[1:]
	}
	o = censusCLIOptions{Sources: []string{"."}, DownDepth: census.DefaultDownDepth, UpDepth: census.DefaultUpDepth, MaxNodes: census.DefaultMaxNodes, Timeout: census.DefaultTimeout, RequestTimeout: census.DefaultRequestTimeout, Machine: machine}
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
	fs.BoolVar(&o.Help, "help", false, "show census help")
	fs.BoolVar(&o.Help, "h", false, "show census help")
	if err := fs.Parse(clean); err != nil {
		return o, errors.New("invalid census syntax")
	}
	if len(src) > 0 {
		o.Sources = []string(src)
	}
	o.Includes, o.Excludes, o.ServerArgs = []string(inc), []string(exc), []string(serverArgs)
	if fs.NArg() != 0 {
		return o, errors.New("census accepts no positional arguments")
	}
	if err := validateCensusCLIOptions(o); err != nil {
		return o, err
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
	if o.Server == "" && o.Profile == "" {
		return errors.New("server or profile is required")
	}
	if o.ConfigPath != "" && o.Profile == "" {
		return errors.New("config requires profile")
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
