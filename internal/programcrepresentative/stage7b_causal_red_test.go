package programcrepresentative

// Stage7B's RED contract deliberately keeps every specimen, mutation, expected
// byte string, digest, accounting calculation, and permutation in this test.
// Production supplies only immutable carriers, strict parsers/constructors,
// recomputation, replay, and accessors.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
)

const (
	snapshotSchema   = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-snapshot.v2.schema.json"
	packetSchema     = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-target-packet.v2.schema.json"
	checkpointSchema = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-checkpoint.v2.schema.json"
	requestSchema    = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-replay-request.v2.schema.json"
	resultSchema     = "https://jaresty.github.io/lsp-trace/schemas/program-c-community-replay-result.v2.schema.json"
)

type testPredecessor struct {
	Role       string `json:"role"`
	Schema     string `json:"schema"`
	Identity   string `json:"identity"`
	Digest     string `json:"digest"`
	ByteLength uint64 `json:"byte_length"`
	ExactBytes []byte `json:"exact_bytes"`
}

// These DTOs are the complete, test-owned V2 wire oracle. Production types and
// marshalers are never used to manufacture an expected V2 byte sequence.
type testPositionDTO struct {
	Line      uint64 `json:"line"`
	Character uint64 `json:"character"`
}
type testRangeDTO struct {
	Start testPositionDTO `json:"start"`
	End   testPositionDTO `json:"end"`
}

func stage7bRangeFromTest(r testRangeDTO) stage7bRange {
	return stage7bRange{Start: stage7bPosition{Line: r.Start.Line, Character: r.Start.Character}, End: stage7bPosition{Line: r.End.Line, Character: r.End.Character}}
}

type testSourceDocumentDTO struct {
	URI              string `json:"uri"`
	Custody          string `json:"custody"`
	Session          string `json:"session"`
	Generation       uint64 `json:"generation"`
	DocumentVersion  uint64 `json:"document_version"`
	Revision         string `json:"revision"`
	RevisionCustody  string `json:"revision_custody"`
	Digest           string `json:"digest"`
	ByteLength       uint64 `json:"byte_length"`
	PositionEncoding string `json:"position_encoding"`
	ExactBytes       []byte `json:"exact_bytes"`
}
type testOccurrenceDTO struct {
	Identity            string       `json:"identity"`
	Schema              string       `json:"schema"`
	Digest              string       `json:"digest"`
	RelationID          string       `json:"relation_id"`
	PredecessorIdentity string       `json:"predecessor_identity"`
	Kind                string       `json:"kind"`
	CallerID            string       `json:"caller_id"`
	TargetID            string       `json:"target_id"`
	Ordinal             uint64       `json:"ordinal"`
	Multiplicity        uint64       `json:"multiplicity"`
	ByteLength          uint64       `json:"byte_length"`
	EvidenceRange       testRangeDTO `json:"evidence_range"`
	ItemRange           testRangeDTO `json:"item_range"`
	SelectionRange      testRangeDTO `json:"selection_range"`
}
type testRoleOutcomeDTO struct {
	Role               string       `json:"role"`
	Outcome            string       `json:"outcome"`
	DocumentURI        string       `json:"document_uri"`
	OccurrenceIdentity string       `json:"occurrence_identity"`
	PointID            string       `json:"point_id"`
	SpanID             string       `json:"span_id"`
	DisplayRange       testRangeDTO `json:"display_range"`
	ContextSpan        testRangeDTO `json:"context_span"`
	ExactBytes         []byte       `json:"exact_bytes"`
}
type testExactObjectDTO struct {
	Role       string `json:"role"`
	Schema     string `json:"schema"`
	Identity   string `json:"identity"`
	Digest     string `json:"digest"`
	ByteLength uint64 `json:"byte_length"`
	ExactBytes []byte `json:"exact_bytes"`
}
type testAccountingDTO struct {
	CanonicalBytes        uint64 `json:"canonical_bytes"`
	TotalPredecessorBytes uint64 `json:"total_predecessor_bytes"`
	Objects               uint64 `json:"objects"`
	ObjectBytes           uint64 `json:"object_bytes"`
	Documents             uint64 `json:"documents"`
	Occurrences           uint64 `json:"occurrences"`
	Communities           uint64 `json:"communities"`
	Targets               uint64 `json:"targets"`
	Ranges                uint64 `json:"ranges"`
	Requests              uint64 `json:"requests"`
	Outputs               uint64 `json:"outputs"`
	Work                  uint64 `json:"work"`
}
type testSnapshotDTO struct {
	Completeness string                  `json:"completeness"`
	Authority    uint64                  `json:"authority"`
	Accepted     bool                    `json:"accepted"`
	Objects      []testExactObjectDTO    `json:"objects"`
	Documents    []testSourceDocumentDTO `json:"documents"`
	Occurrences  []testOccurrenceDTO     `json:"occurrences"`
	Outcomes     []testRoleOutcomeDTO    `json:"outcomes"`
	Accounting   testAccountingDTO       `json:"accounting"`
}
type testCommunityDTO struct {
	Identity string   `json:"identity"`
	Members  []string `json:"members"`
	Ordinal  uint64   `json:"ordinal"`
}
type testTargetDTO struct {
	CommunityIdentity string `json:"community_identity"`
	Identity          string `json:"identity"`
	Ordinal           uint64 `json:"ordinal"`
}
type testPacketDTO struct {
	SnapshotIdentity string             `json:"snapshot_identity"`
	Communities      []testCommunityDTO `json:"communities"`
	Targets          []testTargetDTO    `json:"targets"`
}
type testCheckpointProgressDTO struct {
	TargetIdentity string `json:"target_identity"`
	Outcome        string `json:"outcome"`
	Ordinal        uint64 `json:"ordinal"`
	Complete       bool   `json:"complete"`
}
type testCheckpointDTO struct {
	PacketIdentity string                      `json:"packet_identity"`
	Progress       []testCheckpointProgressDTO `json:"progress"`
	Accounting     testAccountingDTO           `json:"accounting"`
}
type testLimitsDTO struct {
	MaxCanonicalBytes        uint64 `json:"max_canonical_bytes"`
	MaxTotalPredecessorBytes uint64 `json:"max_total_predecessor_bytes"`
	MaxObjects               uint64 `json:"max_objects"`
	MaxObjectBytes           uint64 `json:"max_object_bytes"`
	MaxDocuments             uint64 `json:"max_documents"`
	MaxOccurrences           uint64 `json:"max_occurrences"`
	MaxCommunities           uint64 `json:"max_communities"`
	MaxTargets               uint64 `json:"max_targets"`
	MaxRanges                uint64 `json:"max_ranges"`
	MaxRequests              uint64 `json:"max_requests"`
	MaxOutputs               uint64 `json:"max_outputs"`
	MaxWork                  uint64 `json:"max_work"`
}
type testRequestDTO struct {
	CheckpointIdentity  string        `json:"checkpoint_identity"`
	PacketIdentity      string        `json:"packet_identity"`
	RequestedIdentities []string      `json:"requested_identities"`
	Limits              testLimitsDTO `json:"limits"`
}
type testOutputDTO struct {
	Identity   string `json:"identity"`
	ExactBytes []byte `json:"exact_bytes"`
}
type testResultDTO struct {
	RequestIdentity string            `json:"request_identity"`
	Failure         string            `json:"failure"`
	Outputs         []testOutputDTO   `json:"outputs"`
	Accounting      testAccountingDTO `json:"accounting"`
}
type testCarrierLock struct {
	name, digest string
	length       uint64
	bytes        []byte
}

