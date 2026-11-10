package inventorysuccessor

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/describeworker"
	pc "lsp-trace/internal/programc"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/targetpacket"
)

var ErrPrivateSourceLocalV1 = errors.New("private source local preparation refused")
var ErrPrivateSourceLocalDeliveryDisabledV1 = errors.New("Increment A local delivery disabled; no worker executed")

// Fixed-size declarations only. The original owner, never these scalars, grants P.
func sameLocalConfigV1(a, b *PrivateSourceLocalConfigV1) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	x, y := *a, *b
	if (x.Managed == nil) != (y.Managed == nil) {
		return false
	}
	if x.Managed != nil && x.Managed.state != y.Managed.state {
		return false
	}
	x.Managed = nil
	y.Managed = nil
	return x == y
}
func localPinsV1(c describeworker.Config) [6]describeworker.FilePin {
	return [6]describeworker.FilePin{c.Worker, c.Model, c.Library, c.SandboxExecutable, c.SandboxProfile, c.Grammar}
}

const localScratchV1 uint64 = 32768

// At most twelve namespace boundaries are reachable in the one-shot path;
// sixteen prepaid boundaries leave error-path slack. Each permits at most 129
// ancestry nodes, descriptor-wrapper/UID/ACL/stat/close work and fixed stage ops,
// conservatively bounded by 2048 operations. This is cost, never added capacity.
const localStageNamespaceCallsV1 uint64 = 16 * 2048

// The SAME finite allowance is quoted before P reserve and consumed before every
// underlying descriptor Read. Division/remainder avoids overflow in ceil(n/S).
// Positive short reads are permitted, but never create additional call credit.
func localReadCallBoundV1(length uint64) (uint64, error) {
	if length == 0 || length > (^uint64(0)>>1)-1 {
		return 0, ErrPrivateSourceLocalV1
	}
	calls := length / localScratchV1
	if length%localScratchV1 != 0 {
		calls++
	}
	// With S=32768 and length<=MaxInt64-1, calls+1 cannot overflow.
	return calls + 1, nil // ceil(length/S) data calls PLUS the rejecting EOF probe
}

