//go:build darwin || linux

package sourceoperator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/publication"
)

type publicationOwner struct {
	limit, nativeUpper uint64
	upstream, metadata uint64
	admitted           bool
	outputs            [2]*publication.ContentRetentionRoot
	source             *publication.SourceProtectionRootPrivateV2
	protected          []*publication.ObservedRoot
}

func (p *publicationOwner) close() error {
	if p == nil {
		return nil
	}
	var err error
	for _, r := range p.outputs {
		err = errors.Join(err, r.Close())
	}
	for _, r := range p.protected {
		err = errors.Join(err, r.Close())
	}
	err = errors.Join(err, p.source.Close())
	return err
}
func (p *publicationOwner) topology(ctx context.Context) error {
	if p == nil {
		return ErrRefused
	}
	return publication.CheckProducerPacketRetentionTopologyPrivateV2(ctx, p.outputs, p.source, p.protected)
}
func openPublication(ctx context.Context, q Request) (p *publicationOwner, err error) {
	// Shape checks precede source effects. This does NOT reserve native headroom:
	// actual opaque upstream retention and remaining response uppers are combined
	// by admitNativeMetadata before HOST, then actual delivery metadata before seal.
	if q.Contract.CaptureLimits().MaxParentBytes <= 0 || q.Contract.CaptureLimits().MaxParentBytes > nativeMaxBytes || q.Contract.PacketMaxResponseBytes() <= 0 || q.Contract.PacketMaxResponseBytes() > 1<<20 || q.ResponseLimits.MaxAdapterBytes > 1<<20 || q.ResponseLimits.MaxSemanticBytes > 1<<20 {
		return nil, ErrRefused
	}
	metadata, e := operatorMetadataUpper(q)
	if e != nil {
		return nil, e
	}
	p = &publicationOwner{limit: uint64(q.Contract.CaptureLimits().MaxParentBytes), metadata: metadata}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.close())
			p = nil
		}
	}()
	p.source, err = publication.OpenSourceProtectionRootPrivateV2(ctx, q.Capture.Start.Profile.Workspace().String())
	if err != nil {
		return p, err
	}
	for _, path := range []string{q.Capture.Root.Path(), q.Host.Mailbox.Directory} {
		var r *publication.ObservedRoot
		r, err = publication.OpenObservedRoot(ctx, path)
		if err != nil {
			return p, fmt.Errorf("protected root %s: %w", path, err)
		}
		p.protected = append(p.protected, r)
	}
	for i, path := range q.OutputRoots {
		p.outputs[i], err = publication.OpenContentRetentionRoot(ctx, path)
		if err != nil {
			return p, err
		}
	}
	err = p.topology(ctx)
	return p, err
}

// Checked logical live payload bound. Canonical supplied/delivery are trusted
// json.Marshal output: RawMessage embedding cannot expand their existing JSON.
// Semantic is a canonical upper; human includes joined limitations and fixed text
// (3*S+2048), and its JSON string needs at most 6 bytes per source byte.
// Eight W slots cover the native raw, encoder storage/returned serialization,
// newline growth, publication expected clone, exact-FD reread, published reread,
// and explicit readback (including ReadAll growth/old buffers). Four H and four S
// cover formatting/join and accessor/decoded mirrors. Two copies of supplied and
// delivery cover their held originals and accessor results. Sequential roots do
// not retain two waves. This is logical byte liveness, not heap/GC or CPU FIT.
func nativeAggregateUpper(upstream, supplied, delivery, semantic, gaps, cap uint64) (wire, peak uint64, err error) {
	bad := func() (uint64, uint64, error) { return 0, 0, errors.Join(ErrRefused, ErrNativeAggregate) }
	if upstream == 0 || supplied == 0 || delivery == 0 || semantic == 0 || cap == 0 {
		return bad()
	}
	add := func(a, b uint64) (uint64, bool) { return a + b, a <= ^uint64(0)-b }
	mul := func(a, b uint64) (uint64, bool) { return a * b, b == 0 || a <= ^uint64(0)/b }
	human, ok := mul(semantic, 3)
	if !ok {
		return bad()
	}
	human, ok = add(human, 2048)
	if !ok {
		return bad()
	}
	escaped, ok := mul(human, 6)
	if !ok {
		return bad()
	}
	wire = 8192
	for _, v := range []uint64{supplied, delivery, semantic, escaped, gaps} {
		wire, ok = add(wire, v)
		if !ok {
			return bad()
		}
	}
	if wire > nativeMaxBytes {
		return bad()
	}
	peak = upstream
	for _, term := range [][2]uint64{{wire, 8}, {human, 4}, {semantic, 4}, {supplied, 2}, {delivery, 2}, {32768, 1}} {
		v, good := mul(term[0], term[1])
		if !good {
			return bad()
		}
		peak, ok = add(peak, v)
		if !ok {
			return bad()
		}
	}
	if peak > cap {
		return 0, 0, fmt.Errorf("%w: upstream=%d wire=%d peak=%d limit=%d", errors.Join(ErrRefused, ErrNativeAggregate), upstream, wire, peak, cap)
	}
	return wire, peak, nil
}

