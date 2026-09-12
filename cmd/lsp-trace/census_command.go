package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/census"
)

const censusUsage = `usage: lsp-trace census --publication-root ABSOLUTE_PRIVATE_DIRECTORY [--workspace PATH] [--include PATTERN...] [--exclude PATTERN...] [--format human|json | --json]

Validates and projects a census plan only. This command does not start a language server, acquire targets, or publish artifacts.`

func runCensus(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, censusUsage)
		return 0
	}
	fs := flag.NewFlagSet("census", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "census: workspace: %v\n", err)
		return 1
	}
	cfg := census.Config{WorkspaceRoot: workspace, Format: census.OutputHuman}
	var includes, excludes stringsFlag
	var format string
	var jsonOutput bool
	fs.StringVar(&cfg.WorkspaceRoot, "workspace", workspace, "workspace root (defaults to current directory)")
	fs.StringVar(&cfg.PublicationRoot, "publication-root", "", "required cleaned absolute private publication root")
	fs.Var(&includes, "include", "repeatable workspace-relative discovery pattern")
	fs.Var(&excludes, "exclude", "repeatable workspace-relative discovery pattern; exclusions take precedence")
	fs.StringVar(&format, "format", "", "output format: human or json")
	fs.BoolVar(&jsonOutput, "json", false, "emit machine JSON")
	fs.Usage = func() { fmt.Fprintln(fs.Output(), censusUsage) }
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "census: positional arguments are not accepted")
		return 1
	}
	if jsonOutput && format != "" {
		fmt.Fprintln(stderr, "census: --json conflicts with --format")
		return 1
	}
	if format == "" {
		if jsonOutput {
			format = string(census.OutputJSON)
		} else {
			format = string(census.OutputHuman)
		}
	}
	cfg.Format = census.OutputFormat(format)
	cfg.Includes = append([]string(nil), includes...)
	cfg.Excludes = append([]string(nil), excludes...)
	root, err := census.Validate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "census: %v\n", err)
		return 1
	}
	defer root.Close()
	result := census.NewReadyResult(nil)
	if cfg.Format == census.OutputJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(census.ProjectMachine(result)); err != nil {
			fmt.Fprintf(stderr, "census: %v\n", err)
			return 1
		}
		return 0
	}
	out := census.ProjectHuman(result)
	fmt.Fprintf(stdout, "status: %s\ntargets: %d\nbatches: %d\nmax_batch_targets: %d\n", out.Status, out.TargetCount, out.BatchCount, out.MaxBatchTargets)
	return 0
}
