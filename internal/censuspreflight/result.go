package censuspreflight

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

const (
	SchemaVersion           = "lsp-trace.census-enumeration-preflight-result.v1"
	DiagnosticSchemaVersion = "lsp-trace.census-enumeration-diagnostic.v1"
)

type Limits struct {
	MaxEntries   int `json:"max_entries"`
	MaxAccepted  int `json:"max_accepted"`
	MaxWork      int `json:"max_work"`
	MaxPathBytes int `json:"max_path_bytes"`
	MaxDepth     int `json:"max_depth"`
}

type Counts struct {
	Entries  int `json:"entries"`
	Selected int `json:"selected"`
	Excluded int `json:"excluded"`
}

type Result struct {
	SchemaVersion       string   `json:"schema_version"`
	Roots               []string `json:"roots"`
	Includes            []string `json:"includes"`
	Excludes            []string `json:"excludes"`
	PrunedDirectories   []string `json:"pruned_directories"`
	Counts              Counts   `json:"counts"`
	SelectedPathsDigest string   `json:"selected_paths_digest"`
	Limits              Limits   `json:"limits"`
	Terminal            string   `json:"terminal"`
	Authority           int      `json:"authority"`
	Completeness        string   `json:"completeness"`
	AbsenceInference    bool     `json:"absence_inference"`
}

type Diagnostic struct {
	SchemaVersion string `json:"schema_version"`
	Category      string `json:"category"`
	Observed      int    `json:"observed"`
	Limit         int    `json:"limit"`
}

func DigestSelected(paths []string) string {
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	h := sha256.New()
	h.Write([]byte("lsp-trace.census-enumeration-preflight.selected-paths.v1\x00"))
	for _, p := range ordered {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func MarshalResult(v Result) ([]byte, error) {
	if v.SchemaVersion != SchemaVersion || v.Authority != 0 || v.Completeness != "UNKNOWN" || v.AbsenceInference || v.Terminal != "COMPLETE" {
		return nil, errors.New("invalid census enumeration preflight result")
	}
	return json.Marshal(v)
}
func MarshalDiagnostic(v Diagnostic) ([]byte, error) {
	if v.SchemaVersion != DiagnosticSchemaVersion || v.Observed != v.Limit+1 || v.Limit < 0 {
		return nil, errors.New("invalid census enumeration diagnostic")
	}
	switch v.Category {
	case "MAX_ENTRIES", "MAX_ACCEPTED", "MAX_WORK", "MAX_PATH_BYTES", "MAX_DEPTH":
	default:
		return nil, errors.New("invalid census enumeration diagnostic")
	}
	return json.Marshal(v)
}
