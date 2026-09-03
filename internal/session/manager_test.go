package session

import (
	"reflect"
	"runtime"
	"sync"
	"testing"
)

func TestManagerCapacityAdmissionAndEviction(t *testing.T) {
	for _, invalid := range []int{0, -1} {
		if _, err := NewManager(ManagerConfig{MaxSessions: invalid}); err == nil {
			t.Fatalf("ASSERT capacity-positive-only: capacity %d accepted", invalid)
		}
	}
	m, err := NewManager(ManagerConfig{MaxSessions: 2})
	if err != nil {
		t.Fatalf("ASSERT capacity-arbitrary-positive: capacity 2 rejected: %v", err)
	}
	if _, err := NewManager(ManagerConfig{MaxSessions: 3}); err != nil {
		t.Fatalf("ASSERT capacity-arbitrary-positive: capacity 3 rejected: %v", err)
	}
	first := m.Admit("a", 10)
	second := m.Admit("b", 20)
	if first.Kind != AdmissionFree || second.Kind != AdmissionFree {
		t.Fatalf("ASSERT admission-free-slot-first: got %v, %v", first, second)
	}
	blocked1 := m.Admit("c", 30)
	repeated := m.Admit("c", 30)
	if repeated != blocked1 {
		t.Fatalf("ASSERT admission-one-victim-per-contender: first=%+v repeated=%+v", blocked1, repeated)
	}
	blocked2 := m.Admit("d", 40)
	if blocked1.Kind != AdmissionEvict || blocked1.Victim != "a" {
		t.Fatalf("ASSERT victim-deterministic: got %+v", blocked1)
	}
	if blocked2.Kind != AdmissionEvict || blocked2.Victim != "b" {
		t.Fatalf("ASSERT victim-distinct-reservation: got %+v", blocked2)
	}
	if got := m.LiveCount(); got != 2 {
		t.Fatalf("ASSERT capacity-no-oversubscription: got %d", got)
	}
	if got := m.CompleteEviction(blocked1.Reservation, true); got.Kind != AdmissionFree || got.SessionID != "c" {
		t.Fatalf("ASSERT eviction-success-releases-one-slot: got %+v", got)
	}
	if got := m.LiveCount(); got != 2 {
		t.Fatalf("ASSERT eviction-success-exactly-one-slot: got %d", got)
	}
}

func TestManagerFailedEvictionAndTerminalHistory(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.Admit("a", 10)
	blocked := m.Admit("b", 20)
	failed := m.CompleteEviction(blocked.Reservation, false)
	if failed.Kind != AdmissionBlocked || m.LiveCount() != 1 {
		t.Fatalf("ASSERT eviction-failure-retains-capacity: got %+v live=%d", failed, m.LiveCount())
	}
	if got := m.Admit("c", 30); got.Kind != AdmissionBlocked {
		t.Fatalf("ASSERT failed-victim-quarantined: got %+v", got)
	}
	page := m.List(ListOptions{Limit: 10})
	if len(page.Records) != 1 || page.Records[0].State != "POISONED" {
		t.Fatalf("ASSERT failed-evict-authoritative-state-retained: got %+v", page)
	}
}

func TestManagerOrderingPaginationAndDeterminism(t *testing.T) {
	build := func() *Manager {
		m, _ := NewManager(ManagerConfig{MaxSessions: 3})
		for _, id := range []string{"a", "b", "c"} {
			m.RegisterLifecycle(id, 1, Stopped, true)
		}
		return m
	}
	m := build()
	first := m.List(ListOptions{Limit: 2})
	if got := ids(first.Records); !reflect.DeepEqual(got, []string{"a", "b"}) || !first.Truncated || first.NextCursor == "" {
		t.Fatalf("ASSERT manager-global-order-and-pagination: got=%v page=%+v", got, first)
	}
	second := m.List(ListOptions{Limit: 2, Cursor: first.NextCursor})
	if got := ids(second.Records); !reflect.DeepEqual(got, []string{"c"}) || second.Truncated {
		t.Fatalf("ASSERT pagination-boundaries: got %v truncated=%v", got, second.Truncated)
	}
	if other := build().List(ListOptions{Limit: 2}); !reflect.DeepEqual(first, other) {
		t.Fatalf("ASSERT equivalent-inputs-deterministic: first=%+v other=%+v", first, other)
	}

	tied, _ := NewManager(ManagerConfig{MaxSessions: 2})
	tied.Admit("b", 1)
	tied.Admit("a", 1)
	if got := tied.Admit("c", 2); got.Victim != "b" {
		t.Fatalf("ASSERT manager-owned-admission-order: got %+v", got)
	}
}

