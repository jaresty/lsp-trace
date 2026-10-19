package sessionruntime

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

type splitBudgetChild struct {
	input      *io.PipeReader
	stdin      *io.PipeWriter
	output     *io.PipeWriter
	stdout     *io.PipeReader
	messages   []lspwire.Message
	malformed  bool
	mu         sync.Mutex
	dispatched []lspwire.Message
}

func newSplitBudgetChild(messages []lspwire.Message, malformed bool) *splitBudgetChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &splitBudgetChild{input: input, stdin: stdin, output: output, stdout: stdout, messages: messages, malformed: malformed}
	go child.serve()
	return child
}

func (c *splitBudgetChild) serve() {
	reader := lspwire.NewReader(c.input, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(c.output, lspwire.DefaultLimits())
	for {
		request, err := reader.Read()
		if err != nil {
			return
		}
		c.mu.Lock()
		c.dispatched = append(c.dispatched, request)
		c.mu.Unlock()
		if request.Method == "$/cancelRequest" {
			continue
		}
		for _, message := range c.messages {
			if len(message.ID) == 0 && message.Method == "test/matchingResponse" {
				message.Method = ""
				message.ID = append(json.RawMessage(nil), request.ID...)
			}
			if err := writer.Write(message); err != nil {
				return
			}
		}
		if c.malformed {
			_, _ = io.WriteString(c.output, "Content-Length: nope\r\n\r\n{}")
		}
	}
}

func (c *splitBudgetChild) Stdin() io.WriteCloser { return c.stdin }
func (c *splitBudgetChild) Stdout() io.ReadCloser { return c.stdout }
func (c *splitBudgetChild) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *splitBudgetChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func splitBudgetManager(t *testing.T, messages []lspwire.Message, malformed bool) (*Manager, StartResult, *splitBudgetChild) {
	t.Helper()
	child := newSplitBudgetChild(messages, malformed)
	manager, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), StartRequest{Profile: profile(t)})
	if ready := manager.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatal(ready)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	return manager, started, child
}

func splitNotification(method string) lspwire.Message {
	return lspwire.Message{JSONRPC: lspwire.Version, Method: method, Params: json.RawMessage(`{"kind":"split"}`)}
}

func splitControl(id int) lspwire.Message {
	return lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(json.RawMessage(string(rune('0' + id)))), Result: json.RawMessage(`null`)}
}

func splitTerminal() lspwire.Message {
	return lspwire.Message{JSONRPC: lspwire.Version, Method: "test/matchingResponse", Result: json.RawMessage(`{"ok":true}`)}
}

func runSplitBudget(t *testing.T, messages []lspwire.Message, malformed bool, maxMessages int, maxBytes int64, deadline time.Duration) RoundTripResult {
	t.Helper()
	manager, started, _ := splitBudgetManager(t, messages, malformed)
	return manager.RoundTrip(context.Background(), RoundTripRequest{
		SessionID: started.SessionID, Generation: started.Generation, Method: "test/splitBudget",
		Params: json.RawMessage(`{"x":1}`), Deadline: time.Now().Add(deadline), MaxMessages: maxMessages, MaxBytes: maxBytes,
	})
}

