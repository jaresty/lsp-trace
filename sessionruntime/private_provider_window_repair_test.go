package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/runtimeprofile"
)

// Real ordinary Start/readiness uses the same io.Pipe protocol fixture as the
// positive private controls. The counter counts backend writes, not callbacks
// substituted for runtime work.
func repairOrdinary(t *testing.T, m *Manager, start StartRequest, sameManager bool) (*Manager, StartResult, *windowPipeChild, *atomic.Int64) {
	t.Helper()
	root := start.Profile.Workspace().String()
	if sameManager {
		var err error
		root, err = filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
		if err != nil {
			t.Fatal(err)
		}
		start.Profile = runtimeprofile.Resolve(selector)
		start.Process.Dir = root
		if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\nfunc f() {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	child := newWindowPipeChild(uri, "")
	writes := &atomic.Int64{}
	child.beforeWrite = func(int) { writes.Add(1) }
	if !sameManager {
		var err error
		m, err = New(Config{Limits: m.limits, Starter: &windowPipeStarter{child: child}})
		if err != nil {
			t.Fatal(err)
		}
	} else {
		s := m.starter.(*windowPipeStarter)
		previous := s.child
		s.child = child
		defer func() { s.child = previous }()
	}
	go child.serve()
	t.Cleanup(func() {
		_ = child.input.Close()
		_ = child.output.Close()
		_ = child.serverInput.Close()
		_ = child.serverOutput.Close()
		<-child.done
	})
	st := windowTestReady(t, m, start, context.Background())
	return m, st, child, writes
}

func TestPrivateProviderWindowRepairForeignPreparation(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{false: "wrong-manager-same-scalars", true: "wrong-session-same-manager"}[same], func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			windowTestReady(t, m, start, ctx)
			b, st, child, writes := repairOrdinary(t, m, start, same)
			w := privateProviderWindowFrom(ctx)
			if !same && (st.SessionID != w.start.SessionID || st.Generation != w.start.Generation) {
				t.Fatal("fixture must share session/gen")
			}
			before := writes.Load()
			w.mu.Lock()
			next, remaining, preps := w.next, w.remaining, w.preparations
			w.mu.Unlock()
			got := b.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: child.uri, LanguageID: "go"})
			w.mu.Lock()
			untouched := w.next == next && w.remaining == remaining && w.preparations == preps && !w.failed
			w.mu.Unlock()
			if got.Failure == "" || got.Supply != nil || writes.Load() != before || !untouched {
				t.Error("ASSERT_REPAIR_FOREIGN_PREP_ZERO_EFFECT")
			}
			if untouched {
				windowTestQueries(t, m, start, ctx, s.child, PrivateProviderWindowPlan{Definitions: true})
				c, e := m.FinishPrivateProviderWindow(ctx)
				if e != nil || !c.Completed {
					t.Error("ASSERT_REPAIR_FOREIGN_REJECTION_NOT_POISONED")
				}
			}
		})
	}
}

