package main

import (
	"fmt"
	oracle "lsp-trace/internal/adr0007sourcetextsearchv4oracle"
	"os"
)

func main() {
	root := oracle.CasesDir
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if err := oracle.Check(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("oracle verified")
}
