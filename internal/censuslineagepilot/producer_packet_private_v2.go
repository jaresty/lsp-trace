package censuslineagepilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const ProducerPacketSchemaPrivateV2 = "lsp-trace.complete-scope-producer-packet.private.v2"
const ProducerPacketProfilePrivateV2 = "COMPLETE_ORIGIN_PRIVATE_V2_16M_64M_256M"
const producerPacketBytesPrivateV2 = 16 << 20
const producerPacketArtifactBytesPrivateV2 = 4 << 20
const producerPacketLogicalPrivateV2 uint64 = 64 << 20
const producerPacketReadHashPrivateV2 uint64 = 256 << 20
const producerPacketWorkPrivateV2 uint64 = 1000000

// Two operational topology checks: at most 18 roots, 256 ancestors,
// acquire/stat/reopen/stat plus root validation/close accounting per chain.
const producerPacketTopologyWorkPrivateV2 uint64 = 2 * 18 * (256*4 + 8)

var ErrProducerPacketOverBoundPrivateV2 = errors.New("NOT_RETAINED_OVER_BOUND")

// These limits may only narrow the finite named private profile. They do not
// change producer capture bounds, generic acquisition defaults, or old families.
type ProducerPacketLimitsPrivateV2 struct{ LogicalBytes, ReadHashBytes, ValidationWork uint64 }
type ProducerPacketBudgetPrivateV2 struct{ LogicalBytes, ReadHashBytes, ValidationWork uint64 }
type producerPacketBudgetPrivateV2 struct{ left, spent ProducerPacketBudgetPrivateV2 }

func newProducerPacketBudgetPrivateV2(limits []ProducerPacketLimitsPrivateV2) (*producerPacketBudgetPrivateV2, error) {
	if len(limits) > 1 {
		return nil, ErrInvalid
	}
	l := ProducerPacketLimitsPrivateV2{producerPacketLogicalPrivateV2, producerPacketReadHashPrivateV2, producerPacketWorkPrivateV2}
	if len(limits) == 1 {
		if limits[0].LogicalBytes != 0 {
			l.LogicalBytes = limits[0].LogicalBytes
		}
		if limits[0].ReadHashBytes != 0 {
			l.ReadHashBytes = limits[0].ReadHashBytes
		}
		if limits[0].ValidationWork != 0 {
			l.ValidationWork = limits[0].ValidationWork
		}
	}
	if l.LogicalBytes > producerPacketLogicalPrivateV2 || l.ReadHashBytes > producerPacketReadHashPrivateV2 || l.ValidationWork > producerPacketWorkPrivateV2 {
		return nil, ErrProducerPacketOverBoundPrivateV2
	}
	return &producerPacketBudgetPrivateV2{left: ProducerPacketBudgetPrivateV2(l)}, nil
}
func (b *producerPacketBudgetPrivateV2) reserve(logical, readHash, work uint64) error {
	if b == nil || logical > b.left.LogicalBytes || readHash > b.left.ReadHashBytes || work > b.left.ValidationWork {
		return ErrProducerPacketOverBoundPrivateV2
	}
	b.left.LogicalBytes -= logical
	b.left.ReadHashBytes -= readHash
	b.left.ValidationWork -= work
	b.spent.LogicalBytes += logical
	b.spent.ReadHashBytes += readHash
	b.spent.ValidationWork += work
	return nil
}

// Repeated subtraction avoids overflow even for hostile declared lengths. These
// are named logical allocation/read/hash/validation reservations, NOT CPU or heap
// metering. Library parser implementation work is bounded by admitted input size.
func (b *producerPacketBudgetPrivateV2) passes(n uint64, logical, scan int, work uint64) error {
	for i := 0; i < logical; i++ {
		if err := b.reserve(n, 0, 0); err != nil {
			return err
		}
	}
	for i := 0; i < scan; i++ {
		if err := b.reserve(0, n, 0); err != nil {
			return err
		}
	}
	return b.reserve(0, 0, work)
}

