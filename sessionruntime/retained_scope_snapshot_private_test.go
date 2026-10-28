package sessionruntime

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRetainedScopeSnapshotPrivateAvailabilityAndOwnership(t *testing.T) {
	m, s, child, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	before, _ := child.snapshot()
	events, overflow, available := m.RetainedScopeDiagnosticSnapshotPrivate(g)
	if !available || overflow || len(events) != 0 {
		t.Fatal("valid empty unavailable")
	}
	copyGuard := *g
	for _, bad := range []*RetainedScopeGuardPrivate{nil, {}, &copyGuard} {
		if _, _, ok := m.RetainedScopeDiagnosticSnapshotPrivate(bad); ok {
			t.Fatal("invalid identity available")
		}
	}
	// State failure is not identity failure, and a CHECK is not a wire request.
	m.sessions[s.SessionID].documents[members[5].URI].supply.Content[0] = 'X'
	if m.CheckRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, g) != DocumentSupplyUnavailable {
		t.Fatal("state accepted")
	}
	events, overflow, available = m.RetainedScopeDiagnosticSnapshotPrivate(g)
	if !available || overflow || len(events) != 1 || events[0].Phase != "CHECK" || events[0].Method != "NONE" {
		t.Fatal(events, available)
	}
	events[0].Reason = "tampered"
	detached, _, _ := m.RetainedScopeDiagnosticSnapshotPrivate(g)
	if detached[0].Reason != "FROZEN_STATE_MISMATCH" {
		t.Fatal("snapshot aliased")
	}
	after, _ := child.snapshot()
	if len(after) != len(before) {
		t.Fatal("snapshot/check caused wire")
	}
	m.mu.Lock()
	g.session.retainedScopePrivate = nil
	m.mu.Unlock()
	if _, _, ok := m.RetainedScopeDiagnosticSnapshotPrivate(g); ok {
		t.Fatal("reset capability available")
	}
	if detached[0].Reason != "FROZEN_STATE_MISMATCH" {
		t.Fatal("reset changed detached snapshot")
	}
}

func TestRetainedScopeSnapshotPrivateEnvelopeBound(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	req := retainedScopeRequest(s, g, "callHierarchy/outgoingCalls", "file:///external.go")
	for i := 0; i < 256; i++ {
		if m.RoundTrip(context.Background(), req).Failure == "" {
			t.Fatal("external accepted")
		}
	}
	events, overflow, available := m.RetainedScopeDiagnosticSnapshotPrivate(g)
	raw, err := json.Marshal(struct {
		Available bool                             `json:"available"`
		Events    []RetainedScopeDiagnosticPrivate `json:"events"`
		Overflow  bool                             `json:"overflow"`
	}{available, events, overflow})
	if err != nil || !available || !overflow || len(events) > 64 || len(raw) > 8192 {
		t.Fatal("envelope bounds", len(raw), len(events), err)
	}
}
