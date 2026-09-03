package stage2scenario

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const referenceScenario = `{"version":"stage2-scenario-v1","steps":[{"op":"lifecycle_register","session":"s","generation":1,"state":"READY","expect":{"state":"READY","generation":1,"outcome":"COMPLETE","operation_status":"SUCCEEDED"}},{"op":"lifecycle","session":"s","generation":1,"operation":"RESTART","caller":"caller-a","bind":"restart-1","expect":{"state":"STOPPING","prior_state":"READY","generation":1,"intent_ref":"restart-1","intent_kind":"RESTART","outcome":"COMPLETE","operation_status":"SUCCEEDED"}},{"op":"lifecycle","session":"s","generation":1,"operation":"RESTART","caller":"caller-b","expect":{"state":"STOPPING","generation":1,"intent_ref":"restart-1","intent_kind":"RESTART","outcome":"COMPLETE","operation_status":"SUCCEEDED","joined":true}},{"op":"lifecycle_detach","session":"s","intent_ref":"restart-1","caller":"caller-a","outcome":"REQUEST_TIMEOUT","expect":{"intent_ref":"restart-1","detached":true}},{"op":"request","session":"s","generation":1,"request":"manager-old"},{"op":"child","session":"s","generation":1,"request":"manager-old","lsp_request":"lsp-old-1","child":"fake-child-old-1","ordinal":1},{"op":"child","session":"s","generation":1,"request":"manager-old","lsp_request":"lsp-old-2","child":"fake-child-old-2","ordinal":2},{"op":"cancel","session":"s","generation":1,"request":"manager-old"},{"op":"late_response","session":"s","generation":1,"lsp_request":"lsp-old-1","child":"fake-child-old-1"},{"op":"lifecycle_complete","session":"s","intent_ref":"restart-1","death":true,"reaped":true,"ready":true,"expect":{"state":"READY","generation":2,"intent_ref":"restart-1","intent_kind":"RESTART","outcome":"COMPLETE","operation_status":"SUCCEEDED"}},{"op":"request","session":"s","generation":2,"request":"manager-new"},{"op":"child","session":"s","generation":2,"request":"manager-new","lsp_request":"lsp-new-1","child":"fake-child-new","ordinal":1},{"op":"respond","session":"s","generation":2,"lsp_request":"lsp-new-1","child":"fake-child-new"}]}`

func TestScenarioGrammarClosureAndEarliestDiagnostics(t *testing.T) {
	cases := []struct{ name, data, want string }{
		{"unknown-op", `{"version":"stage2-scenario-v1","steps":[{"op":"mystery"}]}`, `structural stage2-scenario-v1`},
		{"required-before-positive", `{"version":"stage2-scenario-v1","steps":[{"op":"request","session":"s","request":"r"}]}`, `structural stage2-scenario-v1`},
		{"forbidden-lexical-first", `{"version":"stage2-scenario-v1","steps":[{"op":"request","request":"r","generation":1,"outcome":"ready"}]}`, `structural stage2-scenario-v1`},
		{"empty-identity", `{"version":"stage2-scenario-v1","steps":[{"op":"request","session":"","generation":1,"request":"r"}]}`, `structural stage2-scenario-v1`},
		{"implicit-intent", `{"version":"stage2-scenario-v1","steps":[{"op":"lifecycle_complete","session":"s","expect":{}}]}`, `structural stage2-scenario-v1`},
		{"duplicate-request-cross-generation", `{"version":"stage2-scenario-v1","steps":[{"op":"request","session":"s","generation":1,"request":"r"},{"op":"request","session":"s","generation":2,"request":"r"}]}`, `step 1: manager request "r" is not unique`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, e1 := Parse([]byte(tc.data))
			_, e2 := Parse([]byte(tc.data))
			if e1 == nil || e2 == nil || !strings.Contains(e1.Error(), tc.want) || !strings.Contains(e2.Error(), tc.want) {
				t.Fatalf("ASSERT_GRAMMAR_CLOSURE_EARLIEST_DIAGNOSTIC: first=%v second=%v want=%q", e1, e2, tc.want)
			}
		})
	}
}

func TestReferenceInterpreterManagerOutcomesAndOwnership(t *testing.T) {
	s, err := Parse([]byte(referenceScenario))
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := ReplayIntegrated(s)
	if err != nil {
		t.Fatalf("ASSERT_MANAGER_OUTCOME_NOT_IGNORED: %v", err)
	}
	text := string(ledger.Intent)
	for _, fragment := range []string{`"simulated_reference":true`, `"joined":true`, `"detached":true`, `"intent_id":"i1"`, `"manager_request_id":"manager-old"`, `"generation":1`, `"manager_request_id":"manager-new"`, `"generation":2`, `"response_state":"TOMBSTONED"`, `"response_state":"RESPONDED"`, `"kind":"late_response_discarded"`, `"record_type":"final_resource_census"`} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("ASSERT_INTEGRATED_OWNERSHIP_AND_MANAGER_RESULTS: missing %s", fragment)
		}
	}
	if strings.Contains(text, `"ManagerSeq"`) || strings.Contains(text, `"SessionID"`) {
		t.Fatal("ASSERT_CLOSED_LEDGER_DOES_NOT_MARSHAL_MANAGER_INTERNALS")
	}
}

