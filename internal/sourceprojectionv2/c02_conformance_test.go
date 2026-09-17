package sourceprojectionv2

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"

	"lsp-trace/internal/sourceprojection"
)

func TestC02IntegratedEncodingRangeCitationCorpusAcrossCustody(t *testing.T) {
	const assertion = "ASSERT_C02_INTEGRATED_BOM_NEWLINE_NONBMP_RANGE_CITATIONS_CROSS_CUSTODY"
	raw := []byte("\xef\xbb\xbfA😀\r\nxy\n")
	sum := sha256.Sum256(raw)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	uri := "file:///fixture/c02.go"

	for _, tc := range []struct {
		encoding          string
		bom, afterBOMAndA uint32
	}{
		{encoding: "utf-8", bom: 3, afterBOMAndA: 4},
		{encoding: "utf-16", bom: 1, afterBOMAndA: 2},
		{encoding: "utf-32", bom: 1, afterBOMAndA: 2},
	} {
		t.Run(tc.encoding, func(t *testing.T) {
			full := testRange(0, 0, 2, 0)
			empty := testRange(0, tc.bom, 0, tc.bom)
			crossLine := testRange(0, tc.afterBOMAndA, 1, 2)
			candidates := []sourceprojection.Candidate{
				{
					UnitID: "endpoint", CitationID: "citation-endpoint", Role: "ENDPOINT", GraphSubjectID: "node-target",
					LogicalSourceID: uri, Range: full, EvidenceRange: empty, ItemRange: full, SelectionRange: empty,
					DisplayProvenance: "SERVER_REPORTED_DOCUMENT_SYMBOL", PositionEncoding: tc.encoding, PrivacyClassification: "PUBLIC",
				},
				{
					UnitID: "relation", CitationID: "citation-relation", Role: "RELATION", GraphSubjectID: "call-target",
					OccurrenceID: "occurrence-target", LogicalSourceID: uri, Range: crossLine, EvidenceRange: crossLine,
					DisplayProvenance: "CALL_SITE_OCCURRENCE", PositionEncoding: tc.encoding, RelationProvenance: "SERVER_REPORTED", PrivacyClassification: "PUBLIC",
				},
			}
			sources := map[string]sourceprojection.Source{uri: {LogicalSourceID: uri, Digest: digest, ByteLength: len(raw), Bytes: raw, Available: true}}
			projection, err := sourceprojection.Project(candidates, sources, sourceprojection.Policy{PolicyID: "policy", BodyRequested: true, MaxBytes: len(raw), MaxRanges: 2, MaxObjects: 2, MaxWork: 2, EnforceLimits: true})
			if err != nil {
				t.Fatalf("%s_PROJECT[%s]: %v", assertion, tc.encoding, err)
			}
			if projection.Status != "COMPLETE" || len(projection.Units) != 2 || len(projection.Citations) != 2 || len(projection.EmittedSpans) != 1 {
				t.Fatalf("%s_CARDINALITY[%s]: %+v", assertion, tc.encoding, projection)
			}
			if projection.Units[0].Body != string(raw) || projection.Units[1].Body != "😀\r\nxy" || projection.Units[0].Range != full || projection.Units[1].Range != crossLine {
				t.Fatalf("%s_EXACT_BODIES_RANGES[%s]: %+v", assertion, tc.encoding, projection.Units)
			}
			if projection.Citations[0] != (sourceprojection.Citation{CitationID: "citation-endpoint", UnitID: "endpoint", Role: "ENDPOINT", SubjectID: "node-target"}) || projection.Citations[1] != (sourceprojection.Citation{CitationID: "citation-relation", UnitID: "relation", Role: "RELATION", SubjectID: "call-target", OccurrenceID: "occurrence-target"}) {
				t.Fatalf("%s_CITATION_ATTRIBUTION[%s]: %+v", assertion, tc.encoding, projection.Citations)
			}
			if projection.EmittedSpans[0].Range != full || !reflect.DeepEqual(projection.EmittedSpans[0].UnitIDs, []string{"endpoint", "relation"}) || projection.EmittedSpans[0].Body != string(raw) {
				t.Fatalf("%s_OVERLAP_SPAN[%s]: %+v", assertion, tc.encoding, projection.EmittedSpans)
			}

			input := Input{
				TargetURI: uri, SelectedURIs: []string{uri},
				Documents:  []DocumentSource{{URI: uri, DocumentVersion: 7, PositionEncoding: tc.encoding, SourceDigest: digest, SourceByteLength: len(raw)}},
				Candidates: candidates, Projection: projection, DocumentsObserved: 1, TotalAcquiredBytes: len(raw), RequestPolicyID: "policy",
			}
			live, err := AssembleBounded(input, "LIVE", map[string]any{"session_id": "s", "generation": 1}, 1<<20)
			if err != nil {
				t.Fatalf("%s_LIVE[%s]: %v", assertion, tc.encoding, err)
			}
			retained, err := AssembleBounded(input, "RETAINED", map[string]any{"artifact_digest": "sha256:artifact"}, 1<<20)
			if err != nil {
				t.Fatalf("%s_RETAINED[%s]: %v", assertion, tc.encoding, err)
			}
			if !reflect.DeepEqual(live.Units, retained.Units) || !reflect.DeepEqual(live.Citations, retained.Citations) || !reflect.DeepEqual(live.EmittedSpans, retained.EmittedSpans) || live.Accounting != retained.Accounting {
				t.Fatalf("%s_CROSS_CUSTODY_LOGICAL_PARITY[%s]: live=%+v retained=%+v", assertion, tc.encoding, live, retained)
			}
			if live.Citations[0].EvidenceRange != empty || live.Citations[0].DisplayRange != full || live.Citations[1].EvidenceRange != crossLine || live.Citations[1].DisplayRange != crossLine {
				t.Fatalf("%s_WIRE_CITATION_RANGES[%s]: %+v", assertion, tc.encoding, live.Citations)
			}
			if live.DocumentBindings[0].PositionEncoding != tc.encoding || retained.DocumentBindings[0].PositionEncoding != tc.encoding {
				t.Fatalf("%s_ENCODING_BINDING[%s]: live=%+v retained=%+v", assertion, tc.encoding, live.DocumentBindings, retained.DocumentBindings)
			}
		})
	}
}
