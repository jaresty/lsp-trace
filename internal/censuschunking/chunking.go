// Package censuschunking owns private canonical census chunk plans and local projections.
// It does not execute acquisition, publish artifacts, or infer global discovery outcomes.
package censuschunking

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/strictjson"
)

const (
	PlanVersion       = "lsp-trace.census-chunk-plan.private.v1"
	IndexVersion      = "lsp-trace.census-chunk-index.private.v1"
	ProjectionVersion = "lsp-trace.census-chunk-projection.private.v1"
	SidecarVersion    = "lsp-trace.census-chunk-projection-sidecar.private.v1"
	ChunkSize         = captureset.MaxTargets
	MaxGlobalTargets  = censusenumerationprojection.MaxSymbols
	MaxCanonicalBytes = censusenumerationprojection.MaxCanonicalBytes
	MaxIdentityBytes  = censusenumerationprojection.MaxIdentityInputBytes
	MaxWork           = censusenumerationprojection.MaxWork
)

const (
	planDomain       = "lsp-trace:census-chunk-plan:private:v1"
	indexDomain      = "lsp-trace:census-chunk-index:private:v1"
	projectionDomain = "lsp-trace:census-chunk-projection:private:v1"
	sidecarDomain    = "lsp-trace:census-chunk-projection-sidecar:private:v1"
)

var (
	ErrZeroTargets    = errors.New("census chunking requires at least one target")
	ErrTooManyTargets = errors.New("census chunking target bound exceeded")
	maxWorkLimit      = uint64(MaxWork)
)

