//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"strconv"
	"time"
)

func applicationInventoryTicketV3(q product.InventoryRequestV3, id string) (applicationRunTicket, error) {
	var t applicationRunTicket
	if !validApplicationDigest(id) || product.ReadApplicationRecord(q.Configuration.Store, id, &t) != nil || t.ApplicationSHA256 != product.ApplicationConfigDigest(q.Configuration) {
		return t, product.ErrInvalid
	}
	var r product.OperationReservation
	if product.ReadApplicationOperation(q.Configuration.Store, t.WaitReservationSHA256, &r) != nil || r.Kind != "coordinator_host_envelope" || r.Bounds.ActiveNS != uint64(10*time.Second) || r.Sequence != t.Sequence {
		return t, product.ErrInvalid
	}
	return t, nil
}
func applicationInventoryHostV3(args []string, q product.InventoryRequestV3, input io.Reader, out, errout io.Writer) int {
	fail := func(s string) int { fmt.Fprintln(errout, "private application inventory HOST V3:", s); return 1 }
	if len(args) < 3 {
		return fail("INVALID_TICKET")
	}
	runID := args[2]
	if _, e := applicationInventoryTicketV3(q, runID); e != nil {
		return fail("TICKET_REFUSED")
	}
	mailbox := product.InventoryMailboxPathV3(q.Configuration.Store, runID)
	switch args[0] {
	case "register":
		if len(args) != 6 {
			return fail("INVALID_READY")
		}
		role, id, prompt := args[3], args[4], args[5]
		idx, e := applicationHostRoleIndex(role)
		if e != nil || id == "" || len(id) > 256 || prompt != q.Configuration.RolePromptSHA256[idx] {
			return fail("INVALID_READY_BINDING")
		}
		ready := product.InventoryReadyV3{Ready: product.Ready{Protocol: product.InventoryProtocolV3, Role: role, LaunchID: id, PromptSHA256: prompt}, ReviewMode: q.Inventory.ReviewMode}
		if product.MarkInventoryReadyV3(mailbox, ready.Ready, ready.ReviewMode) != nil {
			return fail("READY_REFUSED")
		}
		raw, _ := json.Marshal(ready)
		if applicationWriteOnce(applicationRoleReadyPath(q.Configuration.Store, runID, role), raw) != nil {
			return fail("READY_SPENT")
		}
		return 0
	case "respond":
		if len(args) != 5 {
			return fail("INVALID_RESPONSE")
		}
		seq, e := strconv.ParseUint(args[4], 10, 64)
		if e != nil {
			return fail("INVALID_SEQUENCE")
		}
		raw, e := io.ReadAll(io.LimitReader(input, 65538))
		if e != nil || len(raw) > 65537 {
			return fail("RESPONSE_BYTES_REFUSED")
		}
		if product.SubmitInventoryV3(mailbox, args[3], seq, raw, q, runID) != nil {
			return fail("RESPONSE_REFUSED")
		}
		return 0
	case "wait":
		if len(args) != 6 {
			return fail("INVALID_WAIT")
		}
		role := args[3]
		if _, e := applicationHostRoleIndex(role); e != nil {
			return fail("INVALID_ROLE")
		}
		after, e := strconv.ParseUint(args[4], 10, 64)
		ms, me := strconv.Atoi(args[5])
		if e != nil || me != nil || after > 16 || ms < 1 || ms > 270000 {
			return fail("INVALID_WAIT")
		}
		claim, _ := json.Marshal(struct {
			RunID, Role string
			After       uint64
		}{runID, role, after})
		claimPath := q.Configuration.Store + fmt.Sprintf("/host-wait-%s-%s-%02d.json", runID[7:], role, after)
		if applicationWriteOnce(claimPath, claim) != nil {
			return fail("WAIT_IDENTITY_SPENT")
		}
		deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
		expires := time.Unix(0, q.Configuration.Grant.ExpiresUnixNano)
		if expires.Before(deadline) {
			deadline = expires
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		event := func(kind string) int {
			return encodeHostProductV3(out, applicationHostEvent{Event: kind, ApplicationSHA256: product.ApplicationConfigDigest(q.Configuration), RunID: runID})
		}
		for {
			if ctx.Err() != nil {
				if !time.Now().Before(expires) {
					return event("APPLICATION_EXPIRED")
				}
				return event("DEADLINE")
			}
			if applicationClaimWaitSlice(q.Configuration.Store, runID, claim) != nil {
				return event("PAUSED_BUDGET")
			}
			terminal, e := applicationReadBounded(applicationTerminalPath(q.Configuration.Store, runID), 64<<10)
			if e == nil {
				if !applicationInventoryValidTerminalV3(terminal) {
					return fail("TERMINAL_REFUSED")
				}
				return encodeHostProductV3(out, applicationHostEvent{Event: "TERMINAL", ApplicationSHA256: product.ApplicationConfigDigest(q.Configuration), RunID: runID, TerminalSHA256: applicationInventoryDigestV3(terminal)})
			}
			if !os.IsNotExist(e) {
				return fail("TERMINAL_REFUSED")
			}
			slice, stop := context.WithTimeout(ctx, 80*time.Millisecond)
			v, e := product.WaitInventoryV3(slice, mailbox, role, after, q, runID)
			waitErr := slice.Err()
			stop()
			if e == nil && v.Bundle != nil {
				return encodeHostProductV3(out, applicationHostEvent{Event: "NEEDS_HOST", ApplicationSHA256: product.ApplicationConfigDigest(q.Configuration), RunID: runID, UnitID: runID, GlobalSequence: v.Bundle.Sequence, Bundle: v.Bundle})
			}
			if e != nil && waitErr == nil {
				return fail("FRAME_REFUSED")
			}
		}
	}
	return fail("INVALID_HELPER")
}
