package sessionruntime

import (
	"context"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

type c07RuntimeSnapshot struct {
	writes, workers, observations, requests, tombstones       int
	receiptOwners, borrowers                                  uint64
	protocolOwned, historyBorrowed, reservationZero           bool
	legacy, c16, c17, history, historyMetadata                any
	selection                                                 B4DefinitionSelectionKey
	pendingKeyProxy                                           lspwire.RequestKey
	maxCharge                                                 int64
	slot, sourceLeases, documentsAcquired, explicitLeaseCount int
	method                                                    string
	nonCustodial                                              bool
}

func c07Snapshot(t *testing.T, f *fullP1Fixture, transaction *privateB4Transaction) c07RuntimeSnapshot {
	t.Helper()
	f.m.mu.Lock()
	defer f.m.mu.Unlock()
	r := f.m.sessions[f.req.SessionID]
	if r == nil || r.readinessHistory == nil || transaction == nil || transaction.legacy == nil {
		t.Fatal("ASSERT_C07_INTEGRATED_FIXTURE")
	}
	legacy := transaction.legacy
	return c07RuntimeSnapshot{
		writes: int(f.child.writes.Load()), workers: f.m.workers, observations: len(f.m.observations),
		requests: len(r.requests), receiptOwners: r.readinessHistory.receiptOwners, borrowers: r.readinessHistory.borrowers,
		tombstones: r.pending.TombstoneCount(), protocolOwned: r.protocolOwned, historyBorrowed: legacy.historyBorrowed,
		reservationZero: reflect.DeepEqual(*legacy, privateB4Reservation{}), legacy: transaction.legacy, c16: transaction.c16, c17: transaction.c17,
		history: legacy.history, historyMetadata: legacy.historyMetadata, selection: legacy.selection, pendingKeyProxy: legacy.selection.Key,
		maxCharge: legacy.maxCharge, slot: legacy.slot, sourceLeases: len(legacy.sourceLeases),
		documentsAcquired: legacy.documentsAcquired, explicitLeaseCount: legacy.explicitLeaseCount,
		method: legacy.method, nonCustodial: legacy.nonCustodial,
	}
}

func TestC07PrivateP2IntegratedPreBlockBoundary(t *testing.T) {
	base := time.Unix(1700000000, 0)
	cases := []struct {
		name   string
		offset time.Duration
		want   session.Failure
	}{
		{name: "before", offset: -time.Nanosecond, want: DocumentSupplyUnavailable},
		{name: "equality", offset: 0, want: session.RequestTimeout},
		{name: "plus-one", offset: time.Nanosecond, want: session.RequestTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFullP1Fixture(t)
			transaction := &privateB4Transaction{legacy: &privateB4Reservation{}}
			requestStart := base
			calls := 0
			f.m.now = func() time.Time {
				calls++
				switch calls {
				case 1:
					return requestStart
				case 2:
					return requestStart.Add(privateP2RequestLimit + tc.offset)
				default:
					t.Fatalf("ASSERT_C07_CLOCK_CALL_COUNT calls=%d", calls)
					return time.Time{}
				}
			}
			wrong := OwnedDocumentBinding{URI: "file:///definitely-not-open.go", Version: 1, SHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
			req := f.req
			req.ADR0011PrivateP2 = true
			req.Deadline = time.Time{}
			req.ExpectedOwnedDocument = &wrong
			before := c07Snapshot(t, f, transaction)
			if !before.reservationZero || transaction.c16 != nil || transaction.c17 != nil {
				t.Fatalf("ASSERT_C07_TRANSACTION_BASELINE before=%+v", before)
			}
			got := f.m.roundTripWithPrivateTransaction(context.Background(), req, nil, transaction)
			after := c07Snapshot(t, f, transaction)
			if got.Failure != tc.want {
				t.Fatalf("ASSERT_C07_INTEGRATED_BOUNDARY case=%s got=%s want=%s", tc.name, got.Failure, tc.want)
			}
			if calls != 2 {
				t.Fatalf("ASSERT_C07_CLOCK_CALL_COUNT calls=%d", calls)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("ASSERT_C07_PREBLOCK_ZERO_MUTATION case=%s before=%+v after=%+v", tc.name, before, after)
			}
		})
	}
}
