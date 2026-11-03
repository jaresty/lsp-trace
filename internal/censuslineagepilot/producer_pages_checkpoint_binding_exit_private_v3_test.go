//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRetainedCheckpointPrivateV3BindingExit(t *testing.T) {
	ctx := context.Background()
	x := retainedFixtureV3(t)
	owner, err := x.admit(ctx, x.raw, x.d)
	if err != nil {
		t.Fatal(err)
	}
	grant := x.l.state.allowance
	b, err := owner.SelectedBatch(ctx, 0, grant)
	if err != nil {
		t.Fatal(err)
	}
	v, err := b.Read(ctx, grant)
	if err != nil {
		t.Fatal(err)
	}
	expected := RetainedSelectedBatchBindingPrivateV3{v.CheckpointSHA256, v.RuntimeSHA256, v.ObjectSHA256}
	good := func() {
		t.Helper()
		if e := b.RevalidateBindingPrivateV3(ctx, expected, grant); e != nil {
			t.Fatal("BINDING_CURRENT", e)
		}
	}
	good()
	for _, kind := range []string{"zero", "copy-self", "owner-self", "owner-failed", "foreign-checkpoint", "foreign-runtime", "bad-format", "unit", "policy", "hash"} {
		t.Run(kind, func(t *testing.T) {
			batch, want := b, expected
			restore := func() {}
			switch kind {
			case "zero":
				batch = RetainedSelectedBatchPrivateV3{}
			case "copy-self":
				copy := *b.state
				batch.state = &copy
			case "owner-self":
				self := owner.state.self
				owner.state.self = nil
				restore = func() { owner.state.self = self }
			case "owner-failed":
				owner.state.failed = true
				restore = func() { owner.state.failed = false }
			case "foreign-checkpoint":
				want.CheckpointSHA256 = Digest([]byte("foreign"))
			case "foreign-runtime":
				want.RuntimeSHA256 = Digest([]byte("foreign"))
			case "bad-format":
				want.ObjectSHA256[0] = "bad"
			case "unit":
				raw := b.state.raw[0]
				b.state.raw[0] = make([]byte, x.m.Revision.Policy.ComputationUnitBytes+1)
				restore = func() { b.state.raw[0] = raw }
			case "policy":
				policy := owner.state.manifest.Revision.Policy
				owner.state.manifest.Revision.Policy.ComputationUnitBytes = 16<<20 + 1
				restore = func() { owner.state.manifest.Revision.Policy = policy }
			case "hash":
				want.ObjectSHA256[2] = Digest([]byte("foreign"))
			}
			defer restore()
			before := owner.BudgetSpent()
			e := batch.RevalidateBindingPrivateV3(ctx, want, grant)
			if e == nil {
				t.Fatal("BINDING_REFUSAL", kind)
			}
			if kind == "hash" {
				if owner.BudgetSpent() == before {
					t.Fatal("BINDING_HASH_NONREFUND")
				}
			} else if owner.BudgetSpent() != before {
				t.Fatal("BINDING_EARLY_NO_WORK", kind)
			}
			restore()
			good()
			t.Log("BINDING_REFUSAL: refused mutation, restored A-pass", kind)
		})
	}
	var n uint64
	for _, raw := range b.state.raw {
		n += uint64(len(raw))
	}
	price := PageCostPrivateV3{LogicalBytes: 8 * (n + 1), ReadHashBytes: 16 * (n + 1), Work: n + producerPacketTopologyWorkPrivateV2 + 64}
	admitted := costWitnessSumV3(price, pageLedgerOverheadPrivateV3)
	for _, currency := range []string{"logical", "readhash", "write", "work", "zero", "nested"} {
		t.Run(currency, func(t *testing.T) {
			left := admitted
			switch currency {
			case "logical":
				left.LogicalBytes--
			case "readhash":
				left.ReadHashBytes--
			case "write":
				left.WriteBytes--
			case "work":
				left.Work--
			case "zero":
				left = PageCostPrivateV3{}
			}
			before := owner.BudgetSpent()
			if e := b.RevalidateBindingPrivateV3(ctx, expected, left); !errors.Is(e, ErrPagesPausedPrivateV3) {
				t.Fatal("BINDING_PREPAY", e)
			}
			spent := owner.BudgetSpent().minus(before)
			want := PageCostPrivateV3{}
			if currency == "nested" {
				want = admitted
			}
			if spent != want {
				t.Fatalf("BINDING_PREFIX: got %+v want %+v", spent, want)
			}
			good()
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	before := owner.BudgetSpent()
	if e := b.RevalidateBindingPrivateV3(canceled, expected, grant); !errors.Is(e, context.Canceled) || owner.BudgetSpent() != before {
		t.Fatal("BINDING_CANCEL", e)
	}
	if e := b.RevalidateBindingPrivateV3(nil, expected, grant); !errors.Is(e, ErrInvalid) {
		t.Fatal("BINDING_NIL", e)
	}
	// Mutating a returned DTO cannot alter the sealed objects.
	v.SourceArtifact[0] ^= 1
	v.CanonicalSeeds[0] ^= 1
	good()
	for _, kind := range []string{"changed", "grown", "runtime", "root"} {
		t.Run(kind, func(t *testing.T) {
			x := retainedFixtureV3(t)
			owner, e := x.admit(ctx, x.raw, x.d)
			if e != nil {
				t.Fatal(e)
			}
			b, e := owner.SelectedBatch(ctx, 0, grant)
			if e != nil {
				t.Fatal(e)
			}
			v, e := b.Read(ctx, grant)
			if e != nil {
				t.Fatal(e)
			}
			expected := RetainedSelectedBatchBindingPrivateV3{v.CheckpointSHA256, v.RuntimeSHA256, v.ObjectSHA256}
			if e = b.RevalidateBindingPrivateV3(ctx, expected, grant); e != nil {
				t.Fatal(e)
			}
			var restore func()
			if kind == "runtime" {
				left := grant
				d, e := owner.state.findObject(ctx, 6, &left)
				if e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(x.m.Roots[0].Path, d.LastPage)
				old, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(path, []byte("corrupt"), 0600); e != nil {
					t.Fatal(e)
				}
				restore = func() {
					if e := os.WriteFile(path, old, 0600); e != nil {
						t.Fatal(e)
					}
				}
			} else if kind == "root" {
				path := x.m.Roots[0].Path
				moved := path + "-moved"
				if e := os.Rename(path, moved); e != nil {
					t.Fatal(e)
				}
				if e := os.Mkdir(path, 0700); e != nil {
					t.Fatal(e)
				}
				restore = func() {
					if e := os.Remove(path); e != nil {
						t.Fatal(e)
					}
					if e := os.Rename(moved, path); e != nil {
						t.Fatal(e)
					}
				}
			} else {
				f := x.fresh.state.files[0]
				path := filepath.Join(x.fresh.state.spec.Root, f.Path)
				raw := bytes.Repeat([]byte("x"), len(f.Bytes))
				if kind == "grown" {
					raw = bytes.Repeat([]byte("x"), x.fresh.state.spec.EffectivePolicy.FileBytes+1)
				}
				if e := os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
				restore = func() {
					if e := os.WriteFile(path, f.Bytes, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			defer func() { restore() }()
			before := owner.BudgetSpent()
			if e := b.RevalidateBindingPrivateV3(ctx, expected, grant); e == nil {
				t.Fatal("BINDING_FRESH_REFUSAL", kind)
			}
			if owner.BudgetSpent() == before {
				t.Fatal("BINDING_FAILED_SPENT", kind)
			}
			restore()
			restore = func() {}
			if kind == "changed" || kind == "grown" {
				if e := b.RevalidateBindingPrivateV3(ctx, expected, grant); e == nil {
					t.Fatal("BINDING_STAMP_RESTORATION_NOT_AUTHORITY")
				}
				t.Log("BINDING_FRESH_REFUSAL: content restored; changed filesystem stamps still refuse", kind)
			} else {
				if e := b.RevalidateBindingPrivateV3(ctx, expected, grant); e != nil {
					t.Fatal("BINDING_RESTORED", e)
				}
				t.Log("BINDING_FRESH_REFUSAL: refused mutation, restored A-pass", kind)
			}
		})
	}
	// A new checkpoint generation cannot reuse the old selected binding.
	if _, e := owner.CheckpointCompletedBatch(ctx, b, Digest([]byte("next")), grant); e != nil {
		t.Fatal(e)
	}
	if e := b.RevalidateBindingPrivateV3(ctx, expected, grant); !errors.Is(e, ErrInvalid) {
		t.Fatal("BINDING_GENERATION", e)
	}
}
