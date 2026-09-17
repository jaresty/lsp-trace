package liveprojection

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"lsp-trace/internal/mcpcontract"
)

const crossModeV2FixtureDir = "../sourceprojection/testdata/crossmode-v2"

const (
	assertCrossModeV2Inventory   = "ASSERT_CROSSMODE_V2_FIXTURE_INVENTORY"
	assertCrossModeV2Sources     = "ASSERT_CROSSMODE_V2_EXACT_DOCUMENTS"
	assertCrossModeV2Evidence    = "ASSERT_CROSSMODE_V2_SAME_AND_CROSS_DOCUMENT_EVIDENCE"
	assertCrossModeV2Schema      = "ASSERT_CROSSMODE_V2_EXPECTED_SCHEMA"
	assertCrossModeV2Runtime     = "ASSERT_CROSSMODE_V2_RUNTIME_EXACT_ARTIFACT"
	assertCrossModeV2Permutation = "ASSERT_CROSSMODE_V2_PERMUTATION_INVARIANCE"
	assertCrossModeV2Variants    = "ASSERT_CROSSMODE_V2_VARIANT_DENOMINATOR"
	assertCrossModeV2Retained    = "ASSERT_CROSSMODE_V2_LIVE_RETAINED_SEMANTIC_PARITY"
)

var crossModeV2FixtureFiles = []string{"target.go.txt", "caller.go.txt", "structural.json", "requests.json", "variants.json", "expected.json", "retained-snapshot-v2.json", "retained-request.json", "source-object.bin", "source-object.json", "retained-expected.json"}

type crossModeV2Structural struct {
	Schema              string `json:"schema"`
	Authority           int    `json:"authority"`
	SourceGraphComplete string `json:"source_graph_complete"`
	GraphFactsAdded     int    `json:"graph_facts_added"`
	PositionEncoding    string `json:"position_encoding"`
	TargetURI           string `json:"target_uri"`
	Candidates          []struct {
		ID             string `json:"id"`
		Role           string `json:"role"`
		URI            string `json:"uri"`
		EvidenceRange  any    `json:"evidence_range"`
		ItemRange      any    `json:"item_range,omitempty"`
		SelectionRange any    `json:"selection_range,omitempty"`
		Provenance     string `json:"provenance"`
	} `json:"candidates"`
}

type crossModeV2Variants struct {
	Schema   string `json:"schema"`
	Variants []struct {
		ID                string `json:"id"`
		ExpectedStatus    string `json:"expected_status"`
		DocumentOutcome   string `json:"document_outcome"`
		ProjectionOutcome string `json:"projection_outcome"`
		ResponseOutcome   string `json:"response_outcome"`
	} `json:"variants"`
}

func TestCrossModeV2Fixture(t *testing.T) {
	t.Run(assertCrossModeV2Inventory, testCrossModeV2Inventory)
	t.Run(assertCrossModeV2Sources, testCrossModeV2Sources)
	t.Run(assertCrossModeV2Evidence, testCrossModeV2Evidence)
	t.Run(assertCrossModeV2Schema, testCrossModeV2Schema)
	t.Run(assertCrossModeV2Runtime, testCrossModeV2Runtime)
	t.Run(assertCrossModeV2Permutation, testCrossModeV2Permutation)
	t.Run(assertCrossModeV2Variants, testCrossModeV2Variants)
	t.Run(assertCrossModeV2Retained, testCrossModeV2Retained)
}

func testCrossModeV2Inventory(t *testing.T) {
	for _, name := range crossModeV2FixtureFiles {
		if _, err := os.Stat(filepath.Join(crossModeV2FixtureDir, name)); err != nil {
			t.Errorf("%s: required fixture file %s: %v", assertCrossModeV2Inventory, name, err)
		}
	}
}

func testCrossModeV2Sources(t *testing.T) {
	for _, name := range []string{"target.go.txt", "caller.go.txt"} {
		raw := loadCrossModeV2Bytes(t, name)
		want := bytes.TrimSuffix([]byte(name), []byte(".go.txt"))
		want = append(want, []byte("()\n")...)
		if !bytes.Equal(raw, want) {
			t.Fatalf("%s: %s=%q want=%q", assertCrossModeV2Sources, name, raw, want)
		}
	}
}

func testCrossModeV2Evidence(t *testing.T) {
	var fixture crossModeV2Structural
	loadCrossModeV2JSON(t, "structural.json", &fixture)
	if fixture.Authority != 0 || fixture.SourceGraphComplete != "UNKNOWN" || fixture.GraphFactsAdded != 0 || fixture.PositionEncoding != "utf-16" || len(fixture.Candidates) != 3 {
		t.Fatalf("%s: %+v", assertCrossModeV2Evidence, fixture)
	}
	same, cross := false, false
	for _, candidate := range fixture.Candidates {
		if candidate.Provenance != "SERVER_REPORTED" {
			t.Fatalf("%s: provenance=%q", assertCrossModeV2Evidence, candidate.Provenance)
		}
		if candidate.Role == "RELATION" && candidate.URI == fixture.TargetURI {
			same = true
		}
		if candidate.Role == "RELATION" && candidate.URI != fixture.TargetURI {
			cross = true
		}
	}
	if !same || !cross {
		t.Fatalf("%s: same=%v cross=%v", assertCrossModeV2Evidence, same, cross)
	}
}

