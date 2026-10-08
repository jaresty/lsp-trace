package main

import (
	"fmt"
	v4 "lsp-trace/internal/adr0007sourcetextsearchv4private"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: adr0007-source-text-search-v4-private-evaluate <attempt.json>")
		os.Exit(2)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	tr := v4.EvaluateRaw(b)
	os.Stdout.Write(v4.Canon(tr))
}
