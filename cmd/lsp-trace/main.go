package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/jsonrpc"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/server"
	"lsp-trace/internal/source"
	"lsp-trace/internal/traverse"
)

type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }

type seedSpec struct {
	Label string `json:"label"`
	At    string `json:"at"`
}

type config struct {
	workspace, command, at, seedFile, languageID, output string
	args, env, ats                                       stringsFlag
	seeds                                                []seedSpec
	maxDepth, maxNodes, concurrency                      int
	timeout, requestTimeout                              time.Duration
	logLevel, traceLSP                                   string
	pretty                                               bool
}

func main() { code := run(os.Args[1:]); os.Exit(code) }
func run(args []string) int {
	if len(args) == 0 || args[0] != "incoming" {
		fmt.Fprintln(os.Stderr, "usage: lsp-trace incoming --workspace PATH --server COMMAND --at PATH:LINE:COLUMN")
		return 1
	}
	cfg, err := parse(args[1:])
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
	result, code := execute(ctx, cfg)
	if errors.Is(ctx.Err(), context.Canceled) {
		code = 130
	}
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintf(os.Stderr, "%s: %s\n", diagnostic.Phase, diagnostic.Message)
	}
	out := os.Stdout
	var f *os.File
	if cfg.output != "" {
		f, err = os.OpenFile(cfg.output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer f.Close()
		out = f
	}
	enc := json.NewEncoder(out)
	if cfg.pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return code
}
func parse(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("incoming", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&c.workspace, "workspace", "", "workspace path")
	fs.StringVar(&c.command, "server", "", "language server command")
	fs.Var(&c.args, "server-arg", "repeatable server argument")
	fs.Var(&c.env, "server-env", "repeatable KEY=VALUE")
	fs.Var(&c.ats, "at", "repeatable PATH:LINE:COLUMN")
	fs.StringVar(&c.seedFile, "seed-file", "", "JSON file containing labeled seeds")
	fs.StringVar(&c.languageID, "language-id", "", "document language id")
	fs.IntVar(&c.maxDepth, "max-depth", 100, "maximum traversal depth; 0 unlimited")
	fs.IntVar(&c.maxNodes, "max-nodes", 10000, "maximum nodes; 0 unlimited")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Minute, "global timeout; 0 unlimited")
	fs.DurationVar(&c.requestTimeout, "request-timeout", 30*time.Second, "request timeout")
	fs.IntVar(&c.concurrency, "concurrency", 1, "concurrent requests (MVP requires 1)")
	fs.StringVar(&c.output, "output", "", "output file")
	fs.BoolVar(&c.pretty, "pretty", false, "pretty JSON")
	fs.StringVar(&c.logLevel, "log-level", "warn", "error, warn, info, or debug")
	fs.StringVar(&c.traceLSP, "trace-lsp", "", "write JSON-RPC transcript as JSON Lines")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() != 0 {
		return c, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if c.workspace == "" || c.command == "" {
		return c, errors.New("--workspace and --server are required")
	}
	if err := loadSeeds(&c); err != nil {
		return c, err
	}
	if c.maxDepth < 0 || c.maxNodes < 0 || c.timeout < 0 || c.requestTimeout < 0 {
		return c, errors.New("limits and timeouts must be non-negative")
	}
	if c.requestTimeout == 0 {
		return c, errors.New("--request-timeout must be greater than zero")
	}
	if c.concurrency != 1 {
		return c, errors.New("--concurrency must be 1 in the sequential MVP")
	}
	switch c.logLevel {
	case "error", "warn", "info", "debug":
	default:
		return c, fmt.Errorf("invalid --log-level %q: want error, warn, info, or debug", c.logLevel)
	}
	for _, e := range c.env {
		if k, _, ok := strings.Cut(e, "="); !ok || k == "" {
			return c, fmt.Errorf("invalid --server-env %q", e)
		}
	}
	return c, nil
}

