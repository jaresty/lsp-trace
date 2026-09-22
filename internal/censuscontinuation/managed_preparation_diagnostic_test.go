package censuscontinuation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedPreparationDiagnosticRecorderRejectsExtremeMaxBytesBeforeMutation(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "managed-preparation.ndjson")
	if _, err := NewManagedPreparationDiagnosticFileRecorder(path, int64(^uint64(0)>>1), 1); err == nil {
		t.Fatal("ASSERT_MANAGED_DIAGNOSTIC_MAX_BYTES_OVERFLOW_REJECTED")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_MAX_BYTES_NO_MUTATION: %v", err)
	}
}

func TestManagedPreparationDiagnosticFileRecorderRoundTripPrivacyAndCaps(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "managed-preparation.ndjson")
	recorder, err := NewManagedPreparationDiagnosticFileRecorder(path, 512, 1)
	if err != nil {
		t.Fatal(err)
	}
	record := ManagedPreparationDiagnostic{Failure: ManagedDocumentPreparationSupplyMissing, Attempted: 2, Succeeded: 1, Planned: 4, FailingOrdinal: 1}
	if err := recorder.RecordManagedPreparationDiagnostic(record); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_FILE_MODE: info=%v err=%v", info, err)
	}
	got, err := ReadLastManagedPreparationDiagnostic(path)
	if err != nil || got != record {
		t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_STRICT_ROUNDTRIP: got=%+v err=%v raw=%s", got, err, raw)
	}
	for _, forbidden := range []string{"file:", "/private/", "source bytes", "raw error", "PRIVATE_RAW_FAILURE"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_PRIVACY: %q in %s", forbidden, raw)
		}
	}
	if err := recorder.RecordManagedPreparationDiagnostic(record); err == nil || err.Error() != "managed preparation diagnostic record limit" {
		t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_RECORD_CAP: %v", err)
	}
}

func TestManagedPreparationDiagnosticStrictAcceptsSeparatedFailures(t *testing.T) {
	for _, failure := range []ManagedDocumentPreparationFailure{ManagedDocumentPreparationURIUnavailable, ManagedDocumentPreparationOutsideWorkspace, ManagedDocumentPreparationSourceUnavailable, ManagedDocumentPreparationLanguageIDUnavailable} {
		line := []byte(`{"failure":"` + string(failure) + `","attempted":10,"succeeded":9,"planned":14,"failing_ordinal":9}`)
		got, err := ParseManagedPreparationDiagnosticLine(line)
		if err != nil || got.Failure != failure {
			t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_STRICT_ACCEPTED_%s: got=%+v err=%v", failure, got, err)
		}
	}
}

