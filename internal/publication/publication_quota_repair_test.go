//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func repairPrivateDir(t *testing.T, parent, name string) string {
	t.Helper()
	p := filepath.Join(parent, name)
	if err := os.Mkdir(p, 0700); err != nil {
		t.Fatal(err)
	}
	return p
}

func repairOpenRoot(t *testing.T, p string) *Root {
	t.Helper()
	r, err := OpenRoot(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	return r
}

// The alias exists before configuration. All payloads go through real publication
// APIs; no raw payload write substitutes for quota admission.
func repairAliasedStore(t *testing.T) (*Root, *Root) {
	t.Helper()
	base := t.TempDir()
	if err := os.Chmod(base, 0700); err != nil {
		t.Fatal(err)
	}
	store := repairPrivateDir(t, base, "store")
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(filepath.Join(store, "sub"), alias); err != nil {
		t.Fatal(err)
	}
	s := repairOpenRoot(t, store)
	if err := ConfigureStorage(context.Background(), s, 128<<10); err != nil {
		t.Fatal(err)
	}
	sub := repairPrivateDir(t, store, "sub")
	repairPrivateDir(t, sub, "nested")
	return s, repairOpenRoot(t, filepath.Join(alias, "nested"))
}

func TestPublicationQuotaAncestorAliasRefusesOversize(t *testing.T) {
	for _, api := range []string{"bound", "generic"} {
		t.Run(api, func(t *testing.T) {
			s, aliased := repairAliasedStore(t)
			before := *quotaStatus(t, s)
			payload := bytes.Repeat([]byte("x"), 128<<10+1)
			var err error
			if api == "bound" {
				_, err = PublishBoundFile(aliased, "created/payload", payload, func([]byte) error { return nil })
			} else {
				err = NewPublisher().Publish(Request{Root: aliased, Selector: "created/payload", Bytes: payload}).Err()
			}
			if !errors.Is(err, ErrStorageCapacity) {
				t.Error("ALIAS_OVERSIZE_NOT_REFUSED", err)
			}
			entries, e := os.ReadDir(aliased.Path())
			if e != nil || len(entries) != 0 {
				t.Error("ALIAS_REFUSAL_SIDE_EFFECT", entries, e)
			}
			after := *quotaStatus(t, s)
			if after != before {
				t.Error("ALIAS_REFUSAL_CHANGED_ACCOUNTING", before, after)
			}
		})
	}
}

func TestPublicationQuotaCanonicalAndAliasCharged(t *testing.T) {
	for _, alias := range []bool{false, true} {
		t.Run(map[bool]string{false: "canonical", true: "alias"}[alias], func(t *testing.T) {
			s, r := repairAliasedStore(t)
			if !alias {
				p, err := filepath.EvalSymlinks(r.Path())
				if err != nil {
					t.Fatal(err)
				}
				r = repairOpenRoot(t, p)
			}
			before := *quotaStatus(t, s)
			payload := []byte("accounted")
			if _, err := PublishBoundFile(r, "payload", payload, func([]byte) error { return nil }); err != nil {
				t.Fatal(err)
			}
			after := *quotaStatus(t, s)
			if after.Reservations != before.Reservations+1 || after.FileBytes != before.FileBytes+uint64(len(payload)) || after.ReservedBytes != before.ReservedBytes {
				t.Error("CONFIGURED_PUBLICATION_NOT_CHARGED", before, after)
			}
			got, err := r.ReadSelector("payload", 100)
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatal("PUBLISHED_BYTES", string(got), err)
			}
		})
	}
}

func TestPublicationQuotaAliasSharesStorageLock(t *testing.T) {
	s, aliased := repairAliasedStore(t)
	held, err := ReserveStorage(context.Background(), s, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := held.Complete(true); err != nil {
			t.Error(err)
		}
	}()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &quotaLockObservedContext{Context: base, blocked: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		q, e := ReserveStorage(ctx, aliased, 1)
		if q != nil {
			e = errors.Join(e, q.Complete(false))
		}
		done <- e
	}()
	select {
	case <-ctx.blocked:
		t.Log("ALIAS_ACTUAL_FLOCK_CONTENTION_OBSERVED")
	case e := <-done:
		t.Fatal("ALIAS_BYPASSED_SHARED_FLOCK", e)
	case <-time.After(2 * time.Second):
		cancel()
		awaitQuotaResult(t, done)
		t.Fatal("ALIAS_FLOCK_OBSERVATION_TIMEOUT")
	}
	cancel()
	if e, finished := awaitQuotaResult(t, done); !finished || !errors.Is(e, context.Canceled) {
		t.Error("ALIAS_FLOCK_CANCEL_CAUSE", e)
	}
}

func TestPublicationQuotaAliasStickyMarkerLoss(t *testing.T) {
	s, aliased := repairAliasedStore(t)
	if err := os.Remove(filepath.Join(s.Path(), storageLedgerName)); err != nil {
		t.Fatal(err)
	}
	_, err := PublishBoundFile(aliased, "must-not-exist", []byte("x"), func([]byte) error { return nil })
	if !errors.Is(err, ErrStorageIntegrity) {
		t.Error("ALIAS_LOST_EXPECTED_MARKER", err)
	}
	entries, e := os.ReadDir(aliased.Path())
	if e != nil || len(entries) != 0 {
		t.Error("MARKER_LOSS_WROTE_CONTENT", entries, e)
	}
}

