package censuscontinuation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcrepresentative"
)

func TestProgramCStage6AdapterCompleteEntryProducesCanonicalSingletons(t *testing.T) {
	entry := programCStage6CompleteEntryFixture(t)
	got, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if err != nil {
		t.Fatalf("ASSERT_PROGRAM_C_STAGE6_COMPLETE_ADAPTS: %v", err)
	}
	wantEntries := 0
	wantRuns := make([]string, 0)
	wantMembers := make([][]string, 0)
	for i, outcome := range entry.outcomes {
		wantEntries += len(outcome.Communities)
		runID := fmt.Sprintf("seed-%020d-replay-%d", outcome.Seed, i%3+1)
		for _, community := range outcome.Communities {
			wantRuns = append(wantRuns, runID)
			wantMembers = append(wantMembers, community.Members)
		}
	}
	if got.classification != programCStage6Complete || len(got.mechanics) != wantEntries {
		t.Fatalf("ASSERT_PROGRAM_C_STAGE6_COMPLETE_SINGLETON_COUNT classification=%q entries=%d want=%d", got.classification, len(got.mechanics), wantEntries)
	}
	seenRuns := make([]string, 0, len(got.mechanics))
	for i, mechanics := range got.mechanics {
		dto := mechanics.DTO()
		seenRuns = append(seenRuns, dto.RunID)
		if dto.SchemaVersion != programcrepresentative.MechanicsSchemaVersion || dto.Profile != programcrepresentative.ProfileCallsOnlyFrozenCensusV1 || dto.Authority != 0 || dto.Accepted || len(dto.Evidence) != 2 || fmt.Sprint(dto.CommunityMembers) != fmt.Sprint(wantMembers[i]) || len(dto.SelectionContext.Memberships) != 6 || len(dto.SelectionContext.PreparedTargets) != 1 || len(dto.SelectionContext.ReachedNodes) != 3 || len(dto.SelectionContext.RelationIDs) != 2 || len(dto.SelectionContext.OccurrenceIDs) != 2 {
			t.Fatalf("ASSERT_PROGRAM_C_STAGE6_CANONICAL_SINGLETON dto=%+v", dto)
		}
		canonical, err := json.Marshal(dto)
		if err != nil || !bytes.Equal(canonical, mechanics.CanonicalBytes()) {
			t.Fatalf("ASSERT_PROGRAM_C_STAGE6_CANONICAL_BYTES equal=%t err=%v", bytes.Equal(canonical, mechanics.CanonicalBytes()), err)
		}
	}
	if fmt.Sprint(seenRuns) != fmt.Sprint(wantRuns) {
		t.Fatalf("ASSERT_PROGRAM_C_STAGE6_RUN_ORDER got=%v want=%v", seenRuns, wantRuns)
	}
}

func TestProgramCStage6AdapterEmptyAndResourceEntriesProduceNoMechanics(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		entry := programCStage6CompleteEntryFixture(t)
		entry.value.ProgramCOutcome = "EMPTY"
		for i := range entry.outcomes {
			entry.outcomes[i].Outcome = "EMPTY"
			entry.outcomes[i].Communities = nil
		}
		programCStage6RecanonicalizeEntry(t, &entry)
		got, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
		if err != nil || got.classification != programCStage6Empty || len(got.mechanics) != 0 {
			t.Fatalf("ASSERT_PROGRAM_C_STAGE6_EMPTY_ZERO_ENTRIES got=%+v err=%v", got, err)
		}
	})
	t.Run("resource", func(t *testing.T) {
		entry := programCStage6CompleteEntryFixture(t)
		entry.value.Disposition = "COMMITTED_INCOMPLETE_MATCHING_RESOURCE"
		entry.value.InstabilitySchema = programc.InstabilityVersionV3
		entry.value.InstabilityClaimCeiling = programc.InstabilityClaimCeilingV3
		entry.value.ResultBytes = transitionV3IncompleteResultBytes()
		programCStage6RecanonicalizeEntry(t, &entry)
		got, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
		if err != nil || got.classification != programCStage6ResourceIncomplete || len(got.mechanics) != 0 {
			t.Fatalf("ASSERT_PROGRAM_C_STAGE6_RESOURCE_ZERO_ENTRIES got=%+v err=%v", got, err)
		}
	})
}

