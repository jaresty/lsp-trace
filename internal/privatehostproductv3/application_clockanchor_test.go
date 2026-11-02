//go:build darwin || linux

package privatehostproductv3

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

// Public constructor, real production sampler. A caller wall instant ahead of
// the sampler is the same admission input as rollback between those captures.
// No kernel, process time, or persisted record is forged or changed.
func TestApplicationClockAnchorDelayedRollback(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	u := applicationUnit(c, "anchor-delayed-rollback")
	before, e := applicationClockNow()
	if e != nil {
		t.Fatal(e)
	}
	caller := time.Unix(0, before.WallNS).Add(30 * time.Second)
	admission := a.AdmitUnit(u, applicationChild(0), caller)
	// Progressive reading after the fabricated future start: old normalization
	// makes wall and kernel elapsed agree again, reopening almost a full second.
	delayed := before
	delayed.MonotonicNS += int64(30100 * time.Millisecond)
	delayed.WallNS += int64(30100 * time.Millisecond)
	deadline, readErr := readApplicationUnitDeadline(c.Store, u.ApplicationSHA256, u.UnitID, func() (applicationClockStamp, error) { return delayed, nil })
	if readErr == nil && deadline.After(time.Now()) {
		t.Error("ASSERT_CLOCKANCHOR_DELAYED_ROLLBACK_NO_AUTHORITY")
	}
	if admission == nil {
		t.Error("ASSERT_CLOCKANCHOR_INCONSISTENT_ADMISSION_REFUSED")
	}
	if a.implementation.exists("unit", u.UnitID) {
		t.Error("ASSERT_CLOCKANCHOR_REFUSAL_NO_USABLE_CHILD_RECORD")
	}
	t.Log("ASSERT_CLOCKANCHOR_DELAYED_ROLLBACK_NO_AUTHORITY evaluated")
}

func clockAnchorSampler(t *testing.T, clock func() (applicationClockStamp, error)) {
	t.Helper()
	old := applicationClockNow
	applicationClockNow = clock
	t.Cleanup(func() { applicationClockNow = old })
}