func TestPublicationQuotaConfiguredIdentityLoss(t *testing.T) {
	for _, kind := range []string{"missing-alias", "replaced-alias"} {
		t.Run(kind, func(t *testing.T) {
			_, r := repairAliasedStore(t)
			alias := filepath.Dir(r.Path())
			if err := os.Remove(alias); err != nil {
				t.Fatal(err)
			}
			if kind == "replaced-alias" {
				other := repairPrivateDir(t, filepath.Dir(alias), "other")
				repairPrivateDir(t, other, "nested")
				if err := os.Symlink(other, alias); err != nil {
					t.Fatal(err)
				}
			}
			_, err := PublishBoundFile(r, "must-not-exist", []byte("x"), func([]byte) error { return nil })
			if !errors.Is(err, ErrStorageIntegrity) {
				t.Error("CONFIGURED_IDENTITY_LOSS_DOWNGRADED", err)
			}
			if _, err := r.ReadSelectorWithAbsence("must-not-exist", 10); !errors.Is(err, os.ErrNotExist) {
				t.Error("IDENTITY_LOSS_WROTE_CONTENT", err)
			}
		})
	}
}

func TestPublicationQuotaRootBeforeConfiguration(t *testing.T) {
	// ConfigureStorage requires an empty store. Open the descendant outside it,
	// configure the empty store, then move the same directory beneath it and retarget
	// the existing ancestor alias. The admitted directory inode does not change.
	base := t.TempDir()
	if err := os.Chmod(base, 0700); err != nil {
		t.Fatal(err)
	}
	store := repairPrivateDir(t, base, "store")
	sub := repairPrivateDir(t, base, "sub")
	repairPrivateDir(t, sub, "nested")
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(sub, alias); err != nil {
		t.Fatal(err)
	}
	r := repairOpenRoot(t, filepath.Join(alias, "nested"))
	s := repairOpenRoot(t, store)
	if err := ConfigureStorage(context.Background(), s, 128<<10); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(sub, filepath.Join(store, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(store, "sub"), alias); err != nil {
		t.Fatal(err)
	}
	_, err := PublishBoundFile(r, "too-big", bytes.Repeat([]byte("x"), 128<<10+1), func([]byte) error { return nil })
	if !errors.Is(err, ErrStorageCapacity) {
		t.Error("LATE_CONFIGURATION_BYPASSED", err)
	}
}

func TestPublicationQuotaLegacyStillPublishes(t *testing.T) {
	base := t.TempDir()
	if err := os.Chmod(base, 0700); err != nil {
		t.Fatal(err)
	}
	r := repairOpenRoot(t, base)
	result := NewPublisher().Publish(Request{Root: r, Selector: "legacy", Bytes: []byte("legacy")})
	if result.Err() != nil || result.Receipt == nil {
		t.Fatal("LEGACY_PUBLICATION_REFUSED", result)
	}
	status, err := ReadStorageStatus(context.Background(), r)
	if err != nil || status != nil {
		t.Fatal("LEGACY_CONFIGURED_IMPLICITLY", status, err)
	}
}

func TestPublicationQuotaCompletionWithholdsReceipt(t *testing.T) {
	r := quotaRoot(t, 128<<10)
	var injectionErr error
	published := false
	result := NewPublisher().Publish(Request{Root: r, Selector: "payload", Bytes: []byte("committed"), Trace: func(event TraceEvent) {
		if event.Stage == "EXIT" && event.Reason == "PUBLISHED" {
			published = true
			injectionErr = os.Symlink("absent", filepath.Join(r.Path(), "scan-refuses-symlink"))
		}
	}})
	if !published || injectionErr != nil {
		t.Fatal("RECEIPT_FAULT_SEAM_NOT_REACHED", published, injectionErr)
	}
	if result.Receipt != nil || result.Failure == nil || !result.Failure.AtomicRename || !errors.Is(result.Err(), ErrStorageIntegrity) {
		t.Fatal("COMPLETION_RECEIPT_FAILURE_CONTRACT", result)
	}
	got, err := r.ReadSelector("payload", 100)
	if err != nil || string(got) != "committed" {
		t.Fatal("COMPLETION_UNDID_INSTALLED_BYTES", string(got), err)
	}
}

func TestPublicationQuotaCompletionKeepsCommittedFailure(t *testing.T) {
	for _, quotaFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "reread-only", true: "dual-failure"}[quotaFailure], func(t *testing.T) {
			r := quotaRoot(t, 128<<10)
			installed, rereadFailed := false, false
			var injectionErr error
			result := NewPublisher().Publish(Request{Root: r, Selector: "payload", Bytes: []byte("committed"), Trace: func(event TraceEvent) {
				if event.Stage == "NO_REPLACE" && event.Reason == "HARD_LINK_INSTALLED" {
					installed = true
					// Existing callback seam: remove only after the real install.
					injectionErr = errors.Join(injectionErr, os.Remove(filepath.Join(r.Path(), "payload")))
				}
				if event.Stage == "REREAD_EQUAL" && event.Reason == "REREAD_FAILED" {
					rereadFailed = true
					if quotaFailure {
						injectionErr = errors.Join(injectionErr, os.Symlink("absent", filepath.Join(r.Path(), "scan-refuses-symlink")))
					}
				}
			}})
			if injectionErr != nil || !installed || !rereadFailed {
				t.Fatal("FAULT_SEAM_NOT_REACHED", installed, rereadFailed, injectionErr)
			}
			if result.Failure == nil || !result.Failure.AtomicRename {
				t.Error("COMPLETION_LOST_KNOWN_COMMIT", result.Failure)
			}
			if !errors.Is(result.Err(), os.ErrNotExist) {
				t.Error("COMPLETION_LOST_REREAD_CAUSE", result.Err())
			}
			if quotaFailure && !errors.Is(result.Err(), ErrStorageIntegrity) {
				t.Error("COMPLETION_LOST_QUOTA_CAUSE", result.Err())
			}
			if result.Receipt != nil || result.Err() == nil {
				t.Error("COMPLETION_FALSE_VERIFIED_SUCCESS", result)
			}
		})
	}
}
