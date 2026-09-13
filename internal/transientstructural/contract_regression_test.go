package transientstructural

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/session"
)

var adrErrorMatrix = map[string][]string{
	"PREFLIGHT":      {"AMBIGUOUS_TARGET", "CANCELLED", "GENERATION_CHANGED", "INVALID_SERVER_RESPONSE", "RESOURCE_LIMIT", "TARGET_NOT_FOUND", "TIMEOUT", "UNSUPPORTED"},
	"TRAVERSAL":      {"CANCELLED", "GENERATION_CHANGED", "INVALID_SERVER_RESPONSE", "PARTIAL", "RESOURCE_LIMIT", "TIMEOUT", "TRUNCATED"},
	"ADMISSION":      {"CANCELLED", "GENERATION_CHANGED", "INVALID_SERVER_RESPONSE", "RESOURCE_LIMIT"},
	"ANALYSIS":       {"ANALYSIS_FAILED", "CANCELLED", "GENERATION_CHANGED", "RESOURCE_LIMIT", "TIMEOUT"},
	"DELIVERY_CHECK": {"CANCELLED", "GENERATION_CHANGED"},
}

func TestADR0006ExactClosedVocabularyAndMatrixDocument(t *testing.T) {
	var document struct {
		Phases  []string            `json:"phases"`
		Errors  map[string][]string `json:"errors"`
		Success struct {
			Phase  string   `json:"phase"`
			States []string `json:"states"`
		} `json:"success"`
	}
	if err := json.Unmarshal([]byte(lifecyclePolicyDocument), &document); err != nil {
		t.Fatalf("ASSERT_ADR0006_MATRIX_DOCUMENT_JSON: %v", err)
	}
	wantPhases := []string{"PREFLIGHT", "TRAVERSAL", "ADMISSION", "ANALYSIS", "DELIVERY_CHECK"}
	if !reflect.DeepEqual(document.Phases, wantPhases) {
		t.Fatalf("ASSERT_ADR0006_EXACT_PHASE_VOCABULARY: got=%v want=%v", document.Phases, wantPhases)
	}
	for phase := range document.Errors {
		sort.Strings(document.Errors[phase])
	}
	if !reflect.DeepEqual(document.Errors, adrErrorMatrix) {
		t.Fatalf("ASSERT_ADR0006_EXACT_ERROR_MATRIX: got=%v want=%v", document.Errors, adrErrorMatrix)
	}
	if document.Success.Phase != "DELIVERY_CHECK" || !reflect.DeepEqual(document.Success.States, []string{"COMPLETE", "EMPTY"}) {
		t.Fatalf("ASSERT_ADR0006_DELIVERY_ONLY_SUCCESS: %+v", document.Success)
	}

	phases, states := declaredStringConstants(t, "types.go")
	wantDeclaredPhases := append([]string(nil), wantPhases...)
	sort.Strings(wantDeclaredPhases)
	if !reflect.DeepEqual(phases, wantDeclaredPhases) {
		t.Fatalf("ASSERT_ADR0006_NO_EXTRA_PHASES: got=%v want=%v", phases, wantDeclaredPhases)
	}
	wantStates := []string{"AMBIGUOUS_TARGET", "ANALYSIS_FAILED", "CANCELLED", "COMPLETE", "EMPTY", "GENERATION_CHANGED", "INVALID_SERVER_RESPONSE", "PARTIAL", "RESOURCE_LIMIT", "TARGET_NOT_FOUND", "TIMEOUT", "TRUNCATED", "UNSUPPORTED"}
	if !reflect.DeepEqual(states, wantStates) {
		t.Fatalf("ASSERT_ADR0006_NO_EXTRA_STATES: got=%v want=%v", states, wantStates)
	}

	allStates := append([]string(nil), wantStates...)
	for _, phase := range wantPhases {
		allowed := map[string]bool{}
		for _, state := range adrErrorMatrix[phase] {
			allowed[state] = true
		}
		if phase == document.Success.Phase {
			for _, state := range document.Success.States {
				allowed[state] = true
			}
		}
		for _, state := range allStates {
			actual := legalTerminalPair(Phase(phase), TerminalState(state))
			if actual != allowed[state] {
				t.Fatalf("ASSERT_ADR0006_ALL_PAIR_NEGATIVE: phase=%s state=%s got=%v want=%v", phase, state, actual, allowed[state])
			}
		}
	}
}

