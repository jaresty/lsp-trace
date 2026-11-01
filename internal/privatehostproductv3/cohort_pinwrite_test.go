//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPinRepairFailedSnapshotRetainsKnownSpend(t *testing.T) {
	c, items := cohortInputs(t)
	reply := []byte("returned physical bytes")
	b := cohortBackendFunc(func(context.Context, []byte) ([]byte, error) {
		// Disposable fixture only: force response/final snapshot writes to fail.
		if e := os.RemoveAll(filepath.Join(c.Store, "pages")); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(c.Store, "pages"), nil, 0600); e != nil {
			t.Fatal(e)
		}
		return reply, nil
	})
	r, e := RunCohort(context.Background(), c, items, b)
	if e == nil || r.Schema == "" || r.ResponseBytes != len(reply) || r.AccountingValidation != cohortAccountingUnknown || r.MachinePaths != [2]string{} || r.HumanPaths != [2]string{} {
		t.Fatalf("ASSERT_PIN_FAILED_SNAPSHOT_KNOWN_RX: schema=%q response=%d qualification=%q err=%v", r.Schema, r.ResponseBytes, r.AccountingValidation, e)
	}
}
