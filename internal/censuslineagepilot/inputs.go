package censuslineagepilot

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceplan"
)

// These schemas and kinds describe only the private input prerequisite, not a
// complete Census inventory or admission to the durable/monitored pilot.
const (
	SourceSelectionSchemaVersion       = "lsp-trace.census-lineage-pilot-source-selection.private.v1"
	PartitionPlanSchemaVersion         = "lsp-trace.census-lineage-pilot-partition-plan.private.v1"
	SourceSelectionKind                = "SOURCE_SELECTION"
	PartitionPlanKind                  = "PARTITION_PLAN"
	MaxSelectionInputBytes       int64 = 4 << 20
	MaxPlanInputBytes            int64 = 16 << 20
	MaxInputFiles                      = 10000
)

// SourceIdentity is an explicit private wire DTO, independent of SourceFile's
// Go serialization. Disposition retains the source-plan policy verbatim.
type SourceIdentity struct {
	Path        string `json:"path"`
	URI         string `json:"uri"`
	LanguageID  string `json:"language_id"`
	Disposition string `json:"disposition"`
}
type SourceSelection struct {
	SchemaVersion string           `json:"schema_version"`
	Authority     uint8            `json:"authority"`
	Accepted      bool             `json:"accepted"`
	Workspace     string           `json:"workspace"`
	Files         []SourceIdentity `json:"files"`
}
type InputPartition struct {
	Ordinal int              `json:"ordinal"`
	Members []SourceIdentity `json:"members"`
}
type InputSourcePlan struct {
	Identity   string           `json:"identity"`
	Authority  uint8            `json:"authority"`
	Files      []SourceIdentity `json:"files"`
	Partitions []InputPartition `json:"partitions"`
}
type PartitionPlan struct {
	SchemaVersion             string          `json:"schema_version"`
	Authority                 uint8           `json:"authority"`
	Accepted                  bool            `json:"accepted"`
	SourceSelectionSHA256     string          `json:"source_selection_sha256"`
	SourceSelectionByteLength uint64          `json:"source_selection_byte_length"`
	PartitionSize             int             `json:"partition_size"`
	Plan                      InputSourcePlan `json:"plan"`
}

func identities(files []censusacquisition.SourceFile) []SourceIdentity {
	out := make([]SourceIdentity, len(files))
	for i, f := range files {
		out[i] = SourceIdentity{f.Path, f.URI, f.LanguageID, string(f.Disposition)}
	}
	return out
}
func sourceFiles(files []SourceIdentity) []censusacquisition.SourceFile {
	out := make([]censusacquisition.SourceFile, len(files))
	for i, f := range files {
		out[i] = censusacquisition.SourceFile{Path: f.Path, URI: f.URI, LanguageID: f.LanguageID, Disposition: census.FileDisposition(f.Disposition)}
	}
	return out
}
func planDTO(p censussourceplan.Plan) InputSourcePlan {
	out := InputSourcePlan{Identity: p.Identity, Authority: p.Authority, Files: identities(p.Files), Partitions: make([]InputPartition, len(p.Partitions))}
	for i, part := range p.Partitions {
		out.Partitions[i] = InputPartition{Ordinal: part.Ordinal, Members: identities(part.Members)}
	}
	return out
}

