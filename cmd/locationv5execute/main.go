package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"lsp-trace/internal/adr0007locationv5"
)

type processCustody struct {
	Executable    string            `json:"executable"`
	Argv          []string          `json:"argv"`
	SourceDigests map[string]string `json:"sourceDigests"`
	InputDigests  map[string]string `json:"inputDigests"`
	ExitCode      int               `json:"exitCode"`
	StdoutSHA256  string            `json:"stdoutSha256"`
	StderrSHA256  string            `json:"stderrSha256"`
	StdoutBytes   int               `json:"stdoutBytes"`
	StderrBytes   int               `json:"stderrBytes"`
}

type envelope struct {
	Schema         string          `json:"schema"`
	CaseID         string          `json:"caseId"`
	AssignmentID   string          `json:"assignmentId"`
	Result         json.RawMessage `json:"result"`
	ProcessCustody processCustody  `json:"processCustody"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }

func read(path string) ([]byte, error) { return os.ReadFile(path) }

func sourceDigest() map[string]string {
	_, file, _, ok := runtime.Caller(0)
	out := map[string]string{}
	if ok {
		if b, err := os.ReadFile(file); err == nil {
			out[filepath.ToSlash(file)] = digest(b)
		}
	}
	_, evalFile, _, ok := runtime.Caller(1)
	_ = evalFile
	return out
}

func main() {
	stderr := []byte{}
	if len(os.Args) != 5 {
		stderr = []byte("usage: locationv5execute <case-id> <assignment-id> <input-dir> <out-json>\n")
		os.Stderr.Write(stderr)
		os.Exit(2)
	}
	caseID, assignmentID, inputDir, outPath := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	if strings.Contains(inputDir, "evaluator-candidate") || strings.Contains(inputDir, "oracle-candidate") {
		stderr = []byte("producer input dir must not be evaluator-candidate or oracle-candidate\n")
		os.Stderr.Write(stderr)
		os.Exit(2)
	}
	raw, err := read(filepath.Join(inputDir, "REQUEST.raw.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var bind []byte
	if b, err := read(filepath.Join(inputDir, "BINDING.json")); err == nil {
		bind = b
	} else if _, err2 := os.Stat(filepath.Join(inputDir, "BINDING.ABSENT")); err2 == nil {
		bind = nil
	} else {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cond, err := read(filepath.Join(inputDir, "CONDITION.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctrl := adr0007locationv5.StaticControl{}
	if strings.Contains(string(cond), "CANCEL") {
		ctrl.Cancel = true
	}
	if strings.Contains(string(cond), "DEADLINE") {
		ctrl.Deadline = true
	}
	res, err := adr0007locationv5.Evaluate(raw, bind, ctrl, adr0007locationv5.PublishedLimits())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	resBytes, err := adr0007locationv5.Canonical(res)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	custody := processCustody{Executable: os.Args[0], Argv: append([]string{}, os.Args...), SourceDigests: sourceDigest(), InputDigests: map[string]string{"REQUEST.raw.json": digest(raw), "CONDITION.json": digest(cond)}, ExitCode: 0, StdoutSHA256: digest(resBytes), StderrSHA256: digest(nil), StdoutBytes: len(resBytes), StderrBytes: 0}
	if bind == nil {
		custody.InputDigests["BINDING.ABSENT"] = "present"
	} else {
		custody.InputDigests["BINDING.json"] = digest(bind)
	}
	env := envelope{Schema: "lsp-trace.adr0007.location-v5.producer-execution.v1", CaseID: caseID, AssignmentID: assignmentID, Result: json.RawMessage(resBytes), ProcessCustody: custody}
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(resBytes)
}