func nativeGapUpper(source *sc.PrivateAllModeSourcesResult) (uint64, error) {
	if source == nil || len(source.Outcomes) > 1000 {
		return 0, ErrRefused
	}
	upper := uint64(0)
	for _, o := range source.Outcomes {
		n := uint64(512)
		for _, s := range []string{o.Status, o.Gap, o.AdmissionID, o.RequestID} {
			if uint64(len(s)) > (^uint64(0)-n)/6 {
				return 0, ErrRefused
			}
			n += 6 * uint64(len(s))
		}
		if n > ^uint64(0)-upper {
			return 0, ErrRefused
		}
		upper += n
	}
	return upper, nil
}

// Sizing only: no DTO or successful arithmetic can mint HOST/source custody.
// Each phase is checked with ALL external retained values before its fixed work.
func (p *publicationOwner) admitHostMetadata(prepared uint64, phases sc.SourceHostPhaseBounds) error {
	if p == nil || p.limit == 0 || p.metadata == 0 || prepared == 0 {
		return ErrRefused
	}
	for _, peak := range []uint64{phases.Source, phases.Lower, phases.Binder} {
		if peak == 0 || prepared > ^uint64(0)-peak {
			return ErrRefused
		}
		combined := peak + prepared
		if p.metadata > ^uint64(0)-combined || combined+p.metadata > p.limit {
			return fmt.Errorf("%w: prepared=%d metadata=%d host-phase=%d limit=%d", ErrRefused, prepared, p.metadata, peak, p.limit)
		}
	}
	return nil
}

func (p *publicationOwner) admitNativeMetadata(source *sc.PrivateAllModeSourcesResult, prepared, held, supplied, delivery, semantic uint64) error {
	if p == nil || prepared == 0 || held == 0 {
		return ErrRefused
	}
	upstream := p.metadata
	for _, n := range []uint64{prepared, held} {
		if n > ^uint64(0)-upstream {
			return ErrRefused
		}
		upstream += n
	}
	gaps, e := nativeGapUpper(source)
	if e != nil {
		return e
	}
	wire, _, e := nativeAggregateUpper(upstream, supplied, delivery, semantic, gaps, p.limit)
	if e != nil {
		return e
	}
	p.upstream, p.nativeUpper, p.admitted = upstream, wire, true
	return nil
}

// Fixed owner/job and root metadata plus actual variable configuration lengths.
// At most five topology chains; charge 4KiB per actual path component for held
// and recheck metadata, and 6x path/config bytes. No filesystem or payload reads.
func operatorMetadataUpper(q Request) (uint64, error) {
	n := uint64(128 << 10)
	add := func(v uint64) bool {
		if v > ^uint64(0)-n {
			return false
		}
		n += v
		return true
	}
	for _, path := range []string{q.Capture.Start.Profile.Workspace().String(), q.Capture.Root.Path(), q.Host.Mailbox.Directory, q.OutputRoots[0], q.OutputRoots[1]} {
		if len(path) > 8192 || !add(uint64(strings.Count(path, "/")+1)*4096) || !add(uint64(len(path))*6) {
			return 0, ErrRefused
		}
	}
	for _, s := range []string{q.CaptureAttempt, q.PreparationAttempt, q.RoleAttempt, q.RunID, q.SeedNamespace, q.Capture.URI, q.Capture.OccurrenceID, q.Capture.Revision, q.Capture.Privacy, q.Capture.Start.LanguageID, q.Capture.Start.Process.Path, q.Capture.Start.Process.Dir, q.Host.Protocol, q.Host.Mailbox.Invocation, q.Host.Mailbox.InterpreterRole, q.Host.Mailbox.ReviewerRole, q.Host.Mailbox.ScopeSHA256, q.Host.Mailbox.ApprovalSHA256} {
		if uint64(len(s)) > (^uint64(0)-n)/6 {
			return 0, ErrRefused
		}
		n += 6 * uint64(len(s))
	}
	for _, list := range [][]string{q.Capture.Start.Process.Args, q.Capture.Start.Process.Env, q.Capture.Start.RelationProviders} {
		for _, s := range list {
			if uint64(len(s)) > (^uint64(0)-n)/6 {
				return 0, ErrRefused
			}
			n += 6 * uint64(len(s))
		}
	}
	return n, nil
}

// Independent no-replace publications, NOT an atomic pair or power-loss proof.
// On partial failure preserve the flags and committed object; no retry or refund.
func (p *publicationOwner) publish(ctx context.Context, n nativeOutcome, out *Outcome) error {
	if n.id == "" || len(n.raw) == 0 || len(n.raw) > nativeMaxBytes || uint64(len(n.raw)) > p.limit || uint64(len(n.raw)) > p.nativeUpper {
		return ErrRefused
	}
	for i, r := range p.outputs {
		if e := p.topology(ctx); e != nil {
			return e
		}
		receipt, e := r.PublishPostPreparationPrivateV2(ctx, n.id, n.raw)
		if receipt != nil {
			out.Published[i] = receipt.VerificationStatus == "VERIFIED"
		}
		if e != nil {
			return e
		}
		if !out.Published[i] || receipt.CloseStatus == publication.CloseFailed {
			return ErrRefused
		}
		got, e := r.ReadPostPreparationPrivateV2(ctx, n.id, int64(len(n.raw)))
		if e != nil {
			return e
		}
		if !bytes.Equal(got, n.raw) {
			return ErrRefused
		}
		out.Readback[i] = true
	}
	if e := p.topology(ctx); e != nil {
		return e
	}
	return publication.CheckPostPreparationObjectsDistinctPrivateV2(ctx, p.outputs, n.id)
}