// sourceLocalConfigChargeV1 is a closed A/B quote: one streaming read/hash
// plus rejecting byte per bound pin; six open/stat/recheck/close lifetimes; fixed
// streaming scratch; retained immutable config and planned prompt/temp/stdout/stderr
// capacities. No model-sized copy or profile parsing is permitted. The explicitly
// selected B adapter adds its process terms below. 64 parent-envelope passes cover the existing source-only
// continuation's encoding/hash/copy/store joins while metadata remains retained.
// Capacities are debits, NOT caller grants. B uses this SAME pending lifetime;
// the base preparation ActiveNS already prepays the entire remaining source window.
func sourceLocalConfigChargeV1(c PrivateSourceLocalConfigV1) (countersV2, error) {
	fail := func() (countersV2, error) { return countersV2{}, ErrPrivateSourceLocalV1 }
	n := uint64(0)
	calls := uint64(0)
	meta := uint64(4096)
	add := func(dst *uint64, v uint64) bool {
		if v > ^uint64(0)-*dst {
			return false
		}
		*dst += v
		return true
	}
	for i, p := range localPinsV1(c.Config) {
		if len(p.Path) == 0 || len(p.Path) > 4096 || !filepath.IsAbs(p.Path) || filepath.Clean(p.Path) != p.Path || !hashV2(p.SHA256) || c.Lengths[i] == 0 || c.Lengths[i] > uint64(^uint64(0)>>1)-1 {
			return fail()
		}
		// Each descriptor-relative ancestor open and close, two stats, bounded
		// reads plus rejecting read, hash initialization/finalization, leaf close.
		depth := uint64(strings.Count(p.Path, "/"))
		readCalls, err := localReadCallBoundV1(c.Lengths[i])
		if err != nil || depth > 128 || !add(&calls, 2*depth+8+readCalls) {
			return fail()
		}
		if !add(&n, c.Lengths[i]+1) || !add(&meta, 8*uint64(len(p.Path)+len(p.SHA256))) {
			return fail()
		}
	}
	l := c.Config.Limits
	if l.TimeoutMS <= 0 || l.TimeoutMS > 15000 || l.MaxTokens <= 0 || l.ContextTokens <= 0 || c.Config.ResponseVersion != describeworker.ResponseVersionV2 {
		return fail()
	}
	for _, s := range []string{c.Config.RuntimeIdentity, c.Config.AdapterIdentity, c.Config.ModelIdentity} {
		if len(s) == 0 || len(s) > 4096 || !add(&meta, 8*uint64(len(s))) {
			return fail()
		}
	}
	for _, v := range []int{l.WorkBytes, l.TempBytes, l.StdoutBytes, l.StderrBytes} {
		if v <= 0 || !add(&meta, uint64(v)) {
			return fail()
		}
	}
	if !add(&meta, localScratchV1) {
		return fail()
	}
	// B adds process/prompt/adapter/armed-response retention and finite process
	// operations to the SAME quote before reservation. No caller capacity is credit.
	if c.Config.AdapterIdentity == PrivateSourceLocalWorkerAdapterV1 {
		if l.TimeoutMS > 5000 || l.MaxTokens > 2048 || l.ContextTokens > 8192 || l.StdoutBytes > 2080 || l.StderrBytes > 2080-l.StdoutBytes || l.WorkBytes > 65536 || l.TempBytes > 65536 {
			return fail()
		}
		if _, e := localStageBytesV1(c); e != nil {
			return fail()
		}
		// 8 stdout copies cover raw, decoded semantic, canonical parse, armed
		// payload, escaped/base64 response and transient encoding. Prompt twice;
		// fixed 16 KiB covers exec argv/env, FD/channel state and scratch metadata.
		// Additional protected namespace work: <=16 checks, <=129 nodes
		// (root included), <=2048 FD/stat/ACL/UID/wrapper/cleanup operations.
		// Includes error/partial-creation cleanup, three retained FDs and infos,
		// and the bounded 4096-byte ACL scratch. Same original P, no new window.
		if !add(&meta, 8*uint64(localWorkerFrameCapV1(l.StdoutBytes))+2*uint64(l.TempBytes)+uint64(l.StderrBytes)+16384+1024+24*uint64(len(c.Config.Worker.Path)+128)+6144) || !add(&calls, 65536+localStageNamespaceCallsV1) {
			return fail()
		}
		if meta > 149536 {
			return fail()
		}
	}
	data := meta
	if !add(&data, n) || !add(&data, 64*(8<<20)) || data > ^uint64(0)/4 {
		return fail()
	}
	charge := countersV2{LogicalBytes: 4 * data, ReadHashBytes: 2*n + 64*(8<<20), WriteBytes: uint64(l.TempBytes) + uint64(l.WorkBytes), Records: 6 * 8, ParserBytes: 64 * (8 << 20), Work: 4 * data, Calls: calls + 64, OutputBytes: meta}
	if c.Config.AdapterIdentity == PrivateSourceLocalWorkerAdapterV1 {
		// One prep/request/response whole-frame wave, maximum 149536 transport
		// bytes; bounded SOURCE reads include EOF. 65536 process/metadata calls
		// cover 32 fixed check boundaries times <=2048 ancestor/FD operations;
		// local-armed Exchange never polls on a missing response. Charge 4096
		// logical work bytes per such operation, not merely a context deadline.
		// Worker-side artifact reads are not free because pin verification already
		// happened. The declared ADR7 decode envelope is charged separately too;
		// this is a logical configured envelope, not physical runtime qualification.
		modelWork := c.Lengths[1]
		for _, factor := range []uint64{uint64(l.ContextTokens + l.MaxTokens + 1), uint64(l.MaxTokens + 1)} {
			if modelWork > ^uint64(0)/factor {
				return fail()
			}
			modelWork *= factor
		}
		if !add(&modelWork, (65536+localStageNamespaceCallsV1)*4096) {
			return fail()
		}
		stageBytes, stageErr := localStageBytesV1(c)
		if stageErr != nil {
			return fail()
		}
		combined, err := addCountersV2(charge, countersV2{LogicalBytes: 8*149536 + 2*n + 2*stageBytes + localStageNamespaceCallsV1*4096, ReadHashBytes: 4*149536 + 2*n + localStageNamespaceCallsV1*4096, WriteBytes: 149536 + stageBytes + 16*4096, Records: 32 + localStageNamespaceCallsV1, ParserBytes: 8*149536 + localStageNamespaceCallsV1*4096, ProviderRequests: 2, ProviderMessages: 5, RequestBytes: 2*65536 + uint64(l.TempBytes), ResponseBytes: 16384 + uint64(l.StdoutBytes) + uint64(l.StderrBytes), WireBytes: 147456 + uint64(l.StdoutBytes) + uint64(l.StderrBytes), Work: modelWork})
		if err != nil {
			return fail()
		}
		if c.Managed != nil {
			extra, e := managedUseQuoteV1(c)
			if e != nil {
				return fail()
			}
			return addCountersV2(combined, extra)
		}
		return combined, nil
	}
	return charge, nil
}

