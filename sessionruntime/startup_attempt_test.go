package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
)

const (
	assertAttemptFailedLookup = "ASSERT_FR23_FAILED_ATTEMPT_LOOKUP_NO_ADMISSION"
	assertAttemptSuccessBind  = "ASSERT_FR23_SUCCESS_ATTEMPT_EXACT_BINDING"
	assertAttemptUnique       = "ASSERT_FR23_ATTEMPT_IDS_UNIQUE"
	assertAttemptEviction     = "ASSERT_FR23_ATTEMPT_EVICTION_EXPLICIT"
	assertAttemptClone        = "ASSERT_FR23_ATTEMPT_QUERY_CLONE"
	assertAttemptValidation   = "ASSERT_FR23_ATTEMPT_SCHEMA_TAMPER_REJECTED"
	assertAttemptRestart      = "ASSERT_FR23_RESTART_ATTEMPT_EXACT_NON1_GENERATION"
)

type failingStarter struct{ reason string }
type unavailableStarter struct{}

func (unavailableStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	return nil, managedprocess.StartObservation{Kind: managedprocess.StartUnavailable, Reason: "containment unavailable"}
}

func (s failingStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	return nil, managedprocess.StartObservation{Kind: managedprocess.StartFailed, Reason: s.reason, Err: errors.New("SECRET raw spawn failure")}
}

func attemptConfig(starter Starter, ids ...manageddiagnostic.StartupAttemptID) Config {
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 2, MaxBytes: 4096})
	var mu sync.Mutex
	n := 0
	return Config{
		Limits:  Limits{MaxSessions: 4, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16},
		Starter: starter, Diagnostics: store,
		startupAttemptRandom: bytes.NewReader(make([]byte, 16)),
		startupAttemptEntropy: func(uint64) []byte {
			mu.Lock()
			defer mu.Unlock()
			id := ids[n]
			n++
			return []byte(id)
		},
	}
}

func TestStartupAttemptFailedLookupNoAdmissionAndSafe(t *testing.T) {
	cfg := attemptConfig(failingStarter{reason: "process start failed"}, "attempt-a")
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if got.AttemptID == "" {
		t.Fatalf("%s: result=%+v", assertAttemptFailedLookup, got)
	}
	q := m.GetStartupAttempt(got.AttemptID)
	if q.Status != manageddiagnostic.AttemptAvailable || q.Record == nil || q.Record.Admission != nil || q.Record.Outcome != manageddiagnostic.StartupFailed {
		t.Fatalf("%s: query=%+v", assertAttemptFailedLookup, q)
	}
	if q.Record.Reason.Value != "process-start-failed" || q.Record.Stderr.Status != manageddiagnostic.Withheld {
		t.Fatalf("ASSERT_FR23_SAFE_STATIC_START_REASON: %+v", q.Record)
	}
	raw, _ := json.Marshal(q.Record)
	for _, secret := range []string{"SECRET", "raw spawn", "command", "args", "env", "stderr_bytes"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("ASSERT_FR23_START_RECORD_NO_RAW_SECRET: %q in %s", secret, raw)
		}
	}
}

func TestStartupAttemptUnavailableAndEarlyDeadlineStillRetained(t *testing.T) {
	for _, tc := range []struct {
		name       string
		starter    Starter
		request    StartRequest
		wantReason string
	}{
		{name: "unavailable", starter: unavailableStarter{}, request: StartRequest{Profile: profile(t)}, wantReason: "containment-unavailable"},
		{name: "deadline", starter: failingStarter{}, request: StartRequest{Profile: profile(t), Deadline: time.Unix(1, 0)}, wantReason: "request-timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := attemptConfig(tc.starter, manageddiagnostic.StartupAttemptID("attempt-"+tc.name))
			m, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			got := m.Start(context.Background(), tc.request)
			q := m.GetStartupAttempt(got.AttemptID)
			if got.AttemptID == "" || q.Record == nil || q.Record.Admission != nil || q.Record.Reason.Value != tc.wantReason {
				t.Fatalf("ASSERT_FR23_EVERY_START_HAS_ATTEMPT_%s: result=%+v query=%+v", tc.name, got, q)
			}
		})
	}
}

func TestStartupAttemptSuccessBindsExactGeneration(t *testing.T) {
	cfg := attemptConfig(oneChildStarter{referenceChild{}}, "attempt-success")
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	q := m.GetStartupAttempt(got.AttemptID)
	if got.AttemptID == "" || got.Generation != 1 || q.Record == nil || q.Record.Admission == nil || q.Record.Admission.SessionID != got.SessionID || q.Record.Admission.Generation != got.Generation {
		t.Fatalf("%s: result=%+v query=%+v", assertAttemptSuccessBind, got, q)
	}
}

