package vcssymbolsidecar

import (
	"context"
	"testing"
	"time"
)

func TestLSPProcessProviderRejectsRelativeExecutable(t *testing.T) {
	_, err := (LSPProcessProvider{Command: "gopls", RequestTimeout: time.Second}).Start(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("ASSERT_HISTORICAL_LSP_REQUIRES_ABSOLUTE_EXECUTABLE")
	}
}
func TestLSPProcessProviderBoundsInitialize(t *testing.T) {
	_, err := (LSPProcessProvider{Command: "/bin/cat", RequestTimeout: 50 * time.Millisecond}).Start(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("ASSERT_HISTORICAL_LSP_INITIALIZE_TIMEOUT")
	}
}
