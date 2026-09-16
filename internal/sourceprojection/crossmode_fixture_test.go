package sourceprojection

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"unicode/utf16"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
)

const fixtureDir = "testdata/crossmode-v1"

const (
	assertFixtureInventory = "ASSERT_CROSSMODE_FIXTURE_INVENTORY"
	assertExactSpecimen    = "ASSERT_CROSSMODE_FIXTURE_EXACT_BYTES_UTF16_RANGES"
	assertServerCalls      = "ASSERT_SERVER_ONLY_CALLS_OCCURRENCE_BINDING"
	assertRetainedCustody  = "ASSERT_V5_EXACT_CUSTODY_AND_SOURCE_OBJECT_BINDING"
	assertLiveAdmission    = "ASSERT_LIVE_EXACT_SESSION_DOCUMENT_ADMISSION"
	assertSharedCore       = "ASSERT_SYMBOL_FACADE_DELEGATES_SHARED_EXACT_TARGET_CORE"
	assertVariants         = "ASSERT_PROJECTION_VARIANTS_ACCOUNTING_AND_CAUSES"
	assertCanonical        = "ASSERT_INPUT_PERMUTATION_CANONICAL_OUTPUT"
	assertNeutrality       = "ASSERT_AUTHORITY_GRAPH_FACTS_COMPLETENESS_INVARIANT"
	assertSemanticAbsent   = "ASSERT_STRUCTURAL_PROJECTION_SEMANTIC_ACCOUNTING_INDEPENDENT"
)

