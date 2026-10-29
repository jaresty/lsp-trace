package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
)

// Prospective PRIVATE policy, not public Census defaults or a qualification.
const scopeMetadataPrivateV2 = 128 << 10
const scopePathDepthPrivateV2 = 32

type scopeReadDescriptorPrivate struct{ path, role string }
type scopeReadBoundsPrivate struct {
	fileBytes, totalBytes, maxDepth int
	rejectAliases                   bool
}

type ScopeMemberPrivateV2 struct {
	CanonicalRelativePath string `json:"path"`
	Role                  string `json:"role"`
	LanguageID            string `json:"language_id"`
	ExpectedSHA256        string `json:"expected_sha256"`
	ByteLength            uint64 `json:"byte_length"`
}

// Every numeric bound is explicit and finite. Zero never invokes generic defaults.
// MaxContextFiles is a nonzero capacity; an actual scope may contain zero context.
type ScopeEffectivePolicyPrivateV2 struct {
	MaxNominationFiles int                               `json:"max_nomination_files"`
	MaxContextFiles    int                               `json:"max_context_files"`
	MaxMembers         int                               `json:"max_members"`
	FileBytes          int                               `json:"file_bytes"`
	TotalBytes         int                               `json:"total_bytes"`
	MaxPathDepth       int                               `json:"max_path_depth"`
	Discovery          censusacquisition.DiscoveryLimits `json:"discovery"`
	MaxTargets         int                               `json:"max_targets"`
	Planning           censusacquisition.PlanningConfig  `json:"planning"`
	MaxNodes           int                               `json:"max_nodes"`
	MaxRequests        int                               `json:"max_requests"`
	MaxEvidenceBytes   int                               `json:"max_evidence_bytes"`
	MaxPathWork        int                               `json:"max_path_work"`
	TimeoutMS          int                               `json:"timeout_ms"`
	RequestTimeoutMS   int                               `json:"request_timeout_ms"`
	MaxResponseBytes   int                               `json:"max_response_bytes"`
	MaxMessages        int                               `json:"max_messages"`
}

func (p ScopeEffectivePolicyPrivateV2) validate() error {
	bounds := [][2]int{{p.MaxNominationFiles, 8}, {p.MaxContextFiles, 8}, {p.MaxMembers, 16}, {p.FileBytes, featureScopeFileLimit}, {p.TotalBytes, featureScopeTotalLimit}, {p.MaxPathDepth, scopePathDepthPrivateV2}, {p.Discovery.MaxFiles, 8}, {p.Discovery.MaxSymbols, 256}, {p.Discovery.MaxDepth, 64}, {p.Discovery.MaxStringBytes, 1 << 20}, {p.MaxTargets, 63}, {p.Planning.MaxBatchTargets, 63}, {p.Planning.DownDepth, 8}, {p.Planning.UpDepth, 8}, {p.MaxNodes, 256}, {p.MaxRequests, scopeAdapterRequests}, {p.MaxEvidenceBytes, 4 << 20}, {p.MaxPathWork, 100000}, {p.TimeoutMS, 30000}, {p.RequestTimeoutMS, 5000}, {p.MaxResponseBytes, scopeAdapterResponse}, {p.MaxMessages, scopeAdapterMessages}}
	for _, b := range bounds {
		if b[0] < 1 || b[0] > b[1] {
			return ErrInvalid
		}
	}
	if p.Discovery.MaxFiles != p.MaxNominationFiles || p.Planning.MaxBatchTargets > p.MaxTargets || p.RequestTimeoutMS > p.TimeoutMS || p.FileBytes > p.TotalBytes {
		return ErrInvalid
	}
	return nil
}
func (p ScopeEffectivePolicyPrivateV2) acquisitionLimits() acquisitionmanifest.Limits {
	return acquisitionmanifest.Limits{MaxNodes: &p.MaxNodes, MaxRequests: &p.MaxRequests, MaxEvidenceBytes: &p.MaxEvidenceBytes, MaxPathWork: &p.MaxPathWork, TimeoutMS: &p.TimeoutMS, RequestTimeoutMS: &p.RequestTimeoutMS, MaxResponseBytes: &p.MaxResponseBytes, MaxMessages: &p.MaxMessages}
}

type ScopeSpecPrivateV2 struct {
	Root              string                        `json:"root"`
	RevisionAssertion string                        `json:"revision_assertion"`
	Members           []ScopeMemberPrivateV2        `json:"members"`
	PartitionSize     int                           `json:"partition_size"`
	EffectivePolicy   ScopeEffectivePolicyPrivateV2 `json:"effective_policy"`
}

