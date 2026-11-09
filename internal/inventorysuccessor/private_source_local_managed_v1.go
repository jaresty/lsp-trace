//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"lsp-trace/internal/describeworker"
)

type localManagedGenerationV1 struct {
	mu                sync.Mutex
	owner             *PrivateManagedInputOwnerV2
	head, attempt     string
	ctx               context.Context
	cancel            context.CancelFunc
	deadline          time.Time
	stage             privateSourceRPreparationStateV1 // namespace machinery only; no SOURCE proof fabricated
	config            PrivateSourceLocalConfigV1
	dependencies      [16]PrivateSourceLocalDependencyV1
	count             int
	paths             [22]string
	files             [22]*os.File
	infos             [22]*syscall.Stat_t
	fileCount         int
	maxUses, issued   uint64
	uses              map[*localManagedUseV1]struct{}
	revoked, closed   bool
	retirementStarted bool
}
type localManagedUseV1 struct {
	g             *localManagedGenerationV1
	head, attempt string
	cancel        context.CancelFunc
	released      bool // protected by g.mu
}

// Existing kernel verification is bounded separately from artifact bytes. As in
// privateManagedInputEnvelopeChargeV2, reserve and Observe each consume a full
// owner verification envelope. Sixteen additional head-sized allowances cover
// both commits' canonical sealing and bounded validation/readback callbacks.
// Eight byte passes cover UTF-8/decode/validation/unsigned+signed marshal/hash/
// comparison/copies conservatively. Bounds describe cost, never supply credit.
// budgetV2 validates the trusted owner limits BEFORE arithmetic or any IO; its
// hard maxima make these products finite. addCountersV2 checks the final sum.
func managedLifecycleVerificationChargeV1(l verificationLimitsV2) (countersV2, error) {
	if _, e := budgetV2(l); e != nil {
		return countersV2{}, e
	}
	data := 8 * (2*l.Bytes + 16*headLimitV2)
	return countersV2{LogicalBytes: data, ReadHashBytes: data, WriteBytes: 4 * (recordLimitV2 + 2*headLimitV2), Records: 2*l.Records + 16, ParserBytes: data, Work: data + 2*l.Work + 64, Calls: 2*l.Work + 64, OutputBytes: 8*(headLimitV2+recordLimitV2) + 128*l.Records}, nil
}
func managedArtifactQuoteV1(q PrivateSourceLocalManagedRequestV1, now time.Time, trusted verificationLimitsV2) (countersV2, error) {
	fail := func() (countersV2, error) { return countersV2{}, ErrPrivateSourceLocalV1 }
	if q.Config.Config.AdapterIdentity != PrivateSourceLocalWorkerAdapterV1 || q.Config.Managed != nil || q.Count < 0 || q.Count > len(q.Dependencies) || q.MaxUses == 0 || q.MaxUses > (^uint64(0)-65536)/4096 || q.MaxBytes == 0 || !hashV2(q.Attempt) || !hashV2(q.ExpectedHead) || !now.Before(q.Deadline) {
		return fail()
	}
	c := q.Config
	c.managedAdmission = true
	if _, e := sourceLocalConfigChargeV1(c); e != nil {
		return fail()
	}
	total, calls := uint64(0), uint64(65536)
	pins := localPinsV1(c.Config)
	names := map[string]bool{filepath.Base(c.Config.Library.Path): true}
	for i := 0; i < 6+q.Count; i++ {
		var p describeworker.FilePin
		var n uint64
		if i < 6 {
			p = pins[i]
			n = c.Lengths[i]
		} else {
			dep := q.Dependencies[i-6]
			p = dep.Pin
			n = dep.Bytes
			name := filepath.Base(p.Path)
			if filepath.Dir(p.Path) != filepath.Dir(c.Config.Library.Path) || names[name] || name == "." || name == ".." {
				return fail()
			}
			names[name] = true
		}
		if !filepath.IsAbs(p.Path) || filepath.Clean(p.Path) != p.Path || len(p.Path) > 4096 || strings.Count(p.Path, "/") > 128 || !hashV2(p.SHA256) {
			return fail()
		}
		r, e := localReadCallBoundV1(n)
		if e != nil || n > q.MaxBytes-total {
			return fail()
		}
		total += n
		if calls > ^uint64(0)-r-4096-managedAdmissionMetadataCallsV1 {
			return fail()
		}
		calls += r + 4096 + managedAdmissionMetadataCallsV1
	}
	for i := q.Count; i < len(q.Dependencies); i++ {
		if q.Dependencies[i] != (PrivateSourceLocalDependencyV1{}) {
			return fail()
		}
	}
	// Admission verifies and copies once, retains read-only files/FDs until explicit
	// retirement, and prepays fixed-name deletion, revocation and bounded metadata.
	// OutputBytes is retained storage, not SOURCE privacy capacity. All fourteen
	// currencies are explicit; no provider/transport effect occurs in this operation.
	if calls > ^uint64(0)/4096 || q.MaxUses > ^uint64(0)/4096 || calls*4096 > ^uint64(0)-q.MaxUses*4096 {
		return fail()
	}
	meta := calls*4096 + q.MaxUses*4096
	if total > (^uint64(0)-meta)/8 || total > ^uint64(0)-localScratchV1-65536 || meta > ^uint64(0)-total-localScratchV1-65536 {
		return fail()
	}
	verification, e := managedLifecycleVerificationChargeV1(trusted)
	if e != nil {
		return fail()
	}
	return addCountersV2(countersV2{LogicalBytes: 8*total + meta, ReadHashBytes: 2*total + meta, WriteBytes: total + 16*4096, Records: calls, ParserBytes: 16384, Work: 8*total + meta, ActiveNS: uint64(q.Deadline.Sub(now)), Calls: calls, OutputBytes: total + meta + localScratchV1 + 65536}, verification)
}

