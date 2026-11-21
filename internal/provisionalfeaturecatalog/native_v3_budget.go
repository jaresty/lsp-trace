package provisionalfeaturecatalog

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrCallerBudgetV3 = errors.New("caller inventory v3 limit or deadline")

// CallerBudgetV3 is a cumulative logical payload/copy/work allowance, not native
// credit, an OS heap bound or physical containment of callbacks. Failed attempts
// spend their reservations. Exhaustion is sticky and there are no refunds.
type CallerBudgetV3 struct {
	mu           sync.Mutex
	deadline     time.Time
	limit, spent uint64
	semantic     time.Duration
	exhausted    bool
}

func NewCallerBudgetV3(deadline time.Time, bytes uint64) (*CallerBudgetV3, error) {
	if bytes == 0 || bytes > 64<<20 || !time.Now().Before(deadline) || time.Until(deadline) > 180*time.Second {
		return nil, ErrCallerBudgetV3
	}
	return &CallerBudgetV3{deadline: deadline, limit: bytes}, nil
}

// NewReservedCallerBudgetV3 spends only a slice already reserved by the durable
// run's CAS ledger. Previously spent bytes and semantic windows are not reset.
// This is logical accounting, never a native grant or imported capability.
func NewReservedCallerBudgetV3(deadline time.Time, allowance, previouslySpent uint64, semantic time.Duration) (*CallerBudgetV3, error) {
	if allowance == 0 || previouslySpent > 64<<20 || allowance > (64<<20)-previouslySpent || semantic < 0 || semantic > 120*time.Second {
		return nil, ErrCallerBudgetV3
	}
	b, err := NewCallerBudgetV3(deadline, previouslySpent+allowance)
	if err != nil {
		return nil, err
	}
	b.spent, b.semantic = previouslySpent, semantic
	return b, nil
}

func (b *CallerBudgetV3) Reserve(ctx context.Context, bytes uint64) error {
	if b == nil || ctx == nil {
		return ErrCallerBudgetV3
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.exhausted || ctx.Err() != nil || !time.Now().Before(b.deadline) || bytes == 0 || bytes > b.limit-b.spent {
		b.exhausted = true
		return ErrCallerBudgetV3
	}
	b.spent += bytes
	return nil
}
func (b *CallerBudgetV3) Exchange(ctx context.Context, window time.Duration) (context.Context, context.CancelFunc, error) {
	if b == nil || ctx == nil {
		return nil, nil, ErrCallerBudgetV3
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.exhausted || ctx.Err() != nil || window <= 0 || window > 30*time.Second || window > 120*time.Second-b.semantic || !time.Now().Before(b.deadline) {
		b.exhausted = true
		return nil, nil, ErrCallerBudgetV3
	}
	b.semantic += window
	end := time.Now().Add(window)
	if b.deadline.Before(end) {
		end = b.deadline
	}
	c, cancel := context.WithDeadline(ctx, end)
	return c, cancel, nil
}
func (b *CallerBudgetV3) Usage() (uint64, time.Duration) {
	if b == nil {
		return 0, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spent, b.semantic
}
func (b *CallerBudgetV3) Exhausted() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exhausted
}
