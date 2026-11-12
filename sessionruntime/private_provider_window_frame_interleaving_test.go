package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

// These are scripted transport observations, not identities of live messages.
// The whole burst is encoded before delivery. A refused iteration says
// nothing about physical read-ahead by the framing reader or the OS.
type frameInterleavingScript struct {
	notifications int
	responseID    json.RawMessage
	burst         []byte
}

type frameInterleavingConfig struct {
	notifications, unmatched int
	fault                    string // test-only closed cases: malformed, request
}

func frameInterleavingServe(c *windowPipeChild, config frameInterleavingConfig, scripted chan<- frameInterleavingScript) {
	count := config.notifications
	defer close(c.done)
	defer c.serverInput.Close()
	defer c.serverOutput.Close()
	reader := lspwire.NewReader(c.serverInput, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(c.serverOutput, lspwire.DefaultLimits())
	for {
		msg, err := reader.Read()
		if err != nil || msg.Method == "exit" {
			return
		}
		if len(msg.ID) == 0 {
			continue
		}
		raw := json.RawMessage(`null`)
		switch msg.Method {
		case "initialize":
			raw = json.RawMessage(`{"capabilities":{"documentSymbolProvider":true,"definitionProvider":true}}`)
		case "textDocument/documentSymbol":
			var burst bytes.Buffer
			encoded := lspwire.NewWriter(&burst, lspwire.DefaultLimits())
			for i := 0; i < count; i++ {
				if encoded.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress", Params: json.RawMessage(fmt.Sprintf(`{"fixtureOrdinal":%d}`, i))}) != nil {
					return
				}
			}
			for i := 0; i < config.unmatched; i++ {
				if encoded.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(fmt.Sprint(1000 + i)), Result: json.RawMessage(`null`)}) != nil {
					return
				}
			}
			raw = json.RawMessage(`[{"name":"fixture","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`)
			if encoded.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: raw}) != nil {
				return
			}
			switch config.fault {
			case "malformed":
				burst.Reset()
				burst.WriteString("Content-Length: 1\r\n\r\n{")
			case "request":
				burst.Reset()
				if encoded.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`777`), Method: "private-canary/server-request", Params: json.RawMessage(`{}`)}) != nil {
					return
				}
			}
			scripted <- frameInterleavingScript{count, append(json.RawMessage(nil), msg.ID...), append([]byte(nil), burst.Bytes()...)}
			// Closing the owned pipe unblocks this write if an iteration is refused.
			if _, err := c.serverOutput.Write(burst.Bytes()); err != nil {
				return
			}
			continue
		}
		if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: raw}) != nil {
			return
		}
	}
}

func frameInterleavingSetup(t *testing.T, count int) (*Manager, StartRequest, context.Context, *windowPipeChild, <-chan frameInterleavingScript) {
	t.Helper()
	return frameInterleavingSetupConfig(t, frameInterleavingConfig{notifications: count})
}

