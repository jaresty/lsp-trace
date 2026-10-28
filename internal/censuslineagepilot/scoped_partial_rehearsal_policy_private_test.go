//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"testing"
)

func TestScopedPartialRehearsalClaimIsolationPrivate(t *testing.T) {
	h, _, cfg, _ := NewScopedPartialRehearsalFixturePrivate(t)
	partial, err := parseScopedExecutionHeadPrivate(h.owner.HeadBytes())
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []scopedExecutionPolicyPrivate{scopedExecutionPolicyFixedPrivate(), scopedExecutionPolicyV2Private()} {
		old, err := newScopedExecutionClaimWithPolicyPrivate(context.Background(), h.restart, cfg.CodeHEAD, cfg.Session, cfg.Workspace, h.roots, policy)
		if err != nil {
			t.Fatal(err)
		}
		if old.selector != h.owner.selector || bytes.Equal(old.HeadBytes(), h.owner.HeadBytes()) {
			t.Fatal("ASSERT_PARTIAL_SINGLE_SLOT_DISTINCT_CLAIM")
		}
		mixed := partial
		mixed.Binding.Policy = policy
		raw, _ := json.Marshal(mixed)
		// v2 layout is deliberately shared; policy binding and owner equality, not a
		// schema-name disguise, distinguish partial purpose from successful constituent.
		if bytes.Equal(raw, h.owner.HeadBytes()) {
			t.Fatal("ASSERT_PARTIAL_PURPOSE_HASH")
		}
	}
	policy := scopedExecutionPartialInspectionPolicyPrivate()
	policy.Name = "one-partial-inspection-private-v2"
	if _, e := policy.claimSchema(); e == nil {
		t.Fatal("ASSERT_UNKNOWN_POLICY_CLOSED")
	}
}

func TestScopedPartialRehearsalClaimAndSupplyRefusalPrivate(t *testing.T) {
	kinds := []string{"existing", "uncertain", "nil-pending", "copied-pending"}
	for i := 0; i < 6; i++ {
		kinds = append(kinds, fmt.Sprintf("supply-%d", i))
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			h, m, cfg, f := NewScopedPartialRehearsalFixturePrivate(t)
			lease := sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}
			if kind == "uncertain" {
				if _, e := h.owner.claim(context.Background(), &executionClaimMemoryPrivate{loadErr: errors.New("uncertain read close")}); e == nil {
					t.Fatal("fixture uncertain claim")
				}
			}
			for i := 0; i < 6; i++ {
				if kind == fmt.Sprintf("supply-%d", i) {
					if e := os.WriteFile(filepath.Join(cfg.Workspace, h.restart.Scope().Files()[i].Path), []byte("package drift\n"), 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			e := m.WithReadySessionLease(context.Background(), lease, func(ctx context.Context) error {
				_, e := h.Bind(ctx, m)
				if kind == "existing" {
					if e != nil {
						return e
					}
					if e = h.Close(); e != nil {
						return e
					}
					again, e := openScopedRehearsalPrivate(ctx, cfg, false)
					if e != nil {
						return e
					}
					defer again.Close()
					_, e = again.Bind(ctx, m)
					return e
				}
				if kind == "nil-pending" || kind == "copied-pending" {
					if e != nil {
						return e
					}
					if kind == "nil-pending" {
						h.pending = nil
					} else {
						copy := *h.pending
						h.pending = &copy
					}
					_, e = h.Discover(ctx, cfg.Session)
				}
				return e
			})
			if e == nil {
				t.Fatal("ASSERT_PARTIAL_CLAIM_SUPPLY_REFUSAL", kind)
			}
			semantic, _ := f.Traffic()
			if semantic != 0 {
				t.Fatal("ASSERT_PARTIAL_REFUSAL_ZERO_SEMANTIC", kind)
			}
		})
	}
}
