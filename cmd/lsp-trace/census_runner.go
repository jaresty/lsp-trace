package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censuscontrolflow"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censuspreflight"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

const censusInvocationUsage = "lsp-trace census [FOLDER] [--machine] [scope/limit flags] (caller-hosted inventory entry; semantic handoff unavailable)\n  Legacy explicit acquisition: " + censusLegacyInvocationUsage
const censusLegacyInvocationUsage = "lsp-trace census (((--workspace PATH (--server COMMAND | --profile NAME [--config PATH]) --publication-root ABSOLUTE_PATH [--catalog --catalog-config HOST_JSON] [--stop-after describe-requests] [--source PATH...] [--include PATTERN...] [--exclude PATTERN...] [--prune-dir DIR...] [--server-arg VALUE...] [--down-depth N] [--up-depth N] [--max-nodes N] [--batch-targets N] [--timeout DURATION] [--request-timeout DURATION]) | (--workspace PATH [--source PATH...] [--include PATTERN...] [--exclude PATTERN...] [--prune-dir DIR...] --enumeration-preflight --machine)) | (--resume SELECTOR --catalog-config HOST_JSON --publication-root ABSOLUTE_PATH [--workspace PATH] [--stop-after describe-requests])) [--machine]"
const censusUsage = "usage: lsp-trace census [FOLDER] [--machine] [--server COMMAND | --profile NAME [--config PATH]] [--publication-root ABSOLUTE_PATH] [scope/limit flags]\nNormal inventory entry: FOLDER defaults to the current directory. Automatic Go/gopls selection requires root go.mod or go.work and no competing root language marker. Explicit server/profile selection is supported.\nRuns use fresh private storage under the OS per-user configuration/application-support directory (lsp-trace/census), or beneath --publication-root; storage must be outside the workspace. Failed runs are retained.\nCaller-hosted semantic inventory handoff is currently unavailable. Normal folder entry fails before native catalog acquisition/assembly, never reports inventory success, and requires no provider credentials. Explicit --workspace acquisition remains a separate legacy mode.\nLegacy explicit acquisition (selected with --workspace; without --catalog it does not imply inventory):\n       " + censusLegacyInvocationUsage + "\n       lsp-trace census --catalog --caller-host --catalog-config ABSOLUTE_PATH --machine\n       Add --save-caller-host-settings ABSOLUTE_PATH to export non-expiring settings without execution.\n       With saved settings, supply --caller-host-attempt-config ABSOLUTE_PATH for separately admitted fresh execution state.\nCaller-HOST catalog: connected bridge configuration owns source, publication root, authorization and lifecycle action; ordinary acquisition flags are rejected. Resume through the configuration's frozen RunID and lifecycle action, not legacy --resume.\n       lsp-trace census --machine --private-inventory-qualified-prepare-config ABSOLUTE_PATH\nQualified preparation only: strict caller-declared complete universe, original bindings and explicit selection; private bounded artifacts; no provider startup or execution admission."

type censusRunnerDependencies struct {
	resolveFolder            func(censusCLIOptions) (censusCLIOptions, error)
	openRoot                 func(string) (*publication.Root, error)
	lookPath                 func(string) (string, error)
	abs                      func(string) (string, error)
	newManager               func(sessionruntime.Config) (*sessionruntime.Manager, error)
	shutdownManager          func(*sessionruntime.Manager) error
	newStarter               func() (sessionruntime.Starter, error)
	runCore                  func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome
	runNativeCatalog         func(censusCLIOptions, censusCoreConfig, io.Writer, io.Writer) int
	runPrivateInventory      func(censusCLIOptions, censusCoreConfig, io.Writer, io.Writer) int
	openCatalogHost          func(string, string) (*continuationhost.Bundle, error)
	runCatalog               func(context.Context, censuscontinuation.Request) censuscontinuation.Result
	resumeCatalog            func(context.Context, censuscontinuation.ResumeRequest) censuscontinuation.Result
	publishCatalogDescriptor func(context.Context, *continuationhost.Bundle, continuationhost.DescriptorInput) (string, error)
	resolveCatalogDescriptor func(context.Context, *continuationhost.Bundle, string) (continuationhost.DescriptorInput, error)
	buildCatalogContract     func(string) (censuscontinuation.ContinuationContract, error)
	newCatalogWorker         func(string, string) censuscontinuation.WorkerV2
	enumeratePreflight       func(censusCLIOptions) ([]censusacquisition.SourceFile, error)
}

