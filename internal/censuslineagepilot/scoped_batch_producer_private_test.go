//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionmanifest"
	orchestration "lsp-trace/internal/acquisitionorchestration"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/seedformat"
)

func batchFixtureRequest(t *testing.T, f *pilot.ScopedBatchFixturePrivate) orchestration.PlannedBatchRequest {
	t.Helper()
	zero := uint32(0)
	one := 1
	manifest := acquisitionmanifest.Manifest{SchemaVersion: acquisitionmanifest.ManifestVersion, CoordinateConvention: "zero-based-session", Root: acquisitionmanifest.Target{ID: "root", Locator: acquisition.Locator{URI: f.Nomination.URI, LanguageID: "go", Line: &zero, Character: &zero}, DownDepth: &one, UpDepth: &one}, RequiredTargets: []acquisitionmanifest.Target{}}
	seeds, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Defaults: seedformat.Defaults{DownDepth: &one, UpDepth: &one}, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "RootSeed", Path: f.Nomination.Path, Line: 1, Column: 1}}}}, f.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	return orchestration.PlannedBatchRequest{SessionID: f.Identity.SessionID, Generation: f.Identity.Generation, RequestID: "scoped-batch-fixture", Manifest: manifest, CanonicalSeedsV2: seeds}
}
func mint(t *testing.T, ctx context.Context, f *pilot.ScopedBatchFixturePrivate, r orchestration.PlannedBatchRequest) (*pilot.ScopedBatchSelectionPrivate, acquisitionauthority.Binding) {
	t.Helper()
	s, b, err := f.Mint(ctx, r.RequestID, r.Manifest, r.CanonicalSeedsV2)
	if err != nil {
		t.Fatal(err)
	}
	return s, b
}
func assertReleased(t *testing.T, f *pilot.ScopedBatchFixturePrivate, attached bool) {
	t.Helper()
	live, total, _, releases, general, occupied := f.Manager.DirectionalAccountingPrivate()
	active, closed, started, released := f.Manager.DirectionalGenericTerminalPrivate()
	prepLive, prepTotal, _, prepReleases, prepOccupied := f.Manager.DirectionalPreparationAccountingPrivate()
	if live != 0 || general != 0 || occupied || active != 0 || prepLive != 0 || prepOccupied {
		t.Fatalf("ASSERT_SCOPED_MANAGER_BYTES_RELEASED live=%d general=%d occupied=%t active=%d", live, general, occupied, active)
	}
	if attached && (total+prepTotal == 0 || releases+prepReleases == 0 || !closed || !started || !released) {
		t.Fatalf("ASSERT_SCOPED_GENUINE_TERMINAL total=%d releases=%d closed=%t started=%t released=%t", total, releases, closed, started, released)
	}
	t.Log("ASSERT_SCOPED_MANAGER_BYTES_RELEASED: PASS")
}
func TestScopedPlannedBatchPrivateActualProducer(t *testing.T) {
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		f.Fault("empty-directions")
		request := batchFixtureRequest(t, f)
		selection, _ := mint(t, ctx, f, request)
		copySelection := *selection
		_, prepared := f.Traffic()
		result, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, selection)
		if failure != nil {
			t.Fatalf("ASSERT_SCOPED_ACTUAL_PRODUCER: %v", failure)
		}
		if !result.BoundedTraversalComplete || result.SessionID != request.SessionID || result.Generation != request.Generation {
			t.Fatal("ASSERT_SCOPED_BOUNDED_IDENTITY")
		}
		assertReleased(t, f, true)
		if _, err := graphprovenance.ValidateFor(result.RawV5, graphprovenance.Family, "v5"); err != nil {
			t.Fatal(err)
		}
		var envelope graphprovenance.EvidenceV5
		if err := json.Unmarshal(result.RawV5, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.SeedSpec == nil || !bytes.Equal(envelope.SeedSpec.Bytes, request.CanonicalSeedsV2) {
			t.Fatal("ASSERT_SCOPED_EXACT_CANONICAL_SEEDS")
		}
		native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
		if err != nil {
			t.Fatal(err)
		}
		var g graph.Result
		if err = json.Unmarshal(native, &g); err != nil {
			t.Fatal(err)
		}
		if g.SchemaVersion != graph.SchemaVersionV5 || len(g.Invocation.Seeds) != 1 || g.Invocation.Seeds[0].Label != "root" {
			t.Fatalf("ASSERT_SCOPED_NATIVE_GRAPH_LABELS %+v", g.Invocation.Seeds)
		}
		semantic, afterPrepared := f.Traffic()
		if semantic != 3 || afterPrepared != prepared {
			t.Fatalf("ASSERT_SCOPED_FROZEN_SUPPLY semantic=%d preparations=%d/%d", semantic, prepared, afterPrepared)
		}
		detached := bytes.Clone(result.RawV5)
		if _, failure = orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, &copySelection); failure == nil {
			t.Fatal("ASSERT_SCOPED_COPY_SPENT_REJECTED")
		}
		afterSemantic, _ := f.Traffic()
		if afterSemantic != semantic || !bytes.Equal(result.RawV5, detached) {
			t.Fatal("ASSERT_SCOPED_DETACHED_NO_REPLAY_TRAFFIC")
		}
		t.Log("ASSERT_SCOPED_ACTUAL_PRODUCER: PASS")
	})
}