func TestManagerOwnsAdmissionSequenceAndEvictionLifecycle(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 2})
	m.Admit("a", 900)
	m.Admit("b", 1)
	if m.live["a"].managerSeq != 1 || m.live["b"].managerSeq != 2 {
		t.Fatalf("ASSERT_AUTHORITY_MANAGER_ALLOCATES_ADMISSION_SEQUENCE: a=%d b=%d", m.live["a"].managerSeq, m.live["b"].managerSeq)
	}
	pending := m.Admit("c", 1)
	victim := m.lifecycles[pending.Victim]
	if victim == nil || victim.incumbent == nil || victim.incumbent.kind != EvictIntent {
		t.Fatalf("ASSERT_AUTHORITY_EVICT_IS_LIFECYCLE_INTENT: admission=%+v victim=%+v", pending, victim)
	}
	conflict := m.Lifecycle(LifecycleRequest{SessionID: pending.Victim, Generation: victim.generation, Operation: LifecycleStop, CallerID: "public"})
	if conflict.Failure != LifecycleConflict {
		t.Fatalf("ASSERT_AUTHORITY_EVICT_PUBLIC_CONFLICT: %+v", conflict)
	}
}

func TestManagerEvictionReevaluatesWaitersByAdmissionSequence(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 2})
	m.Admit("a", 1)
	m.Admit("b", 2)
	earlier := m.Admit("c", 3)
	later := m.Admit("d", 4)
	got := m.CompleteEviction(later.Reservation, true)
	if got.Kind != AdmissionFree || got.SessionID != "c" || m.live["c"] == nil || m.live["d"] != nil {
		t.Fatalf("ASSERT_AUTHORITY_EVICT_REEVALUATES_WAITERS_ASCENDING: earlier=%+v later=%+v completed=%+v live=%v", earlier, later, got, m.live)
	}
	assertSupersededEviction(t, m, earlier, "a")
	pending, ok := reservationForContender(m, "d")
	if !ok || pending.victim != "a" || pending.id == later.Reservation || pending.id == earlier.Reservation || !m.live["a"].reserved {
		t.Fatalf("ASSERT_AUTHORITY_LATER_WAITER_RERESERVES_ELIGIBLE_VICTIM: reservations=%v live=%v", m.reservations, m.live)
	}
	late := m.CompleteEviction(earlier.Reservation, true)
	if late.Kind != AdmissionBlocked || m.live["c"] == nil || m.reservations[pending.id].contender != "d" {
		t.Fatalf("ASSERT_AUTHORITY_SUPERSEDED_COMPLETION_LATE_NOOP: late=%+v live=%v reservations=%v", late, m.live, m.reservations)
	}
}

func TestReleaseStoppedRequiresExactSafeGenerationAndRetainsHistory(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	if got := m.Admit("a", 0); got.Kind != AdmissionFree {
		t.Fatal(got)
	}
	if page := m.List(ListOptions{SessionID: "a", Limit: 1}); len(page.Records) != 1 || page.Records[0].State != string(Stopped) {
		t.Fatalf("ASSERT_ALGEBRA_RELEASE_CONFIRMED_STOP: page=%+v", page)
	}
	if m.ReleaseStopped("a", 2) || !m.ReleaseStopped("a", 1) || m.LiveCount() != 0 {
		t.Fatalf("ASSERT_ALGEBRA_RELEASE_EXACT_GENERATION: live=%d", m.LiveCount())
	}
	if page := m.List(ListOptions{SessionID: "a", Limit: 1}); len(page.Records) != 1 || page.Records[0].State != string(Stopped) {
		t.Fatalf("ASSERT_ALGEBRA_RELEASE_RETAINS_HISTORY: page=%+v", page)
	}
}

