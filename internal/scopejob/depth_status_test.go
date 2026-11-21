package scopejob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestScopeJobDepthReplayBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		depth int
		roots []string
		parts []segment
		valid bool
	}{
		{"original-counterexample", 1, []string{"."}, []segment{{Records: []Record{directory("."), directory("a")}, Done: true}}, false},
		{"exact-boundary", 2, []string{"."}, []segment{{Records: []Record{directory("."), directory("a")}, Done: true}}, true},
		{"cross-segment-overflow", 1, []string{"."}, []segment{{Records: []Record{directory(".")}, Frontier: []string{"."}}, {Records: []Record{directory("a")}, Done: true}}, false},
		{"cross-segment-exact", 2, []string{"."}, []segment{{Records: []Record{directory(".")}, Frontier: []string{"."}}, {Records: []Record{directory("a")}, Done: true}}, true},
		{"partial-frontier", 1, []string{"."}, []segment{{Records: []Record{directory(".")}, Frontier: []string{"."}, Reason: "TRAVERSAL_UNAVAILABLE"}}, true},
		{"sibling-roots-reset", 1, []string{"a", "b", "c"}, []segment{{Records: []Record{directory("a"), directory("b"), directory("c")}, Done: true}}, true},
		{"sibling-segments-reset", 1, []string{"a", "b"}, []segment{{Records: []Record{directory("a")}, Frontier: []string{"b"}}, {Records: []Record{directory("b")}, Done: true}}, true},
		{"unavailable-root", 1, []string{"a", "b"}, []segment{{Records: []Record{{Path: "a", Kind: "OTHER", Role: "UNAVAILABLE", UnknownDescendants: true}, directory("b")}, Done: true}}, true},
		{"pruned-root", 1, []string{"a", "b"}, []segment{{Records: []Record{{Path: "a", Kind: "DIRECTORY", Role: "EXCLUDED", MetadataObserved: true, UnknownDescendants: true, Mode: uint32(os.ModeDir)}, directory("b")}, Done: true}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, root, _ := fixture(t)
			c.Budget.Depth = tc.depth
			c.Roots = tc.roots
			if tc.name == "pruned-root" {
				c.Prunes = []string{"a"}
			}
			r, e := Start(context.Background(), root, c, Hooks{AfterClaim: func(uint64) error { return errors.New("stop") }})
			if e == nil {
				t.Fatal("fixture")
			}
			var h Head
			if _, e = get(root, prefix(r.JobID)+"head.json", &h, HeadBytes); e != nil {
				t.Fatal(e)
			}
			var q claim
			qr, e := get(root, filename(r.JobID, "claim", 0), &q, ClaimBytes)
			if e != nil {
				t.Fatal(e)
			}
			counts := Counts{}
			var sr []byte
			for i, s := range tc.parts {
				if i > 0 {
					spent, ok := quote(h, q.Spent, q.Steps)
					if !ok {
						t.Fatal("quote")
					}
					q.Sequence = uint64(i)
					q.Previous = digest(sr)
					q.Spent = spent
					q.Frontier = tc.parts[i-1].Frontier
					qr, e = put(root, filename(r.JobID, "claim", uint64(i)), q, ClaimBytes)
					if e != nil {
						t.Fatal(e)
					}
				}
				s.Claim = digest(qr)
				sr, e = put(root, filename(r.JobID, "segment", uint64(i)), s, SegmentBytes)
				if e != nil {
					t.Fatal(e)
				}
				for _, record := range s.Records {
					counts.add(record)
				}
			}
			last := tc.parts[len(tc.parts)-1]
			state, reason := "SCOPE_READY", "ACQUISITION_NOT_IMPLEMENTED"
			if !last.Done {
				state, reason = "PARTIAL", last.Reason
			}
			if _, e = put(root, prefix(r.JobID)+"stop.json", stop{Previous: digest(sr), State: state, Reason: reason, Counts: counts, Frontier: last.Frontier}, StopBytes); e != nil {
				t.Fatal(e)
			}
			got, e := Inspect(root, r.JobID, time.Now())
			if (e == nil) != tc.valid {
				t.Fatalf("frame admission valid=%v want=%v report=%+v error=%v", e == nil, tc.valid, got, e)
			}
			if !tc.valid && !errors.Is(e, ErrIntegrity) {
				t.Fatalf("not integrity rejection: %v", e)
			}
		})
	}
}

