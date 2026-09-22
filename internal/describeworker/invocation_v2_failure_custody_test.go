package describeworker

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const emptySHA256 = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestInvocationRecordV2EmptyStreamDigestCustodyAndTamper(t *testing.T) {
	invocation, err := NewInvocationRecordV2(validBinding(), StatusTimeout, ResourceReceipt{Started: true, TerminalOutcomes: 1, Reaped: true}, ResponseRecordV2{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := invocation.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	receipt, ok := wire["receipt"].(map[string]any)
	if !ok || receipt["stdout_sha256"] != emptySHA256 {
		t.Fatalf("ASSERT_INVOCATION_V2_STDOUT_DIGEST: %s", raw)
	}
	if receipt["stderr_sha256"] != emptySHA256 {
		t.Fatalf("ASSERT_INVOCATION_V2_STDERR_DIGEST: %s", raw)
	}
	if bytes.Contains(raw, []byte("stdout_body")) || bytes.Contains(raw, []byte("stderr_body")) || bytes.Contains(raw, []byte("/tmp/")) {
		t.Fatalf("ASSERT_INVOCATION_V2_NO_RAW_OR_PATH: %s", raw)
	}
	tampered := bytes.Replace(raw, []byte(emptySHA256), []byte("sha256:"+strings.Repeat("a", 64)), 1)
	if _, err := ParseInvocationRecordV2(tampered); err == nil {
		t.Fatal("ASSERT_INVOCATION_V2_CUSTODY_TAMPER_REJECTED")
	}
}
