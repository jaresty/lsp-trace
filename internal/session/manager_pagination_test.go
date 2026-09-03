package session

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestListADRExactManagerGlobalOrder(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 6})
	for _, tc := range []struct {
		id    string
		state State
	}{{"starting", Starting}, {"ready", Ready}, {"stopped", Stopped}, {"crashed", Crashed}, {"poisoned", Poisoned}} {
		m.RegisterLifecycle(tc.id, 1, tc.state, tc.state == Stopped || tc.state == Crashed || tc.state == Poisoned)
	}
	page := m.List(ListOptions{})
	want := []string{"starting", "ready", "stopped", "crashed", "poisoned"}
	if page.Code != "" || !reflect.DeepEqual(ids(page.Records), want) {
		t.Fatalf("ASSERT_LIST_MANAGER_GLOBAL_ADR_ORDER: code=%q got=%v want=%v", page.Code, ids(page.Records), want)
	}
}

func TestListADRExactSessionLocalProjection(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("s", 1, Initializing, false)
	m.ObserveInitialization("s", 1, true)
	m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStatus})
	page := m.List(ListOptions{SessionID: "s"})
	if page.Code != "" || len(page.Records) != 1 || page.Records[0].LocalSeq != 3 || page.Records[0].State != "READY" {
		t.Fatalf("ASSERT_LIST_SESSION_LOCAL_EVENT_PROJECTION: %+v", page)
	}
}

func TestListOverlayEquivalentEventAuthorityAndContinuation(t *testing.T) {
	m := paginationManager()
	beforeManager := m.nextManagerEvent
	beforeLastUse := map[string]uint64{}
	for id, lifecycle := range m.lifecycles {
		beforeLastUse[id] = lifecycle.lastUseManagerSeq
	}
	first := m.List(ListOptions{Limit: 1})
	if m.nextManagerEvent != beforeManager+1 {
		t.Fatalf("ASSERT_LIST_OVERLAY_EXACTLY_ONE_SNAPSHOT_EVENT before=%d after=%d", beforeManager, m.nextManagerEvent)
	}
	cursor := cursorPayload(t, first.NextCursor)
	wantPrefix := "list-" + strconv.FormatUint(m.nextManagerEvent, 10) + "-"
	if snapshot, _ := cursor["snapshot"].(string); !strings.HasPrefix(snapshot, wantPrefix) {
		t.Fatalf("ASSERT_LIST_OVERLAY_SNAPSHOT_BINDS_EVENT snapshot=%q want_prefix=%q", snapshot, wantPrefix)
	}
	firstEvent := m.nextManagerEvent
	second := m.List(ListOptions{Limit: 1, Cursor: first.NextCursor})
	if second.Code != "" || m.nextManagerEvent != firstEvent {
		t.Fatalf("ASSERT_LIST_CONTINUATION_NO_NEW_SNAPSHOT_EVENT page=%+v before=%d after=%d", second, firstEvent, m.nextManagerEvent)
	}
	for id, want := range beforeLastUse {
		if got := m.lifecycles[id].lastUseManagerSeq; got != want {
			t.Fatalf("ASSERT_LIST_OVERLAY_NO_LAST_USE session=%q before=%d after=%d", id, want, got)
		}
	}
}

func TestListCursorBindingAndInvalidity(t *testing.T) {
	m := paginationManager()
	first := m.List(ListOptions{Limit: 1})
	validShape := cursorPayload(t, first.NextCursor)
	invalid := map[string]ListOptions{
		"malformed":       {Limit: 1, Cursor: "%%%"},
		"unknown-version": {Limit: 1, Cursor: forgedCursor(t, validShape, "version", float64(99))},
		"filter-mismatch": {Limit: 1, Cursor: first.NextCursor, SessionID: "a"},
		"order-mismatch":  {Limit: 1, Cursor: forgedCursor(t, validShape, "order", "session-local")},
		"bad-boundary":    {Limit: 1, Cursor: forgedCursor(t, validShape, "snapshot", "unknown")},
		"bad-position":    {Limit: 1, Cursor: forgedCursor(t, validShape, "position", float64(3))},
	}
	for name, options := range invalid {
		t.Run(name, func(t *testing.T) {
			page := m.List(options)
			if page.Code != ListCursorInvalid || len(page.Records) != 0 || page.NextCursor != "" || page.Truncated {
				t.Fatalf("ASSERT_LIST_CURSOR_INVALID_EXPLICIT: page=%+v", page)
			}
		})
	}
}

