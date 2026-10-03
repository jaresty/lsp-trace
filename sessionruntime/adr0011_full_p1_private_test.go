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

func fullP1Red(t *testing.T, assertion, detail string) {
	t.Helper()
	t.Fatalf(`{"kind":"ADR0011_FULL_P1_SEMANTIC_RED","assertion":%q,"detail":%q}`, assertion, detail)
}

type fullP1Child struct {
	input, stdout *io.PipeReader
	stdin, output *io.PipeWriter
	result        json.RawMessage
	writes        atomic.Int64
	done          chan error
}

func newFullP1Child(result json.RawMessage) *fullP1Child {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &fullP1Child{input: input, stdin: stdin, stdout: stdout, output: output, result: append(json.RawMessage(nil), result...), done: make(chan error, 1)}
	go func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(output, lspwire.DefaultLimits())
		for {
			msg, err := reader.Read()
			if err != nil {
				c.done <- err
				return
			}
			if msg.Method != "textDocument/definition" {
				continue
			}
			c.writes.Add(1)
			if err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: append(json.RawMessage(nil), msg.ID...), Result: append(json.RawMessage(nil), c.result...)}); err != nil {
				c.done <- err
				return
			}
		}
	}()
	return c
}

func (c *fullP1Child) Stdin() io.WriteCloser { return c.stdin }
func (c *fullP1Child) Stdout() io.ReadCloser { return c.stdout }
func (c *fullP1Child) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *fullP1Child) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

var _ Child = (*fullP1Child)(nil)

type fullP1Fixture struct {
	m              *Manager
	child          *fullP1Child
	req            RoundTripRequest
	owner          B4DefinitionOwner
	selectionBase  B4DefinitionSelectionKey
	source         B4DefinitionTargetSource
	expectedFrame  []byte
	expectedResult []byte
	expectedParams []byte
}

func newFullP1Fixture(t *testing.T) *fullP1Fixture {
	t.Helper()
	assets := b4LeaseFixture(t)
	var declaration struct {
		Transaction       string `json:"transaction"`
		CompletedOwnerKey string `json:"completed_owner_key"`
		Session           string `json:"session"`
		Generation        uint64 `json:"generation"`
		Method            string `json:"method"`
		ProfileSelector   struct {
			TrustDomain          string `json:"trust_domain"`
			Workspace            string `json:"workspace"`
			Profile              string `json:"profile"`
			EnvironmentReference string `json:"environment_reference"`
		} `json:"profile_selector"`
	}
	if err := json.Unmarshal(b4LeaseGet(t, assets, "A/DECLARATION.json"), &declaration); err != nil {
		t.Fatal(err)
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: declaration.ProfileSelector.TrustDomain, Workspace: declaration.ProfileSelector.Workspace, Profile: declaration.ProfileSelector.Profile, EnvironmentReference: declaration.ProfileSelector.EnvironmentReference})
	if err != nil {
		t.Fatal(err)
	}
	expectedResult := b4LeaseGet(t, assets, "A/response.result")
	child := newFullP1Child(expectedResult)
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	started := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if started.Failure != "" || started.SessionID != declaration.Session || started.Generation != declaration.Generation {
		t.Fatalf("BLOCKED_NOT_RED full P1 start: %+v", started)
	}
	if ready := m.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatalf("BLOCKED_NOT_RED full P1 readiness: %+v", ready)
	}

	sourceBytes := b4LeaseGet(t, assets, "A/target-a.go")
	sourceURI := "file:///w/target-a.go"
	m.mu.Lock()
	m.sessions[started.SessionID].seedSources[sourceURI] = append([]byte(nil), sourceBytes...)
	m.mu.Unlock()
	prepared := m.PrepareDocument(context.Background(), DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: sourceURI, LanguageID: "go", CaptureSupply: true})
	if prepared.Failure != "" || prepared.Supply == nil || !bytes.Equal(prepared.Supply.Content, sourceBytes) {
		t.Fatalf("BLOCKED_NOT_RED full P1 source supply: %+v", prepared)
	}
	sourceLease, status := m.PreparePrivateB4DefinitionSource(B4DefinitionSourceReference{SessionID: started.SessionID, Generation: started.Generation, URI: sourceURI, DocumentVersion: prepared.Version})
	if status != PrivateB4Selected || sourceLease.state == nil {
		t.Fatalf("BLOCKED_NOT_RED full P1 source lease: %s", status)
	}
	source := sourceLease.state.source
	params := b4LeaseGet(t, assets, "A/request.params")
	frame := b4LeaseGet(t, assets, "A/response.frame")
	return &fullP1Fixture{
		m: m, child: child,
		req:           RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: declaration.Method, Params: json.RawMessage(params), Deadline: time.Now().Add(2 * time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureDefinitionResponseFrameMaxBytes: int64(len(frame)), CaptureMethodRequestFrameMaxBytes: int64(len(b4LeaseGet(t, assets, "A/request.frame")))},
		owner:         B4DefinitionOwner{Transaction: declaration.Transaction, CompletedOwnerKey: declaration.CompletedOwnerKey, TargetSources: []B4DefinitionSourceLease{sourceLease}},
		selectionBase: B4DefinitionSelectionKey{SessionID: started.SessionID, Transaction: declaration.Transaction, CompletedOwnerKey: declaration.CompletedOwnerKey},
		source:        source, expectedFrame: frame, expectedResult: expectedResult, expectedParams: params,
	}
}