func testCrossModeV2Schema(t *testing.T) {
	raw := loadCrossModeV2Bytes(t, "expected.json")
	if err := mcpcontract.ValidateJSON(mcpcontract.SourceProjectionResultV2ID, raw); err != nil {
		t.Fatalf("%s: %v", assertCrossModeV2Schema, err)
	}
}

func testCrossModeV2Runtime(t *testing.T) {
	got := assembleCrossModeV2(t, false)
	if os.Getenv("LSP_TRACE_UPDATE_CROSSMODE_V2") == "1" {
		var value any
		if err := json.Unmarshal(got, &value); err != nil {
			t.Fatal(err)
		}
		formatted, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		formatted = append(formatted, '\n')
		if err := os.WriteFile(filepath.Join(crossModeV2FixtureDir, "expected.json"), formatted, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	assertCrossModeV2JSONEqual(t, assertCrossModeV2Runtime, loadCrossModeV2Bytes(t, "expected.json"), got)
}

func testCrossModeV2Permutation(t *testing.T) {
	normal := assembleCrossModeV2(t, false)
	permuted := assembleCrossModeV2(t, true)
	assertCrossModeV2JSONEqual(t, assertCrossModeV2Permutation, normal, permuted)
}

func testCrossModeV2Variants(t *testing.T) {
	var fixture crossModeV2Variants
	loadCrossModeV2JSON(t, "variants.json", &fixture)
	got := make([]string, 0, len(fixture.Variants))
	for _, variant := range fixture.Variants {
		got = append(got, variant.ID)
	}
	sort.Strings(got)
	want := []string{"complete", "cross-document-caller", "document-limit", "permuted-input", "privacy-withheld", "projection-limit", "response-limit", "same-document-caller", "unavailable-source"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: got=%v want=%v", assertCrossModeV2Variants, got, want)
	}
}

func testCrossModeV2Retained(t *testing.T) {
	var live, retained map[string]any
	loadCrossModeV2JSON(t, "expected.json", &live)
	loadCrossModeV2JSON(t, "retained-expected.json", &retained)
	for _, key := range []string{"schema_version", "authority", "source_graph_complete", "graph_facts_added", "status"} {
		if !reflect.DeepEqual(live[key], retained[key]) {
			t.Fatalf("%s: %s live=%v retained=%v", assertCrossModeV2Retained, key, live[key], retained[key])
		}
	}
	liveCustody := live["custody_mode"]
	retainedCustody := retained["custody_mode"]
	if liveCustody != "LIVE" || retainedCustody != "RETAINED" {
		t.Fatalf("%s: custody live=%v retained=%v", assertCrossModeV2Retained, liveCustody, retainedCustody)
	}
	for label, artifact := range map[string]map[string]any{"live": live, "retained": retained} {
		selection := artifact["document_selection"].(map[string]any)
		uris := selection["selected_uris"].([]any)
		if selection["ordering"] != "TARGET_FIRST_THEN_URI_LEXICOGRAPHIC" || len(uris) < 1 || uris[0] != selection["target_uri"] {
			t.Fatalf("%s: %s selection=%v", assertCrossModeV2Retained, label, selection)
		}
		if artifact["authority"] != float64(0) || artifact["source_graph_complete"] != "UNKNOWN" || artifact["graph_facts_added"] != float64(0) {
			t.Fatalf("%s: %s neutrality=%v", assertCrossModeV2Retained, label, artifact)
		}
	}
}

func assembleCrossModeV2(t *testing.T, permute bool) []byte {
	t.Helper()
	prepared, candidates, policy := compositionFixture()
	normalizeV2CandidateIDs(candidates)
	if permute {
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}
	composed := Compose(prepared, "session", 4, candidates, policy)
	prepared.Accounting.Documents.Observed, prepared.Accounting.Bytes.Observed = 2, 18
	got, err := AssembleV2Bounded(composed, prepared, "file:///workspace/target.go", []string{"file:///workspace/target.go", "file:///workspace/caller.go"}, "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertCrossModeV2JSONEqual(t *testing.T, assertion string, left, right []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(left, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(right, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("%s: JSON artifacts differ", assertion)
	}
}

func loadCrossModeV2Bytes(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(crossModeV2FixtureDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func loadCrossModeV2JSON(t *testing.T, name string, dst any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(loadCrossModeV2Bytes(t, name)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatal(err)
	}
}
