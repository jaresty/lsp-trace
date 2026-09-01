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
	workspace, command, fromFile, seedFile, languageID, output string
	args, env, ats                                             stringsFlag
	downDepth, upDepth, maxNodes                               int
	timeout, requestTimeout                                    time.Duration
	pretty                                                     bool
}

func parseSlice(args []string) (sliceConfig, error) {
	var c sliceConfig
	fs := flag.NewFlagSet("slice", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&c.workspace, "workspace", "", "workspace path")
	fs.StringVar(&c.command, "server", "", "language server command")
	fs.Var(&c.args, "server-arg", "repeatable server argument")
	fs.Var(&c.env, "server-env", "repeatable KEY=VALUE")
	fs.StringVar(&c.fromFile, "from-file", "", "enumerate starting symbols from one source file")
	fs.Var(&c.ats, "at", "repeatable PATH:LINE:COLUMN starting position")
	fs.StringVar(&c.seedFile, "seed-file", "", "JSON file containing labeled starting positions")
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
	if c.workspace == "" || c.command == "" {
		return c, fmt.Errorf("--workspace and --server are required")
	}
	modes := 0
	if c.fromFile != "" {
		modes++
	}
	if len(c.ats) > 0 {
		modes++
	}
	if c.seedFile != "" {
		modes++
	}
	if modes != 1 {
		return c, fmt.Errorf("exactly one of --from-file, --at, or --seed-file is required")
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

type resolvedSliceSource struct {
	spec                  seedSpec
	path, uri, text, lang string
	line, column          int
}

func resolveSliceSources(cfg sliceConfig) (string, string, []resolvedSliceSource, error) {
	var specs []seedSpec
	if cfg.fromFile != "" {
		specs = []seedSpec{{Label: "slice-source", At: cfg.fromFile + ":1:1"}}
	} else {
		seedConfig := config{seedFile: cfg.seedFile, ats: cfg.ats}
		if err := loadSeeds(&seedConfig); err != nil {
			return "", "", nil, err
		}
		specs = seedConfig.seeds
	}
	var workspaceURI string
	resolved := make([]resolvedSliceSource, 0, len(specs))
	for _, spec := range specs {
		path, line, column, err := parseAt(spec.At)
		if err != nil {
			return "", "", nil, err
		}
		wsURI, uri, resolvedPath, err := source.ResolveTarget(cfg.workspace, path)
		if err != nil {
			return "", "", nil, err
		}
		text, err := os.ReadFile(resolvedPath)
		if err != nil {
			return "", "", nil, err
		}
		if workspaceURI == "" {
			workspaceURI = wsURI
		}
		lang := cfg.languageID
		if lang == "" {
			lang = source.LanguageID(path)
		}
		resolved = append(resolved, resolvedSliceSource{spec: spec, path: path, uri: uri, text: string(text), lang: lang, line: line, column: column})
	}
	scopeURI := workspaceURI
	if cfg.fromFile != "" && len(resolved) == 1 {
		scopeURI = resolved[0].uri
	}
	return workspaceURI, scopeURI, resolved, nil
}

func sliceNode(item lsp.CallHierarchyItem) graph.Node {
	convert := func(r lsp.Range) graph.Range {
		return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
	}
	return graph.NewNode(graph.Item{Name: item.Name, Kind: item.Kind, Detail: item.Detail, URI: item.URI, Range: convert(item.Range), SelectionRange: convert(item.SelectionRange), Data: item.Data})
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
	workspaceURI, sourceURI, sources, err := resolveSliceSources(cfg)
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
	opened := map[string]bool{}
	for _, source := range sources {
		if opened[source.uri] {
			continue
		}
		if err := client.DidOpen(source.uri, source.lang, source.text); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		opened[source.uri] = true
	}
	timed := requestTimeoutClient{parent: ctx, timeout: cfg.requestTimeout, client: client}
	seedResults := make([]graph.SeedResult, 0, len(sources))
	prepared := []lsp.CallHierarchyItem{}
	if cfg.fromFile == "" {
		for _, source := range sources {
			requested := graph.Target{URI: source.uri, Line: source.line, Column: source.column}
			params := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: source.uri}, Position: lsp.Position{Line: uint32(source.line - 1), Character: uint32(source.column - 1)}}
			items, prepareErr := timed.PrepareCallHierarchy(ctx, params)
			seedResult := graph.SeedResult{Label: source.spec.Label, Requested: requested}
			if prepareErr != nil {
				seedResult.Failure = &graph.SeedFailure{Phase: "slice-prepare", Message: prepareErr.Error()}
			} else if len(items) == 0 {
				seedResult.Failure = &graph.SeedFailure{Phase: "slice-prepare", Message: string(graph.PrepareReturnedNoItem)}
			} else {
				for _, item := range items {
					seedResult.PreparedTargetIDs = append(seedResult.PreparedTargetIDs, sliceNode(item).ID)
				}
				prepared = append(prepared, items...)
			}
			seedResults = append(seedResults, seedResult)
		}
	}
	var discovery slicer.Discovery
	if cfg.fromFile != "" {
		discovery = slicer.Discover(ctx, timed, sourceURI, slicer.Options{DownDepth: cfg.downDepth, MaxNodes: cfg.maxNodes})
	} else {
		discovery = slicer.DiscoverPrepared(ctx, timed, prepared, slicer.Options{DownDepth: cfg.downDepth, MaxNodes: cfg.maxNodes})
	}
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
	startMode := "from_file"
	if len(cfg.ats) > 0 {
		startMode = "at"
	}
	if cfg.seedFile != "" {
		startMode = "seed_file"
	}
	result.Slice = &graph.SliceEvidence{StartMode: startMode, SourceURI: sourceURI, DownDepth: cfg.downDepth, UpDepth: cfg.upDepth, StartingNodeIDs: discovery.StartNodeIDs, Layers: layers, FrontierNodeIDs: frontierIDs, OutgoingRelationIDs: outgoingIDs}
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
	invocationSeeds := make([]graph.InvocationSeed, 0, len(sources))
	for _, source := range sources {
		sum := sha256.Sum256([]byte(source.text))
		invocationSeeds = append(invocationSeeds, graph.InvocationSeed{Label: source.spec.Label, At: source.spec.At, ResolvedURI: source.uri, ContentSHA256: fmt.Sprintf("sha256:%x", sum[:]), LanguageID: source.lang})
	}
	primaryTarget := graph.Target{}
	languageID := cfg.languageID
	if len(sources) > 0 {
		primaryTarget = graph.Target{URI: sources[0].uri, Line: sources[0].line, Column: sources[0].column}
		if languageID == "" {
			languageID = sources[0].lang
		}
	}
	result.Invocation = graph.Invocation{
		WorkspaceURI: workspaceURI, WorkingDirectory: workingDirectory, EffectiveEnvironment: append(os.Environ(), cfg.env...),
		Target: primaryTarget, Server: graph.ServerInvocation{Command: cfg.command, Arguments: cfg.args, Environment: environment},
		Limits: graph.Limits{MaxDepth: cfg.upDepth, MaxNodes: cfg.maxNodes, TimeoutMS: cfg.timeout.Milliseconds()}, RequestTimeoutMS: cfg.requestTimeout.Milliseconds(), Concurrency: 1,
		LanguageID: languageID, OutputMode: outputMode, OutputPath: cfg.output, Seeds: invocationSeeds,
	}
	if cfg.fromFile != "" {
		seedResults = []graph.SeedResult{{Label: sources[0].spec.Label, Requested: primaryTarget, PreparedTargetIDs: discovery.StartNodeIDs}}
	}
	result.Seeds = seedResults
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