// Admission holds its own original reservation through retention and deletion.
// It cannot borrow either SOURCE window, observe early then reuse a closed debit,
// issue credit, or turn a digest into custody. Caller must supply an already-owned
// separately funded job. Missing/insufficient funding refuses before file IO.
func (o *PrivateManagedInputOwnerV2) AdmitLocalArtifactsV1(ctx context.Context, q PrivateSourceLocalManagedRequestV1) (PrivateSourceLocalManagedV1, error) {
	fail := func() (PrivateSourceLocalManagedV1, error) {
		return PrivateSourceLocalManagedV1{}, ErrPrivateSourceLocalV1
	}
	if o == nil || ctx == nil || ctx.Err() != nil {
		return fail()
	}
	now := time.Now()
	quote, e := managedArtifactQuoteV1(q, now, o.limits)
	if e != nil {
		return fail()
	}
	if d, ok := ctx.Deadline(); ok && d.Before(q.Deadline) {
		return fail()
	}
	o.mu.Lock()
	if o.k == nil || o.uncertain || o.head != q.ExpectedHead {
		o.mu.Unlock()
		return fail()
	}
	h, e := o.k.reserve(ctx, o.job, q.ExpectedHead, q.Attempt, quote, q.Deadline.UnixNano(), now.UnixNano(), o.limits)
	if e != nil {
		if errors.Is(e, errUncertainV2) {
			o.uncertain = true
		}
		o.mu.Unlock()
		return fail()
	}
	o.head = h.Identity
	o.mu.Unlock()
	c := q.Config
	c.managedAdmission = true
	lifeCtx, lifeCancel := context.WithDeadline(ctx, q.Deadline)
	g := &localManagedGenerationV1{owner: o, head: h.Identity, attempt: q.Attempt, ctx: lifeCtx, cancel: lifeCancel, deadline: q.Deadline, config: q.Config, dependencies: q.Dependencies, count: q.Count, maxUses: q.MaxUses, uses: make(map[*localManagedUseV1]struct{})}
	g.stage.local = &c
	g.stage.request.Attempt = q.Attempt
	// Every acquired object remains owned even on partial admission. Failure cleanup
	// uses the same fixed-name machinery, never recursive enumeration/removal.
	cleanup := func(err error) (PrivateSourceLocalManagedV1, error) {
		defer lifeCancel()
		for _, f := range g.files {
			if f != nil {
				err = errors.Join(err, f.Close())
			}
		}
		for i := 0; i < g.count; i++ {
			if g.stage.localStage != nil && g.stage.localStage.lib != nil {
				err = errors.Join(err, unlinkManagedDependencyV1(g.stage.localStage.lib, filepath.Base(g.dependencies[i].Pin.Path)))
			}
		}
		err = errors.Join(err, g.stage.cleanupLocalStageV1())
		return PrivateSourceLocalManagedV1{}, err
	}
	if e = g.stage.prepareLocalStageV1(); e != nil {
		return cleanup(e)
	}
	live := func() bool { return ctx.Err() == nil && time.Now().Before(q.Deadline) }
	buf := make([]byte, localScratchV1)
	for i, pin := range localPinsV1(c.Config) {
		ioOwner := c.io
		if ioOwner == nil {
			ioOwner = localOSIOV1{}
		}
		if !live() {
			return cleanup(ErrPrivateSourceLocalV1)
		}
		f, e := ioOwner.open(pin.Path)
		if e != nil {
			return cleanup(e)
		}
		f = newManagedAdmissionFileV1(ctx, live, ioOwner, "base", pin.Path, f)
		w, e := g.stage.stageWriterV1(i)
		if e != nil {
			return cleanup(errors.Join(e, f.Close()))
		}
		system := i == 3 && !localWorkerFixtureV1(c)
		if w == nil {
			e = verifyBoundLocalFileV1(ctx, live, system, f, pin, c.Lengths[i], buf)
		} else {
			e = verifyBoundLocalFileV1(ctx, live, system, f, pin, c.Lengths[i], buf, w)
		}
		e = errors.Join(e, finishLocalStageWriterV1(w, i), f.Close())
		if e != nil {
			return cleanup(e)
		}
	}
	for i := 0; i < q.Count; i++ {
		d := q.Dependencies[i]
		ioOwner := c.io
		if ioOwner == nil {
			ioOwner = localOSIOV1{}
		}
		if !live() {
			return cleanup(ErrPrivateSourceLocalV1)
		}
		f, e := ioOwner.open(d.Pin.Path)
		if e != nil {
			return cleanup(e)
		}
		f = newManagedAdmissionFileV1(ctx, live, ioOwner, "dependency", d.Pin.Path, f)
		out, e := openLocalStageFileV1(g.stage.localStage.lib, filepath.Base(d.Pin.Path))
		if e != nil {
			return cleanup(errors.Join(e, f.Close()))
		}
		e = verifyBoundLocalFileV1(ctx, live, false, f, d.Pin, d.Bytes, buf, out)
		e = errors.Join(e, out.Sync(), out.Chmod(0400), out.Close(), f.Close())
		if e != nil {
			return cleanup(e)
		}
	}
	for i, p := range localPinsV1(g.stage.localStagedConfig) {
		g.paths[i] = p.Path
	}
	g.fileCount = 6 + q.Count
	for i := 0; i < q.Count; i++ {
		g.paths[6+i] = filepath.Join(g.stage.localStageDir, "lib", filepath.Base(q.Dependencies[i].Pin.Path))
	}
	for i := 0; i < g.fileCount; i++ {
		f, e := openLocalPinV1(g.paths[i])
		if e != nil {
			return cleanup(e)
		}
		g.files[i] = f
		st, e := statLocalNamespaceDirectoryV1(f, c.io)
		if e != nil {
			return cleanup(e)
		}
		g.infos[i] = st
	}
	if e = g.checkLocked(); e != nil {
		return cleanup(e)
	}
	return PrivateSourceLocalManagedV1{g}, nil
}

