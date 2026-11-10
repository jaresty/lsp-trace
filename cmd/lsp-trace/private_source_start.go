//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
)

const privateStartInvocation = "host-assisted-full-definition-r-XoBFapmT"
const privateStartProvider = "/Users/schwa/.local/bin/gopls"
const privateStartProducerSession = "01a1c289-d47d-7659-964e-7784260251e4"
const privateStartReviewerSession = "01a1c289-d483-7659-964e-77879e176a82"

// This is a declaration over ALREADY initialized, distinct canonical control
// and SOURCE jobs. It never initializes, issues, renews, or authenticates either.
// Source.Deadline and Source.Pin MUST be zero: the single parent start sets them.
type privateSourceStartConfig struct {
	Schema, Invocation, Base, Pin              string
	ControlRoot                                string
	CanonicalControlJob                        []byte
	ControlHead                                string
	ControlVerification                        inv.PrivateManagedInputVerificationV2
	ControlAttempt, LaunchAttempt              string
	ControlDeadline, LaunchDeadline            time.Time
	ProducerSession, ReviewerSession           string
	ProducerReadySHA256, ReviewerReadySHA256   string
	FrozenProducerSHA256, FrozenReviewerSHA256 string
	Source                                     privateSourceHostConfig
}
type privateConnectedReady struct {
	Schema, Invocation, ConfigPin, SourceTemplatePin, ControlReceipt string
	LaunchDeadline                                                   time.Time
}
type privateParentStartReady struct {
	Schema, Invocation, DriverReadySHA256 string
	ProducerSession, ReviewerSession      string
	ProducerWaiterActive                  bool
}
type privateParentStartSignal struct {
	Operation, Invocation                                   string
	ProducerWaiterActive                                    bool
	DriverReadySHA256, ParentReadySHA256, SourceTemplatePin string
}

