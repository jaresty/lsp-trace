//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Separate trusted Prepare declaration. No HOST, mailbox, output/protection
// roots, original roles, local worker, timing extension or serialized grant.
// Canonical owner bootstrap verification is the existing trusted caller-funded
// admission contract, not a credit inferred from this configuration or a hash.
type privateSourcePrepareDescribeConfig struct {
	Schema                                                                  string
	AccountingRoot                                                          string
	CanonicalJob                                                            []byte
	ExpectedHead                                                            string
	Verification                                                            inv.PrivateManagedInputVerificationV2
	Profile                                                                 runtimeprofile.Selector
	LanguageID, Provider, ProcessPath, Workspace                            string
	ProcessArgs                                                             []string
	URI, Revision, Privacy, OccurrenceID                                    string
	Line, Character                                                         uint32
	CaptureAttempt, PreparationAttempt, PacketAttempt, RunID, SeedNamespace string
	Seed                                                                    uint64
	Persist                                                                 inv.PreparedDescribePersistRequestV1
	PacketLimit                                                             uint64
}

func decodePrivateSourcePrepareDescribeConfig(raw []byte) (privateSourcePrepareDescribeConfig, error) {
	var c privateSourcePrepareDescribeConfig
	fail := func() (privateSourcePrepareDescribeConfig, error) {
		return privateSourcePrepareDescribeConfig{}, inv.ErrPreparedDescribeV1
	}
	if len(raw) == 0 || len(raw) > 64<<10 {
		return fail()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return fail()
	}
	canonical, err := json.Marshal(c)
	// Canonical spelling also rejects duplicate keys and alternate control forms.
	if err != nil || !bytes.Equal(bytes.TrimSuffix(raw, []byte("\n")), canonical) || c.Schema != "private-source-prepare-describe.v1" || len(c.CanonicalJob) == 0 || len(c.CanonicalJob) > 32<<10 || !filepath.IsAbs(c.AccountingRoot) || filepath.Clean(c.AccountingRoot) != c.AccountingRoot || c.Persist.RootPath != c.AccountingRoot || c.Persist.InitialHead != c.ExpectedHead || !filepath.IsAbs(c.ProcessPath) || len(c.ProcessArgs) > 64 || c.Workspace != c.Profile.Workspace || c.PacketLimit == 0 || c.PacketLimit > 1<<20 || c.Persist.Deadline.IsZero() {
		return fail()
	}
	for _, s := range c.ProcessArgs {
		if len(s) > 4096 {
			return fail()
		}
	}
	return c, nil
}

// Only invocation-local reported-error seams. Real Shutdown/Close ALWAYS run
// first; these functions cannot produce a source, owner, result or authority.
type privatePrepareDescribeCloseReports struct {
	manager, root, dataReport func() error
	data                      io.Closer
}