type ArtifactRef struct {
	Selector   string `json:"selector"`
	SHA256     string `json:"sha256"`
	ByteLength uint64 `json:"byte_length"`
}
type SourceCoordinate struct {
	Path      string `json:"path"`
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}
type GlobalFile struct {
	Ordinal     int    `json:"ordinal"`
	Path        string `json:"path"`
	Disposition string `json:"disposition"`
}
type GlobalTarget struct {
	GlobalOrdinal         int              `json:"global_ordinal"`
	GlobalFileOrdinal     int              `json:"global_file_ordinal"`
	GlobalIdentity        string           `json:"global_identity"`
	CanonicalSeedV2       string           `json:"canonical_seed_v2"`
	CanonicalSeedV2SHA256 string           `json:"canonical_seed_v2_sha256"`
	Coordinate            SourceCoordinate `json:"coordinate"`
	Kind                  int              `json:"kind"`
	Name                  string           `json:"name"`
	URI                   string           `json:"uri"`
	LanguageID            string           `json:"language_id"`
	Disposition           string           `json:"disposition"`
}
type Input struct {
	GlobalProjection       ArtifactRef    `json:"global_projection"`
	RequestFingerprint     string         `json:"request_fingerprint"`
	SessionID              string         `json:"session_id"`
	Generation             uint64         `json:"generation"`
	WorkspaceURI           string         `json:"workspace_uri"`
	WorkspaceDigest        string         `json:"workspace_digest"`
	CanonicalOrderPolicy   string         `json:"canonical_order_policy"`
	DuplicatePolicy        string         `json:"duplicate_policy"`
	BatchTargets           int            `json:"batch_targets"`
	DownDepth              int            `json:"down_depth"`
	UpDepth                int            `json:"up_depth"`
	Files                  []GlobalFile   `json:"files"`
	Targets                []GlobalTarget `json:"targets"`
	GlobalProviderFailures []string       `json:"global_provider_failures"`
}
type ChunkDescriptor struct {
	Ordinal     int         `json:"ordinal"`
	TargetStart int         `json:"target_start"`
	TargetCount int         `json:"target_count"`
	Projection  ArtifactRef `json:"projection"`
	Sidecar     ArtifactRef `json:"sidecar"`
}
type Plan struct {
	SchemaVersion        string            `json:"schema_version"`
	LogicalDigest        string            `json:"logical_digest"`
	ImmutableSelector    string            `json:"immutable_selector"`
	Accepted             bool              `json:"accepted"`
	Authority            int               `json:"authority"`
	Completeness         string            `json:"completeness"`
	GlobalProjection     ArtifactRef       `json:"global_projection"`
	RequestFingerprint   string            `json:"request_fingerprint"`
	SessionID            string            `json:"session_id"`
	Generation           uint64            `json:"generation"`
	WorkspaceDigest      string            `json:"workspace_digest"`
	CanonicalOrderPolicy string            `json:"canonical_order_policy"`
	DuplicatePolicy      string            `json:"duplicate_policy"`
	BatchTargets         int               `json:"batch_targets"`
	DownDepth            int               `json:"down_depth"`
	UpDepth              int               `json:"up_depth"`
	Chunks               []ChunkDescriptor `json:"chunks"`
}
type Index struct {
	SchemaVersion     string            `json:"schema_version"`
	LogicalDigest     string            `json:"logical_digest"`
	ImmutableSelector string            `json:"immutable_selector"`
	Accepted          bool              `json:"accepted"`
	Authority         int               `json:"authority"`
	Completeness      string            `json:"completeness"`
	PlanDigest        string            `json:"plan_digest"`
	Chunks            []ChunkDescriptor `json:"chunks"`
}
type LocalFile struct {
	Ordinal       int    `json:"ordinal"`
	GlobalOrdinal int    `json:"global_ordinal"`
	Path          string `json:"path"`
	Disposition   string `json:"disposition"`
}
type LocalSymbol struct {
	Ordinal        int              `json:"ordinal"`
	FileOrdinal    int              `json:"file_ordinal"`
	IdentityDigest string           `json:"identity_digest"`
	Disposition    string           `json:"disposition"`
	Coordinate     SourceCoordinate `json:"coordinate"`
	Kind           int              `json:"kind"`
}
type LocalTarget struct {
	CensusOrdinal         int    `json:"census_ordinal"`
	CanonicalSeedV2       string `json:"canonical_seed_v2"`
	CanonicalSeedV2SHA256 string `json:"canonical_seed_v2_sha256"`
	SymbolIdentity        string `json:"symbol_identity"`
}
type Projection struct {
	SchemaVersion     string        `json:"schema_version"`
	LogicalDigest     string        `json:"logical_digest"`
	ImmutableSelector string        `json:"immutable_selector"`
	Accepted          bool          `json:"accepted"`
	Authority         int           `json:"authority"`
	Completeness      string        `json:"completeness"`
	ProjectionStatus  string        `json:"projection_status"`
	ChunkOrdinal      int           `json:"chunk_ordinal"`
	Files             []LocalFile   `json:"files"`
	Symbols           []LocalSymbol `json:"symbols"`
	Targets           []LocalTarget `json:"targets"`
	ProviderFailures  []string      `json:"provider_failures"`
}
type Mapping struct {
	ChunkOrdinal              int              `json:"chunk_ordinal"`
	LocalOrdinal              int              `json:"local_ordinal"`
	GlobalOrdinal             int              `json:"global_ordinal"`
	GlobalIdentity            string           `json:"global_identity"`
	GlobalCanonicalSeedDigest string           `json:"global_canonical_seed_digest"`
	GlobalFileOrdinal         int              `json:"global_file_ordinal"`
	Coordinate                SourceCoordinate `json:"coordinate"`
}
type Sidecar struct {
	SchemaVersion     string    `json:"schema_version"`
	LogicalDigest     string    `json:"logical_digest"`
	ImmutableSelector string    `json:"immutable_selector"`
	Accepted          bool      `json:"accepted"`
	Authority         int       `json:"authority"`
	Completeness      string    `json:"completeness"`
	ChunkOrdinal      int       `json:"chunk_ordinal"`
	Mappings          []Mapping `json:"mappings"`
}
type Chunk struct {
	Descriptor      ChunkDescriptor
	Projection      Projection
	Sidecar         Sidecar
	ProjectionBytes []byte
	SidecarBytes    []byte
}
type Result struct {
	Compatibility      bool
	CompatibilityBytes []byte
	Plan               Plan
	Index              Index
	Chunks             []Chunk
}

// CompatibilityForTargetCount decides the routing branch from authoritative
// discovered target cardinality without requiring global retained fields.
func CompatibilityForTargetCount(targetCount int) (bool, error) {
	if targetCount == 0 {
		return false, ErrZeroTargets
	}
	if targetCount > MaxGlobalTargets {
		return false, ErrTooManyTargets
	}
	return targetCount <= ChunkSize, nil
}