func TestPrivateProviderWindowRepairForeignTransaction(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	st := windowTestReady(t, m, start, ctx)
	if got := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"}); got.Failure != "" {
		t.Fatal(got.Failure)
	}
	b, bs, child, writes := repairOrdinary(t, m, start, false)
	before := writes.Load()
	w := privateProviderWindowFrom(ctx)
	w.mu.Lock()
	next, remaining := w.next, w.remaining
	w.mu.Unlock()
	got := b.RoundTrip(ctx, RoundTripRequest{SessionID: bs.SessionID, Generation: bs.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, child.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
	w.mu.Lock()
	untouched := w.next == next && w.remaining == remaining && !w.failed
	w.mu.Unlock()
	if got.Failure == "" || writes.Load() != before || !untouched {
		t.Error("ASSERT_REPAIR_FOREIGN_TRANSACTION_ZERO_EFFECT")
	}
}

// Cancelling a context derived from A must not let a foreign public request
// cancel A before the inner transaction's exact-record refusal can run.
func TestPrivateProviderWindowRepairForeignCancelledRoundTrip(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{false: "wrong-manager-same-scalars", true: "wrong-session-same-manager"}[same], func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			st := windowTestReady(t, m, start, ctx)
			if got := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"}); got.Failure != "" {
				t.Fatal(got.Failure)
			}
			b, bs, child, writes := repairOrdinary(t, m, start, same)
			w := privateProviderWindowFrom(ctx)
			if !same && (bs.SessionID != st.SessionID || bs.Generation != st.Generation) {
				t.Fatal("fixture must share session/gen")
			}
			beforeWrites := writes.Load()
			w.mu.Lock()
			next, remaining, active, preparations, changed := w.next, w.remaining, w.active, w.preparations, w.changed
			w.mu.Unlock()
			derived, cancel := context.WithCancel(ctx)
			cancel()
			got := b.RoundTrip(derived, RoundTripRequest{SessionID: bs.SessionID, Generation: bs.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, child.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
			w.mu.Lock()
			untouched := w.next == next && w.remaining == remaining && w.active == active && w.preparations == preparations && w.changed == changed && !w.expired && !w.failed && !w.finishing && !w.released
			w.mu.Unlock()
			live := ctx.Err() == nil
			zeroWrites := writes.Load() == beforeWrites
			// Exercise the actual legitimate suffix, even when the old outer
			// wrapper has poisoned A. Do not replace completion with a proxy.
			suffix := true
			for _, method := range []string{"textDocument/documentSymbol", "textDocument/definition"} {
				result := m.RoundTrip(ctx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: method, Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"character":0}}`, s.child.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
				suffix = suffix && result.Failure == "" && result.ServerError == nil
			}
			completion, err := m.FinishPrivateProviderWindow(ctx)
			if got.Failure == "" || !untouched || !live || !zeroWrites || !suffix || err != nil || !completion.Completed {
				t.Errorf("ASSERT_REPAIR_FOREIGN_CANCEL_ISOLATED: refusal=%s untouched=%t live=%t zeroWrites=%t suffix=%t completed=%t err=%v", got.Failure, untouched, live, zeroWrites, suffix, completion.Completed, err)
			} else {
				t.Log("ASSERT_REPAIR_FOREIGN_CANCEL_ISOLATED: PASS")
			}
		})
	}
}

func TestPrivateProviderWindowRepairOwnedDerivedCancellation(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "textDocument/documentSymbol")
	st := windowTestReady(t, m, start, ctx)
	if got := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"}); got.Failure != "" {
		t.Fatal(got.Failure)
	}
	derived, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan RoundTripResult, 1)
	go func() {
		done <- m.RoundTrip(derived, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, s.child.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
	}()
	select {
	case <-s.child.entered:
	case <-ctx.Done():
		t.Fatal("owned query did not reach controlled provider")
	}
	cancel()
	result := <-done
	close(s.child.release)
	completion, err := m.FinishPrivateProviderWindow(ctx)
	if result.Failure == "" || ctx.Err() == nil || err == nil || completion.Completed {
		t.Errorf("ASSERT_REPAIR_OWNED_CANCEL_RETAINED: failure=%s context=%v completed=%t err=%v", result.Failure, ctx.Err(), completion.Completed, err)
	} else {
		t.Log("ASSERT_REPAIR_OWNED_CANCEL_RETAINED: PASS")
	}
}

// Exercise each outer wrapper with valid native parameters and a real reservation.
// Only the derived context is cancelled; A must remain usable through native C.
func TestPrivateProviderWindowRepairForeignCancelledWrappers(t *testing.T) {
	for _, site := range []string{"shared", "ready-lease", "native", "directional"} {
		for _, same := range []bool{false, true} {
			t.Run(site+"/"+map[bool]string{false: "wrong-manager-same-scalars", true: "wrong-session-same-manager"}[same], func(t *testing.T) {
				m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true, Calls: true}, "")
				st := windowTestReady(t, m, start, ctx)
				if got := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"}); got.Failure != "" {
					t.Fatal(got.Failure)
				}
				for _, method := range []string{"textDocument/documentSymbol", "textDocument/definition"} {
					got := m.RoundTrip(ctx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: method, Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"character":0}}`, s.child.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
					if got.Failure != "" || got.ServerError != nil {
						t.Fatal("fixture prefix", got.Failure)
					}
				}
				b, bs, child, writes := repairOrdinary(t, m, start, same)
				if !same && (bs.SessionID != st.SessionID || bs.Generation != st.Generation) {
					t.Fatal("fixture must share session/gen")
				}
				w := privateProviderWindowFrom(ctx)
				w.mu.Lock()
				next, remaining, active, preparations, changed := w.next, w.remaining, w.active, w.preparations, w.changed
				w.mu.Unlock()
				beforeWrites := writes.Load()
				derived, cancel := context.WithCancel(ctx)
				cancel()
				var callbacks atomic.Int64
				refused := repairInvokeWrapper(site, b, derived, bs, start, child.uri, &callbacks)
				w.mu.Lock()
				untouched := w.next == next && w.remaining == remaining && w.active == active && w.preparations == preparations && w.changed == changed && !w.expired && !w.failed && !w.finishing && !w.released
				w.mu.Unlock()
				live := ctx.Err() == nil
				zeroEffects := writes.Load() == beforeWrites && callbacks.Load() == 0
				line, ch := uint32(0), uint32(0)
				native, nativeErr := m.ExecuteCallHierarchyDirectionsPrivate(ctx, DirectionalCallHierarchyRequestPrivate{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go", Line: &line, Character: &ch, Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
				suffix := nativeErr == nil && native.Failure == "" && len(native.Incoming) == 2 && len(native.Outgoing) == 2
				completion, err := m.FinishPrivateProviderWindow(ctx)
				if !refused || !untouched || !live || !zeroEffects || !suffix || err != nil || !completion.Completed {
					t.Errorf("ASSERT_WRAPPER_FOREIGN_ISOLATED: site=%s refused=%t untouched=%t live=%t zeroEffects=%t suffix=%t completed=%t err=%v", site, refused, untouched, live, zeroEffects, suffix, completion.Completed, err)
				} else {
					t.Logf("ASSERT_WRAPPER_FOREIGN_ISOLATED: site=%s PASS", site)
				}
			})
		}
	}
}

