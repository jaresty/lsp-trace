package source

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReadRegularInputBounded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input"), []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, tc := range []struct {
		name    string
		limit   int64
		success bool
	}{
		{"input", 3, true}, {"input", 2, false}, {"input", 0, false}, {"input", 1 << 31, false},
		{"../input", 3, false}, {"./input", 3, false}, {"/input", 3, false}, {"input:alias", 3, false}, {"missing", 3, false}, {".", 3, false},
	} {
		data, err := ReadRegularInputBounded(root, tc.name, tc.limit)
		if tc.success {
			if err != nil || !bytes.Equal(data, []byte("abc")) {
				t.Fatalf("ASSERT_COMPLETE_BOUNDED_READ: %q %v", data, err)
			}
		} else if err == nil || data != nil {
			t.Fatalf("ASSERT_REJECT_NO_PARTIAL_BYTES: %q %d %q %v", tc.name, tc.limit, data, err)
		}
	}
}
