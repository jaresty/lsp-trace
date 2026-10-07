package programcpresentation

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
)

func validArtifact() Artifact {
	id := "sha256:" + strings.Repeat("a", 64)
	members := []string{"n"}
	machineNode := graph.Node{ID: "n", Item: graph.Item{Name: "Name", Detail: "Container", Kind: 12, URI: "file:///x", Range: graph.Range{End: graph.Position{Line: 1, Character: 2}}, SelectionRange: graph.Range{}}}
	return Artifact{
		SchemaVersion: Version, Authority: Authority, Outcome: "EMPTY",
		ProfileID: programc.ProfileID, ProfileSHA256: programc.ProfileDigest, Algorithm: "fixture", PartitionSHA256: id,
		Policy: programc.BoundaryPolicy, Request: programc.BoundaryRequest{PageRankTopK: 1, HubTopK: 1}, ClaimCeiling: programc.BoundaryClaimCeiling,
		Disclaimer: Disclaimer, CrossSeedStability: Stability,
		Communities: []Community{{
			CommunityID: id,
			Members: []Node{{NodeID: "n", Name: "Name", EnclosingDetail: "Container", Location: Location{
				URI: "file:///x", StartLine: 1, StartCharacter: 1, EndLine: 2, EndCharacter: 3,
			}}}, Evidence: programc.BoundaryCommunity{CommunityID: id, Members: members},
		}},
		CrossCommunityCalls: []Call{}, HighCentralityCrossingNodes: []programc.BoundaryNodeScore{},
		HubCrossingNodes: []programc.BoundaryNodeScore{}, CrossingWitnesses: []programc.CrossingWitness{},
		Bridges: []string{}, ArticulationPoints: []string{}, machineNodes: map[string]graph.Node{"n": machineNode},
	}
}

func TestTextIsOneBasedCommunitiesFirstAndHonest(t *testing.T) {
	a := validArtifact()
	var out bytes.Buffer
	if err := Text(&out, a); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"COMMUNITIES", "Name (enclosing/detail: Container) @ file:///x:1:1-2:3", Disclaimer, Stability, "CROSS-COMMUNITY CALLS", "CONDUCTANCE", "PAGERANK", "HUBS", "CANONICAL WITNESSES", "BRIDGES", "ARTICULATION POINTS"} {
		if !strings.Contains(s, want) {
			t.Fatalf("ASSERT_TEXT_PRESENT_%q: %s", want, s)
		}
	}
	if strings.Index(s, "COMMUNITIES") > strings.Index(s, "CROSS-COMMUNITY CALLS") {
		t.Fatal("ASSERT_COMMUNITIES_FIRST")
	}
}
func TestValidationRejectsDuplicateAndCoordinates(t *testing.T) {
	a := validArtifact()
	a.Communities[0].Members = append(a.Communities[0].Members, a.Communities[0].Members[0])
	if Validate(a) == nil {
		t.Fatal("ASSERT_DUPLICATE_REJECTED")
	}
	a = validArtifact()
	a.Communities[0].Members[0].Location.StartLine = 0
	if Validate(a) == nil {
		t.Fatal("ASSERT_ZERO_BASED_REJECTED")
	}
}
func TestJSONExcludesOpaqueAndSourceBodies(t *testing.T) {
	b, err := JSON(validArtifact())
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"\"data\"", "source_body", "source_text"} {
		if bytes.Contains(b, []byte(bad)) {
			t.Fatalf("ASSERT_NO_LEAK_%s: %s", bad, b)
		}
	}
}