func productionCensusRunnerDependencies() censusRunnerDependencies {
	return censusRunnerDependencies{
		resolveFolder: resolveCensusFolderInvocation,
		openRoot:      publication.OpenRoot,
		lookPath:      exec.LookPath,
		abs:           filepath.Abs,
		newManager: func(cfg sessionruntime.Config) (*sessionruntime.Manager, error) {
			return sessionruntime.New(cfg)
		},
		shutdownManager: func(manager *sessionruntime.Manager) error {
			return manager.Shutdown(context.Background())
		},
		newStarter: func() (sessionruntime.Starter, error) {
			supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
			if err != nil {
				return nil, err
			}
			return sessionruntime.ManagedStarter{Manager: supervisor}, nil
		},
		runCore:                  runCensusCore,
		runNativeCatalog:         runCensusNativeCatalog,
		runPrivateInventory:      runCensusInventoryPrivate,
		openCatalogHost:          openCensusCatalogHost,
		runCatalog:               censuscontinuation.Run,
		resumeCatalog:            censuscontinuation.Resume,
		publishCatalogDescriptor: publishCensusCatalogDescriptor,
		resolveCatalogDescriptor: resolveCensusCatalogDescriptor,
		buildCatalogContract:     censusCatalogContract,
		newCatalogWorker:         newLazyCensusCatalogWorker,
		enumeratePreflight:       enumerateCensusPreflight,
	}
}

func runCensus(args []string, stdout, stderr io.Writer) int {
	return runCensusWithDependencies(args, stdout, stderr, productionCensusRunnerDependencies())
}