// Conservative per-use logical contract: at most eight complete retained-FD,
// path/ancestry/ACL checks, 2048 operations per file, metadata work charged at
// 4096 bytes/operation, plus dependency reads by the worker. Model reads/decode,
// prompts, process streams and cleanup remain in the original local P quote.
// This bounds logical operations, not physical mmap/page faults/RAM/CPU.
func managedUseQuoteV1(c PrivateSourceLocalConfigV1) (countersV2, error) {
	if c.Managed == nil || c.Managed.state == nil {
		return countersV2{}, ErrPrivateSourceLocalV1
	}
	g := c.Managed.state
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.liveLocked() || g.config.Config != c.Config || g.config.Lengths != c.Lengths {
		return countersV2{}, ErrPrivateSourceLocalV1
	}
	calls := uint64(8 * (6 + g.count) * 2048)
	meta := calls * 4096
	n := uint64(0)
	// Generation strings, FD arrays and dependency declarations are retained by
	// the separately funded lifecycle, not copied into a use. This output bound
	// covers two bounded directory-read buffers (16KiB), 1024 bytes/file for
	// entries/maps/stats/lease metadata, and four copies of all bounded path text.
	// The original local quote separately retains its namespace/exec scratch.
	output := uint64(16384 + 1024*g.fileCount)
	for i := 0; i < g.fileCount; i++ {
		output += 4 * uint64(len(g.paths[i]))
	}
	for i := 0; i < g.count; i++ {
		v := g.dependencies[i].Bytes
		if n > (^uint64(0)-meta)/4 || v > (^uint64(0)-meta)/4-n {
			return countersV2{}, ErrPrivateSourceLocalV1
		}
		n += v
	}
	return countersV2{LogicalBytes: meta + 4*n, ReadHashBytes: meta + n, Records: calls, Work: meta + 4*n, Calls: calls, OutputBytes: output}, nil
}
func (g *localManagedGenerationV1) liveLocked() bool {
	if g == nil || g.revoked || g.closed || g.ctx == nil || g.ctx.Err() != nil || !time.Now().Before(g.deadline) {
		return false
	}
	g.owner.mu.Lock()
	defer g.owner.mu.Unlock()
	return !g.owner.uncertain && g.owner.head == g.head
}
func (m *PrivateSourceLocalManagedV1) matches(c PrivateSourceLocalConfigV1) bool {
	if m == nil || m.state == nil {
		return false
	}
	g := m.state
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.liveLocked() && g.config.Config == c.Config && g.config.Lengths == c.Lengths
}
func localFileTimeV1(st *syscall.Stat_t) string {
	v := reflect.ValueOf(st).Elem()
	m := v.FieldByName("Mtimespec")
	if !m.IsValid() {
		m = v.FieldByName("Mtim")
	}
	return fmt.Sprint(m)
}
func unlinkManagedDependencyV1(dir *os.File, name string) error {
	e := unix.Unlinkat(int(dir.Fd()), name, 0)
	if errors.Is(e, unix.ENOENT) {
		return nil
	}
	return e
}
func sameManagedFileV1(a, b *syscall.Stat_t) bool {
	return a != nil && b != nil && a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Uid == b.Uid && a.Gid == b.Gid && a.Size == b.Size && a.Nlink == b.Nlink && localFileTimeV1(a) == localFileTimeV1(b)
}
func (g *localManagedGenerationV1) checkLocked() error {
	if !g.liveLocked() {
		return fmt.Errorf("%w: managed generation not live", ErrPrivateSourceLocalV1)
	}
	if e := g.stage.validateLocalStageV1(); e != nil {
		return fmt.Errorf("managed namespace: %w", e)
	}
	if e := g.checkEntriesLocked(); e != nil {
		return e
	}
	for i := 0; i < g.fileCount; i++ {
		st, e := statLocalNamespaceDirectoryV1(g.files[i], g.config.io)
		if e != nil || !sameManagedFileV1(g.infos[i], st) {
			return fmt.Errorf("%w: managed retained file %d: %v", ErrPrivateSourceLocalV1, i, e)
		}
		if i != 3 || localWorkerFixtureV1(g.config) {
			if st.Mode&0222 != 0 || st.Uid != uint32(os.Geteuid()) {
				return ErrPrivateSourceLocalV1
			}
		}
		// Path continuity is checked in the protected namespace against the retained
		// FD. No content read or digest replay occurs at use.
		f, e := openLocalPinV1(g.paths[i])
		if e != nil {
			return e
		}
		cur, e := statLocalNamespaceDirectoryV1(f, g.config.io)
		ce := f.Close()
		if e != nil || ce != nil || !sameManagedFileV1(st, cur) {
			return fmt.Errorf("%w: managed path file %d: %v", ErrPrivateSourceLocalV1, i, errors.Join(e, ce))
		}
	}
	return nil
}

