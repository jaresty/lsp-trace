//go:build darwin || linux

package privatehostproductv3

import (
	"math"
	"time"

	"golang.org/x/sys/unix"
)

// The boot epoch and kernel monotonic domain survive process restart, not reboot.
// Neither these local observations nor writable same-UID files authenticate time.
// Clock discontinuity or unavailable epoch/clock fails closed; no automatic replay.
type applicationClockStamp struct {
	Epoch               string
	WallNS, MonotonicNS int64
	// Live process-monotonic observation; deliberately never serialized.
	captured time.Time
}

// One production seam for admission and public deadline/prepayment reads. Tests
// replace it only serially; no external clock/configuration override is exposed.
var applicationClockNow = applicationSystemClockNow

func applicationSystemClockNow() (applicationClockStamp, error) {
	epoch, e := applicationBootEpoch()
	if e != nil || epoch == "" {
		return applicationClockStamp{}, ErrInvalid
	}
	var ts unix.Timespec
	if e = unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); e != nil {
		return applicationClockStamp{}, e
	}
	captured := time.Now()
	return applicationClockStamp{Epoch: epoch, WallNS: captured.UnixNano(), MonotonicNS: ts.Nano(), captured: captured}, nil
}

const applicationClockTolerance = int64(250 * time.Millisecond)

// A serialized wall-only caller supplies no trustworthy elapsed-time anchor.
// Require live Go monotonic time, never reconstruct a kernel coordinate from
// wall displacement. Refusing ambiguity creates no PREPAY_CHILD object.
func applicationAdmissionDeadline(caller time.Time, ns uint64, expires int64, first, last applicationClockStamp) (int64, error) {
	if ns == 0 || ns > uint64(240*time.Second) || caller == caller.Round(0) || first.captured == first.captured.Round(0) || last.captured == last.captured.Round(0) || first.Epoch == "" || last.Epoch != first.Epoch || first.MonotonicNS <= 0 || last.MonotonicNS < first.MonotonicNS || first.WallNS <= 0 || last.WallNS <= 0 {
		return 0, ErrInvalid
	}
	elapsed := last.captured.Sub(caller)
	processing := last.captured.Sub(first.captured)
	kernel := last.MonotonicNS - first.MonotonicNS
	if elapsed < 0 || processing < 0 || elapsed < processing || elapsed >= time.Duration(ns) || kernel > math.MaxInt64-applicationClockTolerance || kernel-processing.Nanoseconds() > applicationClockTolerance || processing.Nanoseconds()-kernel > applicationClockTolerance {
		return 0, ErrInvalid
	}
	wall := last.WallNS - caller.UnixNano()
	if caller.UnixNano() <= 0 || !time.Unix(0, caller.UnixNano()).Equal(caller) || wall < elapsed.Nanoseconds()-applicationClockTolerance || wall > elapsed.Nanoseconds()+applicationClockTolerance {
		return 0, ErrInvalid
	}
	// Charge admission work in the actual elapsed domain, including sampling.
	// The kernel processing lower bound can only shorten, never extend it.
	if kernel > elapsed.Nanoseconds() {
		elapsed = time.Duration(kernel)
	}
	if elapsed >= time.Duration(ns) {
		return 0, ErrInvalid
	}
	remaining := int64(ns) - elapsed.Nanoseconds()
	if last.WallNS > math.MaxInt64-remaining {
		return 0, ErrInvalid
	}
	deadline := last.WallNS + remaining
	callerDeadline := applicationUnitDeadline(caller, ns, expires)
	if callerDeadline <= 0 {
		return 0, ErrInvalid
	}
	if callerDeadline < deadline {
		deadline = callerDeadline
	}
	if deadline <= last.WallNS {
		return 0, ErrInvalid
	}
	return deadline, nil
}

func applicationIntervalRemaining(u applicationUnitLedger, current applicationClockStamp) (time.Duration, error) {
	start := u.Clock
	if start.Epoch == "" || current.Epoch != start.Epoch || start.MonotonicNS <= 0 || current.MonotonicNS < start.MonotonicNS || start.WallNS <= 0 || current.WallNS <= 0 || u.ActiveDeadlineUnixNano <= start.WallNS || u.Allowance.ActiveNS == 0 || u.Allowance.ActiveNS > uint64(240*time.Second) {
		return 0, ErrInvalid
	}
	elapsed := current.MonotonicNS - start.MonotonicNS
	wallElapsed := current.WallNS - start.WallNS
	// Small sampling/slew differences cannot mint authority: monotonic remaining
	// always caps it. Larger forward/backward jumps conservatively refuse reuse.
	if elapsed > math.MaxInt64-applicationClockTolerance || wallElapsed < elapsed-applicationClockTolerance || wallElapsed > elapsed+applicationClockTolerance {
		return 0, ErrInvalid
	}
	duration := u.ActiveDeadlineUnixNano - start.WallNS
	if duration > int64(u.Allowance.ActiveNS) {
		return 0, ErrInvalid
	}
	remaining := duration - elapsed
	if wall := u.ActiveDeadlineUnixNano - current.WallNS; wall < remaining {
		remaining = wall
	}
	if remaining < 0 {
		remaining = 0
	}
	return time.Duration(remaining), nil
}
