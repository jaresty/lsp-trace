package censuscontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/publication"
)

const (
	programCV2MeasurementOptInEnv = "LSP_TRACE_ADR0014_V2_MEASURE"
	programCV2MeasurementHeadNS   = "census-program-c-transition/heads/private/v1"
)

type programCV2Measurement struct {
	Constituents                               uint64 `json:"constituents"`
	CompleteConstituents                       uint64 `json:"complete_constituents"`
	EmptyConstituents                          uint64 `json:"empty_constituents"`
	ResourceIncompleteConstituents             uint64 `json:"resource_incomplete_constituents"`
	EligibleRuns                               uint64 `json:"eligible_runs"`
	EligibleRunCommunityResultsD               uint64 `json:"eligible_run_community_results_d"`
	SeedGroupResultsG                          uint64 `json:"seed_group_results_g"`
	MaxSeedGroupsPerEligibleSource             uint64 `json:"max_seed_groups_per_eligible_source"`
	MaxMembershipsPerEligibleSource            uint64 `json:"max_memberships_per_eligible_source"`
	MaxMembershipsPerSeedGroup                 uint64 `json:"max_memberships_per_seed_group"`
	MaxGroupOccurrenceIDsPerSeedGroup          uint64 `json:"max_group_occurrence_ids_per_seed_group"`
	MaxSumGroupOccurrencesPerEligibleSource    uint64 `json:"max_sum_group_occurrences_per_eligible_source"`
	MaxRelevantStringBytes                     uint64 `json:"max_relevant_string_bytes"`
	MaxCommunitiesPerRun                       uint64 `json:"max_communities_per_run"`
	MaxTotalPartitionMembersPerRun             uint64 `json:"max_total_partition_members_per_run"`
	MaxCandidatesPerCommunity                  uint64 `json:"max_candidates_per_community"`
	MaxCompositionOccurrences                  uint64 `json:"max_composition_occurrences"`
	MaxWorkV2PerOuterRunCommunityResult        uint64 `json:"max_work_v2_per_outer_run_community_result"`
	MaxWorkV2PerRunSource                      uint64 `json:"max_work_v2_per_run_source"`
	TotalSeedGroupsAcrossEligibleSources       uint64 `json:"total_seed_groups_across_eligible_sources"`
	TotalMembershipsAcrossEligibleSources      uint64 `json:"total_memberships_across_eligible_sources"`
	TotalGroupOccurrencesAcrossEligibleSources uint64 `json:"total_group_occurrences_across_eligible_sources"`
	TotalCompositionOccurrencesAcrossRuns      uint64 `json:"total_composition_occurrences_across_runs"`
	TotalPartitionMembersAcrossRuns            uint64 `json:"total_partition_members_across_runs"`
	TotalCommunityCandidatesAcrossResults      uint64 `json:"total_community_candidates_across_results"`
	TotalWorkV2AcrossOuterResults              uint64 `json:"total_work_v2_across_outer_results"`
	TotalWorkV2AcrossRunSources                uint64 `json:"total_work_v2_across_run_sources"`
}

type programCV2SeedKey struct {
	ExecutionBundleID string
	SeedLabel         string
	SeedAt            string
}

type programCV2SeedGroup struct {
	Key             programCV2SeedKey
	Memberships     []graph.SeedMembership
	PreparedTargets []string
	ReachedNodes    []string
	RelationIDs     []string
	OccurrenceIDs   []string
}

type programCV2SourceMeasurement struct {
	Groups                 []programCV2SeedGroup
	MembershipCount        uint64
	MaxMembershipsPerGroup uint64
	MaxGroupOccurrences    uint64
	SumGroupOccurrences    uint64
	MaxRelevantStringBytes uint64
}

type programCV2ReadOnlyObjects struct {
	inner censussourcecontinuation.ImmutableStore
	gets  uint64
	puts  uint64
}

func (s *programCV2ReadOnlyObjects) Get(ctx context.Context, selector string) ([]byte, error) {
	s.gets++
	return s.inner.Get(ctx, selector)
}
func (s *programCV2ReadOnlyObjects) Put(context.Context, []byte) (string, error) {
	s.puts++
	return "", errors.New("ADR0014 measurement object write prohibited")
}

