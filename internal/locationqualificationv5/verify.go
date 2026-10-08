package locationqualificationv5

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const AuthorizedRootIdentity = "sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d"

type fileID struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type freezeFile struct {
	Schema       string          `json:"schema"`
	Status       string          `json:"status"`
	Design       json.RawMessage `json:"design"`
	Custody      json.RawMessage `json:"custody"`
	Counts       json.RawMessage `json:"counts"`
	RootIdentity string          `json:"rootIdentity"`
	Files        []fileID        `json:"files"`
}

type executionManifest struct {
	Schema                       string `json:"schema"`
	Status                       string `json:"status"`
	Frozen230Unchanged           bool   `json:"frozen230Unchanged"`
	ProducerAttempts             int    `json:"producerAttempts"`
	ReviewerAttempts             int    `json:"reviewerAttempts"`
	ByteEquality                 int    `json:"byteEquality"`
	DerivationBindings           int    `json:"derivationBindings"`
	BoundaryReplays              int    `json:"boundaryReplays"`
	Retries                      int    `json:"retries"`
	SemanticRepairs              int    `json:"semanticRepairs"`
	Substitutions                int    `json:"substitutions"`
	ExternalInference            int    `json:"externalInference"`
	Authority                    int    `json:"authority"`
	Accepted                     bool   `json:"accepted"`
	Completeness                 string `json:"completeness"`
	FeatureIdentity              string `json:"featureIdentity"`
	FinalLocationCustodyGoIssued bool   `json:"finalLocationCustodyGoIssued"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }

func readStrict(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.Decode(&struct{}{}) == nil {
		return errors.New("trailing json")
	}
	return nil
}

func treeFiles(root string) ([]fileID, error) {
	var out []fileID
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if rel == "FREEZE.json" {
			b = nil
		}
		out = append(out, fileID{Path: rel, Bytes: int64(len(b)), SHA256: digest(b)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

func VerifyFreeze(frozenRoot string) (string, error) {
	var f freezeFile
	if err := readStrict(filepath.Join(frozenRoot, "FREEZE.json"), &f); err != nil {
		return "", err
	}
	if f.RootIdentity != AuthorizedRootIdentity {
		return "", fmt.Errorf("root identity %s", f.RootIdentity)
	}
	if len(f.Files) != 230 {
		return "", fmt.Errorf("freeze manifest count %d", len(f.Files))
	}
	actual, err := treeFiles(frozenRoot)
	if err != nil {
		return "", err
	}
	if len(actual) != 230 {
		return "", fmt.Errorf("actual file count %d", len(actual))
	}
	for i := range actual {
		if actual[i] != f.Files[i] {
			return "", fmt.Errorf("frozen file mismatch %s", f.Files[i].Path)
		}
	}
	return f.RootIdentity, nil
}

func VerifyExecution(execRoot, frozenRoot string) error {
	if _, err := VerifyFreeze(frozenRoot); err != nil {
		return err
	}
	var m executionManifest
	if err := readStrict(filepath.Join(execRoot, "EXECUTION_MANIFEST.json"), &m); err != nil {
		return err
	}
	if m.Schema != "lsp-trace.adr0007.location-v5-execution.manifest.v1" || m.Status != "FINAL_AUDIT_CANDIDATE_PENDING_INDEPENDENT_AUDIT" {
		return errors.New("manifest status")
	}
	if !m.Frozen230Unchanged || m.ProducerAttempts != 26 || m.ReviewerAttempts != 26 || m.ByteEquality != 26 || m.DerivationBindings != 26 || m.BoundaryReplays != 4 {
		return errors.New("manifest counts")
	}
	if m.Retries != 0 || m.SemanticRepairs != 0 || m.Substitutions != 0 || m.ExternalInference != 0 {
		return errors.New("forbidden activity")
	}
	if m.Authority != 0 || m.Accepted || m.Completeness != "UNKNOWN" || m.FeatureIdentity != "UNRESOLVED" || m.FinalLocationCustodyGoIssued {
		return errors.New("authority/custody")
	}
	return nil
}
