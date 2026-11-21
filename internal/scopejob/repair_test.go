package scopejob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScopeJobRepairLedgerGaps(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		index      uint64
		orphan     bool
	}{
		{"first-result", "segment", 0, false}, {"middle-result", "segment", 1, false},
		{"first-claim", "claim", 0, false}, {"middle-claim", "claim", 1, false},
		{"orphan-result", "claim", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, root, store := fixture(t)
			for i := 0; i < 300; i++ {
				file(t, c.Workspace, fmt.Sprintf("%d.go", i))
			}
			crash := errors.New("controlled interruption")
			r, e := Start(context.Background(), root, c, Hooks{AfterClaim: func(n uint64) error {
				if n == 2 {
					return crash
				}
				return nil
			}})
			if !errors.Is(e, crash) {
				t.Fatal(e)
			}
			before, e := Inspect(root, r.JobID, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			if before.Spent.Steps != 384 || !before.PendingClaim {
				t.Fatal("bad fixture")
			}
			if tc.orphan {
				for _, n := range []string{filename(r.JobID, "claim", 1), filename(r.JobID, "segment", 1), filename(r.JobID, "claim", 2)} {
					if e := os.Remove(filepath.Join(store, n)); e != nil {
						t.Fatal(e)
					}
				}
			}
			if e := os.Remove(filepath.Join(store, filename(r.JobID, tc.kind, tc.index))); e != nil {
				t.Fatal(e)
			}
			if got, e := Inspect(root, r.JobID, time.Now()); e == nil {
				t.Fatalf("gap accepted: spent=%+v pending=%v", got.Spent, got.PendingClaim)
			}
		})
	}
}
func TestScopeJobRepairSparseOrphan(t *testing.T) {
	for _, kind := range []string{"claim", "segment", "nonsense"} {
		t.Run(kind, func(t *testing.T) {
			c, root, store := fixture(t)
			r, _ := Start(context.Background(), root, c, Hooks{AfterClaim: func(uint64) error { return errors.New("interrupt") }})
			if e := os.WriteFile(filepath.Join(store, filename(r.JobID, kind, c.Budget.Packets-1)), []byte("{}"), 0600); e != nil {
				t.Fatal(e)
			}
			if _, e := Inspect(root, r.JobID, time.Now()); e == nil {
				t.Fatal("sparse orphan accepted")
			}
		})
	}
}
func TestScopeJobRepairDepthAndNULAdmission(t *testing.T) {
	for _, name := range []string{"depth", "root-nul", "prune-nul", "include-nul", "exclude-nul", "workspace-nul"} {
		t.Run(name, func(t *testing.T) {
			c, root, store := fixture(t)
			switch name {
			case "depth":
				c.Roots = []string{strings.Repeat("d/", 127) + "d"}
				c.Budget.Depth = 1
				c.Budget.Steps = 1
				c.Budget.Work = 9
			case "root-nul":
				c.Roots = []string{"bad\x00root"}
			case "prune-nul":
				c.Prunes = []string{"bad\x00root"}
			case "include-nul":
				c.Includes = []string{"bad\x00*"}
			case "exclude-nul":
				c.Excludes = []string{"bad\x00*"}
			case "workspace-nul":
				c.Workspace += "\x00"
			}
			if _, e := Canonical(c); e == nil {
				t.Error("invalid configuration accepted")
			}
			started := false
			if _, e := Start(context.Background(), root, c, Hooks{Started: func(string) error { started = true; return nil }}); e == nil {
				t.Error("invalid start accepted")
			}
			names, e := os.ReadDir(store)
			if e != nil || len(names) != 0 || started {
				t.Fatalf("effects: names=%d started=%v error=%v", len(names), started, e)
			}
		})
	}
}
func TestScopeJobRepairRootDepthEquality(t *testing.T) {
	c, root, _ := fixture(t)
	file(t, c.Workspace, "a/b.go")
	c.Roots = []string{"a/b.go"}
	c.Budget.Depth = 2
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil || r.State != "SCOPE_READY" || r.Counts.Selected != 1 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestScopeJobRepairPolicyQuote(t *testing.T) {
	for _, n := range []int{0, 1, 128} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			c, _, _ := fixture(t)
			for i := 0; i < n; i++ {
				c.Prunes = append(c.Prunes, fmt.Sprintf("p%d", i))
				c.Includes = append(c.Includes, fmt.Sprintf("i%d", i))
				c.Excludes = append(c.Excludes, fmt.Sprintf("e%d", i))
				if i > 0 {
					c.Roots = append(c.Roots, fmt.Sprintf("r%d", i))
				}
			}
			// Eight fixed metadata/control units, root prefix checks, frontier frame/root
			// copying, prune checks, and pattern validation plus matching.
			want := uint64(8 + 2*c.Budget.Depth + len(c.Roots) + len(c.Prunes) + 2*(len(c.Includes)+len(c.Excludes)))
			c.Budget.Work = want
			got, ok := quote(Head{Config: c}, baseSpent(), 1)
			if !ok || got.Work != want {
				t.Fatalf("policy quote got=%d ok=%v want=%d", got.Work, ok, want)
			}
			c.Budget.Work = want - 1
			if _, ok := quote(Head{Config: c}, baseSpent(), 1); ok {
				t.Fatal("underfunded step admitted")
			}
		})
	}
}
func TestScopeJobRepairRecordSemantics(t *testing.T) {
	c, _, _ := fixture(t)
	c.Roots = []string{"src"}
	c.Excludes = []string{"skip.go"}
	c.Prunes = []string{"src/vendor"}
	for _, tc := range []struct {
		name  string
		r     Record
		valid bool
	}{
		{"selected", Record{Path: "src/a.go", Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED", MetadataObserved: true}, true},
		{"excluded", Record{Path: "src/skip.go", Kind: "REGULAR_FILE", Role: "EXCLUDED", MetadataObserved: true}, true},
		{"language", Record{Path: "src/a.ts", Kind: "REGULAR_FILE", Role: "UNSUPPORTED_LANGUAGE", MetadataObserved: true}, true},
		{"not-source", Record{Path: "src/a.txt", Kind: "REGULAR_FILE", Role: "NOT_SOURCE", MetadataObserved: true}, true},
		{"directory", Record{Path: "src", Kind: "DIRECTORY", Role: "DIRECTORY", Mode: uint32(os.ModeDir), MetadataObserved: true}, true},
		{"pruned", Record{Path: "src/vendor", Kind: "DIRECTORY", Role: "EXCLUDED", Mode: uint32(os.ModeDir), MetadataObserved: true, UnknownDescendants: true}, true},
		{"unavailable-dir", Record{Path: "src/closed", Kind: "DIRECTORY", Role: "UNAVAILABLE", Mode: uint32(os.ModeDir), MetadataObserved: true, UnknownDescendants: true}, true},
		{"unavailable", Record{Path: "src/missing", Kind: "OTHER", Role: "UNAVAILABLE", UnknownDescendants: true}, true},
		{"symlink", Record{Path: "src/link.go", Kind: "OTHER", Role: "UNSUPPORTED_KIND", Mode: uint32(os.ModeSymlink), MetadataObserved: true}, true},
		{"review", Record{Path: "src/a.go", Kind: "OTHER", Role: "SUPPORTED_UNACQUIRED"}, false},
		{"wrong-language", Record{Path: "src/a.ts", Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED", MetadataObserved: true}, false},
		{"wrong-filter", Record{Path: "src/skip.go", Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED", MetadataObserved: true}, false},
		{"outside-root", Record{Path: "a.go", Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED", MetadataObserved: true}, false},
		{"mode", Record{Path: "src/a.go", Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED", Mode: uint32(os.ModeDir), MetadataObserved: true}, false},
		{"missing-metadata", Record{Path: "src/a.go", Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED"}, false},
		{"false-boundary", Record{Path: "src/a.go", Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED", MetadataObserved: true, UnknownDescendants: true}, false},
		{"false-prune", Record{Path: "src/open", Kind: "DIRECTORY", Role: "EXCLUDED", Mode: uint32(os.ModeDir), MetadataObserved: true, UnknownDescendants: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validRecord(c, tc.r); got != tc.valid {
				t.Fatalf("record validity=%v want=%v", got, tc.valid)
			}
		})
	}
}
func TestScopeJobRepairHashValidImpossibleRecord(t *testing.T) {
	c, root, _ := fixture(t)
	r, _ := Start(context.Background(), root, c, Hooks{AfterClaim: func(uint64) error { return errors.New("interrupt") }})
	var q claim
	raw, e := get(root, filename(r.JobID, "claim", 0), &q, ClaimBytes)
	if e != nil {
		t.Fatal(e)
	}
	_, e = put(root, filename(r.JobID, "segment", 0), segment{Claim: digest(raw), Records: []Record{{Path: "a.go", Kind: "OTHER", Role: "SUPPORTED_UNACQUIRED"}}, Frontier: []string{"."}}, SegmentBytes)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Inspect(root, r.JobID, time.Now()); e == nil {
		t.Fatal("hash-valid impossible record accepted")
	}
}
func TestScopeJobRepairFrontier(t *testing.T) {
	for _, tc := range []struct {
		name     string
		frontier []string
		done     bool
		reason   string
	}{
		{"outside", []string{"outside"}, false, ""}, {"depth", []string{"src/a/b"}, false, ""},
		{"duplicate", []string{"src", "src"}, false, ""}, {"empty-undone", nil, false, ""},
		{"unknown-reason", []string{"src"}, false, "MADE_UP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, root, _ := fixture(t)
			c.Roots = []string{"src"}
			c.Budget.Depth = 2
			r, _ := Start(context.Background(), root, c, Hooks{AfterClaim: func(uint64) error { return errors.New("interrupt") }})
			var q claim
			raw, e := get(root, filename(r.JobID, "claim", 0), &q, ClaimBytes)
			if e != nil {
				t.Fatal(e)
			}
			_, e = put(root, filename(r.JobID, "segment", 0), segment{Claim: digest(raw), Frontier: tc.frontier, Done: tc.done, Reason: tc.reason}, SegmentBytes)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = Inspect(root, r.JobID, time.Now()); e == nil {
				t.Fatal("impossible frontier accepted")
			}
		})
	}
}