func runCensusWithDependencies(args []string, stdout, stderr io.Writer, deps censusRunnerDependencies) int {
	options, err := parseCensusCLIOptions(append([]string{"census"}, args...))
	if err != nil {
		return writeCensusFailure(stderr, censusStageSyntax, options.Machine, err)
	}
	if options.Help {
		fmt.Fprintln(stdout, censusUsage)
		return 0
	}
	if options.CallerHost {
		// Use the existing connected partition/target lifecycle, including its
		// funding, custody and publication readback checks. Do not construct a
		// legacy catalog worker or reconstruct native authority from discovery.
		return runCensusCallerHost(options, stdout, stderr, deps)
	}
	if options.PrivateInventoryQualifiedPrepareConfigPath != "" {
		return runCensusApplicationQualifiedCLIPrivate(options, stdout, stderr)
	}
	if options.PrivateInventoryBridgeConfigPath != "" {
		return runCensusInventoryBridgePrivate(options, stdout, stderr, deps)
	}
	if options.EnumerationPreflight {
		return runCensusEnumerationPreflight(options, stdout, stderr, deps.enumeratePreflight)
	}
	if options.Resume != "" {
		return runCensusCatalogResume(options, stdout, stderr, deps)
	}

	if options.FolderInvocation {
		resolve := deps.resolveFolder
		if resolve == nil {
			resolve = resolveCensusFolderInvocation
		}
		options, err = resolve(options)
		if err != nil {
			return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
		}
		stderr = censusFolderDiagnosticWriter{Writer: stderr, runPath: options.PublicationRoot}
	}
	profile, err := loadRequestedProfile(options.Workspace, profileFlags{Name: options.Profile, ConfigPath: options.ConfigPath})
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	if options.FolderInvocation && options.Profile == "" && options.Server == "gopls" {
		profile.LanguageID = "go"
	}
	command := options.Server
	if command == "" {
		command = profile.Command
	}
	serverArgs := append([]string(nil), options.ServerArgs...)
	if len(serverArgs) == 0 {
		serverArgs = append(serverArgs, profile.Args...)
	}
	if command == "" {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, fmt.Errorf("server or profile command is required"))
	}

	workspace, err := deps.abs(options.Workspace)
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	workspace = filepath.Clean(workspace)
	options.Workspace = workspace
	root, err := deps.openRoot(options.PublicationRoot)
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	if err := root.Close(); err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	command, err = deps.lookPath(command)
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	command, err = deps.abs(command)
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "public-acquisition", Workspace: workspace, Profile: "cli", EnvironmentReference: "cli"})
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	var diagnostic *censusdiagnostic.Recorder
	if options.PrivateAcquisitionDiagnosticPath != "" {
		diagnostic, err = censusdiagnostic.NewRecorder(options.PrivateAcquisitionDiagnosticPath, censusdiagnostic.DefaultMaxBytes, censusdiagnostic.DefaultMaxRecords)
		if err != nil {
			return writeCensusFailure(stderr, censusStageConfig, options.Machine, errors.New("invalid census configuration"))
		}
	}
	var controlFlow *censuscontrolflow.Recorder
	if options.PrivateControlFlowDiagnosticPath != "" {
		controlFlow, err = censuscontrolflow.NewRecorder(options.PrivateControlFlowDiagnosticPath, censuscontrolflow.DefaultMaxBytes, censuscontrolflow.DefaultMaxRecords)
		if err != nil {
			return writeCensusFailure(stderr, censusStageConfig, options.Machine, errors.New("invalid census configuration"))
		}
	}
	var providerProbe *privateProviderProbe
	if options.PrivateProviderProbePath != "" {
		providerProbe, err = newPrivateProviderProbeVersion(options.PrivateProviderProbePath, options.PrivateProviderProbeVersion)
		if err != nil {
			return writeCensusFailure(stderr, censusStageConfig, options.Machine, errors.New("invalid census configuration"))
		}
		providerProbe.byteTimeNotifications = options.PrivateProviderProbeByteTimeNotifications
		defer func() { _ = providerProbe.finalize() }()
	}
	maxRequests, err := censusacquisition.AcquisitionMaxRequests(options.PrivateEnumerationProjection, options.MaxNodes, options.MaxBatchTargets)
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, errors.New("invalid census configuration"))
	}
	starter, err := deps.newStarter()
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	manager, err := deps.newManager(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64}, Starter: starter})
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	shutdown := deps.shutdownManager
	if shutdown == nil {
		shutdown = func(manager *sessionruntime.Manager) error { return manager.Shutdown(context.Background()) }
	}
	maxEvidenceBytes, maxPathWork := 4<<20, 100000
	maxResponseBytes, maxMessages := 4<<20, 64
	timeoutMS, requestTimeoutMS := int(options.Timeout.Milliseconds()), int(options.RequestTimeout.Milliseconds())
	if timeoutMS > 60000 {
		timeoutMS = 60000
	}
	limits := acquisitionops.Limits{MaxNodes: &options.MaxNodes, MaxRequests: &maxRequests, MaxEvidenceBytes: &maxEvidenceBytes, MaxPathWork: &maxPathWork, TimeoutMS: &timeoutMS, RequestTimeoutMS: &requestTimeoutMS, MaxResponseBytes: &maxResponseBytes, MaxMessages: &maxMessages}
	var controlFlowObserver *censusControlFlowObserver
	if controlFlow != nil {
		controlFlowObserver = newCensusControlFlowObserver()
	}
	cfg := censusCoreConfig{runner: initializedAcquisitionRunnerConfig{manager: manager, providerProbe: providerProbe, privateEnumerationProjection: options.PrivateEnumerationProjection, start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: profile.LanguageID, Process: managedprocess.Spec{Path: command, Args: serverArgs, Dir: workspace, Env: append(os.Environ(), profile.Environment...)}}, timeout: options.Timeout, requestTimeout: options.RequestTimeout, stderr: io.Discard}, limits: limits, callHierarchy: true, diagnostic: diagnostic, controlFlow: controlFlowObserver}
	code := 0
	if options.PrivateInventoryConfigPath != "" {
		// Only the private route may construct a subsequent original owner. Each
		// factory call uses the same admitted process/profile, never a saved DTO.
		cfg.privateFreshRunner = func() (initializedAcquisitionRunnerConfig, error) {
			s, err := deps.newStarter()
			if err != nil {
				return initializedAcquisitionRunnerConfig{}, err
			}
			m, err := deps.newManager(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64}, Starter: s})
			if err != nil {
				return initializedAcquisitionRunnerConfig{}, err
			}
			next := cfg.runner
			next.manager = m
			return next, nil
		}
		// The private route owns every join, including the unused first owner.
		return deps.runPrivateInventory(options, cfg, stdout, stderr)
	} else if options.Catalog {
		code = deps.runNativeCatalog(options, cfg, stdout, stderr)
	} else {
		outcome := deps.runCore(options, cfg, productionCensusCoreDependencies())
		code = writeCensusOutcome(stdout, stderr, options.Machine, outcome)
		if controlFlow != nil && outcome.controlFlow != nil {
			_ = controlFlow.Record(*outcome.controlFlow)
		}
	}
	if options.Catalog && controlFlow != nil {
		if record, ok := controlFlowObserver.record(); ok {
			_ = controlFlow.Record(record)
		}
	}
	if manager != nil {
		if shutdownErr := shutdown(manager); shutdownErr != nil && code == 0 {
			return writeCensusFailure(stderr, censusStageAcquisition, options.Machine, shutdownErr)
		}
	}
	return code
}

