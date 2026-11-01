//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

type cohortFixture struct {
	calls, request, response int
	reject                   bool
}

func (f *cohortFixture) ExchangeCohort(ctx context.Context, raw []byte) ([]byte, error) {
	f.calls++
	f.request += len(raw)
	var task CohortTask
	if e := json.Unmarshal(raw, &task); e != nil {
		return nil, e
	}
	out := CohortResponse{Protocol: CohortProtocol, RequestDigest: task.RequestDigest, Items: []CohortDecision{}}
	for _, i := range task.Items {
		d := CohortDecision{ItemID: i.ItemID, InputID: i.InputID, Target: i.Target, Attempt: i.Attempt, Predecessor: i.Predecessor, OwnBodySHA256: i.OwnBodySHA256, Revision: i.Revision, Disposition: "SUPPORTED", Label: "bounded lead", Explanation: "Own body description", Alternative: "another purpose", Falsifier: "caller contradicts this", Quote: i.Body, Start: 0, End: len(i.Body)}
		if f.reject && task.Role == "reviewer" && i.ItemID == "item-00" && i.Revision == 1 {
			d.Disposition = "UNSUPPORTED"
		}
		out.Items = append(out.Items, d)
	}
	b, e := json.Marshal(out)
	f.response += len(b)
	return b, e
}
func cohortInputs(t *testing.T) (CohortConfig, []CohortItem) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	c := CohortConfig{RolePromptSHA256: [2]string{hash([]byte(CohortRolePrompt("interpreter"))), hash([]byte(CohortRolePrompt("reviewer")))}, Requested: 20, Profile: CohortProfile, Protocol: CohortProtocol, SourceConfigSHA256: hash([]byte("source-config")), ScopeSHA256: hash([]byte("scope")), Store: filepath.Join(root, "cohort"), Grant: hash([]byte("explicit controlled synthetic grant")), MaxCalls: 16, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}
	var items []CohortItem
	for n := 0; n < 20; n++ {
		body := fmt.Sprintf("func Own%d() { /* own %d */ }", n, n)
		items = append(items, CohortItem{ItemID: fmt.Sprintf("item-%02d", n), InputID: fmt.Sprintf("input-%02d", n), Target: fmt.Sprintf("own-%02d", n), Attempt: "grant-1", Grant: c.Grant, ConfigSHA256: c.SourceConfigSHA256, ScopeSHA256: hash([]byte("scope")), PreparedSHA256: hash([]byte("prepared")), CheckpointSHA256: hash([]byte("checkpoint")), OriginalRawReferences: [4]string{hash([]byte("raw0")), hash([]byte("raw1")), hash([]byte("raw2")), hash([]byte("raw3"))}, Body: body, OwnBodySHA256: hash([]byte(body)), Revision: 1})
	}
	return c, items
}
func TestCohortTwentyReviewedWithinSixteen(t *testing.T) {
	c, items := cohortInputs(t)
	f := &cohortFixture{}
	r, e := RunCohort(context.Background(), c, items, f)
	if e != nil || r.Reviewed != 20 || r.Pending != 0 || r.Gaps != 0 || r.Calls != 4 || f.calls != 4 || r.Calls > 16 {
		t.Fatalf("ASSERT_COHORT_TWENTY_REVIEWED: reviewed=%d calls=%d err=%v", r.Reviewed, r.Calls, e)
	}
	if r.RequestBytes != f.request || r.ResponseBytes != f.response {
		t.Fatal("ASSERT_COHORT_PHYSICAL_BYTES_EXACT")
	}
	if r.Accepted || r.Authority != 0 || r.FeatureIdentity != "UNRESOLVED" || r.SourceCompleteness != "UNKNOWN" || r.AbsenceInference != "PROHIBITED" {
		t.Fatal("ASSERT_COHORT_AUTHORITY_CEILING")
	}
	if r.MachinePaths[0] == "" || r.HumanPaths[1] == "" {
		t.Fatal("ASSERT_COHORT_VERIFIED_QUARTET")
	}
}
func TestCohortCorrectionSameGrant(t *testing.T) {
	c, items := cohortInputs(t)
	f := &cohortFixture{reject: true}
	r, e := RunCohort(context.Background(), c, items, f)
	if e != nil || r.Reviewed != 20 || r.Calls != 6 || len(r.Outcomes[0].History) != 4 || r.Outcomes[0].Item.Revision != 2 {
		t.Fatalf("ASSERT_COHORT_CORRECTION_LINEAGE: %+v %v", r, e)
	}
}
