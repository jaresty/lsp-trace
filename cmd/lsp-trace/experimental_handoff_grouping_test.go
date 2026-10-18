package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
)

func TestExperimentalHandoffRetainGroupingStrictReceiptAndRawIngress(t *testing.T) {
	candidate := "sha256:" + strings.Repeat("a", 64)
	grouping := "sha256:" + strings.Repeat("b", 64)
	var events []string
	var retained []byte
	calls := 0
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	deps := experimentalHandoffDependencies{
		openStore: func(root string) (censuscontinuation.Store, io.Closer, error) {
			events = append(events, "store-open:"+root)
			return store, experimentalHandoffReadCloser{Reader: strings.NewReader(""), close: func() error {
				events = append(events, "root-close")
				return nil
			}}, nil
		},
		retainGrouping: func(_ context.Context, got censuscontinuation.Store, gotCandidate string, raw []byte) (string, error) {
			calls++
			events = append(events, "retain-grouping")
			if got != store || gotCandidate != candidate {
				t.Fatalf("inputs store=%t candidate=%q", got == store, gotCandidate)
			}
			retained = append([]byte(nil), raw...)
			return grouping, nil
		},
	}
	stdin := &experimentalHandoffCountingReader{r: strings.NewReader(" {\"raw\":true}\n")}
	var stdout, stderr bytes.Buffer
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"retain-grouping", "--publication-root", "/root", "--candidate-selector", candidate, "--submission", "-"}, stdin, &stdout, &stderr, deps)
	want := `{"schema_version":"lsp-trace.experimental-handoff-grouping-selector-receipt.v1","grouping_selector":"` + grouping + `"}` + "\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 || string(retained) != " {\"raw\":true}\n" || calls != 1 || stdin.reads == 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q retained=%q calls=%d reads=%d", code, stdout.String(), stderr.String(), retained, calls, stdin.reads)
	}
	if strings.Join(events, ",") != "store-open:/root,retain-grouping,root-close" {
		t.Fatalf("events=%v", events)
	}
}

func TestExperimentalHandoffRetainGroupingNamedFileCloseBeforeStore(t *testing.T) {
	candidate := "sha256:" + strings.Repeat("a", 64)
	var events []string
	deps := experimentalHandoffDependencies{
		openFile: func(path string) (io.ReadCloser, error) {
			events = append(events, "file-open:"+path)
			return experimentalHandoffReadCloser{Reader: strings.NewReader("file bytes\n"), close: func() error {
				events = append(events, "file-close")
				return nil
			}}, nil
		},
		openStore: func(root string) (censuscontinuation.Store, io.Closer, error) {
			events = append(events, "store-open:"+root)
			return &experimentalHandoffTestStore{objects: map[string][]byte{}}, experimentalHandoffReadCloser{Reader: strings.NewReader(""), close: func() error {
				events = append(events, "root-close")
				return nil
			}}, nil
		},
		retainGrouping: func(_ context.Context, _ censuscontinuation.Store, _ string, raw []byte) (string, error) {
			events = append(events, "retain-grouping:"+string(raw))
			return "sha256:" + strings.Repeat("b", 64), nil
		},
	}
	var stdout, stderr bytes.Buffer
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"retain-grouping", "--publication-root", "/root", "--candidate-selector", candidate, "--submission", "submission.json"}, experimentalHandoffPanicReader{}, &stdout, &stderr, deps)
	if code != 0 || stderr.Len() != 0 || strings.Join(events, ",") != "file-open:submission.json,file-close,store-open:/root,retain-grouping:file bytes\n,root-close" {
		t.Fatalf("code=%d stdout=%q stderr=%q events=%v", code, stdout.String(), stderr.String(), events)
	}
}

func TestExperimentalHandoffRetainGroupingNoReceiptOnFailure(t *testing.T) {
	candidate := "sha256:" + strings.Repeat("a", 64)
	grouping := "sha256:" + strings.Repeat("b", 64)
	for _, tc := range []struct {
		name       string
		selector   string
		apiErr     error
		closeErr   error
		cancel     bool
		wantCode   int
		wantStderr bool
	}{
		{name: "api error", apiErr: errors.New("grouping API failed"), wantCode: 1, wantStderr: true},
		{name: "cancellation", selector: grouping, cancel: true, wantCode: 130},
		{name: "close error", selector: grouping, closeErr: errors.New("close failed"), wantCode: 1, wantStderr: true},
		{name: "empty selector", wantCode: 1, wantStderr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deps := experimentalHandoffDependencies{
				openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
					return &experimentalHandoffTestStore{objects: map[string][]byte{}}, experimentalHandoffReadCloser{Reader: strings.NewReader(""), close: func() error { return tc.closeErr }}, nil
				},
				retainGrouping: func(context.Context, censuscontinuation.Store, string, []byte) (string, error) {
					if tc.cancel {
						cancel()
					}
					return tc.selector, tc.apiErr
				},
			}
			var stdout, stderr bytes.Buffer
			code := runExperimentalHandoffWithDependencies(ctx, []string{"retain-grouping", "--publication-root", "/root", "--candidate-selector", candidate, "--submission", "-"}, strings.NewReader("x"), &stdout, &stderr, deps)
			if code != tc.wantCode || stdout.Len() != 0 || (stderr.Len() != 0) != tc.wantStderr {
				t.Fatalf("code=%d want=%d stdout=%q stderr=%q", code, tc.wantCode, stdout.String(), stderr.String())
			}
		})
	}
}

func TestExperimentalHandoffRetainGroupingAcceptedAPI(t *testing.T) {
	deps := productionExperimentalHandoffDependencies()
	if deps.retainGrouping == nil {
		t.Fatal("accepted grouping API is not installed")
	}
	selector, err := deps.retainGrouping(context.Background(), &experimentalHandoffTestStore{objects: map[string][]byte{}}, "not-a-selector", []byte(`{}`))
	if err == nil || selector != "" || !strings.Contains(err.Error(), "private grouping input invalid") {
		t.Fatalf("selector=%q err=%v", selector, err)
	}
}

func TestExperimentalHandoffRetainGroupingUsageCompatibility(t *testing.T) {
	command := "experimental-handoff retain-grouping --publication-root ROOT --candidate-selector sha256:HEX --submission PATH|-"
	if strings.Count(experimentalHandoffUsage, command) != 1 || strings.Count(usageText, "lsp-trace "+command) != 1 {
		t.Fatalf("usage missing or duplicated %q", command)
	}
}
