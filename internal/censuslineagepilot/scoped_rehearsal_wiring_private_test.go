//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func TestScopedRehearsalWiringLostPendingPrivate(t *testing.T) {
	for _, kind := range []string{"nil", "copy", "closed-proof"} {
		t.Run(kind, func(t *testing.T) {
			h, m, cfg, f := NewScopedRehearsalFixturePrivate(t)
			err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(ctx context.Context) error {
				if _, err := h.Bind(ctx, m); err != nil {
					return err
				}
				switch kind {
				case "nil":
					h.pending = nil
				case "copy":
					p := *h.pending
					h.pending = &p
				case "closed-proof":
					if err := h.topology.Close(); err != nil {
						return err
					}
				}
				before, _ := f.Traffic()
				if _, err := h.Discover(ctx, cfg.Session); err == nil {
					t.Fatal("ASSERT_REHEARSAL_PENDING_REFUSED")
				}
				after, _ := f.Traffic()
				if before != after || after != 0 {
					t.Fatal("ASSERT_REHEARSAL_NO_SEMANTIC")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestScopedRehearsalWiringExistingClaimPrivate(t *testing.T) {
	h, m, cfg, f := NewScopedRehearsalFixturePrivate(t)
	lease := sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}
	if err := m.WithReadySessionLease(context.Background(), lease, func(ctx context.Context) error { _, e := h.Bind(ctx, m); return e }); err != nil {
		t.Fatal(err)
	}
	claimed := h.owner.HeadBytes()
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := openScopedRehearsalPrivate(context.Background(), cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	err = m.WithReadySessionLease(context.Background(), lease, func(ctx context.Context) error {
		head, e := again.Bind(ctx, m)
		if !bytes.Equal(head, claimed) {
			t.Fatal("existing head bytes changed")
		}
		return e
	})
	if err == nil {
		t.Fatal("ASSERT_REHEARSAL_EXISTING_REFUSED")
	}
	semantic, _ := f.Traffic()
	if semantic != 0 {
		t.Fatal("ASSERT_REHEARSAL_EXISTING_NO_SEMANTIC")
	}
}
func TestScopedRehearsalWiringUncertainClaimPrivate(t *testing.T) {
	h, m, cfg, f := NewScopedRehearsalFixturePrivate(t)
	// Existing claim-owner matrix covers receipt ambiguity/CAS failures. This
	// integration assertion proves an uncertain attempt cannot reach Discover.
	if _, err := h.owner.claim(context.Background(), &executionClaimMemoryPrivate{loadErr: errors.New("uncertain read close")}); err == nil {
		t.Fatal("uncertain attempt accepted")
	}
	err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(ctx context.Context) error { _, err := h.Bind(ctx, m); return err })
	if err == nil {
		t.Fatal("uncertain owner replay")
	}
	if events, overflow, available := h.Diagnostics(); available || overflow || len(events) != 0 {
		t.Fatal("claim failure before guard must be unavailable")
	}
	semantic, _ := f.Traffic()
	if semantic != 0 {
		t.Fatal("uncertain attempt semantic")
	}
}
func TestScopedRehearsalWiringSixSupplyDriftsPrivate(t *testing.T) {
	for i := 0; i < 6; i++ {
		t.Run(string(rune('0'+i)), func(t *testing.T) {
			h, m, cfg, f := NewScopedRehearsalFixturePrivate(t)
			file := h.restart.Scope().Files()[i]
			if err := os.WriteFile(filepath.Join(cfg.Workspace, file.Path), []byte("package drift\n"), 0600); err != nil {
				t.Fatal(err)
			}
			err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(ctx context.Context) error { _, e := h.Bind(ctx, m); return e })
			if err == nil {
				t.Fatal("ASSERT_REHEARSAL_SUPPLY_DRIFT_REFUSED")
			}
			if events, overflow, available := h.Diagnostics(); available || overflow || len(events) != 0 {
				t.Fatal("adapter failure before guard must be unavailable")
			}
			semantic, _ := f.Traffic()
			if semantic != 0 {
				t.Fatal("ASSERT_REHEARSAL_SUPPLY_NO_DISCOVERY")
			}
		})
	}
}
func TestScopedRehearsalWiringConfigDriftsPrivate(t *testing.T) {
	for _, kind := range []string{"workspace", "root", "budget"} {
		t.Run(kind, func(t *testing.T) {
			_, _, cfg, f := NewScopedRehearsalFixturePrivate(t)
			switch kind {
			case "workspace":
				cfg.Workspace = filepath.Dir(cfg.Workspace)
			case "root":
				cfg.OutputIdentities[0].Inode++
			case "budget":
				cfg.Budget.TotalBytes++
			}
			h, e := openScopedRehearsalPrivate(context.Background(), cfg, false)
			if e == nil {
				h.Close()
				t.Fatal("ASSERT_REHEARSAL_PREFLIGHT_DRIFT")
			}
			semantic, _ := f.Traffic()
			if semantic != 0 {
				t.Fatal("semantic preflight")
			}
		})
	}
}
func TestScopedRehearsalWiringPartialPairPrivate(t *testing.T) {
	for _, kind := range []string{"backup-read", "backup-hash", "backup-close"} {
		t.Run(kind, func(t *testing.T) {
			h, _, _, _ := NewScopedRehearsalFixturePrivate(t)
			ops := defaultScopeRetentionOps()
			read := ops.read
			calls := 0
			ops.read = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
				if r == h.roots[1] {
					calls++
					if kind == "backup-read" {
						return nil, errors.New("backup unavailable")
					}
				}
				raw, e := read(ctx, r, id, n)
				if r == h.roots[1] && calls > 1 && e == nil {
					if kind == "backup-hash" {
						return []byte("corrupt"), nil
					}
					if kind == "backup-close" {
						return raw, errors.New("read close uncertain")
					}
				}
				return raw, e
			}
			h.retentionOps = ops
			raw := []byte(`{"observation":"bounded"}`)
			r := h.Retain(context.Background(), "fixture", raw)
			if r.Verified || r.Error == "" || r.PrimaryReceipt == nil {
				t.Fatalf("ASSERT_REHEARSAL_PARTIAL_RECEIPT: %+v", r)
			}
			observed, e := h.roots[0].Read(context.Background(), Digest(raw), int64(len(raw)))
			if e != nil || !bytes.Equal(observed, raw) {
				t.Fatal("primary rolled back", e)
			}
		})
	}
}
func TestScopedRehearsalWiringCloseAndBoundPrivate(t *testing.T) {
	h, _, _, _ := NewScopedRehearsalFixturePrivate(t)
	store, proof := h.store, h.topology
	if err := h.CloseExecution(); err != nil {
		t.Fatal(err)
	}
	if !store.closed || proof.state != nil {
		t.Fatal("store/proof lifetime")
	}
	for _, root := range h.roots {
		if _, err := root.DirectoryIdentity(); err != nil {
			t.Fatal("roots closed before outcome", err)
		}
	}
	if r := h.Retain(context.Background(), "oversize", bytes.Repeat([]byte("x"), scopedRehearsalArtifactPrivate+1)); r.Error == "" || r.PrimaryReceipt != nil {
		t.Fatal("serialized budget not enforced")
	}
	raw, _ := json.Marshal(h.config)
	if len(raw) > ScopedRehearsalConfigLimitPrivate {
		t.Fatal("fixture config bound")
	}
}
