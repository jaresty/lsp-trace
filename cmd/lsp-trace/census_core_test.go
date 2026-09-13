package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/publication"
)

func discoveryFromProjection(p censusacquisition.Projection) censusacquisition.Discovery {
	d := censusacquisition.Discovery{Session: p.Session, Workspace: p.Workspace, Complete: true, FileLedger: p.Manifest.FileLedger, SymbolLedger: p.Manifest.SymbolLedger}
	d.Accounting.FileDenominator = p.Manifest.FileLedger.Denominator
	d.Accounting.SymbolDenominator = p.Manifest.SymbolLedger.Denominator
	for _, e := range p.Manifest.FileLedger.Entries {
		d.Accounting.Files = append(d.Accounting.Files, censusFileEntry(e.Ordinal, e.Disposition))
	}
	for _, e := range p.Manifest.SymbolLedger.Entries {
		d.Accounting.Symbols = append(d.Accounting.Symbols, censusSymbolEntry(e.Ordinal, e.Disposition))
	}
	for _, b := range p.Batches {
		d.Targets = append(d.Targets, b.Targets...)
	}
	return d
}

func successfulCorePublication(t *testing.T, capability censusPublicationCapability) censusPublicationOutcome {
	t.Helper()
	result, err := buildCensusCLIResult(capability.state.projection, publication.BoundFileReceipt{
		FinalSelector: "captureset:sha256:" + repeatHex('a'), Digest: "sha256:" + repeatHex('b'), ByteLength: 1,
		VerificationStatus: "VERIFIED", DirectorySyncStatus: publication.DirectorySyncComplete, CloseStatus: publication.CloseComplete,
	})
	if err != nil {
		t.Fatal(err)
	}
	return censusPublicationOutcome{Result: &result}
}

func repeatHex(b byte) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = b
	}
	return string(out)
}

// Local adapters avoid importing census internals merely to reconstruct ledger dispositions.
func censusFileEntry(ordinal int, disposition string) census.FileEntry {
	return census.FileEntry{Ordinal: ordinal, Disposition: census.FileDisposition(disposition)}
}
func censusSymbolEntry(ordinal int, disposition string) census.SymbolEntry {
	d := census.SymbolDisposition(disposition)
	if disposition == censusacquisition.SymbolPrepared {
		d = census.SymbolSelected
	}
	return census.SymbolEntry{Ordinal: ordinal, Disposition: d}
}

func TestRunCensusCoreCardinalityLifecycleAndDeterminism(t *testing.T) {
	for _, n := range []int{0, 1, 63, 64, 127} {
		t.Run(string(rune('A'+n%26)), func(t *testing.T) {
			var projection censusacquisition.Projection
			if n > 0 {
				projection = censusPublicationProjection(t, n)
			}
			discovery := censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: "publication-session", Generation: 7}, Complete: true}
			if n > 0 {
				discovery = discoveryFromProjection(projection)
			}
			initialized, callbacks, shutdown, publishes := 0, 0, 0, 0
			var acquired censusacquisition.Discovery
			deps := censusCoreDependencies{
				runSession: func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
					initialized++
					callbacks++
					defer func() { shutdown++ }()
					return cb(context.Background(), &initializedAcquisitionRuntime{sessionID: "publication-session", generation: 7})
				},
				discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
					return discovery, nil
				},
				acquire: func(ctx context.Context, _ *initializedAcquisitionRuntime, d censusacquisition.Discoverer, _ acquisitionops.Limits) (censusAssembly, error) {
					acquired, _ = d.Discover(ctx, discovery.Session)
					cap := censusPublicationCapabilityFor(t, projection)
					return censusAssembly{state: cap.state, token: cap.token}, nil
				},
				capability: func(a censusAssembly) (censusPublicationCapability, error) { return a.publicationCapability() },
				publish: func(_ context.Context, c censusPublicationCapability, _ string) censusPublicationOutcome {
					publishes++
					return successfulCorePublication(t, c)
				},
			}
			first := runCensusCore(censusCLIOptions{PublicationRoot: "private"}, censusCoreConfig{}, deps)
			if initialized != 1 || callbacks != 1 || shutdown != 1 {
				t.Fatalf("ASSERT_ONE_LIFECYCLE init=%d callback=%d shutdown=%d", initialized, callbacks, shutdown)
			}
			if n == 0 {
				if first.Result != nil || first.Diagnostic == nil || first.Diagnostic.Stage != censusStageDiscovery || publishes != 0 {
					t.Fatalf("ASSERT_EMPTY_FAILS_BEFORE_PUBLISH %+v calls=%d", first, publishes)
				}
				return
			}
			if first.Result == nil || first.Diagnostic != nil || publishes != 1 || len(acquired.Targets) != n {
				t.Fatalf("ASSERT_CORE_SUCCESS %+v calls=%d targets=%d", first, publishes, len(acquired.Targets))
			}
			wantBatches := (n + 62) / 63
			if len(projection.Batches) != wantBatches {
				t.Fatalf("ASSERT_BATCH_COUNT got=%d want=%d", len(projection.Batches), wantBatches)
			}
			ordinal := 0
			for i, batch := range projection.Batches {
				if batch.Ordinal != i || batch.DownDepth != 1 || batch.UpDepth != 0 || len(batch.Targets) > 63 {
					t.Fatal("ASSERT_BATCH_ORDER_DEPTH_BOUND")
				}
				for _, target := range batch.Targets {
					if target.CensusOrdinal != ordinal {
						t.Fatal("ASSERT_TARGET_ORDER")
					}
					ordinal++
				}
			}
			secondProjection := censusPublicationProjection(t, n)
			if !reflect.DeepEqual(projection, secondProjection) || first.Result.CensusID != secondProjection.CensusID {
				t.Fatal("ASSERT_DETERMINISTIC_RESULT")
			}
		})
	}
}