func TestScopedPlannedBatchPrivateRejectsMixedBindings(t *testing.T) {
	for _, kind := range []string{"zero", "nil", "request", "session", "generation", "input", "seeds", "manager", "expired", "cancelled", "foreign-context"} {
		t.Run(kind, func(t *testing.T) {
			pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
				request := batchFixtureRequest(t, f)
				selection, _ := mint(t, ctx, f, request)
				manager := f.Manager
				switch kind {
				case "zero":
					selection = &pilot.ScopedBatchSelectionPrivate{}
				case "nil":
					selection = nil
				case "request":
					request.RequestID += "-other"
				case "session":
					request.SessionID += "-other"
				case "generation":
					request.Generation++
				case "input":
					n := 16
					request.Manifest.Limits.MaxNodes = &n
				case "seeds":
					request.CanonicalSeedsV2 = bytes.Replace(request.CanonicalSeedsV2, []byte("RootSeed"), []byte("OtherSeed"), 1)
				case "manager":
					manager = f.ForeignManager(t)
				case "expired":
					f.Expire()
				case "foreign-context":
					ctx = context.Background()
				case "cancelled":
					c, cancel := context.WithCancel(ctx)
					cancel()
					ctx = c
				}
				before, _ := f.Traffic()
				out, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, manager, request, selection)
				if failure == nil || len(out.RawV5) != 0 {
					t.Fatal("ASSERT_SCOPED_MIXED_BINDING_REFUSED")
				}
				after, _ := f.Traffic()
				if after != before {
					t.Fatal("ASSERT_SCOPED_INVALID_ZERO_TRAFFIC")
				}
				assertReleased(t, f, false)
				t.Log("ASSERT_SCOPED_MIXED_BINDING_REFUSED: PASS")
			})
		})
	}
}

func TestScopedPlannedBatchPrivateConsumedFailures(t *testing.T) {
	for _, kind := range []string{"malformed", "provider-budget", "poison"} {
		t.Run(kind, func(t *testing.T) {
			pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
				request := batchFixtureRequest(t, f)
				selection, _ := mint(t, ctx, f, request)
				f.Fault(kind)
				out, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, selection)
				if failure == nil && out.BoundedTraversalComplete {
					t.Fatal("ASSERT_SCOPED_CONSUMED_FAILURE_NOT_COMPLETE")
				}
				assertReleased(t, f, true)
				before, _ := f.Traffic()
				if _, failure = orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, selection); failure == nil {
					t.Fatal("ASSERT_SCOPED_CONSUMED_NOT_REUSABLE")
				}
				after, _ := f.Traffic()
				if before != after {
					t.Fatal("ASSERT_SCOPED_NO_AUTO_RETRY")
				}
				t.Log("ASSERT_SCOPED_CONSUMED_FAILURE_NOT_COMPLETE: PASS")
			})
		})
	}
}

func TestScopedSelectionPrivateNominationBoundary(t *testing.T) {
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		request := batchFixtureRequest(t, f)
		for _, uri := range []string{f.ContextURI, "file:///external.go"} {
			request.Manifest.Root.Locator.URI = uri
			if _, _, err := f.Mint(ctx, request.RequestID, request.Manifest, request.CanonicalSeedsV2); err == nil {
				t.Fatal("ASSERT_SCOPED_CONTEXT_EXTERNAL_NOT_NOMINATED")
			}
		}
		t.Log("ASSERT_SCOPED_CONTEXT_EXTERNAL_NOT_NOMINATED: PASS")
	})
}

