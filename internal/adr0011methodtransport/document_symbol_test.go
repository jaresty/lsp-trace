package adr0011methodtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func documentSymbolRequest() Request {
	return Request{SessionID: "exact", Generation: 1, Method: methodDocumentSymbol, Params: json.RawMessage(`{"textDocument":{"uri":"file:///w/a.go"}}`), Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096, CaptureOwnedMethodPair: true}
}

type symbolManagedRuntime struct{ *sessionruntime.Manager }

func (r symbolManagedRuntime) Metadata(id string, generation uint64) (sessionruntime.SessionMetadata, session.Failure) {
	m, f := r.Manager.Metadata(id, generation)
	m.DocumentSymbolSupport = true
	return m, f
}

func TestDocumentSymbolManagedThroughTransport(t *testing.T) {
	child := newCorrespondenceChild()
	t.Cleanup(func() { child.Close() })
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 16}, Starter: correspondenceStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/workspace", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile)})
	if started.Failure != "" {
		t.Fatal(started.Failure)
	}
	if ready := manager.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatal(ready)
	}
	req := documentSymbolRequest()
	req.SessionID = started.SessionID
	req.Generation = started.Generation
	got := New(symbolManagedRuntime{manager}).Execute(context.Background(), req)
	pair, ok := got.OwnedPair()
	obs, present := got.Observation()
	if got.Outcome() != OutcomeTransportSuccess || !ok || !present || obs.DeclaredMethod != methodDocumentSymbol || obs.DeclaredLine != 0 || obs.DeclaredCharacter != 0 || obs.DeclaredQueryURI != "file:///w/a.go" || pair.Key.ID == 0 || pair.Key.Generation != req.Generation || pair.Method != req.Method || pair.SessionID != req.SessionID || pair.Generation != req.Generation || !bytes.Equal(pair.Params, req.Params) || !bytes.Equal(pair.Result, got.Raw()) || pair.Write.Key != pair.Key || pair.Read.Key != pair.Key || pair.Write.Method != req.Method || obs.LocalWriteCorrespondence != LocalWriteMatch {
		t.Fatalf("ASSERT_DOC_SYMBOL_MANAGED_PAIR: outcome=%s failure=%s ok=%v obs=%+v pair=%+v", got.Outcome(), got.FailureText(), ok, obs, pair)
	}
	t.Log("ASSERT_DOC_SYMBOL_MANAGED_PAIR: PASS managed write/read key and copied params/result")
}

func TestDocumentSymbolUnsupported(t *testing.T) {
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}}
	got := New(f).Execute(context.Background(), documentSymbolRequest())
	if got.Outcome() != OutcomeUnsupportedCapability || f.calls != 0 {
		t.Fatalf("ASSERT_DOC_SYMBOL_UNSUPPORTED: outcome=%s calls=%d", got.Outcome(), f.calls)
	}
	t.Log("ASSERT_DOC_SYMBOL_UNSUPPORTED: PASS no wire")
}

func TestDocumentSymbolParams(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"textDocument":{}}`, `{"textDocument":{"uri":""}}`, `{"textDocument":{"URI":"file:///w/a.go"}}`, `{"textDocument":{"uri":"file:///w/a.go","uri":"file:///w/a.go"}}`, `{"textDocument":{"uri":"file:///w/a.go"},"position":{"line":0,"character":0}}`, `{"textDocument":{"uri":"file:///w/a.go"},"context":{"includeDeclaration":false}}`, `{"textDocument":{"uri":"file:///w/a.go"},"other":1}`} {
		f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DocumentSymbolSupport: true}}
		req := documentSymbolRequest()
		req.Params = json.RawMessage(raw)
		got := New(f).Execute(context.Background(), req)
		if got.Outcome() != OutcomePreflightFailure || f.calls != 0 || f.metadataCalls != 0 {
			t.Fatalf("ASSERT_DOC_SYMBOL_PARAMS: raw=%s outcome=%s calls=%d", raw, got.Outcome(), f.calls)
		}
	}
	t.Log("ASSERT_DOC_SYMBOL_PARAMS: PASS exact canonical shape")
}

func TestDocumentSymbolMissingPair(t *testing.T) {
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DocumentSymbolSupport: true}, result: sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`), Messages: 1, Bytes: 2}}
	got := New(f).Execute(context.Background(), documentSymbolRequest())
	if got.Outcome() != OutcomeTransportFailure || len(got.Raw()) != 0 || f.calls != 1 {
		t.Fatalf("ASSERT_DOC_SYMBOL_MISSING_PAIR: outcome=%s calls=%d", got.Outcome(), f.calls)
	}
	t.Log("ASSERT_DOC_SYMBOL_MISSING_PAIR: PASS raw withheld")
}
