package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"lsp-trace/internal/session"
)

func c13c14Key(n int) privateB4SourceDocumentKey {
	return privateB4SourceDocumentKey{
		URI:             fmt.Sprintf("file:///w/source-%03d.go", n),
		SessionID:       "manager-session",
		Generation:      7,
		DocumentVersion: n + 1,
		SHA256:          fmt.Sprintf("sha256:%064x", n+1),
	}
}

func TestPrivateB4C13DocumentAccountingBoundaries(t *testing.T) {
	reservation := &privateB4Reservation{}
	for i := 0; i < 256; i++ {
		if failure := reservation.admitPrivateB4SourceAccounting(c13c14Key(i), 0); failure != "" {
			t.Fatalf("ASSERT_C13_256_ALLOWED index=%d failure=%s", i, failure)
		}
	}
	before := reservation.privateB4SourceAccountingSnapshot()
	if failure := reservation.admitPrivateB4SourceAccounting(c13c14Key(256), 0); failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C13_257_REFUSED failure=%s", failure)
	}
	after := reservation.privateB4SourceAccountingSnapshot()
	if before.DocumentsAcquired != 256 || after.DocumentsAcquired != before.DocumentsAcquired || len(after.AdmittedLengths) != 256 {
		t.Fatalf("ASSERT_C13_257_ZERO_EFFECT before=%+v after=%+v", before, after)
	}
}

func TestPrivateB4C13IdentityDedupeAndTransactionReset(t *testing.T) {
	base := c13c14Key(1)
	reservation := &privateB4Reservation{}
	if failure := reservation.admitPrivateB4SourceAccounting(base, 17); failure != "" {
		t.Fatal(failure)
	}
	if failure := reservation.admitPrivateB4SourceAccounting(base, 17); failure != "" {
		t.Fatalf("ASSERT_C13_SAME_KEY_RETRY failure=%s", failure)
	}
	version := base
	version.DocumentVersion++
	digest := base
	digest.SHA256 = "sha256:" + fmt.Sprintf("%064x", 999)
	if failure := reservation.admitPrivateB4SourceAccounting(version, 17); failure != "" {
		t.Fatalf("ASSERT_C13_DISTINCT_VERSION failure=%s", failure)
	}
	if failure := reservation.admitPrivateB4SourceAccounting(digest, 17); failure != "" {
		t.Fatalf("ASSERT_C13_DISTINCT_DIGEST failure=%s", failure)
	}
	snapshot := reservation.privateB4SourceAccountingSnapshot()
	if snapshot.DocumentsAcquired != 3 || len(snapshot.AdmittedLengths) != 3 {
		t.Fatalf("ASSERT_C13_IDENTITY_CARDINALITY snapshot=%+v", snapshot)
	}
	fresh := (&privateB4Reservation{}).privateB4SourceAccountingSnapshot()
	if fresh.DocumentsAcquired != 0 || len(fresh.AdmittedLengths) != 0 {
		t.Fatalf("ASSERT_C13_NEW_TRANSACTION_RESET snapshot=%+v", fresh)
	}
}

func TestPrivateB4C14ExactIndependentDocumentBytes(t *testing.T) {
	reservation := &privateB4Reservation{}
	if failure := reservation.admitPrivateB4SourceAccounting(c13c14Key(1), 0); failure != "" {
		t.Fatalf("ASSERT_C14_EMPTY_ALLOWED failure=%s", failure)
	}
	if failure := reservation.admitPrivateB4SourceAccounting(c13c14Key(2), 4194304); failure != "" {
		t.Fatalf("ASSERT_C14_AT_CAP_ALLOWED failure=%s", failure)
	}
	if failure := reservation.admitPrivateB4SourceAccounting(c13c14Key(3), 4194304); failure != "" {
		t.Fatalf("ASSERT_C14_INDEPENDENT_SUM_OVER_CAP failure=%s", failure)
	}
	before := reservation.privateB4SourceAccountingSnapshot()
	if failure := reservation.admitPrivateB4SourceAccounting(c13c14Key(4), 4194305); failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C14_PLUS_ONE_REFUSED failure=%s", failure)
	}
	after := reservation.privateB4SourceAccountingSnapshot()
	if after.DocumentsAcquired != before.DocumentsAcquired || len(after.AdmittedLengths) != len(before.AdmittedLengths) ||
		after.AdmittedLengths[c13c14Key(1)] != 0 || after.AdmittedLengths[c13c14Key(2)] != 4194304 ||
		after.AdmittedLengths[c13c14Key(3)] != 4194304 {
		t.Fatalf("ASSERT_C14_EXACT_LENGTHS_ZERO_EFFECT before=%+v after=%+v", before, after)
	}
}