func TestScopedSelectionPrivateVerifiedConcurrentExecutionAndDetachedInputs(t *testing.T) {
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		f.Fault("empty-directions")
		request := batchFixtureRequest(t, f)
		selection, b := mint(t, ctx, f, request)
		seeds := bytes.Clone(request.CanonicalSeedsV2)
		clear(b.Input)
		clear(request.CanonicalSeedsV2)
		request.CanonicalSeedsV2 = seeds
		var successes atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			copied := *selection
			wg.Add(1)
			go func() {
				defer wg.Done()
				out, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, &copied)
				if failure == nil && out.BoundedTraversalComplete {
					successes.Add(1)
				}
			}()
		}
		wg.Wait()
		if successes.Load() != 1 {
			t.Fatalf("ASSERT_SCOPED_CONCURRENT_ACTUAL_EXECUTION: %d", successes.Load())
		}
		semantic, _ := f.Traffic()
		if semantic != 3 {
			t.Fatalf("ASSERT_SCOPED_SINGLE_EXECUTION_WIRE: %d", semantic)
		}
		assertReleased(t, f, true)
		t.Log("ASSERT_SCOPED_CONCURRENT_ACTUAL_EXECUTION: PASS")
	})
}

func TestScopedSelectionPrivateMintBoundsAndLifetime(t *testing.T) {
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		request := batchFixtureRequest(t, f)
		for _, kind := range []string{"empty-id", "bad-manifest", "invalid-limits", "noncanonical-seeds", "oversize-seeds", "invalid-seeds"} {
			r := request
			switch kind {
			case "empty-id":
				r.RequestID = ""
			case "bad-manifest":
				r.Manifest.SchemaVersion = "bad"
			case "invalid-limits":
				zero := 0
				r.Manifest.Limits.TimeoutMS = &zero
			case "noncanonical-seeds":
				r.CanonicalSeedsV2 = append(bytes.Clone(r.CanonicalSeedsV2), ' ')
			case "oversize-seeds":
				r.CanonicalSeedsV2 = bytes.Repeat([]byte("x"), seedformat.MaxInputBytes+1)
			case "invalid-seeds":
				r.CanonicalSeedsV2 = []byte(`{"schema_version":"bad"}`)
			}
			if _, _, err := f.Mint(ctx, r.RequestID, r.Manifest, r.CanonicalSeedsV2); err == nil {
				t.Fatalf("ASSERT_SCOPED_MINT_STRICT_BOUNDED: %s", kind)
			}
		}
		mintContext, cancel := context.WithCancel(ctx)
		selection, _ := mint(t, mintContext, f, request)
		cancel()
		if _, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, selection); failure == nil {
			t.Fatal("ASSERT_SCOPED_MINT_LIFETIME")
		}
		t.Log("ASSERT_SCOPED_MINT_STRICT_BOUNDED: PASS")
		t.Log("ASSERT_SCOPED_MINT_LIFETIME: PASS")
	})
}

func TestScopedSelectionPrivateRejectsExpiredManagerLease(t *testing.T) {
	var held *pilot.ScopedBatchSelectionPrivate
	var request orchestration.PlannedBatchRequest
	var fixture *pilot.ScopedBatchFixturePrivate
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		request = batchFixtureRequest(t, f)
		held, _ = mint(t, ctx, f, request)
		fixture = f
	})
	if _, failure := orchestration.ExecuteScopedPlannedBatchPrivate(context.Background(), fixture.Manager, request, held); failure == nil {
		t.Fatal("ASSERT_SCOPED_EXPIRED_MANAGER_LEASE")
	}
	t.Log("ASSERT_SCOPED_EXPIRED_MANAGER_LEASE: PASS")
}

func TestScopedPlannedBatchPrivateAuthorityPrecedesSelection(t *testing.T) {
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		f.Fault("empty-directions")
		request := batchFixtureRequest(t, f)
		selection, _ := mint(t, ctx, f, request)
		bad := request
		bad.CanonicalSeedsV2 = []byte(`{}`)
		if _, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, bad, selection); failure == nil {
			t.Fatal("ASSERT_SCOPED_AUTHORITY_REQUIRED")
		}
		// Invalid custody never spends the client selector; the original valid call
		// must still execute through real authority verification and native capture.
		if _, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, selection); failure != nil {
			t.Fatal("ASSERT_SCOPED_AUTHORITY_BEFORE_SELECTION: ", failure)
		}
		assertReleased(t, f, true)
		t.Log("ASSERT_SCOPED_AUTHORITY_BEFORE_SELECTION: PASS")
	})
}

func TestScopedPlannedBatchPrivateCancelAfterAttachedResponse(t *testing.T) {
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		request := batchFixtureRequest(t, f)
		c, cancel := context.WithCancel(ctx)
		defer cancel()
		selection, _ := mint(t, c, f, request)
		f.CancelAfterResponse(cancel)
		out, failure := orchestration.ExecuteScopedPlannedBatchPrivate(c, f.Manager, request, selection)
		if failure == nil && out.BoundedTraversalComplete {
			t.Fatal("ASSERT_SCOPED_ATTACHED_CANCEL_NOT_COMPLETE")
		}
		assertReleased(t, f, true)
		t.Log("ASSERT_SCOPED_ATTACHED_CANCEL_NOT_COMPLETE: PASS")
	})
}

