package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/publication"
)

type experimentalHandoffReadCloser struct {
	io.Reader
	close func() error
}

func (r experimentalHandoffReadCloser) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}

type experimentalHandoffCountingReader struct {
	r     io.Reader
	reads int
}

func (r *experimentalHandoffCountingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.r.Read(p)
}

type experimentalHandoffPanicReader struct{}

func (experimentalHandoffPanicReader) Read([]byte) (int, error) {
	panic("named submission consumed stdin")
}

func experimentalHandoffResultDeps(t *testing.T, events *[]string, retained *[]byte, retainCalls, admitCalls *int) experimentalHandoffDependencies {
	t.Helper()
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	return experimentalHandoffDependencies{
		openStore: func(root string) (censuscontinuation.Store, io.Closer, error) {
			*events = append(*events, "store-open:"+root)
			return store, experimentalHandoffReadCloser{Reader: strings.NewReader(""), close: func() error {
				*events = append(*events, "root-close")
				return nil
			}}, nil
		},
		openFile: func(path string) (io.ReadCloser, error) {
			*events = append(*events, "file-open:"+path)
			return experimentalHandoffReadCloser{Reader: strings.NewReader("file bytes\n"), close: func() error {
				*events = append(*events, "file-close")
				return nil
			}}, nil
		},
		prepare: func(context.Context, censuscontinuation.Store, string) (string, error) {
			t.Fatal("prepare invoked")
			return "", nil
		},
		retain: func(_ context.Context, got censuscontinuation.Store, prepared string, raw []byte) (string, error) {
			*retainCalls++
			*events = append(*events, "retain")
			if got != store || prepared != "sha256:"+strings.Repeat("a", 64) {
				t.Fatalf("retain inputs store=%t prepared=%q", got == store, prepared)
			}
			*retained = append([]byte(nil), raw...)
			return "sha256:" + strings.Repeat("b", 64), nil
		},
		admit: func(_ context.Context, got censuscontinuation.Store, result string) (string, error) {
			*admitCalls++
			*events = append(*events, "admit")
			if got != store || result != "sha256:"+strings.Repeat("b", 64) {
				t.Fatalf("admit inputs store=%t result=%q", got == store, result)
			}
			return "sha256:" + strings.Repeat("c", 64), nil
		},
	}
}

func TestExperimentalHandoffRetainAndAdmitStrictReceiptsAndSeparation(t *testing.T) {
	prepared := "sha256:" + strings.Repeat("a", 64)
	result := "sha256:" + strings.Repeat("b", 64)
	var events []string
	var retained []byte
	var retainCalls, admitCalls int
	deps := experimentalHandoffResultDeps(t, &events, &retained, &retainCalls, &admitCalls)

	var stdout, stderr bytes.Buffer
	stdin := &experimentalHandoffCountingReader{r: strings.NewReader(" {\"raw\":true}\n")}
	stdinDeps := deps
	stdinDeps.openFile = nil
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"retain-result", "--publication-root", "/root", "--prepared-selector", prepared, "--submission", "-"}, stdin, &stdout, &stderr, stdinDeps)
	wantRetain := `{"schema_version":"lsp-trace.experimental-handoff-retained-result-receipt.v1","result_selector":"` + result + `"}` + "\n"
	if code != 0 || stdout.String() != wantRetain || stderr.Len() != 0 || string(retained) != " {\"raw\":true}\n" || retainCalls != 1 || admitCalls != 0 || stdin.reads == 0 {
		t.Fatalf("retain code=%d stdout=%q stderr=%q raw=%q calls=%d/%d reads=%d", code, stdout.String(), stderr.String(), retained, retainCalls, admitCalls, stdin.reads)
	}
	if strings.Join(events, ",") != "store-open:/root,retain,root-close" {
		t.Fatalf("retain events=%v", events)
	}

	events = nil
	stdout.Reset()
	stderr.Reset()
	code = runExperimentalHandoffWithDependencies(context.Background(), []string{"admit-result", "--publication-root", "/root", "--result-selector", result}, experimentalHandoffPanicReader{}, &stdout, &stderr, deps)
	wantAdmit := `{"schema_version":"lsp-trace.experimental-handoff-admitted-result-receipt.v1","admission_selector":"sha256:` + strings.Repeat("c", 64) + `"}` + "\n"
	if code != 0 || stdout.String() != wantAdmit || stderr.Len() != 0 || retainCalls != 1 || admitCalls != 1 {
		t.Fatalf("admit code=%d stdout=%q stderr=%q calls=%d/%d", code, stdout.String(), stderr.String(), retainCalls, admitCalls)
	}
	if strings.Join(events, ",") != "store-open:/root,admit,root-close" {
		t.Fatalf("admit events=%v", events)
	}
}

