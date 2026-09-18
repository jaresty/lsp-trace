package sourceprojection

import "testing"

func TestC01EveryCandidateIdentityFieldIsSensitive(t *testing.T) {
	base := Candidate{
		Role: "RELATION", GraphSubjectID: "relation-subject", OccurrenceID: "occurrence",
		LogicalSourceID: "file:///source.go", Range: Range{Start: Position{Line: 1, Character: 2}, End: Position{Line: 3, Character: 4}},
		PositionEncoding: "utf-16", PrivacyClassification: "PUBLIC",
	}
	base.UnitID = candidateUnitID(base)
	base.CitationID = candidateCitationID(base)
	for _, tc := range []struct {
		name   string
		mutate func(*Candidate)
	}{
		{name: "role", mutate: func(c *Candidate) { c.Role = "ENDPOINT" }},
		{name: "graph-subject", mutate: func(c *Candidate) { c.GraphSubjectID = "other-subject" }},
		{name: "occurrence", mutate: func(c *Candidate) { c.OccurrenceID = "other-occurrence" }},
		{name: "logical-source", mutate: func(c *Candidate) { c.LogicalSourceID = "file:///other.go" }},
		{name: "range-start", mutate: func(c *Candidate) { c.Range.Start.Character++ }},
		{name: "range-end", mutate: func(c *Candidate) { c.Range.End.Character++ }},
		{name: "position-encoding", mutate: func(c *Candidate) { c.PositionEncoding = "utf-8" }},
		{name: "privacy-class", mutate: func(c *Candidate) { c.PrivacyClassification = "RESTRICTED" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := base
			tc.mutate(&mutated)
			mutated.UnitID = candidateUnitID(mutated)
			mutated.CitationID = candidateCitationID(mutated)
			if mutated.UnitID == base.UnitID {
				t.Fatalf("ASSERT_C01_%s_UNIT_ID_SENSITIVE: base=%s mutated=%s", tc.name, base.UnitID, mutated.UnitID)
			}
			if mutated.CitationID == base.CitationID {
				t.Fatalf("ASSERT_C01_%s_CITATION_ID_SENSITIVE: base=%s mutated=%s", tc.name, base.CitationID, mutated.CitationID)
			}
		})
	}
}