func TestJSONPreservesV1NodeAndDisplayLocation(t *testing.T) {
	b, err := JSON(validArtifact())
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Communities []struct {
			Members []struct {
				NodeID   string         `json:"node_id"`
				Location Location       `json:"location"`
				Target   map[string]any `json:"target"`
			} `json:"members"`
		} `json:"communities"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	member := wire.Communities[0].Members[0]
	if member.Location != (Location{URI: "file:///x", StartLine: 1, StartCharacter: 1, EndLine: 2, EndCharacter: 3}) {
		t.Fatalf("ASSERT_PROGRAM_C_DISPLAY_LOCATION_PRESERVED: %+v", member.Location)
	}
	if member.Target != nil || bytes.Contains(b, []byte(`"target"`)) {
		t.Fatalf("ASSERT_PROGRAM_C_V1_PUBLIC_NODE_UNCHANGED: %s", b)
	}
}
func TestMachineTargetUsesRealMemberSelectionStarts(t *testing.T) {
	for _, tc := range []struct {
		name       string
		line, char uint32
		kind       int
		wantLine   int
		wantChar   int
	}{
		{name: "Validate", line: 236, char: 5, kind: 12, wantLine: 236, wantChar: 5},
		{name: "RunnerAdapter.Run", line: 39, char: 23, kind: 6, wantLine: 39, wantChar: 23},
	} {
		n := graph.NewNode(graph.Item{Name: tc.name, Kind: tc.kind, URI: "file:///workspace/member.go", Range: graph.Range{Start: graph.Position{Line: tc.line}}, SelectionRange: graph.Range{Start: graph.Position{Line: tc.line, Character: tc.char}}})
		got := machineTarget(n)
		if got.Line != tc.wantLine || got.Character != tc.wantChar || got.PositionEncoding != "utf-16" || got.CoordinateBase != 0 || got.RangeRole != "SELECTION_RANGE" || got.NodeID != n.ID || got.NodeKind != tc.kind {
			t.Fatalf("ASSERT_REAL_MEMBER_MACHINE_TARGET_%s: %+v", tc.name, got)
		}
	}
}

func TestPrivateMachineTargetRoundTripAndIdentityValidation(t *testing.T) {
	a := validArtifact()
	handoff, err := PrivateMachineTargetHandoff(a)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(handoff)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip MachineTargetHandoff
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, handoff) {
		t.Fatalf("ASSERT_PROGRAM_C_PRIVATE_TARGET_ROUND_TRIP_IDENTITY: got=%+v want=%+v", roundTrip, handoff)
	}
	if err := ValidatePrivateMachineTargetHandoff(a, roundTrip); err != nil {
		t.Fatalf("ASSERT_PROGRAM_C_PRIVATE_TARGET_ROUND_TRIP_VALID: %v", err)
	}
}

func TestPrivateMachineTargetValidationRejectsNoncanonicalOrder(t *testing.T) {
	a := validArtifact()
	second := graph.Node{ID: "z", Item: graph.Item{Name: "Second", Kind: 12, URI: "file:///z", Range: graph.Range{End: graph.Position{Line: 1}}, SelectionRange: graph.Range{}}}
	a.machineNodes[second.ID] = second
	handoff, err := PrivateMachineTargetHandoff(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoff.Targets) != 2 || handoff.Targets[0].NodeID >= handoff.Targets[1].NodeID {
		t.Fatalf("ASSERT_PROGRAM_C_PRIVATE_TARGET_CANONICAL_FIXTURE: %+v", handoff.Targets)
	}
	handoff.Targets[0], handoff.Targets[1] = handoff.Targets[1], handoff.Targets[0]
	if err := ValidatePrivateMachineTargetHandoff(a, handoff); err == nil {
		t.Fatal("ASSERT_PROGRAM_C_PRIVATE_TARGET_ORDER_REJECTED")
	}
}

func TestMachineTargetValidationRejectsResealedIdentityMutations(t *testing.T) {
	for _, mutation := range []struct {
		name string
		edit func(*MachineTarget)
	}{
		{name: "line", edit: func(target *MachineTarget) { target.Line++ }},
		{name: "character", edit: func(target *MachineTarget) { target.Character++ }},
		{name: "uri", edit: func(target *MachineTarget) { target.URI = "file:///shifted.go" }},
		{name: "kind", edit: func(target *MachineTarget) { target.NodeKind++ }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			a := validArtifact()
			handoff, err := PrivateMachineTargetHandoff(a)
			if err != nil {
				t.Fatal(err)
			}
			mutation.edit(&handoff.Targets[0])
			if err := ValidatePrivateMachineTargetHandoff(a, handoff); err == nil {
				t.Fatalf("ASSERT_PROGRAM_C_RESEALED_MACHINE_TARGET_IDENTITY_REJECTED_%s", mutation.name)
			}
		})
	}
}

func TestMandatoryTopK(t *testing.T) {
	if _, err := Handle(Request{}); err == nil || !strings.Contains(err.Error(), "mandatory positive") {
		t.Fatalf("ASSERT_TOP_K_MANDATORY: %v", err)
	}
}
