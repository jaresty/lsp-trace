package main

import (
	"fmt"
	"os"

	"lsp-trace/internal/adr0007v4contractvalidator"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: adr0007-source-text-search-v4-contract-validate FILE...")
		os.Exit(2)
	}
	for _, p := range os.Args[1:] {
		if err := adr0007v4contractvalidator.ValidateFile(p); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			os.Exit(1)
		}
	}
}
