package main

import (
	"flag"
	"fmt"
	"os"

	"lsp-trace/internal/adr0007locationv5"
)

func main() {
	root := flag.String("root", "", "freeze root")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "usage: designverify -root ROOT")
		os.Exit(2)
	}
	f, err := adr0007locationv5.VerifyFreeze(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s files=%d cases=%d\n", f.RootIdentity, f.Counts["files"], f.Counts["inputCases"])
}
