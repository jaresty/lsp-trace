package censuscontinuation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/liveprojection"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/internal/v5sourcesnapshotv6"
	"lsp-trace/sessionruntime"
)

type managedPrepareFunc func(context.Context, string, string, uint64, string, []string, ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error)

func (f managedPrepareFunc) PrepareManagedDocuments(ctx context.Context, workspace, sessionID string, generation uint64, encoding string, uris []string, limits ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error) {
	return f(ctx, workspace, sessionID, generation, encoding, uris, limits)
}

type documentPrepareFunc func(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult

func (f documentPrepareFunc) PrepareDocument(ctx context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	return f(ctx, request)
}

type workspaceRootFunc func(string, uint64) (string, bool)

func (f workspaceRootFunc) WorkspaceRoot(sessionID string, generation uint64) (string, bool) {
	return f(sessionID, generation)
}

type managedCaptureWriter struct{ bytes.Buffer }

func (*managedCaptureWriter) Close() error { return nil }

type managedCaptureChild struct{ writer *managedCaptureWriter }

func (c managedCaptureChild) Stdin() io.WriteCloser { return c.writer }
func (managedCaptureChild) Stdout() io.ReadCloser   { return io.NopCloser(bytes.NewReader(nil)) }
func (managedCaptureChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (managedCaptureChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type managedCaptureStarter struct{ child sessionruntime.Child }

func (s managedCaptureStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func managedCaptureFixture(t *testing.T) (fixture, CommittedHandoff) {
	t.Helper()
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	return f, h
}

func partitionHandoff(t *testing.T, workspace string, f partitionFixture) CommittedHandoff {
	t.Helper()
	committed, err := censusresult.Build(f.projection, censusresult.PublicationEvidence{Selector: f.publication.Receipt.Selector, Digest: f.publication.Receipt.ArtifactSHA256, ByteLength: f.publication.Receipt.ByteLength, VerificationStatus: f.publication.Receipt.VerificationStatus, DirectorySyncStatus: censusresult.DirectorySyncComplete, CloseStatus: censusresult.CloseComplete})
	if err != nil {
		t.Fatal(err)
	}
	workspaceURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspace)}).String()
	h, err := BuildHandoff(BuildInput{Result: committed, Projection: f.projection, Publication: f.publication, Metadata: partitionMetadata(), Workspace: WorkspaceIdentity{URI: workspaceURI, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func partitionCaptureFixture(t *testing.T, reverse bool) (string, partitionFixture, CommittedHandoff) {
	t.Helper()
	const targetCount = 23
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "a.go"), partitionSource(targetCount), 0o600); err != nil {
		t.Fatal(err)
	}
	f := buildPartitionFixture(t, workspace, targetCount, 16, reverse, reverse)
	return workspace, f, partitionHandoff(t, workspace, f)
}

func successfulManagedDependencies(t *testing.T, workspace string, prepareCalls, resolverCalls *int, failAfterFirst bool) FreshCaptureDependencies {
	t.Helper()
	preparer := managedPrepareFunc(func(_ context.Context, _ string, sessionID string, generation uint64, encoding string, uris []string, _ ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error) {
		*prepareCalls++
		if failAfterFirst && *prepareCalls > 1 {
			return nil, errors.New("second constituent preparation unavailable")
		}
		out := make([]v5sourcesnapshotv6.PreparedDocument, 0, len(uris))
		for _, uri := range uris {
			u, err := url.Parse(uri)
			if err != nil {
				return nil, err
			}
			raw, err := os.ReadFile(u.Path)
			if err != nil {
				return nil, err
			}
			out = append(out, v5sourcesnapshotv6.PreparedDocument{URI: uri, Bytes: raw, Digest: digestBytes(raw), ByteLength: uint64(len(raw)), Version: "fixture-v1", SessionID: sessionID, Generation: generation, PositionEncoding: encoding})
		}
		return out, nil
	})
	resolver := v5sourcesnapshotv6.ResolverFunc(func(_ context.Context, request v5sourcesnapshotv6.ResolveRequest) (v5sourcesnapshotv6.ResolveResult, error) {
		*resolverCalls++
		return v5sourcesnapshotv6.ResolveResult{DisplayRange: request.ItemRange, ItemRange: request.ItemRange, SelectionRange: request.SelectionRange, ProvenanceKind: v5sourcesnapshotv6.ProvenanceKind, Method: v5sourcesnapshotv6.ProvenanceMethod, DocumentDigest: request.DocumentDigest, DocumentVersion: request.DocumentVersion, DocumentByteLength: uint64(len(request.Bytes))}, nil
	})
	return FreshCaptureDependencies{Context: context.Background(), Preparer: preparer, Resolver: resolver, Limits: ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 100, MaxWork: 1000, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20}}
}

func TestFreshMixedScopeManagedDocuments(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "a.go"), partitionSource(17), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideRoot := t.TempDir()
	outsideURIs := map[string]bool{}
	f := buildPartitionFixtureWithURISelector(t, workspace, 17, 16, false, false, func(item graph.Item) string {
		var ordinal int
		if _, err := fmt.Sscanf(item.Name, "T%d", &ordinal); err == nil && ordinal > 0 && ordinal%3 == 0 {
			outsideURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(outsideRoot, "outside-1.go"))}).String()
			outsideURIs[outsideURI] = true
			return outsideURI
		}
		return item.URI
	})
	h := partitionHandoff(t, workspace, f)

	uriByNode := map[string]string{}
	for _, constituent := range f.projection.Constituents {
		native, _, err := decodeNative(constituent.Raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, node := range native.Nodes {
			uriByNode[node.ID] = node.URI
		}
	}
	eligible, outside := 0, 0
	for _, nomination := range f.result.Representatives.Nominations {
		uri := uriByNode[nomination.SelectedNode]
		if outsideURIs[uri] {
			outside++
		} else if eligibleFreshManagedDocumentURI(uri, workspace) {
			eligible++
		}
	}
	if eligible == 0 || outside == 0 {
		t.Fatalf("ASSERT_OUTSIDE_WORKSPACE_ENDPOINT_RETAINED: nominations eligible=%d outside=%d", eligible, outside)
	}

	var preparedBatches [][]string
	resolverCalls := []v5sourcesnapshotv6.ResolveRequest{}
	resolverStarted := false
	preparer := managedPrepareFunc(func(_ context.Context, workspaceArg, sessionID string, generation uint64, encoding string, uris []string, _ ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error) {
		if workspaceArg != workspace || sessionID != f.projection.Session.SessionID || generation != f.projection.Session.Generation || encoding != h.PositionEncoding() {
			t.Fatalf("ASSERT_ELIGIBLE_EXACT_CUSTODY: workspace=%q session=%q generation=%d encoding=%q", workspaceArg, sessionID, generation, encoding)
		}
		if resolverStarted {
			t.Fatal("ASSERT_ELIGIBLE_PREPARATION_BEFORE_RESOLVER: resolver started before preparation completed")
		}
		preparedBatches = append(preparedBatches, append([]string(nil), uris...))
		for _, uri := range uris {
			if outsideURIs[uri] || !eligibleFreshManagedDocumentURI(uri, workspace) {
				t.Fatalf("ASSERT_NO_FALLBACK_OR_URI_LEAKAGE: ineligible URI prepared: %q", uri)
			}
		}
		documents := make([]v5sourcesnapshotv6.PreparedDocument, 0, len(uris))
		for _, uri := range uris {
			u, err := url.Parse(uri)
			if err != nil {
				t.Fatalf("ASSERT_ELIGIBLE_EXACT_CUSTODY: parse URI %q: %v", uri, err)
			}
			content, err := os.ReadFile(u.Path)
			if err != nil {
				t.Fatalf("ASSERT_ELIGIBLE_EXACT_CUSTODY: read fixture %q: %v", uri, err)
			}
			documents = append(documents, v5sourcesnapshotv6.PreparedDocument{URI: uri, Bytes: content, Digest: digestBytes(content), ByteLength: uint64(len(content)), Version: "fixture-v1", SessionID: sessionID, Generation: generation, PositionEncoding: encoding})
		}
		return documents, nil
	})
	resolver := v5sourcesnapshotv6.ResolverFunc(func(_ context.Context, request v5sourcesnapshotv6.ResolveRequest) (v5sourcesnapshotv6.ResolveResult, error) {
		resolverStarted = true
		resolverCalls = append(resolverCalls, request)
		if outsideURIs[request.URI] {
			t.Fatal("ASSERT_OUTSIDE_WORKSPACE_ZERO_RESOLVER: outside endpoint reached resolver")
		}
		return v5sourcesnapshotv6.ResolveResult{DisplayRange: request.ItemRange, ItemRange: request.ItemRange, SelectionRange: request.SelectionRange, ProvenanceKind: v5sourcesnapshotv6.ProvenanceKind, Method: v5sourcesnapshotv6.ProvenanceMethod, DocumentDigest: request.DocumentDigest, DocumentVersion: request.DocumentVersion, DocumentByteLength: uint64(len(request.Bytes))}, nil
	})
	deps := FreshCaptureDependencies{Context: context.Background(), Preparer: preparer, Resolver: resolver, Limits: ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 100, MaxWork: 1000, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20}}
	captured, err := CaptureSnapshots(h, f.result, workspace, h.PositionEncoding(), testContract(t).CaptureLimits(), deps)
	if err != nil {
		t.Fatalf("ASSERT_OUTSIDE_WORKSPACE_SOURCE_UNAVAILABLE: capture failed: %v", err)
	}
	if len(preparedBatches) == 0 || len(resolverCalls) == 0 {
		t.Fatalf("ASSERT_ELIGIBLE_PREPARATION_BEFORE_RESOLVER: batches=%d resolver_calls=%d", len(preparedBatches), len(resolverCalls))
	}

	seenOutside, seenEligible := 0, 0
	for _, constituent := range captured.Constituents {
		for _, outcome := range constituent.EndpointOutcomes {
			uri := uriByNode[outcome.Endpoint.GraphSubjectID]
			if outsideURIs[uri] {
				seenOutside++
				if outcome.Status != EndpointStatusSourceUnavailable || outcome.Code != SourceCodeExactEndpointUnavailable {
					t.Fatalf("ASSERT_OUTSIDE_WORKSPACE_SOURCE_UNAVAILABLE: outcome=%+v", outcome)
				}
			} else {
				if !eligibleFreshManagedDocumentURI(uri, workspace) {
					t.Fatalf("ASSERT_NO_FALLBACK_OR_URI_LEAKAGE: unexpected URI=%q", uri)
				}
				seenEligible++
				if outcome.Status != EndpointStatusCaptured {
					t.Fatalf("ASSERT_ELIGIBLE_EXACT_CUSTODY: outcome=%+v", outcome)
				}
			}
		}
	}
	if seenOutside != outside || seenEligible != eligible {
		t.Fatalf("ASSERT_OUTSIDE_WORKSPACE_ENDPOINT_RETAINED: final outside=%d/%d eligible=%d/%d", seenOutside, outside, seenEligible, eligible)
	}
}

