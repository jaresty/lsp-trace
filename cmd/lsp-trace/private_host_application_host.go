//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

type applicationRunTicket struct {
	ApplicationSHA256, CursorSHA256 string
	WaitReservationSHA256           string
	Sequence, StartGlobal           uint64
}
type applicationHostEvent struct {
	Event, ApplicationSHA256, RunID, UnitID, TerminalSHA256 string
	GlobalSequence                                          uint64
	Bundle                                                  *product.Bundle
}

func applicationTerminalPath(root, ticket string) string {
	return filepath.Join(root, "terminal-"+ticket[7:]+".json")
}
func applicationWriteOnce(path string, raw []byte) error {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return product.ErrInvalid
	}
	parent := filepath.Dir(path)
	before, e := os.Lstat(parent)
	if e != nil {
		return e
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return product.ErrInvalid
	}
	root, e := os.OpenRoot(parent)
	if e != nil {
		return e
	}
	defer root.Close()
	dir, e := root.OpenFile(".", os.O_RDONLY, 0)
	if e != nil {
		return e
	}
	defer dir.Close()
	held, e := dir.Stat()
	if e != nil || !os.SameFile(before, held) {
		return product.ErrInvalid
	}
	name := filepath.Base(path)
	// Existing destinations are never modified or retroactively enrolled.
	if _, se := root.Lstat(name); se == nil {
		return os.ErrExist
	} else if !os.IsNotExist(se) {
		return se
	}
	// The fixed create-only intent is a spent identity even if every later IO
	// fails. Sync it BEFORE making any final inode visible. One intent admits
	// at most one stage and one completion witness; retries cannot grow orphans.
	intent := applicationPublicationIntent(name)
	if e = root.Mkdir(intent, 0700); e != nil {
		return e
	}
	intentDir, ie := root.OpenFile(intent, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if ie != nil {
		return ie
	}
	ie = intentDir.Sync()
	intentClose := intentDir.Close()
	if ie != nil || intentClose != nil {
		return errors.Join(ie, intentClose)
	}
	if ie := dir.Sync(); ie != nil {
		return ie
	}
	var nonce [16]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return e
	}
	stage := ".application-" + name + "-" + hex.EncodeToString(nonce[:])
	if len(stage) > 255 {
		return product.ErrInvalid
	}
	f, e := root.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	// Never unlink a pathname on failure: even a checked inode can be replaced
	// between stat and unlink. Leave the single bounded, unadmitted orphan.
	// A duplicate descriptor retains the exclusive publication lock while the
	// writer's own Close is checked before publication. Readers never block.
	if e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		f.Close()
		return e
	}
	guardFD, e := unix.FcntlInt(f.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if e != nil {
		f.Close()
		return e
	}
	guard := os.NewFile(uintptr(guardFD), stage)
	defer guard.Close()
	n, e := f.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	// The final name does not exist until synced, closed bytes are ready. The
	// platform primitive refuses even a concurrent destination (including symlinks).
	if e = applicationPublishNoReplace(dir, stage, name); e != nil {
		return e
	}
	// The lock alone is not commit evidence: a crash or failed Sync releases it.
	// No completion witness is created on any required durability failure.
	if e = dir.Sync(); e != nil {
		return e
	}
	if e = dir.Close(); e != nil {
		return e
	}
	st, e := guard.Stat()
	if e != nil {
		return e
	}
	visible, e := root.Lstat(name)
	if e != nil || !os.SameFile(st, visible) || applicationFileIdentity(st) != applicationFileIdentity(visible) {
		return product.ErrInvalid
	}
	// This empty create-only witness records an ALREADY completed durability
	// boundary. It needs no further durability IO: losing this advisory witness
	// on a crash causes a false negative only, never admission of unsynced data.
	// Its exact name binds the target, held inode/metadata and payload digest.
	return root.Mkdir(filepath.Join(intent, applicationPublicationCompletion(name, st, raw)), 0700)
}
func applicationTicket(a *product.ApplicationJournal, root string, maxUnits int, calls ...int) (string, error) {
	unitCalls := 2
	if len(calls) > 1 {
		return "", product.ErrInvalid
	}
	if len(calls) == 1 {
		unitCalls = calls[0]
	}
	if unitCalls < 2 || unitCalls > 16 {
		return "", product.ErrInvalid
	}
	remaining := int((16 - a.Head().Reserved.Calls) / uint64(unitCalls))
	if maxUnits > remaining {
		maxUnits = remaining
	}
	if maxUnits < 0 {
		maxUnits = 0
	}
	waits := uint64(maxUnits*unitCalls + 2)
	// The existing metadata/work envelope includes the publication protocol:
	// per create-only target <=1 intent directory, <=1 stage/final inode, <=1
	// empty completion directory. Failure never allocates another stage for that
	// target. Admission adds <=4 fixed stats/read, never an enumeration.
	// Finalize partitions 128 records/slot from this SAME root-wide envelope.
	metadata := uint64(64<<20) + waits*(80<<20)
	id := pilot.Digest([]byte(fmt.Sprintf("%s/coordinator/%d", a.Head().ApplicationSHA256, a.Head().Sequence)))
	if _, e := a.ReserveOperation(id, "coordinator_host_envelope", product.ApplicationBounds{ReadHashBytes: metadata, WriteBytes: 16 << 20, Records: 16384 + waits*16384, ParserBytes: metadata, Work: 1 << 20, ActiveNS: uint64(10 * time.Second)}, time.Now()); e != nil {
		return "", e
	}
	if e := applicationPrepareFinalize(a, root, id); e != nil {
		return "", e
	}
	return a.PutRecord(applicationRunTicket{ApplicationSHA256: a.Head().ApplicationSHA256, CursorSHA256: a.Head().CursorSHA256, WaitReservationSHA256: id, Sequence: a.Head().Sequence, StartGlobal: a.Head().Reserved.Calls})
}

