package programcpresentation

import (
	"bytes"
	"strings"
	"testing"

	"lsp-trace/internal/programc"
)

func validArtifact() Artifact {
	return Artifact{
		SchemaVersion: Version, Authority: Authority, Outcome: "EMPTY",
		Disclaimer: Disclaimer, CrossSeedStability: Stability,
		Communities: []Community{{
			CommunityID: "sha256:" + strings.Repeat("a", 64),
			Members: []Node{{NodeID: "n", Name: "Name", EnclosingDetail: "Container", Location: Location{
				URI: "file:///x", StartLine: 1, StartCharacter: 1, EndLine: 2, EndCharacter: 3,
			}}},
		}},
		CrossCommunityCalls: []Call{}, HighCentralityCrossingNodes: []programc.BoundaryNodeScore{},
		HubCrossingNodes: []programc.BoundaryNodeScore{}, CrossingWitnesses: []programc.CrossingWitness{},
		Bridges: []string{}, ArticulationPoints: []string{},
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
func TestMandatoryTopK(t *testing.T) {
	if _, err := Handle(Request{}); err == nil || !strings.Contains(err.Error(), "mandatory positive") {
		t.Fatalf("ASSERT_TOP_K_MANDATORY: %v", err)
	}
}
