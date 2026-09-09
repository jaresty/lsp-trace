package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/normativeanalytics"
)

type relationFilters []string

func (r *relationFilters) String() string     { return fmt.Sprint([]string(*r)) }
func (r *relationFilters) Set(v string) error { *r = append(*r, v); return nil }

func runBoundedAnalyticsV2(command string, op normativeanalytics.Operation, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	operationName := fs.String("operation", "", "ANALYSIS, METRICS or RANKING")
	maxWork := fs.Int64("max-work", 0, "positive local work bound")
	claim := fs.String("claim-level", "", "VERIFIED or UNVERIFIED")
	output := fs.String("output", "", "immutable generation selector (no replace)")
	_ = fs.String("provenance", "", "optional provenance JSON (does not grant authority)")
	var filters relationFilters
	fs.Var(&filters, "filter", "repeatable retained relation")
	if fs.Parse(args) != nil || fs.NArg() != 1 || normativeanalytics.Operation(*operationName) != op || (*claim != "VERIFIED" && *claim != "UNVERIFIED") || len(filters) == 0 {
		fmt.Fprintf(stderr, "usage: lsp-trace %s --operation %s --filter RELATION --max-work N --claim-level VERIFIED|UNVERIFIED [--provenance JSON] [--output SELECTOR] PATH|-\n", command, op)
		return 1
	}
	raw, err := readBoundedV2Input(fs.Arg(0), stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	graph, _, err := normativeanalytics.DecodeRetainedGraph(raw)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result, err := normativeanalytics.EvaluateLocal(normativeanalytics.LocalRequest{Operation: op, BuildRevision: graph.BuildRevision, RetainedJSON: raw, Relations: filters, Policy: normativeanalytics.LocalPolicy{MaxWork: *maxWork}})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	artifact, err := normativeanalytics.MarshalLocalResult(result)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *output != "" {
		if err := publishValidatedBundle(*output, artifact, func(candidate []byte) error { return normativeanalytics.ValidateLocalResultJSON(candidate, raw) }); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	stdout.Write(artifact)
	return 0
}

func readBoundedV2Input(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(io.LimitReader(stdin, normativeanalytics.MaxRetainedBytes+1))
	}
	return os.ReadFile(path)
}
