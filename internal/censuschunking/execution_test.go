package censuschunking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
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
	chunkOrdinal := 1
	if chunkTargets == ChunkSize {
		globalCount = ChunkSize + 1
		chunkOrdinal = 0
	}
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
	return ExecutionRequest{Input: input, Plan: built.Plan, Index: built.Index, Projections: projections, Sidecars: sidecars, ChunkOrdinal: chunkOrdinal, Core: core}, &calls
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
	baselineRequest, _ := executionFixture(t, 64)
	baselineReceipt, baselineErr := ExecuteChunk(context.Background(), baselineRequest)
	if baselineErr != nil {
		t.Fatal(baselineErr)
	}
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
			p, err := censusacquisition.CloneProjection(baselineReceipt.Projection)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&p)
			if _, err := reconcileExecutionProjection(baselineRequest, p); err == nil || !IsExecutionStage(err, StageProjection) {
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

func maliciousCallerMutation(request *ExecutionRequest) {
	request.Input.Files = request.Input.Files[:0]
	request.Input.Targets = request.Input.Targets[:0]
	request.Input.GlobalProviderFailures = append(request.Input.GlobalProviderFailures, "fabricated")
	request.Plan.Chunks = request.Plan.Chunks[:0]
	request.Index.Chunks = request.Index.Chunks[:0]
	for i := range request.Projections {
		request.Projections[i].Files = request.Projections[i].Files[:0]
		request.Projections[i].Symbols = request.Projections[i].Symbols[:0]
		request.Projections[i].Targets = request.Projections[i].Targets[:0]
		request.Projections[i].ProviderFailures = append(request.Projections[i].ProviderFailures, "fabricated")
		request.Sidecars[i].Mappings = request.Sidecars[i].Mappings[:0]
	}
	if len(request.Projections) > 1 {
		request.Projections[0], request.Projections[1] = request.Projections[1], request.Projections[0]
		request.Sidecars[0], request.Sidecars[1] = request.Sidecars[1], request.Sidecars[0]
	}
}

func TestExecuteChunkFreezesCallerAliasesDuringCore(t *testing.T) {
	request, calls := executionFixture(t, 64)
	request.Core.Acquirer = chunkAcquirerFunc(func(_ context.Context, batch censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		*calls++
		raw := exactChunkV5(t, batch)
		maliciousCallerMutation(&request)
		return censusacquisition.AcquiredV5{Session: batch.Session, Raw: raw}, nil
	})
	receipt, err := ExecuteChunk(context.Background(), request)
	if err != nil || receipt.TargetCount != 64 || *calls != 2 {
		t.Fatalf("ASSERT_FROZEN_CALLER_ALIASES err=%v calls=%d receipt=%+v", err, *calls, receipt)
	}
}

func TestExecuteChunkFreezesCallerAliasesConcurrentSynchronized(t *testing.T) {
	request, calls := executionFixture(t, 1)
	mutate := make(chan struct{})
	mutated := make(chan struct{})
	go func() {
		<-mutate
		maliciousCallerMutation(&request)
		close(mutated)
	}()
	request.Core.Acquirer = chunkAcquirerFunc(func(_ context.Context, batch censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		*calls++
		raw := exactChunkV5(t, batch)
		close(mutate)
		<-mutated
		return censusacquisition.AcquiredV5{Session: batch.Session, Raw: raw}, nil
	})
	receipt, err := ExecuteChunk(context.Background(), request)
	if err != nil || receipt.TargetCount != 1 || *calls != 1 {
		t.Fatalf("ASSERT_SYNCHRONIZED_FROZEN_CALLER_ALIASES err=%v calls=%d receipt=%+v", err, *calls, receipt)
	}
}

func TestChunkExecutionReceiptV1StrictCanonicalIdentity(t *testing.T) {
	request, _ := executionFixture(t, 1)
	receipt, err := ExecuteChunk(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeChunkExecution(receipt)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseChunkExecution(raw)
	if err != nil || ValidateChunkExecution(parsed, request.Plan, request.Index, request.Plan.Chunks[request.ChunkOrdinal], request.Projections[request.ChunkOrdinal], request.Sidecars[request.ChunkOrdinal], receipt.Projection) != nil {
		t.Fatalf("ASSERT_RECEIPT_V1_ROUND_TRIP err=%v parsed=%+v", err, parsed)
	}
	empty := receipt
	empty.ConstituentDigests = []string{}
	empty.LogicalDigest, empty.ImmutableSelector = "", ""
	emptyRaw, err := EncodeChunkExecution(empty)
	if err != nil || !bytes.Contains(emptyRaw, []byte(`"constituent_digests":[]`)) || bytes.Contains(emptyRaw, []byte(`"constituent_digests":null`)) {
		t.Fatalf("ASSERT_RECEIPT_COLLECTION_ARRAY err=%v raw=%s", err, emptyRaw)
	}
	malformed := map[string][]byte{
		"duplicate":    []byte(strings.Replace(string(raw), `"schema_version":`, `"schema_version":"`+ChunkExecutionVersion+`","schema_version":`, 1)),
		"unknown":      []byte(strings.Replace(string(raw), `{`, `{"unknown":0,`, 1)),
		"trailing":     append(append([]byte(nil), raw...), []byte(` {}`)...),
		"reordered":    append([]byte(" "), raw...),
		"stale_digest": []byte(strings.Replace(string(raw), receipt.PlanDigest, digest([]byte("stale")), 1)),
	}
	for name, candidate := range malformed {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseChunkExecution(candidate); err == nil {
				t.Fatalf("ASSERT_RECEIPT_NONCANONICAL_REJECTED raw=%s", candidate)
			}
		})
	}
	mutations := map[string]func(*ChunkExecution){
		"schema":              func(v *ChunkExecution) { v.SchemaVersion = "unknown" },
		"plan":                func(v *ChunkExecution) { v.PlanDigest = digest([]byte("mutated")) },
		"index":               func(v *ChunkExecution) { v.IndexDigest = digest([]byte("mutated")) },
		"ordinal":             func(v *ChunkExecution) { v.ChunkOrdinal++ },
		"range":               func(v *ChunkExecution) { v.TargetStart++ },
		"count":               func(v *ChunkExecution) { v.TargetCount++ },
		"projection_identity": func(v *ChunkExecution) { v.ProjectionIdentity = digest([]byte("mutated")) },
		"projection_digest":   func(v *ChunkExecution) { v.ProjectionDigest = digest([]byte("mutated")) },
		"sidecar_identity":    func(v *ChunkExecution) { v.SidecarIdentity = digest([]byte("mutated")) },
		"sidecar_digest":      func(v *ChunkExecution) { v.SidecarDigest = digest([]byte("mutated")) },
		"capture_set":         func(v *ChunkExecution) { v.CaptureSetIdentity = "mutated" },
		"capture_set_digest":  func(v *ChunkExecution) { v.CaptureSetDigest = digest([]byte("mutated")) },
		"manifest":            func(v *ChunkExecution) { v.ManifestDigest = digest([]byte("mutated")) },
		"manifest_selector":   func(v *ChunkExecution) { v.ManifestSelector = "mutated" },
		"manifest_bytes":      func(v *ChunkExecution) { v.ManifestBytesDigest = digest([]byte("mutated")) },
		"constituents": func(v *ChunkExecution) {
			v.ConstituentDigests = append(v.ConstituentDigests, digest([]byte("mutated")))
		},
		"authority":    func(v *ChunkExecution) { v.Authority = 1 },
		"accepted":     func(v *ChunkExecution) { v.Accepted = true },
		"completeness": func(v *ChunkExecution) { v.Completeness = "COMPLETE" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := receipt
			candidate.ConstituentDigests = append([]string(nil), receipt.ConstituentDigests...)
			mutate(&candidate)
			candidate.LogicalDigest, candidate.ImmutableSelector = "", ""
			setChunkExecutionIdentity(&candidate)
			if ValidateChunkExecution(candidate, request.Plan, request.Index, request.Plan.Chunks[request.ChunkOrdinal], request.Projections[request.ChunkOrdinal], request.Sidecars[request.ChunkOrdinal], receipt.Projection) == nil {
				t.Fatalf("ASSERT_RECEIPT_SEMANTIC_MUTATION_REJECTED candidate=%+v", candidate)
			}
		})
	}
}
