//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCohortCanonicalNamespaceAndQuartet(t *testing.T) {
	for _, name := range []string{"foreign.json", "host/report-00000001.json", "claim/response-00000001.json", "plan/config-00000002.json", "result/report.json"} {
		t.Run(name, func(t *testing.T) {
			c, _ := cohortInputs(t)
			if e := InitializeCohort(c); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(c.Store, name), []byte(`{}`), 0600); e != nil {
				t.Fatal(e)
			}
			if InitializeCohort(c) == nil {
				t.Fatal("ASSERT_COHORT_FOREIGN_CANONICAL_NAMESPACE_REFUSED")
			}
		})
	}
	for _, mode := range []string{"missing", "mismatch", "shared-inode", "mode"} {
		t.Run(mode, func(t *testing.T) {
			c, items := cohortInputs(t)
			r, e := RunCohort(context.Background(), c, items, &cohortFixture{})
			if e != nil {
				t.Fatal(e)
			}
			p := filepath.Join(c.Store, r.HumanPaths[1])
			switch mode {
			case "missing":
				os.Remove(p)
			case "mismatch":
				os.WriteFile(p, []byte("not regenerated human"), 0600)
			case "shared-inode":
				os.Remove(p)
				os.Link(filepath.Join(c.Store, r.HumanPaths[0]), p)
			case "mode":
				os.Chmod(p, 0644)
			}
			if _, e = LatestCohort(c); e == nil {
				t.Fatal("ASSERT_COHORT_EXACT_PRIVATE_FOUR_DISTINCT_VIEWS")
			}
		})
	}
}
func TestCohortCancellationRetainsTen(t *testing.T) {
	c, items := cohortInputs(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &cohortFixture{}
	r, e := RunCohort(ctx, c, items, cohortBackendFunc(func(ctx context.Context, raw []byte) ([]byte, error) {
		if f.calls == 2 {
			cancel()
			return nil, ctx.Err()
		}
		return f.ExchangeCohort(ctx, raw)
	}))
	if e == nil || r.Status != "CANCELLED" || r.Reviewed != 10 || r.Calls != 3 || r.Pending != 10 {
		t.Fatalf("ASSERT_COHORT_CANCELLED_PARTIAL: status=%s reviewed=%d calls=%d error=%v", r.Status, r.Reviewed, r.Calls, e)
	}
	if _, e = LatestCohort(c); e != nil {
		t.Fatal("ASSERT_COHORT_CANCELLED_SPENT_HISTORY", e)
	}
}
