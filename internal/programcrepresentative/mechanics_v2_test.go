package programcrepresentative_test

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

func TestMechanicsV2CanonicalSeedCounts(t *testing.T) {
	for _, count := range []int{1, 5, 63, 64} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			input := mechanicsV2Input(t, count)
			got, err := programcrepresentative.NewMechanicsV2(input)
			if err != nil || !got.Valid() || !got.Trusted() {
				t.Fatalf("ASSERT_MECHANICS_V2_CANONICAL_%d_SEEDS err=%v", count, err)
			}
			dto := got.DTO()
			if len(dto.SelectionContext.SeedGroups) != count {
				t.Fatalf("ASSERT_MECHANICS_V2_ONE_CONTEXT_ORDERED_GROUPS got=%d want=%d", len(dto.SelectionContext.SeedGroups), count)
			}
			seenMemberships := map[string]bool{}
			for i, group := range dto.SelectionContext.SeedGroups {
				if group.InvocationSeedOrdinal != i || group.SeedLabel != fmt.Sprintf("seed-%03d", i) || len(group.Memberships) == 0 {
					t.Fatalf("ASSERT_MECHANICS_V2_INVOCATION_GROUP_%d dto=%+v", i, group)
				}
				for _, membership := range group.Memberships {
					if seenMemberships[membership.MembershipID] {
						t.Fatalf("ASSERT_MECHANICS_V2_MEMBERSHIP_BIJECTION duplicate=%s", membership.MembershipID)
					}
					seenMemberships[membership.MembershipID] = true
				}
			}
			parsed, err := programcrepresentative.ParseMechanicsV2(got.CanonicalBytes())
			if err != nil || parsed.Trusted() || parsed.Valid() {
				t.Fatalf("ASSERT_MECHANICS_V2_STANDALONE_UNTRUSTED err=%v trusted=%v", err, parsed.Trusted())
			}
			if err := programcrepresentative.ValidateMechanicsDTOv2(parsed.DTO(), input); err != nil {
				t.Fatalf("ASSERT_MECHANICS_V2_CONTEXT_RECONSTRUCTION err=%v", err)
			}
		})
	}
	if _, err := programcrepresentative.NewMechanicsV2(mechanicsV2Input(t, 65)); err == nil {
		t.Fatal("ASSERT_MECHANICS_V2_MAX_SEED_GROUPS_PLUS_ONE_REJECTED")
	}
}

func TestMechanicsV2ConstructorSourceLimitsEndToEnd(t *testing.T) {
	equalMembershipGroup := mechanicsV2SeedShape{prepared: 127, reached: 128, relation: 0}
	tests := []struct {
		name      string
		shapes    []mechanicsV2SeedShape
		callSites []int
		want      mechanicsV2ShapeCounts
		accept    bool
	}{
		{"seed-groups-equality", repeatedMechanicsV2Shape(64, mechanicsV2SeedShape{prepared: 1, reached: 2, relation: 0}), []int{1}, mechanicsV2ShapeCounts{seedGroups: 64}, true},
		{"seed-groups-plus-one", repeatedMechanicsV2Shape(65, mechanicsV2SeedShape{prepared: 1, reached: 2, relation: 0}), []int{1}, mechanicsV2ShapeCounts{}, false},
		{"memberships-per-group-equality", []mechanicsV2SeedShape{equalMembershipGroup}, []int{1}, mechanicsV2ShapeCounts{seedGroups: 1, memberships: 256, maxMemberships: 256}, true},
		{"memberships-per-group-plus-one", []mechanicsV2SeedShape{{prepared: 128, reached: 128, relation: 0}}, []int{1}, mechanicsV2ShapeCounts{}, false},
		{"memberships-total-equality", repeatedMechanicsV2Shape(8, equalMembershipGroup), []int{1}, mechanicsV2ShapeCounts{seedGroups: 8, memberships: 2048, maxMemberships: 256}, true},
		{"memberships-total-plus-one", append(repeatedMechanicsV2Shape(8, equalMembershipGroup), mechanicsV2SeedShape{prepared: 1, relation: -1}), []int{1}, mechanicsV2ShapeCounts{}, false},
		{"occurrences-per-group-equality", []mechanicsV2SeedShape{{prepared: 1, reached: 2, relation: 0}}, []int{256}, mechanicsV2ShapeCounts{seedGroups: 1, maxOccurrences: 256, sumOccurrences: 256}, true},
		{"occurrences-per-group-plus-one", []mechanicsV2SeedShape{{prepared: 1, reached: 2, relation: 0}}, []int{257}, mechanicsV2ShapeCounts{}, false},
		{"summed-occurrences-equality", repeatedMechanicsV2Shape(8, mechanicsV2SeedShape{prepared: 1, reached: 2, relation: 0}), []int{256}, mechanicsV2ShapeCounts{seedGroups: 8, maxOccurrences: 256, sumOccurrences: 2048}, true},
		{"summed-occurrences-plus-one", append(repeatedMechanicsV2Shape(8, mechanicsV2SeedShape{prepared: 1, reached: 2, relation: 0}), mechanicsV2SeedShape{prepared: 1, reached: 2, relation: 1}), []int{1, 256}, mechanicsV2ShapeCounts{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := mechanicsV2ShapedInput(t, tc.shapes, tc.callSites)
			got, err := programcrepresentative.NewMechanicsV2(input)
			if !tc.accept {
				if err == nil || got.Valid() {
					t.Fatalf("ASSERT_MECHANICS_V2_CONSTRUCTOR_LIMIT_PLUS_ONE_ACCEPTED shape=%+v counts=%+v", tc, mechanicsV2Counts(got.SelectionContext().SeedGroups))
				}
				return
			}
			if err != nil || !got.Valid() || !got.Trusted() {
				t.Fatalf("ASSERT_MECHANICS_V2_CONSTRUCTOR_LIMIT_EQUALITY_REJECTED shape=%+v err=%v", tc, err)
			}
			counts := mechanicsV2Counts(got.SelectionContext().SeedGroups)
			if tc.want.seedGroups != 0 && counts.seedGroups != tc.want.seedGroups ||
				tc.want.memberships != 0 && counts.memberships != tc.want.memberships ||
				tc.want.maxMemberships != 0 && counts.maxMemberships != tc.want.maxMemberships ||
				tc.want.maxOccurrences != 0 && counts.maxOccurrences != tc.want.maxOccurrences ||
				tc.want.sumOccurrences != 0 && counts.sumOccurrences != tc.want.sumOccurrences {
				t.Fatalf("ASSERT_MECHANICS_V2_CONSTRUCTOR_LIMIT_COUNTS got=%+v want=%+v", counts, tc.want)
			}
		})
	}
}

