package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/incomingops"
	"lsp-trace/internal/custodyevidence"
	executionruntime "lsp-trace/internal/execution"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/observationadapter"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/provider"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/session"
	"lsp-trace/lifecycleops"
	"lsp-trace/sessionruntime"
	"lsp-trace/sliceops"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lsp-trace-mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	enableLiveLSP := fs.Bool("enable-live-lsp", false, "enable accepted persistent live-LSP tools")
	publicationRootPath := fs.String("publication-root", "", "permit output_selector publication beneath this pinned root")
	bootstrapConfigPath := fs.String("bootstrap-config", "", "host-owned managed-process startup configuration")
	custodyTrustPath := fs.String("custody-trust-config", "", "host-owned policy-pinned operational custody grants")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "lsp-trace-mcp accepts no positional arguments")
		return 2
	}
	fmt.Fprintln(stderr, "WARNING: local LSP child processes run with the developer's permissions, are not sandboxed, may access local files and network, and must be trusted.")
	var publicationRoot *publication.Root
	if *publicationRootPath != "" {
		var err error
		publicationRoot, err = publication.OpenRoot(*publicationRootPath)
		if err != nil {
			fmt.Fprintln(stderr, "publication root:", err)
			return 1
		}
		defer publicationRoot.Close()
	}
	var config *bootstrapConfig
	var provisioned provider.Provisioned
	inventory := provider.NewConfiguredInventory(provisioned)
	if *bootstrapConfigPath != "" {
		loaded, err := loadBootstrapConfig(*bootstrapConfigPath)
		if err != nil {
			fmt.Fprintln(stderr, "bootstrap config:", err)
			return 1
		}
		config = &loaded
		if len(loaded.Providers) != 0 {
			provisioned, err = loaded.provisionProviders()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			inventory = provider.NewConfiguredInventory(provisioned)
		}
	}
	custodyTrust, err := executionruntime.LoadHostTrustStore(*custodyTrustPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var seedValidator seedbinding.Validator
	var seedRevision seedbinding.RevisionAuthority
	if config != nil && config.SeedValidator != nil {
		external, validatorErr := seedbinding.NewExternalValidator(*config.SeedValidator)
		if validatorErr != nil {
			fmt.Fprintln(stderr, "seed validator:", validatorErr)
			return 1
		}
		seedValidator = external
		seedRevision = seedbinding.ExactRevisionAuthority{Revision: seedRevisionFromConfig(*config)}
	}
	server, manager, err := newServerRuntimeWithSeedAuthorities(*enableLiveLSP, inventory, custodyTrust, seedRevision, seedValidator, publicationRoot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var bootstrapSessions []bootstrapSession
	if config != nil {
		bootstrapSessions, err = startBootstrap(context.Background(), manager, *config, 10*time.Second)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		selected := newHostSelectorRuntime(manager, bootstrapSessions)
		if len(config.Providers) == 0 {
			composeHostSelectorExecutors(server, selected)
		} else {
			adapterIdentity := observationadapter.Identity{Name: "lsp-trace-observation-adapter", Version: "1"}
			admitter, err := provider.NewAdmissionResolver(provisioned, adapterIdentity.Name+"@"+adapterIdentity.Version)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			gitRevisions := make([]provider.ManagedGitRevision, 0, len(bootstrapSessions))
			for _, session := range bootstrapSessions {
				if session.RepositoryRoot != "" && session.GitCommit != "" {
					revision := provider.ManagedGitRevision{SessionID: session.SessionID, Generation: session.Generation, RepositoryRoot: session.RepositoryRoot, Commit: session.GitCommit}
					gitRevisions = append(gitRevisions, revision)
					if session.Alias != "" {
						revision.SessionID = session.Alias
						gitRevisions = append(gitRevisions, revision)
					}
				}
			}
			adapter, err := provider.NewObservationSemanticAdapter(provisioned, adapterIdentity, provider.NewGitRevisionVerifier(gitRevisions))
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if _, err := composeMCPProviderLifecycle(selected, provisioned.Registry, admitter, adapter, func(runtime *hostSelectorRuntime) {
				composeHostSelectorExecutors(server, runtime)
			}); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
	}
	serveErr := server.Serve(stdin, stdout)
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownErr := stopBootstrap(shutdownContext, manager, bootstrapSessions)
	if serveErr != nil {
		fmt.Fprintln(stderr, serveErr)
		return 1
	}
	if shutdownErr != nil {
		fmt.Fprintln(stderr, shutdownErr)
		return 1
	}
	return 0
}

type hostSelectorRuntime struct {
	*sessionruntime.Manager
	aliases           map[string]string
	relationCollector productionRelationCollector
}

func newHostSelectorRuntime(manager *sessionruntime.Manager, sessions []bootstrapSession) *hostSelectorRuntime {
	aliases := make(map[string]string, len(sessions))
	for _, started := range sessions {
		if started.Alias != "" {
			aliases[started.Alias] = started.SessionID
		}
	}
	return &hostSelectorRuntime{Manager: manager, aliases: aliases}
}

func composeHostSelectorRuntime(server *mcp.Server, manager *sessionruntime.Manager, sessions []bootstrapSession) *hostSelectorRuntime {
	selected := newHostSelectorRuntime(manager, sessions)
	composeHostSelectorExecutors(server, selected)
	return selected
}

func composeHostSelectorExecutors(server *mcp.Server, selected *hostSelectorRuntime) {
	server.Executors[mcp.LifecycleExecutorFamily] = lifecycleops.NewExecutor(lifecycleops.New(selected))
	server.Executors[mcp.IncomingExecutorFamily] = incomingops.NewExecutor(selected)
	server.Executors[mcp.SliceExecutorFamily] = sliceops.NewExecutor(selected)
	server.Executors[mcp.AcquisitionV2ExecutorFamily] = acquisitionops.NewExecutor(selected)
}

func (r *hostSelectorRuntime) ResolveSessionSelector(id string, generation uint64) (string, uint64, session.Failure) {
	if canonical, ok := r.aliases[id]; ok {
		id = canonical
	}
	if generation != 0 {
		return id, generation, ""
	}
	var match *sessionruntime.Record
	for _, record := range r.Records() {
		if record.SessionID != id {
			continue
		}
		if match != nil {
			return "", 0, session.Failure("AMBIGUOUS_SESSION_SELECTOR")
		}
		copy := record
		match = &copy
	}
	if match == nil {
		return "", 0, session.SessionNotFound
	}
	if match.State != session.Ready {
		return "", 0, session.Failure("SESSION_NOT_READY")
	}
	return match.SessionID, match.Generation, ""
}

func newServer(enableLiveLSP bool, roots ...*publication.Root) (*mcp.Server, error) {
	server, _, err := newServerRuntime(enableLiveLSP, roots...)
	return server, err
}

func newServerRuntime(enableLiveLSP bool, roots ...*publication.Root) (*mcp.Server, *sessionruntime.Manager, error) {
	return newServerRuntimeWithInventory(enableLiveLSP, provider.ConfiguredInventory{}, roots...)
}

func newServerRuntimeWithInventory(enableLiveLSP bool, inventory provider.ConfiguredInventory, roots ...*publication.Root) (*mcp.Server, *sessionruntime.Manager, error) {
	return newServerRuntimeWithCustodyTrust(enableLiveLSP, inventory, nil, roots...)
}

func newServerRuntimeWithCustodyTrust(enableLiveLSP bool, inventory provider.ConfiguredInventory, trust *custodyevidence.HostTrustStore, roots ...*publication.Root) (*mcp.Server, *sessionruntime.Manager, error) {
	return newServerRuntimeWithSeedAuthorities(enableLiveLSP, inventory, trust, nil, nil, roots...)
}

func newServerRuntimeWithSeedAuthorities(enableLiveLSP bool, inventory provider.ConfiguredInventory, trust *custodyevidence.HostTrustStore, revision seedbinding.RevisionAuthority, seedValidator seedbinding.Validator, roots ...*publication.Root) (*mcp.Server, *sessionruntime.Manager, error) {
	var publicationRoot *publication.Root
	if len(roots) != 0 {
		publicationRoot = roots[0]
	}
	registry := mcp.NewRegistryWithProviderInventory(enableLiveLSP, publicationRoot != nil, inventory)
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		return nil, nil, err
	}
	handlers, err := operation.NewRequiredHandlers(map[operation.Name]operation.Handler{
		operation.Capabilities: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
			return operation.Result{Value: registry.Capabilities()}, nil
		},
		operation.SchemaGet: operation.SchemaGetHandler,
		operation.Validate:  operation.ValidateHandler,
		operation.Verify:    operation.NewVerifyHandler(commandCustodyLoader{}),
		operation.Inspect:   operation.NewInspectHandler(),
		operation.Filter:    operation.NewFilterHandler(),
	})
	if err != nil {
		return nil, nil, err
	}
	handlers[operation.InspectHydrated] = operation.InspectHydratedHandler
	handlers[operation.VerifyV2] = operation.NewVerifyV2Handler(commandCustodyLoader{})
	handlers[operation.VerifyRetainedCallsV2] = operation.NewVerifyRetainedCallsV2Handler(commandCustodyLoader{})
	handlers[operation.ExportRetainedCalls] = operation.ExportRetainedCallsHandler
	handlers[operation.ExportRetainedCallsV2] = operation.ExportRetainedCallsHandler
	handlers[operation.BoundedRetainedAnalysis] = operation.BoundedRetainedAnalysisHandler
	handlers[operation.BoundedRetainedMetrics] = operation.BoundedRetainedMetricsHandler
	handlers[operation.BoundedRetainedRanking] = operation.BoundedRetainedRankingHandler
	handlers[operation.CustodyExecute] = executionruntime.NewProductionExecutorWithTrust(trust).Execute
	var starter sessionruntime.Starter = sessionruntime.ManagedStarter{}
	if runtime.GOOS == "darwin" {
		supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 64 * 1024, GracePeriod: 250 * time.Millisecond})
		if err != nil {
			return nil, nil, err
		}
		starter = sessionruntime.ManagedStarter{Manager: supervisor}
	}
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 8, MaxRequests: 128, MaxChildren: 8, MaxCancels: 128, MaxTombstones: 128, MaxObservations: 1024, MaxOperations: 128},
		Starter: starter, ReadinessTimeout: 10 * time.Second, SeedRevisionAuthority: revision, SeedBindingValidator: seedValidator,
	})
	if err != nil {
		return nil, nil, err
	}
	return &mcp.Server{
		Registry: registry, Executor: operation.NewOffline(validator, handlers),
		Executors: map[mcp.ExecutorFamily]mcp.Executor{
			mcp.LifecycleExecutorFamily:     lifecycleops.NewExecutor(lifecycleops.New(manager)),
			mcp.IncomingExecutorFamily:      incomingops.NewExecutor(manager),
			mcp.SliceExecutorFamily:         sliceops.NewExecutor(manager),
			mcp.AcquisitionV2ExecutorFamily: acquisitionops.NewExecutor(manager),
		},
		PublicationRoot: publicationRoot, Publisher: publication.NewPublisher(),
	}, manager, nil
}
