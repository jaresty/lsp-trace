//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/internal/seedformat"
	"lsp-trace/sessionruntime"
)

// Pin the closed exported signature as well as checking the former escape by
// source. Results and provenance data are not executable clients or permits.
var _ func(context.Context, *sessionruntime.Manager, *ScopedBatchSelectionPrivate, acquisitionauthority.SeedAuthority, acquisitionauthority.Binding, acquisition.Request) (acquisition.Result, acquisitionauthority.SeedGrant, error) = AcquireScopedBatchPrivate

func TestScopedClosedAPINeverReturnsClient(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "scoped_batch_selection_private.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !fn.Name.IsExported() || !strings.Contains(fn.Name.Name, "ScopedBatch") {
			continue
		}
		if fn.Name.Name == "ConsumeScopedBatchSelectionPrivate" {
			t.Error("ASSERT_CLOSED_SCOPED_NO_RAW_CONSUMER")
		}
		for _, fields := range []*ast.FieldList{fn.Type.Params, fn.Type.Results} {
			if fields == nil {
				continue
			}
			ast.Inspect(fields, func(n ast.Node) bool {
				if selector, ok := n.(*ast.SelectorExpr); ok && selector.Sel.Name == "Client" {
					t.Errorf("ASSERT_CLOSED_SCOPED_NO_EXPORTED_CLIENT: %s exposes client", fn.Name.Name)
				}
				if _, ok := n.(*ast.FuncType); ok {
					t.Error("ASSERT_CLOSED_SCOPED_NO_EXPORTED_CLIENT: callback seam")
				}
				return true
			})
		}
	}
	t.Log("ASSERT_CLOSED_SCOPED_NO_EXPORTED_CLIENT: PASS")
}

func closedInputsPrivate(t *testing.T, ctx context.Context, f *ScopedBatchFixturePrivate) (*ScopedBatchSelectionPrivate, acquisitionauthority.SeedAuthority, acquisitionauthority.Binding, acquisition.Request) {
	t.Helper()
	one := 1
	zero := uint32(0)
	manifest := acquisitionmanifest.Manifest{SchemaVersion: acquisitionmanifest.ManifestVersion, CoordinateConvention: "zero-based-session", Root: acquisitionmanifest.Target{ID: "root", Locator: acquisition.Locator{URI: f.Nomination.URI, Line: &zero, Character: &zero, LanguageID: "go"}, DownDepth: &one, UpDepth: &one}, RequiredTargets: []acquisitionmanifest.Target{}}
	seeds, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Defaults: seedformat.Defaults{DownDepth: &one, UpDepth: &one}, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "RootSeed", Path: f.Nomination.Path, Line: 1, Column: 1}}}}, f.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	selection, binding, err := f.Mint(ctx, "closed-acquire-fixture", manifest, seeds)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := acquisitionauthority.MintSeedAuthority(binding, seeds, seedbinding.CallerAssertedLocal, false, "", true)
	if err != nil {
		t.Fatal(err)
	}
	req, err := manifest.Request(acquisitionmanifest.SliceV3)
	if err != nil {
		t.Fatal(err)
	}
	req.Context = acquisition.AcquisitionContext{SessionID: f.Identity.SessionID, Generation: f.Identity.Generation, PositionEncoding: "utf-16"}
	identity, _ := json.Marshal(req)
	req.Context.ID = fmt.Sprintf("public-acquisition-v2:%x", sha256.Sum256(identity))
	return selection, authority, binding, req
}

