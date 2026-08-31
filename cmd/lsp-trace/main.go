package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

type config struct {
	workspace, command, at, languageID, output string
	args, env                                  stringsFlag
	maxDepth, maxNodes                         int
	timeout, requestTimeout                    time.Duration
	pretty                                     bool
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
	fs.StringVar(&c.at, "at", "", "PATH:LINE:COLUMN")
	fs.StringVar(&c.languageID, "language-id", "", "document language id")
	fs.IntVar(&c.maxDepth, "max-depth", 100, "maximum traversal depth; 0 unlimited")
	fs.IntVar(&c.maxNodes, "max-nodes", 10000, "maximum nodes; 0 unlimited")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Minute, "global timeout; 0 unlimited")
	fs.DurationVar(&c.requestTimeout, "request-timeout", 30*time.Second, "request timeout")
	fs.StringVar(&c.output, "output", "", "output file")
	fs.BoolVar(&c.pretty, "pretty", false, "pretty JSON")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if c.workspace == "" || c.command == "" || c.at == "" {
		return c, errors.New("--workspace, --server, and --at are required")
	}
	if c.maxDepth < 0 || c.maxNodes < 0 || c.timeout < 0 || c.requestTimeout < 0 {
		return c, errors.New("limits and timeouts must be non-negative")
	}
	for _, e := range c.env {
		if k, _, ok := strings.Cut(e, "="); !ok || k == "" {
			return c, fmt.Errorf("invalid --server-env %q", e)
		}
	}
	return c, nil
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
	return s[:prev], line, col, nil
}
func execute(ctx context.Context, c config) (graph.Result, int) {
	path, line, col, err := parseAt(c.at)
	base := graph.Result{SchemaVersion: graph.SchemaVersion, Summary: graph.Summary{Complete: false}}
	if err != nil {
		base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "invocation", Message: err.Error()})
		return base, 1
	}
	workspaceURI, targetURI, err := source.ResolveTarget(c.workspace, path)
	if err != nil {
		base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "source", Message: err.Error()})
		return base, 1
	}
	text, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if !filepath.IsAbs(path) {
			text, err = os.ReadFile(filepath.Join(c.workspace, path))
		}
		if err != nil {
			base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "source", Message: err.Error()})
			return base, 1
		}
	}
	base.Invocation = graph.Invocation{WorkspaceURI: workspaceURI, Target: graph.Target{URI: targetURI, Line: line, Column: col}, Server: graph.ServerInvocation{Command: c.command, Arguments: c.args}, Limits: graph.Limits{MaxDepth: c.maxDepth, MaxNodes: c.maxNodes, TimeoutMS: c.timeout.Milliseconds()}}
	proc, err := server.Start(ctx, c.command, c.args, c.env)
	if err != nil {
		base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "spawn", Message: err.Error()})
		return base, 1
	}
	conn := jsonrpc.New(proc.Stdout, proc.Stdin)
	client := lsp.NewClient(conn)
	defer proc.Stop(2 * time.Second)
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
		return base, 1
	}
	base.Capabilities.CallHierarchyProvider = client.SupportsCallHierarchy()
	if !base.Capabilities.CallHierarchyProvider {
		base.Terminals = []graph.Boundary{{Reason: graph.UnsupportedCallHierarchy}}
		base.Summary.TerminalCount = 1
		_ = client.Shutdown(context.Background())
		return base, 2
	}
	lang := c.languageID
	if lang == "" {
		lang = source.LanguageID(path)
	}
	if err = client.DidOpen(targetURI, lang, string(text)); err != nil {
		base.Diagnostics = append(base.Diagnostics, graph.Diagnostic{Phase: "didOpen", Message: err.Error()})
		return base, 1
	}
	params := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: targetURI}, Position: lsp.Position{Line: uint32(line - 1), Character: uint32(col - 1)}}
	result := traverse.Incoming(ctx, client, params, traverse.Options{MaxDepth: c.maxDepth, MaxNodes: c.maxNodes})
	result.Invocation = base.Invocation
	result.Capabilities = base.Capabilities
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
