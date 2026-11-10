//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"lsp-trace/internal/describeworker"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"
)

var ErrPrivateProviderControlV2 = errors.New("fixed private provider control refused; retain effects")

const fixedProviderPathV2 = "/Users/schwa/.local/bin/gopls"
const fixedProviderLengthV2 uint64 = 41038194
const fixedProviderDigestV2 = "sha256:82600f8b7f6ad337120b27ab738a5e44897ff1701919bc4620836ff7d98c2f7a"
const fixedProviderVersionV2 = "v0.23.0"

// PrivateProviderControlV2 is invocation-local completed custody, not SOURCE
// admission. No exported field, JSON, digest, or zero value can construct it.
type PrivateProviderControlV2 struct {
	state *privateProviderControlStateV2
}
type privateProviderControlStateV2 struct {
	owner    *PrivateManagedInputOwnerV2
	head     string
	deadline time.Time
	pin      describeworker.FilePin
	identity syscall.Stat_t
}

// CheckCurrent checks only this invocation's connected owner identity, not the
// durable kernel chain or a fresh provider generation. It performs NO I/O and
// confers NO launch permission. A launch consumer must establish its own paid
// freshness binding; this receipt cannot spend a completed control debit again.
func (r PrivateProviderControlV2) CheckCurrent(ctx context.Context) error {
	s := r.state
	if s == nil || ctx == nil || ctx.Err() != nil || !time.Now().Before(s.deadline) {
		return ErrPrivateProviderControlV2
	}
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head {
		return ErrPrivateProviderControlV2
	}
	return nil
}

// FixedProviderControlQuoteV2 is a quote, never credit. All fourteen positions
// are explicit; unused provider/wire currencies are zero. Verification terms
// derive from the opener's trusted finite limits, not historical global maxima.
const providerNamespaceBytesV2 uint64 = 129 * (4096 + 512) // bounded ACL bytes and raw stat metadata
const providerNamespaceRecordsV2 uint64 = 129 * (512 + 1)  // <=512 ACL entries plus stat per node

func FixedProviderControlQuoteV2(l PrivateManagedInputVerificationV2, duration time.Duration) ([14]uint64, error) {
	c, e := providerControlChargeV2(verificationLimitsV2{l.Records, l.Bytes, l.Work}, fixedProviderLengthV2, fixedProviderPathV2, duration)
	return c.vector(), e
}
func providerControlChargeV2(l verificationLimitsV2, n uint64, path string, d time.Duration) (countersV2, error) {
	if _, e := budgetV2(l); e != nil {
		return countersV2{}, e
	}
	reads, e := localReadCallBoundV1(n)
	if e != nil || n > fixedProviderLengthV2 || !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 || strings.Count(path, "/") > 128 || d <= 0 || d > 15*time.Second {
		return countersV2{}, ErrPrivateProviderControlV2
	}
	// Namespace: <=129 nodes; each open/stat/ACL/close is prepaid, plus
	// error cleanup. Read/hash is one streaming pass and a rejecting EOF byte.
	// Initial owner reopen plus reserve/observe verifications and bounded publications match
	// the existing kernel contract. This is logical work, NOT physical FIT.
	const namespace uint64 = 129 * 8
	data := n + 1 + localScratchV1 + providerNamespaceBytesV2
	c := countersV2{LogicalBytes: 2 * data, ReadHashBytes: n + 1 + providerNamespaceBytesV2, ParserBytes: providerNamespaceBytesV2, WriteBytes: 2 * (recordLimitV2 + 2*headLimitV2), Records: 8 + providerNamespaceRecordsV2, Work: 2 * data, ActiveNS: uint64(d), Calls: namespace + reads + 8, OutputBytes: 4096}
	return addCountersV2(c, countersV2{ReadHashBytes: 3 * l.Bytes, ParserBytes: 3 * l.Bytes, Records: 3 * l.Records, Work: 3 * l.Work, Calls: 3 * l.Work})
}

// FixedProviderControlRulesV2 identifies this separate owned control job. It
// cannot consume a SOURCE job's third window or reuse a spent acquisition.
func FixedProviderControlRulesV2() string {
	return hashLabelV2("fixed-provider-control/v2\x00" + fixedProviderPathV2 + "\x00" + fixedProviderDigestV2 + "\x00" + fixedProviderVersionV2)
}

