package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

// Each assertion has its own terminal: no setup/transport failure is a RED.
func lifecycleBlocked(t *testing.T, detail string) {
	t.Helper()
	t.Fatalf(`{"kind":"BLOCKED_NOT_RED","detail":%q}`, detail)
}
func lifecycleRed(t *testing.T, assertion, detail string) {
	t.Helper()
	t.Fatalf(`{"kind":"LIFECYCLE_SEMANTIC_RED_CANDIDATE","assertion":%q,"detail":%q}`, assertion, detail)
}
func lifecycleCharge(m *Manager) (int, int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.privateB4LeaseCountLocked(), m.privateB4Bytes
}
func lifecycleWait(t *testing.T, m *Manager, id string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		op, ok := m.Operation(id)
		if !ok {
			lifecycleBlocked(t, "operation missing")
		}
		if op.State == OperationFailed {
			lifecycleBlocked(t, "operation failed: "+string(op.Failure))
		}
		if op.State == OperationComplete {
			return
		}
		select {
		case <-deadline:
			lifecycleBlocked(t, "operation timeout")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// The successor is a real wire child: initialize, initialized and a new
// definition WRITE/READ occur after RESTART. It cannot borrow the old child.
func lifecycleSuccessor(t *testing.T, f b4ID1Fixture) (*b4ID1Child, <-chan error) {
	t.Helper()
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &b4ID1Child{input: input, stdin: stdin, output: output, stdout: stdout}
	done := make(chan error, 1)
	request := b4LeaseGet(t, f, "A/request.frame")
	response := b4LeaseGet(t, f, "A/response.frame")
	go func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(output, lspwire.DefaultLimits())
		init, err := reader.Read()
		if err != nil || init.Method != "initialize" || string(init.ID) != "1" {
			done <- fmt.Errorf("initialize: %v", err)
			return
		}
		if err = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: init.ID, Result: json.RawMessage(`{"capabilities":{"positionEncoding":"utf-16","definitionProvider":true}}`)}); err != nil {
			done <- err
			return
		}
		notification, err := reader.Read()
		if err != nil || notification.Method != "initialized" {
			done <- fmt.Errorf("initialized: %v", err)
			return
		}
		msg, _, frame, kept, err := reader.ReadWithFrameIfWithin(4096)
		if err != nil || !kept || !bytes.Equal(frame, request) || msg.Method != "textDocument/definition" || string(msg.ID) != "1" {
			done <- fmt.Errorf("successor definition WRITE: %v", err)
			return
		}
		_, err = output.Write(response)
		done <- err
	}()
	return child, done
}
func TestADR0011PrivateLifecycleRestartRetiresOldLease(t *testing.T) {
	f := stageBControls(t)
	m, old, oldLease, key := stageBStopSelected(t, f)
	slots, charge := lifecycleCharge(m)
	if slots != 1 || charge <= 0 {
		lifecycleBlocked(t, "old lease prestate")
	}
	// A second manager and its lease demonstrate isolation from this restart.
	other, _, otherLease, otherKey := stageBSelected(t, f)
	otherSlots, otherBytes := lifecycleCharge(other)
	if otherSlots != 1 || otherBytes <= 0 {
		lifecycleBlocked(t, "unrelated lease prestate")
	}
	successor, done := lifecycleSuccessor(t, f)
	m.mu.Lock()
	m.starter = oneChildStarter{successor}
	m.mu.Unlock()
	t.Cleanup(func() { _ = successor.Teardown(context.Background()); _ = successor.Close() })
	watchdog := time.AfterFunc(2*time.Second, func() { _ = old.Teardown(context.Background()); _ = old.Close() })
	defer watchdog.Stop()
	served := make(chan error, 1)
	go func() {
		reader := lspwire.NewReader(old.input, lspwire.DefaultLimits())
		shutdown, err := reader.Read()
		if err != nil || shutdown.Method != "shutdown" || string(shutdown.ID) != "2" {
			served <- fmt.Errorf("shutdown: %v", err)
			return
		}
		err = lspwire.NewWriter(old.output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: shutdown.ID, Result: json.RawMessage("null")})
		if err != nil {
			served <- err
			return
		}
		exit, err := reader.Read()
		if err != nil || exit.Method != "exit" {
			served <- fmt.Errorf("exit: %v", err)
			return
		}
		served <- nil
	}()
	intent := m.Restart(context.Background(), key.SessionID, "lifecycle-restart")
	if intent.Failure != "" || intent.IntentID == "" {
		lifecycleBlocked(t, "restart intent: "+string(intent.Failure))
	}
	lifecycleWait(t, m, intent.IntentID)
	select {
	case err := <-served:
		if err != nil {
			lifecycleBlocked(t, "old shutdown: "+err.Error())
		}
	case <-time.After(time.Second):
		lifecycleBlocked(t, "old shutdown not joined")
	}
	m.mu.Lock()
	current := m.sessions[key.SessionID]
	generation := uint64(0)
	if current != nil {
		generation = current.record.Generation
	}
	m.mu.Unlock()
	if generation != 2 {
		lifecycleBlocked(t, fmt.Sprintf("successor generation %d", generation))
	}
	// Action reached; a lookup refusal alone does not prove accounting release.
	stale, availability := consumePrivateB4SnapshotForTest(m, oldLease, key)
	if availability != PrivateB4Unavailable || stale.SessionID != "" || len(stale.ResponseFrame) != 0 {
		lifecycleRed(t, "RESTART_OLD_LEASE", "late old-generation consumption succeeded")
	}
	slots, charge = lifecycleCharge(m)
	if slots != 0 || charge != 0 {
		lifecycleRed(t, "RESTART_OLD_ACCOUNTING", fmt.Sprintf("old slots=%d charge=%d", slots, charge))
	}
	if n, b := lifecycleCharge(other); n != otherSlots || b != otherBytes {
		lifecycleRed(t, "RESTART_UNRELATED_LEASE", "unrelated manager charge changed")
	}
	req, owner := b4LeaseRequestForRestart(t, f, key.SessionID)
	req.Generation = 2
	req.SessionID = key.SessionID
	req.Deadline = time.Now().Add(2 * time.Second)
	result, newLease := m.RoundTripPrivateB4(context.Background(), req, owner)
	select {
	case err := <-done:
		if err != nil {
			lifecycleBlocked(t, "successor WRITE/READ: "+err.Error())
		}
	case <-time.After(2 * time.Second):
		lifecycleBlocked(t, "successor fixture did not complete")
	}
	if result.Failure != "" || result.Key != (lspwire.RequestKey{Generation: 2, ID: 1}) || newLease == (B4DefinitionLease{}) {
		lifecycleRed(t, "RESTART_NEW_LEASE", fmt.Sprintf("new selection failure=%s key=%+v", result.Failure, result.Key))
	}
	newKey := B4DefinitionSelectionKey{SessionID: key.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
	capture, status := consumePrivateB4SnapshotForTest(m, newLease, newKey)
	if status != PrivateB4Selected || !bytes.Equal(capture.ResponseFrame, b4LeaseGet(t, f, "A/response.frame")) {
		lifecycleRed(t, "RESTART_NEW_LEASE", "new lease not consumable")
	}
	if retained, got := consumePrivateB4SnapshotForTest(other, otherLease, otherKey); got != PrivateB4Selected || len(retained.ResponseFrame) == 0 {
		lifecycleRed(t, "RESTART_UNRELATED_LEASE", "unrelated manager lease lost")
	}
}