func TestRoundTripSplitMessageBudgetBoundaries(t *testing.T) {
	for _, n := range []int{1, 4, 67} {
		t.Run("notifications-plus-terminal", func(t *testing.T) {
			messages := make([]lspwire.Message, 0, n+1)
			for i := 0; i < n; i++ {
				method := "window/logMessage"
				if i < n-3 || n < 3 {
					method = "textDocument/publishDiagnostics"
				}
				messages = append(messages, splitNotification(method))
			}
			messages = append(messages, splitTerminal())
			got := runSplitBudget(t, messages, false, n, 1<<20, time.Second)
			if got.Failure != "" || got.Messages != n+1 || len(got.Notifications) != n || string(got.Result) != `{"ok":true}` {
				t.Fatalf("ASSERT_SPLIT_BUDGET_N_NOTIFICATIONS_PLUS_RESPONSE: n=%d got=%+v", n, got)
			}
		})
	}

	t.Run("notification-overflow-post-effect-no-dispatch", func(t *testing.T) {
		const n = 3
		messages := make([]lspwire.Message, n+1)
		for i := range messages {
			messages[i] = splitNotification("window/logMessage")
		}
		got := runSplitBudget(t, messages, false, n, 1<<20, time.Second)
		if got.Failure != session.ResourceExhausted || got.Messages != n+1 || len(got.Notifications) != n {
			t.Fatalf("ASSERT_SPLIT_BUDGET_NOTIFICATION_OVERFLOW_ACCOUNTED_NOT_DISPATCHED: %+v", got)
		}
	})

	t.Run("control-budget-exhausted-before-terminal", func(t *testing.T) {
		const n = 3
		messages := []lspwire.Message{splitControl(7), splitControl(8), splitControl(9), splitTerminal()}
		got := runSplitBudget(t, messages, false, n, 1<<20, time.Second)
		if got.Failure != session.ResourceExhausted || got.Messages != n+1 || len(got.Responses) != n || len(got.Result) != 0 {
			t.Fatalf("ASSERT_SPLIT_BUDGET_CONTROL_OVERFLOW_ACCOUNTED_NOT_DISPATCHED: %+v", got)
		}
	})

	t.Run("both-class-bounds-reach-two-n", func(t *testing.T) {
		const n = 4
		messages := make([]lspwire.Message, 0, 2*n)
		for i := 0; i < n; i++ {
			messages = append(messages, splitNotification("window/logMessage"))
		}
		for i := 0; i < n-1; i++ {
			messages = append(messages, splitControl(7+i))
		}
		messages = append(messages, splitTerminal())
		got := runSplitBudget(t, messages, false, n, 1<<20, time.Second)
		if got.Failure != "" || got.Messages != 2*n || len(got.Notifications) != n || len(got.Responses) != n-1 || string(got.Result) != `{"ok":true}` {
			t.Fatalf("ASSERT_SPLIT_BUDGET_BOTH_CLASSES_EXACT_TOTAL: %+v", got)
		}
	})
}

func TestRoundTripSplitMessageBudgetByteMalformedAndCancellation(t *testing.T) {
	t.Run("terminal-byte-crossing-wins", func(t *testing.T) {
		notification := splitNotification("window/logMessage")
		body, err := json.Marshal(notification)
		if err != nil {
			t.Fatal(err)
		}
		got := runSplitBudget(t, []lspwire.Message{notification, splitTerminal()}, false, 1, int64(len(body)), time.Second)
		if got.Failure != session.ResourceExhausted || got.Messages != 2 || got.Bytes <= int64(len(body)) || len(got.Result) != 0 {
			t.Fatalf("ASSERT_SPLIT_BUDGET_BYTE_CROSSING_PRECEDES_CLASS_AND_DISPATCH: %+v", got)
		}
	})

	t.Run("malformed-after-full-notification-budget", func(t *testing.T) {
		const n = 3
		messages := []lspwire.Message{splitNotification("window/logMessage"), splitNotification("window/logMessage"), splitNotification("window/logMessage")}
		got := runSplitBudget(t, messages, true, n, 1<<20, time.Second)
		if got.Failure != session.SessionPoisoned || got.Messages != n || len(got.Notifications) != n {
			t.Fatalf("ASSERT_SPLIT_BUDGET_MALFORMED_UNDECODED_NOT_CLASS_CHARGED: %+v", got)
		}
	})

	t.Run("timeout-after-full-notification-budget", func(t *testing.T) {
		const n = 3
		messages := []lspwire.Message{splitNotification("window/logMessage"), splitNotification("window/logMessage"), splitNotification("window/logMessage")}
		got := runSplitBudget(t, messages, false, n, 1<<20, 20*time.Millisecond)
		if got.Failure != session.RequestTimeout || got.Messages != n || len(got.Notifications) != n {
			t.Fatalf("ASSERT_SPLIT_BUDGET_TIMEOUT_AFTER_FULL_NOTIFICATION_CLASS: %+v", got)
		}
	})

	t.Run("cancellation-after-full-notification-budget", func(t *testing.T) {
		const n = 3
		messages := []lspwire.Message{splitNotification("window/logMessage"), splitNotification("window/logMessage"), splitNotification("window/logMessage")}
		manager, started, _ := splitBudgetManager(t, messages, false)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan RoundTripResult, 1)
		go func() {
			done <- manager.RoundTrip(ctx, RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "test/splitBudget", Params: json.RawMessage(`{"x":1}`), Deadline: time.Now().Add(time.Second), MaxMessages: n, MaxBytes: 1 << 20})
		}()
		deadline := time.Now().Add(time.Second)
		for manager.Census().Workers == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		time.Sleep(10 * time.Millisecond)
		cancel()
		got := <-done
		if got.Failure != session.RequestCancelled || got.Messages != n || len(got.Notifications) != n {
			t.Fatalf("ASSERT_SPLIT_BUDGET_CANCELLATION_AFTER_FULL_NOTIFICATION_CLASS: %+v", got)
		}
	})
}