func loadSeeds(c *config) error {
	var seeds []seedSpec
	if c.seedFile != "" {
		f, err := os.Open(c.seedFile)
		if err != nil {
			return fmt.Errorf("read --seed-file: %w", err)
		}
		defer f.Close()
		var payload struct {
			Seeds []seedSpec `json:"seeds"`
		}
		decoder := json.NewDecoder(f)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			return fmt.Errorf("invalid --seed-file: %w", err)
		}
		seeds = append(seeds, payload.Seeds...)
	}
	for i, at := range c.ats {
		seeds = append(seeds, seedSpec{Label: fmt.Sprintf("seed-%d", i+1), At: at})
	}
	if len(seeds) == 0 && c.at != "" {
		seeds = append(seeds, seedSpec{Label: "seed-1", At: c.at})
	}
	if len(seeds) == 0 {
		return errors.New("at least one seed is required via --at or --seed-file")
	}
	labels := map[string]struct{}{}
	for _, seed := range seeds {
		if !validSeedLabel(seed.Label) {
			return fmt.Errorf("invalid seed label %q", seed.Label)
		}
		if _, exists := labels[seed.Label]; exists {
			return fmt.Errorf("duplicate seed label %q", seed.Label)
		}
		labels[seed.Label] = struct{}{}
		if _, _, _, err := parseAt(seed.At); err != nil {
			return fmt.Errorf("seed %q: %w", seed.Label, err)
		}
	}
	c.seeds = seeds
	c.at = seeds[0].At
	return nil
}

func validSeedLabel(label string) bool {
	if label == "" {
		return false
	}
	for i, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') || (i > 0 && (r == '-' || r == '_' || r == '.')) {
			continue
		}
		return false
	}
	return true
}

func parseAt(s string) (string, int, int, error) {
	last := strings.LastIndex(s, ":")
	if last < 0 {
		return "", 0, 0, fmt.Errorf("invalid --at %q", s)
	}
	prev := strings.LastIndex(s[:last], ":")
	if prev < 0 {
		return "", 0, 0, fmt.Errorf("invalid --at %q", s)
	}
	line, err := strconv.Atoi(s[prev+1 : last])
	if err != nil || line < 1 {
		return "", 0, 0, fmt.Errorf("invalid line in --at %q", s)
	}
	col, err := strconv.Atoi(s[last+1:])
	if err != nil || col < 1 {
		return "", 0, 0, fmt.Errorf("invalid column in --at %q", s)
	}
	path := s[:prev]
	if path == "" {
		return "", 0, 0, fmt.Errorf("invalid path in --at %q", s)
	}
	return path, line, col, nil
}

type requestTimeoutClient struct {
	parent  context.Context
	timeout time.Duration
	client  *lsp.Client
}

func (c requestTimeoutClient) callContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.parent, c.timeout)
}

func (c requestTimeoutClient) PrepareCallHierarchy(_ context.Context, params lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	ctx, cancel := c.callContext()
	defer cancel()
	return c.client.PrepareCallHierarchy(ctx, params)
}

func (c requestTimeoutClient) IncomingCalls(_ context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	ctx, cancel := c.callContext()
	defer cancel()
	return c.client.IncomingCalls(ctx, item)
}

