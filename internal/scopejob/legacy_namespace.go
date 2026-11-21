package scopejob

import (
	"context"
	"errors"
	"lsp-trace/internal/publication"
	"os"
)

// Legacy flat storage has no bounded complete ownership index. Probe every legal
// ordinal (and one boundary ordinal), never the shared directory. This detects
// missing-middle / orphan packets within the frozen legal range, independently
// of siblings. Arbitrary names and out-of-range sparse ordinals remain unknown;
// the report explicitly withholds namespace and denominator completeness.
// At most budget.Packets+2 exact-name probes, constant memory, no source IO.
func inspectLegacyNames(ctx context.Context, root *publication.Root, id string, limit, claims, segments uint64) error {
	return probeLegacyNames(ctx, id, limit, claims, segments, func(name string) error {
		_, err := root.ReadSelectorWithAbsence(name, 1)
		return err
	})
}

// The probe seam is private, local to this call, and permits bounded fault tests.
// Exhaustion/cancellation never substitutes for completing the legal gap check.
func probeLegacyNames(ctx context.Context, id string, limit, claims, segments uint64, probe func(string) error) error {
	for seq := uint64(0); seq <= (limit-2)/2; seq++ {
		for _, k := range []struct {
			kind  string
			count uint64
		}{{"claim", claims}, {"segment", segments}} {
			if seq < k.count {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			err := probe(filename(id, k.kind, seq))
			if e := ctx.Err(); e != nil {
				return e
			}
			if !errors.Is(err, os.ErrNotExist) {
				return errors.Join(ErrIntegrity, err)
			}
		}
	}
	return ctx.Err()
}
