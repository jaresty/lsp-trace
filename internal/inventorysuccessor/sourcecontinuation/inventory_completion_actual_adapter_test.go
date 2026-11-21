//go:build darwin || linux

package sourcecontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	dw "lsp-trace/internal/describeworker"
	"strings"
	"testing"
	"time"
)

func TestInventoryActualAdapterMaximalEscaping(t *testing.T) {
	limits := dw.SuppliedResponseLimits{MaxAdapterBytes: 65536, MaxSemanticBytes: 32768}
	for _, char := range []string{"<", "\\", "\""} {
		t.Run(char, func(t *testing.T) {
			// Search the legal boundary, never lower the grammar caps to fit a specimen.
			makeAnswer := func(n int) []byte {
				s := dw.SemanticResponseV2{Verdict: "COMPLETE", TargetRole: strings.Repeat(char, 4096), ConsumerNeed: dw.ConsumerNeedV2{Status: "UNRESOLVED"}, ProvidedBehavior: dw.ProvidedBehaviorV2{Value: strings.Repeat(char, 4096)}, BoundaryContribution: strings.Repeat(char, n), Limitations: []string{}}
				var raw bytes.Buffer
				enc := json.NewEncoder(&raw)
				enc.SetEscapeHTML(false)
				_ = enc.Encode(s)
				return dw.EncodeAdapterResultV2(strings.TrimSpace(raw.String()))
			}
			lo, hi := 1, 4096
			best := 0
			for lo <= hi {
				mid := (lo + hi) / 2
				if _, e := dw.ParseSuppliedTerminalResponse(makeAnswer(mid), limits); e == nil {
					best = mid
					lo = mid + 1
				} else {
					hi = mid - 1
				}
			}
			if best == 0 {
				t.Fatal("no legal specimen")
			}
			answer := makeAnswer(best)
			parsed, e := dw.ParseSuppliedTerminalResponse(answer, limits)
			if e != nil {
				t.Fatal(e)
			}
			if best < 4096 {
				if _, e = dw.ParseSuppliedTerminalResponse(makeAnswer(best+1), limits); e == nil {
					t.Fatal("boundary not maximal")
				}
			}
			host := quoteTestHost("captured-owner")
			b, _ := NewCallerBudgetV3(time.Now().Add(time.Minute), 64<<20)
			actualUpper, e := inventoryRecordUpper(context.Background(), host, len(answer), b)
			if e != nil {
				t.Fatal(e)
			}
			maxUpper, e := inventoryRecordUpper(context.Background(), host, 65536, b)
			if e != nil {
				t.Fatal(e)
			}
			record, e := dw.NewResponseRecordV2(parsed.CanonicalSemantic(), host)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := record.Bytes()
			if e != nil || uint64(len(raw)) > actualUpper || maxUpper-actualUpper != uint64(65536-len(answer)) {
				t.Fatal("actual adapter bound", e, len(raw), actualUpper, maxUpper)
			}
			t.Logf("ASSERT_ACTUAL_ADAPTER_BOUND: PASS char=%q fieldBytes=%d adapter=%d semantic=%d record=%d bound=%d", char, best, len(answer), len(parsed.CanonicalSemantic()), len(raw), actualUpper)
		})
	}
}
