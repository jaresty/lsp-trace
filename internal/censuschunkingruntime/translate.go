package censuschunkingruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censusrequest"
)

// Translate converts one complete census discovery and its exact request receipt
// into the transport-neutral chunk coordinator input and enumeration projection.
func Translate(discovery censusacquisition.Discovery, request censusrequest.Receipt) (censuschunking.Input, censusenumerationprojection.Projection, error) {
	workspaceURI := (&url.URL{Scheme: "file", Path: discovery.Workspace}).String()
	files := make([]censusenumerationprojection.File, len(discovery.FileLedger.Entries))
	globalFiles := make([]censuschunking.GlobalFile, len(files))
	for i, entry := range discovery.FileLedger.Entries {
		files[i] = censusenumerationprojection.File{Ordinal: entry.Ordinal, Path: entry.Identity, Disposition: entry.Disposition}
		globalFiles[i] = censuschunking.GlobalFile{Ordinal: entry.Ordinal, Path: entry.Identity, Disposition: entry.Disposition}
	}
	if len(discovery.EnumerationSymbols) != len(discovery.Targets) {
		return censuschunking.Input{}, censusenumerationprojection.Projection{}, errors.New("private census chunk target enumeration mismatch")
	}
	symbols := make([]censusenumerationprojection.Symbol, len(discovery.EnumerationSymbols))
	targets := make([]censuschunking.GlobalTarget, len(symbols))
	for i, symbol := range discovery.EnumerationSymbols {
		if symbol.Ordinal != i || symbol.FileOrdinal < 0 || symbol.FileOrdinal >= len(files) {
			return censuschunking.Input{}, censusenumerationprojection.Projection{}, errors.New("private census chunk ordinal mismatch")
		}
		target := discovery.Targets[i]
		position := censusenumerationprojection.Position{Line: symbol.SelectionStart.Line, Character: symbol.SelectionStart.Character}
		identity, _, digestErr := censusenumerationprojection.SymbolIdentityDigest(files[symbol.FileOrdinal].Path, position, symbol.Kind, symbol.Name, symbol.Ordinal)
		if digestErr != nil || target.CensusOrdinal != i || target.Name != symbol.Name || target.Kind != symbol.Kind || target.Position() != symbol.SelectionStart {
			return censuschunking.Input{}, censusenumerationprojection.Projection{}, errors.New("private census chunk target identity mismatch")
		}
		symbols[i] = censusenumerationprojection.Symbol{Ordinal: symbol.Ordinal, FileOrdinal: symbol.FileOrdinal, SelectionStart: position, Kind: symbol.Kind, Disposition: string(symbol.Disposition), IdentityDigest: identity}
		targets[i] = censuschunking.GlobalTarget{GlobalOrdinal: i, GlobalFileOrdinal: symbol.FileOrdinal, GlobalIdentity: identity, CanonicalSeedV2: string(target.CanonicalSeedV2), CanonicalSeedV2SHA256: objectSelector(target.CanonicalSeedV2), Coordinate: censuschunking.SourceCoordinate{Path: files[symbol.FileOrdinal].Path, Line: position.Line, Character: position.Character}, Kind: symbol.Kind, Name: symbol.Name, URI: target.URI, LanguageID: target.LanguageID, Disposition: "prepared"}
	}
	projection, err := censusenumerationprojection.Build(censusenumerationprojection.Input{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, RequestFingerprint: request.Fingerprint, WorkspaceURI: workspaceURI, Files: files, Symbols: symbols, Observations: []censusenumerationprojection.Observation{}, ProviderFailures: []censusenumerationprojection.ProviderFailure{}, IncompleteCauses: []censusenumerationprojection.IncompleteCause{}, RequestDownDepth: request.Semantic.DownDepth, RequestUpDepth: request.Semantic.UpDepth, RequestMaxNodes: request.Semantic.MaxNodes, RequestBatchTargets: request.Semantic.BatchTargets, RequestTimeoutMS: request.Semantic.RequestTimeoutMS, TimeoutMS: request.Semantic.TimeoutMS})
	if err != nil {
		return censuschunking.Input{}, censusenumerationprojection.Projection{}, err
	}
	workspaceDigest, err := censusenumerationprojection.WorkspaceDigest(workspaceURI)
	if err != nil {
		return censuschunking.Input{}, censusenumerationprojection.Projection{}, err
	}
	input := censuschunking.Input{GlobalProjection: censuschunking.ArtifactRef{Selector: "pending", SHA256: objectSelector([]byte("pending")), ByteLength: 1}, RequestFingerprint: request.Fingerprint, SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, WorkspaceURI: workspaceURI, WorkspaceDigest: workspaceDigest, CanonicalOrderPolicy: "captureset-order-targets-v1", DuplicatePolicy: "reject-global-ordinal-identity-seed-bytes-digest-v1", BatchTargets: int(request.Semantic.BatchTargets), DownDepth: int(request.Semantic.DownDepth), UpDepth: int(request.Semantic.UpDepth), Files: globalFiles, Targets: targets, GlobalProviderFailures: []string{}}
	return input, projection, nil
}

func objectSelector(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
