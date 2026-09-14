package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/presentation"
	"lsp-trace/internal/seedformat"
)

type traceConfig struct {
	sliceConfig
	file     string
	profiles profileFlags
	siblings bool
	format   traceFormat
}

type traceFormat struct {
	value string
	set   bool
}

func (f *traceFormat) String() string {
	if f.value == "" {
		return "json"
	}
	return f.value
}

func (f *traceFormat) Set(value string) error {
	if f.set {
		return errors.New("--format may be specified only once")
	}
	if value != "json" && value != "tree" {
		return fmt.Errorf("invalid --format %q (want json or tree)", value)
	}
	f.value, f.set = value, true
	return nil
}

func parseTrace(args []string) (traceConfig, error) {
	var c traceConfig
	fs := flag.NewFlagSet("trace", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fs.SetOutput(os.Stdout)
			break
		}
	}
	fs.StringVar(&c.workspace, "workspace", "", "workspace path")
	fs.StringVar(&c.profiles.ConfigPath, "config", "", "profile config path")
	fs.StringVar(&c.profiles.Name, "profile", "", "named server profile")
	fs.StringVar(&c.command, "server", "", "language server command")
	fs.Var(&c.args, "server-arg", "repeatable server argument")
	fs.Var(&c.env, "server-env", "repeatable KEY=VALUE")
	fs.StringVar(&c.file, "file", "", "source file containing the exact managed symbol")
	fs.StringVar(&c.symbol, "symbol", "", "exhaustive exact managed-symbol name within --file")
	fs.Var(&c.ats, "at", "repeatable exact target PATH:LINE:COLUMN (one-based)")
	fs.StringVar(&c.languageID, "language-id", "", "document language id")
	fs.IntVar(&c.downDepth, "down-depth", 2, "outgoing discovery depth")
	fs.IntVar(&c.upDepth, "up-depth", 2, "incoming traversal depth")
	fs.IntVar(&c.maxNodes, "max-nodes", 100, "maximum graph nodes")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Second, "global timeout")
	fs.DurationVar(&c.requestTimeout, "request-timeout", time.Second, "request timeout")
	fs.StringVar(&c.output, "output", "", "output destination")
	fs.BoolVar(&c.pretty, "pretty", false, "pretty JSON")
	fs.BoolVar(&c.siblings, "siblings", false, "include exact topmost sibling enrichment")
	fs.Var(&c.format, "format", "output format: json or tree (default json)")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() != 0 {
		return c, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if c.format.String() == "tree" && (c.pretty || c.output != "") {
		return c, errors.New("--format tree conflicts with JSON-only --pretty and --output")
	}
	if c.profiles.ConfigPath != "" && c.profiles.Name == "" {
		return c, errors.New("--config requires --profile")
	}
	symbolMode := c.file != "" && c.symbol != "" && len(c.ats) == 0
	positionMode := c.file == "" && c.symbol == "" && len(c.ats) > 0
	if c.file != "" && c.symbol != "" && len(c.ats) > 0 {
		return c, errors.New("--file/--symbol and --at are mutually exclusive")
	}
	if !symbolMode && !positionMode {
		return c, errors.New("trace requires exactly one target mode: --file PATH --symbol NAME or repeatable --at PATH:LINE:COLUMN")
	}
	for _, at := range c.ats {
		if _, _, _, err := parseAt(at); err != nil {
			return c, err
		}
	}
	if c.workspace == "" {
		return c, errors.New("--workspace is required")
	}
	if err := applySliceProfile(&c.sliceConfig, c.profiles, explicitServerFields(fs)); err != nil {
		return c, err
	}
	if c.command == "" {
		return c, errors.New("--server or --profile is required")
	}
	if c.downDepth < 0 || c.downDepth > 64 || c.upDepth < 0 || c.upDepth > 64 || c.maxNodes < 1 || c.maxNodes > 10000 || c.timeout < time.Millisecond || c.timeout > 60*time.Second || c.requestTimeout < time.Millisecond || c.requestTimeout > 60*time.Second {
		return c, errors.New("trace uses managed bounds: depths 0..64, nodes 1..10000, timeouts 1ms..60s")
	}
	return c, nil
}

