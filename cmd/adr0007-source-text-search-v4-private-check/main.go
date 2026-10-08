package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

type Row struct {
	ID             string `json:"id"`
	Requirement    string `json:"requirement"`
	Stimulus       string `json:"stimulus"`
	ExpectedBranch string `json:"expected_branch"`
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
	ids := []string{}
	by := map[string]Row{}
	for _, r := range rows {
		ids = append(ids, r.ID)
		by[r.ID] = r
	}
	sort.Strings(ids)
	ok := true
	complete, failed := 0, 0
	for _, id := range ids {
		attempt := filepath.Join(root, id, "attempt.json")
		expectedPath := filepath.Join(root, id, "expected.oracle.terminal.json")
		eb, ee := os.ReadFile(expectedPath)
		pb, pe := run("./cmd/adr0007-source-text-search-v4-private-evaluate", attempt)
		ob, oe := run("./cmd/adr0007-source-text-search-v4-private-oracle", attempt)
		prodSame := pe == nil && ee == nil && bytes.Equal(pb, eb)
		oracleSame := oe == nil && ee == nil && bytes.Equal(ob, eb)
		outcome := extract(pb, "outcome")
		if outcome == "COMPLETE" {
			complete++
		} else {
			failed++
		}
		fmt.Printf("%s stimulus=%s expected=%s outcome=%s prod_expected=%v oracle_expected=%v\n", id, by[id].Stimulus, by[id].ExpectedBranch, outcome, prodSame, oracleSame)
		if !prodSame || !oracleSame {
			ok = false
			fmt.Print(firstDiff("production", pb, eb))
			fmt.Print(firstDiff("oracle", ob, eb))
		}
	}
	fmt.Printf("SUMMARY cases=%d complete=%d failed=%d\n", len(rows), complete, failed)
	if !ok {
		os.Exit(1)
	}
}
func run(pkg, path string) ([]byte, error) {
	c := exec.Command("go", "run", pkg, path)
	return c.Output()
}
func extract(b []byte, k string) string {
	var m map[string]any
	json.Unmarshal(b, &m)
	if s, ok := m[k].(string); ok {
		return s
	}
	return ""
}
func firstDiff(label string, a, b []byte) string {
	if bytes.Equal(a, b) {
		return ""
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return fmt.Sprintf("  %s_first_diff offset=%d got_len=%d expected_len=%d got=%q expected=%q\n", label, i, len(a), len(b), window(a, i), window(b, i))
}
func window(b []byte, i int) []byte {
	if i > len(b) {
		i = len(b)
	}
	e := i + 80
	if e > len(b) {
		e = len(b)
	}
	return b[i:e]
}
