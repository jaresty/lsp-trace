package main

import (
	"fmt"
	"os"

	"lsp-trace/internal/locationexecutionv5"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: runcmd <execution-root> <frozen-root> <repo-root>")
		os.Exit(2)
	}
	if err := locationexecutionv5.Run(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("LOCATION_V5_EXECUTION_REPAIRED " + locationexecutionv5.RootIdentity)
}
