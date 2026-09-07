package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"os"
)

func admitBoundedAnalysis(raw []byte) error {
	_, err := boundedanalysis.ValidateFor(raw, boundedanalysis.Family, "v1")
	return err
}
func runBoundedAnalysis(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bounded-retained-analysis", flag.ContinueOnError)
	fs.SetOutput(stderr)
	op := fs.String("operation", "", "PROJECT, PATH or COMPONENTS")
	start := fs.String("start", "", "exact historical start node ID")
	end := fs.String("end", "", "exact historical end node ID")
	mode := fs.String("mode", "", "explicit WEAK or STRONG components")
	work := fs.Int("max-work", boundedanalysis.MaxWork, "bounded traversal work, 1..1000000")
	output := fs.String("output", "", "immutable generation selector (no replace)")
	if fs.Parse(args) != nil || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace bounded-retained-analysis --operation PROJECT|PATH|COMPONENTS [--start ID --end ID | --mode WEAK|STRONG] [--max-work N] [--output SELECTOR] PATH|-")
		return 1
	}
	reader := stdin
	if fs.Arg(0) != "-" {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer f.Close()
		reader = f
	}
	raw, err := io.ReadAll(io.LimitReader(reader, boundedanalysis.MaxInputBytes+1))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(raw) > boundedanalysis.MaxInputBytes {
		fmt.Fprintln(stderr, "bounded retained input byte LIMIT")
		return 1
	}
	value := map[string]any{"input": string(raw), "operation": *op, "max_work": *work}
	if *start != "" {
		value["start"] = *start
	}
	if *end != "" {
		value["end"] = *end
	}
	if *mode != "" {
		value["mode"] = *mode
	}
	input, _ := json.Marshal(value)
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	executor := operation.NewOffline(validator, map[operation.Name]operation.Handler{operation.BoundedRetainedAnalysis: operation.BoundedRetainedAnalysisHandler})
	result, failure := executor.Execute(context.Background(), operation.Request{Name: operation.BoundedRetainedAnalysis, Input: input})
	if failure != nil {
		fmt.Fprintln(stderr, failure)
		return 1
	}
	if *output != "" {
		err = publishValidatedBundle(*output, result.Artifact, admitBoundedAnalysis)
	} else {
		_, err = stdout.Write(result.Artifact)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