func TestCaptureFreshNilDependenciesFailsTypedSourcePreparation(t *testing.T) {
	f, h := managedCaptureFixture(t)
	_, err := captureFresh(h, f.direct, f.workspace, h.PositionEncoding(), testContract(t).CaptureLimits(), nil)
	if err == nil {
		t.Fatal("ASSERT_NIL_MANAGED_DEPENDENCIES_SOURCE_PREPARATION: capture unexpectedly succeeded")
	}
	diagnostic := classifyCaptureFailure(err)
	if diagnostic.Category != "availability" || diagnostic.FailedField != "source_preparation" || diagnostic.CallerAction != "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME" {
		t.Fatalf("ASSERT_NIL_MANAGED_DEPENDENCIES_SOURCE_PREPARATION: diagnostic=%+v err=%v", diagnostic, err)
	}
}

func TestManagedPreparationCompletesGloballyBeforeResolver(t *testing.T) {
	workspace, f, h := partitionCaptureFixture(t, false)
	prepareCalls, resolverCalls := 0, 0
	deps := successfulManagedDependencies(t, workspace, &prepareCalls, &resolverCalls, true)
	_, err := CaptureSnapshots(h, f.result, workspace, h.PositionEncoding(), testContract(t).CaptureLimits(), deps)
	if err != nil {
		t.Fatalf("ASSERT_GLOBAL_MANAGED_PREPARATION_BEFORE_RESOLVER: err=%v prepare_calls=%d resolver_calls=%d", err, prepareCalls, resolverCalls)
	}
	if prepareCalls != 1 || resolverCalls == 0 {
		t.Fatalf("ASSERT_GLOBAL_MANAGED_PREPARATION_BEFORE_RESOLVER: prepare_calls=%d resolver_calls=%d", prepareCalls, resolverCalls)
	}
}