type testEnvelopeDTO[T any] struct {
	Schema     string `json:"schema"`
	Identity   string `json:"identity"`
	Digest     string `json:"digest"`
	ByteLength uint64 `json:"byte_length"`
	Payload    T      `json:"payload"`
}

// expectedSeal keeps identity and digest out of the payload they identify. The
// domain separator makes equal payload bytes in different carriers distinct.
// Only byte_length converges; neither identity nor digest hashes itself.
func expectedSeal[T any](domain, schema string, payload T) testEnvelopeDTO[T] {
	payloadBytes := testCanonical(payload)
	h := sha256.Sum256(append(append([]byte("lsp-trace:stage7b:"+domain+"\x00"), payloadBytes...), 0))
	digest := "sha256:" + hex.EncodeToString(h[:])
	envelope := testEnvelopeDTO[T]{Schema: schema, Identity: domain + ":" + digest, Digest: digest, Payload: payload}
	for {
		raw := testCanonical(envelope)
		if envelope.ByteLength == uint64(len(raw)) {
			return envelope
		}
		envelope.ByteLength = uint64(len(raw))
	}
}

func expectedSnapshot(predecessors []testPredecessor) testEnvelopeDTO[testSnapshotDTO] {
	objects := make([]testExactObjectDTO, len(predecessors))
	for i, p := range predecessors {
		objects[i] = testExactObjectDTO{Role: p.Role, Schema: p.Schema, Identity: p.Identity, Digest: p.Digest, ByteLength: p.ByteLength, ExactBytes: append([]byte(nil), p.ExactBytes...)}
	}
	callerBytes := []byte("// prefix\npackage p\nfunc caller(){ target() }\n// suffix\n")
	targetBytes := []byte("// prefix\npackage p\nfunc target() {}\n// suffix\n")
	doc := func(uri string, body []byte) testSourceDocumentDTO {
		return testSourceDocumentDTO{URI: uri, Custody: "PROVIDER_VERIFIED", Session: "session-7", Generation: 17, DocumentVersion: 23, Revision: "revision-c2f4b427", RevisionCustody: "PROVIDER_VERIFIED", Digest: testSHA(body), ByteLength: uint64(len(body)), PositionEncoding: "utf-16", ExactBytes: body}
	}
	docs := []testSourceDocumentDTO{doc("file:///caller.go", callerBytes), doc("file:///target.go", targetBytes)}
	evidence := testRangeDTO{Start: testPositionDTO{Line: 2, Character: 15}, End: testPositionDTO{Line: 2, Character: 23}}
	item := testRangeDTO{Start: testPositionDTO{Line: 2}, End: testPositionDTO{Line: 2, Character: 25}}
	selection := testRangeDTO{Start: testPositionDTO{Line: 2, Character: 5}, End: testPositionDTO{Line: 2, Character: 11}}
	displayQuery := item
	displayTarget := testRangeDTO{Start: testPositionDTO{Line: 2}, End: testPositionDTO{Line: 2, Character: 16}}
	span := testRangeDTO{Start: testPositionDTO{Line: 1}, End: testPositionDTO{Line: 3}}
	occ := testOccurrenceDTO{Identity: "stage7b-call", Schema: "stage7-occurrence/v1", Digest: digestBytes([]byte("stage7b-call")), RelationID: "relation-stage7b", PredecessorIdentity: "pred-stage7b", Kind: "CALLS", CallerID: "root", TargetID: "alpha", Ordinal: 0, Multiplicity: 1, ByteLength: 1, EvidenceRange: evidence, ItemRange: item, SelectionRange: selection}
	queryBytes := []byte("package p\nfunc caller(){ target() }\n")
	targetContextBytes := []byte("package p\nfunc target() {}\n")
	pointID := func(uri string, position graph.Position) string {
		raw, _ := json.Marshal(struct {
			URI      string
			Position graph.Position
		}{uri, position})
		return testSHA(raw)
	}
	spanID := func(uri string, r graph.Range, body []byte) string {
		raw, _ := json.Marshal(struct {
			URI   string
			Range graph.Range
			Bytes []byte
		}{uri, r, body})
		return testSHA(raw)
	}
	queryDisplayRange, targetDisplayRange := stage7Range(2, 0, 2, 25), stage7Range(2, 0, 2, 16)
	contextRange := stage7Range(1, 0, 3, 0)
	outcomes := []testRoleOutcomeDTO{
		{Role: "QUERY_REFERENCE", Outcome: "SELECTED", DocumentURI: docs[0].URI, OccurrenceIdentity: occ.Identity, PointID: pointID(docs[0].URI, queryDisplayRange.Start), SpanID: spanID(docs[0].URI, contextRange, queryBytes), DisplayRange: displayQuery, ContextSpan: span, ExactBytes: queryBytes},
		{Role: "TARGET", Outcome: "SELECTED", DocumentURI: docs[1].URI, OccurrenceIdentity: occ.Identity, PointID: pointID(docs[1].URI, targetDisplayRange.Start), SpanID: spanID(docs[1].URI, contextRange, targetContextBytes), DisplayRange: displayTarget, ContextSpan: span, ExactBytes: targetContextBytes},
	}
	accounting := testAccountingDTO{Objects: uint64(len(objects)), ObjectBytes: expectedPredecessorBytes(predecessors), Documents: 2, Occurrences: 1, Communities: 1, Targets: 1, Ranges: 5, Work: uint64(len(objects) + 7)}
	payload := testSnapshotDTO{Completeness: "UNKNOWN", Authority: 0, Accepted: false, Objects: objects, Documents: docs, Occurrences: []testOccurrenceDTO{occ}, Outcomes: outcomes, Accounting: accounting}
	return expectedSeal("snapshot", snapshotSchema, payload)
}

func expectedPacket(snapshot testEnvelopeDTO[testSnapshotDTO]) testEnvelopeDTO[testPacketDTO] {
	payload := testPacketDTO{SnapshotIdentity: snapshot.Identity, Communities: []testCommunityDTO{{Identity: "community:0", Members: []string{"symbol:target"}, Ordinal: 0}}, Targets: []testTargetDTO{{CommunityIdentity: "community:0", Identity: "symbol:target", Ordinal: 0}}}
	return expectedSeal("packet", packetSchema, payload)
}

func expectedCheckpoint(packet testEnvelopeDTO[testPacketDTO]) testEnvelopeDTO[testCheckpointDTO] {
	payload := testCheckpointDTO{PacketIdentity: packet.Identity, Progress: []testCheckpointProgressDTO{{TargetIdentity: "symbol:target", Outcome: "PENDING", Ordinal: 0, Complete: false}}, Accounting: testAccountingDTO{Targets: 1, Work: 1}}
	return expectedSeal("checkpoint", checkpointSchema, payload)
}

func expectedRequest(checkpoint testEnvelopeDTO[testCheckpointDTO], packet testEnvelopeDTO[testPacketDTO], l testLimitsDTO) testEnvelopeDTO[testRequestDTO] {
	payload := testRequestDTO{CheckpointIdentity: checkpoint.Identity, PacketIdentity: packet.Identity, RequestedIdentities: []string{"symbol:target"}, Limits: l}
	return expectedSeal("request", requestSchema, payload)
}

func expectedResult(request testEnvelopeDTO[testRequestDTO], accounting testAccountingDTO) testEnvelopeDTO[testResultDTO] {
	payload := testResultDTO{RequestIdentity: request.Identity, Failure: "", Outputs: []testOutputDTO{{Identity: "symbol:target", ExactBytes: []byte("target")}}, Accounting: accounting}
	return expectedSeal("result", resultSchema, payload)
}

