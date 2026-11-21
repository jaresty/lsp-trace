package scopejob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestScopeJobBlockersSiblingIsolation(t *testing.T) {
	c, root, store := fixture(t)
	c.Budget.Packets = 4
	first, e := Start(context.Background(), root, c, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(store, prefix(first.JobID)+"head.json"))
	for i := 0; i < 8; i++ {
		if _, e = Start(context.Background(), root, c, Hooks{}); e != nil {
			t.Fatal("legitimate sibling failed", e)
		}
	}
	for i := 0; i < 100; i++ {
		file(t, store, fmt.Sprintf("unrelated-%d", i))
	}
	got, e := Inspect(root, first.JobID, time.Now().Add(48*time.Hour))
	if e != nil || !reflect.DeepEqual(first, got) {
		t.Fatalf("unchanged job changed: %v %+v", e, got)
	}
	after, _ := os.ReadFile(filepath.Join(store, prefix(first.JobID)+"head.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("old head rewritten")
	}
	if got.NamespaceVerification != "COMPLETE_PER_JOB" {
		t.Fatal(got.NamespaceVerification)
	}
	for _, name := range []string{"foreign", filename(first.JobID, "claim", 999999)} {
		name = filepath.Base(name)
		p := filepath.Join(store, "scope-"+first.JobID, name)
		if e = os.WriteFile(p, nil, 0600); e != nil {
			t.Fatal(e)
		}
		if _, e = Inspect(root, first.JobID, time.Now()); e == nil {
			t.Fatal("unexpected owned entry accepted")
		}
		if e = os.Remove(p); e != nil {
			t.Fatal(e)
		}
	}
}

func TestScopeJobBlockersReceiptFailureAndPrepayment(t *testing.T) {
	c, root, store := fixture(t)
	c.Budget.Work = NamespaceWork - 1
	if _, e := Start(context.Background(), root, c, Hooks{}); e == nil {
		t.Fatal("unpaid namespace admitted")
	}
	names, _ := os.ReadDir(store)
	if len(names) != 0 {
		t.Fatal("pre-header effects")
	}
	c.Budget.Work = NamespaceWork
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil || r.Spent.Work != NamespaceWork || r.Counts.Entries != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	c.Budget = DefaultBudget()
	var id string
	r, e = Start(context.Background(), root, c, Hooks{Started: func(v string) error {
		id = v
		file(t, store, "scope-"+v+"/unexpected")
		return errors.New("receipt output interrupted")
	}})
	if e == nil || id == "" || r.JobID != id {
		t.Fatalf("published receipt ID lost: %q %q %v", id, r.JobID, e)
	}
}

func TestScopeJobBlockersNamespaceCustody(t *testing.T) {
	c, root, store := fixture(t)
	r, e := Start(context.Background(), root, c, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	child := filepath.Join(store, "scope-"+r.JobID)
	moved := child + "-moved"
	if e = os.Rename(child, moved); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(moved, child); e != nil {
		t.Fatal(e)
	}
	if _, e = Inspect(root, r.JobID, time.Now()); e == nil {
		t.Fatal("symlink job directory admitted")
	}
	if e = os.Remove(child); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(moved, child); e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(child, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e = Inspect(root, r.JobID, time.Now()); e == nil {
		t.Fatal("public child admitted")
	}
	if e = os.Chmod(child, 0700); e != nil {
		t.Fatal(e)
	}
	// Existing publication contract: the pinned root survives ambient replacement.
	original := store + "-original"
	if e = os.Rename(store, original); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = os.RemoveAll(original) })
	if e = os.Mkdir(store, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = Inspect(root, r.JobID, time.Now()); e != nil {
		t.Fatal("pinned read redirected", e)
	}
	if _, e = Start(context.Background(), root, c, Hooks{}); e != nil {
		t.Fatal("pinned write redirected", e)
	}
	names, _ := os.ReadDir(store)
	if len(names) != 0 {
		t.Fatal("ambient replacement written")
	}
}

func TestScopeJobBlockersRootClosure(t *testing.T) {
	for _, tc := range []struct {
		name          string
		first, second []Record
		frontier      []string
		valid         bool
	}{
		{"duplicate-omission", []Record{regular("a.go"), regular("a.go")}, nil, nil, false},
		{"omission", []Record{regular("a.go")}, nil, nil, false},
		{"reversed", []Record{regular("b.go"), regular("a.go")}, nil, nil, false},
		{"cross-segment-duplicate", []Record{regular("a.go")}, []Record{regular("a.go")}, []string{"b.go"}, false},
		{"two-roots", []Record{regular("a.go"), regular("b.go")}, nil, nil, true},
		{"two-segments", []Record{regular("a.go")}, []Record{regular("b.go")}, []string{"b.go"}, true},
		{"unavailable", []Record{{Path: "a.go", Kind: "OTHER", Role: "UNAVAILABLE", UnknownDescendants: true}, regular("b.go")}, nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, root, _ := fixture(t)
			c.Roots = []string{"a.go", "b.go"}
			r, e := Start(context.Background(), root, c, Hooks{AfterClaim: func(uint64) error { return errors.New("stop") }})
			if e == nil {
				t.Fatal("fixture")
			}
			var q claim
			qr, e := get(root, filename(r.JobID, "claim", 0), &q, ClaimBytes)
			if e != nil {
				t.Fatal(e)
			}
			s := segment{Claim: digest(qr), Records: tc.first, Frontier: tc.frontier, Done: tc.second == nil}
			sr, e := put(root, filename(r.JobID, "segment", 0), s, SegmentBytes)
			if e != nil {
				t.Fatal(e)
			}
			counts := Counts{}
			for _, r := range tc.first {
				counts.add(r)
			}
			if tc.second != nil {
				var h Head
				if _, e = get(root, prefix(r.JobID)+"head.json", &h, HeadBytes); e != nil {
					t.Fatal(e)
				}
				spent, ok := quote(h, q.Spent, BatchSteps)
				if !ok {
					t.Fatal("quote")
				}
				q.Sequence = 1
				q.Previous = digest(sr)
				q.Spent = spent
				q.Frontier = tc.frontier
				qr, e = put(root, filename(r.JobID, "claim", 1), q, ClaimBytes)
				if e != nil {
					t.Fatal(e)
				}
				sr, e = put(root, filename(r.JobID, "segment", 1), segment{Claim: digest(qr), Records: tc.second, Done: true}, SegmentBytes)
				if e != nil {
					t.Fatal(e)
				}
				for _, r := range tc.second {
					counts.add(r)
				}
			}
			if _, e = put(root, prefix(r.JobID)+"stop.json", stop{Previous: digest(sr), State: "SCOPE_READY", Reason: "ACQUISITION_NOT_IMPLEMENTED", Counts: counts}, StopBytes); e != nil {
				t.Fatal(e)
			}
			got, e := Inspect(root, r.JobID, time.Now())
			if (e == nil) != tc.valid {
				t.Fatalf("closure validity=%v want=%v report=%+v", e == nil, tc.valid, got)
			}
		})
	}
}
func regular(p string) Record {
	return Record{Path: p, Kind: "REGULAR_FILE", Role: "SUPPORTED_UNACQUIRED", MetadataObserved: true}
}
func directory(p string) Record {
	return Record{Path: p, Kind: "DIRECTORY", Role: "DIRECTORY", MetadataObserved: true, Mode: uint32(os.ModeDir)}
}

func TestScopeJobBlockersFrontierReplay(t *testing.T) {
	c, _, _ := fixture(t)
	c.Roots = []string{"a", "b"}
	for _, tc := range []struct {
		name     string
		records  []Record
		frontier []string
		done     bool
		steps    uint64
		valid    bool
	}{
		{"empty-directory", []Record{directory("a"), directory("b")}, nil, true, 4, true},
		{"unpaid-eof", []Record{directory("a"), directory("b")}, nil, true, 2, false},
		{"missing-parent", []Record{directory("a"), regular("a/child/f.go")}, []string{"b", "a"}, false, 128, false},
		{"invented-frame", []Record{directory("a")}, []string{"b", "a", "a/child"}, false, 128, false},
		{"unvisited-root-erased", []Record{directory("a")}, []string{"a"}, false, 128, false},
		{"valid-child", []Record{directory("a"), directory("a/child"), regular("a/child/f.go")}, []string{"b", "a", "a/child"}, false, 128, true},
		{"root-duplicate", []Record{directory("a"), directory("a")}, []string{"b", "a"}, false, 128, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := progress{}
			if got := p.segment(c, segment{Records: tc.records, Frontier: tc.frontier, Done: tc.done}, tc.steps); got != tc.valid {
				t.Fatalf("transition=%v want=%v", got, tc.valid)
			}
		})
	}
}