func (f *fullP1Fixture) transact(t *testing.T) (RoundTripResult, B4DefinitionLease, B4DefinitionSelectionKey) {
	t.Helper()
	result, lease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	selection := f.selectionBase
	selection.Key = result.Key
	if result.Failure != "" || result.ServerError != nil || lease == (B4DefinitionLease{}) {
		t.Fatalf("BLOCKED_NOT_RED full P1 transaction: failure=%s key=%+v", result.Failure, result.Key)
	}
	return result, lease, selection
}

func fullP1AssertCapture(t *testing.T, f *fullP1Fixture, capture PrivateB4DefinitionCapture, selection B4DefinitionSelectionKey) {
	t.Helper()
	var encoded bytes.Buffer
	if err := lspwire.NewWriter(&encoded, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(fmt.Sprintf("%d", selection.Key.ID)), Method: f.req.Method, Params: f.req.Params}); err != nil {
		t.Fatal(err)
	}
	requestFrame := encoded.Bytes()
	if capture.SessionID != selection.SessionID || capture.Key != selection.Key || capture.Transaction != selection.Transaction || capture.CompletedOwnerKey != selection.CompletedOwnerKey || capture.Method != f.req.Method ||
		!bytes.Equal(capture.RequestFrame, requestFrame) || !bytes.Equal(capture.RequestParams, f.expectedParams) || !bytes.Equal(capture.ResponseFrame, f.expectedFrame) || !bytes.Equal(capture.Result, f.expectedResult) ||
		capture.RequestFrameSHA256 != privateB4Hash(requestFrame) || capture.ResponseFrameSHA256 != privateB4Hash(f.expectedFrame) || capture.ResultSHA256 != privateB4Hash(f.expectedResult) || capture.QueryOccurrenceID != privateB4QueryOccurrenceID(capture) || len(capture.TargetSources) != 1 {
		fullP1Red(t, "EXACT_COMPOSED_CAPTURE", fmt.Sprintf("capture=%+v", capture))
	}
	source := capture.TargetSources[0]
	if source.SessionID != f.source.SessionID || source.Generation != f.source.Generation || source.URI != f.source.URI || source.DocumentVersion != f.source.DocumentVersion || source.SHA256 != f.source.SHA256 || source.AcquisitionID != f.source.AcquisitionID || !bytes.Equal(source.Bytes, f.source.Bytes) {
		fullP1Red(t, "EXACT_MANAGER_SOURCE_CUSTODY", fmt.Sprintf("source=%+v", source))
	}
}