func TestManagerEvictionMultipleReleasedSlotsAndReversedCompletion(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 3})
	for _, id := range []string{"a", "b", "c"} {
		m.Admit(id, 0)
	}
	earlier := m.Admit("d", 0)
	middle := m.Admit("e", 0)
	later := m.Admit("f", 0)

	first := m.CompleteEviction(later.Reservation, true)
	second := m.CompleteEviction(middle.Reservation, true)
	if first.Kind != AdmissionFree || first.SessionID != "d" || second.Kind != AdmissionFree || second.SessionID != "e" {
		t.Fatalf("ASSERT_AUTHORITY_MULTIPLE_RELEASED_SLOTS_ASCENDING: earlier=%+v middle=%+v later=%+v first=%+v second=%+v", earlier, middle, later, first, second)
	}
	if m.live["d"] == nil || m.live["e"] == nil || m.live["f"] != nil || len(m.live) != 3 {
		t.Fatalf("ASSERT_AUTHORITY_MULTIPLE_RELEASED_SLOTS_CAPACITY: live=%v", m.live)
	}
}

func TestManagerEvictionMaxSessionsOneAndLateSupersededCompletion(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.Admit("a", 0)
	earlier := m.Admit("b", 0)
	if repeated := m.Admit("b", 0); repeated != earlier || len(m.waiters) != 1 {
		t.Fatalf("ASSERT_AUTHORITY_MAX_ONE_DUPLICATE_CONTENDER: first=%+v repeated=%+v waiters=%v", earlier, repeated, m.waiters)
	}
	later := m.Admit("c", 0)
	if later.Kind != AdmissionBlocked || len(m.waiters) != 1 {
		t.Fatalf("ASSERT_AUTHORITY_WAITER_BOUND_BY_CAPACITY: later=%+v waiters=%v", later, m.waiters)
	}

	m.mu.Lock()
	m.releaseVictimSlot(m.reservations[earlier.Reservation])
	m.reprocessWaiters()
	m.mu.Unlock()
	if m.live["b"] == nil || len(m.live) != 1 {
		t.Fatalf("ASSERT_AUTHORITY_MAX_ONE_SLOT_ASSIGNMENT: live=%v", m.live)
	}
	late := m.CompleteEviction(earlier.Reservation, true)
	if late.Kind != AdmissionBlocked || m.live["b"] == nil || len(m.live) != 1 || len(m.reservations) != 0 {
		t.Fatalf("ASSERT_AUTHORITY_SUPERSEDED_COMPLETION_LATE_NOOP: late=%+v live=%v reservations=%v", late, m.live, m.reservations)
	}
}

func TestManagerFailedVictimQuarantinePreservesLaterWaiterState(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 2})
	m.Admit("a", 0)
	m.Admit("b", 0)
	first := m.Admit("c", 0)
	second := m.Admit("d", 0)
	failed := m.CompleteEviction(first.Reservation, false)
	if failed.Kind != AdmissionBlocked || !m.live["a"].quarantined || m.live["a"].reserved {
		t.Fatalf("ASSERT_AUTHORITY_FAILED_VICTIM_QUARANTINE: failed=%+v a=%+v", failed, m.live["a"])
	}
	pending, ok := m.reservations[second.Reservation]
	if !ok || pending.victim != "b" || pending.contender != "d" || len(m.waiters) != 1 {
		t.Fatalf("ASSERT_AUTHORITY_FAILED_VICTIM_PRESERVES_DISTINCT_WAITER: reservations=%v waiters=%v", m.reservations, m.waiters)
	}
}

