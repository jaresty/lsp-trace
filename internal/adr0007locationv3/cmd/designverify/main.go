package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	loc "lsp-trace/internal/adr0007locationv2"
	v3 "lsp-trace/internal/adr0007locationv3"
	src "lsp-trace/internal/sourceadmissionv2"
	"os"
	"path/filepath"
	"strings"
)

type control struct{ c, d bool }

func (x control) Cancelled() bool        { return x.c }
func (x control) DeadlineExceeded() bool { return x.d }
func main() {
	root := flag.String("root", "", "v3 root")
	flag.Parse()
	if *root == "" {
		fail("root")
	}
	expected := loc.BuildDesignCases()
	dirs, e := os.ReadDir(filepath.Join(*root, "cases"))
	must(e)
	if len(dirs) != 24 {
		fail("case census")
	}
	schemaDir := filepath.Join(*root, "schemas")
	schemas, e := os.ReadDir(schemaDir)
	must(e)
	compiled := map[string]*jsonschema.Schema{}
	for _, de := range schemas {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".schema.json") {
			continue
		}
		b := read(filepath.Join(schemaDir, de.Name()))
		var doc any
		must(json.Unmarshal(b, &doc))
		id := doc.(map[string]any)["$id"].(string)
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		must(c.AddResource(id, doc))
		s, e := c.Compile(id)
		must(e)
		compiled[strings.TrimSuffix(de.Name(), ".schema.json")] = s
	}
	if len(compiled) != 22 {
		fail(fmt.Sprintf("schema census=%d", len(compiled)))
	}
	artifactSchema := map[string]string{"CONDITION.json": "condition", "REQUEST.json": "request", "SOURCE_ADMISSION.json": "binding", "EXPECTED_RESULT.json": "result", "EXPECTED_ATTEMPTS.json": "attempt", "EXPECTED_RAW.json": "raw", "EXPECTED_CONFLICTS.json": "conflict", "EXPECTED_REVIEW.json": "review", "EXPECTED_ACCOUNT.json": "account", "FIXTURE.json": "fixture-envelope"}
	actual := 0
	for i, de := range dirs {
		if !de.IsDir() {
			fail("non-directory case")
		}
		want := fmt.Sprintf("%02d-%s", i+1, expected[i].Name)
		if de.Name() != want {
			fail("missing extra duplicate reordered or substituted case: " + de.Name() + " want " + want)
		}
		d := filepath.Join(*root, "cases", de.Name())
		for file, sn := range artifactSchema {
			validate(compiled[sn], read(filepath.Join(d, file)), file)
		}
		var cond loc.Condition
		decode(read(filepath.Join(d, "CONDITION.json")), &cond)
		var req loc.Request
		decode(read(filepath.Join(d, "REQUEST.json")), &req)
		if cond.CaseID != fmt.Sprintf("%02d", i+1) || req.ID != cond.CaseID {
			fail("condition/request identity " + de.Name())
		}
		var adm loc.Admission
		decode(read(filepath.Join(d, "SOURCE_ADMISSION.json")), &adm)
		req.Schema = loc.Schema
		adm.Schema = src.Schema
		for _, s := range adm.Sources {
			p := filepath.Join(d, "source", strings.ReplaceAll(s.Path, "/", "__"))
			if !bytes.Equal(read(p), s.Bytes) {
				fail("source bytes " + de.Name())
			}
		}
		l := cond.Limits
		var ap *loc.Admission
		if cond.AdmissionAvailable {
			ap = &adm
		}
		got := loc.Evaluate(req, ap, loc.Options{Limits: loc.Limits{MaxRequestBytes: l.MaxRequestBytes, MaxMembers: l.MaxMembers, MaxPaths: l.MaxPaths, MaxSelectorRanges: l.MaxSelectorRanges, MaxMemberRanges: l.MaxMemberRanges, MaxPrefixExpansion: l.MaxPrefixExpansion, MaxWitnesses: l.MaxWitnesses, MaxSourceBytes: l.MaxSourceBytes, MaxOutputBytes: l.MaxOutputBytes, MaxWork: l.MaxWork}, Control: control{cond.Cancel, cond.Deadline}, ExpectedPolicyDigest: req.PolicyDigest})
		got.Schema = strings.ReplaceAll(got.Schema, ".v2", ".v3")
		gb, _ := loc.CanonicalJSON(got)
		wantb := read(filepath.Join(d, "EXPECTED_RESULT.json"))
		if !bytes.Equal(gb, wantb) {
			fail("runtime projection " + de.Name())
		}
		verifyAccounting(got)
		key := loc.AttemptKey{CaseID: cond.CaseID, Role: "producer", Ordinal: 1}
		ledger, _ := loc.Commit(loc.Ledger{}, key, gb)
		ledger, _ = loc.AddReview(ledger, loc.Review{Key: loc.AttemptKey{CaseID: cond.CaseID, Role: "reviewer", Ordinal: 1}, SchemaValid: true, OutcomeMatches: true, AccountingComplete: true, WitnessesConcrete: true, BindingExact: true})
		exact(d, "EXPECTED_ATTEMPTS.json", ledger.Attempts)
		exact(d, "EXPECTED_RAW.json", ledger.Raw)
		exact(d, "EXPECTED_CONFLICTS.json", ledger.Conflicts)
		exact(d, "EXPECTED_REVIEW.json", ledger.Reviews)
		exact(d, "EXPECTED_ACCOUNT.json", ledger.Account)
		actual++
	}
	verifyTop(*root, compiled)
	fmt.Printf("V3_VERIFY_OK cases=%d schemas=%d actual_evaluations=%d\n", len(dirs), len(compiled), actual)
}
func verifyTop(r string, s map[string]*jsonschema.Schema) {
	validate(s["authorization"], read(filepath.Join(r, "AUTHORIZATION_CANDIDATE.json")), "authorization")
	validate(s["policy"], read(filepath.Join(r, "POLICY.json")), "policy")
	if b, e := os.ReadFile(filepath.Join(r, "FREEZE.json")); e == nil {
		validate(s["freeze"], b, "freeze")
		var f v3.Freeze
		decode(b, &f)
		if e := v3.Verify(r, f); e != nil {
			fail(e.Error())
		}
	}
}
func exact(d, n string, v any) {
	b, e := loc.CanonicalJSON(v)
	must(e)
	b = bytes.ReplaceAll(b, []byte(".v2"), []byte(".v3"))
	if !bytes.Equal(b, read(filepath.Join(d, n))) {
		fail("custody projection " + n)
	}
}
func verifyAccounting(r loc.Result) {
	if r.Outcome != loc.Complete {
		return
	}
	c := r.Counters
	if len(r.Members) != c.Input || c.Input != c.Eligible+c.Ineligible+c.UnavailableLocation+c.InvalidLocation+c.DuplicateMember+c.FilteredByPolicy {
		fail("accounting partition")
	}
	w := 0
	for i, m := range r.Members {
		if m.Ordinal != i {
			fail("ordinal")
		}
		w += len(m.Witnesses)
	}
	if w != c.Witnesses || len(r.Ranked) != c.Ranked {
		fail("witness/ranked accounting")
	}
}
func validate(s *jsonschema.Schema, b []byte, n string) {
	var v any
	must(json.Unmarshal(b, &v))
	if e := s.Validate(v); e != nil {
		fail(n + ": " + e.Error())
	}
}
func decode(b []byte, v any) { must(loc.StrictDecode(b, v)) }
func read(p string) []byte   { b, e := os.ReadFile(p); must(e); return b }
func must(e error) {
	if e != nil {
		panic(e)
	}
}
func fail(s string) { panic(s) }