func TestADR0011FullP1ExactSuccessorSourceTransaction(t *testing.T) {
	f := newFullP1Fixture(t)
	result, lease, selection := f.transact(t)
	m := f.m
	m.mu.Lock()
	reservation := m.privateB4ReservationLocked(lease.token)
	if reservation == nil || len(reservation.capture.ResponseFrame) == 0 || len(result.definitionResponseFrame) == 0 {
		m.mu.Unlock()
		t.Fatal("BLOCKED_NOT_RED successor capture absent")
	}
	sameOriginalFrame := &reservation.capture.ResponseFrame[0] == &result.definitionResponseFrame[0]
	m.mu.Unlock()
	if !sameOriginalFrame {
		fullP1Red(t, "SUCCESSOR_EXACT_PREDECODE_FRAME", "reservation and result do not share the Successor-captured original frame")
	}
	capture, status := m.PreparePrivateB4Definition(lease, selection)
	if status != PrivateB4Selected {
		fullP1Red(t, "PREPARE_AVAILABLE", string(status))
	}
	fullP1AssertCapture(t, f, capture, selection)
	capture.ResponseFrame[0] ^= 1
	capture.TargetSources[0].Bytes[0] ^= 1
	retry, status := m.PreparePrivateB4Definition(lease, selection)
	if status != PrivateB4Selected {
		fullP1Red(t, "PREPARE_NON_CONSUMING", string(status))
	}
	fullP1AssertCapture(t, f, retry, selection)
	if frame, ok := result.CompletedDefinitionResponseFrame(); !ok || !bytes.Equal(frame, f.expectedFrame) {
		fullP1Red(t, "RESULT_EXACT_FRAME", "completed response frame differs")
	}
}

func TestADR0011FullP1RejectRetryCommitOrderingAndReplay(t *testing.T) {
	f := newFullP1Fixture(t)
	_, lease, selection := f.transact(t)
	published := 0
	if capture, status := f.m.CommitPrivateB4Definition(lease, selection, func(c PrivateB4DefinitionCapture) bool { fullP1AssertCapture(t, f, c, selection); return false }, func(PrivateB4DefinitionCapture) { published++ }); status != PrivateB4Unavailable || capture.SessionID != "" || published != 0 {
		fullP1Red(t, "REJECT_ZERO_PUBLICATION", fmt.Sprintf("status=%s published=%d", status, published))
	}
	if slots, _ := lifecycleCharge(f.m); slots != 1 || f.owner.TargetSources[0].state.state.Load() != privateB4SourceTransferred {
		fullP1Red(t, "REJECT_PRESERVES_RETRY", fmt.Sprintf("slots=%d source=%d", slots, f.owner.TargetSources[0].state.state.Load()))
	}
	var publishedCapture PrivateB4DefinitionCapture
	capture, status := f.m.CommitPrivateB4Definition(lease, selection, func(c PrivateB4DefinitionCapture) bool { fullP1AssertCapture(t, f, c, selection); return true }, func(c PrivateB4DefinitionCapture) {
		if f.m.privateB4LeaseCountLocked() != 1 || f.owner.TargetSources[0].state.state.Load() != privateB4SourceTransferred {
			fullP1Red(t, "PUBLICATION_BEFORE_RELEASE", "lease/source released before callback completed")
		}
		published++
		publishedCapture = c
	})
	if status != PrivateB4Selected || published != 1 {
		fullP1Red(t, "COMMIT_EXACTLY_ONE_PUBLICATION", fmt.Sprintf("status=%s published=%d", status, published))
	}
	fullP1AssertCapture(t, f, capture, selection)
	fullP1AssertCapture(t, f, publishedCapture, selection)
	if slots, charged := lifecycleCharge(f.m); slots != 0 || charged != 0 {
		fullP1Red(t, "COMMIT_RELEASE", fmt.Sprintf("slots=%d bytes=%d", slots, charged))
	}
	if replay, replayStatus := f.m.ConsumePrivateB4Definition(lease, selection); replayStatus != PrivateB4Unavailable || replay.SessionID != "" {
		fullP1Red(t, "COMMIT_REPLAY", string(replayStatus))
	}
}

