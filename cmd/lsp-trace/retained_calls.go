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
	"lsp-trace/internal/retainedrelations"
)

func admitRetainedCallsVersion(version string) func([]byte) error {
	return func(raw []byte) error {
		_, err := retainedcalls.ValidateFor(raw, retainedcalls.Family, version)
		return err
	}
}
func admitRetainedCalls(raw []byte) error { return admitRetainedCallsVersion("v1")(raw) }
func runRetainedCalls(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export-retained-calls", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", "", "immutable generation selector (no replace)")
	version := fs.String("version", "v1", "retained contract version: v1, v2, or v3")
	if fs.Parse(args) != nil || fs.NArg() != 1 || (*version != "v1" && *version != "v2" && *version != "v3") {
		fmt.Fprintln(stderr, "usage: lsp-trace export-retained-calls [--version v1|v2|v3] [--output SELECTOR] PATH|-")
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
	var artifact []byte
	if *version == "v2" {
		artifact, err = retainedcalls.ExportV2(raw)
	} else if *version == "v3" {
		artifact, err = retainedrelations.Export(raw)
	} else {
		// Keep the historical v1 operation request byte-for-byte unchanged.
		input, _ := json.Marshal(map[string]any{"input": string(raw)})
		validator, validatorErr := mcpcontract.NewOperationInputValidator()
		if validatorErr != nil {
			fmt.Fprintln(stderr, validatorErr)
			return 1
		}
		executor := operation.NewOffline(validator, map[operation.Name]operation.Handler{operation.ExportRetainedCalls: operation.ExportRetainedCallsHandler})
		result, failure := executor.ExecuteExportRetainedCalls(context.Background(), operation.Request{Input: input})
		if failure != nil {
			fmt.Fprintln(stderr, failure)
			return 1
		}
		artifact = result.Artifact
	}
	if err == nil {
		if *version == "v3" {
			_, err = retainedrelations.Validate(artifact)
		} else {
			_, err = retainedcalls.ValidateFor(artifact, retainedcalls.Family, *version)
		}
	}
	admit := admitRetainedCallsVersion(*version)
	if *version == "v3" {
		admit = func(raw []byte) error { _, err := retainedrelations.Validate(raw); return err }
	}
	if *output != "" && err == nil {
		err = publishValidatedBundle(*output, artifact, admit)
	} else if err == nil {
		_, err = stdout.Write(artifact)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
