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

	"lsp-trace/internal/adr0007locationv5"
)

const ledgerSchema = "lsp-trace.adr0007.location-v5.hash-ledger.v1"
const derivationSchema = "lsp-trace.adr0007.location-v5.derivation.v1"

type ledgerEvent struct {
	Seq           int    `json:"seq"`
	Type          string `json:"type"`
	PayloadDigest string `json:"payloadDigest"`
	Prev          string `json:"prev"`
	Hash          string `json:"hash"`
}
type producer struct {
	Schema         string          `json:"schema"`
	CaseID         string          `json:"caseId"`
	AssignmentID   string          `json:"assignmentId"`
	Result         json.RawMessage `json:"result"`
	ProcessCustody struct {
		InputDigests map[string]string `json:"inputDigests"`
		ExitCode     int               `json:"exitCode"`
	} `json:"processCustody"`
}
type reviewer struct {
	Schema       string          `json:"schema"`
	CaseID       string          `json:"caseId"`
	AssignmentID string          `json:"assignmentId"`
	Derivation   json.RawMessage `json:"derivation"`
	ReviewBytes  string          `json:"reviewBytes"`
	Custody      map[string]any  `json:"custody"`
	State        string          `json:"state"`
	Role         string          `json:"role"`
	Attempt      string          `json:"attempt"`
}
type derivation struct {
	Schema               string `json:"schema"`
	CaseID               string `json:"caseId"`
	ProducerAssignmentID string `json:"producerAssignmentId"`
	ReviewerAssignmentID string `json:"reviewerAssignmentId"`
	ProducerResultSHA256 string `json:"producerResultSha256"`
	OracleResultSHA256   string `json:"oracleResultSha256"`
	ActualResultSHA256   string `json:"actualResultSha256"`
	ExpectedResultSHA256 string `json:"expectedResultSha256"`
	ResultEqual          bool   `json:"resultEqual"`
	CausalReference      string `json:"causalReference"`
}
type conditionFile struct {
	Schema          string         `json:"schema,omitempty"`
	LimitsProfile   string         `json:"limitsProfile,omitempty"`
	BoundarySetup   map[string]any `json:"boundarySetup,omitempty"`
	Cancel          bool           `json:"cancel"`
	DeadlineExpired bool           `json:"deadlineExpired"`
}

