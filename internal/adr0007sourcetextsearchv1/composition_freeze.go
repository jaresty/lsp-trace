package adr0007sourcetextsearchv1

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LocationRangeUnionCandidate struct {
	SchemaVersion           string        `json:"schema_version"`
	LocationDesignRoot      string        `json:"location_design_root"`
	LocationSuccessorCommit string        `json:"location_successor_commit"`
	LocationSealCommit      string        `json:"location_seal_commit"`
	LocationFinalSeal       string        `json:"location_final_seal"`
	Source                  SourceBinding `json:"source"`
	Ranges                  []Range       `json:"ranges"`
	ExecutedLocation        bool          `json:"executed_location"`
}

func ComposeLocationCandidate(root, successor, sealCommit, finalSeal string, src SourceBinding, matches []Match) LocationRangeUnionCandidate {
	rs := make([]Range, len(matches))
	for i := range matches {
		rs[i] = matches[i].Range
	}
	return LocationRangeUnionCandidate{SchemaVersion: SchemaLocationV1, LocationDesignRoot: root, LocationSuccessorCommit: successor, LocationSealCommit: sealCommit, LocationFinalSeal: finalSeal, Source: src, Ranges: rs, ExecutedLocation: false}
}

type FreezeFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type Freeze struct {
	SchemaVersion string       `json:"schema_version"`
	RootSHA256    string       `json:"root_sha256"`
	Files         []FreezeFile `json:"files"`
}

func Census(root string) (Freeze, error) {
	var files []FreezeFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return ErrFailClosed
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, path)
		files = append(files, FreezeFile{Path: filepath.ToSlash(rel), SHA256: "sha256:" + hex.EncodeToString(h[:]), Bytes: int64(len(b))})
		return nil
	})
	if err != nil {
		return Freeze{}, err
	}
	sort.Slice(files, func(i, j int) bool { return strings.Compare(files[i].Path, files[j].Path) < 0 })
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.SHA256))
		h.Write([]byte{0})
	}
	return Freeze{SchemaVersion: SchemaFreezeV1, RootSHA256: "sha256:" + hex.EncodeToString(h.Sum(nil)), Files: files}, nil
}