func TestProgramCStage6AdapterRejectsInvalidRunOrderAndPairDigest(t *testing.T) {
	t.Run("run order", func(t *testing.T) {
		entry := programCStage6CompleteEntryFixture(t)
		entry.outcomes[0], entry.outcomes[3] = entry.outcomes[3], entry.outcomes[0]
		if _, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry); err == nil {
			t.Fatal("ASSERT_PROGRAM_C_STAGE6_INVALID_RUN_ORDER_REJECTED")
		}
	})
	t.Run("pair digest", func(t *testing.T) {
		entry := programCStage6CompleteEntryFixture(t)
		var artifact programc.InstabilityArtifact
		if err := json.Unmarshal(entry.value.ResultBytes, &artifact); err != nil {
			t.Fatal(err)
		}
		artifact.PairwiseComparisons[14].RightCommunityArtifactSHA256 = "sha256:" + strings.Repeat("0", 64)
		entry.value.ResultBytes, _ = json.Marshal(artifact)
		programCStage6RecanonicalizeEntry(t, &entry)
		if _, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry); err == nil {
			t.Fatal("ASSERT_PROGRAM_C_STAGE6_ALL_PAIR_DIGESTS_RECHECKED")
		}
	})
}

func TestProgramCStage6AdapterDeterministicBytesAndMutationIsolation(t *testing.T) {
	entry := programCStage6CompleteEntryFixture(t)
	first, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if err != nil || len(first.mechanics) != len(second.mechanics) {
		t.Fatalf("ASSERT_PROGRAM_C_STAGE6_REPEATABLE_COUNT err=%v", err)
	}
	for i := range first.mechanics {
		before := first.mechanics[i].CanonicalBytes()
		if !bytes.Equal(before, second.mechanics[i].CanonicalBytes()) {
			t.Fatalf("ASSERT_PROGRAM_C_STAGE6_DETERMINISTIC_BYTES ordinal=%d", i)
		}
		dto := first.mechanics[i].DTO()
		dto.CommunityMembers[0] = "mutated-dto"
		dto.Evidence[0].RelationID = "mutated-evidence"
		context := first.mechanics[i].SelectionContext()
		context.Memberships[0].EndpointID = "mutated-membership"
		members := first.mechanics[i].CommunityMembers()
		members[0] = "mutated-members"
		evidence := first.mechanics[i].Evidence()
		evidence[0].RelationID = "mutated-accessor"
		if !bytes.Equal(before, first.mechanics[i].CanonicalBytes()) {
			t.Fatalf("ASSERT_PROGRAM_C_STAGE6_ACCESSOR_MUTATION_ISOLATED ordinal=%d", i)
		}
		if i == 0 {
			parsed, parseErr := programcrepresentative.ParseMechanics(before)
			if parseErr != nil || !bytes.Equal(parsed.CanonicalBytes(), before) {
				t.Fatalf("ASSERT_PROGRAM_C_STAGE6_STRICT_CANONICAL_PARSE err=%v", parseErr)
			}
			unknown := append(bytes.TrimSuffix(bytes.Clone(before), []byte("}")), []byte(`,"unknown":true}`)...)
			if _, parseErr := programcrepresentative.ParseMechanics(unknown); parseErr == nil {
				t.Fatal("ASSERT_PROGRAM_C_STAGE6_UNKNOWN_FIELD_REJECTED")
			}
			hashDTO := first.mechanics[i].DTO()
			identity := hashDTO.Identity
			hashDTO.Identity = ""
			payload, marshalErr := json.Marshal(hashDTO)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			preimage := append([]byte("lsp-trace:program-c-stage6-mechanics:v1\x00"), payload...)
			sum := sha256.Sum256(preimage)
			if want := "sha256:" + hex.EncodeToString(sum[:]); identity != want {
				t.Fatalf("ASSERT_PROGRAM_C_STAGE6_DOMAIN_SEPARATED_HASH got=%q want=%q", identity, want)
			}
		}
	}
	entry.outcomes[0].Communities[0].Members[0] = "mutated-source"
	if !bytes.Equal(first.mechanics[0].CanonicalBytes(), second.mechanics[0].CanonicalBytes()) {
		t.Fatal("ASSERT_PROGRAM_C_STAGE6_OUTCOME_MUTATION_ISOLATED")
	}
}

