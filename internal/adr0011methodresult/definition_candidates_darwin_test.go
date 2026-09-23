//go:build darwin

package adr0011methodresult

import (
	"context"
	"errors"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
)

func TestDefinitionCandidateProjectionKeepsTargetsAndResponseOrdinals(t *testing.T) {
	for _, tc := range []struct {
		method, mode string
		count        int
		kind         Kind
		wantFailure  bool
	}{
		{transport.MethodDefinition, "D-04", 2, Location, false},
		{transport.MethodDefinition, "D-05", 2, LocationLink, false},
		{transport.MethodDefinition, "", 0, "", false},
		{transport.MethodReferences, "R-05", 2, Location, true},
	} {
		t.Run(tc.method+"/"+tc.mode, func(t *testing.T) {
			manager, started, _ := startManagedMethodPeer(t, tc.mode)
			observed := &methodWireObservedRuntime{Manager: manager}
			req := fixtureRequest(tc.method, false)
			req.SessionID, req.Generation = started.SessionID, started.Generation
			req.Deadline = time.Now().Add(5 * time.Second)
			wire := transport.New(observed).Execute(context.Background(), req)
			candidate, err := BuildCanonicalCandidate(req, wire, 8)
			if err != nil {
				t.Fatalf("managed candidate: %v", err)
			}
			c, err := VerifyCanonicalCandidate(candidate)
			if err != nil {
				t.Fatal(err)
			}
			member := CandidateQueryMember{OccurrenceID: "predeclared-query", Began: true,
				Envelope: EnvelopeObservation{Known: true, Form: EnvelopeArray, Count: tc.count}, Terminal: CandidateItems}
			if tc.count == 0 {
				member.Terminal = CandidateEmpty
				member.Envelope.Form = EnvelopeNull
			}
			for ordinal := 0; ordinal < tc.count; ordinal++ {
				member.Elements = append(member.Elements, CandidateElement{Ordinal: ordinal, Began: true, Terminal: CandidateValidElement})
			}
			ledger := CandidateLedger{Queries: []CandidateQuery{{OccurrenceID: member.OccurrenceID,
				SessionID: req.SessionID, Generation: req.Generation, Method: req.Method, ParamsSHA256: c.ParamsSHA256}},
				Members: []CandidateQueryMember{member}}
			got, err := ProjectDefinitionCandidates(candidate, ledger, member.OccurrenceID)
			if tc.wantFailure {
				if !errors.Is(err, ErrDefinitionCandidate) || len(got) != 0 {
					t.Fatalf("references turned into definitions: %+v %v", got, err)
				}
				return
			}
			if err != nil || len(got) != tc.count {
				t.Fatalf("unadmitted definition targets missing: %+v %v", got, err)
			}
			for ordinal, item := range got {
				if item.Ordinal != ordinal || item.TargetKind != tc.kind || item.OccurrenceID == "" || item.TargetID == "" || item.QueryOccurrenceID != member.OccurrenceID || item.QueryURI != "file:///w/q.go" {
					t.Fatalf("candidate direction/ordinal lost: %+v", item)
				}
				if tc.kind == LocationLink && item.TargetRange == nil {
					t.Fatal("LocationLink target range lost")
				}
			}
			if tc.count == 2 && (got[0].OccurrenceID == got[1].OccurrenceID || got[0].TargetID != got[1].TargetID) {
				t.Fatalf("repeated target occurrence collapsed or split: %+v", got)
			}
			if _, err := ProjectDefinitionCandidates(candidate, ledger, "result-inferred-query"); !errors.Is(err, ErrDefinitionCandidate) {
				t.Fatalf("query substitution accepted: %v", err)
			}
		})
	}
}
