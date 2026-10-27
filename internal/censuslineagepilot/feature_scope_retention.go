package censuslineagepilot

import (
	"bytes"
	"encoding/json"
	"math"
	"net/url"
	"path/filepath"

	"lsp-trace/internal/censusacquisition"
)

// These are logical accounting ceilings, NOT filesystem free-space promises.
// Reserve metadata in both copies and two largest-object staging allowances.
const scopeRetentionMetadataLimit uint64 = 64 << 10
const scopeRetentionByteCeiling uint64 = 64 << 20
const scopeRetentionWorkCeiling uint64 = 512 << 20

// No mutable objects or caller-selected object list escape this private plan.
// References remain ordered even when storage objects share identical bytes.
type scopeRetentionPlan struct{ state *scopeRetentionState }
type scopeRetentionState struct {
	objects    []scopeRetentionObject
	references []scopeRetentionReference
	metadata   []byte
	budget     scopeRetentionBudget
}
type scopeRetentionObject struct {
	digest string
	raw    []byte
}
type scopeRetentionReference struct {
	Role       string `json:"role"`
	Path       string `json:"path,omitempty"`
	Object     int    `json:"object"`
	SHA256     string `json:"sha256"`
	ByteLength uint64 `json:"byte_length"`
}
type scopeRetentionBudget struct{ bytes, work uint64 }

func retentionAdd(a, b uint64) (uint64, error) {
	if b > math.MaxUint64-a {
		return 0, ErrInvalid
	}
	return a + b, nil
}
func retentionMultiply(a, b uint64) (uint64, error) {
	if a != 0 && b > math.MaxUint64/a {
		return 0, ErrInvalid
	}
	return a * b, nil
}

// Budget includes both stored copies, bounded reference metadata in each, and
// temporary object space. Work bounds bytes processed, not CPU time or syscalls.
func retentionBudget(unique, largest, input uint64, objects uint64) (scopeRetentionBudget, error) {
	if objects < 1 || objects > 9 || largest > unique {
		return scopeRetentionBudget{}, ErrInvalid
	}
	per, err := retentionAdd(unique, scopeRetentionMetadataLimit)
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	per, err = retentionAdd(per, largest)
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	total, err := retentionMultiply(per, 2)
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	// Conservative allowance for initial hash, equality, publication and repeated
	// close/reopen verification; this unit itself performs no publication.
	work, err := retentionMultiply(input, 32)
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	work, err = retentionAdd(work, 2*scopeRetentionMetadataLimit)
	if err != nil {
		return scopeRetentionBudget{}, err
	}
	if total > scopeRetentionByteCeiling || work > scopeRetentionWorkCeiling {
		return scopeRetentionBudget{}, ErrInvalid
	}
	return scopeRetentionBudget{total, work}, nil
}

