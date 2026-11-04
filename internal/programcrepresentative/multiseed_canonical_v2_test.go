package programcrepresentative

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMultiSeedV2CanonicalStringSize(t *testing.T) {
	t.Log("procedure: canonicalStringSizeV2(input) equals uint64(len(json.Marshal(input))); invalid UTF-8 refuses")
	for _, s := range []string{"", "plain", "\x00\b\t\n\f\r\"\\<>&", "é世\u2028\u2029", strings.Repeat("<", 1000)} {
		got, err := canonicalStringSizeV2(s)
		raw, _ := json.Marshal(s)
		if err != nil || got != uint64(len(raw)) {
			t.Fatalf("EXACT_ESCAPED_STRING_SIZE: got=%d expected=%d err=%v", got, len(raw), err)
		}
	}
	if _, err := canonicalStringSizeV2(string([]byte{0xff})); err == nil {
		t.Fatal("INVALID_UTF8_REFUSAL")
	}
}