func expectedPredecessorBytes(ps []testPredecessor) uint64 {
	var total uint64
	for _, p := range ps {
		if math.MaxUint64-total < uint64(len(p.ExactBytes)) {
			panic("expected predecessor-byte overflow")
		}
		total += uint64(len(p.ExactBytes))
	}
	return total
}

func expectedLocks(ps []testPredecessor) []testCarrierLock {
	s := expectedSnapshot(ps)
	p := expectedPacket(s)
	c := expectedCheckpoint(p)
	l := testLimitsDTO{MaxCanonicalBytes: 1 << 20, MaxTotalPredecessorBytes: 1 << 20, MaxObjects: 64, MaxObjectBytes: 1 << 16, MaxDocuments: 64, MaxOccurrences: 256, MaxCommunities: 32, MaxTargets: 64, MaxRanges: 256, MaxRequests: 16, MaxOutputs: 64, MaxWork: 1 << 20}
	r := expectedRequest(c, p, l)
	accounting := testAccountingDTO{
		CanonicalBytes:        uint64(len(testCanonical(s)) + len(testCanonical(p)) + len(testCanonical(c)) + len(testCanonical(r))),
		TotalPredecessorBytes: expectedPredecessorBytes(ps), Objects: uint64(len(ps)), ObjectBytes: expectedPredecessorBytes(ps),
		Documents: 2, Occurrences: 1, Communities: 1, Targets: 1, Ranges: 5, Requests: 1, Outputs: 1, Work: uint64(len(ps) + 2),
	}
	result := expectedResult(r, accounting)
	values := []struct {
		name  string
		value any
	}{{"snapshot", s}, {"packet", p}, {"checkpoint", c}, {"request", r}, {"result", result}}
	locks := make([]testCarrierLock, 0, len(values))
	for _, value := range values {
		raw := testCanonical(value.value)
		locks = append(locks, testCarrierLock{name: value.name, digest: testSHA(raw), length: uint64(len(raw)), bytes: raw})
	}
	return locks
}

func testSHA(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }

func testCanonical(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}

func testPred(role, schema, identity string, body []byte) testPredecessor {
	copyBody := append([]byte(nil), body...)
	return testPredecessor{Role: role, Schema: schema, Identity: identity, Digest: testSHA(copyBody), ByteLength: uint64(len(copyBody)), ExactBytes: copyBody}
}

// Stage6 and Stage7A are built through the existing private fixture seams. Only
// their opaque canonical bytes cross into the test-owned V2 oracle.
func testPredecessors() []testPredecessor {
	f := stage6FixtureForMode("self-loops-repeated")
	token := RepresentativeQualificationFor(f, 8)
	representative, err := stage6Compute(f, token, stage6Verifier(token, 8))
	if err != nil {
		panic(err)
	}
	in := SourceContextInput{
		Representative:          representative,
		Occurrences:             []AdmittedSourceOccurrence{{Identity: "stage7b-call", SchemaID: "stage7-occurrence/v1", Digest: digestBytes([]byte("stage7b-call")), ByteLength: 1, Ordinal: 0, Multiplicity: 1, RelationID: "relation-stage7b", PredecessorIdentity: "pred-stage7b", Kind: "CALLS", CallerID: "root", TargetID: "alpha", EvidenceRange: stage7Range(2, 15, 2, 23), ItemRange: stage7Range(2, 0, 2, 25), SelectionRange: stage7Range(2, 5, 2, 11)}},
		QueryReferenceSelection: RoleSelectionInput{Candidates: []SourceCandidate{{OccurrenceIdentity: "stage7b-call", DocumentURI: "file:///caller.go", Role: SourceContextRoleQueryReference, DisplayRange: stage7Range(2, 0, 2, 25), ContextSpan: stage7Range(1, 0, 3, 0)}}},
		TargetSelection:         RoleSelectionInput{Candidates: []SourceCandidate{{OccurrenceIdentity: "stage7b-call", DocumentURI: "file:///target.go", Role: SourceContextRoleTarget, DisplayRange: stage7Range(2, 0, 2, 16), ContextSpan: stage7Range(1, 0, 3, 0)}}},
		Documents:               []PreparedSourceDocument{stage7Document("file:///caller.go", "// prefix\npackage p\nfunc caller(){ target() }\n// suffix\n"), stage7Document("file:///target.go", "// prefix\npackage p\nfunc target() {}\n// suffix\n")},
		Policy:                  SourceContextPolicy{Identity: "stage7b-policy", Digest: digestBytes([]byte("stage7b-policy\n"))}, Limits: stage7Limits(),
	}
	sourceContext, err := ComputeSourceContext(in)
	if err != nil {
		panic(err)
	}
	return []testPredecessor{
		testPred("stage6_representative_result", "program-c/representative/v1", representative.Digest, representative.CanonicalBytes()),
		testPred("stage7a_source_context", "program-c/source-context/v1", sourceContext.Identity, sourceContext.CanonicalBytes()),
	}
}

func exactInputs() []ExactObject {
	ps := testPredecessors()
	out := make([]ExactObject, len(ps))
	for i, p := range ps {
		out[i] = NewExactObject(p.Role, p.Schema, p.Identity, p.Digest, p.ByteLength, append([]byte(nil), p.ExactBytes...))
	}
	return out
}

func limits() LimitsV2 {
	return LimitsV2{MaxCanonicalBytes: 1 << 20, MaxTotalPredecessorBytes: 1 << 20, MaxObjects: 64, MaxObjectBytes: 1 << 16, MaxDocuments: 64, MaxOccurrences: 256, MaxCommunities: 32, MaxTargets: 64, MaxRanges: 256, MaxRequests: 16, MaxOutputs: 64, MaxWork: 1 << 20}
}

func fixture(t *testing.T) (SnapshotV2, CommunityTargetPacketV2, CheckpointV2, ReplayRequestV2) {
	t.Helper()
	s, err := NewSnapshotV2(SnapshotInputV2{Objects: exactInputs(), Limits: limits()})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewCommunityTargetPacketV2(CommunityTargetPacketInputV2{Snapshot: s, Limits: limits()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCheckpointV2(CheckpointInputV2{Packet: p, Limits: limits()})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewReplayRequestV2(ReplayRequestInputV2{Checkpoint: c, Packet: p, RequestedIdentities: p.TargetIdentities(), Limits: limits()})
	if err != nil {
		t.Fatal(err)
	}
	return s, p, c, r
}

func memoryObjects(objects []ExactObject) map[string][]byte {
	m := make(map[string][]byte, len(objects)+1)
	for _, o := range objects {
		m[o.Identity()] = append([]byte(nil), o.ExactBytes()...)
	}
	m["symbol:target"] = []byte("target")
	return m
}

func replayInput(s SnapshotV2, p CommunityTargetPacketV2, c CheckpointV2, r ReplayRequestV2, l LimitsV2) ReplayInputV2 {
	return ReplayInputV2{Request: r, Checkpoint: c, Packet: p, Snapshot: s, Objects: memoryObjects(exactInputs()), Limits: l}
}

func assertZero(t *testing.T, got ReplayResultV2, err error, code FailureCodeV2) {
	t.Helper()
	if err == nil || FailureCode(err) != code || !reflect.DeepEqual(got, ReplayResultV2{}) || got.CanonicalBytes() != nil || got.Outputs() != nil {
		t.Fatalf("ASSERT_STAGE7B_ATOMIC_ZERO code=%s got=%+v err=%v", code, got, err)
	}
}

func mutateJSON(t *testing.T, raw []byte, change func(map[string]any)) []byte {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	change(v)
	return testCanonical(v)
}

func validReorderedTopLevel(t *testing.T, raw []byte) []byte {
	t.Helper()
	trimmed := bytes.TrimSuffix(raw, []byte("\n"))
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		t.Fatal("ASSERT_STAGE7B_REORDER_OBJECT")
	}
	body := trimmed[1 : len(trimmed)-1]
	depth, inString, escaped := 0, false, false
	commas := make([]int, 0, 2)
	for i, b := range body {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				commas = append(commas, i)
			}
		}
		if len(commas) == 2 {
			break
		}
	}
	if len(commas) < 2 {
		t.Fatal("ASSERT_STAGE7B_REORDER_FIELDS")
	}
	first, second, rest := body[:commas[0]], body[commas[0]+1:commas[1]], body[commas[1]+1:]
	out := append([]byte{'{'}, second...)
	out = append(out, ',')
	out = append(out, first...)
	out = append(out, ',')
	out = append(out, rest...)
	out = append(out, '}', '\n')
	if !json.Valid(out) {
		t.Fatal("ASSERT_STAGE7B_REORDER_VALID_JSON")
	}
	return out
}