// No parser can construct this live in-memory provenance. Copies share immutable
// state; all byte accessors detach. Revision is NOT authenticated Git provenance.
type FrozenScopePrivateV2 struct{ state *frozenScopeStatePrivateV2 }
type frozenScopeStatePrivateV2 struct {
	self                                     *frozenScopeStatePrivateV2
	discoveryMu                              sync.Mutex
	discoveryAttempted                       bool
	spec                                     ScopeSpecPrivateV2
	files                                    []scopeObservedFile
	rootIdentity                             scopeFileIdentity
	specBytes, manifest, selection, filePlan []byte
}
type scopeManifestPrivateV2 struct {
	Schema                string              `json:"schema"`
	Authority             uint8               `json:"authority"`
	Accepted              bool                `json:"accepted"`
	Feature               string              `json:"feature_identity"`
	Source                string              `json:"source_completeness"`
	Absence               string              `json:"absence_inference"`
	RevisionQualification string              `json:"revision_qualification"`
	SourceRootClaim       string              `json:"source_root_claim"`
	SpecSHA256            string              `json:"spec_sha256"`
	SelectionSHA256       string              `json:"selection_sha256"`
	FilePlanSHA256        string              `json:"file_plan_sha256"`
	RootIdentity          scopeFileIdentity   `json:"root_identity"`
	Files                 []scopeManifestFile `json:"files"`
}

func (f FrozenScopePrivateV2) valid() bool { return f.state != nil && f.state.self == f.state }
func (f FrozenScopePrivateV2) ManifestBytes() []byte {
	if !f.valid() {
		return nil
	}
	return bytes.Clone(f.state.manifest)
}
func (f FrozenScopePrivateV2) SpecBytes() []byte {
	if !f.valid() {
		return nil
	}
	return bytes.Clone(f.state.specBytes)
}
func (f FrozenScopePrivateV2) SelectionBytes() []byte {
	if !f.valid() {
		return nil
	}
	return bytes.Clone(f.state.selection)
}
func (f FrozenScopePrivateV2) FilePlanBytes() []byte {
	if !f.valid() {
		return nil
	}
	return bytes.Clone(f.state.filePlan)
}
func (FrozenScopePrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }

