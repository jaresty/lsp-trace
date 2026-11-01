//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// None_Model: controlled substitutions at the actual pre-exchange boundary.
type pinSwapBackend struct {
	CohortMailbox
	swap func()
}

func (b pinSwapBackend) ExchangeCohort(ctx context.Context, raw []byte) ([]byte, error) {
	b.swap()
	return b.CohortMailbox.ExchangeCohort(ctx, raw)
}
func TestPinRepairBeforeDisclosure(t *testing.T) {
	for _, kind := range []string{"ready", "directory"} {
		t.Run(kind, func(t *testing.T) {
			c, items := cohortInputs(t)
			m := repairMailbox(t)
			m.ExchangeTimeout = 30 * time.Millisecond
			foreign := m.Directory
			b := pinSwapBackend{m, func() {
				if kind == "ready" {
					path := filepath.Join(m.Directory, "ready-interpreter.json")
					raw, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					var r Ready
					if strict(raw, &r) != nil {
						t.Fatal("ready fixture")
					}
					r.LaunchID = "different-canonical-launch"
					if e = os.WriteFile(path, mustJSON(r), 0600); e != nil {
						t.Fatal(e)
					}
				} else {
					if e := os.Rename(m.Directory, m.Directory+"-original"); e != nil {
						t.Fatal(e)
					}
					if e := PrepareMailbox(m.Directory); e != nil {
						t.Fatal(e)
					}
					for _, role := range []string{"interpreter", "reviewer"} {
						if e := MarkCohortReady(m.Directory, Ready{CohortProtocol, role, "foreign-" + role, hash([]byte(CohortRolePrompt(role)))}); e != nil {
							t.Fatal(e)
						}
					}
				}
			}}
			r, e := RunCohort(context.Background(), c, items, b)
			names, readErr := os.ReadDir(foreign)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, n := range names {
				if n.Name() != "ready-interpreter.json" && n.Name() != "ready-reviewer.json" {
					t.Fatalf("ASSERT_PIN_NO_FOREIGN_DISCLOSURE: %s", n.Name())
				}
			}
			if e == nil || r.Calls != 1 || r.RequestBytes == 0 {
				t.Fatalf("ASSERT_PIN_CLAIM_NOT_REFUNDED: calls=%d bytes=%d err=%v", r.Calls, r.RequestBytes, e)
			}
		})
	}
}

func TestPinRepairImmediateRefusalSpend(t *testing.T) {
	for _, nav := range []bool{false, true} {
		t.Run(fmt.Sprint(nav), func(t *testing.T) {
			c, items := cohortInputs(t)
			c.Navigation = nav
			f := &cohortFixture{}
			known := 0
			stop := 3
			if nav {
				stop = 5
			}
			b := cohortBackendFunc(func(ctx context.Context, raw []byte) ([]byte, error) {
				h, e := parseCohortFrame(raw)
				if e != nil {
					return nil, e
				}
				var reply []byte
				if h.Protocol == CohortNavigationProtocol {
					reply = []byte("{\"Protocol\":\"WRONG\"}")
				} else {
					reply, e = f.ExchangeCohort(ctx, raw)
				}
				known += len(reply)
				if h.Sequence == stop {
					if e := os.WriteFile(filepath.Join(c.Store, fmt.Sprintf("host/response-%08d.json", stop)), []byte("x"), 0600); e != nil {
						t.Fatal(e)
					}
				}
				return reply, e
			})
			r, e := RunCohort(context.Background(), c, items, b)
			if e == nil || r.ResponseBytes != known {
				t.Fatalf("ASSERT_PIN_IMMEDIATE_KNOWN_RX: got=%d known=%d err=%v", r.ResponseBytes, known, e)
			}
			if r.AccountingValidation != cohortAccountingUnknown || r.MachinePaths != [2]string{} || r.HumanPaths != [2]string{} {
				t.Fatal("ASSERT_PIN_REFUSAL_NOT_EXACT_NO_PAIRS", r.AccountingValidation, r.MachinePaths)
			}
			if r.Reviewed != map[bool]int{false: 10, true: 20}[nav] {
				t.Fatal("ASSERT_PIN_PRIOR_REVIEWS_RETAINED", r.Reviewed)
			}
			var receipt cohortReceipt
			raw, readErr := os.ReadFile(filepath.Join(c.Store, fmt.Sprintf("host/claim-%08d.json", stop)))
			if readErr != nil || strict(raw, &receipt) != nil || receipt.Bytes <= 1 {
				t.Fatalf("ASSERT_PIN_RETURN_RECEIPT_DURABLE: receipt=%+v err=%v", receipt, readErr)
			}
		})
	}
}