func TestListCursorSnapshotDoesNotDriftAfterTransition(t *testing.T) {
	m := paginationManager()
	first := m.List(ListOptions{Limit: 1})
	m.Lifecycle(LifecycleRequest{SessionID: "c", Generation: 1, Operation: LifecycleStatus})
	second := m.List(ListOptions{Limit: 1, Cursor: first.NextCursor})
	third := m.List(ListOptions{Limit: 1, Cursor: second.NextCursor})
	got := append(append(ids(first.Records), ids(second.Records)...), ids(third.Records)...)
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("ASSERT_LIST_CURSOR_SNAPSHOT_STABLE: got=%v", got)
	}
}

func TestListBoundsSnapshotRetentionAndExpiry(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 300, MaxListSnapshots: 100})
	for i := 0; i < 300; i++ {
		m.RegisterLifecycle(string(rune(i+1)), 1, Stopped, true)
	}
	bounded := m.List(ListOptions{Limit: 1000})
	if len(bounded.Records) > 100 || !bounded.Truncated || bounded.NextCursor == "" {
		t.Fatalf("ASSERT_BOUNDED_CONFIGURED_LIST_MAX: page=%+v", bounded)
	}
	for i := 0; i < 65; i++ {
		_ = m.List(ListOptions{Limit: 1})
	}
	if expired := m.List(ListOptions{Limit: 1, Cursor: bounded.NextCursor}); expired.Code != ListCursorInvalid {
		t.Fatalf("ASSERT_BOUNDED_CURSOR_EXACT_INVALID_AFTER_EXPIRY: %+v", expired)
	}

	retained := paginationManager()
	for i := 0; i < 40; i++ {
		_ = retained.List(ListOptions{Limit: 1})
	}
	if len(retained.listSnapshots) > 16 || len(retained.issuedCursors) > 16 {
		t.Fatalf("ASSERT_BOUNDED_SNAPSHOT_RETENTION: snapshots=%d cursors=%d", len(retained.listSnapshots), len(retained.issuedCursors))
	}
}

func TestListPaginationBoundaries(t *testing.T) {
	empty, _ := NewManager(ManagerConfig{MaxSessions: 1})
	if page := empty.List(ListOptions{Limit: 2}); len(page.Records) != 0 || page.Truncated || page.NextCursor != "" {
		t.Fatalf("ASSERT_LIST_EMPTY_BOUNDARY: %+v", page)
	}
	m := paginationManager()
	if page := m.List(ListOptions{Limit: 3}); len(page.Records) != 3 || page.Truncated || page.NextCursor != "" {
		t.Fatalf("ASSERT_LIST_EXACT_LIMIT_BOUNDARY: %+v", page)
	}
	first := m.List(ListOptions{Limit: 2})
	second := m.List(ListOptions{Limit: 2, Cursor: first.NextCursor})
	if !first.Truncated || first.NextCursor == "" || second.Truncated || second.NextCursor != "" || len(second.Records) != 1 {
		t.Fatalf("ASSERT_LIST_MULTI_PAGE_BOUNDARY: first=%+v second=%+v", first, second)
	}
}

func paginationManager() *Manager {
	m, _ := NewManager(ManagerConfig{MaxSessions: 3})
	for _, id := range []string{"a", "b", "c"} {
		m.RegisterLifecycle(id, 1, Stopped, true)
	}
	return m
}

func cursorPayload(t *testing.T, cursor string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func forgedCursor(t *testing.T, base map[string]any, field string, value any) string {
	t.Helper()
	payload := make(map[string]any, len(base))
	for key, original := range base {
		payload[key] = original
	}
	payload[field] = value
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}