// resolveCensusFolderInvocation is the normal-entry adapter only. The parser
// remains pure, and legacy explicit acquisition, resume and private routes never
// enter here. No host configuration or authority is manufactured.
func resolveCensusFolderInvocation(o censusCLIOptions) (censusCLIOptions, error) {
	refuse := func(reason string) (censusCLIOptions, error) {
		return o, fmt.Errorf("census folder configuration: %s", reason)
	}
	workspace, err := filepath.Abs(o.Workspace)
	if err != nil {
		return refuse("cannot resolve folder; supply an existing directory")
	}
	workspace, err = filepath.EvalSymlinks(workspace)
	if err != nil {
		return refuse("folder is missing or unreadable; supply an existing directory")
	}
	info, err := os.Stat(workspace)
	if err != nil || !info.IsDir() {
		return refuse("folder must be a directory")
	}
	o.Workspace = workspace
	for _, source := range o.Sources {
		resolved, e := filepath.EvalSymlinks(filepath.Join(workspace, source))
		if e != nil {
			return refuse("source directory is missing or unreadable: " + source)
		}
		info, e := os.Stat(resolved)
		if e != nil || !info.IsDir() || !censusPathWithin(workspace, resolved) {
			return refuse("source must be a directory within the workspace: " + source)
		}
	}
	if o.Server == "" && o.Profile == "" {
		goEvidence := false
		for _, name := range []string{"go.mod", "go.work"} {
			info, e := os.Lstat(filepath.Join(workspace, name))
			if e == nil && info.Mode().IsRegular() {
				goEvidence = true
			}
			if e != nil && !os.IsNotExist(e) {
				return refuse("cannot inspect Go workspace evidence; supply --server or --profile")
			}
		}
		if !goEvidence {
			return refuse("automatic profile requires a root go.mod or go.work; supply --server COMMAND or --profile NAME [--config PATH]")
		}
		for _, name := range []string{"package.json", "Cargo.toml", "pyproject.toml", "CMakeLists.txt"} {
			if _, e := os.Lstat(filepath.Join(workspace, name)); e == nil || !os.IsNotExist(e) {
				return refuse("ambiguous workspace profiles; supply --server COMMAND or --profile NAME [--config PATH]")
			}
		}
		o.Server = "gopls"
	}
	base := o.PublicationRoot
	if base == "" {
		base, err = os.UserConfigDir()
		if err != nil || !filepath.IsAbs(base) {
			return refuse("no durable per-user store; supply --publication-root ABSOLUTE_PRIVATE_DIRECTORY")
		}
		base, err = censusDefaultConfigDirectory(base, workspace)
		if err != nil {
			return refuse("unsafe per-user configuration ancestry; supply --publication-root ABSOLUTE_PRIVATE_DIRECTORY")
		}
		for _, child := range []string{"lsp-trace", "census"} {
			base, err = censusOwnedDirectory(base, child, false)
			if err != nil {
				return refuse("unsafe per-user census store; supply a private --publication-root outside the workspace")
			}
		}
	} else {
		if !filepath.IsAbs(base) || filepath.Clean(base) != base {
			return refuse("publication root must be a canonical absolute private directory")
		}
		canonical, e := filepath.EvalSymlinks(base)
		if e != nil || canonical != base {
			return refuse("publication root must not contain symlinks")
		}
	}
	if censusPathWithin(workspace, base) {
		return refuse("publication root must be outside the workspace to avoid scanning output")
	}
	root, err := publication.OpenRoot(base)
	if err != nil {
		return refuse("cannot open private census store; supply --publication-root ABSOLUTE_PRIVATE_DIRECTORY")
	}
	err = errors.Join(root.ValidatePrivate(), root.Close())
	if err != nil {
		return refuse("census store must be owned by the current user with mode 0700")
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return refuse("cannot allocate fresh census run")
	}
	o.PublicationRoot, err = censusOwnedDirectory(base, "run-"+hex.EncodeToString(nonce[:]), true)
	if err != nil {
		return refuse("cannot create fresh private census run (no overwrite)")
	}
	return o, nil
}

