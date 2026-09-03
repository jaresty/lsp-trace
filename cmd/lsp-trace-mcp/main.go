package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/lifecycleops"
	"lsp-trace/sessionruntime"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lsp-trace-mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	enableLiveLSP := fs.Bool("enable-live-lsp", false, "enable accepted persistent live-LSP tools")
	publicationRootPath := fs.String("publication-root", "", "permit output_selector publication beneath this pinned root")
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
	server, err := newServer(*enableLiveLSP, publicationRoot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := server.Serve(stdin, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func newServer(enableLiveLSP bool, roots ...*publication.Root) (*mcp.Server, error) {
	server, _, err := newServerRuntime(enableLiveLSP, roots...)
	return server, err
}

func newServerRuntime(enableLiveLSP bool, roots ...*publication.Root) (*mcp.Server, *sessionruntime.Manager, error) {
	var publicationRoot *publication.Root
	if len(roots) != 0 {
		publicationRoot = roots[0]
	}
	registry := mcp.NewRegistryWithPublication(enableLiveLSP, publicationRoot != nil)
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
		Starter: starter, ReadinessTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, nil, err
	}
	return &mcp.Server{
		Registry: registry, Executor: operation.NewOffline(validator, handlers),
		Executors: map[mcp.ExecutorFamily]mcp.Executor{
			mcp.LifecycleExecutorFamily: lifecycleops.NewExecutor(lifecycleops.New(manager)),
			mcp.IncomingExecutorFamily:  incomingops.NewExecutor(manager),
		},
		PublicationRoot: publicationRoot, Publisher: publication.NewPublisher(),
	}, manager, nil
}
