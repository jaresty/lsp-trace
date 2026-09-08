package graphprovenance

import (
	"strings"
	"testing"
)

func TestV2NumericPreflight(t *testing.T) {
	for _, number := range []string{"1e1000000000", "1e-1000000000", strings.Repeat("9", 65)} {
		if err := preflightV2([]byte(`{"known":`+number+`}`), MaxEnvelopeBytesV2); err == nil {
			t.Fatal("ASSERT_V2_KNOWN_NUMERIC_PREFLIGHT:", number)
		}
	}
	for _, raw := range []string{`{"known":18446744073709551615}`, `{"data":{"opaque":1e1000000000}}`} {
		if err := preflightV2([]byte(raw), MaxEnvelopeBytesV2); err != nil {
			t.Fatal("ASSERT_V2_NUMERIC_DOMAIN_OR_OPACITY:", err)
		}
	}
	t.Log("ASSERT_V2_KNOWN_NUMERIC_PREFLIGHT: PASS; ASSERT_V2_NUMERIC_DOMAIN_OR_OPACITY: PASS")
}
