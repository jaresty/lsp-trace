package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	loc "lsp-trace/internal/adr0007locationv2"
	src "lsp-trace/internal/sourceadmissionv2"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
type FreezeEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}
type Freeze struct {
	Schema           string        `json:"schema"`
	State            string        `json:"state"`
	DispatchAllowed  bool          `json:"dispatchAllowed"`
	LocationExecuted bool          `json:"locationExecuted"`
	LocationDesignGO bool          `json:"locationDesignGO"`
	Files            []FreezeEntry `json:"files"`
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
	cases := loc.BuildDesignCases()
	for i, c := range cases {
		writeCase(*root, i, c)
	}
	if *freeze {
		writeFreeze(*root)
	}
}
func writeDocs(r string) {
	docs := map[string]string{"DESIGN.md": "# ADR0007 Location Intersection Prospective v2\n\nDESIGN_FROZEN_NON_DISPATCHING candidate only. No qualification, dispatch, semantic acceptance, or LOCATION_DESIGN_GO.\n", "PREDECESSORS.md": "# PREDECESSORS\n\n- source admission history: `2f21811f`\n- Location v1 history: `945bf7c2`\n- immutable blocked predecessor: `v1blocked@13638e0deda0e3e3021a5b0c877280dbcfc16b45`\n\nCurrent `sourceadmissionv2` admits explicit NFC canonical paths, exact revision and UTF-8 bytes, verifies file and object SHA-256, includes both digests in the admission digest, rejects duplicates and returns defensive copies. It replaces the narrower current `sourceadmissionv1`, whose admission digest omitted ObjectDigest and whose path check did not enforce NFC.\n", "SOURCE_ADMISSION_BINDING.md": "# SOURCE ADMISSION BINDING\n\nEvery selector path is admitted. PATH_PREFIX FrozenPaths is sorted, duplicate-free, and exactly equals admitted paths under the prefix.\n", "POLICY.md": "# POLICY\n\nRelations and outcomes are closed. One terminal row is retained per input ordinal; ranking is a projection.\n", "LIMITS.md": "# LIMITS\n\nFinite limits cover request/output/source bytes, members, all path populations, selector/member ranges, prefix expansion, witnesses and work. Precharge includes cross-products, sorting and serialization.\n", "REVIEW_POLICY.md": "# REVIEW POLICY\n\nReview verdict is the recomputed conjunction of schema, outcome, accounting, witness and binding booleans.\n", "PRE_EXECUTION_AUDIT.md": "# PRE-EXECUTION AUDIT\n\nIndependent audit pending. DispatchAllowed=false; LocationExecuted=false; LocationDesignGO=false.\n"}
	for n, b := range docs {
		write(filepath.Join(r, n), []byte(b))
	}
	writeJSON(filepath.Join(r, "AUTHORIZATION_CANDIDATE.json"), Authorization{"lsp-trace.adr0007.location.authorization.v2", "DESIGN_FROZEN_NON_DISPATCHING", false, false, false})
	writeJSON(filepath.Join(r, "POLICY.json"), Policy{"lsp-trace.adr0007.location.policy.v2", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", []loc.Relation{loc.Intersects, loc.ContainedBy, loc.Contains}, []loc.MemberOutcomeKind{loc.Eligible, loc.Ineligible, loc.UnavailableLocation, loc.InvalidLocation, loc.DuplicateMember, loc.FilteredByPolicy}})
}
func writeSchemas(r string) {
	d := filepath.Join(r, "schemas")
	must(os.MkdirAll(d, 0755))
	vals := map[string]any{"authorization": Authorization{}, "policy": Policy{}, "limits": loc.LimitsArtifact{}, "binding": src.Binding{}, "condition": loc.Condition{}, "selected-source": src.SelectedSource{}, "selector": loc.Selector{}, "range": loc.Range{}, "request": loc.Request{}, "candidate": loc.Candidate{}, "witness": loc.Witness{}, "ledger": loc.Ledger{}, "counters": loc.Counters{}, "result": loc.Result{}, "raw": []loc.RawRecord{}, "attempt": []loc.Attempt{}, "key": loc.AttemptKey{}, "conflict": []loc.Conflict{}, "review": []loc.Review{}, "account": loc.Account{}, "fixture-envelope": loc.FixtureEnvelope{}, "freeze": Freeze{}}
	for n, v := range vals {
		writeJSON(filepath.Join(d, n+".schema.json"), schemaFor(reflect.TypeOf(v), n))
	}
}
func schemaFor(t reflect.Type, name string) map[string]any {
	defs := map[string]any{}
	root := typeSchema(t, defs)
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["$id"] = "https://jaresty.github.io/lsp-trace/schemas/adr0007-location-v2-" + name + ".schema.json"
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
	if strings.Contains(strings.ToLower(n), "digest") {
		s["pattern"] = "^sha256:[0-9a-f]{64}$"
	}
	if n == "path" {
		s["minLength"] = 1
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
func writeCase(r string, i int, c loc.DesignCase) {
	id := fmt.Sprintf("%02d-%s", i+1, c.Name)
	d := filepath.Join(r, "cases", id)
	must(os.MkdirAll(filepath.Join(d, "source"), 0755))
	writeJSON(filepath.Join(d, "CONDITION.json"), c.Condition)
	writeJSON(filepath.Join(d, "REQUEST.json"), c.Request)
	writeJSON(filepath.Join(d, "SOURCE_ADMISSION.json"), c.Admission)
	for _, s := range c.Admission.Sources {
		write(filepath.Join(d, "source", strings.ReplaceAll(s.Path, "/", "__")), s.Bytes)
	}
	writeJSON(filepath.Join(d, "EXPECTED_RESULT.json"), c.Expected)
	key := loc.AttemptKey{CaseID: c.Condition.CaseID, Role: "producer", Ordinal: 1}
	raw, _ := loc.CanonicalJSON(c.Expected)
	ledger, _ := loc.Commit(loc.Ledger{}, key, raw)
	ledger, _ = loc.AddReview(ledger, loc.Review{Key: loc.AttemptKey{CaseID: c.Condition.CaseID, Role: "reviewer", Ordinal: 1}, SchemaValid: true, OutcomeMatches: true, AccountingComplete: true, WitnessesConcrete: true, BindingExact: true})
	writeJSON(filepath.Join(d, "EXPECTED_ATTEMPTS.json"), ledger.Attempts)
	writeJSON(filepath.Join(d, "EXPECTED_RAW.json"), ledger.Raw)
	writeJSON(filepath.Join(d, "EXPECTED_CONFLICTS.json"), ledger.Conflicts)
	writeJSON(filepath.Join(d, "EXPECTED_REVIEW.json"), ledger.Reviews)
	writeJSON(filepath.Join(d, "EXPECTED_ACCOUNT.json"), ledger.Account)
	writeJSON(filepath.Join(d, "FIXTURE.json"), loc.FixtureEnvelope{Schema: "lsp-trace.adr0007.location.fixture.v2", CaseID: c.Condition.CaseID, Ordinal: i + 1, Name: c.Name, Artifacts: []string{"CONDITION.json", "REQUEST.json", "SOURCE_ADMISSION.json", "EXPECTED_RESULT.json", "EXPECTED_ATTEMPTS.json", "EXPECTED_RAW.json", "EXPECTED_CONFLICTS.json", "EXPECTED_REVIEW.json", "EXPECTED_ACCOUNT.json"}})
}
func writeFreeze(r string) {
	var paths []string
	filepath.Walk(r, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if !i.IsDir() && filepath.Base(p) != "FREEZE.json" {
			rel, _ := filepath.Rel(r, p)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(paths)
	es := []FreezeEntry{}
	for _, p := range paths {
		b := read(filepath.Join(r, p))
		s := sha256.Sum256(b)
		es = append(es, FreezeEntry{p, "sha256:" + hex.EncodeToString(s[:]), len(b)})
	}
	writeJSON(filepath.Join(r, "FREEZE.json"), Freeze{"lsp-trace.adr0007.location.freeze.v2", "DESIGN_FROZEN_NON_DISPATCHING", false, false, false, es})
}
func writeJSON(p string, v any) { b, e := json.Marshal(v); must(e); write(p, append(b, '\n')) }
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
