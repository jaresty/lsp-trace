package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	traceschema "lsp-trace/internal/schema"
)

const schemaGetUsage = "usage: lsp-trace schema get (--family graph|inspect|filter --version VERSION | --schema v1|v2|v3)"

func runSchema(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "get" {
		fmt.Fprintln(stderr, schemaGetUsage)
		return 1
	}
	fs := flag.NewFlagSet("schema get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	family := fs.String("family", "", "schema family: graph, inspect, or filter")
	version := fs.String("version", "", "family schema version")
	alias := fs.String("schema", "", "graph schema version compatibility alias")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		if err == nil {
			fmt.Fprintln(stderr, schemaGetUsage)
		}
		return 1
	}
	var data []byte
	var err error
	switch {
	case *alias != "" && *family == "" && *version == "":
		data, err = traceschema.Bytes(*alias)
	case *alias == "" && *family != "" && *version != "":
		data, err = traceschema.BytesFor(*family, *version)
	default:
		fmt.Fprintln(stderr, schemaGetUsage)
		return 1
	}
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

const validateUsage = "usage: lsp-trace validate [--family graph|inspect|filter --version VERSION | --schema v1|v2|v3] PATH|-"

func runValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	family := fs.String("family", "", "schema family: graph, inspect, or filter")
	version := fs.String("version", "", "family schema version")
	alias := fs.String("schema", "", "graph schema version compatibility alias")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || (*family == "") != (*version == "") || (*alias != "" && *family != "") {
		fmt.Fprintln(stderr, validateUsage)
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
	var detected string
	if *family == "" {
		detected, err = traceschema.Validate(data, *alias)
	} else {
		detected, err = traceschema.ValidateFor(data, *family, *version)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "valid %s\n", detected)
	return 0
}
