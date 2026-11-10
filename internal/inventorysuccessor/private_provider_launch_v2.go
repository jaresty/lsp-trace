//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"lsp-trace/internal/publication"
)

const providerStartPollsV2 uint64 = 18000 //180s /10ms, never an unbounded wait

// PendingProviderLaunchV2 is a separately reserved, original-context operation;
// neither a completed pin debit nor a JSON request can construct it.
type PendingProviderLaunchV2 struct{ state *privateProviderLaunchStateV2 }
type privateProviderLaunchStateV2 struct {
	mu                                                   sync.Mutex
	receipt                                              PrivateProviderControlV2
	ctx                                                  context.Context
	cancel                                               context.CancelFunc
	deadline                                             time.Time
	attempt, head, base                                  string
	root                                                 *publication.Root
	metadata                                             *providerControlFileV2
	startFound                                           bool
	io                                                   *providerControlIOV2
	reads                                                map[string]uint64
	sourceVerification                                   PrivateManagedInputVerificationV2
	consumed, finished, failed, expiryRead, readyWritten bool
}

// FixedProviderLaunchQuoteV2 pays for the CLOSED metadata schedule, retention,
// source authorization verification, provider generation recheck, publication,
// all error cleanup, and control reserve/observe. No provider/source requests or
// source-window credit is added. These are logical bounds, not physical FIT.
func FixedProviderLaunchQuoteV2(control, source PrivateManagedInputVerificationV2, d time.Duration) ([14]uint64, error) {
	c, e := providerLaunchChargeV2(verificationLimitsV2{control.Records, control.Bytes, control.Work}, source, d)
	return c.vector(), e
}
func providerLaunchChargeV2(control verificationLimitsV2, source PrivateManagedInputVerificationV2, d time.Duration) (countersV2, error) {
	if _, e := budgetV2(control); e != nil {
		return countersV2{}, e
	}
	sl := verificationLimitsV2{source.Records, source.Bytes, source.Work}
	if _, e := budgetV2(sl); e != nil {
		return countersV2{}, e
	}
	if d <= 0 || d > 180*time.Second {
		return countersV2{}, ErrPrivateProviderControlV2
	}
	// Each bounded publication primitive may traverse <=129 namespace nodes;
	//2048 logical calls per boundary includes metadata/ACL/close/error cleanup.
	// Poll input is <=4096+EOF; six other named records plus config <=128KiB.
	const bytes = providerStartPollsV2*4097 + 128*1024
	const boundaries = providerStartPollsV2 + 16
	c := countersV2{LogicalBytes: 8*bytes + 32768, ReadHashBytes: 4 * bytes, WriteBytes: 2*(recordLimitV2+2*headLimitV2) + 8192, Records: 4*boundaries + 4*129*512 + 128*1024, ParserBytes: 4 * bytes, Work: 8*bytes + boundaries*2048, ActiveNS: uint64(d), Calls: boundaries * 2048, OutputBytes: 8192}
	return addCountersV2(c, countersV2{ReadHashBytes: 2*control.Bytes + 3*sl.Bytes, ParserBytes: 2*control.Bytes + 3*sl.Bytes, Records: 2*control.Records + 3*sl.Records, Work: 2*control.Work + 3*sl.Work, Calls: 2*control.Work + 3*sl.Work})
}