func traceSeeds(c traceConfig) (seedformat.File, []byte, error) {
	file := seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Defaults: seedformat.Defaults{DownDepth: &c.downDepth, UpDepth: &c.upDepth}}
	if c.symbol != "" {
		file.Seeds = []seedformat.Seed{{Type: seedformat.SymbolType, Symbol: &seedformat.Symbol{Label: "root", Path: c.file, Symbol: c.symbol}}}
	} else {
		for i, at := range c.ats {
			path, line, column, err := parseAt(at)
			if err != nil {
				return file, nil, err
			}
			file.Seeds = append(file.Seeds, seedformat.Seed{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("target-%d", i+1), Path: path, Line: uint64(line), Column: uint64(column)}})
		}
	}
	raw, err := seedformat.EncodeCanonical(file, c.workspace)
	return file, raw, err
}

func traceManifest(c traceConfig, file seedformat.File) (acquisitionops.Manifest, error) {
	return seedformat.Translate(file, seedformat.TranslateOptions{Workspace: c.workspace, Limits: traceLimits(c), TopmostSiblings: c.siblings})
}

func runTrace(args []string, stdout, stderr io.Writer) int {
	c, err := parseTrace(args)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	seeds, _, err := traceSeeds(c)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	manifest, err := traceManifest(c, seeds)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	forward := []string{"--workspace", c.workspace, "--server", c.command, "--output-version", "lsp-trace.graph-provenance.v5"}
	for _, arg := range c.args {
		forward = append(forward, "--server-arg", arg)
	}
	for _, env := range c.env {
		forward = append(forward, "--server-env", env)
	}
	if c.languageID != "" {
		forward = append(forward, "--language-id", c.languageID)
	}
	if c.output != "" {
		forward = append(forward, "--output", c.output)
	}
	if c.pretty {
		forward = append(forward, "--pretty")
	}
	if c.format.String() == "json" {
		return runTraceAcquisition("slice", "v3", forward, stdout, stderr, traceAcquisitionInput{manifest: manifest, seeds: seeds})
	}
	var acquired bytes.Buffer
	code := runTraceAcquisition("slice", "v3", forward, &acquired, stderr, traceAcquisitionInput{manifest: manifest, seeds: seeds})
	if code != 0 {
		return code
	}
	return writeTraceTree(acquired.Bytes(), stdout, stderr)
}

func writeTraceTree(acquired []byte, stdout, stderr io.Writer) int {
	tree, err := presentation.RenderTraceV5(acquired, presentation.TraceV5Options{})
	if err != nil {
		fmt.Fprintf(stderr, "trace tree rendering failed: %v\n", err)
		return 1
	}
	if _, err := io.WriteString(stdout, tree); err != nil {
		fmt.Fprintf(stderr, "trace tree output failed: %v\n", err)
		return 1
	}
	return 0
}

func traceLimits(c traceConfig) acquisitionops.Limits {
	maxRequests, maxEvidenceBytes, maxPathWork := 1000, 4<<20, 100000
	timeoutMS, requestTimeoutMS := int(c.timeout.Milliseconds()), int(c.requestTimeout.Milliseconds())
	maxResponseBytes, maxMessages := 4<<20, 64
	return acquisitionops.Limits{MaxNodes: &c.maxNodes, MaxRequests: &maxRequests, MaxEvidenceBytes: &maxEvidenceBytes, MaxPathWork: &maxPathWork, TimeoutMS: &timeoutMS, RequestTimeoutMS: &requestTimeoutMS, MaxResponseBytes: &maxResponseBytes, MaxMessages: &maxMessages}
}
