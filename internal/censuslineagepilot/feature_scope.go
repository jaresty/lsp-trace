package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"regexp"
	"unicode/utf8"

	"lsp-trace/internal/censusacquisition"
)

const featureScopeFileLimit = 2 << 20
const featureScopeTotalLimit = 8 << 20
const featureScopeSchema = "lsp-trace.feature-scope.private.v1"
const featureScopeDomain = "lsp-trace:feature-scope:private:v1"

type scopeFileIdentity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}
type scopeManifestFile struct {
	Path        string            `json:"path"`
	Role        string            `json:"role"`
	URI         string            `json:"uri"`
	SHA256      string            `json:"sha256"`
	ByteLength  uint64            `json:"byte_length"`
	Identity    scopeFileIdentity `json:"identity"`
	Mode        uint32            `json:"mode"`
	ModifiedNS  int64             `json:"modified_ns"`
	ChangedSec  int64             `json:"changed_sec"`
	ChangedNsec int64             `json:"changed_nsec"`
}
type scopeObservedFile struct {
	scopeManifestFile
	Bytes []byte
}
type scopeManifest struct {
	SchemaVersion         string              `json:"schema_version"`
	Domain                string              `json:"domain"`
	Authority             uint8               `json:"authority"`
	Accepted              bool                `json:"accepted"`
	Completeness          string              `json:"completeness"`
	SourceGraphComplete   string              `json:"source_graph_complete"`
	FeatureIdentity       string              `json:"feature_identity"`
	RevisionAssertion     string              `json:"revision_assertion"`
	RevisionQualification string              `json:"revision_qualification"`
	IncludedRule          string              `json:"included_rule"`
	ExcludedRule          string              `json:"excluded_rule"`
	Root                  string              `json:"root"`
	RootIdentity          scopeFileIdentity   `json:"root_identity"`
	Files                 []scopeManifestFile `json:"files"`
	PartitionSize         int                 `json:"partition_size"`
	SelectionSHA256       string              `json:"selection_sha256"`
	SelectionByteLength   uint64              `json:"selection_byte_length"`
	PlanSHA256            string              `json:"plan_sha256"`
	PlanByteLength        uint64              `json:"plan_byte_length"`
}
type featureScope struct{ state *featureScopeState }
type featureScopeState struct {
	files                     []scopeObservedFile
	manifest, selection, plan []byte
}

func (b featureScope) Files() []scopeObservedFile {
	if b.state == nil {
		return nil
	}
	out := append([]scopeObservedFile(nil), b.state.files...)
	for i := range out {
		out[i].Bytes = bytes.Clone(out[i].Bytes)
	}
	return out
}
func (b featureScope) ManifestBytes() []byte {
	if b.state == nil {
		return nil
	}
	return bytes.Clone(b.state.manifest)
}
func (b featureScope) SelectionBytes() []byte {
	if b.state == nil {
		return nil
	}
	return bytes.Clone(b.state.selection)
}
func (b featureScope) PlanBytes() []byte {
	if b.state == nil {
		return nil
	}
	return bytes.Clone(b.state.plan)
}
func (b featureScope) ManifestDigest() string {
	if b.state == nil {
		return ""
	}
	return Digest(append([]byte(featureScopeDomain+"\x00"), b.state.manifest...))
}
func (b featureScope) Selection() SourceSelection {
	if b.state == nil {
		return SourceSelection{}
	}
	s, _ := ParseSourceSelection(b.state.selection)
	return s
}
func (b featureScope) PartitionPlan() PartitionPlan {
	if b.state == nil {
		return PartitionPlan{}
	}
	p, _ := ParsePartitionPlan(b.state.plan, b.Selection())
	return p
}
func canonicalFeatureScope(m scopeManifest) ([]byte, error) { return json.Marshal(m) }

// Fixed policy, not mutable selectors or public Census defaults. Context is never
// a nomination source; all unlisted source paths remain excluded/frontier gaps.
func featureScopePaths() [6]string {
	return [6]string{"cmd/lsp-trace/filter_command.go", "cmd/lsp-trace/inspect_command.go", "internal/filter/filter.go", "internal/inspection/projection.go", "cmd/lsp-trace/main.go", "internal/schema/schema.go"}
}

var featureScopeRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)

// freezeFeatureScope prepares only in-memory inputs. Revision is comparison data,
// not authenticated Git provenance. No publication, execution, or retention occurs.
func freezeFeatureScope(ctx context.Context, root, revision string, expected []scopeObservedFile) (featureScope, error) {
	return freezeFeatureScopeWithOps(ctx, root, revision, expected, scopeReadOps{})
}
func freezeFeatureScopeWithOps(ctx context.Context, root, revision string, expected []scopeObservedFile, ops scopeReadOps) (featureScope, error) {
	if ctx == nil || !featureScopeRevision.MatchString(revision) || !utf8.ValidString(root) || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return featureScope{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return featureScope{}, err
	}
	if expected != nil && len(expected) != 6 {
		return featureScope{}, ErrInvalid
	}
	files, identity, err := readFeatureScope(ctx, root, ops)
	if err != nil {
		return featureScope{}, err
	}
	if expected != nil {
		for i, f := range files {
			p := expected[i]
			if p.Path != f.Path || p.Role != f.Role || p.SHA256 != f.SHA256 || p.ByteLength != f.ByteLength {
				return featureScope{}, ErrInvalid
			}
		}
	}
	sources := make([]censusacquisition.SourceFile, 4)
	for i := range files {
		files[i].URI = (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(root, files[i].Path))}).String()
		if i < 4 {
			sources[i] = censusacquisition.SourceFile{Path: files[i].Path, URI: files[i].URI, LanguageID: "go"}
		}
	}
	selection, err := NewSourceSelection(root, sources)
	if err != nil {
		return featureScope{}, err
	}
	sb, err := EncodeSourceSelection(selection)
	if err != nil {
		return featureScope{}, err
	}
	plan, err := NewPartitionPlan(selection, 2)
	if err != nil {
		return featureScope{}, err
	}
	pb, err := EncodePartitionPlan(plan, selection)
	if err != nil {
		return featureScope{}, err
	}
	m := scopeManifest{SchemaVersion: featureScopeSchema, Domain: featureScopeDomain, Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN", FeatureIdentity: "UNRESOLVED", RevisionAssertion: revision, RevisionQualification: "CALLER_ASSERTED_NOT_AUTHENTICATED", IncludedRule: "EXACT_FOUR_NOMINATION_TWO_CONTEXT", ExcludedRule: "ALL_OTHER_PATHS_EXCLUDED_EXTERNAL_RELATIONS_ARE_FRONTIER_GAPS", Root: root, RootIdentity: identity, Files: make([]scopeManifestFile, 6), PartitionSize: 2, SelectionSHA256: Digest(sb), SelectionByteLength: uint64(len(sb)), PlanSHA256: Digest(pb), PlanByteLength: uint64(len(pb))}
	for i := range files {
		m.Files[i] = files[i].scopeManifestFile
	}
	raw, err := canonicalFeatureScope(m)
	if err != nil {
		return featureScope{}, err
	}
	if err = ctx.Err(); err != nil {
		return featureScope{}, err
	}
	return featureScope{&featureScopeState{files: files, manifest: raw, selection: sb, plan: pb}}, nil
}