// These are closed-helper boundary unit fixtures, not replacements for the
// producer-created reservation in the actual V5/Core.Run integration tests.
// Any attached carrier below is released only through its real Manager owner.
func TestScopedClosedAcquireRequiresOriginalAuthorityAndExactRequest(t *testing.T) {
	WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *ScopedBatchFixturePrivate) {
		f.Fault("empty-directions")
		selection, authority, binding, req := closedInputsPrivate(t, ctx, f)
		reservation := directionalraw.NewReservation(binding.RequestID)
		callCtx := reservation.Context(ctx)
		defer reservation.Release()
		for _, kind := range []string{"zero-authority", "foreign-authority", "different-seed-grant", "authenticated-grant", "another-nomination", "context", "budget", "route", "session", "manager"} {
			supplied := authority
			actual := binding
			request := req
			manager := f.Manager
			switch kind {
			case "zero-authority":
				supplied = acquisitionauthority.SeedAuthority{}
			case "foreign-authority":
				foreign := binding
				foreign.RequestID = "other"
				var err error
				supplied, err = acquisitionauthority.MintSeedAuthority(foreign, selection.state.seeds, seedbinding.CallerAssertedLocal, false, "", true)
				if err != nil {
					t.Fatal(err)
				}
			case "different-seed-grant", "authenticated-grant":
				seeds := bytes.Clone(selection.state.seeds)
				authenticated := kind == "authenticated-grant"
				if !authenticated {
					seeds = bytes.Replace(seeds, []byte("RootSeed"), []byte("OtherSeed"), 1)
				}
				var err error
				supplied, err = acquisitionauthority.MintSeedAuthority(binding, seeds, seedbinding.CallerAssertedLocal, authenticated, "", true)
				if err != nil {
					t.Fatal(err)
				}
			case "another-nomination":
				request.Root.Locator.URI = f.adapter.files[1].URI
			case "context":
				request.Context.Generation++
			case "budget":
				request.Limits.MaxRequests++
			case "route":
				actual.Route = "other"
			case "session":
				actual.SessionID = "other"
			case "manager":
				manager = f.ForeignManager(t)
			}
			before, _ := f.Traffic()
			result, grant, err := AcquireScopedBatchPrivate(callCtx, manager, selection, supplied, actual, request)
			after, _ := f.Traffic()
			if err == nil || result.Graph.SchemaVersion != "" || len(grant.SeedSpec) != 0 || before != after || selection.state.spent {
				t.Fatalf("ASSERT_CLOSED_SCOPED_PROOF_PREFLIGHT: %s err=%v spent=%t traffic=%d/%d", kind, err, selection.state.spent, before, after)
			}
		}
		result, grant, err := AcquireScopedBatchPrivate(callCtx, f.Manager, selection, authority, binding, req)
		if err != nil || len(result.Targets) != 1 || !selection.state.spent || grant.Provenance != seedbinding.CallerAssertedLocal || grant.Authenticated || grant.HostReceiptID != "" || !bytes.Equal(grant.SeedSpec, selection.state.seeds) {
			t.Fatalf("ASSERT_CLOSED_SCOPED_VERIFIED_EXECUTION: %v %+v", err, grant)
		}
		if !reservation.Release() {
			t.Fatal("ASSERT_CLOSED_SCOPED_REAL_TERMINAL")
		}
		before, _ := f.Traffic()
		copied := *selection
		if _, _, err := AcquireScopedBatchPrivate(callCtx, f.Manager, &copied, authority, binding, req); err == nil {
			t.Fatal("ASSERT_CLOSED_SCOPED_RETURN_REVOKED")
		}
		after, _ := f.Traffic()
		if before != after {
			t.Fatal("ASSERT_CLOSED_SCOPED_RETURN_NO_REUSE_TRAFFIC")
		}
		t.Log("ASSERT_CLOSED_SCOPED_PROOF_PREFLIGHT: PASS")
		t.Log("ASSERT_CLOSED_SCOPED_VERIFIED_EXECUTION: PASS")
		t.Log("ASSERT_CLOSED_SCOPED_RETURN_REVOKED: PASS")
	})
}

func TestScopedClosedAcquireReservationPreflightDoesNotSpend(t *testing.T) {
	WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *ScopedBatchFixturePrivate) {
		f.Fault("empty-directions")
		selection, authority, binding, req := closedInputsPrivate(t, ctx, f)
		wrong := directionalraw.NewReservation("another-operation")
		for _, callCtx := range []context.Context{ctx, wrong.Context(ctx)} {
			if _, _, err := AcquireScopedBatchPrivate(callCtx, f.Manager, selection, authority, binding, req); err == nil || selection.state.spent {
				t.Fatal("ASSERT_CLOSED_SCOPED_RESERVATION_PREFLIGHT")
			}
		}
		reservation := directionalraw.NewReservation(binding.RequestID)
		callCtx := reservation.Context(ctx)
		defer reservation.Release()
		if _, _, err := AcquireScopedBatchPrivate(callCtx, f.Manager, selection, authority, binding, req); err != nil {
			t.Fatal("ASSERT_CLOSED_SCOPED_PREFLIGHT_PRESERVES_EXECUTION: ", err)
		}
		// Even a newly minted selector cannot reuse the live carrier already
		// attached to the previous closed acquisition operation.
		second, secondAuthority, secondBinding, secondReq := closedInputsPrivate(t, ctx, f)
		if _, _, err := AcquireScopedBatchPrivate(callCtx, f.Manager, second, secondAuthority, secondBinding, secondReq); err == nil || second.state.spent {
			t.Fatal("ASSERT_CLOSED_SCOPED_ATTACHED_RESERVATION")
		}
		if !reservation.Release() {
			t.Fatal("ASSERT_CLOSED_SCOPED_REAL_TERMINAL")
		}
		// The same unspent selection also refuses that genuinely closed carrier.
		if _, _, err := AcquireScopedBatchPrivate(callCtx, f.Manager, second, secondAuthority, secondBinding, secondReq); err == nil || second.state.spent {
			t.Fatal("ASSERT_CLOSED_SCOPED_CLOSED_RESERVATION")
		}
		t.Log("ASSERT_CLOSED_SCOPED_RESERVATION_PREFLIGHT: PASS")
		t.Log("ASSERT_CLOSED_SCOPED_CLOSED_RESERVATION: PASS")
	})
}