func TestExperimentalHandoffRetainNamedFileIngressBeforeStore(t *testing.T) {
	prepared := "sha256:" + strings.Repeat("a", 64)
	var events []string
	var retained []byte
	var retainCalls, admitCalls int
	deps := experimentalHandoffResultDeps(t, &events, &retained, &retainCalls, &admitCalls)
	var stdout, stderr bytes.Buffer
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"retain-result", "--publication-root", "/root", "--prepared-selector", prepared, "--submission", "submission.json"}, nil, &stdout, &stderr, deps)
	if code != 0 || stderr.Len() != 0 || string(retained) != "file bytes\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q retained=%q", code, stdout.String(), stderr.String(), retained)
	}
	if strings.Join(events, ",") != "file-open:submission.json,file-close,store-open:/root,retain,root-close" {
		t.Fatalf("events=%v", events)
	}
}

func TestExperimentalHandoffRetainFileCloseFailureBeforeStore(t *testing.T) {
	prepared := "sha256:" + strings.Repeat("a", 64)
	var opened, retained int
	deps := experimentalHandoffDependencies{
		openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
			opened++
			return nil, nil, errors.New("store must not open")
		},
		openFile: func(string) (io.ReadCloser, error) {
			return experimentalHandoffReadCloser{Reader: strings.NewReader("bytes"), close: func() error { return errors.New("file close failed") }}, nil
		},
		retain: func(context.Context, censuscontinuation.Store, string, []byte) (string, error) {
			retained++
			return "", errors.New("retain must not run")
		},
	}
	var stdout, stderr bytes.Buffer
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"retain-result", "--publication-root", "/root", "--prepared-selector", prepared, "--submission", "submission.json"}, experimentalHandoffPanicReader{}, &stdout, &stderr, deps)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "file close failed") || opened != 0 || retained != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q opened=%d retained=%d", code, stdout.String(), stderr.String(), opened, retained)
	}
}

func TestExperimentalHandoffRetainSubmissionBounds(t *testing.T) {
	prepared := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name string
		size int
		want int
	}{
		{name: "empty", size: 0, want: 1},
		{name: "exactly 64 MiB", size: experimentalHandoffMaxSubmissionBytes, want: 0},
		{name: "64 MiB plus one", size: experimentalHandoffMaxSubmissionBytes + 1, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			var retained []byte
			var retainCalls, admitCalls int
			deps := experimentalHandoffResultDeps(t, &events, &retained, &retainCalls, &admitCalls)
			var stdout, stderr bytes.Buffer
			code := runExperimentalHandoffWithDependencies(context.Background(), []string{"retain-result", "--publication-root", "/root", "--prepared-selector", prepared, "--submission", "-"}, bytes.NewReader(make([]byte, tc.size)), &stdout, &stderr, deps)
			if code != tc.want {
				t.Fatalf("code=%d want=%d stderr=%q", code, tc.want, stderr.String())
			}
			if tc.want != 0 && (stdout.Len() != 0 || retainCalls != 0 || len(events) != 0) {
				t.Fatalf("failure stdout=%q calls=%d events=%v", stdout.String(), retainCalls, events)
			}
			if tc.want == 0 && (len(retained) != tc.size || retainCalls != 1) {
				t.Fatalf("success retained=%d calls=%d", len(retained), retainCalls)
			}
		})
	}
}

func TestExperimentalHandoffResultFailuresAndCancellation(t *testing.T) {
	prepared := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name       string
		selector   string
		transition error
		closeErr   error
		cancel     bool
		want       int
	}{
		{name: "empty selector", want: 1},
		{name: "transition failure", transition: errors.New("transition failed"), want: 1},
		{name: "deadline is unrelated", transition: context.DeadlineExceeded, want: 1},
		{name: "transition canceled", transition: context.Canceled, want: 130},
		{name: "context observably canceled", transition: errors.New("observed"), cancel: true, want: 130},
		{name: "close failure", selector: "sha256:" + strings.Repeat("b", 64), closeErr: errors.New("close failed"), want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var stdout, stderr bytes.Buffer
			deps := experimentalHandoffDependencies{
				openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
					return &experimentalHandoffTestStore{objects: map[string][]byte{}}, experimentalHandoffReadCloser{Reader: strings.NewReader(""), close: func() error { return tc.closeErr }}, nil
				},
				openFile: func(string) (io.ReadCloser, error) { return nil, errors.New("unused") },
				retain: func(context.Context, censuscontinuation.Store, string, []byte) (string, error) {
					if tc.cancel {
						cancel()
					}
					return tc.selector, tc.transition
				},
			}
			code := runExperimentalHandoffWithDependencies(ctx, []string{"retain-result", "--publication-root", "/root", "--prepared-selector", prepared, "--submission", "-"}, strings.NewReader("x"), &stdout, &stderr, deps)
			if code != tc.want || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code=%d want=%d stdout=%q stderr=%q", code, tc.want, stdout.String(), stderr.String())
			}
		})
	}
}