type programCV2ReadOnlyHeads struct {
	inner censussourcecontinuation.HeadStore
	loads uint64
	cas   uint64
}

func (s *programCV2ReadOnlyHeads) Load(ctx context.Context, key string) ([]byte, error) {
	s.loads++
	return s.inner.Load(ctx, key)
}
func (s *programCV2ReadOnlyHeads) CompareAndSwap(context.Context, string, censussourcecontinuation.HeadRevision, []byte) censussourcecontinuation.CASResult {
	s.cas++
	return censussourcecontinuation.CASResult{Err: errors.New("ADR0014 measurement head CAS prohibited")}
}

type programCV2ExactFileReader struct{ path string }

func (r programCV2ExactFileReader) ReadBounded(_ context.Context, limit int64) ([]byte, error) {
	info, err := os.Lstat(r.path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limit {
		return nil, ErrInvalidProgramCTransition
	}
	return os.ReadFile(r.path)
}

func programCV2ExactInput(path string) (ExactProgramCTransitionInput, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ExactProgramCTransitionInput{}, err
	}
	return ExactProgramCTransitionInput{Reader: programCV2ExactFileReader{path: path}, ByteLength: int64(len(raw)), SHA256: transitionSHA(raw)}, nil
}

// observeProgramCV2FrozenCorpus is test-only. The completion verifier returns no
// inventory until the reconstructed terminal bytes equal the one loaded terminal
// head, so no accounting is observable before that equality barrier.
func observeProgramCV2FrozenCorpus(ctx context.Context, request ProgramCTransitionVerificationRequest) (programCV2Measurement, error) {
	if ctx == nil || request.PlanSource == nil || request.PlanSource.VerifyPublication != nil {
		return programCV2Measurement{}, ErrInvalidProgramCTransition
	}
	planRaw, err := request.PlanSource.Load(ctx)
	if err != nil {
		return programCV2Measurement{}, err
	}
	runner := NewProgramCProductionRunner(request.PlanSource)
	result, inventory, err := verifyProgramCTransitionReceiptInventory(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, request.Objects, request.Heads, runner)
	if err != nil {
		return programCV2Measurement{}, err
	}
	if result.Status != ProgramCTransitionComplete || result.CommittedPrefix != len(inventory) || result.ConstituentCount != len(inventory) {
		return programCV2Measurement{}, ErrInvalidProgramCTransition
	}

	measurement := programCV2Measurement{Constituents: uint64(len(inventory))}
	for ordinal := range inventory {
		entry := inventory[ordinal]
		classification, err := classifyProgramCV2MeasurementEntry(entry)
		if err != nil {
			return programCV2Measurement{}, fmt.Errorf("ordinal=%d: %w", ordinal, err)
		}
		switch classification {
		case programCStage6Empty:
			measurement.EmptyConstituents++
			continue
		case programCStage6ResourceIncomplete:
			measurement.ResourceIncompleteConstituents++
			continue
		case programCStage6Complete:
			measurement.CompleteConstituents++
		default:
			return programCV2Measurement{}, ErrInvalidProgramCTransition
		}

		sourceMeasurement, err := measureProgramCV2Source(entry)
		if err != nil {
			return programCV2Measurement{}, fmt.Errorf("ordinal=%d: %w", ordinal, err)
		}
		seedGroups := uint64(len(sourceMeasurement.Groups))
		measurement.MaxSeedGroupsPerEligibleSource = maximumObserved(measurement.MaxSeedGroupsPerEligibleSource, seedGroups)
		measurement.MaxMembershipsPerEligibleSource = maximumObserved(measurement.MaxMembershipsPerEligibleSource, sourceMeasurement.MembershipCount)
		measurement.MaxMembershipsPerSeedGroup = maximumObserved(measurement.MaxMembershipsPerSeedGroup, sourceMeasurement.MaxMembershipsPerGroup)
		measurement.MaxGroupOccurrenceIDsPerSeedGroup = maximumObserved(measurement.MaxGroupOccurrenceIDsPerSeedGroup, sourceMeasurement.MaxGroupOccurrences)
		measurement.MaxSumGroupOccurrencesPerEligibleSource = maximumObserved(measurement.MaxSumGroupOccurrencesPerEligibleSource, sourceMeasurement.SumGroupOccurrences)
		measurement.MaxRelevantStringBytes = maximumObserved(measurement.MaxRelevantStringBytes, sourceMeasurement.MaxRelevantStringBytes)
		if err := addProgramCV2(&measurement.TotalSeedGroupsAcrossEligibleSources, seedGroups); err != nil {
			return programCV2Measurement{}, err
		}
		if err := addProgramCV2(&measurement.TotalMembershipsAcrossEligibleSources, sourceMeasurement.MembershipCount); err != nil {
			return programCV2Measurement{}, err
		}
		if err := addProgramCV2(&measurement.TotalGroupOccurrencesAcrossEligibleSources, sourceMeasurement.SumGroupOccurrences); err != nil {
			return programCV2Measurement{}, err
		}

		for _, outcome := range entry.outcomes {
			measurement.EligibleRuns++
			communities := uint64(len(outcome.Communities))
			occurrences := uint64(len(outcome.Projection.Occurrences))
			measurement.MaxCommunitiesPerRun = maximumObserved(measurement.MaxCommunitiesPerRun, communities)
			measurement.MaxCompositionOccurrences = maximumObserved(measurement.MaxCompositionOccurrences, occurrences)
			partitionMembers := uint64(0)
			for _, community := range outcome.Communities {
				if len(community.Members) == 0 {
					return programCV2Measurement{}, ErrInvalidProgramCTransition
				}
				candidates := uint64(len(community.Members))
				if err := addProgramCV2(&partitionMembers, candidates); err != nil {
					return programCV2Measurement{}, err
				}
				measurement.MaxCandidatesPerCommunity = maximumObserved(measurement.MaxCandidatesPerCommunity, candidates)
				measurement.EligibleRunCommunityResultsD++
				if err := addProgramCV2(&measurement.SeedGroupResultsG, seedGroups); err != nil {
					return programCV2Measurement{}, err
				}
				groupCandidates, err := multiplyProgramCV2(seedGroups, candidates)
				if err != nil {
					return programCV2Measurement{}, err
				}
				if err = addProgramCV2(&measurement.TotalCommunityCandidatesAcrossResults, groupCandidates); err != nil {
					return programCV2Measurement{}, err
				}
				outerWork, err := sumProgramCV2(occurrences, uint64(len(outcome.Projection.NodeIdentities)), seedGroups, sourceMeasurement.MembershipCount, sourceMeasurement.SumGroupOccurrences, groupCandidates)
				if err != nil {
					return programCV2Measurement{}, err
				}
				measurement.MaxWorkV2PerOuterRunCommunityResult = maximumObserved(measurement.MaxWorkV2PerOuterRunCommunityResult, outerWork)
				if err = addProgramCV2(&measurement.TotalWorkV2AcrossOuterResults, outerWork); err != nil {
					return programCV2Measurement{}, err
				}
			}
			if partitionMembers != uint64(len(outcome.Projection.NodeIdentities)) {
				return programCV2Measurement{}, fmt.Errorf("%w: non-bijective community partition", ErrInvalidProgramCTransition)
			}
			measurement.MaxTotalPartitionMembersPerRun = maximumObserved(measurement.MaxTotalPartitionMembersPerRun, partitionMembers)
			if err := addProgramCV2(&measurement.TotalCompositionOccurrencesAcrossRuns, occurrences); err != nil {
				return programCV2Measurement{}, err
			}
			if err := addProgramCV2(&measurement.TotalPartitionMembersAcrossRuns, partitionMembers); err != nil {
				return programCV2Measurement{}, err
			}
			runCandidates, err := multiplyProgramCV2(seedGroups, partitionMembers)
			if err != nil {
				return programCV2Measurement{}, err
			}
			runWork, err := sumProgramCV2(occurrences, partitionMembers, seedGroups, sourceMeasurement.MembershipCount, sourceMeasurement.SumGroupOccurrences, runCandidates)
			if err != nil {
				return programCV2Measurement{}, err
			}
			measurement.MaxWorkV2PerRunSource = maximumObserved(measurement.MaxWorkV2PerRunSource, runWork)
			if err = addProgramCV2(&measurement.TotalWorkV2AcrossRunSources, runWork); err != nil {
				return programCV2Measurement{}, err
			}
		}
	}
	if measurement.CompleteConstituents+measurement.EmptyConstituents+measurement.ResourceIncompleteConstituents != measurement.Constituents {
		return programCV2Measurement{}, ErrInvalidProgramCTransition
	}
	return measurement, nil
}