func frameInterleavingSetupConfig(t *testing.T, config frameInterleavingConfig) (*Manager, StartRequest, context.Context, *windowPipeChild, <-chan frameInterleavingScript) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "fixture.go")
	if err := os.WriteFile(path, []byte("package p\nfunc f() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c := newWindowPipeChild((&url.URL{Scheme: "file", Path: path}).String(), "")
	starter := &windowPipeStarter{child: c}
	m, err := New(Config{Limits: Limits{MaxSessions: 2, MaxRequests: 4, MaxChildren: 16, MaxCancels: 8, MaxTombstones: 16, MaxObservations: 64, MaxOperations: 8}, Starter: starter})
	if err != nil {
		t.Fatal(err)
	}
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	start := StartRequest{Profile: runtimeprofile.Resolve(selector), Process: managedprocess.Spec{Path: "controlled-io.Pipe", Dir: root}, LanguageID: "go", Deadline: time.Now().Add(5 * time.Second)}
	envelope, err := m.PrivateProviderWindowEnvelope(start, PrivateProviderWindowPlan{Definitions: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := m.PrivateProviderWindowContext(context.Background(), envelope, start.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	scripted := make(chan frameInterleavingScript, 1)
	go frameInterleavingServe(c, config, scripted)
	t.Cleanup(func() {
		w := privateProviderWindowFrom(ctx)
		w.cancel()
		_ = c.input.Close()
		_ = c.output.Close()
		_ = c.serverInput.Close()
		_ = c.serverOutput.Close()
		<-c.done
		_, _ = m.FinishPrivateProviderWindow(ctx)
		select {
		case <-w.finishDone:
		case <-time.After(2 * time.Second):
			t.Error("controlled cleanup did not join")
		}
	})
	return m, start, ctx, c, scripted
}

func TestPrivateProviderWindowFrameInterleavingTransport(t *testing.T) {
	for _, count := range []int{3, 4} {
		t.Run(fmt.Sprintf("notifications=%d", count), func(t *testing.T) {
			m, start, ctx, c, scripted := frameInterleavingSetup(t, count)
			st := windowTestReady(t, m, start, ctx)
			w := privateProviderWindowFrom(ctx)
			supplied := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: c.uri, LanguageID: "go"})
			if supplied.Failure != "" {
				t.Fatalf("prepare: %+v", supplied)
			}
			w.mu.Lock()
			before := w.frames
			bound := w.spec.bounds.Frames
			w.mu.Unlock()
			var got RoundTripResult
			var leaseCancelled bool
			err := m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(lease context.Context) error {
				got = m.RoundTrip(lease, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, c.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 65536})
				leaseCancelled = lease.Err() != nil
				return nil
			})
			script := <-scripted
			if script.notifications != count || string(script.responseID) != fmt.Sprint(got.Key.ID) || len(script.burst) == 0 {
				t.Fatalf("script/key mismatch: %+v key=%+v", script, got.Key)
			}
			if len(got.Notifications) != count {
				t.Fatalf("notification selection: %d want %d", len(got.Notifications), count)
			}
			for i, n := range got.Notifications {
				if n.Method != "$/progress" || string(n.Params) != fmt.Sprintf(`{"fixtureOrdinal":%d}`, i) {
					t.Fatalf("notification %d: %+v", i, n)
				}
			}
			w.mu.Lock()
			exchange, frames, failed := w.exchangeFrames, w.frames, w.failed
			w.mu.Unlock()
			if exchange != uint64(count+1) || frames-before != uint64(count+1) || frames >= bound {
				t.Fatalf("read admission: exchange=%d delta=%d global=%d/%d", exchange, frames-before, frames, bound)
			}
			if got.Failure != "" || len(got.Result) == 0 || !got.providerTransaction.selectedResponseObserved || failed || leaseCancelled || err != nil {
				t.Fatalf("boundary: failure=%s result=%s selected=%t failed=%t cancelled=%t leaseErr=%v", got.Failure, got.Result, got.providerTransaction.selectedResponseObserved, failed, leaseCancelled, err)
			}
			if got.notificationMessages != count || got.controlMessages != 1 {
				t.Fatalf("class accounting: notifications=%d controls=%d", got.notificationMessages, got.controlMessages)
			}
			t.Logf("script notifications=%d matchingID=%s selected=%t failure=%s admittedIterations=%d global=%d/%d leaseCancelled=%t leaseErr=%v", count, script.responseID, got.providerTransaction.selectedResponseObserved, got.Failure, exchange, frames, bound, leaseCancelled, err)
		})
	}
}

func TestPrivateProviderWindowFrameInterleavingPredicates(t *testing.T) {
	for _, kind := range []string{"live", "per-exchange", "global", "expired", "failed", "cancelled", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			w := &privateProviderWindow{ctx: ctx, deadline: time.Now().Add(time.Minute), spec: &privateProviderWindowSpec{bounds: PrivateProviderWindowBounds{Frames: 16, FramesPerExchange: 4}}, exchangeFrames: 1, frames: 2}
			switch kind {
			case "per-exchange":
				w.exchangeFrames = 4
				w.frames = 4
			case "global":
				w.frames = 16
			case "expired":
				w.expired = true
			case "failed":
				w.failed = true
			case "cancelled":
				cancel()
			case "deadline":
				w.deadline = time.Now().Add(-time.Second)
			}
			beforeExchange, beforeGlobal := w.exchangeFrames, w.frames
			admitted := w.frame()
			if admitted != (kind == "live") {
				t.Fatalf("frame predicate %s admitted=%t", kind, admitted)
			}
			if !admitted && (w.exchangeFrames != beforeExchange || w.frames != beforeGlobal || !w.failed) {
				t.Fatal("refusal mutated counters or did not fail window")
			}
			t.Logf("predicate=%s admitted=%t exchangeBefore=%d globalBefore=%d expired=%t failed=%t", kind, admitted, beforeExchange, beforeGlobal, w.expired, w.failed)
		})
	}
}

func TestPrivateProviderWindowFrameInterleavingExchangeReset(t *testing.T) {
	w := &privateProviderWindow{ctx: context.Background(), deadline: time.Now().Add(time.Minute), spec: &privateProviderWindowSpec{bounds: PrivateProviderWindowBounds{Frames: 16, FramesPerExchange: 4}}, schedule: []string{"initialize", "initialized"}}
	if !w.enter("initialize") {
		t.Fatal("first legitimate exchange")
	}
	for i := 0; i < 4; i++ {
		if !w.frame() {
			t.Fatal("first four frames")
		}
	}
	if !w.enter("initialized") || w.exchangeFrames != 0 || w.frames != 4 || !w.frame() || w.frames != 5 || w.exchangeFrames != 1 {
		t.Fatal("legitimate exchange must reset only local count")
	}
	t.Log("legitimate next exchange resets local count and preserves global debit")
}

