package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"lsp-trace/internal/adr0007locationv5"
)

type conditionFile struct {
	Cancel          bool `json:"cancel"`
	DeadlineExpired bool `json:"deadlineExpired"`
}

type childToken struct {
	Schema       string `json:"schema"`
	Token        string `json:"token"`
	Mode         string `json:"mode"`
	Phase        string `json:"phase"`
	CaseID       string `json:"caseId"`
	AssignmentID string `json:"assignmentId"`
	DispatchID   string `json:"dispatchId"`
	Head         string `json:"head"`
}

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
	out := map[string]string{}
	if _, file, _, ok := runtime.Caller(0); ok {
		if b, err := os.ReadFile(file); err == nil {
			out[filepath.ToSlash(file)] = digest(b)
		}
	}
	if _, file, _, ok := runtime.Caller(1); ok {
		if b, err := os.ReadFile(file); err == nil {
			out[filepath.ToSlash(file)] = digest(b)
		}
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		eval := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "internal", "adr0007locationv5", "evaluator.go"))
		if b, err := os.ReadFile(eval); err == nil {
			out[filepath.ToSlash(eval)] = digest(b)
		}
	}
	return out
}

func strictJSON(path string, raw []byte, v any) error {
	if err := rejectDuplicateKeys(raw); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
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

func verifyChildToken(path, mode, phase, caseID, assignmentID string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("child token required: %w", err)
	}
	var tok childToken
	if err := strictJSON(path, raw, &tok); err != nil {
		return err
	}
	if tok.Schema != "lsp-trace.adr0007.location-v5.child-token.v1" || tok.Mode != mode || tok.Phase != phase || tok.CaseID != caseID || tok.AssignmentID != assignmentID || tok.DispatchID == "" || tok.Head == "" {
		return errors.New("child token binding mismatch")
	}
	if tok.Token == "" || os.Getenv("LOCATIONV5_CHILD_TOKEN") != tok.Token {
		return errors.New("child token authentication failed")
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
					ktok, err := dec.Token()
					if err != nil {
						return err
					}
					k, ok := ktok.(string)
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
	if dec.More() {
		return errors.New("trailing json")
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing json")
	}
	return nil
}

func main() {
	stderr := []byte{}
	if len(os.Args) != 6 {
		stderr = []byte("usage: locationv5execute <case-id> <assignment-id> <input-dir> <out-json> <child-token-json>\n")
		os.Stderr.Write(stderr)
		os.Exit(2)
	}
	caseID, assignmentID, inputDir, outPath, tokenPath := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	if err := verifyChildToken(tokenPath, "__producer", "producer", caseID, assignmentID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cleanInput := filepath.ToSlash(filepath.Clean(inputDir))
	if strings.Contains(cleanInput, "evaluator-candidate") || strings.Contains(cleanInput, "oracle-candidate") {
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
	condRaw, err := read(filepath.Join(inputDir, "CONDITION.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var cond conditionFile
	if err := strictJSON("CONDITION.json", condRaw, &cond); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctrl := adr0007locationv5.StaticControl{Cancel: cond.Cancel, Deadline: cond.DeadlineExpired}
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
	custody := processCustody{Executable: os.Args[0], Argv: append([]string{}, os.Args...), SourceDigests: sourceDigest(), InputDigests: map[string]string{"REQUEST.raw.json": digest(raw), "CONDITION.json": digest(condRaw)}, ExitCode: 0, StdoutSHA256: digest(resBytes), StderrSHA256: digest(nil), StdoutBytes: len(resBytes), StderrBytes: 0}
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