func TestProgramCStage6MechanicsStandaloneParseIsUntrusted(t *testing.T) {
	entry := programCStage6CompleteEntryFixture(t)
	got, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if err != nil || len(got.mechanics) == 0 {
		t.Fatalf("build mechanics: %v", err)
	}
	parsed, err := programcrepresentative.ParseMechanics(got.mechanics[0].CanonicalBytes())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Valid() {
		t.Fatal("ASSERT_PROGRAM_C_STAGE6_STANDALONE_PARSE_UNTRUSTED")
	}
}

func TestProgramCStage6MechanicsRejectsSelfConsistentForgeries(t *testing.T) {
	entry := programCStage6CompleteEntryFixture(t)
	got, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry)
	if err != nil || len(got.mechanics) == 0 {
		t.Fatalf("build mechanics: %v", err)
	}
	base := got.mechanics[0].DTO()
	expected := programcrepresentative.SingletonMechanicsInput{
		WorkflowKey: "workflow", TerminalProgramCHeadSelector: "sha256:" + strings.Repeat("f", 64),
		ReceiptSelector: entry.selector, ReceiptOrdinal: entry.value.ReceiptOrdinal,
		ConstituentOrdinal: entry.ordinal, PublicationSelector: entry.value.PublicationSelector,
		ConstituentSelector: entry.value.ConstituentSelector, Source: entry.outcomes[0].Source,
		ProgramCAlgorithmVersion: entry.value.Bindings.AlgorithmVersion,
		RunID:                    base.RunID, Seed: base.Seed, ReplayOrdinal: base.ReplayOrdinal,
		CommunityOrdinal: base.CommunityOrdinal, CommunityArtifactSHA256: base.CommunityArtifactSHA256,
		CommunityMembers: append([]string(nil), base.CommunityMembers...),
	}
	if err := programcrepresentative.ValidateMechanicsDTO(base, expected); err != nil {
		t.Fatalf("validate canonical mechanics context: %v", err)
	}
	_, alternateSource := programCStage6AlternateSourceFixture(t)
	t.Run("cross constituent source substitution", func(t *testing.T) {
		substituted := expected
		substituted.Source = alternateSource
		if err := programcrepresentative.ValidateMechanicsDTO(base, substituted); err == nil {
			t.Fatal("ASSERT_PROGRAM_C_STAGE6_CONTEXTUAL_RECONSTRUCTION_REJECTS_FORGERY")
		}
	})
	tests := []struct {
		name   string
		mutate func(*programcrepresentative.MechanicsIdentityDTO)
	}{
		{name: "contract", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.AdapterContractSHA256 = "sha256:" + strings.Repeat("9", 64)
			dto.SelectionContext.AdapterContractSHA256 = dto.AdapterContractSHA256
		}},
		{name: "custody", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.CustodyIdentity = "sha256:" + strings.Repeat("8", 64)
			dto.SelectionContext.CustodyIdentity = dto.CustodyIdentity
		}},
		{name: "seed and run", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.Seed, dto.SelectionContext.Seed = 99, 99
			dto.RunID, dto.SelectionContext.RunID = "forged-run", "forged-run"
		}},
		{name: "constituent bundle and receipt", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.ReceiptSelector = "sha256:" + strings.Repeat("7", 64)
			dto.SelectionContext.ConstituentSelector = "forged-constituent"
			dto.SelectionContext.ExecutionBundleID = "forged-bundle"
			for i := range dto.Evidence {
				dto.Evidence[i].ConstituentSelector = "forged-constituent"
				dto.Evidence[i].ExecutionBundleID = "forged-bundle"
				dto.Evidence[i].ReceiptSelector = dto.ReceiptSelector
			}
		}},
		{name: "membership arrays", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.SelectionContext.Memberships[0].EndpointID = "forged-membership-endpoint"
		}},
		{name: "community membership projection", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.CommunityMembers = []string{"forged-community-member"}
			dto.SelectionContext.CommunityMembers = append([]string(nil), dto.CommunityMembers...)
		}},
		{name: "relation occurrence closure", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.SelectionContext.RelationIDs = []string{"forged-relation"}
			dto.SelectionContext.OccurrenceIDs = []string{"forged-occurrence"}
		}},
		{name: "endpoint validity", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.Evidence[0].FromNodeID = "forged-from-node"
			dto.Evidence[0].ToNodeID = "forged-to-node"
		}},
		{name: "call site ordinal", mutate: func(dto *programcrepresentative.MechanicsIdentityDTO) {
			dto.Evidence[0].CallSiteOrdinal = 99
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := cloneProgramCStage6MechanicsDTO(t, base)
			tc.mutate(&dto)
			programCStage6RehashMechanicsForgery(t, &dto)
			if err := programcrepresentative.ValidateMechanicsDTO(dto, expected); err == nil {
				t.Fatal("ASSERT_PROGRAM_C_STAGE6_CONTEXTUAL_RECONSTRUCTION_REJECTS_FORGERY")
			}
		})
	}
}

