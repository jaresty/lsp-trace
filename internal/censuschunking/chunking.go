// Package censuschunking owns private canonical census chunk plans and local projections.
package censuschunking

import (
	"errors"

	"lsp-trace/internal/captureset"
)

const (
	PlanVersion       = "lsp-trace.census-chunk-plan.private.v1"
	IndexVersion      = "lsp-trace.census-chunk-index.private.v1"
	ProjectionVersion = "lsp-trace.census-chunk-projection.private.v1"
	SidecarVersion    = "lsp-trace.census-chunk-projection-sidecar.private.v1"
	ChunkSize         = 10000
	MaxGlobalTargets  = 100000
)

var ErrNotImplemented = errors.New("census chunking not implemented")

type ArtifactRef struct {
	Selector, SHA256 string
	ByteLength       uint64
}
type SourceCoordinate struct {
	Path            string
	Line, Character uint32
}
type GlobalFile struct {
	Ordinal           int
	Path, Disposition string
}
type GlobalTarget struct {
	GlobalOrdinal, GlobalFileOrdinal                       int
	GlobalIdentity, CanonicalSeedV2, CanonicalSeedV2SHA256 string
	Coordinate                                             SourceCoordinate
	Kind                                                   int
	Name, URI, LanguageID, Disposition                     string
}
type Input struct {
	GlobalProjection                      ArtifactRef
	RequestFingerprint, SessionID         string
	Generation                            uint64
	WorkspaceURI, WorkspaceDigest         string
	CanonicalOrderPolicy, DuplicatePolicy string
	BatchTargets, DownDepth, UpDepth      int
	Files                                 []GlobalFile
	Targets                               []GlobalTarget
	GlobalProviderFailures                []string
}
type ChunkDescriptor struct {
	Ordinal, TargetStart, TargetCount int
	Projection, Sidecar               ArtifactRef
}
type Plan struct {
	SchemaVersion, LogicalDigest, ImmutableSelector        string
	Accepted                                               bool
	Authority                                              int
	Completeness                                           string
	GlobalProjection                                       ArtifactRef
	RequestFingerprint, SessionID                          string
	Generation                                             uint64
	WorkspaceDigest, CanonicalOrderPolicy, DuplicatePolicy string
	BatchTargets, DownDepth, UpDepth                       int
	Chunks                                                 []ChunkDescriptor
}
type Index struct {
	SchemaVersion, PlanDigest string
	Chunks                    []ChunkDescriptor
}
type LocalFile struct {
	Ordinal, GlobalOrdinal int
	Path, Disposition      string
}
type LocalSymbol struct {
	Ordinal, FileOrdinal        int
	IdentityDigest, Disposition string
	Coordinate                  SourceCoordinate
	Kind                        int
}
type LocalTarget struct {
	CensusOrdinal                                          int
	CanonicalSeedV2, CanonicalSeedV2SHA256, SymbolIdentity string
}
type Projection struct {
	SchemaVersion, LogicalDigest, ImmutableSelector string
	Accepted                                        bool
	Authority                                       int
	Completeness, ProjectionStatus                  string
	ChunkOrdinal                                    int
	Files                                           []LocalFile
	Symbols                                         []LocalSymbol
	Targets                                         []LocalTarget
	ProviderFailures                                []string
}
type Mapping struct {
	ChunkOrdinal, LocalOrdinal, GlobalOrdinal int
	GlobalIdentity, GlobalCanonicalSeedDigest string
	GlobalFileOrdinal                         int
	Coordinate                                SourceCoordinate
}
type Sidecar struct {
	SchemaVersion, LogicalDigest, ImmutableSelector string
	ChunkOrdinal                                    int
	Mappings                                        []Mapping
}
type Chunk struct {
	Descriptor                    ChunkDescriptor
	Projection                    Projection
	Sidecar                       Sidecar
	ProjectionBytes, SidecarBytes []byte
}
type Result struct {
	Compatibility      bool
	CompatibilityBytes []byte
	Plan               Plan
	Index              Index
	Chunks             []Chunk
}

func Build(Input, []byte) (Result, error)         { return Result{}, ErrNotImplemented }
func EncodePlan(Plan) ([]byte, error)             { return nil, ErrNotImplemented }
func DecodePlan([]byte) (Plan, error)             { return Plan{}, ErrNotImplemented }
func EncodeIndex(Index) ([]byte, error)           { return nil, ErrNotImplemented }
func DecodeIndex([]byte) (Index, error)           { return Index{}, ErrNotImplemented }
func EncodeProjection(Projection) ([]byte, error) { return nil, ErrNotImplemented }
func DecodeProjection([]byte) (Projection, error) { return Projection{}, ErrNotImplemented }
func EncodeSidecar(Sidecar) ([]byte, error)       { return nil, ErrNotImplemented }
func DecodeSidecar([]byte) (Sidecar, error)       { return Sidecar{}, ErrNotImplemented }

var _ = captureset.MaxTargets