// VerifyFixedProviderV2 never launches the provider and never reads source.
func (o *PrivateManagedInputOwnerV2) VerifyFixedProviderV2(ctx context.Context, attempt string, deadline time.Time) (PrivateProviderControlV2, error) {
	return o.verifyProviderControlV2(ctx, attempt, deadline, describeworker.FilePin{Path: fixedProviderPathV2, SHA256: fixedProviderDigestV2}, fixedProviderLengthV2)
}
func (o *PrivateManagedInputOwnerV2) verifyProviderControlV2(ctx context.Context, attempt string, deadline time.Time, pin describeworker.FilePin, length uint64, fixture ...providerControlFaultV2) (PrivateProviderControlV2, error) {
	fail := func(e error) (PrivateProviderControlV2, error) {
		return PrivateProviderControlV2{}, errors.Join(ErrPrivateProviderControlV2, e)
	}
	if o == nil || ctx == nil || ctx.Err() != nil || !hashV2(attempt) || !hashV2(pin.SHA256) {
		return fail(nil)
	}
	now := time.Now()
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		return fail(nil)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.k == nil || o.uncertain || o.job.ResourceRules != FixedProviderControlRulesV2() {
		return fail(nil)
	}
	charge, e := providerControlChargeV2(o.limits, length, pin.Path, deadline.Sub(now))
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
	defer cancel()
	ioOwner := &providerControlIOV2{ctx: execution, left: charge.Calls - 3*o.limits.Work}
	if len(fixture) == 1 {
		ioOwner.fixture = fixture[0]
	} // only same-package _test callers supply this
	f, e := ioOwner.open(pin.Path)
	if e != nil {
		return fail(e)
	}
	e = verifyBoundLocalFileV1(execution, func() bool { return execution.Err() == nil && time.Now().Before(deadline) }, false, f, pin, length, make([]byte, localScratchV1))
	closeErr := f.Close() // final owned pin FD; ancestors already closed by open
	if e != nil || closeErr != nil || ioOwner.closeCredits != 0 || execution.Err() != nil || !time.Now().Before(deadline) {
		return fail(errors.Join(e, closeErr, execution.Err()))
	}
	h, e = o.k.observe(execution, o.job, o.head, attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if e != nil {
		o.uncertain = true
		return fail(e)
	}
	o.head = h.Identity
	if execution.Err() != nil || !time.Now().Before(deadline) {
		return fail(execution.Err())
	}
	return PrivateProviderControlV2{&privateProviderControlStateV2{owner: o, head: h.Identity, deadline: deadline, pin: pin, identity: f.last}}, nil
}

// This counted descriptor implementation is private to the fixed operation.
// Every underlying attempt, including cleanup/EOF/errors, spends before I/O;
// there is no EINTR retry and no os.File.Stat retry hidden behind the budget.
// Invocation-local raw boundary seam, never exposed through public config/API.
// Its implementations live only in _test.go; production has no alternate IO.
type providerControlFaultV2 interface {
	providerFstat(uintptr, *syscall.Stat_t) error
	providerRead(uintptr, []byte) (int, error)
	providerClose(*os.File) error
}
type providerControlIOV2 struct {
	ctx          context.Context
	left         uint64
	closeCredits uint64 // prepaid before successful descriptor creation
	fixture      providerControlFaultV2
}

func (p *providerControlIOV2) spend(cleanup bool) error {
	if p.left == 0 || (!cleanup && p.ctx.Err() != nil) {
		return ErrPrivateProviderControlV2
	}
	p.left--
	return nil
}
func (p *providerControlIOV2) prepayOpen() error {
	if e := p.spend(false); e != nil {
		return e
	}
	return p.spend(false)
}
func (p *providerControlIOV2) close(f *os.File) error {
	if p.closeCredits == 0 {
		return ErrPrivateProviderControlV2
	}
	p.closeCredits--
	if p.fixture != nil {
		return p.fixture.providerClose(f)
	}
	return f.Close()
}
func (p *providerControlIOV2) stat(f *os.File) (syscall.Stat_t, error) {
	var st syscall.Stat_t
	if e := p.spend(false); e != nil {
		return st, e
	}
	c, e := f.SyscallConn()
	if e != nil {
		return st, e
	}
	var se error
	e = c.Control(func(fd uintptr) {
		if p.fixture != nil {
			se = p.fixture.providerFstat(fd, &st)
		} else {
			se = singleLocalDirectoryFstatV1(fd, &st)
		}
	})
	return st, errors.Join(e, se)
}
func (p *providerControlIOV2) admitDirectory(f *os.File) error {
	st, e := p.stat(f)
	if e != nil || !protectedLocalDirectoryInfoV1(&st, false) {
		return errors.Join(ErrPrivateProviderControlV2, e)
	}
	if e = p.spend(false); e != nil {
		return e
	}
	var scratch [4096]byte
	if e = p.spend(false); e != nil {
		return e
	} // second bounded ACL syscall, prepaid before primitive
	return localDirectoryACLAdmissionV1(f, &scratch)
}
func (p *providerControlIOV2) open(path string) (*providerControlFileV2, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if e := p.prepayOpen(); e != nil {
		return nil, e
	}
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	p.closeCredits++
	dir := os.NewFile(uintptr(fd), "/")
	for _, part := range parts[:len(parts)-1] {
		if e = p.admitDirectory(dir); e != nil {
			return nil, errors.Join(e, p.close(dir))
		}
		if e = p.prepayOpen(); e != nil {
			return nil, errors.Join(e, p.close(dir))
		}
		fd, e = unix.Openat(int(dir.Fd()), part, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_DIRECTORY, 0)
		if e == nil {
			p.closeCredits++
		}
		ce := p.close(dir)
		if e != nil {
			return nil, errors.Join(e, ce)
		}
		dir = os.NewFile(uintptr(fd), part)
		if ce != nil {
			return nil, errors.Join(ce, p.close(dir))
		}
	}
	if e = p.admitDirectory(dir); e != nil {
		return nil, errors.Join(e, p.close(dir))
	}
	if e = p.prepayOpen(); e != nil {
		return nil, errors.Join(e, p.close(dir))
	}
	fd, e = unix.Openat(int(dir.Fd()), parts[len(parts)-1], unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if e == nil {
		p.closeCredits++
	}
	ce := p.close(dir)
	if e != nil {
		return nil, errors.Join(e, ce)
	}
	f := os.NewFile(uintptr(fd), path)
	if ce != nil {
		return nil, errors.Join(ce, p.close(f))
	}
	if e = p.spend(false); e != nil {
		return nil, errors.Join(e, p.close(f))
	}
	var scratch [4096]byte
	if e = p.spend(false); e != nil {
		return nil, errors.Join(e, p.close(f))
	}
	if e = localDirectoryACLAdmissionV1(f, &scratch); e != nil {
		return nil, errors.Join(e, p.close(f))
	}
	return &providerControlFileV2{file: f, io: p, name: filepath.Base(path)}, nil
}

type providerControlFileV2 struct {
	file *os.File
	io   *providerControlIOV2
	name string
	last syscall.Stat_t
}

func (f *providerControlFileV2) Read(b []byte) (int, error) {
	if e := f.io.spend(false); e != nil {
		return 0, e
	}
	c, e := f.file.SyscallConn()
	if e != nil {
		return 0, e
	}
	var n int
	var re error
	e = c.Control(func(fd uintptr) {
		if f.io.fixture != nil {
			n, re = f.io.fixture.providerRead(fd, b)
		} else {
			n, re = unix.Read(int(fd), b)
		}
	})
	if e != nil || re != nil {
		return n, errors.Join(e, re)
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}
func (f *providerControlFileV2) Stat() (os.FileInfo, error) {
	st, e := f.io.stat(f.file)
	if e != nil {
		return nil, e
	}
	f.last = st
	return managedAdmissionInfoV1{f.name, &st}, nil
}
func (f *providerControlFileV2) Close() error { return f.io.close(f.file) }
func (f *providerControlFileV2) sameManagedPinMetadata(a, b os.FileInfo) bool {
	x, xok := a.(managedAdmissionInfoV1)
	y, yok := b.(managedAdmissionInfoV1)
	return xok && yok && sameProviderGenerationV2(x.stat, y.stat)
}

// ctime distinguishes a same-size in-place rewrite even if mtime is restored.
// Raw stat snapshots are already prepaid; atime intentionally is not identity.
func sameProviderGenerationV2(a, b *syscall.Stat_t) bool {
	if !sameManagedFileV1(a, b) {
		return false
	}
	x, y := reflect.ValueOf(a).Elem(), reflect.ValueOf(b).Elem()
	c, d := x.FieldByName("Ctimespec"), y.FieldByName("Ctimespec")
	if !c.IsValid() {
		c = x.FieldByName("Ctim")
		d = y.FieldByName("Ctim")
	}
	return c.IsValid() && d.IsValid() && reflect.DeepEqual(c.Interface(), d.Interface())
}
func (f *providerControlFileV2) providerModeSafeV2(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && info.Mode().Perm()&0022 == 0 && info.Mode().Perm()&0111 != 0 && (st.Uid == 0 || st.Uid == uint32(os.Getuid()))
}

func (r PrivateProviderControlV2) String() string {
	if r.state == nil {
		return "INVALID"
	}
	return fmt.Sprintf("fixed-provider-control/v2 %s %s %s", r.state.pin.Path, fixedProviderVersionV2, r.state.head)
}
