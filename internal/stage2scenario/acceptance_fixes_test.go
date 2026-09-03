package stage2scenario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func scenarioWithSteps(steps ...string) string {
	return `{"version":"stage2-scenario-v1","steps":[` + strings.Join(steps, ",") + `]}`
}

func requireParseError(t *testing.T, raw, contains string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ASSERT_EXACT_TUPLE_TOTAL_NO_PANIC: panic=%v", r)
		}
	}()
	_, err := Parse([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), contains) {
		t.Fatalf("want error containing %q, got %v", contains, err)
	}
}

func replayText(t *testing.T, raw string) string {
	t.Helper()
	s, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReplayIntegrated(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(got.Intent)
}

func TestStructuralErrorsPrecedeSemanticsForEveryOp(t *testing.T) {
	valid := map[string]string{
		"startup":            `"session":"s","generation":1,"outcome":"ok"`,
		"initialize":         `"session":"s","generation":1,"outcome":"ok"`,
		"request":            `"session":"s","generation":1,"request":"r"`,
		"child":              `"session":"s","generation":1,"request":"missing","lsp_request":"l","child":"c","ordinal":1`,
		"respond":            `"session":"s","generation":1,"lsp_request":"missing","child":"c"`,
		"timeout":            `"session":"s","generation":1,"request":"missing"`,
		"cancel":             `"session":"s","generation":1,"request":"missing"`,
		"late_response":      `"session":"s","generation":1,"lsp_request":"missing","child":"c"`,
		"crash":              `"session":"s","generation":1,"outcome":"ok"`,
		"poison":             `"session":"s","generation":1,"outcome":"ok"`,
		"lifecycle_register": `"session":"s","generation":1,"state":"READY","expect":{}`,
		"lifecycle":          `"session":"s","generation":1,"operation":"STATUS","caller":"c","expect":{}`,
		"lifecycle_complete": `"session":"s","intent_id":"i","expect":{}`,
		"lifecycle_detach":   `"session":"s","intent_id":"i","caller":"c","outcome":"REQUEST_TIMEOUT","expect":{}`,
		"admit":              `"session":"s","expect":{}`,
		"evict_complete":     `"intent_id":"i","success":true,"expect":{}`,
	}
	for op, fields := range valid {
		t.Run(op, func(t *testing.T) {
			raw := fmt.Sprintf(`{"version":"stage2-scenario-v1","steps":[{"op":%q,%s,"zzz":true}]}`, op, fields)
			requireParseError(t, raw, "structural stage2-scenario-v1")
		})
	}
}

func TestLSPIdentityScopedBySessionAndGeneration(t *testing.T) {
	raw := scenarioWithSteps(
		`{"op":"request","session":"a","generation":1,"request":"r1"}`,
		`{"op":"child","session":"a","generation":1,"request":"r1","lsp_request":"l","child":"c1","ordinal":1}`,
		`{"op":"request","session":"a","generation":2,"request":"r2"}`,
		`{"op":"child","session":"a","generation":2,"request":"r2","lsp_request":"l","child":"c2","ordinal":1}`,
		`{"op":"request","session":"b","generation":1,"request":"r3"}`,
		`{"op":"child","session":"b","generation":1,"request":"r3","lsp_request":"l","child":"c3","ordinal":1}`,
	)
	if _, err := Parse([]byte(raw)); err != nil {
		t.Fatalf("ASSERT_LSP_ID_SCOPE_SESSION_GENERATION: %v", err)
	}
}

func TestExactTuplePrerequisitesAndNoPanic(t *testing.T) {
	base := []string{
		`{"op":"request","session":"s","generation":1,"request":"r"}`,
		`{"op":"child","session":"s","generation":1,"request":"r","lsp_request":"l","child":"c","ordinal":1}`,
	}
	for name, step := range map[string]string{
		"respond-session":   `{"op":"respond","session":"wrong","generation":1,"lsp_request":"l","child":"c"}`,
		"respond-child":     `{"op":"respond","session":"s","generation":1,"lsp_request":"l","child":"wrong"}`,
		"cancel-session":    `{"op":"cancel","session":"wrong","generation":1,"request":"r"}`,
		"cancel-generation": `{"op":"cancel","session":"s","generation":2,"request":"r"}`,
		"timeout-unknown":   `{"op":"timeout","session":"s","generation":1,"request":"missing"}`,
	} {
		t.Run(name, func(t *testing.T) { requireParseError(t, scenarioWithSteps(append(base, step)...), "prerequisite") })
	}
}

func TestCancelWriteFailureAndTerminalSequence(t *testing.T) {
	raw := scenarioWithSteps(
		`{"op":"request","session":"s","generation":1,"request":"r"}`,
		`{"op":"child","session":"s","generation":1,"request":"r","lsp_request":"l","child":"c","ordinal":1}`,
		`{"op":"cancel","session":"s","generation":1,"request":"r"}`,
		`{"op":"cancel_write_failed","session":"s","generation":1,"request":"r","lsp_request":"l","child":"c"}`,
	)
	text := replayText(t, raw)
	if !strings.Contains(text, `"cancel_state":"WRITE_FAILED"`) || !strings.Contains(text, `"kind":"cancel_write_failed"`) || !strings.Contains(text, `"terminal_event_seq":`) {
		t.Fatalf("ASSERT_WRITE_FAILED_EXECUTABLE_TERMINAL_SEQUENCE: %s", text)
	}
}

func TestResponseAssignsTerminalSequence(t *testing.T) {
	text := replayText(t, scenarioWithSteps(
		`{"op":"request","session":"s","generation":1,"request":"r"}`,
		`{"op":"child","session":"s","generation":1,"request":"r","lsp_request":"l","child":"c","ordinal":1}`,
		`{"op":"respond","session":"s","generation":1,"lsp_request":"l","child":"c"}`,
	))
	if !strings.Contains(text, `"response_event_seq":3,"terminal_event_seq":3`) {
		t.Fatalf("ASSERT_RESPONSE_TERMINAL_EVENT_SEQUENCE: %s", text)
	}
}

func decodeLedgerRecords(t *testing.T, text string) []record {
	t.Helper()
	var records []record
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		records = append(records, r)
	}
	return records
}