func repairInvokeWrapper(site string, m *Manager, ctx context.Context, st StartResult, start StartRequest, uri string, callbacks *atomic.Int64) bool {
	line, ch := uint32(0), uint32(0)
	req := RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"character":0}}`, uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10}
	switch site {
	case "shared":
		req.Method = "textDocument/documentSymbol"
		got := m.roundTripWithPrivateTransaction(ctx, req, nil, nil)
		return got.Failure != "" && len(got.Responses) == 0
	case "ready-lease":
		return m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(context.Context) error { callbacks.Add(1); return nil }) != nil
	case "native":
		got, err := m.ExecuteCallHierarchyDirectionsPrivate(ctx, DirectionalCallHierarchyRequestPrivate{SessionID: st.SessionID, Generation: st.Generation, URI: uri, LanguageID: "go", Line: &line, Character: &ch, Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
		return err != nil && len(got.Incoming) == 0 && len(got.Outgoing) == 0
	case "directional":
		r := directionalraw.NewReservation("repair-foreign-wrapper")
		defer r.Release()
		got, _ := m.RoundTripDirectionalPrivate(r.Context(ctx), req)
		return got.Failure != "" && len(got.Responses) == 0
	default:
		panic("unknown wrapper")
	}
}

func TestPrivateProviderWindowRepairOwnedCancelledWrappers(t *testing.T) {
	for _, site := range []string{"shared", "ready-lease", "native", "directional"} {
		t.Run(site, func(t *testing.T) {
			stall := "textDocument/prepareCallHierarchy"
			if site == "shared" {
				stall = "textDocument/documentSymbol"
			}
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Calls: true}, stall)
			st := windowTestReady(t, m, start, ctx)
			if got := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"}); got.Failure != "" {
				t.Fatal(got.Failure)
			}
			if site != "shared" {
				got := m.RoundTrip(ctx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, s.child.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
				if got.Failure != "" {
					t.Fatal(got.Failure)
				}
			}
			if site == "directional" {
				if prep := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"}); prep.Failure != "" {
					t.Fatal(prep.Failure)
				}
			}
			derived, cancel := context.WithCancel(ctx)
			defer cancel()
			var callbacks atomic.Int64
			entered := s.child.entered
			callbackEntered := make(chan struct{})
			if site == "ready-lease" {
				entered = callbackEntered
			}
			done := make(chan bool, 1)
			go func() {
				if site == "ready-lease" {
					err := m.WithReadySessionLease(derived, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(inner context.Context) error {
						callbacks.Add(1)
						close(callbackEntered)
						<-inner.Done()
						return inner.Err()
					})
					done <- err != nil
				} else {
					done <- repairInvokeWrapper(site, m, derived, st, start, s.child.uri, &callbacks)
				}
			}()
			select {
			case <-entered:
			case early := <-done:
				t.Fatalf("owned wrapper did not reach actual work: refused=%t", early)
			case <-ctx.Done():
				t.Fatal("owned wrapper did not reach actual work before deadline")
			}
			cancel()
			close(s.child.release)
			refused := <-done
			completion, err := m.FinishPrivateProviderWindow(ctx)
			if !refused || ctx.Err() == nil || err == nil || completion.Completed {
				t.Errorf("ASSERT_WRAPPER_OWNED_CANCEL: site=%s refused=%t ctx=%v completed=%t err=%v", site, refused, ctx.Err(), completion.Completed, err)
			} else {
				t.Logf("ASSERT_WRAPPER_OWNED_CANCEL: site=%s PASS", site)
			}
		})
	}
}

// Hold an actual initialize response after its first byte, without changing
// the runtime/starter or synthesizing a readiness operation.
func repairReadinessResponseGate(t *testing.T, child *windowPipeChild) (<-chan struct{}, func()) {
	t.Helper()
	original := child.output
	reader, writer := io.Pipe()
	child.output = reader
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	go func() {
		defer close(done)
		defer writer.Close()
		var first [1]byte
		n, err := original.Read(first[:])
		if n != 1 || err != nil {
			return
		}
		close(entered)
		<-release
		if _, err := writer.Write(first[:]); err != nil {
			return
		}
		_, _ = io.Copy(writer, original)
	}()
	t.Cleanup(func() { unblock(); _ = original.Close(); _ = writer.Close(); _ = reader.Close(); <-done })
	return entered, unblock
}

func repairReadinessFingerprint(w *privateProviderWindow) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return fmt.Sprintf("%d/%d/%d/%d/%p/%t/%t/%t/%t", w.next, w.remaining, w.active, w.preparations, w.changed, w.expired, w.failed, w.finishing, w.released)
}

func repairReadinessSuffix(m *Manager, st StartResult, start StartRequest, ctx context.Context, uri string) (bool, PrivateProviderWindowCompletion, error) {
	prep := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: uri, LanguageID: "go"})
	suffix := prep.Failure == ""
	for _, method := range []string{"textDocument/documentSymbol", "textDocument/definition"} {
		got := m.RoundTrip(ctx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: method, Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"character":0}}`, uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
		suffix = suffix && got.Failure == "" && got.ServerError == nil
	}
	completion, err := m.FinishPrivateProviderWindow(ctx)
	<-privateProviderWindowFrom(ctx).finishDone
	return suffix, completion, err
}