func BeginProviderLaunchV2(ctx context.Context, receipt PrivateProviderControlV2, attempt string, deadline time.Time, base string, sourceVerification PrivateManagedInputVerificationV2) (PendingProviderLaunchV2, error) {
	fail := func(e error) (PendingProviderLaunchV2, error) {
		return PendingProviderLaunchV2{}, errors.Join(ErrPrivateProviderControlV2, e)
	}
	if receipt.CheckCurrent(ctx) != nil || !hashV2(attempt) || !filepath.IsAbs(base) || filepath.Clean(base) != base || len(base) > 4096 {
		return fail(nil)
	}
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		return fail(nil)
	}
	o := receipt.state.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != receipt.state.head {
		return fail(nil)
	}
	now := time.Now()
	charge, e := providerLaunchChargeV2(o.limits, sourceVerification, deadline.Sub(now))
	if e != nil {
		return fail(e)
	}
	h, e := o.k.reserve(ctx, o.job, o.head, attempt, charge, deadline.UnixNano(), now.UnixNano(), o.limits)
	if e != nil {
		if errors.Is(e, errUncertainV2) {
			o.uncertain = true
		}
		return fail(e)
	}
	o.head = h.Identity
	execution, cancel := context.WithDeadline(ctx, deadline)
	// Admission is after confirmed reservation. Existing private Root primitives
	// own their descriptor cleanup, namespace bounds, and no-follow selectors.
	root, e := publication.OpenRoot(base)
	if e != nil {
		cancel()
		return fail(e)
	}
	if e = root.ValidatePrivate(); e != nil {
		e = errors.Join(e, root.Close())
		cancel()
		return fail(e)
	}
	s := &privateProviderLaunchStateV2{receipt: receipt, ctx: execution, cancel: cancel, deadline: deadline, attempt: attempt, head: h.Identity, root: root, base: base, io: &providerControlIOV2{ctx: execution, left: charge.Calls - 2*o.limits.Work - 3*sourceVerification.Work}, reads: map[string]uint64{}, sourceVerification: sourceVerification}
	s.metadata, e = s.io.open(filepath.Join(base, "control"))
	if e == nil {
		e = s.io.admitDirectory(s.metadata.file)
	}
	if e != nil {
		if s.metadata != nil {
			e = errors.Join(e, s.metadata.Close())
		}
		e = errors.Join(e, root.Close())
		cancel()
		return fail(e)
	}
	return PendingProviderLaunchV2{s}, nil
}
func (s *privateProviderLaunchStateV2) live() bool {
	if s.failed || s.finished || s.ctx.Err() != nil || !time.Now().Before(s.deadline) {
		return false
	}
	o := s.receipt.state.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.uncertain && o.head == s.head
}

