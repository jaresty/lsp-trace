package targetpacket

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv2"
)

func canonicalObject(t *testing.T, p Packet) map[string]json.RawMessage {
	t.Helper()
	raw, err := EncodeCanonical(p)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestConsumerZeroPredecessorV2Compatibility(t *testing.T) {
	r := validRequest(t)
	out, err := Build(r)
	if err != nil || len(out.Packets) != 1 {
		t.Fatalf("ASSERT_CONSUMER_ONE_PACKET_PER_NOMINATION: packets=%d err=%v", len(out.Packets), err)
	}
	fields := canonicalObject(t, out.Packets[0])
	if !bytes.Equal(fields["consumer_resolution"], []byte(`"OUTWARD_CONSUMER_UNRESOLVED"`)) {
		t.Fatalf("ASSERT_CONSUMER_ZERO_EXPLICIT_UNRESOLVED: %s", fields["consumer_resolution"])
	}
	if !bytes.Equal(fields["consumer_alternatives"], []byte(`[]`)) {
		t.Fatalf("ASSERT_CONSUMER_ZERO_NO_ARBITRARY_CALLER: %s", fields["consumer_alternatives"])
	}
}

func TestConsumerResolutionCoveredByCanonicalIdentity(t *testing.T) {
	r := validRequest(t)
	out, err := Build(r)
	if err != nil {
		t.Fatal(err)
	}
	base := out.Packets[0]
	raw, err := EncodeCanonical(base)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["consumer_resolution"] = json.RawMessage(`"RESOLVED"`)
	mutated, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(mutated); err == nil {
		t.Fatal("ASSERT_CONSUMER_RESOLUTION_CANONICAL_TAMPER_REJECTED: accepted")
	}
}

func consumerProof(predecessors ...censusprogramc.RepresentativePredecessor) sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding] {
	wire := sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]{}
	seenCaller := map[string]bool{}
	for _, p := range predecessors {
		logical := "file:///" + p.CallerID + ".go"
		if !seenCaller[p.CallerID] {
			seenCaller[p.CallerID] = true
			wire.Units = append(wire.Units, sourceprojectionv2.Unit{UnitID: "endpoint-" + p.CallerID, Role: "ENDPOINT", GraphSubjectID: p.CallerID, LogicalSourceID: logical})
		}
		rangeValue := projectionRange(p.CallSite)
		digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		unit := sourceprojectionv2.Unit{UnitID: "relation-" + p.OccurrenceID, Role: "RELATION", GraphSubjectID: p.RelationID, OccurrenceID: p.OccurrenceID, LogicalSourceID: logical, EvidenceRange: rangeValue, DisplayRange: rangeValue, PositionEncoding: "utf-16", SourceDigest: digest, SourceByteLength: 3, BodyDisposition: "RETURNED", Body: "abc"}
		wire.Units = append(wire.Units, unit)
		wire.Citations = append(wire.Citations, sourceprojectionv2.Citation{UnitID: unit.UnitID, Role: unit.Role, SubjectID: p.RelationID, OccurrenceID: p.OccurrenceID, EvidenceRange: rangeValue, DisplayRange: rangeValue})
		wire.EmittedSpans = append(wire.EmittedSpans, sourceprojection.Span{LogicalSourceID: logical, Range: rangeValue, SourceDigest: digest, ByteLength: 3, UnitIDs: []string{unit.UnitID}, Body: "abc"})
	}
	return wire
}

func predecessor(relation, occurrence, caller string, line uint32) censusprogramc.RepresentativePredecessor {
	return censusprogramc.RepresentativePredecessor{RelationID: relation, OccurrenceID: occurrence, CallerID: caller, TargetID: "target", CallSite: graph.Range{Start: graph.Position{Line: line, Character: 1}, End: graph.Position{Line: line, Character: 4}}}
}