// PendingLocalSourceRV1 is deliberately NOT PreparedSourceRV1. No conversion or
// observed proof exists. Copies share one consumable owner-local lifetime, original
// job/head/attempt/context/deadline, and sticky failure. Zero and retired values
// cannot expose metadata or perform IO. Future B belongs inside this lifetime,
// before Finalize; it must not reserve, reopen a closed debit, or mint Prepared.
type PendingLocalSourceRV1 struct {
	state *privateSourceRPreparationStateV1
}

func ExecuteLocalRPreparationV1(ctx context.Context, op PrivateSourceRPreparationOperationV1, q PrivateSourceRPreparationRequestV1) (PendingLocalSourceRV1, error) {
	if _, err := executeRMetadataV1(ctx, op, q, true); err != nil {
		return PendingLocalSourceRV1{}, err
	}
	return PendingLocalSourceRV1{op.state}, nil
}
func (s *privateSourceRPreparationStateV1) localLive() bool {
	if s == nil || s.owner == nil || s.local == nil || s.finalized || s.metadata == nil || s.execution == nil || s.execution.Err() != nil || !time.Now().Before(s.request.Deadline) {
		return false
	}
	if s.localManagedUse != nil && !s.localManagedUse.live(s) {
		return false
	}
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.uncertain && o.head == s.head && o.k != nil && o.k.root == s.request.Root
}
func (p PendingLocalSourceRV1) lockMetadata() (sourceRMetadataV1, func()) {
	if p.state == nil {
		return sourceRMetadataV1{}, func() {}
	}
	s := p.state
	s.mu.Lock()
	if !s.localLive() {
		s.mu.Unlock()
		return sourceRMetadataV1{}, func() {}
	}
	return sourceRMetadataV1{s.metadata}, s.mu.Unlock
}
func (p PendingLocalSourceRV1) Values() (rg.AdmittedInput, rg.GroupingPolicy, pc.GroupingEvaluationCasePrivate, pc.GroupingEvaluationResultPrivate, PrivateProducerOriginals, error) {
	m, unlock := p.lockMetadata()
	defer unlock()
	return m.Values()
}
func (p PendingLocalSourceRV1) SourceObjects() ([]sourceobject.Object, error) {
	m, unlock := p.lockMetadata()
	defer unlock()
	return m.SourceObjects()
}
func (p PendingLocalSourceRV1) OriginalQueryTargetID(u []targetpacket.PrivateSourceUnit) (string, error) {
	m, unlock := p.lockMetadata()
	defer unlock()
	return m.OriginalQueryTargetID(u)
}
func (p PendingLocalSourceRV1) RetainedPayloadUpper() (uint64, bool) {
	m, unlock := p.lockMetadata()
	defer unlock()
	n, ok := m.RetainedPayloadUpper()
	if !ok {
		return 0, false
	}
	c, e := sourceLocalConfigChargeV1(*p.state.local)
	if e != nil || c.OutputBytes > ^uint64(0)-n {
		return 0, false
	}
	return n + c.OutputBytes, true
}

// privateLocalFileV1 is only an IO seam, not an executor or funding callback.
// Production always opens the exact no-follow private pin. Tests wrap the same
// descriptor to observe reads/closes and inject errors at the actual IO boundary.
type privateLocalFileV1 interface {
	Read([]byte) (int, error)
	Stat() (os.FileInfo, error)
	Close() error
}
type privateLocalIOV1 interface {
	open(string) (privateLocalFileV1, error)
}
type localOSIOV1 struct{}

func (localOSIOV1) open(path string) (privateLocalFileV1, error) { return openLocalPinV1(path) }