func privateStartDigest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func privateStartConfigPin(c privateSourceStartConfig) string {
	c.Pin = ""
	b, e := json.Marshal(c)
	if e != nil {
		return ""
	}
	return privateStartDigest(b)
}
func privateStartCanonical(raw []byte, v any) error {
	if len(raw) == 0 || len(raw) > 65536 {
		return errors.New("start metadata size")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return errors.New("start metadata shape")
	}
	b, e := json.Marshal(v)
	if e != nil || (!bytes.Equal(raw, b) && !bytes.Equal(raw, append(b, '\n'))) {
		return errors.New("start metadata canonicality")
	}
	return nil
}
func decodePrivateStartConfig(raw []byte) (privateSourceStartConfig, error) {
	var c privateSourceStartConfig
	if e := privateStartCanonical(raw, &c); e != nil {
		return c, e
	}
	s := c.Source
	if c.Schema != "private-source-start.v2" || c.Invocation != privateStartInvocation || c.Pin != privateStartConfigPin(c) || !filepath.IsAbs(c.Base) || filepath.Clean(c.Base) != c.Base || filepath.Base(c.Base) != c.Invocation || c.ControlRoot == s.AccountingRoot || c.ControlRoot == c.Base || !filepath.IsAbs(c.ControlRoot) || len(c.CanonicalControlJob) == 0 || len(c.CanonicalControlJob) > 16384 || c.ControlDeadline.IsZero() || c.LaunchDeadline.IsZero() || !s.Deadline.IsZero() || s.TimingFrame != nil || s.Pin != "" || s.Mode != "EXECUTE" || s.ProcessPath != privateStartProvider || len(s.ProcessArgs) != 0 || s.Host.Mailbox.Invocation != c.Invocation || s.Host.Mailbox.InterpreterRole != "host-source-producer-1" || s.Host.Mailbox.ReviewerRole != "host-source-reviewer-1" || c.ProducerSession != privateStartProducerSession || c.ReviewerSession != privateStartReviewerSession {
		return c, errors.New("start binding")
	}
	for _, v := range []string{c.ControlHead, c.ControlAttempt, c.LaunchAttempt, c.ProducerReadySHA256, c.ReviewerReadySHA256, c.FrozenProducerSHA256, c.FrozenReviewerSHA256} {
		if !validApplicationDigest(v) {
			return c, errors.New("start digest")
		}
	}
	if s.Seed != 7 || s.SeedNamespace != "source-original-schedule" || !privateStartTimingConfigValid(s) || s.Host.Mailbox.PollMS != 10 || s.SourceLimits.MaxDocumentBytes > 16<<10 || s.SourceLimits.MaxTotalBytes > 64<<10 || s.SourceLimits.MaxWork > 4096 || s.SourceLimits.MaxObjects > 16 || s.Contract.PacketMaxResponseBytes > 131072 || s.ResponseLimits.MaxAdapterBytes > 8192 || s.ResponseLimits.MaxSemanticBytes > 4096 || s.Host.Mailbox.Limits.MaxCalls != 1 || s.Host.Mailbox.Limits.MaxRequestBytes > 65536 || s.Host.Mailbox.Limits.MaxResponseBytes > 16384 || s.Host.Mailbox.Limits.MaxTotalBytes > 147456 || s.Host.Mailbox.Limits.MaxOutputBytes > 65536 {
		return c, errors.New("fixed SOURCE ceilings")
	}
	// Validate the exact existing typed config shape with a disposable validation
	// instant ONLY. This value is never used as the runtime SOURCE deadline.
	s.Deadline = c.ControlDeadline
	if s.Timing != nil {
		total, _ := s.Timing.Total()
		frame, e := s.Timing.Bind(c.ControlDeadline.Add(-total))
		if e != nil {
			return c, e
		}
		s.TimingFrame = &frame
	}
	s.Pin = privateSourceConfigPin(s)
	b, e := json.Marshal(s)
	if e != nil {
		return c, e
	}
	if _, e = decodePrivateSourceHostConfig(b); e != nil {
		return c, e
	}
	return c, nil
}
func privateStartTimingConfigValid(s privateSourceHostConfig) bool {
	if s.Timing == nil {
		return s.Host.Mailbox.TimeoutMS > 0 && s.Host.Mailbox.TimeoutMS <= 5000 && s.Host.Mailbox.OverallTimeoutMS > 0 && s.Host.Mailbox.OverallTimeoutMS <= 5000
	}
	_, e := s.Timing.Total()
	return e == nil && s.Host.Mailbox.TimeoutMS == 60000 && s.Host.Mailbox.OverallTimeoutMS == 60000
}
func validatePrivateParentStart(raw, parentRaw, readyRaw []byte, c privateSourceStartConfig) (privateParentStartSignal, error) {
	var signal privateParentStartSignal
	var parent privateParentStartReady
	if e := privateStartCanonical(raw, &signal); e != nil {
		return signal, e
	}
	if e := privateStartCanonical(parentRaw, &parent); e != nil {
		return signal, e
	}
	readyHash := privateStartDigest(readyRaw)
	if parent.Schema != "private-parent-start-ready.v2" || parent.Invocation != c.Invocation || parent.DriverReadySHA256 != readyHash || parent.ProducerSession != c.ProducerSession || parent.ReviewerSession != c.ReviewerSession || !parent.ProducerWaiterActive || signal.Operation != "START_CONFIRM" || signal.Invocation != c.Invocation || !signal.ProducerWaiterActive || signal.DriverReadySHA256 != readyHash || signal.ParentReadySHA256 != privateStartDigest(parentRaw) || signal.SourceTemplatePin != privateSourceConfigPin(c.Source) {
		return signal, errors.New("parent start not ready or mismatched")
	}
	return signal, nil
}