// A test-only delegation probe makes a missing wrapper guard observable without
// substituting a fake acquirer or forging producer bytes/completion. It is never
// installed by an exported operation or used for V5 qualification.
type scopedClosedDelegationProbePrivateTest struct{ calls int }

func (p *scopedClosedDelegationProbePrivateTest) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	p.calls++
	return nil, nil
}
func (p *scopedClosedDelegationProbePrivateTest) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	p.calls++
	return nil, nil
}
func (p *scopedClosedDelegationProbePrivateTest) IncomingCalls(context.Context, lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	p.calls++
	return nil, false, nil
}
func (p *scopedClosedDelegationProbePrivateTest) OutgoingCalls(context.Context, lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	p.calls++
	return nil, false, nil
}

func TestScopedClosedWrapperChecksEveryMethodAndSupply(t *testing.T) {
	WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *ScopedBatchFixturePrivate) {
		_, _, binding, req := closedInputsPrivate(t, ctx, f)
		r := directionalraw.NewReservation(binding.RequestID)
		operation := r.Context(ctx)
		// This private wrapper is never obtained from an exported operation. Unit
		// inspection tests otherwise unreachable substitution/revocation states.
		probe := &scopedClosedDelegationProbePrivateTest{}
		wrapper := &scopedBatchOperationClientPrivate{adapter: f.adapter, client: probe, reservation: r, operation: operation, lifetime: ctx, ac: req.Context}
		wrapper.active.Store(true)
		other := directionalraw.NewReservation(binding.RequestID)
		p := lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: f.Nomination.URI}}
		prepare := lsp.PrepareCallHierarchyParams{TextDocument: p.TextDocument}
		item := scopeAdapterItem(f.Nomination.URI)
		checkAll := func(callCtx context.Context, label string) {
			t.Helper()
			before, _ := f.Traffic()
			beforeDelegation := probe.calls
			refused := func(method string, err error) {
				t.Helper()
				if err == nil {
					t.Fatalf("ASSERT_CLOSED_SCOPED_EVERY_METHOD: %s %s delegated without operation validity", label, method)
				}
			}
			_, err := wrapper.DocumentSymbols(callCtx, p)
			refused("documentSymbols", err)
			_, err = wrapper.PrepareCallHierarchy(callCtx, prepare)
			refused("prepare", err)
			_, _, err = wrapper.IncomingCalls(callCtx, item)
			refused("incoming", err)
			_, _, err = wrapper.OutgoingCalls(callCtx, item)
			refused("outgoing", err)
			_, err = wrapper.PrepareDocument(callCtx, req.Context, req.Root.Locator)
			refused("supply", err)
			after, _ := f.Traffic()
			if before != after || beforeDelegation != probe.calls {
				t.Fatalf("ASSERT_CLOSED_SCOPED_REFUSAL_NO_WIRE: %s", label)
			}
		}
		checkAll(ctx, "missing")
		checkAll(other.Context(ctx), "substituted-same-label")
		mintCtx, cancelMint := context.WithCancel(ctx)
		wrapper.lifetime = mintCtx
		cancelMint()
		checkAll(operation, "cancelled-mint")
		wrapper.lifetime = ctx
		opCtx, cancelOp := context.WithCancel(operation)
		wrapper.operation = opCtx
		cancelOp()
		checkAll(operation, "cancelled-operation")
		wrapper.operation = operation
		wrapper.active.Store(false)
		checkAll(operation, "returned")
		wrapper.active.Store(true)
		if _, err := wrapper.DocumentSymbols(operation, p); err != nil || probe.calls != 1 {
			t.Fatal("ASSERT_CLOSED_SCOPED_VALID_METHOD_DELEGATES")
		}
		params, _ := json.Marshal(prepare)
		if err := f.adapter.invoke(func() error {
			_, err := f.adapter.nativeDirectionalRoundTripLocked(operation, acquisition.WireRequest{Context: req.Context, Method: "textDocument/prepareCallHierarchy", Params: params, MaxBytes: scopeAdapterResponse, MaxMessages: scopeAdapterMessages})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if !r.Release() {
			t.Fatal("ASSERT_CLOSED_SCOPED_REAL_TERMINAL")
		}
		checkAll(operation, "closed")
		t.Log("ASSERT_CLOSED_SCOPED_EVERY_METHOD: PASS")
	})
}
