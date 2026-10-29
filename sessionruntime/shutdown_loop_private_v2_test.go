package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
)

// A single coalesced read deliberately forces bufio read-ahead. Replacing the
// Reader for each frame loses the matching response even though it is on wire.
type shutdownLoopChild struct {
	input                                      *io.PipeReader
	stdin                                      *io.PipeWriter
	output                                     *io.PipeWriter
	stdout                                     *io.PipeReader
	done                                       chan struct{}
	mode                                       string
	teardowns, closes, shutdowns, exits, other atomic.Int32
	resource                                   managedprocess.ResourceKind
	death                                      managedprocess.DeathKind
	reap                                       managedprocess.ReapKind
}

func newShutdownLoopChild(mode string) *shutdownLoopChild {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	c := &shutdownLoopChild{input: in, stdin: stdin, output: out, stdout: stdout, done: make(chan struct{}), mode: mode, resource: managedprocess.ResourcesClosed, death: managedprocess.DeathExited, reap: managedprocess.ReapComplete}
	go func() {
		defer close(c.done)
		reader := lspwire.NewReader(in, lspwire.DefaultLimits())
		msg, err := reader.Read()
		if err != nil {
			return
		}
		if msg.Method != "shutdown" {
			c.other.Add(1)
			return
		}
		c.shutdowns.Add(1)
		var b bytes.Buffer
		w := lspwire.NewWriter(&b, lspwire.DefaultLimits())
		notification := func() {
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "fixture", Params: json.RawMessage(`{"content":"withheld"}`)})
		}
		switch mode {
		case "notification":
			notification()
		case "notification-array":
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "fixture", Params: json.RawMessage(`[]`)})
		case "notification-empty":
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "fixture"})
		case "coalesced":
			notification()
			notification()
		case "count-at":
			for i := 0; i < 3; i++ {
				notification()
			}
		case "bytes-at", "bytes-plus":
			body := `{"jsonrpc":"2.0","method":"fixture"}`
			response := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":null}`, msg.ID)
			responseFrame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(response), response)
			prefix := fmt.Sprintf("Content-Length: %d\r\nX-Padding: ", len(body))
			padding := 512 - len(responseFrame) - len(prefix) - len("\r\n\r\n") - len(body)
			if mode == "bytes-plus" {
				padding++
			}
			fmt.Fprintf(&b, "%s%s\r\n\r\n%s", prefix, strings.Repeat("x", padding), body)
		case "count":
			for i := 0; i < 4; i++ {
				notification()
			}
		case "bytes":
			// Whitespace and extension headers count on wire, not after re-marshaling.
			body := `{"jsonrpc":"2.0","method":"fixture"}`
			fmt.Fprintf(&b, "Content-Length: %d\r\nX-Padding: %s\r\n\r\n%s", len(body), strings.Repeat("x", 440), body)
		case "wrong-id":
			msg.ID = json.RawMessage(`999`)
		case "request":
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`77`), Method: "fixture", Params: json.RawMessage(`{}`)})
		case "notification-scalar", "notification-null", "malformed-json", "wrong-version", "invalid-message":
			body := `{"jsonrpc":"2.0","method":"fixture","params":7}`
			switch mode {
			case "notification-null":
				body = `{"jsonrpc":"2.0","method":"fixture","params":null}`
			case "malformed-json":
				body = `{`
			case "wrong-version":
				body = `{"jsonrpc":"1.0","method":"fixture"}`
			case "invalid-message":
				body = `{"jsonrpc":"2.0"}`
			}
			fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n%s", len(body), body)
		case "malformed":
			b.WriteString("Content-Length: nope\r\n\r\n")
		case "partial-timeout":
			_, _ = io.WriteString(out, "Content-Length: 50\r\n\r\n{")
			_, _ = reader.Read()
			return
		case "timeout":
			// Block on the client's input until its existing teardown interrupts it.
			_, _ = reader.Read()
			return
		}
		if mode == "error" {
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Error: &lspwire.RPCError{Code: -32603, Message: "withheld"}})
		} else {
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(`null`)})
		}
		if mode == "exit-failed" {
			_ = in.Close()
		}
		if _, err := out.Write(b.Bytes()); err != nil {
			return
		}
		for {
			msg, err = reader.Read()
			if err != nil {
				return
			}
			if msg.Method == "exit" {
				c.exits.Add(1)
			} else {
				c.other.Add(1)
			}
		}
	}()
	return c
}
func (c *shutdownLoopChild) Stdin() io.WriteCloser { return c.stdin }
func (c *shutdownLoopChild) Stdout() io.ReadCloser { return c.stdout }
func (c *shutdownLoopChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.teardowns.Add(1)
	_ = c.input.Close()
	_ = c.output.Close()
	<-c.done
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: c.death, Reap: managedprocess.ReapObservation{Kind: c.reap}}}
}
func (c *shutdownLoopChild) Close() managedprocess.ResourceObservation {
	c.closes.Add(1)
	_ = c.stdin.Close()
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: c.resource}
}
func shutdownLoopRun(t *testing.T, c *shutdownLoopChild) (OperationSnapshot, *Manager) {
	t.Helper()
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 4}, Wire: lspwire.Limits{MaxBodyBytes: 512, MaxHeaderBytes: 1024}, Starter: oneChildStarter{c}})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	op := m.Stop(context.Background(), s.SessionID, "shutdown-loop")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := m.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	terminal, ok := m.Operation(op.IntentID)
	if !ok {
		t.Fatal("missing terminal")
	}
	return terminal, m
}
func TestShutdownLoopPrivateV2(t *testing.T) {
	for _, mode := range []string{"exact", "notification", "coalesced", "notification-array", "notification-empty", "count-at", "bytes-at", "wrong-id", "error", "request", "malformed", "count", "bytes", "bytes-plus", "timeout", "exit-failed", "notification-scalar", "notification-null", "malformed-json", "wrong-version", "invalid-message"} {
		t.Run(mode, func(t *testing.T) {
			c := newShutdownLoopChild(mode)
			if mode == "exit-failed" {
				c.death = managedprocess.DeathSignaled
			}
			op, m := shutdownLoopRun(t, c)
			success := mode == "exact" || mode == "notification" || mode == "coalesced" || mode == "notification-array" || mode == "notification-empty" || mode == "count-at" || mode == "bytes-at"
			if (op.State == OperationComplete) != success {
				t.Fatalf("ASSERT_SHUTDOWN_LOOP_ACCEPTANCE[%s]: got=%s failure=%s wantSuccess=%v", mode, op.State, op.Failure, success)
			}
			wantExit := int32(0)
			if success {
				wantExit = 1
			}
			if c.shutdowns.Load() != 1 || c.exits.Load() != wantExit || c.other.Load() != 0 || c.teardowns.Load() != 1 || c.closes.Load() != 1 || m.Census().Workers != 0 {
				t.Fatalf("ASSERT_SHUTDOWN_LOOP_NO_EXTRA_EFFECTS: shutdown=%d exit=%d other=%d teardown=%d close=%d", c.shutdowns.Load(), c.exits.Load(), c.other.Load(), c.teardowns.Load(), c.closes.Load())
			}
			facts, present := op.CleanupObservationPrivateV2()
			wantRead := ShutdownMatchedPrivateV2
			switch mode {
			case "wrong-id", "error", "request", "notification-scalar", "notification-null":
				wantRead = ShutdownUnexpectedPrivateV2
			case "malformed", "malformed-json", "wrong-version", "invalid-message":
				wantRead = ShutdownReadFailedPrivateV2
			case "count":
				wantRead = ShutdownCountLimitPrivateV2
			case "bytes", "bytes-plus":
				wantRead = ShutdownByteLimitPrivateV2
			case "timeout":
				wantRead = ShutdownTimeoutPrivateV2
			}
			if !present || facts.ShutdownWrite != CleanupSucceededPrivateV2 || facts.ShutdownRead != wantRead || facts.ReadFrames > 4 || facts.ReadBytes > 512 || facts.ResourceClose != managedprocess.ResourcesClosed || facts.Reap != managedprocess.ReapComplete || !facts.StderrDrainAssumed {
				t.Fatalf("ASSERT_SHUTDOWN_DIAGNOSTIC_DISTINCTIONS[%s]: %+v present=%v", mode, facts, present)
			}
			if mode == "bytes-at" && facts.ReadBytes != 512 {
				t.Fatalf("original wire bytes not counted: %+v", facts)
			}
			if mode == "exit-failed" && facts.ExitWrite != CleanupFailedPrivateV2 {
				t.Fatal("exit write failure lost")
			}
			facts.ReadFrames = -1
			again, _ := op.CleanupObservationPrivateV2()
			if again.ReadFrames < 0 {
				t.Fatal("accessor aliases immutable snapshot")
			}
			raw, _ := json.Marshal(op)
			if bytes.Contains(raw, []byte("ShutdownRead")) || bytes.Contains(raw, []byte("cleanupPrivateV2")) {
				t.Fatal("ordinary Operation JSON widened")
			}
			t.Logf("ASSERT_SHUTDOWN_LOOP_ACCEPTANCE[%s]: PASS", mode)
		})
	}
}
