package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
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
	var publicationRoot *publication.Root
	if len(roots) != 0 {
		publicationRoot = roots[0]
	}
	registry := mcp.NewRegistryWithPublication(enableLiveLSP, publicationRoot != nil)
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	return &mcp.Server{
		Registry: registry, Executor: operation.NewOffline(validator, handlers),
		PublicationRoot: publicationRoot, Publisher: publication.NewPublisher(),
	}, nil
}