func classifyProgramCV2MeasurementEntry(entry programCTransitionReceiptInventoryEntry) (programCStage6Classification, error) {
	outcomes := cloneProgramCOutcomes(entry.outcomes)
	if entry.ordinal < 0 || entry.value.Ordinal != entry.ordinal || entry.value.ReceiptOrdinal <= 0 || entry.selector == "" || transitionSHA(entry.canonicalReceipt) != entry.selector || entry.value.Outcome != "SUCCEEDED" || len(entry.nativeV5) == 0 || entry.value.ResultSHA256 != transitionSHA(entry.value.ResultBytes) || entry.value.ResultByteLength != len(entry.value.ResultBytes) || len(outcomes) != 6 {
		return "", ErrInvalidProgramCTransition
	}
	canonicalReceipt, err := json.Marshal(entry.value)
	if err != nil || !bytes.Equal(canonicalReceipt, entry.canonicalReceipt) || !validProgramCStage6RunSequence(outcomes, entry.value.Bindings.AlgorithmVersion) {
		return "", ErrInvalidProgramCTransition
	}
	source := outcomes[0].Source
	sourceBytes := source.InputBytes()
	if !bytes.Equal(entry.nativeV5, sourceBytes) || source.InputLength != len(sourceBytes) || source.InputSHA256 != transitionSHA(sourceBytes) {
		return "", ErrInvalidProgramCTransition
	}
	for _, outcome := range outcomes {
		if !reflect.DeepEqual(outcome.Source, source) || !reflect.DeepEqual(outcome.Projection.Source, source) {
			return "", ErrInvalidProgramCTransition
		}
	}
	switch {
	case entry.value.ProgramCOutcome == "EMPTY" && entry.value.Disposition == "":
		for _, outcome := range outcomes {
			if outcome.Outcome != "EMPTY" {
				return "", ErrInvalidProgramCTransition
			}
		}
		return programCStage6Empty, nil
	case entry.value.ProgramCOutcome == "COMPLETE" && entry.value.Disposition == "COMMITTED_INCOMPLETE_MATCHING_RESOURCE":
		for _, outcome := range outcomes {
			if outcome.Outcome != "COMPLETE" {
				return "", ErrInvalidProgramCTransition
			}
		}
		if entry.value.InstabilitySchema != programc.InstabilityVersionV3 {
			return "", ErrInvalidProgramCTransition
		}
		return programCStage6ResourceIncomplete, nil
	case entry.value.ProgramCOutcome != "COMPLETE" || entry.value.Disposition != "" || entry.value.InstabilitySchema != programc.InstabilityVersionV2 || entry.value.InstabilityClaimCeiling != programc.InstabilityClaimCeilingV2:
		return "", ErrInvalidProgramCTransition
	}
	for _, outcome := range outcomes {
		if outcome.Outcome != "COMPLETE" {
			return "", ErrInvalidProgramCTransition
		}
	}
	identity, pairs, err := decodeCompleteProgramCInstability(entry.value.ResultBytes)
	if err != nil || identity.AdmittedGraphSHA256 != source.InputSHA256 || identity.ProjectionSHA256 != programc.ProfileDigest || identity.ProjectionPolicyID != programc.ProfileID || identity.AlgorithmName != "gonum.org/v1/gonum/graph/community.Leiden" || identity.AlgorithmVersion != entry.value.Bindings.AlgorithmVersion || identity.ParametersCanonicalSHA256 != entry.value.Bindings.ParametersSHA256 || identity.ResourcePolicySHA256 != entry.value.Bindings.ResourcePolicySHA256 {
		return "", ErrInvalidProgramCTransition
	}
	if _, err = verifyProgramCStage6CommunityDigests(outcomes, identity, pairs); err != nil {
		return "", ErrInvalidProgramCTransition
	}
	return programCStage6Complete, nil
}