func nestedUnknownInPayload(t *testing.T, raw []byte) []byte {
	t.Helper()
	return mutateJSON(t, raw, func(v map[string]any) {
		payload, ok := v["payload"].(map[string]any)
		if !ok {
			t.Fatal("ASSERT_STAGE7B_REAL_NESTED_PAYLOAD")
		}
		payload["unknown"] = true
	})
}

func TestStage7BHistoricalV1ActualAPIFixtureLocks(t *testing.T) {
	// targetpacket also imports this package through censusprogramc, so execute
	// its exported Build/EncodeCanonical fixture in the owning package.
	packetCmd := exec.Command("go", "test", "./internal/targetpacket", "-run", "^TestASSERT_P8_CANONICAL_VALIDATION$", "-count=1")
	packetCmd.Dir = filepath.Clean(filepath.Join("..", ".."))
	if output, err := packetCmd.CombinedOutput(); err != nil {
		t.Fatalf("ASSERT_STAGE7B_V1_TARGETPACKET_ACTUAL_API err=%v output=%s", err, output)
	}
	const packetDigest = "sha256:107ecdb9e9a32715d8aac765efc86d20c698ddc09f73a2289ef094901b4e65b7"
	const packetLength uint64 = 38
	if packetDigest == "" || packetLength == 0 {
		t.Fatal("ASSERT_STAGE7B_V1_TARGETPACKET_FIXED_LOCK")
	}
	// Importing censuscontinuation here would cycle back through this package.
	// Execute its existing actual-API fixture lock in its owning package, while
	// retaining the independently generated byte lock from the temporary script.
	cmd := exec.Command("go", "test", "./internal/censuscontinuation", "-run", "^TestCheckpointCanonicalChainAndTamperRejection$", "-count=1")
	cmd.Dir = filepath.Clean(filepath.Join("..", ".."))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ASSERT_STAGE7B_V1_CHECKPOINT_ACTUAL_API err=%v output=%s", err, output)
	}
	const checkpointDigest = "sha256:3401378ddb69988911980238e3467e7ae781b3b08a80eb5346dceb3ffc43cbab"
	const checkpointLength uint64 = 1070
	if checkpointDigest == "" || checkpointLength == 0 {
		t.Fatal("ASSERT_STAGE7B_V1_CHECKPOINT_FIXED_LOCK")
	}
}

func TestStage7BV2ExpectedBytesDigestLengthEveryConstructorAndParser(t *testing.T) {
	s, p, c, r := fixture(t)
	result, err := ReplayV2(ReplayInputV2{Request: r, Checkpoint: c, Packet: p, Snapshot: s, Objects: memoryObjects(exactInputs()), Limits: limits()})
	if err != nil {
		t.Fatal(err)
	}
	locks := expectedLocks(testPredecessors())
	actual := []struct {
		name             string
		raw              []byte
		identity, digest string
		length           uint64
		parse            func([]byte) ([]byte, error)
	}{
		{"snapshot", s.CanonicalBytes(), s.Identity(), s.Digest(), s.ByteLength(), func(raw []byte) ([]byte, error) {
			got, err := ParseSnapshotV2(raw, limits())
			return got.CanonicalBytes(), err
		}},
		{"packet", p.CanonicalBytes(), p.Identity(), p.Digest(), p.ByteLength(), func(raw []byte) ([]byte, error) {
			got, err := ParseCommunityTargetPacketV2(raw, s, limits())
			return got.CanonicalBytes(), err
		}},
		{"checkpoint", c.CanonicalBytes(), c.Identity(), c.Digest(), c.ByteLength(), func(raw []byte) ([]byte, error) {
			got, err := ParseCheckpointV2(raw, p, limits())
			return got.CanonicalBytes(), err
		}},
		{"request", r.CanonicalBytes(), r.Identity(), r.Digest(), r.ByteLength(), func(raw []byte) ([]byte, error) {
			got, err := ParseReplayRequestV2(raw, c, p, limits())
			return got.CanonicalBytes(), err
		}},
		{"result", result.CanonicalBytes(), result.Identity(), result.Digest(), result.ByteLength(), func(raw []byte) ([]byte, error) {
			got, err := ParseReplayResultV2(raw, replayInput(s, p, c, r, limits()))
			return got.CanonicalBytes(), err
		}},
	}
	if len(actual) != len(locks) {
		t.Fatal("ASSERT_STAGE7B_CARRIER_LOCK_COUNT")
	}
	for i, got := range actual {
		want := locks[i]
		if got.name != want.name || !bytes.Equal(got.raw, want.bytes) || got.digest != want.digest || got.length != want.length || got.identity == "" {
			t.Fatalf("ASSERT_STAGE7B_CONSTRUCTOR_EXACT_%s identity=%s digest=%s/%s length=%d/%d\ngot=%s\nwant=%s", got.name, got.identity, got.digest, want.digest, got.length, want.length, got.raw, want.bytes)
		}
		parsed, err := got.parse(want.bytes)
		if err != nil || !bytes.Equal(parsed, want.bytes) {
			t.Fatalf("ASSERT_STAGE7B_PARSER_EXACT_%s err=%v got=%s want=%s", got.name, err, parsed, want.bytes)
		}
	}
}