// bindPrivateSourceStart sets the typed deadline/pin exactly once. Repeated
// binding, expired authorization and pre-populated clocks all refuse.
func bindPrivateSourceStart(c privateSourceHostConfig, start, expiry time.Time) (privateSourceHostConfig, error) {
	if !c.Deadline.IsZero() || c.TimingFrame != nil || c.Pin != "" || start.IsZero() || !expiry.After(start) {
		return c, errors.New("source clock already bound or expired")
	}
	if c.Timing == nil {
		c.Deadline = start.Add(15 * time.Second)
		if expiry.Before(c.Deadline) {
			c.Deadline = expiry
		}
	} else {
		frame, e := c.Timing.Bind(start)
		if e != nil || frame.TotalDeadline.After(expiry) {
			return c, errors.New("HOST timing original authorization does not cover selected total")
		}
		c.TimingFrame = &frame
		c.Deadline = frame.TotalDeadline
	}
	c.Pin = privateSourceConfigPin(c)
	b, e := json.Marshal(c)
	if e != nil {
		return c, e
	}
	return decodePrivateSourceHostConfig(b)
}
func runPrivateSourceStart(args []string, stdout, stderr io.Writer) int {
	fail := func(class string) int { fmt.Fprintln(stderr, "private connected source:", class); return 1 }
	var startup *applicationStartupNotifier
	if len(args) == 4 && args[0] == "start-source" && args[2] == "--startup-notify-fd" {
		var e error
		startup, e = openApplicationStartupNotifier(args[3])
		if e != nil {
			return fail("STARTUP_NOTIFY_REFUSED")
		}
		defer startup.close()
		args = args[:2]
	}
	if len(args) != 2 || args[0] != "start-source" {
		return fail("INVALID_INVOCATION")
	}
	raw, e := readHostProductRequestRawV3(args[1])
	if e != nil {
		return fail("CONFIG_REFUSED")
	}
	c, e := decodePrivateStartConfig(raw)
	if e != nil {
		return fail("CONFIG_REFUSED")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root, e := publication.OpenRoot(c.ControlRoot)
	if e != nil {
		return fail("CONTROL_ROOT_REFUSED")
	}
	// All early closes are joined by the root guard. No successful SOURCE return
	// can precede the explicit close below and the original-context check.
	closed := false
	defer func() {
		if !closed {
			_ = root.Close()
		}
	}()
	owner, e := inv.OpenPrivateManagedInputOwnerV2(ctx, root, c.CanonicalControlJob, c.ControlHead, c.ControlVerification)
	if e != nil {
		return fail("CONTROL_OWNER_REFUSED")
	}
	receipt, e := owner.VerifyFixedProviderV2(ctx, c.ControlAttempt, c.ControlDeadline)
	if e != nil {
		return fail("CONTROL_VERIFICATION_RETAINED")
	}
	pending, e := inv.BeginProviderLaunchV2(ctx, receipt, c.LaunchAttempt, c.LaunchDeadline, c.Base, c.Source.Verification)
	if e != nil {
		return fail("CONTROL_LAUNCH_RESERVE_RETAINED")
	}
	defer pending.Close()
	// Frozen files and role-ready markers are read only AFTER the second reserve.
	for _, binding := range []struct{ name, want string }{{"FROZEN-PRODUCER.md", c.FrozenProducerSHA256}, {"FROZEN-REVIEWER.md", c.FrozenReviewerSHA256}, {"control/producer-ready.json", c.ProducerReadySHA256}, {"control/reviewer-ready.json", c.ReviewerReadySHA256}} {
		b, re := pending.ReadStartMetadataV2(binding.name)
		if re != nil || privateStartDigest(b) != binding.want {
			return fail("FROZEN_BINDING_REFUSED_RETAINED")
		}
	}
	ready := privateConnectedReady{"private-connected-source-ready.v2", c.Invocation, c.Pin, privateSourceConfigPin(c.Source), receipt.String(), c.LaunchDeadline}
	readyRaw, e := json.Marshal(ready)
	if e != nil {
		return fail("READY_ENCODING_REFUSED")
	}
	readyRaw = append(readyRaw, '\n')
	if e = pending.PublishStartReadyV2(readyRaw); e != nil {
		return fail("READY_PUBLICATION_RETAINED")
	}
	if e = notifyPrivateSourceReady(ctx, startup, readyRaw); e != nil {
		return fail("READY_TRANSPORT_RETAINED")
	}
	var startRaw []byte
	// The bounded paid wait is NOT a SOURCE window. Parent must separately bind
	// actual producer waiting; role-marker existence alone is never sufficient.
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	for i := uint64(0); i < 18000; i++ {
		startRaw, e = pending.ReadStartMetadataV2("control/start.json")
		if e == nil {
			break
		}
		if !errors.Is(e, os.ErrNotExist) {
			return fail("START_READ_REFUSED_RETAINED")
		}
		select {
		case <-ctx.Done():
			return fail("CANCELLED_RETAINED")
		case <-timer.C:
		}
	}
	if e != nil {
		return fail("START_WAIT_EXPIRED_RETAINED")
	}
	parentRaw, e := pending.ReadStartMetadataV2("control/parent-ready.json")
	if e != nil {
		return fail("PARENT_READY_REFUSED_RETAINED")
	}
	if _, e = validatePrivateParentStart(startRaw, parentRaw, readyRaw, c); e != nil {
		return fail("START_BINDING_REFUSED_RETAINED")
	}
	start := time.Now() // chosen ONCE, before create-only consumption or later effects
	if e = pending.ConsumeStartV2(startRaw); e != nil {
		return fail("START_ALREADY_CONSUMED_RETAINED")
	}
	sourceRoot, e := publication.OpenRoot(c.Source.AccountingRoot)
	if e != nil {
		return fail("SOURCE_OWNER_REFUSED")
	}
	sourceTransferred := false
	defer func() {
		if !sourceTransferred {
			_ = sourceRoot.Close()
		}
	}()
	sourceOwner, e := inv.OpenPrivateManagedInputOwnerV2(ctx, sourceRoot, c.Source.CanonicalJob, c.Source.ExpectedHead, c.Source.Verification)
	if e != nil {
		return fail("SOURCE_OWNER_REFUSED")
	}
	expiry, e := pending.SourceAuthorizationExpiryV2(ctx, sourceOwner)
	if e != nil {
		return fail("SOURCE_EXPIRY_REFUSED_RETAINED")
	}
	c.Source, e = bindPrivateSourceStart(c.Source, start, expiry)
	if e != nil {
		return fail("SOURCE_CONFIG_REFUSED")
	}
	sourceCtx, cancel := context.WithDeadline(ctx, c.Source.Deadline)
	defer cancel()
	if e = pending.FinishProviderLaunchV2(sourceCtx, c.Source.ProcessPath); e != nil {
		return fail("PROVIDER_GENERATION_REFUSED_RETAINED")
	}
	closed = true
	sourceTransferred = true
	return finishPrivateSourceStart(sourceCtx, c, stdout, stderr, root, sourceRoot, sourceOwner, nil)
}

// Same admitted anonymous-pipe domain and <=PIPE_BUF single-attempt write as
// application activation, but never relabel SOURCE as an application packet.
// This carries no semantic text, approval, credit, or readiness from markers.
func notifyPrivateSourceReady(ctx context.Context, n *applicationStartupNotifier, raw []byte) (err error) {
	if n == nil {
		return nil
	}
	if n.attempted || n.file == nil || len(raw) == 0 || len(raw) > applicationStartupPacketLimit {
		return errors.New("source readiness transport")
	}
	n.attempted = true
	defer func() { err = errors.Join(err, n.close()) }()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	fd := int(n.file.Fd())
	if err = unix.SetNonblock(fd, true); err != nil {
		return err
	}
	wrote, err := unix.Write(fd, raw)
	if err != nil {
		return err
	}
	if wrote != len(raw) {
		return io.ErrShortWrite
	}
	return ctx.Err()
}

// Invocation-local test boundary only; production supplies nil. Implementations
// live in _test.go and delegate the real owned close, not a substitute root.
type privateStartFinalCloseIO interface {
	Close(*publication.Root) error
	Now() time.Time
}

// Takes ownership of both accounting roots after successful launch Observe.
// Admission is decided after the final owned control close, never from the
// successful launch operation's deliberately canceled execution context.
func finishPrivateSourceStart(sourceCtx context.Context, c privateSourceStartConfig, stdout, stderr io.Writer, root, sourceRoot *publication.Root, sourceOwner *inv.PrivateManagedInputOwnerV2, fixture privateStartFinalCloseIO) int {
	closeRoot, now := root.Close, time.Now
	if fixture != nil {
		closeRoot = func() error { return fixture.Close(root) }
		now = fixture.Now
	}
	closeErr := closeRoot()
	afterClose := now()
	if closeErr != nil || sourceCtx.Err() != nil || !afterClose.Before(c.Source.Deadline) || !afterClose.Before(c.LaunchDeadline) {
		_ = sourceRoot.Close()
		fmt.Fprintln(stderr, "private connected source:", "CONTROL_FINAL_CLOSE_RETAINED")
		return 1
	}
	return executePrivateSourceHostOwned(sourceCtx, c.Source, stdout, stderr, sourceRoot, sourceOwner)
}
