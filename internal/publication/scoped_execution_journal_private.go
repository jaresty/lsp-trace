package publication

import (
	"context"
	"errors"
	"os"
	"strings"
)

const scopedExecutionHeadPrefixPrivate = "feature-scope-execution-head-private-v1-"
const scopedExecutionHeadLimitPrivate = 16 << 10

// ScopedExecutionJournalPrivate is a borrowed, single-selector capability. It
// exposes neither a Root nor paths/FDs. Close consumes only this wrapper; the
// caller owns the ContentRetentionRoot. Close is not concurrent with use.
// This private primitive is not a general replacement/recovery API.
type ScopedExecutionJournalPrivate struct {
	root     *ContentRetentionRoot
	selector string
}

func validScopedExecutionSelectorPrivate(s string) bool {
	if len(s) != len(scopedExecutionHeadPrefixPrivate)+64+len(".json") || !strings.HasPrefix(s, scopedExecutionHeadPrefixPrivate) || !strings.HasSuffix(s, ".json") {
		return false
	}
	for _, c := range s[len(scopedExecutionHeadPrefixPrivate) : len(s)-5] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// OpenScopedExecutionJournalPrivate performs no writes. Namespace validation
// precedes any root IO; all later operations revalidate the borrowed lifetime.
func (r *ContentRetentionRoot) OpenScopedExecutionJournalPrivate(selector string) (*ScopedExecutionJournalPrivate, error) {
	if !validScopedExecutionSelectorPrivate(selector) {
		return nil, ErrContentObservation
	}
	if err := r.ValidatePrivate(); err != nil {
		return nil, err
	}
	if !scopedExecutionSupportedPrivate() {
		return nil, ErrObservedRootUnsupported
	}
	return &ScopedExecutionJournalPrivate{r, selector}, nil
}
func (j *ScopedExecutionJournalPrivate) validate(ctx context.Context) error {
	if ctx == nil || j == nil || j.root == nil || !validScopedExecutionSelectorPrivate(j.selector) {
		return ErrContentObservation
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return j.root.ValidatePrivate()
}
func (j *ScopedExecutionJournalPrivate) Close() error {
	if j != nil {
		j.root = nil
	}
	return nil
}
func (j *ScopedExecutionJournalPrivate) Load(ctx context.Context) ([]byte, error) {
	if err := j.validate(ctx); err != nil {
		return nil, err
	}
	return loadScopedExecutionPrivate(ctx, j, defaultScopedExecutionOpsPrivate())
}

// CompareAbsent performs exactly one atomic no-overwrite install. EEXIST is
// never retried or read back into success. Receipt/error are independent: a
// known commit survives every subsequent sync, verification or close failure.
func (j *ScopedExecutionJournalPrivate) CompareAbsent(ctx context.Context, raw []byte) (*CompareAndReplaceReceipt, error) {
	if len(raw) == 0 || len(raw) > scopedExecutionHeadLimitPrivate {
		return nil, ErrContentObservation
	}
	if err := j.validate(ctx); err != nil {
		return nil, err
	}
	return compareScopedExecutionPrivate(ctx, j, raw, defaultScopedExecutionOpsPrivate())
}

// An absent result requires ENOENT alone, not an error tree that also contains
// cancellation/cleanup uncertainty. This classification never masks joined IO.
func scopedExecutionOnlyAbsentPrivate(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(interface{ Unwrap() []error }); ok {
		es := e.Unwrap()
		if len(es) == 0 {
			return false
		}
		for _, child := range es {
			if !scopedExecutionOnlyAbsentPrivate(child) {
				return false
			}
		}
		return true
	}
	if e, ok := err.(interface{ Unwrap() error }); ok {
		return scopedExecutionOnlyAbsentPrivate(e.Unwrap())
	}
	return errors.Is(err, os.ErrNotExist)
}
