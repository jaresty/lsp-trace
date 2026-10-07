package main

import (
	"fmt"
	"os"
	"sort"

	group "lsp-trace/internal/groupqualificationv1"
)

func main() {
	if len(os.Args) == 2 {
		id, err := group.VerifyFreeze(os.Args[1])
		if err == nil {
			err = group.VerifyArtifacts(os.Args[1])
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("FREEZE_VERIFIED " + id)
		printCounts(os.Args[1])
		return
	}
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: freeze ROOT [PACKAGE_DIR]")
		os.Exit(2)
	}
	id, err := group.GenerateArtifacts(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = group.VerifyArtifacts(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("FREEZE_VERIFIED " + id)
	printCounts(os.Args[1])
}

func printCounts(root string) {
	counts, err := group.ValidateAllSchemaArtifacts(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("SCHEMA_ARTIFACTS %s %d\n", name, counts[name])
	}
}