func TestMechanicsV2ConstructorJointNearLimitFixture(t *testing.T) {
	shapes := repeatedMechanicsV2Shape(8, mechanicsV2SeedShape{prepared: 127, reached: 128, relation: 0})
	input := mechanicsV2ShapedInput(t, shapes, []int{256})
	got, err := programcrepresentative.NewMechanicsV2(input)
	if err != nil || !got.Valid() || !got.Trusted() {
		t.Fatalf("ASSERT_MECHANICS_V2_JOINT_NEAR_LIMIT_ACCEPTED err=%v", err)
	}
	want := mechanicsV2ShapeCounts{seedGroups: 8, memberships: 2048, maxMemberships: 256, maxOccurrences: 256, sumOccurrences: 2048}
	if counts := mechanicsV2Counts(got.SelectionContext().SeedGroups); counts != want {
		t.Fatalf("ASSERT_MECHANICS_V2_JOINT_NEAR_LIMIT_COUNTS got=%+v want=%+v", counts, want)
	}
}

func TestMechanicsV2StringLimitThroughConstructor(t *testing.T) {
	input := mechanicsV2Input(t, 1)
	input.ProgramCAlgorithmVersion = strings.Repeat("x", programcrepresentative.MaxStringBytesV2)
	if got, err := programcrepresentative.NewMechanicsV2(input); err != nil || !got.Valid() {
		t.Fatalf("ASSERT_MECHANICS_V2_STRING_LIMIT_CONSTRUCTOR_EQUALITY err=%v", err)
	}
	input.ProgramCAlgorithmVersion += "x"
	if got, err := programcrepresentative.NewMechanicsV2(input); err == nil || got.Valid() {
		t.Fatal("ASSERT_MECHANICS_V2_STRING_LIMIT_CONSTRUCTOR_PLUS_ONE")
	}
}

func TestMechanicsV2WireAndDefensiveCopies(t *testing.T) {
	input := mechanicsV2Input(t, 5)
	got, err := programcrepresentative.NewMechanicsV2(input)
	if err != nil {
		t.Fatal(err)
	}
	before := got.CanonicalBytes()
	dto := got.DTO()
	if dto.SchemaVersion != programcrepresentative.MechanicsSchemaVersionV2 || dto.Profile != programcrepresentative.ProfileCallsOnlyFrozenCensusV1 || dto.Authority != 0 || dto.Accepted {
		t.Fatalf("ASSERT_MECHANICS_V2_WIRE_HEADER dto=%+v", dto)
	}
	if bytes.Contains(before, []byte("semantic_authority")) || bytes.Contains(before, []byte("public_authority")) || bytes.Contains(before, []byte(":null")) {
		t.Fatalf("ASSERT_MECHANICS_V2_WIRE_SCOPE_OR_NULL raw=%s", before)
	}
	input.CommunityMembers[0] = "mutated-input"
	dto.Evidence[0].RelationID = "mutated-evidence"
	dto.CommunityMembers[0] = "mutated-community"
	dto.SelectionContext.CommunityMembers[0] = "mutated-context-community"
	dto.SelectionContext.SeedGroups[0].Memberships[0].EndpointID = "mutated-membership"
	dto.SelectionContext.SeedGroups[0].PreparedTargets[0] = "mutated-prepared"
	dto.SelectionContext.SeedGroups[0].ReachedNodes[0] = "mutated-reached"
	dto.SelectionContext.SeedGroups[0].RelationIDs[0] = "mutated-relation"
	dto.SelectionContext.SeedGroups[0].OccurrenceIDs[0] = "mutated-occurrence"
	evidence := got.Evidence()
	evidence[0].RelationID = "mutated-evidence-accessor"
	community := got.CommunityMembers()
	community[0] = "mutated-community-accessor"
	context := got.SelectionContext()
	context.CommunityMembers[0] = "mutated-context-accessor"
	context.SeedGroups[0].Memberships[0].EndpointID = "mutated-membership-accessor"
	context.SeedGroups[0].PreparedTargets[0] = "mutated-prepared-accessor"
	context.SeedGroups[0].ReachedNodes[0] = "mutated-reached-accessor"
	context.SeedGroups[0].RelationIDs[0] = "mutated-relation-accessor"
	context.SeedGroups[0].OccurrenceIDs[0] = "mutated-occurrence-accessor"
	canonical := got.CanonicalBytes()
	canonical[0] ^= 0xff
	if !bytes.Equal(before, got.CanonicalBytes()) {
		t.Fatal("ASSERT_MECHANICS_V2_ALL_NESTED_DEFENSIVE_COPIES")
	}
}

