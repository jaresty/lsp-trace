package sessionruntime

import "testing"

func mustPrivateB4ByteAccountV2(t *testing.T) *privateB4ByteAccountV2 {
	t.Helper()
	account, failure := newPrivateB4ByteAccountV2()
	if failure != "" {
		t.Fatal(failure)
	}
	return account
}