func TestProgramCStage6SingletonInputUsesOnlyVerifiedSourceBinding(t *testing.T) {
	typeOf := reflect.TypeOf(programcrepresentative.SingletonMechanicsInput{})
	_, hasSource := typeOf.FieldByName("Source")
	_, hasBytes := typeOf.FieldByName("NativeV5Bytes")
	_, hasView := typeOf.FieldByName("NativeV5View")
	if !hasSource || hasBytes || hasView {
		t.Fatalf("ASSERT_PROGRAM_C_STAGE6_SINGLE_OPAQUE_SOURCE_INPUT source=%t bytes=%t view=%t", hasSource, hasBytes, hasView)
	}
}

func TestProgramCStage6AdapterRejectsInvalidOrSubstitutedSourceBindings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*programCTransitionReceiptInventoryEntry)
	}{
		{name: "invalid retained bytes", mutate: func(entry *programCTransitionReceiptInventoryEntry) {
			entry.nativeV5 = []byte("invalid retained bytes")
		}},
		{name: "source input length", mutate: func(entry *programCTransitionReceiptInventoryEntry) {
			for i := range entry.outcomes {
				entry.outcomes[i].Source.InputLength++
			}
		}},
		{name: "cross outcome source substitution", mutate: func(entry *programCTransitionReceiptInventoryEntry) {
			entry.outcomes[5].Source.SessionID = "substituted-source-session"
		}},
		{name: "empty outcome source substitution", mutate: func(entry *programCTransitionReceiptInventoryEntry) {
			entry.value.ProgramCOutcome = "EMPTY"
			for i := range entry.outcomes {
				entry.outcomes[i].Outcome = "EMPTY"
				entry.outcomes[i].Communities = nil
			}
			entry.outcomes[5].Source.SessionID = "substituted-empty-source-session"
			programCStage6RecanonicalizeEntry(t, entry)
		}},
		{name: "resource outcome source substitution", mutate: func(entry *programCTransitionReceiptInventoryEntry) {
			entry.value.Disposition = "COMMITTED_INCOMPLETE_MATCHING_RESOURCE"
			entry.value.InstabilitySchema = programc.InstabilityVersionV3
			entry.value.InstabilityClaimCeiling = programc.InstabilityClaimCeilingV3
			entry.value.ResultBytes = transitionV3IncompleteResultBytes()
			entry.outcomes[5].Source.SessionID = "substituted-resource-source-session"
			programCStage6RecanonicalizeEntry(t, entry)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := programCStage6CompleteEntryFixture(t)
			tc.mutate(&entry)
			if _, err := adaptProgramCStage6InventoryEntry("workflow", "sha256:"+strings.Repeat("f", 64), entry); err == nil {
				t.Fatal("ASSERT_PROGRAM_C_STAGE6_INVALID_OR_SUBSTITUTED_SOURCE_REJECTED")
			}
		})
	}
}