// SourcePlan returns a fresh reconstruction of the DTO, not a trusted admission.
func (p PartitionPlan) SourcePlan() censussourceplan.Plan {
	out := censussourceplan.Plan{Identity: p.Plan.Identity, Authority: p.Plan.Authority, Files: sourceFiles(p.Plan.Files), Partitions: make([]censussourceplan.Partition, len(p.Plan.Partitions))}
	for i, part := range p.Plan.Partitions {
		out.Partitions[i] = censussourceplan.Partition{Ordinal: part.Ordinal, Members: sourceFiles(part.Members)}
	}
	return out
}
func NewSourceSelection(workspace string, files []censusacquisition.SourceFile) (SourceSelection, error) {
	if len(files) > MaxInputFiles {
		return SourceSelection{}, ErrInvalid
	}
	built, err := censussourceplan.Build(files, workspace, 1)
	if err != nil {
		return SourceSelection{}, ErrInvalid
	}
	s := SourceSelection{SchemaVersion: SourceSelectionSchemaVersion, Workspace: workspace, Files: identities(built.Files)}
	if _, err = EncodeSourceSelection(s); err != nil {
		return SourceSelection{}, err
	}
	return s, nil
}
func validateSelection(s SourceSelection) error {
	if s.SchemaVersion != SourceSelectionSchemaVersion || s.Authority != 0 || s.Accepted || len(s.Files) == 0 || len(s.Files) > MaxInputFiles {
		return ErrInvalid
	}
	built, err := censussourceplan.Build(sourceFiles(s.Files), s.Workspace, 1)
	if err != nil || !reflect.DeepEqual(identities(built.Files), s.Files) {
		return ErrInvalid
	}
	return nil
}
func EncodeSourceSelection(s SourceSelection) ([]byte, error) {
	if validateSelection(s) != nil {
		return nil, ErrInvalid
	}
	raw, err := json.Marshal(s)
	if err != nil || int64(len(raw)) > MaxSelectionInputBytes {
		return nil, ErrInvalid
	}
	return raw, nil
}
func ParseSourceSelection(raw []byte) (SourceSelection, error) {
	if int64(len(raw)) > MaxSelectionInputBytes {
		return SourceSelection{}, ErrInvalid
	}
	var s SourceSelection
	if strict(raw, &s) != nil {
		return SourceSelection{}, ErrInvalid
	}
	canonical, err := EncodeSourceSelection(s)
	if err != nil || !bytes.Equal(raw, canonical) {
		return SourceSelection{}, ErrInvalid
	}
	return s, nil
}
func NewPartitionPlan(s SourceSelection, size int) (PartitionPlan, error) {
	raw, err := EncodeSourceSelection(s)
	if err != nil {
		return PartitionPlan{}, err
	}
	built, err := censussourceplan.Build(sourceFiles(s.Files), s.Workspace, size)
	if err != nil {
		return PartitionPlan{}, ErrInvalid
	}
	p := PartitionPlan{SchemaVersion: PartitionPlanSchemaVersion, SourceSelectionSHA256: Digest(raw), SourceSelectionByteLength: uint64(len(raw)), PartitionSize: size, Plan: planDTO(built)}
	encoded, err := json.Marshal(p)
	if err != nil || int64(len(encoded)) > MaxPlanInputBytes {
		return PartitionPlan{}, ErrInvalid
	}
	return p, nil
}
func EncodePartitionPlan(p PartitionPlan, s SourceSelection) ([]byte, error) {
	want, err := NewPartitionPlan(s, p.PartitionSize)
	if err != nil || !reflect.DeepEqual(p, want) {
		return nil, ErrInvalid
	}
	return json.Marshal(p)
}
func ParsePartitionPlan(raw []byte, s SourceSelection) (PartitionPlan, error) {
	if int64(len(raw)) > MaxPlanInputBytes {
		return PartitionPlan{}, ErrInvalid
	}
	var p PartitionPlan
	if strict(raw, &p) != nil {
		return PartitionPlan{}, ErrInvalid
	}
	canonical, err := EncodePartitionPlan(p, s)
	if err != nil || !bytes.Equal(raw, canonical) {
		return PartitionPlan{}, ErrInvalid
	}
	return p, nil
}

// InputSource opens only the two input kinds for read-only observation. This
// interface grants no store/head/write operation. Implementations own transport
// safety (e.g. no-follow for filesystem opens); this unit claims no filesystem
// ownership or monitoring completeness. Readers must obey io.Reader's contract.
type InputSource interface {
	OpenInput(kind string) (io.ReadCloser, error)
}

// InputLimits must be explicit, positive, and no larger than the private ceilings.
type InputLimits struct {
	SelectionBytes, PlanBytes int64
	Files                     int
}