// deriveScopeRetentionPlan only consumes an opaque in-memory feature scope. It
// validates exact canonical linkage without reopening mutable source paths.
// Available budgets may restrict, but never raise, repository-owned ceilings.
// No storage writes, root admissions, or retained-bundle claims occur here.
func deriveScopeRetentionPlan(scope featureScope, availableBytes, availableWork uint64) (scopeRetentionPlan, error) {
	if scope.state == nil || len(scope.state.files) != 6 || len(scope.state.manifest) > int(scopeRetentionMetadataLimit) || len(scope.state.selection) > int(scopeRetentionMetadataLimit) || len(scope.state.plan) > int(scopeRetentionMetadataLimit) {
		return scopeRetentionPlan{}, ErrInvalid
	}
	var m scopeManifest
	if json.Unmarshal(scope.state.manifest, &m) != nil || !featureScopeRevision.MatchString(m.RevisionAssertion) || !filepath.IsAbs(m.Root) || filepath.Clean(m.Root) != m.Root || m.RootIdentity.Inode == 0 {
		return scopeRetentionPlan{}, ErrInvalid
	}
	// Check every body bound before cloning or hashing any caller-inaccessible
	// state. Inconsistent package-local test values must not bypass work limits.
	var input uint64
	for _, f := range scope.state.files {
		size := uint64(len(f.Bytes))
		var err error
		input, err = retentionAdd(input, size)
		if err != nil || size == 0 || size > featureScopeFileLimit || input > featureScopeTotalLimit {
			return scopeRetentionPlan{}, ErrInvalid
		}
	}
	files := scope.Files()
	sources := make([]censusacquisition.SourceFile, 4)
	for i, path := range featureScopePaths() {
		f := files[i]
		role := "NOMINATION"
		if i >= 4 {
			role = "CONTEXT_ONLY"
		}
		uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(m.Root, path))}).String()
		if f.Path != path || f.Role != role || f.URI != uri || len(f.Bytes) == 0 || len(f.Bytes) > featureScopeFileLimit || f.ByteLength != uint64(len(f.Bytes)) || f.SHA256 != Digest(f.Bytes) || f.Identity.Inode == 0 {
			return scopeRetentionPlan{}, ErrInvalid
		}
		if i < 4 {
			sources[i] = censusacquisition.SourceFile{Path: path, URI: uri, LanguageID: "go"}
		}
	}
	selection, err := NewSourceSelection(m.Root, sources)
	if err != nil {
		return scopeRetentionPlan{}, err
	}
	sb, err := EncodeSourceSelection(selection)
	if err != nil || !bytes.Equal(sb, scope.state.selection) {
		return scopeRetentionPlan{}, ErrInvalid
	}
	partition, err := NewPartitionPlan(selection, 2)
	if err != nil {
		return scopeRetentionPlan{}, err
	}
	pb, err := EncodePartitionPlan(partition, selection)
	if err != nil || !bytes.Equal(pb, scope.state.plan) {
		return scopeRetentionPlan{}, ErrInvalid
	}
	expected := scopeManifest{SchemaVersion: featureScopeSchema, Domain: featureScopeDomain, Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN", FeatureIdentity: "UNRESOLVED", RevisionAssertion: m.RevisionAssertion, RevisionQualification: "CALLER_ASSERTED_NOT_AUTHENTICATED", IncludedRule: "EXACT_FOUR_NOMINATION_TWO_CONTEXT", ExcludedRule: "ALL_OTHER_PATHS_EXCLUDED_EXTERNAL_RELATIONS_ARE_FRONTIER_GAPS", Root: m.Root, RootIdentity: m.RootIdentity, Files: make([]scopeManifestFile, 6), PartitionSize: 2, SelectionSHA256: Digest(sb), SelectionByteLength: uint64(len(sb)), PlanSHA256: Digest(pb), PlanByteLength: uint64(len(pb))}
	for i, f := range files {
		expected.Files[i] = f.scopeManifestFile
	}
	mb, err := canonicalFeatureScope(expected)
	if err != nil || !bytes.Equal(mb, scope.state.manifest) {
		return scopeRetentionPlan{}, ErrInvalid
	}
	state := &scopeRetentionState{}
	appendObject := func(role, path string, raw []byte) error {
		digest := Digest(raw)
		ordinal := len(state.objects)
		for i, obj := range state.objects {
			if obj.digest == digest {
				if !bytes.Equal(obj.raw, raw) {
					return ErrInvalid
				}
				ordinal = i
				break
			}
		}
		if ordinal == len(state.objects) {
			state.objects = append(state.objects, scopeRetentionObject{digest, bytes.Clone(raw)})
		}
		state.references = append(state.references, scopeRetentionReference{role, path, ordinal, digest, uint64(len(raw))})
		return nil
	}
	for _, f := range files {
		if err = appendObject(f.Role, f.Path, f.Bytes); err != nil {
			return scopeRetentionPlan{}, err
		}
	}
	for _, v := range []struct {
		role string
		raw  []byte
	}{{"SOURCE_SELECTION", sb}, {"PARTITION_PLAN", pb}, {"SCOPE_MANIFEST", mb}} {
		if err = appendObject(v.role, "", v.raw); err != nil {
			return scopeRetentionPlan{}, err
		}
		input, err = retentionAdd(input, uint64(len(v.raw)))
		if err != nil {
			return scopeRetentionPlan{}, err
		}
	}
	// Finite active dependency metadata; no timestamps, expiry, deletion policy,
	// acceptance, execution admission, durable head, or workflow state.
	state.metadata, err = json.Marshal(struct {
		Schema     string                    `json:"schema"`
		Authority  uint8                     `json:"authority"`
		References []scopeRetentionReference `json:"active_references"`
	}{"lsp-trace.feature-scope-retention.private.v1", 0, state.references})
	if err != nil || uint64(len(state.metadata)) > scopeRetentionMetadataLimit {
		return scopeRetentionPlan{}, ErrInvalid
	}
	var unique, largest uint64
	for _, obj := range state.objects {
		size := uint64(len(obj.raw))
		unique, err = retentionAdd(unique, size)
		if err != nil {
			return scopeRetentionPlan{}, err
		}
		if size > largest {
			largest = size
		}
	}
	state.budget, err = retentionBudget(unique, largest, input, uint64(len(state.objects)))
	if err != nil || state.budget.bytes > availableBytes || state.budget.work > availableWork {
		return scopeRetentionPlan{}, ErrInvalid
	}
	return scopeRetentionPlan{state}, nil
}
func (p scopeRetentionPlan) metadataBytes() []byte {
	if p.state == nil {
		return nil
	}
	return bytes.Clone(p.state.metadata)
}
func (p scopeRetentionPlan) objectBytes(ordinal int) []byte {
	if p.state == nil || ordinal < 0 || ordinal >= len(p.state.objects) {
		return nil
	}
	return bytes.Clone(p.state.objects[ordinal].raw)
}
func (p scopeRetentionPlan) references() []scopeRetentionReference {
	if p.state == nil {
		return nil
	}
	return append([]scopeRetentionReference(nil), p.state.references...)
}
