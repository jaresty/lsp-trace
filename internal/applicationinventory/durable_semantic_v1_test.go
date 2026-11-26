//go:build (darwin || linux) && lsp_trace_controlled_describe_fixture

package applicationinventory

import (
	"context"
	"encoding/json"
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func checkSemanticTask(t *testing.T, task DurableTask, window time.Duration) {
	t.Helper()
	if task.Schema != "lsp-trace.durable-inventory-task.v3" || task.Contract.Version != "lsp-trace.census-task-contract.v3" || task.SemanticPolicy == nil || !task.SemanticPolicy.Valid() || task.SemanticPolicy.TaskWindowNS != int64(window) || task.DeadlineUnixNano-task.TaskIssuedNS != int64(window) || task.RunDeadlineUnixNano != 0 || task.SemanticReservedNS != int64(task.Sequence)*int64(window) || strings.Contains(task.Contract.Rules, "120-second") || task.Contract.Operations != nil {
		t.Fatalf("SEMANTIC_POLICY_TASK: %+v", task)
	}
}
func TestInventorySemanticPolicy(t *testing.T) {
	p, e := cat.NewInventorySemanticPolicyV1(10 * time.Minute)
	if e != nil || !p.Valid() || p.CumulativeWindowNS != int64(40*time.Minute) {
		t.Fatal(p, e)
	}
	for _, w := range []time.Duration{0, -1, time.Duration(math.MaxInt64), cat.InventorySemanticMaxTaskWindowV1 + 1} {
		if _, e := cat.NewInventorySemanticPolicyV1(w); e == nil {
			t.Fatal("INVALID_WINDOW", w)
		}
	}
	for _, change := range []func(*cat.InventorySemanticPolicyV1){func(p *cat.InventorySemanticPolicyV1) { p.Schema = "" }, func(p *cat.InventorySemanticPolicyV1) { p.Stages = 5 }, func(p *cat.InventorySemanticPolicyV1) { p.CumulativeWindowNS++ }, func(p *cat.InventorySemanticPolicyV1) { p.TaskWindowNS = math.MaxInt64 }} {
		bad := p
		change(&bad)
		if bad.Valid() {
			t.Fatal("ALTERED_POLICY_ACCEPTED", bad)
		}
	}
	if json.Unmarshal([]byte(`{"TaskWindowNS":9223372036854775808}`), &cat.InventorySemanticPolicyV1{}) == nil {
		t.Fatal("JSON_OVERFLOW")
	}
	max, e := cat.NewInventorySemanticPolicyV1(cat.InventorySemanticMaxTaskWindowV1)
	if e != nil || !max.Valid() {
		t.Fatal("VALIDATION_BOUND", e)
	}
	now := time.Unix(1700000000, 0)
	h := durableInventoryHead{Schema: is.InventorySemanticHeadVersionV1, SemanticPolicy: &p, Window: p.TaskWindowNS, Phase: "GroupDecision", Issued: now.Add(-365 * 24 * time.Hour).UnixNano(), AcceptanceDeadline: 1}
	v := durableInventoryState{Interpreter: "i", Reviewer: "r"}
	s := DurableStore{}
	for i := 1; i <= 4; i++ {
		if e := s.issueNextAt(&h, v, now); e != nil || h.SemanticNS != int64(i)*p.TaskWindowNS || !h.semanticTimingValid() {
			t.Fatal("FINITE_ISSUANCE", i, h, e)
		}
		now = now.Add(48 * time.Hour)
	}
	if h.RunDeadline != 0 || !validInventoryDeadlines(h) {
		t.Fatal("OVERALL_EXPIRY")
	}
	exhausted := h
	if e := s.issueNextAt(&exhausted, v, now); e == nil {
		t.Fatal("CUMULATIVE_EXHAUSTION_BYPASSED")
	}
	h.Phase = "Published"
	if e := s.issueNextAt(&h, v, now); e != nil || h.SemanticNS != p.CumulativeWindowNS || !h.semanticTimingValid() {
		t.Fatal("PUBLICATION_NOT_A_FIFTH_LEASE", e)
	}
	for _, change := range []func(*durableInventoryHead){func(h *durableInventoryHead) { h.SemanticNS-- }, func(h *durableInventoryHead) { h.Window++ }, func(h *durableInventoryHead) { h.Deadline++ }, func(h *durableInventoryHead) { h.RunDeadline = 1 }, func(h *durableInventoryHead) { h.SemanticPolicy = nil }} {
		bad := h
		change(&bad)
		if bad.semanticTimingValid() {
			t.Fatal("ALTERED_TIMING_ACCEPTED")
		}
	}
	overflow := durableInventoryHead{Schema: is.InventorySemanticHeadVersionV1, SemanticPolicy: &p, Window: p.TaskWindowNS, Phase: "GroupDecision"}
	if e := s.issueNextAt(&overflow, v, time.Unix(0, math.MaxInt64-1)); e == nil {
		t.Fatal("DEADLINE_OVERFLOW")
	}
	old := durableInventoryHead{Schema: is.InventoryReferenceHeadVersionV1, Window: int64(30 * time.Second), Phase: "GroupDecision"}
	for i := 0; i < 4; i++ {
		if s.issueNextAt(&old, v, now) != nil {
			t.Fatal("HISTORICAL_ISSUANCE")
		}
	}
	if old.SemanticNS != int64(120*time.Second) || s.issueNextAt(&old, v, now) == nil {
		t.Fatal("HISTORICAL_CEILING")
	}
	old.SemanticPolicy = &p
	if old.semanticTimingValid() {
		t.Fatal("OLD_POLICY_INJECTION")
	}
	raw, _ := json.Marshal(old)
	var parsed durableInventoryHead
	if decodeVersionedInventoryHead(raw, &parsed) == nil {
		t.Fatal("OLD_SCHEMA_POLICY_INJECTION")
	}
	for _, schema := range []string{is.InventoryReferenceHeadVersionV1, is.InventoryConnectedHeadVersionV1, is.InventoryResumeHeadVersionV1} {
		bad := h
		bad.Schema = schema
		raw, _ := json.Marshal(bad)
		if decodeVersionedInventoryHead(raw, &parsed) == nil {
			t.Fatal("OLD_SCHEMA_INHERITED_DEFAULT", schema)
		}
	}
	b, e := cat.NewInventorySemanticBudgetV1(time.Now().Add(time.Second), 1024, 2048, time.Duration(p.CumulativeWindowNS), p)
	if e != nil {
		t.Fatal(e)
	}
	used, sem := b.Usage()
	time.Sleep(time.Millisecond)
	u, ss := b.Usage()
	if u != used || ss != sem || u != 2048 {
		t.Fatal("WAIT_CHARGED_IO")
	}
	if b.Reserve(context.Background(), 1) != nil {
		t.Fatal("RESERVE")
	}
	if _, _, e := b.Exchange(context.Background(), time.Nanosecond); e == nil {
		t.Fatal("SEMANTIC_BUDGET_NATIVE_CALLBACK")
	}
	if _, e := cat.NewInventorySemanticBudgetV1(time.Now().Add(time.Second), 1, 0, time.Duration(p.CumulativeWindowNS+1), p); e == nil {
		t.Fatal("SEMANTIC_COUNTER_OVERFLOW")
	}
	if _, e := cat.NewInventoryReferenceBudgetV1(time.Now().Add(time.Second), 1024, 0, 121*time.Second); e == nil {
		t.Fatal("HISTORICAL_BUDGET_ENLARGED")
	}
	for _, tc := range []struct {
		deadline         time.Time
		allowance, prior uint64
	}{
		{time.Now().Add(-time.Second), 1, 0}, {time.Now().Add(31 * time.Second), 1, 0}, {time.Now().Add(time.Second), 0, 0}, {time.Now().Add(time.Second), 1, math.MaxUint64}, {time.Now().Add(time.Second), 2, cat.InventoryReferenceLimitV1 - 1},
	} {
		if _, err := cat.NewInventorySemanticBudgetV1(tc.deadline, tc.allowance, tc.prior, time.Duration(p.TaskWindowNS), p); err == nil {
			t.Fatal("INVALID_ACTIVE_BUDGET")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cb, err := cat.NewInventorySemanticBudgetV1(time.Now().Add(time.Second), 1, 0, time.Duration(p.TaskWindowNS), p)
	if err != nil || cb.Reserve(cancelled, 1) == nil || !cb.Exhausted() {
		t.Fatal("ACTIVE_CANCELLATION")
	}
	live := durableInventoryHead{Schema: is.InventorySemanticHeadVersionV1, Deadline: time.Now().Add(10 * time.Minute).UnixNano()}
	if active := live.activeDeadline(); active >= live.Deadline || time.Until(time.Unix(0, active)) > 30*time.Second {
		t.Fatal("ACTIVE_WINDOW_ENLARGED")
	}
	t.Logf("POLICY task=%d cumulative=%d max=%s H=%d work=%d", p.TaskWindowNS, p.CumulativeWindowNS, cat.InventorySemanticMaxTaskWindowV1, cat.InventoryReferenceLimitV1, cat.InventoryReferenceLimitV1*32)
}
func semanticAcquired(t *testing.T, short bool) (DurableStore, InventoryJobStatus) {
	t.Helper()
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(base, "store")
	for _, p := range []string{dir, filepath.Join(base, "source")} {
		if e = os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	prefix := "semantic-"
	if short {
		prefix += "short-"
	}
	first, _ := runResumeProcess(t, dir, "", prefix+"references-first")
	second, _ := runResumeProcess(t, dir, first.Status.RunID, prefix+"references-success")
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Close() })
	s := DurableStore{Root: root, Directory: dir}
	h, _, e := s.head(second.Status.RunID)
	if e != nil {
		t.Fatal(e)
	}
	c, e := s.ownedConfig(h)
	if e != nil || !semanticConfigValid(c) || c.SemanticPolicy == nil || c.TaskWindow != time.Duration(c.SemanticPolicy.TaskWindowNS) || second.Status.JobExpiry != "NONE" {
		t.Fatal("FROZEN_POLICY_CONFIG", e)
	}
	for _, change := range []func(*OwnedInventoryConfig){func(c *OwnedInventoryConfig) { c.SemanticPolicy = nil }, func(c *OwnedInventoryConfig) { c.TaskWindow++ }, func(c *OwnedInventoryConfig) { c.Schema = is.InventoryReferenceConfigVersionV1 }} {
		bad := c
		change(&bad)
		if ownedConfigValid(bad) {
			t.Fatal("ALTERED_CONFIG_ACCEPTED")
		}
	}
	return s, second.Status
}
func TestInventorySemanticRefusal(t *testing.T) {
	for _, mode := range []string{"expired", "bad-answer"} {
		t.Run(mode, func(t *testing.T) {
			s, st := semanticAcquired(t, mode == "expired")
			ctx := context.Background()
			task, e := s.Seal(ctx, st.RunID, st.Head)
			if e != nil {
				t.Fatal("SEAL", e)
			}
			window := 10 * time.Minute
			if mode == "expired" {
				window = time.Second
			}
			checkSemanticTask(t, task, window)
			before, raw, e := s.head(st.RunID)
			if e != nil {
				t.Fatal(e)
			}
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()
			if _, e = s.Submit(cancelCtx, task.RunID, task.TaskID, task.Role, []byte("{}")); e == nil {
				t.Fatal("CANCELLED_SUBMISSION")
			}
			for _, args := range [][3]string{{task.RunID, task.TaskID, "wrong-role"}, {task.RunID, strings.Repeat("0", 64), task.Role}, {strings.Repeat("1", 64), task.TaskID, task.Role}} {
				if _, e = s.Submit(ctx, args[0], args[1], args[2], []byte("{}")); e == nil {
					t.Fatal("WRONG_BINDING")
				}
			}
			after, raw2, e := s.head(st.RunID)
			if e != nil || string(raw) != string(raw2) || !reflect.DeepEqual(before, after) {
				t.Fatal("REFUSAL_RESET_OR_CHARGE", e)
			}
			if mode == "expired" {
				time.Sleep(time.Until(time.Unix(0, task.DeadlineUnixNano)) + time.Millisecond)
				current, e := s.Task(ctx, task.RunID, task.TaskID)
				if e != nil || current.Status != "EXPIRED" || current.DeadlineUnixNano != task.DeadlineUnixNano || current.Used != task.Used {
					t.Fatal("EXPIRED_RENEWED_OR_CHARGED", current, e)
				}
				if _, e = s.Read(ctx, task.RunID, task.TaskID, "input", 0, 32768); e == nil {
					t.Fatal("EXPIRED_READ")
				}
				if _, e = s.Submit(ctx, task.RunID, task.TaskID, task.Role, []byte("{}")); e == nil {
					t.Fatal("LATE_RESPONSE")
				}
				_, raw2, e = s.head(st.RunID)
				if e != nil || string(raw) != string(raw2) {
					t.Fatal("EXPIRY_MUTATED_HEAD", e)
				}
			} else {
				if _, e = s.Submit(ctx, task.RunID, task.TaskID, task.Role, []byte("{}")); e == nil {
					t.Fatal("BAD_ANSWER")
				}
				spent, e := s.Task(ctx, task.RunID, task.TaskID)
				if e != nil || spent.Status != "UNCERTAIN_SPENT" || spent.Used <= task.Used || spent.SemanticReservedNS != task.SemanticReservedNS {
					t.Fatal("FAILED_WORK_NOT_SPENT", spent, e)
				}
				if _, e = s.Submit(ctx, task.RunID, task.TaskID, task.Role, []byte("{}")); e == nil {
					t.Fatal("DUPLICATE_RESPONSE")
				}
			}
			if _, e = s.Seal(ctx, st.RunID, st.Head); e == nil {
				t.Fatal("RESEAL_RENEWAL")
			}
			files, _ := json.Marshal(retentionPreflightFiles(t, s.Directory))
			t.Log("SEMANTIC_REFUSAL_FILES " + string(files))
		})
	}
}
