package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	semanticfilter "lsp-trace/internal/filter"
	"lsp-trace/internal/schema"
)

const filterUsage = "usage: lsp-trace filter INSPECTION --compare-seeds LABEL --compare-seeds LABEL [--json]"

func runFilter(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(stdout, filterUsage)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, filterUsage)
		return 1
	}
	input := args[0]
	fs := flag.NewFlagSet("filter", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, filterUsage) }
	var labels stringsFlag
	fs.Var(&labels, "compare-seeds", "seed label to compare; repeat exactly twice")
	_ = fs.Bool("json", false, "emit JSON (currently the only format)")
	if err := fs.Parse(args[1:]); err != nil {
		fs.Usage()
		return 1
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return 1
	}
	if len(labels) != 2 {
		fmt.Fprintln(stderr, "filter: exactly two --compare-seeds values are required")
		return 1
	}
	if labels[0] == "" || labels[1] == "" {
		fmt.Fprintln(stderr, "filter: compared seed labels must be nonempty")
		return 1
	}
	if labels[0] == labels[1] {
		fmt.Fprintln(stderr, "filter: compared seed labels must be distinct")
		return 1
	}
	data, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	projection, err := semanticfilter.ProjectPairwise(data, labels[0], labels[1])
	if err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	if _, err := schema.ValidateFor(encoded, schema.FamilyFilter, "v1"); err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		fmt.Fprintf(stderr, "filter: %v\n", err)
		return 1
	}
	return 0
}
