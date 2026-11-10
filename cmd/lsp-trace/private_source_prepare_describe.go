//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	terminalErr := errors.Join(executionErr, shutdownErr, dataErr, closeErr, control.Err(), ctx.Err())
	if !time.Now().Before(c.Persist.Deadline) {
		terminalErr = errors.Join(terminalErr, context.DeadlineExceeded)
	}
	cancel()
	if control.Err() != context.Canceled || ctx.Err() != nil || !time.Now().Before(c.Persist.Deadline) {
		terminalErr = errors.Join(terminalErr, inv.ErrPreparedDescribeV1)
	}
	// Commit may already have selected durable data. Cleanup failure consumes
	// this invocation without undo, refund, repair Observe or packet availability.
	if terminalErr != nil {
		return inv.StagedPrepareDescribeV1{}, terminalErr
	}
	return staged, nil
}

func finishPrivateSourcePrepareDescribe(ctx context.Context, deadline time.Time, staged inv.StagedPrepareDescribeV1, err error, stdout, stderr io.Writer) int {
	fail := func(class string) int { fmt.Fprintln(stderr, "private Prepare:", class); return 1 }
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
	defer func() {
		if data != nil {
			if err := data.Close(); err != nil {
				code = 1
			}
		}
	}()
	fail := func(class string) int { fmt.Fprintln(stderr, "private Prepare:", class); return 1 }
	if ctx == nil || ctx.Err() != nil || !time.Now().Before(c.Persist.Deadline) || time.Until(c.Persist.Deadline) > 15*time.Second {
		return fail("FRESH_ADMISSION_REFUSED")
	}
	control, cancel := context.WithDeadline(ctx, c.Persist.Deadline)
	defer cancel()
	root, err := publication.OpenRoot(c.AccountingRoot)
	if err != nil {
		return fail("ROOT_REFUSED")
	}
	owner, err := inv.OpenPrivateManagedInputOwnerV2(control, root, c.CanonicalJob, c.ExpectedHead, c.Verification)
	if err != nil {
		err = errors.Join(err, root.Close())
		return fail("CANONICAL_OWNER_REFUSED")
	}
	selected, err := runtimeprofile.Validate(c.Profile)
	if err != nil {
		err = errors.Join(err, root.Close())
		return fail("PROFILE_REFUSED")
	}
	starter, err := productionCensusRunnerDependencies().newStarter()
	if err != nil {
		err = errors.Join(err, root.Close())
		return fail("STARTER_REFUSED")
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 16, MaxChildren: 16, MaxCancels: 8, MaxTombstones: 32, MaxObservations: 256, MaxOperations: 8}, Starter: starter})
	if err != nil {
		err = errors.Join(err, root.Close())
		return fail("MANAGER_REFUSED")
	}
	start := sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: c.LanguageID, RelationProviders: []string{c.Provider}, Deadline: c.Persist.Deadline, Process: managedprocess.Spec{Path: c.ProcessPath, Dir: c.Workspace, Args: c.ProcessArgs}}
	ownedData := data
	data = nil
	staged, err := privateSourcePrepareDescribeOwned(control, c, root, owner, manager, start, privatePrepareDescribeCloseReports{data: ownedData})
	return finishPrivateSourcePrepareDescribe(control, c.Persist.Deadline, staged, err, stdout, stderr)
}

func runPrivateSourcePrepareDescribe(args []string, stdout, stderr io.Writer) int {
	fail := func() int { fmt.Fprintln(stderr, "private Prepare: CONFIG_REFUSED"); return 1 }
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
		_ = errors.Join(statErr, closeErr)
		return fail()
	}
	raw, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	c, decodeErr := decodePrivateSourcePrepareDescribeConfig(raw)
	if err != nil || decodeErr != nil {
		closeErr := f.Close()
		_ = errors.Join(err, decodeErr, closeErr)
		return fail()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return executePrivateSourcePrepareDescribe(ctx, c, stdout, stderr, f)
}
