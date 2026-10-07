package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	loc "lsp-trace/internal/adr0007locationv2"
	v4 "lsp-trace/internal/adr0007locationv4"
	src "lsp-trace/internal/sourceadmissionv2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type Authorization struct {
	Schema           string `json:"schema"`
	State            string `json:"state"`
	DispatchAllowed  bool   `json:"dispatchAllowed"`
	LocationExecuted bool   `json:"locationExecuted"`
	LocationDesignGO bool   `json:"locationDesignGO"`
}
type Policy struct {
	Schema         string                  `json:"schema"`
	Digest         string                  `json:"digest"`
	Relations      []loc.Relation          `json:"relations"`
	MemberOutcomes []loc.MemberOutcomeKind `json:"memberOutcomes"`
}

func main() {
	root := flag.String("root", "", "new empty output root")
	freeze := flag.Bool("freeze", false, "include freeze inventory")
	flag.Parse()
	if *root == "" {
		panic("-root required")
	}
	if entries, e := os.ReadDir(*root); e == nil && len(entries) > 0 {
		panic("root must be empty")
	}
	must(os.MkdirAll(*root, 0755))
	writeDocs(*root)
	writeSchemas(*root)
	cases, e := v4.OracleCases()
	must(e)
	for i, name := range cases {
		writeCase(*root, i, name)
	}
	if *freeze {
		writeFreeze(*root)
	}
}
func writeDocs(r string) {
	docs := map[string]string{"DESIGN.md": "# ADR0007 Location Intersection Prospective v4\n\nDESIGN_FROZEN_NON_DISPATCHING candidate only. Runtime census is authoritative. No qualification, dispatch, semantic acceptance, or LOCATION_DESIGN_GO.\n", "PREDECESSORS.md": "# PREDECESSORS\n\n- blocked v3 implementation: `ecc9e1fa0b016a8bb9b959abfc5c3159dc52d8ef`\n- immutable v3 freeze: `sha256:2a47c53e61be31711f3e2486b1e000c325cd1d6c0f569cf8c2352297363ed081` (`47,769` bytes; 319 entries)\n- binding blocked audit: `location-intersection-v3-design-audit-2026-10-07` with terminal reason `ROOT_FREEZE_OPTIONAL_NESTED_FREEZE_EXCLUDED_AND_SELF_GENERATED_ORACLE`\n\nV4 preserves the 24-case semantics under v4 persisted identities, requires the exact root freeze manifest, includes nested same-name files in the complete census, and generates expected semantic results only from fixed literal oracle bytes.\n", "SOURCE_ADMISSION_BINDING.md": "# SOURCE ADMISSION BINDING\n\nUnchanged semantic v2 source admission: NFC paths, exact revision/file/object digest, object digest committed into admission digest, exact frozen prefix expansion.\n", "POLICY.md": "# POLICY\n\nRelations and outcomes are closed. One terminal row is retained per input ordinal; ranking is a projection.\n", "LIMITS.md": "# LIMITS\n\nV2 finite precharge and loop cancellation rules are carried forward unchanged.\n", "REVIEW_POLICY.md": "# REVIEW POLICY\n\nV2 strict custody and reviewer recomputation are carried forward unchanged.\n", "PRE_EXECUTION_AUDIT.md": "# PRE-EXECUTION AUDIT\n\nIndependent audit pending. DispatchAllowed=false; LocationExecuted=false; LocationDesignGO=false.\n"}
	for n, b := range docs {
		write(filepath.Join(r, n), []byte(b))
	}
	writeJSON(filepath.Join(r, "AUTHORIZATION_CANDIDATE.json"), Authorization{"lsp-trace.adr0007.location.authorization.v4", "DESIGN_FROZEN_NON_DISPATCHING", false, false, false})
	writeJSON(filepath.Join(r, "POLICY.json"), Policy{"lsp-trace.adr0007.location.policy.v4", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", []loc.Relation{loc.Intersects, loc.ContainedBy, loc.Contains}, []loc.MemberOutcomeKind{loc.Eligible, loc.Ineligible, loc.UnavailableLocation, loc.InvalidLocation, loc.DuplicateMember, loc.FilteredByPolicy}})
}
func writeSchemas(r string) {
	d := filepath.Join(r, "schemas")
	must(os.MkdirAll(d, 0755))
	vals := map[string]any{"authorization": Authorization{}, "policy": Policy{}, "limits": loc.LimitsArtifact{}, "binding": src.Binding{}, "condition": loc.Condition{}, "selected-source": src.SelectedSource{}, "selector": loc.Selector{}, "range": loc.Range{}, "request": loc.Request{}, "candidate": loc.Candidate{}, "witness": loc.Witness{}, "ledger": loc.Ledger{}, "counters": loc.Counters{}, "result": loc.Result{}, "raw": []loc.RawRecord{}, "attempt": []loc.Attempt{}, "key": loc.AttemptKey{}, "conflict": []loc.Conflict{}, "review": []loc.Review{}, "account": loc.Account{}, "fixture-envelope": loc.FixtureEnvelope{}, "freeze": v4.Freeze{}}
	for n, v := range vals {
		writeJSON(filepath.Join(d, n+".schema.json"), schemaFor(reflect.TypeOf(v), n))
	}
}
func schemaFor(t reflect.Type, name string) map[string]any {
	defs := map[string]any{}
	root := typeSchema(t, defs)
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["$id"] = "https://jaresty.github.io/lsp-trace/schemas/adr0007-location-v4-" + name + ".schema.json"
	if len(defs) > 0 {
		root["$defs"] = defs
	}
	return root
}
func typeSchema(t reflect.Type, defs map[string]any) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		p := map[string]any{}
		req := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			if tag[0] == "" || tag[0] == "-" {
				continue
			}
			s := typeSchema(f.Type, defs)
			applyField(tag[0], f.Type, s)
			p[tag[0]] = s
			if len(tag) < 2 || tag[1] != "omitempty" {
				req = append(req, tag[0])
			}
		}
		return map[string]any{"type": "object", "additionalProperties": false, "properties": p, "required": req}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{"type": "string", "contentEncoding": "base64", "minLength": 4}
		}
		return map[string]any{"type": "array", "items": typeSchema(t.Elem(), defs), "minItems": 0}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer", "minimum": 0}
	default:
		return map[string]any{}
	}
}
func applyField(n string, t reflect.Type, s map[string]any) {
	if strings.Contains(strings.ToLower(n), "digest") || n == "SHA256" || n == "sha256" {
		s["pattern"] = "^sha256:[0-9a-f]{64}$"
	}
	if n == "path" {
		s["minLength"] = 1
		s["pattern"] = "^[^\\\\:]+$"
	}
	if n == "schema" {
		s["minLength"] = 1
	}
	if t == reflect.TypeOf(loc.SelectorKind("")) {
		s["enum"] = []string{"EXACT_FILE", "RANGE_UNION", "PATH_PREFIX"}
	}
	if t == reflect.TypeOf(loc.Relation("")) {
		s["enum"] = []string{"INTERSECTS", "CONTAINED_BY", "CONTAINS"}
	}
	if t == reflect.TypeOf(loc.OperationOutcome("")) {
		s["enum"] = []string{"COMPLETE", "SOURCE_ADMISSION_UNAVAILABLE", "SOURCE_ADMISSION_MISMATCH", "INVALID_REQUEST", "INVALID_SELECTOR", "INVALID_RANGE", "CANCELLED", "TIMEOUT", "RESOURCE_LIMIT", "BACKEND_FAILURE", "POLICY_MISMATCH"}
	}
	if t == reflect.TypeOf(loc.MemberOutcomeKind("")) {
		s["enum"] = []string{"ELIGIBLE", "INELIGIBLE", "UNAVAILABLE_LOCATION", "INVALID_LOCATION", "DUPLICATE_MEMBER", "FILTERED_BY_POLICY"}
	}
	if n == "role" {
		s["enum"] = []string{"producer", "reviewer"}
	}
}
func writeCase(r string, i int, caseName string) {
	d := filepath.Join(r, "cases", caseName)
	must(os.MkdirAll(filepath.Join(d, "source"), 0755))
	for _, name := range []string{"CONDITION.json", "REQUEST.json", "SOURCE_ADMISSION.json", "EXPECTED_RESULT.json"} {
		b, e := v4.OracleRead(caseName, name)
		must(e)
		write(filepath.Join(d, name), b)
	}
	sources, e := v4.OracleSourceNames(caseName)
	must(e)
	for _, name := range sources {
		b, e := v4.OracleRead(caseName, "source/"+name)
		must(e)
		write(filepath.Join(d, "source", strings.TrimSuffix(name, ".txt")), b)
	}
	var condition loc.Condition
	must(loc.StrictDecode(read(filepath.Join(d, "CONDITION.json")), &condition))
	var expected loc.Result
	must(loc.StrictDecode(read(filepath.Join(d, "EXPECTED_RESULT.json")), &expected))
	raw := read(filepath.Join(d, "EXPECTED_RESULT.json"))
	key := loc.AttemptKey{CaseID: condition.CaseID, Role: "producer", Ordinal: 1}
	ledger, e := loc.Commit(loc.Ledger{}, key, raw)
	must(e)
	ledger, e = loc.AddReview(ledger, loc.Review{Key: loc.AttemptKey{CaseID: condition.CaseID, Role: "reviewer", Ordinal: 1}, SchemaValid: true, OutcomeMatches: true, AccountingComplete: true, WitnessesConcrete: true, BindingExact: true})
	must(e)
	writeJSON(filepath.Join(d, "EXPECTED_ATTEMPTS.json"), ledger.Attempts)
	writeJSON(filepath.Join(d, "EXPECTED_RAW.json"), ledger.Raw)
	writeJSON(filepath.Join(d, "EXPECTED_CONFLICTS.json"), ledger.Conflicts)
	writeJSON(filepath.Join(d, "EXPECTED_REVIEW.json"), ledger.Reviews)
	writeJSON(filepath.Join(d, "EXPECTED_ACCOUNT.json"), ledger.Account)
	name := strings.TrimPrefix(caseName, fmt.Sprintf("%02d-", i+1))
	writeJSON(filepath.Join(d, "FIXTURE.json"), loc.FixtureEnvelope{Schema: "lsp-trace.adr0007.location.fixture.v4", CaseID: condition.CaseID, Ordinal: i + 1, Name: name, Artifacts: []string{"CONDITION.json", "REQUEST.json", "SOURCE_ADMISSION.json", "EXPECTED_RESULT.json", "EXPECTED_ATTEMPTS.json", "EXPECTED_RAW.json", "EXPECTED_CONFLICTS.json", "EXPECTED_REVIEW.json", "EXPECTED_ACCOUNT.json"}})
}
func writeFreeze(r string) {
	es, e := v4.Census(r)
	must(e)
	writeJSON(filepath.Join(r, "FREEZE.json"), v4.Freeze{Schema: v4.FreezeSchema, State: "DESIGN_FROZEN_NON_DISPATCHING", FileCount: len(es), Files: es})
}
func writeJSON(p string, v any) {
	b, e := json.Marshal(v)
	must(e)
	b = bytes.ReplaceAll(b, []byte(".v2"), []byte(".v4"))
	write(p, append(b, '\n'))
}
func write(p string, b []byte) {
	must(os.MkdirAll(filepath.Dir(p), 0755))
	must(os.WriteFile(p, b, 0644))
}
func read(p string) []byte { b, e := os.ReadFile(p); must(e); return b }
func must(e error) {
	if e != nil {
		panic(e)
	}
}
