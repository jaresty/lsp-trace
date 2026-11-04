package programcrepresentative

import (
	"bytes"
	"encoding/json"
	"testing"
)

func multiSeedTestCustodyV2() CustodyIdentityDTO {
	d := rawDigest([]byte("test-only"))
	c := CustodyIdentityDTO{SchemaVersion: CustodySchemaVersion, WorkflowKey: "test", TerminalProgramCHeadSelector: d, ReceiptSelector: d, ReceiptOrdinal: 1, ConstituentOrdinal: 0, PublicationSelector: "publication", ConstituentSelector: "constituent", NativeV5SHA256: d, NativeV5ByteLength: 9, ExecutionBundleID: "bundle"}
	c.Identity, _ = hashIdentity(custodyDomain, c)
	return c
}
func multiSeedTestQualificationV2(m MechanicsV2) CheckedQualificationCalculationV2 {
	// This fixture function, and every synthetic authority it produces, is test
	// only. Production has no pin declaration or independent-review substitute.
	c := multiSeedTestCustodyV2()
	d := rawDigest([]byte("test-only-envelope"))
	return CheckedQualificationCalculationV2{state: &checkedQualificationCalculationStateV2{envelope: ArtifactBindingDTO{SchemaVersion: "lsp-trace.program-c-representative-qualification.private.v2", Identity: d, SHA256: d, ByteLength: 20}, terminalGeneration: 12, mechanics: map[string][]byte{m.Identity(): append([]byte{}, m.canonical...)}, custody: map[string]CustodyIdentityDTO{c.Identity: c}}}
}
func TestMultiSeedV2ResultCanonicalAndCopies(t *testing.T) {
	t.Log("procedure: execute finalization, parse exact result, mutate nested returned groups, reject resealed wire substitutions")
	m := multiSeedFinalizationFixture(t)
	q := multiSeedTestQualificationV2(m)
	result, err := FinalizeMultiSeedV2(m, q)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := result.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] == '\n' {
		t.Fatal("CANONICAL_NO_LF")
	}
	parsed, err := ParseMultiSeedResultV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Calculated() || !result.Calculated() {
		t.Fatal("PARSE_IS_NOT_QUALIFICATION")
	}
	dto, err := result.DTO()
	if err != nil {
		t.Fatal(err)
	}
	if dto.Terminal != "GROUPS_TERMINAL_COMPLETE" || len(dto.GroupResults) != 3 || dto.Qualification.ValidFrom != 12 || dto.Qualification.ValidThrough != 12 {
		t.Fatal("EXECUTED_ORDERED_FINALIZATION")
	}
	dto.Composition.Mechanics.SelectionContext.SeedGroups[0].PreparedTargets[0] = "changed"
	dto.Grouping.SelectionContext.SeedGroups[0].Memberships[0].EndpointID = "changed"
	dto.GroupResults[0].Record.SCCMembers[0] = "changed"
	dto.GroupResults[0].Record.IncomingPredecessors[0].RelationID = "changed"
	again, _ := result.CanonicalBytes()
	fresh, _ := result.DTO()
	if !bytes.Equal(again, raw) || fresh.GroupResults[0].Record.SCCMembers[0] != "b" || fresh.Composition.Mechanics.SelectionContext.SeedGroups[0].PreparedTargets[0] != "a" || fresh.Grouping.SelectionContext.SeedGroups[0].Memberships[0].EndpointID != "a" || fresh.GroupResults[0].Record.IncomingPredecessors[0].RelationID != "ab" {
		t.Fatal("DEFENSIVE_NESTED_RESULT_COPY")
	}
	raw[0] = 'x'
	again, _ = result.CanonicalBytes()
	if again[0] != '{' {
		t.Fatal("DEFENSIVE_CANONICAL_COPY")
	}
}
func TestMultiSeedV2ResultStrictRefusal(t *testing.T) {
	m := multiSeedFinalizationFixture(t)
	good, err := FinalizeMultiSeedV2(m, multiSeedTestQualificationV2(m))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := good.CanonicalBytes()
	for _, name := range []string{"omission", "reorder", "seed-substitution", "false-singleton", "wrong-accounting", "nonselection-node", "mixed-version", "authority"} {
		t.Run(name, func(t *testing.T) {
			dto, _ := good.DTO()
			switch name {
			case "omission":
				dto.GroupResults = dto.GroupResults[:2]
			case "reorder":
				dto.GroupResults[0], dto.GroupResults[1] = dto.GroupResults[1], dto.GroupResults[0]
			case "seed-substitution":
				dto.GroupResults[0].SeedGroupIdentity = dto.GroupResults[1].SeedGroupIdentity
			case "false-singleton":
				dto.GroupResults = dto.GroupResults[:1]
				dto.GroupAccounting = GroupAccountingV2{Expected: 1, Selected: 1}
			case "wrong-accounting":
				dto.GroupAccounting.NoCalls = 0
				dto.GroupAccounting.Selected = 2
			case "nonselection-node":
				dto.GroupResults[1].Record.SelectedNode = "b"
			case "mixed-version":
				dto.SchemaVersion = "lsp-trace.program-c-representative-result.private.v1"
			case "authority":
				dto.Authority = 1
			}
			dto.Identity = ""
			n, err := CanonicalSizeResultV2(dto)
			pre, err := marshalSizedV2(dto, n, err)
			if err != nil {
				t.Fatal(err)
			}
			dto.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-result:v2", pre)
			changed, _ := json.Marshal(dto)
			if value, err := ParseMultiSeedResultV2(changed); err == nil || value.Identity() != "" {
				t.Fatal("RESEALED_RESULT_REFUSAL")
			}
		})
	}
	for _, changed := range [][]byte{append(append([]byte{}, raw...), '\n'), bytes.Replace(raw, []byte(`"schema_version":`), []byte(`"Schema_Version":`), 1), bytes.Replace(raw, []byte(`"schema_version":`), []byte(`"extra":0,"schema_version":`), 1), bytes.Replace(raw, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1)} {
		if _, err := ParseMultiSeedResultV2(changed); err == nil {
			t.Fatal("STRICT_CANONICAL_WIRE_REFUSAL")
		}
	}
	if r, err := FinalizeMultiSeedV2(m, CheckedQualificationCalculationV2{}); err == nil || r.Identity() != "" {
		t.Fatal("NO_UNREVIEWED_QUALIFICATION")
	}
}
func TestMultiSeedV2FixedArtifacts(t *testing.T) {
	p, pb, pr, err := BuildStage6PolicyArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	l, lb, lr, err := BuildStage6LimitsArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	f, fb, fr, err := BuildStage6FinalizationArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	if !p.NoOuterWinner || p.RelationOccurrenceClosure != "EXACT_ALL_CALLS_OCCURRENCES_PER_SEED_GROUP_RELATION" || p.OuterTerminalRule != "ALL_EXPECTED_GROUPS_QUALIFIED_TERMINAL" {
		t.Fatal("FIXED_MULTISEED_POLICY")
	}
	if l.MaxCanonicalBytes != 67108864 || l.MaxOuterWork != 10000 || l.MaxRunSourceWork != 20000 || l.MaxSeedGroups != 64 || l.MaxMemberships != 2048 || l.MaxMembershipsPerGroup != 256 || l.MaxGroupOccurrences != 256 || l.MaxSumGroupOccurrences != 2048 {
		t.Fatal("FIXED_MULTISEED_LIMITS")
	}
	if len(f.FinalizationOrder) != 5 || f.FinalizationOrder[4] != "OUTER_TERMINAL_RESULT" || len(f.GroupMappings) != 4 {
		t.Fatal("FIXED_FINALIZATION_ORDER")
	}
	for _, pair := range []struct {
		b   ArtifactBindingDTO
		raw []byte
	}{{pb, pr}, {lb, lr}, {fb, fr}} {
		if pair.b.SHA256 != rawDigest(pair.raw) || pair.b.ByteLength != uint64(len(pair.raw)) {
			t.Fatal("POPULATED_BINDING")
		}
	}
}