func cloneProgramCStage6MechanicsDTO(t *testing.T, in programcrepresentative.MechanicsIdentityDTO) programcrepresentative.MechanicsIdentityDTO {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out programcrepresentative.MechanicsIdentityDTO
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func programCStage6RehashMechanicsForgery(t *testing.T, dto *programcrepresentative.MechanicsIdentityDTO) {
	t.Helper()
	for i := range dto.Evidence {
		dto.Evidence[i].Identity = ""
		dto.Evidence[i].Identity = programCStage6TestIdentity(t, "lsp-trace:program-c-stage6-evidence:v1", dto.Evidence[i])
	}
	if len(dto.SelectionContext.OccurrenceIDs) == len(dto.Evidence) {
		dto.SelectionContext.OccurrenceIDs = make([]string, len(dto.Evidence))
		for i := range dto.Evidence {
			dto.SelectionContext.OccurrenceIDs[i] = dto.Evidence[i].Identity
		}
		sort.Strings(dto.SelectionContext.OccurrenceIDs)
	}
	dto.SelectionContext.Identity = ""
	dto.SelectionContext.Identity = programCStage6TestIdentity(t, "lsp-trace:program-c-stage6-context:v1", dto.SelectionContext)
	dto.Identity = ""
	dto.Identity = programCStage6TestIdentity(t, "lsp-trace:program-c-stage6-mechanics:v1", *dto)
}

func programCStage6TestIdentity(t *testing.T, domain string, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(append(append([]byte(domain), 0), payload...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func programCStage6AlternateSourceFixture(t *testing.T) ([]byte, programc.SourceBinding) {
	t.Helper()
	node := func(name string, line uint32) graph.Node {
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 2}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///fixture/alternate.go", Range: r, SelectionRange: r})
	}
	caller, callee := node("alternate-caller", 10), node("alternate-callee", 12)
	canonical := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: []graph.Node{caller, callee}, Edges: []graph.Edge{{CallerNodeID: caller.ID, CalleeNodeID: callee.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 10, Character: 1}, End: graph.Position{Line: 10, Character: 2}}}}}}
	canonical.Canonicalize()
	seed := graph.InvocationSeed{Label: "alternate-seed", At: "alternate.go:11:1", ResolvedURI: "file:///fixture/alternate.go", ContentSHA256: "sha256:" + strings.Repeat("d", 64), LanguageID: "go"}
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "fixture-ls"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "alternate-session", SourceRevision: "alternate-revision", ServerVersion: "fixture-ls@1"}},
		Nodes:         canonical.Nodes, Edges: canonical.Edges,
		Seeds:   []graph.SeedResult{{Label: seed.Label, PreparedTargetIDs: []string{caller.ID}, ReachedNodeIDs: []string{caller.ID, callee.ID}, ReachedRelationIDs: []string{canonical.Edges[0].RelationID}}},
		Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	native, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := graphprovenance.CaptureV5(native, "alternate-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatal(err)
	}
	outcome, failure := programc.Compute(retained, 1)
	if failure != nil {
		t.Fatal(failure)
	}
	return retained, outcome.Source
}

