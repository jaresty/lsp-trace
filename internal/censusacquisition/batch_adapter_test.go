package censusacquisition

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/operation"
)

type fakeBatchSession struct {
	id         string
	generation uint64
	requests   []operation.Request
	respond    func(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

func (s *fakeBatchSession) identity() SessionIdentity {
	return SessionIdentity{SessionID: s.id, Generation: s.generation}
}
func (s *fakeBatchSession) workspace() (string, error) { return "/w", nil }
func (s *fakeBatchSession) execute(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
	s.requests = append(s.requests, cloneOperationRequest(request))
	return s.respond(ctx, request)
}

func cloneOperationRequest(in operation.Request) operation.Request {
	in.Input = bytes.Clone(in.Input)
	return in
}

func successfulBatchResponse(t *testing.T, request operation.Request, complete, truncated bool) operation.Result {
	t.Helper()
	var in acquisitionops.Input
	if err := json.Unmarshal(request.Input, &in); err != nil {
		t.Fatal(err)
	}
	targets := append([]acquisitionops.Target{in.SeedManifest.Root}, in.SeedManifest.RequiredTargets...)
	seeds := make([]graph.SeedResult, len(targets))
	invocationSeeds := make([]graph.InvocationSeed, len(targets))
	nodes := make([]graph.Node, len(targets))
	for i, target := range targets {
		position := graph.Position{Line: *target.Locator.Line, Character: *target.Locator.Character}
		nodes[i] = graph.NewNode(graph.Item{Name: target.ID, Kind: 12, URI: target.Locator.URI, Range: graph.Range{Start: position, End: position}, SelectionRange: graph.Range{Start: position, End: position}})
		seeds[i] = graph.SeedResult{Label: target.ID, ReachedNodeIDs: []string{nodes[i].ID}}
		invocationSeeds[i] = graph.InvocationSeed{Label: target.ID, At: target.Locator.URI, ResolvedURI: target.Locator.URI, LanguageID: "go"}
	}
	native, err := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: nodes, Seeds: seeds, Summary: graph.Summary{Complete: complete, Truncated: truncated}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake"}, Seeds: invocationSeeds, Provenance: graph.InvocationProvenance{InvocationID: "i", SourceRevision: "r", ServerVersion: "v"}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5WithSeedSpec(native, in.SessionID, in.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, request.RetainedSeedSpec)
	if err != nil {
		t.Fatal(err)
	}
	if !complete || truncated {
		var envelope graphprovenance.EvidenceV5
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		decoded, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(decoded, &value); err != nil {
			t.Fatal(err)
		}
		summary := value["summary"].(map[string]any)
		summary["complete"], summary["truncated"] = complete, truncated
		decoded, err = json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(decoded)
		envelope.GraphV5 = base64.StdEncoding.EncodeToString(decoded)
		envelope.GraphV5SHA256 = fmt.Sprintf("sha256:%x", digest)
		raw, err = json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, '\n')
	}
	return operation.Result{Artifact: raw}
}

func batchRequest(n, ordinal int) BatchRequest {
	targets := make([]PreparedTarget, n)
	for i := range targets {
		targets[i] = target(ordinal*63 + i)
	}
	seeds, err := combineSeeds(targets, "/w")
	if err != nil {
		panic(err)
	}
	return BatchRequest{Session: SessionIdentity{"s", 7}, CensusID: "c", BatchID: fmt.Sprintf("b%d", ordinal), Ordinal: ordinal, DownDepth: 1, UpDepth: 0, Targets: targets, CanonicalSeedsV2: seeds}
}

func TestBatchAdapterExactRequestIdentityAndImmutability(t *testing.T) {
	for _, n := range []int{1, 63} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := &fakeBatchSession{id: "s", generation: 7}
			s.respond = func(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
				return successfulBatchResponse(t, request, true, false), nil
			}
			a := newBatchAdapter(s, acquisitionops.Limits{})
			req := batchRequest(n, 0)
			originalSeeds := bytes.Clone(req.CanonicalSeedsV2)
			got, failure := a.AcquireBatch(context.Background(), req)
			if failure != nil {
				t.Fatal(failure)
			}
			if got.Session != req.Session || got.CensusID != req.CensusID || got.BatchID != req.BatchID || got.Ordinal != req.Ordinal || len(got.Raw) == 0 {
				t.Fatalf("ASSERT_BATCH_IDENTITY: %+v", got)
			}
			if len(s.requests) != 1 || s.requests[0].Name != acquisitionops.SliceV3 || s.requests[0].PublicationRoot != nil || s.requests[0].ArtifactStore != nil || !bytes.Equal(s.requests[0].RetainedSeedSpec, originalSeeds) {
				t.Fatal("ASSERT_EXACT_PRIVATE_REQUEST_NO_PUBLICATION")
			}
			var input acquisitionops.Input
			if err := json.Unmarshal(s.requests[0].Input, &input); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(input.SeedManifest, req.AcquisitionManifest(acquisitionops.Limits{})) || input.OutputVersion != graphprovenance.VersionV5 {
				t.Fatal("ASSERT_EXACT_MANIFEST_V5")
			}
			req.CanonicalSeedsV2[0] ^= 1
			got.Raw[0] ^= 1
			if bytes.Equal(s.requests[0].RetainedSeedSpec, req.CanonicalSeedsV2) {
				t.Fatal("ASSERT_INPUT_ALIASES")
			}
		})
	}
}

