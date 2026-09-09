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

func runAcquisitionV2(mode string, args []string, stdout, stderr io.Writer) int {
	return runAcquisitionVersion(mode, "v2", args, stdout, stderr)
}

// privateAcquisitionRuntime intentionally exposes only the historical public
// executor contract. Its manager may retain private diagnostics, but those
// records cannot enter or alter the frozen public V3 composition.
type privateAcquisitionRuntime struct {
	manager       *sessionruntime.Manager
	mu            sync.Mutex
	requests      []manageddiagnostic.Record
	retainedBytes int
	omitted       uint64
}

func (r *privateAcquisitionRuntime) Metadata(id string, generation uint64) (sessionruntime.SessionMetadata, session.Failure) {
	return r.manager.Metadata(id, generation)
}
func (r *privateAcquisitionRuntime) RoundTrip(ctx context.Context, request sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	prior := request.DiagnosticObserver
	request.DiagnosticObserver = func(record manageddiagnostic.Record) {
		if prior != nil {
			prior(record)
		}
		encoded, encodeErr := json.Marshal(record)
		withinStrings := true
		if encodeErr == nil {
			var value any
			if json.Unmarshal(encoded, &value) == nil {
				var visit func(any)
				visit = func(v any) {
					switch x := v.(type) {
					case string:
						if len(x) > 4096 {
							withinStrings = false
						}
					case []any:
						for _, item := range x {
							visit(item)
						}
					case map[string]any:
						for key, item := range x {
							if len(key) > 4096 {
								withinStrings = false
							}
							visit(item)
						}
					}
				}
				visit(value)
			}
		}
		r.mu.Lock()
		if encodeErr != nil || !withinStrings || len(r.requests) >= 64 || r.retainedBytes+len(encoded) > 65536 {
			r.omitted++
		} else {
			r.requests = append(r.requests, record.Clone())
			r.retainedBytes += len(encoded)
		}
		r.mu.Unlock()
	}
	return r.manager.RoundTrip(ctx, request)
}
func (r *privateAcquisitionRuntime) privateQuery() manageddiagnostic.QueryResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]manageddiagnostic.Record, len(r.requests))
	for i := range r.requests {
		out[i] = r.requests[i].Clone()
	}
	if len(out) == 0 {
		if r.omitted > 0 {
			return manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryEvicted, EvictedRecords: r.omitted}
		}
		return manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}
	}
	return manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryAvailable, Records: out, EvictedRecords: r.omitted}
}
func (r *privateAcquisitionRuntime) PrepareDocument(ctx context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	return r.manager.PrepareDocument(ctx, request)
}
func (r *privateAcquisitionRuntime) Records() []sessionruntime.Record { return r.manager.Records() }

func runAcquisitionVersion(mode, version string, args []string, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	fs := flag.NewFlagSet(mode+" v2", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c sliceConfig
	var profile profileFlags
	var manifestPath string
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
		decoded, decodeErr := seedbinding.DecodeV2(bindingBytes)
		if decodeErr != nil {
			return fail(fmt.Errorf("seed binding invalid"))
		}
		binding, bindingRevision = &decoded, seedbinding.HostReceiptAuthority{}
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}, Diagnostics: diagnosticStore, SeedRevisionAuthority: bindingRevision})
	if err != nil {
		return fail(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, deadlineCancel := context.WithTimeout(ctx, effective.Limits.Timeout)
	defer deadlineCancel()
	started := manager.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.languageID, SeedBinding: binding, Process: managedprocess.Spec{Path: command, Args: c.args, Dir: workspace, Env: append(os.Environ(), c.env...)}})
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
	input, _ := json.Marshal(acquisitionops.Input{SessionID: started.SessionID, Generation: started.Generation, SeedManifest: manifest})
	privateRuntime := &privateAcquisitionRuntime{manager: manager}
	result, failed := acquisitionops.NewExecutor(privateRuntime).Execute(ctx, operation.Request{Name: op, Input: input})
	if failed != nil {
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
		err = publishValidatedBundle(c.output, data, admit)
	} else {
		_, err = stdout.Write(data)
	}
	if err != nil {
		return fail(err)
	}
	// Private finalization is synchronous but secondary and non-overriding: the
	// already-emitted public V3 bytes and their status remain authoritative.
	if requestDiagnosticsRequested {
		query := privateRuntime.privateQuery()
		privateRaw, privateErr := projectPrivateRequestDiagnostic(string(started.AttemptID), started.SessionID, started.Generation, query, data, privateMaxRecords, privateMaxBytes)
		if privateErr == nil {
			privateErr = publishPrivateRequestDiagnostic(requestDiagnosticRoot, requestDiagnosticSelector, privateRaw)
		}
		if privateErr != nil {
			fmt.Fprintln(stderr, "private request diagnostics unavailable")
		}
	}
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