func TestADR0011FullP1ConcurrentCommitExactlyOneWinner(t *testing.T) {
	f := newFullP1Fixture(t)
	_, lease, selection := f.transact(t)
	start := make(chan struct{})
	var publications atomic.Int64
	statuses := make(chan PrivateB4Status, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, status := f.m.CommitPrivateB4Definition(lease, selection, func(PrivateB4DefinitionCapture) bool { return true }, func(PrivateB4DefinitionCapture) { publications.Add(1) })
			statuses <- status
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)
	winners := 0
	for status := range statuses {
		if status == PrivateB4Selected {
			winners++
		}
	}
	if winners != 1 || publications.Load() != 1 {
		fullP1Red(t, "CONCURRENT_ONE_WINNER_PUBLICATION", fmt.Sprintf("winners=%d publications=%d", winners, publications.Load()))
	}
	if slots, charged := lifecycleCharge(f.m); slots != 0 || charged != 0 {
		fullP1Red(t, "CONCURRENT_ACCOUNTING", fmt.Sprintf("slots=%d bytes=%d", slots, charged))
	}
}

func TestADR0011FullP1StaleSourceComposedRefusal(t *testing.T) {
	for _, tc := range []struct {
		name    string
		stale   func(*fullP1Fixture)
		restore func(*fullP1Fixture)
	}{
		{name: "generation", stale: func(f *fullP1Fixture) { f.owner.TargetSources[0].state.source.Generation++ }, restore: func(f *fullP1Fixture) { f.owner.TargetSources[0].state.source.Generation-- }},
		{name: "version", stale: func(f *fullP1Fixture) {
			f.m.mu.Lock()
			d := f.m.sessions[f.req.SessionID].documents[f.source.URI]
			d.version++
			f.m.sessions[f.req.SessionID].documents[f.source.URI] = d
			f.m.mu.Unlock()
		}, restore: func(f *fullP1Fixture) {
			f.m.mu.Lock()
			d := f.m.sessions[f.req.SessionID].documents[f.source.URI]
			d.version--
			f.m.sessions[f.req.SessionID].documents[f.source.URI] = d
			f.m.mu.Unlock()
		}},
		{name: "identity", stale: func(f *fullP1Fixture) { f.owner.TargetSources[0].state.source.SessionID += "-stale" }, restore: func(f *fullP1Fixture) { f.owner.TargetSources[0].state.source.SessionID = f.req.SessionID }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFullP1Fixture(t)
			beforeWrites := f.child.writes.Load()
			tc.stale(f)
			result, lease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
			if result.Failure != session.ToolNotImplemented || lease != (B4DefinitionLease{}) || f.child.writes.Load() != beforeWrites {
				fullP1Red(t, "STALE_SOURCE_ZERO_WRITE_PUBLICATION", fmt.Sprintf("failure=%s lease=%v writes=%d/%d", result.Failure, lease != (B4DefinitionLease{}), f.child.writes.Load(), beforeWrites))
			}
			if slots, charged := lifecycleCharge(f.m); slots != 0 || charged != 0 || f.owner.TargetSources[0].state.state.Load() != privateB4SourceHeld {
				fullP1Red(t, "STALE_SOURCE_ACCOUNTING", fmt.Sprintf("slots=%d bytes=%d source=%d", slots, charged, f.owner.TargetSources[0].state.state.Load()))
			}
			tc.restore(f)
			if status := f.m.ReleasePrivateB4DefinitionSource(f.owner.TargetSources[0]); status != PrivateB4Selected {
				fullP1Red(t, "STALE_SOURCE_RETRY_STATE", string(status))
			}
		})
	}
}