func measureProgramCV2Source(entry programCTransitionReceiptInventoryEntry) (programCV2SourceMeasurement, error) {
	source := entry.outcomes[0].Source
	graphV5Bytes := source.GraphV5Bytes()
	if len(graphV5Bytes) == 0 || source.GraphV5Length != len(graphV5Bytes) || source.GraphV5SHA256 != transitionSHA(graphV5Bytes) {
		return programCV2SourceMeasurement{}, ErrInvalidProgramCTransition
	}
	view, err := graph.NewNativeV5RepresentativeView(graphV5Bytes)
	if err != nil || !view.Valid() || view.ExecutionBundleID() == "" {
		return programCV2SourceMeasurement{}, ErrInvalidProgramCTransition
	}
	base := entry.outcomes[0].Projection
	for _, outcome := range entry.outcomes {
		if !reflect.DeepEqual(outcome.Projection.NodeIdentities, base.NodeIdentities) || !reflect.DeepEqual(outcome.Projection.Occurrences, base.Occurrences) {
			return programCV2SourceMeasurement{}, fmt.Errorf("%w: inconsistent run projection", ErrInvalidProgramCTransition)
		}
	}
	groups, err := partitionProgramCV2SeedGroups(view.ExecutionBundleID(), view.InvocationSeeds(), view.SeedMemberships(), view.NodeIDs(), view.Edges(), base)
	if err != nil {
		return programCV2SourceMeasurement{}, err
	}
	measurement := programCV2SourceMeasurement{Groups: groups, MembershipCount: uint64(len(view.SeedMemberships()))}
	for _, group := range groups {
		measurement.MaxMembershipsPerGroup = maximumObserved(measurement.MaxMembershipsPerGroup, uint64(len(group.Memberships)))
		measurement.MaxGroupOccurrences = maximumObserved(measurement.MaxGroupOccurrences, uint64(len(group.OccurrenceIDs)))
		if err := addProgramCV2(&measurement.SumGroupOccurrences, uint64(len(group.OccurrenceIDs))); err != nil {
			return programCV2SourceMeasurement{}, err
		}
	}
	stringsCarrier := struct {
		Seeds       []graph.InvocationSeed
		Memberships []graph.SeedMembership
		Edges       []graph.Edge
		Groups      []programCV2SeedGroup
		Occurrences []programc.Occurrence
		Communities []programc.Community
	}{view.InvocationSeeds(), view.SeedMemberships(), view.Edges(), groups, base.Occurrences, nil}
	for _, outcome := range entry.outcomes {
		stringsCarrier.Communities = append(stringsCarrier.Communities, outcome.Communities...)
	}
	measurement.MaxRelevantStringBytes = maximumStringBytes(reflect.ValueOf(stringsCarrier))
	return measurement, nil
}

