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
	freezePath := filepath.Join("docs", "pilot", "adr0007", "source-text-search-v3", "FREEZE.json")
	b, err := os.ReadFile(freezePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var f freeze
	if err := json.Unmarshal(b, &f); err != nil {
		fmt.Fprintln(os.Stderr, "bad freeze json", err)
		os.Exit(1)
	}
	if f.SchemaVersion != "lsp-trace.adr0007.source-text-search.freeze.private.v3" {
		fmt.Fprintln(os.Stderr, "bad freeze schema")
		os.Exit(1)
	}
	seen := map[string]bool{}
	actual := make([]entry, 0, len(f.Files))
	bad := false
	for _, e := range f.Files {
		if seen[e.Path] {
			fmt.Fprintln(os.Stderr, "duplicate", e.Path)
			bad = true
		}
		seen[e.Path] = true
		rb, err := os.ReadFile(e.Path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "missing", e.Path, err)
			bad = true
			continue
		}
		ae := entry{Path: e.Path, SHA256: digest(rb), Bytes: int64(len(rb))}
		if e.Path == freezePath {
			ae.SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			ae.Bytes = 0
		}
		if ae != e {
			fmt.Fprintf(os.Stderr, "freeze mismatch %s expected %s/%d got %s/%d\n", e.Path, e.SHA256, e.Bytes, ae.SHA256, ae.Bytes)
			bad = true
		}
		actual = append(actual, ae)
	}
	if root := rootOf(actual); root != f.RootSHA256 {
		fmt.Fprintf(os.Stderr, "root mismatch expected %s got %s\n", f.RootSHA256, root)
		bad = true
	}
	if pb, err := os.ReadFile(filepath.Join("docs", "pilot", "adr0007", "source-text-search-v3", "pinned", "sourceadmissionv2", "admission.go")); err != nil || digest(pb) != "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da" {
		fmt.Fprintln(os.Stderr, "pin mismatch")
		bad = true
	}
	if bad {
		os.Exit(1)
	}
	fmt.Printf("verified root=%s files=%d\n", f.RootSHA256, len(f.Files))
}
