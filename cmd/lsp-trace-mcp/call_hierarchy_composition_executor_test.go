package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type fakeCallCompositionRuntime struct {
	resolveID         string
	resolveGeneration uint64
	resolveCalls      int
	executeCalls      int
	request           sessionruntime.DirectionalCallHierarchyRequestPrivate
	result            sessionruntime.DirectionalCallHierarchyResultPrivate
	err               error
}

func (f *fakeCallCompositionRuntime) ResolveSessionSelector(id string, generation uint64) (string, uint64, session.Failure) {
	f.resolveCalls++
	f.resolveID = id
	f.resolveGeneration = generation
	return "canonical", 7, ""
}
func (f *fakeCallCompositionRuntime) ResolveSessionStatePrivate(string) (session.State, bool) {
	return session.Ready, true
}
func (f *fakeCallCompositionRuntime) Poison(string, uint64) session.LifecycleResult {
	return session.LifecycleResult{State: session.Poisoned, Failure: session.SessionPoisoned}
}
func (f *fakeCallCompositionRuntime) ExecuteCallHierarchyDirectionsPrivate(_ context.Context, r sessionruntime.DirectionalCallHierarchyRequestPrivate) (sessionruntime.DirectionalCallHierarchyResultPrivate, error) {
	f.executeCalls++
	f.request = r
	return f.result, f.err
}

func TestCallHierarchyCompositionExecutorResolvesCurrentAndCallsManagerOnce(t *testing.T) {
	fake := &fakeCallCompositionRuntime{result: sessionruntime.DirectionalCallHierarchyResultPrivate{Incoming: []lsp.CallHierarchyIncomingCall{}, Outgoing: []lsp.CallHierarchyOutgoingCall{}, IncomingNull: true, OutgoingNull: false}}
	input := []byte(`{"experimental_version":"1","session_id":"alias","uri":"file:///w/a.go","language_id":"go","symbol":"A","timeout_ms":1000,"max_messages":2,"max_bytes":1024}`)
	got, fail := (callHierarchyCompositionExecutor{runtime: fake}).Execute(context.Background(), operation.Request{Input: input})
	if fail != nil {
		t.Fatal(fail)
	}
	if fake.resolveCalls != 1 || fake.resolveID != "alias" || fake.resolveGeneration != 0 || fake.executeCalls != 1 || fake.request.SessionID != "canonical" || fake.request.Generation != 7 {
		t.Fatalf("fake=%+v", fake)
	}
	var a callCompositionArtifact
	if err := json.Unmarshal(got.Artifact, &a); err != nil {
		t.Fatal(err)
	}
	if a.Status != "SUCCEEDED" || !a.IncomingNull || a.OutgoingNull || a.Authority != 0 || a.Completeness != "UNKNOWN" || a.FeatureIdentity != "UNRESOLVED" || a.SessionPoisoned || a.RetryScope != "NONE" || a.Incoming == nil || a.Outgoing == nil {
		t.Fatalf("artifact=%+v", a)
	}
}

func TestCallHierarchyCompositionResolutionRefusalHasNoManagerEffect(t *testing.T) {
	fake := &fakeCallCompositionRuntime{}
	// Override through a small wrapper returning a typed resolution failure.
	r := failingCallCompositionRuntime{fake: fake}
	got, fail := (callHierarchyCompositionExecutor{runtime: r}).Execute(context.Background(), operation.Request{Input: []byte(`{"experimental_version":"1","session_id":"missing","uri":"file:///w/a.go","language_id":"go","symbol":"A"}`)})
	if fail != nil {
		t.Fatal(fail)
	}
	var a callCompositionArtifact
	_ = json.Unmarshal(got.Artifact, &a)
	if a.Status != "REFUSED" || a.Phase != "SESSION_RESOLUTION" || a.Code != "SESSION_NOT_FOUND" || a.SessionPoisoned || a.RetryScope != "REQUEST_CORRECTION" || fake.executeCalls != 0 {
		t.Fatalf("artifact=%+v calls=%d", a, fake.executeCalls)
	}
}

func TestCallHierarchyCompositionCancellationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		phase      string
		code       string
		postEffect bool
	}{
		{"BEFORE_INCOMING_EFFECT", "CANCELLED_BEFORE_INCOMING_EFFECT", false},
		{"AFTER_INCOMING_BEFORE_OUTGOING_EFFECT", "CANCELLED_AFTER_INCOMING_BEFORE_OUTGOING_EFFECT", true},
		{"AFTER_OUTGOING_REQUEST_EFFECT", "CANCELLED_AFTER_OUTGOING_REQUEST_EFFECT", true},
	} {
		r := sessionruntime.DirectionalCallHierarchyResultPrivate{CancellationPhasePrivate: tc.phase, RequestEffectObservedPrivate: tc.postEffect, TerminalPoisonObservedPrivate: tc.postEffect}
		phase, code, poisoned, retry := classifyCallRefusal("", context.Canceled, r)
		wantRetry := "NONE"
		if tc.postEffect {
			wantRetry = "NEW_SESSION_REQUIRED"
		}
		if phase != "CANCELLATION" || code != tc.code || poisoned != tc.postEffect || retry != wantRetry {
			t.Fatalf("phase=%s code=%s poisoned=%t retry=%s", phase, code, poisoned, retry)
		}
	}
}

