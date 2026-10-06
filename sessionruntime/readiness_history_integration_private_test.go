package sessionruntime

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestC06ReadinessHistoryExactThreeFrames(t *testing.T) {
	child := newReadinessChild("readiness-history-noncanonical")
	m, started := readinessManager(t, child)
	pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, ok := m.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != ReadinessReady {
		t.Fatalf("ASSERT_C06_READINESS_READY: %+v found=%v", ready, ok)
	}
	m.mu.Lock()
	h := m.sessions[started.SessionID].readinessHistory
	if h == nil || h.entries != 3 {
		m.mu.Unlock()
		t.Fatalf("ASSERT_C06_READINESS_HISTORY_THREE: history=%+v", h)
	}
	entries := []privateHistoryEntry{h.chunks[0][0], h.chunks[0][1], h.chunks[0][2]}
	m.mu.Unlock()
	if entries[0].direction != privateHistoryOutbound || entries[1].direction != privateHistoryInbound || entries[2].direction != privateHistoryOutbound {
		t.Fatalf("ASSERT_C06_READINESS_HISTORY_DIRECTIONS: %+v", entries)
	}
	if !bytes.Contains(entries[0].frame, []byte(`"method":"initialize"`)) || !bytes.Contains(entries[1].frame, []byte("content-length :")) || !bytes.Contains(entries[1].frame, []byte("X-Test: exact")) || !bytes.Contains(entries[2].frame, []byte(`"method":"initialized"`)) {
		t.Fatalf("ASSERT_C06_READINESS_HISTORY_EXACT: request=%q response=%q initialized=%q", entries[0].frame, entries[1].frame, entries[2].frame)
	}
}

func TestC06ReadinessFailuresDoNotAppendUnconfirmedFrames(t *testing.T) {
	for _, mode := range []string{"wrong-id", "malformed", "error"} {
		t.Run(mode, func(t *testing.T) {
			m, started := readinessManager(t, newReadinessChild(mode))
			pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
			ready, _ := m.WaitReadiness(context.Background(), pending.ID)
			if ready.State != ReadinessFailed {
				t.Fatalf("ASSERT_C06_READINESS_FAILURE: %+v", ready)
			}
			m.mu.Lock()
			entries := m.sessions[started.SessionID].readinessHistory.entries
			m.mu.Unlock()
			if entries != 1 {
				t.Fatalf("ASSERT_C06_READINESS_FAILURE_APPEND: entries=%d", entries)
			}
		})
	}
}