type ProducerPacketRolePrivateV2 struct {
	Index      int    `json:"index"`
	Role       string `json:"role"`
	Member     int    `json:"member"`
	Batch      int    `json:"batch"`
	SHA256     string `json:"sha256"`
	ByteLength uint64 `json:"byte_length"`
}
type ProducerPacketFilePrivateV2 struct {
	Record scopeManifestFile `json:"record"`
	Body   []byte            `json:"body_base64"`
}
type ProducerPacketBatchPrivateV2 struct {
	Ordinal          int    `json:"ordinal"`
	Seeds            []byte `json:"canonical_batch_seeds_base64"`
	Origin           []byte `json:"full_capture_origin_base64"`
	SourceArtifact   []byte `json:"source_artifact_base64"`
	ManifestInput    []byte `json:"canonical_manifest_input_base64"`
	AttachedObserved bool   `json:"attached_observed"`
	ReleasedObserved bool   `json:"released_observed"`
}

// Pure inspection DTO. No function accepts this DTO as producer or seal input.
// Every []byte field encodes ORIGINAL bytes as JSON base64, never relabelled V1.
type ProducerPacketInspectionPrivateV2 struct {
	Schema              string                         `json:"schema"`
	Profile             string                         `json:"profile"`
	PolicySHA256        string                         `json:"profile_policy_sha256"`
	Authority           uint8                          `json:"authority"`
	Accepted            bool                           `json:"accepted"`
	Feature             string                         `json:"feature_identity"`
	Completeness        string                         `json:"source_completeness"`
	Absence             string                         `json:"absence_inference"`
	FrozenSpec          []byte                         `json:"frozen_spec_base64"`
	FrozenManifest      []byte                         `json:"frozen_manifest_base64"`
	NominationSelection []byte                         `json:"nomination_selection_base64"`
	FilePlan            []byte                         `json:"file_plan_base64"`
	EffectivePolicy     []byte                         `json:"effective_policy_base64"`
	EnumerationOrigin   []byte                         `json:"actual_enumeration_origin_base64"`
	RuntimeMetadata     []byte                         `json:"runtime_metadata_base64"`
	Files               []ProducerPacketFilePrivateV2  `json:"files"`
	Batches             []ProducerPacketBatchPrivateV2 `json:"batches"`
	Roles               []ProducerPacketRolePrivateV2  `json:"ordered_roles"`
}

const producerPacketPolicyPrivateV2 = "packet=16777216;artifact=4194304;metadata=131072;nomination=1..8;context=0..8;members<=16;targets=1..63;batches=1..63;logical=67108864;read_hash=268435456;work=1000000;authority=0;accepted=false;feature=UNRESOLVED;completeness=UNKNOWN;absence=PROHIBITED;source=NONATOMIC;copies=SEPARATE_INODES"

func producerPacketRolesPrivateV2(p *ProducerPacketInspectionPrivateV2) []ProducerPacketRolePrivateV2 {
	roles := make([]ProducerPacketRolePrivateV2, 0, 7+len(p.Files)+4*len(p.Batches))
	add := func(role string, member, batch int, raw []byte) {
		roles = append(roles, ProducerPacketRolePrivateV2{len(roles), role, member, batch, Digest(raw), uint64(len(raw))})
	}
	for i, v := range [][]byte{p.FrozenSpec, p.FrozenManifest, p.NominationSelection, p.FilePlan, p.EffectivePolicy, p.EnumerationOrigin, p.RuntimeMetadata} {
		add([]string{"FROZEN_SPEC", "FROZEN_MANIFEST", "NOMINATION_SELECTION", "FILE_PLAN", "EFFECTIVE_POLICY", "ENUMERATION_ORIGIN", "RUNTIME_METADATA"}[i], -1, -1, v)
	}
	for i, f := range p.Files {
		add("OBSERVED_SOURCE_BODY", i, -1, f.Body)
	}
	for i, b := range p.Batches {
		add("CANONICAL_BATCH_SEEDS", -1, i, b.Seeds)
		add("FULL_CAPTURE_ORIGIN", -1, i, b.Origin)
		add("SOURCE_ARTIFACT_PRIVATE_V2", -1, i, b.SourceArtifact)
		add("CANONICAL_MANIFEST_INPUT", -1, i, b.ManifestInput)
	}
	return roles
}