func Build(in Input, compatibilityBytes []byte) (Result, error) {
	n := len(in.Targets)
	compatibility, err := CompatibilityForTargetCount(n)
	if err != nil {
		return Result{}, err
	}
	if compatibility {
		return Result{Compatibility: true, CompatibilityBytes: bytes.Clone(compatibilityBytes), Chunks: []Chunk{}}, nil
	}
	workspace, err := validateInput(in)
	if err != nil {
		return Result{}, err
	}
	ordered := append([]GlobalTarget(nil), in.Targets...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CanonicalSeedV2 != ordered[j].CanonicalSeedV2 {
			return ordered[i].CanonicalSeedV2 < ordered[j].CanonicalSeedV2
		}
		return ordered[i].GlobalOrdinal < ordered[j].GlobalOrdinal
	})
	chunks := make([]Chunk, 0, (n+ChunkSize-1)/ChunkSize)
	descriptors := make([]ChunkDescriptor, 0, cap(chunks))
	for start, ordinal := 0, 0; start < n; start, ordinal = start+ChunkSize, ordinal+1 {
		end := start + ChunkSize
		if end > n {
			end = n
		}
		chunk, err := projectChunk(ordinal, ordered[start:end], in.Files, workspace)
		if err != nil {
			return Result{}, err
		}
		chunk.Descriptor.TargetStart, chunk.Descriptor.TargetCount = start, end-start
		descriptors = append(descriptors, chunk.Descriptor)
		chunks = append(chunks, chunk)
	}
	plan := Plan{SchemaVersion: PlanVersion, Completeness: "UNKNOWN", GlobalProjection: in.GlobalProjection, RequestFingerprint: in.RequestFingerprint, SessionID: in.SessionID, Generation: in.Generation, WorkspaceDigest: in.WorkspaceDigest, CanonicalOrderPolicy: in.CanonicalOrderPolicy, DuplicatePolicy: in.DuplicatePolicy, BatchTargets: in.BatchTargets, DownDepth: in.DownDepth, UpDepth: in.UpDepth, Chunks: append([]ChunkDescriptor(nil), descriptors...)}
	setPlanIdentity(&plan)
	if _, err := EncodePlan(plan); err != nil {
		return Result{}, err
	}
	index := Index{SchemaVersion: IndexVersion, Completeness: "UNKNOWN", PlanDigest: plan.LogicalDigest, Chunks: append([]ChunkDescriptor(nil), descriptors...)}
	setIndexIdentity(&index)
	if _, err := EncodeIndex(index); err != nil {
		return Result{}, err
	}
	projections := make([]Projection, len(chunks))
	sidecars := make([]Sidecar, len(chunks))
	for i := range chunks {
		projections[i] = chunks[i].Projection
		sidecars[i] = chunks[i].Sidecar
	}
	work, err := aggregateWork(in, plan, index, projections, sidecars)
	if err != nil || work > maxWorkLimit {
		return Result{}, errors.New("work limit exceeded")
	}
	return Result{Plan: plan, Index: index, Chunks: chunks}, nil
}

// Reconcile validates one independently supplied aggregate. It grants no
// authority and performs no acquisition, persistence, or publication.
func Reconcile(in Input, plan Plan, index Index, projections []Projection, sidecars []Sidecar) error {
	workspace, err := validateInput(in)
	if err != nil {
		return err
	}
	if err := validatePlan(plan); err != nil {
		return err
	}
	if err := validateIndex(index); err != nil {
		return err
	}
	if plan.GlobalProjection != in.GlobalProjection || plan.RequestFingerprint != in.RequestFingerprint || plan.SessionID != in.SessionID || plan.Generation != in.Generation || plan.WorkspaceDigest != in.WorkspaceDigest || plan.CanonicalOrderPolicy != in.CanonicalOrderPolicy || plan.DuplicatePolicy != in.DuplicatePolicy || plan.BatchTargets != in.BatchTargets || plan.DownDepth != in.DownDepth || plan.UpDepth != in.UpDepth {
		return errors.New("plan global binding mismatch")
	}
	if index.PlanDigest != plan.LogicalDigest || !reflect.DeepEqual(index.Chunks, plan.Chunks) {
		return errors.New("index plan binding mismatch")
	}
	if len(projections) != len(plan.Chunks) || len(sidecars) != len(plan.Chunks) {
		return errors.New("chunk artifact count mismatch")
	}
	ordered := append([]GlobalTarget(nil), in.Targets...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CanonicalSeedV2 != ordered[j].CanonicalSeedV2 {
			return ordered[i].CanonicalSeedV2 < ordered[j].CanonicalSeedV2
		}
		return ordered[i].GlobalOrdinal < ordered[j].GlobalOrdinal
	})
	seenGlobal := make(map[int]bool, len(ordered))
	for i, descriptor := range plan.Chunks {
		projection, sidecar := projections[i], sidecars[i]
		if err := validateProjection(projection); err != nil {
			return err
		}
		if err := validateSidecar(sidecar); err != nil {
			return err
		}
		if projection.ChunkOrdinal != i || sidecar.ChunkOrdinal != i || len(projection.Targets) != descriptor.TargetCount || len(sidecar.Mappings) != descriptor.TargetCount {
			return errors.New("chunk ordinal or cardinality mismatch")
		}
		projectionBytes, err := EncodeProjection(projection)
		if err != nil || artifactRef("census-chunk-projections/v1/", projectionBytes) != descriptor.Projection {
			return errors.New("projection artifact binding mismatch")
		}
		sidecarBytes, err := EncodeSidecar(sidecar)
		if err != nil || artifactRef("census-chunk-sidecars/v1/", sidecarBytes) != descriptor.Sidecar {
			return errors.New("sidecar artifact binding mismatch")
		}
		if descriptor.TargetStart < 0 || descriptor.TargetCount < 0 {
			return errors.New("chunk range mismatch")
		}
		end, ok := checkedAdd(uint64(descriptor.TargetStart), uint64(descriptor.TargetCount))
		if !ok || end > uint64(len(ordered)) {
			return errors.New("chunk range mismatch")
		}
		expected, err := projectChunk(i, ordered[descriptor.TargetStart:int(end)], in.Files, workspace)
		if err != nil || !reflect.DeepEqual(projection, expected.Projection) || !reflect.DeepEqual(sidecar, expected.Sidecar) {
			return errors.New("chunk projection reconciliation mismatch")
		}
		for j, mapping := range sidecar.Mappings {
			global := ordered[descriptor.TargetStart+j]
			if seenGlobal[mapping.GlobalOrdinal] || mapping.GlobalOrdinal != global.GlobalOrdinal || mapping.GlobalIdentity != global.GlobalIdentity || mapping.GlobalCanonicalSeedDigest != global.CanonicalSeedV2SHA256 || mapping.GlobalFileOrdinal != global.GlobalFileOrdinal || mapping.Coordinate != global.Coordinate {
				return errors.New("global target concatenation mismatch")
			}
			seenGlobal[mapping.GlobalOrdinal] = true
		}
	}
	if len(seenGlobal) != len(ordered) {
		return errors.New("global target omission")
	}
	work, err := aggregateWork(in, plan, index, projections, sidecars)
	if err != nil || work > maxWorkLimit {
		return errors.New("work limit exceeded")
	}
	return nil
}

