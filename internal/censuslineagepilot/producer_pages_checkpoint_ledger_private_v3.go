//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"syscall"
)

// Fresh source acquisition is a separate CURRENT caller admission. Its maximum
// source policy work is charged to the SAME ledger BEFORE Freeze reads bytes.
// It does not restore an old source/capture/Manager capability from JSON.
func FreezeRetainedCheckpointSourcePrivateV3(ctx context.Context, spec ScopeSpecPrivateV2, current ValidatedCurrentLedgerPrivateV3, exchange PageCostPrivateV3) (FrozenScopePrivateV2, error) {
	return freezeRetainedCheckpointSourcePrivateV3(ctx, spec, current, exchange, scopeReadOps{})
}

func freezeRetainedCheckpointSourcePrivateV3(ctx context.Context, spec ScopeSpecPrivateV2, current ValidatedCurrentLedgerPrivateV3, exchange PageCostPrivateV3, ops scopeReadOps) (FrozenScopePrivateV2, error) {
	if ctx == nil || ctx.Err() != nil || !current.valid() {
		return FrozenScopePrivateV2{}, ErrInvalid
	}
	l := current.PageLedgerPrivateV3
	if l.state == nil {
		return FrozenScopePrivateV2{}, ErrInvalid
	}
	if e := l.state.reserve(checkpointReadCostPrivateV3(4*scopeMetadataPrivateV2), &exchange); e != nil {
		return FrozenScopePrivateV2{}, e
	}
	if _, _, e := validateScopeSpecPrivateV2(spec); e != nil {
		return FrozenScopePrivateV2{}, e
	}
	p := spec.EffectivePolicy
	m := len(spec.Members)
	if m < 1 || m > 16 || p.validate() != nil {
		return FrozenScopePrivateV2{}, ErrInvalid
	}
	n := uint64(p.TotalBytes)
	if x := uint64(m) * uint64(p.FileBytes); x < n {
		n = x
	}
	n += uint64(m) + scopeMetadataPrivateV2
	if e := l.state.reserve(PageCostPrivateV3{LogicalBytes: 24 * n, ReadHashBytes: 32 * n, Work: 2 * producerPacketTopologyWorkPrivateV2}, &exchange); e != nil {
		return FrozenScopePrivateV2{}, e
	}
	return freezeScopePrivateV2(ctx, spec, ops)
}

func retainedSourceReservationPrivateV3(f FrozenScopePrivateV2) (PageCostPrivateV3, error) {
	if !f.valid() {
		return PageCostPrivateV3{}, ErrInvalid
	}
	p := f.state.spec.EffectivePolicy
	m := len(f.state.files)
	if p.validate() != nil || m < 1 || m > 16 || m > p.MaxMembers {
		return PageCostPrivateV3{}, ErrInvalid
	}
	n := uint64(p.TotalBytes)
	if x := uint64(m) * uint64(p.FileBytes); x < n {
		n = x
	}
	n += uint64(m) + scopeMetadataPrivateV2
	return PageCostPrivateV3{LogicalBytes: 24 * n, ReadHashBytes: 32 * n, Work: 2 * producerPacketTopologyWorkPrivateV2}, nil
}

// Verify the complete canonical journal, bounded one record at a time. The
// frontier is frozen before verification charges are added, so this terminates.
// Newer charges are preserved; only the last checkpoint's data progress resumes.
func verifyRetainedLedgerChainPrivateV3(ctx context.Context, l PageLedgerPrivateV3, g uint64, tip string, left *PageCostPrivateV3) error {
	if g == ^uint64(0) || g+1 > l.state.allowance.Work/pageLedgerOverheadPrivateV3.Work {
		return ErrInvalid
	}
	one := checkpointReadCostPrivateV3(producerPageHeaderPrivateV3)
	count := g + 1
	vals := []uint64{one.LogicalBytes, one.ReadHashBytes, one.WriteBytes, one.Work}
	for _, v := range vals {
		if v != 0 && count > ^uint64(0)/v {
			return ErrInvalid
		}
	}
	cost := PageCostPrivateV3{one.LogicalBytes * count, one.ReadHashBytes * count, one.WriteBytes * count, one.Work * count}
	if e := l.state.reserve(cost, left); e != nil {
		return e
	}
	_, e := inspectRetainedLedgerChainPrivateV3(ctx, l, g, tip)
	return e
}

