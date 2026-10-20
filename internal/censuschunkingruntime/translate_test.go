package censuschunkingruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/seedformat"
)

func TestTranslatePreservesExactChunkIdentity(t *testing.T) {
	workspace := filepath.Clean(t.TempDir())
	position := lsp.Position{Line: 4, Character: 2}
	seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "target", Path: "a.go", Line: 5, Column: 3}}}}, workspace)
	if err != nil {
		t.Fatal(err)
	}
	discovery := censusacquisition.Discovery{
		Session: censusacquisition.SessionIdentity{SessionID: "session", Generation: 7}, Workspace: workspace, Complete: true,
		FileLedger:         captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "a.go", Disposition: "selected"}}},
		EnumerationSymbols: []censusacquisition.EnumerationSymbol{{Ordinal: 0, FileOrdinal: 0, SelectionStart: position, Kind: 12, Name: "Target", Disposition: census.SymbolSelected}},
		Targets:            []censusacquisition.PreparedTarget{{CensusOrdinal: 0, CanonicalSeedV2: seed, URI: "file:///workspace/a.go", LanguageID: "go", SelectionRange: lsp.Range{Start: position, End: position}, Name: "Target", Kind: 12, Range: lsp.Range{Start: position, End: position}}},
	}
	receipt, err := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: "session", Generation: 7}, censusrequest.SemanticFields{Sources: []string{"."}, Includes: []string{"**/*.go"}, Excludes: []string{"vendor/**"}, DownDepth: 3, UpDepth: 2, MaxNodes: 10000, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000})
	if err != nil {
		t.Fatal(err)
	}
	input, projection, err := Translate(discovery, receipt)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(seed)
	wantSeedDigest := "sha256:" + hex.EncodeToString(sum[:])
	if input.RequestFingerprint != receipt.Fingerprint || input.SessionID != "session" || input.Generation != 7 || input.BatchTargets != 63 || input.DownDepth != 3 || input.UpDepth != 2 || input.WorkspaceURI == "" || input.WorkspaceDigest == "" {
		t.Fatalf("ASSERT_CENSUS_CHUNK_TRANSLATION_REQUEST_IDENTITY: input=%+v receipt=%+v", input, receipt)
	}
	if len(input.Files) != 1 || input.Files[0].Ordinal != 0 || input.Files[0].Path != "a.go" || len(input.Targets) != 1 {
		t.Fatalf("ASSERT_CENSUS_CHUNK_TRANSLATION_ORDINALS: files=%+v targets=%+v", input.Files, input.Targets)
	}
	target := input.Targets[0]
	if target.GlobalOrdinal != 0 || target.GlobalFileOrdinal != 0 || target.GlobalIdentity == "" || target.CanonicalSeedV2 != string(seed) || target.CanonicalSeedV2SHA256 != wantSeedDigest || target.Name != "Target" || target.Kind != 12 || target.URI != "file:///workspace/a.go" || target.LanguageID != "go" || target.Coordinate.Line != 4 || target.Coordinate.Character != 2 {
		t.Fatalf("ASSERT_CENSUS_CHUNK_TRANSLATION_TARGET_IDENTITY: %+v", target)
	}
	if projection.SessionID != "session" || projection.Generation != 7 || projection.RequestFingerprint != receipt.Fingerprint || projection.WorkspaceURIDigest != input.WorkspaceDigest || projection.Bounds.RequestDownDepth != 3 || projection.Bounds.RequestUpDepth != 2 || projection.Bounds.RequestMaxNodes != 10000 || projection.Bounds.RequestBatchTargets != 63 || projection.Bounds.TimeoutMS != 60000 || projection.Bounds.RequestTimeoutMS != 30000 || len(projection.Files) != 1 || len(projection.Symbols) != 1 || projection.Symbols[0].IdentityDigest != target.GlobalIdentity {
		t.Fatalf("ASSERT_CENSUS_CHUNK_TRANSLATION_PROJECTION_IDENTITY: %+v", projection)
	}
	if !reflect.DeepEqual(discovery.Targets[0].CanonicalSeedV2, seed) {
		t.Fatal("ASSERT_CENSUS_CHUNK_TRANSLATION_SEED_PRESERVED")
	}
}