func TestScopeJobBlockersProductionRootControls(t *testing.T) {
	for _, kind := range []string{"files", "nested-overlap", "pruned", "ancestor-prune", "unavailable", "empty", "partial"} {
		t.Run(kind, func(t *testing.T) {
			c, root, _ := fixture(t)
			c.Roots = []string{"a", "b"}
			switch kind {
			case "files":
				file(t, c.Workspace, "a")
				file(t, c.Workspace, "b")
			case "nested-overlap":
				file(t, c.Workspace, "a/n/f.go")
				file(t, c.Workspace, "b/f.go")
				c.Roots = []string{"a", "a/n", "b"}
			case "pruned":
				file(t, c.Workspace, "a/f.go")
				file(t, c.Workspace, "b/f.go")
				c.Prunes = []string{"a"}
			case "ancestor-prune":
				file(t, c.Workspace, "a/n/f.go")
				c.Roots = []string{"a/n"}
				c.Prunes = []string{"a"}
			case "empty":
				for _, p := range c.Roots {
					if e := os.Mkdir(filepath.Join(c.Workspace, p), 0700); e != nil {
						t.Fatal(e)
					}
				}
			case "partial":
				file(t, c.Workspace, "a/f.go")
				c.Budget.Steps = 1
			}
			r, e := Start(context.Background(), root, c, Hooks{})
			if e != nil {
				t.Fatal(e)
			}
			if kind == "partial" {
				if r.DenominatorComplete || r.State != "PARTIAL" {
					t.Fatal(r)
				}
				return
			}
			if r.State != "SCOPE_READY" {
				t.Fatal(r)
			}
			if (kind == "pruned" || kind == "ancestor-prune" || kind == "unavailable") && r.DenominatorComplete {
				t.Fatal("unknown descendants complete")
			}
		})
	}
}

