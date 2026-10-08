package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type fileRec struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type manifest struct {
	SchemaVersion string    `json:"schema_version"`
	RootSHA256    string    `json:"root_sha256"`
	Files         []fileRec `json:"files"`
}

const zero = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: adr0007-source-text-search-v2-private-freeze ROOT")
		os.Exit(2)
	}
	m, err := census(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(m, "", "  ")
	os.Stdout.Write(append(out, '\n'))
}
func census(root string) (manifest, error) {
	var files []fileRec
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink rejected")
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "FREEZE.json" {
			files = append(files, fileRec{Path: rel, SHA256: zero, Bytes: 0})
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		files = append(files, fileRec{Path: rel, SHA256: "sha256:" + hex.EncodeToString(h[:]), Bytes: int64(len(b))})
		return nil
	})
	if err != nil {
		return manifest{}, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.SHA256))
		h.Write([]byte{0})
		h.Write([]byte(fmt.Sprint(f.Bytes)))
		h.Write([]byte{0})
	}
	return manifest{SchemaVersion: "lsp-trace.adr0007.source-text-search.freeze.private.v2", RootSHA256: "sha256:" + hex.EncodeToString(h.Sum(nil)), Files: files}, nil
}