// Exact finite directory contents, including the declared native closure. No
// unknown loader input may hide under the otherwise permitted read-only root.
func (g *localManagedGenerationV1) checkEntriesLocked() error {
	expected := map[string]bool{"worker": true, "model": true, "profile.sb": true, "grammar.gbnf": true, "lib": true}
	if localWorkerFixtureV1(g.config) {
		expected["sandbox"] = true
	}
	libs := map[string]bool{filepath.Base(g.config.Config.Library.Path): true}
	for i := 0; i < g.count; i++ {
		libs[filepath.Base(g.dependencies[i].Pin.Path)] = true
	}
	for _, item := range []struct {
		dir   *os.File
		names map[string]bool
	}{{g.stage.localStage.dir, expected}, {g.stage.localStage.lib, libs}} {
		fd, e := unix.Openat(int(item.dir.Fd()), ".", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_DIRECTORY, 0)
		if e != nil {
			return e
		}
		f := os.NewFile(uintptr(fd), "managed-list")
		entries, re := f.ReadDir(len(item.names) + 1)
		ce := f.Close()
		if (re != nil && re != io.EOF) || ce != nil || len(entries) != len(item.names) {
			return errors.Join(ErrPrivateSourceLocalV1, re, ce)
		}
		for _, entry := range entries {
			if !item.names[entry.Name()] {
				return ErrPrivateSourceLocalV1
			}
		}
	}
	return nil
}
func (s *privateSourceRPreparationStateV1) bindManagedV1(ctx context.Context) error {
	m := s.local.Managed
	if m == nil || m.state == nil {
		return ErrPrivateSourceLocalV1
	}
	g := m.state
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.owner == s.owner || (g.owner.k.root.Path() == s.owner.k.root.Path() && g.owner.job.Identity == s.owner.job.Identity) || g.issued >= g.maxUses || g.config.Config != s.local.Config || g.config.Lengths != s.local.Lengths {
		return fmt.Errorf("%w: managed use owner/config/capacity binding", ErrPrivateSourceLocalV1)
	}
	if e := g.checkLocked(); e != nil {
		return e
	}
	// This call is inside the already reserved original P and its one-shot VerifyPins.
	// Namespace and all model/worker/dependency reads are in P's code-derived quote.
	c := *s.local
	c.Managed = nil
	c.managedAdmission = true
	old := s.local
	s.local = &c
	e := s.prepareLocalStageV1()
	s.local = old
	if e != nil {
		return e
	}
	s.localStagedConfig = g.stage.localStagedConfig
	useCtx, cancel := context.WithDeadline(s.execution, g.deadline)
	previous := s.cancel
	s.cancel = func() {
		cancel()
		if previous != nil {
			previous()
		}
	}
	s.execution = useCtx
	u := &localManagedUseV1{g: g, head: s.head, attempt: s.request.Attempt, cancel: cancel}
	g.issued++
	g.uses[u] = struct{}{}
	s.localManagedUse = u
	return nil
}
func (u *localManagedUseV1) live(s *privateSourceRPreparationStateV1) bool {
	g := u.g
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.uses[u]
	return ok && !u.released && g.liveLocked() && s != nil && s.head == u.head && s.request.Attempt == u.attempt
}

