package session

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestManagerStatusAllocatesExactlyOneSessionEvent(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("s", 1, Ready, false)
	before := m.lifecycles["s"].nextEvent
	got := m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStatus})
	if got.Failure != "" {
		t.Fatalf("ASSERT_STATUS_EVENT status failed: %+v", got)
	}
	if after := m.lifecycles["s"].nextEvent; after != before+1 {
		t.Fatalf("ASSERT_STATUS_EVENT exactly one event required: before=%d after=%d", before, after)
	}
	ledger := m.LifecycleLedger()
	if last := ledger[len(ledger)-1]; last.Action != "status" || last.Seq != before+1 {
		t.Fatalf("ASSERT_STATUS_EVENT authoritative ledger entry missing: %+v", last)
	}
}

func TestManagerLifecycleDecisionsAllocateExactlyOneSessionEventWithoutLastUse(t *testing.T) {
	tests := []struct {
		name    string
		state   State
		reaped  bool
		request LifecycleRequest
	}{
		{"status", Ready, false, LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStatus}},
		{"stale", Ready, false, LifecycleRequest{SessionID: "s", Generation: 2, Operation: LifecycleStatus}},
		{"unknown-operation", Ready, false, LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleOperation("UNKNOWN"), CallerID: "c"}},
		{"empty-caller", Ready, false, LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop}},
		{"stop-noop", Stopped, true, LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStop, CallerID: "c"}},
		{"restart-unreaped", Crashed, false, LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleRestart, CallerID: "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := NewManager(ManagerConfig{MaxSessions: 1})
			m.RegisterLifecycle("s", 1, tc.state, tc.reaped)
			beforeLocal := m.lifecycles["s"].nextEvent
			beforeLastUse := m.lifecycles["s"].lastUseManagerSeq
			beforeManager := m.nextManagerEvent
			m.Lifecycle(tc.request)
			if after := m.lifecycles["s"].nextEvent; after != beforeLocal+1 {
				t.Fatalf("ASSERT_LIFECYCLE_DECISION_ONE_SESSION_EVENT before=%d after=%d", beforeLocal, after)
			}
			if after := m.nextManagerEvent; after != beforeManager {
				t.Fatalf("ASSERT_LIFECYCLE_DECISION_NO_MANAGER_EVENT before=%d after=%d", beforeManager, after)
			}
			if after := m.lifecycles["s"].lastUseManagerSeq; after != beforeLastUse {
				t.Fatalf("ASSERT_LIFECYCLE_DECISION_NO_LAST_USE before=%d after=%d", beforeLastUse, after)
			}
		})
	}
}

func TestManagerSelectedLiveAdmissionAloneUpdatesLastUse(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	if got := m.Admit("s", 0); got.Kind != AdmissionFree {
		t.Fatalf("ASSERT_SELECTED_ADMISSION_SETUP: %+v", got)
	}
	before := m.lifecycles["s"].lastUseManagerSeq
	if got := m.Admit("s", 0); got.Kind != AdmissionFree {
		t.Fatalf("ASSERT_SELECTED_REUSE_ADMISSION: %+v", got)
	}
	if after := m.lifecycles["s"].lastUseManagerSeq; after <= before || after != m.nextManagerEvent {
		t.Fatalf("ASSERT_SELECTED_ADMISSION_UPDATES_LAST_USE before=%d after=%d manager=%d", before, after, m.nextManagerEvent)
	}
	selectedLastUse := m.lifecycles["s"].lastUseManagerSeq
	m.Lifecycle(LifecycleRequest{SessionID: "s", Generation: 1, Operation: LifecycleStatus})
	_ = m.List(ListOptions{})
	if after := m.lifecycles["s"].lastUseManagerSeq; after != selectedLastUse {
		t.Fatalf("ASSERT_ONLY_SELECTED_ADMISSION_UPDATES_LAST_USE before=%d after=%d", selectedLastUse, after)
	}
}

