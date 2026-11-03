//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

const applicationFinalizeAttempts = 16
const applicationFinalizeSlice = 125 * time.Millisecond

// This is a partition of coordinator_host_envelope, never an added reservation:
// 16*(512 KiB read/parser, 128 records, 4096 work, 64 KiB write, 125ms).
// The 128-record partition includes fixed intent/completion/name observations
// (at most four admission stats plus one identity recheck/read), not another allowance.
// It fits its existing 64MiB metadata base, 16MiB writes, 16384 records,
// 1Mi work and 10s active slice. Remaining wait-only active time is 8s.
// Calls/request/response/wire/output semantic currencies remain unchanged.
type applicationFinalizePolicy struct {
	Schema, ApplicationSHA256, ReservationSHA256 string
	Attempts                                     int
	SliceNS                                      int64
	ExpiresUnixNano                              int64
}

func applicationPrepareFinalize(a *product.ApplicationJournal, path, id string) error {
	root, e := applicationFinalizeOpenRoot(path)
	if e != nil {
		return e
	}
	defer root.Close()
	// Read the approved, already-admitted header, never the caller's expiry.
	header, e := applicationFinalizeRead(root, "header.json", 64<<10)
	var config product.ApplicationConfig
	if e != nil || json.Unmarshal(header, &config) != nil || product.ApplicationConfigDigest(config) != a.Head().ApplicationSHA256 {
		return product.ErrInvalid
	}
	policy := applicationFinalizePolicy{"private-application-finalize-attempts.v1", a.Head().ApplicationSHA256, id, applicationFinalizeAttempts, int64(applicationFinalizeSlice), config.Grant.ExpiresUnixNano}
	prior, e := applicationFinalizeRead(root, "host-finalize-policy.json", 4096)
	if e == nil {
		var p applicationFinalizePolicy
		if json.Unmarshal(prior, &p) != nil || p.Schema != policy.Schema || p.ApplicationSHA256 != policy.ApplicationSHA256 || p.Attempts != policy.Attempts || p.SliceNS != policy.SliceNS || p.ExpiresUnixNano != policy.ExpiresUnixNano || !validApplicationDigest(p.ReservationSHA256) {
			return product.ErrInvalid
		}
		return applicationFinalizePublish(root, "host-finalize-policy.json", prior, true)
	}
	if !os.IsNotExist(e) {
		return e
	}
	raw, _ := json.Marshal(policy)
	return applicationFinalizePublish(root, "host-finalize-policy.json", raw, false)
}

