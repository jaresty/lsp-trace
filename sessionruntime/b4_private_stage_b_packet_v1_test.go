package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

// Stage B has four independent terminals. Setup failures are never semantic REDs.
func stageBBlocked(t *testing.T, detail string) {
	t.Helper()
	t.Fatalf(`{"kind":"BLOCKED_NOT_RED","detail":%q}`, detail)
}
func stageBFailure(t *testing.T, assertion, detail string) {
	t.Helper()
	t.Fatalf(`{"kind":"STAGE_B_SEMANTIC_RED_CANDIDATE","assertion":%q,"detail":%q}`, assertion, detail)
}
func stageBSelected(t *testing.T, f b4ID1Fixture) (*Manager, *b4LeaseChild, B4DefinitionLease, B4DefinitionSelectionKey) {
	t.Helper()
	m, s, req, owner, child := b4LeaseManager(t, f, "A")
	result, lease := m.RoundTripPrivateB4(context.Background(), req, owner)
	if result.Failure != "" || result.ServerError != nil || lease == (B4DefinitionLease{}) || result.Key != (lspwire.RequestKey{Generation: s.Generation, ID: 1}) {
		stageBBlocked(t, fmt.Sprintf("selected lease not issued: failure=%s key=%+v", result.Failure, result.Key))
	}
	select {
	case err := <-child.observed:
		if err != nil {
			stageBBlocked(t, "fixture WRITE/READ: "+err.Error())
		}
	case <-time.After(time.Second):
		stageBBlocked(t, "fixture WRITE/READ did not finish")
	}
	write, wok := result.CompletedMethodRequestFrame()
	read, rok := result.CompletedDefinitionResponseFrame()
	if !wok || !rok || !bytes.Equal(write, b4LeaseGet(t, f, "A/request.frame")) || !bytes.Equal(read, b4LeaseGet(t, f, "A/response.frame")) || !bytes.Equal(result.Result, b4LeaseGet(t, f, "A/response.result")) {
		stageBBlocked(t, "actual selected WRITE/READ/result not pinned A bytes")
	}
	return m, child, lease, B4DefinitionSelectionKey{SessionID: s.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
}
func stageBControls(t *testing.T) b4ID1Fixture {
	t.Helper()
	f := b4LeaseFixture(t)
	for _, c := range []string{"A", "B"} {
		b4LeaseControls(t, f, c)
		b4LeaseManagerControl(t, f, c)
	}
	return f
}

// T2: each component of the exact selection identity must reject independently.
func TestADR0011StageB02WrongIdentityV1(t *testing.T) {
	f := stageBControls(t)
	m, _, lease, selection := stageBSelected(t, f)
	bad := []B4DefinitionSelectionKey{
		{SessionID: "wrong", Key: selection.Key, Transaction: selection.Transaction, CompletedOwnerKey: selection.CompletedOwnerKey},
		{SessionID: selection.SessionID, Key: lspwire.RequestKey{Generation: selection.Key.Generation + 1, ID: selection.Key.ID}, Transaction: selection.Transaction, CompletedOwnerKey: selection.CompletedOwnerKey},
		{SessionID: selection.SessionID, Key: lspwire.RequestKey{Generation: selection.Key.Generation, ID: selection.Key.ID + 1}, Transaction: selection.Transaction, CompletedOwnerKey: selection.CompletedOwnerKey},
		{SessionID: selection.SessionID, Key: selection.Key, Transaction: "wrong", CompletedOwnerKey: selection.CompletedOwnerKey},
		{SessionID: selection.SessionID, Key: selection.Key, Transaction: selection.Transaction, CompletedOwnerKey: "wrong"},
	}
	for i, key := range bad {
		capture, status := m.ConsumePrivateB4Definition(lease, key)
		if status != PrivateB4Unavailable || capture.SessionID != "" || capture.Key != (lspwire.RequestKey{}) || len(capture.RequestFrame) != 0 || len(capture.ResponseFrame) != 0 || len(capture.Result) != 0 {
			stageBFailure(t, "B02_WRONG_IDENTITY", fmt.Sprintf("component %d accepted or disclosed", i))
		}
	}
	capture, status := m.ConsumePrivateB4Definition(lease, selection)
	if status != PrivateB4Selected || !bytes.Equal(capture.ResponseFrame, b4LeaseGet(t, f, "A/response.frame")) {
		stageBBlocked(t, "negative attempts destroyed valid lease control")
	}
}

// T3: first exact consumption must succeed before replay rejection is meaningful.
func TestADR0011StageB03ReplayV1(t *testing.T) {
	f := stageBControls(t)
	m, _, lease, key := stageBSelected(t, f)
	first, status := m.ConsumePrivateB4Definition(lease, key)
	if status != PrivateB4Selected || !bytes.Equal(first.RequestFrame, b4LeaseGet(t, f, "A/request.frame")) || !bytes.Equal(first.ResponseFrame, b4LeaseGet(t, f, "A/response.frame")) || !bytes.Equal(first.Result, b4LeaseGet(t, f, "A/response.result")) {
		stageBBlocked(t, "first exact consumption absent")
	}
	second, again := m.ConsumePrivateB4Definition(lease, key)
	if again != PrivateB4Unavailable || second.SessionID != "" || len(second.ResponseFrame) != 0 || len(second.Result) != 0 {
		stageBFailure(t, "B03_REPLAY", "second consumption yielded state or captured bytes")
	}
}

// This STOP fixture deliberately does not use b4LeaseManager's background-context
// cleanup: a broken lifecycle must not hang the test process after a timeout.
func stageBStopSelected(t *testing.T, f b4ID1Fixture) (*Manager, *b4LeaseChild, B4DefinitionLease, B4DefinitionSelectionKey) {
	t.Helper()
	var declaration struct {
		Session           string `json:"session"`
		Generation        uint64 `json:"generation"`
		Transaction       string `json:"transaction"`
		CompletedOwnerKey string `json:"completed_owner_key"`
		ProfileSelector   struct {
			TrustDomain          string `json:"trust_domain"`
			Workspace            string `json:"workspace"`
			Profile              string `json:"profile"`
			EnvironmentReference string `json:"environment_reference"`
		} `json:"profile_selector"`
	}
	if err := json.Unmarshal(b4LeaseGet(t, f, "A/DECLARATION.json"), &declaration); err != nil {
		stageBBlocked(t, "STOP declaration: "+err.Error())
	}
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: declaration.ProfileSelector.TrustDomain, Workspace: declaration.ProfileSelector.Workspace, Profile: declaration.ProfileSelector.Profile, EnvironmentReference: declaration.ProfileSelector.EnvironmentReference})
	if err != nil {
		stageBBlocked(t, "STOP profile: "+err.Error())
	}
	child := b4LeaseScript(f, t, "A")
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		stageBBlocked(t, "STOP manager: "+err.Error())
	}
	t.Cleanup(func() {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := m.Shutdown(ctx); err != nil {
			t.Logf("BLOCKED_NOT_RED cleanup: %v", err)
		}
	})
	s := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(profile)})
	if s.SessionID != declaration.Session || s.Generation != declaration.Generation || s.Generation != 1 {
		stageBBlocked(t, "STOP session/generation identity")
	}
	if ready := m.ObserveInitialization(s.SessionID, s.Generation, true); ready.State != session.Ready {
		stageBBlocked(t, "STOP readiness")
	}
	req := RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/definition", Params: json.RawMessage(b4LeaseGet(t, f, "A/request.params")), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureDefinitionResponseFrameMaxBytes: int64(len(b4LeaseGet(t, f, "A/response.frame"))), CaptureMethodRequestFrameMaxBytes: int64(len(b4LeaseGet(t, f, "A/request.frame")))}
	owner := B4DefinitionOwner{Transaction: declaration.Transaction, CompletedOwnerKey: declaration.CompletedOwnerKey}
	result, lease := m.RoundTripPrivateB4(context.Background(), req, owner)
	if result.Failure != "" || result.ServerError != nil || result.Key != (lspwire.RequestKey{Generation: 1, ID: 1}) || lease == (B4DefinitionLease{}) {
		stageBBlocked(t, "STOP selected lease absent")
	}
	select {
	case err := <-child.observed:
		if err != nil {
			stageBBlocked(t, "STOP fixture: "+err.Error())
		}
	case <-time.After(time.Second):
		stageBBlocked(t, "STOP fixture no WRITE/READ")
	}
	write, wok := result.CompletedMethodRequestFrame()
	read, rok := result.CompletedDefinitionResponseFrame()
	if !wok || !rok || !bytes.Equal(write, b4LeaseGet(t, f, "A/request.frame")) || !bytes.Equal(read, b4LeaseGet(t, f, "A/response.frame")) || !bytes.Equal(result.Result, b4LeaseGet(t, f, "A/response.result")) {
		stageBBlocked(t, "STOP selected bytes mismatch")
	}
	return m, child, lease, B4DefinitionSelectionKey{SessionID: s.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
}