func TestMechanicsV2FiveKindsRemainIdentityBoundWithIndependentSharedClosure(t *testing.T) {
	input := mechanicsV2FiveKindInput(t)
	got, err := programcrepresentative.NewMechanicsV2(input)
	if err != nil {
		t.Fatal(err)
	}
	groups := got.SelectionContext().SeedGroups
	if len(groups) != 2 {
		t.Fatalf("ASSERT_MECHANICS_V2_FIVE_KIND_TWO_GROUP_FIXTURE groups=%d", len(groups))
	}
	wantKinds := map[string]bool{
		"PREPARED_TARGET": true, "REACHED_NODE": true, "CALL_RELATION": true,
		"SIBLING_CANDIDATE": true, "DISPATCH_ASSOCIATION": true,
	}
	for groupIndex, group := range groups {
		gotKinds := map[string]bool{}
		for _, membership := range group.Memberships {
			if !wantKinds[membership.EvidenceKind] {
				t.Fatalf("ASSERT_MECHANICS_V2_ONLY_FIVE_MEMBERSHIP_KINDS group=%d kind=%q", groupIndex, membership.EvidenceKind)
			}
			gotKinds[membership.EvidenceKind] = true
			switch membership.EvidenceKind {
			case "PREPARED_TARGET":
				if !containsMechanicsV2String(group.PreparedTargets, membership.EndpointID) {
					t.Fatalf("ASSERT_MECHANICS_V2_PREPARED_TARGET_PROJECTED group=%d membership=%+v", groupIndex, membership)
				}
			case "REACHED_NODE":
				if !containsMechanicsV2String(group.ReachedNodes, membership.EndpointID) {
					t.Fatalf("ASSERT_MECHANICS_V2_REACHED_NODE_PROJECTED group=%d membership=%+v", groupIndex, membership)
				}
			case "CALL_RELATION":
				if !containsMechanicsV2String(group.RelationIDs, membership.EndpointID) {
					t.Fatalf("ASSERT_MECHANICS_V2_CALL_RELATION_PROJECTED group=%d membership=%+v", groupIndex, membership)
				}
			case "SIBLING_CANDIDATE", "DISPATCH_ASSOCIATION":
				if containsMechanicsV2String(group.PreparedTargets, membership.EndpointID) || containsMechanicsV2String(group.ReachedNodes, membership.EndpointID) || containsMechanicsV2String(group.RelationIDs, membership.EndpointID) || containsMechanicsV2String(group.OccurrenceIDs, membership.EndpointID) {
					t.Fatalf("ASSERT_MECHANICS_V2_DISCOVERY_MEMBERSHIP_IDENTITY_ONLY group=%d membership=%+v", groupIndex, membership)
				}
			}
		}
		if !reflect.DeepEqual(gotKinds, wantKinds) {
			t.Fatalf("ASSERT_MECHANICS_V2_ALL_FIVE_MEMBERSHIP_KINDS group=%d got=%v", groupIndex, gotKinds)
		}
	}
	if len(groups[0].RelationIDs) != 1 || !reflect.DeepEqual(groups[0].RelationIDs, groups[1].RelationIDs) {
		t.Fatalf("ASSERT_MECHANICS_V2_SHARED_CANONICAL_CALL_RELATION groups=%+v", groups)
	}
	if len(groups[0].OccurrenceIDs) != 2 || !reflect.DeepEqual(groups[0].OccurrenceIDs, groups[1].OccurrenceIDs) {
		t.Fatalf("ASSERT_MECHANICS_V2_COMPLETE_OCCURRENCE_CLOSURE_PER_GROUP left=%v right=%v", groups[0].OccurrenceIDs, groups[1].OccurrenceIDs)
	}
	secondOccurrence := groups[1].OccurrenceIDs[0]
	groups[0].OccurrenceIDs[0] = "mutated-first-group-only"
	if groups[1].OccurrenceIDs[0] != secondOccurrence || got.SelectionContext().SeedGroups[0].OccurrenceIDs[0] == "mutated-first-group-only" {
		t.Fatal("ASSERT_MECHANICS_V2_NO_CROSS_GROUP_OCCURRENCE_DEDUP_OR_ALIAS")
	}
}