// Entry bootstrap is deliberately narrow and repeatable: <=64KiB private request
// locator, <=4096-byte fixed policy, <=16 exclusive-create collisions and one
// fixed refusal. It does not hash, parse operands, read ticket/header/terminal or
// emit a diagnostic. Arbitrary API invocations are not zero-cost authenticated
// calls; this bootstrap is not an application/semantic authorization claim.
// A root-wide pool prevents run aliases, roles, reasons and new tickets from
// replenishing attempts. Existing empty/torn claims are spent forever.
func applicationFinalizeDispatch(args []string, stdin io.Reader, out, errout io.Writer) int {
	fail := func(s string) int { fmt.Fprintln(errout, "private application HOST:", s); return 1 }
	if len(args) != 3 || !validApplicationDigest(args[2]) {
		return fail("INVALID_FINALIZE")
	}
	raw, e := applicationReadLocatorBounded(args[1], 64<<10)
	if e != nil {
		return fail("FINALIZE_LOCATOR_REFUSED")
	}
	var locator struct{ Configuration struct{ Store string } }
	if json.Unmarshal(raw, &locator) != nil {
		return fail("FINALIZE_LOCATOR_REFUSED")
	}
	root, e := applicationFinalizeOpenRoot(locator.Configuration.Store)
	if e != nil {
		return fail("FINALIZE_ROOT_REFUSED")
	}
	defer root.Close()
	pRaw, e := applicationFinalizeRead(root, "host-finalize-policy.json", 4096)
	var p applicationFinalizePolicy
	if e != nil || json.Unmarshal(pRaw, &p) != nil || p.Schema != "private-application-finalize-attempts.v1" || p.Attempts != applicationFinalizeAttempts || p.SliceNS != int64(applicationFinalizeSlice) || p.ExpiresUnixNano <= 0 || !validApplicationDigest(p.ApplicationSHA256) || !validApplicationDigest(p.ReservationSHA256) {
		return fail("FINALIZE_POLICY_REFUSED")
	}
	claimed := false
	start := time.Now()
	deadline := start.Add(applicationFinalizeSlice)
	claimLane := &applicationFinalizeLane{root: root, deadline: deadline}
	// Project the approved absolute expiry onto this process's monotonic clock.
	// Keep the wall expiry too: a forward wall-clock jump can only shorten it.
	expires := time.Unix(0, p.ExpiresUnixNano)
	appDeadline := start.Add(expires.Sub(start))
	if appDeadline.Before(deadline) {
		deadline = appDeadline
	}
	lane := &applicationFinalizeLane{root: root, deadline: deadline, expires: expires}
	for n := 0; n < applicationFinalizeAttempts; n++ {
		name := fmt.Sprintf("host-finalize-attempt-%02d.json", n)
		e = applicationFinalizeCharge(root, name, claimLane, lane)
		if e == nil {
			claimed = true
			break
		}
		if !os.IsExist(e) && !time.Now().Before(expires) {
			return fail("FINALIZE_APPLICATION_EXPIRED_CHARGED")
		}
		if claimLane.check() != nil {
			return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
		}
		if !os.IsExist(e) {
			return fail("FINALIZE_ATTEMPT_SPENT_IO_FAILURE")
		}
	}
	if !claimed {
		return fail("FINALIZE_ATTEMPTS_EXHAUSTED")
	}
	// Every subsequent failure consumes the claim. Do not refund on expiry,
	// config/ticket substitution, parser failures or publication/synchronization.
	if !time.Now().Before(expires) {
		return fail("FINALIZE_APPLICATION_EXPIRED_CHARGED")
	}
	if lane.check() != nil {
		return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
	}
	r, e := decodeHostApplicationRaw(raw, true, lane)
	if lane.check() != nil {
		return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
	}
	if e != nil || r.Configuration.Store != locator.Configuration.Store || applicationFinalizeConfigDigest(r.Configuration, lane) != p.ApplicationSHA256 {
		return fail("FINALIZE_CONFIG_REFUSED_CHARGED")
	}
	header, e := applicationFinalizeRead(root, "header.json", 64<<10, lane)
	if lane.check() != nil {
		return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
	}
	canonical, _ := json.Marshal(r.Configuration)
	if e != nil || !bytes.Equal(header, canonical) {
		return fail("FINALIZE_APPLICATION_REFUSED_CHARGED")
	}
	if !time.Now().Before(time.Unix(0, r.Configuration.Grant.ExpiresUnixNano)) {
		return fail("FINALIZE_APPLICATION_EXPIRED_CHARGED")
	}
	// Verify the actual prepaid operation under the pinned root, not two copies
	// supplied by the caller. The pool draws only from its first reservation.
	var reservation product.OperationReservation
	if lane.object("claim", p.ReservationSHA256, &reservation) != nil || reservation.EventID != p.ReservationSHA256 || reservation.Kind != "coordinator_host_envelope" || reservation.Bounds.ActiveNS != uint64(10*time.Second) || reservation.Bounds.ReadHashBytes < 64<<20 || reservation.Bounds.ParserBytes < 64<<20 || reservation.Bounds.WriteBytes < 16<<20 || reservation.Bounds.Records < 16384 || reservation.Bounds.Work < 1<<20 {
		return fail("FINALIZE_PREPAYMENT_REFUSED_CHARGED")
	}
	if !time.Now().Before(deadline) {
		return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
	}
	return applicationHostHelper(args, r, stdin, out, errout, lane)
}

type applicationFinalizeLane struct {
	root     *os.Root
	deadline time.Time
	expires  time.Time
}

func (l *applicationFinalizeLane) check() error {
	if l != nil && (!time.Now().Before(l.deadline) || !l.expires.IsZero() && !time.Now().Before(l.expires)) {
		return product.ErrPaused
	}
	return nil
}
func applicationFinalizeCheck(lanes ...*applicationFinalizeLane) error {
	for _, l := range lanes {
		if e := l.check(); e != nil {
			return e
		}
	}
	return nil
}

// Gate each Read issued by io.ReadAll as well as the completion of that IO.
// No goroutines or claims of cancelling synchronous kernel work are involved.
type applicationFinalizeReader struct {
	input io.Reader
	lane  *applicationFinalizeLane
}