func TestManagerConcurrentListSnapshotsUseOneOrderedManagerEventEach(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 2, MaxListSnapshots: 64})
	m.RegisterLifecycle("a", 1, Ready, false)
	m.RegisterLifecycle("b", 1, Ready, false)
	const calls = 32
	before := m.nextManagerEvent
	cursors := make(chan string, calls)
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			page := m.List(ListOptions{Limit: 1})
			if page.Code != "" || page.NextCursor == "" {
				t.Errorf("ASSERT_LIST_CONCURRENT_SNAPSHOT_SETUP: %+v", page)
				return
			}
			cursors <- page.NextCursor
		}()
	}
	wg.Wait()
	close(cursors)
	if after := m.nextManagerEvent; after != before+calls {
		t.Fatalf("ASSERT_LIST_CONCURRENT_ONE_MANAGER_EVENT_EACH before=%d after=%d calls=%d", before, after, calls)
	}
	seen := make(map[string]uint64, calls)
	for encoded := range cursors {
		cursor, ok := decodeListCursor(encoded)
		if !ok {
			t.Fatalf("ASSERT_LIST_CONCURRENT_CURSOR_DECODES: %q", encoded)
		}
		var event uint64
		if _, err := fmt.Sscanf(cursor.Snapshot, "list-%d-", &event); err != nil {
			t.Fatalf("ASSERT_LIST_SNAPSHOT_ID_BINDS_EVENT snapshot=%q err=%v", cursor.Snapshot, err)
		}
		if event <= before || event > before+calls {
			t.Fatalf("ASSERT_LIST_SNAPSHOT_EVENT_BOUNDED event=%d before=%d calls=%d", event, before, calls)
		}
		if prior, duplicate := seen[cursor.Snapshot]; duplicate {
			t.Fatalf("ASSERT_LIST_SNAPSHOT_ID_UNIQUE snapshot=%q prior=%d event=%d", cursor.Snapshot, prior, event)
		}
		seen[cursor.Snapshot] = event
	}
	if len(seen) != calls {
		t.Fatalf("ASSERT_LIST_CONCURRENT_SNAPSHOT_COUNT got=%d want=%d", len(seen), calls)
	}
}

func TestManagerObservedLifecycleFailuresAllocateOneEvent(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.RegisterLifecycle("s", 1, Initializing, false)
	checks := []struct {
		name string
		act  func()
	}{
		{"completion-conflict", func() { m.CompleteLifecycleObserved("s", "missing", LifecycleCompletion{}) }},
		{"initialization-stale", func() { m.ObserveInitialization("s", 2, true) }},
		{"poison-stale", func() { m.ObservePoison("s", 2) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			before := m.lifecycles["s"].nextEvent
			check.act()
			if after := m.lifecycles["s"].nextEvent; after != before+1 {
				t.Fatalf("ASSERT_OBSERVED_LIFECYCLE_FAILURE_ONE_EVENT before=%d after=%d", before, after)
			}
		})
	}
}

func TestManagerListIsTransitionDriven(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 2})
	if got := m.Admit("admitted", 0); got.Kind != AdmissionFree {
		t.Fatalf("ASSERT_LIST_TRANSITION_ADMIT: %+v", got)
	}
	m.RegisterLifecycle("registered", 7, Initializing, false)
	assertListStates(t, m, map[string]string{"admitted": "STOPPED", "registered": "INITIALIZING"})
	if got := m.ObserveInitialization("registered", 7, true); got.State != Ready {
		t.Fatalf("ASSERT_LIST_TRANSITION_READY setup: %+v", got)
	}
	assertListStates(t, m, map[string]string{"admitted": "STOPPED", "registered": "READY"})
	status := m.Lifecycle(LifecycleRequest{SessionID: "registered", Generation: 7, Operation: LifecycleStatus})
	page := m.List(ListOptions{SessionID: "registered"})
	if status.State != Ready || len(page.Records) != 1 || page.Records[0].LocalSeq != m.lifecycles["registered"].nextEvent {
		t.Fatalf("ASSERT_LIST_TRANSITION_STATUS authoritative projection mismatch: status=%+v page=%+v", status, page)
	}
}