// This historical proof check deliberately performs no Manager, lease, source,
// endpoint, or acquisition calls. It cannot recover execution authority.
func successfulProducerLineagePrivateV2(f FrozenScopePrivateV2, o EnumerationOriginPrivateV2, c OpaqueSuccessfulV2Capture) bool {
	if !f.valid() || !o.valid() || !c.valid() || o.state.frozen.state != f.state {
		return false
	}
	s := c.state
	if s.enumeration != o.state || !s.attachedObserved || !s.releasedObserved || s.ordinal < 0 || s.ordinal >= len(o.state.batches) {
		return false
	}
	b := o.state.batches[s.ordinal]
	if b == nil || b.self != b || b.origin != o.state || b.ordinal != s.ordinal {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spent && b.failure == nil && b.success == s && bytes.Equal(b.seeds, s.seeds)
}

func buildProducerPacketPrivateV2(ctx context.Context, f FrozenScopePrivateV2, o EnumerationOriginPrivateV2, captures []OpaqueSuccessfulV2Capture, budget *producerPacketBudgetPrivateV2) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || !f.valid() || !o.valid() || o.state.frozen.state != f.state || len(o.state.batches) < 1 || len(o.state.batches) > 63 || len(captures) != len(o.state.batches) {
		return nil, ErrInvalid
	}
	// Identity, exact ordinal coverage and source size preflight BEFORE any clone.
	ordered := make([]*successfulCaptureStatePrivateV2, len(captures))
	total := uint64(0)
	for _, c := range captures {
		if err := budget.passes(scopeMetadataPrivateV2, 0, 1, 1); err != nil {
			return nil, err
		}
		if !successfulProducerLineagePrivateV2(f, o, c) {
			return nil, ErrInvalid
		}
		s := c.state
		if ordered[s.ordinal] != nil || len(s.artifact) == 0 {
			return nil, ErrInvalid
		}
		if len(s.artifact) > producerPacketArtifactBytesPrivateV2 || len(s.origin) > scopeMetadataPrivateV2 || len(s.seeds) > scopeMetadataPrivateV2 {
			return nil, ErrProducerPacketOverBoundPrivateV2
		}
		ordered[s.ordinal] = s
	}
	for _, s := range ordered {
		if s == nil {
			return nil, ErrInvalid
		}
	}
	for _, file := range f.state.files {
		if len(file.Bytes) > 1<<20 || uint64(len(file.Bytes)) > 4<<20-total {
			return nil, ErrProducerPacketOverBoundPrivateV2
		}
		total += uint64(len(file.Bytes))
	}
	if err := budget.passes(scopeMetadataPrivateV2, 2, 2, 16); err != nil {
		return nil, err
	}
	p := ProducerPacketInspectionPrivateV2{Schema: ProducerPacketSchemaPrivateV2, Profile: ProducerPacketProfilePrivateV2, PolicySHA256: Digest([]byte(producerPacketPolicyPrivateV2)), Feature: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", FrozenSpec: f.state.specBytes, FrozenManifest: f.state.manifest, NominationSelection: f.state.selection, FilePlan: f.state.filePlan, EnumerationOrigin: o.state.raw}
	p.EffectivePolicy, _ = json.Marshal(f.state.spec.EffectivePolicy)
	p.RuntimeMetadata, _ = json.Marshal(o.state.adapter.metadata)
	for _, file := range f.state.files {
		p.Files = append(p.Files, ProducerPacketFilePrivateV2{file.scopeManifestFile, file.Bytes})
	}
	for i, s := range ordered {
		p.Batches = append(p.Batches, ProducerPacketBatchPrivateV2{i, s.seeds, s.origin, s.artifact, o.state.batches[i].binding.Input, s.attachedObserved, s.releasedObserved})
	}
	// Exact JSON base64-size reservation before encoding: field payload expansion
	// plus conservative finite structural metadata. A final exact bound follows.
	payload := uint64(0)
	add := func(raw []byte) error {
		n := uint64(len(raw))
		if n > producerPacketBytesPrivateV2 || n > producerPacketBytesPrivateV2-payload {
			return ErrProducerPacketOverBoundPrivateV2
		}
		payload += n
		return nil
	}
	for _, raw := range [][]byte{p.FrozenSpec, p.FrozenManifest, p.NominationSelection, p.FilePlan, p.EffectivePolicy, p.EnumerationOrigin, p.RuntimeMetadata} {
		if err := add(raw); err != nil {
			return nil, err
		}
	}
	for _, file := range p.Files {
		if err := add(file.Body); err != nil {
			return nil, err
		}
	}
	for _, b := range p.Batches {
		for _, raw := range [][]byte{b.Seeds, b.Origin, b.SourceArtifact, b.ManifestInput} {
			if err := add(raw); err != nil {
				return nil, err
			}
		}
	}
	// Reserve role hashes, source-role scan and encoding before they run. The
	// separate marshaler pass includes base64 expansion, not just logical payload.
	if err := budget.passes(payload, 1, 2, uint64(7+len(p.Files)+4*len(p.Batches))); err != nil {
		return nil, err
	}
	p.Roles = producerPacketRolesPrivateV2(&p)
	// The size preflight marshals metadata ONLY, replacing each payload with an
	// empty base64 string. It never allocates an encoded full-packet trial copy.
	if err := budget.passes(4*scopeMetadataPrivateV2, 2, 2, 1); err != nil {
		return nil, err
	}
	size, err := producerPacketEncodedLengthPrivateV2(p)
	if err != nil {
		return nil, err
	}
	if err := budget.passes(uint64(size), 1, 1, 1); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) != size || len(raw) > producerPacketBytesPrivateV2 {
		return nil, ErrProducerPacketOverBoundPrivateV2
	}
	if err = validateProducerPacketPrivateV2(p, budget); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}

