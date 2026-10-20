package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/censuschunkingruntime"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/seedformat"
)

type terminalFixtureStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newTerminalFixtureStore() *terminalFixtureStore {
	return &terminalFixtureStore{objects: map[string][]byte{}}
}

func (s *terminalFixtureStore) MaxObjectBytes() int64 { return censuschunking.MaxCanonicalBytes }
func (s *terminalFixtureStore) Put(_ context.Context, raw []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	selector := censuschunkingruntime.ObjectSelector(raw)
	s.objects[selector] = append([]byte(nil), raw...)
	return selector, nil
}
func (s *terminalFixtureStore) Get(_ context.Context, selector string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.objects[selector]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), raw...), nil
}

type terminalFixtureHeads struct {
	mu       sync.Mutex
	rawByKey map[string][]byte
}

func newTerminalFixtureHeads() *terminalFixtureHeads {
	return &terminalFixtureHeads{rawByKey: map[string][]byte{}}
}
func (h *terminalFixtureHeads) Load(_ context.Context, key string) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	raw, ok := h.rawByKey[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), raw...), nil
}
func (h *terminalFixtureHeads) CompareAndSwap(_ context.Context, key string, expected censuschunking.HeadRevision, replacement []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	current, ok := h.rawByKey[key]
	if !ok {
		if expected != (censuschunking.HeadRevision{}) {
			return errors.New("terminal fixture CAS conflict")
		}
	} else {
		revision, err := censuschunking.ParseCoordinatorHeadRevision(current)
		if err != nil || revision != expected {
			return errors.New("terminal fixture CAS conflict")
		}
	}
	if _, err := censuschunking.ParseCoordinatorHeadRevision(replacement); err != nil {
		return err
	}
	h.rawByKey[key] = append([]byte(nil), replacement...)
	return nil
}

type terminalFixtureAcquirer func(context.Context, censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error)

func (f terminalFixtureAcquirer) AcquireV5(ctx context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	return f(ctx, request)
}

type terminalFixtureSession struct{ t *testing.T }

func (s terminalFixtureSession) WithReadySession(ctx context.Context, expected censuschunking.ExpectedSession, run func(censusacquisition.Core) error) error {
	core := censusacquisition.Core{Acquirer: terminalFixtureAcquirer(func(_ context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		native, err := json.Marshal(graph.Result{
			SchemaVersion: graph.SchemaVersionV5,
			Invocation: graph.Invocation{
				Server:     graph.ServerInvocation{Command: "fixture"},
				Seeds:      []graph.InvocationSeed{},
				Provenance: graph.InvocationProvenance{InvocationID: "terminal-fixture", SourceRevision: "test", ServerVersion: "fixture"},
				Expansion:  graph.ExpansionConfig{TopmostSiblings: true},
			},
			Seeds: []graph.SeedResult{}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
		})
		if err != nil {
			return censusacquisition.AcquiredV5{}, err
		}
		raw, err := graphprovenance.CaptureV5WithSeedSpec(native, request.Session.SessionID, request.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, request.CanonicalSeedsV2)
		return censusacquisition.AcquiredV5{Session: request.Session, Raw: raw}, err
	})}
	return run(core)
}

func realCoordinatorTerminalFixture(t *testing.T) *privateCensusChunkCompletion {
	t.Helper()
	workspace := filepath.Clean(t.TempDir())
	workspaceURI := (&url.URL{Scheme: "file", Path: workspace}).String()
	workspaceDigest, err := censusenumerationprojection.WorkspaceDigest(workspaceURI)
	if err != nil {
		t.Fatal(err)
	}
	input := censuschunking.Input{
		GlobalProjection:   censuschunking.ArtifactRef{Selector: "global/selector", SHA256: censuschunkingruntime.ObjectSelector([]byte("global")), ByteLength: 6},
		RequestFingerprint: censuschunkingruntime.ObjectSelector([]byte("request")), SessionID: "session", Generation: 7,
		WorkspaceURI: workspaceURI, WorkspaceDigest: workspaceDigest, CanonicalOrderPolicy: "captureset-order-targets-v1",
		DuplicatePolicy: "reject-global-ordinal-identity-seed-bytes-digest-v1", BatchTargets: 63, DownDepth: 1, UpDepth: 1,
		Files: []censuschunking.GlobalFile{{Ordinal: 0, Path: "a.go", Disposition: "processed"}}, GlobalProviderFailures: []string{},
	}
	input.Targets = make([]censuschunking.GlobalTarget, censuschunking.ChunkSize+1)
	for i := range input.Targets {
		position := censusenumerationprojection.Position{Line: uint32(i), Character: 1}
		globalOrdinal := i*2 + 3
		seed, seedErr := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", globalOrdinal), Path: "a.go", Line: uint64(i) + 1, Column: 2}}}}, workspace)
		if seedErr != nil {
			t.Fatal(seedErr)
		}
		identity, _, identityErr := censusenumerationprojection.SymbolIdentityDigest("a.go", position, 12, "", globalOrdinal)
		if identityErr != nil {
			t.Fatal(identityErr)
		}
		input.Targets[i] = censuschunking.GlobalTarget{GlobalOrdinal: globalOrdinal, GlobalFileOrdinal: 0, GlobalIdentity: identity, CanonicalSeedV2: string(seed), CanonicalSeedV2SHA256: censuschunkingruntime.ObjectSelector(seed), Coordinate: censuschunking.SourceCoordinate{Path: "a.go", Line: uint32(i), Character: 1}, Kind: 12, Name: fmt.Sprintf("Target%05d", i), URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "a.go")}).String(), LanguageID: "go", Disposition: "prepared"}
	}
	built, err := censuschunking.Build(input, nil)
	if err != nil || built.Compatibility {
		t.Fatalf("build real terminal fixture: compatibility=%v err=%v", built.Compatibility, err)
	}
	projections := make([]censuschunking.Projection, len(built.Chunks))
	sidecars := make([]censuschunking.Sidecar, len(built.Chunks))
	for i := range built.Chunks {
		projections[i], sidecars[i] = built.Chunks[i].Projection, built.Chunks[i].Sidecar
	}
	store := newTerminalFixtureStore()
	result, err := censuschunking.Coordinate(context.Background(), censuschunking.CoordinatorRequest{Input: input, Plan: built.Plan, Index: built.Index, Projections: projections, Sidecars: sidecars, Store: store, Heads: newTerminalFixtureHeads(), Sessions: terminalFixtureSession{t: t}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.Get(context.Background(), result.TerminalSelector)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := censuschunking.ParseTerminalIndex(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &privateCensusChunkCompletion{terminalSelector: terminal.ImmutableSelector, terminalIndex: raw}
}
