package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	traceschema "lsp-trace/internal/schema"
)

func runSchema(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "get" {
		fmt.Fprintln(stderr, "usage: lsp-trace schema get --schema v1|v2|v3")
		return 1
	}
	fs := flag.NewFlagSet("schema get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	version := fs.String("schema", "v3", "schema version")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		if err == nil {
			fmt.Fprintln(stderr, "usage: lsp-trace schema get --schema v1|v2|v3")
		}
		return 1
	}
	data, err := traceschema.Bytes(*version)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err := stdout.Write(data); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	version := fs.String("schema", "", "required schema version")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace validate [--schema v1|v2|v3] PATH|-")
		return 1
	}
	var data []byte
	var err error
	if fs.Arg(0) == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(fs.Arg(0))
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	detected, err := traceschema.Validate(data, *version)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "valid %s\n", detected)
	return 0
}
