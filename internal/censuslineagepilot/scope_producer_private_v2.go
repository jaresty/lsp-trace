package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/sessionruntime"
)

var ErrScopeDiscoveryIncompletePrivateV2 = errors.New("private V2 discovery incomplete; no origin")
var ErrScopeDiscoveryEmptyPrivateV2 = errors.New("private V2 discovery empty; no origin")

// The only V2 constructor accepts opaque frozen provenance, a concrete Manager,
// and the original active lease context. No caller-controlled version switch.
func newScopeRuntimeAdapterPrivateV2(ctx context.Context, f FrozenScopePrivateV2, m *sessionruntime.Manager, id censusacquisition.SessionIdentity) (*scopeRuntimeAdapter, error) {
	return newScopeRuntimeAdapterObservedPrivateV2(ctx, f, m, id, nil)
}

func newScopeRuntimeAdapterObservedPrivateV2(ctx context.Context, f FrozenScopePrivateV2, m *sessionruntime.Manager, id censusacquisition.SessionIdentity, observation *ScopeFailureObservationPrivateV2) (adapter *scopeRuntimeAdapter, err error) {
	stage, member := "ADAPTER_ADMISSION", -1
	subcause := "UNKNOWN"
	defer func() {
		if err != nil && observation != nil {
			*observation = ScopeFailureObservationPrivateV2{Stage: stage, Class: ScopeFailureClassPrivateV2(err), Subcause: subcause, DiscoveryAvailability: "NOT_REACHED_DISCOVERY"}
			if member >= 0 {
				n := member
				observation.MemberOrdinal = &n
			}
		}
	}()
	started := time.Now()
	if ctx == nil || m == nil || !f.valid() || id.Validate() != nil || len(id.SessionID) > 1024 {
		return nil, ErrInvalid
	}
	p := f.state.spec.EffectivePolicy
	if p.validate() != nil {
		return nil, ErrInvalid
	}
	deadline := started.Add(time.Duration(p.TimeoutMS) * time.Millisecond)
	constructionCtx, constructionCancel := context.WithDeadline(ctx, deadline)
	defer constructionCancel()
	lease := sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: f.state.spec.Root}
	checkActive := func() error {
		stage = "ADAPTER_ACTIVE_LEASE"
		if failure := m.CheckActiveSessionLeasePrivateV2(constructionCtx, lease); failure != "" {
			return scopeAdapterFailure(failure, "")
		}
		return nil
	}
	// Generic guard admission intentionally permits ordinary tokenless use.
	// This producer instead requires the exact active token BEFORE source IO.
	if err := checkActive(); err != nil {
		return nil, err
	}
	stage = "ADAPTER_SOURCE_REVALIDATION"
	if err := f.revalidate(constructionCtx); err != nil {
		return nil, err
	}
	stage = "ADAPTER_WORKSPACE_BINDING"
	root, ok := m.WorkspaceRoot(id.SessionID, id.Generation)
	if !ok || root != f.state.spec.Root {
		return nil, ErrInvalid
	}
	stage = "ADAPTER_METADATA"
	metadata, failure := m.Metadata(id.SessionID, id.Generation)
	if failure != "" {
		return nil, scopeAdapterFailure(failure, "")
	}
	// Store the ORIGINAL lease lifetime, never the canceled construction child.
	// The shared overall deadline starts at constructor entry and is not reset.
	a := &scopeRuntimeAdapter{manager: m, identity: id, metadata: metadata, workspace: root, frozenV2: f, lifetimeV2: ctx, deadline: deadline}
	members := make([]sessionruntime.RetainedScopeMemberPrivate, 0, len(f.state.files))
	for i, file := range f.state.files {
		member = i
		if err := checkActive(); err != nil {
			return nil, err
		}
		stage = "ADAPTER_SOURCE_REVALIDATION"
		if err := f.revalidate(constructionCtx); err != nil {
			return nil, err
		}
		language := f.state.spec.Members[i].LanguageID
		supplied := func() sessionruntime.DocumentResult {
			stage = "ADAPTER_MEMBER_ACTIVE_LEASE"
			requestCtx, cancel := context.WithTimeout(constructionCtx, time.Duration(p.RequestTimeoutMS)*time.Millisecond)
			defer cancel()
			if failure := m.CheckActiveSessionLeasePrivateV2(requestCtx, lease); failure != "" {
				return sessionruntime.DocumentResult{Failure: failure}
			}
			stage = "ADAPTER_MEMBER_PREPARE"
			return m.PrepareDocument(requestCtx, sessionruntime.DocumentRequest{SessionID: id.SessionID, Generation: id.Generation, URI: file.URI, LanguageID: language, CaptureSupply: true})
		}()
		if supplied.Failure != "" {
			subcause = scopeFailureSubcausePrivateV2(supplied.LifecycleSubcause)
			return nil, scopeAdapterFailure(supplied.Failure, supplied.LifecycleSubcause)
		}
		stage = "ADAPTER_MEMBER_SUPPLY_VALIDATION"
		if !scopeAdapterSupplyMatchesForLanguagePrivate(supplied, file, id, language) || !scopeSupplyKeysPrivateV2(supplied) {
			return nil, ErrInvalid
		}
		if err := checkActive(); err != nil {
			return nil, err
		}
		stage = "ADAPTER_SOURCE_REVALIDATION"
		if err := f.revalidate(constructionCtx); err != nil {
			return nil, err
		}
		role := sessionruntime.RetainedScopeRolePrivate(file.Role)
		members = append(members, sessionruntime.RetainedScopeMemberPrivate{URI: file.URI, LanguageID: language, Expected: file.Bytes, Role: role})
		if role == sessionruntime.RetainedScopeNominationPrivate {
			a.files = append(a.files, censusacquisition.SourceFile{Path: file.Path, URI: file.URI, LanguageID: language})
		}
	}
	member = -1
	if err := checkActive(); err != nil {
		return nil, err
	}
	stage = "ADAPTER_RETAINED_GUARD_BINDING"
	a.guardV2, failure = m.BindRetainedScopePrivateV2(constructionCtx, id.SessionID, id.Generation, members)
	if failure != "" {
		return nil, scopeAdapterFailure(failure, "")
	}
	stage = "ADAPTER_GUARD_REVALIDATION"
	if err := a.check(constructionCtx); err != nil {
		return nil, err
	}
	return a, nil
}
func scopeSupplyKeysPrivateV2(r sessionruntime.DocumentResult) bool {
	if r.Supply == nil {
		return false
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(r.Supply.Params, &top) != nil {
		return false
	}
	allowed := func(m map[string]json.RawMessage, keys ...string) bool {
		for k := range m {
			found := false
			for _, a := range keys {
				if k == a {
					found = true
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	if !allowed(top, "textDocument", "contentChanges") {
		return false
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(top["textDocument"], &doc) != nil {
		return false
	}
	if r.Supply.Method == "textDocument/didOpen" {
		return len(top) == 1 && allowed(doc, "uri", "languageId", "version", "text")
	}
	if r.Supply.Method != "textDocument/didChange" || len(top) != 2 || !allowed(doc, "uri", "version") {
		return false
	}
	var changes []map[string]json.RawMessage
	if json.Unmarshal(top["contentChanges"], &changes) != nil || len(changes) != 1 {
		return false
	}
	return allowed(changes[0], "text")
}

// Reports are detached observations, not origin constructors or live authority.
type ScopeDiscoveryReportPrivateV2 struct {
	raw     []byte
	failure *ScopeFailureObservationPrivateV2
}

func (r ScopeDiscoveryReportPrivateV2) Bytes() []byte { return bytes.Clone(r.raw) }

type EnumerationOriginPrivateV2 struct {
	state *enumerationOriginStatePrivateV2
}
type enumerationOriginStatePrivateV2 struct {
	self      *enumerationOriginStatePrivateV2
	adapter   *scopeRuntimeAdapter
	manager   *sessionruntime.Manager
	guard     *sessionruntime.RetainedScopeGuardPrivateV2
	identity  censusacquisition.SessionIdentity
	frozen    FrozenScopePrivateV2
	lifetime  context.Context
	raw       []byte
	queryURIs []string
	batches   []*batchSelectionStatePrivateV2
}
type originFramePrivateV2 struct {
	Schema             string                           `json:"schema"`
	Authority          uint8                            `json:"authority"`
	Accepted           bool                             `json:"accepted"`
	Feature            string                           `json:"feature_identity"`
	Source             string                           `json:"source_completeness"`
	Absence            string                           `json:"absence_inference"`
	ScopeSHA256        string                           `json:"scope_sha256"`
	SpecSHA256         string                           `json:"spec_sha256"`
	SelectionSHA256    string                           `json:"selection_sha256"`
	FilePlanSHA256     string                           `json:"file_plan_sha256"`
	PolicySHA256       string                           `json:"effective_policy_sha256"`
	EnumerationSHA256  string                           `json:"enumeration_sha256"`
	Enumeration        json.RawMessage                  `json:"actual_enumeration"`
	CanonicalQueryURIs []string                         `json:"canonical_query_uris"`
	PlannedBatches     []censusacquisition.BatchRequest `json:"target_batches"`
}

func (o EnumerationOriginPrivateV2) valid() bool {
	return o.state != nil && o.state.self == o.state && o.state.adapter != nil && o.state.frozen.valid() && o.state.adapter.frozenV2.state == o.state.frozen.state && o.state.manager == o.state.adapter.manager && o.state.guard != nil && o.state.guard == o.state.adapter.guardV2 && o.state.adapter.guard == nil && o.state.identity == o.state.adapter.identity && o.state.lifetime == o.state.adapter.lifetimeV2
}
func (o EnumerationOriginPrivateV2) Bytes() []byte {
	if !o.valid() {
		return nil
	}
	return bytes.Clone(o.state.raw)
}
func (o EnumerationOriginPrivateV2) BatchCount() int {
	if !o.valid() {
		return 0
	}
	return len(o.state.batches)
}
func (EnumerationOriginPrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }

// Exactly one discovery attempt per frozen owner; no automatic rescan, retry,
// reset, caller DTO constructor, or cross-constituent graph merge.
func DiscoverScopePrivateV2(ctx context.Context, f FrozenScopePrivateV2, m *sessionruntime.Manager, id censusacquisition.SessionIdentity) (EnumerationOriginPrivateV2, ScopeDiscoveryReportPrivateV2, error) {
	return discoverScopeOutcomePrivateV2(ctx, f, m, id, nil)
}

func discoverScopeOutcomePrivateV2(ctx context.Context, f FrozenScopePrivateV2, m *sessionruntime.Manager, id censusacquisition.SessionIdentity, terminal *TerminalDiscoveryOutcomePrivateV2) (origin EnumerationOriginPrivateV2, report ScopeDiscoveryReportPrivateV2, err error) {
	stage, availability := "DISCOVERY_ADMISSION", "NOT_REACHED_ADAPTER"
	defer func() {
		if err != nil && report.failure == nil {
			report.failure = &ScopeFailureObservationPrivateV2{Stage: stage, Class: ScopeFailureClassPrivateV2(err), Subcause: "UNKNOWN", DiscoveryAvailability: availability}
		}
	}()
	if !f.valid() || ctx == nil || m == nil {
		return EnumerationOriginPrivateV2{}, report, ErrInvalid
	}
	f.state.discoveryMu.Lock()
	if f.state.discoveryAttempted {
		f.state.discoveryMu.Unlock()
		return EnumerationOriginPrivateV2{}, report, ErrInvalid
	}
	f.state.discoveryAttempted = true
	f.state.discoveryMu.Unlock()
	var constructorFailure ScopeFailureObservationPrivateV2
	a, err := newScopeRuntimeAdapterObservedPrivateV2(ctx, f, m, id, &constructorFailure)
	if err != nil {
		report.failure = &constructorFailure
		return EnumerationOriginPrivateV2{}, report, err
	}
	stage, availability = "DOCUMENT_SYMBOL_DISCOVERY", "DISCOVERY_REPORT_UNAVAILABLE"
	p := f.state.spec.EffectivePolicy
	d, err := a.discoverWithLimitsPrivate(ctx, id, p.Discovery)
	raw, encodingErr := json.Marshal(d)
	if encodingErr == nil && len(raw) <= scopeMetadataPrivateV2 {
		report.raw = bytes.Clone(raw)
		availability = "AVAILABLE_ERROR_REPORT"
	} else if encodingErr != nil {
		availability = "DISCOVERY_ENCODING_FAILED"
	} else {
		availability = "DISCOVERY_ENCODING_BOUND"
	}
	if err != nil {
		return EnumerationOriginPrivateV2{}, report, err
	}
	if encodingErr != nil || len(raw) > scopeMetadataPrivateV2 {
		stage = "DISCOVERY_SERIALIZATION"
		return EnumerationOriginPrivateV2{}, report, ErrInvalid
	}
	// Only this opt-in path, during the actual original attempt, can mint a
	// terminal owner. The legacy API reaches exactly its old guards below.
	if terminal != nil && (len(d.Targets) == 0 || len(d.Targets) > p.MaxTargets) {
		stage = "TERMINAL_DISCOVERY"
		owned, e := newTerminalDiscoveryOutcomePrivateV2(ctx, a, d, raw)
		if e != nil {
			return EnumerationOriginPrivateV2{}, report, e
		}
		*terminal = owned
		return EnumerationOriginPrivateV2{}, report, nil
	}
	stage = "DISCOVERY_ACCOUNTING"
	if !d.Complete || d.Accounting.FileDenominator != len(a.files) {
		return EnumerationOriginPrivateV2{}, report, ErrScopeDiscoveryIncompletePrivateV2
	}
	if len(d.Targets) == 0 {
		return EnumerationOriginPrivateV2{}, report, ErrScopeDiscoveryEmptyPrivateV2
	}
	if len(d.Targets) > p.MaxTargets {
		return EnumerationOriginPrivateV2{}, report, ErrScopeDiscoveryIncompletePrivateV2
	}
	stage = "DISCOVERY_SCOPE_REVALIDATION"
	if err = a.check(ctx); err != nil {
		return EnumerationOriginPrivateV2{}, report, err
	}
	stage = "DISCOVERY_BATCH_PLANNING"
	batches, err := (censusacquisition.Core{Planning: &p.Planning}).PlanDiscovery(id, d)
	if err != nil {
		return EnumerationOriginPrivateV2{}, report, err
	}
	if len(batches) == 0 {
		return EnumerationOriginPrivateV2{}, report, ErrScopeDiscoveryEmptyPrivateV2
	}
	stage = "DISCOVERY_SOURCE_BINDING"
	boundSource, err := graphprovenance.BindSourcePrivateV2(ctx, m, a.guardV2, id.SessionID, id.Generation, a.workspace)
	if err != nil {
		return EnumerationOriginPrivateV2{}, report, err
	}
	stage = "DISCOVERY_QUERY_DECLARATION"
	declaration, err := boundSource.FrozenQueryDeclarationPrivateV2(ctx)
	if err != nil {
		return EnumerationOriginPrivateV2{}, report, err
	}
	queryURIs := declaration.MembersPrivateV2()
	policyBytes, _ := json.Marshal(p)
	frame := originFramePrivateV2{Schema: "lsp-trace.enumeration-origin.private.v2", Feature: "UNRESOLVED", Source: "UNKNOWN", Absence: "PROHIBITED", ScopeSHA256: Digest(f.state.manifest), SpecSHA256: Digest(f.state.specBytes), SelectionSHA256: Digest(f.state.selection), FilePlanSHA256: Digest(f.state.filePlan), PolicySHA256: Digest(policyBytes), EnumerationSHA256: Digest(raw), Enumeration: raw, CanonicalQueryURIs: queryURIs, PlannedBatches: batches}
	stage = "DISCOVERY_ORIGIN_ENCODING"
	originBytes, err := json.Marshal(frame)
	if err != nil || len(originBytes) > scopeMetadataPrivateV2 {
		return EnumerationOriginPrivateV2{}, report, ErrInvalid
	}
	st := &enumerationOriginStatePrivateV2{adapter: a, manager: m, guard: a.guardV2, identity: id, frozen: f, lifetime: ctx, raw: originBytes, queryURIs: queryURIs}
	st.self = st
	origin = EnumerationOriginPrivateV2{st}
	stage = "DISCOVERY_BATCH_BINDING"
	for _, batch := range batches {
		manifest := batch.AcquisitionManifest(p.acquisitionLimits())
		input, err := json.Marshal(scopedBatchInputPrivate{SessionID: id.SessionID, Generation: id.Generation, Manifest: manifest, OutputVersion: graphprovenance.VersionV5})
		if err != nil {
			return EnumerationOriginPrivateV2{}, report, err
		}
		binding := acquisitionauthority.Binding{Operation: acquisitionmanifest.SliceV3, Route: "private-census-batch", RequestID: batch.BatchID, Workspace: a.workspace, SessionID: id.SessionID, Generation: id.Generation, Input: input}
		if _, err = validateScopedBatchBindingPrivate(a, binding); err != nil {
			return EnumerationOriginPrivateV2{}, report, err
		}
		authority, err := acquisitionauthority.MintSeedAuthority(binding, batch.CanonicalSeedsV2, seedbinding.CallerAssertedLocal, false, "", true)
		if err != nil {
			return EnumerationOriginPrivateV2{}, report, err
		}
		s := &batchSelectionStatePrivateV2{origin: st, binding: binding, seeds: bytes.Clone(batch.CanonicalSeedsV2), authority: authority, ordinal: batch.Ordinal}
		s.self = s
		st.batches = append(st.batches, s)
	}
	stage = "DISCOVERY_FINAL_REVALIDATION"
	if err = a.check(ctx); err != nil {
		return EnumerationOriginPrivateV2{}, report, err
	}
	return origin, report, nil
}