// VerifyPins is fixed, one-shot and context-aware. Stat length/mode checks occur
// before any read/hash, including the rejecting extra byte. No ReadFile/io.Copy,
// profile interpretation, source browse, dependencies, or execution occurs.
func (p PendingLocalSourceRV1) VerifyPins(ctx context.Context) (err error) {
	if p.state == nil {
		return ErrPrivateSourceLocalV1
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.localLive() || s.verified || s.localVerificationStarted {
		s.localFailure = errors.Join(s.localFailure, ErrPrivateSourceLocalV1)
		return ErrPrivateSourceLocalV1
	}
	s.localVerificationStarted = true
	defer func() {
		if err != nil {
			s.localFailure = errors.Join(s.localFailure, err)
		}
	}()
	ioOwner := s.local.io
	if ioOwner == nil {
		ioOwner = localOSIOV1{}
	}
	localWorker := s.local.Config.AdapterIdentity == PrivateSourceLocalWorkerAdapterV1
	if localWorker && s.local.Managed == nil {
		if e := s.prepareLocalStageV1(); e != nil {
			return e
		}
	}
	if s.local.Managed != nil {
		if e := s.bindManagedV1(ctx); e != nil {
			return e
		}
		s.verified = true
		return nil
	}
	buf := make([]byte, int(localScratchV1))
	for i, pin := range localPinsV1(s.local.Config) {
		if ctx == nil || ctx.Err() != nil || !s.localLive() {
			return ErrPrivateSourceLocalV1
		}
		f, e := ioOwner.open(pin.Path)
		if e != nil {
			return e
		}
		var stage *localStageWriterV1
		if localWorker {
			stage, e = s.stageWriterV1(i)
			if e != nil {
				return errors.Join(e, f.Close())
			}
		}
		var writers []io.Writer
		if stage != nil {
			writers = []io.Writer{stage}
		}
		e = verifyLocalFileV1(ctx, s, f, pin, s.local.Lengths[i], buf, writers...)
		// Seal the copy before closing the SAME verified source descriptor.
		closeErr := errors.Join(finishLocalStageWriterV1(stage, i), f.Close())
		if e != nil || closeErr != nil {
			return errors.Join(e, closeErr)
		}
	}
	if !s.localLive() {
		return ErrPrivateSourceLocalV1
	}
	s.verified = true
	return nil
}
func verifyLocalFileV1(ctx context.Context, s *privateSourceRPreparationStateV1, f privateLocalFileV1, pin describeworker.FilePin, length uint64, buf []byte, staged ...io.Writer) error {
	systemExecutor := s.local != nil && s.local.Config.AdapterIdentity == PrivateSourceLocalWorkerAdapterV1 && pin == s.local.Config.SandboxExecutable && pin.Path == localSystemSandboxV1 && !localWorkerFixtureV1(*s.local)
	return verifyBoundLocalFileV1(ctx, s.localLive, systemExecutor, f, pin, length, buf, staged...)
}

// Only the new managed raw-stat file implements this private comparison. Legacy
// A/copy-all keeps os.File.Stat and standard-library os.SameFile behavior.
type managedPinMetadataComparisonV1 interface {
	sameManagedPinMetadata(os.FileInfo, os.FileInfo) bool
}

func sameVerifiedLocalFileV1(f privateLocalFileV1, before, after os.FileInfo) bool {
	if raw, ok := f.(managedPinMetadataComparisonV1); ok {
		return raw.sameManagedPinMetadata(before, after)
	}
	return os.SameFile(before, after)
}
func verifyBoundLocalFileV1(ctx context.Context, live func() bool, systemExecutor bool, f privateLocalFileV1, pin describeworker.FilePin, length uint64, buf []byte, staged ...io.Writer) error {
	before, e := f.Stat()
	modeSafe := e == nil && before.Mode().IsRegular() && before.Mode().Perm()&0077 == 0
	if systemExecutor {
		modeSafe = trustedSystemSandboxInfoV1(before)
	}
	if fixed, ok := f.(interface{ providerModeSafeV2(os.FileInfo) bool }); ok && e == nil {
		modeSafe = fixed.providerModeSafeV2(before)
	}
	if e != nil || !modeSafe || before.Size() < 0 || uint64(before.Size()) != length {
		if _, raw := f.(managedPinMetadataComparisonV1); raw {
			return errors.Join(ErrPrivateSourceLocalV1, e)
		}
		return ErrPrivateSourceLocalV1
	}
	readCallsLeft, e := localReadCallBoundV1(length)
	if e != nil {
		return e
	}
	read := func(b []byte) (int, error) {
		if ctx.Err() != nil || !live() {
			return 0, ErrPrivateSourceLocalV1
		}
		if readCallsLeft == 0 {
			return 0, fmt.Errorf("%w: prepaid descriptor read-call ceiling exhausted", ErrPrivateSourceLocalV1)
		}
		readCallsLeft-- // debit BEFORE the underlying call, including EOF/errors
		return f.Read(b)
	}
	h := sha256.New()
	remaining := length
	for remaining > 0 {
		size := uint64(len(buf))
		if remaining < size {
			size = remaining
		}
		n, e := read(buf[:int(size)])
		if n < 0 || n > int(size) {
			return ErrPrivateSourceLocalV1
		}
		if n > 0 {
			h.Write(buf[:n])
			if len(staged) == 1 {
				written, we := staged[0].Write(buf[:n])
				if we != nil || written != n {
					return errors.Join(we, io.ErrShortWrite)
				}
			}
			remaining -= uint64(n)
		}
		if e != nil && !(e == io.EOF && remaining == 0) {
			return errors.Join(ErrPrivateSourceLocalV1, e)
		}
		if n == 0 && e == nil {
			return io.ErrNoProgress
		}
	}
	n, e := read(buf[:1])
	if n != 0 || e != io.EOF {
		return errors.Join(ErrPrivateSourceLocalV1, e)
	}
	after, e := f.Stat()
	if e != nil || !sameVerifiedLocalFileV1(f, before, after) || after.Size() != before.Size() || after.Mode() != before.Mode() || !after.ModTime().Equal(before.ModTime()) || fmt.Sprintf("sha256:%x", h.Sum(nil)) != pin.SHA256 {
		if _, raw := f.(managedPinMetadataComparisonV1); raw {
			return errors.Join(ErrPrivateSourceLocalV1, e)
		}
		return ErrPrivateSourceLocalV1
	}
	return nil
}

// Finalize never returns a preparation or publication proof. The operator calls
// it only after its last owned root close. Failed cleanup/time/binding keeps the
// real PENDING head and cumulative debit; no synthetic READY/reset/refund exists.
func (p PendingLocalSourceRV1) Finalize(ctx context.Context, prior error) error {
	if p.state == nil {
		return ErrPrivateSourceLocalV1
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finalized {
		return ErrPrivateSourceLocalV1
	}
	live := s.localLive()
	prior = errors.Join(prior, s.cleanupLocalStageV1())
	prior = errors.Join(prior, s.releaseManagedV1(prior == nil && s.localFailure == nil && live))
	s.finalized = true
	if s.cancel != nil {
		defer s.cancel()
	}
	if prior != nil || s.localFailure != nil || !live || !s.verified || ctx == nil || ctx.Err() != nil {
		return errors.Join(ErrPrivateSourceLocalV1, prior, s.localFailure)
	}
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.uncertain || o.head != s.head || s.execution.Err() != nil || !time.Now().Before(s.request.Deadline) {
		return ErrPrivateSourceLocalV1
	}
	h, e := o.k.observe(s.execution, o.job, s.head, s.request.Attempt, "OBSERVED", time.Now().UnixNano(), o.limits)
	if e != nil {
		o.uncertain = true
		return errors.Join(ErrPrivateSourceLocalV1, e)
	}
	o.head = h.Identity
	if s.execution.Err() != nil || !time.Now().Before(s.request.Deadline) {
		o.uncertain = true
		return ErrPrivateSourceLocalV1
	}
	return nil
}

// Legacy observed-only accessors. Pending metadata never passes through this type.
func (p PreparedSourceRV1) MatchesOriginalQueryTarget(u targetpacket.PrivateSourceUnit) bool {
	return (sourceRMetadataV1{p.state}).MatchesOriginalQueryTarget(u)
}
func (p PreparedSourceRV1) OriginalQueryTargetID(u []targetpacket.PrivateSourceUnit) (string, error) {
	return (sourceRMetadataV1{p.state}).OriginalQueryTargetID(u)
}
func (p PreparedSourceRV1) RetainedPayloadUpper() (uint64, bool) {
	return (sourceRMetadataV1{p.state}).RetainedPayloadUpper()
}
func (p PreparedSourceRV1) SourceObjects() ([]sourceobject.Object, error) {
	return (sourceRMetadataV1{p.state}).SourceObjects()
}
func (p PreparedSourceRV1) Values() (rg.AdmittedInput, rg.GroupingPolicy, pc.GroupingEvaluationCasePrivate, pc.GroupingEvaluationResultPrivate, PrivateProducerOriginals, error) {
	return (sourceRMetadataV1{p.state}).Values()
}