func TestScopeJobDepthLiveControls(t *testing.T) {
	for _, depth := range []int{1, 2} {
		t.Run(string(rune('0'+depth)), func(t *testing.T) {
			c, root, _ := fixture(t)
			c.Budget.Depth = depth
			if e := os.Mkdir(filepath.Join(c.Workspace, "a"), 0700); e != nil {
				t.Fatal(e)
			}
			got, e := Start(context.Background(), root, c, Hooks{})
			if e != nil {
				t.Fatal(e)
			}
			if depth == 1 {
				if got.State != "PARTIAL" || got.Reason != "TRAVERSAL_UNAVAILABLE" || got.Counts.Directories != 1 || got.DenominatorComplete {
					t.Fatal(got)
				}
			} else if got.State != "SCOPE_READY" || got.Counts.Directories != 2 || !got.DenominatorComplete {
				t.Fatal(got)
			}
		})
	}
}

// No sleeps or millions of actual syscalls: the private probe sees a clock whose
// deadline becomes reached after exactly three controlled absent-name results.
type probeDeadlineContext struct {
	context.Context
	expired bool
}

func (c *probeDeadlineContext) Err() error {
	if c.expired {
		return context.DeadlineExceeded
	}
	return nil
}
func TestScopeJobStatusBoundedLegacyProbes(t *testing.T) {
	for _, kind := range []string{"deadline", "cancel", "success", "late-evidence"} {
		t.Run(kind, func(t *testing.T) {
			clock := &probeDeadlineContext{Context: context.Background()}
			cancelled, cancel := context.WithCancel(context.Background())
			defer cancel()
			var ctx context.Context = clock
			if kind == "cancel" {
				ctx = cancelled
			}
			calls := 0
			limit := uint64(8)
			e := probeLegacyNames(ctx, "00000000000000000000000000000012", limit, 0, 0, func(string) error {
				calls++
				if calls == 3 {
					if kind == "deadline" {
						clock.expired = true
					}
					if kind == "cancel" {
						cancel()
					}
				}
				if kind == "late-evidence" && calls == 8 {
					return nil
				}
				return os.ErrNotExist
			})
			switch kind {
			case "deadline":
				if !errors.Is(e, context.DeadlineExceeded) || calls != 3 {
					t.Fatalf("deadline probes=%d error=%v", calls, e)
				}
			case "cancel":
				if !errors.Is(e, context.Canceled) || calls != 3 {
					t.Fatalf("cancel probes=%d error=%v", calls, e)
				}
			case "success":
				if e != nil || calls != 8 {
					t.Fatalf("complete probes=%d error=%v", calls, e)
				}
			case "late-evidence":
				if !errors.Is(e, ErrIntegrity) || calls != 8 {
					t.Fatalf("gap probes=%d error=%v", calls, e)
				}
			}
		})
	}
}

func TestScopeJobStatusDeadlineCap(t *testing.T) {
	start := time.Now()
	for _, timeout := range []time.Duration{time.Second, time.Hour, 24 * time.Hour} {
		ctx, cancel := inspectionContext(context.Background(), start, timeout)
		got, ok := ctx.Deadline()
		cancel()
		want := start.Add(min(timeout, DefaultBudget().OperationTimeout))
		if !ok || !got.Equal(want) {
			t.Fatalf("deadline=%v want=%v", got, want)
		}
	}
	parent, stop := context.WithDeadline(context.Background(), start.Add(time.Second))
	defer stop()
	ctx, cancel := inspectionContext(parent, start, time.Hour)
	defer cancel()
	got, _ := ctx.Deadline()
	if !got.Equal(start.Add(time.Second)) {
		t.Fatal("caller deadline extended")
	}
	ctx2, cancel2 := inspectionContext(context.Background(), start.Add(-time.Hour), time.Second)
	defer cancel2()
	if !errors.Is(ctx2.Err(), context.DeadlineExceeded) {
		t.Fatal("timer renewed")
	}
	if (Config{}).JobDeadline != nil {
		t.Fatal("default overall expiry")
	}
}