func TestStartupAttemptUniqueConcurrentAndEvictionClone(t *testing.T) {
	cfg := attemptConfig(failingStarter{reason: "stdin pipe failed"}, "attempt-1", "attempt-2", "attempt-3")
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	ids := make(chan manageddiagnostic.StartupAttemptID, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids <- m.Start(context.Background(), StartRequest{Profile: profile(t)}).AttemptID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[manageddiagnostic.StartupAttemptID]bool{}
	for id := range ids {
		if id == "" || seen[id] {
			t.Fatalf("%s: %q", assertAttemptUnique, id)
		}
		seen[id] = true
	}
	third := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	evicted := 0
	for id := range seen {
		if q := m.GetStartupAttempt(id); q.Status == manageddiagnostic.AttemptEvicted {
			evicted++
		}
	}
	if evicted != 1 {
		t.Fatalf("%s: evicted=%d", assertAttemptEviction, evicted)
	}
	q := m.GetStartupAttempt(third.AttemptID)
	q.Record.SafeSubcodes = append(q.Record.SafeSubcodes, "mutated")
	if len(m.GetStartupAttempt(third.AttemptID).Record.SafeSubcodes) != 1 {
		t.Fatal(assertAttemptClone)
	}
}

func TestStartupAttemptReadinessSharesAttemptIdentity(t *testing.T) {
	child := newReadinessChild("ready")
	m, started := readinessManager(t, child)
	pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	if pending.AttemptID == "" || pending.AttemptID != started.AttemptID || pending.Generation != started.Generation {
		t.Fatalf("ASSERT_FR23_READINESS_ATTEMPT_CORRELATION: start=%+v readiness=%+v", started, pending)
	}
	_, _ = m.WaitReadiness(context.Background(), pending.ID)
}

func TestStartupAttemptRestartBindsActualNextGeneration(t *testing.T) {
	starter := &sequenceStarter{children: []Child{referenceChild{}, referenceChild{}}}
	cfg := attemptConfig(starter, "attempt-initial", "attempt-restart")
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	first := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	accepted := m.Restart(context.Background(), first.SessionID, "attempt-restart-caller")
	terminal := waitOperation(t, m, accepted.IntentID, OperationComplete)
	q := m.GetStartupAttempt(deriveStartupAttemptID([16]byte{}, 2, []byte("attempt-restart")))
	if terminal.Failure != "" || q.Record == nil || q.Record.Admission == nil || q.Record.Admission.Generation != 2 || q.Record.Admission.SessionID != first.SessionID {
		t.Fatalf("%s: operation=%+v query=%+v", assertAttemptRestart, terminal, q)
	}
	if original := m.GetStartupAttempt(first.AttemptID); original.Record == nil || original.Record.Admission == nil || original.Record.Admission.Generation != 1 {
		t.Fatalf("ASSERT_FR23_STALE_GENERATION_NOT_OVERWRITTEN: %+v", original)
	}
}

func TestStartupAttemptValidationRejectsFakeAdmission(t *testing.T) {
	r := manageddiagnostic.StartupAttemptRecord{SchemaVersion: manageddiagnostic.StartupAttemptSchemaVersion, AttemptID: "attempt-x", Sequence: 1, Outcome: manageddiagnostic.StartupFailed, Reason: manageddiagnostic.Fact[string]{Status: manageddiagnostic.Observed, Value: "process-start-failed"}, Admission: &manageddiagnostic.StartupAdmission{SessionID: "caller", Generation: 99}, Stderr: manageddiagnostic.Stderr{Status: manageddiagnostic.Withheld}, ProcessExit: manageddiagnostic.ProcessExit{Status: manageddiagnostic.Unavailable}}
	if manageddiagnostic.ValidateStartupAttempt(r) == nil {
		t.Fatal(assertAttemptValidation)
	}
	r.Admission = nil
	for name, mutate := range map[string]func(*manageddiagnostic.StartupAttemptRecord){
		"version":              func(r *manageddiagnostic.StartupAttemptRecord) { r.SchemaVersion = "v2" },
		"generation-shaped-id": func(r *manageddiagnostic.StartupAttemptRecord) { r.AttemptID = "42" },
	} {
		bad := r.Clone()
		mutate(&bad)
		if manageddiagnostic.ValidateStartupAttempt(bad) == nil {
			t.Errorf("%s_%s", assertAttemptValidation, name)
		}
	}
}