func TestPrivateB4C13C14TerminalAccountingIsCumulative(t *testing.T) {
	reservation := &privateB4Reservation{}
	key := c13c14Key(1)
	if failure := reservation.admitPrivateB4SourceAccounting(key, 23); failure != "" {
		t.Fatal(failure)
	}
	before := reservation.privateB4SourceAccountingSnapshot()
	reservation.releasePrivateB4SourceStorage()
	reservation.releasePrivateB4SourceStorage()
	after := reservation.privateB4SourceAccountingSnapshot()
	if after.DocumentsAcquired != before.DocumentsAcquired || after.AdmittedLengths[key] != 23 {
		t.Fatalf("ASSERT_C13_C14_TERMINAL_FACTS_PRESERVED before=%+v after=%+v", before, after)
	}
}

func TestPrivateB4C13C14PreservesSupplyBound(t *testing.T) {
	if MaxDocumentSupplyBytes != 1<<20 {
		t.Fatalf("ASSERT_C14_SUPPLY_BOUND_UNCHANGED got=%d", MaxDocumentSupplyBytes)
	}
}

func TestPrivateB4C13C14RealDefinitionPathRecordsExactUniqueSource(t *testing.T) {
	f := newFullP1Fixture(t)
	f.owner.TargetSources = append(f.owner.TargetSources, f.owner.TargetSources[0])
	_, lease, selection := f.transact(t)
	f.m.mu.Lock()
	reservation := f.m.privateB4ReservationLocked(lease.token)
	if reservation == nil {
		f.m.mu.Unlock()
		t.Fatal("ASSERT_C13_C14_REAL_PATH_RESERVATION")
	}
	snapshot := reservation.privateB4SourceAccountingSnapshot()
	captureSources := len(reservation.capture.TargetSources)
	f.m.mu.Unlock()
	key := privateB4SourceDocumentKey{URI: f.source.URI, SessionID: f.source.SessionID, Generation: f.source.Generation,
		DocumentVersion: f.source.DocumentVersion, SHA256: f.source.SHA256}
	if snapshot.DocumentsAcquired != 1 || len(snapshot.AdmittedLengths) != 1 || snapshot.AdmittedLengths[key] != len(f.source.Bytes) || captureSources != 1 {
		t.Fatalf("ASSERT_C13_C14_REAL_PATH_EXACT_UNIQUE snapshot=%+v sources=%d", snapshot, captureSources)
	}
	if _, status := consumePrivateB4SnapshotForTest(f.m, lease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C13_C14_REAL_PATH_RELEASE status=%s", status)
	}
	if after := reservation.privateB4SourceAccountingSnapshot(); after.DocumentsAcquired != 1 || after.AdmittedLengths[key] != len(f.source.Bytes) {
		t.Fatalf("ASSERT_C13_C14_REAL_PATH_TERMINAL_FACTS snapshot=%+v", after)
	}
}