// Resolve only the OS-owned base aliases. Missing ancestry is derived from an
// existing canonical, user-owned, non-group/other-writable ancestor. No existing
// directory is chmodded; each new component is created through a pinned parent.
func censusDefaultConfigDirectory(base, workspace string) (string, error) {
	if !filepath.IsAbs(base) || filepath.Clean(base) != base {
		return "", errors.New("invalid config base")
	}
	ancestor := base
	var missing []string
	for {
		canonical, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			ancestor = canonical
			break
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(ancestor) == ancestor {
			return "", errors.New("unresolvable config ancestor")
		}
		// A dangling alias is not missing ancestry and must not be replaced.
		if _, e := os.Lstat(ancestor); !errors.Is(e, os.ErrNotExist) {
			return "", errors.New("unsafe config ancestor")
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = filepath.Dir(ancestor)
	}
	resolved := ancestor
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	if censusPathWithin(workspace, resolved) || censusPathWithin(filepath.Join(resolved, "lsp-trace", "census"), workspace) {
		return "", errors.New("config store overlaps source")
	}
	handle, err := os.OpenRoot(ancestor)
	if err != nil {
		return "", err
	}
	defer func() { _ = handle.Close() }()
	validate := func(h *os.Root) error {
		info, e := h.Stat(".")
		if e != nil {
			return e
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !info.IsDir() || info.Mode().Perm()&0022 != 0 || !ok || owner.Uid != uint32(os.Geteuid()) {
			return errors.New("unsafe config ancestor ownership or permissions")
		}
		return nil
	}
	if err := validate(handle); err != nil {
		return "", err
	}
	for i := len(missing) - 1; i >= 0; i-- {
		child := missing[i]
		// No adoption of a concurrently inserted component, including a symlink.
		if err := handle.Mkdir(child, 0700); err != nil {
			return "", err
		}
		before, err := handle.Lstat(child)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 || before.Mode().Perm()&0077 != 0 {
			return "", errors.New("unsafe new config directory")
		}
		next, err := handle.OpenRoot(child)
		if err != nil {
			return "", err
		}
		after, err := next.Stat(".")
		if err != nil || !os.SameFile(before, after) {
			_ = next.Close()
			return "", errors.New("config directory changed")
		}
		if err := validate(next); err != nil {
			_ = next.Close()
			return "", err
		}
		// Sync the new child and its pinned parent before advancing custody.
		for _, h := range []*os.Root{next, handle} {
			f, e := h.Open(".")
			if e == nil {
				e = errors.Join(f.Sync(), f.Close())
			}
			if e != nil {
				_ = next.Close()
				return "", e
			}
		}
		if err := handle.Close(); err != nil {
			_ = next.Close()
			return "", err
		}
		handle = next
	}
	return resolved, nil
}

func censusPathWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

// Create beneath a pinned directory, reject existing links and nonprivate
// children, then reuse publication ownership/no-follow validation and sync.
// Existing directories are never chmodded and fresh run names are never reused.
func censusOwnedDirectory(parent, child string, fresh bool) (string, error) {
	handle, err := os.OpenRoot(parent)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	err = handle.Mkdir(child, 0700)
	if err != nil && (fresh || !errors.Is(err, os.ErrExist)) {
		return "", err
	}
	before, err := handle.Lstat(child)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("unsafe census directory")
	}
	path := filepath.Join(parent, child)
	root, err := publication.OpenRoot(path)
	if err != nil {
		return "", err
	}
	defer root.Close()
	after, err := handle.Stat(child)
	if err != nil || !os.SameFile(before, after) {
		return "", errors.New("census directory changed")
	}
	if err := root.ValidatePrivate(); err != nil {
		return "", err
	}
	if _, err := root.SyncDirectory(); err != nil {
		return "", err
	}
	parentRoot, err := publication.OpenRoot(parent)
	if err != nil {
		return "", err
	}
	_, syncErr := parentRoot.SyncDirectory()
	if err := errors.Join(syncErr, parentRoot.Close()); err != nil {
		return "", err
	}
	return path, nil
}

func enumerateCensusPreflight(options censusCLIOptions) ([]censusacquisition.SourceFile, error) {
	return enumerateCensusPreflightContextPrivate(context.Background(), options)
}

// The private continuation supplies its operation/signal lifetime. The legacy
// ordinary Census adapter deliberately retains its existing background context.
func enumerateCensusPreflightContextPrivate(ctx context.Context, options censusCLIOptions) ([]censusacquisition.SourceFile, error) {
	workspace, err := filepath.Abs(options.Workspace)
	if err != nil {
		return nil, err
	}
	workspace = filepath.Clean(workspace)
	roots := append([]string(nil), options.Sources...)
	sort.Strings(roots)
	includes := append([]string(nil), options.Includes...)
	sort.Strings(includes)
	excludes := append([]string(nil), options.Excludes...)
	sort.Strings(excludes)
	return (censusWorkspaceEnumerator{workspace: workspace, roots: roots, filters: censusacquisition.Filters{Includes: includes, Excludes: excludes}, maxFiles: censusMaxDiscoveredEntries, prunedDirectories: options.PrunedDirectories}).Enumerate(ctx)
}

func runCensusEnumerationPreflight(options censusCLIOptions, stdout, stderr io.Writer, enumerate func(censusCLIOptions) ([]censusacquisition.SourceFile, error)) int {
	workspace, err := filepath.Abs(options.Workspace)
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, true, err)
	}
	workspace = filepath.Clean(workspace)
	roots := append([]string(nil), options.Sources...)
	sort.Strings(roots)
	includes := append([]string(nil), options.Includes...)
	sort.Strings(includes)
	excludes := append([]string(nil), options.Excludes...)
	sort.Strings(excludes)
	if enumerate == nil {
		return writeCensusFailure(stderr, censusStageConfig, true, errors.New("enumeration preflight unavailable"))
	}
	files, err := enumerate(options)
	if err != nil {
		var limit *source.LimitError
		if errors.As(err, &limit) {
			raw, marshalErr := censuspreflight.MarshalDiagnostic(censuspreflight.Diagnostic{SchemaVersion: censuspreflight.DiagnosticSchemaVersion, Category: limit.Category, Observed: limit.Observed, Limit: limit.Limit})
			if marshalErr == nil {
				_, _ = stderr.Write(append(raw, '\n'))
				return 1
			}
		}
		return writeCensusFailure(stderr, censusStageDiscovery, true, err)
	}
	selected := make([]string, 0, len(files))
	excluded := 0
	for _, file := range files {
		if !matchesAnyDiscoveryPattern(excludes, file.Path) && (len(includes) == 0 || matchesAnyDiscoveryPattern(includes, file.Path)) {
			selected = append(selected, file.Path)
		} else {
			excluded++
		}
	}
	sort.Strings(selected)
	result := censuspreflight.Result{SchemaVersion: censuspreflight.SchemaVersion, Roots: roots, Includes: includes, Excludes: excludes, PrunedDirectories: append([]string(nil), options.PrunedDirectories...), Counts: censuspreflight.Counts{Entries: len(files), Selected: len(selected), Excluded: excluded}, SelectedPathsDigest: censuspreflight.DigestSelected(selected), Limits: censuspreflight.Limits{MaxEntries: censusMaxDiscoveredEntries, MaxAccepted: censusMaxDiscoveredEntries, MaxWork: censusMaxEnumerationWork, MaxPathBytes: censusMaxSourcePathBytes, MaxDepth: censusMaxSourceDepth}, Terminal: "COMPLETE", Authority: 0, Completeness: "UNKNOWN", AbsenceInference: false}
	raw, err := censuspreflight.MarshalResult(result)
	if err != nil {
		return writeCensusFailure(stderr, censusStageAssembly, true, err)
	}
	_, _ = stdout.Write(append(raw, '\n'))
	return 0
}

