//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	ai "lsp-trace/internal/applicationinventory"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func isApplicationInventoryV3(raw []byte) bool {
	var h struct{ Schema string }
	return json.Unmarshal(raw, &h) == nil && h.Schema == "private-host-application-inventory-request.v3"
}

// Typed inputs are invocation-local. The CLI supplies nil; neither a serialized
// owner nor a process-global callback can produce a selected native result.
func runApplicationInventoryControllerV3(args []string, raw []byte, stdin io.Reader, out, errout io.Writer, inputs product.InventoryNativeInputsV3, startup *applicationStartupNotifier) int {
	fail := func(s string) int { fmt.Fprintln(errout, "private application inventory V3:", s); return 1 }
	q, e := product.DecodeInventoryRequestV3(raw)
	if e != nil {
		return fail("CONFIG_REFUSED")
	}
	if q.Inventory.MaxNativeAttempts == 0 {
		return fail("NATIVE_ACQUISITION_DISABLED_NO_EFFECTS")
	}
	if args[0] == "register" || args[0] == "wait" || args[0] == "respond" {
		return applicationInventoryHostV3(args, q, stdin, out, errout)
	}
	if args[0] != "setup" && args[0] != "run" && args[0] != "fail" {
		return fail("INVALID_OPERATION")
	}
	if args[0] == "setup" && len(args) != 2 || args[0] != "setup" && (len(args) != 3 || !validApplicationDigest(args[2])) {
		return fail("INVALID_INVOCATION")
	}
	var a *product.ApplicationJournal
	if _, e = os.Lstat(q.Configuration.Store); os.IsNotExist(e) && args[0] == "setup" {
		a, e = product.CreateApplication(q.Configuration, time.Now())
	} else {
		a, e = product.OpenApplication(q.Configuration, time.Now())
	}
	if e != nil {
		return fail("JOURNAL_ADMISSION_REFUSED")
	}
	defer a.Close()
	if args[0] == "setup" {
		ticket, e := applicationTicket(a, q.Configuration.Store, 1, 16)
		if e != nil {
			return fail("COORDINATOR_ENVELOPE_REFUSED")
		}
		mailbox := product.InventoryMailboxPathV3(q.Configuration.Store, ticket)
		if e = os.MkdirAll(filepath.Dir(mailbox), 0700); e != nil {
			return fail("MAILBOX_PARENT_REFUSED")
		}
		if e = product.PrepareMailbox(mailbox); e != nil {
			return fail("MAILBOX_REFUSED")
		}
		// Only activation metadata is shared with the fixed lifecycle protocol.
		if e = a.WriteActivation(1, applicationActivation{mailbox, ticket, 0}); e != nil {
			return fail("ACTIVATION_REFUSED")
		}
		type roleTask struct{ Role, Prompt, PromptSHA256 string }
		roles := []roleTask{}
		for _, role := range []string{"interpreter", "reviewer"} {
			p := product.InventoryRolePromptV3(role)
			roles = append(roles, roleTask{role, p, pilot.Digest([]byte(p))})
		}
		return encodeHostProductV3(out, struct {
			Schema, RunID string
			RoleTasks     []roleTask
		}{"private-application-inventory-host-setup.v3", ticket, roles})
	}
	runID := args[2]
	var ticket applicationRunTicket
	if a.GetRecord(runID, &ticket) != nil || ticket.ApplicationSHA256 != a.Head().ApplicationSHA256 {
		return fail("TICKET_REFUSED")
	}
	if args[0] == "fail" {
		recoveryErr := a.ReconcileInventoryReturnsV3(q, runID)
		return applicationInventoryTerminalV3(a, q, runID, ai.Report{}, "HOST_TOOL_UNAVAILABLE_COST_ONLY_NO_REPLAY", 1, out, errout, recoveryErr)
	}
	if ticket.CursorSHA256 != a.Head().CursorSHA256 {
		return fail("TICKET_CURSOR_REFUSED")
	}
	if _, e = a.ReserveOperation(pilot.Digest([]byte(runID+"/inventory-single-run")), "inventory_single_run.v3", product.ApplicationBounds{}, time.Now()); e != nil {
		return fail("RUN_TICKET_SPENT")
	}
	failed := func(reason string) int {
		return applicationInventoryTerminalV3(a, q, runID, ai.Report{}, "FAILED_INVENTORY_UNKNOWN", 1, out, errout, fmt.Errorf("%s", reason))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	deadline := time.Unix(0, q.Configuration.Grant.ExpiresUnixNano)
	if d := time.Now().Add(120 * time.Second); d.Before(deadline) {
		deadline = d
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	mailbox := product.InventoryMailboxPathV3(q.Configuration.Store, runID)
	if startup != nil && !startup.attempted {
		if e = startup.emit(ctx, applicationActivationPacket(a.Head().ApplicationSHA256, runID, runID, 1, applicationActivation{mailbox, runID, 0})); e != nil {
			return failed("STARTUP_NOTIFICATION_REFUSED")
		}
	}
	// Readiness waits use the existing finite coordinator pool, not a renewed
	// HOST/native window. No model, source or provider is launched by readiness.
	var transport *product.InventoryTransportV3
	for {
		if ctx.Err() != nil {
			return failed("CANCELLED_BEFORE_READY")
		}
		// Check both registrations before attempting irreversible V3 prepayment.
		ready := true
		for _, role := range []string{"interpreter", "reviewer"} {
			var r product.Ready
			b, re := applicationReadBounded(applicationRoleReadyPath(q.Configuration.Store, runID, role), 4096)
			if re != nil || json.Unmarshal(b, &r) != nil {
				ready = false
			}
		}
		if ready {
			transport, e = product.NewInventoryTransportV3(a, q, runID, mailbox)
			break
		}
		claim, _ := json.Marshal(struct{ RunID, Phase string }{runID, "V3_READY"})
		if e = applicationClaimWaitSlice(q.Configuration.Store, runID, claim); e != nil {
			return failed("READY_POOL_SPENT")
		}
		select {
		case <-ctx.Done():
			return failed("CANCELLED_BEFORE_READY")
		case <-time.After(80 * time.Millisecond):
		}
	}
	if e != nil {
		return applicationInventoryTerminalV3(a, q, runID, ai.Report{}, "FAILED_INVENTORY_UNKNOWN", 1, out, errout, e)
	}
	defer transport.Close()
	destination := filepath.Join(q.Configuration.Store, "inventory-result-"+runID[7:])
	request := q.Inventory.RunRequest(ctx, inputs, q.Configuration.SourceRoot, destination, transport.Binding.Ready[0].LaunchID, transport.Binding.Ready[1].LaunchID, transport)
	report, runErr := ai.Run(ctx, request)
	runErr = errors.Join(runErr, transport.ReconcilePhysicalReturnsV3())
	status, code := "PROVISIONAL", 0
	if runErr == nil && report.Published {
		_, e = a.PublishInventoryRunV3(q, runID, destination, report)
		runErr = errors.Join(runErr, e)
	}
	if runErr != nil || !report.Published {
		status, code = "FAILED_INVENTORY_UNKNOWN", 1
	} else if report.Stages["native_preparation"].Failed > 0 {
		status, code = "PROVISIONAL_WITH_PREPARATION_REFUSALS", 1
	}
	return applicationInventoryTerminalV3(a, q, runID, report, status, code, out, errout, runErr, transport.LastSequences())
}