func repairReadinessPeer(t *testing.T, m *Manager, start StartRequest, same bool) (*Manager, StartResult, ReadinessSnapshot, *windowPipeChild, *atomic.Int64, func()) {
	t.Helper()
	root := start.Profile.Workspace().String()
	if same {
		var err error
		root, err = filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
		if err != nil {
			t.Fatal(err)
		}
		start.Profile, start.Process.Dir = runtimeprofile.Resolve(selector), root
		if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\nfunc f() {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	child := newWindowPipeChild((&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String(), "")
	writes := &atomic.Int64{}
	child.beforeWrite = func(int) { writes.Add(1) }
	entered, unblock := repairReadinessResponseGate(t, child)
	if !same {
		var err error
		m, err = New(Config{Limits: m.limits, Starter: &windowPipeStarter{child: child}})
		if err != nil {
			t.Fatal(err)
		}
	} else {
		s := m.starter.(*windowPipeStarter)
		previous := s.child
		s.child = child
		defer func() { s.child = previous }()
	}
	go child.serve()
	t.Cleanup(func() {
		unblock()
		_ = child.input.Close()
		_ = child.output.Close()
		_ = child.serverInput.Close()
		_ = child.serverOutput.Close()
		<-child.done
	})
	st := m.Start(context.Background(), start)
	if st.Failure != "" {
		t.Fatal(st.Failure)
	}
	pending := m.BeginReadiness(context.Background(), st.SessionID, st.Generation, start.Deadline)
	if pending.ID == "" || pending.State != ReadinessPending {
		t.Fatal("peer readiness was not pending", pending)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("peer initialize response did not reach gate")
	}
	return m, st, pending, child, writes, unblock
}

func TestPrivateProviderWindowRepairForeignReadiness(t *testing.T) {
	for _, same := range []bool{false, true} {
		for _, state := range []string{"pending", "ready", "failed"} {
			t.Run(map[bool]string{false: "wrong-manager-same-scalars", true: "wrong-session-same-manager"}[same]+"/"+state, func(t *testing.T) {
				m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
				st := windowTestReady(t, m, start, ctx)
				own := m.BeginReadiness(ctx, st.SessionID, st.Generation, start.Deadline)
				b, bs, pending, child, writes, unblock := repairReadinessPeer(t, m, start, same)
				if !same && (bs.SessionID != st.SessionID || bs.Generation != st.Generation || pending.ID != own.ID) {
					t.Fatal("peer must share scalar readiness/session/generation")
				}
				if state != "pending" {
					if state == "failed" {
						_ = child.serverOutput.Close()
					}
					unblock()
					cached, ok := b.WaitReadiness(context.Background(), pending.ID)
					want := ReadinessReady
					if state == "failed" {
						want = ReadinessFailed
					}
					if !ok || cached.State != want {
						t.Fatal("peer real cached state", cached)
					}
				}
				before := repairReadinessFingerprint(privateProviderWindowFrom(ctx))
				beforeWrites := writes.Load()
				derived, cancel := context.WithCancel(ctx)
				cancel()
				got, ok := b.WaitReadiness(derived, pending.ID)
				untouched := before == repairReadinessFingerprint(privateProviderWindowFrom(ctx))
				live, zeroIO := ctx.Err() == nil, writes.Load() == beforeWrites
				refused := !ok && reflect.DeepEqual(got, ReadinessSnapshot{})
				suffix, completion, err := repairReadinessSuffix(m, st, start, ctx, s.child.uri)
				if !refused || !untouched || !live || !zeroIO || !suffix || !completion.Completed || err != nil {
					t.Errorf("ASSERT_READINESS_FOREIGN_ISOLATED: state=%s refused=%t untouched=%t live=%t zeroIO=%t suffix=%t completed=%t err=%v", state, refused, untouched, live, zeroIO, suffix, completion.Completed, err)
				} else {
					t.Logf("ASSERT_READINESS_FOREIGN_ISOLATED: state=%s PASS", state)
				}
				unblock()
				if state == "pending" {
					peer, ok := b.WaitReadiness(context.Background(), pending.ID)
					if !ok || peer.State != ReadinessReady {
						t.Error("foreign peer readiness was interfered with", peer)
					}
				}
			})
		}
	}
}

func TestPrivateProviderWindowRepairOwnedReadiness(t *testing.T) {
	for _, state := range []string{"ready", "failed", "cancelled"} {
		t.Run(state, func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			entered, unblock := repairReadinessResponseGate(t, s.child)
			st := m.Start(ctx, start)
			if st.Failure != "" {
				t.Fatal(st.Failure)
			}
			pending := m.BeginReadiness(ctx, st.SessionID, st.Generation, start.Deadline)
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("owned initialize response not reached")
			}
			if pending.State != ReadinessPending {
				t.Fatal("owned readiness not pending")
			}
			waitCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			if state == "cancelled" {
				cancel()
				got, ok := m.WaitReadiness(waitCtx, pending.ID)
				unblock()
				completion, err := m.FinishPrivateProviderWindow(ctx)
				<-privateProviderWindowFrom(ctx).finishDone
				if !ok || got.State != ReadinessFailed || got.Failure != privateProviderWindowUnjoined || ctx.Err() == nil || completion.Completed || err == nil {
					t.Errorf("ASSERT_READINESS_OWNED_CANCEL: got=%+v ok=%t ctx=%v completed=%t err=%v", got, ok, ctx.Err(), completion.Completed, err)
				} else {
					t.Log("ASSERT_READINESS_OWNED_CANCEL: PASS")
				}
				return
			}
			type result struct {
				snapshot ReadinessSnapshot
				ok       bool
			}
			done := make(chan result, 1)
			go func() { got, ok := m.WaitReadiness(waitCtx, pending.ID); done <- result{got, ok} }()
			if state == "failed" {
				_ = s.child.serverOutput.Close()
			}
			unblock()
			got := <-done
			if state == "ready" {
				suffix, completion, err := repairReadinessSuffix(m, st, start, ctx, s.child.uri)
				if !got.ok || got.snapshot.State != ReadinessReady || !suffix || !completion.Completed || err != nil {
					t.Errorf("ASSERT_READINESS_OWNED_READY: state=%s ok=%t suffix=%t completed=%t err=%v", got.snapshot.State, got.ok, suffix, completion.Completed, err)
				} else {
					t.Log("ASSERT_READINESS_OWNED_READY: PASS")
				}
			} else {
				completion, err := m.FinishPrivateProviderWindow(ctx)
				<-privateProviderWindowFrom(ctx).finishDone
				if !got.ok || got.snapshot.State != ReadinessFailed || got.snapshot.Failure == "" || completion.Completed || err == nil {
					t.Errorf("ASSERT_READINESS_OWNED_FAILED: state=%s ok=%t completed=%t err=%v", got.snapshot.State, got.ok, completion.Completed, err)
				} else {
					t.Log("ASSERT_READINESS_OWNED_FAILED: PASS")
				}
			}
		})
	}
}

// Synthetic negative perturbations of a real completed operation are binding
// counterexamples, not fabricated evidence of successful provider behavior.
func TestPrivateProviderWindowRepairReadinessBinding(t *testing.T) {
	for _, kind := range []string{"generation", "attempt", "operation-id", "mapping", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
			st := windowTestReady(t, m, start, ctx)
			own := m.BeginReadiness(ctx, st.SessionID, st.Generation, start.Deadline)
			before := repairReadinessFingerprint(privateProviderWindowFrom(ctx))
			m.mu.Lock()
			op := m.readiness[own.ID]
			original := op.snapshot
			key := st.SessionID + fmt.Sprintf(":%d", st.Generation)
			mapping := m.readinessIDs[key]
			switch kind {
			case "generation":
				op.snapshot.Generation++
			case "attempt":
				op.snapshot.AttemptID = ""
			case "operation-id":
				op.snapshot.ID = "other"
			case "mapping":
				m.readinessIDs[key] = "other"
			}
			m.mu.Unlock()
			id := own.ID
			if kind == "unknown" {
				id = "unknown"
			}
			got, ok := m.WaitReadiness(ctx, id)
			m.mu.Lock()
			op.snapshot = original
			m.readinessIDs[key] = mapping
			m.mu.Unlock()
			untouched := before == repairReadinessFingerprint(privateProviderWindowFrom(ctx)) && ctx.Err() == nil
			suffix, completion, err := repairReadinessSuffix(m, st, start, ctx, s.child.uri)
			if ok || !reflect.DeepEqual(got, ReadinessSnapshot{}) || !untouched || !suffix || !completion.Completed || err != nil {
				t.Errorf("ASSERT_READINESS_EXACT_BINDING: kind=%s refused=%t untouched=%t suffix=%t completed=%t err=%v", kind, !ok && reflect.DeepEqual(got, ReadinessSnapshot{}), untouched, suffix, completion.Completed, err)
			} else {
				t.Logf("ASSERT_READINESS_EXACT_BINDING: kind=%s PASS", kind)
			}
		})
	}
}

