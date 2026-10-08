package main

import (
	"fmt"
	"os"

	"lsp-trace/internal/locationqualificationv5"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: verifycmd <execution-root> <frozen-root>")
		os.Exit(2)
	}
	if _, err := locationqualificationv5.VerifyFreeze(os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := locationqualificationv5.VerifyExecution(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("LOCATION_V5_EXECUTION_VERIFIED sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d")
}