func producerPacketEncodedLengthPrivateV2(p ProducerPacketInspectionPrivateV2) (int, error) {
	var lengths []int
	empty := func(raw []byte) []byte {
		if raw == nil {
			return nil
		}
		lengths = append(lengths, len(raw))
		return []byte{}
	}
	p.FrozenSpec = empty(p.FrozenSpec)
	p.FrozenManifest = empty(p.FrozenManifest)
	p.NominationSelection = empty(p.NominationSelection)
	p.FilePlan = empty(p.FilePlan)
	p.EffectivePolicy = empty(p.EffectivePolicy)
	p.EnumerationOrigin = empty(p.EnumerationOrigin)
	p.RuntimeMetadata = empty(p.RuntimeMetadata)
	p.Files = append(p.Files[:0:0], p.Files...)
	for i := range p.Files {
		p.Files[i].Body = empty(p.Files[i].Body)
	}
	p.Batches = append(p.Batches[:0:0], p.Batches...)
	for i := range p.Batches {
		b := &p.Batches[i]
		b.Seeds = empty(b.Seeds)
		b.Origin = empty(b.Origin)
		b.SourceArtifact = empty(b.SourceArtifact)
		b.ManifestInput = empty(b.ManifestInput)
	}
	metadata, err := json.Marshal(p)
	if err != nil {
		return 0, err
	}
	if len(metadata) > 4*scopeMetadataPrivateV2 {
		return 0, ErrProducerPacketOverBoundPrivateV2
	}
	size := len(metadata)
	for _, n := range lengths {
		if n > producerPacketBytesPrivateV2 {
			return 0, ErrProducerPacketOverBoundPrivateV2
		}
		encoded := base64.StdEncoding.EncodedLen(n)
		if encoded > producerPacketBytesPrivateV2-size {
			return 0, ErrProducerPacketOverBoundPrivateV2
		}
		size += encoded
	}
	return size, nil
}

// Pure parse/validation is inspection only. No seal state is reachable here.
func PureParseProducerPacketPrivateV2(raw []byte) (ProducerPacketInspectionPrivateV2, error) {
	b, _ := newProducerPacketBudgetPrivateV2(nil)
	return parseProducerPacketPrivateV2(raw, b)
}
func parseProducerPacketPrivateV2(raw []byte, b *producerPacketBudgetPrivateV2) (p ProducerPacketInspectionPrivateV2, err error) {
	if len(raw) == 0 || len(raw) > producerPacketBytesPrivateV2 {
		return p, ErrProducerPacketOverBoundPrivateV2
	}
	// Decode/base64, strict duplicate scan, canonical re-encoding and exact compare.
	if err = b.passes(uint64(len(raw)), 4, 5, 100000); err != nil {
		return p, err
	}
	if err = preflightProducerPacketWirePrivateV2(raw); err != nil {
		return p, err
	}
	if err = strictjson.RejectDuplicates(raw); err != nil {
		return p, err
	}
	if err = producerPacketStrictPrivateV2(raw, &p); err != nil {
		return p, err
	}
	if err = validateProducerPacketPrivateV2(p, b); err != nil {
		return ProducerPacketInspectionPrivateV2{}, err
	}
	return p, nil
}

