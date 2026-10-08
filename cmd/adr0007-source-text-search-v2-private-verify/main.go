package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

type manifest struct {
	SchemaVersion string `json:"schema_version"`
	RootSHA256    string `json:"root_sha256"`
	Files         []struct {
		Path, SHA256 string
		Bytes        int64
	} `json:"files"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: adr0007-source-text-search-v2-private-verify ROOT")
		os.Exit(2)
	}
	got, err := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v2-private-freeze", os.Args[1]).Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var m manifest
	if err := json.Unmarshal(got, &m); err != nil || m.SchemaVersion != "lsp-trace.adr0007.source-text-search.freeze.private.v2" || len(m.Files) == 0 {
		fmt.Fprintln(os.Stderr, "invalid freeze")
		os.Exit(1)
	}
	fmt.Printf("verified %s files=%d\n", m.RootSHA256, len(m.Files))
}