// Validate counts, roles, canonical strings and declared byte budgets BEFORE any
// source IO or owned-content copy. No symlink resolution or ambient normalization.
func validateScopeSpecPrivateV2(s ScopeSpecPrivateV2) ([]scopeReadDescriptorPrivate, []byte, error) {
	if s.EffectivePolicy.validate() != nil || len(s.Root) == 0 || len(s.Root) > 8192 || !utf8.ValidString(s.Root) || strings.ContainsRune(s.Root, 0) || !filepath.IsAbs(s.Root) || filepath.Clean(s.Root) != s.Root || !featureScopeRevision.MatchString(s.RevisionAssertion) || len(s.Members) < 1 || len(s.Members) > 16 || len(s.Members) > s.EffectivePolicy.MaxMembers {
		return nil, nil, ErrInvalid
	}
	if len(strings.Split(strings.Trim(s.Root, "/"), "/")) > s.EffectivePolicy.MaxPathDepth {
		return nil, nil, ErrInvalid
	}
	nom, con, total, metadata := 0, 0, uint64(0), len(s.Root)
	paths, uris := map[string]bool{}, map[string]bool{}
	descriptors := make([]scopeReadDescriptorPrivate, 0, len(s.Members))
	for _, m := range s.Members {
		rel := m.CanonicalRelativePath
		if len(rel) == 0 || len(rel) > 8192 || !utf8.ValidString(rel) || strings.ContainsAny(rel, "\\\x00") || path.IsAbs(rel) || path.Clean(rel) != rel || len(strings.Split(rel, "/")) > s.EffectivePolicy.MaxPathDepth || len(m.LanguageID) == 0 || len(m.LanguageID) > 256 || !utf8.ValidString(m.LanguageID) || strings.ContainsRune(m.LanguageID, 0) || !validDigest(m.ExpectedSHA256) || m.ByteLength < 1 || m.ByteLength > uint64(s.EffectivePolicy.FileBytes) {
			return nil, nil, ErrInvalid
		}
		for _, c := range strings.Split(rel, "/") {
			if c == "" || c == "." || c == ".." {
				return nil, nil, ErrInvalid
			}
		}
		uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(s.Root, rel))}).String()
		metadata += len(rel) + len(uri) + len(m.LanguageID) + len(m.ExpectedSHA256) + len(m.Role) + 256
		if len(uri) > 8192 || paths[rel] || uris[uri] || metadata > scopeMetadataPrivateV2/4 {
			return nil, nil, ErrInvalid
		}
		paths[rel], uris[uri] = true, true
		switch m.Role {
		case "NOMINATION":
			nom++
		case "CONTEXT_ONLY":
			con++
		default:
			return nil, nil, ErrInvalid
		}
		total += m.ByteLength
		if total > uint64(s.EffectivePolicy.TotalBytes) {
			return nil, nil, ErrInvalid
		}
		descriptors = append(descriptors, scopeReadDescriptorPrivate{rel, m.Role})
	}
	if nom < 1 || nom > s.EffectivePolicy.MaxNominationFiles || con > s.EffectivePolicy.MaxContextFiles || s.PartitionSize < 1 || s.PartitionSize > nom {
		return nil, nil, ErrInvalid
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > scopeMetadataPrivateV2/4 {
		return nil, nil, ErrInvalid
	}
	return descriptors, raw, nil
}
func FreezeScopePrivateV2(ctx context.Context, spec ScopeSpecPrivateV2) (FrozenScopePrivateV2, error) {
	return freezeScopePrivateV2(ctx, spec, scopeReadOps{})
}
func freezeScopePrivateV2(ctx context.Context, spec ScopeSpecPrivateV2, ops scopeReadOps) (FrozenScopePrivateV2, error) {
	if ctx == nil {
		return FrozenScopePrivateV2{}, ErrInvalid
	}
	descriptors, raw, err := validateScopeSpecPrivateV2(spec)
	if err != nil {
		return FrozenScopePrivateV2{}, err
	}
	// Detached DTO, including caller's ordered member slice, before source IO.
	var owned ScopeSpecPrivateV2
	if json.Unmarshal(raw, &owned) != nil {
		return FrozenScopePrivateV2{}, ErrInvalid
	}
	files, root, err := readScopeDescriptorsPrivate(ctx, owned.Root, descriptors, scopeReadBoundsPrivate{owned.EffectivePolicy.FileBytes, owned.EffectivePolicy.TotalBytes, owned.EffectivePolicy.MaxPathDepth, true}, ops)
	if err != nil {
		return FrozenScopePrivateV2{}, err
	}
	var nominations []censusacquisition.SourceFile
	for i := range files {
		m := owned.Members[i]
		if files[i].SHA256 != m.ExpectedSHA256 || files[i].ByteLength != m.ByteLength {
			return FrozenScopePrivateV2{}, ErrInvalid
		}
		files[i].URI = (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(owned.Root, files[i].Path))}).String()
		if m.Role == "NOMINATION" {
			nominations = append(nominations, censusacquisition.SourceFile{Path: files[i].Path, URI: files[i].URI, LanguageID: m.LanguageID})
		}
	}
	selection, err := NewSourceSelection(owned.Root, nominations)
	if err != nil {
		return FrozenScopePrivateV2{}, err
	}
	sb, err := EncodeSourceSelection(selection)
	if err != nil {
		return FrozenScopePrivateV2{}, err
	}
	plan, err := NewPartitionPlan(selection, owned.PartitionSize)
	if err != nil {
		return FrozenScopePrivateV2{}, err
	}
	pb, err := EncodePartitionPlan(plan, selection)
	if err != nil {
		return FrozenScopePrivateV2{}, err
	}
	m := scopeManifestPrivateV2{Schema: "lsp-trace.frozen-scope.private.v2", Feature: "UNRESOLVED", Source: "UNKNOWN", Absence: "PROHIBITED", RevisionQualification: "CALLER_ASSERTED_NOT_AUTHENTICATED", SourceRootClaim: sourceRootNoAtomicSnapshotPrivateV2, SpecSHA256: Digest(raw), SelectionSHA256: Digest(sb), FilePlanSHA256: Digest(pb), RootIdentity: root, Files: make([]scopeManifestFile, len(files))}
	for i := range files {
		m.Files[i] = files[i].scopeManifestFile
	}
	mb, err := json.Marshal(m)
	if err != nil || len(raw)+len(sb)+len(pb)+len(mb) > scopeMetadataPrivateV2 {
		return FrozenScopePrivateV2{}, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return FrozenScopePrivateV2{}, err
	}
	st := &frozenScopeStatePrivateV2{spec: owned, files: files, rootIdentity: root, specBytes: raw, manifest: mb, selection: sb, filePlan: pb}
	st.self = st
	return FrozenScopePrivateV2{st}, nil
}

const sourceRootNoAtomicSnapshotPrivateV2 = "BORROWED_NONCONCURRENT_SAME_USER_PRACTICAL_NO_ATOMIC_SNAPSHOT"

func (f FrozenScopePrivateV2) revalidate(ctx context.Context) error {
	if !f.valid() || ctx == nil {
		return ErrInvalid
	}
	s := f.state
	d := make([]scopeReadDescriptorPrivate, len(s.files))
	for i, v := range s.files {
		d[i] = scopeReadDescriptorPrivate{v.Path, v.Role}
	}
	files, root, err := readScopeDescriptorsPrivate(ctx, s.spec.Root, d, scopeReadBoundsPrivate{s.spec.EffectivePolicy.FileBytes, s.spec.EffectivePolicy.TotalBytes, s.spec.EffectivePolicy.MaxPathDepth, true}, scopeReadOps{})
	if err != nil {
		return err
	}
	if root != s.rootIdentity || len(files) != len(s.files) {
		return ErrInvalid
	}
	for i, v := range files {
		old := s.files[i]
		v.URI = old.URI
		if v.scopeManifestFile != old.scopeManifestFile || !bytes.Equal(v.Bytes, old.Bytes) {
			return ErrInvalid
		}
	}
	return nil
}
