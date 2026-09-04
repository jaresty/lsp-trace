package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func helperRegistration(mode string) Registration {
	return Registration{ID: "test", Path: os.Args[0], Args: []string{"-test.run=TestProviderHelper", "--", mode}, Env: append(os.Environ(), "GO_WANT_PROVIDER_HELPER=1")}
}

func testRuntime(t *testing.T, mode string) *Runtime {
	t.Helper()
	r := NewRegistry()
	if err := r.Register(helperRegistration(mode)); err != nil {
		t.Fatal(err)
	}
	return NewRuntime(r)
}

func testLimits() Limits {
	return Limits{RequestBytes: 1024, ResponseBytes: 1024, ProtocolMessages: 1, StderrBytes: 4, WallTime: 10 * time.Second, TerminationGrace: 20 * time.Millisecond}
}

func TestRegistrationRejectsInvalidAndDuplicate(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Registration{}); err == nil {
		t.Fatal("P1 registration validation: empty registration accepted")
	}
	reg := helperRegistration("echo")
	if err := r.Register(reg); err != nil {
		t.Fatalf("P1 registration validation: valid registration rejected: %v", err)
	}
	if err := r.Register(reg); err == nil {
		t.Fatal("P1 registration validation: duplicate registration accepted")
	}
	got, ok := r.Resolve(reg.ID)
	if !ok || got.Path != reg.Path {
		t.Fatal("P1 registration validation: registered provider did not resolve")
	}
	got.Args[0] = "mutated"
	again, _ := r.Resolve(reg.ID)
	if again.Args[0] == "mutated" {
		t.Fatal("P1 registration validation: resolution leaked mutable registration storage")
	}
}

func TestUnknownProviderFailsClosed(t *testing.T) {
	receipt := NewRuntime(NewRegistry()).Execute(context.Background(), "missing", json.RawMessage(`{"x":1}`), testLimits())
	if receipt.Failure == nil || receipt.Failure.Kind != ProviderUnknown {
		t.Fatalf("P2 host provisioning: got failure %#v", receipt.Failure)
	}
}

func TestStrictSingleFrameRoundTrip(t *testing.T) {
	receipt := testRuntime(t, "echo").Execute(context.Background(), "test", json.RawMessage(`{"x":1}`), testLimits())
	if receipt.Failure != nil {
		t.Fatalf("P3 strict framing: %v", receipt.Failure)
	}
	if string(receipt.Response) != `{"ok":true}` || receipt.Messages != 1 {
		t.Fatalf("P3 strict framing: response=%s messages=%d", receipt.Response, receipt.Messages)
	}
}

func TestMalformedAndMultipleFramesFailProtocol(t *testing.T) {
	for _, mode := range []string{"malformed", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			receipt := testRuntime(t, mode).Execute(context.Background(), "test", json.RawMessage(`{}`), testLimits())
			if receipt.Failure == nil || receipt.Failure.Kind != ProtocolFailed {
				t.Fatalf("P3 strict framing: got %#v", receipt.Failure)
			}
		})
	}
}

func TestIndependentByteAndMessageLimits(t *testing.T) {
	limits := testLimits()
	limits.RequestBytes = 1
	if got := testRuntime(t, "echo").Execute(context.Background(), "test", json.RawMessage(`{}`), limits); got.Failure == nil || got.Failure.Kind != LimitExceeded {
		t.Fatalf("P4 request limit: %#v", got.Failure)
	}
	limits = testLimits()
	limits.ResponseBytes = 4
	if got := testRuntime(t, "echo").Execute(context.Background(), "test", json.RawMessage(`{}`), limits); got.Failure == nil || got.Failure.Kind != LimitExceeded {
		t.Fatalf("P4 response limit: %#v", got.Failure)
	}
	limits = testLimits()
	limits.ProtocolMessages = 0
	if got := testRuntime(t, "echo").Execute(context.Background(), "test", json.RawMessage(`{}`), limits); got.Failure == nil || got.Failure.Kind != LimitExceeded {
		t.Fatalf("P4 message limit: %#v", got.Failure)
	}
}

func TestCancellationAndTimeoutAreDistinct(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := testRuntime(t, "block").Execute(ctx, "test", json.RawMessage(`{}`), testLimits()); got.Failure == nil || got.Failure.Kind != Canceled {
		t.Fatalf("P5 caller cancellation: %#v", got.Failure)
	}
	limits := testLimits()
	limits.WallTime = 15 * time.Millisecond
	if got := testRuntime(t, "block").Execute(context.Background(), "test", json.RawMessage(`{}`), limits); got.Failure == nil || got.Failure.Kind != TimedOut {
		t.Fatalf("P5 wall timeout: %#v", got.Failure)
	}
}

func TestStderrAccountingIsBounded(t *testing.T) {
	got := testRuntime(t, "stderr").Execute(context.Background(), "test", json.RawMessage(`{}`), testLimits())
	if got.Failure != nil {
		t.Fatalf("P6 stderr accounting: %v", got.Failure)
	}
	if string(got.Stderr.Bytes) != "abcd" || got.Stderr.TotalBytes != 6 || !got.Stderr.Truncated {
		t.Fatalf("P6 stderr accounting: %#v", got.Stderr)
	}
}

func TestEveryReturnReapsChild(t *testing.T) {
	limits := testLimits()
	limits.WallTime = 15 * time.Millisecond
	got := testRuntime(t, "block").Execute(context.Background(), "test", json.RawMessage(`{}`), limits)
	if !got.Terminated || !got.Reaped {
		t.Fatalf("P7 termination and reap: terminated=%v reaped=%v", got.Terminated, got.Reaped)
	}
}

func TestAbnormalExitIsTransportFailure(t *testing.T) {
	got := testRuntime(t, "abnormal").Execute(context.Background(), "test", json.RawMessage(`{}`), testLimits())
	if got.Failure == nil || got.Failure.Kind != AbnormalExit {
		t.Fatalf("P8 failure projection: %#v", got.Failure)
	}
	if len(got.Response) != 0 {
		t.Fatalf("P8 authority boundary: abnormal exit retained successful response %s", got.Response)
	}
}

func TestProviderHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PROVIDER_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if mode == "block" {
		for {
			time.Sleep(time.Second)
		}
	}
	if mode == "malformed" {
		_, _ = io.WriteString(os.Stdout, "bad\r\n\r\n{}")
		os.Exit(0)
	}
	if _, err := readHelperFrame(bufio.NewReader(os.Stdin)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if mode == "abnormal" {
		os.Exit(7)
	}
	if mode == "stderr" {
		_, _ = io.WriteString(os.Stderr, "abcdef")
	}
	body := `{"ok":true}`
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
	if mode == "duplicate" {
		_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}
	os.Exit(0)
}

func readHelperFrame(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(line, "\r\n") {
		return nil, fmt.Errorf("bad line ending")
	}
	parts := strings.Split(strings.TrimSuffix(line, "\r\n"), ": ")
	if len(parts) != 2 || parts[0] != "Content-Length" {
		return nil, fmt.Errorf("bad header")
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}
	blank, err := r.ReadString('\n')
	if err != nil || blank != "\r\n" {
		return nil, fmt.Errorf("bad separator")
	}
	body := make([]byte, n)
	_, err = io.ReadFull(r, body)
	return body, err
}