func applicationWaitUnitPath(root, ticket string, n int) string {
	return filepath.Join(root, fmt.Sprintf("host-wait-unit-%s-%02d.json", ticket[7:], n))
}

func applicationWaitChildDeadline(root, app, ticket string) (time.Time, error) {
	var latest time.Time
	for n := 1; n <= 16; n++ {
		raw, e := applicationReadBounded(applicationWaitUnitPath(root, ticket, n), 4096)
		if os.IsNotExist(e) {
			continue
		}
		var ref struct{ UnitID string }
		if e != nil || json.Unmarshal(raw, &ref) != nil || !validApplicationDigest(ref.UnitID) {
			return latest, product.ErrInvalid
		}
		deadline, e := product.ReadApplicationUnitDeadline(root, app, ref.UnitID)
		if e != nil {
			return latest, e
		}
		if deadline.After(latest) {
			latest = deadline
		}
	}
	return latest, nil
}

// Policy-active time is the sum of these prepaid owned wall-time slices, not
// measured child spend plus parent prepayment. Concurrent roles spend separate
// slots; no role, frontier or process receives a new ten-second allowance.
func applicationClaimWaitSlice(root, ticket string, claim []byte) error {
	for n := 0; n < 100; n++ {
		path := filepath.Join(root, fmt.Sprintf("host-wait-slice-%s-%03d.json", ticket[7:], n))
		if e := applicationWriteOnce(path, claim); e == nil {
			return nil
		} else if !os.IsExist(e) {
			return e
		}
	}
	return product.ErrPaused
}

func applicationRoleReadyPath(root, ticket, role string) string {
	return filepath.Join(root, "role-"+ticket[7:]+"-"+role+".json")
}
func applicationHostRoleIndex(role string) (int, error) {
	switch role {
	case "interpreter":
		return 0, nil
	case "reviewer":
		return 1, nil
	}
	return 0, product.ErrInvalid
}

