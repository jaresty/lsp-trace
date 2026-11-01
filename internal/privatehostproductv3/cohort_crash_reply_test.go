//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCohortCrashAfterPhysicalReplyBeforeSnapshot(t *testing.T) {
	c, items := cohortInputs(t)
	f := &cohortFixture{}
	var physical int
	func() {
		defer func() {
			if recover() == nil {
				t.Error("controlled crash absent")
			}
		}()
		RunCohort(context.Background(), c, items, cohortBackendFunc(func(ctx context.Context, raw []byte) ([]byte, error) {
			reply, e := f.ExchangeCohort(ctx, raw)
			if f.calls == 3 {
				physical = len(reply)
				if e = os.WriteFile(filepath.Join(c.Store, "host/response-00000003.json"), reply, 0600); e != nil {
					t.Fatal(e)
				}
				panic("physical reply durable; response counter snapshot not saved")
			}
			return reply, e
		}))
	}()
	r, e := DeliverCohortFailure(c, 20, "AMBIGUOUS_REPLY")
	if e != nil || r.Calls != 3 || r.Reviewed != 10 || r.ResponseBytes != f.response || r.ResponseBytes < physical {
		t.Fatalf("ASSERT_COHORT_AMBIGUOUS_REPLY_SPENT_NOT_ADOPTED: calls=%d reviewed=%d response=%d expected=%d error=%v", r.Calls, r.Reviewed, r.ResponseBytes, f.response, e)
	}
	if _, e = LatestCohort(c); e != nil {
		t.Fatal("ASSERT_COHORT_AMBIGUOUS_QUALIFIED_PRIOR", e)
	}
}