func TestManagerEvictionEligibilityMatrix(t *testing.T) {
	states := PublicStates()
	for _, state := range states {
		for _, work := range []bool{false, true} {
			for _, incumbent := range []bool{false, true} {
				for _, reaped := range []bool{false, true} {
					for _, reserved := range []bool{false, true} {
						for _, quarantined := range []bool{false, true} {
							for _, queued := range []int{0, 1} {
								for _, active := range []int{0, 1} {
									for _, unsafe := range []bool{false, true} {
										name := string(state)
										t.Run(name, func(t *testing.T) {
											m, _ := NewManager(ManagerConfig{MaxSessions: 1})
											m.Admit("victim", 0)
											s := m.lifecycles["victim"]
											s.state, s.reaped, s.hasWork, s.queued, s.active, s.unsafe = state, reaped, work, queued, active, unsafe
											if incumbent {
												s.incumbent = &lifecycleIntent{id: "busy", kind: StopIntent}
											}
											m.live["victim"].reserved, m.live["victim"].quarantined = reserved, quarantined
											got := m.Admit("contender", 0)
											terminal := state == Stopped || state == Crashed || state == Poisoned
											want := !work && !incumbent && !reserved && !quarantined && queued == 0 && active == 0 && !unsafe && (state == Ready || (terminal && reaped))
											if eligible := got.Kind == AdmissionEvict; eligible != want {
												t.Fatalf("ASSERT_EVICTION_ELIGIBILITY_MATRIX state=%s work=%v incumbent=%v reaped=%v reserved=%v quarantined=%v queued=%d active=%d unsafe=%v: got=%+v wantEligible=%v", state, work, incumbent, reaped, reserved, quarantined, queued, active, unsafe, got, want)
											}
										})
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestManagerEvictionTerminalClassPrecedesReady(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 2})
	m.Admit("ready-older", 0)
	m.Admit("terminal-newer", 0)
	m.lifecycles["ready-older"].state = Ready
	m.lifecycles["terminal-newer"].state = Stopped
	m.lifecycles["terminal-newer"].reaped = true
	got := m.Admit("contender", 0)
	if got.Kind != AdmissionEvict || got.Victim != "terminal-newer" {
		t.Fatalf("ASSERT_EVICTION_TERMINAL_BEFORE_READY: %+v", got)
	}
}

func TestManagerEvictionOverlayEquivalentAndPerturbations(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 2})
	m.Admit("starting-old", 0)
	m.Admit("ready-new", 0)
	m.lifecycles["starting-old"].state = Starting
	m.lifecycles["starting-old"].reaped = false
	m.lifecycles["ready-new"].state = Ready
	got := m.Admit("contender", 0)
	if got.Kind != AdmissionEvict || got.Victim != "ready-new" {
		t.Fatalf("ASSERT_EVICTION_OVERLAY_EQUIVALENT: %+v", got)
	}

	for _, perturb := range []struct {
		name  string
		apply func(*Manager)
	}{
		{"queued", func(m *Manager) { m.lifecycles["ready"].queued = 1 }},
		{"active", func(m *Manager) { m.lifecycles["ready"].active = 1 }},
		{"failed-evict", func(m *Manager) { m.live["ready"].quarantined = true }},
		{"reserved", func(m *Manager) { m.live["ready"].reserved = true }},
		{"unsafe", func(m *Manager) { m.lifecycles["ready"].unsafe = true }},
		{"unreaped-terminal", func(m *Manager) { m.lifecycles["ready"].state, m.lifecycles["ready"].reaped = Stopped, false }},
	} {
		t.Run(perturb.name, func(t *testing.T) {
			m, _ := NewManager(ManagerConfig{MaxSessions: 1})
			m.Admit("ready", 0)
			m.lifecycles["ready"].state = Ready
			perturb.apply(m)
			if got := m.Admit("contender", 0); got.Kind != AdmissionBlocked {
				t.Fatalf("ASSERT_EVICTION_PERTURBATION_%s: %+v", perturb.name, got)
			}
		})
	}
}

func assertListStates(t *testing.T, m *Manager, want map[string]string) {
	t.Helper()
	page := m.List(ListOptions{Limit: 100})
	got := make(map[string]string, len(page.Records))
	for _, record := range page.Records {
		got[record.SessionID] = record.State
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_LIST_TRANSITION_STATES: got=%v want=%v page=%+v", got, want, page)
	}
}
