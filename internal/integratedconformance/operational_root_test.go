package integratedconformance

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOperationalNativeRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("colon in a filename is not a Windows-native directory; ordinary operational fixtures exercise drive-qualified roots")
	}
	h := newOperationalHarness(t)
	request := operationalRequest(t)
	op := operationalMode(request)
	root := filepath.Join(t.TempDir(), "source:root")
	if err := os.Rename(op["source_root"].(string), root); err != nil {
		t.Fatal(err)
	}
	op["source_root"] = root
	for _, mode := range []string{"direct", "cli", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			resetOperationalPublication(t, request)
			got := h.run(mode, request, "", nil)
			if got.Failed {
				t.Fatalf("ASSERT_OPERATIONAL_NATIVE_ROOT: valid absolute filesystem root rejected: %s", got.Output)
			}
		})
	}
}
