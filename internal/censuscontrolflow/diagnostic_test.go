package censuscontrolflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestRecorderConcurrentAppendSerializesCanonicalLedgerAtExactLimits(t *testing.T) {
	records := []Record{
		{SchemaVersion, CoreFailed, PhaseDiscovery, BatchNotAttempted},
		{SchemaVersion, CoreFailed, PhaseBatchAcquire, BatchAccepted},
		{SchemaVersion, PostCoreRuntimeInvalid, PhaseCompleted, BatchRejected},
	}
	var maxBytes int64
	wantLines := make(map[string]int, len(records))
	for _, record := range records {
		line, err := Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		maxBytes += int64(len(line))
		wantLines[string(line)]++
	}
	path := privatePath(t)
	recorder, err := NewRecorder(path, maxBytes, len(records))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make([]error, len(records))
	var ready, done sync.WaitGroup
	ready.Add(len(records))
	done.Add(len(records))
	for i := range records {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			errs[i] = recorder.Record(records[i])
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("record %d rejected: %v", i, err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(raw)) != maxBytes || bytes.Count(raw, []byte{'\n'}) != len(records) {
		t.Fatalf("bytes=%d/%d records=%d/%d ledger=%q", len(raw), maxBytes, bytes.Count(raw, []byte{'\n'}), len(records), raw)
	}
	gotLines := make(map[string]int, len(records))
	for _, line := range bytes.SplitAfter(raw, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		if _, err := Parse(bytes.TrimSuffix(line, []byte{'\n'})); err != nil {
			t.Fatalf("corrupt line %q: %v", line, err)
		}
		gotLines[string(line)]++
	}
	if len(gotLines) != len(wantLines) {
		t.Fatalf("got lines=%v want=%v", gotLines, wantLines)
	}
	for line, count := range wantLines {
		if gotLines[line] != count {
			t.Fatalf("line %q count=%d want=%d", line, gotLines[line], count)
		}
	}
	if err := recorder.Record(records[0]); err == nil {
		t.Fatal("accepted write beyond exact record/byte limits")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, raw) {
		t.Fatalf("ledger changed after rejected write: %q err=%v", after, err)
	}
}

func TestRecorderConcurrentAppendHonorsOneRecordAndByteLimitWithoutCorruption(t *testing.T) {
	record := Record{SchemaVersion, FinalNonzeroFallback, PhaseCompleted, BatchNotAttempted}
	line, err := Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		maxBytes   int64
		maxRecords int
	}{
		{"one-record-limit", DefaultMaxBytes, 1},
		{"one-record-byte-cap", int64(len(line)), 32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := privatePath(t)
			recorder, err := NewRecorder(path, tc.maxBytes, tc.maxRecords)
			if err != nil {
				t.Fatal(err)
			}
			const workers = 24
			start := make(chan struct{})
			errs := make(chan error, workers)
			var ready, done sync.WaitGroup
			ready.Add(workers)
			done.Add(workers)
			for i := 0; i < workers; i++ {
				go func() {
					defer done.Done()
					ready.Done()
					<-start
					errs <- recorder.Record(record)
				}()
			}
			ready.Wait()
			close(start)
			done.Wait()
			close(errs)
			accepted := 0
			for err := range errs {
				if err == nil {
					accepted++
				}
			}
			if accepted != 1 {
				t.Fatalf("accepted=%d want=1", accepted)
			}
			raw, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(raw, line) {
				t.Fatalf("ledger=%q want=%q err=%v", raw, line, err)
			}
			if parsed, err := Parse(bytes.TrimSuffix(raw, []byte{'\n'})); err != nil || parsed != record {
				t.Fatalf("parsed=%+v err=%v", parsed, err)
			}
		})
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
