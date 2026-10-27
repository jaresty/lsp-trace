//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

func retentionRoots(t *testing.T) (*publication.ContentRetentionRoot, *publication.ContentRetentionRoot, string, string) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(home, ".scope-retention-fixture-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	paths := []string{filepath.Join(base, "primary"), filepath.Join(base, "backup")}
	roots := make([]*publication.ContentRetentionRoot, 2)
	for i, path := range paths {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		roots[i], err = publication.OpenContentRetentionRoot(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		root := roots[i]
		t.Cleanup(func() {
			if err := root.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	return roots[0], roots[1], paths[0], paths[1]
}
func pairPlan(t *testing.T) scopeRetentionPlan {
	t.Helper()
	p, err := deriveScopeRetentionPlan(retentionFixture(t, false), scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestScopeRetentionPairPreflightRefusal(t *testing.T) {
	for _, kind := range []string{"same-pointer", "same-identity", "zero-plan", "forged-plan", "budget-bytes", "budget-work", "ordinal", "closed", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			p, b, pp, bp := retentionRoots(t)
			plan := pairPlan(t)
			limitBytes, limitWork := scopeRetentionByteCeiling, scopeRetentionWorkCeiling
			ordinal := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "same-pointer":
				b = p
			case "same-identity":
				var err error
				b, err = publication.OpenContentRetentionRoot(ctx, pp)
				if err != nil {
					t.Fatal(err)
				}
				defer b.Close()
			case "zero-plan":
				plan = scopeRetentionPlan{}
			case "forged-plan":
				plan.state.references[0].Role = "FORGED"
			case "budget-bytes":
				limitBytes = plan.state.budget.bytes - 1
			case "budget-work":
				limitWork = plan.state.budget.work - 1
			case "ordinal":
				ordinal = 99
			case "closed":
				if err := b.Close(); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				cancel()
			}
			ops := defaultScopeRetentionOps()
			calls := 0
			ops.publish = func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error) {
				calls++
				return nil, errors.New("must not publish")
			}
			out, err := retainScopeObjectWithOps(ctx, plan, ordinal, p, b, limitBytes, limitWork, ops)
			if err == nil || calls != 0 || out.phase != "PREFLIGHT" || out.primary.outcome != "NOT_ATTEMPTED" || out.backup.outcome != "NOT_ATTEMPTED" || out.verified.state != nil {
				t.Fatal(out, err, calls)
			}
			for _, path := range []string{pp, bp} {
				entries, err := os.ReadDir(path)
				if err != nil || len(entries) != 0 {
					t.Fatal("preflight wrote", err)
				}
			}
		})
	}
}
func TestScopeRetentionPairFailuresPreserveEvidence(t *testing.T) {
	for _, kind := range []string{"primary-error", "backup-error", "primary-unknown", "cancel-primary-commit", "directory-sync", "close", "verification", "backup-directory-sync", "read-close", "corrupt-primary", "corrupt-backup"} {
		t.Run(kind, func(t *testing.T) {
			p, b, pp, bp := retentionRoots(t)
			plan := pairPlan(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("fixture fault")
			ops := defaultScopeRetentionOps()
			normalPublish := ops.publish
			normalRead := ops.read
			calls := 0
			reads := 0
			ops.publish = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
				calls++
				if kind == "primary-error" && calls == 1 || kind == "backup-error" && calls == 2 {
					return nil, sentinel
				}
				receipt, err := normalPublish(ctx, r, id, raw)
				if err != nil {
					return receipt, err
				}
				if calls == 1 {
					switch kind {
					case "primary-unknown":
						return nil, sentinel
					case "cancel-primary-commit":
						cancel()
					case "directory-sync":
						receipt.DirectorySyncStatus = publication.DirectorySyncFailed
						receipt.CrashDurability = publication.DirectorySyncFailed
					case "close":
						receipt.CloseStatus = publication.CloseFailed
					case "verification":
						receipt.VerificationStatus = "COMMITTED_VERIFICATION_FAILED"
					}
				}
				if calls == 2 {
					switch kind {
					case "backup-directory-sync":
						receipt.DirectorySyncStatus = publication.DirectorySyncFailed
					case "corrupt-primary":
						if e := os.WriteFile(filepath.Join(pp, id), []byte("corrupt"), 0600); e != nil {
							t.Fatal(e)
						}
					case "corrupt-backup":
						if e := os.WriteFile(filepath.Join(bp, id), []byte("corrupt"), 0600); e != nil {
							t.Fatal(e)
						}
					}
				}
				return receipt, err
			}
			ops.read = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, limit int64) ([]byte, error) {
				reads++
				raw, err := normalRead(ctx, r, id, limit)
				if kind == "read-close" && reads == 3 {
					return nil, errors.Join(err, sentinel)
				}
				return raw, err
			}
			out, err := retainScopeObjectWithOps(ctx, plan, 0, p, b, scopeRetentionByteCeiling, scopeRetentionWorkCeiling, ops)
			if err == nil || out.verified.state != nil {
				t.Fatal("fault qualified", out, err)
			}
			if kind == "primary-error" || kind == "primary-unknown" {
				if out.primary.outcome != "INDETERMINATE" || out.primary.receipt != nil || calls != 1 || out.backup.outcome != "NOT_ATTEMPTED" {
					t.Fatal(out, calls)
				}
			} else if out.primary.outcome != "NAMESPACE_COMMITTED" || out.primary.receipt == nil {
				t.Fatal("known primary write lost", out)
			}
			if kind == "backup-error" {
				if out.phase != "BACKUP_PUBLICATION" || out.backup.outcome != "INDETERMINATE" || out.primary.receipt == nil {
					t.Fatal(out)
				}
			}
			if kind == "backup-directory-sync" || kind == "read-close" || kind == "corrupt-primary" || kind == "corrupt-backup" {
				if out.backup.outcome != "NAMESPACE_COMMITTED" || out.backup.receipt == nil {
					t.Fatal("known backup write lost", out)
				}
			}
			if kind == "cancel-primary-commit" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation lost", err)
			}
			if kind != "primary-error" {
				if _, e := os.Stat(filepath.Join(pp, plan.state.objects[0].digest)); e != nil {
					t.Fatal("known/unknown object removed", e)
				}
			}
		})
	}
}
func TestScopeRetentionPairExistingMismatch(t *testing.T) {
	for _, which := range []string{"primary", "backup"} {
		t.Run(which, func(t *testing.T) {
			p, b, pp, bp := retentionRoots(t)
			plan := pairPlan(t)
			path := pp
			if which == "backup" {
				path = bp
			}
			id := plan.state.objects[0].digest
			wrong := []byte("wrong existing bytes")
			if err := os.WriteFile(filepath.Join(path, id), wrong, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := retainScopeObject(context.Background(), plan, 0, p, b, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
			if err == nil || out.verified.state != nil {
				t.Fatal("mismatch admitted")
			}
			evidence := out.primary
			if which == "backup" {
				evidence = out.backup
				if out.primary.outcome != "NAMESPACE_COMMITTED" {
					t.Fatal(out)
				}
			}
			if evidence.outcome != "EXISTING_UNVERIFIED" {
				t.Fatal(evidence)
			}
			got, err := os.ReadFile(filepath.Join(path, id))
			if err != nil || !bytes.Equal(got, wrong) {
				t.Fatal("overwritten", err)
			}
		})
	}
}

// The publication-layer hook regression proves a real hidden-close failure can
// arrive here as PURE ErrExist. Insert an equal competitor only after Read has
// observed absence; never qualify that attempted publication by later readback.
func TestScopeRetentionPairLateExistsFailsClosed(t *testing.T) {
	for _, which := range []string{"primary", "backup"} {
		t.Run(which, func(t *testing.T) {
			p, b, pp, bp := retentionRoots(t)
			plan := pairPlan(t)
			ops := defaultScopeRetentionOps()
			normal := ops.publish
			calls := 0
			reads := 0
			normalRead := ops.read
			ops.read = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, limit int64) ([]byte, error) {
				reads++
				return normalRead(ctx, r, id, limit)
			}
			ops.publish = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
				calls++
				target := p
				path := pp
				if which == "backup" {
					target = b
					path = bp
				}
				if r == target {
					if err := os.WriteFile(filepath.Join(path, id), raw, 0600); err != nil {
						t.Fatal(err)
					}
					return nil, os.ErrExist
				}
				return normal(ctx, r, id, raw)
			}
			out, err := retainScopeObjectWithOps(context.Background(), plan, 0, p, b, scopeRetentionByteCeiling, scopeRetentionWorkCeiling, ops)
			if !errors.Is(err, os.ErrExist) || out.verified.state != nil {
				t.Fatal("late exists admitted", out, err)
			}
			if which == "primary" {
				if calls != 1 || reads != 1 || out.primary.outcome != "INDETERMINATE" || out.backup.outcome != "NOT_ATTEMPTED" {
					t.Fatal(out, calls, reads)
				}
			} else {
				if calls != 2 || reads != 2 || out.primary.outcome != "NAMESPACE_COMMITTED" || out.primary.receipt == nil || out.backup.outcome != "INDETERMINATE" {
					t.Fatal(out, calls, reads)
				}
			}
		})
	}
}
func TestScopeRetentionPairExistingReadFirstNoPublish(t *testing.T) {
	p, b, pp, bp := retentionRoots(t)
	plan := pairPlan(t)
	obj := plan.state.objects[0]
	for _, path := range []string{pp, bp} {
		if err := os.WriteFile(filepath.Join(path, obj.digest), obj.raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ops := defaultScopeRetentionOps()
	calls := 0
	ops.publish = func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error) {
		calls++
		return nil, errors.New("unexpected publish")
	}
	out, err := retainScopeObjectWithOps(context.Background(), plan, 0, p, b, scopeRetentionByteCeiling, scopeRetentionWorkCeiling, ops)
	if err != nil || calls != 0 || out.primary.outcome != "EXISTING_VERIFIED" || out.backup.outcome != "EXISTING_VERIFIED" || out.verified.state == nil {
		t.Fatal(out, err, calls)
	}
}
func TestScopeRetentionPairPreReadFailureNoPublish(t *testing.T) {
	for _, kind := range []string{"close", "joined-absence", "permission"} {
		t.Run(kind, func(t *testing.T) {
			p, b, _, _ := retentionRoots(t)
			plan := pairPlan(t)
			sentinel := errors.New("pre-read close failed")
			ops := defaultScopeRetentionOps()
			calls := 0
			ops.publish = func(context.Context, *publication.ContentRetentionRoot, string, []byte) (*publication.BoundFileReceipt, error) {
				calls++
				return nil, errors.New("unexpected publish")
			}
			ops.read = func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error) {
				switch kind {
				case "joined-absence":
					return nil, errors.Join(os.ErrNotExist, sentinel)
				case "permission":
					return nil, os.ErrPermission
				default:
					return nil, sentinel
				}
			}
			out, err := retainScopeObjectWithOps(context.Background(), plan, 0, p, b, scopeRetentionByteCeiling, scopeRetentionWorkCeiling, ops)
			if err == nil || calls != 0 || out.phase != "PRIMARY_PRE_READ" || out.primary.outcome != "EXISTING_UNVERIFIED" || out.backup.outcome != "NOT_ATTEMPTED" || out.verified.state != nil {
				t.Fatal("pre-read error erased", out, err, calls)
			}
		})
	}
}
func TestScopeRetentionPairActual(t *testing.T) {
	p, b, pp, bp := retentionRoots(t)
	plan := pairPlan(t)
	for i := 0; i < 2; i++ {
		attempt, err := retainScopeObject(context.Background(), plan, 0, p, b, scopeRetentionByteCeiling, scopeRetentionWorkCeiling)
		if err != nil || attempt.verified.state == nil {
			t.Fatal(attempt, err)
		}
		want := "NAMESPACE_COMMITTED"
		if i == 1 {
			want = "EXISTING_VERIFIED"
		}
		if attempt.primary.outcome != want || attempt.backup.outcome != want || attempt.verified.state.authority != 0 || attempt.verified.state.referencesDigest != Digest(plan.metadataBytes()) {
			t.Fatal("copy evidence", attempt)
		}
		for _, path := range []string{pp, bp} {
			raw, err := os.ReadFile(filepath.Join(path, plan.state.objects[0].digest))
			if err != nil || !bytes.Equal(raw, plan.objectBytes(0)) {
				t.Fatal(err)
			}
		}
	}
}
