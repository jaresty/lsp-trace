// Package adr0007locationv3 owns the prospective v3 freeze census contract.
package adr0007locationv3

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const FreezeSchema = "lsp-trace.adr0007.location.freeze.v3"

type FreezeEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type Freeze struct {
	Schema           string        `json:"schema"`
	State            string        `json:"state"`
	DispatchAllowed  bool          `json:"dispatchAllowed"`
	LocationExecuted bool          `json:"locationExecuted"`
	LocationDesignGO bool          `json:"locationDesignGO"`
	FileCount        int           `json:"fileCount"`
	Files            []FreezeEntry `json:"files"`
}

func CanonicalPath(p string) bool {
	return p != "" && p == filepath.ToSlash(p) && !strings.HasPrefix(p, "/") && !strings.Contains(p, "\\") && !strings.Contains(p, "//") && p != "." && p != ".." && !strings.HasPrefix(p, "../") && !strings.Contains(p, "/../") && path.Clean(p) == p
}
func Census(root string) ([]FreezeEntry, error) {
	abs, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	var out []FreezeEntry
	e = filepath.WalkDir(abs, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == abs {
			return nil
		}
		info, e := os.Lstat(p)
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink forbidden: %s", p)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special file forbidden: %s", p)
		}
		if filepath.Base(p) == "FREEZE.json" {
			return nil
		}
		rel, e := filepath.Rel(abs, p)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if !CanonicalPath(rel) {
			return fmt.Errorf("noncanonical census path %q", rel)
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		s := sha256.Sum256(b)
		out = append(out, FreezeEntry{rel, "sha256:" + hex.EncodeToString(s[:]), int64(len(b))})
		return nil
	})
	if e != nil {
		return nil, e
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
func Verify(root string, f Freeze) error {
	if f.Schema != FreezeSchema || f.State != "DESIGN_FROZEN_NON_DISPATCHING" || f.DispatchAllowed || f.LocationExecuted || f.LocationDesignGO {
		return errors.New("freeze header")
	}
	if f.FileCount != len(f.Files) {
		return errors.New("freeze count")
	}
	actual, e := Census(root)
	if e != nil {
		return e
	}
	if len(actual) != len(f.Files) {
		return fmt.Errorf("freeze census cardinality: actual=%d listed=%d", len(actual), len(f.Files))
	}
	seen := map[string]bool{}
	for i, w := range f.Files {
		if !CanonicalPath(w.Path) {
			return fmt.Errorf("noncanonical freeze path %q", w.Path)
		}
		if seen[w.Path] {
			return fmt.Errorf("duplicate freeze path %q", w.Path)
		}
		seen[w.Path] = true
		if i > 0 && f.Files[i-1].Path >= w.Path {
			return errors.New("freeze paths not strictly sorted")
		}
		if len(w.SHA256) != 71 || !strings.HasPrefix(w.SHA256, "sha256:") {
			return errors.New("malformed digest")
		}
		if _, e := hex.DecodeString(w.SHA256[7:]); e != nil {
			return errors.New("malformed digest")
		}
		if w.Bytes < 0 {
			return errors.New("negative bytes")
		}
		a := actual[i]
		if a != w {
			return fmt.Errorf("freeze identity mismatch at %d: actual=%+v listed=%+v", i, a, w)
		}
	}
	return nil
}