// No journal permission is derived from the unchecked chain. The caller has
// already debited a separate, explicitly bounded bootstrap or verified budget.
func inspectRetainedLedgerChainPrivateV3(ctx context.Context, l PageLedgerPrivateV3, g uint64, tip string) (PageCostPrivateV3, error) {
	previous := ""
	cumulative := PageCostPrivateV3{}
	for i := uint64(0); i <= g; i++ {
		if e := ctx.Err(); e != nil {
			return PageCostPrivateV3{}, e
		}
		info, e := l.state.root.Lstat(ledgerNamePrivateV3(i))
		if e != nil {
			return PageCostPrivateV3{}, e
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || st.Nlink != 1 {
			return PageCostPrivateV3{}, ErrInvalid
		}
		r, raw, e := readLedgerRecordPrivateV3(l.state.root, i)
		if e != nil {
			return PageCostPrivateV3{}, e
		}
		canonical, _ := json.Marshal(r)
		if !bytes.Equal(canonical, raw) || r.Schema != pageLedgerSchemaPrivateV3 || r.Application != l.state.application || r.Generation != i || r.Previous != previous || r.Allowance != l.state.allowance || !r.Cumulative.fits(r.Allowance) || !pageLedgerOverheadPrivateV3.fits(r.Reserved) {
			return PageCostPrivateV3{}, ErrInvalid
		}
		cumulative, e = cumulative.plus(r.Reserved)
		if e != nil || cumulative != r.Cumulative {
			return PageCostPrivateV3{}, ErrInvalid
		}
		previous = Digest(raw)
	}
	if previous != tip {
		return PageCostPrivateV3{}, ErrInvalid
	}
	return cumulative, nil
}

// ValidatedCurrentLedgerPrivateV3 is not source or original-capture authority.
// The private seal cannot be obtained from a plain (terminal-only) ledger.
// Reservations through this shared state extend the verified chain under the
// ordinary ledger mutex/flock/O_EXCL frontier protocol; a competing tip refuses.
type ValidatedCurrentLedgerPrivateV3 struct {
	PageLedgerPrivateV3
	seal *pageLedgerStatePrivateV3
}

func (l ValidatedCurrentLedgerPrivateV3) valid() bool {
	return l.seal != nil && l.seal == l.state && l.state.self == l.state
}

// OpenValidatedCurrentLedgerPrivateV3 spends explicit caller bootstrap permission
// BEFORE opening/reading the journal. This is not application-total accounting:
// failed bootstrap attempts debit bootstrap, never an unchecked application grant.
// On success the verification cost is additionally recorded in the application
// journal before a spendable retained-family capability escapes. No source runs.
func OpenValidatedCurrentLedgerPrivateV3(ctx context.Context, path, application string, allowance PageCostPrivateV3, generation uint64, tip string, bootstrap *PageCostPrivateV3) (ValidatedCurrentLedgerPrivateV3, error) {
	if ctx == nil || ctx.Err() != nil || bootstrap == nil || generation == ^uint64(0) || !validPageDigestPrivateV3(tip) {
		return ValidatedCurrentLedgerPrivateV3{}, ErrInvalid
	}
	one := checkpointReadCostPrivateV3(producerPageHeaderPrivateV3)
	count := generation + 1
	cost := PageCostPrivateV3{}
	inputs := []uint64{one.LogicalBytes, one.ReadHashBytes, one.WriteBytes, one.Work}
	outputs := []*uint64{&cost.LogicalBytes, &cost.ReadHashBytes, &cost.WriteBytes, &cost.Work}
	for i, v := range inputs {
		if v != 0 && count > ^uint64(0)/v {
			return ValidatedCurrentLedgerPrivateV3{}, ErrInvalid
		}
		*outputs[i] = count * v
	}
	var e error
	cost, e = cost.plus(checkpointReadCostPrivateV3(4 * scopeMetadataPrivateV2))
	if e != nil {
		return ValidatedCurrentLedgerPrivateV3{}, e
	}
	if !cost.fits(*bootstrap) {
		return ValidatedCurrentLedgerPrivateV3{}, ErrPagesPausedPrivateV3
	}
	*bootstrap = bootstrap.minus(cost)
	l, e := openPageLedgerRootPrivateV3(ctx, path, application, allowance)
	if e != nil {
		return ValidatedCurrentLedgerPrivateV3{}, e
	}
	success := false
	defer func() {
		if !success {
			_ = l.Close()
		}
	}()
	s := l.state
	lock, e := s.root.OpenFile(ledgerNamePrivateV3(generation), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return ValidatedCurrentLedgerPrivateV3{}, e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); e != nil {
		return ValidatedCurrentLedgerPrivateV3{}, e
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if _, e = s.root.Lstat(ledgerNamePrivateV3(generation + 1)); !os.IsNotExist(e) {
		return ValidatedCurrentLedgerPrivateV3{}, ErrInvalid
	}
	spent, e := inspectRetainedLedgerChainPrivateV3(ctx, l, generation, tip)
	if e != nil {
		return ValidatedCurrentLedgerPrivateV3{}, e
	}
	if ctx.Err() != nil {
		return ValidatedCurrentLedgerPrivateV3{}, ctx.Err()
	}
	s.generation, s.last, s.spent = generation, tip, spent
	// The frontier lock is held through the create-only accounting record. Avoid
	// recursively taking flock via reserve; use the same charged-record arithmetic.
	charged, e := cost.plus(pageLedgerOverheadPrivateV3)
	if e != nil {
		return ValidatedCurrentLedgerPrivateV3{}, e
	}
	total, e := spent.plus(charged)
	if e != nil {
		return ValidatedCurrentLedgerPrivateV3{}, e
	}
	if !total.fits(allowance) {
		return ValidatedCurrentLedgerPrivateV3{}, ErrPagesPausedPrivateV3
	}
	raw, _ := json.Marshal(pageLedgerRecordPrivateV3{pageLedgerSchemaPrivateV3, application, generation + 1, tip, allowance, charged, total})
	if e = s.createRecord(generation+1, raw); e != nil {
		return ValidatedCurrentLedgerPrivateV3{}, e
	}
	s.generation, s.last, s.spent = generation+1, Digest(raw), total
	success = true
	return ValidatedCurrentLedgerPrivateV3{l, s}, nil
}
