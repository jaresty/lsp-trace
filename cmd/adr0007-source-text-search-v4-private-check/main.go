package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

type Row struct {
	ID              string `json:"id"`
	Requirement     string `json:"requirement"`
	Stimulus        string `json:"stimulus"`
	Assertion       string `json:"assertion"`
	ExpectedOutcome string `json:"expected_outcome"`
	ExpectedCode    string `json:"expected_code"`
}

func main() {
	manifest := readManifest("docs/pilot/adr0007/source-text-search-v4/PAYLOAD_MANIFEST.json")
	root := "docs/pilot/adr0007/source-text-search-v4/cases"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	rows := readRows(root)
	dirs := map[string]bool{}
	entries, err := os.ReadDir(root)
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			fail("symlink in case root: " + e.Name())
		}
		if e.IsDir() {
			dirs[e.Name()] = true
		}
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.ID] {
			fail("duplicate id " + r.ID)
		}
		seen[r.ID] = true
		ids = append(ids, r.ID)
		if !dirs[r.ID] {
			fail("missing case dir " + r.ID)
		}
		validateRow(root, r)
	}
	for d := range dirs {
		if !seen[d] {
			fail("extra case dir " + d)
		}
	}
	sort.Strings(ids)
	complete, failed := 0, 0
	for _, id := range ids {
		r := find(rows, id)
		out, err := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v4-private-evaluate", filepath.Join(root, id, "attempt.json")).Output()
		if err != nil {
			fail(fmt.Sprintf("%s process error: %v", id, err))
		}
		var tr map[string]any
		if err := json.Unmarshal(out, &tr); err != nil {
			fail(id + " invalid terminal json")
		}
		outcome, _ := tr["outcome"].(string)
		code := ""
		if f, ok := tr["failure"].(map[string]any); ok && f != nil {
			code, _ = f["code"].(string)
		}
		if outcome != r.ExpectedOutcome || code != r.ExpectedCode {
			fail(fmt.Sprintf("%s expected %s/%s got %s/%s", id, r.ExpectedOutcome, r.ExpectedCode, outcome, code))
		}
		replay, _ := tr["replay"].(map[string]any)
		if replay["tooling_identity_sha256"] != manifest["tooling_digest"] || replay["predecessor_lock_sha256"] != manifest["predecessor_lock_digest"] {
			fail(id + " replay manifest digest mismatch")
		}
		if outcome == "COMPLETE" {
			complete++
		} else {
			failed++
		}
		fmt.Printf("%s stimulus=%s assertion=%s outcome=%s code=%s\n", id, r.Stimulus, r.Assertion, outcome, code)
	}
	fmt.Printf("SUMMARY cases=%d complete=%d failed=%d\n", len(rows), complete, failed)
}
func readManifest(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		panic(err)
	}
	return m
}

func readRows(root string) []Row {
	b, err := os.ReadFile(filepath.Join(root, "CASE_MATRIX.json"))
	if err != nil {
		panic(err)
	}
	var rows []Row
	if err := json.Unmarshal(b, &rows); err != nil {
		panic(err)
	}
	return rows
}
func find(rows []Row, id string) Row {
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	panic(id)
}
func validateRow(root string, r Row) {
	if r.ID == "" || r.Requirement == "" || r.Stimulus == "" || r.Assertion == "" || r.ExpectedOutcome == "" {
		fail("incomplete row " + r.ID)
	}
	if r.ExpectedOutcome != "COMPLETE" && r.ExpectedOutcome != "FAILED" {
		fail("bad outcome " + r.ID)
	}
	if r.ExpectedOutcome == "COMPLETE" && r.ExpectedCode != "" {
		fail("complete with code " + r.ID)
	}
	if r.ExpectedOutcome == "FAILED" && r.ExpectedCode == "" {
		fail("failed missing code " + r.ID)
	}
	dir := filepath.Join(root, r.ID)
	mustFile(filepath.Join(dir, "attempt.json"))
	if _, err := os.Lstat(filepath.Join(dir, "source")); err != nil {
		fail("missing source dir " + r.ID)
	}
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			fail(err.Error())
		}
		if d.Type()&os.ModeSymlink != 0 {
			fail("symlink " + p)
		}
		return nil
	})
}
func mustFile(p string) {
	st, err := os.Lstat(p)
	if err != nil || !st.Mode().IsRegular() {
		fail("missing file " + p)
	}
}
func fail(s string) { fmt.Fprintln(os.Stderr, s); os.Exit(1) }