func TestManagedPreparationDiagnosticStrictUnknownFailsClosed(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"failure":"PRIVATE_RAW_FAILURE","attempted":1,"succeeded":0,"planned":1,"failing_ordinal":0}`),
		[]byte(`{"failure":"SUPPLY_MISSING","attempted":1,"succeeded":0,"planned":1,"failing_ordinal":0,"extra":"forbidden"}`),
		[]byte(`{"failure":"SUPPLY_MISSING","attempted":1,"succeeded":1,"planned":1,"failing_ordinal":0}`),
		[]byte(`not-json`),
	}
	for _, raw := range cases {
		if _, err := ParseManagedPreparationDiagnosticLine(raw); err == nil {
			t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_STRICT_REJECTED: %s", raw)
		}
	}
}

func TestManagedPreparationDiagnosticRecorderRejectsCustodyAndCorruptionWithoutMutation(t *testing.T) {
	valid := ManagedPreparationDiagnostic{Failure: ManagedDocumentPreparationSupplyMissing, Attempted: 1, Succeeded: 0, Planned: 1, FailingOrdinal: 0}
	validLine := []byte(`{"failure":"SUPPLY_MISSING","attempted":1,"succeeded":0,"planned":1,"failing_ordinal":0}` + "\n")
	for _, tc := range []struct {
		name    string
		prepare func(t *testing.T, parent, path string)
		record  ManagedPreparationDiagnostic
	}{
		{name: "parent-mode-drift", prepare: func(t *testing.T, parent, _ string) {
			if err := os.Chmod(parent, 0o755); err != nil {
				t.Fatal(err)
			}
		}, record: valid},
		{name: "parent-deleted", prepare: func(t *testing.T, parent, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(parent); err != nil {
				t.Fatal(err)
			}
		}, record: valid},
		{name: "parent-replaced-by-file", prepare: func(t *testing.T, parent, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(parent); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(parent, []byte("replacement"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, record: valid},
		{name: "parent-replaced-by-symlink", prepare: func(t *testing.T, parent, path string) {
			target := t.TempDir()
			if err := os.Chmod(target, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(parent); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, parent); err != nil {
				t.Fatal(err)
			}
		}, record: valid},
		{name: "file-replaced-by-symlink", prepare: func(t *testing.T, _, path string) {
			target := filepath.Join(t.TempDir(), "target")
			if err := os.WriteFile(target, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}, record: valid},
		{name: "partial-ledger", prepare: func(t *testing.T, _, path string) {
			if err := os.WriteFile(path, validLine[:len(validLine)-1], 0o600); err != nil {
				t.Fatal(err)
			}
		}, record: valid},
		{name: "corrupt-ledger", prepare: func(t *testing.T, _, path string) {
			if err := os.WriteFile(path, []byte("not-json\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, record: valid},
		{name: "unknown-field-ledger", prepare: func(t *testing.T, _, path string) {
			if err := os.WriteFile(path, []byte(`{"failure":"SUPPLY_MISSING","attempted":1,"succeeded":0,"planned":1,"failing_ordinal":0,"extra":true}`+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, record: valid},
		{name: "invalid-incoming", prepare: func(t *testing.T, _, _ string) {}, record: ManagedPreparationDiagnostic{Failure: "PRIVATE_RAW_FAILURE", Attempted: 1, Planned: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), "private")
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(parent, "diagnostic.ndjson")
			recorder, err := NewManagedPreparationDiagnosticFileRecorder(path, 512, 10)
			if err != nil {
				t.Fatal(err)
			}
			tc.prepare(t, parent, path)
			before, _ := os.ReadFile(path)
			if err := recorder.RecordManagedPreparationDiagnostic(tc.record); err == nil {
				t.Fatal("ASSERT_MANAGED_DIAGNOSTIC_REJECTION_REQUIRED")
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_REJECTION_NO_MUTATION: before=%q after=%q", before, after)
			}
		})
	}
}

func TestManagedPreparationDiagnosticRecorderRejectsOversizedExistingFileWithoutMutation(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "diagnostic.ndjson")
	recorder, err := NewManagedPreparationDiagnosticFileRecorder(path, 64, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, 65); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	record := ManagedPreparationDiagnostic{Failure: ManagedDocumentPreparationSupplyMissing, Attempted: 1, Planned: 1, FailingOrdinal: 0}
	if err := recorder.RecordManagedPreparationDiagnostic(record); err == nil {
		t.Fatal("ASSERT_MANAGED_DIAGNOSTIC_OVERSIZED_REJECTED")
	}
	after, err := os.Stat(path)
	if err != nil || after.Size() != before.Size() {
		t.Fatalf("ASSERT_MANAGED_DIAGNOSTIC_OVERSIZED_NO_MUTATION: before=%v after=%v err=%v", before.Size(), after.Size(), err)
	}
}

func TestManagedPreparationDiagnosticReadRejectsIncompleteAndUnsafeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostic.ndjson")
	line := []byte(`{"failure":"SUPPLY_MISSING","attempted":1,"succeeded":0,"planned":1,"failing_ordinal":0}`)
	if err := os.WriteFile(path, line, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLastManagedPreparationDiagnostic(path); err == nil {
		t.Fatal("ASSERT_MANAGED_DIAGNOSTIC_INCOMPLETE_NEWLINE_REJECTED")
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLastManagedPreparationDiagnostic(path); err == nil {
		t.Fatal("ASSERT_MANAGED_DIAGNOSTIC_UNSAFE_MODE_REJECTED")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLastManagedPreparationDiagnostic(path); err == nil {
		t.Fatal("ASSERT_MANAGED_DIAGNOSTIC_READ_PARENT_MODE_REJECTED")
	}
}