func TestPrivateProviderWindowRepairOrdinaryAdoption(t *testing.T) {
	m, start, ctx, s := windowTestSetup(t, PrivateProviderWindowPlan{Definitions: true}, "")
	// The ordinary INITIALIZING child is started on this exact Manager/profile
	// before private Start. Admission must not initialize this unwrapped child.
	var writes atomic.Int64
	s.child.beforeWrite = func(int) { writes.Add(1) }
	st := m.Start(context.Background(), start)
	if st.Failure != "" {
		t.Fatal(st.Failure)
	}
	got := m.Start(ctx, start)
	if got.Failure == "" || got.Generation != 0 {
		t.Error("ASSERT_REPAIR_ORDINARY_START_REFUSED")
	}
	ready := m.BeginReadiness(ctx, st.SessionID, st.Generation, start.Deadline)
	if ready.ID != "" {
		ready, _ = m.WaitReadiness(context.Background(), ready.ID)
	}
	if ready.State != ReadinessFailed || writes.Load() != 0 {
		t.Error("ASSERT_REPAIR_ORDINARY_READINESS_REFUSED")
	}
}

func TestPrivateProviderWindowRepairFailedPreparationCannotComplete(t *testing.T) {
	for _, second := range []bool{false, true} {
		for _, kind := range []string{"malformed", "oversize"} {
			t.Run(fmt.Sprintf("second=%t/%s", second, kind), func(t *testing.T) {
				plan := PrivateProviderWindowPlan{Definitions: true, Calls: second}
				m, start, ctx, s := windowTestSetup(t, plan, "")
				st := windowTestReady(t, m, start, ctx)
				query := func(method string) {
					_ = m.RoundTrip(ctx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: method, Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"character":0}}`, s.child.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10})
				}
				if second {
					good := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go"})
					if good.Failure != "" {
						t.Fatal(good.Failure)
					}
					query("textDocument/documentSymbol")
					query("textDocument/definition")
				}
				badURI := s.child.uri
				if kind == "malformed" {
					badURI = "%"
				} else {
					if err := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "a.go"), bytes.Repeat([]byte("x"), (16<<10)+1), 0600); err != nil {
						t.Fatal(err)
					}
				}
				bad := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: badURI, LanguageID: "go"})
				if bad.Failure == "" {
					t.Fatal("fixture preparation must fail")
				}
				// Do not stop at the failure: exercise the review's complete suffix.
				if second {
					// Attempt the native suffix without retrying the failed preparation.
					// These are the production native transaction helpers, not a fake callback.
					line, ch := uint32(0), uint32(0)
					req := DirectionalCallHierarchyRequestPrivate{SessionID: st.SessionID, Generation: st.Generation, URI: s.child.uri, LanguageID: "go", Line: &line, Character: &ch, Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 128 << 10}
					reservation := directionalraw.NewReservation("repair-second-preparation")
					_, selected, _ := m.prepareDirectionalItemPrivate(ctx, req, reservation)
					var native DirectionalCallHierarchyResultPrivate
					_, _, _ = m.executeDirectionalCallPrivate(ctx, req, reservation, "callHierarchy/incomingCalls", selected, &native.Incoming)
					_, _, _ = m.executeDirectionalCallPrivate(ctx, req, reservation, "callHierarchy/outgoingCalls", selected, &native.Outgoing)
					reservation.Release()
				} else {
					query("textDocument/documentSymbol")
					query("textDocument/definition")
				}
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				t.Cleanup(unblock)
				leaseDone := make(chan error, 1)
				go func() {
					leaseDone <- m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(context.Context) error { close(entered); <-release; return nil })
				}()
				select {
				case <-entered:
				case e := <-leaseDone:
					t.Fatalf("lease unavailable: %v", e)
				}
				type outcome struct {
					c PrivateProviderWindowCompletion
					e error
				}
				done := make(chan outcome, 1)
				go func() { c, e := m.FinishPrivateProviderWindow(ctx); done <- outcome{c, e} }()
				select {
				case <-done:
					t.Error("ASSERT_REPAIR_PREP_FAILURE_PREMATURE_RELEASE")
				case <-time.After(10 * time.Millisecond):
				}
				w := privateProviderWindowFrom(ctx)
				w.mu.Lock()
				released := w.released
				w.mu.Unlock()
				if released || m.Census().Workers < 2 {
					t.Error("ASSERT_REPAIR_PREP_FAILURE_OWNER_RETAINED")
				}
				unblock()
				<-leaseDone
				result := <-done
				if result.e == nil || result.c.Completed {
					t.Error("ASSERT_REPAIR_FAILED_PREP_NEVER_COMPLETES")
				}
			})
		}
	}
}
