package sessionruntime

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

func directionalSessionStateForTest(m *Manager, sessionID string) (session.State, bool) {
	for _, record := range m.Records() {
		if record.SessionID == sessionID {
			return record.State, true
		}
	}
	return "", false
}

func directionalCompositionManagerForTest(t *testing.T, mode string) (*Manager, StartResult, string) {
	t.Helper()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(path, []byte("package fixture\n\nfunc Target() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	child := newRoundTripChild(mode)
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if ready := m.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatal(ready)
	}
	return m, started, (&url.URL{Scheme: "file", Path: path}).String()
}

func directionalTimeoutRequestForTest(started StartResult, uri string) DirectionalCallHierarchyRequestPrivate {
	line, character := uint32(0), uint32(0)
	return DirectionalCallHierarchyRequestPrivate{
		SessionID: started.SessionID, Generation: started.Generation,
		URI: uri, LanguageID: "go", Line: &line, Character: &character,
		Deadline: time.Now().Add(30 * time.Millisecond), MaxMessages: 8, MaxBytes: 1 << 20,
	}
}

func TestCallHierarchyCompositionPreWriteTimeoutRemainsUnpoisoned(t *testing.T) {
	m, started, uri := directionalCompositionManagerForTest(t, "call-hierarchy-hang-incoming")
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	req := directionalTimeoutRequestForTest(started, uri)
	req.Deadline = time.Now().Add(-time.Millisecond)

	got, err := m.ExecuteCallHierarchyDirectionsPrivate(context.Background(), req)
	if err == nil || err.Error() != string(session.RequestTimeout) || got.SessionPoisoned || got.FailurePostEffect || got.RequestWriteCompletedPrivate || got.TerminalPoisonObservedPrivate {
		t.Fatalf("ASSERT_CALL_COMPOSITION_PREWRITE_TIMEOUT_UNPOISONED: result=%+v err=%v", got, err)
	}
	if state, ok := directionalSessionStateForTest(m, started.SessionID); !ok || state != session.Ready {
		t.Fatalf("ASSERT_CALL_COMPOSITION_PREWRITE_TIMEOUT_READY: state=%s ok=%t", state, ok)
	}
}

func TestCallHierarchyCompositionPostWriteTimeoutPoisons(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode string
	}{
		{name: "incoming", mode: "call-hierarchy-hang-incoming"},
		{name: "outgoing", mode: "call-hierarchy-hang-outgoing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, started, uri := directionalCompositionManagerForTest(t, tc.mode)
			t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
			req := directionalTimeoutRequestForTest(started, uri)

			got, err := m.ExecuteCallHierarchyDirectionsPrivate(context.Background(), req)
			if !errors.Is(err, context.DeadlineExceeded) || got.Failure != session.RequestTimeout || !got.FailurePostEffect || !got.RequestWriteCompletedPrivate {
				t.Fatalf("ASSERT_CALL_COMPOSITION_%s_POSTWRITE_TIMEOUT_TYPED: result=%+v err=%v", tc.name, got, err)
			}
			if !got.SessionPoisoned || !got.TerminalPoisonObservedPrivate || got.TerminalLifecyclePrivate != session.Poisoned {
				t.Fatalf("ASSERT_CALL_COMPOSITION_%s_POSTWRITE_TIMEOUT_POISONED: result=%+v", tc.name, got)
			}
			if state, ok := directionalSessionStateForTest(m, started.SessionID); !ok || state != session.Poisoned {
				t.Fatalf("ASSERT_CALL_COMPOSITION_%s_MANAGER_POISON_FACT: state=%s ok=%t", tc.name, state, ok)
			}

			retry, retryErr := m.ExecuteCallHierarchyDirectionsPrivate(context.Background(), req)
			if retry.Failure != session.LifecycleConflict || retryErr == nil {
				t.Fatalf("ASSERT_CALL_COMPOSITION_%s_SAME_SESSION_REFUSAL: result=%+v err=%v", tc.name, retry, retryErr)
			}
		})
	}
}