// aggregateWork counts retained canonical artifact bytes, global identity input
// bytes, and every global or chunk-local ledger element. Input has no retained
// canonical encoding, and localized identities are deterministic projections of
// the already-counted global identity inputs, so neither is double-counted.
func aggregateWork(in Input, plan Plan, index Index, projections []Projection, sidecars []Sidecar) (uint64, error) {
	work := uint64(0)
	add := func(n uint64) bool {
		var ok bool
		work, ok = checkedAdd(work, n)
		return ok
	}
	planBytes, err := EncodePlan(plan)
	if err != nil || !add(uint64(len(planBytes))) {
		return 0, errors.New("work overflow")
	}
	indexBytes, err := EncodeIndex(index)
	if err != nil || !add(uint64(len(indexBytes))) {
		return 0, errors.New("work overflow")
	}
	if !add(uint64(len(in.Files))) || !add(uint64(len(in.Targets))) || !add(uint64(len(plan.Chunks))) || !add(uint64(len(index.Chunks))) {
		return 0, errors.New("work overflow")
	}
	for _, target := range in.Targets {
		_, size, err := censusenumerationprojection.SymbolIdentityDigest(target.Coordinate.Path, censusenumerationprojection.Position{Line: target.Coordinate.Line, Character: target.Coordinate.Character}, target.Kind, "", target.GlobalOrdinal)
		if err != nil || !add(uint64(size)) {
			return 0, errors.New("work overflow")
		}
	}
	for _, projection := range projections {
		raw, err := EncodeProjection(projection)
		if err != nil || !add(uint64(len(raw))) || !add(uint64(len(projection.Files))) || !add(uint64(len(projection.Symbols))) || !add(uint64(len(projection.Targets))) || !add(uint64(len(projection.ProviderFailures))) {
			return 0, errors.New("work overflow")
		}
	}
	for _, sidecar := range sidecars {
		raw, err := EncodeSidecar(sidecar)
		if err != nil || !add(uint64(len(raw))) || !add(uint64(len(sidecar.Mappings))) {
			return 0, errors.New("work overflow")
		}
	}
	return work, nil
}

func checkedAdd(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, false
	}
	return a + b, true
}

