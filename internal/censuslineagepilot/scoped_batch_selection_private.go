package censuslineagepilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/seedformat"
	"lsp-trace/sessionruntime"
)

// ScopedBatchSelectionPrivate binds one closed adapter-owned acquisition. It
// conveys neither seed custody nor completeness authority. Copies share spending.
// There is no exported constructor, consumer returning a client, or callback seam.
type ScopedBatchSelectionPrivate struct{ state *scopedBatchSelectionState }
type scopedBatchSelectionState struct {
	mu       sync.Mutex
	spent    bool
	adapter  *scopeRuntimeAdapter
	manager  *sessionruntime.Manager
	guard    *sessionruntime.RetainedScopeGuardPrivate
	binding  acquisitionauthority.Binding
	seeds    []byte
	lifetime context.Context
}

// This restricted input projection uses neutral manifest validation; the engine
// remains authoritative for its full canonical input and seed-authority binding.
type scopedBatchInputPrivate struct {
	SessionID     string                       `json:"session_id"`
	Generation    uint64                       `json:"generation"`
	Manifest      acquisitionmanifest.Manifest `json:"seed_manifest"`
	OutputVersion string                       `json:"output_version,omitempty"`
}

func (a *scopeRuntimeAdapter) mintScopedBatchSelectionPrivate(ctx context.Context, b acquisitionauthority.Binding, seeds []byte) (*ScopedBatchSelectionPrivate, error) {
	if a == nil || a.manager == nil || a.guard == nil || len(a.guard.MembersPrivate()) != 6 || len(a.files) != 4 || a.check(ctx) != nil {
		return nil, ErrInvalid
	}
	if _, err := validateScopedBatchBindingPrivate(a, b); err != nil {
		return nil, err
	}
	if len(seeds) == 0 || len(seeds) > seedformat.MaxInputBytes {
		return nil, ErrInvalid
	}
	f, err := seedformat.Decode(seeds, b.Workspace)
	if err != nil {
		return nil, err
	}
	canonicalSeeds, err := seedformat.EncodeCanonical(f, b.Workspace)
	if err != nil || !bytes.Equal(canonicalSeeds, seeds) {
		return nil, ErrInvalid
	}
	b.Input = bytes.Clone(b.Input)
	return &ScopedBatchSelectionPrivate{state: &scopedBatchSelectionState{adapter: a, manager: a.manager, guard: a.guard, binding: b, seeds: bytes.Clone(seeds), lifetime: ctx}}, nil
}

// validateScopedBatchBindingPrivate is shared by mint and execution. The
// restricted canonical input, neutral manifest and nomination policy cannot drift.
func validateScopedBatchBindingPrivate(a *scopeRuntimeAdapter, b acquisitionauthority.Binding) (acquisition.Request, error) {
	var empty acquisition.Request
	if a == nil || b.Operation != acquisitionmanifest.SliceV3 || b.Route != "private-census-batch" || b.RequestID == "" || len(b.RequestID) > acquisitionmanifest.MaxInputBytes || b.SessionID != a.identity.SessionID || b.Generation != a.identity.Generation || b.Workspace != a.workspace || b.PublicationRoot != nil || b.ArtifactStore != nil || len(b.Input) == 0 || len(b.Input) > acquisitionmanifest.MaxInputBytes {
		return empty, ErrInvalid
	}
	var in scopedBatchInputPrivate
	if scopeBundleStrict(b.Input, &in) != nil || in.SessionID != b.SessionID || in.Generation != b.Generation || in.OutputVersion != graphprovenance.VersionV5 {
		return empty, ErrInvalid
	}
	canonical, err := json.Marshal(in)
	if err != nil || !bytes.Equal(canonical, b.Input) {
		return empty, ErrInvalid
	}
	req, err := in.Manifest.Request(acquisitionmanifest.SliceV3)
	if err != nil {
		return empty, err
	}
	req.TopmostSiblings = in.Manifest.Expansion.TopmostSiblings
	for _, target := range append([]acquisition.Target{req.Root}, req.RequiredTargets...) {
		found := false
		for _, file := range a.files {
			if target.Locator.URI == file.URI && (target.Locator.LanguageID == "" || target.Locator.LanguageID == file.LanguageID) {
				found = true
				break
			}
		}
		if !found {
			return empty, ErrInvalid
		}
	}
	return req, nil
}
func sameScopedBatchBindingPrivate(a, b acquisitionauthority.Binding) bool {
	return a.Operation == b.Operation && a.Route == b.Route && a.RequestID == b.RequestID && a.Workspace == b.Workspace && a.SessionID == b.SessionID && a.Generation == b.Generation && a.PublicationRoot == b.PublicationRoot && a.ArtifactStore == b.ArtifactStore && bytes.Equal(a.Input, b.Input)
}

