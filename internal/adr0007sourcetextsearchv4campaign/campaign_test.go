package adr0007sourcetextsearchv4campaign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteNewRejectsOverwrite(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x")
	if err := writeNew(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := writeNew(p, []byte("two")); err == nil {
		t.Fatal("expected overwrite rejection")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "one" {
		t.Fatalf("changed bytes: %q", b)
	}
}

func TestAppendEventChains(t *testing.T) {
	d := t.TempDir()
	if err := appendEvent(d, "A", "", "", "", "", 0); err != nil {
		t.Fatal(err)
	}
	if err := appendEvent(d, "B", "", "", "", "", 0); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d, "EVENTS.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if got := len(lines); got != 2 {
		t.Fatalf("events=%d", got)
	}
	if !strings.Contains(string(b), "prev_event_sha256") {
		t.Fatal("missing chain")
	}
}

func TestCaseIDValidationRejectsMissing(t *testing.T) {
	d := t.TempDir()
	_ = os.MkdirAll(filepath.Join(d, "docs/pilot/adr0007/source-text-search-v4/cases/case-01-only"), 0755)
	_, err := caseIDs(Options{Root: d})
	if err == nil || !strings.Contains(err.Error(), "case count") {
		t.Fatalf("expected count error, got %v", err)
	}
}

func TestConstantsPreserveCeilings(t *testing.T) {
	if ExpectedHEAD != "34ed9915313b652be1fd816b51a6e5ec91799728" {
		t.Fatal("head drift")
	}
	if DesignRoot == "" || DesignManifest == "" || DesignCensus == "" || DesignEnvelope == "" {
		t.Fatal("missing design identity")
	}
}
