//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type cohortBackendFunc func(context.Context, []byte) ([]byte, error)

func (f cohortBackendFunc) ExchangeCohort(c context.Context, b []byte) ([]byte, error) {
	return f(c, b)
}
func TestCohortStrictRecursiveAndCitations(t *testing.T) {
	c, items := cohortInputs(t)
	task := bindCohortTask(c, items[:1], "reviewer", "REVIEW", 1)
	f := &cohortFixture{}
	raw, e := f.ExchangeCohort(context.Background(), mustJSON(task))
	if e != nil {
		t.Fatal(e)
	}
	// Whitespace and field order are compatible; duplicate decoded/alias/unknown
	// fields and nested laundering are not, BEFORE canonicalization.
	valid := append([]byte(" \n"), raw...)
	if _, e = DecodeCohortResponse(valid); e != nil {
		t.Fatal("ASSERT_COHORT_WHITESPACE_COMPATIBLE", e)
	}
	for name, mut := range map[string][]byte{
		"duplicate": bytes.Replace(raw, []byte(`"Protocol":`), []byte(`"Protocol":"x","Protocol":`), 1),
		"escaped":   bytes.Replace(raw, []byte(`"ItemID":`), []byte(`"Item\u0049D":"x","ItemID":`), 1),
		"alias":     bytes.Replace(raw, []byte(`"Revision":`), []byte(`"revision":1,"Revision":`), 1),
		"unknown":   bytes.Replace(raw, []byte(`"Revision":`), []byte(`"Private":{},"Revision":`), 1),
		"trailing":  append(append([]byte{}, raw...), []byte(`{}`)...),
		"utf8":      append(append([]byte{}, raw...), 0xff),
		"mixed":     bytes.Replace(raw, []byte(CohortProtocol), []byte(Protocol), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := DecodeCohortResponse(mut); e == nil {
				t.Fatal("ASSERT_COHORT_STRICT_RECURSIVE_REFUSAL")
			}
		})
	}
	r, _ := DecodeCohortResponse(raw)
	for _, bounds := range [][2]int{{math.MaxInt, math.MinInt}, {math.MinInt, math.MaxInt}, {1, 0}, {0, math.MaxInt}, {-1, 0}} {
		changed := r
		changed.Items = append([]CohortDecision{}, r.Items...)
		changed.Items[0].Start = bounds[0]
		changed.Items[0].End = bounds[1]
		if validateCohortResponse(task, changed) == nil {
			t.Fatal("ASSERT_COHORT_ORDERED_OVERFLOW_CITATION_REFUSAL")
		}
	}
	unicodeItem := items[0]
	unicodeItem.Body = "πx"
	unicodeItem.OwnBodySHA256 = hash([]byte(unicodeItem.Body))
	unicodeTask := bindCohortTask(c, []CohortItem{unicodeItem}, "reviewer", "REVIEW", 1)
	unicodeRaw, _ := f.ExchangeCohort(context.Background(), mustJSON(unicodeTask))
	unicodeResponse, _ := DecodeCohortResponse(unicodeRaw)
	unicodeResponse.Items[0].Start = 1
	unicodeResponse.Items[0].End = 2
	unicodeResponse.Items[0].Quote = unicodeItem.Body[1:2]
	if validateCohortResponse(unicodeTask, unicodeResponse) == nil {
		t.Fatal("ASSERT_COHORT_UTF8_CITATION_BOUNDARY")
	}
	if _, e := decodeResponse(raw); e == nil {
		t.Fatal("ASSERT_COHORT_NOT_OLD_FLAT")
	}
}
func TestCohortInvalidLaterRetainsEarlierAndCharges(t *testing.T) {
	for _, mutation := range []string{"foreign", "omitted", "duplicated", "extra", "mixed", "private", "error"} {
		t.Run(mutation, func(t *testing.T) {
			c, items := cohortInputs(t)
			f := &cohortFixture{}
			backend := cohortBackendFunc(func(ctx context.Context, raw []byte) ([]byte, error) {
				reply, e := f.ExchangeCohort(ctx, raw)
				if f.calls != 4 {
					return reply, e
				}
				var r CohortResponse
				json.Unmarshal(reply, &r)
				switch mutation {
				case "foreign":
					r.Items[0].Quote = items[0].Body
				case "omitted":
					r.Items = r.Items[:len(r.Items)-1]
				case "duplicated":
					r.Items[1] = r.Items[0]
				case "extra":
					r.Items = append(r.Items, r.Items[0])
				case "mixed":
					r.Protocol = Protocol
				case "private":
					return bytes.Replace(reply, []byte(`"Revision":`), []byte(`"Private":"unknown","Revision":`), 1), nil
				case "error":
					return reply, errors.New("controlled backend error")
				}
				return mustJSON(r), nil
			})
			r, e := RunCohort(context.Background(), c, items, backend)
			if e == nil || r.Reviewed != 10 || r.Calls != 4 || r.Pending != 10 || r.ResponseBytes == 0 {
				t.Fatalf("ASSERT_COHORT_PARTIAL_CHARGED: %+v %v", r, e)
			}
			prior, e := LatestCohort(c)
			if e != nil || prior.Reviewed != 10 {
				t.Fatal("ASSERT_COHORT_OWN_HISTORY_ADMISSION", e)
			}
			before := f.calls
			_, e = RunCohort(context.Background(), c, items, backend)
			if e == nil || f.calls != before {
				t.Fatal("ASSERT_COHORT_NO_AUTOREPLAY")
			}
		})
	}
}
func TestCohortBudgetCorrectionAndOversized(t *testing.T) {
	c, items := cohortInputs(t)
	c.MaxCalls = 4
	r, e := RunCohort(context.Background(), c, items, &cohortFixture{reject: true})
	if !errors.Is(e, ErrPaused) || r.Calls > 4 || r.Reviewed < 9 || r.Requested != r.Reviewed+r.Gaps+r.Pending {
		t.Fatal("ASSERT_COHORT_CORRECTION_SAME_ALLOWANCE", r, e)
	}
	c, items = cohortInputs(t)
	items[0].Body = strings.Repeat("<", 1<<20)
	items[0].OwnBodySHA256 = hash([]byte(items[0].Body))
	r, e = RunCohort(context.Background(), c, items, &cohortFixture{})
	if e == nil || r.Calls != 0 {
		t.Fatal("ASSERT_COHORT_OVERSIZE_BEFORE_DISCLOSURE")
	}
}
func TestCohortDurableClaimCrashNoRefund(t *testing.T) {
	c, items := cohortInputs(t)
	called := 0
	func() {
		defer func() {
			if recover() == nil {
				t.Error("controlled crash absent")
			}
		}()
		RunCohort(context.Background(), c, items, cohortBackendFunc(func(context.Context, []byte) ([]byte, error) {
			called++
			panic("controlled crash AFTER claim and reservation")
		}))
	}()
	_, e := RunCohort(context.Background(), c, items, &cohortFixture{})
	if e == nil || called != 1 {
		t.Fatal("ASSERT_COHORT_CRASH_NO_REDISCLOSURE")
	}
	r, e := DeliverCohortFailure(c, 20, "CRASH")
	if e != nil || r.Calls != 1 || r.RequestBytes == 0 || r.ResponseBytes != 0 {
		t.Fatal("ASSERT_COHORT_CRASH_RESERVATION_RETAINED", r, e)
	}
}
func TestCohortQuartetAndPriorCitationOverflow(t *testing.T) {
	for _, correction := range []bool{false, true} {
		t.Run(map[bool]string{false: "base", true: "correction"}[correction], func(t *testing.T) {
			c, items := cohortInputs(t)
			r, e := RunCohort(context.Background(), c, items, &cohortFixture{reject: correction})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = LatestCohort(c); e != nil {
				t.Fatal("ASSERT_COHORT_PRIOR_REPLAY", e)
			}
			// Mutate ALL four views consistently: malformed history still must refuse.
			r.Outcomes[0].History[0].Start = math.MaxInt
			r.Outcomes[0].History[0].End = math.MinInt
			for _, p := range r.MachinePaths {
				if e = os.WriteFile(filepath.Join(c.Store, p), mustJSON(r), 0600); e != nil {
					t.Fatal(e)
				}
			}
			for _, p := range r.HumanPaths {
				os.WriteFile(filepath.Join(c.Store, p), cohortHuman(r), 0600)
			}
			if _, e = LatestCohort(c); e == nil {
				t.Fatal("ASSERT_COHORT_MALFORMED_PRIOR_REFUSED")
			}
		})
	}
}
func TestCohortPhysicalMailboxTerminalAndHostReturns(t *testing.T) {
	c, items := cohortInputs(t)
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	os.Chmod(dir, 0700)
	m := CohortMailbox{Directory: dir, ExchangeTimeout: time.Second}
	for _, role := range []string{"interpreter", "reviewer"} {
		if e = MarkCohortReady(dir, Ready{Protocol: CohortProtocol, Role: role, LaunchID: "scripted-" + role, PromptSHA256: hash([]byte(CohortRolePrompt(role)))}); e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	returns := make(chan CohortRoleReturn, 2)
	errs := make(chan error, 2)
	for _, role := range []string{"interpreter", "reviewer"} {
		go func(role string) {
			var after uint64
			f := &cohortFixture{}
			for {
				w, e := WaitCohort(ctx, dir, role, after)
				if e != nil {
					errs <- e
					return
				}
				if w.Event == "TERMINAL" {
					returns <- CohortRoleReturn{Kind: "SCRIPTED_CONTRACT", Role: role, LaunchID: "scripted-" + role, TerminalSHA256: w.TerminalSHA256, Status: "RETURNED", ReturnSHA256: hash([]byte("scripted return")), Sequence: after}
					return
				}
				task, e := ReadCohortBundle(*w.Bundle)
				if e != nil {
					errs <- e
					return
				}
				raw, e := f.ExchangeCohort(ctx, mustJSON(task))
				if e != nil {
					errs <- e
					return
				}
				if e = SubmitCohort(dir, role, w.Bundle.Sequence, raw); e != nil {
					errs <- e
					return
				}
				after = w.Bundle.Sequence
			}
		}(role)
	}
	r, e := RunCohort(ctx, c, items, m)
	if e != nil || r.Reviewed != 20 {
		select {
		case roleErr := <-errs:
			t.Fatalf("ASSERT_COHORT_PHYSICAL_TWENTY: reviewed=%d calls=%d error=%v role=%v", r.Reviewed, r.Calls, e, roleErr)
		default:
			t.Fatalf("ASSERT_COHORT_PHYSICAL_TWENTY: reviewed=%d calls=%d error=%v", r.Reviewed, r.Calls, e)
		}
	}
	if _, e = FinalizeCohortHost(c, dir, [2]CohortRoleReturn{}); e == nil {
		t.Fatal("ASSERT_TERMINAL_NOT_ROLE_RETURN")
	}
	if e = m.TerminalCohort(r); e != nil {
		t.Fatal(e)
	}
	var observed [2]CohortRoleReturn
	for n := 0; n < 2; n++ {
		select {
		case ret := <-returns:
			idx := 0
			if ret.Role == "reviewer" {
				idx = 1
			}
			observed[idx] = ret
		case e := <-errs:
			t.Fatal(e)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	bad := observed
	bad[1].Status = "UNKNOWN"
	if _, e = FinalizeCohortHost(c, dir, bad); e == nil {
		t.Fatal("ASSERT_MISSING_RETURN_UNKNOWN")
	}
	view, e := FinalizeCohortHost(c, dir, observed)
	if e != nil || view.Status != "SCRIPTED_CONTRACT_RETURNS_RECORDED" {
		t.Fatal("ASSERT_SCRIPTED_NOT_ACTUAL_AGENT", view, e)
	}
	if e = m.TerminalCohort(r); e == nil {
		t.Fatal("ASSERT_TERMINAL_WRITE_FAILURE_SURFACED")
	}
	deadline, stop := context.WithTimeout(context.Background(), time.Millisecond)
	defer stop()
	empty, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	os.Chmod(empty, 0700)
	w, e := WaitCohort(deadline, empty, "reviewer", 0)
	if e == nil || w.Event != "DEADLINE" {
		t.Fatal("ASSERT_DEADLINE_DISTINCT_TERMINAL")
	}
}
