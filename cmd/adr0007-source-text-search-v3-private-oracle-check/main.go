package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func sha(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func main() {
	root := "."
	freeze := filepath.Join(root, "docs/pilot/adr0007/source-text-search-v3/FREEZE.json")
	fb, err := os.ReadFile(freeze)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !bytes.Contains(fb, []byte("root_sha256")) {
		fmt.Fprintln(os.Stderr, "bad freeze")
		os.Exit(1)
	}
	bad := false
	filepath.WalkDir(filepath.Join(root, "docs/pilot/adr0007/source-text-search-v3"), func(p string, d fs.DirEntry, e error) error {
		if e != nil || d.IsDir() {
			return nil
		}
		b, _ := os.ReadFile(p)
		if len(b) == 0 && !strings.HasSuffix(p, ".expected.json") {
			fmt.Fprintln(os.Stderr, "empty", p)
			bad = true
		}
		return nil
	})
	if b, err := os.ReadFile(filepath.Join(root, "docs/pilot/adr0007/source-text-search-v3/pinned/sourceadmissionv2/admission.go")); err != nil || sha(b) != "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da" {
		fmt.Fprintln(os.Stderr, "pin mismatch")
		bad = true
	}
	if bad {
		os.Exit(1)
	}
	fmt.Println("verified")
}
