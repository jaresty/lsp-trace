package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lsp-trace/internal/dispatch"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/jsonrpc"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/server"
	"lsp-trace/internal/source"
	"lsp-trace/internal/traverse"
)

//go:embed SKILL.md
var embeddedSkill string

const usageText = `usage:
  lsp-trace incoming --workspace PATH --server COMMAND --at PATH:LINE:COLUMN
  lsp-trace slice --workspace PATH --server COMMAND (--from-file PATH | --at PATH:LINE:COLUMN... | --seed-file PATH) --down-depth N --up-depth N
  lsp-trace inspect SELECTOR_OR_ARTIFACT (--seed LABEL | --all-seeds) [--json]
  lsp-trace filter INSPECTION --compare-seeds LABEL --compare-seeds LABEL [--json]
  lsp-trace verify PATH
  lsp-trace schema get --family graph|inspect|filter --version VERSION
  lsp-trace schema get --schema v1|v2|v3
  lsp-trace validate --family graph|inspect|filter --version VERSION PATH|-
  lsp-trace validate [--schema v1|v2|v3] PATH|-
  lsp-trace skill get`

type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }

type seedSpec struct {
	Label string `json:"label"`
	At    string `json:"at"`
}

type config struct {
	workspace, command, at, seedFile, languageID, output, schema           string
	provenanceInvocationID, provenanceCaller, provenanceSource             string
	provenanceSourceRevision, provenanceServerVersion, provenanceTimestamp string
	provenanceToolVersion                                                  string
	args, env, ats                                                         stringsFlag
	seeds                                                                  []seedSpec
	maxDepth, maxNodes, concurrency                                        int
	timeout, requestTimeout                                                time.Duration
	logLevel, traceLSP                                                     string
	pretty, topmostSiblings, expandDispatchFamily                          bool
}

func main() { code := run(os.Args[1:]); os.Exit(code) }
func run(args []string) int {
	if len(args) > 0 && args[0] == "skill" {
		return runSkill(args[1:], os.Stdout, os.Stderr)
	}
	if len(args) > 0 && args[0] == "inspect" {
		return runInspect(args[1:], os.Stdout, os.Stderr)
	}
	if len(args) > 0 && args[0] == "verify" {
		return runVerify(args[1:], os.Stdout, os.Stderr)
	}
	if len(args) > 0 && args[0] == "filter" {
		return runFilter(args[1:], os.Stdout, os.Stderr)
	}
	if len(args) > 0 && args[0] == "schema" {
		return runSchema(args[1:], os.Stdout, os.Stderr)
	}
	if len(args) > 0 && args[0] == "validate" {
		return runValidate(args[1:], os.Stdin, os.Stdout, os.Stderr)
	}
	if len(args) > 0 && args[0] == "slice" {
		return runSlice(args[1:])
	}
	if len(args) == 0 || args[0] != "incoming" {
		fmt.Fprintln(os.Stderr, usageText)
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
	if cfg.traceLSP != "" {
		if transcript, readErr := os.ReadFile(cfg.traceLSP); readErr == nil {
			sum := sha256.Sum256(transcript)
			result.Invocation.Trace.ContentSHA256 = fmt.Sprintf("sha256:%x", sum[:])
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		code = 130
	}
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintf(os.Stderr, "%s: %s\n", diagnostic.Phase, diagnostic.Message)
	}
	data, err := marshalResult(result, cfg.pretty)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if cfg.output == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return code
	}
	publish := publishArtifact
	if result.SchemaVersion == graph.SchemaVersionV3 {
		publish = publishBundle
	}
	if err := publish(cfg.output, data); err != nil {
		_, _ = os.Stdout.Write(data)
		recordPath, retainErr := retainPublicationFailure(cfg.output, data, err)
		if retainErr != nil {
			fmt.Fprintf(os.Stderr, "publish failure evidence: %v\n", retainErr)
		} else {
			fmt.Fprintf(os.Stderr, "publish failure evidence: %s\n", recordPath)
		}
		fmt.Fprintf(os.Stderr, "publish: %v\n", err)
		return 1
	}
	return code
}

