package sessionruntime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRetainedScopeDiagnosticPrivateClosedReasons(t *testing.T) {
	for _, tc := range []struct{ name, reason, class string }{
		{"nomination", "ACCEPTED", "NOMINATION"},
		{"context", "ACCEPTED", "CONTEXT"},
		{"external", "URI_OUTSIDE_FROZEN", "FROZEN_EXTERNAL"},
		{"dto", "REQUEST_DTO_INVALID", "INVALID"},
		{"state", "FROZEN_STATE_MISMATCH", "NOMINATION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, s, _, members := retainedScopeFixture(t)
			g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
			if f != "" {
				t.Fatal(f)
			}
			uri := members[0].URI
			if tc.name == "context" {
				uri = members[4].URI
			}
			if tc.name == "external" {
				uri = "file:///private/not-frozen.go"
			}
			req := retainedScopeRequest(s, g, "callHierarchy/outgoingCalls", uri)
			if tc.name == "dto" {
				req.Params = json.RawMessage(`{"item":{"URI":"file:///private/secret.go"}}`)
			}
			if tc.name == "state" {
				m.sessions[s.SessionID].documents[members[5].URI].supply.Content[0] = 'X'
			}
			got := m.RoundTrip(context.Background(), req)
			if (got.Failure == "") != (tc.reason == "ACCEPTED") {
				t.Fatalf("failure=%s", got.Failure)
			}
			if tc.reason != "ACCEPTED" && got.Failure != DocumentSupplyUnavailable {
				t.Fatal("legacy failure changed", got.Failure)
			}
			events, overflow := m.RetainedScopeDiagnosticsPrivate(g)
			if overflow || len(events) != 1 || events[0].Reason != tc.reason || events[0].URIClass != tc.class || events[0].Ordinal != 1 {
				t.Fatalf("%+v overflow=%v", events, overflow)
			}
			raw, _ := json.Marshal(events)
			if strings.Contains(string(raw), "file:") {
				t.Fatal("URI leak")
			}
			events[0].Reason = "tampered"
			detached, _ := m.RetainedScopeDiagnosticsPrivate(g)
			if detached[0].Reason != tc.reason {
				t.Fatal("aliased observations")
			}
			cp := *g
			if events, _ = m.RetainedScopeDiagnosticsPrivate(&cp); len(events) != 0 {
				t.Fatal("copied capability accepted")
			}
		})
	}
}

func TestRetainedScopeDiagnosticPrivateCheckIsNotWire(t *testing.T) {
	m, s, c, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	if f = m.CheckRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, g); f != "" {
		t.Fatal(f)
	}
	if events, _ := m.RetainedScopeDiagnosticsPrivate(g); len(events) != 0 {
		t.Fatal("successful check fabricated request")
	}
	before, _ := c.snapshot()
	m.sessions[s.SessionID].documents[members[5].URI].supply.Content[0] = 'X'
	if f = m.CheckRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, g); f != DocumentSupplyUnavailable {
		t.Fatal(f)
	}
	events, _ := m.RetainedScopeDiagnosticsPrivate(g)
	if len(events) != 1 || events[0].Phase != "CHECK" || events[0].Method != "NONE" || events[0].Reason != "FROZEN_STATE_MISMATCH" {
		t.Fatal(events)
	}
	after, _ := c.snapshot()
	if len(after) != len(before) {
		t.Fatal("check wrote wire")
	}
}

func TestRetainedScopeDiagnosticPrivateBound(t *testing.T) {
	m, s, _, members := retainedScopeFixture(t)
	g, f := m.BindRetainedScopePrivate(context.Background(), s.SessionID, s.Generation, members)
	if f != "" {
		t.Fatal(f)
	}
	req := retainedScopeRequest(s, g, "callHierarchy/outgoingCalls", "file:///external.go")
	for i := 0; i < 256; i++ {
		if got := m.RoundTrip(context.Background(), req); got.Failure == "" {
			t.Fatal("external accepted")
		}
	}
	events, overflow := m.RetainedScopeDiagnosticsPrivate(g)
	raw, err := json.Marshal(events)
	if err != nil || !overflow || len(events) > 64 || len(events) == 0 || len(raw) >= 8192 {
		t.Fatalf("events=%d bytes=%d overflow=%v err=%v", len(events), len(raw), overflow, err)
	}
	// Even after overflow, the latest refusal survives instead of being hidden
	// by a successful prefix. No raw query material enters the snapshot.
	for i, e := range events {
		if e.Ordinal != uint64(256-len(events)+i+1) || e.Reason != "URI_OUTSIDE_FROZEN" {
			t.Fatal(e)
		}
	}
}
