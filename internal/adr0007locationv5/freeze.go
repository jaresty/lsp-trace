package adr0007locationv5

import (
	"bytes"
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

const FreezeSchema = "lsp-trace.adr0007.location-intersection.freeze.private.v5"

var NormativeRootFiles = []string{
	"ALGORITHM.md", "DEFERRED_BOUNDARIES.json", "DESIGN.md", "FINAL_CUSTODY.json", "REPORT.md",
	"INPUT_CORPUS.json", "INPUT_SCHEMAS.json", "LIMITS.json", "POLICY.json", "PREDECESSORS.md",
	"SPEC_MANIFEST.json", "SPEC_REVIEW_PENDING.md", "limits.schema.json", "policy.schema.json",
	"request.schema.json", "result.schema.json", "source-binding.schema.json",
}

var RequiredDirs = []string{"inputs", "evaluator-candidate", "oracle-candidate"}

type FileMeasurement struct {
	Path   string `json:"path"`
	Bytes  uint64 `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Freeze struct {
	Schema       string            `json:"schema"`
	Status       string            `json:"status"`
	Design       map[string]any    `json:"design"`
	Custody      map[string]string `json:"custody"`
	Counts       map[string]uint64 `json:"counts"`
	Files        []FileMeasurement `json:"files"`
	RootIdentity string            `json:"rootIdentity"`
}

func HashBytes(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }

func Census(root string) ([]FileMeasurement, error) {
	var out []FileMeasurement
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(rel, ".git/") || rel == ".git" || strings.Contains(rel, "__pycache__") {
			return fs.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode&fs.ModeSymlink != 0 {
			return fmt.Errorf("fail closed: symlink %s", rel)
		}
		if d.IsDir() {
			return nil
		}
		if !mode.IsRegular() {
			return fmt.Errorf("fail closed: special file %s", rel)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out = append(out, FileMeasurement{Path: rel, Bytes: uint64(len(b)), SHA256: HashBytes(b)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func BuildFreeze(root string) (Freeze, []byte, error) {
	files, err := Census(root)
	if err != nil {
		return Freeze{}, nil, err
	}
	counts, err := validateCensus(root, files)
	if err != nil {
		return Freeze{}, nil, err
	}
	seenFreeze := false
	for i := range files {
		if files[i].Path == "FREEZE.json" {
			files[i].Bytes = 0
			files[i].SHA256 = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
			seenFreeze = true
		}
	}
	if !seenFreeze {
		files = append(files, FileMeasurement{Path: "FREEZE.json", Bytes: 0, SHA256: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"})
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		counts["files"] = uint64(len(files))
	}
	f := Freeze{Schema: FreezeSchema, Status: "IMPLEMENTATION_AGREEMENT_GO", Design: map[string]any{"accepted": false, "authority": 0, "completeness": "UNKNOWN", "featureIdentity": "UNRESOLVED", "execution": "designGO", "dispatch": false, "qualificationExecution": false, "immutablePredecessors": []string{"v1", "v2", "v3", "v4"}}, Custody: map[string]string{"spec": "637680d2", "inputBase": "base", "inputCorrectionA": "64afd44b", "inputCorrectionB": "1546da22", "evaluatorSource": "fccc0a03", "oracleSource": "1cff6a76", "integration": "a9c82f1f", "mergeBase": "d78e54d3", "skippedPatchEquivalent": "7354f53c", "independence": "procedural-not-cryptographic"}, Counts: counts, Files: files}
	identityInput := bytes.Buffer{}
	for _, m := range files {
		fmt.Fprintf(&identityInput, "%s %d %s\n", m.SHA256, m.Bytes, m.Path)
	}
	f.RootIdentity = HashBytes(identityInput.Bytes())
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return Freeze{}, nil, err
	}
	return f, append(b, '\n'), nil
}

func VerifyFreeze(root string) (Freeze, error) {
	want, b, err := BuildFreeze(root)
	if err != nil {
		return Freeze{}, err
	}
	got, err := os.ReadFile(filepath.Join(root, "FREEZE.json"))
	if err != nil {
		return Freeze{}, err
	}
	if !bytes.Equal(got, b) {
		return Freeze{}, errors.New("FREEZE mismatch")
	}
	return want, nil
}

func validateCensus(root string, files []FileMeasurement) (map[string]uint64, error) {
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.Path] = true
	}
	for _, f := range NormativeRootFiles {
		if !seen[f] {
			return nil, fmt.Errorf("missing normative file %s", f)
		}
	}
	for _, d := range RequiredDirs {
		if info, err := os.Stat(filepath.Join(root, d)); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("missing required dir %s", d)
		}
	}
	inputs, err := os.ReadDir(filepath.Join(root, "inputs"))
	if err != nil {
		return nil, err
	}
	caseCount := uint64(0)
	for _, e := range inputs {
		if e.IsDir() {
			caseCount++
		}
	}
	if caseCount != 26 {
		return nil, fmt.Errorf("input case count got %d want 26", caseCount)
	}
	for _, base := range []string{"evaluator-candidate", "oracle-candidate"} {
		if !seen[base+"/MANIFEST.json"] {
			return nil, fmt.Errorf("missing %s/MANIFEST.json", base)
		}
	}
	evalResults := countSuffix(files, "evaluator-candidate/cases/", "/RESULT.json")
	oracleResults := countSuffix(files, "oracle-candidate/cases/", "/RESULT.json")
	oracleDerivations := countSuffix(files, "oracle-candidate/cases/", "/DERIVATION.json")
	boundaries := countSuffix(files, "oracle-candidate/boundaries/", "/BOUNDARY.json")
	if evalResults != 26 || oracleResults != 26 || oracleDerivations != 26 || boundaries != 4 {
		return nil, fmt.Errorf("bad generated counts eval=%d oracle=%d deriv=%d boundary=%d", evalResults, oracleResults, oracleDerivations, boundaries)
	}
	return map[string]uint64{"inputCases": caseCount, "evaluatorResults": evalResults, "oracleResults": oracleResults, "oracleDerivations": oracleDerivations, "boundaryBundles": boundaries, "files": uint64(len(files))}, nil
}

func countSuffix(files []FileMeasurement, prefix, suffix string) uint64 {
	var n uint64
	for _, f := range files {
		if strings.HasPrefix(f.Path, prefix) && strings.HasSuffix(f.Path, suffix) {
			n++
		}
	}
	return n
}

func CopyTree(src, dst string) error {
	if entries, err := os.ReadDir(dst); err != nil {
		return err
	} else if len(entries) != 0 {
		return errors.New("output root must be empty")
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if strings.Contains(rel, "__pycache__") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("fail closed: symlink %s", rel)
		}
		to := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("fail closed: special file %s", rel)
		}
		if rel == "FREEZE.json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o644)
	})
}

func WriteFreezeLast(root string) (Freeze, error) {
	f, b, err := BuildFreeze(root)
	if err != nil {
		return Freeze{}, err
	}
	if err := os.WriteFile(filepath.Join(root, "FREEZE.json"), b, 0o644); err != nil {
		return Freeze{}, err
	}
	return f, nil
}