func declaredStringConstants(t *testing.T, path string) (phases, states []string) {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		decl, ok := declaration.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			continue
		}
		for _, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Values) != 1 || len(value.Names) != 1 {
				continue
			}
			typeName, ok := value.Type.(*ast.Ident)
			literal, literalOK := value.Values[0].(*ast.BasicLit)
			if !ok || !literalOK || literal.Kind != token.STRING {
				continue
			}
			decoded, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			switch typeName.Name {
			case "Phase":
				phases = append(phases, decoded)
			case "TerminalState":
				states = append(states, decoded)
			}
		}
	}
	sort.Strings(phases)
	sort.Strings(states)
	return phases, states
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func TestResultStructuralPolicyAndClaimCeiling(t *testing.T) {
	resultType := reflect.TypeOf(Result{})
	want := map[string]reflect.Kind{
		"SchemaVersion":       reflect.String,
		"EvidenceClass":       reflect.String,
		"Authority":           reflect.Int,
		"SourceGraphComplete": reflect.String,
		"Retained":            reflect.Bool,
		"Replayable":          reflect.Bool,
		"PublicationEligible": reflect.Bool,
		"HydrationEligible":   reflect.Bool,
		"ClaimCeiling":        reflect.String,
	}
	for name, kind := range want {
		field, ok := resultType.FieldByName(name)
		if !ok || field.Type.Kind() != kind {
			t.Errorf("ASSERT_RESULT_STRUCTURAL_POLICY_FIELD: field=%s kind=%s", name, kind)
		}
	}
	const exactCeiling = "Under the named managed session generation, exact target, server responses, traversal bounds, and analysis policy, this bounded server-reported call graph has the reported structural properties."
	if claimCeiling != exactCeiling {
		t.Fatalf("ASSERT_ADR0006_EXACT_CLAIM_CEILING: got=%q", claimCeiling)
	}
}

func TestFrozenPolicyHasNoMutablePackageVariable(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "policy.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		decl, ok := declaration.(*ast.GenDecl)
		if !ok || decl.Tok != token.VAR {
			continue
		}
		for _, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range value.Names {
				if name.Name == "frozenPolicy" {
					t.Fatal("ASSERT_FROZEN_POLICY_IMMUTABLE: mutable package variable frozenPolicy")
				}
			}
		}
	}
}

func TestOccurrenceDedupReconcilesProducerAsymmetryAndPreservesSites(t *testing.T) {
	root := testNode("root", 1)
	first := testEdge(root, root, 10)
	first.CallSites = append(first.CallSites, graph.Range{Start: graph.Position{Line: 11}, End: graph.Position{Line: 11, Character: 1}})
	second := first
	second.RelationID = "optional-producer-specific-relation-id"
	projection, err := project("session", 1,
		traversalProjection{root: root.ID, nodes: []graph.Node{root}, edges: []graph.Edge{first}},
		traversalProjection{root: root.ID, nodes: []graph.Node{root}, edges: []graph.Edge{second}}, testBounds(), Accounting{})
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.occurrences) != 2 {
		t.Fatalf("ASSERT_SELF_CALL_CROSS_DIRECTION_DEDUP: got=%d want=2", len(projection.occurrences))
	}
	if projection.accounting.Occurrences != (AdmissionAccounting{Observed: 4, Admitted: 2, Omitted: 2}) || !hasOmission(projection.accounting, OmissionDuplicate) {
		t.Fatalf("ASSERT_DEDUP_OMISSION_ACCOUNTING: %+v", projection.accounting)
	}
	for _, occurrence := range projection.occurrences {
		if !reflect.DeepEqual(occurrence.Witnesses, []Witness{{Direction: DirectionIncoming, Depth: 1}, {Direction: DirectionOutgoing, Depth: 1}}) {
			t.Fatalf("ASSERT_SELF_CALL_BOTH_WITNESSES: %+v", occurrence)
		}
	}
}

