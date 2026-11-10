//go:build (darwin || linux) && !lsp_trace_controlled_clock

package inventorysuccessor_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Keep the controlled consumer regressions in ordinary go test. The alternate
// clock is compiled only into this bounded child test binary, never production.
func TestConnectedHOSTTimingV2(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 170*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-tags=lsp_trace_controlled_clock", "-timeout=120s", "-count=1", "-run=^TestConnectedHOSTTimingV2$", "-v", ".")
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=-mod=readonly", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "PYTHONDONTWRITEBYTECODE=1")
	raw, e := cmd.CombinedOutput()
	t.Logf("%s", raw)
	if e != nil {
		t.Fatal("ASSERT_HOST_TIMING_CONTROLLED_BUILD", e)
	}
}
