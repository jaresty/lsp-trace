//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCohortPriorStageCannotLaunderConsistentBytes(t *testing.T) {
	write := func(path string, b []byte) {
		t.Helper()
		if e := os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	c, items := cohortInputs(t)
	r, e := RunCohort(context.Background(), c, items, &cohortFixture{})
	if e != nil {
		t.Fatal(e)
	}
	// Preserve lengths/costs and all candidate decisions while changing the stage.
	// Recompute request/reply bindings and the backend receipt. Hash consistency
	// alone is insufficient: the actual cohort grammar still must refuse.
	claimPath := filepath.Join(c.Store, "claim/claim-00000001.json")
	raw, e := os.ReadFile(claimPath)
	if e != nil {
		t.Fatal(e)
	}
	var task CohortTask
	json.Unmarshal(raw, &task)
	task.Stage = "HACKED!"
	task.RequestDigest = ""
	task.RequestDigest = hash(mustJSON(task))
	claim := mustJSON(task)
	if len(claim) != len(raw) {
		t.Fatal("fixture must preserve physical size")
	}
	write(claimPath, claim)
	replyPath := filepath.Join(c.Store, "host/response-00000001.json")
	raw, e = os.ReadFile(replyPath)
	if e != nil {
		t.Fatal(e)
	}
	var response CohortResponse
	json.Unmarshal(raw, &response)
	response.RequestDigest = task.RequestDigest
	reply := mustJSON(response)
	if len(reply) != len(raw) {
		t.Fatal("fixture response size")
	}
	write(replyPath, reply)
	receiptPath := filepath.Join(c.Store, "host/claim-00000001.json")
	write(receiptPath, mustJSON(cohortReceipt{1, hash(reply), len(reply), true}))
	if r.Reviewed != 20 {
		t.Fatal("fixture")
	}
	if _, e = LatestCohort(c); e == nil {
		t.Fatal("ASSERT_COHORT_STAGE_GRAMMAR_NOT_HASH_AUTHENTICATION")
	}
}