var fixtureFiles = []string{
	"source.ts",
	"structural.json",
	"retained-v5.json",
	"retained-availability.json",
	"live-admission.json",
	"requests.json",
	"variants.json",
	"expected.json",
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type sourceRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type structuralFixture struct {
	Schema              string `json:"schema"`
	URI                 string `json:"uri"`
	LogicalSource       string `json:"logical_source"`
	PositionEncoding    string `json:"position_encoding"`
	Authority           int    `json:"authority"`
	GraphFactsAdded     int    `json:"graph_facts_added"`
	SourceGraphComplete string `json:"source_graph_complete"`
	Nodes               []struct {
		ID    string      `json:"id"`
		Name  string      `json:"name"`
		Range sourceRange `json:"range"`
	} `json:"nodes"`
	Relations []struct {
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		Caller   string `json:"caller"`
		Callee   string `json:"callee"`
		Evidence string `json:"evidence"`
	} `json:"relations"`
	Occurrences []struct {
		ID         string      `json:"id"`
		RelationID string      `json:"relation_id"`
		Range      sourceRange `json:"range"`
	} `json:"occurrences"`
}

type retainedAvailabilityFixture struct {
	Schema             string `json:"schema"`
	ArtifactDigest     string `json:"artifact_digest"`
	CaptureID          string `json:"capture_id"`
	ResolverKind       string `json:"resolver_kind"`
	ResolverCount      int    `json:"resolver_count"`
	SourceSHA256       string `json:"source_sha256"`
	SourceByteLength   int    `json:"source_byte_length"`
	Availability       string `json:"availability"`
	PhysicalID         string `json:"physical_projection_id"`
	PhysicalIDPreimage struct {
		Custody          string `json:"custody"`
		ArtifactDigest   string `json:"artifact_digest"`
		CaptureID        string `json:"capture_id"`
		SourceSHA256     string `json:"source_sha256"`
		SourceByteLength int    `json:"source_byte_length"`
	} `json:"physical_id_preimage"`
}

type liveAdmissionFixture struct {
	Schema             string `json:"schema"`
	SessionID          string `json:"session_id"`
	Generation         int    `json:"generation"`
	URI                string `json:"uri"`
	DocumentVersion    int    `json:"document_version"`
	PositionEncoding   string `json:"position_encoding"`
	ContentSHA256      string `json:"content_sha256"`
	ContentByteLength  int    `json:"content_byte_length"`
	ResolverKind       string `json:"resolver_kind"`
	ResolverCount      int    `json:"resolver_count"`
	PhysicalID         string `json:"physical_projection_id"`
	PhysicalIDPreimage struct {
		Custody           string `json:"custody"`
		SessionID         string `json:"session_id"`
		Generation        int    `json:"generation"`
		URI               string `json:"uri"`
		DocumentVersion   int    `json:"document_version"`
		ContentSHA256     string `json:"content_sha256"`
		ContentByteLength int    `json:"content_byte_length"`
	} `json:"physical_id_preimage"`
}

type requestFixture struct {
	Schema   string `json:"schema"`
	Requests []struct {
		ID         string `json:"id"`
		EntryPoint string `json:"entry_point"`
		Mode       string `json:"mode"`
		Symbol     string `json:"symbol,omitempty"`
		URI        string `json:"uri,omitempty"`
		Line       *int   `json:"line,omitempty"`
		Character  *int   `json:"character,omitempty"`
		UpDepth    int    `json:"up_depth"`
		DownDepth  int    `json:"down_depth"`
		Projection string `json:"projection"`
	} `json:"requests"`
	ParityPairs [][]string `json:"parity_pairs"`
}

type variantFixture struct {
	Schema   string    `json:"schema"`
	Variants []variant `json:"variants"`
}

type variant struct {
	ID                   string `json:"id"`
	Disposition          string `json:"disposition"`
	Candidates           int    `json:"candidates"`
	Selected             int    `json:"selected"`
	Omitted              int    `json:"omitted"`
	LogicalSelectedBytes int    `json:"logical_selected_bytes"`
	UniqueEmittedBytes   int    `json:"unique_emitted_bytes"`
	ByteLimit            *int   `json:"byte_limit,omitempty"`
	RangeLimit           *int   `json:"range_limit,omitempty"`
	WholeRanges          bool   `json:"whole_ranges"`
	OmissionCause        string `json:"omission_cause,omitempty"`
}

type expectedFixture struct {
	Schema       string `json:"schema"`
	LogicalUnits []struct {
		ID               string      `json:"id"`
		Role             string      `json:"role"`
		GraphSubject     string      `json:"graph_subject"`
		LogicalSource    string      `json:"logical_source"`
		Range            sourceRange `json:"range"`
		PositionEncoding string      `json:"position_encoding"`
		SelectedBytes    int         `json:"selected_bytes"`
	} `json:"logical_units"`
	Citations []struct {
		ID           string `json:"id"`
		Role         string `json:"role"`
		UnitID       string `json:"unit_id"`
		SubjectID    string `json:"subject_id"`
		OccurrenceID string `json:"occurrence_id,omitempty"`
	} `json:"citations"`
	Projection struct {
		LogicalSelectedBytes int           `json:"logical_selected_bytes"`
		UniqueEmittedBytes   int           `json:"unique_emitted_bytes"`
		EmittedSpans         []sourceRange `json:"emitted_spans"`
	} `json:"projection"`
	Canonical struct {
		NormalOrder            []string `json:"normal_order"`
		PermutedInput          []string `json:"permuted_input"`
		PermutedCanonicalOrder []string `json:"permuted_canonical_order"`
	} `json:"canonical"`
	Semantic struct {
		Executions int  `json:"executions"`
		Accepted   bool `json:"accepted"`
		Authority  int  `json:"authority"`
	} `json:"semantic"`
}

func TestCrossModeFixture(t *testing.T) {
	t.Run(assertFixtureInventory, testFixtureInventory)
	t.Run(assertExactSpecimen, testExactSpecimen)
	t.Run(assertServerCalls, testServerCalls)
	t.Run(assertRetainedCustody, testRetainedCustody)
	t.Run(assertLiveAdmission, testLiveAdmission)
	t.Run(assertSharedCore, testSharedCore)
	t.Run(assertVariants, testVariants)
	t.Run(assertCanonical, testCanonical)
	t.Run(assertNeutrality, testNeutrality)
	t.Run(assertSemanticAbsent, testSemanticAbsent)
}

func testFixtureInventory(t *testing.T) {
	for _, name := range fixtureFiles {
		if _, err := os.Stat(filepath.Join(fixtureDir, name)); err != nil {
			t.Errorf("%s: required fixture file %s: %v", assertFixtureInventory, name, err)
		}
	}
}

func testExactSpecimen(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "source.ts"))
	if err != nil {
		t.Fatalf("%s: read source: %v", assertExactSpecimen, err)
	}
	const want = "function target() {\n  \"😀\"; target();\n}\n"
	if string(raw) != want {
		t.Errorf("%s: source bytes differ", assertExactSpecimen)
	}
	if len(raw) != 42 {
		t.Errorf("%s: byte length=%d want=42", assertExactSpecimen, len(raw))
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != "b2f615ccc7e60a8994f75a1e5cae0a73e0e7f34967a71288eabaad7840d121d5" {
		t.Errorf("%s: sha256=%s", assertExactSpecimen, got)
	}
	var structural structuralFixture
	loadJSON(t, "structural.json", &structural)
	if structural.PositionEncoding != "utf-16" || len(structural.Nodes) != 1 || len(structural.Occurrences) != 1 {
		t.Fatalf("%s: encoding/nodes/occurrences mismatch", assertExactSpecimen)
	}
	wantEndpoint := sourceRange{Start: position{0, 0}, End: position{3, 0}}
	wantCall := sourceRange{Start: position{1, 8}, End: position{1, 16}}
	if structural.Nodes[0].Range != wantEndpoint || structural.Occurrences[0].Range != wantCall {
		t.Errorf("%s: ranges mismatch", assertExactSpecimen)
	}
	selected := sliceUTF16(t, raw, wantCall)
	if string(selected) != "target()" || len(selected) != 8 {
		t.Errorf("%s: selected=%q bytes=%d want target()/8", assertExactSpecimen, selected, len(selected))
	}
}