func partitionProgramCV2SeedGroups(bundleID string, seeds []graph.InvocationSeed, memberships []graph.SeedMembership, nodeIDs []string, edges []graph.Edge, projection programc.Projection) ([]programCV2SeedGroup, error) {
	if bundleID == "" || len(seeds) == 0 || len(memberships) == 0 {
		return nil, ErrInvalidProgramCTransition
	}
	nodes := make(map[string]bool, len(nodeIDs))
	for _, id := range nodeIDs {
		if id == "" || nodes[id] {
			return nil, fmt.Errorf("%w: noncanonical nodes", ErrInvalidProgramCTransition)
		}
		nodes[id] = true
	}
	edgeByRelation := make(map[string]graph.Edge, len(edges))
	for _, edge := range edges {
		if edge.RelationID == "" || edge.ExecutionBundleID != bundleID || edgeByRelation[edge.RelationID].RelationID != "" || !nodes[edge.CallerNodeID] || !nodes[edge.CalleeNodeID] || len(edge.CallSites) == 0 {
			return nil, fmt.Errorf("%w: invalid canonical CALLS edge", ErrInvalidProgramCTransition)
		}
		edgeByRelation[edge.RelationID] = edge
	}
	occurrencesByRelation := make(map[string][]programc.Occurrence, len(edges))
	seenOccurrence := map[string]bool{}
	for _, occurrence := range projection.Occurrences {
		if occurrence.Identity == "" || seenOccurrence[occurrence.Identity] || occurrence.From < 0 || occurrence.To < 0 || int(occurrence.From) >= len(projection.NodeIdentities) || int(occurrence.To) >= len(projection.NodeIdentities) || occurrence.Weight != 1 {
			return nil, fmt.Errorf("%w: invalid canonical occurrence", ErrInvalidProgramCTransition)
		}
		edge, ok := edgeByRelation[occurrence.RelationID]
		if !ok || projection.NodeIdentities[occurrence.From] != edge.CallerNodeID || projection.NodeIdentities[occurrence.To] != edge.CalleeNodeID {
			return nil, fmt.Errorf("%w: occurrence edge mismatch", ErrInvalidProgramCTransition)
		}
		seenOccurrence[occurrence.Identity] = true
		occurrencesByRelation[occurrence.RelationID] = append(occurrencesByRelation[occurrence.RelationID], occurrence)
	}
	for relationID, edge := range edgeByRelation {
		occurrences := occurrencesByRelation[relationID]
		if len(occurrences) != len(edge.CallSites) {
			return nil, fmt.Errorf("%w: partial occurrence closure", ErrInvalidProgramCTransition)
		}
		remaining := append([]graph.Range(nil), edge.CallSites...)
		for _, occurrence := range occurrences {
			matched := -1
			for i, site := range remaining {
				if site == occurrence.CallSite {
					matched = i
					break
				}
			}
			if matched < 0 {
				return nil, fmt.Errorf("%w: occurrence call-site mismatch", ErrInvalidProgramCTransition)
			}
			remaining = append(remaining[:matched], remaining[matched+1:]...)
		}
		if len(remaining) != 0 {
			return nil, fmt.Errorf("%w: incomplete occurrence call-site closure", ErrInvalidProgramCTransition)
		}
	}

	seedOrder := make([]programCV2SeedKey, len(seeds))
	seedIndexes := make(map[programCV2SeedKey]int, len(seeds))
	for i, seed := range seeds {
		key := programCV2SeedKey{bundleID, seed.Label, seed.At}
		if seed.Label == "" || seed.At == "" || seed.ResolvedURI == "" || seed.LanguageID == "" {
			return nil, fmt.Errorf("%w: incomplete invocation seed", ErrInvalidProgramCTransition)
		}
		if _, duplicate := seedIndexes[key]; duplicate {
			return nil, fmt.Errorf("%w: duplicate invocation seed", ErrInvalidProgramCTransition)
		}
		seedIndexes[key] = i
		seedOrder[i] = key
	}
	groups := make([]programCV2SeedGroup, len(seeds))
	for i, key := range seedOrder {
		groups[i].Key = key
	}
	seenMembership := map[string]bool{}
	for i, membership := range memberships {
		key := programCV2SeedKey{membership.ExecutionBundleID, membership.SeedLabel, membership.SeedAt}
		groupIndex, ok := seedIndexes[key]
		if !ok || membership.MembershipID == "" || membership.EndpointID == "" || seenMembership[membership.MembershipID] || (i > 0 && memberships[i-1].MembershipID >= membership.MembershipID) {
			return nil, fmt.Errorf("%w: non-bijective seed membership", ErrInvalidProgramCTransition)
		}
		seenMembership[membership.MembershipID] = true
		group := &groups[groupIndex]
		group.Memberships = append(group.Memberships, membership)
		switch membership.EvidenceKind {
		case "PREPARED_TARGET":
			if !nodes[membership.EndpointID] {
				return nil, fmt.Errorf("%w: unknown prepared target", ErrInvalidProgramCTransition)
			}
			group.PreparedTargets = append(group.PreparedTargets, membership.EndpointID)
		case "REACHED_NODE":
			if !nodes[membership.EndpointID] {
				return nil, fmt.Errorf("%w: unknown reached node", ErrInvalidProgramCTransition)
			}
			group.ReachedNodes = append(group.ReachedNodes, membership.EndpointID)
		case "CALL_RELATION":
			if _, ok := edgeByRelation[membership.EndpointID]; !ok {
				return nil, fmt.Errorf("%w: unknown CALLS relation", ErrInvalidProgramCTransition)
			}
			group.RelationIDs = append(group.RelationIDs, membership.EndpointID)
		case "SIBLING_CANDIDATE", "DISPATCH_ASSOCIATION":
		default:
			return nil, fmt.Errorf("%w: unknown membership kind", ErrInvalidProgramCTransition)
		}
	}
	if len(seenMembership) != len(memberships) {
		return nil, fmt.Errorf("%w: membership accounting mismatch", ErrInvalidProgramCTransition)
	}
	for i := range groups {
		group := &groups[i]
		if len(group.Memberships) == 0 {
			return nil, fmt.Errorf("%w: empty invocation seed group", ErrInvalidProgramCTransition)
		}
		group.PreparedTargets = programCV2SortedUnique(group.PreparedTargets)
		group.ReachedNodes = programCV2SortedUnique(group.ReachedNodes)
		group.RelationIDs = programCV2SortedUnique(group.RelationIDs)
		for _, relationID := range group.RelationIDs {
			for _, occurrence := range occurrencesByRelation[relationID] {
				group.OccurrenceIDs = append(group.OccurrenceIDs, occurrence.Identity)
			}
		}
		group.OccurrenceIDs = programCV2SortedUnique(group.OccurrenceIDs)
	}
	return groups, nil
}

