package censuschunking

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

type chunkAcquirerFunc func(context.Context, censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error)

func (f chunkAcquirerFunc) AcquireV5(ctx context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	return f(ctx, request)
}

func exactChunkV5(t *testing.T, request censusacquisition.BatchRequest) []byte {
	t.Helper()
	native, err := json.Marshal(graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation: graph.Invocation{
			Server:     graph.ServerInvocation{Command: "fake"},
			Seeds:      []graph.InvocationSeed{},
			Provenance: graph.InvocationProvenance{InvocationID: "chunk", SourceRevision: "test", ServerVersion: "fake"},
			Expansion:  graph.ExpansionConfig{TopmostSiblings: true},
		},
		Seeds:        []graph.SeedResult{},
		Summary:      graph.Summary{Complete: true},
		Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5WithSeedSpec(native, request.Session.SessionID, request.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, request.CanonicalSeedsV2)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func executionFixture(t *testing.T, chunkTargets int) (ExecutionRequest, *int) {
	t.Helper()
	if chunkTargets < 1 || chunkTargets > ChunkSize {
		t.Fatalf("invalid fixture chunk cardinality %d", chunkTargets)
	}
	globalCount := ChunkSize + chunkTargets
	input := fixture(t, globalCount)
	built, err := Build(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	projections := make([]Projection, len(built.Chunks))
	sidecars := make([]Sidecar, len(built.Chunks))
	for i := range built.Chunks {
		projections[i] = built.Chunks[i].Projection
		sidecars[i] = built.Chunks[i].Sidecar
	}
	calls := 0
	core := censusacquisition.Core{Acquirer: chunkAcquirerFunc(func(_ context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		calls++
		return censusacquisition.AcquiredV5{Session: request.Session, Raw: exactChunkV5(t, request)}, nil
	})}
	return ExecutionRequest{Input: input, Plan: built.Plan, Index: built.Index, Projections: projections, Sidecars: sidecars, ChunkOrdinal: len(built.Chunks) - 1, Core: core}, &calls
}

func TestExecuteChunkCardinalityBatchBoundaryAndExactlyOnce(t *testing.T) {
	for _, n := range []int{1, 63, 64, 10000} {
		t.Run(string(rune(n)), func(t *testing.T) {
			request, calls := executionFixture(t, n)
			receipt, err := ExecuteChunk(context.Background(), request)
			if err != nil {
				t.Fatalf("ASSERT_CHUNK_EXECUTION_ACCEPTED n=%d: %v", n, err)
			}
			wantCalls := (n + 62) / 63
			if *calls != wantCalls || receipt.Descriptor != request.Plan.Chunks[request.ChunkOrdinal] || receipt.PlanDigest != request.Plan.LogicalDigest || receipt.IndexDigest != request.Index.LogicalDigest {
				t.Fatalf("ASSERT_CORE_RUN_ONCE_WITH_BOUNDED_BATCHES n=%d calls=%d receipt=%+v", n, *calls, receipt)
			}
			if receipt.Authority != 0 || receipt.Accepted || receipt.Completeness != "UNKNOWN" {
				t.Fatalf("ASSERT_PRIVATE_CONSERVATIVE_RECEIPT: %+v", receipt)
			}
		})
	}
}

func TestExecuteChunkRejectsBeforeCore(t *testing.T) {
	cases := map[string]func(*ExecutionRequest){
		"batch_64": func(r *ExecutionRequest) {
			r.Plan.BatchTargets = 64
			r.Plan.LogicalDigest, r.Plan.ImmutableSelector = "", ""
		},
		"stale_session": func(r *ExecutionRequest) { r.Input.SessionID = "stale" },
		"local_ordinal": func(r *ExecutionRequest) {
			r.Projections[r.ChunkOrdinal].Targets[0].CensusOrdinal = 1
			r.Projections[r.ChunkOrdinal].LogicalDigest, r.Projections[r.ChunkOrdinal].ImmutableSelector = "", ""
		},
		"seed": func(r *ExecutionRequest) {
			r.Projections[r.ChunkOrdinal].Targets[0].CanonicalSeedV2SHA256 = digest([]byte("mutated"))
			r.Projections[r.ChunkOrdinal].LogicalDigest, r.Projections[r.ChunkOrdinal].ImmutableSelector = "", ""
		},
		"file_ledger": func(r *ExecutionRequest) {
			r.Projections[r.ChunkOrdinal].Files[0].Disposition = "failed"
			r.Projections[r.ChunkOrdinal].LogicalDigest, r.Projections[r.ChunkOrdinal].ImmutableSelector = "", ""
		},
		"descriptor_digest": func(r *ExecutionRequest) {
			r.Plan.Chunks[r.ChunkOrdinal].Projection.SHA256 = digest([]byte("mutated"))
			r.Plan.LogicalDigest, r.Plan.ImmutableSelector = "", ""
		},
		"cardinality_10001": func(r *ExecutionRequest) {
			chunk := r.ChunkOrdinal
			r.Projections[chunk].Targets = append(r.Projections[chunk].Targets, make([]LocalTarget, ChunkSize)...)
			r.Sidecars[chunk].Mappings = append(r.Sidecars[chunk].Mappings, make([]Mapping, ChunkSize)...)
			r.Plan.Chunks[chunk].TargetCount = ChunkSize + 1
			r.Plan.LogicalDigest, r.Plan.ImmutableSelector = "", ""
		},
		"sidecar": func(r *ExecutionRequest) {
			r.Sidecars[r.ChunkOrdinal].Mappings[0].LocalOrdinal = 1
			r.Sidecars[r.ChunkOrdinal].LogicalDigest, r.Sidecars[r.ChunkOrdinal].ImmutableSelector = "", ""
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			request, calls := executionFixture(t, 1)
			mutate(&request)
			if _, err := ExecuteChunk(context.Background(), request); err == nil || !IsExecutionStage(err, StagePrecondition) || *calls != 0 {
				t.Fatalf("ASSERT_FAIL_CLOSED_BEFORE_CORE err=%v calls=%d", err, *calls)
			}
		})
	}
}

func TestExecuteChunkPreservesCoreFailure(t *testing.T) {
	request, calls := executionFixture(t, 1)
	want := errors.New("core refusal sentinel")
	request.Core.Acquirer = chunkAcquirerFunc(func(context.Context, censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		*calls++
		return censusacquisition.AcquiredV5{}, want
	})
	if _, err := ExecuteChunk(context.Background(), request); !errors.Is(err, want) || !IsExecutionStage(err, StageCore) || *calls != 1 {
		t.Fatalf("ASSERT_CORE_FAILURE_PRESERVED err=%v calls=%d", err, *calls)
	}
}

func TestReconcileExecutionProjectionRejectsMutations(t *testing.T) {
	mutations := map[string]func(*censusacquisition.Projection){
		"manifest_bytes": func(p *censusacquisition.Projection) { p.ManifestBytes[0] ^= 1 },
		"reordered_result": func(p *censusacquisition.Projection) {
			p.Batches[0].Targets[0], p.Batches[0].Targets[1] = p.Batches[0].Targets[1], p.Batches[0].Targets[0]
		},
		"missing_result": func(p *censusacquisition.Projection) { p.Batches[0].Targets = p.Batches[0].Targets[1:] },
		"extra_result": func(p *censusacquisition.Projection) {
			p.Batches[0].Targets = append(p.Batches[0].Targets, p.Batches[0].Targets[0])
		},
		"symbol_ledger": func(p *censusacquisition.Projection) { p.Manifest.SymbolLedger.Entries[0].Disposition = "failed" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			request, _ := executionFixture(t, 64)
			receipt, err := ExecuteChunk(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			p, err := censusacquisition.CloneProjection(receipt.Projection)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&p)
			if _, err := reconcileExecutionProjection(request, p); err == nil || !IsExecutionStage(err, StageProjection) {
				t.Fatalf("ASSERT_PROJECTION_MAPPING_MUTATION_REJECTED: %v", err)
			}
		})
	}
	request, _ := executionFixture(t, 1)
	receipt, err := ExecuteChunk(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	mappingRequest := request
	mappingRequest.Sidecars = append([]Sidecar(nil), request.Sidecars...)
	mappingRequest.Sidecars[mappingRequest.ChunkOrdinal].Mappings = append([]Mapping(nil), request.Sidecars[request.ChunkOrdinal].Mappings...)
	mappingRequest.Sidecars[mappingRequest.ChunkOrdinal].Mappings[0].GlobalOrdinal++
	if _, err := reconcileExecutionProjection(mappingRequest, receipt.Projection); err == nil || !IsExecutionStage(err, StageMapping) {
		t.Fatalf("ASSERT_POST_CORE_GLOBAL_MAPPING_MUTATION_REJECTED: %v", err)
	}
	p := receipt.Projection
	p.ManifestBytes[0] ^= 1
	if _, err := censusacquisition.CloneProjection(p); err == nil {
		t.Fatal("ASSERT_UNCHANGED_CAPTURE_SET_VALIDATOR_DETECTS_MUTATION")
	}
}

func TestExecutionReceiptOwnsExactCopies(t *testing.T) {
	request, _ := executionFixture(t, 1)
	receipt, err := ExecuteChunk(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), receipt.ManifestBytes...)
	receipt.Projection.ManifestBytes[0] ^= 1
	if !reflect.DeepEqual(before, receipt.ManifestBytes) {
		t.Fatal("ASSERT_RECEIPT_BYTES_ALIAS_PROJECTION")
	}
}
