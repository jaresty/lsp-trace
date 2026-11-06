package publication

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

type publicationCountingReader struct {
	reader   io.Reader
	consumed int
}

func (r *publicationCountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.consumed += n
	return n, err
}

// This finite source-shape guard binds the counted primitive to the real-file
// path. Behavioral tests below independently exercise both sides of the call.
func TestPublishedReaderProductionWiring(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(source), "selector.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "func readPublished(parent ")
	end := strings.Index(text, "\nfunc createTemporarySibling(")
	if start < 0 || end <= start || !strings.Contains(text[start:end], "return readPublishedBytes(file, limit)") || strings.Contains(text[start:end], "io.ReadAll(") {
		t.Fatal("ASSERT_PRODUCTION_BOUNDED_PRIMITIVE: reread must use the counted bounded primitive")
	}
	if !strings.Contains(text, "readPublished(target.parent, target.name, uint64(len(req.Bytes)))") {
		t.Fatal("ASSERT_ORIGINAL_PAYLOAD_LENGTH: reread bound must originate in request bytes")
	}
}

func TestPublishedReaderActualConsumption(t *testing.T) {
	for _, expected := range []uint64{0, 3} {
		limit, err := publishedReadLimit(expected)
		if err != nil {
			t.Fatal(err)
		}
		r := &publicationCountingReader{reader: strings.NewReader("finite growing file contents")}
		got, err := readPublishedBytes(r, limit)
		if err != nil || r.consumed != int(expected+1) || len(got) != int(expected+1) {
			t.Fatalf("ASSERT_ACTUAL_READER_BOUND: expected=%d consumed=%d returned=%d err=%v", expected, r.consumed, len(got), err)
		}
	}
}

func TestPublishedReaderSentinelArithmetic(t *testing.T) {
	maxInt := uint64(^uint(0) >> 1)
	for _, expected := range []uint64{^uint64(0), uint64(1<<63 - 1), maxInt} {
		if _, err := publishedReadLimit(expected); err == nil {
			t.Fatalf("ASSERT_SENTINEL_REFUSES: expected=%d", expected)
		}
		// A nil root would panic on Open: rejection must precede even opening.
		if _, err := readPublished(nil, "unused", expected); err == nil {
			t.Fatalf("ASSERT_SENTINEL_BEFORE_OPEN: expected=%d", expected)
		}
	}
	limit, err := publishedReadLimit(maxInt - 1)
	if err != nil || limit != int64(maxInt) {
		t.Fatalf("ASSERT_SENTINEL_MAX_ADMITTED: limit=%d err=%v", limit, err)
	}
	r := &publicationCountingReader{reader: strings.NewReader("")}
	if got, err := readPublishedBytes(r, limit); err != nil || len(got) != 0 || r.consumed != 0 {
		t.Fatalf("ASSERT_MAX_BOUND_TINY_EOF: got=%q err=%v consumed=%d", got, err, r.consumed)
	}
}

type publicationErrorReader struct{}

func (publicationErrorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestPublishedReaderErrorAndRegularFile(t *testing.T) {
	if _, err := readPublishedBytes(publicationErrorReader{}, 4); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ASSERT_READER_ERROR: %v", err)
	}
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := readPublished(root, ".", 3); err == nil || err.Error() != "published target is not a regular file" {
		t.Fatalf("ASSERT_REGULAR_REREAD: %v", err)
	}
	for _, tc := range []struct {
		name, contents, want string
		expected             uint64
	}{
		{"exact", "abc", "abc", 3}, {"growth", "abcdefghi", "abcd", 3}, {"truncated", "ab", "ab", 3}, {"same-length", "xyz", "xyz", 3}, {"empty", "", "", 0}, {"empty-growth", "x", "x", 0},
	} {
		if err := os.WriteFile(filepath.Join(dir, tc.name), []byte(tc.contents), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := readPublished(root, tc.name, tc.expected)
		if err != nil || string(got) != tc.want {
			t.Fatalf("ASSERT_REALFILE_BOUND: %s got=%q err=%v", tc.name, got, err)
		}
	}
}

func TestPublishRereadPostinstallEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		original, replacement []byte
		readError             bool
	}{
		{"growth", []byte("abc"), []byte("abcdefghi"), false},
		{"cleanup-failure", []byte("abc"), []byte("abcdefghi"), false},
		{"truncated", []byte("abc"), []byte("ab"), false},
		{"same-length", []byte("abc"), []byte("xyz"), false},
		{"empty-growth", nil, []byte("x"), false},
		{"read-error", []byte("abc"), nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			var events []TraceEvent
			installed := false
			result := NewPublisher().Publish(Request{Root: root, Selector: "out", Bytes: tc.original, Trace: func(e TraceEvent) {
				events = append(events, e)
				if e.Stage == "NO_REPLACE" && e.Reason == "HARD_LINK_INSTALLED" {
					installed = true
					if tc.name == "cleanup-failure" {
						entries, err := os.ReadDir(root.Path())
						if err != nil {
							t.Fatal(err)
						}
						removed := false
						for _, entry := range entries {
							if strings.HasPrefix(entry.Name(), ".lsp-trace-publication-") {
								if err := root.handle.Remove(entry.Name()); err != nil {
									t.Fatal(err)
								}
								removed = true
							}
						}
						if !removed {
							t.Fatal("temporary sibling not reached")
						}
					}
					if tc.readError {
						// Retain a final entry, but force the existing regular-file read error.
						if err := root.handle.Remove("out"); err != nil {
							t.Fatal(err)
						}
						if err := root.handle.Mkdir("out", 0700); err != nil {
							t.Fatal(err)
						}
					} else if err := os.WriteFile(filepath.Join(root.Path(), "out"), tc.replacement, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}})
			f := result.Failure
			if !installed || result.Receipt != nil || f == nil || f.Stage != "reread" || f.Code != CodePublicationFailed || f.Cleanup == (tc.name == "cleanup-failure") || !f.AtomicRename || f.Truncated || result.Partial != nil {
				t.Fatalf("ASSERT_POSTINSTALL_ENVELOPE: installed=%t result=%+v failure=%+v", installed, result, f)
			}
			cause, exit := "published exact bytes mismatch", "REREAD_MISMATCH"
			if tc.readError {
				cause, exit = "published target is not a regular file", "REREAD_FAILED"
			}
			if !slices.Contains(events, TraceEvent{Stage: "EXIT", Reason: exit, OK: false}) || f.Unwrap() == nil || f.Unwrap().Error() != cause {
				t.Fatalf("ASSERT_POSTINSTALL_CAUSE_TRACE: cause=%v events=%v", f.Unwrap(), events)
			}
			if _, err := root.handle.Lstat("out"); err != nil {
				t.Fatalf("ASSERT_FINAL_ENTRY_RETAINED: %v", err)
			}
			entries, err := os.ReadDir(root.Path())
			if err != nil || len(entries) != 1 || entries[0].Name() != "out" {
				t.Fatalf("ASSERT_ACTUAL_CLEANUP: entries=%v err=%v", entries, err)
			}
		})
	}
}

func TestPublishEmptyReceipt(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	r := NewPublisher().Publish(Request{Root: root, Selector: "empty", ArtifactSchemaID: "empty-schema"})
	sum := sha256.Sum256(nil)
	want := Receipt{Digest: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: 0, ArtifactSchemaID: "empty-schema", PublicationMechanism: PublicationMechanism}
	if r.Failure != nil || r.Receipt == nil || *r.Receipt != want {
		t.Fatalf("ASSERT_EMPTY_EXACT_RECEIPT: result=%+v receipt=%+v", r, r.Receipt)
	}
}

func TestPublishLinkCollisionPreservesFinal(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	var events []TraceEvent
	r := NewPublisher().Publish(Request{Root: root, Selector: "out", Bytes: []byte("candidate"), Trace: func(e TraceEvent) {
		events = append(events, e)
		if e.Stage == "TEMP_SYNC_CLOSE" && e.OK {
			if err := os.WriteFile(filepath.Join(root.Path(), "out"), []byte("winner"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}})
	got, err := os.ReadFile(filepath.Join(root.Path(), "out"))
	if err != nil || string(got) != "winner" || r.Receipt != nil || r.Failure == nil || r.Failure.Code != CodeTargetExists || r.Failure.Stage != "install" || !r.Failure.Cleanup || r.Failure.AtomicRename || !errors.Is(r.Err(), os.ErrExist) {
		t.Fatalf("ASSERT_LINK_COLLISION: bytes=%q err=%v result=%+v failure=%+v", got, err, r, r.Failure)
	}
	for _, e := range events {
		if e.Stage == "REREAD_EQUAL" {
			t.Fatalf("ASSERT_COLLISION_NO_REREAD: %v", events)
		}
	}
}

func TestSelectorPathPartsArePlatformPortable(t *testing.T) {
	const assertion = "ASSERT_PLATFORM_PORTABLE_SELECTOR_COMPONENTS"
	for _, tc := range []struct {
		goos     string
		selector string
		want     []string
		wantErr  bool
	}{
		{goos: "linux", selector: "nested/output.json", want: []string{"nested", "output.json"}},
		{goos: "darwin", selector: "nested/output.json", want: []string{"nested", "output.json"}},
		{goos: "windows", selector: "nested/output.json", want: []string{"nested", "output.json"}},
		{goos: "windows", selector: `nested\output.json`, wantErr: true},
	} {
		got, err := selectorPathParts(tc.goos, tc.selector)
		if (err != nil) != tc.wantErr || !tc.wantErr && !slices.Equal(got, tc.want) {
			t.Fatalf("%s: goos=%s selector=%q parts=%q err=%v want=%q want_err=%t", assertion, tc.goos, tc.selector, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestReadSelectorWithAbsenceDistinguishesOnlyConfirmedAbsence(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.ReadSelectorWithAbsence("missing.json", 16); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ASSERT_SELECTOR_FINAL_ABSENCE_DISTINGUISHED: %v", err)
	}
	if _, err := root.ReadSelectorWithAbsence("missing/child.json", 16); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ASSERT_SELECTOR_PARENT_ABSENCE_DISTINGUISHED: %v", err)
	}
	if _, err := root.ReadSelectorWithAbsence("../escape", 16); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ASSERT_SELECTOR_UNSAFE_NOT_ABSENCE: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root.Path(), "present.json"), []byte("present"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := root.ReadSelectorWithAbsence("present.json", 16); err != nil || string(got) != "present" {
		t.Fatalf("ASSERT_SELECTOR_PRESENT_READ got=%q err=%v", got, err)
	}
}

func TestOwnerOnlyModeVerificationIsPlatformAppropriate(t *testing.T) {
	const assertion = "ASSERT_PLATFORM_APPROPRIATE_OWNER_ONLY_MODE_VERIFICATION"
	for _, tc := range []struct {
		goos string
		want bool
	}{{"linux", true}, {"darwin", true}, {"windows", false}} {
		if got := enforcePOSIXOwnerOnlyMode(tc.goos); got != tc.want {
			t.Fatalf("%s: goos=%s got=%t want=%t", assertion, tc.goos, got, tc.want)
		}
	}
}

type chunkWriter struct {
	bytes.Buffer
	limit int
}

func (w *chunkWriter) Write(data []byte) (int, error) {
	if len(data) > w.limit {
		data = data[:w.limit]
	}
	return w.Buffer.Write(data)
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

func TestWriteAllRejectsNoProgressAndCompletesShortWrites(t *testing.T) {
	const assertion = "ASSERT_PUBLICATION_NO_SILENT_TRUNCATION"
	writer := &chunkWriter{limit: 2}
	data := []byte("complete canonical bytes")
	if err := writeAll(writer, data); err != nil || !bytes.Equal(writer.Bytes(), data) {
		t.Fatalf("%s: bytes=%q err=%v", assertion, writer.Bytes(), err)
	}
	if err := writeAll(zeroWriter{}, data); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("%s: zero-progress err=%v", assertion, err)
	}
}

func TestPublishRejectsCollisionWithoutChangingBytes(t *testing.T) {
	const assertion = "ASSERT_PUBLICATION_COLLISION_PRESERVES_IMMUTABLE_BYTES"
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publisher := NewPublisher()
	if result := publisher.Publish(Request{Root: root, Selector: "immutable", Bytes: []byte("first")}); result.Failure != nil {
		t.Fatalf("%s: first=%#v", assertion, result)
	}
	var events []TraceEvent
	result := publisher.Publish(Request{Root: root, Selector: "immutable", Bytes: []byte("second"), Trace: func(e TraceEvent) { events = append(events, e) }})
	got, err := os.ReadFile(filepath.Join(root.Path(), "immutable"))
	if result.Failure == nil || result.Receipt != nil || result.Failure.Code != CodeTargetExists || result.Failure.Stage != "install" || !result.Failure.Cleanup || result.Failure.AtomicRename || !errors.Is(result.Err(), os.ErrExist) || err != nil || string(got) != "first" {
		t.Fatalf("%s: result=%#v bytes=%q err=%v", assertion, result, got, err)
	}
	for _, e := range events {
		if e.Stage == "REREAD_EQUAL" {
			t.Fatalf("ASSERT_EARLY_COLLISION_NO_REREAD: %v", events)
		}
	}
}

func TestPublishExactBytesAndReceipt(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	data := []byte("exact\x00bytes")
	r := NewPublisher().Publish(Request{Root: root, Selector: "out.bin", Bytes: data, ArtifactSchemaID: "schema-id"})
	if r.Failure != nil {
		t.Fatalf("publish: %+v", r.Failure)
	}
	got, err := os.ReadFile(filepath.Join(root.Path(), "out.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("bytes = %q", got)
	}
	sum := sha256.Sum256(data)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if r.Receipt == nil || r.Receipt.ByteLength != uint64(len(data)) || r.Receipt.Digest != wantDigest || r.Receipt.ArtifactSchemaID != "schema-id" || r.Receipt.PublicationMechanism != PublicationMechanism {
		t.Fatalf("receipt = %#v", r.Receipt)
	}
	info, err := os.Stat(filepath.Join(root.Path(), "out.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if enforcePOSIXOwnerOnlyMode(runtime.GOOS) && info.Mode().Perm() != 0600 {
		t.Fatalf("published mode = %o", info.Mode().Perm())
	}
}

func TestUnsafeSelectorsFailClosed(t *testing.T) {
	root, _ := OpenRoot(t.TempDir())
	defer root.Close()
	for _, selector := range []string{"", ".", "../escape", "/absolute", "a/../../escape", "a//b", "a/./b", "nul\x00byte"} {
		r := NewPublisher().Publish(Request{Root: root, Selector: selector, Bytes: []byte("x")})
		if r.Failure == nil || r.Failure.Code != CodeOutputSelectorUnsafe {
			t.Errorf("selector %q: %#v", selector, r)
		}
	}
}

func TestNestedParentsAreOwnerOnly(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	result := NewPublisher().Publish(Request{Root: root, Selector: "nested/deep/out", Bytes: []byte("x")})
	if result.Failure != nil {
		t.Fatalf("publish: %#v", result.Failure)
	}
	published, err := os.ReadFile(filepath.Join(root.Path(), "nested/deep/out"))
	if err != nil {
		t.Fatal(err)
	}
	if string(published) != "x" {
		t.Fatalf("nested published bytes = %q", published)
	}
	for _, name := range []string{"nested", "nested/deep"} {
		info, err := os.Stat(filepath.Join(root.Path(), name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() || enforcePOSIXOwnerOnlyMode(runtime.GOOS) && info.Mode().Perm() != 0700 {
			t.Fatalf("%s mode = %v", name, info.Mode())
		}
	}
}

func TestSymlinkComponentsFailClosed(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "real"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(dir, "alias")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, selector := range []string{"alias/out", "alias"} {
		result := NewPublisher().Publish(Request{Root: root, Selector: selector, Bytes: []byte("x")})
		if result.Failure == nil || result.Failure.Code != CodeOutputSelectorUnsafe {
			t.Errorf("selector %q: %#v", selector, result)
		}
	}
}

func TestPinnedRootSubstitutionFailsClosed(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "root")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(path, path+".old"); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("platform prevents replacing an open pinned root: %v", err)
		}
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	r := NewPublisher().Publish(Request{Root: root, Selector: "out", Bytes: []byte("x")})
	if r.Failure == nil || r.Failure.Code != CodeOutputSelectorUnsafe {
		t.Fatalf("result = %#v", r)
	}
}

func TestConcurrentCollisionHasOneSuccess(t *testing.T) {
	root, _ := OpenRoot(t.TempDir())
	defer root.Close()
	p := NewPublisher()
	results := make(chan Result, 2)
	var wg sync.WaitGroup
	for _, data := range [][]byte{[]byte("a"), []byte("b")} {
		wg.Add(1)
		go func(data []byte) {
			defer wg.Done()
			results <- p.Publish(Request{Root: root, Selector: "same", Bytes: data})
		}(data)
	}
	wg.Wait()
	close(results)
	success := 0
	for r := range results {
		if r.Receipt != nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successes = %d", success)
	}
}

func TestConcurrentEquivalentRootsHaveOneSuccess(t *testing.T) {
	dir := t.TempDir()
	first, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	publisher := NewPublisher()
	results := make(chan Result, 2)
	var wg sync.WaitGroup
	for _, root := range []*Root{first, second} {
		wg.Add(1)
		go func(root *Root) {
			defer wg.Done()
			results <- publisher.Publish(Request{Root: root, Selector: "same", Bytes: []byte("x")})
		}(root)
	}
	wg.Wait()
	close(results)
	successes := 0
	for result := range results {
		if result.Receipt != nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successes = %d", successes)
	}
}

func TestOperationIsTerminalOnce(t *testing.T) {
	root, _ := OpenRoot(t.TempDir())
	defer root.Close()
	op := NewOperation(NewPublisher(), Request{Root: root, Selector: "once", Bytes: []byte("x")})
	first, second := op.Publish(), op.Publish()
	if first.Receipt == nil {
		t.Fatalf("first = %#v", first)
	}
	if !errors.Is(second.Err(), ErrOperationTerminal) {
		t.Fatalf("second = %#v", second)
	}
}

func TestOperationConcurrentPublishIsTerminalOnce(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	op := NewOperation(NewPublisher(), Request{Root: root, Selector: "once-concurrent", Bytes: []byte("x")})
	results := make(chan Result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- op.Publish()
		}()
	}
	wg.Wait()
	close(results)
	receipts, terminals := 0, 0
	for result := range results {
		if result.Receipt != nil {
			receipts++
		}
		if errors.Is(result.Err(), ErrOperationTerminal) {
			terminals++
		}
	}
	if receipts != 1 || terminals != 1 {
		t.Fatalf("receipts=%d terminal_failures=%d", receipts, terminals)
	}
}

func TestFailureHasNoReceiptOrPathEcho(t *testing.T) {
	root, _ := OpenRoot(t.TempDir())
	defer root.Close()
	secret := "secret-selector"
	r := NewPublisher().Publish(Request{Root: root, Selector: "../" + secret, Bytes: []byte("x")})
	if r.Receipt != nil || r.Failure == nil {
		t.Fatalf("result = %#v", r)
	}
	if strings.Contains(r.Failure.Error(), secret) {
		t.Fatalf("failure leaked selector: %v", r.Failure)
	}
	if r.Failure.Stage == "" || r.Failure.Code == "" {
		t.Fatalf("unbounded failure = %#v", r.Failure)
	}
}