func TestManagerEvictionConcurrentStressNoOrphansAndCapacityBound(t *testing.T) {
	for attempt := 0; attempt < 200; attempt++ {
		m, _ := NewManager(ManagerConfig{MaxSessions: 4})
		for _, id := range []string{"a", "b", "c", "d"} {
			m.Admit(id, 0)
		}
		pending := []Admission{m.Admit("e", 0), m.Admit("f", 0), m.Admit("g", 0), m.Admit("h", 0)}
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(len(pending))
		for i := range pending {
			i := i
			go func() {
				defer wg.Done()
				<-start
				m.CompleteEviction(pending[len(pending)-1-i].Reservation, true)
			}()
		}
		close(start)
		wg.Wait()

		m.mu.Lock()
		if len(m.live) > m.max || len(m.waiters) > m.max {
			t.Fatalf("ASSERT_AUTHORITY_CONCURRENT_CAPACITY_AND_WAITER_BOUND: attempt=%d live=%d waiters=%d", attempt, len(m.live), len(m.waiters))
		}
		for id, reservation := range m.reservations {
			victim := m.live[reservation.victim]
			lifecycle := m.lifecycles[reservation.victim]
			if victim == nil || !victim.reserved || lifecycle == nil || lifecycle.incumbent == nil || lifecycle.incumbent.kind != EvictIntent {
				t.Fatalf("ASSERT_AUTHORITY_CONCURRENT_NO_ORPHAN_RESERVATION: attempt=%d id=%s reservation=%+v", attempt, id, reservation)
			}
		}
		for sessionID, lifecycle := range m.lifecycles {
			if lifecycle.incumbent != nil && lifecycle.incumbent.kind == EvictIntent {
				if victim := m.live[sessionID]; victim == nil || !victim.reserved {
					t.Fatalf("ASSERT_AUTHORITY_CONCURRENT_NO_ORPHAN_INCUMBENT: attempt=%d session=%s", attempt, sessionID)
				}
			}
		}
		m.mu.Unlock()
	}
}

func reservationForContender(m *Manager, contender string) (reservation, bool) {
	for _, pending := range m.reservations {
		if pending.contender == contender {
			return pending, true
		}
	}
	return reservation{}, false
}

func assertSupersededEviction(t *testing.T, m *Manager, admission Admission, victimID string) {
	t.Helper()
	lifecycle := m.lifecycles[victimID]
	if lifecycle == nil || len(lifecycle.history) == 0 {
		t.Fatalf("ASSERT_AUTHORITY_SUPERSEDED_EVICT_HISTORY_RETAINED: admission=%+v lifecycle=%+v", admission, lifecycle)
	}
	result := lifecycle.history[len(lifecycle.history)-1].result
	if result.Kind != EvictIntent || !result.Superseded || result.Failure != "" {
		t.Fatalf("ASSERT_AUTHORITY_SUPERSEDED_EVICT_TERMINAL_RESULT: result=%+v", result)
	}
}

func TestManagerEvictionMissingVictimFailsClosed(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.Admit("victim", 1)
	pending := m.Admit("contender", 2)
	delete(m.live, pending.Victim)
	got := m.CompleteEviction(pending.Reservation, false)
	if got.Kind != AdmissionBlocked || got.SessionID != "contender" || got.Victim != "victim" {
		t.Fatalf("ASSERT_AUTHORITY_EVICT_MISSING_VICTIM_FAILS_CLOSED: %+v", got)
	}
}

func TestManagerConcurrentDuplicateContenderAdmission(t *testing.T) {
	for attempt := 0; attempt < 200; attempt++ {
		m, _ := NewManager(ManagerConfig{MaxSessions: 1})
		m.Admit("victim", 1)
		results := concurrentAdmissions(m, []string{"contender", "contender"}, 2)
		if results[0].Kind != AdmissionEvict || results[1] != results[0] {
			t.Fatalf("ASSERT concurrent-duplicate-contender-one-reservation: attempt=%d results=%+v", attempt, results)
		}
		if got := len(m.reservations); got != 1 {
			t.Fatalf("ASSERT concurrent-duplicate-contender-no-orphan-reservation: attempt=%d reservations=%d", attempt, got)
		}
	}
}

func TestManagerConcurrentDistinctContendersAtCapacity(t *testing.T) {
	for attempt := 0; attempt < 200; attempt++ {
		m, _ := NewManager(ManagerConfig{MaxSessions: 2})
		m.Admit("a", 1)
		m.Admit("b", 2)
		results := concurrentAdmissions(m, []string{"c", "d"}, 3)
		if results[0].Kind != AdmissionEvict || results[1].Kind != AdmissionEvict || results[0].Reservation == results[1].Reservation || results[0].Victim == results[1].Victim {
			t.Fatalf("ASSERT concurrent-distinct-contenders-distinct-victims: attempt=%d results=%+v", attempt, results)
		}
		if got := m.LiveCount(); got != 2 {
			t.Fatalf("ASSERT concurrent-distinct-contenders-capacity-bound: attempt=%d live=%d", attempt, got)
		}
	}
}

