package main

import (
	"fmt"
	"os"

	"lsp-trace/internal/censuscontinuation"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: read_diagnostic PATH")
		os.Exit(2)
	}
	record, err := censuscontinuation.ReadLastManagedPreparationDiagnostic(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(record.String())
}