func TestPrivateProviderWindowFrameInterleavingClassBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                                              string
		notifications, unmatched, policy, globalRemaining int
		wantN, wantC                                      int
		wantFrames                                        uint64
		refuse                                            bool
	}{
		{name: "four-notifications", notifications: 4, wantN: 4, wantC: 1, wantFrames: 5},
		{name: "fifth-notification", notifications: 5, wantN: 5, wantFrames: 5, refuse: true},
		{name: "fifth-control", unmatched: 4, wantC: 5, wantFrames: 5, refuse: true},
		{name: "fifth-unmatched-control", unmatched: 5, wantC: 5, wantFrames: 5, refuse: true},
		{name: "mixed-eight-matching", notifications: 4, unmatched: 3, wantN: 4, wantC: 4, wantFrames: 8},
		{name: "mixed-eight-unmatched-ninth-refused", notifications: 4, unmatched: 4, wantN: 4, wantC: 4, wantFrames: 8, refuse: true},
		{name: "global-before-local", notifications: 3, globalRemaining: 2, wantN: 2, wantFrames: 2, refuse: true},
		{name: "positive-lower-success", notifications: 1, policy: 1, wantN: 1, wantC: 1, wantFrames: 2},
		{name: "positive-lower-refusal", notifications: 3, policy: 2, wantN: 3, wantFrames: 3, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, start, ctx, c, scripted := frameInterleavingSetupConfig(t, frameInterleavingConfig{notifications: tc.notifications, unmatched: tc.unmatched})
			st := windowTestReady(t, m, start, ctx)
			if got := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: c.uri, LanguageID: "go"}); got.Failure != "" {
				t.Fatal(got.Failure)
			}
			w := privateProviderWindowFrom(ctx)
			w.mu.Lock()
			if tc.globalRemaining != 0 {
				w.frames = w.spec.bounds.Frames - uint64(tc.globalRemaining)
			}
			before := w.frames
			w.mu.Unlock()
			var got RoundTripResult
			var cancelled bool
			err := m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(lease context.Context) error {
				got = m.RoundTrip(lease, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, c.uri)), Deadline: start.Deadline, MaxMessages: 4, PrivateNotificationMaxMessages: tc.policy, MaxBytes: 65536})
				cancelled = lease.Err() != nil
				return nil
			})
			<-scripted // Whole-burst source record, not evidence of selected/read frames.
			w.mu.Lock()
			exchange, delta, failed := w.exchangeFrames, w.frames-before, w.failed
			w.mu.Unlock()
			if got.notificationMessages != tc.wantN || got.controlMessages != tc.wantC || delta != tc.wantFrames || exchange != tc.wantFrames {
				t.Fatalf("ASSERT_CLASS_GLOBAL: N=%d C=%d local=%d debit=%d want N=%d C=%d frames=%d", got.notificationMessages, got.controlMessages, exchange, delta, tc.wantN, tc.wantC, tc.wantFrames)
			}
			if tc.refuse {
				if got.Failure != session.ResourceExhausted || got.providerTransaction.selectedResponseObserved || len(got.Result) != 0 || !cancelled {
					t.Fatalf("ASSERT_CLASS_REFUSAL: %+v failed=%t cancelled=%t", got, failed, cancelled)
				}
			} else if got.Failure != "" || !got.providerTransaction.selectedResponseObserved || len(got.Result) == 0 || failed || cancelled || err != nil {
				t.Fatalf("ASSERT_CLASS_SUCCESS: %+v failed=%t cancelled=%t err=%v", got, failed, cancelled, err)
			}
			t.Logf("ASSERT_CLASS_GLOBAL PASS N=%d C=%d local=%d globalDebit=%d refusal=%t", got.notificationMessages, got.controlMessages, exchange, delta, tc.refuse)
		})
	}
}

// Wakeup-channel replacement is an owned wrapper effect, not pending/provider admission.
func frameInterleavingAdmissionState(w *privateProviderWindow) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return fmt.Sprintf("%d/%d/%d/%d/%d/%t/%t", w.next, w.remaining, w.frames, w.exchangeFrames, w.preparations, w.expired, w.failed)
}

