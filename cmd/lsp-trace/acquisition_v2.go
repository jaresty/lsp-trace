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
	"lsp-trace/internal/projectionfailure"
	"lsp-trace/internal/requestlifecycle"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/session"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

// Version dispatch is explicit. Without the flag, legacy parsing and bytes are
// untouched. Duplicate flags fail closed, including conflicting v1/v2 values.
func acquisitionVersion(args []string) (string, []string, error) {
	version := ""
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") || arg == "-" {
			out = append(out, args[i:]...)
			break
		}
		name := strings.TrimPrefix(arg, "-")
		name = strings.TrimPrefix(name, "-")
		if name != "acquisition-version" && !strings.HasPrefix(name, "acquisition-version=") {
			out = append(out, arg)
			// All existing non-boolean flags consume one following value.
			// Do not mistake a server argument or filename for our selector.
			boolean := name == "pretty" || name == "graph-provenance" || name == "expand-topmost-siblings" || name == "expand-dispatch-family" || name == "help" || name == "h"
			if !boolean && !strings.Contains(name, "=") && i+1 < len(args) {
				i++
				out = append(out, args[i])
			}
			continue
		}
		if version != "" {
			return "", nil, fmt.Errorf("duplicate acquisition-version")
		}
		if _, v, ok := strings.Cut(name, "="); ok {
			version = v
		} else {
			i++
			if i >= len(args) {
				return "", nil, fmt.Errorf("acquisition-version requires a value")
			}
			version = args[i]
		}
		if version != "v1" && version != "v2" && version != "v3" {
			return "", nil, fmt.Errorf("unsupported acquisition version %q", version)
		}
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
	var manifestPath, outputVersion string
	var diagnosticRoot, diagnosticSelector string
	var bindingRoot, bindingSelector string
	var requestDiagnosticRoot, requestDiagnosticSelector string
	fs.StringVar(&c.workspace, "workspace", "", "host workspace path")
	fs.StringVar(&c.command, "server", "", "trusted language server executable")
	fs.StringVar(&profile.Name, "profile", "", "named server profile")
	fs.StringVar(&profile.ConfigPath, "config", "", "profile configuration")
	fs.Var(&c.args, "server-arg", "server argument")
	fs.Var(&c.env, "server-env", "server environment KEY=VALUE")
	fs.StringVar(&c.languageID, "language-id", "", "runtime default document language")
	fs.StringVar(&manifestPath, "seed-manifest", "", "versioned v2 seed manifest file; no inline selector/limit overrides")
	fs.StringVar(&outputVersion, "output-version", "", "managed output version (lsp-trace.graph-provenance.v5 requires v3 and topmost siblings)")
	fs.StringVar(&c.output, "output", "", "immutable output selector")
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
	if outputVersion != graphprovenance.VersionV5 {
		fmt.Fprintf(stderr, "DEPRECATED: Graph Provenance %s production is deprecated; migrate new production to source-qualified Graph Provenance V5. Historical readers, replay, and validation remain supported.\n", strings.ToUpper(version))
	}
	if fs.NArg() != 0 || c.workspace == "" || manifestPath == "" {
		return fail(fmt.Errorf("v2 requires --workspace and --seed-manifest with no positional arguments"))
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
	// Bounded regular-file input: no FIFO or unbounded ReadAll, no MCP path analogue.
	manifestRoot, err := os.OpenRoot(filepath.Dir(manifestPath))
	if err != nil {
		return fail(err)
	}
	raw, err := source.ReadRegularInputBounded(manifestRoot, filepath.Base(manifestPath), acquisitionops.MaxInputBytes)
	manifestRoot.Close()
	if err != nil {
		return fail(err)
	}
	manifest, err := acquisitionops.DecodeManifest(raw, op)
	if err != nil {
		return fail(err)
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
	input, _ := json.Marshal(acquisitionops.Input{SessionID: started.SessionID, Generation: started.Generation, SeedManifest: manifest, OutputVersion: outputVersion})
	privateRuntime := &privateAcquisitionRuntime{manager: manager}
	privateRuntime.retainHandle(ready.DiagnosticOperation)
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
		}
	}
	result, failed := acquisitionops.NewExecutor(privateRuntime).Execute(ctx, operation.Request{Name: op, Input: input})
	if failed != nil {
		if len(result.Artifact) > 0 {
			finalizeRequestDiagnostics(result.Artifact)
		} else if requestDiagnosticsRequested {
			failureDiagnosticPublished := false
			failureDiagnosticStage := "PUBLIC_ARTIFACT_UNAVAILABLE"
			if outputVersion == graphprovenance.VersionV5 && failed.Code == "OUTPUT_VALIDATION_FAILED" && failed.Err != nil && failed.Err.Error() == "topmost sibling expansion produced no exact relations" {
				handles := privateRuntime.diagnosticHandles()
				sourceSet, certified := manager.DiagnosticSnapshotSetFor(started.AttemptID, started.DiagnosticGeneration, handles, projectionfailure.MaxRecords)
				if certified {
					privateRaw, projectionErr := projectionfailure.Project(sourceSet, projectionfailure.Request{
						Operation: mode + "-v3", RequestPolicy: raw,
						Stage: "GRAPH_PROVENANCE_V5_PROJECTION", Code: failed.Code, MismatchReason: "NO_EXACT_TOPMOST_SIBLING_RELATIONS",
					})
					if projectionErr == nil {
						failureDiagnosticStage = "PRIVATE_PUBLICATION_REJECTED"
						projectionErr = manageddiagnostic.PublishHardened(requestDiagnosticRoot, requestDiagnosticSelector, privateRaw, projectionfailure.Validate)
					} else {
						failureDiagnosticStage = "PRIVATE_PROJECTION_REJECTED"
					}
					failureDiagnosticPublished = projectionErr == nil
				}
			}
			if !failureDiagnosticPublished {
				fmt.Fprintf(stderr, "private request diagnostics unavailable: %s; PUBLIC_ARTIFACT_UNAVAILABLE; lifecycle diagnostics require successful public artifact bytes for integrity binding\n", failureDiagnosticStage)
			}
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
