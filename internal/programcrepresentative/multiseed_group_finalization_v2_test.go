package programcrepresentative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// All synthetic custody/trust in this fixture is test-local; it is not an
// implementation pin, independent review, or a positive qualification anchor.
func multiSeedFinalizationFixture(t *testing.T) MechanicsV2 {
	t.Helper()
	digest := rawDigest([]byte("test-only"))
	evidence := []EvidenceIdentityDTO{}
	for i, edge := range [][3]string{{"ab", "a", "b"}, {"bb", "b", "b"}} {
		e := EvidenceIdentityDTO{SchemaVersion: EvidenceSchemaVersion, ConstituentSelector: "constituent", ExecutionBundleID: "bundle", RelationID: edge[0], CallSiteOrdinal: 0, FromNodeID: edge[1], ToNodeID: edge[2], CallSite: RangeDTO{Start: PositionDTO{Line: uint32(i)}, End: PositionDTO{Line: uint32(i), Character: 1}}, ReceiptSelector: digest}
		e.Identity, _ = hashIdentity(evidenceDomain, e)
		evidence = append(evidence, e)
	}
	groups := []SeedGroupIdentityDTOv2{}
	for i, relations := range [][]string{{"ab"}, {}, {"bb"}} {
		g := SeedGroupIdentityDTOv2{SchemaVersion: SeedGroupSchemaVersionV2, ExecutionBundleID: "bundle", InvocationSeedOrdinal: i, SeedLabel: fmt.Sprint(i), SeedAt: "file:///x#x", ResolvedURI: "file:///x", ContentSHA256: digest, LanguageID: "go", Memberships: []MembershipIdentityDTO{}, PreparedTargets: []string{"a"}, ReachedNodes: []string{"b"}, RelationIDs: append([]string{}, relations...), OccurrenceIDs: []string{}}
		g.Memberships = append(g.Memberships, MembershipIdentityDTO{MembershipID: fmt.Sprintf("m%d", i), ExecutionBundleID: g.ExecutionBundleID, SeedLabel: g.SeedLabel, SeedAt: g.SeedAt, EvidenceKind: "PREPARED_TARGET", EndpointID: "a"})
		for _, e := range evidence {
			for _, r := range relations {
				if r == e.RelationID {
					g.OccurrenceIDs = append(g.OccurrenceIDs, e.Identity)
				}
			}
		}
		g.Identity, _ = hashIdentity(seedGroupDomainV2, g)
		groups = append(groups, g)
	}
	custody := multiSeedTestCustodyV2()
	context := SelectionContextIdentityDTOv2{SchemaVersion: SelectionContextSchemaVersionV2, AdapterContractSHA256: digest, CustodyIdentity: custody.Identity, ConstituentSelector: "constituent", ExecutionBundleID: "bundle", RunID: fmt.Sprintf("seed-%020d-replay-1", 1), Seed: 1, ReplayOrdinal: 1, CommunityOrdinal: 0, CommunityArtifactSHA256: digest, CommunityMembers: []string{"a", "b"}, SeedGroups: groups}
	context.Identity, _ = hashIdentity(contextDomainV2, context)
	dto := MechanicsIdentityDTOv2{SchemaVersion: MechanicsSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, AdapterContractSHA256: digest, CustodyIdentity: custody.Identity, ReceiptSelector: digest, RunID: context.RunID, Seed: 1, ReplayOrdinal: 1, CommunityOrdinal: 0, CommunityArtifactSHA256: digest, Evidence: evidence, CommunityMembers: []string{"a", "b"}, SelectionContext: context}
	dto.Identity, _ = hashIdentity(mechanicsDomainV2, dto)
	raw, _ := json.Marshal(dto)
	return MechanicsV2{dto: dto, canonical: raw, trusted: true}
}