func TestCallHierarchyCompositionReceiptStates(t *testing.T) {
	unavailable := unavailableCallReceipt()
	if unavailable.State != "UNAVAILABLE" || unavailable.Method != "" || unavailable.RequestSHA256 != "" {
		t.Fatalf("unavailable=%+v", unavailable)
	}
	b, err := json.Marshal(unavailable)
	if err != nil || string(b) != `{"state":"UNAVAILABLE"}` {
		t.Fatalf("json=%s err=%v", b, err)
	}
}

func TestCallHierarchyCompositionRefusalCoherence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		result     sessionruntime.DirectionalCallHierarchyResultPrivate
		err        error
		phase      string
		code       string
		poisoned   bool
		retryScope string
	}{
		{"post-effect-timeout", sessionruntime.DirectionalCallHierarchyResultPrivate{Failure: session.RequestTimeout, FailurePostEffect: true, SessionPoisoned: true, RequestEffectObservedPrivate: true, TerminalPoisonObservedPrivate: true, RequestWriteCompletedPrivate: true, TerminalLifecyclePrivate: session.Poisoned}, context.DeadlineExceeded, "POST_EFFECT_FAILURE", "REQUEST_TIMEOUT", true, "NEW_SESSION_REQUIRED"},
		{"post-effect-resource", sessionruntime.DirectionalCallHierarchyResultPrivate{Failure: session.ResourceExhausted, FailurePostEffect: true, SessionPoisoned: true, RequestEffectObservedPrivate: true, TerminalPoisonObservedPrivate: true}, errors.New(string(session.ResourceExhausted)), "POST_EFFECT_RESOURCE_REFUSAL", "RESOURCE_EXHAUSTED", true, "NEW_SESSION_REQUIRED"},
		{"post-effect-malformed", sessionruntime.DirectionalCallHierarchyResultPrivate{Failure: session.SessionPoisoned, FailurePostEffect: true, SessionPoisoned: true, RequestEffectObservedPrivate: true, TerminalPoisonObservedPrivate: true, TerminalLifecyclePrivate: session.Poisoned}, errors.New(string(session.SessionPoisoned)), "POST_EFFECT_FAILURE", "SESSION_POISONED", true, "NEW_SESSION_REQUIRED"},
		{"post-effect-crashed", sessionruntime.DirectionalCallHierarchyResultPrivate{Failure: session.SessionCrashed, FailurePostEffect: true, RequestEffectObservedPrivate: true, TerminalPoisonObservedPrivate: true, TerminalLifecyclePrivate: session.Crashed}, errors.New(string(session.SessionCrashed)), "POST_EFFECT_FAILURE", "SESSION_CRASHED", true, "NEW_SESSION_REQUIRED"},
		{"post-effect-code-without-manager-facts", sessionruntime.DirectionalCallHierarchyResultPrivate{Failure: session.SessionPoisoned, FailurePostEffect: true, SessionPoisoned: true, TerminalLifecyclePrivate: session.Poisoned}, errors.New(string(session.SessionPoisoned)), "POST_EFFECT_FAILURE", "SESSION_POISONED", false, "NONE"},
		{"pre-effect-resource", sessionruntime.DirectionalCallHierarchyResultPrivate{Failure: session.ResourceExhausted}, errors.New(string(session.ResourceExhausted)), "RESOURCE_LIMIT", "RESOURCE_EXHAUSTED", false, "NONE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeCallCompositionRuntime{result: tc.result, err: tc.err}
			got, fail := (callHierarchyCompositionExecutor{runtime: fake}).Execute(context.Background(), operation.Request{Input: []byte(`{"experimental_version":"1","session_id":"s","uri":"file:///w/a.go","language_id":"go","symbol":"A"}`)})
			if fail != nil {
				t.Fatal(fail)
			}
			var a callCompositionArtifact
			if err := json.Unmarshal(got.Artifact, &a); err != nil {
				t.Fatal(err)
			}
			if a.Status != "REFUSED" || a.Phase != tc.phase || a.Code != tc.code || a.SessionPoisoned != tc.poisoned || a.RetryScope != tc.retryScope {
				t.Fatalf("artifact=%+v", a)
			}
		})
	}
}

type failingCallCompositionRuntime struct{ fake *fakeCallCompositionRuntime }

func (f failingCallCompositionRuntime) ResolveSessionSelector(string, uint64) (string, uint64, session.Failure) {
	return "", 0, session.SessionNotFound
}
func (f failingCallCompositionRuntime) ResolveSessionStatePrivate(string) (session.State, bool) {
	return "", false
}
func (f failingCallCompositionRuntime) Poison(id string, generation uint64) session.LifecycleResult {
	return f.fake.Poison(id, generation)
}
func (f failingCallCompositionRuntime) ExecuteCallHierarchyDirectionsPrivate(ctx context.Context, r sessionruntime.DirectionalCallHierarchyRequestPrivate) (sessionruntime.DirectionalCallHierarchyResultPrivate, error) {
	return f.fake.ExecuteCallHierarchyDirectionsPrivate(ctx, r)
}