func programCV2SortedUnique(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	for i := 1; i < len(out); i++ {
		if out[i-1] == out[i] {
			return nil
		}
	}
	return out
}

func addProgramCV2(total *uint64, value uint64) error {
	if math.MaxUint64-*total < value {
		return ErrInvalidProgramCTransition
	}
	*total += value
	return nil
}

func multiplyProgramCV2(left, right uint64) (uint64, error) {
	if left != 0 && right > math.MaxUint64/left {
		return 0, ErrInvalidProgramCTransition
	}
	return left * right, nil
}

func sumProgramCV2(values ...uint64) (uint64, error) {
	var total uint64
	for _, value := range values {
		if err := addProgramCV2(&total, value); err != nil {
			return 0, err
		}
	}
	return total, nil
}

func TestADR0014MultiSeedMeasurementBijectionGuard(t *testing.T) {
	entry := programCQualificationMultiSeedEntryFixture(t, 5)
	if classification, err := classifyProgramCV2MeasurementEntry(entry); err != nil || classification != programCStage6Complete {
		t.Fatalf("ASSERT_ADR0014_CANONICAL_CLASSIFICATION classification=%q err=%v", classification, err)
	}
	view, err := graph.NewNativeV5RepresentativeView(entry.outcomes[0].Source.GraphV5Bytes())
	if err != nil {
		t.Fatal(err)
	}
	groups, err := partitionProgramCV2SeedGroups(view.ExecutionBundleID(), view.InvocationSeeds(), view.SeedMemberships(), view.NodeIDs(), view.Edges(), entry.outcomes[0].Projection)
	if err != nil || len(groups) != 5 {
		t.Fatalf("ASSERT_ADR0014_MULTI_SEED_BIJECTION groups=%d err=%v", len(groups), err)
	}
	for i, group := range groups {
		if len(group.Memberships) == 0 || len(group.PreparedTargets) != 1 || len(group.ReachedNodes) != 3 || len(group.RelationIDs) != 2 || len(group.OccurrenceIDs) != 2 {
			t.Fatalf("ASSERT_ADR0014_COMPLETE_ACCOUNTING group=%d value=%+v", i, group)
		}
	}
	memberships := view.SeedMemberships()
	memberships = append(memberships, memberships[len(memberships)-1])
	if _, err = partitionProgramCV2SeedGroups(view.ExecutionBundleID(), view.InvocationSeeds(), memberships, view.NodeIDs(), view.Edges(), entry.outcomes[0].Projection); err == nil {
		t.Fatal("ASSERT_ADR0014_MULTI_SEED_BIJECTION accepted duplicate membership")
	}
	t.Log("ASSERT_ADR0014_CANONICAL_CLASSIFICATION PASS")
	t.Log("ASSERT_ADR0014_MULTI_SEED_BIJECTION PASS")
	t.Log("ASSERT_ADR0014_COMPLETE_ACCOUNTING PASS")
}