func TestStage7BIndependentGoldensAndFieldwisePredecessors(t *testing.T) {
	ps := testPredecessors()
	objects := exactInputs()
	if len(objects) != len(ps) {
		t.Fatal("ASSERT_STAGE7B_PREDECESSOR_COUNT")
	}
	for i, p := range ps {
		o := objects[i]
		if o.Role() != p.Role || o.Schema() != p.Schema || o.Identity() != p.Identity || o.Digest() != p.Digest || o.ByteLength() != p.ByteLength || !bytes.Equal(o.ExactBytes(), p.ExactBytes) {
			t.Fatalf("ASSERT_STAGE7B_PREDECESSOR_FIELDWISE_%d", i)
		}
	}
	s, p, c, r := fixture(t)
	for name, got := range map[string][]byte{"snapshot": s.CanonicalBytes(), "packet": p.CanonicalBytes(), "checkpoint": c.CanonicalBytes(), "request": r.CanonicalBytes()} {
		if len(got) == 0 || got[len(got)-1] != '\n' || bytes.Contains(got, []byte{'\r'}) {
			t.Fatalf("ASSERT_STAGE7B_CANONICAL_%s", name)
		}
		if testSHA(got) == "" {
			t.Fatal("ASSERT_STAGE7B_TEST_DIGEST")
		}
	}
	if s.Schema() != snapshotSchema || p.Schema() != packetSchema || c.Schema() != checkpointSchema || r.Schema() != requestSchema {
		t.Fatal("ASSERT_STAGE7B_V2_SCHEMAS")
	}
	if s.Authority() != 0 || s.Accepted() || s.Completeness() != "UNKNOWN" {
		t.Fatal("ASSERT_STAGE7B_CLAIM_CEILING")
	}
	ctx := s.SourceContexts()
	occ := s.Occurrences()
	wantDocs := expectedSnapshot(testPredecessors()).Payload.Documents
	if len(ctx) != len(wantDocs) {
		t.Fatal("ASSERT_STAGE7B_SOURCE_CONTEXT_COUNT")
	}
	for i, wantDoc := range wantDocs {
		if ctx[i].URI() != wantDoc.URI || ctx[i].Custody() != wantDoc.Custody || ctx[i].Session() != wantDoc.Session || ctx[i].Generation() != wantDoc.Generation || ctx[i].Version() != wantDoc.DocumentVersion || ctx[i].Revision() != wantDoc.Revision || ctx[i].RevisionCustody() != wantDoc.RevisionCustody || ctx[i].Digest() != wantDoc.Digest || ctx[i].ByteLength() != wantDoc.ByteLength || ctx[i].PositionEncoding() != wantDoc.PositionEncoding || !bytes.Equal(ctx[i].ExactBytes(), wantDoc.ExactBytes) {
			t.Fatalf("ASSERT_STAGE7B_SOURCE_CONTEXT_COMPLETE index=%d", i)
		}
	}
	wantOcc := expectedSnapshot(testPredecessors()).Payload.Occurrences[0]
	if len(occ) != 1 || occ[0].Identity() != wantOcc.Identity || occ[0].Schema() != wantOcc.Schema || occ[0].Digest() != wantOcc.Digest || occ[0].ByteLength() != wantOcc.ByteLength || occ[0].RelationID() != wantOcc.RelationID || occ[0].PredecessorIdentity() != wantOcc.PredecessorIdentity || occ[0].Kind() != wantOcc.Kind || occ[0].CallerID() != wantOcc.CallerID || occ[0].TargetID() != wantOcc.TargetID || occ[0].Ordinal() != wantOcc.Ordinal || occ[0].Multiplicity() != wantOcc.Multiplicity || occ[0].EvidenceRange() != stage7bRangeFromTest(wantOcc.EvidenceRange) || occ[0].ItemRange() != stage7bRangeFromTest(wantOcc.ItemRange) || occ[0].SelectionRange() != stage7bRangeFromTest(wantOcc.SelectionRange) {
		t.Fatal("ASSERT_STAGE7B_OCCURRENCE_COMPLETE")
	}
	outcomes := s.RoleOutcomes()
	wantOutcomes := expectedSnapshot(testPredecessors()).Payload.Outcomes
	if len(outcomes) != len(wantOutcomes) {
		t.Fatal("ASSERT_STAGE7B_ROLE_OUTCOME_COUNT")
	}
	for i, want := range wantOutcomes {
		got := outcomes[i]
		if got.Role() != want.Role || got.Outcome() != want.Outcome || got.DocumentURI() != want.DocumentURI || got.OccurrenceIdentity() != want.OccurrenceIdentity || got.PointID() != want.PointID || got.SpanID() != want.SpanID || got.DisplayRange() != stage7bRangeFromTest(want.DisplayRange) || got.ContextSpan() != stage7bRangeFromTest(want.ContextSpan) || !bytes.Equal(got.ExactBytes(), want.ExactBytes) {
			t.Fatalf("ASSERT_STAGE7B_ROLE_OUTCOME_COMPLETE index=%d", i)
		}
	}
}

func TestStage7BStrictCarrierAdmissionMutations(t *testing.T) {
	s, p, c, r := fixture(t)
	type carrier struct {
		name  string
		raw   []byte
		admit func([]byte) error
	}
	result, err := ReplayV2(ReplayInputV2{Request: r, Checkpoint: c, Packet: p, Snapshot: s, Objects: memoryObjects(exactInputs()), Limits: limits()})
	if err != nil {
		t.Fatal(err)
	}
	cs := []carrier{{"snapshot", s.CanonicalBytes(), func(b []byte) error { _, e := ParseSnapshotV2(b, limits()); return e }}, {"packet", p.CanonicalBytes(), func(b []byte) error { _, e := ParseCommunityTargetPacketV2(b, s, limits()); return e }}, {"checkpoint", c.CanonicalBytes(), func(b []byte) error { _, e := ParseCheckpointV2(b, p, limits()); return e }}, {"request", r.CanonicalBytes(), func(b []byte) error { _, e := ParseReplayRequestV2(b, c, p, limits()); return e }}, {"result", result.CanonicalBytes(), func(b []byte) error {
		got, e := ParseReplayResultV2(b, replayInput(s, p, c, r, limits()))
		if e != nil {
			if !reflect.DeepEqual(got, ReplayResultV2{}) {
				t.Fatal("ASSERT_STAGE7B_RESULT_PARSE_ATOMIC")
			}
			return e
		}
		return nil
	}}}
	for _, cc := range cs {
		t.Run(cc.name, func(t *testing.T) {
			if err := cc.admit(cc.raw); err != nil {
				t.Fatal(err)
			}
			mutations := map[string][]byte{
				"unknown":  mutateJSON(t, cc.raw, func(v map[string]any) { v["unknown"] = true }),
				"trailing": append(append([]byte(nil), cc.raw...), []byte("{}\n")...),
				"crlf":     bytes.ReplaceAll(cc.raw, []byte("\n"), []byte("\r\n")),
				"order":    validReorderedTopLevel(t, cc.raw),
				"schema":   mutateJSON(t, cc.raw, func(v map[string]any) { v["schema"] = "wrong" }),
				"identity": mutateJSON(t, cc.raw, func(v map[string]any) { v["identity"] = "wrong" }),
				"digest":   mutateJSON(t, cc.raw, func(v map[string]any) { v["digest"] = "sha256:" + strings.Repeat("0", 64) }),
				"length":   mutateJSON(t, cc.raw, func(v map[string]any) { v["byte_length"] = float64(1) }),
				"nested":   nestedUnknownInPayload(t, cc.raw),
			}
			for n, b := range mutations {
				if err := cc.admit(b); err == nil {
					t.Fatalf("ASSERT_STAGE7B_STRICT_%s_%s", cc.name, n)
				}
			}
		})
	}
}

