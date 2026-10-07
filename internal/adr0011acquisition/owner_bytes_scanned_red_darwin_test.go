//go:build darwin

package adr0011acquisition

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/adr0011work"
)

func TestOwnerBytesScannedCausalBoundary(t *testing.T) {
	acceptedImplementationPin := func() (string, error) { return reviewedSyntheticSourceDigest, nil }

	t.Run("successful attached account charges exact retained raw scan", func(t *testing.T) {
		fixture := newOwnerAcquisitionFixture(t)
		account := adr0011work.New(fixture.Query.OccurrenceID)
		fixture.Owner.finalImplementationPin = acceptedImplementationPin

		receipt, err := fixture.Owner.acquirePrivateFinal(adr0011work.WithAccount(fixture.Context, account), fixture.Query)
		if err != nil || receipt == nil {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_SUCCESS: receipt=%+v err=%v", receipt, err)
		}
		matches, err := filepath.Glob(filepath.Join(fixture.Root.Path(), "adr0011-references-raw-payload-v1-*.bin"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_RAW_PAYLOAD: matches=%v err=%v", matches, err)
		}
		payload, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		const (
			ownerScannerTraversals      = 5
			proposalContextTraversals   = 2
			proposalRecordTraversals    = proposalContextTraversals + 1
			candidateRecordTraversals   = proposalRecordTraversals + proposalContextTraversals
			finalRecordTraversals       = candidateRecordTraversals + proposalContextTraversals + candidateRecordTraversals
			publishReplayOccurrences    = 3
			productionScannerTraversals = ownerScannerTraversals + proposalContextTraversals +
				publishReplayOccurrences*proposalRecordTraversals +
				publishReplayOccurrences*candidateRecordTraversals +
				publishReplayOccurrences*finalRecordTraversals
		)
		perTraversal := uint64(scanRawTopLevel(payload).workUnits)
		if perTraversal != 199 {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_PER_TRAVERSAL: got=%d want=199", perTraversal)
		}
		if productionScannerTraversals != 67 {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_TRAVERSALS: got=%d want=67", productionScannerTraversals)
		}
		want := perTraversal * productionScannerTraversals
		got := account.Snapshot()
		if got.Identity != fixture.Query.OccurrenceID || got.BytesScanned != 13333 || got.BytesScanned != want {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_EXACT_ACCOUNT: snapshot=%+v want_identity=%q traversal_bytes=%d traversal_occurrences=%d want_bytes=13333", got, fixture.Query.OccurrenceID, perTraversal, productionScannerTraversals)
		}
	})

	t.Run("default off preserves acquisition result", func(t *testing.T) {
		fixture := newOwnerAcquisitionFixture(t)
		fixture.Owner.finalImplementationPin = acceptedImplementationPin

		receipt, err := fixture.Owner.acquirePrivateFinal(fixture.Context, fixture.Query)
		if err != nil || receipt == nil || receipt.T != 1 || receipt.A != 2 || len(receipt.OrdinalRefs) != 2 {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_DEFAULT_OFF_PARITY: receipt=%+v err=%v", receipt, err)
		}
		if _, attached := adr0011work.AccountFromContext(fixture.Context); attached {
			t.Fatal("ASSERT_OWNER_BYTES_SCANNED_DEFAULT_OFF_CONTEXT: fixture context unexpectedly has an account")
		}
	})

	t.Run("identity mismatch refuses before scan", func(t *testing.T) {
		fixture := newOwnerAcquisitionFixture(t)
		account := adr0011work.New(fixture.Query.OccurrenceID + "-wrong")
		fixture.Owner.finalImplementationPin = acceptedImplementationPin

		receipt, err := fixture.Owner.acquirePrivateFinal(adr0011work.WithAccount(fixture.Context, account), fixture.Query)
		if receipt != nil || !errors.Is(err, adr0011work.ErrIdentityMismatch) {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_IDENTITY_PREFLIGHT: receipt=%+v err=%v", receipt, err)
		}
		got := account.Snapshot()
		if got.BytesScanned != 0 || !got.Terminal {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_IDENTITY_ZERO_EFFECT: snapshot=%+v", got)
		}
	})

	t.Run("account is terminal when acquisition returns", func(t *testing.T) {
		fixture := newOwnerAcquisitionFixture(t)
		account := adr0011work.New(fixture.Query.OccurrenceID)
		fixture.Owner.finalImplementationPin = acceptedImplementationPin

		receipt, err := fixture.Owner.acquirePrivateFinal(adr0011work.WithAccount(fixture.Context, account), fixture.Query)
		if err != nil || receipt == nil {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_TERMINAL_SUCCESS: receipt=%+v err=%v", receipt, err)
		}
		got := account.Snapshot()
		if !got.Terminal || got.Failure != nil {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_TERMINAL_SEAL: snapshot=%+v", got)
		}
		if err := account.ChargeBytesScanned(fixture.Query.OccurrenceID, 1); !errors.Is(err, adr0011work.ErrTerminal) {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_POST_RETURN_REFUSAL: err=%v", err)
		}
	})

	t.Run("scanner overflow fails and seals account", func(t *testing.T) {
		fixture := newOwnerAcquisitionFixture(t)
		account := adr0011work.New(fixture.Query.OccurrenceID)
		if err := account.ChargeBytesScanned(fixture.Query.OccurrenceID, math.MaxUint64-199); err != nil {
			t.Fatal(err)
		}
		fixture.Owner.finalImplementationPin = acceptedImplementationPin

		receipt, err := fixture.Owner.acquirePrivateFinal(adr0011work.WithAccount(fixture.Context, account), fixture.Query)
		if receipt != nil || !errors.Is(err, ErrAcquisition) || !errors.Is(err, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_OVERFLOW: receipt=%+v err=%v", receipt, err)
		}
		got := account.Snapshot()
		if !got.Terminal || got.BytesScanned != math.MaxUint64 || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_OVERFLOW_TERMINAL: snapshot=%+v", got)
		}
	})

	t.Run("later proposal context overflow preserves cause and freezes account", func(t *testing.T) {
		fixture := newOwnerAcquisitionFixture(t)
		account := adr0011work.New(fixture.Query.OccurrenceID)
		const (
			ownerScannerTraversals = 5
			perTraversal           = 199
			preAdditionBytes       = math.MaxUint64 - ownerScannerTraversals*perTraversal
		)
		if err := account.ChargeBytesScanned(fixture.Query.OccurrenceID, preAdditionBytes); err != nil {
			t.Fatal(err)
		}
		fixture.Owner.finalImplementationPin = acceptedImplementationPin

		receipt, err := fixture.Owner.acquirePrivateFinal(adr0011work.WithAccount(fixture.Context, account), fixture.Query)
		if receipt != nil || !errors.Is(err, ErrAcquisition) || !errors.Is(err, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_LATER_PROPOSAL_CONTEXT_CLASSIFICATION: receipt=%+v err=%v", receipt, err)
		}
		got := account.Snapshot()
		if got.BytesScanned != math.MaxUint64 {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_LATER_PROPOSAL_CONTEXT_PRE_ADDITION: snapshot=%+v", got)
		}
		if !got.Terminal || !errors.Is(got.Failure, adr0011work.ErrOverflow) {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_LATER_PROPOSAL_CONTEXT_TERMINAL: snapshot=%+v", got)
		}
	})

	t.Run("later panic seals account", func(t *testing.T) {
		fixture := newOwnerAcquisitionFixture(t)
		account := adr0011work.New(fixture.Query.OccurrenceID)
		fixture.Owner.finalImplementationPin = acceptedImplementationPin
		fixture.Owner.afterRawPayloadTestHook = func() { panic("late scanner panic") }

		deferred := false
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("ASSERT_OWNER_BYTES_SCANNED_PANIC_MISSING")
				}
				deferred = true
			}()
			_, _ = fixture.Owner.acquirePrivateFinal(adr0011work.WithAccount(fixture.Context, account), fixture.Query)
		}()
		got := account.Snapshot()
		if !deferred || !got.Terminal {
			t.Fatalf("ASSERT_OWNER_BYTES_SCANNED_PANIC_TERMINAL: recovered=%v snapshot=%+v", deferred, got)
		}
	})
}
