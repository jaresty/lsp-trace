package locationexecutionv5

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLedgerRawMessageCanonicalRoundTrip(t *testing.T) {
	root := t.TempDir()
	payload := map[string]any{"z": []any{json.RawMessage(`{"b":2,"a":1}`)}, "a": "first"}
	if err := AppendLedger(root, "payload", payload); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLedger(root); err != nil {
		t.Fatal(err)
	}
	var l Ledger
	if err := readStrict(filepath.Join(root, "EVENT_LEDGER.json"), &l); err != nil {
		t.Fatal(err)
	}
	if got := string(l.Entries[0].Payload); !strings.Contains(got, `"a":"first"`) || !strings.Contains(got, `"z":[{"a":1,"b":2}]`) {
		t.Fatalf("payload not canonicalized: %s", got)
	}
	if err := os.WriteFile(filepath.Join(root, "EVENT_LEDGER.copy.json"), canon(l), 0644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLedger(root); err != nil {
		t.Fatalf("persisted roundtrip failed: %v", err)
	}
}

func TestLedgerRejectsPayload2MutationRegression(t *testing.T) {
	root := t.TempDir()
	if err := AppendLedger(root, "one", map[string]any{"case": "01"}); err != nil {
		t.Fatal(err)
	}
	if err := AppendLedger(root, "two", map[string]any{"case": "02", "nested": map[string]any{"b": 2, "a": 1}}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLedger(root); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "EVENT_LEDGER.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(b), `"case":"02"`, `"case":"02x"`, 1)
	if err := os.WriteFile(p, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLedger(root); err == nil || !strings.Contains(err.Error(), "ledger payload 2") {
		t.Fatalf("expected payload 2 rejection, got %v", err)
	}
}

func TestStrictJSONRejectsUnknownDuplicateTrailing(t *testing.T) {
	root := t.TempDir()
	unknown := filepath.Join(root, "unknown.json")
	if err := os.WriteFile(unknown, []byte(`{"schema":"x","extra":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	var c Condition
	if err := readStrict(unknown, &c); err == nil {
		t.Fatal("expected unknown field rejection")
	}
	if _, err := canonicalRaw([]byte(`{"a":1}{"b":2}`)); err == nil {
		t.Fatal("expected trailing json rejection")
	}
	if _, err := canonicalRaw([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("expected duplicate key rejection")
	}
}
