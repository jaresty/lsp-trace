package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
)

func runProgramCCompose(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("program-c compose", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if fs.Parse(args) != nil || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace program-c compose PATH|-")
		return 1
	}
	var raw []byte
	var err error
	if fs.Arg(0) == "-" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(fs.Arg(0))
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = validator.ValidateOperationInput(operation.ProgramCCompose, raw); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result, failure := operation.ProgramCComposeHandler(context.Background(), operation.Request{Name: operation.ProgramCCompose, Input: raw})
	if failure != nil {
		fmt.Fprintln(stderr, failure)
		return 1
	}
	if _, err = stdout.Write(result.Artifact); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
