package main

import (
	"flag"
	"fmt"
	"io"

	"lsp-trace/internal/presentation"
)

const renderUsage = "usage: lsp-trace render SELECTOR_OR_ARTIFACT [--format summary|tree|mermaid] [--detail compact|full]"

var renderLoadArtifact = loadInspectArtifact

func runRender(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, renderUsage)
		return 1
	}
	input := args[0]
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", string(presentation.FormatSummary), "summary, tree, or mermaid")
	detail := fs.String("detail", string(presentation.DetailCompact), "compact or full evidence detail")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, renderUsage)
		return 1
	}
	data, err := renderLoadArtifact(input)
	if err != nil {
		fmt.Fprintf(stderr, "render: %v\n", err)
		return 1
	}
	output, err := presentation.Render(data, presentation.Options{Format: presentation.Format(*format), Detail: presentation.Detail(*detail)})
	if err != nil {
		fmt.Fprintf(stderr, "render: %v\n", err)
		return 1
	}
	if _, err := io.WriteString(stdout, output); err != nil {
		fmt.Fprintf(stderr, "render: %v\n", err)
		return 1
	}
	return 0
}