func testServerCalls(t *testing.T) {
	var f structuralFixture
	loadJSON(t, "structural.json", &f)
	if len(f.Nodes) != 1 || len(f.Relations) != 1 || len(f.Occurrences) != 1 {
		t.Fatalf("%s: want one node/relation/occurrence", assertServerCalls)
	}
	r := f.Relations[0]
	if r.Kind != "CALLS" || r.Caller != f.Nodes[0].ID || r.Callee != f.Nodes[0].ID || r.Evidence != "SERVER_REPORTED" {
		t.Errorf("%s: relation=%+v", assertServerCalls, r)
	}
	if f.Occurrences[0].RelationID != r.ID {
		t.Errorf("%s: occurrence relation=%q want=%q", assertServerCalls, f.Occurrences[0].RelationID, r.ID)
	}
}

func testRetainedCustody(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir, "retained-v5.json"))
	if err != nil {
		t.Fatalf("%s: read retained V5: %v", assertRetainedCustody, err)
	}
	if detected, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v5"); err != nil || detected != graphprovenance.VersionV5 {
		t.Fatalf("%s: real Graph Provenance V5 validation: detected=%q err=%v", assertRetainedCustody, detected, err)
	}
	var carrier graphprovenance.EvidenceV5
	if err := json.Unmarshal(raw, &carrier); err != nil {
		t.Fatalf("%s: decode retained V5: %v", assertRetainedCustody, err)
	}
	native, err := base64.StdEncoding.DecodeString(carrier.GraphV5)
	if err != nil {
		t.Fatalf("%s: decode embedded graph V5: %v", assertRetainedCustody, err)
	}
	if got := digestBytes(native); got != carrier.GraphV5SHA256 {
		t.Errorf("%s: embedded graph digest=%q want=%q", assertRetainedCustody, carrier.GraphV5SHA256, got)
	}
	var retainedGraph graph.Result
	if err := json.Unmarshal(native, &retainedGraph); err != nil {
		t.Fatalf("%s: decode embedded graph: %v", assertRetainedCustody, err)
	}
	var structural structuralFixture
	loadJSON(t, "structural.json", &structural)
	if len(retainedGraph.Nodes) != 1 || len(retainedGraph.Edges) != 1 || len(retainedGraph.Edges[0].CallSites) != 1 || len(structural.Nodes) != 1 || len(structural.Relations) != 1 || len(structural.Occurrences) != 1 {
		t.Fatalf("%s: cross-record structural cardinality mismatch", assertRetainedCustody)
	}
	if retainedGraph.Nodes[0].ID != structural.Nodes[0].ID || retainedGraph.Edges[0].RelationID != structural.Relations[0].ID || retainedGraph.Edges[0].CallerNodeID != structural.Relations[0].Caller || retainedGraph.Edges[0].CalleeNodeID != structural.Relations[0].Callee || graphRange(retainedGraph.Edges[0].CallSites[0]) != structural.Occurrences[0].Range {
		t.Errorf("%s: embedded V5 graph does not match structural fixture", assertRetainedCustody)
	}

	var availability retainedAvailabilityFixture
	loadJSON(t, "retained-availability.json", &availability)
	if availability.Schema != "lsp-trace.fixture.retained-availability.v1" {
		t.Errorf("%s: availability schema mismatch", assertRetainedCustody)
	}
	if availability.ArtifactDigest != digestBytes(raw) || availability.CaptureID != carrier.SessionID {
		t.Errorf("%s: carrier binding mismatch", assertRetainedCustody)
	}
	if availability.ResolverKind != "retained_source_object" || availability.ResolverCount != 1 || availability.Availability != "AVAILABLE" {
		t.Errorf("%s: resolver/availability mismatch", assertRetainedCustody)
	}
	checkSourceIdentity(t, assertRetainedCustody, availability.SourceSHA256, availability.SourceByteLength)
	pre := availability.PhysicalIDPreimage
	if pre.Custody != "retained" || pre.ArtifactDigest != availability.ArtifactDigest || pre.CaptureID != availability.CaptureID || pre.SourceSHA256 != availability.SourceSHA256 || pre.SourceByteLength != availability.SourceByteLength {
		t.Errorf("%s: physical preimage mismatch", assertRetainedCustody)
	}
	if availability.PhysicalID != digestJSON(t, pre) {
		t.Errorf("%s: physical id is not digest of declared preimage", assertRetainedCustody)
	}
}

