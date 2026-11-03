//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"math"
	"testing"
	"time"
)

// Owned sampled-clock observations only. This does not sleep for historical
// latency, alter any system clock, or grant a new interval after preparation.
func TestApplicationUnit180ClockWindow(t *testing.T) {
	c, now := applicationFixture(t)
	c.Grant.ExpiresUnixNano = now.Add(10 * time.Minute).UnixNano()
	c.ApprovalSHA256 = ApplicationConfigDigest(c)
	a := applicationCreate(t, c, now)
	u := applicationUnit(c, "controlled-180-prepaid")
	b := applicationChild(4)
	b.ActiveNS = uint64(180 * time.Second)
	if e := a.AdmitUnit(u, b, time.Now()); e != nil {
		t.Fatal(e)
	}
	var stored applicationUnitLedger
	if e := a.implementation.object("unit", u.UnitID, &stored); e != nil {
		t.Fatal(e)
	}
	if stored.Allowance.ActiveNS != uint64(180*time.Second) || stored.ActiveDeadlineUnixNano > stored.Clock.WallNS+int64(180*time.Second) {
		t.Fatal("ASSERT_UNIT180_STORED_ADMISSION_WINDOW")
	}
	deadline, e := ReadApplicationUnitDeadline(c.Store, u.ApplicationSHA256, u.UnitID)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if d, ok := ctx.Deadline(); !ok || !d.Equal(deadline) {
		t.Fatal("actual unit context deadline")
	}
	for _, tc := range []struct {
		name          string
		elapsed, skew time.Duration
		reject        bool
	}{
		{"historic-elapsed-not-latency-promise", 129560 * time.Millisecond, 0, false},
		{"rollback", 100 * time.Millisecond, -30 * time.Second, true},
		{"forward", 100 * time.Millisecond, 30 * time.Second, true},
		{"expired", 180*time.Second + time.Millisecond, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stamp := stored.Clock
			stamp.MonotonicNS += int64(tc.elapsed)
			stamp.WallNS += int64(tc.elapsed + tc.skew)
			left, e := applicationIntervalRemaining(stored, stamp)
			if tc.reject {
				if e == nil {
					t.Fatal("ASSERT_UNIT180_CLOCK_SHIFT_REFUSED")
				}
				return
			}
			if e != nil || left > 180*time.Second-tc.elapsed && tc.elapsed < 180*time.Second || tc.elapsed >= 180*time.Second && left != 0 {
				t.Fatal("ASSERT_UNIT180_ELAPSED_NEVER_RESETS", left, e)
			}
			d, e := readApplicationUnitDeadline(c.Store, u.ApplicationSHA256, u.UnitID, func() (applicationClockStamp, error) { return stamp, nil })
			if e != nil {
				t.Fatal(e)
			}
			if tc.elapsed >= 180*time.Second && d.After(time.Now()) {
				t.Fatal("ASSERT_UNIT180_LATE_READER_NO_NEW_WINDOW")
			}
			t.Logf("ASSERT_UNIT180_CLOCK_WINDOW A-pass elapsed=%s remaining=%s", tc.elapsed, left)
		})
	}
	for _, ns := range []uint64{0, math.MaxUint64} {
		if _, e := applicationAdmissionDeadline(time.Now(), ns, c.Grant.ExpiresUnixNano, stored.Clock, stored.Clock); e == nil {
			t.Fatal("ASSERT_UNIT180_DURATION_OVERFLOW_REFUSED")
		}
	}
}
