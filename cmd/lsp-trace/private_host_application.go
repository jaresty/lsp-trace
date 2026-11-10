//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

type hostApplicationRequest struct {
	Schema            string
	Configuration     product.ApplicationConfig
	Template          hostProductRequestV3
	MaxUnitsPerRun    int
	UnitResponseBytes int
	UnitTimeoutMS     int
	UnitEnvelope      *hostApplicationUnitEnvelope `json:",omitempty"`
	ApprovalSHA256    string
}

func hostApplicationUnitPolicyDigest(r hostApplicationRequest, lanes ...*applicationFinalizeLane) string {
	return applicationFinalizeDigest(struct {
		Template                         hostProductRequestV3
		UnitResponseBytes, UnitTimeoutMS int
		UnitEnvelope                     *hostApplicationUnitEnvelope `json:",omitempty"`
	}{r.Template, r.UnitResponseBytes, r.UnitTimeoutMS, r.UnitEnvelope}, lanes...)
}
func hostApplicationDigest(r hostApplicationRequest, lanes ...*applicationFinalizeLane) string {
	r.ApprovalSHA256 = ""
	return applicationFinalizeDigest(r, lanes...)
}

func applicationFinalizeDigest(value any, lanes ...*applicationFinalizeLane) string {
	if applicationFinalizeCheck(lanes...) != nil {
		return ""
	}
	raw, e := json.Marshal(value)
	if e != nil || applicationFinalizeCheck(lanes...) != nil {
		return ""
	}
	return pilot.Digest(raw)
}
func applicationFinalizeConfigDigest(c product.ApplicationConfig, lanes ...*applicationFinalizeLane) string {
	c.ApprovalSHA256 = ""
	return applicationFinalizeDigest(c, lanes...)
}
func readHostApplication(path string) (hostApplicationRequest, error) {
	return decodeHostApplication(path, true)
}
func decodeHostApplication(path string, validateTemplate bool) (hostApplicationRequest, error) {
	var r hostApplicationRequest
	raw, e := readHostProductRequestRawV3(path)
	if e != nil {
		return r, e
	}
	return decodeHostApplicationRaw(raw, validateTemplate)
}

