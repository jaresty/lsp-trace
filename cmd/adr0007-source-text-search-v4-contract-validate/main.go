package main

import (
	"encoding/json"
	"fmt"
	"os"

	"lsp-trace/internal/adr0007v4contractvalidator"
)

func main() {
	derive := false
	args := []string{}
	for _, a := range os.Args[1:] {
		if a == "--derive" {
			derive = true
			continue
		}
		args = append(args, a)
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: adr0007-source-text-search-v4-contract-validate [--derive] FILE...")
		os.Exit(2)
	}
	for _, p := range args {
		if !derive {
			if err := adr0007v4contractvalidator.ValidateFile(p); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
				os.Exit(1)
			}
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			os.Exit(1)
		}
		var bun adr0007v4contractvalidator.Bundle
		if err := json.Unmarshal(b, &bun); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			os.Exit(1)
		}
		got, err := adr0007v4contractvalidator.Derive(adr0007v4contractvalidator.DeriveInput{RawAttemptBytes: bun.RawAttemptBytes, AdmittedSourceBytes: bun.AdmittedSourceBytes, AdmittedBindingBytes: bun.AdmittedBindingBytes, ToolingManifestBytes: bun.ToolingManifestBytes, PredecessorManifestBytes: bun.PredecessorManifestBytes, PayloadFreezeBytes: bun.PayloadFreezeBytes})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s derive: %v\n", p, err)
			os.Exit(1)
		}
		fmt.Print(got)
	}
}
