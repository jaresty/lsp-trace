//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func normalizedCreateConfig() OwnedInventoryConfig {
	c := ownedTestConfig()
	c.Schema = ownedConfigVersion
	c.BudgetBytes = durableRunLimit
	c.MaxAdmittedEvidenceBytes = 2 << 30
	return c
}

func TestOwnedCreateQuoteBounds(t *testing.T) {
	for _, n := range []int{-1, 0, 32769, int(^uint(0) >> 1)} {
		if _, e := ownedCreationQuote(n); e == nil {
			t.Fatal("invalid quote", n)
		}
	}
	for _, n := range []int{1, 32767, 32768} {
		q, e := ownedCreationQuote(n)
		if e != nil || q != uint64(8*n)+32*(65536+16384)+32768 {
			t.Fatal("quote", n, q, e)
		}
	}
	for _, target := range []int{32767, 32768, 32769} {
		c := normalizedCreateConfig()
		c.Scope = strings.Repeat("<", 4096)
		c.Provenance = strings.Repeat("a", 4096)
		raw, _ := json.Marshal(c)
		pad := target - len(raw)
		if pad < 0 || pad+len(c.Policy.RolePrompts[0]) > 4096 {
			t.Fatal("boundary setup", pad)
		}
		c.Policy.RolePrompts[0] += strings.Repeat("a", pad)
		raw, _ = json.Marshal(c)
		if len(raw) != target {
			t.Fatal(len(raw))
		}
		s := ownedTestStore(t)
		j, e := s.Create(context.Background(), c)
		if target <= 32768 {
			if e != nil || j.Used != uint64(target)*8+ownedCreateMinimum {
				t.Fatal(target, j, e)
			}
		} else {
			xs, _ := os.ReadDir(s.Directory)
			if e == nil || len(xs) != 0 {
				t.Fatal("over-bound effects", e, xs)
			}
		}
	}
	c := normalizedCreateConfig()
	c.Scope = strings.Repeat("a", 4096)
	c.Provenance = strings.Repeat("a", 4096)
	c.Policy.RolePrompts = [2]string{strings.Repeat("a", 4096), strings.Repeat("a", 4096)}
	// Solve the small decimal-length dependency before checking the exact funding boundary.
	for i := 0; i < 4; i++ {
		raw, _ := json.Marshal(c)
		q, _ := ownedCreationQuote(len(raw))
		c.BudgetBytes = q + c.AdmissionBytes + ownedSealCost
	}
	s := ownedTestStore(t)
	j, e := s.Create(context.Background(), c)
	if e != nil || j.Used+c.AdmissionBytes+ownedSealCost != c.BudgetBytes {
		t.Fatal("exact funding", j, e)
	}
	c.BudgetBytes--
	s = ownedTestStore(t)
	if _, e = s.Create(context.Background(), c); e == nil {
		t.Fatal("underfunded Create")
	}
	xs, _ := os.ReadDir(s.Directory)
	if len(xs) != 0 {
		t.Fatal("underfunded effects")
	}
	c = normalizedCreateConfig()
	c.Scope = string([]byte{0xff})
	s = ownedTestStore(t)
	if _, e = s.Create(context.Background(), c); e == nil {
		t.Fatal("lossy config encoding")
	}
	xs, _ = os.ReadDir(s.Directory)
	if len(xs) != 0 {
		t.Fatal("lossy config had effects")
	}
}