func TestMechanicsV2ReusesV1EvidenceCustodyAndMembershipIdentitiesExactly(t *testing.T) {
	input := mechanicsV2Input(t, 1)
	v2, err := programcrepresentative.NewMechanicsV2(input)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := programcrepresentative.NewSingletonMechanics(singletonMechanicsInputV2(input))
	if err != nil {
		t.Fatal(err)
	}
	v1DTO, v2DTO := v1.DTO(), v2.DTO()
	assertMechanicsV2ReusedJSON(t, "evidence", v1DTO.Evidence, v2DTO.Evidence)
	assertMechanicsV2ReusedJSON(t, "membership", v1DTO.SelectionContext.Memberships, v2DTO.SelectionContext.SeedGroups[0].Memberships)
	native := input.Source.InputBytes()
	v1Custody := programcrepresentative.CustodyIdentityDTO{
		SchemaVersion: programcrepresentative.CustodySchemaVersion,
		WorkflowKey:   input.WorkflowKey, TerminalProgramCHeadSelector: input.TerminalProgramCHeadSelector,
		ReceiptSelector: input.ReceiptSelector, ReceiptOrdinal: input.ReceiptOrdinal,
		ConstituentOrdinal: input.ConstituentOrdinal, PublicationSelector: input.PublicationSelector,
		ConstituentSelector: input.ConstituentSelector, NativeV5SHA256: digestMechanicsV2(native),
		NativeV5ByteLength: uint64(len(native)), ExecutionBundleID: v1DTO.SelectionContext.ExecutionBundleID,
		Identity: v1DTO.CustodyIdentity,
	}
	v2Custody := v1Custody
	v2Custody.ExecutionBundleID = v2DTO.SelectionContext.ExecutionBundleID
	v2Custody.Identity = v2DTO.CustodyIdentity
	assertMechanicsV2ReusedJSON(t, "custody", v1Custody, v2Custody)
}

func TestMechanicsV2NestedCanonicalFingerprints(t *testing.T) {
	got, err := programcrepresentative.NewMechanicsV2(mechanicsV2Input(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	dto := got.DTO()
	assertMechanicsV2Fingerprint(t, "seed-group", dto.SelectionContext.SeedGroups[0], "75d1db78f05323e1b817848848b07e527c368e20af08aed5ca1c60ee822b6db4")
	assertMechanicsV2Fingerprint(t, "selection-context", dto.SelectionContext, "9e51fb4e1354c178ef55f0fbfba3251508e7a78cc53ed4e5664ec90437979a9b")
}

func TestMechanicsV2RejectsNullForEveryArray(t *testing.T) {
	got, err := programcrepresentative.NewMechanicsV2(mechanicsV2Input(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*programcrepresentative.MechanicsIdentityDTOv2)
	}{
		{"evidence", func(d *programcrepresentative.MechanicsIdentityDTOv2) { d.Evidence = nil }},
		{"community-members", func(d *programcrepresentative.MechanicsIdentityDTOv2) { d.CommunityMembers = nil }},
		{"context-community-members", func(d *programcrepresentative.MechanicsIdentityDTOv2) { d.SelectionContext.CommunityMembers = nil }},
		{"seed-groups", func(d *programcrepresentative.MechanicsIdentityDTOv2) { d.SelectionContext.SeedGroups = nil }},
		{"memberships", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].Memberships = nil
		}},
		{"prepared-targets", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].PreparedTargets = nil
		}},
		{"reached-nodes", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].ReachedNodes = nil
		}},
		{"relation-ids", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].RelationIDs = nil
		}},
		{"occurrence-ids", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].OccurrenceIDs = nil
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := got.DTO()
			tc.mutate(&dto)
			raw, marshalErr := json.Marshal(dto)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if _, parseErr := programcrepresentative.ParseMechanicsV2(raw); parseErr == nil {
				t.Fatal("ASSERT_MECHANICS_V2_NULL_ARRAY_REJECTED")
			}
		})
	}
}