func TestApplicationClockAnchorPublicProgression(t *testing.T) {
	cases := []struct {
		name        string
		delay, skew time.Duration
		ns          uint64
		refuse      bool
	}{
		{"genuine", 100 * time.Millisecond, 0, uint64(time.Second), false},
		{"small-rollback", 100 * time.Millisecond, -100 * time.Millisecond, uint64(time.Second), false},
		{"small-forward", 100 * time.Millisecond, 100 * time.Millisecond, uint64(time.Second), false},
		{"tolerance-rollback", 100 * time.Millisecond, -250 * time.Millisecond, uint64(time.Second), false},
		{"tolerance-forward", 100 * time.Millisecond, 250 * time.Millisecond, uint64(time.Second), false},
		{"outside-tolerance", 100 * time.Millisecond, -250*time.Millisecond - 1, uint64(time.Second), true},
		{"rollback30", 0, -30 * time.Second, uint64(time.Second), true},
		{"forward30", 0, 30 * time.Second, uint64(time.Second), true},
		{"delayed", 900 * time.Millisecond, 0, uint64(time.Second), false},
		{"elapsed-equality", time.Second, 0, uint64(time.Second), true},
		{"elapsed-over", 1100 * time.Millisecond, 0, uint64(time.Second), true},
		{"zero", 0, 0, 0, true},
		{"overflow", 0, 0, math.MaxUint64, true},
		{"over-policy", 0, 0, uint64(240*time.Second) + 1, true},
		{"large-valid", 100 * time.Millisecond, 0, uint64(239 * time.Second), false},
		{"serialized", 0, 0, uint64(time.Second), true},
		{"future-caller", 0, 0, uint64(time.Second), true},
		{"epoch", 0, 0, uint64(time.Second), true},
		{"empty-epoch", 0, 0, uint64(time.Second), true},
		{"future-kernel", 0, 0, uint64(time.Second), true},
		{"kernel-jump", 0, 0, uint64(time.Second), true},
		{"clock-error", 0, 0, uint64(time.Second), true},
		{"last-clock-error", 0, 0, uint64(time.Second), true},
		{"zero-kernel", 0, 0, uint64(time.Second), true},
		{"negative-kernel", 0, 0, uint64(time.Second), true},
		{"zero-wall", 0, 0, uint64(time.Second), true},
		{"missing-go-sample", 0, 0, uint64(time.Second), true},
		{"wall-negative", 0, 0, uint64(time.Second), true},
		{"wall-overflow", 0, 0, uint64(time.Second), true},
		{"expiry", 100 * time.Millisecond, 0, uint64(time.Second), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, create := applicationFixture(t)
			if tc.name == "expiry" {
				c.Grant.ExpiresUnixNano = time.Now().Add(500 * time.Millisecond).UnixNano()
				c.ApprovalSHA256 = ApplicationConfigDigest(c)
			}
			a := applicationCreate(t, c, create)
			u := applicationUnit(c, "clockanchor-"+tc.name)
			u.RevisionSHA256 = hash([]byte(c.RevisionAssertion))
			u.ScopeSHA256 = hash([]byte("scope"))
			root := filepath.Join(c.Store, "units", u.UnitID[7:])
			source := testConfig(t)
			source.Store = filepath.Join(root, "original")
			source.ScopeSHA256 = u.ScopeSHA256
			source.MaxCalls = 2
			source.Requested = 0
			source.ApprovalSHA256 = ""
			cohort, _ := cohortInputs(t)
			cohort.Store = filepath.Join(root, "cohort")
			cohort.SourceConfigSHA256 = ""
			cohort.Requested = 0
			now := time.Now()
			first := applicationClockStamp{Epoch: "controlled-actual-seam", WallNS: now.UnixNano(), MonotonicNS: int64(100 * time.Second), captured: now}
			last := first
			last.WallNS += int64(tc.delay + tc.skew)
			last.MonotonicNS += int64(tc.delay)
			last.captured = now.Add(tc.delay)
			caller := now
			switch tc.name {
			case "serialized":
				caller = now.Round(0)
			case "future-caller":
				caller = now.Add(time.Millisecond)
			case "epoch":
				last.Epoch = "other"
			case "empty-epoch":
				first.Epoch = ""
				last.Epoch = ""
			case "future-kernel":
				last.MonotonicNS--
			case "kernel-jump":
				last.MonotonicNS += int64(time.Second)
			case "zero-kernel":
				first.MonotonicNS, last.MonotonicNS = 0, 0
			case "negative-kernel":
				first.MonotonicNS, last.MonotonicNS = -1, -1
			case "zero-wall":
				last.WallNS = 0
			case "missing-go-sample":
				last.captured = last.captured.Round(0)
			case "wall-negative":
				last.WallNS = math.MinInt64
			case "wall-overflow":
				last.WallNS = math.MaxInt64
			}
			n := 0
			clockAnchorSampler(t, func() (applicationClockStamp, error) {
				n++
				if tc.name == "clock-error" || tc.name == "last-clock-error" && n > 1 {
					return applicationClockStamp{}, ErrInvalid
				}
				if n == 1 {
					return first, nil
				}
				return last, nil
			})
			b := applicationChild(2)
			b.ActiveNS = tc.ns
			err := a.AdmitUnitWithDerivation(u, b, source, cohort, caller)
			if tc.refuse {
				if err == nil {
					t.Fatal("ASSERT_CLOCKANCHOR_PUBLIC_INCONSISTENT_REFUSED")
				}
				if a.implementation.exists("unit", u.UnitID) {
					t.Fatal("ASSERT_CLOCKANCHOR_PUBLIC_REFUSAL_NO_RECORD")
				}
				// Neither delayed catch-up nor reboot can materialize refused authority.
				last = first
				last.MonotonicNS += int64(30100 * time.Millisecond)
				last.WallNS += int64(30100 * time.Millisecond)
				if tc.name == "rollback30" {
					last.WallNS -= int64(30 * time.Second)
				}
				if d, e := ReadApplicationUnitDeadline(c.Store, u.ApplicationSHA256, u.UnitID); e == nil && d.After(time.Now()) {
					t.Fatal("ASSERT_CLOCKANCHOR_PUBLIC_REFUSED_NEVER_REVIVES")
				}
				if VerifyApplicationUnitPrepayment(c.Store, u, b, source) == nil {
					t.Fatal("ASSERT_CLOCKANCHOR_REFUSED_PREPAYMENT_UNAVAILABLE")
				}
				return
			}
			if err != nil {
				t.Fatal("ASSERT_CLOCKANCHOR_GENUINE_ADMISSION", err)
			}
			var ledger applicationUnitLedger
			if e := a.implementation.object("unit", u.UnitID, &ledger); e != nil {
				t.Fatal(e)
			}
			if ledger.Clock.WallNS != last.WallNS || ledger.Clock.MonotonicNS != last.MonotonicNS || ledger.Clock.Epoch != last.Epoch {
				t.Fatal("ASSERT_CLOCKANCHOR_ACTUAL_PAIR_UNCHANGED")
			}
			duration := ledger.ActiveDeadlineUnixNano - ledger.Clock.WallNS
			if duration <= 0 || duration > int64(tc.ns)-int64(tc.delay) || ledger.ActiveDeadlineUnixNano > caller.UnixNano()+int64(tc.ns) || ledger.ActiveDeadlineUnixNano > c.Grant.ExpiresUnixNano {
				t.Fatal("ASSERT_CLOCKANCHOR_ACTUAL_CHARGED_INTERVAL_CAP")
			}
			if VerifyApplicationUnitPrepayment(c.Store, u, b, source) != nil {
				t.Fatal("ASSERT_CLOCKANCHOR_VALID_PUBLIC_PREPAYMENT")
			}
			for _, mode := range []string{"true-expiry-small-rollback", "forward", "reversal", "reboot", "future-start", "overflow"} {
				last = ledger.Clock
				last.MonotonicNS += int64(tc.ns) + int64(100*time.Millisecond)
				last.WallNS += int64(tc.ns)
				switch mode {
				case "forward":
					last.WallNS += int64(30 * time.Second)
				case "reversal":
					last.WallNS -= int64(30 * time.Second)
				case "reboot":
					last.Epoch = "reboot"
				case "future-start":
					last.MonotonicNS = ledger.Clock.MonotonicNS - 1
				case "overflow":
					last.WallNS = math.MinInt64
					last.MonotonicNS = math.MaxInt64
				}
				if d, e := ReadApplicationUnitDeadline(c.Store, u.ApplicationSHA256, u.UnitID); e == nil && d.After(time.Now()) {
					t.Fatal("ASSERT_CLOCKANCHOR_PUBLIC_EXPIRED_NEVER_REVIVES", mode)
				}
				if VerifyApplicationUnitPrepayment(c.Store, u, b, source) == nil {
					t.Fatal("ASSERT_CLOCKANCHOR_PUBLIC_EXPIRED_PREPAYMENT_REFUSED", mode)
				}
			}
		})
	}
}
