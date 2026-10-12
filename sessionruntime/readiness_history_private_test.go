package sessionruntime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

func c06HistoryBackingCleared(h *privateReadinessHistory) bool {
	if h == nil || h.outstanding != 0 {
		return false
	}
	for _, chunk := range h.chunks {
		if chunk != nil {
			return false
		}
	}
	return true
}

func TestC06PrivateHistoryStoreBoundsCopiesAndImmutableCut(t *testing.T) {
	h := &privateReadinessHistory{identity: "generation"}
	frame := []byte("exact")
	if err := h.appendOwned(privateHistoryOutbound, frame); err != nil {
		t.Fatal(err)
	}
	frame[0] = 'X'
	if got := h.chunks[0][0].frame; !bytes.Equal(got, []byte("exact")) {
		t.Fatalf("ASSERT_C06_HISTORY_EXACT_COPY: %q", got)
	}
	first, err := h.freezeReceiptLocked()
	if err != nil {
		t.Fatal(err)
	}
	if err := h.appendOwned(privateHistoryInbound, []byte("response")); err != nil {
		t.Fatal(err)
	}
	if first.metadata.Entries != 1 || first.metadata.CutOrdinal != 1 || first.metadata.Cut == h.cutDigest() {
		t.Fatalf("ASSERT_C06_HISTORY_IMMUTABLE_CUT: first=%+v current=%s", first.metadata, h.cutDigest())
	}

	full := &privateReadinessHistory{identity: "full", entries: privateHistoryMaxEntries}
	before := *full
	if err := full.appendOwned(privateHistoryOutbound, []byte("x")); err == nil || full.entries != before.entries || full.acquired != before.acquired || full.outstanding != before.outstanding {
		t.Fatalf("ASSERT_C06_HISTORY_PLUS_ONE_ZERO_MUTATION: err=%v before=%+v after=%+v", err, before, *full)
	}
	over := &privateReadinessHistory{identity: "bytes", acquired: privateHistoryMaxBytes, outstanding: privateHistoryMaxBytes}
	if err := over.appendOwned(privateHistoryOutbound, []byte("x")); err == nil || over.entries != 0 || over.acquired != privateHistoryMaxBytes || over.outstanding != privateHistoryMaxBytes {
		t.Fatalf("ASSERT_C06_HISTORY_BYTE_REFUSAL_ZERO_MUTATION: err=%v state=%+v", err, *over)
	}
	h.retireLocked()
	if h.outstanding == 0 {
		t.Fatal("ASSERT_C06_HISTORY_OWNER_RETIREMENT_RETAINED: retirement released owner-held backing")
	}
	acquired, entries, identity, metadata := h.acquired, h.entries, h.identity, first.metadata
	h.releaseReceiptOwnerLocked()
	if !c06HistoryBackingCleared(h) || h.acquired != acquired || h.entries != entries || h.identity != identity || first.metadata != metadata {
		t.Fatalf("ASSERT_C06_HISTORY_FINAL_OWNER_RELEASE_SETTLES: history=%+v metadata=%+v", h, first.metadata)
	}
	if _, err := h.freezeReceiptLocked(); err == nil || h.receiptOwners != 0 {
		t.Fatalf("ASSERT_C06_HISTORY_RETIRED_FREEZE_REFUSED: err=%v owners=%d", err, h.receiptOwners)
	}
	h.releaseReceiptOwnerLocked()
	h.retireLocked()
	if !c06HistoryBackingCleared(h) || h.acquired != acquired || h.entries != entries {
		t.Fatalf("ASSERT_C06_HISTORY_DUPLICATE_TERMINAL_IDEMPOTENT: history=%+v", h)
	}
}

func TestC06PrivateHistoryUnownedRetirementSettlesBacking(t *testing.T) {
	h := &privateReadinessHistory{identity: "unowned"}
	if err := h.appendOwned(privateHistoryOutbound, []byte("request")); err != nil {
		t.Fatal(err)
	}
	acquired, entries := h.acquired, h.entries
	h.retireLocked()
	if !c06HistoryBackingCleared(h) || h.acquired != acquired || h.entries != entries || !h.retired {
		t.Fatalf("ASSERT_C06_HISTORY_UNOWNED_RETIREMENT_SETTLES: history=%+v", h)
	}
}

