package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/programcpresentation"
	"lsp-trace/internal/projectionfailure"
	"lsp-trace/internal/requestlifecycle"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/session"
	"lsp-trace/internal/slicer"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

func productionV5Requested(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") || arg == "-" {
			return false
		}
		name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
		if name == "production-v5" {
			return true
		}
		boolean := name == "pretty" || name == "graph-provenance" || name == "expand-topmost-siblings" || name == "expand-dispatch-family" || name == "help" || name == "h"
		if !boolean && !strings.Contains(name, "=") && i+1 < len(args) {
			i++
		}
	}
	return false
}

// Version dispatch is explicit. Without a selector, legacy parsing and bytes
// are untouched. --production-v5 is removed before dispatch and contributes
// only the canonical v3 acquisition and Graph Provenance V5 output selectors.
func acquisitionVersion(args []string) (string, []string, error) {
	version, outputVersion := "", ""
	productionV5 := productionV5Requested(args)
	productionV5Seen := false
	out := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") || arg == "-" {
			out = append(out, args[i:]...)
			break
		}
		name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
		isAcquisition := name == "acquisition-version" || strings.HasPrefix(name, "acquisition-version=")
		isProductionV5 := name == "production-v5"
		isOutput := name == "output-version" || strings.HasPrefix(name, "output-version=")
		if !isAcquisition && !isProductionV5 && !(productionV5 && isOutput) {
			out = append(out, arg)
			// All existing non-boolean flags consume one following value.
			// Do not mistake a server argument or filename for our selector.
			boolean := name == "pretty" || name == "graph-provenance" || name == "expand-topmost-siblings" || name == "expand-dispatch-family" || name == "production-v5" || name == "help" || name == "h"
			if !boolean && !strings.Contains(name, "=") && i+1 < len(args) {
				i++
				out = append(out, args[i])
			}
			continue
		}
		if isProductionV5 {
			if productionV5Seen {
				return "", nil, fmt.Errorf("duplicate production-v5")
			}
			productionV5Seen = true
			continue
		}
		value := ""
		if _, v, ok := strings.Cut(name, "="); ok {
			value = v
		} else {
			i++
			if i >= len(args) {
				return "", nil, fmt.Errorf("%s requires a value", strings.TrimSuffix(name, "="))
			}
			value = args[i]
		}
		if isOutput {
			if productionV5 && value != graphprovenance.VersionV5 {
				return "", nil, fmt.Errorf("--production-v5 conflicts with --output-version %s", value)
			}
			outputVersion = value
			continue
		}
		if version != "" {
			return "", nil, fmt.Errorf("duplicate acquisition-version")
		}
		version = value
		if version != "v1" && version != "v2" && version != "v3" {
			return "", nil, fmt.Errorf("unsupported acquisition version %q", version)
		}
	}
	if productionV5 {
		if version != "" && version != "v3" {
			return "", nil, fmt.Errorf("--production-v5 conflicts with --acquisition-version %s", version)
		}
		if outputVersion != "" && outputVersion != graphprovenance.VersionV5 {
			return "", nil, fmt.Errorf("--production-v5 conflicts with --output-version %s", outputVersion)
		}
		version = "v3"
		out = append(out, "--output-version", graphprovenance.VersionV5)
	}
	return version, out, nil
}

func admitAcquisitionV2(data []byte) error {
	_, err := graphprovenance.ValidateFor(data, graphprovenance.Family, "v2")
	return err
}

func admitAcquisitionV3(data []byte) error {
	_, err := graphprovenance.ValidateFor(data, graphprovenance.Family, "v3")
	return err
}

func admitAcquisitionV5(data []byte) error {
	_, err := graphprovenance.ValidateFor(data, graphprovenance.Family, "v5")
	return err
}

func runAcquisitionV2(mode string, args []string, stdout, stderr io.Writer) int {
	return runAcquisitionVersion(mode, "v2", args, stdout, stderr)
}

// privateAcquisitionRuntime intentionally exposes only the historical public
// executor contract. Its manager may retain private diagnostics, but those
// records cannot enter or alter the frozen public V3 composition.
type privateAcquisitionRuntime struct {
	manager *sessionruntime.Manager
	mu      sync.Mutex
	handles []sessionruntime.DiagnosticOperationHandle
}

