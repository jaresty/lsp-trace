package captureset

import (
	"os"
	"testing"

	"lsp-trace/internal/publication"
)

func TestPrivateCaptureSetDiagnostic(t *testing.T) {
	rootPath := os.Getenv("LSP_TRACE_TEST_PUBLICATION_ROOT")
	selector := os.Getenv("LSP_TRACE_TEST_CAPTURE_SET_SELECTOR")
	if rootPath == "" || selector == "" {
		t.Skip("private diagnostic inputs not supplied")
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("PRIVATE_CAPTURE_SET_ROOT_OPEN_FAILED")
	}
	defer root.Close()
	manifest, err := NewPublisher(root).Verify(selector, NativeV5Authority())
	if err != nil {
		t.Fatalf("PRIVATE_CAPTURE_SET_VERIFY_FAILED: %v", err)
	}
	t.Logf("PRIVATE_CAPTURE_SET_VERIFIED constituents=%d", len(manifest.Constituents))
}