func validateInput(in Input) (string, error) {
	if !validRef(in.GlobalProjection) || !validDigest(in.RequestFingerprint) || in.SessionID == "" || in.Generation == 0 || in.WorkspaceDigest == "" || in.CanonicalOrderPolicy != "captureset-order-targets-v1" || in.DuplicatePolicy != "reject-global-ordinal-identity-seed-bytes-digest-v1" || in.BatchTargets < 1 || in.BatchTargets > captureset.MaxBatchTargets || in.DownDepth < 0 || in.DownDepth > 64 || in.UpDepth < 0 || in.UpDepth > 64 || in.Files == nil || in.Targets == nil || in.GlobalProviderFailures == nil {
		return "", errors.New("invalid global chunk input")
	}
	wantWorkspace, err := censusenumerationprojection.WorkspaceDigest(in.WorkspaceURI)
	if err != nil || wantWorkspace != in.WorkspaceDigest {
		return "", errors.New("workspace binding mismatch")
	}
	u, err := url.Parse(in.WorkspaceURI)
	if err != nil {
		return "", err
	}
	workspace := filepath.FromSlash(u.Path)
	files := map[int]GlobalFile{}
	filePaths := map[string]bool{}
	for _, f := range in.Files {
		if f.Ordinal < 0 || f.Path == "" || f.Disposition == "" {
			return "", errors.New("invalid global file")
		}
		if _, ok := files[f.Ordinal]; ok {
			return "", errors.New("duplicate global file ordinal")
		}
		if filePaths[f.Path] {
			return "", errors.New("duplicate global file path")
		}
		files[f.Ordinal] = f
		filePaths[f.Path] = true
	}
	seenOrdinal, seenIdentity, seenSeed, seenDigest := map[int]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	identityBytes := uint64(0)
	for i, t := range in.Targets {
		f, ok := files[t.GlobalFileOrdinal]
		if !ok || f.Path != t.Coordinate.Path || t.GlobalOrdinal < 0 || t.Kind < 0 || t.Disposition != "prepared" || t.Name == "" || t.URI == "" || t.LanguageID == "" {
			return "", fmt.Errorf("target %d global mapping", i)
		}
		wantIdentity, size, e := censusenumerationprojection.SymbolIdentityDigest(t.Coordinate.Path, censusenumerationprojection.Position{Line: t.Coordinate.Line, Character: t.Coordinate.Character}, t.Kind, "", t.GlobalOrdinal)
		if e != nil || wantIdentity != t.GlobalIdentity {
			return "", fmt.Errorf("target %d global identity", i)
		}
		if math.MaxUint64-identityBytes < uint64(size) {
			return "", errors.New("identity work overflow")
		}
		identityBytes += uint64(size)
		if identityBytes > MaxIdentityBytes {
			return "", errors.New("identity input limit exceeded")
		}
		if digestBytes([]byte(t.CanonicalSeedV2)) != t.CanonicalSeedV2SHA256 {
			return "", fmt.Errorf("target %d seed digest", i)
		}
		if seenOrdinal[t.GlobalOrdinal] || seenIdentity[t.GlobalIdentity] || seenSeed[t.CanonicalSeedV2] || seenDigest[t.CanonicalSeedV2SHA256] {
			return "", fmt.Errorf("target %d duplicate global identity", i)
		}
		if err := validateGlobalSeed(t, workspace); err != nil {
			return "", fmt.Errorf("target %d: %w", i, err)
		}
		seenOrdinal[t.GlobalOrdinal], seenIdentity[t.GlobalIdentity], seenSeed[t.CanonicalSeedV2], seenDigest[t.CanonicalSeedV2SHA256] = true, true, true, true
	}
	return workspace, nil
}

func validateGlobalSeed(t GlobalTarget, workspace string) error {
	f, err := seedformat.Decode([]byte(t.CanonicalSeedV2), workspace)
	if err != nil || len(f.Seeds) != 1 || f.Seeds[0].Type != seedformat.PositionType || f.Seeds[0].Position == nil {
		return errors.New("invalid global canonical seed")
	}
	b, err := seedformat.EncodeCanonical(f, workspace)
	if err != nil || !bytes.Equal(b, []byte(t.CanonicalSeedV2)) {
		return errors.New("noncanonical global seed")
	}
	p := f.Seeds[0].Position
	if p.Label != fmt.Sprintf("census-%06d", t.GlobalOrdinal) || p.Path != t.Coordinate.Path || p.Line != uint64(t.Coordinate.Line)+1 || p.Column != uint64(t.Coordinate.Character)+1 {
		return errors.New("global seed reconciliation mismatch")
	}
	return nil
}

