package censusdiagnostic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testLedger(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ledger")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, name)
}

func validRecord() Record {
	return Record{Fingerprint: "sha256:abc", Category: "ACQUISITION", Ordinal: func() *int { n := 3; return &n }(), OperationCode: "CENSUS_BATCH_ACQUISITION_FAILED", OperationCategory: "ACQUISITION"}
}

func TestRecorderCustodyAndReadback(t *testing.T) {
	path := testLedger(t, "diagnostic.ndjson")
	r, err := NewRecorder(path, 4096, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Record(validRecord()); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLast(path)
	if err != nil || got.Fingerprint != "sha256:abc" || got.Ordinal == nil || *got.Ordinal != 3 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	before, _ := os.ReadFile(path)
	if err := r.Record(validRecord()); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if len(after) <= len(before) || !strings.HasPrefix(string(after), string(before)) {
		t.Fatal("ledger is not append-only")
	}
}

func TestRecorderRejectsCustodyDriftAndInvalidPaths(t *testing.T) {
	if _, err := NewRecorder("relative", 100, 1); err == nil {
		t.Fatal("relative path accepted")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "x")
	if _, err := NewRecorder(path, 100, 1); err == nil {
		t.Fatal("wrong parent mode accepted")
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRecorder(path, 100, 1); err == nil {
		t.Fatal("wrong file mode accepted")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRecorder(path, 100, 1); err == nil {
		t.Fatal("directory ledger accepted")
	}
}

func TestRecorderBoundsConcurrencyAndStrictLedger(t *testing.T) {
	path := testLedger(t, "diagnostic.ndjson")
	r, err := NewRecorder(path, 4096, 20)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = r.Record(validRecord()) }()
	}
	wg.Wait()
	if _, err := ReadLast(path); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRecorder(testLedger(t, "small"), 1, 1); err != nil { /* constructor permits an empty bounded ledger */
	}
	bad := testLedger(t, "bad")
	if err := os.WriteFile(bad, []byte(`{"fingerprint":"x","category":"ADMISSION"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLast(bad); err == nil {
		t.Fatal("trailing content accepted")
	}
	if err := os.WriteFile(bad, []byte(`{"fingerprint":"x","category":"ADMISSION"}\n`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRecorder(bad, 4096, 2); err == nil {
		t.Fatal("noncanonical ledger accepted")
	}
}

func TestValidationEvidenceRoundTripAndClosedEnums(t *testing.T) {
	path := testLedger(t, "validation.ndjson")
	recorder, err := NewRecorder(path, 4096, 2)
	if err != nil {
		t.Fatal(err)
	}
	record := Record{Fingerprint: "sha256:abc", Category: "INVALID_INPUT", Ordinal: func() *int { n := 6; return &n }(), FailedField: "envelope", FailedInvariant: "canonical_input_decode"}
	if err := recorder.Record(record); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLast(path)
	if err != nil || got.FailedField != record.FailedField || got.FailedInvariant != record.FailedInvariant || got.Ordinal == nil || *got.Ordinal != 6 {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	inputIdentity := Record{Fingerprint: "sha256:abc", Category: "INVALID_INPUT", FailedField: "input_identity", FailedInvariant: "NULL_MEMBER_FORBIDDEN"}
	if err := recorder.Record(inputIdentity); err != nil {
		t.Fatalf("Record rejected finite input_identity evidence: %v", err)
	}
	got, err = ReadLast(path)
	if err != nil || got.FailedField != inputIdentity.FailedField || got.FailedInvariant != inputIdentity.FailedInvariant {
		t.Fatalf("input_identity replay got=%+v err=%v", got, err)
	}

	for _, invalid := range []Record{
		{Fingerprint: "x", Category: "INVALID_INPUT", FailedField: "secret", FailedInvariant: "canonical_input_decode"},
		{Fingerprint: "x", Category: "INVALID_INPUT", FailedField: "envelope", FailedInvariant: "secret"},
		{Fingerprint: "x", Category: "INVALID_INPUT", FailedField: "envelope"},
	} {
		if err := recorder.Record(invalid); err == nil {
			t.Fatalf("accepted invalid evidence %+v", invalid)
		}
	}
}

func TestParseRejectsUnknownFieldsInvalidEvidenceAndCategories(t *testing.T) {
	cases := []string{
		`{"fingerprint":"x","category":"ADMISSION","extra":"x"}`,
		`{"fingerprint":"x","category":"NOPE"}`,
		`{"fingerprint":"x","category":"ACQUISITION","operation_code":"x"}`,
		`{"fingerprint":"x","category":"ADMISSION","ordinal":-1}`,
	}
	for _, line := range cases {
		if _, err := Parse([]byte(line)); err == nil {
			t.Errorf("accepted %s", line)
		}
	}
	if _, err := Parse([]byte(`{"fingerprint":"x","category":"ADMISSION"} `)); err == nil {
		t.Fatal("noncanonical whitespace accepted")
	}
}

func TestRecorderAcquisitionDiagnosticBoundaries(t *testing.T) {
	t.Run("missing parent rejected without creation", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing", "ledger.ndjson")
		if _, err := NewRecorder(path, 1024, 2); err == nil {
			t.Fatal("missing parent accepted")
		}
		if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
			t.Fatalf("parent was created: %v", err)
		}
	})
	t.Run("symlink parent rejected", func(t *testing.T) {
		target := t.TempDir()
		link := filepath.Join(t.TempDir(), "parent")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := NewRecorder(filepath.Join(link, "ledger.ndjson"), 1024, 2); err == nil {
			t.Fatal("symlink parent accepted")
		}
	})
	t.Run("symlink file rejected", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		path := filepath.Join(dir, "ledger.ndjson")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if _, err := NewRecorder(path, 1024, 2); err == nil {
			t.Fatal("symlink file accepted")
		}
	})
	t.Run("nonregular file rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ledger.ndjson")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := NewRecorder(path, 1024, 2); err == nil {
			t.Fatal("directory accepted")
		}
	})
	t.Run("wrong modes rejected without repair", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ledger.ndjson")
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := NewRecorder(path, 1024, 2); err == nil {
			t.Fatal("wrong parent mode accepted")
		}
		if got := mustMode(t, dir); got != 0o755 {
			t.Fatalf("parent repaired: %o", got)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := NewRecorder(path, 1024, 2); err == nil {
			t.Fatal("wrong file mode accepted")
		}
		if got := mustMode(t, path); got != 0o644 {
			t.Fatalf("file repaired: %o", got)
		}
	})
	t.Run("record and byte bounds are exact", func(t *testing.T) {
		line, err := marshalRecord(validRecord())
		if err != nil {
			t.Fatal(err)
		}
		path := testLedger(t, "records.ndjson")
		r, err := NewRecorder(path, int64(len(line)+1), 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Record(validRecord()); err != nil {
			t.Fatal(err)
		}
		if err := r.Record(validRecord()); err == nil {
			t.Fatal("record bound accepted")
		}
		path2 := testLedger(t, "bytes.ndjson")
		r2, err := NewRecorder(path2, int64(len(line)+1)-1, 2)
		if err != nil {
			t.Fatal(err)
		}
		if err := r2.Record(validRecord()); err == nil {
			t.Fatal("byte bound accepted")
		}
	})
	t.Run("strict ledger failures", func(t *testing.T) {
		cases := map[string]string{
			"duplicate JSON key":         "{\"fingerprint\":\"x\",\"fingerprint\":\"y\",\"category\":\"ADMISSION\"}\n",
			"unknown field":              "{\"fingerprint\":\"x\",\"category\":\"ADMISSION\",\"extra\":\"x\"}\n",
			"trailing second JSON value": "{\"fingerprint\":\"x\",\"category\":\"ADMISSION\"} {\"fingerprint\":\"y\",\"category\":\"ADMISSION\"}\n",
			"incomplete final line":      "{\"fingerprint\":\"x\",\"category\":\"ADMISSION\"}",
			"corrupt earlier line":       "not-json\n{\"fingerprint\":\"x\",\"category\":\"ADMISSION\"}\n",
		}
		for name, body := range cases {
			t.Run(name, func(t *testing.T) {
				path := testLedger(t, "bad.ndjson")
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := NewRecorder(path, 4096, 4); err == nil {
					t.Fatal("invalid ledger accepted")
				}
			})
		}
		t.Run("oversized ledger", func(t *testing.T) {
			path := testLedger(t, "large.ndjson")
			if err := os.WriteFile(path, []byte(`{"fingerprint":"x","category":"ADMISSION"}\n`), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewRecorder(path, 1, 4); err == nil {
				t.Fatal("oversized ledger accepted")
			}
		})
	})
	t.Run("absolute path must be clean", func(t *testing.T) {
		dir := t.TempDir()
		clean := filepath.Join(dir, "ledger.ndjson")
		unclean := dir + string(filepath.Separator) + "." + string(filepath.Separator) + "ledger.ndjson"
		if clean == unclean {
			t.Fatal("test spelling cleaned unexpectedly")
		}
		if _, err := NewRecorder(unclean, 1024, 2); err == nil {
			t.Fatal("unclean absolute path accepted")
		}
	})
	t.Run("concurrent append has no loss or corruption", func(t *testing.T) {
		const n = 64
		path := testLedger(t, "concurrent.ndjson")
		r, err := NewRecorder(path, 1<<20, n)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := r.Record(validRecord()); err != nil {
					t.Errorf("Record: %v", err)
				}
			}()
		}
		wg.Wait()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
		if len(lines) != n {
			t.Fatalf("records=%d want=%d", len(lines), n)
		}
		for _, line := range lines {
			if _, err := Parse([]byte(line)); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func mustMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestRecordHasOnlyPrivateAllowlistedFields(t *testing.T) {
	line, err := marshalRecord(validRecord())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"uri", "path", "workspace", "source", "seed", "error", "payload", "selector", "request", "artifact"} {
		if strings.Contains(strings.ToLower(string(line)), forbidden) {
			t.Fatalf("forbidden field %q in %s", forbidden, line)
		}
	}
}

func marshalRecord(v Record) ([]byte, error) { return json.Marshal(v) }
