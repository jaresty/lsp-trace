//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/inventorysuccessor/sourceoperator"
	"lsp-trace/internal/managedprocess"
	host "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

// A pin is only an exact configuration declaration, never issuance or admission.
// CanonicalJob contains exact previously issued bytes; no bootstrap is reachable.
type privateSourceHostConfig struct {
	Schema, Mode, Pin                                                     string
	AccountingRoot                                                        string
	CanonicalJob                                                          []byte
	ExpectedHead                                                          string
	Verification                                                          inv.PrivateManagedInputVerificationV2
	Profile                                                               runtimeprofile.Selector
	LanguageID, Provider                                                  string
	ProcessPath                                                           string
	ProcessArgs                                                           []string
	Workspace, Revision, Privacy, URI, OccurrenceID                       string
	Line, Character                                                       uint32
	CaptureAttempt, PreparationAttempt, RoleAttempt, RunID, SeedNamespace string
	Seed                                                                  uint64
	Deadline                                                              time.Time
	Timing                                                                *inv.PrivateSourceTimingV2      `json:",omitempty"`
	TimingFrame                                                           *inv.PrivateSourceTimingFrameV2 `json:",omitempty"`
	Contract                                                              cc.ContractInput
	SourceLimits                                                          targetpacket.PrivateSourceLimits
	Host                                                                  host.SourceHostConfig
	ResponseLimits                                                        describeworker.SuppliedResponseLimits
	OutputRoots                                                           [2]string
}