func (r *privateAcquisitionRuntime) Metadata(id string, generation uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return r.manager.Metadata(id, generation)
}
func (r *privateAcquisitionRuntime) retainHandle(handle sessionruntime.DiagnosticOperationHandle) {
	if handle == (sessionruntime.DiagnosticOperationHandle{}) {
		return
	}
	r.mu.Lock()
	r.handles = append(r.handles, handle)
	r.mu.Unlock()
}
func (r *privateAcquisitionRuntime) diagnosticHandles() []sessionruntime.DiagnosticOperationHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sessionruntime.DiagnosticOperationHandle(nil), r.handles...)
}
func (r *privateAcquisitionRuntime) RoundTrip(ctx context.Context, request sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	result := r.manager.RoundTrip(ctx, request)
	r.retainHandle(result.DiagnosticOperation)
	return result
}
func (r *privateAcquisitionRuntime) PrepareDocument(ctx context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	result := r.manager.PrepareDocument(ctx, request)
	r.retainHandle(result.DiagnosticOperation)
	return result
}
func (r *privateAcquisitionRuntime) Records() []sessionruntime.Record { return r.manager.Records() }
func (r *privateAcquisitionRuntime) SeedCustodyProvenance(sessionID string, generation uint64) (seedbinding.CustodyMode, bool) {
	if provenance, found := r.manager.SeedCustodyProvenance(sessionID, generation); found {
		return provenance, true
	}
	// This private wrapper is constructed only after the CLI has read the caller's
	// bounded local manifest and workspace and started that exact managed session.
	return seedbinding.CallerAssertedLocal, true
}

