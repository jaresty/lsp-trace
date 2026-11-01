//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// None_Model: own-store admission fixtures. The actual physical mailbox / CLI
// restart counterexample is exercised by TestHostCohortPinRepairImmediateFailure.
func TestCohortRestartDurableAdmission(t *testing.T) {
	for _, kind := range []string{"intact", "receipt-bytes", "receipt-digest", "receipt-sequence", "response", "claim-config", "claim-grant", "claim-role", "claim-sequence", "claim-digest", "snapshot-high", "snapshot-low", "missing-snapshot", "missing-response", "oversized-receipt", "oversized-response", "oversized-claim", "config", "unsafe-class"} {
		t.Run(kind, func(t *testing.T) {
			c, items := cohortInputs(t)
			before, err := RunCohort(context.Background(), c, items, &cohortFixture{})
			if err != nil {
				t.Fatal(err)
			}
			m := repairMailbox(t)
			pin, err := mailboxPin(c, m)
			if err != nil {
				t.Fatal(err)
			}
			put := func(rel string, raw []byte) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(c.Store, rel), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			get := func(rel string) []byte {
				t.Helper()
				b, err := os.ReadFile(filepath.Join(c.Store, rel))
				if err != nil {
					t.Fatal(err)
				}
				return b
			}
			put(cohortMailboxPinName, mustJSON(pin))
			if err := os.WriteFile(filepath.Join(m.Directory, "ready-interpreter.json"), mustJSON(Ready{CohortProtocol, "interpreter", "replacement-launch", c.RolePromptSHA256[0]}), 0600); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "receipt-bytes", "receipt-digest", "receipt-sequence":
				var receipt cohortReceipt
				if strict(get("host/claim-00000001.json"), &receipt) != nil {
					t.Fatal("fixture receipt")
				}
				if kind == "receipt-bytes" {
					receipt.Bytes--
				}
				if kind == "receipt-digest" {
					receipt.ResponseSHA256 = hash([]byte("foreign"))
				}
				if kind == "receipt-sequence" {
					receipt.Sequence++
				}
				put("host/claim-00000001.json", mustJSON(receipt))
			case "response":
				put("host/response-00000001.json", []byte("cheaper conflicting copy"))
			case "claim-config", "claim-grant", "claim-role", "claim-sequence", "claim-digest":
				var task CohortTask
				if strict(get("claim/claim-00000001.json"), &task) != nil {
					t.Fatal("fixture claim")
				}
				switch kind {
				case "claim-config":
					task.ConfigSHA256 = hash([]byte("foreign"))
				case "claim-grant":
					task.Grant = hash([]byte("foreign"))
				case "claim-role":
					task.Role = "reviewer"
				case "claim-sequence":
					task.Sequence++
				}
				task.RequestDigest = ""
				task.RequestDigest = hash(mustJSON(task))
				if kind == "claim-digest" {
					task.RequestDigest = hash([]byte("foreign"))
				}
				put("claim/claim-00000001.json", mustJSON(task))
			case "snapshot-high", "snapshot-low":
				paths, err := filepath.Glob(filepath.Join(c.Store, "pages/state-*.json"))
				if err != nil || len(paths) == 0 {
					t.Fatal("fixture snapshots", err)
				}
				last := paths[len(paths)-1]
				raw, err := os.ReadFile(last)
				if err != nil {
					t.Fatal(err)
				}
				var p cohortProgress
				if strict(raw, &p) != nil {
					t.Fatal("fixture snapshot")
				}
				if kind == "snapshot-high" {
					p.ResponseBytes++
				} else {
					p.ResponseBytes--
				}
				if err = os.WriteFile(last, mustJSON(p), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing-snapshot":
				paths, err := filepath.Glob(filepath.Join(c.Store, "pages/state-*.json"))
				if err != nil {
					t.Fatal(err)
				}
				for _, p := range paths {
					if err = os.Remove(p); err != nil {
						t.Fatal(err)
					}
				}
			case "missing-response":
				if err := os.Remove(filepath.Join(c.Store, "host/response-00000001.json")); err != nil {
					t.Fatal(err)
				}
			case "oversized-receipt":
				put("host/claim-00000001.json", []byte(strings.Repeat("x", 4097)))
			case "oversized-response":
				put("host/response-00000001.json", []byte(strings.Repeat("x", (64<<10)+2)))
			case "oversized-claim":
				put("claim/claim-00000001.json", []byte(strings.Repeat("x", c.MaxRequestBytes+1)))
			case "config":
				c.Grant = hash([]byte("not this store"))
			}
			class := "HOST_TOOL_UNAVAILABLE"
			if kind == "unsafe-class" {
				class = "raw source/environment text"
			}
			for attempt := 0; attempt < 2; attempt++ {
				got, err := DeliverCohortFailure(c, c.Requested, class)
				if err == nil {
					t.Fatal("ASSERT_RESTART_API_HISTORY_REFUSED")
				}
				if kind == "config" {
					if got.Schema != "" {
						t.Fatal("ASSERT_RESTART_FOREIGN_CONFIG_REFUSED")
					}
					continue
				}
				want := cohortAccountingUnknown
				switch kind {
				case "intact", "snapshot-low", "missing-snapshot", "unsafe-class":
				default:
					want = "UNKNOWN_LOCAL_PREVIOUS_OBSERVATION"
				}
				if got.Schema == "" || got.AccountingValidation != want || got.MachinePaths != [2]string{} || got.HumanPaths != [2]string{} {
					t.Fatalf("ASSERT_RESTART_RECEIPT_ADMISSION: kind=%s schema=%q accounting=%q want=%q", kind, got.Schema, got.AccountingValidation, want)
				}
				if want == cohortAccountingUnknown && (got.ResponseBytes != before.ResponseBytes || got.Calls != before.Calls || got.RequestBytes != before.RequestBytes) {
					t.Fatal("ASSERT_RESTART_KNOWN_BYTE_EXACT")
				}
				if got.ResponseBytes < before.ResponseBytes || got.Calls < before.Calls || got.RequestBytes < before.RequestBytes {
					t.Fatalf("ASSERT_RESTART_LOCAL_OBSERVATION_NOT_REFUNDED: kind=%s got=%d want>=%d", kind, got.ResponseBytes, before.ResponseBytes)
				}
				if kind == "unsafe-class" && got.Status != "FAILED_RECOVERY_REFUSED" {
					t.Fatal("ASSERT_RESTART_SAFE_CLASS")
				}
				if got.Accepted || got.Authority != 0 || got.SourceCompleteness != "UNKNOWN" || got.FeatureIdentity != "UNRESOLVED" || got.AbsenceInference != "PROHIBITED" {
					t.Fatal("ASSERT_RESTART_CEILINGS")
				}
				if _, err := LatestCohort(c); err == nil {
					t.Fatal("ASSERT_RESTART_LATEST_REFUSED")
				}
				if _, err := FinalizeCohortHost(c, m.Directory, [2]CohortRoleReturn{}); err == nil {
					t.Fatal("ASSERT_RESTART_FINALIZE_REFUSED")
				}
			}
			t.Logf("ASSERT_RESTART_RECEIPT_ADMISSION: PASS %s", kind)
		})
	}
}

