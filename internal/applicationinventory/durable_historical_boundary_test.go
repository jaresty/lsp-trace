//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Optional immutable accepted-state closure; copied before mutation. No native admission is fabricated.
const historicalRun = "2e8d5a62e7e7f611898d6a188ea322e591d2654f42e4f4490934c87dbaa012ef"

func histBudget(t *testing.T) *sc.CallerBudgetV3 {
	t.Helper()
	b, e := sc.NewCallerBudgetV3(time.Now().Add(30*time.Second), 64<<20)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func histRead(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func histWrite(t *testing.T, p string, b []byte) {
	t.Helper()
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func histFixture(t *testing.T) (DurableStore, durableInventoryHead, durableInventoryState) {
	t.Helper()
	fixtureRoot := os.Getenv("LSP_TRACE_OWNER_FIXTURE")
	if fixtureRoot == "" {
		t.Skip("optional retained accepted-state closure not configured")
	}
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	e := filepath.WalkDir(fixtureRoot, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(fixtureRoot, p)
		q := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(q, 0700)
		}
		return os.WriteFile(q, histRead(t, p), 0600)
	})
	if e != nil {
		t.Fatal(e)
	}
	r, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	s := DurableStore{Root: r, Directory: dir}
	h, _, e := s.head(historicalRun)
	if e != nil {
		t.Fatalf("head baseline: %v", e)
	}
	if e = s.accepted(h); e != nil {
		t.Fatalf("accepted baseline: %v", e)
	}
	v, e := s.state(context.Background(), h, histBudget(t))
	if e != nil {
		t.Fatalf("state baseline: %v", e)
	}
	if len(v.Descriptions) != 3 {
		t.Fatalf("descriptions %d", len(v.Descriptions))
	}
	return s, h, v
}
func TestHistoricalBoundary(t *testing.T) {
	ctx := context.Background()
	t.Run("00_baseline", func(t *testing.T) {
		s, h, v := histFixture(t)
		if time.Now().UnixNano() <= h.RunDeadline {
			t.Fatal("fixture not historical")
		}
		ds, e := s.descriptions(ctx, v, histBudget(t))
		if e != nil || len(ds) != 3 {
			t.Fatalf("descriptions baseline %d %v", len(ds), e)
		}
		t.Log("head -> accepted historic proof -> state -> descriptions: PASS")
	})
	for _, kind := range []string{"description-corrupt", "description-swap", "unreferenced-rehash", "state-order", "head-state-ref", "preparation-delete", "preparation-corrupt"} {
		t.Run(kind, func(t *testing.T) {
			s, h, v := histFixture(t)
			headPath := filepath.Join(s.Directory, inventoryHeadName(h.RunID))
			headBefore := histRead(t, headPath)
			acceptBefore := histRead(t, filepath.Join(s.Directory, h.Acceptance))
			receiptBefore := histRead(t, filepath.Join(s.Directory, h.Acceptance+".receipt"))
			statePath := filepath.Join(s.Directory, "inventory-state-"+h.State.ID+".json")
			stateBefore := histRead(t, statePath)
			desc := filepath.Join(s.Directory, "inventory-description-"+v.Descriptions[0].ID+".json")
			prep := filepath.Join(s.Directory, "inventory-evidence-"+v.Prepared[0].ID+".json")
			switch kind {
			case "description-corrupt":
				raw := histRead(t, desc)
				raw[len(raw)/2] ^= 1
				histWrite(t, desc, raw)
			case "description-swap":
				histWrite(t, desc, histRead(t, filepath.Join(s.Directory, "inventory-description-"+v.Descriptions[1].ID+".json")))
			case "unreferenced-rehash":
				raw := append(histRead(t, desc), ' ')
				ref := publication.StoredObject{ID: publication.DigestForTest(raw)[7:], Bytes: len(raw)}
				if ref == v.Descriptions[0] {
					t.Fatal("unchanged mutation")
				}
				histWrite(t, filepath.Join(s.Directory, "inventory-description-"+ref.ID+".json"), raw)
			case "state-order":
				v.Descriptions[0], v.Descriptions[1] = v.Descriptions[1], v.Descriptions[0]
				raw, e := json.Marshal(v)
				if e != nil {
					t.Fatal(e)
				}
				histWrite(t, statePath, raw)
			case "head-state-ref":
				h.State.ID = v.Descriptions[0].ID
				raw, e := encodeInventoryHead(h)
				if e != nil {
					t.Fatal(e)
				}
				histWrite(t, headPath, raw)
			case "preparation-delete":
				if e := os.Remove(prep); e != nil {
					t.Fatal(e)
				}
			case "preparation-corrupt":
				raw := histRead(t, prep)
				raw[len(raw)/2] ^= 1
				histWrite(t, prep, raw)
			}
			if !bytes.Equal(acceptBefore, histRead(t, filepath.Join(s.Directory, h.Acceptance))) || !bytes.Equal(receiptBefore, histRead(t, filepath.Join(s.Directory, h.Acceptance+".receipt"))) {
				t.Fatal("proof mutated")
			}
			if kind != "head-state-ref" && !bytes.Equal(headBefore, histRead(t, headPath)) {
				t.Fatal("head mutated")
			}
			if kind != "state-order" && !bytes.Equal(stateBefore, histRead(t, statePath)) {
				t.Fatal("state mutated")
			}
			got, _, e := s.head(h.RunID)
			if e != nil {
				t.Fatalf("head decode stage: %v", e)
			}
			e = s.accepted(got)
			if kind == "head-state-ref" {
				if !errors.Is(e, publication.ErrTimedCompletion) {
					t.Fatalf("accepted expected ErrTimedCompletion, got %v", e)
				}
				t.Log("changed-head: accepted -> ErrTimedCompletion; stored acceptance unchanged")
				return
			}
			if e != nil {
				t.Fatalf("accepted unexpectedly failed %v", e)
			}
			v, e = s.state(ctx, got, histBudget(t))
			if kind == "state-order" {
				if !errors.Is(e, ErrDurableInventory) {
					t.Fatalf("state expected ErrDurableInventory got %v", e)
				}
				t.Log("state -> ReadObject old reference -> ErrDurableInventory")
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			_, e = s.descriptions(ctx, v, histBudget(t))
			switch kind {
			case "description-corrupt", "description-swap":
				if !errors.Is(e, sc.ErrDurableDescribeV1) {
					t.Fatalf("reopen expected ErrDurableDescribeV1 got %v", e)
				}
				if _, re := publication.ReadObject(s.Root, "inventory-description-", v.Descriptions[0]); re == nil {
					t.Fatal("ReadObject accepted corrupt description")
				}
				t.Log("descriptions -> ReopenInventoryDescription -> ReadObject -> ErrDurableDescribeV1")
			case "preparation-delete", "preparation-corrupt":
				if e != nil {
					t.Fatalf("ordinary historical reopen %v", e)
				}
				if _, e = sc.ReopenInventoryDescription(ctx, s.Root, v.Descriptions[0], histBudget(t)); e != nil {
					t.Fatalf("intrinsic reopen %v", e)
				}
				t.Log("intrinsic and ordinary reopen pass without historical preparation")
			default:
				if e != nil {
					t.Fatalf("unreferenced object affected traversal %v", e)
				}
				t.Log("rehash ignored: accepted state refs unchanged")
			}
		})
	}
}
