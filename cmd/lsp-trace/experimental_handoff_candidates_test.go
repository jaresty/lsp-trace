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

func experimentalHandoffCandidateDeps(t *testing.T, events *[]string, transition func(context.Context, censuscontinuation.Store, string) (string, error), closeErr error) experimentalHandoffDependencies {
	t.Helper()
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	return experimentalHandoffDependencies{
		openStore: func(root string) (censuscontinuation.Store, io.Closer, error) {
			*events = append(*events, "store-open:"+root)
			return store, experimentalHandoffReadCloser{Reader: strings.NewReader(""), close: func() error {
				*events = append(*events, "root-close")
				return closeErr
			}}, nil
		},
		prepareCandidates: func(ctx context.Context, got censuscontinuation.Store, admission string) (string, error) {
			*events = append(*events, "prepare-candidates")
			if got != store {
				t.Fatal("wrong store")
			}
			return transition(ctx, got, admission)
		},
	}
}

func TestExperimentalHandoffPrepareCandidatesSuccessClosesBeforeMinimalReceipt(t *testing.T) {
	admission := "sha256:" + strings.Repeat("a", 64)
	candidate := "sha256:" + strings.Repeat("b", 64)
	var events []string
	deps := experimentalHandoffCandidateDeps(t, &events, func(_ context.Context, _ censuscontinuation.Store, got string) (string, error) {
		if got != admission {
			t.Fatalf("admission=%q", got)
		}
		return candidate, nil
	}, nil)
	var stdout, stderr bytes.Buffer
	writer := experimentalHandoffWriterFunc(func(p []byte) (int, error) {
		events = append(events, "output")
		return stdout.Write(p)
	})
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"prepare-candidates", "--publication-root", "/root", "--admission-selector", admission}, experimentalHandoffPanicReader{}, writer, &stderr, deps)
	want := `{"schema_version":"lsp-trace.experimental-handoff-candidate-selector-receipt.v1","candidate_selector":"` + candidate + `"}` + "\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := strings.Join(events, ","); got != "store-open:/root,prepare-candidates,root-close,output" {
		t.Fatalf("events=%s", got)
	}
}

func TestExperimentalHandoffPrepareCandidatesUsageAndHelpExact(t *testing.T) {
	want := experimentalHandoffPrepareCandidatesUsage + "\n"
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{name: "missing", args: []string{"prepare-candidates"}, code: 1},
		{name: "unknown flag", args: []string{"prepare-candidates", "--unknown"}, code: 1},
		{name: "positional", args: []string{"prepare-candidates", "--publication-root", "/root", "--admission-selector", "sha256:" + strings.Repeat("a", 64), "extra"}, code: 1},
		{name: "help", args: []string{"prepare-candidates", "--help"}, code: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runExperimentalHandoffWithDependencies(context.Background(), tc.args, experimentalHandoffPanicReader{}, &stdout, &stderr, experimentalHandoffDependencies{})
			if code != tc.code || stdout.Len() != 0 || stderr.String() != want {
				t.Fatalf("code=%d stdout=%q stderr=%q want=%q", code, stdout.String(), stderr.String(), want)
			}
		})
	}
	if strings.Count(experimentalHandoffUsage, experimentalHandoffPrepareCandidatesUsage[len("usage: lsp-trace "):]) != 1 || strings.Count(usageText, experimentalHandoffPrepareCandidatesUsage[len("usage: "):]) != 1 {
		t.Fatalf("usage not represented exactly once")
	}
}

func TestExperimentalHandoffPrepareCandidatesFailuresEmitNoReceipt(t *testing.T) {
	admission := "sha256:" + strings.Repeat("a", 64)
	candidate := "sha256:" + strings.Repeat("b", 64)
	for _, tc := range []struct {
		name       string
		selector   string
		apiErr     error
		closeErr   error
		cancel     bool
		wantCode   int
		wantStderr bool
	}{
		{name: "api error", apiErr: errors.New("candidate API failed"), wantCode: 1, wantStderr: true},
		{name: "close failure after possible put", selector: candidate, closeErr: errors.New("close failed"), wantCode: 1, wantStderr: true},
		{name: "cancellation after possible put", selector: candidate, cancel: true, wantCode: 130},
		{name: "empty selector", wantCode: 1, wantStderr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var events []string
			deps := experimentalHandoffCandidateDeps(t, &events, func(context.Context, censuscontinuation.Store, string) (string, error) {
				if tc.cancel {
					cancel()
				}
				return tc.selector, tc.apiErr
			}, tc.closeErr)
			var stdout, stderr bytes.Buffer
			code := runExperimentalHandoffWithDependencies(ctx, []string{"prepare-candidates", "--publication-root", "/root", "--admission-selector", admission}, experimentalHandoffPanicReader{}, &stdout, &stderr, deps)
			if code != tc.wantCode || stdout.Len() != 0 || (stderr.Len() != 0) != tc.wantStderr {
				t.Fatalf("code=%d stdout=%q stderr=%q events=%v", code, stdout.String(), stderr.String(), events)
			}
			if got := strings.Join(events, ","); got != "store-open:/root,prepare-candidates,root-close" {
				t.Fatalf("events=%s", got)
			}
		})
	}
}

func TestExperimentalHandoffPrepareCandidatesOutputFailureHasNoAmbiguousSuccess(t *testing.T) {
	admission := "sha256:" + strings.Repeat("a", 64)
	candidate := "sha256:" + strings.Repeat("b", 64)
	var events []string
	deps := experimentalHandoffCandidateDeps(t, &events, func(context.Context, censuscontinuation.Store, string) (string, error) {
		return candidate, nil
	}, nil)
	var stderr bytes.Buffer
	written := 0
	writer := experimentalHandoffWriterFunc(func(p []byte) (int, error) {
		events = append(events, "output")
		written = len(p) / 2
		return written, errors.New("output failed")
	})
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"prepare-candidates", "--publication-root", "/root", "--admission-selector", admission}, experimentalHandoffPanicReader{}, writer, &stderr, deps)
	if code != 1 || written == 0 || !strings.Contains(stderr.String(), "output failed") || strings.Contains(stderr.String(), candidate) {
		t.Fatalf("code=%d written=%d stderr=%q", code, written, stderr.String())
	}
	if got := strings.Join(events, ","); got != "store-open:/root,prepare-candidates,root-close,output" {
		t.Fatalf("events=%s", got)
	}
}