func (r applicationFinalizeReader) Read(b []byte) (int, error) {
	if e := r.lane.check(); e != nil {
		return 0, e
	}
	n, e := r.input.Read(b)
	if ce := r.lane.check(); ce != nil {
		return n, ce
	}
	return n, e
}

func (l *applicationFinalizeLane) read(path string, limit int) ([]byte, error) {
	if e := l.check(); e != nil {
		return nil, e
	}
	return applicationFinalizeRead(l.root, filepath.Base(path), limit, l)
}

// A caller-owned pipe must use the same prepaid deadline; no detached reader
// goroutine or extra wait window is created. Buffered test inputs cannot block.
func (l *applicationFinalizeLane) observations(input io.Reader) ([]byte, error) {
	if e := l.check(); e != nil {
		return nil, e
	}
	switch f := input.(type) {
	case *os.File:
		st, e := f.Stat()
		if ce := l.check(); ce != nil {
			return nil, ce
		}
		if e != nil {
			return nil, e
		}
		if !st.Mode().IsRegular() {
			if e := f.SetReadDeadline(l.deadline); e != nil {
				return l.pollObservations(f)
			}
			defer f.SetReadDeadline(time.Time{})
		}
	case *bytes.Reader, *strings.Reader:
	default:
		return nil, product.ErrInvalid
	}
	return io.ReadAll(io.LimitReader(applicationFinalizeReader{input, l}, 16385))
}

// Inherited stdin may not be registered with Go's poller. Duplicate its owned
// descriptor, preserve/restore its flags, and poll against the SAME deadline.
// Reads are nonblocking; no retry creates a fresh interval.
func (l *applicationFinalizeLane) pollObservations(f *os.File) ([]byte, error) {
	if e := l.check(); e != nil {
		return nil, e
	}
	conn, e := f.SyscallConn()
	if e != nil {
		return nil, e
	}
	fd, flags := -1, 0
	if ce := l.check(); ce != nil {
		return nil, ce
	}
	e = conn.Control(func(raw uintptr) {
		if e = l.check(); e != nil {
			return
		}
		flags, e = unix.FcntlInt(raw, unix.F_GETFL, 0)
		if e == nil {
			e = l.check()
		}
		if e == nil {
			fd, e = unix.Dup(int(raw))
		}
	})
	if e != nil || fd < 0 {
		return nil, product.ErrInvalid
	}
	defer unix.Close(fd)
	if ce := l.check(); ce != nil {
		return nil, ce
	}
	if e = unix.SetNonblock(fd, true); e != nil {
		return nil, e
	}
	defer unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags)
	buf := make([]byte, 16385)
	n := 0
	for n < len(buf) {
		if ce := l.check(); ce != nil {
			return nil, ce
		}
		remaining := time.Until(l.deadline)
		if remaining <= 0 {
			return nil, product.ErrPaused
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, pe := unix.Poll(poll, int((remaining+time.Millisecond-1)/time.Millisecond))
		if pe == unix.EINTR {
			continue
		}
		if pe != nil {
			return nil, pe
		}
		if ready == 0 {
			continue
		}
		if ce := l.check(); ce != nil {
			return nil, ce
		}
		got, re := unix.Read(fd, buf[n:])
		if ce := l.check(); ce != nil {
			return nil, ce
		}
		if re == unix.EAGAIN || re == unix.EWOULDBLOCK || re == unix.EINTR {
			continue
		}
		if re != nil {
			return nil, re
		}
		if got == 0 {
			return buf[:n], nil
		}
		n += got
	}
	return buf[:n], nil
}

func (l *applicationFinalizeLane) object(kind, id string, out any) error {
	if e := l.check(); e != nil {
		return e
	}
	if !validApplicationDigest(id) {
		return product.ErrInvalid
	}
	raw, e := applicationFinalizeRead(l.root, "objects/"+id[7:9]+"/"+kind+"-"+id+".json", 4096, l)
	if e != nil {
		return e
	}
	if e = l.check(); e != nil {
		return e
	}
	if kind == "data" && pilot.Digest(raw) != id {
		return product.ErrInvalid
	}
	if e = l.check(); e != nil {
		return e
	}
	if json.Unmarshal(raw, out) != nil {
		return product.ErrInvalid
	}
	return l.check()
}
