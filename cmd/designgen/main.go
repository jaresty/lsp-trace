package main

import (
	"flag"
	"fmt"
	"os"

	"lsp-trace/internal/adr0007locationv5"
)

func main() {
	src := flag.String("src", "", "source root")
	out := flag.String("out", "", "empty output root")
	flag.Parse()
	if *src == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: designgen -src ROOT -out EMPTY_ROOT")
		os.Exit(2)
	}
	if err := adr0007locationv5.CopyTree(*src, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f, err := adr0007locationv5.WriteFreezeLast(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s %s\n", f.RootIdentity, *out)
}