func historyAndCensus(t *testing.T, raw string) ([]sessionRecord, censusRecord) {
	t.Helper()
	var history []sessionRecord
	var census censusRecord
	for _, r := range decodeLedgerRecords(t, replayText(t, raw)) {
		if r.Session != nil {
			history = append(history, *r.Session)
		}
		if r.Census != nil {
			census = *r.Census
		}
	}
	return history, census
}

func TestCompleteGenerationHistoryAndCoherentCensus(t *testing.T) {
	history, census := historyAndCensus(t, referenceScenario)
	if len(history) != 2 || history[0].Generation != 1 || history[1].Generation != 2 {
		t.Fatalf("ASSERT_COMPLETE_RETAINED_GENERATIONS: %+v", history)
	}
	if history[1].EventSeq <= history[0].EventSeq {
		t.Fatalf("ASSERT_WITHIN_SESSION_FINAL_EVENT_RETAINED: %+v", history)
	}
	if census.Generations != 2 {
		t.Fatalf("ASSERT_CENSUS_COUNTS_GENERATIONS: %+v", census)
	}
	if census.LiveSessions != 1 || census.TerminalSessions != 0 {
		t.Fatalf("ASSERT_LATEST_READY_GENERATION_IS_LIVE_NOT_TERMINAL: %+v", census)
	}
	if census.TerminalSessions == census.Generations-1 {
		t.Fatalf("ASSERT_HISTORICAL_GENERATIONS_DO_NOT_COUNT_AS_TERMINAL_SESSIONS: %+v", census)
	}
	if census.LifecycleLedgerTruncated || census.LifecycleLedgerOmitted != 0 {
		t.Fatalf("ASSERT_BOUNDED_LEDGER_TRUNCATION_METADATA: %+v", census)
	}
}