func TestOwnedCreateFailureBoundaries(t *testing.T) {
	for _, mode := range []string{"before-claim", "after-claim", "after-config", "config-write", "stale-final"} {
		t.Run(mode, func(t *testing.T) {
			s := ownedTestStore(t)
			c := normalizedCreateConfig()
			run := strings.Repeat("c", 64)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			raw, _ := json.Marshal(c)
			quote, _ := ownedCreationQuote(len(raw))
			seen := false
			j, e := s.createOwned(ctx, c, run, func(stage string) error {
				if stage == mode {
					seen = true
					cancel()
				}
				if mode == "config-write" && stage == "after-claim" {
					seen = true
					return os.Mkdir(filepath.Join(s.Directory, "inventory-owned-config-"+ownedConfigReference(raw).ID+".json"), 0700)
				}
				if mode == "stale-final" && stage == "after-config" {
					seen = true
					h, old, e := s.head(run)
					if e != nil {
						return e
					}
					h.Issued--
					h.Deadline--
					_, e = s.change(context.Background(), old, h)
					return e
				}
				return nil
			})
			if !seen || e == nil {
				t.Fatal("fault not reached/refused", mode, j, e)
			}
			if mode == "before-claim" {
				xs, _ := os.ReadDir(s.Directory)
				if len(xs) != 0 || j.RunID != "" {
					t.Fatal("pre-effect cancellation", j, xs)
				}
				return
			}
			info, configErr := os.Stat(filepath.Join(s.Directory, "inventory-owned-config-"+ownedConfigReference(raw).ID+".json"))
			if mode == "after-claim" {
				if !os.IsNotExist(configErr) {
					t.Fatal("config published after canceled claim", configErr)
				}
			} else if configErr != nil || mode != "config-write" && !info.Mode().IsRegular() {
				t.Fatal("config failure boundary", mode, configErr)
			}
			if j.RunID != run || j.Status != "PARTIAL_UNCERTAIN" || j.Used != quote || j.Phase != "Creating" || j.Admissions != 0 {
				t.Fatal("lost charged intent", j, e)
			}
			h, _, e := s.head(run)
			if e != nil || h.Creation == nil || h.Creation.Model != ownedCreateModel || h.Creation.Config != c || !ownedCreationValid(h) {
				t.Fatal("self-contained intent", e)
			}
			reopened, e := publication.OpenRoot(s.Directory)
			if e != nil {
				t.Fatal(e)
			}
			defer reopened.Close()
			ss := DurableStore{Root: reopened, Directory: s.Directory}
			again, e := ss.JobStatus(context.Background(), run)
			if e != nil || again != j {
				t.Fatal("readback", again, e)
			}
			if _, e = ss.Seal(context.Background(), run, j.Head); e == nil {
				t.Fatal("sealed creation uncertainty")
			}
			if _, e = ss.ReserveAdmission(context.Background(), run, j.Head, InventoryAdmissionBinding{RequestID: "request", SourceUnitID: "unit"}); e == nil {
				t.Fatal("admitted creation uncertainty")
			}
			if _, e = ss.Task(context.Background(), run, ""); e == nil {
				t.Fatal("creation task")
			}
		})
	}
}
func TestOwnedCreateCollisionAndVersionRefusal(t *testing.T) {
	s := ownedTestStore(t)
	ctx := context.Background()
	c := normalizedCreateConfig()
	run := strings.Repeat("d", 64)
	j, e := s.createOwned(ctx, c, run, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, before, e := s.head(run)
	if e != nil {
		t.Fatal(e)
	}
	c.Scope = strings.Repeat("<", 4096)
	collision, e := s.createOwned(ctx, c, run, nil)
	if e == nil || collision != j {
		t.Fatal("collision lost original quote", collision, e)
	}
	_, after, e := s.head(run)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("collision mutation")
	}
	h, _, _ := s.head(run)
	h.Schema = "lsp-trace.durable-inventory-head.v3"
	h.Used = 128 << 10
	h.Work = h.Used * 32
	oldConfig, _ := json.Marshal(normalizedCreateConfig())
	oldConfig = bytes.ReplaceAll(oldConfig, []byte(ownedConfigVersion), []byte("lsp-trace.inventory-owned-config.v1"))
	oldConfig = bytes.ReplaceAll(oldConfig, []byte("MaxAdmittedEvidenceBytes"), []byte("RetainedBytes"))
	h.State, e = publication.StoreObject(s.Root, "inventory-owned-config-", oldConfig)
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := encodeInventoryHead(h)
	if e = os.WriteFile(filepath.Join(s.Directory, inventoryHeadName(run)), encoded, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.JobStatus(ctx, run); !errors.Is(e, ErrUnsupportedStoredVersion) {
		t.Fatal("v3 not explicitly refused", e)
	}
	preserved, e := os.ReadFile(filepath.Join(s.Directory, inventoryHeadName(run)))
	if e != nil || !bytes.Equal(preserved, encoded) {
		t.Fatal("v3 historical debit changed")
	}
	var config OwnedInventoryConfig
	if strictInventory(oldConfig, &config) == nil {
		t.Fatal("old storage field reinterpreted")
	}
}

func TestOwnedCreateExpiredBeforeEffects(t *testing.T) {
	s := ownedTestStore(t)
	c := ownedTestConfig()
	c.OperationWindow = time.Nanosecond
	if _, e := s.Create(context.Background(), c); e == nil {
		t.Fatal("expected expiry")
	}
	xs, e := os.ReadDir(s.Directory)
	if e != nil {
		t.Fatal(e)
	}
	if len(xs) != 0 {
		t.Fatalf("expired Create left %d uncharged artifacts", len(xs))
	}
}
func TestOwnedCreateEncodedQuote(t *testing.T) {
	for _, scope := range []string{strings.Repeat("a", 4096), strings.Repeat("<", 4096)} {
		s := ownedTestStore(t)
		c := ownedTestConfig()
		c.Scope = scope
		j, e := s.Create(context.Background(), c)
		if e != nil {
			t.Fatal(e)
		}
		h, _, e := s.head(j.RunID)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := publication.ReadObject(s.Root, "inventory-owned-config-", h.State)
		if e != nil {
			t.Fatal(e)
		}
		if j.Used < uint64(len(raw))*8+32768 {
			t.Fatalf("config=%d charge=%d below config IO plus overhead", len(raw), j.Used)
		}
	}
}