func TestADR0014FrozenCorpusMultiSeedV2Measurement(t *testing.T) {
	if os.Getenv(programCV2MeasurementOptInEnv) != "1" {
		t.Skip("set LSP_TRACE_ADR0014_V2_MEASURE=1 with the exact receipt, ledger, and retained state")
	}
	receiptPath := os.Getenv("LSP_TRACE_PROGRAM_C_RECEIPT")
	ledgerPath := os.Getenv("LSP_TRACE_PROGRAM_C_LEDGER")
	stateDir := os.Getenv("LSP_TRACE_PROGRAM_C_RETAINED_STATE")
	if receiptPath == "" || ledgerPath == "" || stateDir == "" {
		t.Fatal("ASSERT_ADR0014_OPT_IN_FROZEN_RUN missing exact frozen input")
	}
	started := time.Now()
	receipt, err := programCV2ExactInput(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := programCV2ExactInput(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	source := &AuthoritativeProgramCTransitionPlanSource{Receipt: receipt, Ledger: ledger}
	planRaw, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bindings := ProgramCTransitionBindings{
		RepositoryCommit: strings.Repeat("a", 40), RepositoryTree: strings.Repeat("b", 40),
		FrozenCorpusSHA256: "sha256:" + strings.Repeat("c", 64), CensusReceiptSHA256: receipt.SHA256, LedgerSHA256: ledger.SHA256,
		AlgorithmVersion: "leiden/v1", Seeds: [2]uint64{1, 2}, ReplaysPerSeed: 3,
		ParametersSHA256: "sha256:" + strings.Repeat("d", 64), ResourcePolicySHA256: "sha256:" + strings.Repeat("e", 64),
		PlanSHA256: transitionSHA(planRaw),
	}
	root, err := publication.OpenRoot(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("close retained state: %v", closeErr)
		}
	}()
	objects, err := censussourceassembly.NewObjects(root)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := censussourceassembly.NewHeadsWithParser(root, programCV2MeasurementHeadNS, ParseProgramCTransitionHeadRevision)
	if err != nil {
		t.Fatal(err)
	}
	readOnlyObjects := &programCV2ReadOnlyObjects{inner: objects}
	readOnlyHeads := &programCV2ReadOnlyHeads{inner: heads}
	measurement, err := observeProgramCV2FrozenCorpus(context.Background(), ProgramCTransitionVerificationRequest{
		WorkflowKey: "production-workflow", HeadKey: "production-workflow", Bindings: bindings,
		PlanSource: source, Objects: readOnlyObjects, Heads: readOnlyHeads,
	})
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Constituents != 395 || readOnlyHeads.loads != 1 || readOnlyObjects.puts != 0 || readOnlyHeads.cas != 0 {
		t.Fatalf("ASSERT_ADR0014_MEASUREMENT_READ_ONLY constituents=%d loads=%d puts=%d cas=%d", measurement.Constituents, readOnlyHeads.loads, readOnlyObjects.puts, readOnlyHeads.cas)
	}
	if measurement.CompleteConstituents+measurement.EmptyConstituents+measurement.ResourceIncompleteConstituents != 395 || measurement.EligibleRuns != measurement.CompleteConstituents*6 || measurement.SeedGroupResultsG < measurement.EligibleRunCommunityResultsD {
		t.Fatalf("ASSERT_ADR0014_TERMINAL_BARRIER measurement=%+v", measurement)
	}
	raw, err := json.Marshal(measurement)
	if err != nil {
		t.Fatal(err)
	}
	if outputPath := os.Getenv("LSP_TRACE_ADR0014_V2_MEASUREMENT_OUTPUT"); outputPath != "" {
		if err := os.WriteFile(outputPath, append(append([]byte(nil), raw...), '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest := sha256.Sum256(raw)
	t.Log("ASSERT_ADR0014_MEASUREMENT_READ_ONLY PASS")
	t.Log("ASSERT_ADR0014_TERMINAL_BARRIER PASS")
	t.Log("ASSERT_ADR0014_OPT_IN_FROZEN_RUN PASS")
	t.Logf("ADR0014_V2_MEASUREMENT runtime=%s sha256=%s json=%s", time.Since(started).Round(time.Millisecond), hex.EncodeToString(digest[:]), raw)
}