func projectChunk(ordinal int, targets []GlobalTarget, globalFiles []GlobalFile, workspace string) (Chunk, error) {
	fileByOrdinal := map[int]GlobalFile{}
	for _, f := range globalFiles {
		fileByOrdinal[f.Ordinal] = f
	}
	used := map[int]bool{}
	for _, t := range targets {
		used[t.GlobalFileOrdinal] = true
	}
	globalOrdinals := make([]int, 0, len(used))
	for o := range used {
		globalOrdinals = append(globalOrdinals, o)
	}
	sort.Ints(globalOrdinals)
	files := make([]LocalFile, len(globalOrdinals))
	localFile := map[int]int{}
	for i, o := range globalOrdinals {
		f := fileByOrdinal[o]
		files[i] = LocalFile{Ordinal: i, GlobalOrdinal: o, Path: f.Path, Disposition: f.Disposition}
		localFile[o] = i
	}
	p := Projection{SchemaVersion: ProjectionVersion, Completeness: "UNKNOWN", ProjectionStatus: "COMPLETE", ChunkOrdinal: ordinal, Files: files, Symbols: make([]LocalSymbol, len(targets)), Targets: make([]LocalTarget, len(targets)), ProviderFailures: []string{}}
	s := Sidecar{SchemaVersion: SidecarVersion, Completeness: "UNKNOWN", ChunkOrdinal: ordinal, Mappings: make([]Mapping, len(targets))}
	for j, t := range targets {
		identity, _, err := censusenumerationprojection.SymbolIdentityDigest(t.Coordinate.Path, censusenumerationprojection.Position{Line: t.Coordinate.Line, Character: t.Coordinate.Character}, t.Kind, "", j)
		if err != nil {
			return Chunk{}, err
		}
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", j), Path: t.Coordinate.Path, Line: uint64(t.Coordinate.Line) + 1, Column: uint64(t.Coordinate.Character) + 1}}}}, workspace)
		if err != nil {
			return Chunk{}, err
		}
		p.Symbols[j] = LocalSymbol{Ordinal: j, FileOrdinal: localFile[t.GlobalFileOrdinal], IdentityDigest: identity, Disposition: "prepared", Coordinate: t.Coordinate, Kind: t.Kind}
		p.Targets[j] = LocalTarget{CensusOrdinal: j, CanonicalSeedV2: string(seed), CanonicalSeedV2SHA256: digestBytes(seed), SymbolIdentity: identity}
		s.Mappings[j] = Mapping{ChunkOrdinal: ordinal, LocalOrdinal: j, GlobalOrdinal: t.GlobalOrdinal, GlobalIdentity: t.GlobalIdentity, GlobalCanonicalSeedDigest: t.CanonicalSeedV2SHA256, GlobalFileOrdinal: t.GlobalFileOrdinal, Coordinate: t.Coordinate}
	}
	setProjectionIdentity(&p)
	setSidecarIdentity(&s)
	pb, err := EncodeProjection(p)
	if err != nil {
		return Chunk{}, err
	}
	sb, err := EncodeSidecar(s)
	if err != nil {
		return Chunk{}, err
	}
	d := ChunkDescriptor{Ordinal: ordinal, Projection: artifactRef("census-chunk-projections/v1/", pb), Sidecar: artifactRef("census-chunk-sidecars/v1/", sb)}
	return Chunk{Descriptor: d, Projection: p, Sidecar: s, ProjectionBytes: pb, SidecarBytes: sb}, nil
}

func EncodePlan(v Plan) ([]byte, error) {
	if v.LogicalDigest == "" && v.ImmutableSelector == "" {
		setPlanIdentity(&v)
	}
	return encodeCanonical(v, validatePlan)
}
func DecodePlan(raw []byte) (Plan, error) { return decodeCanonical(raw, validatePlan) }
func EncodeIndex(v Index) ([]byte, error) {
	if v.LogicalDigest == "" && v.ImmutableSelector == "" {
		setIndexIdentity(&v)
	}
	return encodeCanonical(v, validateIndex)
}
func DecodeIndex(raw []byte) (Index, error) { return decodeCanonical(raw, validateIndex) }
func EncodeProjection(v Projection) ([]byte, error) {
	if v.LogicalDigest == "" && v.ImmutableSelector == "" {
		setProjectionIdentity(&v)
	}
	return encodeCanonical(v, validateProjection)
}
func DecodeProjection(raw []byte) (Projection, error) {
	return decodeCanonical(raw, validateProjection)
}
func EncodeSidecar(v Sidecar) ([]byte, error) {
	if v.LogicalDigest == "" && v.ImmutableSelector == "" {
		setSidecarIdentity(&v)
	}
	return encodeCanonical(v, validateSidecar)
}
func DecodeSidecar(raw []byte) (Sidecar, error) { return decodeCanonical(raw, validateSidecar) }