func TestMechanicsV2ContextRejectsGroupAndMembershipMutations(t *testing.T) {
	input := mechanicsV2Input(t, 5)
	got, err := programcrepresentative.NewMechanicsV2(input)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*programcrepresentative.MechanicsIdentityDTOv2)
	}{
		{"missing-group", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups = d.SelectionContext.SeedGroups[1:]
		}},
		{"duplicate-group", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[1] = d.SelectionContext.SeedGroups[0]
		}},
		{"permuted-groups", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0], d.SelectionContext.SeedGroups[1] = d.SelectionContext.SeedGroups[1], d.SelectionContext.SeedGroups[0]
		}},
		{"foreign-group", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].ExecutionBundleID = "foreign"
		}},
		{"empty-group", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].Memberships = []programcrepresentative.MembershipIdentityDTO{}
		}},
		{"missing-membership", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].Memberships = d.SelectionContext.SeedGroups[0].Memberships[1:]
		}},
		{"duplicate-membership", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].Memberships[1] = d.SelectionContext.SeedGroups[0].Memberships[0]
		}},
		{"seed-label", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].SeedLabel = "substituted"
		}},
		{"seed-at", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].SeedAt = "substituted"
		}},
		{"resolved-uri", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].ResolvedURI = "file:///substituted"
		}},
		{"content", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].ContentSHA256 = "sha256:" + strings.Repeat("f", 64)
		}},
		{"language", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].LanguageID = "substituted"
		}},
		{"occurrence-closure", func(d *programcrepresentative.MechanicsIdentityDTOv2) {
			d.SelectionContext.SeedGroups[0].OccurrenceIDs = d.SelectionContext.SeedGroups[0].OccurrenceIDs[1:]
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := cloneMechanicsV2DTO(t, got.DTO())
			tc.mutate(&dto)
			if err := programcrepresentative.ValidateMechanicsDTOv2(dto, input); err == nil {
				t.Fatal("ASSERT_MECHANICS_V2_CONTEXT_REJECTS_MUTATION")
			}
		})
	}
}