// Original closed header shape: not a new head with its marker removed.
type originalHead struct {
	Schema       string     `json:"schema"`
	ID           string     `json:"id"`
	Config       Config     `json:"config"`
	ConfigDigest string     `json:"config_digest"`
	Started      time.Time  `json:"started"`
	Expires      *time.Time `json:"expires"`
}

func TestScopeJobBlockersVersionMatrix(t *testing.T) {
	for _, model := range []string{"", "policy-v2"} {
		t.Run("legacy-"+model, func(t *testing.T) {
			c, root, store := fixture(t)
			c.Budget.Packets = 4
			c, _ = Canonical(c)
			now := time.Now().UTC()
			expired := now.Add(-time.Hour)
			c.JobDeadline = &expired
			id := "00000000000000000000000000000012"
			config, _ := json.Marshal(c)
			old := originalHead{LegacySchema, id, c, digest(config), now.Add(-2 * time.Hour), &expired}
			var value any = old
			if model != "" {
				value = Head{Schema: old.Schema, ID: id, Config: c, ConfigDigest: old.ConfigDigest, Started: old.Started, Expires: &expired, WorkModel: model}
			}
			raw, e := put(root, prefix(id)+"head.json", value, HeadBytes)
			if e != nil {
				t.Fatal(e)
			}
			spent := baseSpent()
			spent.Steps = 128
			spent.Packets += 2
			spent.LogicalBytes += BatchBytes
			if model == "" {
				spent.Work = 128 * uint64(8+c.Budget.Depth)
			} else {
				spent, _ = quote(Head{Config: c}, baseSpent(), 128)
			}
			q := claim{0, digest(raw), spent, 128, SegmentBytes, c.Roots, now, now.Add(c.Budget.OperationTimeout)}
			if _, e = put(root, filename(id, "claim", 0), q, ClaimBytes); e != nil {
				t.Fatal(e)
			}
			before, e := Inspect(root, id, now)
			if e != nil {
				t.Fatal(e)
			}
			c.JobDeadline = nil
			for i := 0; i < 4; i++ {
				if _, e = Start(context.Background(), root, c, Hooks{}); e != nil {
					t.Fatal(e)
				}
			}
			after, e := Inspect(root, id, now)
			if e != nil || !reflect.DeepEqual(before, after) || after.Spent != spent || !after.Expired || after.NamespaceVerification != "LEGACY_LEGAL_ORDINALS_ONLY" || after.DenominatorComplete {
				t.Fatalf("legacy changed: %+v %v", after, e)
			}
			actual, _ := os.ReadFile(filepath.Join(store, prefix(id)+"head.json"))
			if !bytes.Equal(raw, actual) {
				t.Fatal("legacy rewritten")
			}
			if _, e = put(root, filename(id, "segment", 1), segment{}, SegmentBytes); e != nil {
				t.Fatal(e)
			}
			if _, e = Inspect(root, id, now); e == nil {
				t.Fatal("legacy bounded orphan accepted")
			}
		})
	}
	for _, kind := range []string{"new", "unknown-schema", "unknown-model", "missing-model", "old-schema-new-layout"} {
		t.Run(kind, func(t *testing.T) {
			c, root, store := fixture(t)
			r, e := Start(context.Background(), root, c, Hooks{})
			if e != nil {
				t.Fatal(e)
			}
			var h Head
			raw, e := get(root, prefix(r.JobID)+"head.json", &h, HeadBytes)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "new" {
				var old originalHead
				if h.Schema != Schema || unseal(raw, &old) == nil {
					t.Fatal("new closed format not distinguished")
				}
				return
			}
			switch kind {
			case "unknown-schema":
				h.Schema = "lsp-trace.scope-job.v99"
			case "unknown-model":
				h.WorkModel = "future"
			case "missing-model":
				h.WorkModel = ""
			case "old-schema-new-layout":
				h.Schema = LegacySchema
			}
			raw, _ = seal(h)
			if e = os.WriteFile(filepath.Join(store, prefix(r.JobID)+"head.json"), raw, 0600); e != nil {
				t.Fatal(e)
			}
			if _, e = Inspect(root, r.JobID, time.Now()); !errors.Is(e, ErrUnsupportedVersion) {
				t.Fatalf("unsupported tuple not identified: %v", e)
			}
		})
	}
}