// This context carries only a locator for a successfully allocated normal-folder
// run. It is not a grant, resume command, replay permission or inventory result.
// Embedding the writer leaves all other writes untouched, including native ones.
type censusFolderDiagnosticWriter struct {
	io.Writer
	runPath string
}

func writeCensusFailure(stderr io.Writer, stage censusFailureStage, machine bool, err error) int {
	locator := ""
	if folder, ok := stderr.(censusFolderDiagnosticWriter); ok {
		locator = "Retained census run directory: " + folder.runPath + "; locator only, not resume permission or completed inventory"
	}
	if machine {
		diagnostic, _ := buildCensusCLIDiagnostic(stage, nil)
		if errors.Is(err, errCensusCallerHostUnavailable) {
			detail := errCensusCallerHostUnavailable.Error()
			if locator != "" {
				detail += "\n" + locator
			}
			diagnostic = diagnostic.WithDetail(detail)
		} else if locator != "" {
			diagnostic = diagnostic.WithDetail(locator)
		}
		raw, _ := marshalCensusCLIDiagnostic(diagnostic)
		_, _ = stderr.Write(raw)
	} else {
		fmt.Fprintln(stderr, err)
		if locator != "" {
			fmt.Fprintln(stderr, locator)
		}
	}
	return 1
}