func TestManagedPreparationTaxonomyKeepsCustodyAndMalformedDistinct(t *testing.T) {
	for _, kind := range []ManagedPreparationErrorKind{ManagedPreparationCustody, ManagedPreparationMalformed} {
		diagnostic := classifyCaptureFailure(managedPreparationError(kind, "injected managed supply failure"))
		if diagnostic.Category == "availability" || diagnostic.FailedField == "source_preparation" || diagnostic.CallerAction == "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME" {
			t.Fatalf("ASSERT_MANAGED_PREPARATION_DISTINCT_FAIL_CLOSED_%s: diagnostic=%+v", kind, diagnostic)
		}
	}
}

func TestManagedPreparationFailureDiagnosticPreservesSafeDimensionAndCounts(t *testing.T) {
	records := []ManagedPreparationDiagnostic{}
	preparer := ManagedPreparer{
		Preparer: documentPrepareFunc(func(_ context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
			if strings.HasSuffix(request.URI, "b.go") {
				return sessionruntime.DocumentResult{Failure: session.StaleGeneration}
			}
			return sessionruntime.DocumentResult{URI: request.URI, Supply: &sessionruntime.DocumentSupply{URI: request.URI, SessionID: request.SessionID, Generation: request.Generation, Content: []byte("package p\n")}}
		}),
		Workspace: workspaceRootFunc(func(string, uint64) (string, bool) { return "/workspace", true }),
		Recorder: ManagedPreparationDiagnosticRecorderFunc(func(record ManagedPreparationDiagnostic) error {
			records = append(records, record)
			return errors.New("secondary recorder failure")
		}),
	}
	_, err := preparer.PrepareManagedDocuments(context.Background(), "/workspace", "s", 1, "utf-16", []string{"file:///workspace/a.go", "file:///private/b.go", "file:///workspace/later.go"}, ManagedPreparationLimits{MaxDocuments: 8, MaxMessages: 8, MaxWork: 8, MaxDocumentBytes: 4096, MaxTotalBytes: 4096})
	var managed *ManagedPreparationError
	if !errors.As(err, &managed) || managed.Kind != ManagedPreparationAvailability || managed.Diagnostic.Failure != ManagedDocumentPreparationStaleGeneration || managed.Diagnostic.Attempted != 2 || managed.Diagnostic.Succeeded != 1 || managed.Diagnostic.Planned != 3 || managed.Diagnostic.FailingOrdinal != 1 || len(records) != 1 || records[0] != managed.Diagnostic {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_PRIVATE_DIAGNOSTIC: err=%v managed=%+v records=%+v", err, managed, records)
	}
	if err.Error() != "managed source preparation failed: managed document preparation unavailable" || strings.Contains(fmt.Sprintf("%+v", managed.Diagnostic), "/private/") {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_PUBLIC_GENERIC_PRIVACY: err=%v diagnostic=%+v", err, managed.Diagnostic)
	}
	var limit privateResourceLimit
	if errors.As(err, &limit) {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_AVAILABILITY_NOT_LIMIT: %T", limit)
	}
}

