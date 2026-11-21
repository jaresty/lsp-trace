package scopejob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScopeJobRepairRuntimeDepthBeforeEffects(t *testing.T) {
	c, _, _ := fixture(t)
	c.Roots = []string{"a/b"}
	c.Budget.Depth = 1
	// Nil base is an effect tripwire: any attempted prefix metadata operation
	// panics. This bypasses admission deliberately, without a real metadata probe.
	cur := &cursor{config: c, roots: c.Roots}
	defer func() {
		if v := recover(); v != nil {
			t.Errorf("prefix filesystem effect before depth check: %v", v)
		}
	}()
	if _, e := cur.step(); !errors.Is(e, ErrInvalid) {
		t.Fatalf("depth error=%v", e)
	}
	if len(cur.roots) != 1 {
		t.Fatal("rejected root consumed")
	}
}
func TestScopeJobRepairLegitimateInterruptionPlacement(t *testing.T) {
	for _, last := range []int{-1, 0, 1, 2} {
		t.Run(fmt.Sprint(last), func(t *testing.T) {
			c, root, store := fixture(t)
			for i := 0; i < 300; i++ {
				file(t, c.Workspace, fmt.Sprintf("%d.go", i))
			}
			seq := max(last, 0)
			r, _ := Start(context.Background(), root, c, Hooks{AfterClaim: func(n uint64) error {
				if int(n) == seq {
					return errors.New("interrupt")
				}
				return nil
			}})
			if last < 0 {
				if e := os.Remove(filepath.Join(store, filename(r.JobID, "claim", 0))); e != nil {
					t.Fatal(e)
				}
			}
			got, e := Inspect(root, r.JobID, time.Now().Add(48*time.Hour))
			if e != nil {
				t.Fatal(e)
			}
			want := initialSpent(got.Head)
			if last >= 0 {
				var ok bool
				want, ok = quote(got.Head, want, uint64(last+1)*BatchSteps)
				if !ok {
					t.Fatal("bad expected quote")
				}
				want.LogicalBytes = baseSpent().LogicalBytes + uint64(last+1)*BatchBytes
				want.Packets = 2 + uint64(last+1)*2
			}
			if got.Spent != want || got.PendingClaim != (last >= 0) || got.DenominatorComplete || !got.UnknownRemainder || got.Expired || got.Head.Expires != nil || got.Head.Config.JobDeadline != nil {
				t.Fatalf("%+v want spent=%+v", got, want)
			}
			var actual uint64
			entries, e := os.ReadDir(filepath.Join(store, "scope-"+r.JobID))
			if e != nil {
				t.Fatal(e)
			}
			for _, ent := range entries {
				info, e := ent.Info()
				if e != nil {
					t.Fatal(e)
				}
				actual += uint64(info.Size())
			}
			if actual != got.RetainedBytes {
				t.Fatalf("retained=%d actual=%d", got.RetainedBytes, actual)
			}
		})
	}
}
func TestScopeJobRepairOriginalWorkModelReadback(t *testing.T) {
	c, root, _ := fixture(t)
	c, e := Canonical(c)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	configRaw, _ := json.Marshal(c)
	h := Head{Schema: LegacySchema, ID: "00000000000000000000000000000001", Config: c, ConfigDigest: digest(configRaw), Started: now}
	raw, e := put(root, prefix(h.ID)+"head.json", h, HeadBytes)
	if e != nil {
		t.Fatal(e)
	}
	spent := baseSpent()
	spent.Steps = 128
	spent.Packets += 2
	spent.LogicalBytes += BatchBytes
	spent.Work = 128 * uint64(8+c.Budget.Depth)
	q := claim{Sequence: 0, Previous: digest(raw), Spent: spent, Steps: 128, DiskEscrow: SegmentBytes, Frontier: c.Roots, Started: now, OperationDeadline: now.Add(c.Budget.OperationTimeout)}
	if _, e := put(root, filename(h.ID, "claim", 0), q, ClaimBytes); e != nil {
		t.Fatal(e)
	}
	got, e := Inspect(root, h.ID, now.Add(48*time.Hour))
	if e != nil || got.Spent != spent || !got.PendingClaim || got.Expired {
		t.Fatalf("legacy debit %+v error=%v", got, e)
	}
}
func TestScopeJobRepairFiniteStatusProbe(t *testing.T) {
	c, root, store := fixture(t)
	c.Budget.Packets = 4
	r, _ := Start(context.Background(), root, c, Hooks{AfterClaim: func(uint64) error { return errors.New("interrupt") }})
	for i := 0; i < 3; i++ {
		if e := os.WriteFile(filepath.Join(store, "scope-"+r.JobID, fmt.Sprintf("unrelated-%d", i)), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := Inspect(root, r.JobID, time.Now()); e == nil {
		t.Fatal("directory inspection ceiling ignored")
	}
}
func TestScopeJobRepairTerminalStates(t *testing.T) {
	for _, tc := range []struct {
		name, state, reason string
		done                bool
		last                string
		frontier            []string
		valid               bool
	}{
		{"ready", "SCOPE_READY", "ACQUISITION_NOT_IMPLEMENTED", true, "", nil, true},
		{"ready-frontier", "SCOPE_READY", "ACQUISITION_NOT_IMPLEMENTED", true, "", []string{"."}, false},
		{"ready-undone", "SCOPE_READY", "ACQUISITION_NOT_IMPLEMENTED", false, "", nil, false},
		{"ready-failed", "SCOPE_READY", "ACQUISITION_NOT_IMPLEMENTED", true, "INTERRUPTED", nil, false},
		{"partial-empty-reason", "PARTIAL", "", false, "", []string{"."}, false},
		{"partial-made-up", "PARTIAL", "MADE_UP", false, "", []string{"."}, false},
		{"partial-mismatch", "PARTIAL", "ENTRY_LIMIT", false, "INTERRUPTED", []string{"."}, false},
		{"partial-matched", "PARTIAL", "INTERRUPTED", false, "INTERRUPTED", []string{"."}, true},
		{"partial-close-after-done", "PARTIAL", "CLOSE_FAILED", true, "", nil, true},
		{"budget", "PARTIAL", "BUDGET_EXHAUSTED", false, "", []string{"."}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := stop{State: tc.state, Reason: tc.reason, Frontier: tc.frontier}
			if validStop(s, tc.done, tc.last) != tc.valid {
				t.Fatal("terminal intrinsic validation disagrees")
			}
		})
	}
}
func TestScopeJobRepairCountPartitions(t *testing.T) {
	c, _, _ := fixture(t)
	c.Excludes = []string{"skip.go"}
	c.Prunes = []string{"pruned"}
	records := []Record{
		{Path: "a.go", Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED", MetadataObserved: true},
		{Path: "skip.go", Kind: "REGULAR_FILE", Role: "EXCLUDED", MetadataObserved: true},
		{Path: "a.ts", Kind: "REGULAR_FILE", Role: "UNSUPPORTED_LANGUAGE", MetadataObserved: true},
		{Path: "notes", Kind: "REGULAR_FILE", Role: "NOT_SOURCE", MetadataObserved: true},
		{Path: "dir", Kind: "DIRECTORY", Role: "DIRECTORY", Mode: uint32(os.ModeDir), MetadataObserved: true},
		{Path: "pruned", Kind: "DIRECTORY", Role: "EXCLUDED", Mode: uint32(os.ModeDir), MetadataObserved: true, UnknownDescendants: true},
		{Path: "closed", Kind: "DIRECTORY", Role: "UNAVAILABLE", Mode: uint32(os.ModeDir), MetadataObserved: true, UnknownDescendants: true},
		{Path: "missing", Kind: "OTHER", Role: "UNAVAILABLE", UnknownDescendants: true},
		{Path: "link", Kind: "OTHER", Role: "UNSUPPORTED_KIND", Mode: uint32(os.ModeSymlink), MetadataObserved: true},
	}
	// Every prefix in every rotation checks mixed partitions rather than just a
	// single selected-file counterexample. Expected values are derived independently.
	for rotation := range records {
		var n Counts
		var regularExcluded, dirRoles, other, unavailableDirs uint64
		for k := range records {
			r := records[(rotation+k)%len(records)]
			if !validRecord(c, r) {
				t.Fatal(r)
			}
			n.add(r)
			if r.Kind == "REGULAR_FILE" && r.Role == "EXCLUDED" {
				regularExcluded++
			}
			if r.Role == "DIRECTORY" {
				dirRoles++
			}
			if r.Kind == "OTHER" {
				other++
			}
			if r.Kind == "DIRECTORY" && r.Role == "UNAVAILABLE" {
				unavailableDirs++
			}
			if n.Entries != n.Regular+n.Directories+other || n.Regular != n.Selected+n.UnsupportedLanguage+n.NotSource+regularExcluded || n.Entries != dirRoles+n.Selected+n.Excluded+n.UnsupportedLanguage+n.NotSource+n.UnsupportedKind+n.Unavailable || n.Boundaries != n.Excluded-regularExcluded+n.Unavailable || n.Directories != dirRoles+n.Excluded-regularExcluded+unavailableDirs {
				t.Fatalf("partition rotation=%d prefix=%d counts=%+v", rotation, k, n)
			}
		}
	}
}
