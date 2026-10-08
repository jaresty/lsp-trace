package main

import (
	"encoding/json"
	"fmt"
	sts "lsp-trace/internal/adr0007sourcetextsearchv1"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: adr0007-source-text-search-designverify ROOT")
		os.Exit(2)
	}
	f, err := sts.Census(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	fmt.Println(string(b))
}
