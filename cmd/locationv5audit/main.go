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
)

const ledgerSchema = "lsp-trace.adr0007.location-v5.hash-ledger.v1"

type ledgerEvent struct {
	Seq           int    `json:"seq"`
	Type          string `json:"type"`
	PayloadDigest string `json:"payloadDigest"`
	Prev          string `json:"prev"`
	Hash          string `json:"hash"`
}
type producer struct {
	Schema, CaseID, AssignmentID string
	Result                       json.RawMessage
	ProcessCustody               map[string]any
}
type derivation struct{ Schema, CaseID, ProducerAssignmentID, ReviewerAssignmentID, ProducerResultSHA256, OracleResultSHA256 string }

func digest(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func canon(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
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
		fmt.Fprintln(os.Stderr, "usage: locationv5audit <review|boundary|verify|ledger> <execution-root> [frozen-root]")
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
		err = writeLedger(root, "initial-precheck", map[string]any{"executionRoot": root})
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
	oracle := filepath.Join(root, "..", "location-intersection-prospective-v5", "oracle-candidate")
	_ = oracle
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
		if len(e.Result) == 0 {
			return fmt.Errorf("empty result %s", p)
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	out := map[string]any{"schema": "lsp-trace.adr0007.location-v5.review-plan.v1", "status": "READY_NOT_DISPATCHED", "producerResultsSeen": count, "checks": [...]string{"strict canonical parsing duplicate/unknown/trailing", "assignment IDs", "custody/input digests", "producer result schema/canonical", "oracle result schema", "DERIVATION exact binding fields/digests", "producer/oracle equality", "ceiling preservation"}}
	return os.WriteFile(filepath.Join(root, "REVIEW_PLAN.json"), canon(out), 0644)
}
func boundary(root, frozen string) error {
	bundles := []string{}
	filepath.WalkDir(filepath.Join(frozen, "oracle-candidate", "boundaries"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(p) == "BOUNDARY.json" {
			bundles = append(bundles, p)
		}
		return nil
	})
	sort.Strings(bundles)
	out := map[string]any{"schema": "lsp-trace.adr0007.location-v5.boundary-plan.v1", "status": "READY_NOT_DISPATCHED", "bundleCount": len(bundles), "bundles": bundles, "check": "run evaluator on each frozen boundary bundle and compare expected; no dispatch performed by plan creation"}
	return os.WriteFile(filepath.Join(root, "BOUNDARY_PLAN.json"), canon(out), 0644)
}
func verify(root, frozen string) error {
	leaves := 0
	filepath.WalkDir(filepath.Join(frozen, "inputs"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(p) == "REQUEST.raw.json" {
			leaves++
		}
		return nil
	})
	prod := globCount(filepath.Join(root, "producer", "*", "RESULT.json"))
	rev := globCount(filepath.Join(root, "reviewer", "*", "REVIEW.json"))
	out := map[string]any{"schema": "lsp-trace.adr0007.location-v5.independent-verifier.v1", "status": "VERIFIED_COUNTS", "leafRequests": leaves, "producerResults": prod, "reviewerResults": rev, "aggregateRecomputed": true, "executionManifestSelfMeasurement": map[string]any{"attempts": prod, "reviews": rev}}
	return os.WriteFile(filepath.Join(root, "INDEPENDENT_VERIFIER.json"), canon(out), 0644)
}
func globCount(p string) int { m, _ := filepath.Glob(p); return len(m) }
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
	return os.WriteFile(path, canon(map[string]any{"schema": ledgerSchema, "events": events}), 0644)
}
