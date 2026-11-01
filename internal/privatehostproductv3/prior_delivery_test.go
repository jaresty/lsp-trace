//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestHostProductPriorGrantConsistency(t *testing.T) {
	for _, mode := range []string{"persisted_mismatch", "broken_chain"} {
		t.Run(mode, func(t *testing.T) {
			c := testConfig(t)
			if err := Initialize(c); err != nil {
				t.Fatal(err)
			}
			prior, err := DeliverEarlyFailure(c, "INITIAL", "CONTROLLED")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "persisted_mismatch" {
				if err := os.WriteFile(filepath.Join(c.Store, "plan/config-00000001.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				c.AllowanceRevision++
				c.PreviousConfigSHA256 = "wrong ancestor"
				c.ApprovalSHA256 = ConfigDigest(c)
			}
			got, err := DeliverEarlyFailure(c, "CURRENT", "CONTROLLED")
			if err != nil || got.Admission.PriorMachinePaths != [2]string{} || !got.Admission.UnadmittedPublications || got.Admission.Coverage != "UNKNOWN" || got.MachinePaths == prior.MachinePaths {
				t.Fatal("ASSERT_INCONSISTENT_GRANT_NO_CARRYFORWARD", err)
			}
		})
	}
}

func TestHostProductInterruptedDeliveryNeverAdvertisesPairs(t *testing.T) {
	c := testConfig(t)
	if e := Initialize(c); e != nil {
		t.Fatal(e)
	}
	prior, e := DeliverEarlyFailure(c, "INITIAL", "CONTROLLED")
	if e != nil {
		t.Fatal(e)
	}
	s, e := openStore(c)
	if e != nil {
		t.Fatal(e)
	}
	mp, _ := reportPaths(3)
	fired := false
	s.visibleObserver = func() {
		if fired {
			return
		}
		fired = true
		if e := os.WriteFile(filepath.Join(c.Store, mp[1]), []byte("interrupted second view"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r := prior
	e = s.deliver(&r)
	s.close()
	if e == nil || r.MachinePaths != [2]string{} || r.HumanPaths != [2]string{} {
		t.Fatal("ASSERT_PARTIAL_CURRENT_NOT_PAIRED")
	}
	raw, e := os.ReadFile(filepath.Join(c.Store, mp[0]))
	if e != nil {
		t.Fatal(e)
	}
	next, e := DeliverEarlyFailure(c, "NEXT", "CONTROLLED")
	if e != nil {
		t.Fatal(e)
	}
	if next.Admission.PriorMachinePaths != prior.MachinePaths || next.MachinePaths[0] != "result/report-00000005.json" || next.Admission.PriorPublicationValidation != "UNKNOWN" {
		t.Fatal("ASSERT_NO_SPENT_INDEX_REUSE_OR_PARTIAL_ADMISSION")
	}
	again, e := DeliverEarlyFailure(c, "AGAIN", "CONTROLLED")
	if e != nil {
		t.Fatal(e)
	}
	if again.Admission.PriorPublicationValidation != "UNKNOWN" || again.Admission.PriorMachinePaths != next.MachinePaths {
		t.Fatal("ASSERT_UNKNOWN_SURVIVES_REPEATED_FAILURE")
	}
	after, e := os.ReadFile(filepath.Join(c.Store, mp[0]))
	if e != nil || !bytes.Equal(raw, after) {
		t.Fatal("ASSERT_PARTIAL_BYTES_REMAIN_IMMUTABLE")
	}
}
