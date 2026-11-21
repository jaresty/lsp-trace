package inventorysuccessor

import (
	"context"
	"fmt"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	rg "lsp-trace/internal/referencegrouping"
	"slices"
	"time"
)

type inventorySelectedFundingV1 struct {
	decision string
	ceiling  countersV2
}

// OpenLocalFolderSelectedV1 admits the existing M/P/S schedule for one actual
// retained query. No provider window is available. The three original stage
// formulas, distinct attempts, single original deadline and cumulative accounting
// remain in force. Neither selectors nor configuration can manufacture originals.
func OpenLocalFolderSelectedV1(ctx context.Context, root *publication.Root, acquisition PrivateDiscoveryOperationV2, index int, native PrivateRetainedNativeCallsV1, config PrivateSelectedConfigurationV2) (*PrivateManagedInputOwnerV2, PrivateMixedInputRequestV2, error) {
	return openLocalFolderSelectedV1(ctx, root, acquisition, index, native, config, nil)
}

func openLocalFolderSelectedV1(ctx context.Context, root *publication.Root, acquisition PrivateDiscoveryOperationV2, index int, native PrivateRetainedNativeCallsV1, config PrivateSelectedConfigurationV2, funding *inventorySelectedFundingV1) (*PrivateManagedInputOwnerV2, PrivateMixedInputRequestV2, error) {
	fail := func(e error) (*PrivateManagedInputOwnerV2, PrivateMixedInputRequestV2, error) {
		if e == nil {
			e = ErrPrivateSelectedV2
		}
		return nil, PrivateMixedInputRequestV2{}, e
	}
	if ctx == nil || ctx.Err() != nil || root == nil || acquisition.state == nil || index < 0 || index >= 256 || !selectedConfigValidV2(config) {
		return fail(nil)
	}
	config, e := selectedCheckedContractV2(config)
	if e != nil {
		return fail(e)
	}
	s := acquisition.state
	s.mu.Lock()
	descriptor, ok := s.observation.OwnedMethods.GroupingQuery(index)
	if !s.completed || s.executionErr != nil || !s.observation.CleanupJoined || s.observation.Failed || s.selectedClaimed[index] || !ok || root == s.owner.k.root {
		s.mu.Unlock()
		return fail(nil)
	}
	kind := pa.TypedCalls
	switch descriptor.Method {
	case "textDocument/definition":
		kind = pa.TypedResolvesToDefinition
	case "textDocument/references":
		kind = pa.TypedReferencesSymbol
	case "CALLS":
		if native.state == nil || native.state.acquisition != s || index != 128+native.state.index {
			s.mu.Unlock()
			return fail(nil)
		}
	default:
		s.mu.Unlock()
		return fail(nil)
	}
	if native.state != nil && (native.state.acquisition != s || native.state.summary.URI != descriptor.URI) {
		s.mu.Unlock()
		return fail(nil)
	}
	if config.ExpectedSourceAuthorization != "" && config.ExpectedSourceAuthorization != s.binding.DenominatorDigest {
		s.mu.Unlock()
		return fail(nil)
	}
	s.selectedClaimed[index] = true
	start := s.construction.Start
	start.Deadline = s.binding.Deadline // original whole-acquisition cutoff, never renewal
	r := &privateRetainedMixedInputV1{acquisition: s, owned: s.observation.OwnedMethods, index: index, descriptor: descriptor, native: native.state, root: root, start: start, occurrence: hashLabelV2(fmt.Sprintf("folder-retained-query/%s/%d", s.attempt, index))}
	binding, originalAttempt := s.binding, s.attempt
	s.mu.Unlock()
	q := PrivateMixedInputRequestV2{retained: r, Root: root, Start: start, URI: descriptor.URI, OccurrenceID: r.occurrence, Revision: binding.Revision, Privacy: "PRIVATE", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: descriptor.Line, Character: descriptor.Character, Kinds: []pa.RelationKind{kind}}
	q.Start.Process.Args = slices.Clone(start.Process.Args)
	q.Start.Process.Env = slices.Clone(start.Process.Env)
	q.Start.RelationProviders = slices.Clone(start.RelationProviders)
	if kind != pa.TypedCalls && native.state != nil {
		q.Kinds = append(q.Kinds, pa.TypedCalls)
	}
	now := time.Now()
	m, e := privateRetainedMixedQuoteV1(now, binding.Deadline)
	if e != nil {
		return fail(e)
	}
	p, e := selectedPreparationChargeV2(rg.AllModeMetadataPrivate{Input: rg.CompositionCommitment{ByteLength: 1 << 20}, Contributions: 1024, Custody: 2048, Predecessors: 1024}, now, binding.Deadline)
	if e != nil {
		return fail(e)
	}
	limits := config.Contract.CaptureLimits()
	source, e := selectedSourceChargeV2(1024, uint64(limits.MaxArtifactBytes), uint64(limits.MaxParentBytes), now, binding.Deadline)
	if e != nil {
		return fail(e)
	}
	total, e := addCountersV2(m, p)
	if e != nil {
		return fail(e)
	}
	total, e = addCountersV2(total, source)
	if e != nil {
		return fail(e)
	}
	maximum := m.vector()
	for _, stage := range []countersV2{p, source} {
		for i, n := range stage.vector() {
			if n > maximum[i] {
				maximum[i] = n
			}
		}
	}
	tranche := countersFromV2(maximum)
	domain := "local-folder-selected.v1/"
	stable := hashLabelV2(fmt.Sprintf("%s%s/%d", domain, originalAttempt, index))
	j := jobV2{Schema: jobSchemaV2, StableID: stable, Application: hashLabelV2(domain + "census"), Scope: hashLabelV2(domain + "one-retained-query-MPS"), SourceAuthorization: binding.DenominatorDigest, RevisionPolicy: hashLabelV2(domain + binding.Revision), EvidencePolicies: []string{hashLabelV2(domain + "runtime-originals")}, ProviderPolicies: []string{hashLabelV2(domain + binding.RuntimeFingerprint)}, SemanticPolicies: []string{hashLabelV2(domain + "existing-selected-MPS")}, Privacy: hashLabelV2(domain + "private"), Retention: hashLabelV2(domain + "one-shot"), ResourceRules: hashLabelV2(domain + "existing-selected-quotes-no-provider"), Origin: "NEW_EMPTY"}
	job, e := sealV2(&j)
	if e != nil {
		return fail(e)
	}
	decision := hashLabelV2(domain + "invocation-consent/" + stable)
	if funding != nil {
		if !hashV2(funding.decision) || !total.within(funding.ceiling) {
			return fail(ErrPrivateSelectedBoundsV2)
		}
		decision = funding.decision
		total = funding.ceiling
	}
	a := authorizationV2{Schema: authorizationSchemaV2, Job: j.Identity, Revision: 1, Previous: noneV2, BoundHead: noneV2, Ceiling: total, MaxTranche: tranche, MaxWindows: 3, MaxWindowNS: tranche.ActiveNS, Expires: binding.Deadline.UnixNano(), Decision: decision, Assurance: assuranceV2}
	auth, e := sealV2(&a)
	if e != nil {
		return fail(e)
	}
	verification := PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}
	ready, e := InitializePrivateReadyV2(ctx, root, job, auth, PrivateReadyDecisionAssertionV2{Job: j.Identity, Authorization: a.Identity, Reference: decision}, verification)
	if e != nil {
		return fail(e)
	}
	owner, e := OpenPrivateManagedInputOwnerV2(ctx, root, job, ready.ConfirmedHead(), verification)
	if e != nil {
		return fail(e)
	}
	return owner, q, nil
}
