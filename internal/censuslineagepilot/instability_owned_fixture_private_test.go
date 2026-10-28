//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
	derivedc "lsp-trace/internal/retainedpartialprogramc"
)

type ownedInstabilityContext struct {
	context.Context
	checks, cancelAt int
}

func (c *ownedInstabilityContext) Err() error {
	c.checks++
	if c.checks >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

// Small dedicated producer test: no adversarial qualification matrix is rerun.
func TestOwnedPrivateInstabilityFixture(t *testing.T) {
	t.Run("calls", func(t *testing.T) {
		h, m, cfg, f := pilot.NewScopedPartialExact25V4RehearsalFixturePrivate(t)
		report, err := executeScopedPartialRehearsalPrivate(context.Background(), h, m, cfg)
		if err != nil {
			t.Fatal(err)
		}
		var inputs [3]*publication.ContentRetentionRoot
		var outputs [2]*publication.ContentRetentionRoot
		for i, path := range cfg.InputRoots {
			inputs[i], err = publication.OpenContentRetentionRoot(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer inputs[i].Close()
		}
		for i, path := range cfg.OutputRoots[:2] {
			outputs[i], err = publication.OpenContentRetentionRoot(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer outputs[i].Close()
		}
		pins := pilot.RetainedPartialQualificationPinsPrivate{Schema: "lsp-trace.retained-partial-qualification-input.private.v1", CodeHEAD: cfg.CodeHEAD, VerifierSourceSHA256: pilot.RetainedPartialQualificationVerifierPinPrivate(), PolicySHA256: pilot.RetainedPartialQualificationPolicyDigestPrivate(), MaxReadBytes: 128 << 20, MaxWorkBytes: 2 << 30}
		records := append(append([]pilot.ScopedRehearsalRecordPrivate{}, report.Records...), *report.OutcomeReceipt)
		for i, r := range records {
			pins.Objects[i] = pilot.RetainedPartialObjectPinPrivate{Name: r.Name, ID: r.ID, Bytes: uint64(r.Bytes)}
		}
		raw, _ := json.Marshal(pins)
		in, err := pilot.NewExactRetainedPartialQualificationInput(raw, inputs, outputs)
		if err != nil {
			t.Fatal(err)
		}
		q, err := pilot.QualifyRetainedPartialNativePrivate(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range inputs {
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
		}
		for _, r := range outputs {
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
		}
		proof, err := pilot.DeriveRetainedPartialNativePrivate(context.Background(), q, pins, pilot.RetainedPartialDerivationBudgetPrivate{MaxInputBytes: 8 << 20, MaxOutputBytes: 8 << 20, MaxWorkBytes: 2 << 30})
		if err != nil {
			t.Fatal(err)
		}
		p, failure := derivedc.Project(proof)
		if failure != nil {
			t.Fatal(failure)
		}
		before, prep := f.Traffic()
		seeds := []uint64{42, 43, 44, 45, 46, 47}
		a, err := derivedc.ComputeInstabilityPrivate(context.Background(), p, seeds)
		if err != nil {
			t.Fatal(err)
		}
		want := "RESOURCE_INCOMPLETE"
		if a.Outcome != want || len(a.Runs) != 6 || len(a.Pairs) != 15 || a.ExpectedPairs != 15 || a.NotAttemptedRuns != 0 || a.NotAttemptedPairs != 0 || a.CompleteRuns+a.EmptyRuns != 6 {
			t.Fatalf("accounting %+v", a)
		}
		raw, _ = json.Marshal(a)
		if err := derivedc.ValidateInstabilityPrivate(context.Background(), p, a, raw); err != nil {
			t.Fatal(err)
		}
		if _, err := derivedc.ComputeInstabilityPrivate(nil, p, seeds); err == nil {
			t.Fatal("nil context")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		partial, err := derivedc.ComputeInstabilityPrivate(ctx, p, seeds)
		if !errors.Is(err, context.Canceled) || partial.Outcome != "INCOMPLETE" || partial.NotAttemptedRuns != 6 || partial.NotAttemptedPairs != 15 {
			t.Fatalf("cancel %+v %v", partial, err)
		}
		afterOne := &ownedInstabilityContext{Context: context.Background(), cancelAt: 2}
		one, oneErr := derivedc.ComputeInstabilityPrivate(afterOne, p, seeds)
		if !errors.Is(oneErr, context.Canceled) || one.Outcome != "INCOMPLETE" || one.CompleteRuns != 1 || one.NotAttemptedRuns != 5 || one.NotAttemptedPairs != 15 {
			t.Fatalf("after one run %+v %v", one, oneErr)
		}
		afterPair := &ownedInstabilityContext{Context: context.Background(), cancelAt: 15}
		pair, pairErr := derivedc.ComputeInstabilityPrivate(afterPair, p, seeds)
		if !errors.Is(pairErr, context.Canceled) || pair.Outcome != "INCOMPLETE" || pair.CompleteRuns != 6 || pair.BlockedPairs != 1 || pair.NotAttemptedPairs != 14 {
			t.Fatalf("after one pair %+v %v", pair, pairErr)
		}
		raw, _ = json.Marshal(partial)
		if derivedc.ValidateInstabilityPrivate(context.Background(), p, partial, raw) == nil {
			t.Fatal("accepted incomplete")
		}
		after, prepAfter := f.Traffic()
		if before != after || prep != prepAfter {
			t.Fatal("offline math performed traffic")
		}
		t.Logf("runs=%d pairs=%d outcome=%s complete=%d blocked=%d empty=%d", len(a.Runs), len(a.Pairs), a.Outcome, a.CompletePairs, a.BlockedPairs, a.EmptyPairs)
	})
}