func TestC06PrivateHistoryActiveBorrowerReleaseRetainsBacking(t *testing.T) {
	h := &privateReadinessHistory{identity: "active"}
	if err := h.appendOwned(privateHistoryOutbound, []byte("request")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.freezeReceiptLocked(); err != nil || !h.acquireBorrowerLocked() {
		t.Fatalf("ASSERT_C06_HISTORY_ACTIVE_BORROWER_SETUP: err=%v", err)
	}
	h.releaseBorrowerLocked()
	if h.outstanding == 0 || h.chunks[0] == nil || h.retired {
		t.Fatalf("ASSERT_C06_HISTORY_ACTIVE_RELEASE_RETAINS: history=%+v", h)
	}
}

func TestC06PrivateHistoryIdentityDoesNotAliasAcrossGenerationsWithRepeatedEntropy(t *testing.T) {
	repeated := bytes.Repeat([]byte{7}, 32)
	m, err := New(Config{
		Limits:                 Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16, MaxOperations: 1},
		Starter:                &sequenceStarter{children: []Child{referenceChild{}, referenceChild{}}},
		readinessHistoryRandom: &identityReader{data: [][]byte{repeated, repeated}},
	})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if started.Failure != "" {
		t.Fatal(started)
	}
	m.mu.Lock()
	first := m.sessions[started.SessionID].readinessHistory
	if err := first.appendOwned(privateHistoryOutbound, []byte("same-frame")); err != nil {
		m.mu.Unlock()
		t.Fatal(err)
	}
	firstIdentity, firstCut, firstEntries, firstAcquired := first.identity, first.cutDigest(), first.entries, first.acquired
	m.mu.Unlock()
	accepted := m.Restart(context.Background(), started.SessionID, "history-identity")
	terminal := waitOperation(t, m, accepted.IntentID, OperationComplete)
	if terminal.Failure != "" {
		t.Fatal(terminal)
	}
	m.mu.Lock()
	second := m.sessions[started.SessionID].readinessHistory
	if err := second.appendOwned(privateHistoryOutbound, []byte("same-frame")); err != nil {
		m.mu.Unlock()
		t.Fatal(err)
	}
	secondReceipt, err := second.freezeReceiptLocked()
	m.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if firstIdentity == secondReceipt.metadata.Identity || firstCut == secondReceipt.metadata.Cut {
		t.Fatalf("ASSERT_C06_HISTORY_GENERATION_NON_ALIAS: first=%s/%s second=%+v", firstIdentity, firstCut, secondReceipt.metadata)
	}
	if !c06HistoryBackingCleared(first) || !first.retired || first.entries != firstEntries || first.acquired != firstAcquired {
		t.Fatalf("ASSERT_C06_HISTORY_RESTART_SETTLES_OLD_GENERATION: history=%+v", first)
	}
	if records := m.Records(); len(records) != 1 || records[0].Generation != 2 || records[0].State != session.Initializing {
		t.Fatalf("ASSERT_C06_HISTORY_EXACT_GENERATION_OWNER: records=%+v", records)
	}
}

type historyCleanupChild struct {
	teardowns int
	closes    int
}

func (c *historyCleanupChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.teardowns++
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}

func (c *historyCleanupChild) Close() managedprocess.ResourceObservation {
	c.closes++
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

func TestC06InitialHistoryEntropyFailureCleansAttempt(t *testing.T) {
	child := &historyCleanupChild{}
	m, err := New(Config{
		Limits:  Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16, MaxOperations: 1},
		Starter: oneChildStarter{child}, readinessHistoryRandom: &identityReader{err: errors.New("history entropy unavailable")},
	})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if started.Failure != session.ResourceExhausted || len(m.Records()) != 0 || m.Census().Sessions != 0 || m.Census().Children != 0 || child.teardowns != 1 || child.closes != 1 {
		t.Fatalf("ASSERT_C06_INITIAL_HISTORY_ENTROPY_CLEANUP: result=%+v records=%+v census=%+v teardown=%d close=%d", started, m.Records(), m.Census(), child.teardowns, child.closes)
	}
}

