//go:build darwin || linux

package applicationinventory

import (
	"fmt"
	dw "lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
	"strings"
	"testing"
)

func TestCompletionQuoteCensusPreparedMatrix(t *testing.T) {
	for n := 2; n <= 5; n++ {
		for p := 0; p <= n; p++ {
			t.Run(fmt.Sprintf("N%d_P%d", n, p), func(t *testing.T) {
				refs := make([]publication.StoredObject, n)
				payloads := make([]uint64, n)
				for i := 0; i < p; i++ {
					refs[i] = publication.StoredObject{ID: strings.Repeat("a", 64), Bytes: 1024}
					payloads[i] = 65536 + 4096
				}
				q, e := quoteDurableCompletion(refs, 4096, 32768, dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}, payloads)
				if e != nil {
					if p <= 3 {
						t.Fatal("small scalar quote unexpectedly refused", e)
					}
					t.Log("whole sequence exceeds quote cap; admission refuses")
					return
				}
				if len(q.Stages) != p+3 {
					t.Fatal("prepared denominator", len(q.Stages), p)
				}
				total, ok := capacitySum(q.Stages...)
				if !ok || total > durableRunLimit {
					t.Fatal("unfunded completion", total)
				}
				if q.StateUpper < uint64(n)*256+4096 {
					t.Fatal("admitted denominator lost")
				}
				t.Logf("total=%d stages=%v state=%d", total, q.Stages, q.StateUpper)
			})
		}
	}
}
func TestCompletionQuoteOverflowAndInvalidRefs(t *testing.T) {
	if _, ok := capacitySum(^uint64(0), 1); ok {
		t.Fatal("overflow accepted")
	}
	for _, r := range []publication.StoredObject{{ID: "bad", Bytes: 1}, {ID: strings.Repeat("a", 64), Bytes: int(^uint(0) >> 1)}} {
		if _, e := quoteDurableCompletion([]publication.StoredObject{r}, 4096, 1, dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}, []uint64{65536}); e == nil {
			t.Fatal("invalid ref accepted")
		}
	}
}
func TestCompletionConsumptionValidation(t *testing.T) {
	h := durableInventoryHead{Sequence: 1, Phase: "GroupDecision", Status: "READY", State: publication.StoredObject{Bytes: 1}, Limit: durableRunLimit, Used: 100, Work: 3200,
		Completion: durableCompletionPlan{Version: durableQuoteVersion, Stages: []uint64{1000, 2000, 3000}, StateUpper: 100, PreparationUsed: 100}}
	if !h.completionValid() {
		t.Fatal("initial plan rejected")
	}
	n, p, ok := h.protectedCapacity()
	if !ok || n != 6000 || p != 3 {
		t.Fatal("initial protection", n, p)
	}
	h.CompletionCursor = 1
	h.Status = "CLAIMED"
	if h.completionValid() {
		t.Fatal("transfer without spend accepted")
	}
	h.Used += 1000
	h.Work = h.Used * 32
	h.Packets++
	if !h.completionValid() {
		t.Fatal("charged transfer rejected")
	}
	h.Status = "READY"
	h.Sequence++
	h.Phase = "Reviewer"
	if !h.completionValid() {
		t.Fatal("successor rejected")
	}
	n, p, ok = h.protectedCapacity()
	if !ok || n != 5000 || p != 2 {
		t.Fatal("tail refunded", n, p)
	}
	h.Used = h.Limit - 4999
	h.Work = h.Used * 32
	if h.completionValid() {
		t.Fatal("tail starvation accepted")
	}
}
