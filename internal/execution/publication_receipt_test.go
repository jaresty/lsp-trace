package execution

import (
	"bytes"
	"errors"
	"testing"

	"lsp-trace/internal/verification"
)

func TestReceiptRequiresDirectorySyncResult(t *testing.T) {
	sentinel := errors.New("directory fsync failed after file sync")
	for _, tc := range []struct {
		name    string
		checked bool
		err     error
		want    string
	}{
		{"completed", true, nil, verification.DirectoryDurabilityChecked},
		{"unsupported", false, nil, verification.DirectoryDurabilityUnavailable},
		{"failed", false, sentinel, ""},
		{"failure dominates claimed completion", true, sentinel, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			raw, err := receiptForPublishedArtifact([]byte("artifact"), func() (bool, error) { calls++; return tc.checked, tc.err })
			if calls != 1 {
				t.Fatal("directory sync was not requested")
			}
			if tc.err != nil {
				if !errors.Is(err, sentinel) || len(raw) != 0 {
					t.Fatalf("receipt issued on directory failure: %s %v", raw, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(raw, []byte(`"directory_durability":"`+tc.want+`"`)) {
				t.Fatalf("untruthful result: %s", raw)
			}
			if err := verification.VerifyReceipt([]byte("artifact"), raw); err != nil {
				t.Fatal(err)
			}
		})
	}
}
