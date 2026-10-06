package sessionruntime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"unsafe"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func TestPrivateB4C16RejectsOrdinaryPathAndAtomicBootstrapFailure(t *testing.T) {
	result := (&Manager{}).RoundTrip(context.Background(), RoundTripRequest{EnablePrivateC16ObjectAccounting: true})
	if result.Failure != session.ToolNotImplemented {
		t.Fatalf("ASSERT_C16_NON_PRIVATE failure=%s", result.Failure)
	}
	account := mustPrivateB4ByteAccountV2(t)
	filler, failure := account.reserve(privateB4MaxOwnedBytes - privateB4ByteLedgerV2TableBytes)
	if failure != "" {
		t.Fatal(failure)
	}
	before := account.snapshot()
	profile, failure := newPrivateB4AccountC16WithBytes(account)
	if failure != session.ResourceExhausted || profile != nil || account.snapshot() != before {
		t.Fatalf("ASSERT_C16_BOOTSTRAP_ATOMIC profile=%v failure=%s before=%+v after=%+v", profile != nil, failure, before, account.snapshot())
	}
	filler.release()
	account.requestTerminalRelease()
}

type c16ActivationFixture struct {
	m     *Manager
	req   RoundTripRequest
	owner B4DefinitionOwner
}

func newC16ActivationFixture(t *testing.T) *c16ActivationFixture {
	t.Helper()
	definitionResult := json.RawMessage(`[{"uri":"file:///workspace/target.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":6}}}]`)
	child := newFullP1Child(definitionResult)
	m, err := New(Config{
		Limits:  Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2},
		Starter: oneChildStarter{child},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if started.Failure != "" {
		t.Fatalf("BLOCKED_NOT_RED C16 start: %+v", started)
	}
	pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	if ready, found := m.WaitReadiness(context.Background(), pending.ID); !found || ready.State != ReadinessReady || ready.Failure != "" {
		t.Fatalf("BLOCKED_NOT_RED C16 readiness: %+v found=%v", ready, found)
	}

	const sourceURI = "file:///workspace/target.go"
	sourceBytes := []byte("package c16\nfunc target() {}\n")
	m.mu.Lock()
	m.sessions[started.SessionID].seedSources[sourceURI] = append([]byte(nil), sourceBytes...)
	m.mu.Unlock()
	prepared := m.PrepareDocument(context.Background(), DocumentRequest{
		SessionID:     started.SessionID,
		Generation:    started.Generation,
		URI:           sourceURI,
		LanguageID:    "go",
		CaptureSupply: true,
	})
	if prepared.Failure != "" || prepared.Supply == nil {
		t.Fatalf("BLOCKED_NOT_RED C16 source supply: %+v", prepared)
	}
	sourceLease, status := m.PreparePrivateB4DefinitionSource(B4DefinitionSourceReference{
		SessionID:       started.SessionID,
		Generation:      started.Generation,
		URI:             sourceURI,
		DocumentVersion: prepared.Version,
	})
	if status != PrivateB4Selected || sourceLease.state == nil {
		t.Fatalf("BLOCKED_NOT_RED C16 source lease: %s", status)
	}

	req := RoundTripRequest{
		SessionID:   started.SessionID,
		Generation:  started.Generation,
		Method:      "textDocument/definition",
		Params:      json.RawMessage(`{"textDocument":{"uri":"file:///c16/main.go"},"position":{"line":0,"character":0}}`),
		Deadline:    time.Now().Add(time.Second),
		MaxMessages: 1,
		MaxBytes:    4096,
	}
	req.CaptureDefinitionResponseFrameMaxBytes = int64(len(definitionFixtureFrame(t, lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Result: definitionResult})))
	req.CaptureMethodRequestFrameMaxBytes = int64(len(expectedMethodRequestFrame(t, req, 1)))
	return &c16ActivationFixture{
		m:   m,
		req: req,
		owner: B4DefinitionOwner{
			Transaction:       "c16-activation-transaction",
			CompletedOwnerKey: "c16-activation-owner",
			TargetSources:     []B4DefinitionSourceLease{sourceLease},
		},
	}
}

func TestPrivateB4C16ExplicitActivation(t *testing.T) {
	legacySize := unsafe.Sizeof(privateB4Reservation{})
	legacy := newC16ActivationFixture(t)
	legacyResult, legacyLease := legacy.m.RoundTripPrivateB4(context.Background(), legacy.req, legacy.owner)
	if legacyResult.Failure != "" || legacyLease == (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C16_MARKER_FALSE transaction=%+v", legacyResult)
	}
	legacy.m.mu.Lock()
	legacyReservation := legacy.m.privateB4ReservationLocked(legacyLease.token)
	legacySlot := legacy.m.privateB4Leases[legacyReservation.slot]
	legacy.m.mu.Unlock()
	if legacySlot.c16 != nil || unsafe.Sizeof(*legacyReservation) != legacySize {
		t.Fatalf("ASSERT_C16_MARKER_FALSE_ALLOCATION c16=%v size=%d/%d", legacySlot.c16 != nil, unsafe.Sizeof(*legacyReservation), legacySize)
	}

	enabled := newC16ActivationFixture(t)
	enabled.req.EnablePrivateC16ObjectAccounting = true
	result, lease := enabled.m.RoundTripPrivateB4(context.Background(), enabled.req, enabled.owner)
	if result.Failure != "" || lease == (B4DefinitionLease{}) {
		t.Fatalf("ASSERT_C16_MARKER_TRUE transaction=%+v", result)
	}
	enabled.m.mu.Lock()
	reservation := enabled.m.privateB4ReservationLocked(lease.token)
	slot := enabled.m.privateB4Leases[reservation.slot]
	enabled.m.mu.Unlock()
	if slot.c16 == nil || slot.c16.legacy != reservation || slot.c16.profile == nil {
		t.Fatal("ASSERT_C16_DISTINCT_BOOTSTRAP")
	}
	if got := reservation.explicitBytes.snapshot(); got.Live < privateB4AccountC16SelfCost() {
		t.Fatalf("ASSERT_C16_SELF_COST %+v cost=%d", got, privateB4AccountC16SelfCost())
	}
	borrow := PrivateB4DefinitionBorrow{c16: slot.c16}
	copied := borrow
	if err := borrow.WithObjectAdmission(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := copied.WithObjectAdmission(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got := slot.c16.profile.objectSnapshot(); got.Materialized != 2 {
		t.Fatalf("ASSERT_C16_COPY_CONVERGENCE %+v", got)
	}
}