func TestManagedPreparationReproducesInstrumentedCensusLanguageIDDiagnostic(t *testing.T) {
	workspace := t.TempDir()
	writer := &managedCaptureWriter{}
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64},
		Starter: managedCaptureStarter{child: managedCaptureChild{writer: writer}},
	})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go"})
	if started.Failure != "" {
		t.Fatal(started)
	}
	if ready := manager.ObserveInitialization(started.SessionID, started.Generation, true); ready.Failure != "" {
		t.Fatal(ready)
	}

	uris := make([]string, 17)
	for i := range uris {
		path := filepath.Join(workspace, fmt.Sprintf("document-%02d", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("package fixture%d\n", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		uris[i] = (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	}
	for i := 0; i < 11; i++ {
		prior := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uris[i], LanguageID: "go", CaptureSupply: false})
		if prior.Failure != "" || prior.Supply != nil {
			t.Fatalf("fixture prior preparation %d: %+v", i, prior)
		}
	}

	limits := ManagedPreparationLimits{MaxDocuments: 17, MaxMessages: 17, MaxWork: 17, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20}
	configured := ManagedPreparer{Preparer: manager, Workspace: manager, LanguageResolver: manager}
	documents, err := configured.PrepareManagedDocuments(context.Background(), workspace, started.SessionID, started.Generation, "utf-16", uris, limits)
	if err != nil || len(documents) != 17 {
		t.Fatalf("ASSERT_INSTRUMENTED_CENSUS_CONFIGURED_LANGUAGE_PREPARES_ALL: documents=%d err=%v", len(documents), err)
	}

	prepareCalls := 0
	records := []ManagedPreparationDiagnostic{}
	missing := ManagedPreparer{
		Preparer: documentPrepareFunc(func(_ context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
			prepareCalls++
			if prepareCalls == 12 {
				return sessionruntime.DocumentResult{Failure: sessionruntime.LanguageIDUnavailable}
			}
			return sessionruntime.DocumentResult{URI: request.URI, LanguageID: "go", Version: 1, Supply: &sessionruntime.DocumentSupply{URI: request.URI, SessionID: request.SessionID, Generation: request.Generation, DocumentVersion: 1, Content: []byte("package fixture\n")}}
		}),
		Workspace: workspaceRootFunc(func(sessionID string, generation uint64) (string, bool) {
			return workspace, sessionID == started.SessionID && generation == started.Generation
		}),
		Recorder: ManagedPreparationDiagnosticRecorderFunc(func(record ManagedPreparationDiagnostic) error {
			records = append(records, record)
			return nil
		}),
	}
	failedDocuments, failedErr := missing.PrepareManagedDocuments(context.Background(), workspace, started.SessionID, started.Generation, "utf-16", uris, limits)
	want := ManagedPreparationDiagnostic{Failure: ManagedDocumentPreparationLanguageIDUnavailable, Attempted: 12, Succeeded: 11, Planned: 17, FailingOrdinal: 11}
	var managed *ManagedPreparationError
	public := classifyCaptureFailure(failedErr)
	if len(failedDocuments) != 0 || !errors.As(failedErr, &managed) || managed.Kind != ManagedPreparationAvailability || managed.Diagnostic != want || len(records) != 1 || records[0] != want || prepareCalls != 12 {
		t.Fatalf("ASSERT_INSTRUMENTED_CENSUS_LANGUAGE_ID_DIAGNOSTIC: documents=%d managed=%+v records=%+v prepare_calls=%d err=%v", len(failedDocuments), managed, records, prepareCalls, failedErr)
	}
	if public.Category != "availability" || public.FailedField != "source_preparation" || public.CallerAction != "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME" || failedErr.Error() != "managed source preparation failed: managed document preparation unavailable" {
		t.Fatalf("ASSERT_INSTRUMENTED_CENSUS_PUBLIC_AVAILABILITY: diagnostic=%+v err=%v", public, failedErr)
	}
}

func TestManagedPreparationExplicitLanguageIDIsNotMisclassifiedByURIAdmission(t *testing.T) {
	workspace := t.TempDir()
	writer := &managedCaptureWriter{}
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64},
		Starter: managedCaptureStarter{child: managedCaptureChild{writer: writer}},
	})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go"})
	if started.Failure != "" {
		t.Fatal(started)
	}
	if ready := manager.ObserveInitialization(started.SessionID, started.Generation, true); ready.Failure != "" {
		t.Fatal(ready)
	}

	uris := make([]string, 0, 4)
	for _, name := range []string{"a.go", "b.go"} {
		path := filepath.Join(workspace, name)
		if err := os.WriteFile(path, []byte("package fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		uris = append(uris, (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String())
	}
	outside := filepath.Join(filepath.Dir(workspace), "outside.go")
	uris = append(uris,
		(&url.URL{Scheme: "file", Path: filepath.ToSlash(outside)}).String(),
		(&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(workspace, "later.go"))}).String(),
	)

	records := []ManagedPreparationDiagnostic{}
	preparer := ManagedPreparer{
		Preparer:   manager,
		Workspace:  manager,
		LanguageID: "go",
		Recorder: ManagedPreparationDiagnosticRecorderFunc(func(record ManagedPreparationDiagnostic) error {
			records = append(records, record)
			return nil
		}),
	}
	limits := ManagedPreparationLimits{MaxDocuments: 4, MaxMessages: 4, MaxWork: 4, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 4 << 20}
	documents, prepareErr := preparer.PrepareManagedDocuments(context.Background(), workspace, started.SessionID, started.Generation, "utf-16", uris, limits)
	resolverCalls := 0
	if prepareErr == nil {
		for range documents {
			resolverCalls++
		}
	}

	var managed *ManagedPreparationError
	if !errors.As(prepareErr, &managed) {
		t.Fatalf("fixture expected managed preparation failure: documents=%d err=%v", len(documents), prepareErr)
	}
	if managed.Diagnostic.Failure != ManagedDocumentPreparationOutsideWorkspace {
		t.Fatalf("ASSERT_EXPLICIT_LANGUAGE_ID_NOT_MISCLASSIFIED: language_id=go diagnostic=%+v err=%v", managed.Diagnostic, prepareErr)
	}
	if len(documents) != 0 || resolverCalls != 0 || managed.Diagnostic.Attempted != 3 || managed.Diagnostic.Succeeded != 2 || managed.Diagnostic.Planned != 4 || managed.Diagnostic.FailingOrdinal != 2 || len(records) != 1 || records[0] != managed.Diagnostic {
		t.Fatalf("ASSERT_EXPLICIT_LANGUAGE_ID_PREPARATION_BARRIER: documents=%d resolver_calls=%d diagnostic=%+v records=%+v", len(documents), resolverCalls, managed.Diagnostic, records)
	}
	public := classifyCaptureFailure(prepareErr)
	if public.Category != "availability" || public.FailedField != "source_preparation" || public.CallerAction != "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME" || prepareErr.Error() != "managed source preparation failed: managed document preparation unavailable" {
		t.Fatalf("ASSERT_EXPLICIT_LANGUAGE_ID_PUBLIC_AVAILABILITY: diagnostic=%+v err=%v", public, prepareErr)
	}
}