func programCStage6RecanonicalizeEntry(t *testing.T, entry *programCTransitionReceiptInventoryEntry) {
	t.Helper()
	entry.value.ResultSHA256 = transitionSHA(entry.value.ResultBytes)
	entry.value.ResultByteLength = len(entry.value.ResultBytes)
	raw, err := json.Marshal(entry.value)
	if err != nil {
		t.Fatal(err)
	}
	entry.canonicalReceipt = raw
	entry.selector = transitionSHA(raw)
}

func programCStage6CompleteEntryFixture(t *testing.T) programCTransitionReceiptInventoryEntry {
	t.Helper()
	node := func(name string, line uint32) graph.Node {
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 2}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///fixture/input.go", Range: r, SelectionRange: r})
	}
	a, b, c := node("alpha", 0), node("beta", 2), node("gamma", 4)
	edges := []graph.Edge{
		{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 0, Character: 1}, End: graph.Position{Line: 0, Character: 2}}}},
		{CallerNodeID: b.ID, CalleeNodeID: c.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 2, Character: 1}, End: graph.Position{Line: 2, Character: 2}}}},
	}
	canonical := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: []graph.Node{c, a, b}, Edges: edges}
	canonical.Canonicalize()
	edges = canonical.Edges
	seed := graph.InvocationSeed{Label: "seed", At: "input.go:1:1", ResolvedURI: "file:///fixture/input.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "fixture-ls"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "fixture-session", SourceRevision: "fixture-revision", ServerVersion: "fixture-ls@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}},
		Nodes:         []graph.Node{c, a, b},
		Edges:         edges,
		Seeds:         []graph.SeedResult{{Label: seed.Label, PreparedTargetIDs: []string{a.ID}, ReachedNodeIDs: []string{a.ID, b.ID, c.ID}, ReachedRelationIDs: []string{edges[0].RelationID, edges[1].RelationID}}},
		Summary:       graph.Summary{Complete: true},
		Capabilities:  graph.Capabilities{CallHierarchyProvider: true},
	}
	native, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := graphprovenance.CaptureV5(native, "fixture-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatalf("build exact V5 fixture: %v", err)
	}
	outcomes := make([]programc.Outcome, 0, 6)
	for _, seedValue := range []uint64{1, 1, 1, 2, 2, 2} {
		outcome, failure := programc.Compute(retained, seedValue)
		if failure != nil || outcome.Outcome != "COMPLETE" {
			t.Fatalf("compute fixture outcome=%+v failure=%v", outcome, failure)
		}
		outcomes = append(outcomes, outcome)
	}
	bindings := transitionBindings(transitionPlan(t, records2()[:1]))
	instability, err := (ProgramCProductionRunner{}).Instability(outcomes, bindings)
	if err != nil {
		t.Fatal(err)
	}
	receipt := programCTransitionReceipt{
		SchemaVersion: programCTransitionReceiptVersion,
		WorkflowKey:   "workflow", HeadKey: "program-c", Authority: 0, Bindings: bindings,
		ConstituentCount: 1, Ordinal: 0, ReceiptOrdinal: 1, PublicationByteLength: 1,
		PublicationSelector: "sha256:" + strings.Repeat("b", 64), ConstituentSelector: "sha256:" + strings.Repeat("c", 64),
		Outcome: "SUCCEEDED", ProgramCOutcome: "COMPLETE", ResultSHA256: transitionSHA(instability), ResultByteLength: len(instability), ResultBytes: instability,
		InstabilitySchema: programc.InstabilityVersionV2, InstabilityClaimCeiling: programc.InstabilityClaimCeilingV2,
	}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return programCTransitionReceiptInventoryEntry{ordinal: 0, selector: transitionSHA(receiptRaw), canonicalReceipt: receiptRaw, value: receipt, nativeV5: retained, outcomes: outcomes}
}