func producerPacketStrictPrivateV2(raw []byte, v any) error {
	if len(raw) == 0 || len(raw) > producerPacketBytesPrivateV2 {
		return ErrProducerPacketOverBoundPrivateV2
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return ErrInvalid
	}
	canonical, err := json.Marshal(v)
	if err != nil || !bytes.Equal(canonical, raw) {
		return ErrInvalid
	}
	return nil
}

// Read-only DTO plus finite budget, never state or authority constructors.
func validateProducerPacketPrivateV2(p ProducerPacketInspectionPrivateV2, budget *producerPacketBudgetPrivateV2) error {
	if p.Schema != ProducerPacketSchemaPrivateV2 || p.Profile != ProducerPacketProfilePrivateV2 || p.PolicySHA256 != Digest([]byte(producerPacketPolicyPrivateV2)) || p.Authority != 0 || p.Accepted || p.Feature != "UNRESOLVED" || p.Completeness != "UNKNOWN" || p.Absence != "PROHIBITED" || len(p.Files) < 1 || len(p.Files) > 16 || len(p.Batches) < 1 || len(p.Batches) > 63 || len(p.Roles) != 7+len(p.Files)+4*len(p.Batches) {
		return ErrInvalid
	}
	for _, raw := range [][]byte{p.FrozenSpec, p.FrozenManifest, p.NominationSelection, p.FilePlan, p.EffectivePolicy, p.EnumerationOrigin, p.RuntimeMetadata} {
		if len(raw) == 0 || len(raw) > scopeMetadataPrivateV2 {
			return ErrInvalid
		}
		if err := budget.passes(uint64(len(raw)), 2, 8, 16); err != nil {
			return err
		}
	}
	var spec ScopeSpecPrivateV2
	var manifest scopeManifestPrivateV2
	var origin originFramePrivateV2
	var meta sessionruntime.SessionMetadata
	if producerPacketStrictPrivateV2(p.FrozenSpec, &spec) != nil || producerPacketStrictPrivateV2(p.FrozenManifest, &manifest) != nil || producerPacketStrictPrivateV2(p.EnumerationOrigin, &origin) != nil || producerPacketStrictPrivateV2(p.RuntimeMetadata, &meta) != nil {
		return ErrInvalid
	}
	_, canonical, err := validateScopeSpecPrivateV2(spec)
	if err != nil || !bytes.Equal(canonical, p.FrozenSpec) {
		return ErrInvalid
	}
	policy, _ := json.Marshal(spec.EffectivePolicy)
	if !bytes.Equal(policy, p.EffectivePolicy) {
		return ErrInvalid
	}
	if meta.PositionEncoding != "utf-8" && meta.PositionEncoding != "utf-16" && meta.PositionEncoding != "utf-32" {
		return ErrInvalid
	}
	if !meta.CallHierarchySupport || !meta.DocumentSymbolSupport {
		return ErrInvalid
	}
	if len(manifest.Files) != len(p.Files) || len(spec.Members) != len(p.Files) || manifest.Schema != "lsp-trace.frozen-scope.private.v2" || manifest.Authority != 0 || manifest.Accepted || manifest.Feature != "UNRESOLVED" || manifest.Source != "UNKNOWN" || manifest.Absence != "PROHIBITED" || manifest.RevisionQualification != "CALLER_ASSERTED_NOT_AUTHENTICATED" || manifest.SourceRootClaim != sourceRootNoAtomicSnapshotPrivateV2 || manifest.SpecSHA256 != Digest(p.FrozenSpec) || manifest.SelectionSHA256 != Digest(p.NominationSelection) || manifest.FilePlanSHA256 != Digest(p.FilePlan) {
		return ErrInvalid
	}
	var nominations []censusacquisition.SourceFile
	query := make([]string, len(p.Files))
	byURI := map[string]ProducerPacketFilePrivateV2{}
	languages := map[string]string{}
	identities := map[scopeFileIdentity]bool{}
	if manifest.RootIdentity.Inode == 0 {
		return ErrInvalid
	}
	total := uint64(0)
	for i, file := range p.Files {
		m := spec.Members[i]
		uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(spec.Root, m.CanonicalRelativePath))}).String()
		if len(file.Body) == 0 || len(file.Body) > 1<<20 || uint64(len(file.Body)) > 4<<20-total {
			return ErrInvalid
		}
		total += uint64(len(file.Body))
		if err = budget.passes(uint64(len(file.Body)), 1, 3, 1); err != nil {
			return err
		}
		r := file.Record
		if r != manifest.Files[i] || r.Path != m.CanonicalRelativePath || r.Role != m.Role || r.URI != uri || r.SHA256 != m.ExpectedSHA256 || r.ByteLength != m.ByteLength || r.ByteLength != uint64(len(file.Body)) || r.SHA256 != Digest(file.Body) || r.Identity.Inode == 0 || identities[r.Identity] || !os.FileMode(r.Mode).IsRegular() {
			return ErrInvalid
		}
		identities[r.Identity] = true
		query[i] = uri
		byURI[uri] = file
		languages[uri] = m.LanguageID
		if m.Role == "NOMINATION" {
			nominations = append(nominations, censusacquisition.SourceFile{Path: r.Path, URI: r.URI, LanguageID: m.LanguageID})
		}
	}
	selection, e := NewSourceSelection(spec.Root, nominations)
	if e != nil {
		return e
	}
	selectionBytes, e := EncodeSourceSelection(selection)
	if e != nil || !bytes.Equal(selectionBytes, p.NominationSelection) {
		return ErrInvalid
	}
	plan, e := NewPartitionPlan(selection, spec.PartitionSize)
	if e != nil {
		return e
	}
	planBytes, e := EncodePartitionPlan(plan, selection)
	if e != nil || !bytes.Equal(planBytes, p.FilePlan) {
		return ErrInvalid
	}
	sort.Strings(query)
	if origin.Schema != "lsp-trace.enumeration-origin.private.v2" || origin.Authority != 0 || origin.Accepted || origin.Feature != "UNRESOLVED" || origin.Source != "UNKNOWN" || origin.Absence != "PROHIBITED" || origin.ScopeSHA256 != Digest(p.FrozenManifest) || origin.SpecSHA256 != Digest(p.FrozenSpec) || origin.SelectionSHA256 != Digest(p.NominationSelection) || origin.FilePlanSHA256 != Digest(p.FilePlan) || origin.PolicySHA256 != Digest(p.EffectivePolicy) || origin.EnumerationSHA256 != Digest(origin.Enumeration) || !reflect.DeepEqual(origin.CanonicalQueryURIs, query) {
		return ErrInvalid
	}
	var discovery censusacquisition.Discovery
	if producerPacketStrictPrivateV2(origin.Enumeration, &discovery) != nil || discovery.Workspace != spec.Root || discovery.Accounting.FileDenominator != len(nominations) || len(discovery.Targets) < 1 || len(discovery.Targets) > spec.EffectivePolicy.MaxTargets || len(discovery.Targets) > 63 {
		return ErrInvalid
	}
	if err = validatePacketDiscoveryPrivateV2(discovery, nominations, spec.EffectivePolicy.Discovery.MaxSymbols); err != nil {
		return err
	}
	for _, target := range discovery.Targets {
		file, ok := byURI[target.URI]
		if !ok || file.Record.Role != "NOMINATION" || target.LanguageID != languages[target.URI] {
			return ErrInvalid
		}
	}
	expectedBatches, e := (censusacquisition.Core{Planning: &spec.EffectivePolicy.Planning}).PlanDiscovery(discovery.Session, discovery)
	if e != nil || !reflect.DeepEqual(expectedBatches, origin.PlannedBatches) || len(expectedBatches) != len(p.Batches) {
		return ErrInvalid
	}
	workspace := (&url.URL{Scheme: "file", Path: spec.Root}).String()
	expectedMembers := make([]graphprovenance.SourceMemberPrivateV2, len(query))
	for i, uri := range query {
		file := byURI[uri]
		expectedMembers[i] = graphprovenance.SourceMemberPrivateV2{URI: uri, SHA256: file.Record.SHA256, Role: file.Record.Role}
	}
	for i, b := range p.Batches {
		if b.Ordinal != i || !b.AttachedObserved || !b.ReleasedObserved || len(b.SourceArtifact) == 0 {
			return ErrInvalid
		}
		if len(b.SourceArtifact) > producerPacketArtifactBytesPrivateV2 || len(b.Origin) > scopeMetadataPrivateV2 || len(b.Seeds) > scopeMetadataPrivateV2 || len(b.ManifestInput) > scopeMetadataPrivateV2 {
			return ErrProducerPacketOverBoundPrivateV2
		}
		// Source parser performs strict JSON, source-policy, canonical native graph,
		// receipts, directional raw-custody and typed consistency scans. Reserve each
		// named carrier projection plus conservative bounded parser scan allowance.
		if err = budget.passes(uint64(len(b.SourceArtifact)), 3, 24, 4096); err != nil {
			return err
		}
		for _, raw := range [][]byte{b.Origin, b.Seeds, b.ManifestInput} {
			if err = budget.passes(uint64(len(raw)), 1, 4, 16); err != nil {
				return err
			}
		}
		expected := expectedBatches[i]
		if !bytes.Equal(expected.CanonicalSeedsV2, b.Seeds) {
			return ErrInvalid
		}
		var co captureOriginFramePrivateV2
		if producerPacketStrictPrivateV2(b.Origin, &co) != nil || co.Schema != "lsp-trace.capture-origin.private.v2" || co.Authority != 0 || co.Accepted || co.Feature != "UNRESOLVED" || co.Source != "UNKNOWN" || co.Absence != "PROHIBITED" || co.Ordinal != i || !bytes.Equal(co.EnumerationOrigin, p.EnumerationOrigin) || co.EnumerationOriginSHA256 != Digest(p.EnumerationOrigin) || co.SeedSHA256 != Digest(b.Seeds) || co.SeedByteLength != uint64(len(b.Seeds)) || co.ArtifactSHA256 != Digest(b.SourceArtifact) || co.ArtifactByteLength != uint64(len(b.SourceArtifact)) {
			return ErrInvalid
		}
		mf := expected.AcquisitionManifest(spec.EffectivePolicy.acquisitionLimits())
		input, _ := json.Marshal(scopedBatchInputPrivate{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, Manifest: mf, OutputVersion: graphprovenance.VersionV5})
		if !bytes.Equal(input, b.ManifestInput) {
			return ErrInvalid
		}
		source, e := graphprovenance.ParseSourceWithholdingPrivateV2(b.SourceArtifact)
		if e != nil {
			return e
		}
		if source.Policy.WorkspaceURI != workspace || source.Evidence.WorkspaceURI != workspace || source.Policy.SessionID != discovery.Session.SessionID || source.Policy.Generation != discovery.Session.Generation || !reflect.DeepEqual(source.Policy.Members, expectedMembers) {
			return ErrInvalid
		}
		req, e := mf.Request(acquisitionmanifest.SliceV3)
		if e != nil {
			return e
		}
		req.TopmostSiblings = mf.Expansion.TopmostSiblings
		req.Context = acquisition.AcquisitionContext{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, PositionEncoding: meta.PositionEncoding}
		contextInput, _ := json.Marshal(req)
		req.Context.ID = fmt.Sprintf("public-acquisition-v2:%x", sha256.Sum256(contextInput))
		declaration, e := acquisition.NewFrozenQueryDeclarationPrivateV2(query)
		if e != nil {
			return e
		}
		req, e = req.WithFrozenQueryDeclarationPrivateV2(declaration)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(req, source.Evidence.Acquisition.Request) {
			return ErrInvalid
		}
		native, e := graph.MarshalWithRetainedProcessContextPrivate(source.Evidence.Acquisition.Graph, source.Evidence.GraphBytes)
		if e != nil || !bytes.Equal(native, source.Evidence.GraphBytes) {
			return ErrInvalid
		}
		// Only present readable source observations are cross-compared. Policy-withheld
		// endpoints are never recast as absence; unobserved members remain unobserved.
		checkReceipt := func(r graphprovenance.Receipt) error {
			file, ok := byURI[r.URI]
			if !ok {
				return nil
			}
			if r.Content != nil && !bytes.Equal(r.Content, file.Body) {
				return ErrInvalid
			}
			return nil
		}
		for _, r := range source.Evidence.Captures {
			if e = checkReceipt(r); e != nil {
				return e
			}
		}
		for _, s := range source.Evidence.Supplies {
			if s.Receipt != nil {
				if e = checkReceipt(*s.Receipt); e != nil {
					return e
				}
			}
		}
	}
	if !reflect.DeepEqual(p.Roles, producerPacketRolesPrivateV2(&p)) {
		return ErrInvalid
	}
	return nil
}