func testLiveAdmission(t *testing.T) {
	var live liveAdmissionFixture
	loadJSON(t, "live-admission.json", &live)
	if live.Schema != "lsp-trace.fixture.live-admission.v1" || live.SessionID == "" || live.Generation != 1 || live.DocumentVersion != 7 {
		t.Errorf("%s: exact session/document admission mismatch", assertLiveAdmission)
	}
	if live.URI != "file:///fixture/src/source.ts" || live.PositionEncoding != "utf-16" || live.ResolverKind != "live_document" || live.ResolverCount != 1 {
		t.Errorf("%s: live resolver mismatch", assertLiveAdmission)
	}
	checkSourceIdentity(t, assertLiveAdmission, live.ContentSHA256, live.ContentByteLength)
	pre := live.PhysicalIDPreimage
	if pre.Custody != "live" || pre.SessionID != live.SessionID || pre.Generation != live.Generation || pre.URI != live.URI || pre.DocumentVersion != live.DocumentVersion || pre.ContentSHA256 != live.ContentSHA256 || pre.ContentByteLength != live.ContentByteLength {
		t.Errorf("%s: physical preimage mismatch", assertLiveAdmission)
	}
	if live.PhysicalID != digestJSON(t, pre) {
		t.Errorf("%s: physical id is not digest of declared preimage", assertLiveAdmission)
	}
}

func testSharedCore(t *testing.T) {
	var f requestFixture
	loadJSON(t, "requests.json", &f)
	byID := make(map[string]struct {
		entry, mode, projection string
		up, down                int
	})
	for _, r := range f.Requests {
		byID[r.ID] = struct {
			entry, mode, projection string
			up, down                int
		}{r.EntryPoint, r.Mode, r.Projection, r.UpDepth, r.DownDepth}
	}
	if len(f.Requests) != 4 || len(f.ParityPairs) != 2 {
		t.Fatalf("%s: want four requests and two parity pairs", assertSharedCore)
	}
	for _, pair := range f.ParityPairs {
		if len(pair) != 2 {
			t.Fatalf("%s: malformed parity pair", assertSharedCore)
		}
		a, aok := byID[pair[0]]
		b, bok := byID[pair[1]]
		if !aok || !bok || a.entry == b.entry || a.mode != b.mode || a.projection != b.projection || a.up != b.up || a.down != b.down {
			t.Errorf("%s: incompatible parity pair %v", assertSharedCore, pair)
		}
	}
}

func testVariants(t *testing.T) {
	var f variantFixture
	loadJSON(t, "variants.json", &f)
	want := map[string][5]int{
		"body":        {2, 2, 0, 50, 42},
		"metadata":    {2, 2, 0, 0, 0},
		"withheld":    {2, 0, 2, 0, 0},
		"unavailable": {2, 0, 2, 0, 0},
		"byte-limit":  {2, 0, 2, 0, 0},
		"range-limit": {2, 1, 1, 42, 42},
		"empty":       {0, 0, 0, 0, 0},
	}
	if len(f.Variants) != len(want) {
		t.Fatalf("%s: variants=%d want=%d", assertVariants, len(f.Variants), len(want))
	}
	causes := map[string]bool{}
	for _, v := range f.Variants {
		got := [5]int{v.Candidates, v.Selected, v.Omitted, v.LogicalSelectedBytes, v.UniqueEmittedBytes}
		if expected, ok := want[v.ID]; !ok || got != expected {
			t.Errorf("%s: variant %q=%v want=%v", assertVariants, v.ID, got, expected)
		}
		if !v.WholeRanges {
			t.Errorf("%s: variant %q permits partial ranges", assertVariants, v.ID)
		}
		if v.OmissionCause != "" {
			causes[v.OmissionCause] = true
		}
	}
	for _, cause := range []string{"POLICY_WITHHELD", "SOURCE_UNAVAILABLE", "BYTE_LIMIT", "RANGE_LIMIT"} {
		if !causes[cause] {
			t.Errorf("%s: missing distinct omission cause %s", assertVariants, cause)
		}
	}
}