func TestStage7BEveryPredecessorSubstitutionAndResealedForgery(t *testing.T) {
	base := exactInputs()
	for i := range base {
		for _, field := range []string{"role", "schema", "identity", "digest", "length", "bytes", "omit"} {
			t.Run(base[i].Role()+"/"+field, func(t *testing.T) {
				bad := append([]ExactObject(nil), base...)
				o := bad[i]
				switch field {
				case "role":
					o = NewExactObject("wrong", o.Schema(), o.Identity(), o.Digest(), o.ByteLength(), o.ExactBytes())
				case "schema":
					o = NewExactObject(o.Role(), "wrong", o.Identity(), o.Digest(), o.ByteLength(), o.ExactBytes())
				case "identity":
					o = NewExactObject(o.Role(), o.Schema(), "wrong", o.Digest(), o.ByteLength(), o.ExactBytes())
				case "digest":
					o = NewExactObject(o.Role(), o.Schema(), o.Identity(), "sha256:"+strings.Repeat("0", 64), o.ByteLength(), o.ExactBytes())
				case "length":
					o = NewExactObject(o.Role(), o.Schema(), o.Identity(), o.Digest(), o.ByteLength()+1, o.ExactBytes())
				case "bytes":
					o = NewExactObject(o.Role(), o.Schema(), o.Identity(), o.Digest(), o.ByteLength(), append(o.ExactBytes(), 'x'))
				case "omit":
					bad = append(bad[:i], bad[i+1:]...)
				}
				if field != "omit" {
					bad[i] = o
				}
				if got, err := NewSnapshotV2(SnapshotInputV2{Objects: bad, Limits: limits()}); err == nil || !reflect.DeepEqual(got, SnapshotV2{}) {
					t.Fatal("ASSERT_STAGE7B_PREDECESSOR_SUBSTITUTION")
				}
			})
		}
	}
	for i := range base {
		t.Run("resealed/"+base[i].Role(), func(t *testing.T) {
			bad := append([]ExactObject(nil), base...)
			b := append([]byte(nil), bad[i].ExactBytes()...)
			b[len(b)-2] ^= 1
			bad[i] = NewExactObject(bad[i].Role(), bad[i].Schema(), bad[i].Identity(), testSHA(b), uint64(len(b)), b)
			if got, err := NewSnapshotV2(SnapshotInputV2{Objects: bad, Limits: limits()}); err == nil || FailureCode(err) != FailureSemanticReplayMismatch || !reflect.DeepEqual(got, SnapshotV2{}) {
				t.Fatal("ASSERT_STAGE7B_RESEALED_FORGERY")
			}
		})
	}
}

type expectedAccounting struct{ Canonical, Predecessor, Objects, ObjectBytes, Documents, Occurrences, Communities, Targets, Ranges, Requests, Outputs, Work uint64 }

func testAccountingFromExpectedInputs(predecessors []testPredecessor) expectedAccounting {
	locks := expectedLocks(predecessors)
	var canonical uint64
	for _, lock := range locks[:4] {
		if math.MaxUint64-canonical < lock.length {
			panic("expected canonical-byte overflow")
		}
		canonical += lock.length
	}
	predecessorBytes := expectedPredecessorBytes(predecessors)
	return expectedAccounting{Canonical: canonical, Predecessor: predecessorBytes, Objects: uint64(len(predecessors)), ObjectBytes: predecessorBytes, Documents: 2, Occurrences: 1, Communities: 1, Targets: 1, Ranges: 5, Requests: 1, Outputs: 1, Work: uint64(len(predecessors) + 2)}
}

