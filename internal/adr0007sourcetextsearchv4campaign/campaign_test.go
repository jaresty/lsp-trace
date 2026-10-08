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

func TestAppendEventChainsFromPredecessorTerminal(t *testing.T) {
	d := t.TempDir()
	if err := appendEvent(d, "CORRECTION_AUTHORIZED", "", "", "", "", 0); err != nil {
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
	if !strings.Contains(string(b), "prev_event_sha256") || !strings.Contains(string(lines[0]), PredecessorTerminalEventSHA) {
		t.Fatal("missing predecessor-seeded chain")
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
	if ExpectedHEAD != "91f756c72317f09ba8ffac7c0b8933d46494bf0c" {
		t.Fatal("head drift")
	}
	if CampaignID != "source-text-search-v4-correction-generation-91f756c7" || BlockedCampaignID != "source-text-search-v4-qualification-34ed9915" || ZeroEffectSuccessorCampaignID != "source-text-search-v4-zero-effect-successor-02ca9324" {
		t.Fatal("campaign identity drift")
	}
	if AuthorizationVerdict != "SOURCE_TEXT_SEARCH_STAKEHOLDER_REPAIR_GO" || PredecessorTerminalEventSHA == "" || PredecessorLedgerSHA == "" {
		t.Fatal("missing correction authorization")
	}
	if DesignRoot == "" || DesignManifest == "" || DesignCensus == "" || DesignEnvelope == "" {
		t.Fatal("missing design identity")
	}
}
