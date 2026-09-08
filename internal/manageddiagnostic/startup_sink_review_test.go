package manageddiagnostic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validStartupDocumentForReview(t *testing.T) []byte {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	r := validRecord()
	r.Sequence = 2
	if _, err := (StartupDiagnosticSink{Root: root, Selector: "attempt.json"}).Finalize(sinkAttempt(StartupFailed), QueryResult{Status: QueryAvailable, Records: []Record{r}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "attempt.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mutateStartupDocumentForReview(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	mutate(doc)
	accounting := doc["accounting"].(map[string]any)
	previous := -1
	for range 8 {
		out, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, '\n')
		if previous == len(out) {
			return out
		}
		previous = len(out)
		accounting["retained_bytes"] = previous
	}
	t.Fatal("retained byte accounting did not converge")
	return nil
}

func TestReviewStartupSinkRejectsKnownFieldSemanticSmuggling(t *testing.T) {
	base := validStartupDocumentForReview(t)
	cases := map[string]func(map[string]any){
		"attempt-id-not-lower-hex":     func(d map[string]any) { d["attempt"].(map[string]any)["attempt_id"] = strings.Repeat("G", 64) },
		"unavailable-attempt-details":  func(d map[string]any) { d["attempt"].(map[string]any)["status"] = "unavailable" },
		"unavailable-records-retained": func(d map[string]any) { d["records_status"] = "unavailable" },
		"unknown-record-phase":         func(d map[string]any) { d["records"].([]any)[0].(map[string]any)["phase"] = "secret-phase" },
		"unknown-record-terminal":      func(d map[string]any) { d["records"].([]any)[0].(map[string]any)["terminal"] = "secret-terminal" },
		"negative-record-io": func(d map[string]any) {
			d["records"].([]any)[0].(map[string]any)["read"].(map[string]any)["bytes"] = -1
		},
		"missing-effective-limit": func(d map[string]any) {
			limits := d["records"].([]any)[0].(map[string]any)["limits"].(map[string]any)
			limits["RequestedDeadlineNS"] = map[string]any{"status": "observed", "value": 1}
			limits["EffectiveDeadlineNS"] = map[string]any{"status": "unavailable"}
		},
		"hidden-stderr-detail": func(d map[string]any) {
			stderr := d["records"].([]any)[0].(map[string]any)["stderr"].(map[string]any)
			stderr["Status"] = "withheld"
			stderr["ByteCap"] = 12
		},
		"process-exit-terminal-mismatch": func(d map[string]any) {
			exit := d["records"].([]any)[0].(map[string]any)["process_exit"].(map[string]any)
			exit["Status"] = "observed"
			exit["ObservedBeforeCleanup"] = map[string]any{"status": "observed", "value": true}
		},
		"non-strict-sequence": func(d map[string]any) {
			r := d["records"].([]any)[0].(map[string]any)
			d["records"] = []any{r, r}
			d["accounting"].(map[string]any)["retained_record_count"] = 2
		},
		"inconsistent-eviction": func(d map[string]any) {
			d["records_status"] = "omitted"
			d["records"] = []any{}
			d["accounting"].(map[string]any)["retained_record_count"] = 0
			d["accounting"].(map[string]any)["evicted_record_count"] = 1
		},
		"inconsistent-truncation": func(d map[string]any) { d["accounting"].(map[string]any)["truncated"] = true },
		"oversized-known-string": func(d map[string]any) {
			d["attempt"].(map[string]any)["reason"] = strings.Repeat("s", maxRetainedStringBytes+1)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			raw := mutateStartupDocumentForReview(t, base, mutate)
			if err := ValidateStartupDiagnostics(raw); err == nil {
				t.Fatalf("ASSERT_FR23_STARTUP_DOCUMENT_SEMANTICS_%s: accepted", name)
			}
			sum := sha256.Sum256(raw)
			if err := VerifyStartupDiagnostics(raw, "sha256:"+hex.EncodeToString(sum[:]), len(raw)); err == nil {
				t.Fatalf("ASSERT_FR23_STARTUP_DOCUMENT_VERIFY_SEMANTICS_%s: accepted", name)
			}
		})
	}
}

func TestReviewStartupSinkRejectsRootSymlink(t *testing.T) {
	realRoot := t.TempDir()
	if err := os.Chmod(realRoot, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Fatal(err)
	}
	if _, err := (StartupDiagnosticSink{Root: link, Selector: "attempt.json"}).Finalize(sinkAttempt(StartupFailed), QueryResult{Status: QueryUnavailable}); err == nil {
		t.Fatal("ASSERT_FR23_STARTUP_ROOT_SYMLINK: accepted")
	}
}

func TestReviewStartupSinkRejectsSwappedRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	moved := filepath.Join(parent, "moved")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	sink := StartupDiagnosticSink{Root: root, Selector: "attempt.json", afterRootLstat: func() {
		if err := os.Rename(root, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
	}}
	if _, err := sink.Finalize(sinkAttempt(StartupFailed), QueryResult{Status: QueryUnavailable}); err == nil {
		t.Fatal("ASSERT_FR23_STARTUP_ROOT_SWAP: accepted")
	}
}

func TestReviewStartupSinkRejectsNestedSymlinkEscapeAndCollision(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "nested")); err != nil {
		t.Fatal(err)
	}
	if _, err := (StartupDiagnosticSink{Root: root, Selector: "nested/attempt.json"}).Finalize(sinkAttempt(StartupFailed), QueryResult{Status: QueryUnavailable}); err == nil {
		t.Fatal("ASSERT_FR23_STARTUP_NESTED_SYMLINK_ESCAPE: accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "attempt.json")); !os.IsNotExist(err) {
		t.Fatalf("escape artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "collision.json"), []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (StartupDiagnosticSink{Root: root, Selector: "collision.json"}).Finalize(sinkAttempt(StartupFailed), QueryResult{Status: QueryUnavailable}); err == nil {
		t.Fatal("ASSERT_FR23_STARTUP_DESTINATION_COLLISION: replaced")
	}
	got, err := os.ReadFile(filepath.Join(root, "collision.json"))
	if err != nil || string(got) != "sentinel" {
		t.Fatalf("collision changed: %q %v", got, err)
	}
}
