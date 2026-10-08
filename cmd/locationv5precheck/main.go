package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	Accepted                          bool   `json:"accepted"`
	AttemptID                         string `json:"attemptID"`
	AuthorityCeiling                  int    `json:"authorityCeiling"`
	AuthorizedFreezeRootIdentity      string `json:"authorizedFreezeRootIdentity"`
	BaseCommit                        string `json:"baseCommit"`
	Completeness                      string `json:"completeness"`
	ExecutionRoot                     string `json:"executionRoot"`
	ExternalInference                 string `json:"externalInference"`
	FeatureIdentity                   string `json:"featureIdentity"`
	FreezeRoot                        string `json:"freezeRoot"`
	FrozenFilesImmutable              int    `json:"frozenFilesImmutable"`
	HeadAuthorityCeiling              int    `json:"headAuthorityCeiling"`
	NoPublicProductionReleasePush     bool   `json:"noPublicProductionReleasePush"`
	PhaseCommitRequiredBeforeAttempts bool   `json:"phaseCommitRequiredBeforeAttempts"`
	ProducerAssignments               int    `json:"producerAssignments"`
	ReviewerAssignments               int    `json:"reviewerAssignments"`
	Schema                            string `json:"schema"`
	Status                            string `json:"status"`
}
type freezeFile struct {
	Schema string `json:"schema"`
	Status string `json:"status"`
	Design struct {
		Accepted               bool     `json:"accepted"`
		Authority              int      `json:"authority"`
		Completeness           string   `json:"completeness"`
		Dispatch               bool     `json:"dispatch"`
		Execution              string   `json:"execution"`
		FeatureIdentity        string   `json:"featureIdentity"`
		ImmutablePredecessors  []string `json:"immutablePredecessors"`
		QualificationExecution bool     `json:"qualificationExecution"`
	} `json:"design"`
	Custody struct {
		EvaluatorSource        string `json:"evaluatorSource"`
		Independence           string `json:"independence"`
		InputBase              string `json:"inputBase"`
		InputCorrectionA       string `json:"inputCorrectionA"`
		InputCorrectionB       string `json:"inputCorrectionB"`
		Integration            string `json:"integration"`
		MergeBase              string `json:"mergeBase"`
		OracleSource           string `json:"oracleSource"`
		SkippedPatchEquivalent string `json:"skippedPatchEquivalent"`
		Spec                   string `json:"spec"`
	} `json:"custody"`
	Counts struct {
		BoundaryBundles   int `json:"boundaryBundles"`
		EvaluatorResults  int `json:"evaluatorResults"`
		Files             int `json:"files"`
		InputCases        int `json:"inputCases"`
		OracleDerivations int `json:"oracleDerivations"`
		OracleResults     int `json:"oracleResults"`
	} `json:"counts"`
	Files []struct {
		Path   string `json:"path"`
		Bytes  uint64 `json:"bytes"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
	RootIdentity string `json:"rootIdentity"`
}
type beforeFile struct {
	Schema                 string `json:"schema"`
	AuthorizedRootIdentity string `json:"authorizedRootIdentity"`
	FrozenRoot             string `json:"frozenRoot"`
	FrozenFileCount        int    `json:"frozenFileCount"`
	Files                  []rec  `json:"files"`
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
	if err := rejectDuplicateKeys(b); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: trailing json", path)
	}
	return nil
}
func rejectDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					kt, err := dec.Token()
					if err != nil {
						return err
					}
					k, ok := kt.(string)
					if !ok {
						return errors.New("object key not string")
					}
					if seen[k] {
						return fmt.Errorf("duplicate field %q", k)
					}
					seen[k] = true
					if err := walk(); err != nil {
						return err
					}
				}
				_, err := dec.Token()
				return err
			case '[':
				for dec.More() {
					if err := walk(); err != nil {
						return err
					}
				}
				_, err := dec.Token()
				return err
			}
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing json")
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
func writeNew(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create-new %s: %w", path, err)
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

func sameFiles(a []rec, b []rec) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	if !validAuthorization(a, f.RootIdentity) {
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
		if contains(c.Producer.MayRead, "oracle") || !contains(c.Producer.Forbidden, "external inference") {
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
	var before beforeFile
	beforeStatus := "absent"
	beforeMismatch := map[string]any{"present": false}
	if err := strict(filepath.Join(execRoot, "FROZEN230_MANIFEST.before.json"), &before); err == nil {
		beforeStatus = "present"
		beforeMismatch = map[string]any{"present": true, "authorizedRootIdentityMatches": before.AuthorizedRootIdentity == f.RootIdentity, "physicalSelfImageMatches": sameFiles(before.Files, phys), "zeroImageMatches": sameFiles(before.Files, zero), "preservedHistoricalDefect": before.AuthorizedRootIdentity == f.RootIdentity && !sameFiles(before.Files, phys) && !sameFiles(before.Files, zero), "accounting": "precheck uses authoritative FREEZE.json zero-image census; before manifest physical-self-image mismatch is preserved, accounted historical defect"}
	}
	out := map[string]any{"schema": "lsp-trace.adr0007.location-v5.precheck.v1", "status": "PRECHECK_PASS", "repoRoot": repo, "freezeRootIdentity": f.RootIdentity, "zeroImageFiles": len(zero), "physicalFiles": len(phys), "producerOutputs": 0, "reviewerOutputs": 0, "assignments": len(as.Cases), "authority": 0, "accepted": false, "completeness": "UNKNOWN", "beforeManifestStatus": beforeStatus, "beforeManifestAccounting": beforeMismatch}
	b, _ := json.MarshalIndent(out, "", "  ")
	b = append(b, '\n')
	must(writeNew(filepath.Join(execRoot, "PRECHECK.json"), b))
	fmt.Print(string(b))
}
func validAuthorization(a authFile, rootIdentity string) bool {
	return a.AuthorizedFreezeRootIdentity == rootIdentity &&
		a.AuthorityCeiling == 0 && a.HeadAuthorityCeiling == 0 && !a.Accepted &&
		a.Completeness == "UNKNOWN" && a.FeatureIdentity == "UNRESOLVED" &&
		a.ExternalInference == "DISABLED" && a.NoPublicProductionReleasePush &&
		a.PhaseCommitRequiredBeforeAttempts
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
