//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCohortMalformedPhysicalReplySpent(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	payload := []byte(`{"Protocol":"` + CohortProtocol + `","Protocol":"duplicate"}`)
	go func() {
		w, e := WaitCohort(ctx, dir, "interpreter", 0)
		if e != nil {
			done <- e
			return
		}
		if SubmitCohort(dir, "interpreter", w.Bundle.Sequence, payload) == nil {
			done <- fmt.Errorf("malformed reply accepted")
			return
		}
		done <- nil
	}()
	r, e := RunCohort(ctx, c, items, m)
	if e == nil || r.ResponseBytes != len(payload) || r.Calls != 1 {
		t.Fatalf("ASSERT_COHORT_INVALID_PHYSICAL_REPLY_CHARGED: calls=%d response=%d expected=%d error=%v", r.Calls, r.ResponseBytes, len(payload), e)
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
}
func TestCohortQualifiedPriorAndIncompleteSuccessor(t *testing.T) {
	c, items := cohortInputs(t)
	r, e := RunCohort(context.Background(), c, items, &cohortFixture{})
	if e != nil {
		t.Fatal(e)
	}
	// A last incomplete publication cannot erase the earlier valid quartet.
	if e = os.WriteFile(filepath.Join(c.Store, "result/report-00000003.json"), []byte(`{}`), 0600); e != nil {
		t.Fatal(e)
	}
	failed, e := DeliverCohortFailure(c, 20, "STALE_SOURCE")
	if e != nil || failed.Reviewed != r.Reviewed || failed.SourceValidation != "UNKNOWN" || failed.LedgerValidation != "UNKNOWN" {
		t.Fatal("ASSERT_COHORT_QUALIFIED_PRIOR_SURVIVES", e)
	}
	if _, e = LatestCohort(c); e != nil {
		t.Fatal("ASSERT_COHORT_QUALIFIED_PRIOR_HISTORY", e)
	}
	human, e := os.ReadFile(filepath.Join(c.Store, failed.HumanPaths[0]))
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(human, []byte("Current source validation: UNKNOWN; current ledger validation: UNKNOWN.")) || !bytes.Contains(human, []byte("Unadmitted publications: true")) {
		t.Fatal("ASSERT_COHORT_HUMAN_PRIOR_QUALIFIED_UNKNOWN")
	}
}