func TestMechanicsV2StrictParserAndMixedV1Rejection(t *testing.T) {
	input := mechanicsV2Input(t, 1)
	got, err := programcrepresentative.NewMechanicsV2(input)
	if err != nil {
		t.Fatal(err)
	}
	raw := got.CanonicalBytes()
	dto := got.DTO()
	nestedSchema := cloneMechanicsV2DTO(t, dto)
	nestedSchema.SelectionContext.SeedGroups[0].SchemaVersion = programcrepresentative.SelectionContextSchemaVersionV2
	nestedSchemaRaw, _ := json.Marshal(nestedSchema)
	nestedDomain := cloneMechanicsV2DTO(t, dto)
	nestedDomain.Evidence[0].Identity = nestedDomain.CustodyIdentity
	nestedDomainRaw, _ := json.Marshal(nestedDomain)
	nestedIdentity := cloneMechanicsV2DTO(t, dto)
	nestedIdentity.SelectionContext.SeedGroups[0].Identity = nestedIdentity.SelectionContext.Identity
	nestedIdentityRaw, _ := json.Marshal(nestedIdentity)
	fieldOrder := bytes.Replace(raw,
		[]byte(`{"schema_version":"`+programcrepresentative.MechanicsSchemaVersionV2+`","profile":"`+programcrepresentative.ProfileCallsOnlyFrozenCensusV1+`"`),
		[]byte(`{"profile":"`+programcrepresentative.ProfileCallsOnlyFrozenCensusV1+`","schema_version":"`+programcrepresentative.MechanicsSchemaVersionV2+`"`), 1)
	tests := []struct {
		name string
		raw  []byte
	}{
		{"duplicate-key", append([]byte(`{"schema_version":"lsp-trace.program-c-stage6-mechanics.private.v2",`), raw[1:]...)},
		{"unknown-field", append([]byte(`{"unknown":0,`), raw[1:]...)},
		{"trailing-json", append(append([]byte{}, raw...), []byte(`{}`)...)},
		{"noncanonical-whitespace", append(append([]byte{}, raw...), '\n')},
		{"noncanonical-field-order", fieldOrder},
		{"nested-mixed-schema", nestedSchemaRaw},
		{"nested-mixed-domain", nestedDomainRaw},
		{"nested-mixed-identity", nestedIdentityRaw},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := programcrepresentative.ParseMechanicsV2(tc.raw); err == nil {
				t.Fatal("ASSERT_MECHANICS_V2_STRICT_PARSER_REJECTION")
			}
		})
	}
	v1, err := programcrepresentative.NewSingletonMechanics(programcrepresentative.SingletonMechanicsInput{
		WorkflowKey: input.WorkflowKey, TerminalProgramCHeadSelector: input.TerminalProgramCHeadSelector,
		ReceiptSelector: input.ReceiptSelector, ReceiptOrdinal: input.ReceiptOrdinal,
		ConstituentOrdinal: input.ConstituentOrdinal, PublicationSelector: input.PublicationSelector,
		ConstituentSelector: input.ConstituentSelector, Source: input.Source,
		ProgramCAlgorithmVersion: input.ProgramCAlgorithmVersion, RunID: input.RunID,
		Seed: input.Seed, ReplayOrdinal: input.ReplayOrdinal, CommunityOrdinal: input.CommunityOrdinal,
		CommunityArtifactSHA256: input.CommunityArtifactSHA256, CommunityMembers: input.CommunityMembers,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := programcrepresentative.ParseMechanicsV2(v1.CanonicalBytes()); err == nil {
		t.Fatal("ASSERT_MECHANICS_V2_REJECTS_V1_SCHEMA")
	}
}

func TestMechanicsV2CanonicalFingerprint(t *testing.T) {
	got, err := programcrepresentative.NewMechanicsV2(mechanicsV2Input(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(got.CanonicalBytes())
	fingerprint := hex.EncodeToString(sum[:])
	const wantFingerprint = "61d38620c586fce0ed23512d4a0bc7e7c0664d870091b03c5ae026c2e711230a"
	const wantIdentity = "sha256:1d0da767f628fa4235dd08ce5ca8409f35dc6c570db61f2a938c35d146c637ba"
	if fingerprint != wantFingerprint || len(got.CanonicalBytes()) != 6631 || got.Identity() != wantIdentity {
		t.Fatalf("ASSERT_MECHANICS_V2_CANONICAL_FINGERPRINT sha256=%s bytes=%d identity=%s", fingerprint, len(got.CanonicalBytes()), got.Identity())
	}
	t.Logf("MECHANICS_V2_CANONICAL_SHA256=%s bytes=%d identity=%s", fingerprint, len(got.CanonicalBytes()), got.Identity())
}

type mechanicsV2SeedShape struct {
	prepared int
	reached  int
	relation int
}

type mechanicsV2ShapeCounts struct {
	seedGroups, memberships, maxMemberships, maxOccurrences, sumOccurrences int
}

func repeatedMechanicsV2Shape(count int, shape mechanicsV2SeedShape) []mechanicsV2SeedShape {
	out := make([]mechanicsV2SeedShape, count)
	for i := range out {
		out[i] = shape
	}
	return out
}

func mechanicsV2Counts(groups []programcrepresentative.SeedGroupIdentityDTOv2) mechanicsV2ShapeCounts {
	counts := mechanicsV2ShapeCounts{seedGroups: len(groups)}
	for _, group := range groups {
		counts.memberships += len(group.Memberships)
		counts.sumOccurrences += len(group.OccurrenceIDs)
		if len(group.Memberships) > counts.maxMemberships {
			counts.maxMemberships = len(group.Memberships)
		}
		if len(group.OccurrenceIDs) > counts.maxOccurrences {
			counts.maxOccurrences = len(group.OccurrenceIDs)
		}
	}
	return counts
}

func mechanicsV2ShapedInput(t *testing.T, shapes []mechanicsV2SeedShape, callSiteCounts []int) programcrepresentative.MechanicsInputV2 {
	t.Helper()
	maxNodes := len(callSiteCounts) + 1
	for _, shape := range shapes {
		if shape.prepared > maxNodes {
			maxNodes = shape.prepared
		}
		if shape.reached > maxNodes {
			maxNodes = shape.reached
		}
	}
	if maxNodes < 2 {
		maxNodes = 2
	}
	nodes := make([]graph.Node, maxNodes)
	for i := range nodes {
		line := uint32(i * 2)
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 2}}
		nodes[i] = graph.NewNode(graph.Item{Name: fmt.Sprintf("node-%03d", i), Kind: 12, URI: "file:///fixture/limits.go", Range: r, SelectionRange: r})
	}
	edges := make([]graph.Edge, len(callSiteCounts))
	for edgeIndex, count := range callSiteCounts {
		callSites := make([]graph.Range, count)
		for occurrenceIndex := range callSites {
			line := uint32(edgeIndex*1000 + occurrenceIndex)
			callSites[occurrenceIndex] = graph.Range{Start: graph.Position{Line: line, Character: 1}, End: graph.Position{Line: line, Character: 2}}
		}
		edges[edgeIndex] = graph.Edge{CallerNodeID: nodes[edgeIndex].ID, CalleeNodeID: nodes[edgeIndex+1].ID, CallSites: callSites}
	}
	canonical := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: append([]graph.Node{}, nodes...), Edges: edges}
	canonical.Canonicalize()
	edges = canonical.Edges
	seeds := make([]graph.InvocationSeed, len(shapes))
	results := make([]graph.SeedResult, len(shapes))
	for i, shape := range shapes {
		label := fmt.Sprintf("seed-%03d", i)
		seeds[i] = graph.InvocationSeed{Label: label, At: fmt.Sprintf("limits.go:%d:1", i+1), ResolvedURI: "file:///fixture/limits.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
		result := graph.SeedResult{Label: label}
		for _, node := range nodes[:shape.prepared] {
			result.PreparedTargetIDs = append(result.PreparedTargetIDs, node.ID)
		}
		for _, node := range nodes[:shape.reached] {
			result.ReachedNodeIDs = append(result.ReachedNodeIDs, node.ID)
		}
		if shape.relation >= 0 {
			result.ReachedRelationIDs = []string{edges[shape.relation].RelationID}
		}
		results[i] = result
	}
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "fixture-ls"}, Seeds: seeds, Provenance: graph.InvocationProvenance{InvocationID: "fixture-limits", SourceRevision: "fixture-revision", ServerVersion: "fixture-ls@1"}},
		Nodes:         nodes, Edges: edges, Seeds: results, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	return mechanicsV2InputFromResult(t, result)
}

