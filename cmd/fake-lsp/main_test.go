package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
)

func framed(t *testing.T, messages ...lspwire.Message) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	w := lspwire.NewWriter(&b, fixtureLimits)
	for _, m := range messages {
		if err := w.Write(m); err != nil {
			t.Fatal(err)
		}
	}
	return &b
}

func request(id, method, params string) lspwire.Message {
	return lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(id), Method: method, Params: json.RawMessage(params)}
}
func notification(method, params string) lspwire.Message {
	return lspwire.Message{JSONRPC: lspwire.Version, Method: method, Params: json.RawMessage(params)}
}
func readAll(t *testing.T, b *bytes.Buffer) []lspwire.Message {
	t.Helper()
	r := lspwire.NewReader(b, fixtureLimits)
	var out []lspwire.Message
	for {
		m, err := r.Read()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
}

func TestLifecycleRepliesAndBarriersUseRealFramingDeterministically(t *testing.T) {
	input := framed(t,
		request("1", "initialize", `{}`),
		notification("initialized", `{}`),
		request("2", "fixture/reply", `{"result":{"ok":true}}`),
		request("3", "fixture/barrier", `{"label":"ready"}`),
	)
	var stdout, stderr bytes.Buffer
	if code := run(input, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d", code)
	}
	got := readAll(t, &stdout)
	if len(got) != 3 {
		t.Fatalf("reply count = %d, want 3", len(got))
	}
	if string(got[0].ID) != "1" || !bytes.Contains(got[0].Result, []byte(`"name":"fake-lsp-fixture"`)) {
		t.Fatalf("initialize = %+v", got[0])
	}
	if string(got[1].Result) != `{"ok":true}` {
		t.Fatalf("reply result = %s", got[1].Result)
	}
	if string(got[2].Result) != `{"barrier":"ready"}` {
		t.Fatalf("barrier result = %s", got[2].Result)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestHangCancellationObservationAndLateReply(t *testing.T) {
	input := framed(t,
		request("7", "fixture/hang", `{}`),
		notification("$/cancelRequest", `{"id":7}`),
		notification("fixture/lateReply", `{"id":7,"result":{"late":true}}`),
	)
	var stdout, stderr bytes.Buffer
	if code := run(input, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d", code)
	}
	got := readAll(t, &stdout)
	if len(got) != 2 {
		t.Fatalf("message count = %d, want 2", len(got))
	}
	if got[0].Method != "fixture/cancelObserved" || string(got[0].Params) != `{"id":7}` {
		t.Fatalf("cancel observation = %+v", got[0])
	}
	if string(got[1].ID) != "7" || string(got[1].Result) != `{"late":true}` {
		t.Fatalf("late reply = %+v", got[1])
	}
}

func TestMalformedOutputCrashAndBoundedStderr(t *testing.T) {
	input := framed(t, notification("fixture/malformed", `{}`))
	var stdout, stderr bytes.Buffer
	if code := run(input, &stdout, &stderr); code != 0 {
		t.Fatalf("malformed exit code = %d", code)
	}
	if stdout.String() != "Content-Length: nope\r\n\r\n{}" {
		t.Fatalf("malformed output = %q", stdout.String())
	}

	input = framed(t, notification("fixture/stderr", `{"text":"`+strings.Repeat("x", 5000)+`"}`), notification("fixture/crash", `{}`))
	stdout.Reset()
	stderr.Reset()
	if code := run(input, &stdout, &stderr); code != fixtureCrashCode {
		t.Fatalf("crash exit code = %d", code)
	}
	if stderr.Len() != maxStderrBytes {
		t.Fatalf("stderr bytes = %d, want %d", stderr.Len(), maxStderrBytes)
	}
}

func TestMalformedInboundFrameIsRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(strings.NewReader("Content-Length: nope\r\n\r\n{}"), &stdout, &stderr); code != fixtureInputErrorCode {
		t.Fatalf("input error code = %d", code)
	}
	if stderr.Len() == 0 || stderr.Len() > maxStderrBytes {
		t.Fatalf("stderr bytes = %d", stderr.Len())
	}
}

func TestFixtureHasNoDescendantControl(t *testing.T) {
	input := framed(t, request("9", "fixture/descendant", `{}`))
	var stdout, stderr bytes.Buffer
	if code := run(input, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit code = %d", code)
	}
	got := readAll(t, &stdout)
	if len(got) != 1 || got[0].Error == nil || got[0].Error.Code != methodNotFoundCode {
		t.Fatalf("descendant response = %+v", got)
	}
}
