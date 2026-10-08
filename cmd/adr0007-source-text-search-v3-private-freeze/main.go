package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type entry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type freeze struct {
	SchemaVersion string  `json:"schema_version"`
	RootSHA256    string  `json:"root_sha256"`
	Files         []entry `json:"files"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func rootOf(files []entry) string {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	pre := []byte{}
	for _, e := range files {
		pre = append(pre, []byte(fmt.Sprintf("%s\x00%s\x00%d\x00", e.Path, e.SHA256, e.Bytes))...)
	}
	return digest(pre)
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--check" {
		os.Args = []string{os.Args[0]}
	}
	b, err := os.ReadFile(filepath.Join("docs", "pilot", "adr0007", "source-text-search-v3", "FREEZE.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var f freeze
	if err := json.Unmarshal(b, &f); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	actual := []entry{}
	bad := false
	for _, e := range f.Files {
		rb, err := os.ReadFile(e.Path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "missing", e.Path)
			bad = true
			continue
		}
		ae := entry{Path: e.Path, SHA256: digest(rb), Bytes: int64(len(rb))}
		if filepath.ToSlash(e.Path) == "docs/pilot/adr0007/source-text-search-v3/FREEZE.json" {
			ae.SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			ae.Bytes = 0
		}
		if ae != e {
			fmt.Fprintln(os.Stderr, "mismatch", e.Path)
			bad = true
		}
		actual = append(actual, ae)
	}
	if rootOf(actual) != f.RootSHA256 {
		fmt.Fprintln(os.Stderr, "root mismatch")
		bad = true
	}
	if bad {
		os.Exit(1)
	}
	fmt.Println(f.RootSHA256)
}
