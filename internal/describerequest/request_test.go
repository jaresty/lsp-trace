package describerequest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/retainedprojectiontestfixture"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/targetpacket"
)

type fixtureLookup struct{ body []byte }

func (l fixtureLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), l.body...)}, nil
}

func packetResult(t *testing.T) targetpacket.Result {
	t.Helper()
	fx := retainedprojectiontestfixture.ValidV2Artifact(t)
	n := censusprogramc.Representative{
		Status: censusprogramc.CandidateStatus, Authority: 0, SourceGraphComplete: "UNKNOWN",
		CensusID: "census", ConstituentIdentity: "constituent", ConstituentOrdinal: 0,
		SelectionState: "SELECTED", SelectedNode: fx.NodeID, ClaimCeiling: "STRUCTURAL",
		BatchID: "batch", CommunityIdentity: "community", ExecutionBundleID: "bundle",
		SeedLabel: "seed", SeedAt: "at", Members: []string{"member"}, SCCMembers: []string{"member"},
	}
	c := censusprogramc.Result{CensusID: "census", Admission: programcadmission.Result{Artifact: programcadmission.Artifact{Constituents: []programcadmission.ConstituentReference{{Identity: "constituent", GraphSHA256: fx.GraphV5Digest, GraphByteLength: int(fx.GraphV5ByteLen)}}}}, Representatives: censusprogramc.RepresentativeSelection{State: "SELECTED", Nominations: []censusprogramc.Representative{n}}}
	got, err := targetpacket.Build(targetpacket.Request{
		Census: c, Snapshots: []targetpacket.Snapshot{{ConstituentIdentity: "constituent", Raw: fx.Raw}},
		Lookup: fixtureLookup{fx.Content}, Policy: sourceprojection.Policy{PolicyID: "p", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 10, MaxObjects: 10, MaxWork: 100, EnforceLimits: true},
		ResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 1, MaxUniqueSourceBytes: uint64(len(fx.Content)), MaxLogicalSelections: 1}, MaxResponseBytes: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestParsePreservesCanonicalEmptyArray(t *testing.T) {
	records, err := Parse([]byte(`[]`))
	if err != nil {
		t.Fatalf("ASSERT_CANONICAL_EMPTY_REQUEST_ARRAY_PARSE: %v", err)
	}
	if records == nil || len(records) != 0 {
		t.Fatalf("ASSERT_CANONICAL_EMPTY_REQUEST_ARRAY_SEMANTICS: records=%#v", records)
	}
	raw, err := Bytes(records)
	if err != nil || string(raw) != "[]" {
		t.Fatalf("ASSERT_CANONICAL_EMPTY_REQUEST_ARRAY_ROUND_TRIP: raw=%q err=%v", raw, err)
	}
}

func TestBuildUnresolvedCanonicalReplay(t *testing.T) {
	records, err := Build(packetResult(t), 90000)
	if err != nil || len(records) != 1 {
		t.Fatalf("ASSERT_ZERO_ONE_UNRESOLVED: records=%d err=%v", len(records), err)
	}
	r := records[0]
	if r.Envelope.Protocol != Protocol || r.Envelope.MessageType != MessageType || r.Lineage.ConsumerResolution != string(targetpacket.ConsumerUnresolved) || r.Lineage.Authority != 0 || r.Lineage.Accepted || r.Lineage.Completeness != "UNKNOWN" {
		t.Fatalf("ASSERT_AUTHORITY_ENVELOPE: %+v", r)
	}
	if !strings.Contains(r.Envelope.Prompt, "nearest_outward_consumer=OUTWARD_CONSUMER_UNRESOLVED") || !strings.Contains(r.Envelope.Prompt, "do not invent") {
		t.Fatalf("ASSERT_UNRESOLVED_NO_INVENTION: %q", r.Envelope.Prompt)
	}
	raw, err := Bytes(records)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(raw)
	if err != nil || len(parsed) != 1 || parsed[0].RecordID != r.RecordID {
		t.Fatalf("ASSERT_CANONICAL_REPLAY: parsed=%+v err=%v", parsed, err)
	}
}

func TestStrictNDJSONRoundTripAndRejection(t *testing.T) {
	records, err := Build(packetResult(t), 90000)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := NDJSON(records)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseNDJSON(raw, records)
	if err != nil || len(parsed) != len(records) {
		t.Fatalf("ASSERT_NDJSON_CANONICAL_ROUNDTRIP: records=%d err=%v", len(parsed), err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatal("ASSERT_NDJSON_TERMINAL_LF")
	}
	for name, candidate := range map[string][]byte{
		"bom":       append([]byte{0xef, 0xbb, 0xbf}, raw...),
		"blank":     append([]byte{'\n'}, raw...),
		"trailing":  append(append([]byte(nil), raw...), []byte("{}\n")...),
		"malformed": []byte("{\n"),
		"no lf":     append([]byte(nil), raw[:len(raw)-1]...),
		"unknown":   bytes.Replace(raw, []byte(`{"protocol":`), []byte(`{"unknown":0,"protocol":`), 1),
		"duplicate": bytes.Replace(raw, []byte(`{"protocol":`), []byte(`{"protocol":"bad","protocol":`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseNDJSON(candidate, records); err == nil {
				t.Fatal("ASSERT_NDJSON_STRICT_REJECTION")
			}
		})
	}
	wrong := append([]Record(nil), records...)
	wrong[0].Envelope.Prompt += " substituted"
	if _, err := ParseNDJSON(raw, wrong); err == nil {
		t.Fatal("ASSERT_NDJSON_EXPECTED_ORDER_AND_IDENTITY")
	}
}

func TestStrictParseAndCoordinatedTamper(t *testing.T) {
	records, err := Build(packetResult(t), 90000)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Bytes(records)
	if err != nil {
		t.Fatal(err)
	}
	cases := [][]byte{
		append(append([]byte(nil), raw...), []byte("{}")...),
		bytes.Replace(raw, []byte(`"record_id":`), []byte(`"unknown":0,"record_id":`), 1),
		bytes.Replace(raw, []byte(`"record_id":`), []byte(`"record_id":"duplicate","record_id":`), 1),
	}
	for i, candidate := range cases {
		if _, err := Parse(candidate); err == nil {
			t.Fatalf("ASSERT_STRICT_REJECTION_%d", i)
		}
	}
	mutated := records[0]
	mutated.Lineage.CensusID = "substituted"
	mutated.RecordID = recordID(mutated)
	coordinated, _ := json.Marshal([]Record{mutated})
	if _, err := Parse(coordinated); err == nil {
		t.Fatal("ASSERT_COORDINATED_LINEAGE_SUBSTITUTION_REJECTED")
	}

	mutated = records[0]
	mutated.Envelope.Prompt += "\nsubstituted semantic claim"
	sum := sha256.Sum256([]byte(mutated.Envelope.Prompt))
	mutated.Envelope.InputSHA256 = hex.EncodeToString(sum[:])
	mutated.Envelope.InputBytes = len(mutated.Envelope.Prompt)
	mutated.Envelope.MessageID = envelopeID("message", mutated)
	mutated.Envelope.CorrelationID = envelopeID("correlation", mutated)
	mutated.RecordID = recordID(mutated)
	coordinated, _ = json.Marshal([]Record{mutated})
	if _, err := Parse(coordinated); err == nil {
		t.Fatal("ASSERT_COORDINATED_PROMPT_SUBSTITUTION_REJECTED")
	}
}