func privateSourceConfigPin(c privateSourceHostConfig) string {
	c.Pin = ""
	raw, e := json.Marshal(c)
	if e != nil {
		return ""
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return ""
	}
	raw, e = json.Marshal(fields)
	if e != nil {
		return ""
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
}
func decodePrivateSourceHostConfig(raw []byte) (privateSourceHostConfig, error) {
	var c privateSourceHostConfig
	if len(raw) == 0 || len(raw) > 64<<10 {
		return c, sourceoperator.ErrRefused
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF || c.Schema != "private-source-host-application.v1" || (c.Mode != "PREVIEW" && c.Mode != "EXECUTE") || c.Pin != privateSourceConfigPin(c) || len(c.CanonicalJob) == 0 || len(c.CanonicalJob) > 32<<10 || len(c.ProcessArgs) > 64 || !filepath.IsAbs(c.ProcessPath) || c.Workspace != c.Profile.Workspace || len(c.Provider) == 0 || len(c.Provider) > 256 || c.Deadline.IsZero() {
		return c, sourceoperator.ErrRefused
	}
	if c.Timing == nil {
		if c.TimingFrame != nil {
			return c, sourceoperator.ErrRefused
		}
	} else {
		if _, e := c.Timing.Total(); e != nil || c.TimingFrame == nil || !c.TimingFrame.Valid() || c.TimingFrame.Profile != *c.Timing || !c.Deadline.Equal(c.TimingFrame.TotalDeadline) || c.Host.Mailbox.TimeoutMS != 60000 || c.Host.Mailbox.OverallTimeoutMS != 60000 {
			return c, sourceoperator.ErrRefused
		}
	}
	for _, s := range c.ProcessArgs {
		if len(s) > 4096 {
			return c, sourceoperator.ErrRefused
		}
	}
	return c, nil
}
func runPrivateSourceHostApplication(args []string, stdout, stderr io.Writer) int {
	fail := func(class string) int { fmt.Fprintln(stderr, "private source application:", class); return 1 }
	if len(args) != 2 || args[0] != "source-describe" {
		return fail("INVALID_INVOCATION")
	}
	raw, e := readHostProductRequestRawV3(args[1])
	if e != nil {
		return fail("CONFIG_REFUSED")
	}
	c, e := decodePrivateSourceHostConfig(raw)
	if e != nil {
		return fail("CONFIG_REFUSED")
	}
	if c.Mode == "PREVIEW" {
		if json.NewEncoder(stdout).Encode(struct{ Schema, Disposition, Funding, FIT string }{"SOURCE_NATIVE_V1", "PREVIEW_NO_EXECUTION", "EXISTING_READY_JOB_REQUIRED_UNKNOWN", "UNKNOWN"}) != nil {
			return fail("OUTPUT_REFUSED")
		}
		return 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return executePrivateSourceHostConfig(ctx, c, stdout, stderr)
}

// Both the legacy explicit invocation and connected parent-start path enter the
// SAME production owner/Manager/Run/cleanup implementation. No executor hook.
func executePrivateSourceHostConfig(ctx context.Context, c privateSourceHostConfig, stdout, stderr io.Writer) int {
	fail := func(class string) int { fmt.Fprintln(stderr, "private source application:", class); return 1 }
	ctx, cancel := context.WithDeadline(ctx, c.Deadline)
	defer cancel()
	// Admission comes before Manager/starter construction, provider, mailbox and
	// output-root access. Refusal does not distinguish unissued from non-READY.
	root, e := publication.OpenRoot(c.AccountingRoot)
	if e != nil {
		return fail("OWNER_ADMISSION_REFUSED")
	}
	owner, e := inv.OpenPrivateManagedInputOwnerV2(ctx, root, c.CanonicalJob, c.ExpectedHead, c.Verification)
	if e != nil {
		_ = root.Close()
		return fail("OWNER_ADMISSION_REFUSED")
	}
	return executePrivateSourceHostOwned(ctx, c, stdout, stderr, root, owner)
}

// Takes ownership of the already admitted SOURCE accounting connection. The
// connected start path prepays its reopen/expiry verification and transfers it
// here, avoiding any post-control free re-open or serialized owner substitute.
func executePrivateSourceHostOwned(ctx context.Context, c privateSourceHostConfig, stdout, stderr io.Writer, root *publication.Root, owner *inv.PrivateManagedInputOwnerV2) int {
	fail := func(class string) int { fmt.Fprintln(stderr, "private source application:", class); return 1 }
	selected, e := runtimeprofile.Validate(c.Profile)
	if e != nil {
		_ = root.Close()
		return fail("PROFILE_REFUSED")
	}
	deps := productionCensusRunnerDependencies()
	starter, e := deps.newStarter()
	if e != nil {
		_ = root.Close()
		return fail("STARTER_REFUSED")
	}
	manager, e := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 8, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Starter: starter})
	if e != nil {
		_ = root.Close()
		return fail("MANAGER_REFUSED")
	}
	contract, e := cc.NewContinuationContract(c.Contract)
	contractFailed := e != nil
	var out sourceoperator.Outcome
	if e == nil {
		start := sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.LanguageID, RelationProviders: []string{c.Provider}, Deadline: c.Deadline, Process: managedprocess.Spec{Path: c.ProcessPath, Dir: c.Workspace, Args: c.ProcessArgs}}
		if c.TimingFrame != nil {
			start.Deadline = c.TimingFrame.CaptureDeadline
		}
		capture := inv.PrivateMixedInputRequestV2{Manager: manager, Root: root, Start: start, URI: c.URI, OccurrenceID: c.OccurrenceID, Revision: c.Revision, Privacy: c.Privacy, OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: c.Line, Character: c.Character, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
		out, e = sourceoperator.Run(ctx, sourceoperator.Request{Owner: owner, Capture: capture, CaptureAttempt: c.CaptureAttempt, PreparationAttempt: c.PreparationAttempt, RoleAttempt: c.RoleAttempt, RunID: c.RunID, SeedNamespace: c.SeedNamespace, Seed: c.Seed, Contract: contract, SourceLimits: c.SourceLimits, Host: c.Host, ResponseLimits: c.ResponseLimits, OutputRoots: c.OutputRoots, Timing: c.TimingFrame})
	}
	// No renewed cleanup window. FinishMixed already joined the provider window;
	// shutdown and every owned descriptor close contribute to the terminal result.
	executionErr := e // Preserve the operation result before invalid-tail refusal.
	tailInvalid := false
	terminalDeadline := c.Deadline
	cleanupCtx := ctx
	if tail, ok := out.TerminalDeadlineV2(); ok {
		if tail.After(c.Deadline) {
			tailInvalid = true
		} else {
			terminalDeadline = tail
			var stop context.CancelFunc
			cleanupCtx, stop = context.WithDeadline(ctx, tail)
			defer stop()
		}
	}
	shutdownErr := manager.Shutdown(cleanupCtx)
	closeErr := root.Close()
	// Check both the immutable total and its once-bound publication/cleanup tail
	// after shutdown AND this final owned close. Neither is a termination promise.
	now := time.Now()
	diagnostic := privateSourceDiagnosticV1(out, executionErr, shutdownErr, closeErr, ctx.Err(), cleanupCtx.Err(), contractFailed, tailInvalid, now, terminalDeadline)
	return finishPrivateSourceV1(out, diagnostic, stdout, stderr)
}