func TestDeliveredNodeAccountingRejectsUnreachable(t *testing.T) {
	root, reached, unreachable := testNode("root", 1), testNode("reached", 2), testNode("unreachable", 3)
	projection, err := project("session", 1,
		traversalProjection{root: root.ID, nodes: []graph.Node{root, reached, unreachable}, edges: []graph.Edge{testEdge(root, reached, 10)}},
		traversalProjection{root: root.ID, nodes: []graph.Node{root}}, testBounds(), Accounting{})
	if err == nil {
		t.Fatal("ASSERT_UNREACHABLE_NODE_FAILS_ADMISSION")
	}
	if projection.accounting.Nodes != (AdmissionAccounting{Observed: 4, Admitted: 2, Omitted: 2}) || len(projection.nodes) != projection.accounting.Nodes.Admitted || !hasOmission(projection.accounting, OmissionInvalidResponse) || !reconciles(projection.accounting) {
		t.Fatalf("ASSERT_DELIVERED_NODE_ACCOUNTING: nodes=%+v delivered=%d omissions=%+v", projection.accounting.Nodes, len(projection.nodes), projection.accounting.Omissions)
	}
}

func TestProjectionInputPermutationDeterminism(t *testing.T) {
	root, a := testNode("root", 1), testNode("a", 2)
	self := testEdge(root, root, 10)
	self.CallSites = append(self.CallSites, graph.Range{Start: graph.Position{Line: 11}, End: graph.Position{Line: 11, Character: 1}})
	toA := testEdge(root, a, 12)
	first, err := project("session", 1,
		traversalProjection{root: root.ID, nodes: []graph.Node{root, a}, edges: []graph.Edge{self, toA}},
		traversalProjection{root: root.ID, nodes: []graph.Node{root}}, testBounds(), Accounting{})
	if err != nil {
		t.Fatal(err)
	}
	permutedSelf := self
	permutedSelf.CallSites = []graph.Range{self.CallSites[1], self.CallSites[0]}
	second, err := project("session", 1,
		traversalProjection{root: root.ID, nodes: []graph.Node{a, root}, edges: []graph.Edge{toA, permutedSelf}},
		traversalProjection{root: root.ID, nodes: []graph.Node{root}}, testBounds(), Accounting{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("ASSERT_INPUT_PERMUTATION_DETERMINISM: first=%+v second=%+v", first, second)
	}
}

func TestRecursiveResultPrivacyAndForbiddenDependencies(t *testing.T) {
	forbiddenJSONFields := map[string]bool{"uri": true, "path": true, "name": true, "detail": true, "range": true, "call_site": true, "data": true, "diagnostics": true, "arguments": true, "environment": true, "publication_root": true}
	walkJSONFields(t, reflect.TypeOf(Result{}), forbiddenJSONFields, map[reflect.Type]bool{})

	forbiddenImports := []string{"internal/acquisition", "internal/graphprovenance", "internal/publication", "internal/custody", "internal/hydration", "internal/passage", "internal/selector"}
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range parsed.Imports {
			name, _ := strconv.Unquote(imported.Path.Value)
			for _, forbidden := range forbiddenImports {
				if strings.Contains(name, forbidden) {
					t.Fatalf("ASSERT_FORBIDDEN_TRANSIENT_DEPENDENCY_IMPORT: file=%s import=%s", path, name)
				}
			}
		}
	}

	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "func Set") || strings.Contains(string(raw), "Observer") || strings.Contains(string(raw), "RawGraph") {
			t.Fatalf("ASSERT_NO_EXPORTED_OBSERVER_RAW_GRAPH_SEAM: %s", path)
		}
	}
}