// Linearize spawn with revocation. Once Revoke returns, no not-yet-started
// worker can cross this gate; already-started uses are cancelled and quarantined.
func (u *localManagedUseV1) start(ctx context.Context, cmd *exec.Cmd) error {
	g := u.g
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.uses[u]; !ok || u.released || ctx == nil || ctx.Err() != nil {
		return ErrPrivateSourceLocalV1
	}
	if e := g.checkLocked(); e != nil {
		return e
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return cmd.Start()
}
func (u *localManagedUseV1) check() error {
	g := u.g
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.uses[u]
	if !ok || u.released {
		return ErrPrivateSourceLocalV1
	}
	return g.checkLocked()
}
func (s *privateSourceRPreparationStateV1) releaseManagedV1(clean bool) error {
	u := s.localManagedUse
	if u == nil {
		return nil
	}
	g := u.g
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.checkLocked()
	if !clean || e != nil {
		// No receipt of certain process/attempt cleanup: quarantine the generation
		// and retain the lease. Retirement must not race an unjoined worker.
		g.revoked = true
		for active := range g.uses {
			active.cancel()
		}
		return errors.Join(ErrPrivateSourceLocalV1, e)
	}
	if u.released {
		return ErrPrivateSourceLocalV1
	}
	u.released = true
	delete(g.uses, u)
	s.localManagedUse = nil
	return e
}

// Revocation is immediate and sticky. Active uses are cancelled; retirement must
// wait for their owned cleanup, never delete shared files under a live worker.
func (m PrivateSourceLocalManagedV1) Revoke() error {
	if m.state == nil {
		return ErrPrivateSourceLocalV1
	}
	g := m.state
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.liveLocked() {
		return ErrPrivateSourceLocalV1
	}
	g.revoked = true
	for u := range g.uses {
		u.cancel()
	}
	return nil
}
func (m PrivateSourceLocalManagedV1) Retire(ctx context.Context) error {
	if m.state == nil || ctx == nil || ctx.Err() != nil {
		return ErrPrivateSourceLocalV1
	}
	g := m.state
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || g.retirementStarted || len(g.uses) != 0 {
		return ErrPrivateSourceLocalV1
	}
	// Revalidation may inspect a revoked generation for deletion, but never revives
	// it for a source use. All deletion is prepaid by this same pending lifecycle.
	g.retirementStarted = true // consume before any deletion/revalidation IO; no retry credit
	g.revoked = false
	e := g.checkLocked()
	g.revoked = true
	if e != nil {
		return e
	}
	g.revoked = true
	g.closed = true
	for _, f := range g.files {
		if f != nil {
			e = errors.Join(e, f.Close())
		}
	}
	for i := 0; i < g.count; i++ {
		e = errors.Join(e, unlinkManagedDependencyV1(g.stage.localStage.lib, filepath.Base(g.dependencies[i].Pin.Path)))
	}
	e = errors.Join(e, g.stage.cleanupLocalStageV1())
	if e != nil || g.ctx.Err() != nil || !time.Now().Before(g.deadline) {
		return errors.Join(ErrPrivateSourceLocalV1, e)
	}
	o := g.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != g.head {
		return ErrPrivateSourceLocalV1
	}
	h, e := o.k.observe(g.ctx, o.job, g.head, g.attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if e != nil {
		o.uncertain = true
		return e
	}
	o.head = h.Identity
	defer g.cancel()
	if ctx.Err() != nil || g.ctx.Err() != nil || !time.Now().Before(g.deadline) {
		o.uncertain = true
		return ErrPrivateSourceLocalV1
	}
	return nil
}