func TestRunCensusCoreFailBeforePublishMatrix(t *testing.T) {
	base := censusPublicationProjection(t, 1)
	for _, tc := range []struct {
		name          string
		discovery     func() (censusacquisition.Discovery, error)
		acquireErr    error
		capabilityErr error
	}{
		{"discovery", func() (censusacquisition.Discovery, error) {
			return censusacquisition.Discovery{}, errors.New("/private/provider")
		}, nil, nil},
		{"incomplete", func() (censusacquisition.Discovery, error) {
			d := discoveryFromProjection(base)
			d.Complete = false
			return d, nil
		}, nil, nil},
		{"prepare", func() (censusacquisition.Discovery, error) { return discoveryFromProjection(base), nil }, errors.New("prepare /private/path"), nil},
		{"late-batch", func() (censusacquisition.Discovery, error) { return discoveryFromProjection(base), nil }, errors.New("late batch provider output"), nil},
		{"assembly", func() (censusacquisition.Discovery, error) { return discoveryFromProjection(base), nil }, nil, errors.New("assembly mutation")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			published := 0
			deps := censusCoreDependencies{
				runSession: func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
					return cb(context.Background(), &initializedAcquisitionRuntime{sessionID: "publication-session", generation: 7})
				},
				discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
					return tc.discovery()
				},
				acquire: func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits) (censusAssembly, error) {
					if tc.acquireErr != nil {
						return censusAssembly{}, tc.acquireErr
					}
					c := censusPublicationCapabilityFor(t, base)
					return censusAssembly{state: c.state, token: c.token}, nil
				},
				capability: func(a censusAssembly) (censusPublicationCapability, error) {
					if tc.capabilityErr != nil {
						return censusPublicationCapability{}, tc.capabilityErr
					}
					return a.publicationCapability()
				},
				publish: func(context.Context, censusPublicationCapability, string) censusPublicationOutcome {
					published++
					return censusPublicationOutcome{}
				},
			}
			got := runCensusCore(censusCLIOptions{}, censusCoreConfig{}, deps)
			if got.Result != nil || got.Diagnostic == nil || published != 0 {
				t.Fatalf("ASSERT_FAIL_BEFORE_PUBLISH %+v calls=%d", got, published)
			}
		})
	}
}

func TestRunCensusCoreCancellationDriftAndPublicationOutcomes(t *testing.T) {
	projection := censusPublicationProjection(t, 1)
	for _, tc := range []struct {
		name        string
		context     func() context.Context
		generation  uint64
		publication censusPublicationOutcome
		wantSuccess bool
	}{
		{"cancelled", func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }, 7, censusPublicationOutcome{}, false},
		{"drift", context.Background, 8, censusPublicationOutcome{}, false},
		{"publisher-failure", context.Background, 7, censusPublicationOutcome{Diagnostic: mustDiagnostic(t, censusStagePublication)}, false},
		{"competitor", context.Background, 7, censusPublicationOutcome{Diagnostic: mustDiagnostic(t, censusStagePublication)}, false},
		{"committed-degraded", context.Background, 7, successfulCorePublication(t, censusPublicationCapabilityFor(t, projection)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publishes := 0
			deps := censusCoreDependencies{
				runSession: func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
					return cb(tc.context(), &initializedAcquisitionRuntime{sessionID: "publication-session", generation: tc.generation})
				},
				discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
					return discoveryFromProjection(projection), nil
				},
				acquire: func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits) (censusAssembly, error) {
					c := censusPublicationCapabilityFor(t, projection)
					return censusAssembly{state: c.state, token: c.token}, nil
				},
				capability: func(a censusAssembly) (censusPublicationCapability, error) { return a.publicationCapability() },
				publish: func(context.Context, censusPublicationCapability, string) censusPublicationOutcome {
					publishes++
					out := tc.publication
					if tc.name == "committed-degraded" {
						d := mustDiagnostic(t, censusStageCommitted)
						out.Diagnostic = d
					}
					return out
				},
			}
			got := runCensusCore(censusCLIOptions{}, censusCoreConfig{}, deps)
			if tc.wantSuccess != (got.Result != nil) {
				t.Fatalf("ASSERT_PUBLICATION_SEMANTICS %+v", got)
			}
			if !tc.wantSuccess && (tc.name == "cancelled" || tc.name == "drift") && publishes != 0 {
				t.Fatal("ASSERT_PRECOMMIT_ZERO_PUBLISH")
			}
		})
	}
}

func mustDiagnostic(t *testing.T, stage censusFailureStage) *censusCLIDiagnostic {
	t.Helper()
	d, err := buildCensusCLIDiagnostic(stage, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &d
}