func TestMultiSeedV2GroupLocalTerminals(t *testing.T) {
	t.Log("procedure: finalize all ordered groups; compare SELECTED/NO_CALLS/CALLS_INELIGIBLE and complete group identities")
	m := multiSeedFinalizationFixture(t)
	records, a, err := computeGroupRecordsV2(m)
	if err != nil || len(records) != 3 {
		t.Fatalf("ALL_ORDERED_GROUPS: got=%d err=%v", len(records), err)
	}
	want := []string{"SELECTED", "NO_CALLS_REPRESENTATIVE", "CALLS_EVIDENCE_INELIGIBLE"}
	for i, r := range records {
		if r.InvocationSeedOrdinal != i || r.SeedGroupIdentity != m.dto.SelectionContext.SeedGroups[i].Identity || r.Record.Outcome != want[i] {
			t.Fatalf("GROUP_LOCAL_OUTCOME_%d: %+v", i, r)
		}
		if i > 0 && (r.Record.SelectedNode != "" || r.Record.Distance != 0 || len(r.Record.SCCMembers) != 0 || r.Record.SCCMembers == nil || r.Record.IncomingPredecessors == nil || len(r.Record.IncomingPredecessors) != 0) {
			t.Fatalf("NONSELECTION_EXACT_EMPTY_%d", i)
		}
	}
	if records[0].Record.SelectedNode != "b" || records[0].Record.Distance != 1 || len(records[0].Record.IncomingPredecessors) != 1 {
		t.Fatalf("UNCHANGED_SCC_DISTANCE_WITNESS: %+v", records[0])
	}
	if a != (GroupAccountingV2{Expected: 3, Selected: 1, NoCalls: 1, CallsIneligible: 1}) {
		t.Fatalf("GROUP_ACCOUNTING: %+v", a)
	}
	before := append([]byte{}, m.canonical...)
	records[0].Record.SCCMembers[0] = "changed"
	again, _, err := computeGroupRecordsV2(m)
	if err != nil || again[0].Record.SCCMembers[0] != "b" || !bytes.Equal(before, m.canonical) {
		t.Fatal("DEFENSIVE_GROUP_RECORD_COPY")
	}
}

func TestMultiSeedV2GroupClosureRefusals(t *testing.T) {
	for _, name := range []string{"omission", "reorder", "seed-substitution", "closure-omission", "foreign-occurrence", "untrusted"} {
		t.Run(name, func(t *testing.T) {
			m := multiSeedFinalizationFixture(t)
			switch name {
			case "omission":
				m.dto.SelectionContext.SeedGroups = m.dto.SelectionContext.SeedGroups[:2]
			case "reorder":
				m.dto.SelectionContext.SeedGroups[0], m.dto.SelectionContext.SeedGroups[1] = m.dto.SelectionContext.SeedGroups[1], m.dto.SelectionContext.SeedGroups[0]
			case "seed-substitution":
				m.dto.SelectionContext.SeedGroups[0].SeedLabel = "foreign"
			case "closure-omission":
				m.dto.SelectionContext.SeedGroups[0].OccurrenceIDs = []string{}
			case "foreign-occurrence":
				m.dto.SelectionContext.SeedGroups[1].OccurrenceIDs = []string{m.dto.Evidence[0].Identity}
			case "untrusted":
				m.trusted = false
			}
			// Even a resealed wire mutation cannot replace the opaque retained source.
			if name != "untrusted" {
				for i := range m.dto.SelectionContext.SeedGroups {
					g := &m.dto.SelectionContext.SeedGroups[i]
					g.Identity = ""
					g.Identity, _ = hashIdentity(seedGroupDomainV2, *g)
				}
				m.dto.SelectionContext.Identity = ""
				m.dto.SelectionContext.Identity, _ = hashIdentity(contextDomainV2, m.dto.SelectionContext)
				m.dto.Identity = ""
				m.dto.Identity, _ = hashIdentity(mechanicsDomainV2, m.dto)
			}
			r, _, err := computeGroupRecordsV2(m)
			if err == nil || r != nil {
				t.Fatal("RESEALED_GROUP_REFUSAL")
			}
		})
	}
}

func TestMultiSeedV2TypedSizeShapes(t *testing.T) {
	m := multiSeedFinalizationFixture(t)
	records, accounting, err := computeGroupRecordsV2(m)
	if err != nil {
		t.Fatal(err)
	}
	r := ResultV2{SchemaVersion: ResultSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, Composition: CompositionV2{Mechanics: m.dto}, Grouping: GroupingV2{SelectionContext: m.dto.SelectionContext}, GroupResults: records, GroupAccounting: accounting}
	n, err := CanonicalSizeResultV2(r)
	raw, _ := json.Marshal(r)
	if err != nil || n != uint64(len(raw)) {
		t.Fatalf("EXACT_TYPED_RESULT_SIZE: got=%d want=%d err=%v", n, len(raw), err)
	}
	var z canonicalSizerV2
	z.n = MaxArtifactCanonicalBytesV2 - 1
	z.add(1)
	if z.err != nil {
		t.Fatal("CAP_EQUALITY_ADMITTED")
	}
	z.add(1)
	if z.err == nil {
		t.Fatal("CAP_PLUS_ONE_REFUSED")
	}
	z = canonicalSizerV2{n: ^uint64(0)}
	z.add(1)
	if z.err == nil {
		t.Fatal("CHECKED_UINT64_OVERFLOW")
	}
	if !reflect.DeepEqual(records[1].Record.SCCMembers, []string{}) {
		t.Fatal("EMPTY_ARRAY_NOT_NULL")
	}
}
