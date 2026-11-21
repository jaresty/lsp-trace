//go:build darwin || linux

package provisionalfeaturecatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func intermediateBudget(t *testing.T) NativeBudgetV3 {
	t.Helper()
	b, e := NewCallerBudgetV3(time.Now().Add(30*time.Second), 64<<20)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// Data-only package fixture; never native admission, a task, or accepted head.
func intermediateFixture(t *testing.T, w NativeCatalogWireV3) IntermediateCatalogV3 {
	t.Helper()
	ctx := context.Background()
	b := intermediateBudget(t)
	if b.Reserve(ctx, nativeSealQuoteV3(w)) != nil {
		t.Fatal("reserve")
	}
	s, e := ownIntermediateEvidenceV3(ctx, b, w.Evidence)
	if e != nil {
		t.Fatal(e)
	}
	w.Evidence = s.values
	c, e := sealIntermediateReservedV3(ctx, b, w, "interpreter", false, s)
	if e != nil {
		t.Fatal(e)
	}
	return IntermediateCatalogV3{c, s}
}
func TestIntermediateCanonicalOwnership(t *testing.T) {
	ctx := context.Background()
	for _, body := range []string{"plain", "<>&\u2028\u2029\x00\"\\", string([]byte{'a', 0xff, 'b', 0xfe})} {
		w := sealerFixture(t).state.wire
		w.Actor = body
		w.Evidence[0].Unit.Body = body
		w.Evidence[0].Record = json.RawMessage(` { "text": "<>&" } `)
		w.Groups = []NativeGroupV3{{ID: "group", Label: body, Reason: body, Members: []string{"e1"}}}
		w.Affected = []string{"group"}
		w.WithMembership, w.WithoutMembership = 1, 1
		candidate := intermediateFixture(t, w)
		baseline, e := sealNativeV3(ctx, intermediateBudget(t), w, "interpreter", false)
		if e != nil {
			t.Fatal(e)
		}
		want, e := baseline.Bytes(ctx)
		if e != nil {
			t.Fatal(e)
		}
		got, e := candidate.Catalog().Bytes(ctx)
		if e != nil || !bytes.Equal(got, want) || baseline.Revision() != candidate.Catalog().Revision() {
			t.Fatal("canonical parity", e)
		}
		w.Evidence[0].Record[0] = 'x'
		w.Evidence[0].Unit.Body = "changed"
		w.Evidence[0].ID = "changed"
		w.Groups[0].Members[0] = "changed"
		w.Groups[0].Label = "changed"
		w.Affected[0] = "changed"
		got[0] = 'x'
		copy, e := candidate.Catalog().Bytes(ctx)
		if e != nil || !bytes.Equal(copy, want) {
			t.Fatal("snapshot aliases input/Bytes", e)
		}
	}
}
func TestIntermediateApplyParityAndOwnership(t *testing.T) {
	ctx := context.Background()
	base := sealerFixture(t)
	c := intermediateFixture(t, base.state.wire)
	raw := sealerDecision(t, base)
	p, e := c.Apply(ctx, raw, false)
	if e != nil {
		t.Fatal(e)
	}
	full, e := ApplyNativeDecisionV3(ctx, base, raw, false)
	if e != nil {
		t.Fatal(e)
	}
	x, _ := full.Bytes(ctx)
	y, _ := p.Catalog().Bytes(ctx)
	if !bytes.Equal(x, y) {
		t.Fatal("apply parity")
	}
	matches, e := p.Catalog().SearchGroups(ctx, "group", 10)
	if e != nil {
		t.Fatal(e)
	}
	matches[0].Members[0] = "mutation"
	raw[0] = 'x'
	y, _ = p.Catalog().Bytes(ctx)
	if !bytes.Equal(x, y) {
		t.Fatal("results/input alias")
	}
	if p.evidence != c.evidence {
		t.Fatal("snapshot was not shared")
	}
	review, _ := json.Marshal(NativeDecisionV3{Predecessor: full.Revision(), Actor: "interpreter", Reason: "review", Affected: []string{"group"}, Groups: []NativeGroupV3{}})
	full, e = ApplyNativeDecisionV3(ctx, full, review, true)
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.Apply(ctx, review, true)
	if e != nil {
		t.Fatal(e)
	}
	x, _ = full.Bytes(ctx)
	y, _ = p.Catalog().Bytes(ctx)
	if !bytes.Equal(x, y) {
		t.Fatal("review parity")
	}
}
func TestIntermediateStoreReopenGuards(t *testing.T) {
	for _, kind := range []string{"valid", "reference", "missing", "corrupt", "full-as-compact", "version", "metadata", "wrong-full", "reordered", "rehash-ref", "unknown-field", "duplicate-key", "null", "trailing"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			if e := os.Chmod(dir, 0700); e != nil {
				t.Fatal(e)
			}
			root, e := publication.OpenRoot(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer root.Close()
			c := intermediateFixture(t, sealerFixture(t).state.wire)
			ref, e := c.Store(ctx, root)
			if e != nil {
				t.Fatal(e)
			}
			full, _ := c.Catalog().Bytes(ctx)
			input, e := publication.StoreObject(root, "inventory-catalog-", full)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "unknown-field", "duplicate-key", "null", "trailing":
				raw, err := publication.ReadObject(root, intermediatePrefixV3, ref)
				if err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "unknown-field":
					raw = append([]byte(`{"unknown":0,`), raw[1:]...)
				case "duplicate-key":
					raw = append([]byte(`{"Version":"lsp-trace.intermediate-catalog.v1",`), raw[1:]...)
				case "null":
					raw = []byte("null")
				case "trailing":
					raw = append(raw, []byte("{}")...)
				}
				ref, err = publication.StoreObject(root, intermediatePrefixV3, raw)
				if err != nil {
					t.Fatal(err)
				}
			case "wrong-full":
				input.ID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			case "missing":
				if e = os.Remove(dir + "/" + intermediatePrefixV3 + ref.ID + ".json"); e != nil {
					t.Fatal(e)
				}
			case "corrupt":
				if e = os.WriteFile(dir+"/"+intermediatePrefixV3+ref.ID+".json", []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
			case "full-as-compact":
				ref, e = publication.StoreObject(root, intermediatePrefixV3, full)
				if e != nil {
					t.Fatal(e)
				}
			case "reference", "metadata", "version", "reordered", "rehash-ref":
				raw, e := publication.ReadObject(root, intermediatePrefixV3, ref)
				if e != nil {
					t.Fatal(e)
				}
				var r intermediateRecordV3
				if json.Unmarshal(raw, &r) != nil {
					t.Fatal("decode")
				}
				switch kind {
				case "rehash-ref":
					unowned := c.evidence.values[0]
					unowned.ID = "unreferenced-replacement"
					body, err := json.Marshal(unowned)
					if err != nil {
						t.Fatal(err)
					}
					r.Evidence[0], err = publication.StoreObject(root, "inventory-description-", body)
					if err != nil {
						t.Fatal(err)
					}
				case "reference":
					r.Evidence[0].ID = input.ID
				case "metadata":
					r.Metadata.Reason = "altered"
				case "version":
					r.Version = "unknown"
				case "reordered":
					r.Evidence[0], r.Evidence[1] = r.Evidence[1], r.Evidence[0]
				}
				raw, _ = json.Marshal(r)
				ref, e = publication.StoreObject(root, intermediatePrefixV3, raw)
				if e != nil {
					t.Fatal(e)
				}
			}
			got, e := c.Reopen(ctx, root, ref, input, false)
			if kind != "valid" {
				if e == nil {
					t.Fatal("guard accepted", kind)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			raw, _ := got.Catalog().Bytes(ctx)
			if !bytes.Equal(raw, full) {
				t.Fatal("reopen parity")
			}
			old, e := ReopenStoredNativeV3(ctx, root, input, intermediateBudget(t), "interpreter", false)
			if e != nil {
				t.Fatal("legacy reopen", e)
			}
			oldraw, _ := old.Bytes(ctx)
			if !bytes.Equal(raw, oldraw) {
				t.Fatal("old format changed")
			}
			if _, e = ReopenStoredNativeV3(ctx, root, ref, intermediateBudget(t), "interpreter", false); e == nil {
				t.Fatal("compact accepted as legacy")
			}
		})
	}
}
func TestIntermediateFailedSealRetryAndConcurrency(t *testing.T) {
	ctx := context.Background()
	c := intermediateFixture(t, sealerFixture(t).state.wire)
	raw := sealerDecision(t, c.catalog)
	_, e := applyNativeDecisionWithSeal(ctx, c.catalog, raw, false, func(ctx context.Context, b NativeBudgetV3, w NativeCatalogWireV3, a string, r bool) (NativeCatalogV3, error) {
		// Actual compact sealer rejects an owner mismatch under the shared lock.
		return sealIntermediateReservedV3(ctx, b, w, a, r, &intermediateEvidenceV3{})
	})
	if e != ErrNativeCatalogV3 || c.catalog.state.successor {
		t.Fatal("failed seal consumed")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(public bool) {
			defer wg.Done()
			var e error
			if public {
				_, e = ApplyNativeDecisionV3(ctx, c.Catalog(), raw, false)
			} else {
				_, e = c.Apply(ctx, raw, false)
			}
			if e == nil {
				successes.Add(1)
			} else if e != ErrNativeCatalogV3 {
				t.Errorf("error %v", e)
			}
		}(i%2 == 0)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("single successor", successes.Load())
	}
}
func TestIntermediateEmptyAndBudgetRefusal(t *testing.T) {
	ctx := context.Background()
	b := intermediateBudget(t)
	c, e := BuildIntermediateNativeV3(ctx, b, "interpreter", nil, ReviewModeSameThreadV3)
	if e != nil {
		t.Fatal(e)
	}
	old, e := BuildNativeV3(ctx, intermediateBudget(t), "interpreter", nil, ReviewModeSameThreadV3)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := c.Catalog().Bytes(ctx)
	z, _ := old.Bytes(ctx)
	if !bytes.Equal(a, z) {
		t.Fatal("empty parity")
	}
	tiny, e := NewCallerBudgetV3(time.Now().Add(time.Second), 4095)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ownIntermediateEvidenceV3(ctx, tiny, nil); e == nil || !tiny.Exhausted() {
		t.Fatal("pre-effect reservation")
	}
}
