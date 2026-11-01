//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type thirdReplyCrashMailbox struct{ CohortMailbox }

func (b thirdReplyCrashMailbox) ExchangeCohort(ctx context.Context, raw []byte) ([]byte, error) {
	reply, e := b.CohortMailbox.ExchangeCohort(ctx, raw)
	frame, parseErr := parseCohortFrame(raw)
	if e == nil && parseErr == nil && frame.Sequence == 3 {
		panic("valid third mailbox reply durable before host copy")
	}
	return reply, e
}
func TestReviewRepairMailboxCrashRetainsEarlierTen(t *testing.T) {
	c, items := cohortInputs(t)
	m := repairMailbox(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var mu sync.Mutex
	f := &cohortFixture{}
	done := make(chan error, 2)
	for _, role := range []string{"interpreter", "reviewer"} {
		go func(role string) {
			var after uint64
			for {
				w, e := WaitCohort(ctx, m.Directory, role, after)
				if e != nil {
					done <- e
					return
				}
				task, e := ReadCohortBundle(*w.Bundle)
				if e != nil {
					done <- e
					return
				}
				mu.Lock()
				reply, e := f.ExchangeCohort(ctx, mustJSON(task))
				mu.Unlock()
				if e != nil {
					done <- e
					return
				}
				if e = SubmitCohort(m.Directory, role, w.Bundle.Sequence, reply); e != nil {
					done <- e
					return
				}
				after = w.Bundle.Sequence
			}
		}(role)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("controlled third-reply crash missing")
			}
		}()
		RunCohort(ctx, c, items, thirdReplyCrashMailbox{m})
	}()
	cancel()
	<-done
	<-done
	mu.Lock()
	want := f.response
	mu.Unlock()
	r, e := DeliverCohortFailure(c, 20, "VALID_THIRD_REPLY_CRASH")
	if e != nil || r.Calls != 3 || r.Reviewed != 10 || r.ResponseBytes != want || !r.Inflight {
		t.Fatalf("ASSERT_REPAIR_MAILBOX_PRIOR_TEN_RETAINED: calls=%d reviewed=%d response=%d want=%d err=%v", r.Calls, r.Reviewed, r.ResponseBytes, want, e)
	}
	if _, e = LatestCohort(c); e != nil {
		t.Fatal("ASSERT_REPAIR_MAILBOX_PRIOR_TEN_QUALIFIED", e)
	}
	for n, o := range r.Outcomes {
		if n >= 10 && len(o.History) != 0 {
			t.Fatal("ASSERT_REPAIR_AMBIGUOUS_VALID_REPLY_NOT_ADOPTED")
		}
	}
	// A changed ready identity cannot redirect a pinned observation, even when
	// the response itself is byte-identical and all JSON is otherwise canonical.
	readyPath := filepath.Join(m.Directory, "ready-interpreter.json")
	saved, e := os.ReadFile(readyPath)
	if e != nil {
		t.Fatal(e)
	}
	var ready Ready
	if strict(saved, &ready) != nil {
		t.Fatal("fixture ready")
	}
	ready.LaunchID = "foreign-ready"
	if e = os.WriteFile(readyPath, mustJSON(ready), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = LatestCohort(c); e == nil {
		t.Fatal("ASSERT_REPAIR_MAILBOX_IDENTITY_SUBSTITUTION_REFUSED")
	}
	if e = os.WriteFile(readyPath, saved, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = LatestCohort(c); e != nil {
		t.Fatal(e)
	}
}

type recordingCohortDirectory struct{ requested []int }

func (d *recordingCohortDirectory) Readdirnames(n int) ([]string, error) {
	d.requested = append(d.requested, n)
	return make([]string, n), nil
}
func TestReviewRepairMailboxAcquisitionBound(t *testing.T) {
	d := &recordingCohortDirectory{}
	if _, e := boundedCohortMailboxNames(context.Background(), d); e == nil || len(d.requested) != 1 || d.requested[0] != cohortMailboxEntryCap+1 {
		t.Fatalf("ASSERT_REPAIR_MAILBOX_ACQUISITION_BOUND: reads=%v err=%v", d.requested, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d = &recordingCohortDirectory{}
	if _, e := boundedCohortMailboxNames(ctx, d); e != context.Canceled || len(d.requested) != 0 {
		t.Fatal("ASSERT_REPAIR_CANCEL_BEFORE_DIRECTORY_ACQUISITION", d.requested, e)
	}
}

// Embedding preserves only the concrete mailbox capability; no arbitrary path
// supplied by a generic CohortBackend may become recovery evidence.
type crashCohortMailbox struct {
	CohortMailbox
	t       *testing.T
	payload []byte
}

func (b crashCohortMailbox) ExchangeCohort(ctx context.Context, raw []byte) ([]byte, error) {
	done := make(chan error, 1)
	go func() { _, e := b.CohortMailbox.ExchangeCohort(ctx, raw); done <- e }()
	wait, e := WaitCohort(ctx, b.Directory, "interpreter", 0)
	if e != nil {
		b.t.Fatal(e)
	}
	e = SubmitCohort(b.Directory, "interpreter", wait.Bundle.Sequence, b.payload)
	if e == nil {
		b.t.Fatal("invalid fixture reply unexpectedly accepted")
	}
	if e = <-done; e != nil {
		b.t.Fatal(e)
	}
	panic("controlled coordinator loss after SubmitCohort durable reply, before host copy")
}

type deadlineCohortMailbox struct{ crashCohortMailbox }

func (b deadlineCohortMailbox) ExchangeCohort(ctx context.Context, raw []byte) (reply []byte, err error) {
	defer func() {
		if recover() != nil {
			reply = nil
			err = context.DeadlineExceeded
		}
	}()
	return b.crashCohortMailbox.ExchangeCohort(ctx, raw)
}
func TestReviewRepairDeadlinePublishedReply(t *testing.T) {
	c, items := cohortInputs(t)
	m := repairMailbox(t)
	payload := []byte(`{"Protocol":"WRONG"}`)
	r, e := RunCohort(context.Background(), c, items, deadlineCohortMailbox{crashCohortMailbox{m, t, payload}})
	if e == nil || r.Status != "CANCELLED" || r.ResponseBytes != len(payload) || r.Calls != 1 || !r.Inflight || r.Reviewed != 0 {
		t.Fatalf("ASSERT_REPAIR_DEADLINE_KNOWN_REPLY: response=%d status=%s err=%v", r.ResponseBytes, r.Status, e)
	}
	if _, e = LatestCohort(c); e != nil {
		t.Fatal("ASSERT_REPAIR_DEADLINE_QUALIFIED", e)
	}
}

func repairMailbox(t *testing.T) CohortMailbox {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	m := CohortMailbox{Directory: filepath.Join(root, "mailbox"), ExchangeTimeout: time.Second}
	if e = PrepareMailbox(m.Directory); e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"interpreter", "reviewer"} {
		if e = MarkCohortReady(m.Directory, Ready{CohortProtocol, role, "repair-" + role, hash([]byte(CohortRolePrompt(role)))}); e != nil {
			t.Fatal(e)
		}
	}
	return m
}
func TestReviewRepairProductionMailboxOnlyCrash(t *testing.T) {
	for _, payload := range []string{`{"Protocol":"WRONG"}`, `{"Protocol":"PRIVATE_V3_REVIEWED_COHORT_1","RequestDigest":"wrong","Items":[]}`} {
		t.Run(fmt.Sprint(len(payload)), func(t *testing.T) {
			c, items := cohortInputs(t)
			m := repairMailbox(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			func() {
				defer func() {
					if recover() == nil {
						t.Error("controlled crash missing")
					}
				}()
				RunCohort(ctx, c, items, crashCohortMailbox{m, t, []byte(payload)})
			}()
			if _, e := os.Stat(filepath.Join(c.Store, "host/response-00000001.json")); !os.IsNotExist(e) {
				t.Fatal("fixture already copied host reply", e)
			}
			r, e := DeliverCohortFailure(c, 20, "MAILBOX_ONLY_CRASH")
			if e != nil || r.ResponseBytes != len(payload) || r.Calls != 1 || r.Reviewed != 0 || !r.Inflight {
				t.Fatalf("ASSERT_REPAIR_MAILBOX_ONLY_SPENT: response=%d want=%d calls=%d reviewed=%d err=%v", r.ResponseBytes, len(payload), r.Calls, r.Reviewed, e)
			}
			if _, e = LatestCohort(c); e != nil {
				t.Fatal("ASSERT_REPAIR_MAILBOX_RECOVERY_QUALIFIED", e)
			}
			// The coordinator copy is the SAME physical event, not another reply.
			if e = os.WriteFile(filepath.Join(c.Store, "host/response-00000001.json"), []byte(payload), 0600); e != nil {
				t.Fatal(e)
			}
			if _, e = LatestCohort(c); e != nil {
				t.Fatal("ASSERT_REPAIR_MATCHED_COPY_ADMISSION", e)
			}
			if e = os.WriteFile(filepath.Join(m.Directory, "reply-00000001.json"), []byte(payload+" "), 0600); e != nil {
				t.Fatal(e)
			}
			if _, e = LatestCohort(c); e == nil {
				t.Fatal("ASSERT_REPAIR_CONFLICTING_COPIES_REFUSED")
			}
			if _, e = DeliverCohortFailure(c, 20, "CONFLICT"); e == nil {
				t.Fatal("ASSERT_REPAIR_CONFLICT_NO_CHEAP_QUALIFICATION")
			}
			if e = os.WriteFile(filepath.Join(m.Directory, "reply-00000001.json"), []byte(payload), 0600); e != nil {
				t.Fatal(e)
			}
			again, e := DeliverCohortFailure(c, 20, "AGAIN")
			if e != nil || again.ResponseBytes != len(payload) {
				t.Fatal("ASSERT_REPAIR_REPLY_DEDUP", again.ResponseBytes, e)
			}
		})
	}
}
func TestReviewRepairMailboxClosedNamespace(t *testing.T) {
	m := repairMailbox(t)
	for n := 0; n < 2000; n++ {
		if e := os.WriteFile(filepath.Join(m.Directory, fmt.Sprintf("foreign-%06d", n)), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	w, e := WaitCohort(ctx, m.Directory, "reviewer", 0)
	if e == nil || w.Event != "PUBLICATION_ERROR" {
		t.Fatalf("ASSERT_REPAIR_BOUNDED_MAILBOX_REFUSAL: event=%s err=%v", w.Event, e)
	}
}
func TestReviewRepairNavigationConservativeBoundary(t *testing.T) {
	for _, kind := range []string{"exact", "envelope", "overflow"} {
		t.Run(kind, func(t *testing.T) {
			c, items := cohortInputs(t)
			r, e := RunCohort(context.Background(), c, items, &cohortFixture{})
			if e != nil {
				t.Fatal(e)
			}
			c.Navigation = true
			c.Store += "-boundary"
			p := cohortProgress{Config: c, Outcomes: r.Outcomes, Calls: r.Calls, RequestBytes: r.RequestBytes, ResponseBytes: r.ResponseBytes}
			candidates := navigationCandidates(p.Outcomes)
			proposal := bindNavigation(c, candidates, []CohortNavigation{}, "interpreter", p.Calls+1)
			reviewer := bindNavigation(c, candidates, []CohortNavigation{}, "reviewer", p.Calls+2)
			// At the configured 64KiB response ceiling the sixfold escaping bound is
			// smaller than the seven-field/ten-group maximum. Equality must be admitted.
			c.MaxTotalBytes = p.RequestBytes + p.ResponseBytes + len(mustJSON(proposal)) + len(mustJSON(reviewer)) + 6*c.MaxResponseBytes + 2*c.MaxResponseBytes
			c.MaxOutputBytes = p.ResponseBytes + 2*c.MaxResponseBytes
			if kind == "envelope" {
				c.MaxRequestBytes = len(mustJSON(proposal)) + 1
			}
			if kind == "overflow" {
				c.MaxResponseBytes = int(^uint(0) >> 1)
			}
			s, e := openCohortStore(c)
			if e != nil {
				t.Fatal(e)
			}
			defer s.close()
			calls := 0
			e = runCohortNavigation(context.Background(), c, &p, s, cohortBackendFunc(func(ctx context.Context, raw []byte) ([]byte, error) { calls++; return navigationFixture(raw) }))
			if kind == "exact" {
				if e != nil || calls != 2 || p.NavigationStatus != "REVIEWED_HYPOTHESES_NOT_FEATURE_IDENTITY" {
					t.Fatal("ASSERT_REPAIR_NAV_EXACT_WORST_BOUND", calls, p.NavigationStatus, e)
				}
			} else if e != nil || calls != 0 || !strings.HasPrefix(p.NavigationStatus, "UNAVAILABLE") {
				t.Fatal("ASSERT_REPAIR_NAV_ENVELOPE_OVERFLOW_REFUSAL", kind, calls, p.NavigationStatus, e)
			}
		})
	}
}

func TestReviewRepairNavigationWholePassAdmission(t *testing.T) {
	for _, kind := range []string{"wire", "output"} {
		t.Run(kind, func(t *testing.T) {
			c, items := cohortInputs(t)
			r, e := RunCohort(context.Background(), c, items, &cohortFixture{})
			if e != nil {
				t.Fatal(e)
			}
			c.Navigation = true
			c.Store += "-navigation"
			p := cohortProgress{Config: c, Outcomes: r.Outcomes, Calls: r.Calls, RequestBytes: r.RequestBytes, ResponseBytes: r.ResponseBytes}
			task := bindNavigation(c, navigationCandidates(p.Outcomes), []CohortNavigation{}, "interpreter", p.Calls+1)
			if kind == "wire" {
				c.MaxTotalBytes = p.RequestBytes + p.ResponseBytes + len(mustJSON(task)) + c.MaxResponseBytes
			} else {
				c.MaxOutputBytes = p.ResponseBytes + c.MaxResponseBytes
			}
			s, e := openCohortStore(c)
			if e != nil {
				t.Fatal(e)
			}
			defer s.close()
			before := p
			calls := 0
			e = runCohortNavigation(context.Background(), c, &p, s, cohortBackendFunc(func(ctx context.Context, raw []byte) ([]byte, error) { calls++; return navigationFixture(raw) }))
			if e != nil || calls != 0 || p.Calls != before.Calls || p.RequestBytes != before.RequestBytes || p.ResponseBytes != before.ResponseBytes || !strings.HasPrefix(p.NavigationStatus, "UNAVAILABLE") {
				t.Fatalf("ASSERT_REPAIR_NAV_NO_PARTIAL_SPEND: kind=%s calls=%d status=%s err=%v", kind, calls, p.NavigationStatus, e)
			}
			if _, e := os.Stat(filepath.Join(c.Store, fmt.Sprintf("claim/claim-%08d.json", before.Calls+1))); !os.IsNotExist(e) {
				t.Fatal("ASSERT_REPAIR_NAV_NO_NEW_CLAIM", e)
			}
		})
	}
}
