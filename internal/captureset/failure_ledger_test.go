package captureset

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func TestFailureLedgerOwnerOnlyBoundedAndPathFree(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	ledger, err := OpenFailureLedger(FailureLedgerConfig{Path: filepath.Join(dir, "publication-failures.ndjson"), MaxBytes: 4096, MaxRecords: 1})
	if err != nil {
		t.Fatal(err)
	}
	failure := &PublicationFailure{Stage: "TEMP_WRITE", Reason: "FAILED", CandidateSHA256: "sha256:" + strings.Repeat("a", 64), CandidateByteLength: 17, CodecCategory: "CAPTURE_SET_V1", CodecLimit: 64 << 20, RootSource: "HOST_PUBLICATION_ROOT", TempBytesCommitted: true, FinalBytesCommitted: false, Err: errors.New("/private/secret raw error")}
	if err := ledger.Record(failure); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(dir, "publication-failures.ndjson"))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("ASSERT_FAILURE_LEDGER_FILE_0600: info=%v err=%v", info, err)
	}
	parent, _ := os.Stat(dir)
	if parent.Mode().Perm() != 0o700 {
		t.Fatalf("ASSERT_FAILURE_LEDGER_PARENT_0700: %o", parent.Mode().Perm())
	}
	raw, err := os.ReadFile(filepath.Join(dir, "publication-failures.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"/private", "secret", "raw error"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("ASSERT_FAILURE_LEDGER_NO_PRIVATE_TEXT: %q", raw)
		}
	}
	if !strings.Contains(string(raw), `"stage":"TEMP_WRITE"`) || !strings.Contains(string(raw), `"candidate_byte_length":17`) {
		t.Fatalf("ASSERT_FAILURE_LEDGER_SAFE_RECORD: %q", raw)
	}
	if err := ledger.Record(failure); err == nil {
		t.Fatal("ASSERT_FAILURE_LEDGER_RECORD_CAP")
	}
}

func TestFailureLedgerWriteFailureIsTypedAndNonPrimary(t *testing.T) {
	m, raw, authority := transactionalFixture(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ledger := &FailureLedger{write: func(FailureLedgerRecord) error { return errors.New("private ledger write") }}
	p := NewPublisherWithFailureLedger(root, ledger)
	first := p.PublishCaptureSet(m, raw, authority)
	if first.Receipt == nil {
		t.Fatalf("fixture publication failed: %#v", first)
	}
	second := p.PublishCaptureSet(m, raw, authority)
	if second.Receipt != nil || second.Failure == nil || second.LedgerFailure == nil || second.LedgerFailure.Stage != "LEDGER_WRITE" || second.LedgerFailure.Reason != "SINK_FAILED" {
		t.Fatalf("ASSERT_LEDGER_FAILURE_TYPED_NONPRIMARY: %#v", second)
	}
}
