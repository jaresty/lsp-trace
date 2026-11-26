package inventorysuccessor

import (
	"context"
	"errors"
	"math"
	"time"

	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
)

const PrivateSourceHOST60V2 = "HOST60_V2"

var ErrPrivateSourceTimingV2 = errors.New("private source timing refused")

// PrivateSourceTimingV2 is selected policy, not measured fit, credit, or custody.
// Nil at the consumer retains the legacy 15s capture / 5s HOST control.
// PublicationCleanupMS is mandatory: there is deliberately no default tail.
type PrivateSourceTimingV2 struct {
	Mode                                                   string
	CaptureMS, PreparationMS, HOSTMS, PublicationCleanupMS int64
}

func (p PrivateSourceTimingV2) Total() (time.Duration, error) {
	if p.Mode != PrivateSourceHOST60V2 || p.CaptureMS < 1 || p.CaptureMS > 15000 || p.PreparationMS < 1 || p.PreparationMS > 15000 || p.HOSTMS != 60000 || p.PublicationCleanupMS < 1 {
		return 0, ErrPrivateSourceTimingV2
	}
	const maxMS = int64(math.MaxInt64) / int64(time.Millisecond)
	subtotal := p.CaptureMS + p.PreparationMS + p.HOSTMS
	// Leave the existing M close allowance representable too.
	if p.PublicationCleanupMS > maxMS-subtotal-1000 {
		return 0, ErrPrivateSourceTimingV2
	}
	return time.Duration(subtotal+p.PublicationCleanupMS) * time.Millisecond, nil
}

func (p PrivateSourceTimingV2) Rules() string {
	if _, e := p.Total(); e != nil {
		return ""
	}
	return privateGroupingDigest([]byte("private-source-timing-v2|" + p.Mode + "|" + decimalTimingV2(p.CaptureMS) + "|" + decimalTimingV2(p.PreparationMS) + "|" + decimalTimingV2(p.HOSTMS) + "|" + decimalTimingV2(p.PublicationCleanupMS)))
}

// Frame is a declaration only. ReserveMixedInputV2 checks the canonical job,
// authorization expiry, both windows and all currencies before provider effects.
// Only completed M can bind a frame into opaque producer originals.
type PrivateSourceTimingFrameV2 struct {
	Profile                                 PrivateSourceTimingV2
	Started, CaptureDeadline, TotalDeadline time.Time
}

func (p PrivateSourceTimingV2) Bind(start time.Time) (PrivateSourceTimingFrameV2, error) {
	d, e := p.Total()
	if e != nil || start.IsZero() {
		return PrivateSourceTimingFrameV2{}, ErrPrivateSourceTimingV2
	}
	total := start.Add(d)
	if !total.After(start) || total.UnixNano() <= start.UnixNano() {
		return PrivateSourceTimingFrameV2{}, ErrPrivateSourceTimingV2
	}
	return PrivateSourceTimingFrameV2{p, start, start.Add(time.Duration(p.CaptureMS) * time.Millisecond), total}, nil
}
func (f PrivateSourceTimingFrameV2) Valid() bool {
	v, e := f.Profile.Bind(f.Started)
	return e == nil && v.CaptureDeadline.Equal(f.CaptureDeadline) && v.TotalDeadline.Equal(f.TotalDeadline)
}

func decimalTimingV2(n int64) string {
	// Bounded positive decimal fields; no formatting or caller-controlled strings.
	var a [20]byte
	i := len(a)
	for {
		i--
		a[i] = byte(n%10) + '0'
		n /= 10
		if n == 0 {
			return string(a[i:])
		}
	}
}

type privateSourceTimingStateV2 struct {
	frame               PrivateSourceTimingFrameV2
	preparationDeadline time.Time
	retainedContext     context.Context
}

func sourceTimingPreparationChargeV2(m rg.AllModeMetadataPrivate, now, deadline time.Time) (countersV2, error) {
	// Preserve the existing P bytes/records/work proof; only retained-lifetime
	// ActiveNS changes. The actual preparation executor keeps its <=15s child.
	short := deadline
	if short.After(now.Add(15 * time.Second)) {
		short = now.Add(15 * time.Second)
	}
	c, e := sourceRPreparationChargeV1(m, now, short)
	if e != nil || !now.Before(deadline) {
		return countersV2{}, ErrPrivateSourceTimingV2
	}
	c.ActiveNS = uint64(deadline.Sub(now))
	return c, nil
}
func sourceTimingMaximumPreparationV2(now, deadline time.Time) (countersV2, error) {
	var m rg.AllModeMetadataPrivate
	m.Input.ByteLength = rg.EvaluationMaxBytesPrivate
	m.Contributions = rg.EvaluationMaxOccurrencesPrivate
	m.Custody = 2 * rg.EvaluationMaxOccurrencesPrivate
	m.Predecessors = rg.EvaluationMaxOccurrencesPrivate
	return sourceTimingPreparationChargeV2(m, now, deadline)
}