func DefaultInputLimits() InputLimits {
	return InputLimits{MaxSelectionInputBytes, MaxPlanInputBytes, MaxInputFiles}
}
func (l InputLimits) valid() bool {
	return l.SelectionBytes > 0 && l.SelectionBytes <= MaxSelectionInputBytes && l.PlanBytes > 0 && l.PlanBytes <= MaxPlanInputBytes && l.Files > 0 && l.Files <= MaxInputFiles
}
func observeInput(source InputSource, kind string, limit int64) ([]byte, error) {
	r, err := source.OpenInput(kind)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrInvalid
	}
	raw, readErr := io.ReadAll(io.LimitReader(r, limit+1))
	closeErr := r.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(raw) == 0 || int64(len(raw)) > limit {
		return nil, ErrInvalid
	}
	return bytes.Clone(raw), nil
}

type InputLinkage struct {
	SourceSelectionSHA256     string
	SourceSelectionByteLength uint64
	PartitionPlanSHA256       string
	PartitionPlanByteLength   uint64
}

// ObservedInputs has no caller-set trust flag. Only ObserveInputs populates its
// private state; its zero value contains no validated inputs. It is not a seal,
// monitoring qualification, runtime admission, or completeness assertion.
type ObservedInputs struct{ state *observedInputState }
type observedInputState struct {
	selection, plan []byte
	artifacts       []Artifact
	linkage         InputLinkage
}

func ObserveInputs(source InputSource, limits InputLimits, expected []Artifact) (ObservedInputs, error) {
	if source == nil || !limits.valid() {
		return ObservedInputs{}, ErrInvalid
	}
	// Expectations are comparison-only and admit exactly the finite input kinds.
	if expected != nil {
		if len(expected) != 2 {
			return ObservedInputs{}, ErrInvalid
		}
		seen := map[string]bool{}
		for _, a := range expected {
			if (a.Kind != SourceSelectionKind && a.Kind != PartitionPlanKind) || seen[a.Kind] || !validDigest(a.SHA256) || a.Selector != a.SHA256 || a.ByteLength == 0 {
				return ObservedInputs{}, ErrInvalid
			}
			seen[a.Kind] = true
		}
	}
	sb, err := observeInput(source, SourceSelectionKind, limits.SelectionBytes)
	if err != nil {
		return ObservedInputs{}, err
	}
	s, err := ParseSourceSelection(sb)
	if err != nil || len(s.Files) > limits.Files {
		return ObservedInputs{}, ErrInvalid
	}
	pb, err := observeInput(source, PartitionPlanKind, limits.PlanBytes)
	if err != nil {
		return ObservedInputs{}, err
	}
	if _, err = ParsePartitionPlan(pb, s); err != nil {
		return ObservedInputs{}, err
	}
	sa := Artifact{Kind: SourceSelectionKind, Selector: Digest(sb), SHA256: Digest(sb), ByteLength: uint64(len(sb))}
	pa := Artifact{Kind: PartitionPlanKind, Selector: Digest(pb), SHA256: Digest(pb), ByteLength: uint64(len(pb))}
	artifacts := SortArtifactsForConstruction([]Artifact{sa, pa})
	if expected != nil && !reflect.DeepEqual(SortArtifactsForConstruction(expected), artifacts) {
		return ObservedInputs{}, ErrInvalid
	}
	return ObservedInputs{&observedInputState{selection: sb, plan: pb, artifacts: artifacts, linkage: InputLinkage{sa.SHA256, sa.ByteLength, pa.SHA256, pa.ByteLength}}}, nil
}
func (in ObservedInputs) SelectionBytes() []byte {
	if in.state == nil {
		return nil
	}
	return bytes.Clone(in.state.selection)
}
func (in ObservedInputs) PlanBytes() []byte {
	if in.state == nil {
		return nil
	}
	return bytes.Clone(in.state.plan)
}
func (in ObservedInputs) Selection() SourceSelection {
	if in.state == nil {
		return SourceSelection{}
	}
	s, _ := ParseSourceSelection(in.state.selection)
	return s
}
func (in ObservedInputs) PartitionPlan() PartitionPlan {
	if in.state == nil {
		return PartitionPlan{}
	}
	p, _ := ParsePartitionPlan(in.state.plan, in.Selection())
	return p
}
func (in ObservedInputs) Artifacts() []Artifact {
	if in.state == nil {
		return nil
	}
	return append([]Artifact(nil), in.state.artifacts...)
}
func (in ObservedInputs) Linkage() InputLinkage {
	if in.state == nil {
		return InputLinkage{}
	}
	return in.state.linkage
}