func runAcquisitionVersion(mode, version string, args []string, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	fs := flag.NewFlagSet(mode+" v2", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c sliceConfig
	var profile profileFlags
	var manifestPath, outputVersion, groupBy string
	var communitySeed uint64
	var pageRankTopK, hubTopK int
	var diagnosticRoot, diagnosticSelector string
	var bindingRoot, bindingSelector string
	var requestDiagnosticRoot, requestDiagnosticSelector string
	fs.StringVar(&c.workspace, "workspace", "", "host workspace path")
	fs.StringVar(&c.command, "server", "", "trusted language server executable")
	fs.StringVar(&profile.Name, "profile", "", "named server profile")
	fs.StringVar(&profile.ConfigPath, "config", "", "profile configuration")
	fs.Var(&c.args, "server-arg", "server argument")
	fs.Var(&c.env, "server-env", "server environment KEY=VALUE")
	fs.Var(&c.fromFiles, "from-file", "repeatable source file or directory for automatic callable-symbol discovery (production V5 slice only)")
	fs.Var(&c.includes, "include", "repeatable workspace-relative gitignore-style path pattern for automatic discovery")
	fs.Var(&c.excludes, "exclude", "repeatable workspace-relative gitignore-style path pattern; excludes take precedence")
	fs.StringVar(&c.languageID, "language-id", "", "runtime default document language")
	fs.StringVar(&manifestPath, "seed-manifest", "", "versioned v2 seed manifest file; no inline selector/limit overrides")
	fs.StringVar(&outputVersion, "output-version", "", "managed output version (lsp-trace.graph-provenance.v5 requires v3 and topmost siblings)")
	fs.StringVar(&c.output, "output", "", "immutable graph output selector")
	fs.StringVar(&groupBy, "group-by", "", "optional post-acquisition grouping: leiden or none")
	fs.Uint64Var(&communitySeed, "community-seed", 0, "Leiden deterministic community seed")
	fs.IntVar(&pageRankTopK, "pagerank-top-k", 0, "mandatory positive grouped PageRank result count")
	fs.IntVar(&hubTopK, "hub-top-k", 0, "mandatory positive grouped hub result count")
	fs.StringVar(&diagnosticRoot, "private-startup-diagnostic-root", "", "caller-approved private diagnostic root")
	fs.StringVar(&diagnosticSelector, "private-startup-diagnostic-selector", "", "safe relative startup diagnostic selector")
	fs.StringVar(&bindingRoot, "private-seed-binding-root", "", "caller-approved seed-binding root (v3 only)")
	fs.StringVar(&bindingSelector, "private-seed-binding-selector", "", "safe relative seed-binding selector (v3 only)")
	fs.StringVar(&requestDiagnosticRoot, "private-request-diagnostic-root", "", "caller-approved private request diagnostic root (v3 only)")
	fs.StringVar(&requestDiagnosticSelector, "private-request-diagnostic-selector", "", "safe relative private request diagnostic selector (v3 only)")
	fs.BoolVar(&c.pretty, "pretty", false, "indent outer JSON without changing embedded graph bytes")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if outputVersion != graphprovenance.VersionV5 {
		fmt.Fprintf(stderr, "DEPRECATED: Graph Provenance %s production is deprecated; migrate new production to source-qualified Graph Provenance V5. Historical readers, replay, and validation remain supported.\n", strings.ToUpper(version))
	}
	grouped := groupBy != "" && groupBy != "none"
	if groupBy != "" && groupBy != "none" && groupBy != "leiden" {
		return fail(fmt.Errorf("unsupported --group-by %q", groupBy))
	}
	if grouped && (mode != "slice" || version != "v3" || outputVersion != graphprovenance.VersionV5 || c.output == "" || !explicit["community-seed"] || pageRankTopK < 1 || hubTopK < 1) {
		return fail(fmt.Errorf("--group-by leiden requires slice acquisition v3, Graph Provenance V5/--production-v5, --output, --community-seed, and positive --pagerank-top-k and --hub-top-k"))
	}
	if !grouped && (explicit["community-seed"] || explicit["pagerank-top-k"] || explicit["hub-top-k"]) {
		return fail(fmt.Errorf("community options require --group-by leiden"))
	}
	discoverSeeds := len(c.fromFiles) > 0
	if fs.NArg() != 0 || c.workspace == "" || (manifestPath == "" && !discoverSeeds) {
		return fail(fmt.Errorf("v2 requires --workspace and exactly one of --seed-manifest or production V5 slice --from-file with no positional arguments"))
	}
	if manifestPath != "" && discoverSeeds {
		return fail(fmt.Errorf("--seed-manifest and --from-file are mutually exclusive"))
	}
	if !discoverSeeds && (len(c.includes) > 0 || len(c.excludes) > 0) {
		return fail(fmt.Errorf("--include and --exclude require automatic discovery with --from-file"))
	}
	if discoverSeeds && (mode != "slice" || version != "v3" || outputVersion != graphprovenance.VersionV5) {
		return fail(fmt.Errorf("--from-file automatic discovery requires slice --production-v5"))
	}
	requestDiagnosticsRequested := requestDiagnosticRoot != "" || requestDiagnosticSelector != ""
	if requestDiagnosticsRequested && (version != "v3" || requestDiagnosticRoot == "" || requestDiagnosticSelector == "") {
		return fail(fmt.Errorf("private request diagnostics require v3 and one rooted selector pair"))
	}
	if profile.ConfigPath != "" && profile.Name == "" {
		return fail(fmt.Errorf("--config requires --profile"))
	}
	if (diagnosticRoot == "") != (diagnosticSelector == "") {
		return fail(fmt.Errorf("private startup diagnostic root and selector must be supplied together"))
	}
	bindingRequested := bindingRoot != "" || bindingSelector != ""
	if bindingRequested && (version != "v3" || bindingRoot == "" || bindingSelector == "") {
		return fail(fmt.Errorf("private seed binding requires v3 and one rooted selector pair"))
	}
	op := acquisitionops.Slice
	if mode == "incoming" {
		op = acquisitionops.Incoming
	}
	if version == "v3" {
		op = acquisitionops.SliceV3
		if mode == "incoming" {
			op = acquisitionops.IncomingV3
		}
	}
	var manifest acquisitionops.Manifest
	var requestPolicy []byte
	var retainedSeedSpec []byte
	var sources []resolvedSliceSource
	var discoveryCounts discoveryAccounting
	var err error
	if discoverSeeds {
		_, _, sources, discoveryCounts, err = resolveSliceSourcesWithAccounting(c)
		if err != nil {
			if discoveryCounts.FilesEnumerated > 0 {
				fmt.Fprintln(stderr, formatDiscoveryAccounting(discoveryCounts))
			}
			return fail(err)
		}
		defaults := defaultDiscoveryLimits()
		placeholder := seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "root", Path: sources[0].path, Line: 1, Column: 1}}}}
		manifest, err = seedformat.Translate(placeholder, seedformat.TranslateOptions{Workspace: c.workspace, Limits: defaults, TopmostSiblings: true})
		if err != nil {
			return fail(err)
		}
	} else {
		// Bounded regular-file input: no FIFO or unbounded ReadAll, no MCP path analogue.
		manifestRoot, openErr := os.OpenRoot(filepath.Dir(manifestPath))
		if openErr != nil {
			return fail(openErr)
		}
		var readErr error
		requestPolicy, readErr = source.ReadRegularInputBounded(manifestRoot, filepath.Base(manifestPath), acquisitionops.MaxInputBytes)
		manifestRoot.Close()
		if readErr != nil {
			return fail(readErr)
		}
		manifest, err = acquisitionops.DecodeManifest(requestPolicy, op)
		if err != nil {
			return fail(err)
		}
	}
	if manifest.Expansion.TopmostSiblings && outputVersion != graphprovenance.VersionV5 {
		return fail(fmt.Errorf("expansion.topmost_siblings requires explicit graph-provenance v5 output"))
	}
	if outputVersion != "" && (outputVersion != graphprovenance.VersionV5 || version != "v3" || !manifest.Expansion.TopmostSiblings) {
		return fail(fmt.Errorf("graph-provenance v5 output requires acquisition-version v3 and expansion.topmost_siblings=true"))
	}
	effective, err := manifest.Request(op)
	if err != nil {
		return fail(err)
	}
	if err := applySliceProfile(&c, profile, explicitServerFields(fs)); err != nil {
		return fail(err)
	}
	if c.command == "" {
		return fail(fmt.Errorf("--server or --profile is required"))
	}
	for _, entry := range c.env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			return fail(fmt.Errorf("invalid --server-env"))
		}
	}
	workspace, err := filepath.Abs(c.workspace)
	if err != nil {
		return fail(err)
	}
	command, err := exec.LookPath(c.command)
	if err != nil {
		if diagnosticRoot == "" {
			return fail(err)
		}
		// Opted-in startup diagnostics require manager-owned attempt identity even
		// when execution is not attempted successfully. Do not claim a start.
		command = c.command
	}
	command, err = filepath.Abs(command)
	if err != nil {
		return fail(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "public-acquisition", Workspace: workspace, Profile: "cli", EnvironmentReference: "cli"})
	if err != nil {
		return fail(err)
	}
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		return fail(err)
	}
	const privateMaxRecords = 64
	const privateMaxBytes = 64 << 10
	var diagnosticStore *manageddiagnostic.Store
	if diagnosticRoot != "" || requestDiagnosticsRequested {
		diagnosticStore = manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: manageddiagnostic.StartupDiagnosticsMaxRecords, MaxBytes: manageddiagnostic.StartupDiagnosticsMaxBytes})
	}
	var binding *seedbinding.Manifest
	var bindingRevision seedbinding.RevisionAuthority
	if bindingRequested {
		bindingBytes, readErr := readPrivateSeedInput(bindingRoot, bindingSelector)
		if readErr != nil {
			return fail(fmt.Errorf("seed binding unavailable"))
		}
		decode := seedbinding.DecodeV2
		if version == "v3" {
			decode = seedbinding.DecodeV3
		}
		decoded, decodeErr := decode(bindingBytes)
		if decodeErr != nil {
			return fail(fmt.Errorf("seed binding invalid"))
		}
		binding = &decoded
		if decoded.CustodyMode != seedbinding.CallerAssertedLocal {
			bindingRevision = seedbinding.HostReceiptAuthority{}
		}
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}, Diagnostics: diagnosticStore, SeedRevisionAuthority: bindingRevision})
	if err != nil {
		return fail(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, deadlineCancel := context.WithTimeout(ctx, effective.Limits.Timeout)
	defer deadlineCancel()
	var providerIdentity seedbinding.ProviderIdentity
	if binding != nil {
		providerIdentity = seedbinding.ProviderIdentity{
			Class: binding.Validator.Class, Authority: binding.Validator.Authority, Name: binding.Validator.Name, Version: binding.Validator.Version,
			ExecutableSHA256: binding.Validator.ExecutableSHA256, PayloadSHA256: binding.Validator.PayloadSHA256, ConfigSHA256: binding.Validator.ConfigSHA256,
		}
	}
	started := manager.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.languageID, SeedBinding: binding, ProviderIdentity: providerIdentity, Process: managedprocess.Spec{Path: command, Args: c.args, Dir: workspace, Env: append(os.Environ(), c.env...)}})
	if diagnosticRoot != "" {
		defer func() {
			generation := manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}
			if started.SessionID != "" && started.Generation > 0 {
				generation = manager.Diagnostics(started.SessionID, started.Generation)
			}
			_, _ = (manageddiagnostic.StartupDiagnosticSink{Root: diagnosticRoot, Selector: diagnosticSelector}).Finalize(manager.GetStartupAttempt(started.AttemptID), generation)
		}()
	}
	if started.Failure != "" {
		return fail(fmt.Errorf("managed startup: %s", started.Failure))
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		stopped := manager.Stop(cleanup, started.SessionID, "public-acquisition-cli")
		if stopped.Failure != "" {
			fmt.Fprintln(stderr, "managed cleanup:", stopped.Failure)
		}
		if err := manager.Shutdown(cleanup); err != nil {
			fmt.Fprintln(stderr, "managed cleanup:", err)
		}
	}()
	pending := manager.BeginReadiness(ctx, started.SessionID, started.Generation, time.Now().Add(effective.Limits.RequestTimeout))
	ready, ok := manager.WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		return fail(fmt.Errorf("managed readiness: %s", ready.Failure))
	}
	privateRuntime := &privateAcquisitionRuntime{manager: manager}
	privateRuntime.retainHandle(ready.DiagnosticOperation)
	if discoverSeeds {
		discoveries := make(map[string]slicer.Discovery, len(sources))
		client := productionDiscoveryClient{runtime: privateRuntime, sessionID: started.SessionID, generation: started.Generation, timeout: effective.Limits.RequestTimeout}
		preparationComplete := true
		for _, sourceFile := range sources {
			prepared := manager.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: sourceFile.uri, LanguageID: sourceFile.lang})
			privateRuntime.retainHandle(prepared.DiagnosticOperation)
			if prepared.Failure != "" {
				return fail(fmt.Errorf("automatic seed discovery document supply %s: %s", sourceFile.path, prepared.Failure))
			}
			discovery := slicer.Discover(ctx, client, sourceFile.uri, slicer.Options{DownDepth: 0, MaxNodes: 0})
			discoveryCounts.SymbolsEnumerated += discovery.PreparationAccounting.DocumentSymbols
			discoveryCounts.SymbolsSelected += discovery.PreparationAccounting.Attempted
			discoveryCounts.SymbolsUnsupported += discovery.PreparationAccounting.NotPreparable
			discoveryCounts.SymbolsPreparationFailed += discovery.PreparationAccounting.Failed
			discoveryCounts.SymbolsPrepared += discovery.PreparationAccounting.Prepared
			preparationComplete = preparationComplete && discovery.PreparationCensusComplete
			discoveries[sourceFile.uri] = discovery
		}
		fmt.Fprintln(stderr, formatDiscoveryAccounting(discoveryCounts))
		if !preparationComplete {
			return fail(fmt.Errorf("automatic seed discovery incomplete: selected_files=%d attempted=%d prepared=%d unsupported=%d preparation_failed=%d", discoveryCounts.FilesSelected, discoveryCounts.SymbolsSelected, discoveryCounts.SymbolsPrepared, discoveryCounts.SymbolsUnsupported, discoveryCounts.SymbolsPreparationFailed))
		}
		seedFile, canonical, discoveryErr := canonicalDiscoveredSeeds(c.workspace, sources, discoveries)
		if discoveryErr != nil {
			return fail(discoveryErr)
		}
		manifest, discoveryErr = seedformat.Translate(seedFile, seedformat.TranslateOptions{Workspace: c.workspace, Limits: defaultDiscoveryLimits(), TopmostSiblings: true})
		if discoveryErr != nil {
			return fail(discoveryErr)
		}
		retainedSeedSpec = canonical
		requestPolicy, _ = json.Marshal(manifest)
	}
	input, _ := json.Marshal(acquisitionops.Input{SessionID: started.SessionID, Generation: started.Generation, SeedManifest: manifest, OutputVersion: outputVersion})
	finalizeRequestDiagnostics := func(public []byte) {
		if !requestDiagnosticsRequested {
			return
		}
		handles := privateRuntime.diagnosticHandles()
		sourceSet, certified := manager.DiagnosticSnapshotSetFor(started.AttemptID, started.DiagnosticGeneration, handles, requestlifecycle.MaxRecords)
		var privateErr error
		reason := "SOURCE_UNCERTIFIED"
		if !certified {
			privateErr = fmt.Errorf("private lifecycle source unavailable")
		} else {
			reason = "PROJECTION_REJECTED"
			privateRaw, projectionErr := requestlifecycle.ProjectRuntime(sourceSet, public, "")
			privateErr = projectionErr
			if privateErr == nil {
				reason = "PUBLICATION_REJECTED"
				privateErr = manageddiagnostic.PublishHardened(requestDiagnosticRoot, requestDiagnosticSelector, privateRaw, func(raw []byte) error {
					_, verifyErr := requestlifecycle.Verify(raw, public)
					return verifyErr
				})
			}
		}
		if privateErr != nil {
			fmt.Fprintln(stderr, "private request diagnostics unavailable:", reason)
			if reason == "PUBLICATION_REJECTED" {
				fmt.Fprintln(stderr, "private diagnostic publication remediation:", privateErr)
			}
		}
	}
	result, failed := acquisitionops.NewExecutor(privateRuntime).Execute(ctx, operation.Request{Name: op, Input: input, RetainedSeedSpec: retainedSeedSpec})
	if failed != nil {
		projectionFailure := outputVersion == graphprovenance.VersionV5 && failed.Code == "OUTPUT_VALIDATION_FAILED" && failed.Err != nil && failed.Err.Error() == "topmost sibling expansion produced no exact relations"
		if requestDiagnosticsRequested && projectionFailure {
			failureDiagnosticPublished := false
			failureDiagnosticStage := "PUBLIC_ARTIFACT_UNAVAILABLE"
			var failurePublicationErr error
			handles := privateRuntime.diagnosticHandles()
			sourceSet, certified := manager.DiagnosticSnapshotSetFor(started.AttemptID, started.DiagnosticGeneration, handles, projectionfailure.MaxRecords)
			if certified {
				privateRaw, projectionErr := projectionfailure.Project(sourceSet, projectionfailure.Request{
					Operation: mode + "-v3", RequestPolicy: requestPolicy,
					Stage: "GRAPH_PROVENANCE_V5_PROJECTION", Code: failed.Code, MismatchReason: "NO_EXACT_TOPMOST_SIBLING_RELATIONS",
				})
				if projectionErr == nil {
					failureDiagnosticStage = "PRIVATE_PUBLICATION_REJECTED"
					projectionErr = manageddiagnostic.PublishHardened(requestDiagnosticRoot, requestDiagnosticSelector, privateRaw, projectionfailure.Validate)
					failurePublicationErr = projectionErr
				} else {
					failureDiagnosticStage = "PRIVATE_PROJECTION_REJECTED"
				}
				failureDiagnosticPublished = projectionErr == nil
			}
			if !failureDiagnosticPublished {
				fmt.Fprintf(stderr, "private request diagnostics unavailable: %s; PUBLIC_ARTIFACT_UNAVAILABLE; lifecycle diagnostics require successful public artifact bytes for integrity binding\n", failureDiagnosticStage)
				if failurePublicationErr != nil {
					fmt.Fprintln(stderr, "private diagnostic publication remediation:", failurePublicationErr)
				}
			}
		} else if len(result.Artifact) > 0 {
			finalizeRequestDiagnostics(result.Artifact)
		} else if requestDiagnosticsRequested {
			fmt.Fprintln(stderr, "private request diagnostics unavailable: PUBLIC_ARTIFACT_UNAVAILABLE; lifecycle diagnostics require successful public artifact bytes for integrity binding")
		}
		return fail(failed)
	}
	data := result.Artifact
	if c.pretty {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "  "); err != nil {
			return fail(err)
		}
		data = append(pretty.Bytes(), '\n')
	}
	if c.output != "" {
		admit := admitAcquisitionV2
		if version == "v3" {
			admit = admitAcquisitionV3
		}
		if outputVersion == graphprovenance.VersionV5 {
			admit = admitAcquisitionV5
		}
		err = publishValidatedBundle(c.output, data, admit)
	} else {
		_, err = stdout.Write(data)
	}
	if err != nil {
		return fail(err)
	}
	if grouped {
		presentation, groupErr := programcpresentation.Handle(programcpresentation.Request{Input: result.Artifact, Seed: communitySeed, PageRankTopK: pageRankTopK, HubTopK: hubTopK})
		if groupErr != nil {
			return fail(fmt.Errorf("grouping computation: %w", groupErr))
		}
		if groupErr = programcpresentation.Text(stdout, presentation); groupErr != nil {
			return fail(fmt.Errorf("presentation publication: %w", groupErr))
		}
	}
	// Private finalization is synchronous but secondary and non-overriding: the
	// already-emitted public V3 bytes and their status remain authoritative.
	finalizeRequestDiagnostics(data)
	return 0 // Every structurally valid acquisition retains partial outcomes.
}

func readPrivateSeedInput(rootPath, selector string) ([]byte, error) {
	if !filepath.IsAbs(rootPath) || selector == "" || filepath.IsAbs(selector) || filepath.Clean(selector) != selector || selector == ".." || strings.HasPrefix(selector, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("unsafe private selector")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return source.ReadRegularInputBounded(root, selector, acquisitionops.MaxInputBytes)
}
