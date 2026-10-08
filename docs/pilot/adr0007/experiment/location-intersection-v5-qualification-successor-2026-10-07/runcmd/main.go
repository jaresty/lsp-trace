package main

import (
	"fmt"
	"os"

	"lsp-trace/internal/locationexecutionv5"
)

func main() {
	if len(os.Args) != 4 && len(os.Args) != 5 && len(os.Args) != 6 {
		fmt.Fprintln(os.Stderr, "usage: runcmd <execRoot> <frozenRoot> <repoRoot> [phase [caseID]]")
		os.Exit(2)
	}
	phase := "Simulate"
	if len(os.Args) >= 5 {
		phase = os.Args[4]
	}
	if len(os.Args) == 6 && phase != "__producer" && phase != "__reviewer" {
		fmt.Fprintln(os.Stderr, "caseID argument is restricted to authenticated internal child phases")
		os.Exit(2)
	}
	switch phase {
	case "Plan", "Simulate", "Producers", "Reviewers", "Boundaries", "Reconcile":
	case "__producer", "__reviewer":
		if os.Getenv("LOCATIONEXECUTIONV5_CHILD_TOKEN") == "" {
			fmt.Fprintln(os.Stderr, "internal child phase unavailable publicly")
			os.Exit(2)
		}
	default:
		fmt.Fprintln(os.Stderr, "unsupported public phase: "+phase)
		os.Exit(2)
	}
	if err := locationexecutionv5.RunPhase(os.Args[1], os.Args[2], os.Args[3], phase); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("LOCATION_V5_SUCCESSOR_" + phase + " " + locationexecutionv5.RootIdentity)
}
