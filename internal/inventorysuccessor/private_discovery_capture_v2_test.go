package inventorysuccessor

import (
	"lsp-trace/sessionruntime"
	"testing"
)

func TestCapture88OperationSubaccount(t *testing.T) {
	q, ok := sessionruntime.PrivateDiscoveryCaptureQuoteForOptions(&sessionruntime.PrivateDiscoveryCaptureOptions{Permission: true, Destination: "/operator-owned"})
	if !ok {
		t.Fatal("quote")
	}
	a := privateDiscoveryCaptureAccountNewV2()
	before := a.remaining
	requested := privateDiscoveryCaptureCurrenciesV2(q)
	for i, n := range requested {
		b := privateDiscoveryCaptureAccountNewV2()
		if n > 0 {
			b.remaining[i] = n - 1
		} else {
			b.remaining[i] = 0
		}
		test := q
		if n == 0 {
			switch i {
			case 5:
				test.ProviderRequests = 1
			case 6:
				test.ProviderMessages = 1
			case 8:
				test.ActiveNS = 1
			case 10:
				test.RequestBytes = 1
			case 11:
				test.ResponseBytes = 1
			case 12:
				test.WireBytes = 1
			default:
				continue
			}
		}
		prior := b.remaining
		if b.reserve(test) == nil || b.used || b.remaining != prior {
			t.Fatal("ASSERT_CAPTURE88_ALL_CURRENCY_PREFLIGHT")
		}
	}
	if a.reserve(q) != nil {
		t.Fatal("ASSERT_CAPTURE88_SUBACCOUNT_FIT")
	}
	for i, n := range requested {
		if a.remaining[i] != before[i]-n {
			t.Fatal("ASSERT_CAPTURE88_SUBACCOUNT_DEBIT")
		}
	}
	if a.reserve(q) == nil {
		t.Fatal("ASSERT_CAPTURE88_NO_RENEWAL")
	}
}