func TestManagedPreparationFailureTablePreservesOrdinalAccounting(t *testing.T) {
	workspace := t.TempDir()
	uris := make([]string, 14)
	for i := range uris {
		uris[i] = fmt.Sprintf("file://%s/document-%02d.go", filepath.ToSlash(workspace), i)
	}
	cases := []struct {
		name    string
		failure session.Failure
		want    ManagedDocumentPreparationFailure
	}{
		{name: "uri", failure: sessionruntime.DocumentURIUnavailable, want: ManagedDocumentPreparationURIUnavailable},
		{name: "scope", failure: sessionruntime.DocumentOutsideWorkspace, want: ManagedDocumentPreparationOutsideWorkspace},
		{name: "source", failure: sessionruntime.DocumentSourceUnavailable, want: ManagedDocumentPreparationSourceUnavailable},
		{name: "language", failure: sessionruntime.LanguageIDUnavailable, want: ManagedDocumentPreparationLanguageIDUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var records []ManagedPreparationDiagnostic
			preparer := ManagedPreparer{
				Preparer: documentPrepareFunc(func(_ context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
					calls++
					if calls == 10 {
						return sessionruntime.DocumentResult{Failure: tc.failure}
					}
					return sessionruntime.DocumentResult{URI: request.URI, LanguageID: "go", Supply: &sessionruntime.DocumentSupply{URI: request.URI, SessionID: request.SessionID, Generation: request.Generation, DocumentVersion: 1, Content: []byte("package fixture\\n")}}
				}),
				Workspace:  workspaceRootFunc(func(_ string, _ uint64) (string, bool) { return workspace, true }),
				LanguageID: "go",
				Recorder: ManagedPreparationDiagnosticRecorderFunc(func(record ManagedPreparationDiagnostic) error {
					records = append(records, record)
					return nil
				}),
			}
			documents, err := preparer.PrepareManagedDocuments(context.Background(), workspace, "session", 7, "utf-16", uris, ManagedPreparationLimits{MaxDocuments: 14, MaxMessages: 14, MaxWork: 14, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20})
			want := ManagedPreparationDiagnostic{Failure: tc.want, Attempted: 10, Succeeded: 9, Planned: 14, FailingOrdinal: 9}
			var managed *ManagedPreparationError
			if len(documents) != 0 || !errors.As(err, &managed) || managed.Diagnostic != want || len(records) != 1 || records[0] != want || calls != 10 {
				t.Fatalf("ASSERT_MANAGED_PREPARATION_ORDINAL_9_%s: documents=%d managed=%+v records=%+v calls=%d err=%v", tc.name, len(documents), managed, records, calls, err)
			}
		})
	}
}