func TestPrivateProviderWindowFrameInterleavingNotificationAdmission(t *testing.T) {
	for _, policy := range []int{-2, -1, 5} {
		t.Run(fmt.Sprint(policy), func(t *testing.T) {
			m, start, ctx, c, _ := frameInterleavingSetup(t, 0)
			st := windowTestReady(t, m, start, ctx)
			if got := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: c.uri, LanguageID: "go"}); got.Failure != "" {
				t.Fatal(got.Failure)
			}
			w := privateProviderWindowFrom(ctx)
			before := frameInterleavingAdmissionState(w)
			var writes atomic.Int64
			c.beforeWrite = func(n int) { writes.Add(int64(n)) }
			m.mu.Lock()
			pending := len(m.sessions[st.SessionID].requests)
			m.mu.Unlock()
			var got RoundTripResult
			err := m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(lease context.Context) error {
				got = m.RoundTrip(lease, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Deadline: start.Deadline, MaxMessages: 4, PrivateNotificationMaxMessages: policy, MaxBytes: 65536})
				return nil
			})
			m.mu.Lock()
			afterPending := len(m.sessions[st.SessionID].requests)
			m.mu.Unlock()
			if got.Failure == "" || got.Key.ID != 0 || writes.Load() != 0 || pending != afterPending || before != frameInterleavingAdmissionState(w) || err != nil {
				t.Fatalf("ASSERT_NOTIFICATION_ADMISSION_ZERO_EFFECT: policy=%d failure=%s key=%+v writes=%d pending=%d/%d err=%v", policy, got.Failure, got.Key, writes.Load(), pending, afterPending, err)
			}
			t.Log("ASSERT_NOTIFICATION_ADMISSION_ZERO_EFFECT PASS")
		})
	}
}

func TestPrivateProviderWindowFrameInterleavingAllSevenBounds(t *testing.T) {
	for mask := 1; mask < 8; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			plan := PrivateProviderWindowPlan{Calls: mask&1 != 0, References: mask&2 != 0, Definitions: mask&4 != 0}
			_, _, ctx, _ := windowTestSetup(t, plan, "")
			got := privateProviderWindowFrom(ctx).spec.bounds
			exchanges, preparations := uint64(3), uint64(1)
			if plan.Calls {
				exchanges += 3
				preparations++
			}
			if plan.References {
				exchanges++
			}
			if plan.Definitions {
				exchanges++
			}
			// Full HEAD constructor budget, with only the declared local maximum changed.
			frame := uint64(4096 + 128<<10)
			want := PrivateProviderWindowBounds{SourceBytes: 16 << 10, SourceReadBytes: preparations * ((16 << 10) + 1), SourcePreparations: preparations, HeaderBytes: 4096, BodyBytes: 128 << 10, FrameBytes: frame, Frames: 4 * exchanges, FramesPerExchange: 8, RequestBytes: (exchanges + preparations + 2) * (frame + 1), ResponseBytes: 4 * exchanges * (frame + 4096 + 1), StderrDeliveryBytes: 64 << 10, CensusBytes: 4096, CensusLines: 128, CensusMembers: 64, CensusInvocations: 2}
			want.WireBytes = want.RequestBytes + want.ResponseBytes
			if got != want {
				t.Fatalf("ASSERT_UNCHANGED_GLOBAL_BOUNDS: got=%+v want=%+v", got, want)
			}
			before := want
			before.FramesPerExchange = 4
			b, _ := json.Marshal(before)
			a, _ := json.Marshal(got)
			t.Logf("ASSERT_UNCHANGED_GLOBAL_BOUNDS PASS C=%t R=%t D=%t before=%s after=%s", plan.Calls, plan.References, plan.Definitions, b, a)
		})
	}
}

func TestPrivateProviderWindowFrameInterleavingTransactionPredicates(t *testing.T) {
	for _, kind := range []string{"eight", "global", "expired", "failed", "cancelled", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			w := &privateProviderWindow{ctx: ctx, deadline: time.Now().Add(time.Minute), spec: &privateProviderWindowSpec{bounds: PrivateProviderWindowBounds{Frames: 16, FramesPerExchange: 8}}}
			switch kind {
			case "global":
				w.frames = 16
			case "expired":
				w.expired = true
			case "failed":
				w.failed = true
			case "cancelled":
				cancel()
			case "deadline":
				w.deadline = time.Now().Add(-time.Second)
			}
			if kind == "eight" {
				for i := 0; i < 8; i++ {
					if !w.transactionFrame() {
						t.Fatalf("ASSERT_TRANSACTION_EIGHT: refused %d", i+1)
					}
				}
			}
			beforeLocal, beforeGlobal := w.exchangeFrames, w.frames
			if w.transactionFrame() || w.exchangeFrames != beforeLocal || w.frames != beforeGlobal || !w.failed {
				t.Fatal("ASSERT_TRANSACTION_REFUSAL_NO_DEBIT")
			}
			t.Logf("ASSERT_TRANSACTION_REFUSAL_NO_DEBIT PASS kind=%s local=%d global=%d", kind, beforeLocal, beforeGlobal)
		})
	}
}
