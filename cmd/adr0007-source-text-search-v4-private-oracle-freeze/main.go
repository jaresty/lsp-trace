package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Row struct {
	ID string `json:"id"`
}

func main() {
	root := "docs/pilot/adr0007/source-text-search-v4/cases"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	mb, err := os.ReadFile(filepath.Join(root, "CASE_MATRIX.json"))
	if err != nil {
		panic(err)
	}
	var rows []Row
	if err := json.Unmarshal(mb, &rows); err != nil {
		panic(err)
	}
	for _, r := range rows {
		attempt := filepath.Join(root, r.ID, "attempt.json")
		out, err := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v4-private-oracle", attempt).Output()
		if err != nil {
			panic(fmt.Sprintf("%s: %v", r.ID, err))
		}
		if err := os.WriteFile(filepath.Join(root, r.ID, "expected.oracle.terminal.json"), out, 0444); err != nil {
			panic(err)
		}
		fmt.Println("wrote", r.ID)
	}
}