func TestConsumerAlternativesExactAndDeterministic(t *testing.T) {
	a := predecessor("relation-b", "occ-b", "caller-b", 2)
	b := predecessor("relation-a", "occ-a", "caller-a", 1)
	wire := consumerProof(a, b)
	got, err := reconcileConsumerAlternatives([]censusprogramc.RepresentativePredecessor{a, b}, wire, retainedprojection.RetainedCustodyBinding{})
	if err != nil || len(got) != 2 {
		t.Fatalf("ASSERT_CONSUMER_ONE_MANY_ALTERNATIVES: got=%+v err=%v", got, err)
	}
	shuffled, err := reconcileConsumerAlternatives([]censusprogramc.RepresentativePredecessor{b, a}, wire, retainedprojection.RetainedCustodyBinding{})
	if err != nil || !reflect.DeepEqual(got, shuffled) {
		t.Fatalf("ASSERT_CONSUMER_SHUFFLE_CANONICAL_ORDER: got=%+v shuffled=%+v err=%v", got, shuffled, err)
	}
	if got[0].RelationID != b.RelationID || got[0].OccurrenceID != b.OccurrenceID || got[0].CallSite != b.CallSite || got[0].CallerID != b.CallerID || got[0].TargetID != b.TargetID || got[0].ReconciliationID == "" {
		t.Fatalf("ASSERT_CONSUMER_EXACT_OCCURRENCE_RANGE_ENDPOINTS: %+v", got[0])
	}
	mutated := got[0]
	mutated.CallerID = "substituted"
	if mutated.ReconciliationID == consumerReconciliationID(mutated) {
		t.Fatal("ASSERT_CONSUMER_RECONCILIATION_ID_REJECTS_COORDINATED_SUBSTITUTION")
	}
	before := append([]string(nil), got[1].RelationSpan.UnitIDs...)
	got[0].RelationSpan.UnitIDs[0] = "mutated"
	if !reflect.DeepEqual(got[1].RelationSpan.UnitIDs, before) || wire.EmittedSpans[1].UnitIDs[0] == "mutated" {
		t.Fatal("ASSERT_CONSUMER_DEFENSIVE_MUTATION_NO_ALIAS")
	}
}

func TestConsumerRelationProofRejections(t *testing.T) {
	p := predecessor("relation", "occurrence", "caller", 3)
	base := consumerProof(p)
	cases := []struct {
		name string
		edit func(*sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding])
	}{
		{"missing", func(w *sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]) {
			w.Units = w.Units[:1]
		}},
		{"duplicate", func(w *sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]) {
			w.Units = append(w.Units, w.Units[1])
		}},
		{"reversed-or-foreign-target", func(w *sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]) {
			w.Units[1].GraphSubjectID = "foreign-relation"
		}},
		{"substituted-source", func(w *sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]) {
			w.Units[1].LogicalSourceID = "file:///foreign.go"
		}},
		{"substituted-display", func(w *sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]) {
			w.Units[1].DisplayRange.End.Character++
		}},
		{"substituted-range", func(w *sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]) {
			w.Units[1].EvidenceRange.End.Character++
		}},
		{"ambiguous-citation", func(w *sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]) {
			w.Citations = append(w.Citations, w.Citations[0])
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wire := base
			wire.Units = append([]sourceprojectionv2.Unit(nil), base.Units...)
			wire.Citations = append([]sourceprojectionv2.Citation(nil), base.Citations...)
			wire.EmittedSpans = append([]sourceprojection.Span(nil), base.EmittedSpans...)
			tc.edit(&wire)
			if _, err := reconcileConsumerAlternatives([]censusprogramc.RepresentativePredecessor{p}, wire, retainedprojection.RetainedCustodyBinding{}); err == nil {
				t.Fatalf("ASSERT_CONSUMER_RELATION_%s_REJECTED", tc.name)
			}
		})
	}
}

func TestConsumerV2PredecessorFailsTypedAndSafe(t *testing.T) {
	r := validRequest(t)
	n := &r.Census.Representatives.Nominations[0]
	n.IncomingPredecessors = []censusprogramc.RepresentativePredecessor{predecessor("relation", "occurrence", "foreign-caller", 1)}
	_, err := Build(r)
	var failure *Failure
	if !errors.As(err, &failure) || failure.Stage != StageSelect || failure.Code != CodeInvalidRequest || bytes.Contains([]byte(err.Error()), r.Snapshots[0].Raw) {
		t.Fatalf("ASSERT_CONSUMER_V2_PREDECESSOR_TYPED_SAFE_REJECTION: %v", err)
	}
}

func TestConsumerAlternativeIdentityMutationChangesPacketID(t *testing.T) {
	r := validRequest(t)
	out, err := Build(r)
	if err != nil {
		t.Fatal(err)
	}
	p := out.Packets[0]
	base := p.PacketID
	p.ConsumerResolution = ConsumerResolved
	p.ConsumerAlternatives = []ConsumerAlternative{{RelationID: "r"}}
	id, err := packetIdentity(p)
	if err != nil || id == base {
		t.Fatalf("ASSERT_CONSUMER_IDENTITY_COVERS_ALTERNATIVES: base=%s changed=%s err=%v", base, id, err)
	}
}

func TestConsumerAuthorityCompletenessCeilingsRemain(t *testing.T) {
	r := validRequest(t)
	out, err := Build(r)
	if err != nil {
		t.Fatal(err)
	}
	p := out.Packets[0]
	if p.Authority != 0 || p.Accepted || p.Completeness != "UNKNOWN" {
		t.Fatalf("ASSERT_CONSUMER_AUTHORITY_COMPLETENESS_CEILINGS: %d/%v/%s", p.Authority, p.Accepted, p.Completeness)
	}
}