func validApplicationDigest(id string) bool {
	return len(id) == 71 && len(id) > 7 && id[:7] == "sha256:" && func() bool {
		for _, c := range id[7:] {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
		return true
	}()
}

func applicationHostHelper(args []string, r hostApplicationRequest, stdin io.Reader, stdout, stderr io.Writer, lanes ...*applicationFinalizeLane) int {
	fail := func(s string) int { fmt.Fprintln(stderr, "private application HOST:", s); return 1 }
	if len(args) < 3 || !validApplicationDigest(args[2]) {
		return fail("INVALID_TICKET")
	}
	ticketID := args[2]
	var lane *applicationFinalizeLane
	if args[0] == "finalize" {
		if len(lanes) != 1 || lanes[0] == nil {
			return fail("FINALIZE_LANE_REQUIRED")
		}
		lane = lanes[0]
	}
	if lane.check() != nil {
		return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
	}
	policy, policyErr := applicationUnitEnvelope(r)
	if policyErr != nil {
		return fail("UNIT_POLICY_REFUSED")
	}
	unitCalls := uint64(policy.MaxCalls)
	var ticket applicationRunTicket
	read := applicationReadBounded
	if args[0] == "finalize" {
		if len(lanes) != 1 || lanes[0] == nil {
			return fail("FINALIZE_LANE_REQUIRED")
		}
		lane = lanes[0]
		read = lane.read
		var reservation product.OperationReservation
		if lane.object("data", ticketID, &ticket) != nil || lane.object("claim", ticket.WaitReservationSHA256, &reservation) != nil || reservation.EventID != ticket.WaitReservationSHA256 || reservation.Kind != "coordinator_host_envelope" || reservation.Bounds.ActiveNS != uint64(10*time.Second) || reservation.Sequence != ticket.Sequence {
			return fail("FINALIZE_TICKET_PREPAYMENT_REFUSED")
		}
	} else if product.ReadApplicationRecord(r.Configuration.Store, ticketID, &ticket) != nil {
		return fail("TICKET_REFUSED")
	}
	if lane.check() != nil {
		return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
	}
	if ticket.ApplicationSHA256 != applicationFinalizeConfigDigest(r.Configuration, lane) {
		return fail("TICKET_REFUSED")
	}
	switch args[0] {
	case "register":
		if len(args) != 6 {
			return fail("INVALID_READY")
		}
		role, id, prompt := args[3], args[4], args[5]
		idx, roleErr := applicationHostRoleIndex(role)
		if roleErr != nil {
			return fail("INVALID_ROLE")
		}
		if id == "" || len(id) > 256 || prompt != r.Configuration.RolePromptSHA256[idx] {
			return fail("INVALID_READY_BINDING")
		}
		raw, _ := json.Marshal(product.Ready{Protocol: product.CohortProtocol, Role: role, LaunchID: id, PromptSHA256: prompt})
		if applicationWriteOnce(applicationRoleReadyPath(r.Configuration.Store, ticketID, role), raw) != nil {
			return fail("READY_SPENT")
		}
		return 0
	case "respond":
		if len(args) != 5 {
			return fail("INVALID_RESPONSE")
		}
		role := args[3]
		global, e := strconv.ParseUint(args[4], 10, 64)
		if e != nil || global <= ticket.StartGlobal || global > 16 {
			return fail("INVALID_GLOBAL_SEQUENCE")
		}
		raw, e := io.ReadAll(io.LimitReader(stdin, 65538))
		if e != nil || len(raw) > 65537 {
			return fail("RESPONSE_BYTES_REFUSED")
		}
		for n := 1; n <= 16; n++ {
			var ac applicationActivation
			b, e := applicationReadBounded(filepath.Join(r.Configuration.Store, fmt.Sprintf("activation-%02d.json", n)), 4096)
			if e != nil {
				continue
			}
			if json.Unmarshal(b, &ac) != nil || !validApplicationDigest(ac.UnitID) || ac.Mailbox != filepath.Join(r.Configuration.Store, "units", ac.UnitID[7:], "cohort-mailbox") {
				return fail("FOREIGN_ACTIVATION")
			}
			if ac.Base <= 16-unitCalls && global > ac.Base && global <= ac.Base+unitCalls {
				if product.SubmitCohort(ac.Mailbox, role, global-ac.Base, raw) != nil {
					return fail("RESPONSE_REFUSED")
				}
				return 0
			}
		}
		return fail("GLOBAL_SEQUENCE_UNREGISTERED")
	case "wait":
		if len(args) != 6 {
			return fail("INVALID_WAIT")
		}
		role := args[3]
		after, e := strconv.ParseUint(args[4], 10, 64)
		ms, me := strconv.Atoi(args[5])
		if e != nil || me != nil || ms < 1 || ms > 270000 || after > 16 || role != "interpreter" && role != "reviewer" {
			return fail("INVALID_WAIT")
		}
		// One prepaid wait identity per role/global frontier. Repeated CLI
		// failures cannot restart an unmetered metadata-poll allowance.
		claimPath := filepath.Join(r.Configuration.Store, fmt.Sprintf("host-wait-%s-%s-%02d.json", ticketID[7:], role, after))
		claim, _ := json.Marshal(struct {
			RunID, Role string
			After       uint64
		}{ticketID, role, after})
		if applicationWriteOnce(claimPath, claim) != nil {
			return fail("WAIT_IDENTITY_SPENT")
		}
		var reservation product.OperationReservation
		if product.ReadApplicationOperation(r.Configuration.Store, ticket.WaitReservationSHA256, &reservation) != nil || reservation.Kind != "coordinator_host_envelope" || reservation.Bounds.ActiveNS != uint64(10*time.Second) {
			return fail("WAIT_PREPAYMENT_REFUSED")
		}
		deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
		expires := time.Unix(0, r.Configuration.Grant.ExpiresUnixNano)
		if expires.Before(deadline) {
			deadline = expires
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		event := func(kind string) int {
			return encodeHostProductV3(stdout, applicationHostEvent{Event: kind, ApplicationSHA256: ticket.ApplicationSHA256, RunID: ticketID})
		}
		deadlineEvent := func() int {
			if !time.Now().Before(expires) {
				return event("APPLICATION_EXPIRED")
			}
			return event("DEADLINE")
		}
		for {
			if ctx.Err() != nil {
				return deadlineEvent()
			}
			// All roles/frontiers/restarts draw from the SAME irrevocably prepaid
			// coordinator envelope. Each create-only slot buys at most 80ms of
			// owned polling AND sleep; quick returns do not refund unused time.
			childDeadline, windowErr := applicationWaitChildDeadline(r.Configuration.Store, ticket.ApplicationSHA256, ticketID)
			if windowErr != nil {
				return fail("WAIT_CHILD_WINDOW_REFUSED")
			}
			pollDeadline := time.Now().Add(100 * time.Millisecond)
			if childDeadline.After(time.Now()) {
				// Concurrent waiting during this child's orchestration is inside
				// its existing durable window, not an additional global grant.
				if childDeadline.Before(pollDeadline) {
					pollDeadline = childDeadline
				}
			} else if e := applicationClaimWaitSlice(r.Configuration.Store, ticketID, claim); e != nil {
				if errors.Is(e, product.ErrPaused) {
					return event("PAUSED_BUDGET")
				}
				return fail("WAIT_SLICE_PUBLICATION_FAILED")
			} else {
				// Preserve the finite 100-slot pool. Its 8s partition plus
				// the root-wide 2s finalize lane is the SAME prepaid 10s.
				pollDeadline = pollDeadline.Add(-20 * time.Millisecond)
			}
			pollCtx, pollCancel := context.WithDeadline(ctx, pollDeadline)
			defer pollCancel()
			raw, terminalErr := applicationReadBounded(applicationTerminalPath(r.Configuration.Store, ticketID), 64<<10)
			if terminalErr == nil {
				if !applicationValidTerminal(raw) {
					return fail("TERMINAL_REFUSED")
				}
				event := applicationHostEvent{Event: "TERMINAL", ApplicationSHA256: ticket.ApplicationSHA256, RunID: ticketID, TerminalSHA256: pilot.Digest(raw)}
				pollCancel()
				return encodeHostProductV3(stdout, event)
			}
			if !os.IsNotExist(terminalErr) {
				return fail("TERMINAL_REFUSED")
			}
			readyRaw, re := applicationReadBounded(applicationRoleReadyPath(r.Configuration.Store, ticketID, role), 4096)
			var ready product.Ready
			if re == nil && json.Unmarshal(readyRaw, &ready) == nil {
				for n := 1; n <= 16; n++ {
					if pollCtx.Err() != nil {
						break
					}
					var ac applicationActivation
					b, e := applicationReadBounded(filepath.Join(r.Configuration.Store, fmt.Sprintf("activation-%02d.json", n)), 4096)
					if e != nil {
						continue
					}
					if json.Unmarshal(b, &ac) != nil || !validApplicationDigest(ac.UnitID) || ac.Base > 16-unitCalls || ac.Base < ticket.StartGlobal || ac.Base+unitCalls <= after {
						continue
					}
					if ac.Mailbox != filepath.Join(r.Configuration.Store, "units", ac.UnitID[7:], "cohort-mailbox") {
						return fail("FOREIGN_ACTIVATION")
					}
					if ownReady, readyErr := applicationReadBounded(filepath.Join(ac.Mailbox, "ready-"+role+".json"), 4096); os.IsNotExist(readyErr) {
						if product.MarkCohortReady(ac.Mailbox, ready) != nil {
							return fail("READY_REFUSED")
						}
					} else if readyErr != nil || !bytes.Equal(ownReady, readyRaw) {
						return fail("READY_SUBSTITUTED")
					}
					local := uint64(0)
					if after > ac.Base {
						local = after - ac.Base
					}
					slice, sliceCancel := context.WithTimeout(pollCtx, 100*time.Millisecond)
					event, e := product.WaitCohort(slice, ac.Mailbox, role, local)
					sliceCancel()
					if e != nil {
						continue
					}
					if event.Event == "TERMINAL" {
						continue
					}
					if event.Bundle != nil {
						out := applicationHostEvent{Event: "NEEDS_HOST", ApplicationSHA256: ticket.ApplicationSHA256, RunID: ticketID, UnitID: ac.UnitID, GlobalSequence: ac.Base + event.Bundle.Sequence, Bundle: event.Bundle}
						pollCancel()
						return encodeHostProductV3(stdout, out)
					}
				}
			}
			<-pollCtx.Done()
			pollCancel()
			if ctx.Err() != nil {
				return deadlineEvent()
			}
		}
	case "finalize":
		if len(args) != 3 {
			return fail("INVALID_FINALIZE")
		}
		raw, e := lane.observations(stdin)
		var returns [2]product.CohortRoleReturn
		if e != nil || lane.check() != nil || len(raw) > 16384 || json.Unmarshal(raw, &returns) != nil {
			return fail("INVALID_HOST_OBSERVATIONS")
		}
		terminal, e := read(applicationTerminalPath(r.Configuration.Store, ticketID), 64<<10)
		if e != nil {
			return fail("TERMINAL_UNAVAILABLE")
		}
		if lane.check() != nil {
			return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
		}
		terminalSHA := pilot.Digest(terminal)
		var report applicationTerminal
		if lane.check() != nil || !applicationValidTerminal(terminal) || json.Unmarshal(terminal, &report) != nil {
			return fail("TERMINAL_REFUSED")
		}
		ids := map[string]bool{}
		kind := "HOST_OBSERVED_RETURNS_NOT_AUTHENTICATED"
		for n, role := range []string{"interpreter", "reviewer"} {
			b, e := read(applicationRoleReadyPath(r.Configuration.Store, ticketID, role), 4096)
			var ready product.Ready
			if e != nil || lane.check() != nil || json.Unmarshal(b, &ready) != nil {
				return fail("READY_UNAVAILABLE")
			}
			if lane.check() != nil {
				return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
			}
			ret := returns[n]
			if reason := applicationReturnReason(ret, ready, role, terminalSHA, ids[ret.LaunchID]); reason != "" {
				if lane.check() != nil {
					return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
				}
				return applicationRefusedReturn(lane.root, ticketID, n, ret, ready, role, terminalSHA, reason, ids[ret.LaunchID], stdout, stderr, lane)
			}
			ids[ret.LaunchID] = true
			if ret.Kind == "SCRIPTED_CONTRACT" {
				kind = "SCRIPTED_CONTRACT_RETURNS_NOT_AGENTS"
			}
		}
		view := struct {
			Schema, Status, ApplicationSHA256, RunID, TerminalSHA256, InventoryHead string
			Returns                                                                 [2]product.CohortRoleReturn
			Authority                                                               int
			Accepted                                                                bool
			SourceCompleteness                                                      string
		}{"private-application-host-observation.v1", kind, ticket.ApplicationSHA256, ticketID, terminalSHA, report.InventoryHead, returns, 0, false, "UNKNOWN"}
		if lane.check() != nil {
			return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
		}
		b, _ := json.Marshal(view)
		if lane.check() != nil {
			return fail("FINALIZE_SLICE_EXPIRED_CHARGED")
		}
		if applicationFinalizePublish(lane.root, "host-observation-"+ticketID[7:]+".json", b, true, lane) != nil {
			return fail("HOST_OBSERVATION_PUBLICATION_FAILED")
		}
		if lane.check() != nil {
			return fail("HOST_OBSERVATION_PUBLICATION_FAILED")
		}
		return encodeHostProductV3(stdout, view)
	}
	return fail("INVALID_HELPER")
}