func runSkill(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "get" {
		fmt.Fprintln(stderr, "usage: lsp-trace skill get")
		return 1
	}
	if _, err := io.WriteString(stdout, embeddedSkill); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
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
	fs.StringVar(&c.schema, "schema", "v3", "output schema: v1, v2, or v3")
	fs.BoolVar(&c.pretty, "pretty", false, "pretty JSON")
	fs.BoolVar(&c.topmostSiblings, "expand-topmost-siblings", false, "include top-level document symbols as sibling candidates")
	fs.BoolVar(&c.expandDispatchFamily, "expand-dispatch-family", false, "include implementation-family relationships")
	fs.StringVar(&c.logLevel, "log-level", "warn", "error, warn, info, or debug")
	fs.StringVar(&c.traceLSP, "trace-lsp", "", "write JSON-RPC transcript as JSON Lines")
	fs.StringVar(&c.provenanceInvocationID, "provenance-invocation-id", "", "caller-supplied invocation identifier")
	fs.StringVar(&c.provenanceCaller, "provenance-caller", "", "caller-supplied caller identity")
	fs.StringVar(&c.provenanceSource, "provenance-source", "", "caller-supplied invocation source")
	fs.StringVar(&c.provenanceSourceRevision, "provenance-source-revision", "", "caller-supplied source revision")
	fs.StringVar(&c.provenanceServerVersion, "provenance-server-version", "", "caller-supplied server version")
	fs.StringVar(&c.provenanceTimestamp, "provenance-timestamp", "", "caller-supplied timestamp")
	fs.StringVar(&c.provenanceToolVersion, "provenance-tool-version", "", "caller-supplied lsp-trace version")
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
	if c.schema != "v1" && c.schema != "v2" && c.schema != "v3" {
		return c, errors.New("--schema must be v1, v2, or v3")
	}
	if c.concurrency != 1 {
		return c, errors.New("--concurrency must be 1 in the sequential MVP")
	}
	switch c.logLevel {
	case "error", "warn", "info", "debug":
	default:
		return c, fmt.Errorf("invalid --log-level %q: want error, warn, info, or debug", c.logLevel)
	}
	environmentNames := make(map[string]struct{}, len(c.env))
	for _, e := range c.env {
		k, _, ok := strings.Cut(e, "=")
		if !ok || k == "" {
			return c, fmt.Errorf("invalid --server-env %q", e)
		}
		if _, duplicate := environmentNames[k]; duplicate {
			return c, fmt.Errorf("duplicate --server-env name %q", k)
		}
		environmentNames[k] = struct{}{}
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

func (c requestTimeoutClient) OutgoingCalls(_ context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	ctx, cancel := c.callContext()
	defer cancel()
	return c.client.OutgoingCalls(ctx, item)
}

func (c requestTimeoutClient) SupportsTypeHierarchy() bool {
	return c.client.SupportsTypeHierarchy()
}

func (c requestTimeoutClient) PrepareTypeHierarchy(_ context.Context, params lsp.PrepareTypeHierarchyParams) ([]lsp.TypeHierarchyItem, error) {
	ctx, cancel := c.callContext()
	defer cancel()
	return c.client.PrepareTypeHierarchy(ctx, params)
}

func (c requestTimeoutClient) Subtypes(_ context.Context, item lsp.TypeHierarchyItem) ([]lsp.TypeHierarchyItem, error) {
	ctx, cancel := c.callContext()
	defer cancel()
	return c.client.Subtypes(ctx, item)
}

func (c requestTimeoutClient) SupportsDocumentSymbols() bool {
	return c.client.SupportsDocumentSymbols()
}

func (c requestTimeoutClient) DocumentSymbols(_ context.Context, params lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	ctx, cancel := c.callContext()
	defer cancel()
	return c.client.DocumentSymbols(ctx, params)
}

func resolveDispatchRelationships(ctx context.Context, client dispatch.Client, params lsp.PrepareTypeHierarchyParams, seedLabel string) ([]graph.DispatchRelationship, []graph.Diagnostic) {
	family := dispatch.Resolve(ctx, client, params)
	relationships := make([]graph.DispatchRelationship, 0, len(family.Associations))
	for _, association := range family.Associations {
		relationships = append(relationships, graph.DispatchRelationship{SeedLabel: seedLabel, Interface: dispatchNode(association.Interface), Implementation: dispatchNode(association.Implementation)})
	}
	diagnostics := make([]graph.Diagnostic, 0, len(family.Failures))
	for _, failure := range family.Failures {
		diagnostics = append(diagnostics, graph.Diagnostic{Phase: "dispatch", Method: "typeHierarchy/subtypes", Message: failure.Message})
	}
	return relationships, diagnostics
}

func dispatchNode(item lsp.TypeHierarchyItem) graph.Node {
	return graph.NewNode(graph.Item{
		Name: item.Name, Kind: item.Kind, Detail: item.Detail, URI: item.URI,
		Range:          graph.Range{Start: graph.Position{Line: item.Range.Start.Line, Character: item.Range.Start.Character}, End: graph.Position{Line: item.Range.End.Line, Character: item.Range.End.Character}},
		SelectionRange: graph.Range{Start: graph.Position{Line: item.SelectionRange.Start.Line, Character: item.SelectionRange.Start.Character}, End: graph.Position{Line: item.SelectionRange.End.Line, Character: item.SelectionRange.End.Character}},
		Data:           item.Data,
	})
}

func execute(ctx context.Context, c config) (out graph.Result, code int) {
	unknownIfEmpty := func(value string) string {
		if value == "" {
			return graph.Unknown
		}
		return value
	}
	provenance := graph.InvocationProvenance{
		InvocationID:   unknownIfEmpty(c.provenanceInvocationID),
		Caller:         unknownIfEmpty(c.provenanceCaller),
		Source:         unknownIfEmpty(c.provenanceSource),
		SourceRevision: unknownIfEmpty(c.provenanceSourceRevision),
		ServerVersion:  unknownIfEmpty(c.provenanceServerVersion),
		Timestamp:      unknownIfEmpty(c.provenanceTimestamp),
	}
	tool := graph.ToolIdentity{Name: "lsp-trace", Version: unknownIfEmpty(c.provenanceToolVersion)}
	schemaVersion := map[string]string{"v1": graph.SchemaVersionV1, "v2": graph.SchemaVersionV2, "v3": graph.SchemaVersionV3}[c.schema]
	if schemaVersion == "" {
		schemaVersion = graph.SchemaVersionV3
	}
	type resolvedSeed struct {
		spec              seedSpec
		path, uri, source string
		line, column      int
	}
	if len(c.seeds) == 0 {
		if err := loadSeeds(&c); err != nil {
			base := graph.Result{SchemaVersion: schemaVersion, Tool: tool, Invocation: graph.Invocation{Provenance: provenance}, Summary: graph.Summary{Complete: false}}
			base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "invocation", Message: err.Error()})
			return base, 1
		}
	}
	base := graph.Result{SchemaVersion: schemaVersion, Tool: tool, Invocation: graph.Invocation{Provenance: provenance}, Summary: graph.Summary{Complete: true}}
	parts := make([]graph.Result, 0, len(c.seeds))
	resolved := make([]resolvedSeed, 0, len(c.seeds))
	workspaceURI := ""
	for _, seed := range c.seeds {
		path, line, col, err := parseAt(seed.At)
		requested := graph.Target{Line: line, Column: col}
		failure := func(phase string, err error) {
			part := graph.Result{SchemaVersion: schemaVersion, Summary: graph.Summary{Complete: false}, Seeds: []graph.SeedResult{{Label: seed.Label, Requested: requested, Failure: &graph.SeedFailure{Phase: phase, Message: err.Error()}}}}
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
	env := map[string]string{}
	for _, declaration := range c.env {
		k, v, _ := strings.Cut(declaration, "=")
		env[k] = v
	}
	resolvedByLabel := map[string]resolvedSeed{}
	for _, s := range resolved {
		resolvedByLabel[s.spec.Label] = s
	}
	seeds := make([]graph.InvocationSeed, 0, len(c.seeds))
	for _, spec := range c.seeds {
		path, _, _, _ := parseAt(spec.At)
		languageID := c.languageID
		if languageID == "" {
			languageID = source.LanguageID(path)
		}
		item := graph.InvocationSeed{Label: spec.Label, At: spec.At, LanguageID: languageID}
		if rs, ok := resolvedByLabel[spec.Label]; ok {
			sum := sha256.Sum256([]byte(rs.source))
			item.ResolvedURI = rs.uri
			item.ContentSHA256 = fmt.Sprintf("sha256:%x", sum[:])
		}
		seeds = append(seeds, item)
	}
	outputMode := "stdout"
	if c.output != "" {
		outputMode = "file"
	}
	effectiveLanguageID := c.languageID
	if effectiveLanguageID == "" && len(seeds) > 0 {
		effectiveLanguageID = seeds[0].LanguageID
	}
	workingDirectory, _ := os.Getwd()
	effectiveEnvironment := append([]string(nil), os.Environ()...)
	effectiveEnvironment = append(effectiveEnvironment, c.env...)
	base.Invocation = graph.Invocation{WorkspaceURI: workspaceURI, WorkingDirectory: workingDirectory, EffectiveEnvironment: effectiveEnvironment, Server: graph.ServerInvocation{Command: c.command, Arguments: c.args, Environment: env}, Limits: limits, RequestTimeoutMS: c.requestTimeout.Milliseconds(), Concurrency: c.concurrency, LanguageID: effectiveLanguageID, Expansion: graph.ExpansionConfig{TopmostSiblings: c.topmostSiblings, DispatchFamily: c.expandDispatchFamily}, Trace: graph.TraceConfig{Enabled: c.traceLSP != "", Path: c.traceLSP}, OutputMode: outputMode, OutputPath: c.output, Seeds: seeds, Provenance: provenance}
	if len(resolved) > 0 {
		first := resolved[0]
		base.Invocation.WorkspaceURI = workspaceURI
		base.Invocation.Target = graph.Target{URI: first.uri, Line: first.line, Column: first.column}
	}
	if len(resolved) == 0 {
		result := graph.MergeResults(parts...)
		result.SchemaVersion = schemaVersion
		result.Invocation = base.Invocation
		result.Tool = base.Tool
		return result, 2
	}
	earlyFailure := func(phase string, err error) graph.Result {
		global := graph.Result{SchemaVersion: schemaVersion, Summary: graph.Summary{Complete: false}}
		global.Diagnostics = append(global.Diagnostics, graph.Diagnostic{Phase: phase, Message: err.Error()})
		for _, seed := range resolved {
			global.Seeds = append(global.Seeds, graph.SeedResult{
				Label: seed.spec.Label, Requested: graph.Target{URI: seed.uri, Line: seed.line, Column: seed.column},
				Failure: &graph.SeedFailure{Phase: phase, Message: err.Error()},
			})
		}
		resultParts := append(append([]graph.Result(nil), parts...), global)
		result := graph.MergeResults(resultParts...)
		result.SchemaVersion = schemaVersion
		result.Invocation = base.Invocation
		result.Tool = base.Tool
		result.Capabilities = base.Capabilities
		result.CapabilityQuality.Advertised = base.CapabilityQuality.Advertised
		return result
	}
	var err error
	var traceFile *os.File
	var trace jsonrpc.TraceFunc
	if c.traceLSP != "" {
		traceFile, err = os.OpenFile(c.traceLSP, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return earlyFailure("trace", err), 1
		}
		defer traceFile.Close()
		encoder := json.NewEncoder(traceFile)
		trace = func(event jsonrpc.TraceEvent) {
			_ = encoder.Encode(event)
		}
	}
	proc, err := server.Start(ctx, c.command, c.args, c.env)
	if err != nil {
		return earlyFailure("spawn", err), 1
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
		result := earlyFailure("initialize", err)
		for i := range result.Diagnostics {
			if result.Diagnostics[i].Phase == "initialize" && result.Diagnostics[i].Message == err.Error() {
				result.Diagnostics[i].Method = "initialize"
			}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return result, 2
		}
		return result, 1
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
			part := graph.Result{SchemaVersion: schemaVersion, Summary: graph.Summary{Complete: false}, Seeds: []graph.SeedResult{{Label: seed.spec.Label, Requested: graph.Target{URI: seed.uri, Line: seed.line, Column: seed.column}, Failure: &graph.SeedFailure{Phase: "didOpen", Message: failure.Error()}}}}
			part.Diagnostics = append(part.Diagnostics, graph.Diagnostic{Phase: "didOpen", Message: failure.Error()})
			parts = append(parts, part)
			continue
		}
		lang := c.languageID
		if lang == "" {
			lang = source.LanguageID(seed.path)
		}
		if err = client.DidOpen(seed.uri, lang, seed.source); err != nil {
			part := graph.Result{SchemaVersion: schemaVersion, Summary: graph.Summary{Complete: false}, Seeds: []graph.SeedResult{{Label: seed.spec.Label, Requested: graph.Target{URI: seed.uri, Line: seed.line, Column: seed.column}, Failure: &graph.SeedFailure{Phase: "didOpen", Message: err.Error()}}}}
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
		part := traverse.Incoming(ctx, timedClient, params, traverse.Options{MaxDepth: c.maxDepth, MaxNodes: c.maxNodes, IncludeTopmostSiblings: c.topmostSiblings})
		part.SchemaVersion = schemaVersion
		for i := range part.SiblingCandidates {
			part.SiblingCandidates[i].SeedLabel = seed.spec.Label
		}
		if c.expandDispatchFamily {
			relationships, diagnostics := resolveDispatchRelationships(ctx, timedClient, params, seed.spec.Label)
			part.DispatchRelationships = append(part.DispatchRelationships, relationships...)
			part.Diagnostics = append(part.Diagnostics, diagnostics...)
		}
		part.Canonicalize()
		seedResult := graph.SeedResult{Label: seed.spec.Label, Requested: graph.Target{URI: seed.uri, Line: seed.line, Column: seed.column}, PreparedTargetIDs: append([]string(nil), part.Targets...), ReachedEdges: append([]graph.Edge(nil), part.Edges...)}
		if schemaVersion == graph.SchemaVersionV3 {
			for _, edge := range part.Edges {
				seedResult.ReachedRelationIDs = append(seedResult.ReachedRelationIDs, edge.RelationID)
			}
		}
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
	result.SchemaVersion = schemaVersion
	result.Invocation = base.Invocation
	result.Tool = base.Tool
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