// Prepare a request/owner from the pinned A declaration without starting a
// second manager. Its generation is set by the caller's successor observation.
func b4LeaseRequestForRestart(t *testing.T, f b4ID1Fixture, id string) (RoundTripRequest, B4DefinitionOwner) {
	t.Helper()
	var d struct {
		Transaction       string
		CompletedOwnerKey string `json:"completed_owner_key"`
	}
	if err := json.Unmarshal(b4LeaseGet(t, f, "A/DECLARATION.json"), &d); err != nil || d.Transaction == "" || d.CompletedOwnerKey == "" {
		lifecycleBlocked(t, "successor declaration")
	}
	req := RoundTripRequest{SessionID: id, Method: "textDocument/definition", Params: json.RawMessage(b4LeaseGet(t, f, "A/request.params")), MaxMessages: 1, MaxBytes: 4096, CaptureDefinitionResponseFrameMaxBytes: int64(len(b4LeaseGet(t, f, "A/response.frame"))), CaptureMethodRequestFrameMaxBytes: int64(len(b4LeaseGet(t, f, "A/request.frame")))}
	return req, B4DefinitionOwner{Transaction: d.Transaction, CompletedOwnerKey: d.CompletedOwnerKey}
}

// A controllable child waits after a completed WRITE. Teardown/Close release
// the READ, and a counted actor proves it is joined before RoundTrip returns.
type lifecycleCancelChild struct {
	*b4ID1Child
	teardownCalls, closeCalls, active atomic.Int32
	written                           chan error
	joined                            chan struct{}
	actorMu                           sync.Mutex
	actorLaunched, teardownStarted    bool
	joinOnce                          sync.Once
}