func decodeHostApplicationRaw(raw []byte, validateTemplate bool, lanes ...*applicationFinalizeLane) (hostApplicationRequest, error) {
	var r hostApplicationRequest
	gate := func() error { return applicationFinalizeCheck(lanes...) }
	if e := gate(); e != nil {
		return r, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil {
		return r, product.ErrInvalid
	}
	if e := gate(); e != nil {
		return r, e
	}
	if d.Decode(new(any)) != io.EOF {
		return r, product.ErrInvalid
	}
	if e := gate(); e != nil {
		return r, e
	}
	canonical, _ := json.Marshal(r)
	if e := gate(); e != nil {
		return r, e
	}
	if _, e := applicationChildAllowance(r, r.Template); e != nil {
		return r, e
	}
	// The fixed root/header lane is bounded. Deeper template verification runs
	// only AFTER OpenApplication durably consumes the stored bootstrap attempt.
	if !validateTemplate {
		if len(raw) > 64<<10 || !bytes.Equal(raw, canonical) {
			return r, product.ErrInvalid
		}
		return r, nil
	}
	if e := gate(); e != nil {
		return r, e
	}
	o, e := parseCensusCLIOptions(r.Template.CensusArguments)
	if ce := gate(); ce != nil {
		return r, ce
	}
	requestDigest := hostApplicationDigest(r, lanes...)
	if ce := gate(); ce != nil {
		return r, ce
	}
	configDigest := applicationFinalizeConfigDigest(r.Configuration, lanes...)
	if ce := gate(); ce != nil {
		return r, ce
	}
	unitDigest := hostApplicationUnitPolicyDigest(r, lanes...)
	if ce := gate(); ce != nil {
		return r, ce
	}
	if !bytes.Equal(raw, canonical) || r.Schema != "private-host-application-request.v1" || r.ApprovalSHA256 != requestDigest || r.Configuration.ApprovalSHA256 != configDigest || r.Configuration.UnitPolicySHA256 != unitDigest || r.Configuration.ProducerConfigSHA256 != unitDigest || r.Configuration.SourceRoot != o.Workspace || r.Configuration.RevisionAssertion != r.Template.RevisionAssertion || r.Template.RuntimeGrantSHA256 != "" || r.Template.Configuration.ApprovalSHA256 != "" || r.Template.Configuration.Requested != 0 || r.Template.Configuration.ScopeSHA256 != "" || r.Template.Configuration.ProducerConfigSHA256 != "" || r.MaxUnitsPerRun < 1 || r.MaxUnitsPerRun > 16 || r.UnitResponseBytes < 1024 || r.UnitResponseBytes > 8192 || e != nil || len(o.Sources) != 1 || o.Sources[0] != "." || !o.PrivateEnumerationProjection || r.Template.ExchangeTimeoutMS > 60000 || r.Template.OperationTimeoutMS < 1 || r.Template.OperationTimeoutMS > 240000 || r.Configuration.ProducerExecutableSHA256 != r.Template.ServerSHA256 {
		return r, product.ErrInvalid
	}
	if ce := gate(); ce != nil {
		return r, ce
	}
	if hostProductLexicalRelatedV3(r.Configuration.SourceRoot, r.Configuration.Store) {
		return r, product.ErrInvalid
	}
	return r, nil
}

type applicationState struct {
	Schema, ApplicationSHA256                    string
	Directory, DirectoryStack, Selected          string
	DirectoryIdentity                            string
	DiscoveryFinished                            bool
	Entries, SelectedCount, Excluded, SourceGaps uint64
	CatalogBytes, ViewBytes                      uint64
	Units, Targets                               int
	Inflight                                     string
	CohortRecovery                               string
	LastPage                                     string
	Activation                                   int
	ChildPolicyConsumed                          pilot.PageCostPrivateV3
	LiteralDiscovery                             applicationLiteralDiscovery `json:",omitzero"`
}
type applicationDirectory struct{ Path, Next, Identity string }
type applicationCatalogEntry = product.ApplicationCatalogEntry
type applicationActivation struct {
	Mailbox, UnitID string
	Base            uint64
}
type applicationTerminal struct {
	Schema, Status, Accounting, SourceCompleteness, InventoryHead string
	Units, Targets, Calls, RequestBytes, ResponseBytes            int
	Discovered, Selected, Excluded, SourceGaps                    uint64
	CatalogBytes, ViewBytes                                       uint64
	KnownOwnedWork                                                product.ApplicationBounds
	Denominator                                                   string
	Reserved                                                      product.ApplicationBounds
	OwnedIO                                                       product.ApplicationIO
	ChildLedgerPolicyDebits                                       pilot.PageCostPrivateV3
	ReservationToKnownOwnedRatio                                  string
	ExcludedAccounting                                            string
	Authority                                                     int
	Accepted                                                      bool
	FeatureIdentity, AbsenceInference                             string
	SelectionDomain                                               string                      `json:",omitempty"`
	UnvisitedExtent                                               string                      `json:",omitempty"`
	LiteralDiscovery                                              applicationLiteralDiscovery `json:",omitzero"`
}

func applicationTerminalFor(a *product.ApplicationJournal, s applicationState, status string) applicationTerminal {
	h := a.Head()
	denominator := "UNKNOWN"
	if s.DiscoveryFinished {
		denominator = fmt.Sprintf("%d", s.SelectedCount)
	}
	terminal := applicationTerminal{Schema: "private-host-application-terminal.v1", Status: status, Accounting: "KNOWN_LOWER_BOUND_NOT_EXACT_WHOLE_HISTORY_OR_AUTHENTICATION", SourceCompleteness: "UNKNOWN", InventoryHead: h.LastCompletePublicationSHA256, Units: s.Units, Targets: s.Targets, Calls: int(h.Observed.Calls), RequestBytes: int(h.Observed.RequestBytes), ResponseBytes: int(h.Observed.ResponseBytes), Discovered: s.Entries, Selected: s.SelectedCount, Excluded: s.Excluded, SourceGaps: s.SourceGaps, Denominator: denominator, Reserved: h.Reserved, OwnedIO: a.IO(), CatalogBytes: s.CatalogBytes, ViewBytes: s.ViewBytes, KnownOwnedWork: h.Observed, ChildLedgerPolicyDebits: s.ChildPolicyConsumed, ReservationToKnownOwnedRatio: fmt.Sprintf("read-policy/known-owned=%d/%d;write-policy/known-owned=%d/%d;active-policy/exchange-elapsed-ns=%d/%d", h.Reserved.ReadHashBytes, h.Observed.ReadHashBytes, h.Reserved.WriteBytes, h.Observed.WriteBytes, h.Reserved.ActiveNS, h.Observed.ActiveNS), ExcludedAccounting: "provider internals CPU/IO; HOST outer framing; model billing; actual HOST cleanup/authentication; reservations and child usage are not added", FeatureIdentity: "UNRESOLVED", AbsenceInference: "PROHIBITED"}
	if s.LiteralDiscovery.RootIdentity != "" {
		terminal.SelectionDomain = "APPROVED_SELECTION_DOMAIN_ONLY"
		terminal.UnvisitedExtent = "UNKNOWN_OUTSIDE_APPROVED_SELECTION; EXCLUDED_COUNTS_OBSERVED_ONLY"
		terminal.LiteralDiscovery = s.LiteralDiscovery
	}
	return terminal
}
func saveApplicationState(a *product.ApplicationJournal, s applicationState) error {
	id, e := a.PutRecord(s)
	if e != nil {
		return e
	}
	return a.CommitCursor(id, "")
}

type applicationFullWriter struct{ io.Writer }

func (w applicationFullWriter) Write(p []byte) (int, error) {
	n, e := w.Writer.Write(p)
	if e == nil && n != len(p) {
		e = io.ErrShortWrite
	}
	return n, e
}

func runPrivateHostApplication(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "initialize-ready" {
		return runPrivateReadyProvisioning(args, stdin, stdout, stderr)
	}
	if len(args) > 0 && args[0] == "start-source" {
		return runPrivateSourceStart(args, applicationFullWriter{stdout}, stderr)
	}
	if len(args) > 0 && args[0] == "source-describe" {
		return runPrivateSourceHostApplication(args, applicationFullWriter{stdout}, stderr)
	}
	if len(args) > 0 && args[0] == "source-prepare-describe" {
		return runPrivateSourcePrepareDescribe(args, applicationFullWriter{stdout}, stderr)
	}
	stdout = applicationFullWriter{stdout}
	fail := func(class string) int { fmt.Fprintln(stderr, "private application:", class); return 1 }
	var startup *applicationStartupNotifier
	// Opt-in transport requires the already-issued explicit RunID. Strip only
	// this exact spelling so every legacy invocation retains its original path.
	if len(args) >= 4 && args[0] == "run" {
		if len(args) != 5 || args[3] != "--startup-notify-fd" || !validApplicationDigest(args[2]) {
			return fail("INVALID_STARTUP_NOTIFY_INVOCATION")
		}
		var err error
		startup, err = openApplicationStartupNotifier(args[4])
		if err != nil {
			return fail("STARTUP_NOTIFY_REFUSED")
		}
		defer startup.close() // EOF on early failure, cancellation and panic too
		args = args[:3]
	}
	if len(args) == 1 && args[0] == "prompts" {
		return runPrivateHostCohort([]string{"prompts"}, stdin, stdout, stderr)
	}
	if len(args) < 2 {
		return fail("INVALID_INVOCATION")
	}
	if args[0] == "prompts" {
		return runPrivateHostCohort([]string{"prompts"}, stdin, stdout, stderr)
	}
	if args[0] == "finalize" {
		return applicationFinalizeDispatch(args, stdin, stdout, stderr)
	}
	r, e := decodeHostApplication(args[1], false)
	if e != nil {
		return fail("CONFIG_REFUSED")
	}
	if args[0] == "wait" || args[0] == "register" || args[0] == "respond" || args[0] == "finalize" {
		r, e = readHostApplication(args[1])
		if e != nil {
			return fail("CONFIG_REFUSED")
		}
		return applicationHostHelper(args, r, stdin, stdout, stderr)
	}
	if args[0] == "acquire" {
		r, e = readHostApplication(args[1])
		if e != nil {
			return fail("CONFIG_REFUSED")
		}
		if len(args) != 4 {
			return fail("INVALID_CHILD")
		}
		return acquireApplicationChild(args[1], r, args[2], args[3], stdout, stderr)
	}
	if len(args) != 2 && !(args[0] == "run" && len(args) == 3) && !(args[0] == "fail" && len(args) == 3) {
		return fail("INVALID_INVOCATION")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	remaining := time.Until(time.Unix(0, r.Configuration.Grant.ExpiresUnixNano))
	if remaining > 240*time.Second {
		remaining = 240 * time.Second
	}
	// A caller's shorter operation limit also bounds every phase of this run;
	// it does not create another unit entitlement or reset the stored deadline.
	if operation := time.Duration(r.Template.OperationTimeoutMS) * time.Millisecond; operation < remaining {
		remaining = operation
	}
	ctx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	var a *product.ApplicationJournal
	if _, e = os.Lstat(r.Configuration.Store); os.IsNotExist(e) {
		a, e = product.CreateApplication(r.Configuration, time.Now())
	} else {
		a, e = product.OpenApplication(r.Configuration, time.Now())
	}
	if e != nil {
		return fail("JOURNAL_ADMISSION_REFUSED")
	}
	defer a.Close()
	r, e = readHostApplication(args[1])
	if e != nil {
		return fail("CONFIG_REFUSED_CHARGED")
	}
	unitPolicy, policyErr := applicationUnitEnvelope(r)
	if policyErr != nil {
		return fail("UNIT_POLICY_REFUSED")
	}
	if args[0] == "setup" {
		ticket, e := applicationTicket(a, r.Configuration.Store, r.MaxUnitsPerRun, unitPolicy.MaxCalls)
		if e != nil {
			return fail("COORDINATOR_ENVELOPE_REFUSED")
		}
		type roleTask struct{ Role, Prompt, PromptSHA256 string }
		roles := []roleTask{}
		for _, role := range []string{"interpreter", "reviewer"} {
			p := product.CohortRolePrompt(role)
			roles = append(roles, roleTask{role, p, pilot.Digest([]byte(p))})
		}
		return encodeHostProductV3(stdout, struct {
			Schema, RunID string
			RoleTasks     []roleTask
		}{"private-application-host-setup.v1", ticket, roles})
	}
	runID := ""
	if len(args) == 3 {
		runID = args[2]
		var ticket applicationRunTicket
		if !validApplicationDigest(runID) || a.GetRecord(runID, &ticket) != nil || ticket.ApplicationSHA256 != a.Head().ApplicationSHA256 || ticket.CursorSHA256 != a.Head().CursorSHA256 {
			return fail("RUN_TICKET_REFUSED")
		}
	} else {
		units := r.MaxUnitsPerRun
		if args[0] == "discover" {
			units = 0
		}
		runID, e = applicationTicket(a, r.Configuration.Store, units, unitPolicy.MaxCalls)
		if e != nil {
			return fail("COORDINATOR_ENVELOPE_REFUSED")
		}
	}
	if _, e = a.ReserveOperation(pilot.Digest([]byte(runID+"/single-run")), "single_coordinator_run", product.ApplicationBounds{}, time.Now()); e != nil {
		return fail("RUN_TICKET_SPENT")
	}
	s := applicationState{Schema: "private-application-cursor.v1", ApplicationSHA256: product.ApplicationConfigDigest(r.Configuration)}
	if a.Head().CursorSHA256 != "" {
		if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
			return fail("CURSOR_REFUSED")
		}
	} else {
		info, e := os.Stat(r.Configuration.SourceRoot)
		if e != nil {
			return fail("SOURCE_ROOT_REFUSED")
		}
		s.Directory = "."
		s.DirectoryIdentity = applicationFileIdentity(info)
		if e = saveApplicationState(a, s); e != nil {
			return fail("INITIAL_CURSOR_REFUSED")
		}
	}
	report := func(status string, code int) int {
		terminal := applicationTerminalFor(a, s, status)
		raw, _ := json.Marshal(terminal)
		if applicationWriteOnce(applicationTerminalPath(r.Configuration.Store, runID), raw) != nil {
			return fail("TERMINAL_PUBLICATION_ERROR")
		}
		if encodeHostProductV3(stdout, terminal) != 0 {
			return 1
		}
		return code
	}
	if args[0] == "fail" {
		if s.Inflight != "" && s.CohortRecovery != "" {
			var recovery product.ApplicationCohortRecovery
			if a.GetRecord(s.CohortRecovery, &recovery) == nil {
				_ = a.ReconcileCohortReturns(recovery)
			}
		}
		return report("HOST_TOOL_UNAVAILABLE", 1)
	}
	if args[0] != "run" && args[0] != "discover" {
		return fail("INVALID_OPERATION")
	}
	if s.Inflight != "" {
		if s.CohortRecovery != "" {
			var recovery product.ApplicationCohortRecovery
			if a.GetRecord(s.CohortRecovery, &recovery) == nil {
				_ = a.ReconcileCohortReturns(recovery)
			}
		}
		return report("PAUSED_AMBIGUOUS_SPENT_UNIT_NO_REPLAY", 1)
	}
	if args[0] == "discover" {
		if e = advanceApplicationDiscoveryWithHandoff(ctx, r, a, &s, false); e != nil {
			if errors.Is(e, product.ErrPaused) {
				return report("PAUSED_BUDGET", 0)
			}
			return report("FAILED_SOURCE_DISCOVERY_UNKNOWN", 1)
		}
		return report("DISCOVERY_COMPLETE", 0)
	}
	for count := 0; count < r.MaxUnitsPerRun; count++ {
		if e = advanceApplicationDiscovery(ctx, r, a, &s); e != nil {
			if errors.Is(e, product.ErrPaused) {
				return report("PAUSED_BUDGET", 0)
			}
			return report("FAILED_SOURCE_DISCOVERY_UNKNOWN", 1)
		}
		if s.Selected == "" {
			break
		}
		if e = ctx.Err(); e != nil {
			return report("CANCELLED_OR_DEADLINE", 1)
		}
		var entry applicationCatalogEntry
		if e = a.GetRecord(s.Selected, &entry); e != nil {
			return report("FAILED_SELECTED_ENTRY", 1)
		}
		next := entry.Next
		unitID := pilot.Digest([]byte(s.ApplicationSHA256 + "/" + s.Selected))
		if _, e = a.ReserveOperation(pilot.Digest([]byte(fmt.Sprintf("%s/source-preflight/%d", unitID, a.Head().Sequence))), "source_freeze", product.ApplicationBounds{ReadHashBytes: uint64(r.Template.Policy.TotalBytes) * 2, LogicalBytes: uint64(r.Template.Policy.TotalBytes), Records: 1024, ParserBytes: uint64(r.Template.Policy.TotalBytes), Work: 1 << 20, ActiveNS: uint64(time.Second)}, time.Now()); e != nil {
			return report("PAUSED_BUDGET", 0)
		}
		unitRoot := filepath.Join(r.Configuration.Store, "units", unitID[7:])
		if e = os.MkdirAll(unitRoot, 0700); e != nil {
			return report("FAILED_UNIT_ROOT", 1)
		}
		source, frozen, e := deriveApplicationSource(ctx, r, entry, unitRoot)
		if e != nil {
			if ctx.Err() != nil {
				return report("CANCELLED_OR_DEADLINE_DURING_SOURCE_ADMISSION", 1)
			}
			s.SourceGaps++
			s.Selected = next
			if e = publishApplicationGap(a, &s, entry, unitID, r.Configuration.RevisionAssertion, "SOURCE_ADMISSION_REFUSAL"); e != nil {
				return report("FAILED_SOURCE_GAP_PUBLICATION", 1)
			}
			continue
		}
		u := product.UnitBinding{ApplicationSHA256: s.ApplicationSHA256, UnitID: unitID, InputSHA256: pilot.Digest(frozen.SpecBytes()), RevisionSHA256: pilot.Digest([]byte(r.Configuration.RevisionAssertion)), SourceFreezeSHA256: pilot.Digest(frozen.ManifestBytes()), ScopeSHA256: source.Configuration.ScopeSHA256, ProducerConfigSHA256: r.Configuration.ProducerConfigSHA256, PolicySHA256: r.Configuration.UnitPolicySHA256, RootIdentitySHA256: entry.Identity}
		allowance, allowanceErr := applicationChildAllowance(r, source)
		if allowanceErr != nil {
			return report("INVALID_CHILD_ALLOWANCE", 1)
		}
		base := a.Head().Reserved.Calls
		approvedCohort := applicationDerivedCohort(r, source.Configuration, unitID, unitRoot)
		if e = a.AdmitUnitWithDerivation(u, allowance, source.Configuration, approvedCohort, time.Now()); e != nil {
			return report("PAUSED_BUDGET", 0)
		}
		// Publish only a readonly pointer to the already-prepaid unit interval.
		// Both HOST roles share this interval with child orchestration; no time
		// reservation is added again for overlapping wall-clock waits.
		windowRaw, _ := json.Marshal(struct{ UnitID string }{unitID})
		if e = applicationWriteOnce(applicationWaitUnitPath(r.Configuration.Store, runID, count+1), windowRaw); e != nil {
			return report("FAILED_PREPAID_WAIT_WINDOW", 1)
		}
		s.Inflight = unitID
		if e = saveApplicationState(a, s); e != nil {
			return report("FAILED_SPENT_CURSOR", 1)
		}
		contract := struct {
			Unit   product.UnitBinding
			Entry  applicationCatalogEntry
			Source hostProductRequestV3
		}{u, entry, source}
		contractID, e := a.PutRecord(contract)
		if e != nil {
			return report("FAILED_UNIT_CONTRACT", 1)
		}
		exe, e := os.Executable()
		if e != nil {
			return report("FAILED_EXECUTABLE", 1)
		}
		unitDeadline, deadlineErr := product.ReadApplicationUnitDeadline(r.Configuration.Store, s.ApplicationSHA256, unitID)
		if deadlineErr != nil {
			return report("FAILED_PREPAID_UNIT_DEADLINE", 1)
		}
		unitCtx, unitCancel := context.WithDeadline(ctx, unitDeadline)
		defer unitCancel()
		childctx, childcancel := context.WithTimeout(unitCtx, 20*time.Second)
		// Capture the prepaid unit identity before the child may create auxiliary
		// diagnostics. A replaced unit must never supply a failure projection.
		failureUnitIdentity, _ := os.Lstat(unitRoot)
		child := exec.CommandContext(childctx, exe, "private-host-application", "acquire", args[1], contractID, unitRoot)
		child.Stdout = io.Discard
		child.Stderr = io.Discard
		e = child.Run()
		childcancel()
		if e != nil {
			fmt.Fprintln(stderr, "private application: ORIGINAL_PROCESS_SPENT")
			reportApplicationFailure(stderr, unitRoot, failureUnitIdentity, source)
			return report("FAILED_ORIGINAL_PROCESS_SPENT", 1)
		}
		if terminalRaw, te := applicationReadBounded(filepath.Join(source.AuxiliaryRoot, "application-terminal-discovery.json"), 4096); te == nil {
			var terminal struct{ Reason, ScopeSHA256 string }
			if len(terminalRaw) > 4096 || json.Unmarshal(terminalRaw, &terminal) != nil || terminal.ScopeSHA256 != source.Configuration.ScopeSHA256 {
				return report("FAILED_TERMINAL_DISCOVERY_BINDING", 1)
			}
			if terminal.Reason == "UNIT_SEMANTIC_POLICY_PAUSE" {
				return report("PAUSED_UNIT_SEMANTIC_POLICY", 0)
			}
			s.SourceGaps++
			s.Selected = next
			s.Inflight = ""
			if e = publishApplicationGap(a, &s, entry, unitID, r.Configuration.RevisionAssertion, terminal.Reason); e != nil {
				return report("FAILED_TERMINAL_DISCOVERY_CURSOR", 1)
			}
			unitCancel()
			continue
		}
		expectedSource := source
		raw, e := applicationReadBounded(filepath.Join(unitRoot, "source.json"), 64<<10)
		if e != nil || json.Unmarshal(raw, &source) != nil {
			return report("FAILED_DERIVED_SOURCE", 1)
		}
		expectedSource.Configuration.Requested = source.Configuration.Requested
		expectedSource.Configuration.ApprovalSHA256 = product.ConfigDigest(expectedSource.Configuration)
		expectedRaw, _ := json.Marshal(expectedSource)
		if !bytes.Equal(raw, expectedRaw) {
			return report("FAILED_CHILD_DERIVATION_BINDING", 1)
		}
		transfer, e := product.ReadTransfer(filepath.Join(source.AuxiliaryRoot, "transfer.json"))
		if e != nil || validateHostProductTransferV3(source, transfer) != nil {
			return report("FAILED_TRANSFER", 1)
		}
		bootstrap := source.BootstrapAllowance
		owner, closeOwner, e := product.OpenTransfer(unitCtx, transfer, &bootstrap, source.Configuration.Exchange)
		if e != nil {
			return report("FAILED_VALIDATED_RETAINED_ADMISSION", 1)
		}
		cc := applicationDerivedCohort(r, source.Configuration, unitID, unitRoot)
		if e = a.BindUnitResultConfiguration(u, source.Configuration, cc); e != nil {
			closeOwner()
			return report("FAILED_ADMITTED_CHILD_CONFIGURATION", 1)
		}
		mailbox := product.CohortMailbox{Directory: filepath.Join(unitRoot, "cohort-mailbox"), ExchangeTimeout: time.Duration(source.ExchangeTimeoutMS) * time.Millisecond, Observation: transfer.Cleanup}
		mailbox.Observation.OuterOriginalProcessWaited = true
		if e = product.PrepareMailbox(mailbox.Directory); e != nil {
			closeOwner()
			return report("FAILED_MAILBOX", 1)
		}
		s.Activation++
		activation := applicationActivation{mailbox.Directory, unitID, base}
		if e = a.WriteActivation(s.Activation, activation); e != nil {
			closeOwner()
			return report("FAILED_ACTIVATION", 1)
		}
		// The startup packet discloses no source or role identity. It is sent
		// once AFTER durable activation and inside the SAME prepaid unitCtx.
		// The original child inherited no notifier FD (CLOEXEC), and no fresh
		// clock, wait identity, semantic slot or grant is created here.
		if startup != nil && !startup.attempted {
			if e = startup.emit(unitCtx, applicationActivationPacket(s.ApplicationSHA256, runID, unitID, s.Activation, activation)); e != nil {
				closeOwner()
				return report("FAILED_STARTUP_NOTIFICATION", 1)
			}
		}
		for mailbox.CheckReady() != nil {
			select {
			case <-unitCtx.Done():
				closeOwner()
				return report("CANCELLED_OR_UNIT_DEADLINE_BEFORE_READY", 1)
			case <-time.After(2 * time.Millisecond):
			}
		}
		recoveryID, re := a.PutRecord(product.ApplicationCohortRecovery{UnitID: unitID, Config: cc, Mailbox: mailbox.Directory})
		if re != nil {
			closeOwner()
			return report("FAILED_RECOVERY_BINDING", 1)
		}
		s.CohortRecovery = recoveryID
		if e = saveApplicationState(a, s); e != nil {
			closeOwner()
			return report("FAILED_RECOVERY_CURSOR", 1)
		}
		// No new unit window after original acquisition or preparation. unitCtx is
		// rooted in the parent's durable PREPAY_CHILD deadline and operation/expiry.
		cohort, runErr := product.RunCohortRetained(unitCtx, source.Configuration, cc, owner, product.ApplicationCohortBackend{Journal: a, UnitID: unitID, Mailbox: mailbox, MaxResponseBytes: uint64(r.UnitResponseBytes)})
		closeErr := closeOwner()
		// Preparation may refuse before a cohort report exists. Preserve the spent
		// inflight unit and known costs; the application terminal, not a fabricated
		// empty cohort terminal, communicates this typed pause to both roles.
		if closeErr == nil && errors.Is(runErr, pilot.ErrPagesUnitPrivateV3) {
			return report("PAUSED_UNSUPPORTED_COMPUTATION_UNIT", 0)
		}
		if closeErr == nil && errors.Is(runErr, pilot.ErrPagesPausedPrivateV3) {
			return report("PAUSED_PREPARATION_BUDGET", 0)
		}
		terminalErr := mailbox.TerminalCohort(cohort)
		if errors.Join(runErr, closeErr, terminalErr) != nil {
			fmt.Fprintf(stderr, "retained failure class=%s close=%t terminal=%t\n", pilot.ScopeFailureClassPrivateV2(runErr), closeErr != nil, terminalErr != nil)
			return report("FAILED_RETAINED_OR_SEMANTIC_UNKNOWN", 1)
		}
		if _, e = a.ReserveOperation(pilot.Digest([]byte(unitID+"/child-reference-admission")), "child_reference_admission", product.ApplicationBounds{ReadHashBytes: 64 << 20, ParserBytes: 64 << 20, Records: 256, Work: 1 << 20}, time.Now()); e != nil {
			return report("PAUSED_BEFORE_REFERENCE_ADMISSION", 1)
		}
		ref, e := product.BuildUnitResultRef(s.ApplicationSHA256, u, cc, cohort)
		if e != nil {
			return report("FAILED_CHILD_PAIR_ADMISSION", 1)
		}
		cost := cohort.PolicyReservation
		if ^uint64(0)-s.ChildPolicyConsumed.LogicalBytes < cost.LogicalBytes || ^uint64(0)-s.ChildPolicyConsumed.ReadHashBytes < cost.ReadHashBytes || ^uint64(0)-s.ChildPolicyConsumed.WriteBytes < cost.WriteBytes || ^uint64(0)-s.ChildPolicyConsumed.Work < cost.Work {
			return report("FAILED_POLICY_COUNTER_OVERFLOW", 1)
		}
		s.ChildPolicyConsumed.LogicalBytes += cost.LogicalBytes
		s.ChildPolicyConsumed.ReadHashBytes += cost.ReadHashBytes
		s.ChildPolicyConsumed.WriteBytes += cost.WriteBytes
		s.ChildPolicyConsumed.Work += cost.Work
		s.Units++
		s.Targets += cohort.Requested
		s.Selected = next
		s.Inflight = ""
		s.CohortRecovery = ""
		unitCancel()
		page := product.ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: s.ApplicationSHA256, Previous: a.Head().LastCompletePublicationSHA256, Rows: []product.UnitResultRef{ref}}
		pub, e := a.PublishInventory(page, s)
		if e != nil {
			return report("FAILED_APPLICATION_PUBLICATION", 1)
		}
		s.LastPage = pub
	}
	if s.Selected != "" || !s.DiscoveryFinished {
		return report("PAUSED_UNIT_SLICE", 0)
	}
	return report("PROVISIONAL", 0)
}