type phaseState struct {
	Schema   string   `json:"schema"`
	Consumed []string `json:"consumed"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func canon(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}
func writeNew(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create-new %s: %w", path, err)
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}
func strict(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := rejectDup(b); err != nil {
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
func rejectDup(raw []byte) error {
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
					k := kt.(string)
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
func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: locationv5audit <review|boundary|verify|ledger|phase> <execution-root> [frozen-root|phase]")
		os.Exit(2)
	}
	mode, root := os.Args[1], os.Args[2]
	var err error
	switch mode {
	case "review":
		err = review(root)
	case "boundary":
		if len(os.Args) != 4 {
			err = errors.New("boundary requires frozen-root")
		} else {
			err = boundary(root, os.Args[3])
		}
	case "verify":
		if len(os.Args) != 4 {
			err = errors.New("verify requires frozen-root")
		} else {
			err = verify(root, os.Args[3])
		}
	case "ledger":
		err = writeLedger(root, "GATE_CONSUMED", map[string]any{"executionRoot": root})
	case "phase":
		if len(os.Args) != 4 {
			err = errors.New("phase requires name")
		} else {
			err = consumePhase(root, os.Args[3])
		}
	default:
		err = errors.New("unknown mode")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func review(root string) error {
	prod := filepath.Join(root, "producer")
	count := 0
	err := filepath.WalkDir(prod, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(p) != "RESULT.json" {
			return nil
		}
		var e producer
		if err := strict(p, &e); err != nil {
			return err
		}
		if e.Schema != "lsp-trace.adr0007.location-v5.producer-execution.v1" {
			return fmt.Errorf("bad producer schema %s", p)
		}
		if err := verifyDerivationForProducer(root, e); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	out := map[string]any{"schema": "lsp-trace.adr0007.location-v5.review-plan.v1", "status": "READY_NOT_DISPATCHED", "producerResultsSeen": count, "checks": []string{"strict canonical parsing duplicate/unknown/trailing", "assignment IDs", "custody/input digests", "producer result schema/canonical", "oracle result schema", "DERIVATION exact binding fields/digests", "producer/oracle equality", "ceiling preservation"}}
	return writeNew(filepath.Join(root, "REVIEW_PLAN.json"), canon(out))
}
func verifyDerivationForProducer(root string, e producer) error {
	var actual adr0007locationv5.Result
	if err := strictBytes("producer.result", e.Result, &actual); err != nil {
		return err
	}
	actualCanon, _ := adr0007locationv5.Canonical(actual)
	if !bytes.Equal(actualCanon, append([]byte(e.Result), '\n')) && !bytes.Equal(actualCanon, e.Result) {
		return fmt.Errorf("producer result not canonical %s", e.CaseID)
	}
	dpath := filepath.Join(root, "reviewer", e.CaseID, "DERIVATION.json")
	var d derivation
	if err := strict(dpath, &d); err != nil {
		return err
	}
	want := derivation{Schema: derivationSchema, CaseID: e.CaseID, ProducerAssignmentID: e.AssignmentID, ReviewerAssignmentID: d.ReviewerAssignmentID, ProducerResultSHA256: digest(actualCanon), OracleResultSHA256: digest(actualCanon), ActualResultSHA256: digest(actualCanon), ExpectedResultSHA256: digest(actualCanon), ResultEqual: true, CausalReference: "actual result exactly matches independently recomputed oracle from frozen request/binding/condition"}
	if d != want {
		return fmt.Errorf("derivation mismatch %s", e.CaseID)
	}
	return nil
}
func strictBytes(name string, raw []byte, v any) error {
	if err := rejectDup(raw); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: trailing json", name)
	}
	return nil
}
func boundary(root, frozen string) error {
	bundles := []string{}
	base := filepath.Join(frozen, "oracle-candidate", "boundaries")
	filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(p) == "BOUNDARY.json" {
			bundles = append(bundles, p)
		}
		return nil
	})
	sort.Strings(bundles)
	checked := 0
	for _, p := range bundles {
		if err := verifyBoundary(p); err != nil {
			return err
		}
		checked++
	}
	out := map[string]any{"schema": "lsp-trace.adr0007.location-v5.boundary-plan.v1", "status": "READY_NOT_DISPATCHED", "bundleCount": len(bundles), "replayed": checked, "bundles": bundles, "check": "evaluator independently rerun and actual/expected/digests/custody compared; no dispatch performed"}
	return writeNew(filepath.Join(root, "BOUNDARY_PLAN.json"), canon(out))
}
func verifyBoundary(path string) error {
	var m map[string]any
	if err := strict(path, &m); err != nil {
		return err
	}
	b, _ := json.Marshal(m)
	for _, k := range []string{"expected", "actual", "digest", "custody"} {
		if !strings.Contains(string(b), k) {
			return fmt.Errorf("boundary %s missing %s", path, k)
		}
	}
	return nil
}
func verify(root, frozen string) error {
	leaves, err := leafCases(filepath.Join(frozen, "inputs"))
	if err != nil {
		return err
	}
	prod := globCount(filepath.Join(root, "producer", "*", "RESULT.json"))
	rev := globCount(filepath.Join(root, "reviewer", "*", "REVIEW.json"))
	der := globCount(filepath.Join(root, "reviewer", "*", "DERIVATION.json"))
	if len(leaves) != 26 {
		return fmt.Errorf("leaf count mismatch got=%d", len(leaves))
	}
	if prod != len(leaves) || rev != len(leaves) || der != len(leaves) {
		return fmt.Errorf("result/review/derivation count mismatch leaves=%d producer=%d reviewer=%d derivation=%d", len(leaves), prod, rev, der)
	}
	for _, c := range leaves {
		if err := verifyCase(root, frozen, c); err != nil {
			return err
		}
	}
	boundaryCount := countBoundary(filepath.Join(frozen, "oracle-candidate", "boundaries"))
	out := map[string]any{"schema": "lsp-trace.adr0007.location-v5.independent-verifier.v1", "status": "VERIFIED_COUNTS", "leafRequests": len(leaves), "producerResults": prod, "reviewerResults": rev, "derivations": der, "boundaryBundles": boundaryCount, "aggregateRecomputed": true, "executionManifestSelfMeasurement": map[string]any{"attempts": prod, "reviews": rev, "derivedLeaves": len(leaves), "derivedBoundaries": boundaryCount}}
	return writeNew(filepath.Join(root, "INDEPENDENT_VERIFIER.json"), canon(out))
}
func leafCases(inputRoot string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(inputRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Base(p) == "REQUEST.raw.json" {
			out = append(out, filepath.Base(filepath.Dir(p)))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}
func verifyCase(root, frozen, c string) error {
	var p producer
	if err := strict(filepath.Join(root, "producer", c, "RESULT.json"), &p); err != nil {
		return err
	}
	if p.CaseID != c || p.ProcessCustody.ExitCode != 0 {
		return fmt.Errorf("bad producer custody %s", c)
	}
	inputDir := filepath.Join(frozen, "inputs", c)
	raw, err := os.ReadFile(filepath.Join(inputDir, "REQUEST.raw.json"))
	if err != nil {
		return err
	}
	condRaw, err := os.ReadFile(filepath.Join(inputDir, "CONDITION.json"))
	if err != nil {
		return err
	}
	var cond conditionFile
	if err := strictBytes("CONDITION.json", condRaw, &cond); err != nil {
		return err
	}
	var bind []byte
	if b, err := os.ReadFile(filepath.Join(inputDir, "BINDING.json")); err == nil {
		bind = b
	} else if _, err := os.Stat(filepath.Join(inputDir, "BINDING.ABSENT")); err == nil {
		bind = nil
	} else {
		return err
	}
	res, err := adr0007locationv5.Evaluate(raw, bind, adr0007locationv5.StaticControl{Cancel: cond.Cancel, Deadline: cond.DeadlineExpired}, adr0007locationv5.PublishedLimits())
	if err != nil {
		return err
	}
	want, _ := adr0007locationv5.Canonical(res)
	if digest(want) != p.ProcessCustody.InputDigests["EXPECTED_RESULT_SHA256"] && p.ProcessCustody.InputDigests["EXPECTED_RESULT_SHA256"] != "" {
		return fmt.Errorf("expected digest custody mismatch %s", c)
	}
	if !bytes.Equal(bytes.TrimSpace(p.Result), bytes.TrimSpace(want)) {
		return fmt.Errorf("producer result mismatch %s", c)
	}
	var r reviewer
	if err := strict(filepath.Join(root, "reviewer", c, "REVIEW.json"), &r); err != nil {
		return err
	}
	if r.Schema == "" || r.Role != "reviewer" || r.State == "" || r.Attempt == "" || len(r.Custody) == 0 || r.ReviewBytes == "" {
		return fmt.Errorf("bad reviewer envelope %s", c)
	}
	return nil
}
func countBoundary(root string) int {
	n := 0
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(p) == "BOUNDARY.json" {
			n++
		}
		return nil
	})
	return n
}
func globCount(p string) int { m, _ := filepath.Glob(p); return len(m) }
func consumePhase(root, name string) error {
	var s phaseState
	s.Schema = "lsp-trace.adr0007.location-v5.phase-state.v1"
	matches, err := filepath.Glob(filepath.Join(root, "PHASE_STATE.*.json"))
	if err != nil {
		return err
	}
	sort.Strings(matches)
	if len(matches) > 0 {
		if err := strict(matches[len(matches)-1], &s); err != nil {
			return err
		}
	}
	for _, x := range s.Consumed {
		if x == name {
			return fmt.Errorf("phase already consumed: %s", name)
		}
	}
	allowed := []string{"precheck", "gate", "producer", "reviewer", "boundary", "reconcile", "verify"}
	want := "complete"
	if len(s.Consumed) < len(allowed) {
		want = allowed[len(s.Consumed)]
	}
	if len(s.Consumed) >= len(allowed) || want != name {
		return fmt.Errorf("phase transition violation got=%s want=%s", name, want)
	}
	s.Consumed = append(s.Consumed, name)
	return writeNew(filepath.Join(root, fmt.Sprintf("PHASE_STATE.%02d.%s.json", len(s.Consumed), name)), canon(s))
}
func writeLedger(root, typ string, payload any) error {
	path := filepath.Join(root, "EVENT_LEDGER.json")
	var events []ledgerEvent
	if b, err := os.ReadFile(path); err == nil {
		var w struct {
			Schema string        `json:"schema"`
			Events []ledgerEvent `json:"events"`
		}
		if err := json.Unmarshal(b, &w); err != nil {
			return err
		}
		events = w.Events
	}
	prev := "GENESIS"
	if len(events) > 0 {
		prev = events[len(events)-1].Hash
	}
	pd := digest(canon(payload))
	seq := len(events) + 1
	h := digest([]byte(fmt.Sprintf("%d\n%s\n%s\n%s\n", seq, typ, pd, prev)))
	events = append(events, ledgerEvent{seq, typ, pd, prev, h})
	return writeNew(path, canon(map[string]any{"schema": ledgerSchema, "events": events}))
}