func TestCrossSessionGenerationHistoryUsesStableKeys(t *testing.T) {
	raw := scenarioWithSteps(
		`{"op":"lifecycle_register","session":"a","generation":1,"state":"READY","expect":{"state":"READY","generation":1,"outcome":"COMPLETE","operation_status":"SUCCEEDED"}}`,
		`{"op":"lifecycle","session":"a","generation":1,"operation":"STATUS","caller":"c1","expect":{"state":"READY","generation":1,"outcome":"COMPLETE","operation_status":"SUCCEEDED"}}`,
		`{"op":"lifecycle","session":"a","generation":1,"operation":"STATUS","caller":"c2","expect":{"state":"READY","generation":1,"outcome":"COMPLETE","operation_status":"SUCCEEDED"}}`,
		`{"op":"lifecycle_register","session":"b","generation":1,"state":"READY","expect":{"state":"READY","generation":1,"outcome":"COMPLETE","operation_status":"SUCCEEDED"}}`,
	)
	history, census := historyAndCensus(t, raw)
	if len(history) != 2 || history[0].SessionID != "a" || history[1].SessionID != "b" {
		t.Fatalf("ASSERT_CROSS_SESSION_HISTORY_STABLE_KEY_ORDER: %+v", history)
	}
	if history[0].EventSeq <= history[1].EventSeq {
		t.Fatalf("ASSERT_CROSS_SESSION_ORDER_PERTURBATION: local sequences did not oppose stable key order: %+v", history)
	}
	if census.LiveSessions != 2 || census.TerminalSessions != 0 || census.Generations != 2 {
		t.Fatalf("ASSERT_OVERLAY_EQUIVALENT_CENSUS: %+v", census)
	}
}

func TestNoUnexplainedExportedHelpers(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "scenario.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]bool{"Replay": true, "EncodeEvents": true, "Inspect": true}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && forbidden[fn.Name.Name] {
			t.Fatalf("ASSERT_NO_UNEXPLAINED_EXPORTED_HELPERS: %s", fn.Name.Name)
		}
	}
	file, err = parser.ParseFile(fset, "schemas.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "LedgerRecordTypes" {
			t.Fatal("ASSERT_NO_UNEXPLAINED_EXPORTED_HELPERS: LedgerRecordTypes")
		}
	}
}

func TestTombstoneCountExpiryAndConsumedRetention(t *testing.T) {
	steps := []string{}
	for n := 1; n <= 5; n++ {
		steps = append(steps,
			fmt.Sprintf(`{"op":"request","session":"s","generation":1,"request":"r%d"}`, n),
			fmt.Sprintf(`{"op":"child","session":"s","generation":1,"request":"r%d","lsp_request":"l%d","child":"c%d","ordinal":1}`, n, n, n),
			fmt.Sprintf(`{"op":"cancel","session":"s","generation":1,"request":"r%d"}`, n),
		)
	}
	steps = append(steps, `{"op":"late_response","session":"s","generation":1,"lsp_request":"l5","child":"c5"}`)
	text := replayText(t, scenarioWithSteps(steps...))
	if !strings.Contains(text, `"tombstones_retained":`) || !strings.Contains(text, `"tombstones_evicted":`) || !strings.Contains(text, `"tombstones_consumed":1`) {
		t.Fatalf("ASSERT_BOUNDED_TOMBSTONE_ACCOUNTING: %s", text)
	}
}

