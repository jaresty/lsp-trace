package main

import (
	"fmt"
	"os"

	"lsp-trace/internal/locationexecutionv5"
)

func main() {
	if len(os.Args) != 4 && len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: runcmd <execRoot> <frozenRoot> <repoRoot> [phase]")
		os.Exit(2)
	}
	phase := "Simulate"
	if len(os.Args) == 5 {
		phase = os.Args[4]
	}
	if err := locationexecutionv5.RunPhase(os.Args[1], os.Args[2], os.Args[3], phase); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("LOCATION_V5_SUCCESSOR_" + phase + " " + locationexecutionv5.RootIdentity)
}
