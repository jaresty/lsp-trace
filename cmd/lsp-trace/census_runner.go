package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

const censusInvocationUsage = "lsp-trace census --workspace PATH (--server COMMAND | --profile NAME [--config PATH]) --publication-root ABSOLUTE_PATH [--source PATH...] [--include PATTERN...] [--exclude PATTERN...] [--server-arg VALUE...] [--down-depth N] [--up-depth N] [--max-nodes N] [--timeout DURATION] [--request-timeout DURATION] [--machine]"
const censusUsage = "usage: " + censusInvocationUsage

type censusRunnerDependencies struct {
	openRoot   func(string) (*publication.Root, error)
	lookPath   func(string) (string, error)
	abs        func(string) (string, error)
	newManager func(sessionruntime.Config) (*sessionruntime.Manager, error)
	newStarter func() (sessionruntime.Starter, error)
	runCore    func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome
}

func productionCensusRunnerDependencies() censusRunnerDependencies {
	return censusRunnerDependencies{
		openRoot: publication.OpenRoot,
		lookPath: exec.LookPath,
		abs:      filepath.Abs,
		newManager: func(cfg sessionruntime.Config) (*sessionruntime.Manager, error) {
			return sessionruntime.New(cfg)
		},
		newStarter: func() (sessionruntime.Starter, error) {
			supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
			if err != nil {
				return nil, err
			}
			return sessionruntime.ManagedStarter{Manager: supervisor}, nil
		},
		runCore: runCensusCore,
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

	profile, err := loadRequestedProfile(options.Workspace, profileFlags{Name: options.Profile, ConfigPath: options.ConfigPath})
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
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
	starter, err := deps.newStarter()
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	manager, err := deps.newManager(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 128, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 128, MaxObservations: 64}, Starter: starter})
	if err != nil {
		return writeCensusFailure(stderr, censusStageConfig, options.Machine, err)
	}
	maxRequests, maxEvidenceBytes, maxPathWork := 1000, 4<<20, 100000
	maxResponseBytes, maxMessages := 4<<20, 64
	timeoutMS, requestTimeoutMS := int(options.Timeout.Milliseconds()), int(options.RequestTimeout.Milliseconds())
	if timeoutMS > 60000 {
		timeoutMS = 60000
	}
	limits := acquisitionops.Limits{MaxNodes: &options.MaxNodes, MaxRequests: &maxRequests, MaxEvidenceBytes: &maxEvidenceBytes, MaxPathWork: &maxPathWork, TimeoutMS: &timeoutMS, RequestTimeoutMS: &requestTimeoutMS, MaxResponseBytes: &maxResponseBytes, MaxMessages: &maxMessages}
	outcome := deps.runCore(options, censusCoreConfig{runner: initializedAcquisitionRunnerConfig{manager: manager, start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: profile.LanguageID, Process: managedprocess.Spec{Path: command, Args: serverArgs, Dir: workspace, Env: append(os.Environ(), profile.Environment...)}}, timeout: options.Timeout, requestTimeout: options.RequestTimeout, stderr: io.Discard}, limits: limits, callHierarchy: true}, productionCensusCoreDependencies())
	return writeCensusOutcome(stdout, stderr, options.Machine, outcome)
}

func writeCensusFailure(stderr io.Writer, stage censusFailureStage, machine bool, err error) int {
	if machine {
		diagnostic, _ := buildCensusCLIDiagnostic(stage, nil)
		raw, _ := marshalCensusCLIDiagnostic(diagnostic)
		_, _ = stderr.Write(raw)
	} else {
		fmt.Fprintln(stderr, err)
	}
	return 1
}

func writeCensusOutcome(stdout, stderr io.Writer, machine bool, outcome censusPublicationOutcome) int {
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
