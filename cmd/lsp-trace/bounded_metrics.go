package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/boundedmetrics"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"os"
)

func admitBoundedMetrics(raw []byte) error {
	_, err := boundedmetrics.ValidateFor(raw, boundedmetrics.Family, "v1")
	return err
}
func runBoundedMetrics(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bounded-retained-metrics", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", "", "immutable generation selector (no replace)")
	if fs.Parse(args) != nil || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace bounded-retained-metrics [--output SELECTOR] PATH|-")
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
	raw, err := io.ReadAll(io.LimitReader(reader, boundedmetrics.MaxInputBytes+1))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = boundedanalysis.Preflight(raw, boundedmetrics.MaxInputBytes); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	input, _ := json.Marshal(map[string]any{"input": string(raw)})
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	executor := operation.NewOffline(validator, map[operation.Name]operation.Handler{operation.BoundedRetainedMetrics: operation.BoundedRetainedMetricsHandler})
	result, failure := executor.Execute(context.Background(), operation.Request{Name: operation.BoundedRetainedMetrics, Input: input})
	if failure != nil {
		fmt.Fprintln(stderr, failure)
		return 1
	}
	if *output != "" {
		err = publishValidatedBundle(*output, result.Artifact, admitBoundedMetrics)
	} else {
		_, err = stdout.Write(result.Artifact)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