func encodeCanonical[T any](v T, validate func(T) error) ([]byte, error) {
	if err := validate(v); err != nil {
		return nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(b) > MaxCanonicalBytes {
		return nil, errors.New("canonical byte limit exceeded")
	}
	return b, nil
}
func decodeCanonical[T any](raw []byte, validate func(T) error) (T, error) {
	var zero T
	if len(raw) == 0 || len(raw) > MaxCanonicalBytes || strictjson.RejectDuplicates(raw) != nil {
		return zero, errors.New("invalid canonical JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var v T
	if d.Decode(&v) != nil {
		return zero, errors.New("invalid canonical JSON")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, errors.New("trailing canonical JSON")
	}
	if err := validate(v); err != nil {
		return zero, err
	}
	canonical, _ := json.Marshal(v)
	if !bytes.Equal(raw, canonical) {
		return zero, errors.New("noncanonical JSON")
	}
	return v, nil
}

func validatePlan(p Plan) error {
	if p.SchemaVersion != PlanVersion || p.Accepted || p.Authority != 0 || p.Completeness != "UNKNOWN" || !validRef(p.GlobalProjection) || !validDigest(p.RequestFingerprint) || p.SessionID == "" || p.Generation == 0 || !validDigest(p.WorkspaceDigest) || p.CanonicalOrderPolicy != "captureset-order-targets-v1" || p.DuplicatePolicy != "reject-global-ordinal-identity-seed-bytes-digest-v1" || p.BatchTargets < 1 || p.BatchTargets > captureset.MaxBatchTargets || p.DownDepth < 0 || p.DownDepth > 64 || p.UpDepth < 0 || p.UpDepth > 64 || p.Chunks == nil {
		return errors.New("invalid chunk plan")
	}
	total := uint64(0)
	for i, c := range p.Chunks {
		if c.Ordinal != i || c.TargetStart < 0 || uint64(c.TargetStart) != total || c.TargetCount < 1 || c.TargetCount > ChunkSize || !validRef(c.Projection) || !validRef(c.Sidecar) {
			return errors.New("invalid chunk descriptor")
		}
		if i < len(p.Chunks)-1 && c.TargetCount != ChunkSize {
			return errors.New("non-maximal chunk")
		}
		var ok bool
		total, ok = checkedAdd(total, uint64(c.TargetCount))
		if !ok {
			return errors.New("chunk plan target accounting overflow")
		}
	}
	if total <= uint64(ChunkSize) || total > uint64(MaxGlobalTargets) {
		return errors.New("chunk plan target accounting")
	}
	return validateIdentity(p.LogicalDigest, p.ImmutableSelector, "census-chunk-plan:", planDomain, p, func(v *Plan) { v.LogicalDigest = ""; v.ImmutableSelector = "" })
}
func validateIndex(v Index) error {
	if v.SchemaVersion != IndexVersion || v.Accepted || v.Authority != 0 || v.Completeness != "UNKNOWN" || !validDigest(v.PlanDigest) || v.Chunks == nil {
		return errors.New("invalid chunk index")
	}
	total := uint64(0)
	for i, c := range v.Chunks {
		if c.Ordinal != i || c.TargetStart < 0 || uint64(c.TargetStart) != total || c.TargetCount < 1 || c.TargetCount > ChunkSize || !validRef(c.Projection) || !validRef(c.Sidecar) {
			return errors.New("invalid chunk index descriptor")
		}
		if i < len(v.Chunks)-1 && c.TargetCount != ChunkSize {
			return errors.New("non-maximal chunk index")
		}
		var ok bool
		total, ok = checkedAdd(total, uint64(c.TargetCount))
		if !ok {
			return errors.New("chunk index target accounting overflow")
		}
	}
	if total <= uint64(ChunkSize) || total > uint64(MaxGlobalTargets) {
		return errors.New("chunk index target accounting")
	}
	return validateIdentity(v.LogicalDigest, v.ImmutableSelector, "census-chunk-index:", indexDomain, v, func(x *Index) { x.LogicalDigest = ""; x.ImmutableSelector = "" })
}
func validateProjection(p Projection) error {
	if p.SchemaVersion != ProjectionVersion || p.Accepted || p.Authority != 0 || p.Completeness != "UNKNOWN" || p.ProjectionStatus != "COMPLETE" || p.ChunkOrdinal < 0 || p.Files == nil || p.Symbols == nil || p.Targets == nil || p.ProviderFailures == nil || len(p.ProviderFailures) != 0 || len(p.Targets) < 1 || len(p.Targets) > ChunkSize || len(p.Symbols) != len(p.Targets) {
		return errors.New("invalid chunk projection")
	}
	for i, f := range p.Files {
		if f.Ordinal != i || f.GlobalOrdinal < 0 || f.Path == "" || f.Disposition == "" || (i > 0 && p.Files[i-1].GlobalOrdinal >= f.GlobalOrdinal) {
			return errors.New("invalid local file ledger")
		}
	}
	used := map[int]bool{}
	for i, s := range p.Symbols {
		if s.Ordinal != i || s.FileOrdinal < 0 || s.FileOrdinal >= len(p.Files) || s.Disposition != "prepared" || s.Coordinate.Path != p.Files[s.FileOrdinal].Path {
			return errors.New("invalid local symbol ledger")
		}
		want, _, e := censusenumerationprojection.SymbolIdentityDigest(s.Coordinate.Path, censusenumerationprojection.Position{Line: s.Coordinate.Line, Character: s.Coordinate.Character}, s.Kind, "", i)
		if e != nil || want != s.IdentityDigest {
			return errors.New("local symbol identity mismatch")
		}
		t := p.Targets[i]
		if t.CensusOrdinal != i || t.SymbolIdentity != want || digestBytes([]byte(t.CanonicalSeedV2)) != t.CanonicalSeedV2SHA256 {
			return errors.New("invalid local target")
		}
		if err := validateLocalSeed(t, s, i); err != nil {
			return err
		}
		used[s.FileOrdinal] = true
	}
	if len(used) != len(p.Files) {
		return errors.New("local file omission")
	}
	return validateIdentity(p.LogicalDigest, p.ImmutableSelector, "census-chunk-projection:", projectionDomain, p, func(v *Projection) { v.LogicalDigest = ""; v.ImmutableSelector = "" })
}

func validateLocalSeed(t LocalTarget, s LocalSymbol, ordinal int) error {
	workspace := string(filepath.Separator)
	f, err := seedformat.Decode([]byte(t.CanonicalSeedV2), workspace)
	if err != nil || len(f.Seeds) != 1 || f.Seeds[0].Type != seedformat.PositionType || f.Seeds[0].Position == nil {
		return errors.New("invalid local canonical seed")
	}
	canonical, err := seedformat.EncodeCanonical(f, workspace)
	if err != nil || !bytes.Equal(canonical, []byte(t.CanonicalSeedV2)) {
		return errors.New("noncanonical local seed")
	}
	position := f.Seeds[0].Position
	if position.Label != fmt.Sprintf("census-%06d", ordinal) || position.Path != s.Coordinate.Path || position.Line != uint64(s.Coordinate.Line)+1 || position.Column != uint64(s.Coordinate.Character)+1 {
		return errors.New("local seed reconciliation mismatch")
	}
	return nil
}

func validateSidecar(s Sidecar) error {
	if s.SchemaVersion != SidecarVersion || s.Accepted || s.Authority != 0 || s.Completeness != "UNKNOWN" || s.ChunkOrdinal < 0 || s.Mappings == nil || len(s.Mappings) < 1 || len(s.Mappings) > ChunkSize {
		return errors.New("invalid chunk sidecar")
	}
	ord, ident, dig := map[int]bool{}, map[string]bool{}, map[string]bool{}
	for i, m := range s.Mappings {
		if m.ChunkOrdinal != s.ChunkOrdinal || m.LocalOrdinal != i || m.GlobalOrdinal < 0 || m.GlobalFileOrdinal < 0 || m.Coordinate.Path == "" || !validDigest(m.GlobalIdentity) || !validDigest(m.GlobalCanonicalSeedDigest) || ord[m.GlobalOrdinal] || ident[m.GlobalIdentity] || dig[m.GlobalCanonicalSeedDigest] {
			return errors.New("invalid sidecar mapping")
		}
		ord[m.GlobalOrdinal], ident[m.GlobalIdentity], dig[m.GlobalCanonicalSeedDigest] = true, true, true
	}
	return validateIdentity(s.LogicalDigest, s.ImmutableSelector, "census-chunk-sidecar:", sidecarDomain, s, func(v *Sidecar) { v.LogicalDigest = ""; v.ImmutableSelector = "" })
}

func setPlanIdentity(v *Plan) {
	setIdentity(planDomain, "census-chunk-plan:", v, func(x *Plan, d, s string) { x.LogicalDigest = d; x.ImmutableSelector = s })
}
func setIndexIdentity(v *Index) {
	setIdentity(indexDomain, "census-chunk-index:", v, func(x *Index, d, s string) { x.LogicalDigest = d; x.ImmutableSelector = s })
}
func setProjectionIdentity(v *Projection) {
	setIdentity(projectionDomain, "census-chunk-projection:", v, func(x *Projection, d, s string) { x.LogicalDigest = d; x.ImmutableSelector = s })
}
func setSidecarIdentity(v *Sidecar) {
	setIdentity(sidecarDomain, "census-chunk-sidecar:", v, func(x *Sidecar, d, s string) { x.LogicalDigest = d; x.ImmutableSelector = s })
}
func setIdentity[T any](domain, prefix string, v *T, set func(*T, string, string)) {
	set(v, "", "")
	b, _ := json.Marshal(v)
	d := domainDigest(domain, b)
	set(v, d, prefix+strings.TrimPrefix(d, "sha256:"))
}
func validateIdentity[T any](digest, selector, prefix, domain string, v T, clear func(*T)) error {
	clear(&v)
	b, _ := json.Marshal(v)
	want := domainDigest(domain, b)
	if digest != want || selector != prefix+strings.TrimPrefix(want, "sha256:") {
		return errors.New("logical identity mutation")
	}
	return nil
}
func artifactRef(prefix string, b []byte) ArtifactRef {
	return ArtifactRef{Selector: prefix + strings.TrimPrefix(digestBytes(b), "sha256:") + ".json", SHA256: digestBytes(b), ByteLength: uint64(len(b))}
}
func validRef(r ArtifactRef) bool {
	return r.Selector != "" && validDigest(r.SHA256) && r.ByteLength > 0
}
func validDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") || strings.ToLower(s) != s {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil
}
func digestBytes(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func domainDigest(domain string, b []byte) string {
	return digestBytes(append(append([]byte(domain), 0), b...))
}
