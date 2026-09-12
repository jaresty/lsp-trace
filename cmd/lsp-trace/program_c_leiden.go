package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/communityregister"
	"lsp-trace/internal/programcpresentation"
	"lsp-trace/internal/schema"
)

func runProgramCLeiden(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("program-c leiden", flag.ContinueOnError)
	fs.SetOutput(stderr)
	seed := fs.Uint64("seed", 0, "deterministic Leiden seed")
	pagerankTopK := fs.Int("pagerank-top-k", 0, "mandatory positive PageRank top-k")
	hubTopK := fs.Int("hub-top-k", 0, "mandatory positive hub top-k")
	format := fs.String("format", "text", "text or json")
	output := fs.String("output", "", "immutable generation selector (no replace)")
	emitRegister := fs.String("emit-community-register", "", "write a separate schema-validated technical community register")
	if fs.Parse(args) != nil || fs.NArg() != 1 || (*format != "text" && *format != "json") || *pagerankTopK < 1 || *hubTopK < 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace program-c leiden --seed N --pagerank-top-k N --hub-top-k N [--format text|json] [--output SELECTOR] [--emit-community-register PATH] PATH|-")
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
	a, err := programcpresentation.Handle(programcpresentation.Request{Input: raw, Seed: *seed, PageRankTopK: *pagerankTopK, HubTopK: *hubTopK})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data, err := programcpresentation.JSON(a)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *emitRegister != "" {
		register, registerErr := communityregister.Aggregate(raw, data)
		if registerErr != nil {
			fmt.Fprintln(stderr, registerErr)
			return 1
		}
		registerData, registerErr := communityregister.JSON(register)
		if registerErr != nil {
			fmt.Fprintln(stderr, registerErr)
			return 1
		}
		if registerErr = os.WriteFile(*emitRegister, registerData, 0600); registerErr != nil {
			fmt.Fprintln(stderr, registerErr)
			return 1
		}
	}
	if *output != "" {
		if err = publishValidatedBundle(*output, data, func(b []byte) error {
			_, e := schema.ValidateFor(b, schema.FamilyCommunityPresentation, "v1")
			return e
		}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if *format == "json" {
		if *output == "" {
			_, err = stdout.Write(data)
		}
	} else {
		err = programcpresentation.Text(stdout, a)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
