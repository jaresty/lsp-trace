package main

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

type assignmentFile struct {
	Schema    string `json:"schema"`
	AttemptID string `json:"attemptID"`
	Cases     []struct {
		CaseID   string `json:"caseID"`
		Producer struct {
			AssignmentID, AttemptID, Role string
			Forbidden, MayRead            []string
		} `json:"producer"`
		Reviewer struct {
			AssignmentID, AttemptID, Role string
			Forbidden, MayRead            []string
		} `json:"reviewer"`
	} `json:"cases"`
}
type authFile struct {
	Schema, Status, AttemptID, AuthorizedFreezeRootIdentity, FreezeRoot, ExecutionRoot, Completeness, FeatureIdentity string
	AuthorityCeiling                                                                                                  int  `json:"authorityCeiling"`
	Accepted                                                                                                          bool `json:"accepted"`
	ProducerAssignments                                                                                               int  `json:"producerAssignments"`
	ReviewerAssignments                                                                                               int  `json:"reviewerAssignments"`
	FrozenFilesImmutable                                                                                              int  `json:"frozenFilesImmutable"`
}
type freezeFile struct {
	RootIdentity string `json:"rootIdentity"`
	Files        []struct {
		Path   string `json:"path"`
		Bytes  uint64 `json:"bytes"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}
type rec struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func strict(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if dec.Decode(&struct{}{}) == nil {
		return fmt.Errorf("%s: trailing json", path)
	}
	return nil
}
func files(root string, zeroFreeze bool) ([]rec, error) {
	var out []rec
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if zeroFreeze && rel == "FREEZE.json" {
			b = nil
		}
		out = append(out, rec{rel, len(b), digest(b)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: locationv5precheck <repo-root> <execution-root> <frozen-root>")
		os.Exit(2)
	}
	repo, execRoot, frozenRoot := os.Args[1], os.Args[2], os.Args[3]
	var f freezeFile
	must(strict(filepath.Join(frozenRoot, "FREEZE.json"), &f))
	zero, err := files(frozenRoot, true)
	must(err)
	if len(zero) != len(f.Files) {
		fail("zero-image census count")
	}
	for i := range zero {
		if zero[i].Path != f.Files[i].Path || uint64(zero[i].Bytes) != f.Files[i].Bytes || zero[i].SHA256 != f.Files[i].SHA256 {
			fail("zero-image census mismatch " + zero[i].Path)
		}
	}
	var a authFile
	must(strict(filepath.Join(execRoot, "AUTHORIZATION.json"), &a))
	if a.AuthorizedFreezeRootIdentity != f.RootIdentity || a.AuthorityCeiling != 0 || a.Accepted || a.Completeness != "UNKNOWN" || a.FeatureIdentity != "UNRESOLVED" {
		fail("authorization ceilings")
	}
	var as assignmentFile
	must(strict(filepath.Join(execRoot, "ASSIGNMENTS.json"), &as))
	if len(as.Cases) != 26 || a.ProducerAssignments != 26 || a.ReviewerAssignments != 26 {
		fail("assignment count")
	}
	seen := map[string]bool{}
	for _, c := range as.Cases {
		if c.CaseID == "" || seen[c.Producer.AssignmentID] || seen[c.Reviewer.AssignmentID] {
			fail("assignment uniqueness")
		}
		seen[c.Producer.AssignmentID] = true
		seen[c.Reviewer.AssignmentID] = true
		if c.Producer.Role != "producer" || c.Reviewer.Role != "reviewer" {
			fail("assignment role")
		}
		if contains(c.Producer.MayRead, "oracle") || contains(c.Producer.Forbidden, "external inference") == false {
			fail("producer boundary")
		}
		if _, err := os.Stat(filepath.Join(frozenRoot, "inputs", c.CaseID, "REQUEST.raw.json")); err != nil {
			fail("missing input " + c.CaseID)
		}
	}
	prod, _ := filepath.Glob(filepath.Join(execRoot, "producer", "*"))
	rev, _ := filepath.Glob(filepath.Join(execRoot, "reviewer", "*"))
	if len(prod) != 0 || len(rev) != 0 {
		fail("precheck requires zero producer/reviewer output")
	}
	phys, _ := files(frozenRoot, false)
	out := map[string]any{"schema": "lsp-trace.adr0007.location-v5.precheck.v1", "status": "PRECHECK_PASS", "repoRoot": repo, "freezeRootIdentity": f.RootIdentity, "zeroImageFiles": len(zero), "physicalFiles": len(phys), "producerOutputs": 0, "reviewerOutputs": 0, "assignments": len(as.Cases), "authority": 0, "accepted": false, "completeness": "UNKNOWN"}
	b, _ := json.MarshalIndent(out, "", "  ")
	b = append(b, '\n')
	must(os.WriteFile(filepath.Join(execRoot, "PRECHECK.json"), b, 0o644))
	fmt.Print(string(b))
}
func contains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}
func fail(s string) { must(errors.New(s)) }
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