func TestManagerConcurrentEvictionCompletionAndAdmission(t *testing.T) {
	for attempt := 0; attempt < 200; attempt++ {
		m, _ := NewManager(ManagerConfig{MaxSessions: 2})
		m.Admit("a", 1)
		m.Admit("b", 2)
		pending := m.Admit("c", 3)

		start := make(chan struct{})
		var complete, admit Admission
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			complete = m.CompleteEviction(pending.Reservation, true)
		}()
		go func() {
			defer wg.Done()
			<-start
			admit = m.Admit("d", 4)
		}()
		close(start)
		wg.Wait()

		if complete.Kind != AdmissionFree || complete.SessionID != "c" || admit.Kind != AdmissionEvict || admit.Victim != "b" {
			t.Fatalf("ASSERT concurrent-eviction-admission-serial-result: attempt=%d complete=%+v admit=%+v", attempt, complete, admit)
		}
		if len(m.reservations) != 1 || m.live["c"] == nil || m.live["b"] == nil || m.live["a"] != nil {
			t.Fatalf("ASSERT concurrent-eviction-admission-no-orphan: attempt=%d live=%v reservations=%v", attempt, m.live, m.reservations)
		}
	}
}

func TestManagerConcurrentLifecycleListAndCount(t *testing.T) {
	m, _ := NewManager(ManagerConfig{MaxSessions: 1})
	m.Admit("live", 1)
	const observations = 1000
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < observations; i++ {
			m.Lifecycle(LifecycleRequest{SessionID: "live", Generation: 1, Operation: LifecycleStatus})
			runtime.Gosched()
		}
	}()
	for reader := 0; reader < 2; reader++ {
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < observations; i++ {
				page := m.List(ListOptions{})
				if len(page.Records) != 1 || page.Records[0].SessionID != "live" || m.LiveCount() != 1 {
					t.Errorf("ASSERT concurrent-lifecycle-list-coherent: page=%+v", page)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	page := m.List(ListOptions{})
	page.Records[0].State = "MUTATED"
	if got := m.List(ListOptions{Limit: 1}).Records[0].State; got != "STOPPED" {
		t.Fatalf("ASSERT concurrent-lifecycle-return-does-not-alias: got=%q", got)
	}
}

func TestManagerConcurrentOperationsRepeated(t *testing.T) {
	for attempt := 0; attempt < 25; attempt++ {
		m, _ := NewManager(ManagerConfig{MaxSessions: 2})
		m.Admit("a", 1)
		m.Admit("b", 2)
		pending := m.Admit("c", 3)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(5)
		go func() { defer wg.Done(); <-start; m.Admit("d", 4) }()
		go func() { defer wg.Done(); <-start; m.CompleteEviction(pending.Reservation, true) }()
		go func() {
			defer wg.Done()
			<-start
			m.Lifecycle(LifecycleRequest{SessionID: "b", Generation: 1, Operation: LifecycleStatus})
		}()
		go func() { defer wg.Done(); <-start; _ = m.List(ListOptions{}) }()
		go func() { defer wg.Done(); <-start; _ = m.LiveCount() }()
		close(start)
		wg.Wait()
		if got := m.LiveCount(); got > 2 {
			t.Fatalf("ASSERT concurrent-repeated-capacity-bound: attempt=%d live=%d", attempt, got)
		}
	}
}

func concurrentAdmissions(m *Manager, sessionIDs []string, managerSeq uint64) []Admission {
	results := make([]Admission, len(sessionIDs))
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(sessionIDs))
	for i := range sessionIDs {
		i := i
		go func() {
			defer wg.Done()
			<-start
			results[i] = m.Admit(sessionIDs[i], managerSeq+uint64(i))
		}()
	}
	close(start)
	wg.Wait()
	return results
}

func ids(records []TerminalRecord) []string {
	out := make([]string, len(records))
	for i := range records {
		out[i] = records[i].SessionID
	}
	return out
}
func localSeqs(records []TerminalRecord) []uint64 {
	out := make([]uint64, len(records))
	for i := range records {
		out[i] = records[i].LocalSeq
	}
	return out
}