func sourceTimingMixedChargeV2(f PrivateSourceTimingFrameV2, now time.Time, w sessionruntime.PrivateProviderWindowBounds, l verificationLimitsV2) (countersV2, error) {
	if _, e := budgetV2(l); e != nil {
		return countersV2{}, ErrPrivateSourceTimingV2
	}
	if w.SourceReadBytes > 8<<20 || w.RequestBytes > w.WireBytes || w.ResponseBytes > w.WireBytes || w.Frames > w.WireBytes {
		return countersV2{}, ErrPrivateSourceTimingV2
	}
	if !f.Valid() || now.Before(f.Started) || !now.Before(f.CaptureDeadline) {
		return countersV2{}, ErrPrivateSourceTimingV2
	}
	c, e := privateMixedChargeV2(now, f.CaptureDeadline, w)
	if e != nil {
		return countersV2{}, e
	}
	// Retained M material and its close allowance are explicitly prepaid. Two
	// extra bounded authorization/history checks admit BOTH M and worst-case P.
	extra := countersV2{ReadHashBytes: 2*l.Bytes + 4096, ParserBytes: 2*l.Bytes + 4096, LogicalBytes: 4096, Records: 2*l.Records + 16, Work: 2*l.Work + 4096, Calls: 2*l.Work + 16}
	a, b := c.vector(), extra.vector()
	for i := range a {
		if a[i] > math.MaxUint64-b[i] {
			return countersV2{}, ErrPrivateSourceTimingV2
		}
		a[i] += b[i]
	}
	c = countersFromV2(a)
	c.ActiveNS = uint64(f.TotalDeadline.Sub(now)) + uint64(time.Second)
	return c, nil
}

// PrivateSourceHOSTQuotesV2 quotes the full original lifetime and unchanged
// maximum P metadata, not an observed provider fit. W must be the same enforced
// envelope that will be admitted. Smaller/incompatible credit is refused before M.
func PrivateSourceHOSTQuotesV2(p PrivateSourceTimingV2, w sessionruntime.PrivateProviderWindowBounds, l PrivateManagedInputVerificationV2) (m, pendingP, sum, maxTranche [14]uint64, maxWindowNS uint64, err error) {
	now := time.Now()
	f, e := p.Bind(now)
	if e != nil {
		err = e
		return
	}
	mc, e := sourceTimingMixedChargeV2(f, now, w, verificationLimitsV2{l.Records, l.Bytes, l.Work})
	if e != nil {
		err = e
		return
	}
	pc, e := sourceTimingMaximumPreparationV2(now, f.TotalDeadline)
	if e != nil {
		err = e
		return
	}
	m, pendingP = mc.vector(), pc.vector()
	for i := range m {
		if m[i] > math.MaxUint64-pendingP[i] {
			err = ErrPrivateSourceTimingV2
			return
		}
		sum[i] = m[i] + pendingP[i]
		maxTranche[i] = max(m[i], pendingP[i])
	}
	maxWindowNS = uint64(f.TotalDeadline.Sub(now))
	return
}

func sourceTimingFitsV2(want, have countersV2) bool {
	w, h := want.vector(), have.vector()
	for i := range w {
		if w[i] > h[i] {
			return false
		}
	}
	return true
}

// Called under the original owner mutex before reserve. This is validation,
// neither another SOURCE reservation nor a future-P debit/refund.
func (o *PrivateManagedInputOwnerV2) sourceTimingFundingV2(ctx context.Context, f PrivateSourceTimingFrameV2, charge countersV2, now time.Time) error {
	if !f.Valid() || o.job.ResourceRules != f.Profile.Rules() {
		return ErrPrivateSourceTimingV2
	}
	h, e := o.k.reopen(ctx, o.job, o.limits)
	if e != nil || h.Identity != o.head || !readyV2(h) {
		return ErrPrivateSourceTimingV2
	}
	b, e := budgetV2(o.limits)
	if e != nil {
		return e
	}
	a, e := o.k.currentAuthorization(ctx, h, b)
	if e != nil {
		return e
	}
	future, e := sourceTimingMaximumPreparationV2(now, f.TotalDeadline)
	if e != nil {
		return e
	}
	// A HOST profile starts with the original READY SOURCE job and exactly two
	// windows. No third window, post-close debit reuse, or renewed authorization.
	if a.MaxWindows != 2 || h.Windows != 0 || a.MaxWindowNS < uint64(f.TotalDeadline.Sub(f.Started)) || a.Expires < f.TotalDeadline.UnixNano() || !sourceTimingFitsV2(charge, a.MaxTranche) || !sourceTimingFitsV2(future, a.MaxTranche) {
		return ErrPrivateSourceTimingV2
	}
	v, c, u := h.Charged.vector(), charge.vector(), future.vector()
	for i := range v {
		if v[i] > math.MaxUint64-c[i] {
			return ErrPrivateSourceTimingV2
		}
		v[i] += c[i]
		if v[i] > math.MaxUint64-u[i] {
			return ErrPrivateSourceTimingV2
		}
		v[i] += u[i]
	}
	if !sourceTimingFitsV2(countersFromV2(v), a.Ceiling) {
		return ErrPrivateSourceTimingV2
	}
	return nil
}