// AcquireScopedBatchPrivate is a CLOSED operation, not a client/permit factory.
// The original opaque SeedAuthority is verified here exactly once. Rejected
// identity/request/reservation preflight does not spend selection or authority;
// after verified grant comparison the shared selection is spent before Acquire.
// The producer retains the reservation through capture and detached serialization.
func AcquireScopedBatchPrivate(ctx context.Context, manager *sessionruntime.Manager, selection *ScopedBatchSelectionPrivate, authority acquisitionauthority.SeedAuthority, actual acquisitionauthority.Binding, req acquisition.Request) (acquisition.Result, acquisitionauthority.SeedGrant, error) {
	var result acquisition.Result
	var grant acquisitionauthority.SeedGrant
	if ctx == nil || manager == nil || selection == nil || selection.state == nil {
		return result, grant, ErrInvalid
	}
	if len(actual.Input) == 0 || len(actual.Input) > acquisitionmanifest.MaxInputBytes {
		return result, grant, ErrInvalid
	}
	actual.Input = bytes.Clone(actual.Input)
	s := selection.state
	s.mu.Lock()
	if s.spent || s.manager != manager || s.adapter == nil || s.adapter.manager != manager || s.guard == nil || s.adapter.guard != s.guard || !sameScopedBatchBindingPrivate(actual, s.binding) || s.lifetime == nil {
		s.mu.Unlock()
		return result, grant, ErrInvalid
	}
	reservation, ok := directionalraw.FromContext(ctx)
	if !ok || reservation.IdentityPrivate() != actual.RequestID || reservation.Closed() || reservation.Attached() {
		s.mu.Unlock()
		return result, grant, ErrInvalid
	}
	expected, err := validateScopedBatchBindingPrivate(s.adapter, s.binding)
	if err == nil {
		metadata, failure := manager.Metadata(actual.SessionID, actual.Generation)
		if failure != "" {
			err = scopeAdapterFailure(failure, "")
		} else {
			expected.Context = acquisition.AcquisitionContext{SessionID: actual.SessionID, Generation: actual.Generation, PositionEncoding: metadata.PositionEncoding}
			identity, _ := json.Marshal(expected)
			expected.Context.ID = fmt.Sprintf("public-acquisition-v2:%x", sha256.Sum256(identity))
			if !reflect.DeepEqual(expected, req) {
				err = ErrInvalid
			}
		}
	}
	// Semantic seed-admission is an incompatible extra capability for this
	// frozen-supply route; ordinary/prepared admission order is unchanged.
	if err == nil && manager.SeedBindingRequested(actual.SessionID, actual.Generation) {
		err = ErrInvalid
	}
	if err == nil {
		err = s.adapter.check(s.lifetime)
	}
	if err == nil {
		err = s.adapter.check(ctx)
	}
	if err != nil {
		s.mu.Unlock()
		return result, grant, err
	}
	grant, err = acquisitionauthority.ConsumeSeedAuthority(authority, actual)
	if err == nil && (grant.Provenance != seedbinding.CallerAssertedLocal || grant.Authenticated || grant.HostReceiptID != "" || !grant.RetainSeedSpec || !bytes.Equal(grant.SeedSpec, s.seeds)) {
		err = ErrInvalid
	}
	if err != nil {
		s.mu.Unlock()
		return result, acquisitionauthority.SeedGrant{}, err
	}
	s.spent = true
	s.mu.Unlock()
	opCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.lifetime, cancel)
	defer stop()
	defer cancel()
	client := &scopedBatchOperationClientPrivate{adapter: s.adapter, client: s.adapter.nativeDirectionalClientPrivate(), reservation: reservation, operation: opCtx, lifetime: s.lifetime, ac: expected.Context}
	client.active.Store(true)
	defer client.active.Store(false)
	result, err = acquisition.Acquire(opCtx, client, expected)
	if err == nil {
		err = client.check(opCtx)
	}
	if err != nil {
		return acquisition.Result{}, acquisitionauthority.SeedGrant{}, err
	}
	return result, grant, nil
}

// Private and revocable even inside this package. No exported operation returns
// this wrapper, its client, a callback, or a continuation that could reuse it.
type scopedBatchOperationClientPrivate struct {
	adapter             *scopeRuntimeAdapter
	client              acquisition.Client
	reservation         directionalraw.Reservation
	operation, lifetime context.Context
	ac                  acquisition.AcquisitionContext
	active              atomic.Bool
}

func (c *scopedBatchOperationClientPrivate) check(ctx context.Context) error {
	if c == nil || ctx == nil || !c.active.Load() || c.operation == nil || c.lifetime == nil {
		return ErrInvalid
	}
	if err := c.operation.Err(); err != nil {
		return err
	}
	if err := c.lifetime.Err(); err != nil {
		return err
	}
	r, ok := directionalraw.FromContext(ctx)
	if !ok || !r.Same(c.reservation) || r.Closed() || r.IdentityPrivate() == "" {
		return ErrInvalid
	}
	if err := c.adapter.check(c.lifetime); err != nil {
		return err
	}
	if err := c.adapter.check(c.operation); err != nil {
		return err
	}
	return c.adapter.check(ctx)
}
func (c *scopedBatchOperationClientPrivate) DocumentSymbols(ctx context.Context, p lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	if err := c.check(ctx); err != nil {
		return nil, err
	}
	return c.client.DocumentSymbols(ctx, p)
}
func (c *scopedBatchOperationClientPrivate) PrepareCallHierarchy(ctx context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	if err := c.check(ctx); err != nil {
		return nil, err
	}
	return c.client.PrepareCallHierarchy(ctx, p)
}
func (c *scopedBatchOperationClientPrivate) IncomingCalls(ctx context.Context, p lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	if err := c.check(ctx); err != nil {
		return nil, false, err
	}
	return c.client.IncomingCalls(ctx, p)
}
func (c *scopedBatchOperationClientPrivate) OutgoingCalls(ctx context.Context, p lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	if err := c.check(ctx); err != nil {
		return nil, false, err
	}
	return c.client.OutgoingCalls(ctx, p)
}
func (c *scopedBatchOperationClientPrivate) PrepareDocument(ctx context.Context, ac acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
	if err := c.check(ctx); err != nil {
		return acquisition.Supply{}, err
	}
	if ac != c.ac {
		return acquisition.Supply{}, ErrInvalid
	}
	return c.adapter.nativeSupplyPrivate(ctx, ac, l)
}
