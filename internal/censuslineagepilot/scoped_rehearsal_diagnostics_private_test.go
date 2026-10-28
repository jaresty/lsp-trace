//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/sessionruntime"
)

func TestScopedRehearsalDiagnosticsBridgePrivate(t *testing.T) {
	h, m, cfg, _ := NewScopedRehearsalFixtureV2Private(t)
	var terminal []sessionruntime.RetainedScopeDiagnosticPrivate
	err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(ctx context.Context) error {
		if _, err := h.Bind(ctx, m); err != nil {
			return err
		}
		events, overflow, available := h.Diagnostics()
		if !available || overflow || len(events) != 0 {
			t.Fatal("valid empty bridge", events, overflow, available)
		}
		req := sessionruntime.RoundTripRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, RetainedScopePrivate: h.adapter.guard, Method: "callHierarchy/outgoingCalls", Params: json.RawMessage(`{"item":{"uri":"file:///external.go"}}`), MaxMessages: 1, MaxBytes: 2 << 20, Deadline: time.Now().Add(time.Second)}
		for i := 0; i < 256; i++ {
			if m.RoundTrip(ctx, req).Failure == "" {
				t.Fatal("external accepted")
			}
		}
		events, overflow, available = h.Diagnostics()
		if !available || !overflow || len(events) == 0 || len(events) > 64 {
			t.Fatal("bridge overflow")
		}
		events[0].Reason = "tampered"
		terminal, _, _ = h.Diagnostics()
		if terminal[0].Reason != "URI_OUTSIDE_FROZEN" {
			t.Fatal("bridge aliases Manager")
		}
		return h.CloseExecution()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(terminal) == 0 || terminal[0].Reason != "URI_OUTSIDE_FROZEN" {
		t.Fatal("terminal changed after lease/close")
	}
}
