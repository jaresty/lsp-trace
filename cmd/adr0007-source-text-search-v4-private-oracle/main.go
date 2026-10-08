package main

import (
	"fmt"
	oracle "lsp-trace/internal/adr0007sourcetextsearchv4oracle"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: oracle <attempt.json>")
		os.Exit(2)
	}
	b, err := oracle.EvaluateFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
}