func mechanicsV2FiveKindInput(t *testing.T) programcrepresentative.MechanicsInputV2 {
	t.Helper()
	node := func(name string, line uint32) graph.Node {
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 2}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///fixture/input.go", Range: r, SelectionRange: r})
	}
	caller, callee, candidate := node("caller", 0), node("callee", 3), node("candidate", 6)
	edges := []graph.Edge{{CallerNodeID: caller.ID, CalleeNodeID: callee.ID, CallSites: []graph.Range{
		{Start: graph.Position{Line: 0, Character: 1}, End: graph.Position{Line: 0, Character: 2}},
		{Start: graph.Position{Line: 1, Character: 1}, End: graph.Position{Line: 1, Character: 2}},
	}}}
	canonical := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: []graph.Node{caller, callee}, Edges: edges}
	canonical.Canonicalize()
	edges = canonical.Edges
	seeds := []graph.InvocationSeed{
		{Label: "seed-a", At: "input.go:1:1", ResolvedURI: "file:///fixture/input.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"},
		{Label: "seed-b", At: "input.go:2:1", ResolvedURI: "file:///fixture/input.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"},
	}
	results := make([]graph.SeedResult, len(seeds))
	siblings := make([]graph.SiblingCandidate, len(seeds))
	dispatch := make([]graph.DispatchRelationship, len(seeds))
	for i, seed := range seeds {
		results[i] = graph.SeedResult{Label: seed.Label, PreparedTargetIDs: []string{caller.ID}, ReachedNodeIDs: []string{caller.ID, callee.ID}, ReachedRelationIDs: []string{edges[0].RelationID}}
		siblings[i] = graph.SiblingCandidate{
			SeedURI: seed.ResolvedURI, SeedLabel: seed.Label,
			SeedIdentity: "fixture-session:" + seed.Label + ":" + seed.At,
			Direction:    "SIBLING", Kind: "TOPMOST_SIBLING", Origin: caller, Candidate: candidate,
			ProviderEvidence: []string{"command=fixture-ls;server_version=fixture-ls@1;invocation=fixture-session"},
			LSPEvidence:      []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"},
			SourceDigests:    []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256},
			Custody:          graph.SourceCustodyEvidence{Class: graph.SourceCustodyUnknown, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"},
		}
		dispatch[i] = graph.DispatchRelationship{SeedLabel: seed.Label, SeedLabels: []string{seed.Label}, Interface: caller, Implementation: callee}
	}
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "fixture-ls"}, Seeds: seeds, Provenance: graph.InvocationProvenance{InvocationID: "fixture-session", SourceRevision: "fixture-revision", ServerVersion: "fixture-ls@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true, DispatchFamily: true}},
		Nodes:         []graph.Node{callee, caller}, Edges: edges, Seeds: results,
		SiblingCandidates: siblings, DispatchRelationships: dispatch,
		Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	result.Canonicalize()
	return mechanicsV2InputFromResult(t, result)
}

