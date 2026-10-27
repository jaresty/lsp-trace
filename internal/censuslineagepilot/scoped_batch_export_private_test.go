//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

// Test-only bridge: construction always traverses the validated disposable
// restart reader, six independent supplies, and actual Manager guard binding.
// Neither the adapter nor its guard is exported, even in this test bridge.
type ScopedBatchFixturePrivate struct {
	Manager    *sessionruntime.Manager
	Identity   censusacquisition.SessionIdentity
	Workspace  string
	Nomination censusacquisition.SourceFile
	ContextURI string
	adapter    *scopeRuntimeAdapter
	child      *scopeAdapterChild
}

func WithScopedBatchFixturePrivate(t *testing.T, run func(context.Context, *ScopedBatchFixturePrivate)) {
	t.Helper()
	restart, m, id, child := adapterFixture(t)
	err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: restart.Scope().Selection().Workspace}, func(ctx context.Context) error {
		a, _, err := newScopeRuntimeAdapter(ctx, restart, m, id)
		if err != nil {
			return err
		}
		if len(a.files) != 4 || len(a.guard.MembersPrivate()) != 6 {
			t.Fatal("ASSERT_SCOPED_FOUR_NOMINATIONS_TWO_CONTEXT")
		}
		f := &ScopedBatchFixturePrivate{Manager: m, Identity: id, Workspace: a.workspace, Nomination: a.files[0], ContextURI: restart.Scope().Files()[5].URI, adapter: a, child: child}
		run(ctx, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func (f *ScopedBatchFixturePrivate) Mint(ctx context.Context, requestID string, manifest acquisitionmanifest.Manifest, seeds []byte) (*ScopedBatchSelectionPrivate, acquisitionauthority.Binding, error) {
	raw, err := json.Marshal(scopedBatchInputPrivate{SessionID: f.Identity.SessionID, Generation: f.Identity.Generation, Manifest: manifest, OutputVersion: graphprovenance.VersionV5})
	if err != nil {
		return nil, acquisitionauthority.Binding{}, err
	}
	b := acquisitionauthority.Binding{Operation: acquisitionmanifest.SliceV3, Route: "private-census-batch", RequestID: requestID, Workspace: f.Workspace, SessionID: f.Identity.SessionID, Generation: f.Identity.Generation, Input: raw}
	s, err := f.adapter.mintScopedBatchSelectionPrivate(ctx, b, seeds)
	return s, b, err
}
func (f *ScopedBatchFixturePrivate) Discover(ctx context.Context, id censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	return f.adapter.Discover(ctx, id)
}
func (f *ScopedBatchFixturePrivate) ForeignManager(t *testing.T) *sessionruntime.Manager {
	t.Helper()
	m, err := sessionruntime.New(sessionruntime.Config{Starter: scopeAdapterStarter{f.child}, Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 16, MaxOperations: 1}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	return m
}
func (f *ScopedBatchFixturePrivate) Expire() { f.adapter.deadline = time.Now().Add(-time.Second) }
func (f *ScopedBatchFixturePrivate) CancelAfterResponse(cancel context.CancelFunc) {
	f.adapter.afterResponsePrivateTest = cancel
}
func (f *ScopedBatchFixturePrivate) Fault(kind string) {
	f.child.mu.Lock()
	defer f.child.mu.Unlock()
	switch kind {
	case "poison":
		f.child.poison = true
	case "malformed":
		f.child.response = func(lspwire.Message) json.RawMessage { return json.RawMessage(`[{"name":"missing fields"}]`) }
	case "provider-budget":
		f.child.notifications = 64
	case "empty-directions":
		f.child.response = func(m lspwire.Message) json.RawMessage {
			if m.Method == "textDocument/prepareCallHierarchy" {
				var p struct {
					TextDocument struct {
						URI string `json:"uri"`
					} `json:"textDocument"`
				}
				_ = json.Unmarshal(m.Params, &p)
				raw, _ := json.Marshal([]any{scopeAdapterItem(p.TextDocument.URI)})
				return raw
			}
			return json.RawMessage(`[]`)
		}
	}
}
func (f *ScopedBatchFixturePrivate) Traffic() (semantic, preparations int) {
	for _, m := range f.child.snapshot() {
		switch m.Method {
		case "textDocument/didOpen", "textDocument/didChange":
			preparations++
		case "textDocument/documentSymbol", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
			semantic++
		}
	}
	return
}