func TestPrivateB4C13C14SamePathLimitRefusalPrecedesTransferInstallAndWrite(t *testing.T) {
	t.Run("257th unique document", func(t *testing.T) {
		f := newFullP1Fixture(t)
		reservation := &privateB4Reservation{method: f.req.Method, sourceLeases: f.owner.TargetSources,
			selection: B4DefinitionSelectionKey{Transaction: f.owner.Transaction, CompletedOwnerKey: f.owner.CompletedOwnerKey}}
		for i := 0; i < 256; i++ {
			if failure := reservation.admitPrivateB4SourceAccounting(c13c14Key(i), 0); failure != "" {
				t.Fatal(failure)
			}
		}
		beforeWrites := f.child.writes.Load()
		result := f.m.roundTripWithPrivate(context.Background(), f.req, nil, reservation)
		f.m.mu.Lock()
		requests := len(f.m.sessions[f.req.SessionID].requests)
		f.m.mu.Unlock()
		if result.Failure != session.ResourceExhausted || f.child.writes.Load() != beforeWrites || requests != 0 ||
			f.owner.TargetSources[0].state.state.Load() != privateB4SourceHeld || len(reservation.capture.TargetSources) != 0 {
			t.Fatalf("ASSERT_C13_PRE_TRANSFER_REFUSAL failure=%s writes=%d/%d requests=%d state=%d sources=%d", result.Failure,
				f.child.writes.Load(), beforeWrites, requests, f.owner.TargetSources[0].state.state.Load(), len(reservation.capture.TargetSources))
		}
		if snapshot := reservation.privateB4SourceAccountingSnapshot(); snapshot.DocumentsAcquired != 256 {
			t.Fatalf("ASSERT_C13_REFUSAL_TRUTHFUL_COUNT snapshot=%+v", snapshot)
		}
		if f.m.ReleasePrivateB4DefinitionSource(f.owner.TargetSources[0]) != PrivateB4Selected {
			t.Fatal("ASSERT_C13_REFUSED_LEASE_RETRYABLE")
		}
	})

	t.Run("oversized unique document", func(t *testing.T) {
		f := newFullP1Fixture(t)
		oversized := bytes.Repeat([]byte{'x'}, 4194305)
		lease := f.owner.TargetSources[0]
		f.m.mu.Lock()
		document := f.m.sessions[f.req.SessionID].documents[f.source.URI]
		document.supply.Content = oversized
		document.digest = sha256.Sum256(oversized)
		f.m.sessions[f.req.SessionID].documents[f.source.URI] = document
		lease.state.source.Bytes = oversized
		lease.state.source.SHA256 = privateB4Hash(oversized)
		f.m.mu.Unlock()
		reservation := &privateB4Reservation{method: f.req.Method, sourceLeases: []B4DefinitionSourceLease{lease},
			selection: B4DefinitionSelectionKey{Transaction: f.owner.Transaction, CompletedOwnerKey: f.owner.CompletedOwnerKey}}
		beforeWrites := f.child.writes.Load()
		result := f.m.roundTripWithPrivate(context.Background(), f.req, nil, reservation)
		f.m.mu.Lock()
		requests := len(f.m.sessions[f.req.SessionID].requests)
		f.m.mu.Unlock()
		if result.Failure != session.ResourceExhausted || f.child.writes.Load() != beforeWrites || requests != 0 ||
			lease.state.state.Load() != privateB4SourceHeld || len(reservation.capture.TargetSources) != 0 {
			t.Fatalf("ASSERT_C14_PRE_MATERIALIZATION_REFUSAL failure=%s writes=%d/%d requests=%d state=%d sources=%d", result.Failure,
				f.child.writes.Load(), beforeWrites, requests, lease.state.state.Load(), len(reservation.capture.TargetSources))
		}
		if snapshot := reservation.privateB4SourceAccountingSnapshot(); snapshot.DocumentsAcquired != 0 || len(snapshot.AdmittedLengths) != 0 {
			t.Fatalf("ASSERT_C14_OVERSIZED_ZERO_CHARGE snapshot=%+v", snapshot)
		}
		if f.m.ReleasePrivateB4DefinitionSource(lease) != PrivateB4Selected {
			t.Fatal("ASSERT_C14_REFUSED_LEASE_RETRYABLE")
		}
	})
}

func TestPrivateB4C13C14CancellationBeforeAdmissionChargesZero(t *testing.T) {
	f := newFullP1Fixture(t)
	reservation := &privateB4Reservation{method: f.req.Method, sourceLeases: f.owner.TargetSources}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := f.m.roundTripWithPrivate(ctx, f.req, nil, reservation)
	if result.Failure == "" || f.owner.TargetSources[0].state.state.Load() != privateB4SourceHeld {
		t.Fatalf("ASSERT_C13_C14_CANCEL_PRE_ADMISSION failure=%s state=%d", result.Failure, f.owner.TargetSources[0].state.state.Load())
	}
	if snapshot := reservation.privateB4SourceAccountingSnapshot(); snapshot.DocumentsAcquired != 0 || len(snapshot.AdmittedLengths) != 0 {
		t.Fatalf("ASSERT_C13_C14_CANCEL_ZERO_CHARGE snapshot=%+v", snapshot)
	}
}

func TestPrivateB4C13C14ReferencesPathChargesZero(t *testing.T) {
	f := newFullP1Fixture(t)
	req := f.req
	req.Method = "textDocument/references"
	req.ADR0011PrivateP2 = true
	reservation := &privateB4Reservation{method: req.Method, nonCustodial: true}
	result := f.m.roundTripWithPrivate(context.Background(), req, nil, reservation)
	f.m.releasePrivateP2HistoryBorrower(reservation)
	if result.Failure != "" || result.ServerError != nil {
		t.Fatalf("BLOCKED_NOT_RED references round trip: failure=%s server=%v", result.Failure, result.ServerError)
	}
	snapshot := reservation.privateB4SourceAccountingSnapshot()
	if snapshot.DocumentsAcquired != 0 || len(snapshot.AdmittedLengths) != 0 {
		t.Fatalf("ASSERT_C13_C14_REFERENCES_ZERO snapshot=%+v", snapshot)
	}
}
