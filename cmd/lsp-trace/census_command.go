package main

import (
	"errors"
	"flag"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

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
	FolderInvocation                                                           bool // Normal inventory entry; runtime resolution owns all IO.
	Sources, Includes, Excludes, PrunedDirectories                             []string
	Workspace, Server, Profile, ConfigPath, CatalogConfigPath, PublicationRoot string
	PrivateAcquisitionDiagnosticPath, PrivateControlFlowDiagnosticPath         string
	PrivateProviderProbePath, PrivateProviderProbeVersion                      string
	PrivateInventoryConfigPath                                                 string
	PrivateInventoryQualifiedPrepareConfigPath                                 string
	PrivateInventoryBridgeConfigPath                                           string
	PrivateInventoryScopePreview                                               bool `json:",omitempty"`
	PrivateProviderProbeByteTimeNotifications                                  bool
	ServerArgs                                                                 []string
	DownDepth, UpDepth, MaxNodes, MaxBatchTargets                              int
	Timeout, RequestTimeout                                                    time.Duration
	Resume, StopAfter                                                          string
	Machine, Help, Catalog, PrivateEnumerationProjection, EnumerationPreflight bool
	CallerHost                                                                 bool
	CallerHostAttemptConfigPath, SaveCallerHostSettingsPath                    string
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
	o := censusCLIOptions{Sources: censusrequest.DefaultSources(), DownDepth: int(censusrequest.DefaultDownDepth), UpDepth: int(censusrequest.DefaultUpDepth), MaxNodes: int(censusrequest.DefaultMaxNodes), MaxBatchTargets: int(censusrequest.DefaultBatchTargets), Timeout: time.Duration(censusrequest.DefaultTimeoutMS) * time.Millisecond, RequestTimeout: time.Duration(censusrequest.DefaultRequestTimeoutMS) * time.Millisecond, PrivateProviderProbeVersion: "v1"}
	fs := flag.NewFlagSet("census", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var src, inc, exc, prune, serverArgs censusStringFlags
	fs.Var(&src, "source", "workspace-relative source root")
	fs.Var(&inc, "include", "include pattern")
	fs.Var(&exc, "exclude", "exclude pattern")
	fs.Var(&prune, "prune-dir", "canonical workspace-relative slash directory path to skip during enumeration")
	fs.StringVar(&o.Workspace, "workspace", "", "workspace")
	fs.StringVar(&o.Server, "server", "", "server")
	fs.Var(&serverArgs, "server-arg", "server argument")
	fs.StringVar(&o.Profile, "profile", "", "profile")
	fs.StringVar(&o.ConfigPath, "config", "", "acquisition profile configuration")
	fs.StringVar(&o.CatalogConfigPath, "catalog-config", "", "host-owned catalog continuation configuration")
	fs.StringVar(&o.PublicationRoot, "publication-root", "", "private publication root")
	fs.StringVar(&o.PrivateAcquisitionDiagnosticPath, "private-acquisition-diagnostic-path", "", "")
	fs.StringVar(&o.PrivateControlFlowDiagnosticPath, "private-control-flow-diagnostic-path", "", "")
	fs.StringVar(&o.PrivateProviderProbePath, "private-provider-probe-path", "", "")
	fs.StringVar(&o.PrivateProviderProbeVersion, "private-provider-probe-version", "v1", "")
	fs.BoolVar(&o.PrivateProviderProbeByteTimeNotifications, "private-provider-probe-byte-time-notifications", false, "")
	fs.StringVar(&o.PrivateInventoryBridgeConfigPath, "private-inventory-bridge-config", "", "")
	fs.StringVar(&o.PrivateInventoryConfigPath, "private-inventory-config", "", "")
	fs.StringVar(&o.PrivateInventoryQualifiedPrepareConfigPath, "private-inventory-qualified-prepare-config", "", "absolute caller-authored qualified preparation envelope; offline only")
	fs.BoolVar(&o.PrivateInventoryScopePreview, "private-inventory-scope-preview", false, "")
	fs.BoolVar(&o.Catalog, "catalog", false, "continue to the provisional feature catalog")
	fs.BoolVar(&o.CallerHost, "caller-host", false, "use the connected caller-HOST lifecycle from catalog-config")
	fs.StringVar(&o.CallerHostAttemptConfigPath, "caller-host-attempt-config", "", "fresh connected execution configuration for saved settings")
	fs.StringVar(&o.SaveCallerHostSettingsPath, "save-caller-host-settings", "", "export non-expiring settings to a new private file; no execution")
	fs.StringVar(&o.Resume, "resume", "", "resume an opaque catalog continuation selector")
	fs.StringVar(&o.StopAfter, "stop-after", "", "stop catalog continuation after describe-requests")
	fs.IntVar(&o.DownDepth, "down-depth", o.DownDepth, "")
	fs.IntVar(&o.UpDepth, "up-depth", o.UpDepth, "")
	fs.IntVar(&o.MaxNodes, "max-nodes", o.MaxNodes, "graph traversal node limit")
	fs.IntVar(&o.MaxBatchTargets, "batch-targets", o.MaxBatchTargets, "maximum census targets per acquisition batch (1..63)")
	fs.DurationVar(&o.Timeout, "timeout", o.Timeout, "")
	fs.DurationVar(&o.RequestTimeout, "request-timeout", o.RequestTimeout, "")
	fs.BoolVar(&o.Machine, "machine", false, "emit machine-readable JSON")
	fs.BoolVar(&o.EnumerationPreflight, "enumeration-preflight", false, "enumerate bounded source paths without starting a language server")
	fs.BoolVar(&o.PrivateEnumerationProjection, "private-enumeration-projection", false, "")
	fs.BoolVar(&o.Help, "help", false, "show census help")
	fs.BoolVar(&o.Help, "h", false, "show census help")
	// flag.FlagSet stops at the first positional argument. Move only actual
	// positionals past the flags, preserving flag values (including --server-arg).
	var flags, folders []string
	for i := 0; i < len(clean); i++ {
		arg := clean[i]
		if arg == "--" {
			folders = append(folders, clean[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			folders = append(folders, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
		if f := fs.Lookup(name); f != nil && !strings.Contains(arg, "=") {
			b, isBool := f.Value.(interface{ IsBoolFlag() bool })
			if (!isBool || !b.IsBoolFlag()) && i+1 < len(clean) {
				i++
				flags = append(flags, clean[i])
			}
		}
	}
	if err := fs.Parse(append(flags, append([]string{"--"}, folders...)...)); err != nil {
		return o, errors.New("invalid census syntax")
	}
	if machineCount > 1 {
		return o, errors.New("duplicate machine flag")
	}
	if censusNamedFlagCount(clean, "private-enumeration-projection") > 1 {
		return o, errors.New("duplicate private enumeration projection flag")
	}
	if censusNamedFlagCount(clean, "private-provider-probe-path") > 1 {
		return o, errors.New("duplicate private provider probe path flag")
	}
	byteTimeCount := censusNamedFlagCount(clean, "private-provider-probe-byte-time-notifications")
	if byteTimeCount > 1 {
		return o, errors.New("duplicate private provider probe byte-time notification flag")
	}
	versionCount := censusNamedFlagCount(clean, "private-provider-probe-version")
	if versionCount > 1 {
		return o, errors.New("duplicate private provider probe version flag")
	}
	if versionCount != 0 && o.PrivateProviderProbePath == "" {
		return o, errors.New("private provider probe version requires path")
	}
	if o.PrivateProviderProbeVersion != "v1" && o.PrivateProviderProbeVersion != "v2" && o.PrivateProviderProbeVersion != "v3" && o.PrivateProviderProbeVersion != "v4" && o.PrivateProviderProbeVersion != "v5" && o.PrivateProviderProbeVersion != "v6" && o.PrivateProviderProbeVersion != "v7" && o.PrivateProviderProbeVersion != "v8" {
		return o, errors.New("invalid private provider probe version")
	}
	if o.PrivateProviderProbeByteTimeNotifications && (o.PrivateProviderProbePath == "" || o.PrivateProviderProbeVersion != "v6") {
		return o, errors.New("invalid private provider probe version")
	}
	if len(src) > 0 {
		o.Sources = []string(src)
	}
	o.Includes, o.Excludes, o.ServerArgs = []string(inc), []string(exc), []string(serverArgs)
	var err error
	o.PrunedDirectories, err = canonicalCensusPrunedDirectories([]string(prune))
	if err != nil {
		return o, err
	}
	legacy := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "workspace" || f.Name == "catalog" || f.Name == "catalog-config" || f.Name == "caller-host" || f.Name == "caller-host-attempt-config" || f.Name == "save-caller-host-settings" || f.Name == "resume" || f.Name == "stop-after" || f.Name == "enumeration-preflight" || strings.HasPrefix(f.Name, "private-") {
			legacy = true
		}
	})
	if fs.NArg() > 1 || fs.NArg() != 0 && (legacy || o.Help) {
		return o, errors.New("folder invocation rejects legacy/private controls and accepts one folder")
	}
	o.FolderInvocation = !legacy && !o.Help
	if o.FolderInvocation {
		o.Workspace = "."
		if fs.NArg() == 1 {
			o.Workspace = fs.Arg(0)
		}
		o.Catalog = true
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
	return censusNamedFlagCount(args, wanted) > 0
}

func censusNamedFlagCount(args []string, wanted string) int {
	count := 0
	for i := 0; i < len(args); i++ {
		if args[i] == "--server-arg" || args[i] == "-server-arg" {
			i++
			continue
		}
		if strings.TrimPrefix(strings.SplitN(args[i], "=", 2)[0], "--") == wanted {
			count++
		}
	}
	return count
}

func censusMachineFlagCount(args []string) int {
	count := 0
	for i := 0; i < len(args); i++ {
		if args[i] == "--server-arg" || args[i] == "-server-arg" {
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
	callerHostCount := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--server-arg" || arg == "-server-arg" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
			if name == "caller-host" {
				callerHostCount++
			}
			if !o.Help && (name == "caller-host-attempt-config" || name == "save-caller-host-settings") {
				if !o.CallerHost || name == "caller-host-attempt-config" && o.CallerHostAttemptConfigPath == "" || name == "save-caller-host-settings" && o.SaveCallerHostSettingsPath == "" {
					return errors.New("caller-host settings controls require caller-host and a nonempty path")
				}
			}
		}
	}
	if !o.Help && (o.CallerHost || callerHostCount != 0) {
		if callerHostCount != 1 || !o.CallerHost || !o.Catalog || !o.Machine || o.Resume != "" || !filepath.IsAbs(o.CatalogConfigPath) || filepath.Clean(o.CatalogConfigPath) != o.CatalogConfigPath {
			return errors.New("caller-host requires catalog, machine and an absolute catalog config")
		}
		if o.CallerHostAttemptConfigPath != "" && o.SaveCallerHostSettingsPath != "" {
			return errors.New("caller-host save and execution controls are exclusive")
		}
		for _, path := range []string{o.CallerHostAttemptConfigPath, o.SaveCallerHostSettingsPath} {
			if path != "" && (!filepath.IsAbs(path) || filepath.Clean(path) != path) {
				return errors.New("caller-host settings paths must be absolute and canonical")
			}
		}
		// Source, owner, budgets and lifecycle action come from the one strict
		// connected envelope. Never silently ignore competing acquisition flags.
		allowed := map[string]bool{"catalog": true, "caller-host": true, "catalog-config": true, "machine": true, "caller-host-attempt-config": true, "save-caller-host-settings": true}
		seen := map[string]bool{}
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") {
				name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
				if !allowed[name] || seen[name] {
					return errors.New("caller-host rejects duplicate or ordinary acquisition controls")
				}
				seen[name] = true
			}
		}
		return nil
	}
	prepareCount, prepareMachineCount := 0, 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--server-arg" || arg == "-server-arg" {
			i++
			continue
		}
		switch strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-") {
		case "private-inventory-qualified-prepare-config":
			prepareCount++
		case "machine":
			prepareMachineCount++
		}
	}
	if o.PrivateInventoryQualifiedPrepareConfigPath != "" || prepareCount != 0 {
		if prepareCount != 1 || prepareMachineCount != 1 || !o.Machine || !filepath.IsAbs(o.PrivateInventoryQualifiedPrepareConfigPath) || filepath.Clean(o.PrivateInventoryQualifiedPrepareConfigPath) != o.PrivateInventoryQualifiedPrepareConfigPath {
			return errors.New("invalid qualified preparation syntax")
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") {
				name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
				if name != "private-inventory-qualified-prepare-config" && name != "machine" {
					return errors.New("qualified preparation rejects ordinary controls")
				}
			}
		}
		return nil
	}
	if o.Help {
		return nil
	}
	if o.PrivateInventoryBridgeConfigPath != "" {
		if censusNamedFlagCount(args, "private-inventory-bridge-config") != 1 || !o.Machine || !filepath.IsAbs(o.PrivateInventoryBridgeConfigPath) || filepath.Clean(o.PrivateInventoryBridgeConfigPath) != o.PrivateInventoryBridgeConfigPath {
			return errors.New("invalid private inventory bridge syntax")
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") {
				name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
				if name != "private-inventory-bridge-config" && name != "machine" {
					return errors.New("private inventory bridge rejects ordinary controls")
				}
			}
		}
		return nil
	}
	if o.EnumerationPreflight {
		if !o.Machine || o.Resume != "" {
			return errors.New("enumeration preflight requires fresh machine census")
		}
		allowed := map[string]bool{"workspace": true, "source": true, "include": true, "exclude": true, "prune-dir": true, "enumeration-preflight": true, "machine": true}
		for _, arg := range args {
			if strings.HasPrefix(arg, "--") && !allowed[strings.TrimPrefix(strings.SplitN(arg, "=", 2)[0], "--")] {
				return errors.New("enumeration preflight rejects provider, publication, catalog, and acquisition controls")
			}
		}
		if o.Workspace == "" || len(o.Sources) == 0 {
			return errors.New("enumeration preflight requires workspace and source")
		}
		for _, s := range o.Sources {
			if s == "" || filepath.IsAbs(s) || !filepath.IsLocal(s) || filepath.Clean(s) != s {
				return errors.New("source must be a canonical workspace-relative path")
			}
		}
		return nil
	}
	if censusNamedFlagCount(args, "private-inventory-scope-preview") > 1 || (censusNamedFlagCount(args, "private-inventory-scope-preview") != 0 && o.PrivateInventoryConfigPath == "") {
		return errors.New("private scope preview requires private inventory")
	}
	if o.PrivateInventoryConfigPath != "" {
		if censusNamedFlagCount(args, "private-inventory-config") != 1 || !o.PrivateEnumerationProjection || !o.Machine || o.Resume != "" || o.Catalog || o.StopAfter != "" || o.PrivateProviderProbePath != "" {
			return errors.New("private inventory requires fresh private machine census without catalog, resume or probe")
		}
	}
	if o.PrivateEnumerationProjection && (!o.Machine || o.Resume != "" || o.Catalog) {
		return errors.New("private enumeration projection requires fresh machine census")
	}
	if o.PrivateProviderProbePath != "" && (!o.Machine || o.Resume != "" || o.Catalog) {
		return errors.New("private provider probe requires fresh machine census")
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
		forbidden := map[string]bool{"source": true, "include": true, "exclude": true, "prune-dir": true, "enumeration-preflight": true, "server": true, "server-arg": true, "profile": true, "config": true, "down-depth": true, "up-depth": true, "max-nodes": true, "batch-targets": true, "timeout": true, "request-timeout": true, "private-acquisition-diagnostic-path": true, "private-control-flow-diagnostic-path": true, "private-enumeration-projection": true, "private-provider-probe-path": true, "private-provider-probe-version": true, "private-provider-probe-byte-time-notifications": true}
		for _, arg := range args {
			name := strings.TrimPrefix(strings.SplitN(arg, "=", 2)[0], "--")
			if forbidden[name] {
				return errors.New("resume rejects census acquisition controls")
			}
		}
		return nil
	}
	if o.Workspace == "" || len(o.Sources) == 0 || !o.FolderInvocation && o.PublicationRoot == "" {
		return errors.New("workspace, source, and publication root are required")
	}
	if o.Catalog && !o.FolderInvocation && o.CatalogConfigPath == "" {
		return errors.New("catalog requires catalog config and publication root")
	}
	if !o.Catalog && o.CatalogConfigPath != "" {
		return errors.New("catalog config requires catalog or resume")
	}
	if !o.FolderInvocation && o.Server == "" && o.Profile == "" {
		return errors.New("server or profile is required")
	}
	if o.ConfigPath != "" && o.Profile == "" {
		return errors.New("config requires profile")
	}
	if o.DownDepth < 0 || o.UpDepth < 0 || o.MaxNodes < 1 || o.MaxNodes > 10000 || o.MaxBatchTargets < 1 || o.MaxBatchTargets > 63 || o.Timeout <= 0 || o.RequestTimeout <= 0 || o.RequestTimeout > o.Timeout {
		return errors.New("invalid census bounds")
	}
	if path := o.PrivateAcquisitionDiagnosticPath; path != "" && (!filepath.IsAbs(path) || filepath.Clean(path) != path) {
		return errors.New("invalid private acquisition diagnostic path")
	}
	if path := o.PrivateControlFlowDiagnosticPath; path != "" && (!filepath.IsAbs(path) || filepath.Clean(path) != path) {
		return errors.New("invalid private control-flow diagnostic path")
	}
	if path := o.PrivateProviderProbePath; path != "" && (!filepath.IsAbs(path) || filepath.Clean(path) != path) {
		return errors.New("invalid private provider probe path")
	}
	for _, s := range o.Sources {
		if s == "" || filepath.IsAbs(s) || !filepath.IsLocal(s) || filepath.Clean(s) != s {
			return errors.New("source must be a canonical workspace-relative path")
		}
	}
	return nil
}

func canonicalCensusPrunedDirectories(values []string) ([]string, error) {
	out := append([]string(nil), values...)
	for _, value := range out {
		if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, 0) || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.IsAbs(value) || path.Clean(value) != value {
			return nil, errors.New("prune directory must be a canonical workspace-relative slash path")
		}
		parts := strings.Split(value, "/")
		for i, part := range parts {
			if part == "" || part == "." || part == ".." || i == 0 && strings.Contains(part, ":") {
				return nil, errors.New("prune directory must be a canonical workspace-relative slash path")
			}
		}
	}
	sort.Strings(out)
	minimal := out[:0]
	for _, value := range out {
		if len(minimal) > 0 && (value == minimal[len(minimal)-1] || strings.HasPrefix(value, minimal[len(minimal)-1]+"/")) {
			continue
		}
		minimal = append(minimal, value)
	}
	return minimal, nil
}
