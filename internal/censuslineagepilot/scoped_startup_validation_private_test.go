//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/sessionruntime"
)

func TestScopedLiveRehearsalStartupValidationPrivate(t *testing.T) {
	for _, kind := range []string{"exact-server", "wrong-server", "stale-metadata-generation", "missing-metadata", "wrong-workspace", "wrong-snapshot-session", "wrong-snapshot-generation", "not-ready", "not-found"} {
		t.Run(kind, func(t *testing.T) {
			h, m, cfg, f := pilot.NewScopedRehearsalFixturePrivate(t)
			ctx := context.Background()
			ready := m.BeginReadiness(ctx, cfg.Session.SessionID, cfg.Session.Generation, time.Now().Add(time.Second))
			ready, found := m.WaitReadiness(ctx, ready.ID)
			if !found || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
				t.Fatalf("fake initialize prerequisite: %+v", ready)
			}
			metadata, failure := m.Metadata(cfg.Session.SessionID, cfg.Session.Generation)
			if ready.Metadata.ServerCommand != "" || failure != "" || metadata.ServerCommand != cfg.Server {
				t.Fatalf("distinct metadata contracts: snapshot=%q exact=%q failure=%s", ready.Metadata.ServerCommand, metadata.ServerCommand, failure)
			}
			manager := m
			switch kind {
			case "wrong-server":
				cfg.Server = "/fixture/wrong-server"
			case "stale-metadata-generation":
				cfg.Session.Generation++
				ready.Generation = cfg.Session.Generation
			case "missing-metadata":
				manager = f.ForeignManager(t)
			case "wrong-workspace":
				cfg.Workspace += "/wrong"
			case "wrong-snapshot-session":
				ready.SessionID += "-wrong"
			case "wrong-snapshot-generation":
				ready.Generation++
			case "not-ready":
				ready.State = sessionruntime.ReadinessPending
			case "not-found":
				found = false
			}
			got, err := validateScopedStartupPrivate(ctx, manager, cfg, ready, found)
			if kind == "exact-server" {
				if err != nil || got.ServerCommand != cfg.Server {
					t.Fatalf("ASSERT_STARTUP_EXACT_METADATA_ACCEPTED: metadata=%+v err=%v", got, err)
				}
				t.Log("ASSERT_STARTUP_EXACT_METADATA_ACCEPTED: PASS")
			} else {
				if err == nil {
					t.Fatal("ASSERT_STARTUP_INVALID_REFUSED")
				}
				t.Log("ASSERT_STARTUP_INVALID_REFUSED: PASS")
			}
			semantic, prepared := f.Traffic()
			if len(h.ClaimObservation()) != 0 || semantic != 0 || prepared != 0 {
				t.Fatalf("ASSERT_STARTUP_BEFORE_CLAIM_DISCOVERY: claim=%d semantic=%d prepared=%d", len(h.ClaimObservation()), semantic, prepared)
			}
		})
	}
}

func TestScopedLiveRehearsalStartupExecutionGatePrivate(t *testing.T) {
	for _, kind := range []string{"wrong-server", "stale-generation", "missing-metadata", "wrong-workspace"} {
		t.Run(kind, func(t *testing.T) {
			h, m, cfg, f := pilot.NewScopedRehearsalFixturePrivate(t)
			switch kind {
			case "wrong-server":
				cfg.Server = "/fixture/wrong-server"
			case "stale-generation":
				cfg.Session.Generation++
			case "missing-metadata":
				m = f.ForeignManager(t)
			case "wrong-workspace":
				cfg.Workspace += "/wrong"
			}
			report, err := executeScopedRehearsalPrivate(context.Background(), h, m, cfg)
			semantic, prepared := f.Traffic()
			if err == nil || report.AcquireCalls != 0 || len(h.ClaimObservation()) != 0 || semantic != 0 || prepared != 0 {
				t.Fatalf("ASSERT_STARTUP_EXECUTION_GATE: err=%v claim=%d semantic=%d prepared=%d", err, len(h.ClaimObservation()), semantic, prepared)
			}
			t.Log("ASSERT_STARTUP_EXECUTION_GATE: PASS")
		})
	}
}
