package main

import (
	"fmt"
	"os"

	"lsp-trace/internal/searchqualificationv11"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: verifycmd <frozen-root>")
		os.Exit(2)
	}
	freeze, err := searchqualificationv11.VerifyFreeze(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := searchqualificationv11.VerifyArtifacts(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("EXACT_ROOT_VERIFIED " + freeze)
}