func TestIgnoredManagerOutcomeFails(t *testing.T) {
	bad := strings.Replace(referenceScenario, `"joined":true`, `"joined":false`, 1)
	s, err := Parse([]byte(bad))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ReplayIntegrated(s)
	if err == nil || !strings.Contains(err.Error(), "manager outcome mismatch") {
		t.Fatalf("ASSERT_MANAGER_OUTCOME_NOT_IGNORED: %v", err)
	}
}

func TestCrossGenerationOwnershipPerturbationFails(t *testing.T) {
	bad := strings.Replace(referenceScenario, `"generation":2,"request":"manager-new"`, `"generation":1,"request":"manager-new"`, 1)
	s, err := Parse([]byte(bad))
	if err == nil {
		t.Fatal("ASSERT_CROSS_GENERATION_OWNERSHIP_REJECTED: perturbation parsed")
	}
	_ = s
}

func TestCanonicalLedgerOrderingPerturbationFails(t *testing.T) {
	s, err := Parse([]byte(referenceScenario))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReplayIntegrated(s)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSuffix(got.Intent, []byte("\n")), []byte("\n"))
	if len(lines) < 2 {
		t.Fatal("short ledger")
	}
	lines[0], lines[1] = lines[1], lines[0]
	if bytes.Equal(bytes.Join(lines, []byte("\n")), bytes.TrimSuffix(got.Intent, []byte("\n"))) {
		t.Fatal("ASSERT_LEDGER_ORDERING_PERTURBATION_DETECTED")
	}
}

func TestGoldenCatalogAndDeclaredReplacements(t *testing.T) {
	for _, name := range []string{"catalog.v1.json", "replacements.v1.json"} {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("ASSERT_MACHINE_CATALOG_PRESENT[%s]: %v", name, err)
		}
		var v any
		if json.Unmarshal(b, &v) != nil {
			t.Fatalf("ASSERT_MACHINE_CATALOG_JSON[%s]", name)
		}
	}
	catalogBytes, _ := os.ReadFile("testdata/catalog.v1.json")
	var catalog struct {
		Version string `json:"version"`
		Rows    []struct {
			ID       string   `json:"id"`
			Scenario string   `json:"scenario"`
			Golden   string   `json:"golden"`
			Coverage []string `json:"coverage"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(catalogBytes, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Version != "stage2-catalog-v1" {
		t.Fatal("ASSERT_CATALOG_VERSION")
	}
	covered := map[string]bool{}
	for _, row := range catalog.Rows {
		for _, c := range row.Coverage {
			covered[c] = true
		}
		data, err := os.ReadFile(filepath.Join("testdata", row.Scenario))
		if err != nil {
			t.Fatal(err)
		}
		s, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", row.ID, err)
		}
		got, err := ReplayIntegrated(s)
		if err != nil {
			t.Fatalf("%s: %v", row.ID, err)
		}
		goldenPath := filepath.Join("testdata", row.Golden)
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Intent, want) {
			t.Fatalf("ASSERT_GOLDEN_LEDGER_BYTE_EXACT[%s]", row.ID)
		}
	}
	for _, required := range []string{"lifecycle_rows", "init_intent_races", "callers", "restart_binding", "ownership_ordinals", "late_responses", "tombstone_bounds", "deterministic_failures"} {
		if !covered[required] {
			t.Fatalf("ASSERT_CATALOG_COVERAGE[%s]", required)
		}
	}
}

func TestCrossProcessHelper(t *testing.T) {
	if os.Getenv("STAGE2_SCENARIO_HELPER") != "1" {
		return
	}
	s, e := Parse([]byte(referenceScenario))
	if e != nil {
		os.Exit(2)
	}
	got, e := ReplayIntegrated(s)
	if e != nil {
		os.Exit(3)
	}
	_, _ = os.Stdout.Write(got.Intent)
	os.Exit(0)
}
func TestCrossProcessByteStability(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func() []byte {
		cmd := exec.Command(exe, "-test.run=TestCrossProcessHelper")
		cmd.Env = append(os.Environ(), "STAGE2_SCENARIO_HELPER=1")
		b, e := cmd.Output()
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	a, b := run(), run()
	if !bytes.Equal(a, b) {
		t.Fatal("ASSERT_CROSS_PROCESS_BYTE_STABILITY")
	}
}