func newLifecycleCancelChild(child *b4ID1Child) *lifecycleCancelChild {
	return &lifecycleCancelChild{b4ID1Child: child, written: make(chan error, 1), joined: make(chan struct{})}
}
func (c *lifecycleCancelChild) launchActor(run func()) bool {
	c.actorMu.Lock()
	defer c.actorMu.Unlock()
	if c.actorLaunched || c.teardownStarted {
		return false
	}
	c.actorLaunched = true
	c.active.Add(1)
	go func() {
		defer func() {
			c.active.Add(-1)
			c.joinOnce.Do(func() { close(c.joined) })
		}()
		run()
	}()
	return true
}
func (c *lifecycleCancelChild) Teardown(ctx context.Context) managedprocess.TeardownObservation {
	c.teardownCalls.Add(1)
	c.actorMu.Lock()
	c.teardownStarted = true
	actorLaunched := c.actorLaunched
	c.actorMu.Unlock()
	observation := c.b4ID1Child.Teardown(ctx)
	if !actorLaunched {
		c.joinOnce.Do(func() { close(c.joined) })
	}
	<-c.joined
	return observation
}
func (c *lifecycleCancelChild) Close() managedprocess.ResourceObservation {
	c.closeCalls.Add(1)
	return c.b4ID1Child.Close()
}
func TestLifecycleCancelChildTeardownBeforeActorLaunch(t *testing.T) {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := newLifecycleCancelChild(&b4ID1Child{input: input, stdin: stdin, output: output, stdout: stdout})

	teardownDone := make(chan managedprocess.TeardownObservation, 1)
	go func() { teardownDone <- child.Teardown(context.Background()) }()
	var observation managedprocess.TeardownObservation
	select {
	case observation = <-teardownDone:
	case <-time.After(time.Second):
		t.Fatal("pre-actor teardown deadlocked")
	}
	if observation.Death.Kind != managedprocess.DeathExited || observation.Death.Reap.Kind != managedprocess.ReapComplete {
		t.Fatalf("pre-actor teardown observation = %+v", observation)
	}
	actorRan := make(chan struct{})
	if child.launchActor(func() { close(actorRan) }) {
		t.Fatal("actor launched after teardown")
	}
	select {
	case <-actorRan:
		t.Fatal("actor ran after teardown")
	default:
	}
	_ = child.Teardown(context.Background())
	_ = child.Close()
	_ = child.Close()
	if child.active.Load() != 0 || child.teardownCalls.Load() != 2 || child.closeCalls.Load() != 2 {
		t.Fatalf("repeated cleanup: active=%d teardown=%d close=%d", child.active.Load(), child.teardownCalls.Load(), child.closeCalls.Load())
	}
}
func TestADR0011PrivateLifecycleCancellationJoin(t *testing.T) {
	f := b4LeaseFixture(t)
	b4LeaseControls(t, f, "A")
	var declaration struct {
		Session           string
		Generation        uint64
		Transaction       string
		CompletedOwnerKey string `json:"completed_owner_key"`
		ProfileSelector   struct {
			TrustDomain          string `json:"trust_domain"`
			Workspace, Profile   string
			EnvironmentReference string `json:"environment_reference"`
		} `json:"profile_selector"`
	}
	if err := json.Unmarshal(b4LeaseGet(t, f, "A/DECLARATION.json"), &declaration); err != nil {
		lifecycleBlocked(t, "cancel declaration")
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: declaration.ProfileSelector.TrustDomain, Workspace: declaration.ProfileSelector.Workspace, Profile: declaration.ProfileSelector.Profile, EnvironmentReference: declaration.ProfileSelector.EnvironmentReference})
	if err != nil {
		lifecycleBlocked(t, "cancel profile: "+err.Error())
	}
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := newLifecycleCancelChild(&b4ID1Child{input: input, stdin: stdin, output: output, stdout: stdout})
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		lifecycleBlocked(t, "cancel manager: "+err.Error())
	}
	t.Cleanup(func() { _ = child.Teardown(context.Background()); _ = child.Close() })
	started := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if started.SessionID != declaration.Session || started.Generation != declaration.Generation {
		lifecycleBlocked(t, "cancel readiness/identity")
	}
	readinessServed := make(chan error, 1)
	go func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		initialize, err := reader.Read()
		if err != nil || initialize.Method != "initialize" || len(initialize.ID) == 0 {
			readinessServed <- fmt.Errorf("initialize: %v", err)
			return
		}
		if err := lspwire.NewWriter(output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: initialize.ID, Result: json.RawMessage(`{"capabilities":{}}`)}); err != nil {
			readinessServed <- err
			return
		}
		initialized, err := reader.Read()
		if err != nil || initialized.Method != "initialized" || len(initialized.ID) != 0 {
			readinessServed <- fmt.Errorf("initialized: %v", err)
			return
		}
		readinessServed <- nil
	}()
	pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
	ready, found := m.WaitReadiness(context.Background(), pending.ID)
	if err := <-readinessServed; err != nil || !found || ready.State != ReadinessReady || ready.Failure != "" {
		lifecycleBlocked(t, fmt.Sprintf("cancel readiness=%+v found=%v err=%v", ready, found, err))
	}
	req := RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition", Params: json.RawMessage(b4LeaseGet(t, f, "A/request.params")), Deadline: time.Now().Add(3 * time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureDefinitionResponseFrameMaxBytes: int64(len(b4LeaseGet(t, f, "A/response.frame"))), CaptureMethodRequestFrameMaxBytes: int64(len(b4LeaseGet(t, f, "A/request.frame")))}
	decodedCalls := 0
	m.b4DecodedHooks.reserve = func(*privateB4ByteAccountV2, uint64) { decodedCalls++ }
	owner := B4DefinitionOwner{Transaction: declaration.Transaction, CompletedOwnerKey: declaration.CompletedOwnerKey}
	expectedWrite := b4LeaseGet(t, f, "A/request.frame")
	if !child.launchActor(func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		msg, _, frame, kept, e := reader.ReadWithFrameIfWithin(4096)
		if e != nil || !kept || msg.Method != "textDocument/definition" || !bytes.Equal(frame, expectedWrite) {
			child.written <- fmt.Errorf("selected WRITE: %v", e)
			return
		}
		child.written <- nil
		_, _ = io.Copy(io.Discard, input)
	}) {
		lifecycleBlocked(t, "actor launch after teardown")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type answer struct {
		result RoundTripResult
		lease  B4DefinitionLease
	}
	returned := make(chan answer, 1)
	go func() { r, l := m.RoundTripPrivateB4(ctx, req, owner); returned <- answer{r, l} }()
	select {
	case err := <-child.written:
		if err != nil {
			lifecycleBlocked(t, "WRITE control: "+err.Error())
		}
	case <-time.After(2 * time.Second):
		lifecycleBlocked(t, "WRITE control timeout")
	}
	slots, charged := lifecycleCharge(m)
	if slots != 1 || charged <= 0 {
		lifecycleBlocked(t, "reservation not held before cancellation")
	}
	cancel()
	var got answer
	select {
	case got = <-returned:
	case <-time.After(3 * time.Second):
		lifecycleBlocked(t, "cancellation return timeout")
	}
	if got.result.Failure != session.RequestCancelled || got.lease != (B4DefinitionLease{}) {
		lifecycleRed(t, "CANCEL_NO_LEASE", fmt.Sprintf("failure=%s issued=%v", got.result.Failure, got.lease != (B4DefinitionLease{})))
	}
	if decodedCalls != 0 {
		lifecycleRed(t, "ASSERT_C15_DECODED_CANCEL_ZERO", fmt.Sprintf("decoded owners=%d", decodedCalls))
	}
	if slots, charged = lifecycleCharge(m); slots != 0 || charged != 0 {
		lifecycleRed(t, "CANCEL_RELEASE", fmt.Sprintf("slots=%d bytes=%d", slots, charged))
	}
	if child.active.Load() != 0 || child.closeCalls.Load() != 1 || child.teardownCalls.Load() != 1 {
		lifecycleRed(t, "CANCEL_POST_RETURN_JOIN", fmt.Sprintf("active=%d close=%d teardown=%d", child.active.Load(), child.closeCalls.Load(), child.teardownCalls.Load()))
	}
	// Shutdown after the returned operation is an idempotent worker join, not
	// a second retirement of the same owned I/O actor.
	joinCtx, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	if err := m.Shutdown(joinCtx); err != nil {
		lifecycleBlocked(t, "shutdown join: "+err.Error())
	}
	if err := m.Shutdown(joinCtx); err != nil || child.closeCalls.Load() != 1 || child.teardownCalls.Load() != 1 {
		lifecycleRed(t, "CANCEL_IDEMPOTENT", fmt.Sprintf("repeated cleanup: %v close=%d teardown=%d", err, child.closeCalls.Load(), child.teardownCalls.Load()))
	}
}