func TestModeledStateChangesOnlyOnExactManagerSuccess(t *testing.T) {
	i, err := NewInterpreter(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := i.step(0, Step{Op: "admit", Session: "first", Expect: &ManagerExpectation{Kind: "FREE", SessionID: "first"}}); err != nil {
		t.Fatal(err)
	}
	if err := i.step(1, Step{Op: "admit", Session: "waiting", Expect: &ManagerExpectation{Kind: "EVICT", SessionID: "waiting", Victim: "first", Reservation: "r1"}}); err != nil {
		t.Fatal(err)
	}
	if _, exists := i.sessions["waiting"]; exists {
		t.Fatal("ASSERT_MODELED_ADMISSION_ONLY_ON_EXACT_MANAGER_SUCCESS: eviction reservation modeled as live admission")
	}
	if !i.reservations["r1"] {
		t.Fatal("ASSERT_MODELED_EVICTION_RESERVATION_ON_EXACT_MANAGER_RESULT")
	}
}

func TestManagerActionVocabularySchemaParityBidirectional(t *testing.T) {
	manager, err := os.ReadFile("../session/manager.go")
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]bool{}
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`appendLifecycle\([^\n]*?"([a-z-]+)"`),
		regexp.MustCompile(`observeLifecycleDecision\([^\n]*?"([a-z-]+)"`),
	} {
		for _, match := range re.FindAllSubmatch(manager, -1) {
			actions[string(match[1])] = true
		}
	}
	schema, err := os.ReadFile("testdata/schemas/stage2-ledger-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(schema, &doc); err != nil {
		t.Fatal(err)
	}
	defs := doc["$defs"].(map[string]any)
	intent := defs["intent"].(map[string]any)
	props := intent["properties"].(map[string]any)
	enum := props["action"].(map[string]any)["enum"].([]any)
	schemaActions := map[string]bool{}
	for _, action := range enum {
		schemaActions[action.(string)] = true
	}
	var missing, surplus []string
	for action := range actions {
		if !schemaActions[action] {
			missing = append(missing, action)
		}
	}
	for action := range schemaActions {
		if !actions[action] {
			surplus = append(surplus, action)
		}
	}
	sort.Strings(missing)
	sort.Strings(surplus)
	if len(missing) != 0 || len(surplus) != 0 {
		t.Fatalf("ASSERT_MANAGER_ACTION_SCHEMA_PARITY_BIDIRECTIONAL: missing=%v surplus=%v", missing, surplus)
	}
}

func TestCatalogAcceptanceAndCapacityClaimsAreHonest(t *testing.T) {
	raw, err := os.ReadFile("testdata/catalog.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"acceptance_state":"open"`)) {
		t.Fatal("ASSERT_CATALOG_ACCEPTANCE_OPEN_WHILE_PARTIAL_OR_RUNTIME_GATE")
	}
	var doc struct {
		Rows []struct {
			Scenario string
			Coverage []string
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, row := range doc.Rows {
		for _, coverage := range row.Coverage {
			if coverage != "capacity_evict" {
				continue
			}
			b, err := os.ReadFile("testdata/" + row.Scenario)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(b, []byte(`"op":"admit"`)) && !bytes.Contains(b, []byte(`"op":"evict_complete"`)) {
				t.Fatalf("ASSERT_NO_CAPACITY_CLAIM_WITHOUT_EXERCISING_SCENARIO: %s", row.Scenario)
			}
		}
	}
	if !bytes.Contains(raw, []byte(`"status":"runtime_gate"`)) || !bytes.Contains(raw, []byte(`RUNTIME_GATE_PRODUCTION_EQUIVALENT_FAKE_SERVER_PATH`)) {
		t.Fatal("ASSERT_RUNTIME_GATE_REMAINS_OPEN")
	}
	tmp := t.TempDir()
	if err := os.CopyFS(tmp, os.DirFS("testdata")); err != nil {
		t.Fatal(err)
	}
	perturbed := bytes.Replace(raw, []byte(`"acceptance_state":"open"`), []byte(`"acceptance_state":"accepted"`), 1)
	if err := os.WriteFile(tmp+"/catalog.v1.json", perturbed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCorpus(tmp); err == nil || !strings.Contains(err.Error(), "must remain open") {
		t.Fatalf("ASSERT_CATALOG_ACCEPTANCE_PERTURBATION_REJECTED: %v", err)
	}
}
