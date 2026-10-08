package main

import (
	"fmt"
	oracle "lsp-trace/internal/adr0007sourcetextsearchv4oracle"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: adr0007-source-text-search-v4-private-oracle <attempt.json>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	tr := oracle.EvaluateRaw(raw)
	os.Stdout.Write(oracle.Canon(tr))
}