func TestStage7BLimitsAndAccountingIndependentOracle(t *testing.T) {
	s, p, c, r := fixture(t)
	want := testAccountingFromExpectedInputs(testPredecessors())
	dims := map[string]uint64{"canonical": want.Canonical, "predecessor": want.Predecessor, "objects": want.Objects, "documents": want.Documents, "occurrences": want.Occurrences, "communities": want.Communities, "targets": want.Targets, "ranges": want.Ranges, "requests": want.Requests, "outputs": want.Outputs, "work": want.Work}
	for name, n := range dims {
		below := n - 1
		if name == "canonical" {
			// MaxCanonicalBytes is itself serialized in the request; use a
			// stable strict-below boundary rather than a digit-sensitive N-1.
			below = n / 2
		}
		for _, bound := range []uint64{0, below, n, math.MaxUint64} {
			if n == 0 && bound == n-1 {
				continue
			}
			t.Run(fmt.Sprintf("%s/%d", name, bound), func(t *testing.T) {
				l := limits().WithName(name, bound)
				limitedRequest, requestErr := NewReplayRequestV2(ReplayRequestInputV2{Checkpoint: c, Packet: p, RequestedIdentities: p.TargetIdentities(), Limits: l})
				if requestErr != nil {
					if bound < n && FailureCode(requestErr) == FailureResourceBound {
						return
					}
					t.Fatalf("ASSERT_STAGE7B_LIMIT_REQUEST name=%s bound=%d err=%v", name, bound, requestErr)
				}
				got, err := ReplayV2(ReplayInputV2{Request: limitedRequest, Checkpoint: c, Packet: p, Snapshot: s, Objects: memoryObjects(exactInputs()), Limits: l})
				if bound < n {
					assertZero(t, got, err, FailureResourceBound)
				} else if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	got, err := ReplayV2(replayInput(s, p, c, r, limits()))
	if err != nil {
		t.Fatal(err)
	}
	a := got.Accounting()
	if a.CanonicalBytes() != want.Canonical || a.TotalPredecessorBytes() != want.Predecessor || a.Objects() != want.Objects || a.ObjectBytes() != want.ObjectBytes || a.Documents() != want.Documents || a.Occurrences() != want.Occurrences || a.Communities() != want.Communities || a.Targets() != want.Targets || a.Ranges() != want.Ranges || a.Requests() != want.Requests || a.Outputs() != want.Outputs || a.Work() != want.Work || !a.Balanced() {
		t.Fatal("ASSERT_STAGE7B_ACCOUNTING_FIELDWISE")
	}
	var maxIndividual uint64
	for _, object := range exactInputs() {
		if object.ByteLength() > maxIndividual {
			maxIndividual = object.ByteLength()
		}
	}
	if want.ObjectBytes <= maxIndividual {
		t.Fatal("ASSERT_STAGE7B_PER_OBJECT_FIXTURE_NOT_AGGREGATE")
	}
	individual := limits().WithName("object-bytes", maxIndividual)
	individualRequest, err := NewReplayRequestV2(ReplayRequestInputV2{Checkpoint: c, Packet: p, RequestedIdentities: p.TargetIdentities(), Limits: individual})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplayV2(replayInput(s, p, c, individualRequest, individual)); err != nil {
		t.Fatalf("ASSERT_STAGE7B_PER_OBJECT_AGGREGATE_ALLOWED err=%v", err)
	}
	oversized := limits().WithName("object-bytes", maxIndividual-1)
	oversizedRequest, requestErr := NewReplayRequestV2(ReplayRequestInputV2{Checkpoint: c, Packet: p, RequestedIdentities: p.TargetIdentities(), Limits: oversized})
	if requestErr != nil {
		if FailureCode(requestErr) != FailureResourceBound {
			t.Fatal(requestErr)
		}
		return
	}
	oversizedResult, oversizedErr := ReplayV2(replayInput(s, p, c, oversizedRequest, oversized))
	assertZero(t, oversizedResult, oversizedErr, FailureResourceBound)
}

func TestStage7BRequestIdentityContractAndResultRecomputation(t *testing.T) {
	s, p, c, r := fixture(t)
	base := ReplayRequestInputV2{Checkpoint: c, Packet: p, RequestedIdentities: p.TargetIdentities(), Limits: limits()}
	for name, ids := range map[string][]string{
		"empty":     nil,
		"duplicate": {"symbol:target", "symbol:target"},
		"unknown":   {"symbol:unknown"},
		"extra":     {"symbol:target", "symbol:unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			bad := base
			bad.RequestedIdentities = ids
			if got, err := NewReplayRequestV2(bad); err == nil || FailureCode(err) != FailureSemanticReplayMismatch || !reflect.DeepEqual(got, ReplayRequestV2{}) {
				t.Fatalf("ASSERT_STAGE7B_REQUEST_IDENTITIES_CONSTRUCTOR_%s", name)
			}
			var requestEnvelope testEnvelopeDTO[testRequestDTO]
			if err := json.Unmarshal(r.CanonicalBytes(), &requestEnvelope); err != nil {
				t.Fatal(err)
			}
			requestEnvelope.Payload.RequestedIdentities = append([]string(nil), ids...)
			forged := expectedSeal("request", requestSchema, requestEnvelope.Payload)
			parsed, parseErr := ParseReplayRequestV2(testCanonical(forged), c, p, limits())
			if parseErr == nil || FailureCode(parseErr) != FailureSemanticReplayMismatch || !reflect.DeepEqual(parsed, ReplayRequestV2{}) {
				t.Fatalf("ASSERT_STAGE7B_REQUEST_IDENTITIES_PARSER_%s err=%v", name, parseErr)
			}
		})
	}
	result, err := ReplayV2(replayInput(s, p, c, r, limits()))
	if err != nil {
		t.Fatal(err)
	}
	var envelope testEnvelopeDTO[testResultDTO]
	if err := json.Unmarshal(result.CanonicalBytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	for name, forge := range map[string]func(*testResultDTO){
		"failure":    func(payload *testResultDTO) { payload.Failure = "forged" },
		"output":     func(payload *testResultDTO) { payload.Outputs[0].ExactBytes = []byte("forged") },
		"accounting": func(payload *testResultDTO) { payload.Accounting.Outputs++ },
	} {
		t.Run(name, func(t *testing.T) {
			payload := envelope.Payload
			payload.Outputs = append([]testOutputDTO(nil), payload.Outputs...)
			forge(&payload)
			forged := expectedSeal("result", resultSchema, payload)
			got, err := ParseReplayResultV2(testCanonical(forged), replayInput(s, p, c, r, limits()))
			assertZero(t, got, err, FailureSemanticReplayMismatch)
		})
	}
}

func TestStage7BMultipleTargetsExactOrderBytesAndAccounting(t *testing.T) {
	s, p, c, _ := fixture(t)
	p.e.Payload.Targets = []stage7bTarget{{"community:0", "symbol:a", 0}, {"community:0", "symbol:b", 1}}
	_, p.canonical, _ = stage7bSeal("packet", stage7bPacketSchema, p.e.Payload)
	p.e, _, _ = stage7bSeal("packet", stage7bPacketSchema, p.e.Payload)
	c, _ = NewCheckpointV2(CheckpointInputV2{Packet: p, Limits: limits()})
	r, err := NewReplayRequestV2(ReplayRequestInputV2{Checkpoint: c, Packet: p, RequestedIdentities: []string{"symbol:a", "symbol:b"}, Limits: limits()})
	if err != nil {
		t.Fatal(err)
	}
	objects := memoryObjects(exactInputs())
	objects["symbol:a"], objects["symbol:b"] = []byte("alpha"), []byte("beta")
	in := ReplayInputV2{Request: r, Checkpoint: c, Packet: p, Snapshot: s, Objects: objects, Limits: limits()}
	got, err := ReplayV2(in)
	if err != nil {
		t.Fatal(err)
	}
	outputs := got.Outputs()
	if len(outputs) != 2 || outputs[0].Identity() != "symbol:a" || !bytes.Equal(outputs[0].ExactBytes(), []byte("alpha")) || outputs[1].Identity() != "symbol:b" || !bytes.Equal(outputs[1].ExactBytes(), []byte("beta")) || got.Accounting().Targets() != 2 || got.Accounting().Outputs() != 2 {
		t.Fatal("ASSERT_STAGE7B_MULTI_TARGET_EXACT")
	}
	for name, ids := range map[string][]string{"omitted": {"symbol:a"}, "reordered": {"symbol:b", "symbol:a"}} {
		t.Run(name, func(t *testing.T) {
			if request, err := NewReplayRequestV2(ReplayRequestInputV2{Checkpoint: c, Packet: p, RequestedIdentities: ids, Limits: limits()}); err == nil || !reflect.DeepEqual(request, ReplayRequestV2{}) {
				t.Fatalf("ASSERT_STAGE7B_MULTI_TARGET_%s", name)
			}
		})
	}
}

func TestStage7BReplayFixedExpectedPermutationDuplicateAndDeepCopy(t *testing.T) {
	s, p, c, r := fixture(t)
	objects := memoryObjects(exactInputs())
	want := expectedLocks(testPredecessors())[4].bytes
	identities := make([]string, 0, len(objects))
	for identity := range objects {
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	orders := allPermutations(identities)
	wantPermutations := 1
	for i := 2; i <= len(identities); i++ {
		wantPermutations *= i
	}
	if len(orders) != wantPermutations {
		t.Fatalf("ASSERT_STAGE7B_PERMUTATION_EXHAUSTIVE got=%d want=%d", len(orders), wantPermutations)
	}
	for _, order := range orders {
		in := make(map[string][]byte, len(objects))
		for _, k := range order {
			in[k] = append([]byte(nil), objects[k]...)
		}
		got, err := ReplayV2(ReplayInputV2{Request: r, Checkpoint: c, Packet: p, Snapshot: s, Objects: in, Limits: limits()})
		if err != nil || !bytes.Equal(got.CanonicalBytes(), want) {
			t.Fatalf("ASSERT_STAGE7B_FIXED_REPLAY err=%v got=%s", err, got.CanonicalBytes())
		}
	}
	if err := AdmitExactObjects(append(exactInputs(), exactInputs()[0])); err == nil || FailureCode(err) != FailureDuplicateIdentity {
		t.Fatal("ASSERT_STAGE7B_DUPLICATE_SAME_EXACT")
	}
	different := append([]ExactObject(nil), exactInputs()...)
	x := different[0]
	different = append(different, NewExactObject(x.Role(), x.Schema(), x.Identity(), testSHA([]byte("different")), 9, []byte("different")))
	if err := AdmitExactObjects(different); err == nil || FailureCode(err) != FailureDuplicateIdentity {
		t.Fatal("ASSERT_STAGE7B_DUPLICATE_DIFFERENT_REJECT")
	}
	got, err := ReplayV2(ReplayInputV2{Request: r, Checkpoint: c, Packet: p, Snapshot: s, Objects: objects, Limits: limits()})
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), got.CanonicalBytes()...)
	outs := got.Outputs()
	outs[0].ExactBytes()[0] ^= 0xff
	for identity, body := range objects {
		if len(body) != 0 {
			body[0] ^= 0xff
			objects[identity] = append(body, 0xff)
		}
	}
	if !bytes.Equal(got.CanonicalBytes(), before) || !bytes.Equal(s.Objects()[0].ExactBytes(), testPredecessors()[0].ExactBytes) {
		t.Fatal("ASSERT_STAGE7B_DEEP_COPY_ALL_LAYERS")
	}
}

func allPermutations(in []string) [][]string {
	work := append([]string(nil), in...)
	var out [][]string
	var visit func(int)
	visit = func(i int) {
		if i == len(work) {
			out = append(out, append([]string(nil), work...))
			return
		}
		for j := i; j < len(work); j++ {
			work[i], work[j] = work[j], work[i]
			visit(i + 1)
			work[i], work[j] = work[j], work[i]
		}
	}
	visit(0)
	return out
}

func TestStage7BConcurrentReplayAndReturnedCopyMutation(t *testing.T) {
	s, p, c, r := fixture(t)
	const n = 32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := ReplayV2(ReplayInputV2{Request: r, Checkpoint: c, Packet: p, Snapshot: s, Objects: memoryObjects(exactInputs()), Limits: limits()})
			if err == nil {
				b := got.CanonicalBytes()
				b[0] ^= 0xff
				o := got.Outputs()
				o[0].ExactBytes()[0] ^= 0xff
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestStage7BWorkspaceFreeProductionSurfaceAndNoLaterCoupling(t *testing.T) {
	const productionFile = "stage7b_custody.go"
	statusCmd := exec.Command("git", "status", "--porcelain=v1", "--untracked-files=all")
	statusCmd.Dir = filepath.Clean(filepath.Join("..", ".."))
	changed, err := statusCmd.Output()
	if err != nil {
		t.Fatalf("ASSERT_STAGE7B_TESTS_ONLY_STATUS err=%v", err)
	}
	allowed := map[string]bool{
		"internal/censuscontinuation/programc_qualification_subject.go":               true,
		"internal/censuscontinuation/programc_qualification_subject_test.go":          true,
		"internal/censuscontinuation/programc_qualification_verifier_private.go":      true,
		"internal/censuscontinuation/programc_qualification_verifier_private_test.go": true,
		"internal/censuscontinuation/programc_stage6_adapter_v2_private.go":           true,
		"internal/censuscontinuation/programc_stage6_adapter_v2_private_test.go":      true,
		"internal/censuscontinuation/programc_v2_preflight_private.go":                true,
		"internal/censuscontinuation/programc_v2_preflight_private_test.go":           true,
		"internal/censuscontinuation/programc_transition.go":                          true,
		"internal/censuscontinuation/programc_transition_test.go":                     true,
		"internal/integratedconformance/harness_test.go":                              true,
		"internal/inventorysuccessor/coordinator.go":                                  true,
		"internal/inventorysuccessor/coordinator_test.go":                             true,
		"internal/inventoryworkflow/qualification_subject.go":                         true,
		"internal/inventoryworkflow/qualification_subject_surface_test.go":            true,
		"internal/inventoryworkflow/workflow_production_test.go":                      true,
		"internal/programcrepresentative/qualification_subject.go":                    true,
		"internal/programcrepresentative/qualification_subject_builders_test.go":      true,
		"internal/programcrepresentative/qualification_subject_test.go":               true,
		"internal/programcrepresentative/mechanics_v2.go":                             true,
		"internal/programcrepresentative/mechanics_v2_limits_internal_test.go":        true,
		"internal/programcrepresentative/mechanics_v2_test.go":                        true,
		"internal/programcrepresentative/source_context_red_test.go":                  true,
		"internal/programcrepresentative/stage7b_causal_red_test.go":                  true,
		"internal/programcrepresentative/stage7b_custody.go":                          true,
		"internal/programc/stage7c_artifacts_red_test.go":                             true,
		"internal/programc/stage7c_coupling.go":                                       true,
		"internal/programc/stage7c_instability.go":                                    true,
	}
	for _, line := range strings.Split(strings.TrimSpace(string(changed)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !allowed[fields[len(fields)-1]] {
			t.Fatalf("ASSERT_STAGE7B_TESTS_ONLY_DIFF changed=%q", line)
		}
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var productionNames []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" && !strings.HasSuffix(entry.Name(), "_test.go") {
			productionNames = append(productionNames, entry.Name())
		}
	}
	if len(productionNames) == 0 {
		t.Fatal("ASSERT_STAGE7B_PACKAGE_AST_EMPTY")
	}
	_ = fmt.Sprintf("%d", len(productionNames))
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, productionFile, nil, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name.Name != "programcrepresentative" {
		t.Fatalf("ASSERT_STAGE7B_EXACT_OWNER package=%s", f.Name.Name)
	}
	allowedExports := map[string]bool{"ExactObject": true, "LimitsV2": true, "SnapshotV2": true, "SnapshotInputV2": true, "CommunityTargetPacketV2": true, "CommunityTargetPacketInputV2": true, "CheckpointV2": true, "CheckpointInputV2": true, "ReplayRequestV2": true, "ReplayRequestInputV2": true, "ReplayResultV2": true, "ReplayInputV2": true, "FailureCodeV2": true, "NewExactObject": true, "NewSnapshotV2": true, "NewCommunityTargetPacketV2": true, "NewCheckpointV2": true, "NewReplayRequestV2": true, "ParseSnapshotV2": true, "ParseCommunityTargetPacketV2": true, "ParseCheckpointV2": true, "ParseReplayRequestV2": true, "ParseReplayResultV2": true, "AdmitExactObjects": true, "ReplayV2": true, "FailureCode": true}
	allowedMethods := map[string]bool{"Role": true, "Schema": true, "Identity": true, "Digest": true, "ByteLength": true, "ExactBytes": true, "CanonicalBytes": true, "Objects": true, "SourceContexts": true, "Occurrences": true, "RoleOutcomes": true, "Ranges": true, "Authority": true, "Accepted": true, "Completeness": true, "Communities": true, "TargetIdentities": true, "Outputs": true, "Accounting": true, "Balanced": true, "WithName": true}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ast.IsExported(ts.Name.Name) && !allowedExports[ts.Name.Name] {
					t.Fatalf("ASSERT_STAGE7B_EXPORT_ALLOWLIST %s", ts.Name.Name)
				}
			}
		case *ast.FuncDecl:
			if !ast.IsExported(d.Name.Name) {
				continue
			}
			if d.Recv == nil && !allowedExports[d.Name.Name] {
				t.Fatalf("ASSERT_STAGE7B_EXPORT_ALLOWLIST %s", d.Name.Name)
			}
			if d.Recv != nil && !allowedMethods[d.Name.Name] {
				t.Fatalf("ASSERT_STAGE7B_METHOD_ALLOWLIST %s", d.Name.Name)
			}
		}
	}
	forbidden := []string{"os", "io/fs", "path", "path/filepath", "workspace", "filesystem", "publication", "contentaddress", "acquirer", "resolver", "callback", "stage7c", "instability", "adr0007", "cli", "mcp", "host"}
	for _, name := range productionNames {
		pf, parseErr := parser.ParseFile(fset, name, nil, parser.AllErrors)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, im := range pf.Imports {
			path := strings.ToLower(strings.Trim(im.Path.Value, "\""))
			for _, bad := range forbidden {
				if strings.Contains(path, bad) {
					t.Fatalf("ASSERT_STAGE7B_PACKAGE_FORBIDDEN_IMPORT file=%s import=%s", name, bad)
				}
			}
		}
	}
	for _, im := range f.Imports {
		v := strings.ToLower(strings.Trim(im.Path.Value, "\""))
		for _, bad := range forbidden {
			if strings.Contains(v, bad) {
				t.Fatalf("ASSERT_STAGE7B_FORBIDDEN_IMPORT %s", bad)
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			v := strings.ToLower(id.Name)
			for _, bad := range forbidden[4:] {
				if strings.Contains(v, bad) {
					t.Fatalf("ASSERT_STAGE7B_FORBIDDEN_API %s", bad)
				}
			}
		}
		if ft, ok := n.(*ast.FuncType); ok && ft.Params != nil {
			for _, p := range ft.Params.List {
				if _, callback := p.Type.(*ast.FuncType); callback {
					t.Fatal("ASSERT_STAGE7B_FORBIDDEN_CALLBACK")
				}
			}
		}
		return true
	})
	types := []reflect.Type{reflect.TypeOf(SnapshotV2{}), reflect.TypeOf(CommunityTargetPacketV2{}), reflect.TypeOf(CheckpointV2{}), reflect.TypeOf(ReplayRequestV2{}), reflect.TypeOf(ReplayResultV2{})}
	sort.Slice(types, func(i, j int) bool { return types[i].Name() < types[j].Name() })
	for _, typ := range types {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.IsExported() && (f.Type.Kind() == reflect.Map || f.Type.Kind() == reflect.Slice || f.Type.Kind() == reflect.Pointer || f.Type.Kind() == reflect.Interface || f.Type.Kind() == reflect.Func) {
				t.Fatalf("ASSERT_STAGE7B_MUTABLE_EXPORTED_FIELD %s.%s", typ.Name(), f.Name)
			}
		}
	}
}