func TestScopeJobStatusReadOnlyVersions(t *testing.T) {
	for _, model := range []string{"original", "intermediate", "v2"} {
		t.Run(model, func(t *testing.T) {
			c, root, store := fixture(t)
			c.Budget.Packets = 4
			c.Budget.OperationTimeout = 10 * time.Second
			c, _ = Canonical(c)
			now := time.Now().UTC()
			var id string
			if model == "v2" {
				r, e := Start(context.Background(), root, c, Hooks{AfterClaim: func(uint64) error { return errors.New("stop") }})
				if e == nil {
					t.Fatal("fixture")
				}
				id = r.JobID
			} else {
				id = "00000000000000000000000000000012"
				raw, _ := json.Marshal(c)
				old := originalHead{LegacySchema, id, c, digest(raw), now, nil}
				var h any = old
				spent := baseSpent()
				spent.Steps = 128
				spent.Packets += 2
				spent.LogicalBytes += BatchBytes
				spent.Work = 128 * uint64(8+c.Budget.Depth)
				if model == "intermediate" {
					h = Head{Schema: LegacySchema, ID: id, Config: c, ConfigDigest: old.ConfigDigest, Started: now, WorkModel: "policy-v2"}
					spent, _ = quote(Head{Config: c}, baseSpent(), 128)
				}
				hr, e := put(root, prefix(id)+"head.json", h, HeadBytes)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = put(root, filename(id, "claim", 0), claim{0, digest(hr), spent, 128, SegmentBytes, c.Roots, now, now.Add(c.Budget.OperationTimeout)}, ClaimBytes); e != nil {
					t.Fatal(e)
				}
			}
			headPath, claimPath := filepath.Join(store, prefix(id)+"head.json"), filepath.Join(store, filename(id, "claim", 0))
			headBefore, _ := os.ReadFile(headPath)
			claimBefore, _ := os.ReadFile(claimPath)
			before, e := Inspect(root, id, now)
			if e != nil {
				t.Fatal(e)
			}
			for _, kind := range []string{"cancel", "deadline"} {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if kind == "deadline" {
					ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
					defer cancel()
				}
				got, e := InspectContext(ctx, root, id, now)
				want := context.Canceled
				if kind == "deadline" {
					want = context.DeadlineExceeded
				}
				if !errors.Is(e, want) || !reflect.DeepEqual(got, Report{}) {
					t.Fatalf("authoritative interrupted report: %+v %v", got, e)
				}
			}
			// The 30s entry cap is still live; the frozen 10s limit has elapsed
			// since entry. Reading the header must not restart that allowance.
			elapsed, elapsedErr := inspectContext(context.Background(), root, id, now, time.Now().Add(-15*time.Second))
			if !errors.Is(elapsedErr, context.DeadlineExceeded) || !reflect.DeepEqual(elapsed, Report{}) {
				t.Fatalf("frozen inspection deadline renewed: %+v %v", elapsed, elapsedErr)
			}
			after, e := InspectContext(context.Background(), root, id, now)
			if e != nil || !reflect.DeepEqual(before, after) || !after.PendingClaim || after.Spent.Steps != 128 {
				t.Fatalf("renewed/changed: %+v %v", after, e)
			}
			headAfter, _ := os.ReadFile(headPath)
			claimAfter, _ := os.ReadFile(claimPath)
			if !bytes.Equal(headBefore, headAfter) || !bytes.Equal(claimBefore, claimAfter) {
				t.Fatal("saved records rewritten")
			}
			if after.Head.Expires != nil {
				t.Fatal("inspection imposed job expiry")
			}
		})
	}
}