func walkJSONFields(t *testing.T, typ reflect.Type, forbidden map[string]bool, seen map[reflect.Type]bool) {
	t.Helper()
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || seen[typ] {
		return
	}
	seen[typ] = true
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" {
			name = field.Name
		}
		if forbidden[name] {
			t.Fatalf("ASSERT_RECURSIVE_PRIVACY_FIELD_CORPUS: type=%s field=%s json=%s", typ, field.Name, name)
		}
		walkJSONFields(t, field.Type, forbidden, seen)
	}
}

func TestExactOmissionVocabularyAndEveryAccountingEquationMutation(t *testing.T) {
	wantReasons := []string{"CANCELLATION", "DEDUPLICATION", "DEPTH_BOUND", "MALFORMED_RESPONSE", "NODE_BOUND", "REQUEST_BOUND", "TIMEOUT", "UNSUPPORTED_RESPONSE"}
	gotReasons := declaredValuesOfType(t, "types.go", "OmissionReason")
	if !reflect.DeepEqual(gotReasons, wantReasons) {
		t.Fatalf("ASSERT_EXACT_OMISSION_REASON_VOCABULARY: got=%v want=%v", gotReasons, wantReasons)
	}
	base := Accounting{
		Requests:    RequestAccounting{Attempted: 3, Succeeded: 1, Failed: 1, Cancelled: 1},
		Preparation: PreparationAccounting{Attempted: 3, Returned: 1, Empty: 1, Failed: 1},
		Nodes:       AdmissionAccounting{Observed: 3, Admitted: 1, Rejected: 1, Omitted: 1},
		Occurrences: AdmissionAccounting{Observed: 3, Admitted: 1, Rejected: 1, Omitted: 1},
		Frontier:    FrontierAccounting{Observed: 2, Expanded: 1, Unexpanded: 1},
	}
	if !reconciles(base) {
		t.Fatal("ASSERT_ACCOUNTING_BASE_RECONCILES")
	}
	mutations := map[string]func(*Accounting){
		"requests":    func(a *Accounting) { a.Requests.Attempted++ },
		"preparation": func(a *Accounting) { a.Preparation.Attempted++ },
		"nodes":       func(a *Accounting) { a.Nodes.Observed++ },
		"occurrences": func(a *Accounting) { a.Occurrences.Observed++ },
		"frontier":    func(a *Accounting) { a.Frontier.Observed++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if reconciles(changed) {
				t.Fatalf("ASSERT_ACCOUNTING_EQUATION_MUTATION_REJECTED: %s", name)
			}
		})
	}
}

func declaredValuesOfType(t *testing.T, path, wantedType string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	for _, declaration := range parsed.Decls {
		decl, ok := declaration.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			continue
		}
		for _, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Values) != 1 {
				continue
			}
			typeName, ok := value.Type.(*ast.Ident)
			literal, literalOK := value.Values[0].(*ast.BasicLit)
			if !ok || typeName.Name != wantedType || !literalOK || literal.Kind != token.STRING {
				continue
			}
			decoded, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			values = append(values, decoded)
		}
	}
	sort.Strings(values)
	return values
}

func TestLifecycleFailureMappingAndCaptureSupplyFalse(t *testing.T) {
	if terminalForSessionFailure(session.LifecycleConflict) != StateCancelled {
		t.Fatal("ASSERT_LIFECYCLE_CONFLICT_NOT_RESOURCE_LIMIT")
	}
	if terminalForSessionFailure(session.ResourceExhausted) != StateResourceLimit || terminalForSessionFailure(session.StaleGeneration) != StateGenerationChanged || terminalForSessionFailure(session.RequestTimeout) != StateTimeout || terminalForSessionFailure(session.RequestCancelled) != StateCancelled {
		t.Fatal("ASSERT_LIFECYCLE_TIMEOUT_CANCEL_GENERATION_RESOURCE_MAPPING")
	}
	raw, err := os.ReadFile("executor.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if strings.Count(source, "CaptureSupply: false") != 1 || strings.Contains(source, "CaptureSupply: true") {
		t.Fatal("ASSERT_CAPTURE_SUPPLY_ALWAYS_FALSE")
	}
}