func TestManagedPreparationRuntimeSupplyCapIsDistinctFromConfiguredDocumentBytesLimit(t *testing.T) {
	limits := ManagedPreparationLimits{MaxDocuments: 8, MaxMessages: 8, MaxWork: 8, MaxDocumentBytes: 16, MaxTotalBytes: 4096}
	workspace := workspaceRootFunc(func(string, uint64) (string, bool) { return "/workspace", true })
	runtimeRejected := ManagedPreparer{Preparer: documentPrepareFunc(func(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
		return sessionruntime.DocumentResult{Failure: sessionruntime.DocumentSupplyUnavailable}
	}), Workspace: workspace}
	_, availabilityErr := runtimeRejected.PrepareManagedDocuments(context.Background(), "/workspace", "s", 1, "utf-16", []string{"file:///workspace/too-large.go"}, limits)
	var availability *ManagedPreparationError
	var availabilityLimit privateResourceLimit
	if !errors.As(availabilityErr, &availability) || availability.Diagnostic.Failure != ManagedDocumentPreparationSupplyUnavailable || errors.As(availabilityErr, &availabilityLimit) {
		t.Fatalf("ASSERT_RUNTIME_SUPPLY_CAP_AVAILABILITY: %v", availabilityErr)
	}
	downstreamRejected := ManagedPreparer{Preparer: documentPrepareFunc(func(_ context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
		return sessionruntime.DocumentResult{URI: request.URI, Supply: &sessionruntime.DocumentSupply{URI: request.URI, SessionID: request.SessionID, Generation: request.Generation, Content: bytes.Repeat([]byte{'x'}, 17)}}
	}), Workspace: workspace}
	_, limitErr := downstreamRejected.PrepareManagedDocuments(context.Background(), "/workspace", "s", 1, "utf-16", []string{"file:///workspace/configured-limit.go"}, limits)
	var limit privateResourceLimit
	if !errors.As(limitErr, &limit) || limit.ResourceField() != "document_bytes" || limit.ResourceObserved() != 17 || limit.ResourceLimit() != 16 {
		t.Fatalf("ASSERT_CONFIGURED_DOCUMENT_BYTES_RESOURCE_LIMIT: %v", limitErr)
	}
}

func TestManagedPreparationOnlyResourceLimitImplementsPrivateAccounting(t *testing.T) {
	resource := &managedPreparationResourceLimitError{managed: &ManagedPreparationError{Kind: ManagedPreparationResourceLimit, Cause: errors.New("managed document preparation resource limit")}, field: "messages", observed: 33, limit: 32}
	for _, tc := range []struct {
		name      string
		err       error
		wantLimit bool
		category  string
		field     string
	}{
		{name: "availability", err: managedPreparationError(ManagedPreparationAvailability, "managed document preparation unavailable"), category: "availability", field: "source_preparation"},
		{name: "resource-limit", err: resource, wantLimit: true, category: "input", field: "messages"},
		{name: "custody", err: managedPreparationError(ManagedPreparationCustody, "managed document custody mismatch"), category: "internal", field: "capture"},
		{name: "malformed", err: managedPreparationError(ManagedPreparationMalformed, "managed document malformed supply"), category: "internal", field: "capture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var managed *ManagedPreparationError
			if !errors.As(tc.err, &managed) || managed.Kind == "" {
				t.Fatalf("ASSERT_MANAGED_PREPARATION_TYPED_%s: err=%v", tc.name, tc.err)
			}
			var limit privateResourceLimit
			if got := errors.As(tc.err, &limit); got != tc.wantLimit {
				t.Fatalf("ASSERT_MANAGED_PREPARATION_RESOURCE_INTERFACE_%s: got=%t want=%t", tc.name, got, tc.wantLimit)
			}
			diagnostic := classifyCaptureFailure(tc.err)
			if diagnostic.Category != tc.category || diagnostic.FailedField != tc.field {
				t.Fatalf("ASSERT_MANAGED_PREPARATION_CLASSIFICATION_%s: %+v", tc.name, diagnostic)
			}
			result := Result{}
			applyPrivateResourceLimit(&result, tc.err)
			if tc.wantLimit {
				if result.PrivateResourceComponent != "source_preparation" || result.PrivateResourceCategory != "input" || result.PrivateResourceObserved != 33 || result.PrivateResourceLimit != 32 {
					t.Fatalf("ASSERT_MANAGED_PREPARATION_PRIVATE_ACCOUNTING_resource-limit: %+v", result)
				}
			} else if result.PrivateResourceComponent != "" || result.PrivateResourceCategory != "" || result.PrivateResourceObserved != 0 || result.PrivateResourceLimit != 0 {
				t.Fatalf("ASSERT_MANAGED_PREPARATION_NO_PRIVATE_ACCOUNTING_%s: %+v", tc.name, result)
			}
			if strings.Contains(tc.err.Error(), "/") || strings.Contains(tc.err.Error(), "file:") {
				t.Fatalf("ASSERT_MANAGED_PREPARATION_PRIVATE_SAFE_TEXT_%s: %v", tc.name, tc.err)
			}
		})
	}
}