func TestC06RestartHistoryEntropyFailureCleansAttemptAndPreservesPriorRecord(t *testing.T) {
	first, attempted := &historyCleanupChild{}, &historyCleanupChild{}
	historyEntropy := &identityReader{data: [][]byte{bytes.Repeat([]byte{1}, 32)}, err: io.ErrUnexpectedEOF}
	m, err := New(Config{
		Limits:  Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16, MaxOperations: 1},
		Starter: &sequenceStarter{children: []Child{first, attempted}}, readinessHistoryRandom: historyEntropy,
	})
	if err != nil {
		t.Fatal(err)
	}
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if initialized := m.ObserveInitialization(started.SessionID, started.Generation, true); initialized.State != session.Ready {
		t.Fatal(initialized)
	}
	accepted := m.Restart(context.Background(), started.SessionID, "history-entropy")
	terminal := waitOperation(t, m, accepted.IntentID, OperationFailed)
	records := m.Records()
	if terminal.Failure != session.ResourceExhausted || len(records) != 1 || records[0].SessionID != started.SessionID || records[0].Generation != started.Generation || records[0].State != session.Ready || first.teardowns != 1 || first.closes != 1 || attempted.teardowns != 1 || attempted.closes != 1 || m.Census().Workers != 0 || m.Census().Sessions != 1 || m.Census().Children != 1 {
		t.Fatalf("ASSERT_C06_RESTART_HISTORY_ENTROPY_CLEANUP: terminal=%+v records=%+v census=%+v first=%d/%d attempted=%d/%d", terminal, records, m.Census(), first.teardowns, first.closes, attempted.teardowns, attempted.closes)
	}
}

func TestC06PrivateHistoryBorrowerAndReceiptOwnerAccounting(t *testing.T) {
	h := &privateReadinessHistory{identity: "generation"}
	if err := h.appendOwned(privateHistoryOutbound, []byte("request")); err != nil {
		t.Fatal(err)
	}
	receipt, err := h.freezeReceiptLocked()
	if err != nil || receipt.history != h || h.receiptOwners != 1 {
		t.Fatalf("ASSERT_C06_HISTORY_RECEIPT_OWNER: receipt=%+v err=%v owners=%d", receipt.metadata, err, h.receiptOwners)
	}
	if !h.acquireBorrowerLocked() || h.receiptOwners != 0 || h.borrowers != 1 {
		t.Fatalf("ASSERT_C06_HISTORY_BORROWER_ACQUIRE: owners=%d borrowers=%d", h.receiptOwners, h.borrowers)
	}
	h.retireLocked()
	if h.outstanding == 0 {
		t.Fatal("ASSERT_C06_HISTORY_BORROWED_RETIREMENT: borrowed backing released")
	}
	acquired, entries, identity, metadata := h.acquired, h.entries, h.identity, receipt.metadata
	h.releaseBorrowerLocked()
	if h.borrowers != 0 || !c06HistoryBackingCleared(h) || h.acquired != acquired || h.entries != entries || h.identity != identity || receipt.metadata != metadata {
		t.Fatalf("ASSERT_C06_HISTORY_FINAL_BORROWER_RELEASE_SETTLES: history=%+v metadata=%+v", h, receipt.metadata)
	}
	h.releaseBorrowerLocked()
	h.retireLocked()
	if h.borrowers != 0 || !c06HistoryBackingCleared(h) || h.acquired != acquired || h.entries != entries {
		t.Fatalf("ASSERT_C06_HISTORY_DUPLICATE_BORROWER_RELEASE_IDEMPOTENT: history=%+v", h)
	}
}
