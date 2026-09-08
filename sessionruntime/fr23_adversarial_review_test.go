package sessionruntime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/manageddiagnostic"
)

func TestReviewAttemptIDDoesNotLeakInjectedPreimage(t *testing.T) {
	const secret = "/caller/private/path/SECRET_NONCE"
	cfg := attemptConfig(failingStarter{reason: "process start failed"}, manageddiagnostic.StartupAttemptID(secret))
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	id := string(m.Start(context.Background(), StartRequest{Profile: profile(t)}).AttemptID)
	if strings.Contains(id, secret) || strings.Contains(id, "SECRET_NONCE") {
		t.Fatalf("ASSERT_FR23_ATTEMPT_ID_PREIMAGE_WITHHELD: %q", id)
	}
	if len(id) != 64 {
		t.Fatalf("ASSERT_FR23_ATTEMPT_ID_FIXED_FORMAT: len=%d id=%q", len(id), id)
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("ASSERT_FR23_ATTEMPT_ID_PATH_SAFE: %q", id)
		}
	}
}

func TestReviewDuplicateInjectedAttemptIDDoesNotAlias(t *testing.T) {
	cfg := attemptConfig(failingStarter{reason: "process start failed"}, "duplicate", "duplicate")
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	first := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	second := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if first.AttemptID == second.AttemptID {
		t.Fatalf("duplicate IDs accepted: first=%q second=%q lookup=%+v", first.AttemptID, second.AttemptID, m.GetStartupAttempt(second.AttemptID))
	}
}

func TestReviewConcurrentDuplicateInjectedAttemptIDDoesNotAlias(t *testing.T) {
	cfg := attemptConfig(failingStarter{reason: "process start failed"}, "duplicate", "duplicate")
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
		if seen[id] {
			t.Fatalf("concurrent duplicate ID accepted: %q", id)
		}
		seen[id] = true
	}
}

func TestReviewRetentionFailureIsObservable(t *testing.T) {
	cfg := attemptConfig(failingStarter{reason: "process start failed"}, "attempt")
	cfg.Diagnostics = manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 1, MaxBytes: 1})
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("startup returned an unlookupable attempt after retention refusal")
		}
	}()
	_ = m.Start(context.Background(), StartRequest{Profile: profile(t)})
}

func TestReviewReturnedAdmissionMutationDoesNotAffectLookup(t *testing.T) {
	cfg := attemptConfig(oneChildStarter{referenceChild{}}, "attempt-success")
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	q := m.GetStartupAttempt(got.AttemptID)
	q.Record.Admission.SessionID = "forged"
	if m.GetStartupAttempt(got.AttemptID).Record.Admission.SessionID == "forged" {
		t.Fatal("admission alias")
	}
}

func TestReviewSuccessfulReadinessRecordsCompletion(t *testing.T) {
	store := manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 8, MaxBytes: 16 * 1024})
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 32, MaxOperations: 2}, Starter: oneChildStarter{newReadinessChild("ready")}, Diagnostics: store})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, ok := m.WaitReadiness(context.Background(), pending.ID)
	if !ok || ready.State != ReadinessReady {
		t.Fatalf("readiness failed: %+v", ready)
	}
	records := m.Diagnostics(started.SessionID, started.Generation).Records
	var completion *manageddiagnostic.Record
	for i := range records {
		if err := manageddiagnostic.Validate(records[i]); err != nil {
			t.Fatalf("invalid readiness diagnostic %+v: %v", records[i], err)
		}
		if records[i].Reason.Value == "readiness-complete" {
			completion = &records[i]
		}
	}
	if completion == nil || completion.Phase != manageddiagnostic.PhaseReadinessComplete || completion.Substep.Value != manageddiagnostic.SubstepInitializedNotification || completion.Write.Messages != 2 || completion.Read.Messages != ready.ResponseMessages {
		t.Fatalf("missing truthful successful readiness diagnostic: ready=%+v records=%+v", ready, records)
	}
}

func TestReviewEffectiveLimitsReflectDefaults(t *testing.T) {
	m, s, _ := diagnosticManager(t, "success")
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "x", Deadline: time.Now().Add(time.Second), MaxMessages: 3})
	if got.Failure != "" {
		t.Fatal(got.Failure)
	}
	r := m.Diagnostics(s.SessionID, s.Generation).Records[0]
	if r.Limits.EffectiveMaxMessages.Value != 3 || r.Limits.EffectiveMaxBytes.Value <= 0 {
		t.Fatalf("effective limits do not match runtime defaults: %+v", r.Limits)
	}
}