func TestCohortRestartInvalidConfigRefusal(t *testing.T) {
	c, _ := cohortInputs(t)
	c.Store = "restart-untrusted-" + filepath.Base(t.TempDir())
	t.Cleanup(func() { os.Remove(c.Store) })
	if r, e := DeliverCohortFailure(c, c.Requested, "FAIL"); e == nil || r.Schema != "" {
		t.Fatal("ASSERT_RESTART_INVALID_CONFIG_REFUSAL")
	}
	if _, e := os.Stat(c.Store); !os.IsNotExist(e) {
		t.Fatal("ASSERT_RESTART_UNTRUSTED_PATH_NOT_CREATED")
	}
}

func TestCohortRestartOverBudgetReturnStillSpent(t *testing.T) {
	c, items := cohortInputs(t)
	c.MaxOutputBytes = 64 << 10
	reply := []byte(strings.Repeat("x", (64<<10)+1))
	before, err := RunCohort(context.Background(), c, items, cohortBackendFunc(func(context.Context, []byte) ([]byte, error) { return reply, nil }))
	if err == nil || before.ResponseBytes != len(reply) {
		t.Fatal("fixture did not retain over-budget return")
	}
	for attempt := 0; attempt < 2; attempt++ {
		got, err := DeliverCohortFailure(c, c.Requested, "RESOURCE_LIMIT")
		if err == nil || got.ResponseBytes != len(reply) || got.RequestBytes != before.RequestBytes || got.Calls != before.Calls || got.AccountingValidation != cohortAccountingUnknown || got.MachinePaths != [2]string{} || got.HumanPaths != [2]string{} {
			t.Fatalf("ASSERT_RESTART_OVER_BUDGET_STILL_SPENT: bytes=%d want=%d accounting=%s", got.ResponseBytes, len(reply), got.AccountingValidation)
		}
		if _, err := LatestCohort(c); err == nil {
			t.Fatal("ASSERT_RESTART_OVER_BUDGET_NOT_EXACT")
		}
	}
}