func TestADR0011FullP1SourceBearingCapacityRefusalAndRelease(t *testing.T) {
	f := newFullP1Fixture(t)
	leases := make([]B4DefinitionLease, 0, privateB4MaxSlots)
	selections := make([]B4DefinitionSelectionKey, 0, privateB4MaxSlots)
	for i := 0; i < privateB4MaxSlots; i++ {
		if i > 0 {
			sourceLease, status := f.m.PreparePrivateB4DefinitionSource(B4DefinitionSourceReference{SessionID: f.source.SessionID, Generation: f.source.Generation, URI: f.source.URI, DocumentVersion: f.source.DocumentVersion})
			if status != PrivateB4Selected {
				t.Fatalf("BLOCKED_NOT_RED capacity source %d: %s", i, status)
			}
			f.owner.TargetSources = []B4DefinitionSourceLease{sourceLease}
		}
		_, lease, selection := f.transact(t)
		leases = append(leases, lease)
		selections = append(selections, selection)
	}
	refusedSource, status := f.m.PreparePrivateB4DefinitionSource(B4DefinitionSourceReference{SessionID: f.source.SessionID, Generation: f.source.Generation, URI: f.source.URI, DocumentVersion: f.source.DocumentVersion})
	if status != PrivateB4Selected {
		t.Fatalf("BLOCKED_NOT_RED refused source: %s", status)
	}
	f.owner.TargetSources = []B4DefinitionSourceLease{refusedSource}
	beforeWrites := f.child.writes.Load()
	denied, deniedLease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
	if denied.Failure != session.ResourceExhausted || deniedLease != (B4DefinitionLease{}) || f.child.writes.Load() != beforeWrites || refusedSource.state.state.Load() != privateB4SourceHeld {
		fullP1Red(t, "CAPACITY_REFUSAL_ZERO_EFFECT", fmt.Sprintf("failure=%s lease=%v writes=%d/%d source=%d", denied.Failure, deniedLease != (B4DefinitionLease{}), f.child.writes.Load(), beforeWrites, refusedSource.state.state.Load()))
	}
	f.m.mu.Lock()
	installedRequests := len(f.m.sessions[f.req.SessionID].requests)
	f.m.mu.Unlock()
	if installedRequests != 0 {
		fullP1Red(t, "CAPACITY_REFUSAL_ZERO_REQUEST_INSTALL", fmt.Sprintf("requests=%d", installedRequests))
	}
	if f.m.ReleasePrivateB4DefinitionSource(refusedSource) != PrivateB4Selected {
		fullP1Red(t, "CAPACITY_REFUSED_SOURCE_RELEASE", "refused source not releasable exactly once")
	}
	for i := range leases {
		if _, got := f.m.ConsumePrivateB4Definition(leases[i], selections[i]); got != PrivateB4Selected {
			fullP1Red(t, "CAPACITY_ACCEPTED_RELEASE", fmt.Sprintf("index=%d status=%s", i, got))
		}
		if _, replay := f.m.ConsumePrivateB4Definition(leases[i], selections[i]); replay != PrivateB4Unavailable {
			fullP1Red(t, "CAPACITY_ACCEPTED_REPLAY", fmt.Sprintf("index=%d status=%s", i, replay))
		}
	}
	if slots, charged := lifecycleCharge(f.m); slots != 0 || charged != 0 {
		fullP1Red(t, "CAPACITY_FINAL_ACCOUNTING", fmt.Sprintf("slots=%d bytes=%d", slots, charged))
	}
}

func TestADR0011FullP1FixedSuccessorLimits(t *testing.T) {
	got := privateB4SuccessorOptions(&privateB4Reservation{}).Limits
	want := lspwire.SuccessorIngressLimits{HeaderBytes: 65536, FrameBytes: 2097152, ConsumptionBytes: 8388608, AcquisitionBytes: 8392705, MaxReadBytes: 4096, PrefetchBytes: 4096, HistoryAcquiredBytes: 8388608, HistoryOutstandingBytes: 8388608}
	if got != want {
		fullP1Red(t, "FIXED_SUCCESSOR_LIMITS", fmt.Sprintf("got=%+v want=%+v", got, want))
	}
}
