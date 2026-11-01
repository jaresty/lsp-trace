//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"errors"
	pilot "lsp-trace/internal/censuslineagepilot"
)

// Continue consumes only retained data and an explicit NEW caller allowance
// revision. It does not call a provider, replay an original claim, restore a
// Manager, or change the immutable page-ledger application allowance.
func Continue(ctx context.Context, c Config, t Transfer, bootstrap *pilot.PageCostPrivateV3, backend Backend) (out Report, err error) {
	// Admission failures have no fresh owner. Deliver only prior report DATA,
	// explicitly qualified as an observation rather than current validation.
	defer func() {
		if err != nil && out.Profile == "" {
			var deliveryErr error
			out, deliveryErr = DeliverEarlyFailure(c, "VALIDATED_LEDGER_AND_SOURCE", "RETAINED_ADMISSION_REFUSED")
			err = errors.Join(err, deliveryErr)
		}
	}()
	if ValidateConfig(c) != nil {
		return Report{}, ErrInvalid
	}
	next, e := RetainedContinuation(c, t)
	if e != nil {
		return Report{}, e
	}
	s, e := openStore(c)
	if e != nil {
		return Report{}, e
	}
	prior, e := s.latestReport(c)
	rejected := s.rejectedPrior
	s.close()
	if e != nil || rejected || prior.Status != "PAUSED_BUDGET" || prior.Application != c.Application || prior.NextBatch != c.StartBatch {
		return Report{}, ErrInvalid
	}
	if ValidateTransfer(c, next) != nil {
		return Report{}, ErrInvalid
	}
	owner, close, e := OpenTransferCurrent(ctx, next, prior.LedgerGeneration, prior.LedgerTip, bootstrap, c.Exchange)
	if e != nil {
		return Report{}, e
	}
	report, runErr := RunRetained(ctx, c, owner, backend)
	return report, errors.Join(runErr, close())
}