// ReadStartMetadataV2 admits only this fixed finite schedule, not caller paths,
// arbitrary source, filesystem browsing, or an executor. Every attempted read,
// including missing-start polling and EOF, consumes its prepaid slot FIRST.
func (p PendingProviderLaunchV2) ReadStartMetadataV2(name string) ([]byte, error) {
	if p.state == nil {
		return nil, ErrPrivateProviderControlV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	var limit int64
	max := uint64(1)
	switch name {
	case "control/start.json":
		limit = 4096
		max = providerStartPollsV2
	case "control/driver-ready.json", "control/parent-ready.json":
		limit = 16384
	case "control/producer-ready.json", "control/reviewer-ready.json":
		limit = 4096
	case "FROZEN-PRODUCER.md", "FROZEN-REVIEWER.md":
		limit = 8192
	case "source-config.json":
		limit = 65536
	default:
		return nil, ErrPrivateProviderControlV2
	}
	if !s.live() || s.reads[name] >= max || (name == "control/start.json" && s.startFound) {
		return nil, ErrPrivateProviderControlV2
	}
	s.reads[name]++
	raw, e := s.readMetadata(name, limit)
	if name == "control/start.json" && e == nil {
		s.startFound = true
	}
	return raw, e
}

// The retained control-directory FD avoids repeated ancestor ACL reads on every
// missing-start poll. Every leaf open prepays its own eventual close first.
func (s *privateProviderLaunchStateV2) openMetadata(name string) (*providerControlFileV2, error) {
	if !strings.HasPrefix(name, "control/") {
		return s.io.open(filepath.Join(s.base, name))
	}
	if s.metadata == nil {
		return nil, ErrPrivateProviderControlV2
	}
	if e := s.io.admitDirectory(s.metadata.file); e != nil {
		return nil, e
	}
	if e := s.io.prepayOpen(); e != nil {
		return nil, e
	}
	fd, e := unix.Openat(int(s.metadata.file.Fd()), filepath.Base(name), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	s.io.closeCredits++
	f := &providerControlFileV2{file: os.NewFile(uintptr(fd), filepath.Join(s.base, name)), io: s.io, name: filepath.Base(name)}
	if e = s.io.spend(false); e != nil {
		return nil, errors.Join(e, f.Close())
	}
	var scratch [4096]byte
	if e = s.io.spend(false); e != nil {
		return nil, errors.Join(e, f.Close())
	}
	if e = localDirectoryACLAdmissionV1(f.file, &scratch); e != nil {
		return nil, errors.Join(e, f.Close())
	}
	return f, nil
}

// New launch metadata uses raw single-attempt stat/read just like the provider;
// no os.File.Stat EINTR loop and no unbounded io.ReadAll call is introduced.
func (s *privateProviderLaunchStateV2) readMetadata(name string, limit int64) ([]byte, error) {
	f, e := s.openMetadata(name)
	if e != nil {
		return nil, e
	}
	before, e := s.io.stat(f.file)
	if e != nil || before.Size < 0 || before.Size > limit || before.Mode&syscall.S_IFMT != syscall.S_IFREG || before.Mode&0777 != 0600 || before.Nlink != 1 || before.Uid != uint32(os.Geteuid()) {
		return nil, errors.Join(ErrPrivateProviderControlV2, e, f.Close())
	}
	calls := uint64(1)
	if before.Size > 0 {
		calls, _ = localReadCallBoundV1(uint64(before.Size))
	}
	raw := make([]byte, 0, int(before.Size))
	buf := make([]byte, localScratchV1)
	complete := false
	for i := uint64(0); i < calls; i++ {
		remaining := limit + 1 - int64(len(raw))
		size := int64(len(buf))
		if remaining < size {
			size = remaining
		}
		if size <= 0 {
			e = ErrPrivateProviderControlV2
			break
		}
		var n int
		n, e = f.Read(buf[:size])
		if n < 0 || n > int(size) {
			e = ErrPrivateProviderControlV2
			break
		}
		raw = append(raw, buf[:n]...)
		if int64(len(raw)) > limit {
			e = ErrPrivateProviderControlV2
			break
		}
		if e == io.EOF {
			e = nil
			complete = true
			break
		}
		if e != nil || n == 0 {
			break
		}
	}
	after, se := s.io.stat(f.file)
	ce := f.Close()
	if e != nil || se != nil || ce != nil || !complete || int64(len(raw)) != before.Size || !sameProviderGenerationV2(&before, &after) || s.ctx.Err() != nil {
		return nil, errors.Join(ErrPrivateProviderControlV2, e, se, ce, s.ctx.Err())
	}
	return raw, nil
}
func (p PendingProviderLaunchV2) PublishStartReadyV2(raw []byte) error {
	if p.state == nil {
		return ErrPrivateProviderControlV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.live() || s.readyWritten || len(raw) == 0 || len(raw) > 4096 {
		return ErrPrivateProviderControlV2
	}
	s.readyWritten = true
	receipt, e := publication.PublishBoundFile(s.root, "control/connected-ready.json", raw, func(b []byte) error {
		if string(b) != string(raw) {
			return ErrPrivateProviderControlV2
		}
		return nil
	})
	if e == nil && (receipt == nil || receipt.VerificationStatus != "VERIFIED" || receipt.CloseStatus != publication.CloseComplete || receipt.DirectorySyncStatus != publication.DirectorySyncComplete) {
		e = ErrPrivateProviderControlV2
	}
	if e != nil {
		s.failed = true
	}
	return e
}

// ConsumeStartV2 is an irreversible create-only claim, not caller authentication.
// It runs after semantic binding checks and BEFORE freshness/source effects.
// Even malformed downstream work cannot erase the claim or obtain another window.
func (p PendingProviderLaunchV2) ConsumeStartV2(raw []byte) error {
	if p.state == nil {
		return ErrPrivateProviderControlV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.live() || s.consumed || len(raw) == 0 || len(raw) > 4096 {
		return ErrPrivateProviderControlV2
	}
	s.consumed = true
	// Bound publication is create-only; an existing target is an error, not reuse.
	receipt, e := publication.PublishBoundFile(s.root, "control/start-consumed.json", raw, func(b []byte) error {
		if string(b) != string(raw) {
			return ErrPrivateProviderControlV2
		}
		return nil
	})
	if e == nil && (receipt == nil || receipt.VerificationStatus != "VERIFIED" || receipt.CloseStatus != publication.CloseComplete || receipt.DirectorySyncStatus != publication.DirectorySyncComplete) {
		e = ErrPrivateProviderControlV2
	}
	if e != nil {
		s.failed = true
	}
	return e
}

// SourceAuthorizationExpiryV2 is prepaid by this pending control operation.
// It verifies the source owner's exact current head and original authorization;
// no expiry is inferred from caller JSON or a new600s preparation clock.
func (p PendingProviderLaunchV2) SourceAuthorizationExpiryV2(ctx context.Context, source *PrivateManagedInputOwnerV2) (time.Time, error) {
	if p.state == nil || source == nil {
		return time.Time{}, ErrPrivateProviderControlV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.live() || !s.consumed || s.expiryRead || ctx == nil || ctx.Err() != nil {
		return time.Time{}, ErrPrivateProviderControlV2
	}
	s.expiryRead = true
	source.mu.Lock()
	defer source.mu.Unlock()
	want := verificationLimitsV2{s.sourceVerification.Records, s.sourceVerification.Bytes, s.sourceVerification.Work}
	if source.uncertain || source.k == nil || source.limits != want || source == s.receipt.state.owner || source.job.Identity == s.receipt.state.owner.job.Identity || source.job.ResourceRules == FixedProviderControlRulesV2() || source.k.root.Path() == s.receipt.state.owner.k.root.Path() {
		return time.Time{}, ErrPrivateProviderControlV2
	}
	h, e := source.k.reopen(s.ctx, source.job, source.limits)
	if e != nil || h.Identity != source.head || !readyV2(h) {
		return time.Time{}, ErrPrivateProviderControlV2
	}
	b, e := budgetV2(source.limits)
	if e != nil {
		return time.Time{}, e
	}
	a, e := source.k.currentAuthorization(s.ctx, h, b)
	if e != nil || s.ctx.Err() != nil || time.Now().UnixNano() >= a.Expires {
		return time.Time{}, ErrPrivateProviderControlV2
	}
	return time.Unix(0, a.Expires), nil
}

// FinishProviderLaunchV2 performs a NEW PAID metadata recheck, never a second
// hash pass from a closed debit. Namespace protection and actual dev/inode,
// size,mode,owner,timestamps must match the freshly verified pin generation.
// Both the provider FD and all launch metadata FDs close BEFORE Observe.
func (p PendingProviderLaunchV2) FinishProviderLaunchV2(ctx context.Context, providerPath string) error {
	if p.state == nil {
		return ErrPrivateProviderControlV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.live() || !s.consumed || !s.expiryRead || ctx == nil || ctx.Err() != nil || providerPath != s.receipt.state.pin.Path {
		return ErrPrivateProviderControlV2
	}
	s.finished = true
	f, e := s.io.open(providerPath)
	if e == nil {
		var stErr error
		st, stErr := s.io.stat(f.file)
		e = errors.Join(stErr, f.Close())
		if !sameProviderGenerationV2(&st, &s.receipt.state.identity) {
			e = errors.Join(e, ErrPrivateProviderControlV2)
		}
	}
	if s.metadata != nil {
		e = errors.Join(e, s.metadata.Close())
		s.metadata = nil
	}
	e = errors.Join(e, s.root.Close())
	s.root = nil
	if e != nil || s.io.closeCredits != 0 || s.ctx.Err() != nil || ctx.Err() != nil || !time.Now().Before(s.deadline) {
		s.failed = true
		return errors.Join(ErrPrivateProviderControlV2, e, s.ctx.Err(), ctx.Err())
	}
	o := s.receipt.state.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head {
		return ErrPrivateProviderControlV2
	}
	h, e := o.k.observe(s.ctx, o.job, s.head, s.attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if e != nil {
		o.uncertain = true
		return errors.Join(ErrPrivateProviderControlV2, e)
	}
	o.head = h.Identity
	if s.ctx.Err() != nil || ctx.Err() != nil || !time.Now().Before(s.deadline) {
		return ErrPrivateProviderControlV2
	}
	s.cancel()
	return nil
}

// Close never Observe/refunds a failed or abandoned start. Its original debit
// remains PENDING. It still closes owned metadata descriptors after expiry.
func (p PendingProviderLaunchV2) Close() error {
	if p.state == nil {
		return ErrPrivateProviderControlV2
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	var e error
	if s.metadata != nil {
		e = s.metadata.Close()
		s.metadata = nil
	}
	if s.root != nil {
		e = errors.Join(e, s.root.Close())
		s.root = nil
	}
	s.cancel()
	if !s.finished {
		s.failed = true
	}
	return e
}