// T4: completed STOP retires the old selected lease, not merely its lookup.
func TestADR0011StageB04StoppedLeaseV1(t *testing.T) {
	f := stageBControls(t)
	m, child, lease, key := stageBStopSelected(t, f)
	// Force the test-owned pipes to unblock even if shutdown never completes.
	watchdog := time.AfterFunc(2*time.Second, func() { _ = child.Teardown(context.Background()); _ = child.Close() })
	defer watchdog.Stop()
	served := make(chan error, 1)
	go func() {
		reader := lspwire.NewReader(child.input, lspwire.DefaultLimits())
		message, err := reader.Read()
		if err != nil {
			served <- err
			return
		}
		if message.Method != "shutdown" || string(message.ID) != "2" {
			served <- fmt.Errorf("wrong shutdown request %s/%s", message.Method, message.ID)
			return
		}
		err = lspwire.NewWriter(child.output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage("2"), Result: json.RawMessage("null")})
		if err != nil {
			served <- err
			return
		}
		message, err = reader.Read()
		if err != nil || message.Method != "exit" {
			served <- fmt.Errorf("exit not reached: %v", err)
			return
		}
		served <- nil
	}()
	intent := m.Stop(context.Background(), key.SessionID, "stage-b-stop")
	if intent.Failure != "" || intent.IntentID == "" {
		stageBBlocked(t, "STOP intent not accepted: "+string(intent.Failure))
	}
	deadline := time.After(3 * time.Second)
	for {
		op, ok := m.Operation(intent.IntentID)
		if !ok {
			stageBBlocked(t, "STOP operation missing")
		}
		if op.State == OperationFailed {
			stageBBlocked(t, "STOP operation failed: "+string(op.Failure))
		}
		if op.State == OperationComplete {
			break
		}
		select {
		case <-deadline:
			stageBBlocked(t, "STOP did not complete")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	select {
	case err := <-served:
		if err != nil {
			stageBBlocked(t, "shutdown fixture: "+err.Error())
		}
	case <-time.After(time.Second):
		stageBBlocked(t, "shutdown fixture not joined")
	}
	capture, status := m.ConsumePrivateB4Definition(lease, key)
	if status != PrivateB4Unavailable || capture.SessionID != "" || len(capture.ResponseFrame) != 0 {
		stageBFailure(t, "B04_STOP_STALE", "stopped lease still consumable")
	}
	m.mu.Lock()
	held, charged := m.privateB4LeaseCountLocked(), m.privateB4Bytes
	m.mu.Unlock()
	if held != 0 || charged != 0 {
		stageBFailure(t, "B04_STOP_RETIRE", fmt.Sprintf("stopped lease retained: slots=%d charge=%d", held, charged))
	}
}

// T5: capacity admission is pre-WRITE, and a failed frame capture releases its reservation.
func TestADR0011StageB05CapacityCleanupV1(t *testing.T) {
	f := stageBControls(t)
	m, s, req, owner, child := b4LeaseManager(t, f, "A")
	first, lease := m.RoundTripPrivateB4(context.Background(), req, owner)
	if first.Failure != "" || lease == (B4DefinitionLease{}) {
		stageBBlocked(t, "first private selected lease absent")
	}
	select {
	case err := <-child.observed:
		if err != nil {
			stageBBlocked(t, "initial fixture: "+err.Error())
		}
	case <-time.After(time.Second):
		stageBBlocked(t, "initial fixture did not finish")
	}
	firstFrame, firstRead := first.CompletedDefinitionResponseFrame()
	if first.Key != (lspwire.RequestKey{Generation: s.Generation, ID: 1}) || !firstRead || !bytes.Equal(firstFrame, b4LeaseGet(t, f, "A/response.frame")) {
		stageBBlocked(t, "initial selected ID1 READ mismatch")
	}
	// The same child reads IDs 2–4. Every response is manager-selected, not injected into a result.
	served := make(chan error, 1)
	heldResult := append([]byte(nil), b4LeaseGet(t, f, "A/response.result")...)
	expectedParams := append([]byte(nil), req.Params...)
	fifthWrite := make(chan string, 1)
	fifthReady := make(chan struct{})
	go func() {
		reader := lspwire.NewReader(child.input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(child.output, lspwire.DefaultLimits())
		for i := 2; i <= 4; i++ {
			msg, err := reader.Read()
			if err != nil {
				served <- err
				return
			}
			if msg.Method != "textDocument/definition" || string(msg.ID) != strconv.Itoa(i) || !bytes.Equal(msg.Params, expectedParams) {
				served <- fmt.Errorf("unexpected request id %d", i)
				return
			}
			if err = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(strconv.Itoa(i)), Result: json.RawMessage(heldResult)}); err != nil {
				served <- err
				return
			}
		}
		served <- nil
		close(fifthReady)
		msg, err := reader.Read()
		if err == nil {
			fifthWrite <- msg.Method
		}
	}()
	for i := 2; i <= 4; i++ {
		req.Deadline = time.Now().Add(2 * time.Second)
		got, issued := m.RoundTripPrivateB4(context.Background(), req, owner)
		if got.Failure != "" || issued == (B4DefinitionLease{}) || got.Key.ID != uint64(i) {
			stageBBlocked(t, fmt.Sprintf("live selected lease %d missing: %s", i, got.Failure))
		}
	}
	select {
	case err := <-served:
		if err != nil {
			stageBBlocked(t, "slot fixture: "+err.Error())
		}
	case <-time.After(time.Second):
		stageBBlocked(t, "slot fixture unfinished")
	}
	m.mu.Lock()
	slots, charge := m.privateB4LeaseCountLocked(), m.privateB4Bytes
	m.mu.Unlock()
	if slots != 4 || charge <= 0 || charge > privateB4MaxBytes {
		stageBBlocked(t, fmt.Sprintf("four-slot prestate absent: %d/%d", slots, charge))
	}
	select {
	case <-fifthReady:
	case <-time.After(time.Second):
		stageBBlocked(t, "fifth WRITE observer not armed")
	}
	req.Deadline = time.Now().Add(100 * time.Millisecond)
	denied, extra := m.RoundTripPrivateB4(context.Background(), req, owner)
	select {
	case method := <-fifthWrite:
		stageBFailure(t, "B05_FOUR_SLOTS", "fifth WRITE observed: "+method)
	case <-time.After(150 * time.Millisecond):
	}
	if denied.Failure != session.ResourceExhausted || extra != (B4DefinitionLease{}) {
		stageBFailure(t, "B05_FOUR_SLOTS", "fifth request not rejected at slot boundary")
	}
	// Separate manager: oversized reservation must not be mistaken for a wire-body refusal.
	other, _, oversize, otherOwner, _ := b4LeaseManager(t, f, "B")
	oversize.MaxBytes = 4 << 20
	oversize.CaptureDefinitionResponseFrameMaxBytes = maxMethodFrameCorrespondenceBytes
	oversize.CaptureMethodRequestFrameMaxBytes = maxMethodFrameCorrespondenceBytes
	exhausted, noLease := other.RoundTripPrivateB4(context.Background(), oversize, otherOwner)
	if exhausted.Failure != session.ResourceExhausted || noLease != (B4DefinitionLease{}) {
		stageBFailure(t, "B05_BYTE_BOUND", "oversized reservation admitted")
	}
	// Separate manager: selected READ without retained frame is a post-reservation failure.
	failed, _, small, failedOwner, failedChild := b4LeaseManager(t, f, "A")
	small.CaptureDefinitionResponseFrameMaxBytes = 1
	outcome, noCapture := failed.RoundTripPrivateB4(context.Background(), small, failedOwner)
	select {
	case err := <-failedChild.observed:
		if err != nil {
			stageBBlocked(t, "failed-capture fixture: "+err.Error())
		}
	case <-time.After(time.Second):
		stageBBlocked(t, "failed-capture fixture unfinished")
	}
	selectedRead, readOK := outcome.CompletedResponseRead()
	if outcome.Failure != "" || outcome.ServerError != nil || outcome.Key.ID != 1 || !readOK || selectedRead.Key != outcome.Key || selectedRead.SessionID != small.SessionID || !bytes.Equal(outcome.Result, b4LeaseGet(t, f, "A/response.result")) || noCapture != (B4DefinitionLease{}) {
		stageBBlocked(t, "selected READ with failed frame retention not reached")
	}
	failed.mu.Lock()
	left, bytesLeft := failed.privateB4LeaseCountLocked(), failed.privateB4Bytes
	failed.mu.Unlock()
	if left != 0 || bytesLeft != 0 {
		stageBFailure(t, "B05_FAILURE_CLEANUP", fmt.Sprintf("reservation remained %d/%d", left, bytesLeft))
	}
}

var _ io.Reader = (*io.PipeReader)(nil)