func TestExperimentalHandoffResultCanceledAfterInstalledSelector(t *testing.T) {
	prepared := "sha256:" + strings.Repeat("a", 64)
	ctx, cancel := context.WithCancel(context.Background())
	closed := false
	deps := experimentalHandoffDependencies{
		openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
			return &experimentalHandoffTestStore{objects: map[string][]byte{}}, experimentalHandoffReadCloser{Reader: strings.NewReader(""), close: func() error { closed = true; return nil }}, nil
		},
		retain: func(context.Context, censuscontinuation.Store, string, []byte) (string, error) {
			cancel()
			return "sha256:" + strings.Repeat("b", 64), nil
		},
	}
	var stdout, stderr bytes.Buffer
	code := runExperimentalHandoffWithDependencies(ctx, []string{"retain-result", "--publication-root", "/root", "--prepared-selector", prepared, "--submission", "-"}, strings.NewReader("x"), &stdout, &stderr, deps)
	if code != 130 || !closed || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d closed=%t stdout=%q stderr=%q", code, closed, stdout.String(), stderr.String())
	}
}

func TestExperimentalHandoffResultCloseBeforeWriteAndPartialWrite(t *testing.T) {
	prepared := "sha256:" + strings.Repeat("a", 64)
	closed := false
	deps := experimentalHandoffDependencies{
		openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
			return &experimentalHandoffTestStore{objects: map[string][]byte{}}, experimentalHandoffReadCloser{Reader: strings.NewReader(""), close: func() error { closed = true; return nil }}, nil
		},
		openFile: func(string) (io.ReadCloser, error) { return nil, errors.New("unused") },
		retain: func(context.Context, censuscontinuation.Store, string, []byte) (string, error) {
			return "sha256:" + strings.Repeat("b", 64), nil
		},
	}
	var stderr bytes.Buffer
	writer := experimentalHandoffWriterFunc(func(p []byte) (int, error) {
		if !closed {
			t.Fatal("write before close")
		}
		return len(p) / 2, errors.New("partial write")
	})
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"retain-result", "--publication-root", "/root", "--prepared-selector", prepared, "--submission", "-"}, strings.NewReader("x"), writer, &stderr, deps)
	if code != 1 || !strings.Contains(stderr.String(), "partial write") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestExperimentalHandoffResultProductionRootInventoryAndReplay(t *testing.T) {
	rootPath, pins := copyExperimentalHandoffFixture(t)
	prepareOut, prepareErr, prepareCode := runProductionExperimentalHandoff(t, context.Background(), rootPath, pins.CheckpointSelector)
	if prepareCode != 0 || prepareErr != "" || !strings.Contains(prepareOut, pins.PreparedSelector) {
		t.Fatalf("prepare code=%d stdout=%q stderr=%q", prepareCode, prepareOut, prepareErr)
	}
	before := experimentalHandoffObjectInventory(t, rootPath)

	owner, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := continuationhost.NewStore(owner, experimentalHandoffMaxObjectBytes)
	if err != nil {
		t.Fatal(err)
	}
	preparedRaw, err := store.Get(context.Background(), pins.PreparedSelector)
	if err != nil {
		t.Fatal(err)
	}
	var prepared struct {
		RequestsSelector string `json:"requests_selector"`
	}
	if err := json.Unmarshal(preparedRaw, &prepared); err != nil || prepared.RequestsSelector == "" {
		t.Fatalf("prepared requests selector=%q err=%v", prepared.RequestsSelector, err)
	}
	requestsRaw, err := store.Get(context.Background(), prepared.RequestsSelector)
	if err != nil {
		t.Fatal(err)
	}
	requests, err := describerequest.Parse(requestsRaw)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	type submissionRow struct {
		RequestOrdinal  int             `json:"request_ordinal"`
		RecordID        string          `json:"record_id"`
		MessageID       string          `json:"message_id"`
		LineageIdentity string          `json:"lineage_identity"`
		Outcome         json.RawMessage `json:"outcome"`
	}
	rows := make([]submissionRow, len(requests))
	outcome := json.RawMessage(`{"kind":"UNKNOWN","ambiguities":[{"code":"MULTIPLE_READINGS","subject":"target role","options":["reading-a","reading-b"]}]}`)
	for i, request := range requests {
		rows[i] = submissionRow{RequestOrdinal: i, RecordID: request.RecordID, MessageID: request.Envelope.MessageID, LineageIdentity: request.Lineage.LineageIdentity, Outcome: outcome}
	}
	submission, err := json.Marshal(struct {
		SchemaVersion string          `json:"schema_version"`
		Provenance    string          `json:"provenance"`
		Results       []submissionRow `json:"results"`
	}{SchemaVersion: "lsp-trace.census-caller-result-submission.private.v1", Provenance: "CALLER_OBSERVED", Results: rows})
	if err != nil {
		t.Fatal(err)
	}

	runRetain := func() string {
		var stdout, stderr bytes.Buffer
		code := runExperimentalHandoff(context.Background(), []string{"retain-result", "--publication-root", rootPath, "--prepared-selector", pins.PreparedSelector, "--submission", "-"}, bytes.NewReader(submission), &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 {
			t.Fatalf("retain code=%d stderr=%q", code, stderr.String())
		}
		var receipt experimentalHandoffRetainedResultReceipt
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil || receipt.ResultSelector == "" {
			t.Fatalf("retain receipt=%q parsed=%+v err=%v", stdout.String(), receipt, err)
		}
		return receipt.ResultSelector
	}
	resultSelector := runRetain()
	if replay := runRetain(); replay != resultSelector {
		t.Fatalf("retain replay=%q want=%q", replay, resultSelector)
	}

	runAdmit := func() string {
		var stdout, stderr bytes.Buffer
		code := runExperimentalHandoff(context.Background(), []string{"admit-result", "--publication-root", rootPath, "--result-selector", resultSelector}, strings.NewReader("unused"), &stdout, &stderr)
		if code != 0 || stderr.Len() != 0 {
			t.Fatalf("admit code=%d stderr=%q", code, stderr.String())
		}
		var receipt experimentalHandoffAdmittedResultReceipt
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil || receipt.AdmissionSelector == "" {
			t.Fatalf("admit receipt=%q parsed=%+v err=%v", stdout.String(), receipt, err)
		}
		return receipt.AdmissionSelector
	}
	admissionSelector := runAdmit()
	if replay := runAdmit(); replay != admissionSelector {
		t.Fatalf("admit replay=%q want=%q", replay, admissionSelector)
	}

	after := experimentalHandoffObjectInventory(t, rootPath)
	if len(after) != len(before)+2 {
		t.Fatalf("inventory before=%d after=%d", len(before), len(after))
	}
	for name, identity := range before {
		if after[name] != identity {
			t.Fatalf("preexisting object changed name=%s before=%q after=%q", name, identity, after[name])
		}
	}
	for _, selector := range []string{resultSelector, admissionSelector} {
		name := strings.TrimPrefix(selector, "sha256:")
		if !strings.HasPrefix(after[name], selector+":") {
			t.Fatalf("new private object missing selector=%s entry=%q", selector, after[name])
		}
	}
}

func TestExperimentalHandoffResultUsageDispatchAndReplay(t *testing.T) {
	for _, command := range []string{
		"experimental-handoff retain-result --publication-root ROOT --prepared-selector sha256:HEX --submission PATH|-",
		"experimental-handoff admit-result --publication-root ROOT --result-selector sha256:HEX",
	} {
		if strings.Count(experimentalHandoffUsage, command) != 1 || strings.Count(usageText, "lsp-trace "+command) != 1 {
			t.Fatalf("usage missing or duplicated %q", command)
		}
	}
	var events []string
	var retained []byte
	var retainCalls, admitCalls int
	deps := experimentalHandoffResultDeps(t, &events, &retained, &retainCalls, &admitCalls)
	args := []string{"retain-result", "--publication-root", "/root", "--prepared-selector", "sha256:" + strings.Repeat("a", 64), "--submission", "-"}
	var first, second, stderr bytes.Buffer
	if code := runExperimentalHandoffWithDependencies(context.Background(), args, strings.NewReader("same"), &first, &stderr, deps); code != 0 {
		t.Fatalf("first code=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()
	if code := runExperimentalHandoffWithDependencies(context.Background(), args, strings.NewReader("same"), &second, &stderr, deps); code != 0 || first.String() != second.String() || retainCalls != 2 {
		t.Fatalf("replay code=%d equal=%t calls=%d stderr=%q", code, first.String() == second.String(), retainCalls, stderr.String())
	}
}
