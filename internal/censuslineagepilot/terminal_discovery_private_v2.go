package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

const TerminalDiscoverySchemaPrivateV2 = "lsp-trace.terminal-discovery.private.v2"
const TerminalTargetCapPrivateV2 = "TERMINAL_RESOURCE_GAP_TARGET_CAP"
const TerminalNoTargetsPrivateV2 = "TERMINAL_NO_PREPARABLE_TARGETS"

// This owner is not an acquired EnumerationOrigin. No JSON/DTO constructor,
// source fallback, restored runtime, or generic error classifier exists.
type TerminalDiscoveryOutcomePrivateV2 struct {
	state *terminalDiscoveryStatePrivateV2
}
type terminalDiscoveryStatePrivateV2 struct {
	self     *terminalDiscoveryStatePrivateV2
	adapter  *scopeRuntimeAdapter
	frozen   FrozenScopePrivateV2
	manager  *sessionruntime.Manager
	guard    *sessionruntime.RetainedScopeGuardPrivateV2
	identity censusacquisition.SessionIdentity
	lifetime context.Context
	raw      []byte
	reason   string
	mu       sync.Mutex
	used     bool
}

// Detached read-only inspection, never an owner constructor or qualification.
type TerminalDiscoveryInspectionPrivateV2 struct {
	Schema, InputFamily, Status                                                                 string
	Authority                                                                                   int
	Accepted                                                                                    bool
	FeatureIdentity, SourceCompleteness, AbsenceInference                                       string
	ScopeSHA256, DiscoverySHA256, PolicySHA256                                                  string
	FrozenSpec, FrozenManifest, NominationSelection, FilePlan, EffectivePolicy, RuntimeMetadata json.RawMessage
	Discovery                                                                                   censusacquisition.Discovery
	CanonicalQueryURIs                                                                          []string
}

func (TerminalDiscoveryOutcomePrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func (o TerminalDiscoveryOutcomePrivateV2) valid() bool {
	s := o.state
	return s != nil && s.self == s && s.adapter != nil && s.frozen.valid() && s.adapter.frozenV2.state == s.frozen.state && s.manager == s.adapter.manager && s.guard != nil && s.guard == s.adapter.guardV2 && s.adapter.guard == nil && s.identity == s.adapter.identity && s.lifetime == s.adapter.lifetimeV2
}
func (o TerminalDiscoveryOutcomePrivateV2) Reason() string {
	if !o.valid() {
		return ""
	}
	return o.state.reason
}
func DiscoverScopeOutcomePrivateV2(ctx context.Context, f FrozenScopePrivateV2, m *sessionruntime.Manager, id censusacquisition.SessionIdentity) (EnumerationOriginPrivateV2, TerminalDiscoveryOutcomePrivateV2, ScopeDiscoveryReportPrivateV2, error) {
	var terminal TerminalDiscoveryOutcomePrivateV2
	o, r, err := discoverScopeOutcomePrivateV2(ctx, f, m, id, &terminal)
	return o, terminal, r, err
}
func terminalDiscoveryReasonPrivateV2(d censusacquisition.Discovery, maxTargets, files int) (string, error) {
	if d.Session.Validate() != nil || d.Accounting.FileDenominator != files || len(d.EnumerationSymbols) != d.Accounting.SymbolDenominator || d.Closure != censusacquisition.DiscoveryClosureSuccessful || len(d.ProviderFailures) != 0 || len(d.IncompleteCauses) != 0 || (len(d.Targets) > 0 && len(d.Observations) != 0) || censusacquisition.ValidateTerminalDiscovery(d, censusacquisition.TerminalDiscoveryOutcome(d)) != nil {
		return "", ErrScopeDiscoveryIncompletePrivateV2
	}
	if len(d.Targets) == 0 {
		// Successful empty Census carries these six explicitly UNKNOWN dimensions,
		// not an omission or a claim that provider/source completeness is known.
		dimensions := []censusacquisition.DiscoveryDimension{censusacquisition.DiscoveryNodeCeiling, censusacquisition.DiscoveryDepthBoundary, censusacquisition.DiscoveryTargetSourceOmissions, censusacquisition.DiscoveryProviderCompleteness, censusacquisition.DiscoveryTimeoutRequestBudget, censusacquisition.DiscoveryMessageByteBudget}
		if len(d.Observations) != len(dimensions) {
			return "", ErrScopeDiscoveryIncompletePrivateV2
		}
		for i, observation := range d.Observations {
			if observation.Dimension != dimensions[i] || observation.Known || observation.Observed != nil || observation.Limit != nil || observation.Omitted != nil {
				return "", ErrScopeDiscoveryIncompletePrivateV2
			}
		}
		// Census Complete means non-empty preparable delivery, not ledger closure.
		// Require every original symbol to be explicitly non-callable.
		for _, symbol := range d.EnumerationSymbols {
			if symbol.Disposition != "non-callable" {
				return "", ErrScopeDiscoveryIncompletePrivateV2
			}
		}
		return TerminalNoTargetsPrivateV2, nil
	}
	if d.Complete && len(d.Targets) > maxTargets {
		return TerminalTargetCapPrivateV2, nil
	}
	return "", ErrInvalid
}
func newTerminalDiscoveryOutcomePrivateV2(ctx context.Context, a *scopeRuntimeAdapter, d censusacquisition.Discovery, raw []byte) (TerminalDiscoveryOutcomePrivateV2, error) {
	if a == nil || !a.frozenV2.valid() {
		return TerminalDiscoveryOutcomePrivateV2{}, ErrInvalid
	}
	f := a.frozenV2
	reason, err := terminalDiscoveryReasonPrivateV2(d, f.state.spec.EffectivePolicy.MaxTargets, len(a.files))
	if err != nil {
		return TerminalDiscoveryOutcomePrivateV2{}, err
	}
	if err = validateTerminalSourceLedgerPrivateV2(d, a.files, f.state.spec.EffectivePolicy.Discovery.MaxSymbols, f.state.spec.EffectivePolicy.Discovery.MaxStringBytes); err != nil {
		return TerminalDiscoveryOutcomePrivateV2{}, err
	}
	if d.Session != a.identity || d.Workspace != a.workspace {
		return TerminalDiscoveryOutcomePrivateV2{}, ErrInvalid
	}
	if err = a.check(ctx); err != nil {
		return TerminalDiscoveryOutcomePrivateV2{}, err
	}
	source, err := graphprovenance.BindSourcePrivateV2(ctx, a.manager, a.guardV2, a.identity.SessionID, a.identity.Generation, a.workspace)
	if err != nil {
		return TerminalDiscoveryOutcomePrivateV2{}, err
	}
	declaration, err := source.FrozenQueryDeclarationPrivateV2(ctx)
	if err != nil {
		return TerminalDiscoveryOutcomePrivateV2{}, err
	}
	policy, _ := json.Marshal(f.state.spec.EffectivePolicy)
	metadata, _ := json.Marshal(a.metadata)
	frame := TerminalDiscoveryInspectionPrivateV2{Schema: TerminalDiscoverySchemaPrivateV2, InputFamily: "TERMINAL_ACCOUNTING_NOT_NATIVE", Status: reason, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", ScopeSHA256: Digest(f.state.manifest), DiscoverySHA256: Digest(raw), PolicySHA256: Digest(policy), FrozenSpec: f.SpecBytes(), FrozenManifest: f.ManifestBytes(), NominationSelection: f.SelectionBytes(), FilePlan: f.FilePlanBytes(), EffectivePolicy: policy, RuntimeMetadata: metadata, Discovery: d, CanonicalQueryURIs: declaration.MembersPrivateV2()}
	encoded, err := json.Marshal(frame)
	if err != nil || len(encoded) > scopeMetadataPrivateV2 {
		return TerminalDiscoveryOutcomePrivateV2{}, ErrInvalid
	}
	if err = a.check(ctx); err != nil {
		return TerminalDiscoveryOutcomePrivateV2{}, err
	}
	s := &terminalDiscoveryStatePrivateV2{adapter: a, frozen: f, manager: a.manager, guard: a.guardV2, identity: a.identity, lifetime: a.lifetimeV2, raw: encoded, reason: reason}
	s.self = s
	return TerminalDiscoveryOutcomePrivateV2{s}, nil
}

// One irreversible retention attempt, while the exact original lease is alive.
// Receipts are observations, not restorable authority. No Native seal is minted.
func RetainTerminalDiscoveryPrivateV2(ctx context.Context, f FrozenScopePrivateV2, o TerminalDiscoveryOutcomePrivateV2, roots ProducerPacketRootsPrivateV2) (copies [2]ProducerPacketCopyReceiptPrivateV2, err error) {
	if ctx == nil || !o.valid() || !f.valid() || o.state.frozen.state != f.state {
		return copies, ErrInvalid
	}
	s := o.state
	s.mu.Lock()
	if s.used {
		s.mu.Unlock()
		return copies, ErrInvalid
	}
	s.used = true
	s.mu.Unlock()
	if err = s.adapter.check(ctx); err != nil {
		return copies, err
	}
	if err = roots.check(ctx, f); err != nil {
		return copies, err
	}
	ops := scopeRetentionOps{
		publish: func(c context.Context, r *publication.ContentRetentionRoot, id string, b []byte) (*publication.BoundFileReceipt, error) {
			return r.PublishTerminalDiscoveryPrivateV2(c, id, b)
		},
		read: func(c context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
			return r.ReadTerminalDiscoveryPrivateV2(c, id, n)
		},
	}
	digest := Digest(s.raw)
	attempt, err := retainScopePairBytes(ctx, scopeRetentionObject{digest, s.raw}, digest, 0, roots.state.outputs[0], roots.state.outputs[1], ops)
	copies = [2]ProducerPacketCopyReceiptPrivateV2{{attempt.primary.outcome, attempt.primary.receipt}, {attempt.backup.outcome, attempt.backup.receipt}}
	if err != nil {
		return copies, err
	}
	for _, copy := range copies {
		if copy.Receipt == nil || copy.Outcome != "NAMESPACE_COMMITTED" {
			return copies, ErrInvalid
		}
	}
	if err = publication.CheckTerminalDiscoveryObjectsDistinctPrivateV2(ctx, roots.state.outputs, digest); err != nil {
		return copies, err
	}
	for _, root := range roots.state.outputs {
		got, e := root.ReadTerminalDiscoveryPrivateV2(ctx, digest, int64(len(s.raw)))
		if e != nil {
			return copies, e
		}
		if !bytes.Equal(got, s.raw) {
			return copies, ErrInvalid
		}
	}
	if err = roots.check(ctx, f); err != nil {
		return copies, err
	}
	return copies, s.adapter.check(ctx)
}

// Verification of detached bytes never reconstructs TerminalDiscoveryOutcome.
func InspectTerminalDiscoveryPrivateV2(raw []byte) (v TerminalDiscoveryInspectionPrivateV2, err error) {
	if len(raw) == 0 || len(raw) > scopeMetadataPrivateV2 {
		return v, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&v) != nil {
		return v, ErrInvalid
	}
	canonical, e := json.Marshal(v)
	if e != nil || !bytes.Equal(canonical, raw) {
		return v, ErrInvalid
	}
	if v.Schema != TerminalDiscoverySchemaPrivateV2 || v.InputFamily != "TERMINAL_ACCOUNTING_NOT_NATIVE" || v.Authority != 0 || v.Accepted || v.FeatureIdentity != "UNRESOLVED" || v.SourceCompleteness != "UNKNOWN" || v.AbsenceInference != "PROHIBITED" || Digest(v.FrozenManifest) != v.ScopeSHA256 || Digest(v.EffectivePolicy) != v.PolicySHA256 {
		return v, ErrInvalid
	}
	discovery, _ := json.Marshal(v.Discovery)
	if Digest(discovery) != v.DiscoverySHA256 {
		return v, ErrInvalid
	}
	var p ScopeEffectivePolicyPrivateV2
	var spec ScopeSpecPrivateV2
	var manifest scopeManifestPrivateV2
	var metadata sessionruntime.SessionMetadata
	if producerPacketStrictPrivateV2(v.EffectivePolicy, &p) != nil || p.validate() != nil || producerPacketStrictPrivateV2(v.FrozenSpec, &spec) != nil || producerPacketStrictPrivateV2(v.FrozenManifest, &manifest) != nil || producerPacketStrictPrivateV2(v.RuntimeMetadata, &metadata) != nil {
		return v, ErrInvalid
	}
	_, canonicalSpec, e := validateScopeSpecPrivateV2(spec)
	policy, _ := json.Marshal(spec.EffectivePolicy)
	if e != nil || !bytes.Equal(canonicalSpec, v.FrozenSpec) || !bytes.Equal(policy, v.EffectivePolicy) || spec.Root != v.Discovery.Workspace || len(spec.Members) != len(manifest.Files) || manifest.RootIdentity.Inode == 0 || manifest.Schema != "lsp-trace.frozen-scope.private.v2" || manifest.Authority != 0 || manifest.Accepted || manifest.Feature != "UNRESOLVED" || manifest.Source != "UNKNOWN" || manifest.Absence != "PROHIBITED" || manifest.RevisionQualification != "CALLER_ASSERTED_NOT_AUTHENTICATED" || manifest.SourceRootClaim != sourceRootNoAtomicSnapshotPrivateV2 || manifest.SpecSHA256 != Digest(v.FrozenSpec) || manifest.SelectionSHA256 != Digest(v.NominationSelection) || manifest.FilePlanSHA256 != Digest(v.FilePlan) {
		return v, ErrInvalid
	}
	if !metadata.CallHierarchySupport || !metadata.DocumentSymbolSupport || (metadata.PositionEncoding != "utf-8" && metadata.PositionEncoding != "utf-16" && metadata.PositionEncoding != "utf-32") {
		return v, ErrInvalid
	}
	var nominations []censusacquisition.SourceFile
	var query []string
	identities := map[scopeFileIdentity]bool{}
	for i, m := range spec.Members {
		r := manifest.Files[i]
		uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(spec.Root, m.CanonicalRelativePath))}).String()
		if r.Path != m.CanonicalRelativePath || r.Role != m.Role || r.URI != uri || r.SHA256 != m.ExpectedSHA256 || r.ByteLength != m.ByteLength || r.Identity.Inode == 0 || identities[r.Identity] || !os.FileMode(r.Mode).IsRegular() {
			return v, ErrInvalid
		}
		identities[r.Identity] = true
		query = append(query, uri)
		if m.Role == "NOMINATION" {
			nominations = append(nominations, censusacquisition.SourceFile{Path: r.Path, URI: uri, LanguageID: m.LanguageID})
		}
	}
	sort.Strings(query)
	if !reflect.DeepEqual(query, v.CanonicalQueryURIs) || len(nominations) != v.Discovery.Accounting.FileDenominator {
		return v, ErrInvalid
	}
	selection, e := NewSourceSelection(spec.Root, nominations)
	if e != nil {
		return v, e
	}
	selectionRaw, e := EncodeSourceSelection(selection)
	if e != nil || !bytes.Equal(selectionRaw, v.NominationSelection) {
		return v, ErrInvalid
	}
	plan, e := NewPartitionPlan(selection, spec.PartitionSize)
	if e != nil {
		return v, e
	}
	planRaw, e := EncodePartitionPlan(plan, selection)
	if e != nil || !bytes.Equal(planRaw, v.FilePlan) {
		return v, ErrInvalid
	}
	if e := validateTerminalSourceLedgerPrivateV2(v.Discovery, nominations, p.Discovery.MaxSymbols, p.Discovery.MaxStringBytes); e != nil {
		return v, e
	}
	reason, e := terminalDiscoveryReasonPrivateV2(v.Discovery, p.MaxTargets, v.Discovery.Accounting.FileDenominator)
	if e != nil || reason != v.Status {
		return v, ErrInvalid
	}
	return v, nil
}
