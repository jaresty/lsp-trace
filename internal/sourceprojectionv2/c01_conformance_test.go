package sourceprojectionv2

import (
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/sourceprojection"
)

type c01Custody struct {
	Mode string `json:"mode"`
	ID   string `json:"id"`
}

func TestC01CanonicalBytesAndCompositeIdentityAcrossCustody(t *testing.T) {
	const uri = "file:///fixture/c01.go"
	raw := []byte("aaaa\nbbbb\ncccc\ndddd\n")
	r := func(line uint32) sourceprojection.Range {
		return sourceprojection.Range{Start: sourceprojection.Position{Line: line}, End: sourceprojection.Position{Line: line, Character: 4}}
	}
	candidates := []sourceprojection.Candidate{
		{UnitID: "unit-endpoint", CitationID: "citation-endpoint", Role: "ENDPOINT", GraphSubjectID: "node", LogicalSourceID: uri, Range: r(0), EvidenceRange: r(0), ItemRange: r(0), SelectionRange: r(0), PositionEncoding: "utf-8", PrivacyClassification: "PUBLIC"},
		{UnitID: "unit-relation", CitationID: "citation-relation", Role: "RELATION", GraphSubjectID: "relation", OccurrenceID: "occurrence", LogicalSourceID: uri, Range: r(1), EvidenceRange: r(1), PositionEncoding: "utf-8", RelationProvenance: "SERVER_REPORTED", PrivacyClassification: "PUBLIC"},
		{UnitID: "unit-omitted-a", CitationID: "citation-omitted-a", Role: "ENDPOINT", GraphSubjectID: "omitted-a", LogicalSourceID: uri, Range: r(2), PositionEncoding: "utf-8", PrivacyClassification: "RESTRICTED", Withheld: true},
		{UnitID: "unit-omitted-b", CitationID: "citation-omitted-b", Role: "ENDPOINT", GraphSubjectID: "omitted-b", LogicalSourceID: uri, Range: r(3), PositionEncoding: "utf-8", PrivacyClassification: "RESTRICTED", Withheld: true},
	}
	source := sourceprojection.Source{LogicalSourceID: uri, Digest: "sha256:source-a", ByteLength: len(raw), Bytes: raw, Available: true}
	projection, err := sourceprojection.Project(candidates, map[string]sourceprojection.Source{uri: source}, sourceprojection.Policy{PolicyID: "policy-a", BodyRequested: true})
	if err != nil || len(projection.Units) != 2 || len(projection.Citations) != 2 || len(projection.EmittedSpans) != 2 || len(projection.Omissions) != 2 {
		t.Fatalf("ASSERT_C01_ALL_PROJECTED_FORMS_SETUP: projection=%+v err=%v", projection, err)
	}
	base := Input{
		TargetURI: uri, SelectedURIs: []string{uri},
		Documents:  []DocumentSource{{URI: uri, DocumentVersion: 7, PositionEncoding: "utf-8", SourceDigest: source.Digest, SourceByteLength: len(raw)}},
		Candidates: candidates, Projection: projection, DocumentsObserved: 1, TotalAcquiredBytes: len(raw), RequestPolicyID: "policy-a",
	}
	permuted := base
	permuted.Candidates = reverseCopy(base.Candidates)
	permuted.Projection.Units = reverseCopy(base.Projection.Units)
	permuted.Projection.Citations = reverseCopy(base.Projection.Citations)
	permuted.Projection.EmittedSpans = reverseCopy(base.Projection.EmittedSpans)
	permuted.Projection.Omissions = reverseCopy(base.Projection.Omissions)

	var live, retained WireResult[c01Custody]
	for _, tc := range []struct {
		name    string
		mode    string
		custody c01Custody
	}{
		{name: "live", mode: "LIVE", custody: c01Custody{Mode: "LIVE", ID: "session:7"}},
		{name: "retained", mode: "RETAINED", custody: c01Custody{Mode: "RETAINED", ID: "artifact:7"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			one, err := AssembleBounded(base, tc.mode, tc.custody, 1<<20)
			if err != nil {
				t.Fatalf("ASSERT_C01_%s_BASELINE: %v", tc.mode, err)
			}
			two, err := AssembleBounded(permuted, tc.mode, tc.custody, 1<<20)
			if err != nil {
				t.Fatalf("ASSERT_C01_%s_PERMUTED: %v", tc.mode, err)
			}
			oneRaw, _ := json.Marshal(one)
			twoRaw, _ := json.Marshal(two)
			if string(oneRaw) != string(twoRaw) {
				t.Fatalf("ASSERT_C01_%s_ALL_FORMS_BYTE_CANONICAL:\none=%s\ntwo=%s", tc.mode, oneRaw, twoRaw)
			}
			if tc.mode == "LIVE" {
				live = one
			} else {
				retained = one
			}
		})
	}
	liveRaw, _ := json.Marshal(live)
	retainedRaw, _ := json.Marshal(retained)
	if string(liveRaw) == string(retainedRaw) || live.CustodyMode == retained.CustodyMode || reflect.DeepEqual(live.CustodyBinding, retained.CustodyBinding) {
		t.Fatalf("ASSERT_C01_CUSTODY_MODE_IDENTITY_SENSITIVE: live=%s retained=%s", liveRaw, retainedRaw)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*Input)
		check  func(WireResult[c01Custody], WireResult[c01Custody]) bool
	}{
		{name: "source-digest", mutate: func(in *Input) {
			changed := source
			changed.Digest = "sha256:source-b"
			in.Documents[0].SourceDigest = changed.Digest
			in.Projection, _ = sourceprojection.Project(candidates, map[string]sourceprojection.Source{uri: changed}, sourceprojection.Policy{PolicyID: "policy-a", BodyRequested: true})
		}, check: physicalIdentityDiffers},
		{name: "source-length", mutate: func(in *Input) {
			changed := source
			changed.ByteLength++
			in.Documents[0].SourceByteLength = changed.ByteLength
			in.Projection, _ = sourceprojection.Project(candidates, map[string]sourceprojection.Source{uri: changed}, sourceprojection.Policy{PolicyID: "policy-a", BodyRequested: true})
		}, check: physicalIdentityDiffers},
		{name: "policy-identity", mutate: func(in *Input) { in.RequestPolicyID = "policy-b" }, check: wireBytesDiffer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := base
			mutated.Documents = append([]DocumentSource(nil), base.Documents...)
			tc.mutate(&mutated)
			got, err := AssembleBounded(mutated, "LIVE", c01Custody{Mode: "LIVE", ID: "session:7"}, 1<<20)
			if err != nil || !tc.check(live, got) {
				t.Fatalf("ASSERT_C01_%s_IDENTITY_SENSITIVE: base=%+v mutated=%+v err=%v", tc.name, live, got, err)
			}
		})
	}

	unavailableSource := source
	unavailableSource.Available = false
	unavailableProjection, err := sourceprojection.Project(candidates, map[string]sourceprojection.Source{uri: unavailableSource}, sourceprojection.Policy{PolicyID: "policy-a", BodyRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	unavailableInput := base
	unavailableInput.Projection = unavailableProjection
	unavailable, err := AssembleBounded(unavailableInput, "LIVE", c01Custody{Mode: "LIVE", ID: "session:7"}, 1<<20)
	if err != nil || !wireBytesDiffer(live, unavailable) || unavailable.Status != "SOURCE_UNAVAILABLE" {
		t.Fatalf("ASSERT_C01_AVAILABILITY_IDENTITY_SENSITIVE: live=%+v unavailable=%+v err=%v", live, unavailable, err)
	}
}

func reverseCopy[T any](input []T) []T {
	out := append([]T(nil), input...)
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out
}

func physicalIdentityDiffers(left, right WireResult[c01Custody]) bool {
	return left.PhysicalProjectionID != right.PhysicalProjectionID && wireBytesDiffer(left, right)
}

func wireBytesDiffer(left, right WireResult[c01Custody]) bool {
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)
	return string(leftRaw) != string(rightRaw)
}