func TestBatchAdapterRejectsBoundsCancellationDriftAndIncomplete(t *testing.T) {
	cases := []struct {
		name                string
		mutate              func(*fakeBatchSession, *BatchRequest)
		complete, truncated bool
	}{
		{"zero", func(_ *fakeBatchSession, r *BatchRequest) { r.Targets = nil }, true, false},
		{"over63", func(_ *fakeBatchSession, r *BatchRequest) { *r = batchRequest(64, 0) }, true, false},
		{"drift-before", func(s *fakeBatchSession, _ *BatchRequest) { s.generation++ }, true, false},
		{"drift-after", func(s *fakeBatchSession, _ *BatchRequest) {
			old := s.respond
			s.respond = func(c context.Context, r operation.Request) (operation.Result, *operation.Failure) {
				out, f := old(c, r)
				s.generation++
				return out, f
			}
		}, true, false},
		{"partial", func(_ *fakeBatchSession, _ *BatchRequest) {}, false, false},
		{"truncated", func(_ *fakeBatchSession, _ *BatchRequest) {}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeBatchSession{id: "s", generation: 7}
			s.respond = func(ctx context.Context, request operation.Request) (operation.Result, *operation.Failure) {
				return successfulBatchResponse(t, request, tc.complete, tc.truncated), nil
			}
			r := batchRequest(1, 0)
			tc.mutate(s, &r)
			_, failure := newBatchAdapter(s, acquisitionops.Limits{}).AcquireBatch(context.Background(), r)
			if failure == nil {
				t.Fatal("ASSERT_TYPED_BATCH_FAILURE")
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &fakeBatchSession{id: "s", generation: 7, respond: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
		t.Fatal("executed cancelled batch")
		return operation.Result{}, nil
	}}
	if _, failure := newBatchAdapter(s, acquisitionops.Limits{}).AcquireBatch(ctx, batchRequest(1, 0)); failure == nil {
		t.Fatal("ASSERT_CANCEL_FAILURE")
	}
}

func TestBatchAdapterRejectsSeedSubstitutionReorderAndDuplicates(t *testing.T) {
	base := batchRequest(2, 0)
	cases := map[string]func(*BatchRequest){
		"reordered-targets":     func(r *BatchRequest) { r.Targets[0], r.Targets[1] = r.Targets[1], r.Targets[0] },
		"unrelated-valid-seeds": func(r *BatchRequest) { r.CanonicalSeedsV2 = batchRequest(2, 1).CanonicalSeedsV2 },
		"duplicate-ordinal":     func(r *BatchRequest) { r.Targets[1].CensusOrdinal = r.Targets[0].CensusOrdinal },
		"duplicate-identity":    func(r *BatchRequest) { r.Targets[1].SymbolIdentity = r.Targets[0].SymbolIdentity },
		"duplicate-coordinate": func(r *BatchRequest) {
			r.Targets[1].URI, r.Targets[1].SelectionRange = r.Targets[0].URI, r.Targets[0].SelectionRange
		},
		"duplicate-canonical": func(r *BatchRequest) { r.Targets[1].CanonicalSeedV2 = bytes.Clone(r.Targets[0].CanonicalSeedV2) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := cloneBatchRequest(base)
			mutate(&r)
			s := &fakeBatchSession{id: "s", generation: 7, respond: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
				t.Fatal("invalid seeds reached runtime")
				return operation.Result{}, nil
			}}
			if _, f := newBatchAdapter(s, acquisitionops.Limits{}).AcquireBatch(context.Background(), r); f == nil || f.Code != BatchFailureInvalidInput {
				t.Fatalf("ASSERT_BATCH_SEED_RECONCILIATION: %v", f)
			}
		})
	}
}

func TestBatchAdapterCancellationDuringNativeAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &fakeBatchSession{id: "s", generation: 7}
	s.respond = func(_ context.Context, r operation.Request) (operation.Result, *operation.Failure) {
		return successfulBatchResponse(t, r, true, false), nil
	}
	a := newBatchAdapter(s, acquisitionops.Limits{})
	a.beforeNativeAdmission = cancel
	if got, f := a.AcquireBatch(ctx, batchRequest(1, 0)); f == nil || f.Code != BatchFailureCancelled || len(got.Raw) != 0 {
		t.Fatalf("ASSERT_CANCEL_DURING_NATIVE_ADMISSION: result=%+v failure=%v", got, f)
	}
}

func TestBatchAdapterLateBatchFailureDoesNotMintResult(t *testing.T) {
	s := &fakeBatchSession{id: "s", generation: 7}
	calls := 0
	s.respond = func(_ context.Context, r operation.Request) (operation.Result, *operation.Failure) {
		calls++
		if calls == 2 {
			return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: errors.New("late failure")}
		}
		return successfulBatchResponse(t, r, true, false), nil
	}
	a := newBatchAdapter(s, acquisitionops.Limits{})
	if _, f := a.AcquireBatch(context.Background(), batchRequest(63, 0)); f != nil {
		t.Fatal(f)
	}
	if got, f := a.AcquireBatch(context.Background(), batchRequest(1, 1)); f == nil || f.Code != BatchFailureAcquisition || len(got.Raw) != 0 {
		t.Fatalf("ASSERT_LATE_BATCH_FAILURE_FAILS_CLOSED: result=%+v failure=%v", got, f)
	}
}

func TestBatchAdapterDeterministicMultipleBatches(t *testing.T) {
	for _, total := range []int{64, 127} {
		t.Run(fmt.Sprint(total), func(t *testing.T) {
			plans := []int{63, total - 63}
			if total == 127 {
				plans = []int{63, 63, 1}
			}
			s := &fakeBatchSession{id: "s", generation: 7}
			s.respond = func(ctx context.Context, r operation.Request) (operation.Result, *operation.Failure) {
				return successfulBatchResponse(t, r, true, false), nil
			}
			a := newBatchAdapter(s, acquisitionops.Limits{})
			var first [][]byte
			for ordinal, n := range plans {
				got, f := a.AcquireBatch(context.Background(), batchRequest(n, ordinal))
				if f != nil {
					t.Fatal(f)
				}
				first = append(first, bytes.Clone(got.Raw))
			}
			if len(s.requests) != len(plans) {
				t.Fatal("ASSERT_BATCH_COUNT")
			}
			s.requests = nil
			for ordinal, n := range plans {
				got, f := a.AcquireBatch(context.Background(), batchRequest(n, ordinal))
				if f != nil || !bytes.Equal(got.Raw, first[ordinal]) {
					t.Fatal("ASSERT_DETERMINISTIC_REPLAY", f)
				}
			}
		})
	}
}
