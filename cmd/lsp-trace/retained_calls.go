package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/retainedcalls"
)

func admitRetainedCalls(raw []byte) error {
	_, err := retainedcalls.ValidateFor(raw, retainedcalls.Family, "v1")
	return err
}
func runRetainedCalls(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export-retained-calls", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", "", "immutable generation selector (no replace)")
	if fs.Parse(args) != nil || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace export-retained-calls [--output SELECTOR] PATH|-")
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
	raw, err := io.ReadAll(io.LimitReader(reader, graphprovenance.MaxEnvelopeBytes+1))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(raw) > graphprovenance.MaxEnvelopeBytes {
		fmt.Fprintln(stderr, "provenance envelope byte limit")
		return 1
	}
	input, _ := json.Marshal(map[string]any{"input": string(raw)})
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	executor := operation.NewOffline(validator, map[operation.Name]operation.Handler{operation.ExportRetainedCalls: operation.ExportRetainedCallsHandler})
	result, failure := executor.ExecuteExportRetainedCalls(context.Background(), operation.Request{Input: input})
	if failure != nil {
		fmt.Fprintln(stderr, failure)
		return 1
	}
	if *output != "" {
		err = publishValidatedBundle(*output, result.Artifact, admitRetainedCalls)
	} else {
		_, err = stdout.Write(result.Artifact)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
