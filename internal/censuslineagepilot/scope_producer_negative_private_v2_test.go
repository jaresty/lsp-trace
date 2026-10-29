//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

func TestScopeProducerPrivateV2EveryPolicyBoundRequired(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	base := producerSpecPrivateV2(t, root, 1, 0)
	var visit func(reflect.Type, []int, string)
	visit = func(ty reflect.Type, index []int, prefix string) {
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			indices := append(append([]int(nil), index...), i)
			name := prefix + f.Name
			if f.Type.Kind() == reflect.Struct {
				visit(f.Type, indices, name+"/")
				continue
			}
			if f.Type.Kind() != reflect.Int {
				continue
			}
			t.Run(name, func(t *testing.T) {
				s := base
				reflect.ValueOf(&s.EffectivePolicy).Elem().FieldByIndex(indices).SetInt(0)
				if _, err := FreezeScopePrivateV2(context.Background(), s); err == nil {
					t.Fatal("ASSERT_V2_NO_HIDDEN_POLICY_DEFAULT", name)
				}
			})
		}
	}
	visit(reflect.TypeOf(base.EffectivePolicy), nil, "")
}
func TestScopeProducerPrivateV2ForeignStatePreflight(t *testing.T) {
	otherSpec, otherManager, otherID, _ := producerFixturePrivateV2(t, 1, 0)
	spec, m, id, _ := producerFixturePrivateV2(t, 2, 1)
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		o, _, err := DiscoverScopePrivateV2(ctx, f, m, id)
		if err != nil {
			return err
		}
		b, _ := o.Batch(0)
		a := o.state.adapter
		req, err := scopeBatchRequestPrivateV2(a, b.state.binding)
		if err != nil {
			return err
		}
		originalGuard, originalID, originalLifetime, originalFrozen := a.guardV2, a.identity, a.lifetimeV2, a.frozenV2
		for _, kind := range []string{"guard", "generation", "lifetime", "scope", "mixed-v1"} {
			t.Run(kind, func(t *testing.T) {
				switch kind {
				case "guard":
					a.guardV2 = &sessionruntime.RetainedScopeGuardPrivateV2{}
				case "generation":
					a.identity.Generation++
				case "lifetime":
					a.lifetimeV2 = context.Background()
				case "scope":
					a.frozenV2 = FrozenScopePrivateV2{}
				case "mixed-v1":
					a.guard = &sessionruntime.RetainedScopeGuardPrivate{}
				}
				if _, err := acquireScopeBatchPrivateV2(ctx, m, o, b, b.state.binding, req); err == nil || b.state.spent {
					t.Fatal("ASSERT_V2_FOREIGN_PROVENANCE_UNSPENT", kind)
				}
				a.guardV2, a.identity, a.lifetimeV2, a.frozenV2, a.guard = originalGuard, originalID, originalLifetime, originalFrozen, nil
			})
		}
		otherFrozen, err := FreezeScopePrivateV2(ctx, otherSpec)
		if err != nil {
			return err
		}
		producerLeasePrivateV2(t, otherManager, otherID, otherSpec.Root, func(otherCtx context.Context) error {
			other, _, e := DiscoverScopePrivateV2(otherCtx, otherFrozen, otherManager, otherID)
			if e != nil {
				return e
			}
			if _, e = AcquireScopeBatchPrivateV2(ctx, m, other, b); e == nil || b.state.spent {
				t.Fatal("ASSERT_V2_FOREIGN_ORIGIN_BATCH_UNSPENT")
			}
			return nil
		})
		copyState := *o.state
		forged := EnumerationOriginPrivateV2{&copyState}
		if forged.valid() {
			t.Fatal("ASSERT_V2_ORIGIN_STATE_SELF_SEAL")
		}
		if _, err = AcquireScopeBatchPrivateV2(ctx, m, o, b); err != nil {
			return err
		}
		return nil
	})
}
func TestScopeProducerPrivateV2ExpiredLeasePreflight(t *testing.T) {
	spec, m, id, _ := producerFixturePrivateV2(t, 1, 0)
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	var o EnumerationOriginPrivateV2
	var b ScopeBatchSelectionPrivateV2
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		var err error
		o, _, err = DiscoverScopePrivateV2(ctx, f, m, id)
		if err != nil {
			return err
		}
		b, err = o.Batch(0)
		return err
	})
	if _, err = AcquireScopeBatchPrivateV2(context.Background(), m, o, b); err == nil || b.state.spent {
		t.Fatal("ASSERT_V2_EXPIRED_LEASE_UNSPENT")
	}
}
func TestScopeProducerPrivateV2CaptureReleaseAndClientLifetime(t *testing.T) {
	for _, kind := range []string{"release", "unattached", "cancel", "late-client"} {
		t.Run(kind, func(t *testing.T) {
			spec, m, id, child := producerFixturePrivateV2(t, 1, 0)
			f, err := FreezeScopePrivateV2(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			producerLeasePrivateV2(t, m, id, spec.Root, func(lease context.Context) error {
				ctx, cancel := context.WithCancel(lease)
				defer cancel()
				o, _, err := DiscoverScopePrivateV2(ctx, f, m, id)
				if err != nil {
					return err
				}
				b, _ := o.Batch(0)
				var observed *scopedBatchOperationClientPrivate
				b.state.afterClientPrivateTest = func(c *scopedBatchOperationClientPrivate) { observed = c }
				switch kind {
				case "release":
					b.state.beforeReleasePrivateTest = func(r directionalraw.Reservation) {
						if !r.Attached() {
							t.Fatal("ASSERT_V2_REAL_ATTACH")
						}
						if !r.Release() {
							t.Fatal("ASSERT_V2_FIXTURE_FIRST_RELEASE")
						}
					}
				case "unattached":
					child.mu.Lock()
					child.response = func(lspwire.Message) json.RawMessage { return json.RawMessage(`{"invalid":"owned response"}`) }
					child.mu.Unlock()
				case "cancel":
					o.state.adapter.beforePartialCapturePrivateTest = func(*acquisition.Result) { cancel() }
				}
				pair, err := AcquireScopeBatchPrivateV2(ctx, m, o, b)
				if kind == "late-client" {
					if err != nil || !pair.valid() {
						return err
					}
				} else if err == nil || pair.valid() {
					t.Fatal("ASSERT_V2_NO_SUCCESS_UNCERTAIN_TERMINAL", kind)
				}
				if !b.state.spent || (kind != "late-client" && b.state.failure == nil) {
					t.Fatal("ASSERT_V2_TERMINAL_FAILURE_EXHAUSTED", kind)
				}
				before := len(child.snapshot())
				if observed == nil || observed.active.Load() {
					t.Fatal("ASSERT_V2_CLIENT_REVOKED")
				}
				if _, e := observed.PrepareCallHierarchy(lease, lsp.PrepareCallHierarchyParams{}); e == nil || len(child.snapshot()) != before {
					t.Fatal("ASSERT_V2_LATE_CLIENT_NO_WIRE")
				}
				if _, e := AcquireScopeBatchPrivateV2(lease, m, o, b); e == nil {
					t.Fatal("ASSERT_V2_FAILURE_NO_RETRY")
				}
				live, _, _, _, general, occupied := m.DirectionalAccountingPrivate()
				if live != 0 || general != 0 || occupied {
					t.Fatal("ASSERT_V2_NO_LIVE_DIRECTIONAL_CUSTODY")
				}
				return nil
			})
		})
	}
}
func TestScopeProducerPrivateV2SourceCaptureBoundPreflight(t *testing.T) {
	spec, m, id, _ := producerFixturePrivateV2(t, 1, 0)
	p := filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath)
	raw := make([]byte, 1<<20+1)
	for i := range raw {
		raw[i] = 'x'
	}
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	spec.Members[0].ExpectedSHA256 = Digest(raw)
	spec.Members[0].ByteLength = uint64(len(raw))
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
		o, _, err := DiscoverScopePrivateV2(ctx, f, m, id)
		// Freeze admits <=2 MiB, but genuine captured Manager supply already has
		// a stricter 1 MiB ceiling. Do not broaden it to reach later capture.
		if err == nil || o.valid() || o.BatchCount() != 0 {
			t.Fatal("ASSERT_V2_SUPPLY_CAP_BEFORE_AUTHORITY")
		}
		return nil
	})
}
