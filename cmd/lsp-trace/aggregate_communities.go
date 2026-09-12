package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/communityregister"
)

type pathFlags []string

func (p *pathFlags) String() string     { return fmt.Sprint([]string(*p)) }
func (p *pathFlags) Set(v string) error { *p = append(*p, v); return nil }

func runAggregateCommunities(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("aggregate-communities", flag.ContinueOnError)
	fs.SetOutput(stderr)
	graphPath := fs.String("graph", "", "exact Graph Provenance V5 artifact")
	output := fs.String("output", "", "write register to PATH instead of stdout")
	var partitions pathFlags
	fs.Var(&partitions, "partition", "retained Leiden partition artifact (repeatable)")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *graphPath == "" || len(partitions) == 0 {
		fmt.Fprintln(stderr, "usage: lsp-trace aggregate-communities --graph PATH --partition PATH [--partition PATH...] [--output PATH]")
		return 1
	}
	graphRaw, err := os.ReadFile(*graphPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	raws := make([][]byte, 0, len(partitions))
	for _, p := range partitions {
		b, e := os.ReadFile(p)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		raws = append(raws, b)
	}
	register, err := communityregister.Aggregate(graphRaw, raws...)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data, err := communityregister.JSON(register)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *output != "" {
		if err = os.WriteFile(*output, data, 0600); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if _, err = stdout.Write(data); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