func mechanicsV2Input(t *testing.T, seedCount int) programcrepresentative.MechanicsInputV2 {
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
	seeds := make([]graph.InvocationSeed, seedCount)
	results := make([]graph.SeedResult, seedCount)
	for i := range seeds {
		label := fmt.Sprintf("seed-%03d", i)
		seeds[i] = graph.InvocationSeed{Label: label, At: fmt.Sprintf("input.go:%d:1", i+1), ResolvedURI: "file:///fixture/input.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
		results[i] = graph.SeedResult{Label: label, PreparedTargetIDs: []string{a.ID}, ReachedNodeIDs: []string{a.ID, b.ID, c.ID}, ReachedRelationIDs: []string{edges[0].RelationID, edges[1].RelationID}}
	}
	result := graph.Result{
		SchemaVersion: graph.SchemaVersionV5,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "fixture-ls"}, Seeds: seeds, Provenance: graph.InvocationProvenance{InvocationID: "fixture-session", SourceRevision: "fixture-revision", ServerVersion: "fixture-ls@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}},
		Nodes:         []graph.Node{c, a, b}, Edges: edges, Seeds: results,
		Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	return mechanicsV2InputFromResult(t, result)
}

func mechanicsV2InputFromResult(t *testing.T, result graph.Result) programcrepresentative.MechanicsInputV2 {
	t.Helper()
	native, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	native = completeDispatchMembershipsV2(t, native)
	retained, err := graphprovenance.CaptureV5(native, "fixture-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatal(err)
	}
	outcome, failure := programc.Compute(retained, 1)
	if failure != nil || len(outcome.Communities) == 0 {
		t.Fatalf("fixture outcome=%+v failure=%v", outcome, failure)
	}
	return programcrepresentative.MechanicsInputV2{
		WorkflowKey: "workflow", TerminalProgramCHeadSelector: "sha256:" + strings.Repeat("1", 64),
		ReceiptSelector: "sha256:" + strings.Repeat("2", 64), ReceiptOrdinal: 1,
		ConstituentOrdinal: 0, PublicationSelector: "sha256:" + strings.Repeat("3", 64),
		ConstituentSelector: "sha256:" + strings.Repeat("4", 64), Source: outcome.Source,
		ProgramCAlgorithmVersion: "fixture-version",
		RunID:                    "seed-00000000000000000001-replay-1", Seed: 1, ReplayOrdinal: 1,
		CommunityOrdinal: 0, CommunityArtifactSHA256: "sha256:" + strings.Repeat("5", 64),
		CommunityMembers: append([]string(nil), outcome.Communities[0].Members...),
	}
}

func singletonMechanicsInputV2(in programcrepresentative.MechanicsInputV2) programcrepresentative.SingletonMechanicsInput {
	return programcrepresentative.SingletonMechanicsInput{
		WorkflowKey: in.WorkflowKey, TerminalProgramCHeadSelector: in.TerminalProgramCHeadSelector,
		ReceiptSelector: in.ReceiptSelector, ReceiptOrdinal: in.ReceiptOrdinal,
		ConstituentOrdinal: in.ConstituentOrdinal, PublicationSelector: in.PublicationSelector,
		ConstituentSelector: in.ConstituentSelector, Source: in.Source,
		ProgramCAlgorithmVersion: in.ProgramCAlgorithmVersion, RunID: in.RunID,
		Seed: in.Seed, ReplayOrdinal: in.ReplayOrdinal, CommunityOrdinal: in.CommunityOrdinal,
		CommunityArtifactSHA256: in.CommunityArtifactSHA256, CommunityMembers: append([]string{}, in.CommunityMembers...),
	}
}

func assertMechanicsV2ReusedJSON(t *testing.T, name string, v1, v2 any) {
	t.Helper()
	v1Raw, err := json.Marshal(v1)
	if err != nil {
		t.Fatal(err)
	}
	v2Raw, err := json.Marshal(v2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v1, v2) || !bytes.Equal(v1Raw, v2Raw) {
		t.Fatalf("ASSERT_MECHANICS_V2_EXACT_V1_REUSE_%s v1=%s v2=%s", name, v1Raw, v2Raw)
	}
}

func assertMechanicsV2Fingerprint(t *testing.T, name string, value any, want string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("ASSERT_MECHANICS_V2_%s_CANONICAL_FINGERPRINT got=%s bytes=%d want=%s", name, got, len(raw), want)
	}
}

func digestMechanicsV2(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func completeDispatchMembershipsV2(t *testing.T, raw []byte) []byte {
	t.Helper()
	var envelope struct {
		ExecutionBundleID     string                       `json:"execution_bundle_id"`
		Invocation            graph.Invocation             `json:"invocation"`
		SeedMemberships       []graph.SeedMembership       `json:"seed_memberships"`
		DispatchRelationships []graph.DispatchRelationship `json:"dispatch_relationships"`
		TraceReceipt          struct {
			SemanticCommitmentDigest string `json:"semantic_commitment_digest"`
		} `json:"trace_receipt"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.DispatchRelationships) == 0 {
		return raw
	}
	oldMemberships, err := json.Marshal(envelope.SeedMemberships)
	if err != nil {
		t.Fatal(err)
	}
	for _, relationship := range envelope.DispatchRelationships {
		for _, seed := range envelope.Invocation.Seeds {
			payload, marshalErr := json.Marshal([]string{seed.Label, seed.At, seed.ResolvedURI, "DISPATCH_ASSOCIATION", relationship.RelationID})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			envelope.SeedMemberships = append(envelope.SeedMemberships, graph.SeedMembership{
				MembershipID:      digestMechanicsV2Domain("lsp-trace:seed-membership:v1", payload),
				ExecutionBundleID: envelope.ExecutionBundleID, SeedLabel: seed.Label, SeedAt: seed.At,
				EvidenceKind: "DISPATCH_ASSOCIATION", EndpointID: relationship.RelationID,
			})
		}
	}
	sort.Slice(envelope.SeedMemberships, func(i, j int) bool {
		return envelope.SeedMemberships[i].MembershipID < envelope.SeedMemberships[j].MembershipID
	})
	newMemberships, err := json.Marshal(envelope.SeedMemberships)
	if err != nil {
		t.Fatal(err)
	}
	resealed := bytes.Replace(raw, oldMemberships, newMemberships, 1)
	marker := []byte(`,"trace_receipt":`)
	index := bytes.LastIndex(resealed, marker)
	if index < 0 {
		t.Fatal("missing V5 trace receipt")
	}
	semantic := append(append([]byte{}, resealed[:index]...), '}')
	newDigest := digestMechanicsV2Domain(graph.SemanticDigestDomainV5, semantic)
	resealed = bytes.Replace(resealed, []byte(envelope.TraceReceipt.SemanticCommitmentDigest), []byte(newDigest), 1)
	if err := graph.ValidateSemanticBundle(resealed); err != nil {
		t.Fatalf("complete canonical V5 dispatch memberships: %v", err)
	}
	return resealed
}

func digestMechanicsV2Domain(domain string, payload []byte) string {
	input := append(append([]byte(domain), 0), payload...)
	return digestMechanicsV2(input)
}

func containsMechanicsV2String(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cloneMechanicsV2DTO(t *testing.T, in programcrepresentative.MechanicsIdentityDTOv2) programcrepresentative.MechanicsIdentityDTOv2 {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out programcrepresentative.MechanicsIdentityDTOv2
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