func TestManagedPreparationLateCaptureCompletesWithDocument(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "target.go")
	content := []byte("package fixture\n")
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	writer := &managedCaptureWriter{}
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 16},
		Starter: managedCaptureStarter{child: managedCaptureChild{writer: writer}},
	})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected)})
	if started.Failure != "" {
		t.Fatal(started)
	}
	if ready := manager.ObserveInitialization(started.SessionID, started.Generation, true); ready.Failure != "" {
		t.Fatal(ready)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	initial := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: false})
	if initial.Failure != "" || initial.Supply != nil {
		t.Fatalf("ASSERT_MANAGED_LATE_CAPTURE_INITIAL_NO_SUPPLY: %+v", initial)
	}
	preparer := ManagedPreparer{
		Preparer:   manager,
		Workspace:  manager,
		LanguageID: "go",
	}
	documents, err := preparer.PrepareManagedDocuments(context.Background(), workspace, started.SessionID, started.Generation, "utf-16", []string{uri}, ManagedPreparationLimits{MaxDocuments: 1, MaxMessages: 1, MaxWork: 1, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 1 << 20})
	if err != nil || len(documents) != 1 || documents[0].URI != uri || !bytes.Equal(documents[0].Bytes, content) || documents[0].SessionID != started.SessionID || documents[0].Generation != started.Generation {
		t.Fatalf("ASSERT_MANAGED_LATE_CAPTURE_COMPLETES_ONE_DOCUMENT_NOT_AVAILABILITY: documents=%+v err=%v", documents, err)
	}
}

func TestManagedPreparationDocumentFailuresRemainPrivateAvailability(t *testing.T) {
	workspace := t.TempDir()
	for _, tc := range []struct {
		name   string
		result sessionruntime.DocumentResult
	}{
		{name: "document-failure", result: sessionruntime.DocumentResult{Failure: session.StaleGeneration}},
		{name: "nil-supply", result: sessionruntime.DocumentResult{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareCalls := 0
			preparer := ManagedPreparer{
				Preparer: documentPrepareFunc(func(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
					prepareCalls++
					return tc.result
				}),
				Workspace: workspaceRootFunc(func(string, uint64) (string, bool) { return workspace, true }),
			}
			documents, err := preparer.PrepareManagedDocuments(context.Background(), workspace, "session", 1, "utf-16", []string{"file:///private/workspace/target.go"}, ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 100, MaxWork: 100, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20})
			if err == nil || len(documents) != 0 || prepareCalls != 1 {
				t.Fatalf("ASSERT_MANAGED_DOCUMENT_FAILURE_ATOMIC_%s: documents=%d calls=%d err=%v", tc.name, len(documents), prepareCalls, err)
			}
			diagnostic := classifyCaptureFailure(err)
			if diagnostic.Category != "availability" || diagnostic.FailedField != "source_preparation" || diagnostic.Observed != 0 || diagnostic.Limit != 0 || strings.Contains(err.Error(), "private/workspace") || strings.Contains(err.Error(), string(session.StaleGeneration)) {
				t.Fatalf("ASSERT_MANAGED_DOCUMENT_FAILURE_PRIVATE_%s: diagnostic=%+v err=%v", tc.name, diagnostic, err)
			}
		})
	}
}