func testCanonical(t *testing.T) {
	var f expectedFixture
	loadJSON(t, "expected.json", &f)
	if !reflect.DeepEqual(f.Canonical.NormalOrder, f.Canonical.PermutedCanonicalOrder) {
		t.Errorf("%s: canonical orders differ", assertCanonical)
	}
	if reflect.DeepEqual(f.Canonical.NormalOrder, f.Canonical.PermutedInput) {
		t.Errorf("%s: permutation fixture did not change input order", assertCanonical)
	}
	if !sort.StringsAreSorted(f.Canonical.NormalOrder) {
		t.Errorf("%s: canonical order is not sorted", assertCanonical)
	}
}

func testNeutrality(t *testing.T) {
	var f structuralFixture
	loadJSON(t, "structural.json", &f)
	if f.Authority != 0 || f.GraphFactsAdded != 0 || f.SourceGraphComplete != "UNKNOWN" {
		t.Errorf("%s: authority=%d graph_facts_added=%d complete=%q", assertNeutrality, f.Authority, f.GraphFactsAdded, f.SourceGraphComplete)
	}
	var retained retainedAvailabilityFixture
	var live liveAdmissionFixture
	loadJSON(t, "retained-availability.json", &retained)
	loadJSON(t, "live-admission.json", &live)
	if retained.PhysicalID == live.PhysicalID {
		t.Errorf("%s: retained/live physical identities must differ", assertNeutrality)
	}
}

func testSemanticAbsent(t *testing.T) {
	var f expectedFixture
	loadJSON(t, "expected.json", &f)
	if f.Semantic.Executions != 0 || f.Semantic.Accepted || f.Semantic.Authority != 0 {
		t.Errorf("%s: semantic=%+v", assertSemanticAbsent, f.Semantic)
	}
	if len(f.LogicalUnits) != 2 || len(f.Citations) != 2 || f.Projection.LogicalSelectedBytes != 50 || f.Projection.UniqueEmittedBytes != 42 || len(f.Projection.EmittedSpans) != 1 {
		t.Errorf("%s: projection accounting mismatch", assertSemanticAbsent)
	}
}

func loadJSON(t *testing.T, name string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	if dec.More() {
		t.Fatalf("decode %s: trailing JSON value", name)
	}
}

func checkSourceIdentity(t *testing.T, assertion, digest string, byteLength int) {
	t.Helper()
	if digest != "sha256:b2f615ccc7e60a8994f75a1e5cae0a73e0e7f34967a71288eabaad7840d121d5" || byteLength != 42 {
		t.Errorf("%s: source identity=(%q,%d)", assertion, digest, byteLength)
	}
}

func digestJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return digestBytes(raw)
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func graphRange(r graph.Range) sourceRange {
	return sourceRange{
		Start: position{Line: int(r.Start.Line), Character: int(r.Start.Character)},
		End:   position{Line: int(r.End.Line), Character: int(r.End.Character)},
	}
}

func sliceUTF16(t *testing.T, raw []byte, r sourceRange) []byte {
	t.Helper()
	lines := bytes.Split(raw, []byte("\n"))
	if r.Start.Line != r.End.Line || r.Start.Line < 0 || r.Start.Line >= len(lines) {
		t.Fatalf("unsupported test range %+v", r)
	}
	runes := []rune(string(lines[r.Start.Line]))
	units := utf16.Encode(runes)
	if r.Start.Character < 0 || r.End.Character < r.Start.Character || r.End.Character > len(units) {
		t.Fatalf("invalid UTF-16 range %+v for %d units", r, len(units))
	}
	selected := utf16.Decode(units[r.Start.Character:r.End.Character])
	return []byte(string(selected))
}

func TestFixtureAssertionNamesAreUnique(t *testing.T) {
	names := []string{assertFixtureInventory, assertExactSpecimen, assertServerCalls, assertRetainedCustody, assertLiveAdmission, assertSharedCore, assertVariants, assertCanonical, assertNeutrality, assertSemanticAbsent}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			t.Fatalf("duplicate assertion name %q", name)
		}
		seen[name] = true
	}
	if len(seen) != len(names) {
		t.Fatal(fmt.Sprintf("assertion count=%d want=%d", len(seen), len(names)))
	}
}