func writeCensusOutcome(stdout, stderr io.Writer, machine bool, outcome censusPublicationOutcome) int {
	if outcome.EnumerationDiagnostic != nil {
		raw, err := censuspreflight.MarshalDiagnostic(*outcome.EnumerationDiagnostic)
		if err == nil {
			_, _ = stderr.Write(append(raw, '\n'))
			return 1
		}
		return writeCensusFailure(stderr, censusStageDiscovery, machine, err)
	}
	if outcome.DiscoveryDiagnostic != nil && outcome.RequestReceiptV2 != nil {
		if machine {
			raw, err := json.Marshal(struct {
				Diagnostic     *censusresult.DiscoveryDiagnostic `json:"diagnostic"`
				RequestReceipt *censusrequest.ReceiptV2          `json:"request_receipt"`
			}{outcome.DiscoveryDiagnostic, outcome.RequestReceiptV2})
			if err == nil {
				_, _ = stderr.Write(append(raw, '\n'))
				if outcome.PrivateDiscoveryCauses != nil {
					privateRaw, privateErr := marshalPrivateCensusDiscoveryCauses(*outcome.PrivateDiscoveryCauses)
					if privateErr != nil {
						return writeCensusFailure(stderr, censusStageDiscovery, true, privateErr)
					}
					_, _ = stderr.Write(append(privateRaw, '\n'))
				}
				if outcome.PrivateTerminalObservation != nil {
					privateRaw, privateErr := marshalPrivateCensusTerminalObservation(*outcome.PrivateTerminalObservation)
					if privateErr != nil {
						return writeCensusFailure(stderr, censusStageDiscovery, true, privateErr)
					}
					_, _ = stderr.Write(append(privateRaw, '\n'))
				}
				return 1
			}
		}
		fmt.Fprintln(stderr, "census failed: DISCOVERY_BOUNDED_INCOMPLETE")
		return 1
	}
	if outcome.DiscoveryDiagnostic != nil && outcome.RequestReceipt != nil {
		if machine {
			raw, err := json.Marshal(struct {
				Diagnostic     *censusresult.DiscoveryDiagnostic `json:"diagnostic"`
				RequestReceipt *censusrequest.Receipt            `json:"request_receipt"`
			}{outcome.DiscoveryDiagnostic, outcome.RequestReceipt})
			if err == nil {
				_, _ = stderr.Write(append(raw, '\n'))
				if outcome.EnumerationReceipt != nil {
					privateRaw, privateErr := censusenumerationprojection.MarshalReceipt(*outcome.EnumerationReceipt)
					if privateErr == nil {
						_, _ = stderr.Write(privateRaw)
					}
				}
				if outcome.PrivateDiscoveryCauses != nil {
					privateRaw, privateErr := marshalPrivateCensusDiscoveryCauses(*outcome.PrivateDiscoveryCauses)
					if privateErr != nil {
						return writeCensusFailure(stderr, censusStageDiscovery, true, privateErr)
					}
					_, _ = stderr.Write(append(privateRaw, '\n'))
				}
				if outcome.PrivateTerminalObservation != nil {
					privateRaw, privateErr := marshalPrivateCensusTerminalObservation(*outcome.PrivateTerminalObservation)
					if privateErr != nil {
						return writeCensusFailure(stderr, censusStageDiscovery, true, privateErr)
					}
					_, _ = stderr.Write(append(privateRaw, '\n'))
				}
				return 1
			}
		}
		fmt.Fprintln(stderr, "census failed: DISCOVERY_BOUNDED_INCOMPLETE")
		return 1
	}
	if outcome.PrivateChunkTerminal != nil {
		if !machine {
			return writeCensusFailure(stderr, censusStageCommitted, false, fmt.Errorf("private census chunk terminal requires machine output"))
		}
		raw, err := marshalPrivateCensusChunkTerminal(*outcome.PrivateChunkTerminal)
		if err != nil {
			return writeCensusFailure(stderr, censusStageCommitted, true, err)
		}
		_, _ = stdout.Write(append(raw, '\n'))
		return 0
	}
	if outcome.Result == nil {
		if outcome.Diagnostic == nil {
			return writeCensusFailure(stderr, censusStageAcquisition, machine, fmt.Errorf("census failed"))
		}
		if machine {
			raw, _ := marshalCensusCLIDiagnostic(*outcome.Diagnostic)
			_, _ = stderr.Write(raw)
		} else {
			fmt.Fprintf(stderr, "census failed: %s\n", outcome.Diagnostic.Code)
		}
		return 1
	}
	if machine {
		raw, err := marshalCensusCLIResult(*outcome.Result)
		if err != nil {
			return writeCensusFailure(stderr, censusStageCommitted, true, err)
		}
		_, _ = stdout.Write(raw)
		return 0
	}
	fmt.Fprintf(stdout, "census succeeded: targets=%d batches=%d selector=%s\n", outcome.Result.TargetCount, outcome.Result.BatchCount, outcome.Result.Publication.Selector)
	return 0
}
