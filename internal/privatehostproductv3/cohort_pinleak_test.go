//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestPinRepairDeadlineNoDescriptorLeak(t *testing.T) {
	count := func() int {
		t.Helper()
		d, e := os.Open("/dev/fd")
		if e != nil {
			t.Fatal(e)
		}
		entries, e := d.Readdirnames(-1)
		closeErr := d.Close()
		if e != nil || closeErr != nil {
			t.Fatal(e, closeErr)
		}
		return len(entries)
	}
	run := func() *store {
		t.Helper()
		c, items := cohortInputs(t)
		m := repairMailbox(t)
		m.ExchangeTimeout = time.Millisecond
		var root *store
		if _, e := RunCohort(context.Background(), c, items, pinCloseBackend{m, &root, "deadline"}); e == nil {
			t.Fatal("deadline fixture succeeded")
		}
		return root
	}
	warm := run()
	runtime.KeepAlive(warm)
	before := count()
	var roots []*store
	for n := 0; n < 12; n++ {
		roots = append(roots, run())
	}
	after := count()
	runtime.KeepAlive(roots)
	if after != before {
		t.Fatalf("ASSERT_PIN_DEADLINE_NO_DESCRIPTOR_LEAK: before=%d after=%d", before, after)
	}
}
