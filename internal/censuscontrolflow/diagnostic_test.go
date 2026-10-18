package censuscontrolflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func allRecords() []Record {
	causes := []FirstCause{CallbackNotEntered, CallbackRuntimeInvalid, CoreFailed, PostCoreRuntimeInvalid, FinalNonzeroFallback}
	phases := []CorePhase{PhaseNotEntered, PhaseInput, PhaseDiscovery, PhasePlanning, PhaseBatchAcquire, PhaseBatchAdmission, PhaseManifest, PhaseCompleted}
	batches := []BatchRecord{BatchNotAttempted, BatchAccepted, BatchRejected}
	var out []Record
	for _, cause := range causes {
		for _, phase := range phases {
			for _, batch := range batches {
				out = append(out, Record{SchemaVersion, cause, phase, batch})
			}
		}
	}
	return out
}

func TestCanonicalExactBytesAndAllEnumCombinations(t *testing.T) {
	const want = "{\"schema_version\":\"lsp-trace.census-control-flow-diagnostic.v1\",\"first_cause\":\"CORE_FAILED\",\"core_phase\":\"BATCH_ACQUIRE\",\"batch_record\":\"ACCEPTED\"}\n"
	got, err := Marshal(Record{SchemaVersion, CoreFailed, PhaseBatchAcquire, BatchAccepted})
	if err != nil || string(got) != want {
		t.Fatalf("bytes=%q err=%v", got, err)
	}
	for _, record := range allRecords() {
		raw, err := Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := Parse(raw[:len(raw)-1])
		if err != nil || parsed != record {
			t.Fatalf("record=%+v parsed=%+v err=%v", record, parsed, err)
		}
	}
}

func TestStrictParserRejectsUnknownMissingInvalidAndNoncanonical(t *testing.T) {
	valid, _ := Marshal(Record{SchemaVersion, CoreFailed, PhaseInput, BatchNotAttempted})
	line := string(valid[:len(valid)-1])
	bad := []string{
		strings.Replace(line, "}", ",\"extra\":1}", 1),
		strings.Replace(line, "\"batch_record\":\"NOT_ATTEMPTED\"", "", 1),
		strings.Replace(line, "CORE_FAILED", "UNKNOWN", 1),
		"{\"first_cause\":\"CORE_FAILED\",\"schema_version\":\"lsp-trace.census-control-flow-diagnostic.v1\",\"core_phase\":\"INPUT\",\"batch_record\":\"NOT_ATTEMPTED\"}",
		line + " ",
	}
	for _, raw := range bad {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func privatePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "control.ndjson")
}

func TestRecorderCustodyAppendSyncAndDriftFailClosed(t *testing.T) {
	path := privatePath(t)
	r, err := NewRecorder(path, DefaultMaxBytes, DefaultMaxRecords)
	if err != nil {
		t.Fatal(err)
	}
	record := Record{SchemaVersion, CoreFailed, PhaseBatchAdmission, BatchRejected}
	if err := r.Record(record); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLast(path)
	if err != nil || got != record {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	info, _ := os.Lstat(path)
	if info.Mode().Perm() != 0600 || !info.Mode().IsRegular() {
		t.Fatalf("mode=%v", info.Mode())
	}
	if err := r.Record(record); err == nil {
		t.Fatal("accepted second record")
	}
	if err := os.Chmod(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLast(path); err == nil {
		t.Fatal("accepted parent custody drift")
	}
}

func TestRecorderRejectsNonprivateParentAndSymlinkFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRecorder(filepath.Join(dir, "x"), DefaultMaxBytes, 1); err == nil {
		t.Fatal("accepted public parent")
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(private, "target")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(private, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRecorder(link, DefaultMaxBytes, 1); err == nil {
		t.Fatal("accepted symlink")
	}
}
