package manageddiagnostic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	assertSinkPrivate = "ASSERT_FR23_STARTUP_SINK_PRIVATE_ATOMIC_BOUNDED"
	assertSinkPrivacy = "ASSERT_FR23_STARTUP_SINK_EXACT_PRIVACY_PROJECTION"
	assertSinkVerify  = "ASSERT_FR23_STARTUP_SINK_OFFLINE_VALIDATE_VERIFY"
	assertSinkUnsafe  = "ASSERT_FR23_STARTUP_SINK_UNSAFE_COLLISION_PERMISSION"
)

func sinkAttempt(outcome StartupOutcome) StartupAttemptQuery {
	r := StartupAttemptRecord{SchemaVersion: StartupAttemptSchemaVersion, AttemptID: StartupAttemptID(strings.Repeat("a", 64)), Sequence: 1, Outcome: outcome, Reason: Fact[string]{Status: Observed, Value: "process-start-failed"}, Timing: Timing{Scope: "startup-attempt"}, Stderr: Stderr{Status: Withheld}, ProcessExit: ProcessExit{Status: Unavailable}}
	if outcome == StartupAdmitted {
		r.Reason.Value = "startup-admitted"
		r.Admission = &StartupAdmission{SessionID: "opaque-session", Generation: 7}
	}
	return StartupAttemptQuery{Status: AttemptAvailable, Record: &r}
}

func TestStartupSinkPrivateAtomicBounded(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	receipt, err := (StartupDiagnosticSink{Root: root, Selector: "attempt.json"}).Finalize(sinkAttempt(StartupFailed), QueryResult{Status: QueryUnavailable})
	if err != nil {
		t.Fatalf("%s: %v", assertSinkPrivate, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "attempt.json"))
	if err != nil {
		t.Fatalf("%s: %v", assertSinkPrivate, err)
	}
	info, _ := os.Stat(filepath.Join(root, "attempt.json"))
	if len(raw) > StartupDiagnosticsMaxBytes || receipt.Bytes != len(raw) || info.Mode().Perm() != 0600 {
		t.Fatalf("%s: bytes=%d receipt=%+v mode=%o", assertSinkPrivate, len(raw), receipt, info.Mode().Perm())
	}
	if _, err := (StartupDiagnosticSink{Root: root, Selector: "attempt.json"}).Finalize(sinkAttempt(StartupFailed), QueryResult{}); err == nil {
		t.Fatalf("%s: collision accepted", assertSinkUnsafe)
	}
}

func TestStartupSinkExactPrivacyAndNoFabricatedAdmission(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	_, err := (StartupDiagnosticSink{Root: root, Selector: "attempt.json"}).Finalize(sinkAttempt(StartupFailed), QueryResult{Status: QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "attempt.json"))
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"raw_stderr", "raw_error", "rpc", "message", "params", "source", "uri", "path", "cwd", "env", "command", "args", "opaque", "generation\":1"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("%s/%s: %s", assertSinkPrivacy, forbidden, raw)
		}
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil || doc["schema_version"] != StartupDiagnosticsSchemaVersion {
		t.Fatalf("%s: %s", assertSinkPrivacy, raw)
	}
}

func TestStartupSinkOfflineValidateVerify(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	receipt, err := (StartupDiagnosticSink{Root: root, Selector: "nested/attempt.json"}).Finalize(sinkAttempt(StartupAdmitted), QueryResult{Status: QueryEvicted, EvictedRecords: 3})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "nested/attempt.json"))
	if err := ValidateStartupDiagnostics(raw); err != nil {
		t.Fatalf("%s validate: %v", assertSinkVerify, err)
	}
	if err := VerifyStartupDiagnostics(raw, receipt.Digest, receipt.Bytes); err != nil {
		t.Fatalf("%s verify: %v", assertSinkVerify, err)
	}
	if err := VerifyStartupDiagnostics(append(raw, ' '), receipt.Digest, receipt.Bytes); err == nil {
		t.Fatalf("%s: tamper admitted", assertSinkVerify)
	}
}

func TestStartupSinkRejectsUnsafeSelectorsAndPermissions(t *testing.T) {
	for _, selector := range []string{"", ".", "../x", "/tmp/x"} {
		if _, err := (StartupDiagnosticSink{Root: t.TempDir(), Selector: selector}).Finalize(sinkAttempt(StartupFailed), QueryResult{}); err == nil {
			t.Fatalf("%s: selector %q", assertSinkUnsafe, selector)
		}
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(root, 0700)
	if _, err := (StartupDiagnosticSink{Root: root, Selector: "x"}).Finalize(sinkAttempt(StartupFailed), QueryResult{}); err == nil {
		t.Fatalf("%s: unwritable root accepted", assertSinkUnsafe)
	}
}
