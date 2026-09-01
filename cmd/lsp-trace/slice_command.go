package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/jsonrpc"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/server"
	"lsp-trace/internal/slicer"
	"lsp-trace/internal/source"
	"lsp-trace/internal/traverse"
)

type sliceConfig struct {
	workspace, command, fromFile, languageID, output string
	args, env                                        stringsFlag
	downDepth, upDepth, maxNodes                     int
	timeout, requestTimeout                          time.Duration
	pretty                                           bool
}

func parseSlice(args []string) (sliceConfig, error) {
	var c sliceConfig
	fs := flag.NewFlagSet("slice", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&c.workspace, "workspace", "", "workspace path")
	fs.StringVar(&c.command, "server", "", "language server command")
	fs.Var(&c.args, "server-arg", "repeatable server argument")
	fs.Var(&c.env, "server-env", "repeatable KEY=VALUE")
	fs.StringVar(&c.fromFile, "from-file", "", "starting source file")
	fs.StringVar(&c.languageID, "language-id", "", "document language id")
	fs.IntVar(&c.downDepth, "down-depth", 2, "outgoing discovery depth")
	fs.IntVar(&c.upDepth, "up-depth", 100, "incoming traversal depth; 0 unlimited")
	fs.IntVar(&c.maxNodes, "max-nodes", 10000, "maximum graph nodes; 0 unlimited")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Minute, "global timeout; 0 unlimited")
	fs.DurationVar(&c.requestTimeout, "request-timeout", 30*time.Second, "request timeout")
	fs.StringVar(&c.output, "output", "", "output file")
	fs.BoolVar(&c.pretty, "pretty", false, "pretty JSON")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() != 0 {
		return c, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if c.workspace == "" || c.command == "" || c.fromFile == "" {
		return c, fmt.Errorf("--workspace, --server, and --from-file are required")
	}
	if c.downDepth < 0 || c.upDepth < 0 || c.maxNodes < 0 {
		return c, fmt.Errorf("depth and node limits must be non-negative")
	}
	if c.requestTimeout <= 0 {
		return c, fmt.Errorf("request-timeout must be greater than zero")
	}
	for _, declaration := range c.env {
		key, _, ok := strings.Cut(declaration, "=")
		if !ok || key == "" {
			return c, fmt.Errorf("invalid --server-env %q", declaration)
		}
	}
	return c, nil
}

func runSlice(args []string) int {
	cfg, err := parseSlice(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if cfg.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}
	workspaceURI, sourceURI, resolvedPath, err := source.ResolveTarget(cfg.workspace, cfg.fromFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	text, err := os.ReadFile(resolvedPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	proc, err := server.Start(ctx, cfg.command, cfg.args, cfg.env)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer proc.Stop(2 * time.Second)
	client := lsp.NewClient(jsonrpc.New(proc.Stdout, proc.Stdin))
	rctx, done := context.WithTimeout(ctx, cfg.requestTimeout)
	err = client.Initialize(rctx, workspaceURI)
	done()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Shutdown(context.Background())
	languageID := cfg.languageID
	if languageID == "" {
		languageID = source.LanguageID(cfg.fromFile)
	}
	if err := client.DidOpen(sourceURI, languageID, string(text)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	timed := requestTimeoutClient{parent: ctx, timeout: cfg.requestTimeout, client: client}
	discovery := slicer.Discover(ctx, timed, sourceURI, slicer.Options{DownDepth: cfg.downDepth, MaxNodes: cfg.maxNodes})
	down := graph.Result{SchemaVersion: graph.SchemaVersionV3, Nodes: discovery.Nodes, Edges: discovery.Edges, Diagnostics: discovery.Diagnostics, Summary: graph.Summary{Complete: discovery.Complete, Truncated: discovery.Truncated}}
	down.Canonicalize()
	up := graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: true}}
	if len(discovery.FrontierItems) > 0 && cfg.upDepth > 0 {
		upMaxNodes := 0
		if cfg.maxNodes > 0 {
			remaining := cfg.maxNodes - len(discovery.Nodes)
			if remaining < 0 {
				remaining = 0
			}
			upMaxNodes = len(discovery.FrontierItems) + remaining
		}
		up = traverse.IncomingPrepared(ctx, timed, discovery.FrontierItems, traverse.Options{MaxDepth: cfg.upDepth, MaxNodes: upMaxNodes, SchemaVersion: graph.SchemaVersionV3})
	}
	result := graph.MergeResults(down, up)
	layers := make([]graph.SliceLayer, len(discovery.Layers))
	for i, layer := range discovery.Layers {
		layers[i] = graph.SliceLayer{Depth: layer.Depth, NodeIDs: append([]string(nil), layer.NodeIDs...)}
	}
	frontierIDs := []string{}
	if cfg.downDepth < len(layers) {
		frontierIDs = append(frontierIDs, layers[cfg.downDepth].NodeIDs...)
	}
	outgoingIDs := make([]string, len(down.Edges))
	for i, edge := range down.Edges {
		outgoingIDs[i] = edge.RelationID
	}
	result.Slice = &graph.SliceEvidence{SourceURI: sourceURI, DownDepth: cfg.downDepth, UpDepth: cfg.upDepth, StartingNodeIDs: discovery.StartNodeIDs, Layers: layers, FrontierNodeIDs: frontierIDs, OutgoingRelationIDs: outgoingIDs}
	environment := map[string]string{}
	for _, declaration := range cfg.env {
		k, v, _ := strings.Cut(declaration, "=")
		environment[k] = v
	}
	workingDirectory, _ := os.Getwd()
	outputMode := "stdout"
	if cfg.output != "" {
		outputMode = "file"
	}
	sum := sha256.Sum256(text)
	result.Invocation = graph.Invocation{
		WorkspaceURI: workspaceURI, WorkingDirectory: workingDirectory, EffectiveEnvironment: append(os.Environ(), cfg.env...),
		Target: graph.Target{URI: sourceURI, Line: 1, Column: 1}, Server: graph.ServerInvocation{Command: cfg.command, Arguments: cfg.args, Environment: environment},
		Limits: graph.Limits{MaxDepth: cfg.upDepth, MaxNodes: cfg.maxNodes, TimeoutMS: cfg.timeout.Milliseconds()}, RequestTimeoutMS: cfg.requestTimeout.Milliseconds(), Concurrency: 1,
		LanguageID: languageID, OutputMode: outputMode, OutputPath: cfg.output,
		Seeds: []graph.InvocationSeed{{Label: "slice-source", At: cfg.fromFile + ":1:1", ResolvedURI: sourceURI, ContentSHA256: fmt.Sprintf("sha256:%x", sum[:]), LanguageID: languageID}},
	}
	result.Seeds = []graph.SeedResult{{Label: "slice-source", Requested: result.Invocation.Target, PreparedTargetIDs: discovery.StartNodeIDs}}
	result.Tool = graph.ToolIdentity{Name: "lsp-trace", Version: graph.Unknown}
	result.Capabilities.CallHierarchyProvider = client.SupportsCallHierarchy()
	result.CapabilityQuality.Advertised = result.Capabilities.CallHierarchyProvider
	result.Canonicalize()
	data, err := marshalResult(result, cfg.pretty)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintf(os.Stderr, "%s: %s\n", diagnostic.Phase, diagnostic.Message)
	}
	if cfg.output == "" {
		_, err = os.Stdout.Write(data)
	} else {
		err = publishBundle(cfg.output, data)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !result.Summary.Complete {
		return 2
	}
	return 0
}