func TestScopedPlannedBatchPrivateMintCancelledAfterConsume(t *testing.T) {
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		request := batchFixtureRequest(t, f)
		mintContext, cancelMint := context.WithCancel(ctx)
		defer cancelMint()
		selection, _ := mint(t, mintContext, f, request)
		f.CancelAfterResponse(cancelMint)
		// The operation context deliberately remains live. Only the original mint
		// lifetime is revoked after the first actual Manager-owned response.
		out, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, selection)
		if failure == nil || len(out.RawV5) != 0 || ctx.Err() != nil {
			t.Fatal("ASSERT_CLOSED_SCOPED_MINT_CANCEL_AFTER_SPEND")
		}
		semantic, _ := f.Traffic()
		if semantic != 1 {
			t.Fatalf("ASSERT_CLOSED_SCOPED_MINT_CANCEL_BEFORE_NEXT_WIRE: %d", semantic)
		}
		assertReleased(t, f, true)
		if _, failure = orchestration.ExecuteScopedPlannedBatchPrivate(ctx, f.Manager, request, selection); failure == nil {
			t.Fatal("ASSERT_CLOSED_SCOPED_CANCELLED_EXECUTION_SPENT")
		}
		t.Log("ASSERT_CLOSED_SCOPED_MINT_CANCEL_AFTER_SPEND: PASS")
		t.Log("ASSERT_CLOSED_SCOPED_MINT_CANCEL_BEFORE_NEXT_WIRE: PASS")
	})
}

type producerAcquirer struct {
	t       *testing.T
	fixture *pilot.ScopedBatchFixturePrivate
	calls   int
}

func (a *producerAcquirer) AcquireV5(ctx context.Context, b censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	a.calls++
	manifest := b.AcquisitionManifest(acquisitionmanifest.Limits{})
	selection, _, err := a.fixture.Mint(ctx, b.BatchID, manifest, b.CanonicalSeedsV2)
	if err != nil {
		return censusacquisition.AcquiredV5{}, err
	}
	out, failure := orchestration.ExecuteScopedPlannedBatchPrivate(ctx, a.fixture.Manager, orchestration.PlannedBatchRequest{SessionID: b.Session.SessionID, Generation: b.Session.Generation, RequestID: b.BatchID, Manifest: manifest, CanonicalSeedsV2: b.CanonicalSeedsV2}, selection)
	if failure != nil {
		return censusacquisition.AcquiredV5{}, failure.Err
	}
	if !out.BoundedTraversalComplete {
		return censusacquisition.AcquiredV5{}, errors.New("actual producer bounded traversal incomplete")
	}
	assertReleased(a.t, a.fixture, true)
	return censusacquisition.AcquiredV5{Session: b.Session, Raw: out.RawV5}, nil
}
func TestScopedPlannedBatchPrivateCoreAdmission(t *testing.T) {
	pilot.WithScopedBatchFixturePrivate(t, func(ctx context.Context, f *pilot.ScopedBatchFixturePrivate) {
		// Discovery is the actual adapter; only post-discovery directional replies
		// become empty, so no discovery accounting or producer bytes are fabricated.
		d, err := f.Discover(ctx, f.Identity)
		if err != nil {
			t.Fatal(err)
		}
		f.Fault("empty-directions")
		acquirer := &producerAcquirer{t: t, fixture: f}
		core := censusacquisition.Core{Discoverer: fixedDiscovery{d}, Acquirer: acquirer, Planning: &censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 4}}
		projection, err := core.Run(ctx, f.Identity)
		if err != nil {
			t.Fatal("ASSERT_SCOPED_CORE_ACTUAL_PRODUCER_ADMISSION: ", err)
		}
		if acquirer.calls != 1 || len(projection.Constituents) != 1 || len(projection.Batches[0].Targets) != 4 {
			t.Fatalf("ASSERT_SCOPED_CORE_ADMISSION_COUNTS calls=%d constituents=%d", acquirer.calls, len(projection.Constituents))
		}
		t.Log("ASSERT_SCOPED_CORE_ACTUAL_PRODUCER_ADMISSION: PASS")
	})
}

type fixedDiscovery struct{ value censusacquisition.Discovery }

func (d fixedDiscovery) Discover(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	return d.value, nil
}