func privateSourcePrepareDescribeOwned(ctx context.Context, c privateSourcePrepareDescribeConfig, root *publication.Root, owner *inv.PrivateManagedInputOwnerV2, manager *sessionruntime.Manager, start sessionruntime.StartRequest, reports privatePrepareDescribeCloseReports) (inv.StagedPrepareDescribeV1, error) {
	control, cancel := context.WithDeadline(ctx, c.Persist.Deadline)
	capture := inv.PrivateMixedInputRequestV2{Manager: manager, Root: root, Start: start, URI: c.URI, OccurrenceID: c.OccurrenceID, Revision: c.Revision, Privacy: c.Privacy, OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: c.Line, Character: c.Character, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
	staged, executionErr := inv.PrepareDescribeV1(control, inv.PrepareDescribeRequestV1{Owner: owner, Capture: capture, CaptureAttempt: c.CaptureAttempt, PreparationAttempt: c.PreparationAttempt, PacketAttempt: c.PacketAttempt, RunID: c.RunID, SeedNamespace: c.SeedNamespace, Seed: c.Seed, Persist: c.Persist, PacketLimit: c.PacketLimit})
	shutdownErr := manager.Shutdown(control)
	if reports.manager != nil {
		shutdownErr = errors.Join(shutdownErr, reports.manager())
	}
	var dataErr error
	if reports.data != nil {
		dataErr = reports.data.Close()
	}
	if reports.dataReport != nil {
		dataErr = errors.Join(dataErr, reports.dataReport())
	}
	closeErr := root.Close()
	if reports.root != nil {
		closeErr = errors.Join(closeErr, reports.root())
	}
	controlErr, callerErr := control.Err(), ctx.Err()
	terminalErr := errors.Join(executionErr, shutdownErr, dataErr, closeErr, controlErr, callerErr)
	var deadlineErr error
	if !time.Now().Before(c.Persist.Deadline) {
		deadlineErr = context.DeadlineExceeded
		terminalErr = errors.Join(terminalErr, context.DeadlineExceeded)
	}
	cancel()
	if control.Err() != context.Canceled || ctx.Err() != nil || !time.Now().Before(c.Persist.Deadline) {
		terminalErr = errors.Join(terminalErr, inv.ErrPreparedDescribeV1)
	}
	// Commit may already have selected durable data. Cleanup failure consumes
	// this invocation without undo, refund, repair Observe or packet availability.
	if terminalErr != nil {
		var canceledErr error
		if controlErr == context.Canceled || callerErr == context.Canceled || ctx.Err() == context.Canceled {
			canceledErr = context.Canceled
		}
		if controlErr == context.DeadlineExceeded || callerErr == context.DeadlineExceeded || !time.Now().Before(c.Persist.Deadline) {
			deadlineErr = context.DeadlineExceeded
		}
		return inv.StagedPrepareDescribeV1{}, inv.PrivatePrepareTerminalFailureV1(executionErr, terminalErr, shutdownErr, dataErr, closeErr, canceledErr, deadlineErr)
	}
	return staged, nil
}

// Explicit invocation-local CALLER-funded failure-report allowance, separately
// approved from native job quotes. At most 4096 bytes, ONE Write, no retry;
// fixed projection/formatting uses one closed node, six slots, <256 fixed steps.
// No claim that an arbitrary external writer finishes by the source deadline.
func privatePrepareFailureReport(stderr io.Writer, class string, err error) int {
	const byteAllowance = 4096
	writesRemaining := 1
	switch class {
	case "CONFIG_REFUSED", "FRESH_ADMISSION_REFUSED", "ROOT_REFUSED", "CANONICAL_OWNER_REFUSED", "PROFILE_REFUSED", "STARTER_REFUSED", "MANAGER_REFUSED", "OUTPUT_FAILED_POSSIBLY_PARTIAL":
	default:
		class = "REFUSED_OR_CONSUMED_PENDING_UNCERTAIN"
	}
	codes := inv.ProjectPrivatePrepareFailureV1(err)
	var storage [byteAllowance]byte
	report := append(storage[:0], "private Prepare: "...)
	report = append(report, class...)
	report = append(report, "\nprivate Prepare diagnostic.v1 stage="...)
	report = append(report, codes.Stage.String()...)
	report = append(report, " reason="...)
	report = append(report, codes.Reason.String()...)
	for i, name := range [6]string{" deferred_finish=", " shutdown=", " data_close=", " root_close=", " canceled=", " deadline="} {
		report = append(report, name...)
		report = append(report, codes.Cleanup[i].String()...)
	}
	report = append(report, '\n')
	if len(report) > byteAllowance || writesRemaining == 0 {
		return 1
	}
	writesRemaining--           // consume separate fixed caller allowance BEFORE IO
	_, _ = stderr.Write(report) // partial/error is terminal: never a second write
	return 1
}

func finishPrivateSourcePrepareDescribe(ctx context.Context, deadline time.Time, staged inv.StagedPrepareDescribeV1, err error, stdout, stderr io.Writer) int {
	fail := func(class string) int { return privatePrepareFailureReport(stderr, class, err) }
	if err != nil || len(staged.Packet) == 0 || ctx.Err() != nil || !time.Now().Before(deadline) {
		return fail("REFUSED_OR_CONSUMED_PENDING_UNCERTAIN")
	}
	// Already encoded under a native output reservation. This write is not an
	// atomic pair/publish promise: partial IO remains partial and is reported failed.
	n, e := stdout.Write(staged.Packet)
	if e != nil || n != len(staged.Packet) || ctx.Err() != nil || !time.Now().Before(deadline) {
		return fail("OUTPUT_FAILED_POSSIBLY_PARTIAL")
	}
	return 0
}

func executePrivateSourcePrepareDescribe(ctx context.Context, c privateSourcePrepareDescribeConfig, stdout, stderr io.Writer, data io.Closer) (code int) {
	var earlyClass string
	var earlyCause, earlyRootErr error
	defer func() {
		var dataErr error
		if data != nil {
			dataErr = data.Close()
			if dataErr != nil {
				code = 1
			}
		}
		// Early paths retain their old owned-close cardinality and order. Report
		// only AFTER deferred data cleanup, never once here and again in finish.
		if earlyClass != "" {
			var canceledErr, deadlineErr error
			if ctx != nil {
				if ctx.Err() == context.Canceled {
					canceledErr = context.Canceled
				}
				if ctx.Err() == context.DeadlineExceeded {
					deadlineErr = context.DeadlineExceeded
				}
			}
			terminal := errors.Join(earlyCause, dataErr, earlyRootErr, inv.ErrPreparedDescribeV1)
			diag := inv.PrivatePrepareTerminalFailureV1(nil, terminal, nil, dataErr, earlyRootErr, canceledErr, deadlineErr)
			code = privatePrepareFailureReport(stderr, earlyClass, diag)
		}
	}()
	fail := func(class string, cause error) int { earlyClass, earlyCause = class, cause; return 1 }
	if ctx == nil || ctx.Err() != nil || !time.Now().Before(c.Persist.Deadline) || time.Until(c.Persist.Deadline) > 15*time.Second {
		return fail("FRESH_ADMISSION_REFUSED", nil)
	}
	control, cancel := context.WithDeadline(ctx, c.Persist.Deadline)
	defer cancel()
	root, err := publication.OpenRoot(c.AccountingRoot)
	if err != nil {
		return fail("ROOT_REFUSED", err)
	}
	owner, err := inv.OpenPrivateManagedInputOwnerV2(control, root, c.CanonicalJob, c.ExpectedHead, c.Verification)
	if err != nil {
		earlyRootErr = root.Close()
		err = errors.Join(err, earlyRootErr)
		return fail("CANONICAL_OWNER_REFUSED", err)
	}
	selected, err := runtimeprofile.Validate(c.Profile)
	if err != nil {
		earlyRootErr = root.Close()
		err = errors.Join(err, earlyRootErr)
		return fail("PROFILE_REFUSED", err)
	}
	starter, err := productionCensusRunnerDependencies().newStarter()
	if err != nil {
		earlyRootErr = root.Close()
		err = errors.Join(err, earlyRootErr)
		return fail("STARTER_REFUSED", err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 16, MaxChildren: 16, MaxCancels: 8, MaxTombstones: 32, MaxObservations: 256, MaxOperations: 8}, Starter: starter})
	if err != nil {
		earlyRootErr = root.Close()
		err = errors.Join(err, earlyRootErr)
		return fail("MANAGER_REFUSED", err)
	}
	start := sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.LanguageID, RelationProviders: []string{c.Provider}, Deadline: c.Persist.Deadline, Process: managedprocess.Spec{Path: c.ProcessPath, Dir: c.Workspace, Args: c.ProcessArgs}}
	ownedData := data
	data = nil
	staged, err := privateSourcePrepareDescribeOwned(control, c, root, owner, manager, start, privatePrepareDescribeCloseReports{data: ownedData})
	return finishPrivateSourcePrepareDescribe(control, c.Persist.Deadline, staged, err, stdout, stderr)
}

func runPrivateSourcePrepareDescribe(args []string, stdout, stderr io.Writer) int {
	var dataCloseErr error
	fail := func() int {
		diag := inv.PrivatePrepareTerminalFailureV1(nil, inv.ErrPreparedDescribeV1, nil, dataCloseErr, nil, nil, nil)
		return privatePrepareFailureReport(stderr, "CONFIG_REFUSED", diag)
	}
	if len(args) != 2 || args[0] != "source-prepare-describe" {
		return fail()
	}
	fd, err := syscall.Open(args[1], syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fail()
	}
	f := os.NewFile(uintptr(fd), args[1])
	info, statErr := f.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 64<<10 {
		closeErr := f.Close()
		dataCloseErr = closeErr
		_ = errors.Join(statErr, closeErr)
		return fail()
	}
	raw, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	c, decodeErr := decodePrivateSourcePrepareDescribeConfig(raw)
	if err != nil || decodeErr != nil {
		closeErr := f.Close()
		dataCloseErr = closeErr
		_ = errors.Join(err, decodeErr, closeErr)
		return fail()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return executePrivateSourcePrepareDescribe(ctx, c, stdout, stderr, f)
}