func TestManagedPreparationMessageLimitPreservesSafeAccountingAtomically(t *testing.T) {
	workspace := t.TempDir()
	prepareCalls := 0
	preparer := ManagedPreparer{
		Preparer: documentPrepareFunc(func(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
			prepareCalls++
			return sessionruntime.DocumentResult{}
		}),
		Workspace: workspaceRootFunc(func(string, uint64) (string, bool) { return workspace, true }),
	}
	uris := make([]string, 33)
	for i := range uris {
		uris[i] = fmt.Sprintf("file:///private/workspace/%d.go", i)
	}
	documents, err := preparer.PrepareManagedDocuments(context.Background(), workspace, "session", 1, "utf-16", uris, ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 32, MaxWork: 100, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20})
	if err == nil || len(documents) != 0 || prepareCalls != 0 {
		t.Fatalf("ASSERT_MANAGED_MESSAGE_LIMIT_ATOMIC: documents=%d calls=%d err=%v", len(documents), prepareCalls, err)
	}
	diagnostic := classifyCaptureFailure(err)
	if diagnostic.Category != "input" || diagnostic.FailedField != "messages" || diagnostic.Observed != 33 || diagnostic.Limit != 32 || diagnostic.Invariant != "OBSERVED_MUST_NOT_EXCEED_LIMIT" || diagnostic.CallerAction != "INCREASE_BOUNDED_CAPTURE_LIMIT" || diagnostic.Recovery != "RESTART_FROM_PRESERVED_CENSUS_COMMIT" || strings.Contains(err.Error(), "private/workspace") {
		t.Fatalf("ASSERT_MANAGED_MESSAGE_LIMIT_DIAGNOSTIC: diagnostic=%+v err=%v", diagnostic, err)
	}
}

func TestManagedWorkspaceRoutingMismatchIsTypedBeforePreparation(t *testing.T) {
	f, h := managedCaptureFixture(t)
	prepareCalls, resolverCalls := 0, 0
	preparer := ManagedPreparer{
		Preparer: documentPrepareFunc(func(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
			prepareCalls++
			return sessionruntime.DocumentResult{}
		}),
		Workspace:  workspaceRootFunc(func(string, uint64) (string, bool) { return t.TempDir(), true }),
		LanguageID: "go",
	}
	deps := FreshCaptureDependencies{
		Context:  context.Background(),
		Preparer: preparer,
		Resolver: v5sourcesnapshotv6.ResolverFunc(func(context.Context, v5sourcesnapshotv6.ResolveRequest) (v5sourcesnapshotv6.ResolveResult, error) {
			resolverCalls++
			return v5sourcesnapshotv6.ResolveResult{}, nil
		}),
		Limits: ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 100, MaxWork: 1000, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20},
	}
	_, err := captureFresh(h, f.direct, f.workspace, h.PositionEncoding(), testContract(t).CaptureLimits(), &deps)
	if err == nil {
		t.Fatal("ASSERT_MANAGED_ROUTING_TYPED_BEFORE_CALLS: capture unexpectedly succeeded")
	}
	diagnostic := classifyCaptureFailure(err)
	if diagnostic.Category != "availability" || diagnostic.FailedField != "source_preparation" || diagnostic.CallerAction != "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME" || prepareCalls != 0 || resolverCalls != 0 {
		t.Fatalf("ASSERT_MANAGED_ROUTING_TYPED_BEFORE_CALLS: diagnostic=%+v prepare_calls=%d resolver_calls=%d err=%v", diagnostic, prepareCalls, resolverCalls, err)
	}
}

func TestManagedCaptureDeterministicUnderConstituentOrdering(t *testing.T) {
	workspace, f, h := partitionCaptureFixture(t, false)
	capture := func(reverse bool) CaptureResult {
		program := f.result
		program.Representatives.Nominations = append([]censusprogramc.Representative(nil), f.result.Representatives.Nominations...)
		if reverse {
			for i, j := 0, len(program.Representatives.Nominations)-1; i < j; i, j = i+1, j-1 {
				program.Representatives.Nominations[i], program.Representatives.Nominations[j] = program.Representatives.Nominations[j], program.Representatives.Nominations[i]
			}
		}
		prepareCalls, resolverCalls := 0, 0
		deps := successfulManagedDependencies(t, workspace, &prepareCalls, &resolverCalls, false)
		got, err := CaptureSnapshots(h, program, workspace, h.PositionEncoding(), testContract(t).CaptureLimits(), deps)
		if err != nil {
			t.Fatal(err)
		}
		if prepareCalls != 1 || resolverCalls == 0 {
			t.Fatalf("ASSERT_MANAGED_CAPTURE_ENTERS_RESOLVER_AFTER_ONE_PREPARATION: prepare_calls=%d resolver_calls=%d", prepareCalls, resolverCalls)
		}
		return got
	}
	left, right := capture(false), capture(true)
	leftSnapshots, _, err := ReplaySnapshots(left)
	if err != nil {
		t.Fatal(err)
	}
	rightSnapshots, _, err := ReplaySnapshots(right)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(leftSnapshots, func(i, j int) bool { return leftSnapshots[i].ConstituentOrdinal < leftSnapshots[j].ConstituentOrdinal })
	sort.Slice(rightSnapshots, func(i, j int) bool {
		return rightSnapshots[i].ConstituentOrdinal < rightSnapshots[j].ConstituentOrdinal
	})
	if !reflect.DeepEqual(leftSnapshots, rightSnapshots) {
		t.Fatal("ASSERT_MANAGED_CAPTURE_DETERMINISTIC_UNDER_CONSTITUENT_ORDERING")
	}
}

var _ liveprojection.DocumentPreparer = documentPrepareFunc(nil)
