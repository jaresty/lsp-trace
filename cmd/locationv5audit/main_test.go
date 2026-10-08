package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLedgerHashChainDetectsTamper(t *testing.T) {
	d := t.TempDir()
	if err := writeLedger(d, "one", map[string]any{"n": 1}); err != nil {
		t.Fatal(err)
	}
	if err := writeLedger(d, "two", map[string]any{"n": 2}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d, "EVENT_LEDGER.json"))
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Events []ledgerEvent `json:"events"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	if len(w.Events) != 2 || w.Events[1].Prev != w.Events[0].Hash {
		t.Fatalf("bad chain %#v", w.Events)
	}
	w.Events[0].Hash = "sha256:tamper"
	if w.Events[1].Prev == w.Events[0].Hash {
		t.Fatalf("tamper not visible")
	}
}

func TestAuditStrictRejectsBadProducerJSON(t *testing.T) {
	var p producer
	for _, raw := range [][]byte{[]byte(`{"schema":"x","schema":"y"}`), []byte(`{"schema":"x"}0`), []byte(`{"schema":"x","caseId":"c","assignmentId":"a","result":{},"processCustody":{},"extra":1}`)} {
		f := filepath.Join(t.TempDir(), "x.json")
		if err := os.WriteFile(f, raw, 0644); err != nil {
			t.Fatal(err)
		}
		if err := strict(f, &p); err == nil {
			t.Fatalf("expected strict failure for %s", raw)
		}
	}
}