func execute(ctx context.Context, c config) (out graph.Result, code int) {
	type resolvedSeed struct {
		spec              seedSpec
		path, uri, source string
		line, column      int
	}
	if len(c.seeds) == 0 {
		if err := loadSeeds(&c); err != nil {
			base := graph.Result{SchemaVersion: graph.SchemaVersion, Summary: graph.Summary{Complete: false}}
			base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "invocation", Message: err.Error()})
			return base, 1
		}
	}
	base := graph.Result{SchemaVersion: graph.SchemaVersion, Summary: graph.Summary{Complete: true}}
	parts := make([]graph.Result, 0, len(c.seeds))
	resolved := make([]resolvedSeed, 0, len(c.seeds))
	workspaceURI := ""
	for _, seed := range c.seeds {
		path, line, col, err := parseAt(seed.At)
		requested := graph.Target{Line: line, Column: col}
		failure := func(phase string, err error) {
			part := graph.Result{SchemaVersion: graph.SchemaVersion, Summary: graph.Summary{Complete: false}, Seeds: []graph.SeedResult{{Label: seed.Label, Requested: requested, Failure: &graph.SeedFailure{Phase: phase, Message: err.Error()}}}}
			part.Diagnostics = append(part.Diagnostics, graph.Diagnostic{Phase: phase, Message: err.Error()})
			parts = append(parts, part)
		}
		if err != nil {
			failure("invocation", err)
			continue
		}
		wsURI, targetURI, resolvedPath, err := source.ResolveTarget(c.workspace, path)
		requested.URI = targetURI
		if err != nil {
			failure("source", err)
			continue
		}
		text, err := os.ReadFile(resolvedPath)
		if err != nil {
			failure("source", err)
			continue
		}
		if workspaceURI == "" {
			workspaceURI = wsURI
		}
		resolved = append(resolved, resolvedSeed{spec: seed, path: path, uri: targetURI, source: string(text), line: line, column: col})
	}
	limits := graph.Limits{MaxDepth: c.maxDepth, MaxNodes: c.maxNodes, TimeoutMS: c.timeout.Milliseconds()}
	if len(resolved) > 0 {
		first := resolved[0]
		base.Invocation = graph.Invocation{WorkspaceURI: workspaceURI, Target: graph.Target{URI: first.uri, Line: first.line, Column: first.column}, Server: graph.ServerInvocation{Command: c.command, Arguments: c.args}, Limits: limits}
	}
	if len(resolved) == 0 {
		result := graph.MergeResults(parts...)
		result.Invocation = base.Invocation
		return result, 2
	}
	var err error
	var traceFile *os.File
	var trace jsonrpc.TraceFunc
	if c.traceLSP != "" {
		traceFile, err = os.OpenFile(c.traceLSP, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "trace", Message: err.Error()})
			return base, 1
		}
		defer traceFile.Close()
		encoder := json.NewEncoder(traceFile)
		trace = func(event jsonrpc.TraceEvent) {
			_ = encoder.Encode(event)
		}
	}
	proc, err := server.Start(ctx, c.command, c.args, c.env)
	if err != nil {
		base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "spawn", Message: err.Error()})
		return base, 1
	}
	defer func() {
		_ = proc.Stop(2 * time.Second)
		captured := proc.Stderr()
		if c.logLevel == "debug" && captured != "" {
			fmt.Fprint(os.Stderr, captured)
		}
		if captured != "" && !out.Summary.Complete {
			out.Diagnostics = append(out.Diagnostics, graph.Diagnostic{Phase: "server-stderr", Message: captured})
			out.Canonicalize()
		}
	}()
	conn := jsonrpc.NewWithTrace(proc.Stdout, proc.Stdin, trace)
	client := lsp.NewClient(conn)
	req := func() (context.Context, context.CancelFunc) {
		if c.requestTimeout == 0 {
			return context.WithCancel(ctx)
		}
		return context.WithTimeout(ctx, c.requestTimeout)
	}
	rctx, done := req()
	err = client.Initialize(rctx, workspaceURI)
	done()
	if err != nil {
		base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "initialize", Method: "initialize", Message: err.Error()})
		base.Summary.Complete = false
		return base, 1
	}
	base.Capabilities.CallHierarchyProvider = client.SupportsCallHierarchy()
	base.CapabilityQuality.Advertised = base.Capabilities.CallHierarchyProvider
	base.CapabilityQuality.CrossModuleEdges = graph.Unknown
	if !base.Capabilities.CallHierarchyProvider {
		base.Terminals = []graph.Boundary{{Reason: graph.UnsupportedCallHierarchy}}
		base.Summary.Complete = false
		for _, seed := range resolved {
			base.Seeds = append(base.Seeds, graph.SeedResult{Label: seed.spec.Label, Requested: graph.Target{URI: seed.uri, Line: seed.line, Column: seed.column}, Failure: &graph.SeedFailure{Phase: "capability", Message: string(graph.UnsupportedCallHierarchy)}})
		}
		base.Canonicalize()
		_ = client.Shutdown(context.Background())
		return base, 2
	}
	opened := map[string]struct{}{}
	openFailed := map[string]error{}
	for _, seed := range resolved {
		if _, ok := opened[seed.uri]; ok {
			continue
		}
		if failure, ok := openFailed[seed.uri]; ok {
			part := graph.Result{SchemaVersion: graph.SchemaVersion, Summary: graph.Summary{Complete: false}, Seeds: []graph.SeedResult{{Label: seed.spec.Label, Requested: graph.Target{URI: seed.uri, Line: seed.line, Column: seed.column}, Failure: &graph.SeedFailure{Phase: "didOpen", Message: failure.Error()}}}}
			part.Diagnostics = append(part.Diagnostics, graph.Diagnostic{Phase: "didOpen", Message: failure.Error()})
			parts = append(parts, part)
			continue
		}
		lang := c.languageID
		if lang == "" {
			lang = source.LanguageID(seed.path)
		}
		if err = client.DidOpen(seed.uri, lang, seed.source); err != nil {
			part := graph.Result{SchemaVersion: graph.SchemaVersion, Summary: graph.Summary{Complete: false}, Seeds: []graph.SeedResult{{Label: seed.spec.Label, Requested: graph.Target{URI: seed.uri, Line: seed.line, Column: seed.column}, Failure: &graph.SeedFailure{Phase: "didOpen", Message: err.Error()}}}}
			part.Diagnostics = append(part.Diagnostics, graph.Diagnostic{Phase: "didOpen", Message: err.Error()})
			parts = append(parts, part)
			openFailed[seed.uri] = err
			continue
		}
		opened[seed.uri] = struct{}{}
	}
	timedClient := requestTimeoutClient{parent: ctx, timeout: c.requestTimeout, client: client}
	for _, seed := range resolved {
		if _, ok := opened[seed.uri]; !ok {
			continue
		}
		params := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: seed.uri}, Position: lsp.Position{Line: uint32(seed.line - 1), Character: uint32(seed.column - 1)}}
		part := traverse.Incoming(ctx, timedClient, params, traverse.Options{MaxDepth: c.maxDepth, MaxNodes: c.maxNodes})
		seedResult := graph.SeedResult{Label: seed.spec.Label, Requested: graph.Target{URI: seed.uri, Line: seed.line, Column: seed.column}, PreparedTargetIDs: append([]string(nil), part.Targets...)}
		for _, node := range part.Nodes {
			seedResult.ReachedNodeIDs = append(seedResult.ReachedNodeIDs, node.ID)
		}
		if !part.Summary.Complete {
			phase, message := "traverse", "seed traversal incomplete"
			if len(part.Diagnostics) > 0 {
				phase, message = part.Diagnostics[0].Phase, part.Diagnostics[0].Message
			}
			seedResult.Failure = &graph.SeedFailure{Phase: phase, Message: message}
		}
		part.Seeds = []graph.SeedResult{seedResult}
		part.Capabilities = base.Capabilities
		part.CapabilityQuality.Advertised = base.CapabilityQuality.Advertised
		parts = append(parts, part)
	}
	result := graph.MergeResults(parts...)
	result.Invocation = base.Invocation
	result.Capabilities = base.Capabilities
	result.CapabilityQuality.Advertised = base.CapabilityQuality.Advertised
	shutdownCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	if err = client.Shutdown(shutdownCtx); err != nil {
		result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Phase: "shutdown", Message: err.Error()})
		result.Summary.Complete = false
	}
	cancel()
	result.Canonicalize()
	if result.Summary.Complete {
		return result, 0
	}
	return result, 2
}
